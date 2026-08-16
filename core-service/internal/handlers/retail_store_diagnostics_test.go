package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

type diagnosticsReader struct {
	set *repository.RetailKPIFactSet
	err error
}

func (r diagnosticsReader) QueryFacts(context.Context, string, string, string, string, string, string, []string) (*repository.RetailKPIFactSet, error) {
	return r.set, r.err
}
func (r diagnosticsReader) ListStorePopulation(context.Context, string, string, string, []string) ([]retailkpi.StorePopulation, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.set.ExpectedStores, nil
}

func TestRetailStoreDiagnosticsHandlerValidationAndOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := retailkpi.StorePopulation{StoreID: "00000000-0000-0000-0000-000000000001", StoreCode: "S001", StoreName: "门店1", Brand: "品牌", Region: "区域"}
	h := NewRetailStoreDiagnosticsHandler(diagnosticsReader{set: &repository.RetailKPIFactSet{ExpectedStores: []retailkpi.StorePopulation{store}}})
	r := gin.New()
	r.GET("/options", func(c *gin.Context) { c.Set("legal_entity_id", "entity-a"); h.StoreOptions(c) })
	r.GET("/diagnostics/:store_id", func(c *gin.Context) { c.Set("legal_entity_id", "entity-a"); h.Diagnostics(c) })
	options := httptest.NewRecorder()
	r.ServeHTTP(options, httptest.NewRequest(http.MethodGet, "/options?data_classification=simulated&dataset_version=planA-v1", nil))
	if options.Code != http.StatusOK || len(options.Body.Bytes()) == 0 {
		t.Fatalf("options status=%d body=%s", options.Code, options.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(options.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	items, ok := envelope["data"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("options envelope=%v", envelope)
	}
	item := items[0].(map[string]any)
	for _, key := range []string{"store_id", "store_code", "store_name", "brand", "region"} {
		if _, exists := item[key]; !exists {
			t.Fatalf("lowercase option missing %s: %v", key, item)
		}
	}
	productionDataset := httptest.NewRecorder()
	r.ServeHTTP(productionDataset, httptest.NewRequest(http.MethodGet, "/options?data_classification=production&dataset_version=bad", nil))
	if productionDataset.Code != http.StatusBadRequest {
		t.Fatalf("production dataset status=%d", productionDataset.Code)
	}
	simulatedDataset := httptest.NewRecorder()
	r.ServeHTTP(simulatedDataset, httptest.NewRequest(http.MethodGet, "/options?data_classification=simulated", nil))
	if simulatedDataset.Code != http.StatusBadRequest {
		t.Fatalf("simulated dataset status=%d", simulatedDataset.Code)
	}
	badWindow := httptest.NewRecorder()
	r.ServeHTTP(badWindow, httptest.NewRequest(http.MethodGet, "/diagnostics/00000000-0000-0000-0000-000000000001?data_classification=simulated&dataset_version=planA-v1&as_of=2026-06-14&window_days=5", nil))
	// M2 range contract: 8 is a legal custom window now (pulse always
	// accepted it); 5 is genuinely out of range.
	if badWindow.Code != http.StatusBadRequest {
		t.Fatalf("bad window status=%d", badWindow.Code)
	}
	missing := httptest.NewRecorder()
	r.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/diagnostics/00000000-0000-0000-0000-000000000002?data_classification=simulated&dataset_version=planA-v1&as_of=2026-06-14&window_days=7", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing store status=%d", missing.Code)
	}
}

func TestRetailStoreDiagnosticsHandlerSourceConflictAndEmptyTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := retailkpi.StorePopulation{StoreID: "00000000-0000-0000-0000-000000000001"}
	r := gin.New()
	r.GET("/diagnostics/:store_id", func(c *gin.Context) {
		c.Set("legal_entity_id", "entity-a")
		NewRetailStoreDiagnosticsHandler(diagnosticsReader{set: &repository.RetailKPIFactSet{ExpectedStores: []retailkpi.StorePopulation{store}}, err: repository.ErrRetailKPISourceConflict}).Diagnostics(c)
	})
	r.GET("/empty/:store_id", func(c *gin.Context) {
		NewRetailStoreDiagnosticsHandler(diagnosticsReader{set: &repository.RetailKPIFactSet{ExpectedStores: []retailkpi.StorePopulation{store}}}).Diagnostics(c)
	})
	conflict := httptest.NewRecorder()
	r.ServeHTTP(conflict, httptest.NewRequest(http.MethodGet, "/diagnostics/00000000-0000-0000-0000-000000000001?data_classification=production&as_of=2026-06-14&window_days=7", nil))
	if conflict.Code != http.StatusConflict {
		t.Fatalf("source conflict status=%d", conflict.Code)
	}
	empty := httptest.NewRecorder()
	r.ServeHTTP(empty, httptest.NewRequest(http.MethodGet, "/empty/00000000-0000-0000-0000-000000000001?data_classification=production&as_of=2026-06-14&window_days=7", nil))
	if empty.Code != http.StatusBadRequest {
		t.Fatalf("empty tenant status=%d", empty.Code)
	}
}

func TestRetailStoreDiagnosticsHandlerEmptyOptionsIsStableEmptyArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/options", func(c *gin.Context) {
		c.Set("legal_entity_id", "entity-a")
		NewRetailStoreDiagnosticsHandler(diagnosticsReader{set: &repository.RetailKPIFactSet{ExpectedStores: nil}}).StoreOptions(c)
	})
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/options?data_classification=simulated&dataset_version=planA-v1", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("empty options status=%d", recorder.Code)
	}
	var envelope map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if values, ok := envelope["data"].([]any); !ok || values == nil || len(values) != 0 {
		t.Fatalf("empty options envelope=%v", envelope)
	}
}

func TestRetailStoreDiagnosticsHandlerErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := retailkpi.StorePopulation{StoreID: "00000000-0000-0000-0000-000000000001"}
	r := gin.New()
	r.GET("/diagnostics/:store_id", func(c *gin.Context) {
		c.Set("legal_entity_id", "entity-a")
		NewRetailStoreDiagnosticsHandler(diagnosticsReader{set: &repository.RetailKPIFactSet{ExpectedStores: []retailkpi.StorePopulation{store}}, err: errors.New("db")}).Diagnostics(c)
	})
	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/diagnostics/00000000-0000-0000-0000-000000000001?data_classification=production&as_of=2026-06-14&window_days=7", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("repository status=%d", recorder.Code)
	}
}
