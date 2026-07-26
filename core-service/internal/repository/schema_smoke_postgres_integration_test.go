package repository_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/repository"
)

// This suite exists because two defects reached the demo path unnoticed: a
// column the code wrote but the schema never had (lease_contracts.updated_by,
// SQLSTATE 42703) and a parameter used with two conflicting inferred types
// (SQLSTATE 42P08). Neither could be caught by a unit test with a fake database,
// and neither showed up until someone exercised the flow by hand.
//
// Every test here therefore runs one real write path against a real database
// built from the init script. They assert only "this statement executes", which
// is exactly the class of bug that slipped through; the accounting behaviour is
// covered by the unit and regression suites.
func smokePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect to Postgres: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedUser creates the actor the write paths record as the author of a change.
// The columns are real foreign keys to users, so a placeholder string will not do.
func seedUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()
	suffix := uuid.NewString()[:8]
	var userID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO users (username, email, password_hash, role)
		VALUES ($1, $2, 'smoke', 'admin') RETURNING id
	`, "smoke-"+suffix, "smoke-"+suffix+"@example.test").Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})
	return userID
}

// seedContract creates the master data a contract needs and returns the contract.
func seedContract(t *testing.T, ctx context.Context, pool *pgxpool.Pool) *repository.Contract {
	t.Helper()
	suffix := uuid.NewString()[:8]

	var legalEntityID, storeID, landlordID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO legal_entities (code, name, country, currency, is_active)
		VALUES ($1, $2, 'CN', 'CNY', true) RETURNING id
	`, "SMOKE-LE-"+suffix, "冒烟测试主体 "+suffix).Scan(&legalEntityID); err != nil {
		t.Fatalf("seed legal entity: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO stores (code, name, legal_entity_id, brand, region, is_active)
		VALUES ($1, $2, $3, '冒烟品牌', '冒烟区域', true) RETURNING id
	`, "SMOKE-ST-"+suffix, "冒烟门店 "+suffix, legalEntityID).Scan(&storeID); err != nil {
		t.Fatalf("seed store: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO landlords (code, name, is_active) VALUES ($1, $2, true) RETURNING id
	`, "SMOKE-LL-"+suffix, "冒烟出租方 "+suffix).Scan(&landlordID); err != nil {
		t.Fatalf("seed landlord: %v", err)
	}

	rate := 0.05
	area := 250.0
	contracts := repository.NewContractRepository(pool)
	created, err := contracts.Create(ctx, &repository.Contract{
		ContractNumber:    "SMOKE-" + suffix,
		ContractName:      "冒烟测试合同",
		LegalEntityID:     &legalEntityID,
		StoreID:           &storeID,
		LandlordID:        &landlordID,
		Currency:          "CNY",
		AssetType:         "real_estate",
		AreaSqm:           &area,
		CommencementDate:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		LeaseStartDate:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		LeaseEndDate:      time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		LeaseScope:        "in_scope",
		DiscountRateValue: &rate,
		Status:            "draft",
	})
	if err != nil {
		t.Fatalf("create contract: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM lease_contracts WHERE id = $1`, created.ID)
	})
	return created
}

// TestSchemaSmoke_ContractWritePaths covers create, read back and update. The
// update is where a column missing from the schema surfaced.
func TestSchemaSmoke_ContractWritePaths(t *testing.T) {
	pool := smokePool(t)
	ctx := context.Background()
	contracts := repository.NewContractRepository(pool)

	actor := seedUser(t, ctx, pool)
	created := seedContract(t, ctx, pool)
	if created.AreaSqm == nil || *created.AreaSqm != 250 {
		t.Errorf("area did not round-trip through create: %v", created.AreaSqm)
	}

	fetched, err := contracts.GetByID(ctx, created.ID, "")
	if err != nil || fetched == nil {
		t.Fatalf("read back contract: %v", err)
	}

	fetched.ContractName = "冒烟测试合同(已改)"
	if err := contracts.Update(ctx, fetched, "", actor); err != nil {
		t.Fatalf("update contract: %v", err)
	}

	listed, total, err := contracts.ListPaged(ctx, "", repository.ListContractsFilter{PageSize: 5, Page: 1})
	if err != nil {
		t.Fatalf("list contracts: %v", err)
	}
	if total < 1 || len(listed) == 0 {
		t.Errorf("paged list returned %d rows of %d total", len(listed), total)
	}
}

// TestSchemaSmoke_EventRevisionParameters covers the JSONB clause round-trip.
// A column the code writes but the schema lacks is exactly the defect this suite
// exists for, and a JSONB column adds a second way to fail: writing an empty
// value where the type demands a valid document.
func TestSchemaSmoke_EventRevisionParameters(t *testing.T) {
	pool := smokePool(t)
	ctx := context.Background()

	created := seedContract(t, ctx, pool)
	events := repository.NewEventRepository(pool)

	clause := []byte(`{"kind":"index","base_index":102.4,"new_index":105.1,"cap_percentage":2}`)
	withClause, err := events.Create(ctx, &repository.LeaseEvent{
		ContractID:         created.ID,
		EventType:          "index_update",
		EffectiveDate:      time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		RevisionParameters: clause,
	})
	if err != nil {
		t.Fatalf("create event with clause: %v", err)
	}

	// An event without a clause must still insert: nil, not an empty document.
	if _, err := events.Create(ctx, &repository.LeaseEvent{
		ContractID:    created.ID,
		EventType:     "rent_change",
		EffectiveDate: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("create event without clause: %v", err)
	}

	readBack, err := events.GetByID(ctx, withClause.ID)
	if err != nil || readBack == nil {
		t.Fatalf("read event back: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(readBack.RevisionParameters, &decoded); err != nil {
		t.Fatalf("stored clause is not valid JSON: %v", err)
	}
	if decoded["kind"] != "index" || decoded["cap_percentage"] != 2.0 {
		t.Errorf("clause did not round-trip: %v", decoded)
	}

	if _, err := events.GetByContractID(ctx, created.ID); err != nil {
		t.Fatalf("list events by contract: %v", err)
	}
	if _, err := events.GetApprovedEventsForContract(ctx, created.ID); err != nil {
		t.Fatalf("list approved events: %v", err)
	}
}

// TestSchemaSmoke_ConfirmDiscountRate covers the flow the demo script opens with.
func TestSchemaSmoke_ConfirmDiscountRate(t *testing.T) {
	pool := smokePool(t)
	ctx := context.Background()
	contracts := repository.NewContractRepository(pool)

	actor := seedUser(t, ctx, pool)
	created := seedContract(t, ctx, pool)
	if err := contracts.ConfirmDiscountRate(ctx, created.ID, "",
		"incremental_borrowing_rate", "smoke", 0.055, "manual", nil, actor, time.Now()); err != nil {
		t.Fatalf("confirm discount rate: %v", err)
	}
}

// TestSchemaSmoke_ApprovalTransitions covers submit, review and approve. The
// review statement is where two conflicting types were deduced for one
// parameter.
func TestSchemaSmoke_ApprovalTransitions(t *testing.T) {
	pool := smokePool(t)
	ctx := context.Background()

	actor := seedUser(t, ctx, pool)
	created := seedContract(t, ctx, pool)
	approvals := repository.NewApprovalRepository(pool)

	if err := approvals.SubmitForReview(ctx, created.ID, actor); err != nil {
		t.Fatalf("submit contract: %v", err)
	}
	if err := approvals.Review(ctx, created.ID, actor, true, "冒烟复核通过"); err != nil {
		t.Fatalf("review contract: %v", err)
	}
	if err := approvals.Approve(ctx, created.ID, actor); err != nil {
		t.Fatalf("approve contract: %v", err)
	}
}

// TestSchemaSmoke_ExchangeRatesAndWorkQueue covers the newest tables, so a
// migration that is written but never applied fails here rather than in a demo.
func TestSchemaSmoke_ExchangeRatesAndWorkQueue(t *testing.T) {
	pool := smokePool(t)
	ctx := context.Background()

	rates := repository.NewExchangeRateRepository(pool)
	saved, err := rates.Upsert(ctx, &repository.ExchangeRate{
		FromCurrency: "USD", ToCurrency: "CNY",
		RateDate: time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		RateType: repository.RateTypeClosing, Rate: 7.2,
	})
	if err != nil {
		t.Fatalf("upsert exchange rate: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM exchange_rates WHERE id = $1`, saved.ID)
	})
	if _, err := rates.GetRate(ctx, "USD", "CNY", repository.RateTypeClosing,
		time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatalf("read exchange rate: %v", err)
	}

	// The period query joins journal_entries, lease_contracts and period_locks
	// and aggregates over them, so it is exactly the kind of statement that only
	// fails against a real schema.
	mc := repository.NewMonthlyClosingRepository(pool)
	if _, _, err := mc.ListJournalEntries(ctx, repository.JournalEntryQuery{Period: "2026-03", PageSize: 10}); err != nil {
		t.Fatalf("list journal entries by period: %v", err)
	}
	if _, err := mc.ListEntryPeriods(ctx, "", 12); err != nil {
		t.Fatalf("list entry periods: %v", err)
	}

	if _, err := repository.NewWorkQueueRepository(pool).Load(ctx, "", 30); err != nil {
		t.Fatalf("load work queue: %v", err)
	}
	if _, err := repository.NewBudgetRepository(pool).ListVersions(ctx, ""); err != nil {
		t.Fatalf("list budget versions: %v", err)
	}
}
