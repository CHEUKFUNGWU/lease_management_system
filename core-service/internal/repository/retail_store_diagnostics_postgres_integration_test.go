package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailsimulation"
	"github.com/lease-management-system/core-service/internal/services/retailstore360"
)

func TestRetailStoreDiagnosticsPostgresGoldenAndIsolation(t *testing.T) {
	pool := pulsePostgresPool(t)
	ctx := context.Background()
	entityA := seedPulseTenant(t, ctx, pool, "store360-a")
	entityB := seedPulseTenant(t, ctx, pool, "store360-b")
	t.Cleanup(func() {
		cleanup := func(sql string, args ...any) {
			if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
				t.Errorf("MAX-006 cleanup: %v", err)
			}
		}
		cleanup(`DELETE FROM retail_store_day_facts f USING stores s WHERE f.store_id=s.id AND s.legal_entity_id IN ($1,$2)`, entityA, entityB)
		cleanup(`DELETE FROM fpna_plan_versions WHERE legal_entity_id IN ($1,$2) AND source='retail_simulator_budget'`, entityA, entityB)
		cleanup(`DELETE FROM retail_simulation_datasets WHERE legal_entity_id IN ($1,$2)`, entityA, entityB)
		cleanup(`DELETE FROM operating_fact_batches WHERE legal_entity_id IN ($1,$2) AND source_system='retail_simulator'`, entityA, entityB)
		cleanup(`DELETE FROM stores WHERE legal_entity_id IN ($1,$2)`, entityA, entityB)
		cleanup(`DELETE FROM legal_entities WHERE id IN ($1,$2)`, entityA, entityB)
	})
	simRepo := repository.NewRetailSimulationRepository(pool)
	payload, input, err := retailsimulation.PayloadSHA256(entityA, retailsimulation.Input{})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := retailsimulation.Build(entityA, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := simRepo.Generate(ctx, entityA, nil, "store360-generate-a", payload, plan); err != nil {
		t.Fatal(err)
	}
	kpiRepo := repository.NewRetailKPIRepository(pool)
	options, err := kpiRepo.ListStorePopulation(access.WithScope(ctx, access.Scope{LegalEntityID: entityA}), entityA, "simulated", plan.DatasetVersion, nil)
	if err != nil || len(options) != 60 {
		t.Fatalf("store options count=%d err=%v", len(options), err)
	}
	if options[0].StoreCode > options[len(options)-1].StoreCode {
		t.Fatalf("store options are not stable sorted: first=%s last=%s", options[0].StoreCode, options[len(options)-1].StoreCode)
	}
	region := plan.Stores[1].Region
	regionOptions, err := kpiRepo.ListStorePopulation(access.WithScope(ctx, access.Scope{LegalEntityID: entityA, Regions: []string{region}}), entityA, "simulated", plan.DatasetVersion, nil)
	if err != nil || len(regionOptions) == 0 {
		t.Fatalf("region options=%d err=%v", len(regionOptions), err)
	}
	for _, option := range regionOptions {
		if option.Region != region {
			t.Fatalf("region scope leaked %s", option.Region)
		}
	}
	brand := plan.Stores[1].Brand
	brandOptions, err := kpiRepo.ListStorePopulation(access.WithScope(ctx, access.Scope{LegalEntityID: entityA, Brands: []string{brand}}), entityA, "simulated", plan.DatasetVersion, nil)
	if err != nil || len(brandOptions) == 0 {
		t.Fatalf("brand options=%d err=%v", len(brandOptions), err)
	}
	for _, option := range brandOptions {
		if option.Brand != brand {
			t.Fatalf("brand scope leaked %s", option.Brand)
		}
	}
	var targetID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM stores WHERE legal_entity_id=$1 AND data_classification='simulated' AND simulation_dataset_version=$2 AND code=$3`, entityA, plan.DatasetVersion, plan.Stores[1].Code).Scan(&targetID); err != nil {
		t.Fatal(err)
	}
	storeOptions, err := kpiRepo.ListStorePopulation(access.WithScope(ctx, access.Scope{LegalEntityID: entityA}), entityA, "simulated", plan.DatasetVersion, []string{targetID})
	if err != nil || len(storeOptions) != 1 || storeOptions[0].StoreID != targetID {
		t.Fatalf("store scope options=%+v err=%v", storeOptions, err)
	}

	counting := &pulseCountingDB{DBTX: pool}
	service := retailstore360.NewService(repository.NewRetailKPIRepository(counting))
	response, err := service.Build(access.WithScope(ctx, access.Scope{LegalEntityID: entityA}), retailstore360.Query{LegalEntityID: entityA, StoreID: targetID, AsOf: mustDate("2026-06-05"), WindowDays: 14, Classification: "simulated", DatasetVersion: plan.DatasetVersion})
	if err != nil {
		t.Fatal(err)
	}
	if response.Store.StoreCode != plan.Stores[1].Code || response.TargetCoverage.ExpectedStoreDays != 14 || response.ComparisonCoverage.ExpectedStoreDays != 14 || response.Evidence.FactVersionMax != 1 || len(response.DailyTrend) != 14 {
		t.Fatalf("store 360 envelope=%+v", response)
	}
	if counting.queryCount != 3 || counting.queryRowCount != 1 {
		t.Fatalf("store 360 query count=%d row=%d, want 3+1", counting.queryCount, counting.queryRowCount)
	}
	if len(response.Bridges) != 3 {
		t.Fatalf("bridges=%d", len(response.Bridges))
	}
	for name, scope := range map[string]access.Scope{
		"store":  {LegalEntityID: entityA, StoreIDs: []string{targetID}},
		"region": {LegalEntityID: entityA, Regions: []string{region}},
		"brand":  {LegalEntityID: entityA, Brands: []string{brand}},
	} {
		scoped, scopedErr := retailstore360.NewService(repository.NewRetailKPIRepository(pool)).Build(access.WithScope(ctx, scope), retailstore360.Query{LegalEntityID: entityA, StoreID: targetID, AsOf: mustDate("2026-06-05"), WindowDays: 14, Classification: "simulated", DatasetVersion: plan.DatasetVersion})
		if scopedErr != nil || scoped == nil || scoped.Store.StoreID != targetID {
			t.Fatalf("%s scoped diagnostics leaked/failed: response=%+v err=%v", name, scoped, scopedErr)
		}
	}
	if _, err := retailstore360.NewService(repository.NewRetailKPIRepository(pool)).Build(access.WithScope(ctx, access.Scope{LegalEntityID: entityA, Regions: []string{"not-a-region"}}), retailstore360.Query{LegalEntityID: entityA, StoreID: targetID, AsOf: mustDate("2026-06-05"), WindowDays: 14, Classification: "simulated", DatasetVersion: plan.DatasetVersion}); !errors.Is(err, retailstore360.ErrStoreNotFound) {
		t.Fatalf("out-of-scope diagnostics err=%v", err)
	}
	if _, err := retailstore360.NewService(repository.NewRetailKPIRepository(pool)).Build(access.WithScope(ctx, access.Scope{LegalEntityID: entityA, Brands: []string{"not-a-brand"}}), retailstore360.Query{LegalEntityID: entityA, StoreID: targetID, AsOf: mustDate("2026-06-05"), WindowDays: 14, Classification: "simulated", DatasetVersion: plan.DatasetVersion}); !errors.Is(err, retailstore360.ErrStoreNotFound) {
		t.Fatalf("out-of-scope brand diagnostics err=%v", err)
	}
	if _, err := retailstore360.NewService(repository.NewRetailKPIRepository(pool)).Build(access.WithScope(ctx, access.Scope{LegalEntityID: entityA, StoreIDs: []string{"00000000-0000-0000-0000-000000000000"}}), retailstore360.Query{LegalEntityID: entityA, StoreID: targetID, AsOf: mustDate("2026-06-05"), WindowDays: 14, Classification: "simulated", DatasetVersion: plan.DatasetVersion}); !errors.Is(err, retailstore360.ErrStoreNotFound) {
		t.Fatalf("out-of-scope store diagnostics err=%v", err)
	}

	other, err := service.Build(access.WithScope(ctx, access.Scope{LegalEntityID: entityB}), retailstore360.Query{LegalEntityID: entityB, StoreID: targetID, AsOf: mustDate("2026-06-05"), WindowDays: 14, Classification: "simulated", DatasetVersion: plan.DatasetVersion})
	if !errors.Is(err, retailstore360.ErrStoreNotFound) || other != nil {
		t.Fatalf("cross-tenant result=%v err=%v", other, err)
	}

	// Production is a separate read path and has no dataset version. Insert a
	// complete current/comparison fixture to prove production can compute while
	// remaining outside simulated dataset metadata.
	var productionID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM stores WHERE legal_entity_id=$1 AND data_classification='production' ORDER BY code LIMIT 1`, entityA).Scan(&productionID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO retail_store_day_facts (store_id,business_date,currency,revenue,gross_profit,transactions,footfall,area_sqm,labor_cost,fixed_rent,variable_rent,non_lease_cost,other_controllable_cost,source_system,version,reconciliation_status,mapping_status,data_quality_status,data_classification) SELECT $1, dates::date, 'CNY', 1000, 300, 100, 500, 100, 100, 80, 10, 10, 20, 'store360-production-fixture', 1, 'unreconciled', 'mapped', 'valid', 'production' FROM generate_series(DATE '2026-05-09', DATE '2026-06-05', INTERVAL '1 day') AS series(dates)`, productionID); err != nil {
		t.Fatal(err)
	}
	// Snapshot the accounting/Official boundary after the fixture is seeded;
	// the diagnostics read path must not mutate any of these tables.
	beforeBoundary := pulseBoundaryCounts(t, ctx, pool)
	production, productionErr := retailstore360.NewService(repository.NewRetailKPIRepository(pool)).Build(access.WithScope(ctx, access.Scope{LegalEntityID: entityA}), retailstore360.Query{LegalEntityID: entityA, StoreID: productionID, AsOf: mustDate("2026-06-05"), WindowDays: 14, Classification: "production"})
	if productionErr != nil || production == nil || production.DatasetVersion != "" || production.DataClassification != "production" || !production.DecisionReady || production.Currency != "CNY" || production.TargetCoverage.ObservedStoreDays != 14 || production.TargetCoverage.ExpectedStoreDays != 14 || production.ComparisonCoverage.ObservedStoreDays != 14 || production.ComparisonCoverage.ExpectedStoreDays != 14 {
		t.Fatalf("production envelope=%+v err=%v", production, productionErr)
	}
	if afterBoundary := pulseBoundaryCounts(t, ctx, pool); afterBoundary != beforeBoundary {
		t.Fatalf("store diagnostics touched IFRS16/Official boundary before=%+v after=%+v", beforeBoundary, afterBoundary)
	}
}
