package repository_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/handlers"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailscenario"
	"github.com/lease-management-system/core-service/internal/services/retailsimulation"
)

// This is deliberately an integration test: the scenario service consumes the
// production KPI Repository, so the fixture proves tenant/source/version
// filtering rather than testing a parallel in-memory fact path.
func TestRetailScenarioPostgresGoldenIsolationAndZeroTouch(t *testing.T) {
	pool := pulsePostgresPool(t)
	ctx := context.Background()
	entityA := seedPulseTenant(t, ctx, pool, "scenario-a")
	entityB := seedPulseTenant(t, ctx, pool, "scenario-b")
	t.Cleanup(func() {
		cleanup := func(sql string, args ...interface{}) {
			if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
				t.Errorf("scenario cleanup: %v", err)
			}
		}
		cleanup(`DELETE FROM fpna_action_items WHERE legal_entity_id IN ($1,$2)`, entityA, entityB)
		cleanup(`DELETE FROM retail_store_day_fact_requests WHERE scope_key IN ($1,$2)`, entityA, entityB)
		cleanup(`DELETE FROM retail_store_day_facts f USING stores s WHERE f.store_id=s.id AND s.legal_entity_id IN ($1,$2)`, entityA, entityB)
		cleanup(`DELETE FROM fpna_plan_versions WHERE legal_entity_id IN ($1,$2) AND source='retail_simulator_budget'`, entityA, entityB)
		cleanup(`DELETE FROM retail_simulation_datasets WHERE legal_entity_id IN ($1,$2)`, entityA, entityB)
		cleanup(`DELETE FROM operating_fact_batches WHERE legal_entity_id IN ($1,$2) AND source_system='retail_simulator'`, entityA, entityB)
		cleanup(`DELETE FROM stores WHERE legal_entity_id IN ($1,$2)`, entityA, entityB)
		cleanup(`DELETE FROM legal_entities WHERE id IN ($1,$2)`, entityA, entityB)
		var residual int
		if err := pool.QueryRow(context.Background(), `SELECT
			(SELECT COUNT(*) FROM legal_entities WHERE id IN ($1,$2)) +
			(SELECT COUNT(*) FROM stores WHERE legal_entity_id IN ($1,$2)) +
			(SELECT COUNT(*) FROM operating_fact_batches WHERE legal_entity_id IN ($1,$2)) +
			(SELECT COUNT(*) FROM retail_simulation_datasets WHERE legal_entity_id IN ($1,$2)) +
			(SELECT COUNT(*) FROM fpna_action_items WHERE legal_entity_id IN ($1,$2)) +
			(SELECT COUNT(*) FROM retail_store_day_fact_requests WHERE scope_key::text IN ($1::text,$2::text)) +
			(SELECT COUNT(*) FROM retail_store_day_facts f JOIN stores s ON s.id=f.store_id WHERE s.legal_entity_id IN ($1,$2))`, entityA, entityB).Scan(&residual); err != nil {
			t.Errorf("scenario residual check: %v", err)
		} else if residual != 0 {
			t.Errorf("scenario residual rows=%d", residual)
		}
	})

	productionStore := queryProductionStoreID(t, ctx, pool, entityA)
	if _, err := pool.Exec(ctx, `INSERT INTO retail_store_day_facts (store_id,business_date,currency,revenue,gross_profit,transactions,footfall,area_sqm,labor_cost,fixed_rent,variable_rent,non_lease_cost,other_controllable_cost,source_system,version,reconciliation_status,mapping_status,data_quality_status,data_classification) SELECT $1, d::date, 'CNY', 1000, 300, 100, 500, 100, 100, 80, 10, 10, 20, 'scenario-production-fixture', 1, 'unreconciled', 'mapped', 'valid', 'production' FROM generate_series(DATE '2026-06-03', DATE '2026-06-30', INTERVAL '1 day') d`, productionStore); err != nil {
		t.Fatalf("production fixture: %v", err)
	}
	before := scenarioBoundaryCounts(t, ctx, pool)
	repo := repository.NewRetailSimulationRepository(pool)
	payloadA, inputA, err := retailsimulation.PayloadSHA256(entityA, retailsimulation.Input{})
	if err != nil {
		t.Fatal(err)
	}
	planA, err := retailsimulation.Build(entityA, inputA)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Generate(ctx, entityA, nil, "scenario-generate-a", payloadA, planA); err != nil {
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
	if _, err := repo.Generate(ctx, entityB, nil, "scenario-generate-b", payloadB, planB); err != nil {
		t.Fatal(err)
	}

	storeA := queryStoreID(t, ctx, pool, entityA, "simulated", planA.DatasetVersion, planA.Stores[4].Code)
	storeB := queryStoreID(t, ctx, pool, entityB, "simulated", planB.DatasetVersion, planB.Stores[4].Code)
	countingDB := &pulseCountingDB{DBTX: pool}
	service := retailscenario.NewService(repository.NewRetailKPIRepository(countingDB))
	result, err := service.Evaluate(ctx, retailscenario.Query{LegalEntityID: entityA, StoreID: storeA, AsOf: mustDate("2026-06-30"), WindowDays: 28, Classification: "simulated", DatasetVersion: planA.DatasetVersion, SourceSystem: "retail_simulator"}, retailscenario.EvaluateRequest{HorizonMonths: 12, Scenarios: []retailscenario.ScenarioInput{{Key: "baseline", Name: "Baseline"}, {Key: "plan", Name: "Plan", Assumptions: retailscenario.Assumptions{RevenueChangePct: 10, GrossMarginRateChangePP: 2, LaborCostChangePct: -5, FixedRentChangePct: -10}}}})
	if err != nil {
		t.Fatalf("simulated scenario: %v", err)
	}
	if result.DataClassification != "simulated" || result.DatasetVersion != planA.DatasetVersion || result.SourceSystem != "retail_simulator" || result.Store.StoreID != storeA || result.Evidence.FactVersionMax < 1 {
		t.Fatalf("simulated provenance=%+v", result)
	}
	if result.Baseline.Metrics["revenue"].Result == nil || result.Scenarios[0].Metrics["store_contribution"].Result == nil {
		t.Fatalf("scenario metrics unexpectedly null=%+v", result)
	}
	if countingDB.queryCount != 2 || countingDB.queryRowCount != 0 {
		t.Fatalf("scenario QueryFacts SQL count=%d query_rows=%d, want 2+0 with explicit source", countingDB.queryCount, countingDB.queryRowCount)
	}

	// The action endpoint re-evaluates the same server-side facts and uses the
	// existing FP&A action table only. A replay is byte-stable and a changed
	// payload with the same key is a conflict.
	gin.SetMode(gin.TestMode)
	actionHandler := handlers.NewRetailScenarioHandler(repository.NewRetailKPIRepository(pool), repository.NewOperatingFactsRepository(pool))
	actionRouter := gin.New()
	actionRouter.POST("/stores/:store_id/scenario-action-drafts", func(c *gin.Context) {
		c.Set("legal_entity_id", entityA)
		c.Set("access_scope", access.Scope{LegalEntityID: entityA})
		actionHandler.SaveAction(c)
	})
	actionRouter.POST("/b/stores/:store_id/scenario-action-drafts", func(c *gin.Context) {
		c.Set("legal_entity_id", entityB)
		c.Set("access_scope", access.Scope{LegalEntityID: entityB})
		actionHandler.SaveAction(c)
	})
	actionPath := "/stores/" + storeA + "/scenario-action-drafts?data_classification=simulated&dataset_version=" + planA.DatasetVersion + "&as_of=2026-06-30&window_days=28&source_system=retail_simulator"
	actionBody := `{"horizon_months":12,"selected_scenario":{"key":"plan","name":"Plan","assumptions":{"revenue_change_pct":10,"gross_margin_rate_change_pp":2,"labor_cost_change_pct":-5,"fixed_rent_change_pct":-10,"variable_rent_rate_change_pp":0,"non_lease_cost_change_pct":0,"other_controllable_cost_change_pct":0}},"title":"Scenario action","planned_action":"Verify staffing and rent plan","owner_name":"Reviewer A","due_date":"2026-08-31","verification_period":"2026-08"}`
	var actionCount int
	// Two genuinely first-time requests race on a fresh key. Neither request
	// can rely on the pre-read replay path; the database winner is returned to
	// both callers and the loser is marked as an idempotent replay.
	const concurrentFirstKey = "scenario-pg-concurrent-first"
	startConcurrent := make(chan struct{})
	var concurrentFirst sync.WaitGroup
	concurrentStatuses := make(chan int, 2)
	concurrentIDs := make(chan string, 2)
	concurrentReplays := make(chan bool, 2)
	for i := 0; i < 2; i++ {
		concurrentFirst.Add(1)
		go func() {
			defer concurrentFirst.Done()
			<-startConcurrent
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, actionPath, strings.NewReader(actionBody))
			req.Header.Set("Idempotency-Key", concurrentFirstKey)
			actionRouter.ServeHTTP(recorder, req)
			concurrentStatuses <- recorder.Code
			var envelope struct {
				Data             repository.FPnAActionItem `json:"data"`
				IdempotentReplay bool                      `json:"idempotent_replay"`
			}
			if json.Unmarshal(recorder.Body.Bytes(), &envelope) == nil {
				concurrentIDs <- envelope.Data.ID
				concurrentReplays <- envelope.IdempotentReplay
			}
		}()
	}
	close(startConcurrent)
	concurrentFirst.Wait()
	close(concurrentStatuses)
	close(concurrentIDs)
	close(concurrentReplays)
	for status := range concurrentStatuses {
		if status != http.StatusOK {
			t.Fatalf("first concurrent action status=%d", status)
		}
	}
	var concurrentFirstID string
	for id := range concurrentIDs {
		if id == "" {
			t.Fatal("first concurrent action returned empty id")
		}
		if concurrentFirstID == "" {
			concurrentFirstID = id
		} else if concurrentFirstID != id {
			t.Fatalf("first concurrent action ids=%s and %s", concurrentFirstID, id)
		}
	}
	replayCount := 0
	for replay := range concurrentReplays {
		if replay {
			replayCount++
		}
	}
	if replayCount < 1 {
		t.Fatalf("first concurrent action replay count=%d, want at least one", replayCount)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM fpna_action_items WHERE legal_entity_id=$1 AND idempotency_key=$2`, entityA, concurrentFirstKey).Scan(&actionCount); err != nil || actionCount != 1 {
		t.Fatalf("first concurrent action rows=%d err=%v", actionCount, err)
	}
	firstAction := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, actionPath, strings.NewReader(actionBody))
	request.Header.Set("Idempotency-Key", "scenario-pg-key")
	actionRouter.ServeHTTP(firstAction, request)
	if firstAction.Code != http.StatusOK {
		t.Fatalf("first action status=%d body=%s", firstAction.Code, firstAction.Body.String())
	}
	var firstEnvelope struct {
		Data repository.FPnAActionItem `json:"data"`
	}
	if err := json.Unmarshal(firstAction.Body.Bytes(), &firstEnvelope); err != nil {
		t.Fatal(err)
	}
	if firstEnvelope.Data.ID == "" || firstEnvelope.Data.Status != "open" || firstEnvelope.Data.OwnerName != "Reviewer A" || firstEnvelope.Data.DueDate == nil || firstEnvelope.Data.SourceRecordID != storeA || firstEnvelope.Data.BaselineAmount == nil || firstEnvelope.Data.TargetAmount == nil || firstEnvelope.Data.ExpectedBenefit == nil || !strings.Contains(string(firstEnvelope.Data.Evidence), `"request_fingerprint"`) {
		t.Fatalf("action persistence=%+v", firstEnvelope.Data)
	}
	var firstID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM fpna_action_items WHERE legal_entity_id=$1 AND idempotency_key='scenario-pg-key'`, entityA).Scan(&firstID); err != nil || firstID != firstEnvelope.Data.ID {
		t.Fatalf("action real id db=%s envelope=%s err=%v", firstID, firstEnvelope.Data.ID, err)
	}
	replayAction := httptest.NewRecorder()
	replayRequest := httptest.NewRequest(http.MethodPost, actionPath, strings.NewReader(actionBody))
	replayRequest.Header.Set("Idempotency-Key", "scenario-pg-key")
	actionRouter.ServeHTTP(replayAction, replayRequest)
	if replayAction.Code != http.StatusOK || !strings.Contains(replayAction.Body.String(), `"idempotent_replay":true`) {
		t.Fatalf("replay action status=%d body=%s", replayAction.Code, replayAction.Body.String())
	}
	conflictAction := httptest.NewRecorder()
	conflictRequest := httptest.NewRequest(http.MethodPost, actionPath, strings.NewReader(strings.Replace(actionBody, "Scenario action", "Changed action", 1)))
	conflictRequest.Header.Set("Idempotency-Key", "scenario-pg-key")
	actionRouter.ServeHTTP(conflictAction, conflictRequest)
	if conflictAction.Code != http.StatusConflict {
		t.Fatalf("conflict action status=%d body=%s", conflictAction.Code, conflictAction.Body.String())
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM fpna_action_items WHERE legal_entity_id=$1 AND idempotency_key='scenario-pg-key' AND status='open' AND category='retail_store_scenario'`, entityA).Scan(&actionCount); err != nil {
		t.Fatal(err)
	}
	if actionCount != 1 {
		t.Fatalf("scenario action count=%d, want one open row", actionCount)
	}
	secondAction := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodPost, actionPath, strings.NewReader(strings.Replace(actionBody, "Scenario action", "Scenario action v2", 1)))
	secondRequest.Header.Set("Idempotency-Key", "scenario-pg-key-v2")
	actionRouter.ServeHTTP(secondAction, secondRequest)
	if secondAction.Code != http.StatusOK {
		t.Fatalf("new key action status=%d body=%s", secondAction.Code, secondAction.Body.String())
	}
	var secondEnvelope struct {
		Data repository.FPnAActionItem `json:"data"`
	}
	if err := json.Unmarshal(secondAction.Body.Bytes(), &secondEnvelope); err != nil {
		t.Fatal(err)
	}
	if secondEnvelope.Data.ID == "" || secondEnvelope.Data.ID == firstEnvelope.Data.ID || secondEnvelope.Data.Title != "Scenario action v2" {
		t.Fatalf("new key action=%+v", secondEnvelope.Data)
	}
	var secondID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM fpna_action_items WHERE legal_entity_id=$1 AND idempotency_key='scenario-pg-key-v2' AND source_table='retail_store_day_facts' AND source_record_id=$2 AND category='retail_store_scenario'`, entityA, storeA).Scan(&secondID); err != nil || secondID != secondEnvelope.Data.ID {
		t.Fatalf("new key action real id db=%s envelope=%s err=%v", secondID, secondEnvelope.Data.ID, err)
	}
	if firstID == secondID {
		t.Fatalf("new key action reused first id=%s", firstID)
	}
	var firstKeyRows, secondKeyRows int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM fpna_action_items WHERE legal_entity_id=$1 AND idempotency_key='scenario-pg-key' AND source_table='retail_store_day_facts' AND source_record_id=$2 AND category='retail_store_scenario'`, entityA, storeA).Scan(&firstKeyRows); err != nil || firstKeyRows != 1 {
		t.Fatalf("scenario-pg-key rows=%d err=%v", firstKeyRows, err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM fpna_action_items WHERE legal_entity_id=$1 AND idempotency_key='scenario-pg-key-v2' AND source_table='retail_store_day_facts' AND source_record_id=$2 AND category='retail_store_scenario'`, entityA, storeA).Scan(&secondKeyRows); err != nil || secondKeyRows != 1 {
		t.Fatalf("scenario-pg-key-v2 rows=%d err=%v", secondKeyRows, err)
	}

	// A different legal entity can safely reuse the same idempotency key.
	bForeignPath := "/b/stores/" + storeA + "/scenario-action-drafts?data_classification=simulated&dataset_version=" + planA.DatasetVersion + "&as_of=2026-06-30&window_days=28&source_system=retail_simulator"
	bForeign := httptest.NewRecorder()
	bForeignRequest := httptest.NewRequest(http.MethodPost, bForeignPath, strings.NewReader(actionBody))
	bForeignRequest.Header.Set("Idempotency-Key", "scenario-b-foreign-store")
	actionRouter.ServeHTTP(bForeign, bForeignRequest)
	if bForeign.Code != http.StatusNotFound {
		t.Fatalf("B saving A store status=%d body=%s", bForeign.Code, bForeign.Body.String())
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM fpna_action_items WHERE legal_entity_id=$1 AND idempotency_key='scenario-b-foreign-store'`, entityB).Scan(&actionCount); err != nil || actionCount != 0 {
		t.Fatalf("B foreign action rows=%d err=%v", actionCount, err)
	}

	bActionPath := "/b/stores/" + storeB + "/scenario-action-drafts?data_classification=simulated&dataset_version=" + planB.DatasetVersion + "&as_of=2026-06-30&window_days=28&source_system=retail_simulator"
	bAction := httptest.NewRecorder()
	bRequest := httptest.NewRequest(http.MethodPost, bActionPath, strings.NewReader(actionBody))
	bRequest.Header.Set("Idempotency-Key", "scenario-pg-key")
	actionRouter.ServeHTTP(bAction, bRequest)
	if bAction.Code != http.StatusOK {
		t.Fatalf("B action status=%d body=%s", bAction.Code, bAction.Body.String())
	}
	var bEnvelope struct {
		Data repository.FPnAActionItem `json:"data"`
	}
	if err := json.Unmarshal(bAction.Body.Bytes(), &bEnvelope); err != nil {
		t.Fatal(err)
	}
	if bEnvelope.Data.ID == "" || bEnvelope.Data.ID == firstEnvelope.Data.ID || bEnvelope.Data.LegalEntityID == nil || *bEnvelope.Data.LegalEntityID != entityB {
		t.Fatalf("B action isolation=%+v", bEnvelope.Data)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM fpna_action_items WHERE idempotency_key='scenario-pg-key' AND legal_entity_id IN ($1,$2)`, entityA, entityB).Scan(&actionCount); err != nil || actionCount != 2 {
		t.Fatalf("cross entity action rows=%d err=%v", actionCount, err)
	}
	if _, err := service.Evaluate(ctx, retailscenario.Query{LegalEntityID: entityA, StoreID: storeB, AsOf: mustDate("2026-06-30"), WindowDays: 28, Classification: "simulated", DatasetVersion: planA.DatasetVersion, SourceSystem: "retail_simulator"}, retailscenario.EvaluateRequest{HorizonMonths: 12, Scenarios: []retailscenario.ScenarioInput{{Key: "baseline", Name: "Baseline"}, {Key: "plan", Name: "Plan"}}}); !errors.Is(err, retailscenario.ErrStoreNotFound) {
		t.Fatalf("cross-tenant scenario error=%v", err)
	}
	if _, err := service.Evaluate(access.WithScope(ctx, access.Scope{LegalEntityID: entityA, StoreIDs: []string{storeA}}), retailscenario.Query{LegalEntityID: entityA, StoreID: storeA, AsOf: mustDate("2026-06-30"), WindowDays: 28, Classification: "simulated", DatasetVersion: planA.DatasetVersion, SourceSystem: "retail_simulator"}, retailscenario.EvaluateRequest{HorizonMonths: 3, Scenarios: []retailscenario.ScenarioInput{{Key: "baseline", Name: "Baseline"}, {Key: "plan", Name: "Plan"}}}); err != nil {
		t.Fatalf("store scope scenario: %v", err)
	}
	if _, err := service.Evaluate(access.WithScope(ctx, access.Scope{LegalEntityID: entityA, StoreIDs: []string{storeB}}), retailscenario.Query{LegalEntityID: entityA, StoreID: storeA, AsOf: mustDate("2026-06-30"), WindowDays: 28, Classification: "simulated", DatasetVersion: planA.DatasetVersion, SourceSystem: "retail_simulator"}, retailscenario.EvaluateRequest{HorizonMonths: 3, Scenarios: []retailscenario.ScenarioInput{{Key: "baseline", Name: "Baseline"}, {Key: "plan", Name: "Plan"}}}); !errors.Is(err, retailscenario.ErrStoreNotFound) {
		t.Fatalf("store scope leak error=%v", err)
	}
	region := planA.Stores[4].Region
	if _, err := service.Evaluate(access.WithScope(ctx, access.Scope{LegalEntityID: entityA, Regions: []string{region}}), retailscenario.Query{LegalEntityID: entityA, StoreID: storeA, AsOf: mustDate("2026-06-30"), WindowDays: 28, Classification: "simulated", DatasetVersion: planA.DatasetVersion, SourceSystem: "retail_simulator"}, retailscenario.EvaluateRequest{HorizonMonths: 3, Scenarios: []retailscenario.ScenarioInput{{Key: "baseline", Name: "Baseline"}, {Key: "plan", Name: "Plan"}}}); err != nil {
		t.Fatalf("region scope scenario: %v", err)
	}
	brand := planA.Stores[4].Brand
	if _, err := service.Evaluate(access.WithScope(ctx, access.Scope{LegalEntityID: entityA, Brands: []string{brand}}), retailscenario.Query{LegalEntityID: entityA, StoreID: storeA, AsOf: mustDate("2026-06-30"), WindowDays: 28, Classification: "simulated", DatasetVersion: planA.DatasetVersion, SourceSystem: "retail_simulator"}, retailscenario.EvaluateRequest{HorizonMonths: 3, Scenarios: []retailscenario.ScenarioInput{{Key: "baseline", Name: "Baseline"}, {Key: "plan", Name: "Plan"}}}); err != nil {
		t.Fatalf("brand scope scenario: %v", err)
	}
	if _, err := service.Evaluate(access.WithScope(ctx, access.Scope{LegalEntityID: entityA, Regions: []string{region + "-denied"}}), retailscenario.Query{LegalEntityID: entityA, StoreID: storeA, AsOf: mustDate("2026-06-30"), WindowDays: 28, Classification: "simulated", DatasetVersion: planA.DatasetVersion, SourceSystem: "retail_simulator"}, retailscenario.EvaluateRequest{HorizonMonths: 3, Scenarios: []retailscenario.ScenarioInput{{Key: "baseline", Name: "Baseline"}, {Key: "plan", Name: "Plan"}}}); !errors.Is(err, retailscenario.ErrStoreNotFound) {
		t.Fatalf("mismatched region scope error=%v", err)
	}
	if _, err := service.Evaluate(access.WithScope(ctx, access.Scope{LegalEntityID: entityA, Brands: []string{brand + "-denied"}}), retailscenario.Query{LegalEntityID: entityA, StoreID: storeA, AsOf: mustDate("2026-06-30"), WindowDays: 28, Classification: "simulated", DatasetVersion: planA.DatasetVersion, SourceSystem: "retail_simulator"}, retailscenario.EvaluateRequest{HorizonMonths: 3, Scenarios: []retailscenario.ScenarioInput{{Key: "baseline", Name: "Baseline"}, {Key: "plan", Name: "Plan"}}}); !errors.Is(err, retailscenario.ErrStoreNotFound) {
		t.Fatalf("mismatched brand scope error=%v", err)
	}

	production, err := service.Evaluate(ctx, retailscenario.Query{LegalEntityID: entityA, StoreID: productionStore, AsOf: mustDate("2026-06-30"), WindowDays: 28, Classification: "production", SourceSystem: "scenario-production-fixture"}, retailscenario.EvaluateRequest{HorizonMonths: 3, Scenarios: []retailscenario.ScenarioInput{{Key: "baseline", Name: "Baseline"}, {Key: "plan", Name: "Plan"}}})
	if err != nil || production.DataClassification != "production" || production.DatasetVersion != "" || production.Evidence.ExpectedStoreDays != 28 {
		t.Fatalf("production scenario=%+v err=%v", production, err)
	}
	after := scenarioBoundaryCounts(t, ctx, pool)
	if before != after {
		t.Fatalf("scenario changed IFRS/production boundary before=%+v after=%+v", before, after)
	}
	t.Logf("scenario simulated+production SQL queries=%d query_rows=%d", countingDB.queryCount, countingDB.queryRowCount)
}

func queryStoreID(t *testing.T, ctx context.Context, pool *pgxpool.Pool, entity, classification, dataset, code string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM stores WHERE legal_entity_id=$1 AND data_classification=$2 AND simulation_dataset_version=$3 AND code=$4`, entity, classification, dataset, code).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func queryProductionStoreID(t *testing.T, ctx context.Context, pool *pgxpool.Pool, entity string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM stores WHERE legal_entity_id=$1 AND data_classification='production' ORDER BY code LIMIT 1`, entity).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

type scenarioBoundarySnapshot struct{ LeaseContracts, Measurements, Journals, MonthlyClosingBatches, OfficialReports, ProductionFacts int }

func scenarioBoundaryCounts(t *testing.T, ctx context.Context, pool *pgxpool.Pool) scenarioBoundarySnapshot {
	t.Helper()
	var result scenarioBoundarySnapshot
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM lease_contracts`).Scan(&result.LeaseContracts); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM measurement_results`).Scan(&result.Measurements); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM journal_entries`).Scan(&result.Journals); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM monthly_closing_batches`).Scan(&result.MonthlyClosingBatches); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM fpna_report_artifacts WHERE basis='Official'`).Scan(&result.OfficialReports); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM retail_store_day_facts WHERE data_classification='production'`).Scan(&result.ProductionFacts); err != nil {
		t.Fatal(err)
	}
	return result
}
