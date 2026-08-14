package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
)

// TestOperatingFactsTenantIsolationCharacterization pins down the legal-entity
// filtering of every operating-facts read/write guard before the EntityFilter
// refactor (SEC-003). Entity B must never see, resolve, or mutate entity A's
// batches, stores, equipment, actions, assumptions, or critical dates.
func TestOperatingFactsTenantIsolationCharacterization(t *testing.T) {
	pool := postgresTestPool(t)
	ctx := context.Background()
	pair := seedTenantPair(t, ctx, pool, "opfacts")
	t.Cleanup(func() { cleanupTenantPair(t, ctx, pool, pair) })

	repo := NewOperatingFactsRepository(pool)

	// Batches: List. CreateBatch's RETURNING scans idempotency_key without a
	// COALESCE, so the seed must carry one (as the production handler does).
	batch := &OperatingFactBatch{LegalEntityID: &pair.entityA, SourceSystem: "char-test", ReconciliationStatus: "unreconciled", IdempotencyKey: "batch-key-a"}
	createdBatch, err := repo.CreateBatch(ctx, batch)
	if err != nil {
		t.Fatalf("seed batch: %v", err)
	}
	_ = createdBatch
	batchesA, err := repo.ListBatches(ctx, mustEntityFilter(t, pair.entityA), "")
	if err != nil || len(batchesA) != 1 {
		t.Fatalf("entity A batches = %d, err %v; want 1", len(batchesA), err)
	}
	batchesB, err := repo.ListBatches(ctx, mustEntityFilter(t, pair.entityB), "")
	if err != nil || len(batchesB) != 0 {
		t.Fatalf("entity B saw entity A batches: %d, err %v", len(batchesB), err)
	}

	// Store facts: List / ResolveStoreIDByCode / Overview.
	storeFact := &StoreOperatingFact{
		StoreID: pair.storeA, Period: "2026-07", PeriodBasis: "calendar_month", Currency: "CNY",
		Revenue: 100, SourceSystem: "char-test",
	}
	if _, err := repo.UpsertStore(ctx, storeFact); err != nil {
		t.Fatalf("seed store fact: %v", err)
	}
	storesA, err := repo.ListStores(ctx, mustEntityFilter(t, pair.entityA), "2026-07", "")
	if err != nil || len(storesA) != 1 {
		t.Fatalf("entity A store facts = %d, err %v; want 1", len(storesA), err)
	}
	storesB, err := repo.ListStores(ctx, mustEntityFilter(t, pair.entityB), "2026-07", "")
	if err != nil || len(storesB) != 0 {
		t.Fatalf("entity B saw entity A store facts: %d, err %v", len(storesB), err)
	}
	var storeACode string
	if err := pool.QueryRow(ctx, `SELECT code FROM stores WHERE id = $1`, pair.storeA).Scan(&storeACode); err != nil {
		t.Fatalf("read store code: %v", err)
	}
	resolvedA, err := repo.ResolveStoreIDByCode(ctx, mustEntityFilter(t, pair.entityA), storeACode)
	if err != nil || resolvedA == "" {
		t.Fatalf("entity A ResolveStoreIDByCode = %q, err %v; want store id", resolvedA, err)
	}
	resolvedB, err := repo.ResolveStoreIDByCode(ctx, mustEntityFilter(t, pair.entityB), storeACode)
	if err != nil || resolvedB != "" {
		t.Fatalf("entity B ResolveStoreIDByCode = %q, err %v; want empty", resolvedB, err)
	}

	// Equipment assets and facts: List / Upsert / ListFacts / Overview.
	equipment := &EquipmentAsset{
		LegalEntityID: &pair.entityA, PlantCode: "PL-A", EquipmentCode: "EQ-A", EquipmentName: "Equipment A",
		Active: true,
	}
	createdEquipment, err := repo.UpsertEquipment(ctx, equipment)
	if err != nil {
		t.Fatalf("seed equipment: %v", err)
	}
	equipmentA, err := repo.ListEquipment(ctx, mustEntityFilter(t, pair.entityA), "", "")
	if err != nil || len(equipmentA) != 1 {
		t.Fatalf("entity A equipment = %d, err %v; want 1", len(equipmentA), err)
	}
	equipmentB, err := repo.ListEquipment(ctx, mustEntityFilter(t, pair.entityB), "", "")
	if err != nil || len(equipmentB) != 0 {
		t.Fatalf("entity B saw entity A equipment: %d, err %v", len(equipmentB), err)
	}
	equipmentFact := &EquipmentOperatingFact{
		EquipmentID: createdEquipment.ID, Period: "2026-07", Currency: "CNY", OutputQty: float64Ptr(5),
		SourceSystem: "char-test",
	}
	if _, err := repo.UpsertEquipmentFact(access.WithScope(ctx, access.Scope{LegalEntityID: pair.entityA}), mustEntityFilter(t, pair.entityA), equipmentFact); err != nil {
		t.Fatalf("seed equipment fact as entity A: %v", err)
	}
	if _, err := repo.UpsertEquipmentFact(access.WithScope(ctx, access.Scope{LegalEntityID: pair.entityB}), mustEntityFilter(t, pair.entityB), &EquipmentOperatingFact{
		EquipmentID: createdEquipment.ID, Period: "2026-07", Currency: "CNY", SourceSystem: "char-test-b",
	}); err == nil {
		t.Fatal("entity B wrote an equipment fact for entity A's equipment")
	}
	eqFactsA, err := repo.ListEquipmentFacts(ctx, mustEntityFilter(t, pair.entityA), "2026-07", "", "")
	if err != nil || len(eqFactsA) != 1 {
		t.Fatalf("entity A equipment facts = %d, err %v; want 1", len(eqFactsA), err)
	}
	eqFactsB, err := repo.ListEquipmentFacts(ctx, mustEntityFilter(t, pair.entityB), "2026-07", "", "")
	if err != nil || len(eqFactsB) != 0 {
		t.Fatalf("entity B saw entity A equipment facts: %d, err %v", len(eqFactsB), err)
	}

	// Actions: List / idempotency lookup / Update / ActionInScope is covered in
	// the FP&A governance test.
	action := &FPnAActionItem{
		LegalEntityID: &pair.entityA, Category: "occupancy", Title: "Action A", Severity: "medium",
		Period: "2026-07", RuleCode: "rent-spike", SourceTable: "store_operating_facts", SourceRecordID: "fact-1",
		IdempotencyKey: "action-key-a",
	}
	createdAction, err := repo.CreateAction(ctx, action)
	if err != nil {
		t.Fatalf("seed action: %v", err)
	}
	actionsA, err := repo.ListActions(ctx, mustEntityFilter(t, pair.entityA), "", "", "")
	if err != nil || len(actionsA) != 1 {
		t.Fatalf("entity A actions = %d, err %v; want 1", len(actionsA), err)
	}
	actionsB, err := repo.ListActions(ctx, mustEntityFilter(t, pair.entityB), "", "", "")
	if err != nil || len(actionsB) != 0 {
		t.Fatalf("entity B saw entity A actions: %d, err %v", len(actionsB), err)
	}
	if got, err := repo.GetActionByIdempotency(ctx, mustEntityFilter(t, pair.entityA), "action-key-a"); err != nil || got == nil {
		t.Fatalf("entity A action idempotency lookup = %+v, err %v; want found", got, err)
	}
	if got, err := repo.GetActionByIdempotency(ctx, mustEntityFilter(t, pair.entityB), "action-key-a"); err != nil || got != nil {
		t.Fatalf("entity B action idempotency lookup = %+v, err %v; want nil", got, err)
	}
	if updated, err := repo.UpdateAction(ctx, createdAction.ID, mustEntityFilter(t, pair.entityB), "", FPnAActionItem{Status: "acknowledged"}); err != nil || updated != nil {
		t.Fatalf("entity B updated entity A action: %+v, err %v; want nil", updated, err)
	}
	if updated, err := repo.UpdateAction(ctx, createdAction.ID, mustEntityFilter(t, pair.entityA), "", FPnAActionItem{Status: "acknowledged"}); err != nil || updated == nil {
		t.Fatalf("entity A action update = %+v, err %v; want updated", updated, err)
	}

	// Assumptions: List.
	assumption := &FPnAAssumptionVersion{
		LegalEntityID: &pair.entityA, AssumptionKey: "rent-growth", Category: "occupancy",
		Value: json.RawMessage(`{"rate": 0.05}`), Source: "char-test",
		EffectiveFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if _, err := repo.CreateAssumption(ctx, assumption); err != nil {
		t.Fatalf("seed assumption: %v", err)
	}
	assumptionsA, err := repo.ListAssumptions(ctx, mustEntityFilter(t, pair.entityA), "")
	if err != nil || len(assumptionsA) != 1 {
		t.Fatalf("entity A assumptions = %d, err %v; want 1", len(assumptionsA), err)
	}
	assumptionsB, err := repo.ListAssumptions(ctx, mustEntityFilter(t, pair.entityB), "")
	if err != nil || len(assumptionsB) != 0 {
		t.Fatalf("entity B saw entity A assumptions: %d, err %v", len(assumptionsB), err)
	}

	// Critical-date brief.
	var ignoredDate string
	if err := pool.QueryRow(ctx, `
		INSERT INTO critical_dates (contract_id, date_type, target_date, title)
		VALUES ($1, 'lease_expiry', DATE '2026-07-15', 'Lease end A')
		RETURNING id
	`, pair.contractA).Scan(&ignoredDate); err != nil {
		t.Fatalf("seed critical date: %v", err)
	}
	briefA, err := repo.ListCriticalDateBrief(ctx, mustEntityFilter(t, pair.entityA), "2026-07", 30)
	if err != nil || len(briefA) != 1 {
		t.Fatalf("entity A critical-date brief = %d, err %v; want 1", len(briefA), err)
	}
	briefB, err := repo.ListCriticalDateBrief(ctx, mustEntityFilter(t, pair.entityB), "2026-07", 30)
	if err != nil || len(briefB) != 0 {
		t.Fatalf("entity B saw entity A critical dates: %d, err %v", len(briefB), err)
	}

	// Overview counts every dimension and must be empty for entity B.
	overviewA, err := repo.Overview(ctx, mustEntityFilter(t, pair.entityA), "2026-07")
	if err != nil {
		t.Fatalf("entity A overview: %v", err)
	}
	if overviewA.StoreFactCount != 1 || overviewA.EquipmentFactCount != 1 || overviewA.OpenActionCount != 1 {
		t.Fatalf("entity A overview counts = store %d, equipment %d, actions %d; want 1/1/1",
			overviewA.StoreFactCount, overviewA.EquipmentFactCount, overviewA.OpenActionCount)
	}
	overviewB, err := repo.Overview(ctx, mustEntityFilter(t, pair.entityB), "2026-07")
	if err != nil {
		t.Fatalf("entity B overview: %v", err)
	}
	if overviewB.StoreFactCount != 0 || overviewB.EquipmentFactCount != 0 || overviewB.OpenActionCount != 0 {
		t.Fatalf("entity B overview counts = store %d, equipment %d, actions %d; want 0/0/0",
			overviewB.StoreFactCount, overviewB.EquipmentFactCount, overviewB.OpenActionCount)
	}
}

func float64Ptr(value float64) *float64 { return &value }
