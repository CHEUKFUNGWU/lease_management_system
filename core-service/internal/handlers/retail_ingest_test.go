package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"mime/multipart"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

type fakeIngestPopulation struct {
	stores []retailkpi.StorePopulation
	err    error
}

func (f *fakeIngestPopulation) ListStorePopulation(context.Context, string, string, string, []string) ([]retailkpi.StorePopulation, error) {
	return f.stores, f.err
}

type fakeIngestStore struct {
	fakeRetailStoreDayFactRepo
	state    map[string]int
	batches  []*repository.OperatingFactBatch
	finals   []finalCall
	finalErr error
}

type finalCall struct {
	id       string
	accepted int
	rejected int
	status   string
}

func (f *fakeIngestStore) RetailStoreDayExistingState(context.Context, []string, []string, string) (map[string]int, error) {
	return f.state, nil
}

func (f *fakeIngestStore) CreateBatch(_ context.Context, batch *repository.OperatingFactBatch) (*repository.OperatingFactBatch, error) {
	if batch.ID == "" {
		batch.ID = "batch-00000000-0000-0000-0000-000000000001"
	}
	batch.Status = "received"
	f.batches = append(f.batches, batch)
	return batch, nil
}

func (f *fakeIngestStore) FinalizeBatch(_ context.Context, id string, accepted, rejected int, status, _ string, _ json.RawMessage) (*repository.OperatingFactBatch, error) {
	if f.finalErr != nil {
		return nil, f.finalErr
	}
	f.finals = append(f.finals, finalCall{id: id, accepted: accepted, rejected: rejected, status: status})
	return &repository.OperatingFactBatch{ID: id, Status: status, AcceptedRows: accepted, RejectedRows: rejected}, nil
}

func newIngestTestContext(t *testing.T, body *bytes.Buffer, contentType string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/retail/operating-facts/store-days/import", body)
	request.Header.Set("Content-Type", contentType)
	c, _ := gin.CreateTestContext(recorder)
	c.Request = request
	c.Set("legal_entity_id", "legal-entity-a")
	c.Set("access_scope", access.Scope{LegalEntityID: "legal-entity-a"})
	c.Set("user_id", "user-a")
	return c, recorder
}

func newImportMultipart(t *testing.T, csvContent string, fields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile("file", "pos-export.csv")
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

func newIngestHandler() *RetailIngestHandler {
	population := &fakeIngestPopulation{stores: []retailkpi.StorePopulation{
		{StoreID: "11111111-1111-1111-1111-111111111111", StoreCode: "S001", StoreName: "一号店"},
	}}
	return NewRetailIngestHandler(population, &fakeIngestStore{}, nil)
}

func TestRetailIngestPreviewContract(t *testing.T) {
	handler := newIngestHandler()
	csvContent := "门店编号,日期,币种,营业额\nS001,2026-07-01,CNY,100\nS001,2026-07-02,CNY,101\n"
	body, contentType := newImportMultipart(t, csvContent, map[string]string{"source_system": "pos"})
	c, recorder := newIngestTestContext(t, body, contentType)
	handler.Preview(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["basis"] != "Working" || response["format"] != "csv" {
		t.Fatalf("basis/format=%v/%v", response["basis"], response["format"])
	}
	mapping := response["suggested_mapping"].(map[string]any)
	if mapping["门店编号"] != "store" || mapping["营业额"] != "revenue" {
		t.Fatalf("suggested mapping=%v", mapping)
	}
	report := response["report"].(map[string]any)
	if report["valid_rows"] != float64(2) {
		t.Fatalf("valid rows=%v report=%v", report["valid_rows"], report)
	}
	resolution := response["resolution"].(map[string]any)
	if resolution["matched_count"] != float64(1) {
		t.Fatalf("resolution=%v", resolution)
	}
}

func TestRetailIngestPreviewRequiresSourceSystemAndEntity(t *testing.T) {
	handler := newIngestHandler()
	body, contentType := newImportMultipart(t, "门店编号,日期,币种,营业额\n", map[string]string{})
	c, recorder := newIngestTestContext(t, body, contentType)
	handler.Preview(c)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("missing source_system status=%d", recorder.Code)
	}

	body, contentType = newImportMultipart(t, "门店编号,日期,币种,营业额\nS001,2026-07-01,CNY,1\n", map[string]string{"source_system": "pos"})
	c, recorder = newIngestTestContext(t, body, contentType)
	c.Set("legal_entity_id", "")
	handler.Preview(c)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("missing entity status=%d", recorder.Code)
	}
}

func TestRetailIngestCommitImportsProductionFacts(t *testing.T) {
	population := &fakeIngestPopulation{stores: []retailkpi.StorePopulation{
		{StoreID: "11111111-1111-1111-1111-111111111111", StoreCode: "S001", StoreName: "一号店"},
	}}
	store := &fakeIngestStore{}
	handler := NewRetailIngestHandler(population, store, nil)
	csvContent := "门店编号,日期,币种,营业额\nS001,2026-07-01,CNY,100\nS001,2026-07-02,CNY,101\n"
	body, contentType := newImportMultipart(t, csvContent, map[string]string{"source_system": "pos", "as_of_at": "2026-08-16"})
	c, recorder := newIngestTestContext(t, body, contentType)
	c.Request.Header.Set("Idempotency-Key", "ingest-key-1")
	handler.Commit(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response["saved_count"] != float64(2) || response["idempotent_replay"] != false {
		t.Fatalf("response=%v", response)
	}
	if len(store.items) != 2 {
		t.Fatalf("stored facts=%d", len(store.items))
	}
	for _, fact := range store.items {
		if fact.DataClassification != "production" || fact.SourceSystem != "pos" || fact.Version != 1 {
			t.Fatalf("fact envelope=%+v", fact)
		}
		if fact.ImportBatchID == nil || *fact.ImportBatchID == "" {
			t.Fatalf("fact without batch: %+v", fact)
		}
	}
	if len(store.batches) != 1 || store.batches[0].SourceSystem != "pos" || store.batches[0].IdempotencyKey != "ingest-key-1" {
		t.Fatalf("batch=%+v", store.batches[0])
	}
	if len(store.finals) != 1 || store.finals[0].status != "completed" || store.finals[0].accepted != 2 {
		t.Fatalf("finals=%+v", store.finals)
	}
}

func TestRetailIngestCommitRequiresIdempotencyKey(t *testing.T) {
	handler := newIngestHandler()
	body, contentType := newImportMultipart(t, "门店编号,日期,币种,营业额\nS001,2026-07-01,CNY,100\n", map[string]string{"source_system": "pos", "as_of_at": "2026-08-16"})
	c, recorder := newIngestTestContext(t, body, contentType)
	handler.Commit(c)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestRetailIngestCommitEveryRowFailedFinalizesFailedBatch(t *testing.T) {
	population := &fakeIngestPopulation{}
	store := &fakeIngestStore{}
	handler := NewRetailIngestHandler(population, store, nil)
	body, contentType := newImportMultipart(t, "门店编号,日期,币种,营业额\nGHOST,2026-07-01,CNY,100\n", map[string]string{"source_system": "pos", "as_of_at": "2026-08-16"})
	c, recorder := newIngestTestContext(t, body, contentType)
	c.Request.Header.Set("Idempotency-Key", "ingest-key-2")
	handler.Commit(c)
	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	if len(store.finals) != 1 || store.finals[0].status != "failed" || store.finals[0].rejected != 1 {
		t.Fatalf("finals=%+v", store.finals)
	}
	if len(store.items) != 0 {
		t.Fatalf("no fact should be written, got %d", len(store.items))
	}
}
