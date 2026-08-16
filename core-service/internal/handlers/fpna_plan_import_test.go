package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

type fakePlanImportStore struct {
	versions []*repository.FPnAPlanVersion
	lines    []*repository.FPnAPlanLine
	deleted  bool
}

func (f *fakePlanImportStore) CreatePlanVersion(_ context.Context, item *repository.FPnAPlanVersion) (*repository.FPnAPlanVersion, error) {
	if item.ID == "" {
		item.ID = "plan-00000000-0000-0000-0000-000000000001"
	}
	f.versions = append(f.versions, item)
	return item, nil
}

func (f *fakePlanImportStore) ListPlanVersions(context.Context, access.EntityFilter, string) ([]*repository.FPnAPlanVersion, error) {
	return f.versions, nil
}

func (f *fakePlanImportStore) CreatePlanLine(_ context.Context, item *repository.FPnAPlanLine) (*repository.FPnAPlanLine, error) {
	f.lines = append(f.lines, item)
	return item, nil
}

func (f *fakePlanImportStore) DeletePlanVersion(context.Context, string, access.EntityFilter) error {
	f.deleted = true
	return nil
}

func newPlanImportMultipart(t *testing.T, csvContent string, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile("file", "budget.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(csvContent)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body, writer.FormDataContentType()
}

func TestFPnAPlanImportPartialSuccessAndReplay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	population := &fakeIngestPopulation{stores: []retailkpi.StorePopulation{
		{StoreID: "11111111-1111-1111-1111-111111111111", StoreCode: "S001", StoreName: "一号店"},
	}}
	store := &fakePlanImportStore{}
	handler := NewFPnAPlanImportHandler(population, store)
	csvContent := "store_code,period,currency,revenue,gross_profit\nS001,2026-07,CNY,1000,400\nS001,2026-08,CNY,1100,440\nGHOST,2026-07,CNY,500,200\n"
	body, contentType := newPlanImportMultipart(t, csvContent, map[string]string{
		"name": "FY2026 Budget v1", "version_type": "budget", "source": "excel-import",
		"as_of_period": "2025-12", "from_period": "2026-01", "to_period": "2026-12", "is_official": "true",
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/fpna/plan-versions/import", body)
	request.Header.Set("Content-Type", contentType)
	c, _ := gin.CreateTestContext(recorder)
	c.Request = request
	c.Set("legal_entity_id", "legal-entity-a")
	c.Set("access_scope", access.Scope{LegalEntityID: "legal-entity-a"})
	c.Set("user_id", "user-a")
	handler.Import(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Version   struct {
			Name        string `json:"name"`
			VersionType string `json:"version_type"`
			IsOfficial  bool   `json:"is_official"`
		} `json:"version"`
		Accepted  int    `json:"accepted_rows"`
		Rejected  int    `json:"rejected_rows"`
		Replay    bool   `json:"idempotent_replay"`
		Errors    []planImportRowError `json:"errors"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Accepted != 2 || response.Rejected != 1 || response.Replay {
		t.Fatalf("response=%+v", response)
	}
	if response.Version.Name != "FY2026 Budget v1" || response.Version.VersionType != "budget" || !response.Version.IsOfficial {
		t.Fatalf("version=%+v", response.Version)
	}
	if len(store.lines) != 2 || store.lines[0].Grain != "store" || store.lines[0].StoreID == nil || *store.lines[0].StoreID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("lines=%+v", store.lines)
	}
	if len(response.Errors) != 1 || response.Errors[0].Code != "unmatched_store" {
		t.Fatalf("errors=%+v", response.Errors)
	}
	// Replay of the same (name, as_of_period) returns the frozen version —
	// a fresh multipart body, the first one was consumed.
	replayBody, replayContentType := newPlanImportMultipart(t, csvContent, map[string]string{
		"name": "FY2026 Budget v1", "version_type": "budget", "source": "excel-import",
		"as_of_period": "2025-12", "from_period": "2026-01", "to_period": "2026-12", "is_official": "true",
	})
	recorder = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/fpna/plan-versions/import", replayBody)
	c.Request.Header.Set("Content-Type", replayContentType)
	c.Set("legal_entity_id", "legal-entity-a")
	c.Set("access_scope", access.Scope{LegalEntityID: "legal-entity-a"})
	c.Set("user_id", "user-a")
	handler.Import(c)
	var replay struct{ IdempotentReplay bool `json:"idempotent_replay"` }
	if err := json.Unmarshal(recorder.Body.Bytes(), &replay); err != nil {
		t.Fatal(err)
	}
	if !replay.IdempotentReplay || len(store.versions) != 1 || len(store.lines) != 2 {
		t.Fatalf("replay=%+v versions=%d lines=%d", replay, len(store.versions), len(store.lines))
	}
}
