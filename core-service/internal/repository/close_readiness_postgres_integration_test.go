package repository_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
)

func TestCloseReadinessRepositoryAppliesPopulationAndScopeFacts(t *testing.T) {
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

	contract := seedContract(t, ctx, pool)
	if contract.LegalEntityID == nil || contract.StoreID == nil {
		t.Fatal("fixture contract has no tenant attributes")
	}
	if _, err := pool.Exec(ctx, `UPDATE lease_contracts SET approval_status = 'approved', is_official_version = true WHERE id = $1`, contract.ID); err != nil {
		t.Fatalf("approve fixture contract: %v", err)
	}
	scheduleID := uuid.NewString()
	eventID := uuid.NewString()
	batchID := uuid.NewString()
	suffix := uuid.NewString()[:8]
	if _, err := pool.Exec(ctx, `
		INSERT INTO lease_payment_schedules (
			id, contract_id, effective_start_date, effective_end_date, coverage_start_date, coverage_end_date,
			due_date, payment_timing, amount, currency, amount_type, is_fixed, is_lease_component,
			included_in_liability_pv, approval_status, is_official_version
		) VALUES ($1, $2, '2026-01-01', '2027-01-01', '2026-01-01', '2027-01-01', '2026-08-15',
			'postpaid', 100, 'CNY', 'fixed', true, true, true, 'approved', true)
	`, scheduleID, contract.ID); err != nil {
		t.Fatalf("seed payment schedule: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO lease_events (id, contract_id, event_type, effective_date, status, approval_status)
		VALUES ($1, $2, 'rent_change', '2026-08-01', 'pending', 'submitted')
	`, eventID, contract.ID); err != nil {
		t.Fatalf("seed pending event: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO monthly_closing_batches (id, batch_number, accounting_period, legal_entity_id, scope_contract_id, status, total_contracts, failed_contracts)
		VALUES ($1, $2, '2026-08', $3, $4, 'completed_with_errors', 1, 1)
	`, batchID, "CR-"+suffix, *contract.LegalEntityID, contract.ID); err != nil {
		t.Fatalf("seed failed batch: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM monthly_closing_batches WHERE id = $1`, batchID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM lease_events WHERE id = $1`, eventID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM lease_payment_schedules WHERE id = $1`, scheduleID)
	})

	scopedContext := access.WithScope(ctx, access.Scope{LegalEntityID: *contract.LegalEntityID, StoreIDs: []string{*contract.StoreID}})
	facts, err := repository.NewCloseReadinessRepository(pool).LoadFacts(scopedContext, "2026-08", *contract.LegalEntityID)
	if err != nil {
		t.Fatalf("LoadFacts(): %v", err)
	}
	if len(facts.Contracts) != 1 || facts.Contracts[0].ContractID != contract.ID {
		t.Fatalf("contracts = %#v", facts.Contracts)
	}
	if !facts.Contracts[0].HasApprovedPaymentPlan || !facts.Contracts[0].HasPendingEvent {
		t.Fatalf("contract facts = %#v", facts.Contracts[0])
	}
	if len(facts.FailedBatches) != 1 || facts.FailedBatches[0].BatchID != batchID {
		t.Fatalf("failed batches = %#v", facts.FailedBatches)
	}

	// A contract outside the user's store slice must not enter the population.
	otherStore := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO stores (id, code, name, legal_entity_id) VALUES ($1, $2, $3, $4)`, otherStore, "CR-OTHER-"+suffix, "Other", *contract.LegalEntityID); err != nil {
		t.Fatalf("seed other store: %v", err)
	}
	otherContract := uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO lease_contracts (id, contract_number, contract_name, legal_entity_id, store_id, landlord_id, asset_type, commencement_date, lease_start_date, lease_end_date, approval_status, is_official_version)
		VALUES ($1, $2, 'Other', $3, $4, $5, 'real_estate', '2026-01-01', '2026-01-01', '2027-01-01', 'approved', true)
	`, otherContract, "CR-OTHER-CONTRACT-"+suffix, *contract.LegalEntityID, otherStore, *contract.LandlordID); err != nil {
		t.Fatalf("seed other contract: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM lease_contracts WHERE id = $1`, otherContract)
		_, _ = pool.Exec(context.Background(), `DELETE FROM stores WHERE id = $1`, otherStore)
	})
	facts, err = repository.NewCloseReadinessRepository(pool).LoadFacts(scopedContext, "2026-08", *contract.LegalEntityID)
	if err != nil {
		t.Fatalf("LoadFacts() after scope fixture: %v", err)
	}
	if len(facts.Contracts) != 1 || facts.Contracts[0].ContractID != contract.ID {
		t.Fatalf("scope leaked other contract: %#v", facts.Contracts)
	}
}
