package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

type pulseHandlerReader struct {
	set *repository.RetailKPIFactSet
	err error
}

func (r pulseHandlerReader) QueryFacts(context.Context, string, string, string, string, string, string, []string) (*repository.RetailKPIFactSet, error) {
	return r.set, r.err
}

func TestRetailPulseHandlerValidationAndEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewRetailPulseHandler(pulseHandlerReader{set: &repository.RetailKPIFactSet{Facts: []retailkpi.DailyFact{}, ExpectedStoreCount: 0}})
	router := gin.New()
	router.GET("/pulse", func(c *gin.Context) { c.Set("legal_entity_id", "entity-a"); handler.OperatingPulse(c) })
	router.GET("/pulse-empty-tenant", handler.OperatingPulse)
	emptyTenant := httptest.NewRecorder()
	router.ServeHTTP(emptyTenant, httptest.NewRequest(http.MethodGet, "/pulse-empty-tenant?as_of=2026-01-31&data_classification=production", nil))
	if emptyTenant.Code != http.StatusBadRequest {
		t.Fatalf("empty legal entity status=%d body=%s", emptyTenant.Code, emptyTenant.Body.String())
	}
	invalid := httptest.NewRecorder()
	router.ServeHTTP(invalid, httptest.NewRequest(http.MethodGet, "/pulse?as_of=2026-01-31&window_days=6&data_classification=production", nil))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("window validation status=%d", invalid.Code)
	}
	partialInteger := httptest.NewRecorder()
	router.ServeHTTP(partialInteger, httptest.NewRequest(http.MethodGet, "/pulse?as_of=2026-01-31&window_days=7x&data_classification=production", nil))
	if partialInteger.Code != http.StatusBadRequest {
		t.Fatalf("partial integer status=%d", partialInteger.Code)
	}
	attentionTooLarge := httptest.NewRecorder()
	router.ServeHTTP(attentionTooLarge, httptest.NewRequest(http.MethodGet, "/pulse?as_of=2026-01-31&attention_limit=51&data_classification=production", nil))
	if attentionTooLarge.Code != http.StatusBadRequest {
		t.Fatalf("attention limit status=%d", attentionTooLarge.Code)
	}
	productionWithDataset := httptest.NewRecorder()
	router.ServeHTTP(productionWithDataset, httptest.NewRequest(http.MethodGet, "/pulse?as_of=2026-01-31&data_classification=production&dataset_version=dataset-1", nil))
	if productionWithDataset.Code != http.StatusBadRequest {
		t.Fatalf("production dataset status=%d", productionWithDataset.Code)
	}
	simulatedWithoutDataset := httptest.NewRecorder()
	router.ServeHTTP(simulatedWithoutDataset, httptest.NewRequest(http.MethodGet, "/pulse?as_of=2026-01-31&data_classification=simulated", nil))
	if simulatedWithoutDataset.Code != http.StatusBadRequest {
		t.Fatalf("simulated missing dataset status=%d", simulatedWithoutDataset.Code)
	}
	missingClassification := httptest.NewRecorder()
	router.ServeHTTP(missingClassification, httptest.NewRequest(http.MethodGet, "/pulse?as_of=2026-01-31", nil))
	if missingClassification.Code != http.StatusBadRequest {
		t.Fatalf("classification validation status=%d", missingClassification.Code)
	}
	valid := httptest.NewRecorder()
	router.ServeHTTP(valid, httptest.NewRequest(http.MethodGet, "/pulse?as_of=2026-01-31&window_days=8&data_classification=production", nil))
	if valid.Code != http.StatusOK {
		t.Fatalf("valid status=%d body=%s", valid.Code, valid.Body.String())
	}
}

func TestRetailPulseHandlerSourceConflictIs409(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewRetailPulseHandler(pulseHandlerReader{err: repository.ErrRetailKPISourceConflict})
	router := gin.New()
	router.GET("/pulse", func(c *gin.Context) { c.Set("legal_entity_id", "entity-a"); handler.OperatingPulse(c) })
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/pulse?as_of=2026-01-31&data_classification=production", nil))
	if recorder.Code != http.StatusConflict {
		t.Fatalf("source conflict status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestRetailPulseHandlerRepositoryFailureIs500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewRetailPulseHandler(pulseHandlerReader{err: errors.New("database unavailable")})
	router := gin.New()
	router.GET("/pulse", func(c *gin.Context) { c.Set("legal_entity_id", "entity-a"); handler.OperatingPulse(c) })
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/pulse?as_of=2026-01-31&data_classification=production", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("repository failure status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
