package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/lease-management-system/core-service/internal/services/retailsimulation"
)

func TestCashPlanPostgresReadOperatingHighestVersionDeduplication(t *testing.T) {
	pool := retailStoreDayFactsPool(t)
	ctx := context.Background()
	entityA, storeA := seedRetailStoreDayFactsTenant(t, ctx, pool, "cashplan-dedup-a")
	entityB, _ := seedRetailStoreDayFactsTenant(t, ctx, pool, "cashplan-dedup-b")

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		cleanup := func(statement string, args ...interface{}) {
			if _, err := pool.Exec(cleanupCtx, statement, args...); err != nil {
				t.Errorf("Cashplan PostgreSQL cleanup failed: %v", err)
			}
		}
		cleanup(`DELETE FROM retail_store_day_facts f USING stores s WHERE f.store_id=s.id AND s.legal_entity_id IN ($1,$2)`, entityA, entityB)
		cleanup(`DELETE FROM retail_simulation_datasets WHERE legal_entity_id IN ($1,$2)`, entityA, entityB)
		cleanup(`DELETE FROM operating_fact_batches WHERE legal_entity_id IN ($1,$2) AND source_system='retail_simulator'`, entityA, entityB)
		cleanup(`DELETE FROM stores WHERE legal_entity_id IN ($1,$2)`, entityA, entityB)
		cleanup(`DELETE FROM legal_entities WHERE id IN ($1,$2)`, entityA, entityB)
	})

	// Seed 2 store-day facts for storeA: 2026-01-01 and 2026-01-02
	if _, err := pool.Exec(ctx, `
		INSERT INTO retail_store_day_facts (
			store_id, business_date, currency, revenue, gross_profit, transactions, footfall, area_sqm,
			labor_cost, fixed_rent, variable_rent, non_lease_cost, other_controllable_cost,
			source_system, version, reconciliation_status, mapping_status, data_quality_status, data_classification
		) VALUES
		($1, '2026-01-01', 'CNY', 1000.0, 400.0, 50, 200, 100, 200.0, 100.0, 50.0, 30.0, 20.0, 'pos', 1, 'unreconciled', 'mapped', 'valid', 'production'),
		($1, '2026-01-02', 'CNY', 2000.0, 800.0, 100, 400, 100, 200.0, 100.0, 50.0, 30.0, 20.0, 'pos', 1, 'unreconciled', 'mapped', 'valid', 'production')
	`, storeA); err != nil {
		t.Fatalf("insert initial facts: %v", err)
	}

	cashRepo := NewCashPlanRepository(pool)

	// Step 1: Query initial state
	facts, err := cashRepo.ReadOperating(ctx, entityA, "2026-01", "2026-01", "production", "", []string{storeA})
	if err != nil {
		t.Fatalf("ReadOperating error: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("expected 1 monthly summary, got %d", len(facts))
	}
	if facts[0].Revenue != 3000.0 {
		t.Fatalf("expected initial revenue 3000.0, got %f", facts[0].Revenue)
	}

	// Step 2: Insert a version 2 correction for 2026-01-01 with revenue 1500.0 (old was 1000.0)
	if _, err := pool.Exec(ctx, `
		INSERT INTO retail_store_day_facts (
			store_id, business_date, currency, revenue, gross_profit, transactions, footfall, area_sqm,
			labor_cost, fixed_rent, variable_rent, non_lease_cost, other_controllable_cost,
			source_system, version, reconciliation_status, mapping_status, data_quality_status, data_classification
		) VALUES
		($1, '2026-01-01', 'CNY', 1500.0, 600.0, 60, 250, 100, 200.0, 100.0, 50.0, 30.0, 20.0, 'pos', 2, 'unreconciled', 'mapped', 'valid', 'production')
	`, storeA); err != nil {
		t.Fatalf("insert corrected v2 fact: %v", err)
	}

	// Step 3: Re-query and verify that only the highest version (v2) is used, NOT summed (which would be 4500.0)
	factsAfter, err := cashRepo.ReadOperating(ctx, entityA, "2026-01", "2026-01", "production", "", []string{storeA})
	if err != nil {
		t.Fatalf("ReadOperating after correction error: %v", err)
	}
	if len(factsAfter) != 1 {
		t.Fatalf("expected 1 monthly summary, got %d", len(factsAfter))
	}

	// Expected revenue = 1500.0 (v2 for Jan 1) + 2000.0 (v1 for Jan 2) = 3500.0
	// If double counting existed (v1 + v2 for Jan 1 + Jan 2), it would be 4500.0
	if factsAfter[0].Revenue != 3500.0 {
		t.Fatalf("expected revenue 3500.0 (deduplicated to highest version), got %f (if 4500.0, double counting occurred!)", factsAfter[0].Revenue)
	}
	if factsAfter[0].GrossProfit != 1400.0 { // 600 + 800
		t.Fatalf("expected gross profit 1400.0, got %f", factsAfter[0].GrossProfit)
	}

	// Step 4: Ensure cross-tenant isolation holds
	factsOther, err := cashRepo.ReadOperating(ctx, entityB, "2026-01", "2026-01", "production", "", []string{storeA})
	if err != nil {
		t.Fatalf("ReadOperating other tenant error: %v", err)
	}
	if len(factsOther) != 0 {
		t.Fatalf("expected 0 facts for other tenant, got %d", len(factsOther))
	}
}

func TestCashPlanPostgresSimulationDatasetHighestVersion(t *testing.T) {
	pool := retailStoreDayFactsPool(t)
	ctx := context.Background()
	entityA, _ := seedRetailStoreDayFactsTenant(t, ctx, pool, "cashplan-sim-a")

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		cleanup := func(statement string, args ...interface{}) {
			if _, err := pool.Exec(cleanupCtx, statement, args...); err != nil {
				t.Errorf("Cashplan PostgreSQL cleanup failed: %v", err)
			}
		}
		cleanup(`DELETE FROM retail_store_day_facts f USING stores s WHERE f.store_id=s.id AND s.legal_entity_id = $1`, entityA)
		cleanup(`DELETE FROM fpna_plan_versions WHERE legal_entity_id = $1 AND source='retail_simulator_budget'`, entityA)
		cleanup(`DELETE FROM retail_simulation_datasets WHERE legal_entity_id = $1`, entityA)
		cleanup(`DELETE FROM operating_fact_batches WHERE legal_entity_id = $1 AND source_system='retail_simulator'`, entityA)
		cleanup(`DELETE FROM stores WHERE legal_entity_id = $1`, entityA)
		cleanup(`DELETE FROM legal_entities WHERE id = $1`, entityA)
	})

	simRepo := NewRetailSimulationRepository(pool)
	payloadA, inputA, err := retailsimulation.PayloadSHA256(entityA, retailsimulation.Input{})
	if err != nil {
		t.Fatal(err)
	}
	planA, err := retailsimulation.Build(entityA, inputA)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := simRepo.Generate(ctx, entityA, nil, "cashplan-sim-generate", payloadA, planA); err != nil {
		t.Fatal(err)
	}

	cashRepo := NewCashPlanRepository(pool)
	facts, err := cashRepo.ReadOperating(ctx, entityA, "2026-01", "2026-06", "simulated", planA.DatasetVersion, nil)
	if err != nil {
		t.Fatalf("ReadOperating simulated error: %v", err)
	}
	if len(facts) == 0 {
		t.Fatalf("expected simulated facts, got 0")
	}

	var initialTotalRevenue float64
	for _, f := range facts {
		initialTotalRevenue += f.Revenue
	}

	// Insert duplicate version for a store on 2026-01-01
	var storeID string
	if err := pool.QueryRow(ctx, `SELECT id FROM stores WHERE legal_entity_id=$1 AND data_classification='simulated' AND simulation_dataset_version=$2 ORDER BY code LIMIT 1`, entityA, planA.DatasetVersion).Scan(&storeID); err != nil {
		t.Fatal(err)
	}

	var oldRevenue float64
	if err := pool.QueryRow(ctx, `SELECT revenue FROM retail_store_day_facts WHERE store_id=$1 AND business_date='2026-01-01' AND version=1`, storeID).Scan(&oldRevenue); err != nil {
		t.Fatal(err)
	}

	diffRevenue := 500.0
	newRevenue := oldRevenue + diffRevenue
	if _, err := pool.Exec(ctx, `
		INSERT INTO retail_store_day_facts (
			store_id, business_date, currency, revenue, gross_profit, transactions, footfall, area_sqm,
			labor_cost, fixed_rent, variable_rent, non_lease_cost, other_controllable_cost,
			source_system, version, reconciliation_status, mapping_status, data_quality_status,
			data_classification, simulation_dataset_version
		) VALUES (
			$1, '2026-01-01', 'CNY', $2, 30000, 1000, 2000, 100, 10000, 1000, 100, 100, 100,
			'retail_simulator', 2, 'unreconciled', 'mapped', 'valid', 'simulated', $3
		)
	`, storeID, newRevenue, planA.DatasetVersion); err != nil {
		t.Fatal(err)
	}

	factsAfter, err := cashRepo.ReadOperating(ctx, entityA, "2026-01", "2026-06", "simulated", planA.DatasetVersion, nil)
	if err != nil {
		t.Fatalf("ReadOperating simulated after v2 error: %v", err)
	}
	var newTotalRevenue float64
	for _, f := range factsAfter {
		newTotalRevenue += f.Revenue
	}

	expectedTotal := initialTotalRevenue + diffRevenue
	if newTotalRevenue != expectedTotal {
		t.Fatalf("expected total revenue %f (initial %f + diff %f), got %f", expectedTotal, initialTotalRevenue, diffRevenue, newTotalRevenue)
	}
}

// TestCashPlanPostgresReadOperatingSourceConflictMatchesKPILayer pins the KPI-layer
// semantics for multi-source store-days: both sources survive the per-source dedup,
// and the conflict check raises ErrRetailKPISourceConflict instead of silently
// dropping one source (or double-counting both). The conflict error itself is the
// proof that both rows were retained — if either source had been dropped, there
// would be no conflict.
func TestCashPlanPostgresReadOperatingSourceConflictMatchesKPILayer(t *testing.T) {
	pool := retailStoreDayFactsPool(t)
	ctx := context.Background()
	entityA, storeA := seedRetailStoreDayFactsTenant(t, ctx, pool, "cashplan-conflict")
	t.Cleanup(func() {
		cleanup := func(statement string, args ...interface{}) {
			if _, err := pool.Exec(context.Background(), statement, args...); err != nil {
				t.Errorf("Cashplan conflict cleanup failed: %v", err)
			}
		}
		cleanup(`DELETE FROM retail_store_day_facts f USING stores s WHERE f.store_id=s.id AND s.legal_entity_id = $1`, entityA)
		cleanup(`DELETE FROM stores WHERE legal_entity_id = $1`, entityA)
		cleanup(`DELETE FROM legal_entities WHERE id = $1`, entityA)
	})

	const insertFact = `
		INSERT INTO retail_store_day_facts (
			store_id, business_date, currency, revenue, gross_profit, transactions, footfall, area_sqm,
			labor_cost, fixed_rent, variable_rent, non_lease_cost, other_controllable_cost,
			source_system, version, reconciliation_status, mapping_status, data_quality_status, data_classification
		) VALUES
		($1, '2026-02-10', 'CNY', $2, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		 $3, 1, 'unreconciled', 'mapped', 'valid', 'production')
	`
	if _, err := pool.Exec(ctx, insertFact, storeA, 1000.0, "pos"); err != nil {
		t.Fatalf("insert pos fact: %v", err)
	}
	if _, err := pool.Exec(ctx, insertFact, storeA, 1200.0, "erp"); err != nil {
		t.Fatalf("insert erp fact: %v", err)
	}

	cashRepo := NewCashPlanRepository(pool)
	_, err := cashRepo.ReadOperating(ctx, entityA, "2026-02", "2026-02", "production", "", []string{storeA})
	if !errors.Is(err, ErrRetailKPISourceConflict) {
		t.Fatalf("expected source conflict for two-source store-day, got %v", err)
	}
}
