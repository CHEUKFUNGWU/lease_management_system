package reporting

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
)

func TestOfficialSnapshotPostgresHonorsAccessAndApprovalScope(t *testing.T) {
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

	fixture := createReportSnapshotFixture(t, ctx, pool)
	scoped := access.WithScope(ctx, access.Scope{
		LegalEntityID: fixture.legalEntityID,
		StoreIDs:      []string{fixture.allowedStoreID},
	})
	builder := NewSnapshotBuilder(
		repository.NewContractRepository(pool),
		repository.NewPaymentScheduleRepository(pool),
		repository.NewSystemSettingRepository(pool),
		repository.NewMonthlyClosingRepository(pool),
	)

	snapshot, err := builder.Build(scoped, Request{Mode: Official, LegalEntityID: fixture.legalEntityID})
	if err != nil {
		t.Fatalf("Build(): %v", err)
	}
	if len(snapshot.Contracts) != 1 || snapshot.Contracts[0].Contract.ID != fixture.allowedContractID {
		t.Fatalf("official scoped contracts = %#v", snapshot.Contracts)
	}
	fact := snapshot.Contracts[0]
	if len(fact.PaymentSchedules) != 1 || fact.PaymentSchedules[0].ID != fixture.approvedPaymentID {
		t.Fatalf("official payment facts = %#v", fact.PaymentSchedules)
	}
	if fact.DiscountRate != 0.06 {
		t.Fatalf("discount rate = %v, want contract rate 0.06", fact.DiscountRate)
	}
}

type reportSnapshotFixture struct {
	legalEntityID      string
	allowedStoreID     string
	restrictedStoreID  string
	landlordID         string
	allowedContractID  string
	restrictedContract string
	approvedPaymentID  string
	draftPaymentID     string
	restrictedPayment  string
}

func createReportSnapshotFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) reportSnapshotFixture {
	t.Helper()
	suffix := uuid.NewString()[:8]
	fixture := reportSnapshotFixture{
		legalEntityID: uuid.NewString(), allowedStoreID: uuid.NewString(), restrictedStoreID: uuid.NewString(),
		landlordID: uuid.NewString(), allowedContractID: uuid.NewString(), restrictedContract: uuid.NewString(),
		approvedPaymentID: uuid.NewString(), draftPaymentID: uuid.NewString(), restrictedPayment: uuid.NewString(),
	}
	statements := []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO legal_entities (id, code, name, country, currency) VALUES ($1, $2, $3, 'CN', 'CNY')`, []any{fixture.legalEntityID, "RS-LE-" + suffix, "Report Snapshot " + suffix}},
		{`INSERT INTO stores (id, code, name, legal_entity_id, region, brand) VALUES
			($1, $2, 'Allowed', $3, 'east', 'brand-a'),
			($4, $5, 'Restricted', $3, 'west', 'brand-b')`, []any{fixture.allowedStoreID, "RS-AS-" + suffix, fixture.legalEntityID, fixture.restrictedStoreID, "RS-RS-" + suffix}},
		{`INSERT INTO landlords (id, code, name) VALUES ($1, $2, 'Report Landlord')`, []any{fixture.landlordID, "RS-LL-" + suffix}},
		{`INSERT INTO lease_contracts (
			id, contract_number, contract_name, legal_entity_id, store_id, landlord_id,
			asset_type, commencement_date, lease_start_date, lease_end_date, currency, status,
			approval_status, lease_scope, discount_rate_value
		) VALUES
			($1, $2, 'Allowed Contract', $3, $4, $5, 'real_estate', '2026-01-01', '2026-01-01', '2026-12-31', 'CNY', 'active', 'approved', 'in_scope', 0.06),
			($6, $7, 'Restricted Contract', $3, $8, $5, 'real_estate', '2026-01-01', '2026-01-01', '2026-12-31', 'CNY', 'active', 'approved', 'in_scope', 0.07)`, []any{
			fixture.allowedContractID, "RS-AC-" + suffix, fixture.legalEntityID, fixture.allowedStoreID, fixture.landlordID,
			fixture.restrictedContract, "RS-RC-" + suffix, fixture.restrictedStoreID,
		}},
		{`INSERT INTO lease_payment_schedules (
			id, contract_id, effective_start_date, effective_end_date, coverage_start_date, coverage_end_date,
			due_date, payment_timing, amount, currency, amount_type, is_fixed, is_variable,
			is_lease_component, included_in_liability_pv, approval_status, is_official_version
		) VALUES
			($1, $2, '2026-01-01', '2026-01-31', '2026-01-01', '2026-01-31', '2026-01-15', 'postpaid', 100, 'CNY', 'fixed', true, false, true, true, 'approved', true),
			($3, $2, '2026-02-01', '2026-02-28', '2026-02-01', '2026-02-28', '2026-02-15', 'postpaid', 999, 'CNY', 'fixed', true, false, true, true, 'draft', false),
			($4, $5, '2026-01-01', '2026-01-31', '2026-01-01', '2026-01-31', '2026-01-15', 'postpaid', 200, 'CNY', 'fixed', true, false, true, true, 'approved', true)`, []any{
			fixture.approvedPaymentID, fixture.allowedContractID, fixture.draftPaymentID,
			fixture.restrictedPayment, fixture.restrictedContract,
		}},
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatalf("create report snapshot fixture: %v", err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM lease_payment_schedules WHERE id IN ($1, $2, $3)`, fixture.approvedPaymentID, fixture.draftPaymentID, fixture.restrictedPayment)
		_, _ = pool.Exec(context.Background(), `DELETE FROM lease_contracts WHERE id IN ($1, $2)`, fixture.allowedContractID, fixture.restrictedContract)
		_, _ = pool.Exec(context.Background(), `DELETE FROM landlords WHERE id = $1`, fixture.landlordID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM stores WHERE id IN ($1, $2)`, fixture.allowedStoreID, fixture.restrictedStoreID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM legal_entities WHERE id = $1`, fixture.legalEntityID)
	})
	return fixture
}
