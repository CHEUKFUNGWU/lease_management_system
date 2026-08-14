package monthend

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/money"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
)

func TestClose_PostgresTransactionAndRerunContract(t *testing.T) {
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

	fixture := createMonthEndFixture(t, ctx, pool)
	mcRepo := repository.NewMonthlyClosingRepository(pool)
	contractRepo := repository.NewContractRepository(pool)
	scheduleRepo := repository.NewPaymentScheduleRepository(pool)
	settingRepo := repository.NewSystemSettingRepository(pool)
	auditLogger := audit.NewLogger(repository.NewAuditRepository(pool))
	svc := NewService(pool, mcRepo, contractRepo, scheduleRepo, settingRepo,
		repository.NewExchangeRateRepository(pool), repository.NewMasterDataRepository(pool), auditLogger)

	eventEntry := &repository.JournalEntry{
		ContractID: fixture.contractID, AccountingPeriod: fixture.period,
		EntryDate: time.Date(2098, 1, 31, 0, 0, 0, 0, time.UTC),
		EntryType: "modification", DebitAccount: "2801", CreditAccount: "1701",
		Amount: money.NewFromInt64(125), Currency: "CNY", PostingStatus: "draft",
	}
	if err := mcRepo.CreateJournalEntry(ctx, eventEntry); err != nil {
		t.Fatalf("create event entry: %v", err)
	}

	command := Command{
		AccountingPeriod: fixture.period,
		ContractID:       fixture.contractID,
		LegalEntityID:    fixture.legalEntityID,
		Actor:            audit.Metadata{ChangedBy: fixture.userID},
	}
	first, err := svc.Close(ctx, command)
	if err != nil {
		t.Fatalf("first close: %v", err)
	}
	second, err := svc.Close(ctx, command)
	if err != nil {
		t.Fatalf("draft rerun: %v", err)
	}
	if second.BatchID != first.BatchID {
		t.Fatalf("draft rerun created batch %s; want reused batch %s", second.BatchID, first.BatchID)
	}

	entries, err := mcRepo.GetJournalEntries(ctx, fixture.contractID, fixture.period, "draft")
	if err != nil {
		t.Fatalf("list rerun entries: %v", err)
	}
	seenTypes := map[string]int{}
	for _, entry := range entries {
		seenTypes[entry.EntryType]++
		if entry.BatchID == nil || *entry.BatchID != first.BatchID {
			t.Fatalf("entry %s is attached to stale batch %v", entry.ID, entry.BatchID)
		}
	}
	for _, entryType := range managedEntryTypes {
		if seenTypes[entryType] > 1 {
			t.Fatalf("rerun duplicated %s entries: %d", entryType, seenTypes[entryType])
		}
	}
	if seenTypes["modification"] != 1 {
		t.Fatalf("event entry count = %d, want 1", seenTypes["modification"])
	}
	assertBulkRerunRemovesIneligibleDrafts(t, ctx, pool, mcRepo, svc, fixture)

	if len(entries) == 0 {
		t.Fatal("close produced no journal entries")
	}
	if _, err := pool.Exec(ctx, `UPDATE journal_entries SET posting_status = 'approved' WHERE id = $1`, entries[0].ID); err != nil {
		t.Fatalf("finalize entry: %v", err)
	}
	if _, err := svc.Close(ctx, command); !errors.Is(err, ErrCloseAlreadyFinalized) {
		t.Fatalf("rerun after approval error = %v, want ErrCloseAlreadyFinalized", err)
	}

	assertMixedStatusPersists(t, ctx, mcRepo, fixture)
	assertLockSerializesClose(t, ctx, pool, mcRepo, svc, fixture)
}

func assertBulkRerunRemovesIneligibleDrafts(t *testing.T, ctx context.Context, pool *pgxpool.Pool, repo *repository.MonthlyClosingRepository, svc *Service, f monthEndFixture) {
	t.Helper()
	period := "2098-03"
	eventEntry := &repository.JournalEntry{
		ContractID: f.contractID, AccountingPeriod: period,
		EntryDate: time.Date(2098, 3, 31, 0, 0, 0, 0, time.UTC),
		EntryType: "reassessment", DebitAccount: "2801", CreditAccount: "1701",
		Amount: money.NewFromInt64(50), Currency: "CNY", PostingStatus: "draft",
	}
	if err := repo.CreateJournalEntry(ctx, eventEntry); err != nil {
		t.Fatalf("create bulk event entry: %v", err)
	}
	first, err := svc.Close(ctx, Command{AccountingPeriod: period, LegalEntityID: f.legalEntityID})
	if err != nil {
		t.Fatalf("first bulk close: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE lease_contracts SET approval_status = 'draft' WHERE id = $1`, f.contractID); err != nil {
		t.Fatalf("remove contract from bulk eligibility: %v", err)
	}
	defer pool.Exec(context.Background(), `UPDATE lease_contracts SET approval_status = 'approved' WHERE id = $1`, f.contractID)

	second, err := svc.Close(ctx, Command{AccountingPeriod: period, LegalEntityID: f.legalEntityID})
	if err != nil {
		t.Fatalf("bulk rerun with empty eligibility: %v", err)
	}
	if second.BatchID != first.BatchID || second.TotalEntries != 0 {
		t.Fatalf("bulk rerun = batch %s entries %d; want batch %s entries 0", second.BatchID, second.TotalEntries, first.BatchID)
	}
	entries, err := repo.GetJournalEntries(ctx, f.contractID, period, "draft")
	if err != nil {
		t.Fatalf("read bulk rerun entries: %v", err)
	}
	for _, entry := range entries {
		if entry.EntryType == "reassessment" {
			if entry.BatchID != nil {
				t.Fatalf("ineligible event entry remains on reused batch %s", *entry.BatchID)
			}
			continue
		}
		t.Fatalf("stale system entry %s remains after contract left bulk scope", entry.EntryType)
	}
}

type monthEndFixture struct {
	legalEntityID string
	storeID       string
	landlordID    string
	userID        string
	contractID    string
	period        string
}

func createMonthEndFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) monthEndFixture {
	t.Helper()
	suffix := uuid.NewString()[:8]
	f := monthEndFixture{
		legalEntityID: uuid.NewString(), storeID: uuid.NewString(), landlordID: uuid.NewString(),
		userID: uuid.NewString(), contractID: uuid.NewString(), period: "2098-01",
	}

	statements := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO legal_entities (id, code, name, country, currency) VALUES ($1, $2, $3, 'CN', 'CNY')`, []any{f.legalEntityID, "IT-LE-" + suffix, "Integration LE " + suffix}},
		{`INSERT INTO stores (id, code, name, legal_entity_id) VALUES ($1, $2, $3, $4)`, []any{f.storeID, "IT-ST-" + suffix, "Integration Store " + suffix, f.legalEntityID}},
		{`INSERT INTO landlords (id, code, name) VALUES ($1, $2, $3)`, []any{f.landlordID, "IT-LL-" + suffix, "Integration Landlord " + suffix}},
		{`INSERT INTO users (id, username, email, password_hash, role, legal_entity_id, is_active) VALUES ($1, $2, $3, 'integration-only', 'approver', $4, true)`, []any{f.userID, "it-user-" + suffix, "it-" + suffix + "@example.com", f.legalEntityID}},
		{`INSERT INTO lease_contracts (
			id, contract_number, contract_name, legal_entity_id, store_id, landlord_id, asset_type,
			commencement_date, lease_start_date, lease_end_date, status, approval_status,
			is_official_version, included_in_reporting, report_mode, lease_scope,
			discount_rate_value, currency
		) VALUES ($1, $2, $3, $4, $5, $6, 'real_estate', '2098-01-01', '2098-01-01', '2098-12-31',
			'active', 'approved', true, true, 'official', 'in_scope', 0.05, 'CNY')`, []any{f.contractID, "IT-CT-" + suffix, "Integration Contract " + suffix, f.legalEntityID, f.storeID, f.landlordID}},
		{`INSERT INTO lease_payment_schedules (
			contract_id, effective_start_date, effective_end_date, coverage_start_date,
			coverage_end_date, due_date, payment_timing, amount, currency, amount_type,
			is_fixed, is_variable, is_lease_component, included_in_liability_pv,
			approval_status, is_official_version
		) VALUES ($1, '2098-01-01', '2098-01-31', '2098-01-01', '2098-01-31',
			'2098-01-31', 'postpaid', 100000, 'CNY', 'fixed', true, false, true, true,
			'approved', true)`, []any{f.contractID}},
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatalf("create integration fixture: %v", err)
		}
	}

	t.Cleanup(func() {
		cleanup := []struct {
			sql  string
			args []any
		}{
			{`DELETE FROM audit_logs WHERE changed_by = $1`, []any{f.userID}},
			{`DELETE FROM period_locks WHERE legal_entity_id = $1`, []any{f.legalEntityID}},
			{`DELETE FROM journal_entries WHERE contract_id = $1`, []any{f.contractID}},
			{`DELETE FROM measurement_results WHERE contract_id = $1`, []any{f.contractID}},
			{`DELETE FROM monthly_closing_batches WHERE legal_entity_id = $1`, []any{f.legalEntityID}},
			{`DELETE FROM lease_payment_schedules WHERE contract_id = $1`, []any{f.contractID}},
			{`DELETE FROM lease_contracts WHERE id = $1`, []any{f.contractID}},
			{`DELETE FROM users WHERE id = $1`, []any{f.userID}},
			{`DELETE FROM stores WHERE id = $1`, []any{f.storeID}},
			{`DELETE FROM landlords WHERE id = $1`, []any{f.landlordID}},
			{`DELETE FROM legal_entities WHERE id = $1`, []any{f.legalEntityID}},
		}
		for _, statement := range cleanup {
			_, _ = pool.Exec(context.Background(), statement.sql, statement.args...)
		}
	})
	return f
}

func assertMixedStatusPersists(t *testing.T, ctx context.Context, repo *repository.MonthlyClosingRepository, f monthEndFixture) {
	t.Helper()
	batch, err := repo.CreateBatch(ctx, &repository.MonthlyClosingBatch{
		BatchNumber: "IT-MIXED-" + uuid.NewString(), AccountingPeriod: f.period,
		LegalEntityID: &f.legalEntityID, ScopeContractID: &f.contractID,
	})
	if err != nil {
		t.Fatalf("create mixed-status batch: %v", err)
	}
	if err := repo.UpdateBatchStatus(ctx, batch.ID, "completed_with_errors", 1, 1, 2, 0); err != nil {
		t.Fatalf("persist completed_with_errors: %v", err)
	}
	batches, err := repo.GetBatches(ctx, f.period, f.legalEntityID)
	if err != nil {
		t.Fatalf("read mixed-status batch: %v", err)
	}
	for _, candidate := range batches {
		if candidate.ID == batch.ID {
			if candidate.Status != "completed_with_errors" || candidate.CompletedAt == nil {
				t.Fatalf("mixed-status batch = status %q completed_at %v", candidate.Status, candidate.CompletedAt)
			}
			return
		}
	}
	t.Fatal("mixed-status batch not found")
}

func assertLockSerializesClose(t *testing.T, ctx context.Context, pool *pgxpool.Pool, repo *repository.MonthlyClosingRepository, svc *Service, f monthEndFixture) {
	t.Helper()
	period := "2098-02"
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin lock holder: %v", err)
	}
	defer tx.Rollback(context.Background())
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, closeLockKey(f.legalEntityID, period)); err != nil {
		t.Fatalf("hold close lock: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- repo.LockPeriod(context.Background(), period, f.legalEntityID, f.userID) }()
	select {
	case err := <-done:
		t.Fatalf("period lock bypassed active close lock: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("release close lock: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("lock period after close released: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("period lock did not resume after close released")
	}

	_, err = svc.Close(ctx, Command{
		AccountingPeriod: period, ContractID: f.contractID,
		LegalEntityID: f.legalEntityID,
	})
	if !errors.Is(err, ErrPeriodLocked) {
		t.Fatalf("close after lock error = %v, want ErrPeriodLocked", err)
	}
	if err := repo.UnlockPeriod(ctx, period, f.legalEntityID, f.userID); err != nil {
		t.Fatalf("unlock period: %v", err)
	}
	if _, err := svc.Close(ctx, Command{
		AccountingPeriod: period, ContractID: f.contractID,
		LegalEntityID: f.legalEntityID,
	}); err != nil {
		t.Fatalf("close after unlock: %v", err)
	}
}
