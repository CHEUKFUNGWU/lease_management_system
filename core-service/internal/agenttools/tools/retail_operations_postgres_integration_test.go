package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailpulse"
	"github.com/lease-management-system/core-service/internal/services/retailsimulation"
)

// Reviewer runs this with TEST_DATABASE_URL on the compose network and
// -count=2. It covers both data classifications, tenant/dimension scope,
// deterministic read results, fixed query count and business-table zero write.
func TestRetailOperationsPostgresIsolationNoWrites(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	db, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	dbCtx := context.Background()
	// Every invocation gets its own fixture token. Cleanup is registered before
	// the first INSERT so a failed seed still gets removed, and the post-run
	// cleanup only targets this invocation (not another concurrent test).
	runToken := uuid.NewString()[:12]
	runPrefix := "AGENT-LE-" + runToken + "-"
	runPattern := runPrefix + "%"
	cleanupStatements := []string{
		`DELETE FROM retail_store_day_facts WHERE store_id IN (SELECT s.id FROM stores s JOIN legal_entities le ON le.id=s.legal_entity_id WHERE le.code LIKE $1)`,
		`DELETE FROM retail_simulation_datasets WHERE legal_entity_id IN (SELECT id FROM legal_entities WHERE code LIKE $1)`,
		`DELETE FROM operating_fact_batches WHERE legal_entity_id IN (SELECT id FROM legal_entities WHERE code LIKE $1)`,
		`DELETE FROM stores WHERE legal_entity_id IN (SELECT id FROM legal_entities WHERE code LIKE $1)`,
		`DELETE FROM legal_entities WHERE code LIKE $1`,
	}
	cleanupPattern := func(ctx context.Context, pattern string) error {
		for _, statement := range cleanupStatements {
			if _, err := db.Exec(ctx, statement, pattern); err != nil {
				return fmt.Errorf("cleanup %q: %w", statement, err)
			}
		}
		return nil
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		if err := cleanupPattern(cleanupCtx, runPattern); err != nil {
			t.Errorf("%v", err)
		}
		checks := []struct {
			name  string
			query string
		}{
			{"legal_entities", `SELECT COUNT(*) FROM legal_entities WHERE code LIKE $1`},
			{"stores", `SELECT COUNT(*) FROM stores s JOIN legal_entities le ON le.id=s.legal_entity_id WHERE le.code LIKE $1`},
			{"facts", `SELECT COUNT(*) FROM retail_store_day_facts f JOIN stores s ON s.id=f.store_id JOIN legal_entities le ON le.id=s.legal_entity_id WHERE le.code LIKE $1`},
			{"datasets", `SELECT COUNT(*) FROM retail_simulation_datasets d JOIN legal_entities le ON le.id=d.legal_entity_id WHERE le.code LIKE $1`},
			{"batches", `SELECT COUNT(*) FROM operating_fact_batches b JOIN legal_entities le ON le.id=b.legal_entity_id WHERE le.code LIKE $1`},
		}
		for _, check := range checks {
			var residual int64
			if err := db.QueryRow(cleanupCtx, check.query, runPattern).Scan(&residual); err != nil {
				t.Errorf("cleanup residual %s: %v", check.name, err)
			} else if residual != 0 {
				t.Errorf("cleanup residual %s=%d, want 0", check.name, residual)
			}
		}
		db.Close()
	})
	// Remove only the legacy, pre-token fixtures left by the first version of
	// this test. New runs use runPrefix and therefore cannot be removed by a
	// concurrent invocation's cleanup.
	for _, legacyPattern := range []string{"AGENT-LE-a-%", "AGENT-LE-b-%"} {
		if err := cleanupPattern(dbCtx, legacyPattern); err != nil {
			t.Fatal(err)
		}
	}
	type seedFixture struct {
		entity string
		plan   *retailsimulation.Plan
	}
	seed := func(label string) seedFixture {
		var entity string
		if err := db.QueryRow(dbCtx, `INSERT INTO legal_entities(code,name,country,currency,is_active) VALUES($1,$2,'CN','CNY',true) RETURNING id`, runPrefix+label, "Agent tenant "+label).Scan(&entity); err != nil {
			t.Fatal(err)
		}
		hash, normalized, err := retailsimulation.PayloadSHA256(entity, retailsimulation.Input{})
		if err != nil {
			t.Fatal(err)
		}
		plan, err := retailsimulation.Build(entity, normalized)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := repository.NewRetailSimulationRepository(db).Generate(dbCtx, entity, nil, "agent-pg-"+label, hash, plan); err != nil {
			t.Fatal(err)
		}
		return seedFixture{entity: entity, plan: plan}
	}
	fixtureA, fixtureB := seed("a"), seed("b")
	entityA, entityB := fixtureA.entity, fixtureB.entity
	var expectedAnomaly *retailsimulation.Anomaly
	for i := range fixtureA.plan.Anomalies {
		if fixtureA.plan.Anomalies[i].Type == "occupancy_cost_burden" {
			expectedAnomaly = &fixtureA.plan.Anomalies[i]
			break
		}
	}
	if expectedAnomaly == nil {
		t.Fatal("fixed-seed occupancy anomaly is missing")
	}
	expectedStoreCode := expectedAnomaly.StoreCode
	var productionStoreID, productionBatchID string
	productionCode := "AGENT-PROD-" + uuid.NewString()[:8]
	if err := db.QueryRow(dbCtx, `INSERT INTO stores(code,name,legal_entity_id,brand,region,data_classification) VALUES($1,'Agent production fixture',$2,'FixtureBrand','FixtureRegion','production') RETURNING id`, productionCode, entityA).Scan(&productionStoreID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(dbCtx, `INSERT INTO operating_fact_batches(legal_entity_id,source_system,status,total_rows,accepted_rows,reconciliation_status) VALUES($1,'agent-production-fixture','completed',14,14,'matched') RETURNING id`, entityA).Scan(&productionBatchID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 14; i++ {
		businessDate := time.Date(2026, 6, 17+i, 0, 0, 0, 0, time.UTC)
		if _, err := db.Exec(dbCtx, `INSERT INTO retail_store_day_facts(store_id,business_date,currency,revenue,gross_profit,transactions,footfall,area_sqm,labor_cost,fixed_rent,variable_rent,non_lease_cost,other_controllable_cost,source_system,source_record_id,import_batch_id,as_of_at,version,reconciliation_status,mapping_status,data_quality_status,data_classification) VALUES($1,$2::date,'CNY',1000,300,100,200,100,100,50,20,10,5,'agent-production-fixture',$3,$4,$5::timestamptz,1,'matched','mapped','valid','production')`, productionStoreID, businessDate, "prod-"+businessDate.Format("20060102"), productionBatchID, businessDate.Add(12*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}

	var storeID, dataset, dateTo string
	if err := db.QueryRow(dbCtx, `SELECT s.id::text,d.dataset_version,d.date_to::text FROM stores s JOIN retail_simulation_datasets d ON d.legal_entity_id=s.legal_entity_id WHERE s.legal_entity_id=$1 AND s.code=$2 AND d.status='completed' ORDER BY d.completed_at DESC LIMIT 1`, entityA, expectedStoreCode).Scan(&storeID, &dataset, &dateTo); err != nil {
		t.Fatal(err)
	}
	tableCounts := func() map[string]int64 {
		counts := map[string]int64{}
		for _, table := range []string{"fpna_action_items", "fpna_scenario_drafts", "lease_contracts", "lease_events", "lease_payment_schedules", "measurement_results", "journal_entries"} {
			var count int64
			if err := db.QueryRow(dbCtx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
				t.Fatalf("count %s: %v", table, err)
			}
			counts[table] = count
		}
		return counts
	}
	before := tableCounts()
	newContext := func(scope access.Scope, runID string) context.Context {
		base := access.WithScope(context.Background(), scope)
		return agenttools.WithExecutionContext(base, agenttools.ExecutionContext{Principal: agenttools.Principal{UserID: "agent-pg", Permissions: []string{"reports:read"}, Scope: scope}, RunID: runID, SkillID: retailOperationsSkill, SkillVersion: "v1"})
	}
	ctx := newContext(access.Scope{LegalEntityID: entityA}, "agent-pg")
	reader := &countingRetailReader{base: repository.NewRetailKPIRepository(db)}
	pulse := NewRetailOperatingPulseDefinition(reader)
	diagnostics := NewRetailStoreDiagnosticsDefinition(reader)
	scenario := NewRetailScenarioEvaluateDefinition(reader)
	invoke := func(callCtx context.Context, definition agenttools.ToolDefinition, args any) agenttools.ToolResult {
		result, err := definition.Handler(callCtx, agenttools.ToolCall{CallID: definition.Descriptor.Name, RunID: "agent-pg", ToolName: definition.Descriptor.Name, ToolVersion: "v1", Arguments: mustRetailJSON(args)})
		if err != nil || result.Status != agenttools.StatusCompleted {
			t.Fatalf("%s: result=%+v err=%v", definition.Descriptor.Name, result, err)
		}
		return result
	}
	pulseArgs := RetailOperatingPulseArguments{}
	// The fixed occupancy anomaly is in the 2026-05-30..06-05
	// window; keep this as-of fixed so the Agent path is checked against the
	// committed Pulse Golden rather than an arbitrary dataset end date.
	pulseArgs.AsOf, pulseArgs.WindowDays, pulseArgs.DataClass, pulseArgs.DatasetVersion = "2026-06-05", 7, "simulated", dataset
	first, second := invoke(ctx, pulse, pulseArgs), invoke(ctx, pulse, pulseArgs)
	if string(stableRetailJSON(first.Data)) != string(stableRetailJSON(second.Data)) {
		t.Fatal("pulse result is not deterministic after removing generated_at")
	}
	pulseData, ok := first.Data.(RetailPulseToolData)
	if !ok || pulseData.Response == nil || pulseData.CurrentCoverage.ObservedStoreDays != 420 || pulseData.CurrentCoverage.ExpectedStoreDays != 420 || pulseData.ComparisonCoverage.ObservedStoreDays != 420 || pulseData.ComparisonCoverage.ExpectedStoreDays != 420 {
		t.Fatalf("pulse fixed-seed coverage=%#v", first.Data)
	}
	var goldenStore *retailpulse.Attention
	for i := range pulseData.Attention {
		if pulseData.Attention[i].StoreCode == expectedStoreCode {
			goldenStore = &pulseData.Attention[i]
			break
		}
	}
	if goldenStore == nil || goldenStore.Rank != 1 || goldenStore.Severity != "high" || math.Abs(goldenStore.Score-3.02) > 0.011 {
		t.Fatalf("pulse %s Golden=%#v", expectedStoreCode, goldenStore)
	}
	var occupancySignal *retailpulse.Signal
	for i := range goldenStore.ObservedSignals {
		if goldenStore.ObservedSignals[i].SignalCode == "occupancy_cost_rate_spike" {
			occupancySignal = &goldenStore.ObservedSignals[i]
			break
		}
	}
	if occupancySignal == nil || occupancySignal.ObservedChange == nil || math.Abs(*occupancySignal.ObservedChange-10.08) > 0.011 {
		t.Fatalf("pulse %s occupancy signal Golden=%#v", expectedStoreCode, occupancySignal)
	}
	diagnosticArgs := RetailStoreDiagnosticsArguments{}
	diagnosticArgs.AsOf, diagnosticArgs.WindowDays, diagnosticArgs.DataClass, diagnosticArgs.DatasetVersion, diagnosticArgs.StoreID = "2026-06-05", 7, "simulated", dataset, storeID
	diagnosticResult := invoke(ctx, diagnostics, diagnosticArgs)
	if diagnosticData, ok := diagnosticResult.Data.(RetailDiagnosticsToolData); !ok || diagnosticData.Response == nil || diagnosticData.Store.StoreCode != expectedStoreCode {
		t.Fatalf("%s diagnostics=%#v", expectedStoreCode, diagnosticResult.Data)
	}
	scenarioArgs := RetailScenarioEvaluateArguments{}
	scenarioArgs.AsOf, scenarioArgs.WindowDays, scenarioArgs.DataClass, scenarioArgs.DatasetVersion, scenarioArgs.StoreID = "2026-06-05", 7, "simulated", dataset, storeID
	scenarioArgs.HorizonMonths = 12
	scenarioArgs.Assumptions.LaborCostChangePct = -10
	scenarioResult := invoke(ctx, scenario, scenarioArgs)
	if scenarioData, ok := scenarioResult.Data.(RetailScenarioToolData); !ok || scenarioData.Response == nil || scenarioData.HorizonMonths != 12 || len(scenarioData.Scenarios) != 1 || scenarioData.Scenarios[0].Assumptions.LaborCostChangePct != -10 || scenarioData.Baseline.Metrics["store_contribution"].Result == nil || scenarioData.Scenarios[0].Bridge.Status != "complete" {
		t.Fatalf("%s labor scenario=%#v", expectedStoreCode, scenarioResult.Data)
	}
	productionArgs := RetailOperatingPulseArguments{}
	productionArgs.AsOf, productionArgs.WindowDays, productionArgs.DataClass, productionArgs.SourceSystem = dateTo, 7, "production", "agent-production-fixture"
	productionResult := invoke(ctx, pulse, productionArgs)
	if data, ok := productionResult.Data.(RetailPulseToolData); !ok || data.DataClassification != "production" || data.DatasetVersion != "" || len(data.SourceSystems) != 1 || data.SourceSystems[0] != "agent-production-fixture" || !data.DecisionReady {
		t.Fatalf("production fixture result=%#v", productionResult.Data)
	}
	var region, brand string
	if err := db.QueryRow(dbCtx, `SELECT region,brand FROM stores WHERE id=$1`, storeID).Scan(&region, &brand); err != nil {
		t.Fatal(err)
	}
	scopedCtx := newContext(access.Scope{LegalEntityID: entityA, Regions: []string{region}, Brands: []string{brand}}, "agent-scope")
	scopedResult := invoke(scopedCtx, pulse, pulseArgs)
	if data, ok := scopedResult.Data.(RetailPulseToolData); !ok || len(data.RequestedStores) == 0 {
		t.Fatalf("region/brand scope hid all selected stores: %#v", scopedResult.Data)
	} else {
		for _, store := range data.RequestedStores {
			if store.Region != region || store.Brand != brand {
				t.Fatalf("region/brand scope leaked store: %+v", store)
			}
		}
	}
	foreignCtx := newContext(access.Scope{LegalEntityID: entityB}, "agent-b")
	foreign, _ := diagnostics.Handler(foreignCtx, agenttools.ToolCall{CallID: "foreign", RunID: "agent-b", ToolName: diagnostics.Descriptor.Name, ToolVersion: "v1", Arguments: mustRetailJSON(diagnosticArgs)})
	if foreign.Status == agenttools.StatusCompleted {
		t.Fatal("entity B read entity A store")
	}
	if reader.calls != 7 {
		t.Fatalf("unexpected QueryFacts count: got %d want 7", reader.calls)
	}
	after := tableCounts()
	for table, count := range before {
		if count != after[table] {
			t.Fatalf("read tools changed %s: %d -> %d", table, count, after[table])
		}
	}
}

type countingRetailReader struct {
	base  *repository.RetailKPIRepository
	calls int
}

func (r *countingRetailReader) QueryFacts(ctx context.Context, legal, from, to, class, dataset, source string, stores []string) (*repository.RetailKPIFactSet, error) {
	r.calls++
	return r.base.QueryFacts(ctx, legal, from, to, class, dataset, source, stores)
}

func stableRetailJSON(value any) []byte {
	raw, _ := json.Marshal(value)
	var object map[string]any
	if json.Unmarshal(raw, &object) != nil {
		return raw
	}
	delete(object, "generated_at")
	// ENV-001: the Source Envelope carries its own generated_at; both are
	// wall-clock timestamps and must not break determinism checks.
	if envelope, ok := object["envelope"].(map[string]any); ok {
		delete(envelope, "generated_at")
	}
	result, _ := json.Marshal(object)
	return result
}

func mustRetailJSON(value any) []byte { raw, _ := json.Marshal(value); return raw }
