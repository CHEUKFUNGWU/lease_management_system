package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/services/retailpulse"
	"github.com/lease-management-system/core-service/internal/services/retailsimulation"
)

type pulseGoldenFixture struct {
	Cases []pulseGoldenCase `json:"cases"`
}

type pulseGoldenCase struct {
	AsOf               string              `json:"as_of"`
	StoreIndex         int                 `json:"store_index"`
	SignalCode         string              `json:"signal_code"`
	Rank               int                 `json:"rank"`
	Severity           string              `json:"severity"`
	Score              float64             `json:"score"`
	ObservedChange     float64             `json:"observed_change"`
	Threshold          float64             `json:"threshold"`
	CurrentCoverage    pulseGoldenCoverage `json:"current_coverage"`
	ComparisonCoverage pulseGoldenCoverage `json:"comparison_coverage"`
	Summary            map[string]float64  `json:"summary"`
}

type pulseGoldenCoverage struct {
	ObservedStoreDays int     `json:"observed_store_days"`
	ExpectedStoreDays int     `json:"expected_store_days"`
	CoverageRate      float64 `json:"coverage_rate"`
}

type pulseCountingDB struct {
	repository.DBTX
	queryCount    int
	queryRowCount int
}

func (db *pulseCountingDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	db.queryCount++
	return db.DBTX.Query(ctx, sql, args...)
}

func (db *pulseCountingDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	db.queryRowCount++
	return db.DBTX.QueryRow(ctx, sql, args...)
}

func TestRetailPulsePostgresGoldenAffectsAndIsolation(t *testing.T) {
	pool := pulsePostgresPool(t)
	ctx := context.Background()
	entityA := seedPulseTenant(t, ctx, pool, "pulse-a")
	entityB := seedPulseTenant(t, ctx, pool, "pulse-b")
	t.Cleanup(func() {
		cleanup := func(sql string, args ...interface{}) {
			if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
				t.Errorf("MAX-004 cleanup: %v", err)
			}
		}
		cleanup(`DELETE FROM retail_store_day_facts f USING stores s WHERE f.store_id=s.id AND s.legal_entity_id IN ($1,$2)`, entityA, entityB)
		cleanup(`DELETE FROM fpna_plan_versions WHERE legal_entity_id IN ($1,$2) AND source='retail_simulator_budget'`, entityA, entityB)
		cleanup(`DELETE FROM retail_simulation_datasets WHERE legal_entity_id IN ($1,$2)`, entityA, entityB)
		cleanup(`DELETE FROM operating_fact_batches WHERE legal_entity_id IN ($1,$2) AND source_system='retail_simulator'`, entityA, entityB)
		cleanup(`DELETE FROM stores WHERE legal_entity_id IN ($1,$2)`, entityA, entityB)
		cleanup(`DELETE FROM legal_entities WHERE id IN ($1,$2)`, entityA, entityB)
	})
	repo := repository.NewRetailSimulationRepository(pool)
	payloadA, inputA, err := retailsimulation.PayloadSHA256(entityA, retailsimulation.Input{})
	if err != nil {
		t.Fatal(err)
	}
	planA, err := retailsimulation.Build(entityA, inputA)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Generate(ctx, entityA, nil, "pulse-generate-a", payloadA, planA); err != nil {
		t.Fatal(err)
	}
	payloadB, inputB, err := retailsimulation.PayloadSHA256(entityB, retailsimulation.Input{})
	if err != nil {
		t.Fatal(err)
	}
	planB, err := retailsimulation.Build(entityB, inputB)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Generate(ctx, entityB, nil, "pulse-generate-b", payloadB, planB); err != nil {
		t.Fatal(err)
	}
	var productionStoreID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM stores WHERE legal_entity_id=$1 AND data_classification='production' ORDER BY code LIMIT 1`, entityA).Scan(&productionStoreID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO retail_store_day_facts
			(store_id,business_date,currency,revenue,gross_profit,transactions,footfall,area_sqm,labor_cost,fixed_rent,variable_rent,non_lease_cost,other_controllable_cost,source_system,version,reconciliation_status,mapping_status,data_quality_status,data_classification)
		SELECT $1, business_date, 'CNY', 100, 40, 10, 20, 10, 10, 5, 1, 1, 1, 'pulse-production-fixture', 1, 'unreconciled', 'mapped', 'valid', 'production'
		FROM generate_series(DATE '2026-05-23', DATE '2026-06-05', INTERVAL '1 day') AS dates(business_date)`, productionStoreID); err != nil {
		t.Fatalf("insert production pulse facts: %v", err)
	}
	before := pulseBoundaryCounts(t, ctx, pool)
	golden := loadPulseGolden(t)

	service := retailpulse.NewService(repository.NewRetailKPIRepository(pool))
	scenarios := []struct {
		asOf              string
		storeCode, signal string
	}{
		{"2026-01-31", planA.Stores[1].Code, "footfall_decline"},
		{"2026-02-25", planA.Stores[2].Code, "conversion_drop"},
		{"2026-03-22", planA.Stores[3].Code, "average_ticket_drop"},
		{"2026-04-16", planA.Stores[4].Code, "gross_margin_compression"},
		{"2026-05-11", planA.Stores[5].Code, "labor_cost_rate_spike"},
		{"2026-06-05", planA.Stores[6].Code, "occupancy_cost_rate_spike"},
	}
	for _, scenario := range scenarios {
		asOf, _ := time.Parse("2006-01-02", scenario.asOf)
		response, err := service.Build(access.WithScope(ctx, access.Scope{LegalEntityID: entityA}), retailpulse.Query{LegalEntityID: entityA, AsOf: asOf, WindowDays: 7, Classification: "simulated", DatasetVersion: planA.DatasetVersion, AttentionLimit: 50})
		if err != nil {
			t.Fatalf("scenario %s: %v", scenario.asOf, err)
		}
		if !response.DecisionReady || response.MultiCurrency || len(response.DailyTrend) != 7 {
			t.Fatalf("scenario %s response envelope=%+v", scenario.asOf, response)
		}
		found := false
		for _, attention := range response.Attention {
			if attention.StoreCode == scenario.storeCode {
				for _, signal := range attention.ObservedSignals {
					if signal.SignalCode == scenario.signal {
						found = true
					}
				}
			}
		}
		if !found {
			t.Fatalf("scenario %s missing %s/%s", scenario.asOf, scenario.storeCode, scenario.signal)
		}
		assertPostgresPulseGolden(t, response, golden[scenario.asOf], planA, scenario.storeCode)
	}
	countingDB := &pulseCountingDB{DBTX: pool}
	countedService := retailpulse.NewService(repository.NewRetailKPIRepository(countingDB))
	started := time.Now()
	countedResponse, err := countedService.Build(access.WithScope(ctx, access.Scope{LegalEntityID: entityA}), retailpulse.Query{LegalEntityID: entityA, AsOf: mustDate("2026-06-05"), WindowDays: 7, Classification: "simulated", DatasetVersion: planA.DatasetVersion, AttentionLimit: 50})
	if err != nil {
		t.Fatalf("counted 60-store pulse: %v", err)
	}
	if countingDB.queryCount != 3 || countingDB.queryRowCount != 1 {
		t.Fatalf("pulse query count=%d query_row_count=%d, want fixed 3+1 (population/facts/lifecycles + conflict)", countingDB.queryCount, countingDB.queryRowCount)
	}
	assertPostgresPulseGolden(t, countedResponse, golden["2026-06-05"], planA, planA.Stores[6].Code)
	t.Logf("60-store pulse duration=%s SQL queries=%d query_rows=%d facts=%d", time.Since(started), countingDB.queryCount, countingDB.queryRowCount, len(countedResponse.Attention))

	var scopedStoreID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM stores WHERE legal_entity_id=$1 AND data_classification='simulated' AND simulation_dataset_version=$2 AND code=$3`, entityA, planA.DatasetVersion, planA.Stores[1].Code).Scan(&scopedStoreID); err != nil {
		t.Fatal(err)
	}
	region := planA.Stores[1].Region
	regionResponse, err := service.Build(access.WithScope(ctx, access.Scope{LegalEntityID: entityA, Regions: []string{region}}), retailpulse.Query{LegalEntityID: entityA, AsOf: mustDate("2026-06-05"), WindowDays: 7, Classification: "simulated", DatasetVersion: planA.DatasetVersion, SourceSystem: "retail_simulator", AttentionLimit: 50})
	if err != nil {
		t.Fatalf("region scoped pulse: %v", err)
	}
	if regionResponse.CurrentCoverage.ExpectedStoreDays != countPlanStores(planA, func(store retailsimulation.StorePlan) bool { return store.Region == region })*7 {
		t.Fatalf("region scope expected store-days=%d response=%d", countPlanStores(planA, func(store retailsimulation.StorePlan) bool { return store.Region == region })*7, regionResponse.CurrentCoverage.ExpectedStoreDays)
	}
	for _, attention := range regionResponse.Attention {
		if attention.Region != region {
			t.Fatalf("region scope attention leaked region=%s want=%s store=%s", attention.Region, region, attention.StoreCode)
		}
	}
	for _, suppressed := range regionResponse.SuppressedAttention {
		if suppressed.Region != region {
			t.Fatalf("region scope suppression leaked region=%s want=%s store=%s", suppressed.Region, region, suppressed.StoreCode)
		}
	}
	brand := planA.Stores[1].Brand
	brandResponse, err := service.Build(access.WithScope(ctx, access.Scope{LegalEntityID: entityA, Brands: []string{brand}}), retailpulse.Query{LegalEntityID: entityA, AsOf: mustDate("2026-06-05"), WindowDays: 7, Classification: "simulated", DatasetVersion: planA.DatasetVersion, SourceSystem: "retail_simulator", AttentionLimit: 50})
	if err != nil {
		t.Fatalf("brand scoped pulse: %v", err)
	}
	if brandResponse.CurrentCoverage.ExpectedStoreDays != countPlanStores(planA, func(store retailsimulation.StorePlan) bool { return store.Brand == brand })*7 {
		t.Fatalf("brand scope expected store-days=%d response=%d", countPlanStores(planA, func(store retailsimulation.StorePlan) bool { return store.Brand == brand })*7, brandResponse.CurrentCoverage.ExpectedStoreDays)
	}
	for _, attention := range brandResponse.Attention {
		if attention.Brand != brand {
			t.Fatalf("brand scope attention leaked brand=%s want=%s store=%s", attention.Brand, brand, attention.StoreCode)
		}
	}
	for _, suppressed := range brandResponse.SuppressedAttention {
		if suppressed.Brand != brand {
			t.Fatalf("brand scope suppression leaked brand=%s want=%s store=%s", suppressed.Brand, brand, suppressed.StoreCode)
		}
	}
	storeResponse, err := service.Build(access.WithScope(ctx, access.Scope{LegalEntityID: entityA}), retailpulse.Query{LegalEntityID: entityA, AsOf: mustDate("2026-06-05"), WindowDays: 7, Classification: "simulated", DatasetVersion: planA.DatasetVersion, SourceSystem: "retail_simulator", StoreIDs: []string{scopedStoreID}, AttentionLimit: 50})
	if err != nil {
		t.Fatalf("store scoped pulse: %v", err)
	}
	if storeResponse.CurrentCoverage.ExpectedStoreDays != 7 {
		t.Fatalf("store scope expected store-days=%d", storeResponse.CurrentCoverage.ExpectedStoreDays)
	}

	// Keep version 1 and add version 2 for the same store/date/source inside the
	// queried current window. The ranked Repository query must select version 2
	// without double-counting the store-day.
	if _, err := pool.Exec(ctx, `INSERT INTO retail_store_day_facts (store_id,business_date,currency,revenue,gross_profit,transactions,footfall,area_sqm,labor_cost,fixed_rent,variable_rent,non_lease_cost,other_controllable_cost,source_system,version,reconciliation_status,mapping_status,data_quality_status,data_classification,simulation_dataset_version) SELECT f.store_id,f.business_date,f.currency,f.revenue,f.gross_profit,f.transactions,f.footfall,f.area_sqm,f.labor_cost,f.fixed_rent,f.variable_rent,f.non_lease_cost,f.other_controllable_cost,f.source_system,2,f.reconciliation_status,f.mapping_status,f.data_quality_status,f.data_classification,f.simulation_dataset_version FROM retail_store_day_facts f WHERE f.store_id=$1 AND f.business_date='2026-06-01' AND f.source_system='retail_simulator' AND f.data_classification='simulated' AND f.simulation_dataset_version=$2`, scopedStoreID, planA.DatasetVersion); err != nil {
		t.Fatal(err)
	}
	highestResponse, err := service.Build(access.WithScope(ctx, access.Scope{LegalEntityID: entityA}), retailpulse.Query{LegalEntityID: entityA, AsOf: mustDate("2026-06-05"), WindowDays: 7, Classification: "simulated", DatasetVersion: planA.DatasetVersion, SourceSystem: "retail_simulator", AttentionLimit: 50})
	if err != nil || highestResponse.FactVersionMax != 2 || highestResponse.CurrentCoverage.ObservedStoreDays != 420 || highestResponse.CurrentCoverage.ExpectedStoreDays != 420 || highestResponse.ComparisonCoverage.ObservedStoreDays != 420 || highestResponse.ComparisonCoverage.ExpectedStoreDays != 420 {
		t.Fatalf("highest fact version response=%+v err=%v", highestResponse, err)
	}
	if highestResponse.Summary["revenue"].Current.Value == nil || countedResponse.Summary["revenue"].Current.Value == nil || math.Abs(*highestResponse.Summary["revenue"].Current.Value-*countedResponse.Summary["revenue"].Current.Value) > 0.011 {
		t.Fatalf("versioned row was duplicated or changed aggregate revenue before=%.2f after=%.2f", pulseValueOrZero(countedResponse.Summary["revenue"].Current.Value), pulseValueOrZero(highestResponse.Summary["revenue"].Current.Value))
	}

	// The same store-day receives another source: default selection is 409,
	// while an explicit source remains a valid, deterministic read.
	if _, err := pool.Exec(ctx, `INSERT INTO retail_store_day_facts (store_id,business_date,currency,revenue,gross_profit,transactions,footfall,area_sqm,labor_cost,fixed_rent,variable_rent,non_lease_cost,other_controllable_cost,source_system,version,reconciliation_status,mapping_status,data_quality_status,data_classification,simulation_dataset_version) VALUES ($1,'2026-06-01','CNY',100,30,10,20,10,10,1,1,1,1,'other_source',1,'unreconciled','mapped','valid','simulated',$2)`, scopedStoreID, planA.DatasetVersion); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Build(access.WithScope(ctx, access.Scope{LegalEntityID: entityA}), retailpulse.Query{LegalEntityID: entityA, AsOf: mustDate("2026-06-05"), WindowDays: 7, Classification: "simulated", DatasetVersion: planA.DatasetVersion, AttentionLimit: 50}); !errors.Is(err, repository.ErrRetailKPISourceConflict) {
		t.Fatalf("pulse source conflict=%v", err)
	}
	if _, err := service.Build(access.WithScope(ctx, access.Scope{LegalEntityID: entityA}), retailpulse.Query{LegalEntityID: entityA, AsOf: mustDate("2026-06-05"), WindowDays: 7, Classification: "simulated", DatasetVersion: planA.DatasetVersion, SourceSystem: "retail_simulator", AttentionLimit: 50}); err != nil {
		t.Fatalf("pulse explicit source=%v", err)
	}
	productionResponse, err := service.Build(access.WithScope(ctx, access.Scope{LegalEntityID: entityA}), retailpulse.Query{LegalEntityID: entityA, AsOf: mustDate("2026-06-05"), WindowDays: 7, Classification: "production", AttentionLimit: 50})
	if err != nil || productionResponse.DataClassification != "production" || productionResponse.DatasetVersion != "" || !productionResponse.DecisionReady || productionResponse.Currency != "CNY" || productionResponse.CurrentCoverage.ObservedStoreDays != 7 || productionResponse.CurrentCoverage.ExpectedStoreDays != 7 || productionResponse.ComparisonCoverage.ObservedStoreDays != 7 || productionResponse.ComparisonCoverage.ExpectedStoreDays != 7 || len(productionResponse.DailyTrend) != 7 {
		t.Fatalf("production-only pulse=%+v err=%v", productionResponse, err)
	}
	other, err := service.Build(access.WithScope(ctx, access.Scope{LegalEntityID: entityA}), retailpulse.Query{LegalEntityID: entityB, AsOf: mustDate("2026-06-05"), Classification: "simulated", DatasetVersion: planB.DatasetVersion, AttentionLimit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if other.DecisionReady || len(other.Attention) != 0 {
		t.Fatalf("cross-tenant pulse leaked: %+v", other)
	}
	after := pulseBoundaryCounts(t, ctx, pool)
	if before != after {
		t.Fatalf("pulse query changed IFRS16/production boundary before=%+v after=%+v", before, after)
	}
}

func pulseValueOrZero(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

type pulseBoundarySnapshot struct {
	LeaseContracts, Measurements, Journals, ProductionFacts int
}

func pulseBoundaryCounts(t *testing.T, ctx context.Context, pool *pgxpool.Pool) pulseBoundarySnapshot {
	t.Helper()
	var snapshot pulseBoundarySnapshot
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM lease_contracts`).Scan(&snapshot.LeaseContracts); err != nil {
		t.Fatalf("count pulse lease contracts: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM measurement_results`).Scan(&snapshot.Measurements); err != nil {
		t.Fatalf("count pulse measurements: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM journal_entries`).Scan(&snapshot.Journals); err != nil {
		t.Fatalf("count pulse journals: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM retail_store_day_facts WHERE data_classification='production'`).Scan(&snapshot.ProductionFacts); err != nil {
		t.Fatalf("count pulse production facts: %v", err)
	}
	return snapshot
}

func loadPulseGolden(t *testing.T) map[string]pulseGoldenCase {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve PostgreSQL test source path")
	}
	content, err := os.ReadFile(filepath.Join(filepath.Dir(sourceFile), "..", "services", "retailpulse", "testdata", "retail_pulse_v1_golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture pulseGoldenFixture
	if err := json.Unmarshal(content, &fixture); err != nil {
		t.Fatal(err)
	}
	result := make(map[string]pulseGoldenCase, len(fixture.Cases))
	for _, item := range fixture.Cases {
		result[item.AsOf] = item
	}
	return result
}

func assertPostgresPulseGolden(t *testing.T, response *retailpulse.Response, want pulseGoldenCase, plan *retailsimulation.Plan, storeCode string) {
	t.Helper()
	if response == nil || !response.DecisionReady || response.Currency != "CNY" || len(response.DailyTrend) != 7 {
		t.Fatalf("PostgreSQL pulse envelope=%+v", response)
	}
	assertPostgresCoverage(t, "current", response.CurrentCoverage, want.CurrentCoverage)
	assertPostgresCoverage(t, "comparison", response.ComparisonCoverage, want.ComparisonCoverage)
	var found *retailpulse.Attention
	for i := range response.Attention {
		if response.Attention[i].StoreCode == storeCode {
			found = &response.Attention[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("PostgreSQL pulse missing Golden store %s", storeCode)
	}
	if found.Rank != want.Rank || found.Severity != want.Severity || math.Abs(found.Score-want.Score) > 0.011 {
		t.Fatalf("PostgreSQL Golden rank/score/severity=%d/%.2f/%s want=%d/%.2f/%s", found.Rank, found.Score, found.Severity, want.Rank, want.Score, want.Severity)
	}
	var signal *retailpulse.Signal
	for i := range found.ObservedSignals {
		if found.ObservedSignals[i].SignalCode == want.SignalCode {
			signal = &found.ObservedSignals[i]
			break
		}
	}
	if signal == nil || signal.ObservedChange == nil || math.Abs(*signal.ObservedChange-want.ObservedChange) > 0.011 || math.Abs(signal.Threshold-want.Threshold) > 0.011 {
		t.Fatalf("PostgreSQL Golden signal=%+v want=%+v", signal, want)
	}
	for key, expected := range want.Summary {
		var actual *float64
		switch key {
		case "revenue_current":
			actual = response.Summary["revenue"].Current.Value
		case "revenue_comparison":
			actual = response.Summary["revenue"].Comparison.Value
		case "store_contribution_current":
			actual = response.Summary["store_contribution"].Current.Value
		case "store_contribution_comparison":
			actual = response.Summary["store_contribution"].Comparison.Value
		}
		if actual == nil || math.Abs(*actual-expected) > 0.011 {
			t.Fatalf("PostgreSQL Golden summary %s actual=%v want=%.2f", key, actual, expected)
		}
	}
	if len(plan.Stores) < 8 || found.StoreCode != storeCode {
		t.Fatalf("PostgreSQL Golden store mismatch=%s", found.StoreCode)
	}
}

func assertPostgresCoverage(t *testing.T, label string, actual retailkpi.Coverage, expected pulseGoldenCoverage) {
	t.Helper()
	if actual.ObservedStoreDays != expected.ObservedStoreDays || actual.ExpectedStoreDays != expected.ExpectedStoreDays || actual.CoverageRate == nil || math.Abs(*actual.CoverageRate-expected.CoverageRate) > 0.001 {
		t.Fatalf("PostgreSQL %s coverage=%+v want=%+v", label, actual, expected)
	}
}

func countPlanStores(plan *retailsimulation.Plan, predicate func(retailsimulation.StorePlan) bool) int {
	count := 0
	for _, store := range plan.Stores {
		if predicate(store) {
			count++
		}
	}
	return count
}

func pulsePostgresPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func seedPulseTenant(t *testing.T, ctx context.Context, pool *pgxpool.Pool, label string) string {
	t.Helper()
	suffix := uuid.NewString()[:8]
	var entityID string
	if err := pool.QueryRow(ctx, `INSERT INTO legal_entities (code,name,country,currency,is_active) VALUES ($1,$2,'CN','CNY',true) RETURNING id`, "PULSE-LE-"+label+"-"+suffix, "Pulse tenant "+label).Scan(&entityID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO stores (code,name,legal_entity_id,brand,region,is_active) VALUES ($1,$2,$3,$4,$5,true)`, "PULSE-ST-"+label+"-"+suffix, "Pulse seed store "+label, entityID, "Brand-"+label, "Region-"+label); err != nil {
		t.Fatal(err)
	}
	return entityID
}

func mustDate(value string) time.Time { result, _ := time.Parse("2006-01-02", value); return result }
