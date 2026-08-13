package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

type fakeRetailKPIReader struct {
	result *repository.RetailKPIFactSet
	err    error
}

func (f fakeRetailKPIReader) QueryFacts(context.Context, string, string, string, string, string, string, []string) (*repository.RetailKPIFactSet, error) {
	return f.result, f.err
}

func TestRetailKPIHandlerProtectedContractAndValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	date := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	value := 100.0
	facts := []retailkpi.DailyFact{{StoreID: "store-1", StoreCode: "S1", StoreName: "One", Brand: "Brand", Region: "North", BusinessDate: date, Currency: "CNY", Revenue: &value, GrossProfit: &value, Transactions: &value, Footfall: &value, AreaSqm: &value, LaborCost: &value, FixedRent: &value, VariableRent: &value, NonLeaseCost: &value, OtherControllableCost: &value, MappingStatus: "mapped"}}
	h := NewRetailKPIHandler(fakeRetailKPIReader{result: &repository.RetailKPIFactSet{Facts: facts, ExpectedStoreCount: 1, SourceSystems: []string{"retail_simulator"}, DatasetVersions: []string{"dataset-1"}}})
	r := gin.New()
	r.GET("/definitions", h.Definitions)
	r.GET("/kpis", func(c *gin.Context) { c.Set("legal_entity_id", "entity-a"); h.StoreDays(c) })
	definitionResponse := httptest.NewRecorder()
	r.ServeHTTP(definitionResponse, httptest.NewRequest(http.MethodGet, "/definitions", nil))
	if definitionResponse.Code != http.StatusOK {
		t.Fatalf("definitions status=%d", definitionResponse.Code)
	}
	var definitions map[string]any
	if err := json.Unmarshal(definitionResponse.Body.Bytes(), &definitions); err != nil {
		t.Fatal(err)
	}
	if definitions["formula_version"] != retailkpi.FormulaVersion {
		t.Fatalf("definition version=%v", definitions["formula_version"])
	}
	bad := httptest.NewRecorder()
	r.ServeHTTP(bad, httptest.NewRequest(http.MethodGet, "/kpis?date_from=2026-01-01&date_to=2026-01-01", nil))
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("implicit classification status=%d", bad.Code)
	}
	productionWithDataset := httptest.NewRecorder()
	r.ServeHTTP(productionWithDataset, httptest.NewRequest(http.MethodGet, "/kpis?date_from=2026-01-01&date_to=2026-01-01&data_classification=production&dataset_version=dataset-1", nil))
	if productionWithDataset.Code != http.StatusBadRequest {
		t.Fatalf("production dataset status=%d", productionWithDataset.Code)
	}
	good := httptest.NewRecorder()
	r.ServeHTTP(good, httptest.NewRequest(http.MethodGet, "/kpis?date_from=2026-01-01&date_to=2026-01-01&data_classification=simulated&simulation_dataset_version=dataset-1&group_by=store", nil))
	if good.Code != http.StatusOK {
		t.Fatalf("valid KPI status=%d body=%s", good.Code, good.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(good.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"basis", "formula_version", "data_classification", "simulation_dataset_versions", "coverage", "data", "source", "definition_url"} {
		if _, ok := envelope[key]; !ok {
			t.Fatalf("response missing %s", key)
		}
	}
}

func TestRetailKPIHandlerSourceConflictIs409(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewRetailKPIHandler(fakeRetailKPIReader{err: repository.ErrRetailKPISourceConflict})
	r := gin.New()
	r.GET("/kpis", func(c *gin.Context) { c.Set("legal_entity_id", "entity-a"); h.StoreDays(c) })
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/kpis?date_from=2026-01-01&date_to=2026-01-01&data_classification=production", nil))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("source conflict response status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
