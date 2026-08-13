package repository

import (
	"context"
	"testing"
	"time"
)

// TestBudgetTenantIsolationCharacterization pins down the legal-entity
// filtering of the budget repository before the EntityFilter refactor
// (SEC-003): versions, by-id reads, variance-action scope checks, and the
// actuals join must all refuse entity B.
func TestBudgetTenantIsolationCharacterization(t *testing.T) {
	pool := postgresTestPool(t)
	ctx := context.Background()
	pair := seedTenantPair(t, ctx, pool, "budget")
	t.Cleanup(func() { cleanupTenantPair(t, ctx, pool, pair) })

	repo := NewBudgetRepository(pool)

	version := &BudgetVersion{
		LegalEntityID: &pair.entityA, Name: "char-budget-A", VersionType: "budget", Source: "char-test",
		CoverageScope: "all", AsOfPeriod: "2026-01", FromPeriod: "2026-01", ToPeriod: "2026-12",
	}
	createdVersion, err := repo.CreateVersion(ctx, version, nil)
	if err != nil {
		t.Fatalf("seed budget version: %v", err)
	}
	versionsA, err := repo.ListVersions(ctx, mustEntityFilter(t, pair.entityA))
	if err != nil || len(versionsA) != 1 {
		t.Fatalf("entity A budget versions = %d, err %v; want 1", len(versionsA), err)
	}
	versionsB, err := repo.ListVersions(ctx, mustEntityFilter(t, pair.entityB))
	if err != nil || len(versionsB) != 0 {
		t.Fatalf("entity B saw entity A budget versions: %d, err %v", len(versionsB), err)
	}
	if got, err := repo.GetVersion(ctx, createdVersion.ID, mustEntityFilter(t, pair.entityA)); err != nil || got == nil {
		t.Fatalf("entity A GetVersion = %+v, err %v; want found", got, err)
	}
	if got, err := repo.GetVersion(ctx, createdVersion.ID, mustEntityFilter(t, pair.entityB)); err != nil || got != nil {
		t.Fatalf("entity B GetVersion = %+v, err %v; want nil", got, err)
	}
	if allowed, err := repo.ContractAllowedForVersion(ctx, createdVersion.ID, pair.contractA, mustEntityFilter(t, pair.entityA)); err != nil || !allowed {
		t.Fatalf("entity A ContractAllowedForVersion = %v, err %v; want true", allowed, err)
	}
	if allowed, err := repo.ContractAllowedForVersion(ctx, createdVersion.ID, pair.contractA, mustEntityFilter(t, pair.entityB)); err != nil || allowed {
		t.Fatalf("entity B ContractAllowedForVersion = %v, err %v; want false", allowed, err)
	}

	// ActualsForPeriod joins measurement results to the contract's legal
	// entity; entity B must not see entity A's actuals.
	var ignored string
	if err := pool.QueryRow(ctx, `
		INSERT INTO measurement_results (contract_id, accounting_period, period_start_date, period_end_date, discount_rate)
		VALUES ($1, '2026-07', DATE '2026-07-01', DATE '2026-07-31', 0.05)
		RETURNING id
	`, pair.contractA).Scan(&ignored); err != nil {
		t.Fatalf("seed measurement result: %v", err)
	}
	actualsA, err := repo.ActualsForPeriod(ctx, mustEntityFilter(t, pair.entityA), "2026-07")
	if err != nil || len(actualsA) != 1 {
		t.Fatalf("entity A actuals = %d, err %v; want 1", len(actualsA), err)
	}
	actualsB, err := repo.ActualsForPeriod(ctx, mustEntityFilter(t, pair.entityB), "2026-07")
	if err != nil || len(actualsB) != 0 {
		t.Fatalf("entity B saw entity A actuals: %d, err %v", len(actualsB), err)
	}
}

// TestRenewalDecisionTenantIsolationCharacterization pins down the legal-entity
// filtering of renewal-decision snapshots before the EntityFilter refactor.
func TestRenewalDecisionTenantIsolationCharacterization(t *testing.T) {
	pool := postgresTestPool(t)
	ctx := context.Background()
	pair := seedTenantPair(t, ctx, pool, "renewal")
	t.Cleanup(func() { cleanupTenantPair(t, ctx, pool, pair) })

	repo := NewRenewalDecisionRepository(pool)
	snapshot := &RenewalDecisionSnapshot{
		ContractID: pair.contractA, LegalEntityID: &pair.entityA,
		DecisionDate: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		OwnerName:    "Owner A", BusinessOpinion: "renew", Evidence: "{}",
	}
	created, err := repo.Create(ctx, snapshot)
	if err != nil {
		t.Fatalf("seed renewal decision: %v", err)
	}
	rowsA, err := repo.List(ctx, pair.contractA, mustEntityFilter(t, pair.entityA))
	if err != nil || len(rowsA) != 1 || rowsA[0].ID != created.ID {
		t.Fatalf("entity A renewal decisions = %d (id %s), err %v; want 1 matching row", len(rowsA), firstID(rowsA), err)
	}
	rowsB, err := repo.List(ctx, pair.contractA, mustEntityFilter(t, pair.entityB))
	if err != nil || len(rowsB) != 0 {
		t.Fatalf("entity B saw entity A renewal decisions: %d, err %v", len(rowsB), err)
	}
	existsA, err := repo.Exists(ctx, pair.contractA, mustEntityFilter(t, pair.entityA))
	if err != nil || !existsA {
		t.Fatalf("entity A Exists = %v, err %v; want true", existsA, err)
	}
	existsB, err := repo.Exists(ctx, pair.contractA, mustEntityFilter(t, pair.entityB))
	if err != nil || existsB {
		t.Fatalf("entity B Exists = %v, err %v; want false", existsB, err)
	}
}

func firstID(rows []*RenewalDecisionSnapshot) string {
	if len(rows) == 0 {
		return ""
	}
	return rows[0].ID
}
