package eventaccounting

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
	"github.com/lease-management-system/core-service/internal/services/ifrs16"
)

func TestCommitPostgresIsAtomicAndIdempotent(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to Postgres: %v", err)
	}
	t.Cleanup(pool.Close)

	fixture := createEventAccountingFixture(t, ctx, pool)
	mcRepo := repository.NewMonthlyClosingRepository(pool)
	eventRepo := repository.NewEventRepository(pool)
	service := NewPersistenceService(pool, mcRepo, eventRepo, audit.NewLogger(repository.NewAuditRepository(pool)))
	result := calculateFixtureResult(t, fixture)

	// The audit insert is the final write and casts ChangedBy to UUID. An invalid
	// actor therefore proves that all prior accounting writes and the event link
	// roll back when the last persistence step fails.
	if adjustment, err := service.Commit(ctx, result, audit.Metadata{ChangedBy: "not-a-uuid"}); err == nil || adjustment != nil {
		t.Fatalf("late failure = adjustment %#v, error %v; want nil adjustment and error", adjustment, err)
	}
	assertNoEventAccountingWrites(t, ctx, pool, fixture)

	type outcome struct {
		adjustment *repository.EventAdjustment
		err        error
	}
	start := make(chan struct{})
	outcomes := make(chan outcome, 2)
	for range 2 {
		go func() {
			<-start
			adjustment, err := service.Commit(ctx, result, audit.Metadata{ChangedBy: fixture.userID})
			outcomes <- outcome{adjustment: adjustment, err: err}
		}()
	}
	close(start)
	first, second := <-outcomes, <-outcomes
	for index, candidate := range []outcome{first, second} {
		if candidate.err != nil || candidate.adjustment == nil {
			t.Fatalf("concurrent Commit() %d = adjustment %#v, error %v", index+1, candidate.adjustment, candidate.err)
		}
	}
	if first.adjustment.ID != second.adjustment.ID {
		t.Fatalf("concurrent adjustment IDs = %s and %s", first.adjustment.ID, second.adjustment.ID)
	}
	adjustment := first.adjustment
	assertCommittedEventAccounting(t, ctx, pool, fixture, adjustment.ID, len(result.ForwardSchedule), len(result.JournalEntries))

	again, err := service.Commit(ctx, result, audit.Metadata{ChangedBy: fixture.userID})
	if err != nil {
		t.Fatalf("idempotent Commit(): %v", err)
	}
	if again.ID != adjustment.ID {
		t.Fatalf("idempotent adjustment ID = %s, want %s", again.ID, adjustment.ID)
	}
	assertCommittedEventAccounting(t, ctx, pool, fixture, adjustment.ID, len(result.ForwardSchedule), len(result.JournalEntries))
}

type eventAccountingFixture struct {
	legalEntityID string
	storeID       string
	landlordID    string
	userID        string
	contractID    string
	eventID       string
}

func createEventAccountingFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) eventAccountingFixture {
	t.Helper()
	suffix := uuid.NewString()[:8]
	fixture := eventAccountingFixture{
		legalEntityID: uuid.NewString(), storeID: uuid.NewString(), landlordID: uuid.NewString(),
		userID: uuid.NewString(), contractID: uuid.NewString(), eventID: uuid.NewString(),
	}
	statements := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO legal_entities (id, code, name, country, currency) VALUES ($1, $2, $3, 'CN', 'CNY')`, []any{fixture.legalEntityID, "EA-LE-" + suffix, "Event Accounting LE " + suffix}},
		{`INSERT INTO stores (id, code, name, legal_entity_id) VALUES ($1, $2, $3, $4)`, []any{fixture.storeID, "EA-ST-" + suffix, "Event Accounting Store " + suffix, fixture.legalEntityID}},
		{`INSERT INTO landlords (id, code, name) VALUES ($1, $2, $3)`, []any{fixture.landlordID, "EA-LL-" + suffix, "Event Accounting Landlord " + suffix}},
		{`INSERT INTO users (id, username, email, password_hash, role, legal_entity_id, is_active) VALUES ($1, $2, $3, 'integration-only', 'approver', $4, true)`, []any{fixture.userID, "ea-user-" + suffix, "ea-" + suffix + "@example.com", fixture.legalEntityID}},
		{`INSERT INTO lease_contracts (
			id, contract_number, contract_name, legal_entity_id, store_id, landlord_id,
			commencement_date, lease_start_date, lease_end_date, status, approval_status,
			is_official_version, included_in_reporting, report_mode, lease_scope,
			discount_rate_value, currency
		) VALUES ($1, $2, $3, $4, $5, $6, '2097-01-01', '2097-01-01', '2099-01-01',
			'active', 'approved', true, true, 'official', 'in_scope', 0.05, 'CNY')`, []any{fixture.contractID, "EA-CT-" + suffix, "Event Accounting Contract " + suffix, fixture.legalEntityID, fixture.storeID, fixture.landlordID}},
		{`INSERT INTO lease_events (id, contract_id, event_type, effective_date, new_value, status, approval_status, is_official_version)
		 VALUES ($1, $2, 'early_termination', '2098-01-01', '2098-07-01', 'active', 'approved', true)`, []any{fixture.eventID, fixture.contractID}},
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatalf("create event accounting fixture: %v", err)
		}
	}
	t.Cleanup(func() {
		for _, statement := range []struct {
			sql  string
			args []any
		}{
			{`DELETE FROM audit_logs WHERE record_id = $1`, []any{fixture.eventID}},
			{`DELETE FROM journal_entries WHERE contract_id = $1`, []any{fixture.contractID}},
			{`DELETE FROM measurement_results WHERE contract_id = $1`, []any{fixture.contractID}},
			{`DELETE FROM lease_events WHERE id = $1`, []any{fixture.eventID}},
			{`DELETE FROM lease_contracts WHERE id = $1`, []any{fixture.contractID}},
			{`DELETE FROM users WHERE id = $1`, []any{fixture.userID}},
			{`DELETE FROM stores WHERE id = $1`, []any{fixture.storeID}},
			{`DELETE FROM landlords WHERE id = $1`, []any{fixture.landlordID}},
			{`DELETE FROM legal_entities WHERE id = $1`, []any{fixture.legalEntityID}},
		} {
			_, _ = pool.Exec(context.Background(), statement.sql, statement.args...)
		}
	})
	return fixture
}

func calculateFixtureResult(t *testing.T, fixture eventAccountingFixture) Result {
	t.Helper()
	newEndDate := "2098-07-01"
	result, err := Calculate(Input{
		EventID: fixture.eventID, ContractID: fixture.contractID, EventType: "early_termination",
		EffectiveDate: date("2098-01-01"), CommencementDate: date("2097-01-01"),
		LeaseEndDate: date("2099-01-01"), NewValue: &newEndDate,
		Currency: "CNY", LeaseScope: ifrs16.LeaseScopeInScope, DiscountRate: 0.05,
		Payments: []ifrs16.LeasePayment{
			{Date: date("2098-06-30"), Amount: 100000, Timing: "postpaid", Type: "fixed"},
			{Date: date("2098-12-31"), Amount: 100000, Timing: "postpaid", Type: "fixed"},
		},
	})
	if err != nil {
		t.Fatalf("Calculate() fixture: %v", err)
	}
	return result
}

func assertNoEventAccountingWrites(t *testing.T, ctx context.Context, pool *pgxpool.Pool, fixture eventAccountingFixture) {
	t.Helper()
	for table, check := range map[string]struct {
		query string
		arg   string
	}{
		"adjustments":  {`SELECT count(*) FROM event_adjustments WHERE event_id = $1`, fixture.eventID},
		"measurements": {`SELECT count(*) FROM measurement_results WHERE contract_id = $1`, fixture.contractID},
		"journals":     {`SELECT count(*) FROM journal_entries WHERE contract_id = $1`, fixture.contractID},
		"audits":       {`SELECT count(*) FROM audit_logs WHERE record_id = $1`, fixture.eventID},
	} {
		var count int
		if err := pool.QueryRow(ctx, check.query, check.arg).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Errorf("%s count = %d after rollback, want 0", table, count)
		}
	}
	var linked *string
	if err := pool.QueryRow(ctx, `SELECT recalculation_batch_id::text FROM lease_events WHERE id = $1`, fixture.eventID).Scan(&linked); err != nil {
		t.Fatalf("read event link: %v", err)
	}
	if linked != nil {
		t.Errorf("event link = %v after rollback, want nil", linked)
	}
}

func assertCommittedEventAccounting(t *testing.T, ctx context.Context, pool *pgxpool.Pool, fixture eventAccountingFixture, adjustmentID string, dailyPeriods, journalEntries int) {
	t.Helper()
	checks := []struct {
		name  string
		want  int
		query string
		arg   string
	}{
		{"adjustments", 1, `SELECT count(*) FROM event_adjustments WHERE event_id = $1`, fixture.eventID},
		{"journals", journalEntries, `SELECT count(*) FROM journal_entries WHERE contract_id = $1`, fixture.contractID},
		{"audits", 1, `SELECT count(*) FROM audit_logs WHERE record_id = $1`, fixture.eventID},
	}
	for _, check := range checks {
		var count int
		if err := pool.QueryRow(ctx, check.query, check.arg).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", check.name, err)
		}
		if count != check.want {
			t.Errorf("%s count = %d, want %d", check.name, count, check.want)
		}
	}
	var measurementCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM measurement_results WHERE contract_id = $1`, fixture.contractID).Scan(&measurementCount); err != nil {
		t.Fatalf("count measurements: %v", err)
	}
	if measurementCount == 0 || measurementCount >= dailyPeriods {
		t.Errorf("monthly measurement count = %d, daily schedule = %d", measurementCount, dailyPeriods)
	}
	var linked string
	if err := pool.QueryRow(ctx, `SELECT recalculation_batch_id::text FROM lease_events WHERE id = $1`, fixture.eventID).Scan(&linked); err != nil {
		t.Fatalf("read event link: %v", err)
	}
	if linked != adjustmentID {
		t.Errorf("event link = %s, want %s", linked, adjustmentID)
	}
}
