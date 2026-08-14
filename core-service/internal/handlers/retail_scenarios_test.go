package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

type scenarioHandlerReader struct {
	set *repository.RetailKPIFactSet
	err error
}

func (r scenarioHandlerReader) QueryFacts(context.Context, string, string, string, string, string, string, []string) (*repository.RetailKPIFactSet, error) {
	return r.set, r.err
}

func scenarioFact(date time.Time, store string) retailkpi.DailyFact {
	p := func(value float64) *float64 { return &value }
	dataset := "planA-v1"
	return retailkpi.DailyFact{StoreID: store, StoreCode: "S005", StoreName: "门店5", Brand: "品牌A", Region: "华东", BusinessDate: date, Currency: "CNY", SourceSystem: "retail_simulator", DataClassification: "simulated", SimulationDatasetVersion: &dataset, Version: 1, Revenue: p(1000), GrossProfit: p(300), Transactions: p(100), Footfall: p(500), AreaSqm: p(100), LaborCost: p(100), FixedRent: p(80), VariableRent: p(10), NonLeaseCost: p(10), OtherControllableCost: p(20)}
}

func scenarioSet() *repository.RetailKPIFactSet {
	store := "00000000-0000-0000-0000-000000000005"
	facts := make([]retailkpi.DailyFact, 0, 7)
	for i := 0; i < 7; i++ {
		facts = append(facts, scenarioFact(time.Date(2026, 6, 1+i, 0, 0, 0, 0, time.UTC), store))
	}
	return &repository.RetailKPIFactSet{Facts: facts, ExpectedStores: []retailkpi.StorePopulation{{StoreID: store, StoreCode: "S005", StoreName: "门店5", Brand: "品牌A", Region: "华东"}}}
}

func scenarioRouter(reader scenarioHandlerReader) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewRetailScenarioHandler(reader, nil)
	r := gin.New()
	r.POST("/stores/:store_id/scenarios/evaluate", func(c *gin.Context) { c.Set("legal_entity_id", "entity-a"); h.Evaluate(c) })
	r.POST("/empty/:store_id/scenarios/evaluate", h.Evaluate)
	return r
}

func scenarioBody() string {
	return `{"horizon_months":12,"scenarios":[{"key":"baseline","name":"Baseline","assumptions":{"revenue_change_pct":0,"gross_margin_rate_change_pp":0,"labor_cost_change_pct":0,"fixed_rent_change_pct":0,"variable_rent_rate_change_pp":0,"non_lease_cost_change_pct":0,"other_controllable_cost_change_pct":0}},{"key":"plan","name":"Plan","assumptions":{"revenue_change_pct":10,"gross_margin_rate_change_pp":2,"labor_cost_change_pct":-5,"fixed_rent_change_pct":-10,"variable_rent_rate_change_pp":0,"non_lease_cost_change_pct":0,"other_controllable_cost_change_pct":0}}]}`
}

func TestRetailScenarioHandlerEvaluateEnvelopeAndEmptyTenant(t *testing.T) {
	r := scenarioRouter(scenarioHandlerReader{set: scenarioSet()})
	path := "/stores/00000000-0000-0000-0000-000000000005/scenarios/evaluate?data_classification=simulated&dataset_version=planA-v1&as_of=2026-06-07&window_days=7&source_system=retail_simulator"
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, strings.NewReader(scenarioBody())))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["basis"] != "Scenario" || envelope["ifrs16_impact"] != false {
		t.Fatalf("envelope=%v", envelope)
	}
	empty := httptest.NewRecorder()
	r.ServeHTTP(empty, httptest.NewRequest(http.MethodPost, "/empty/00000000-0000-0000-0000-000000000005/scenarios/evaluate?data_classification=simulated&dataset_version=planA-v1&as_of=2026-06-07&window_days=7", strings.NewReader(scenarioBody())))
	if empty.Code != http.StatusBadRequest {
		t.Fatalf("empty tenant status=%d", empty.Code)
	}
}

func TestRetailScenarioHandlerMapsValidationConflictAndUnavailable(t *testing.T) {
	store := "00000000-0000-0000-0000-000000000005"
	validPath := "/stores/" + store + "/scenarios/evaluate?data_classification=production&as_of=2026-06-07&window_days=7"
	bad := httptest.NewRecorder()
	scenarioRouter(scenarioHandlerReader{set: scenarioSet()}).ServeHTTP(bad, httptest.NewRequest(http.MethodPost, validPath, strings.NewReader(`{}`)))
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("invalid status=%d", bad.Code)
	}
	conflict := httptest.NewRecorder()
	scenarioRouter(scenarioHandlerReader{err: repository.ErrRetailKPISourceConflict}).ServeHTTP(conflict, httptest.NewRequest(http.MethodPost, validPath, strings.NewReader(scenarioBody())))
	if conflict.Code != http.StatusConflict {
		t.Fatalf("conflict status=%d", conflict.Code)
	}
	unavailable := httptest.NewRecorder()
	scenarioRouter(scenarioHandlerReader{set: &repository.RetailKPIFactSet{ExpectedStores: []retailkpi.StorePopulation{{StoreID: store}}}}).ServeHTTP(unavailable, httptest.NewRequest(http.MethodPost, validPath, strings.NewReader(scenarioBody())))
	if unavailable.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unavailable status=%d", unavailable.Code)
	}
	var unavailableBody map[string]any
	if err := json.Unmarshal(unavailable.Body.Bytes(), &unavailableBody); err != nil {
		t.Fatal(err)
	}
	unavailableDetails, _ := unavailableBody["details"].(map[string]any)
	if unavailableBody["code"] != "data_unavailable" || unavailableDetails["reason"] != "no_facts" || unavailableDetails["evidence"] == nil {
		t.Fatalf("unavailable body=%v", unavailableBody)
	}
	resultingRate := strings.Replace(scenarioBody(), `"gross_margin_rate_change_pp":2`, `"gross_margin_rate_change_pp":100`, 1)
	rate := httptest.NewRecorder()
	scenarioRouter(scenarioHandlerReader{set: scenarioSet()}).ServeHTTP(rate, httptest.NewRequest(http.MethodPost, validPath+"&source_system=retail_simulator", strings.NewReader(resultingRate)))
	if rate.Code != http.StatusUnprocessableEntity {
		t.Fatalf("resulting rate status=%d body=%s", rate.Code, rate.Body.String())
	}
	var rateBody map[string]any
	if err := json.Unmarshal(rate.Body.Bytes(), &rateBody); err != nil {
		t.Fatal(err)
	}
	rateDetails, _ := rateBody["details"].(map[string]any)
	if rateBody["code"] != "data_unavailable" || rateDetails["reason"] != "resulting_rate_out_of_range" || rateDetails["evidence"] == nil {
		t.Fatalf("resulting rate body=%v", rateBody)
	}
	err500 := httptest.NewRecorder()
	scenarioRouter(scenarioHandlerReader{err: errors.New("db down")}).ServeHTTP(err500, httptest.NewRequest(http.MethodPost, validPath, strings.NewReader(scenarioBody())))
	if err500.Code != http.StatusInternalServerError {
		t.Fatalf("error status=%d", err500.Code)
	}
}
