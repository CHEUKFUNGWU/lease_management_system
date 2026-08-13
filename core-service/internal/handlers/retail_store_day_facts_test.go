package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/repository"
)

type fakeRetailStoreDayFactRepo struct {
	items      map[string]*repository.RetailStoreDayFact
	listRows   []*repository.RetailStoreDayFact
	upsertErr  error
	listErr    error
	upsertCall int
	listTenant string
	pageTotal  int
	pageSize   int
	pageOffset int
	auditErr   error
	requests   map[string]struct {
		hash  string
		facts []*repository.RetailStoreDayFact
	}
}

func (f *fakeRetailStoreDayFactRepo) UpsertRetailStoreDayFactsAtomic(ctx context.Context, _ string, facts []*repository.RetailStoreDayFact, key, hash string, _ *string, auditFn repository.RetailStoreDayFactAuditFunc) (*repository.RetailStoreDayFactWriteResult, error) {
	if f.upsertErr != nil {
		return nil, f.upsertErr
	}
	if key != "" {
		if f.requests == nil {
			f.requests = make(map[string]struct {
				hash  string
				facts []*repository.RetailStoreDayFact
			})
		}
		if request, ok := f.requests[key]; ok {
			if request.hash != hash {
				return nil, repository.ErrRetailStoreDayFactIdempotencyConflict
			}
			return &repository.RetailStoreDayFactWriteResult{Facts: request.facts, Replayed: true}, nil
		}
	}
	if f.items == nil {
		f.items = make(map[string]*repository.RetailStoreDayFact)
	}
	original := f.items
	working := make(map[string]*repository.RetailStoreDayFact, len(original))
	for key, value := range original {
		copy := *value
		working[key] = &copy
	}
	f.items = working
	saved := make([]*repository.RetailStoreDayFact, 0, len(facts))
	for _, fact := range facts {
		f.upsertCall++
		key := strings.Join([]string{fact.StoreID, fact.BusinessDate, fact.SourceSystem, strconv.Itoa(fact.Version)}, "|")
		var old *repository.RetailStoreDayFact
		if existing, ok := f.items[key]; ok {
			copy := *existing
			old = &copy
		}
		copy := *fact
		if copy.ID == "" {
			copy.ID = uuid.NewString()
		}
		if copy.AsOfAt.IsZero() {
			copy.AsOfAt = time.Now().UTC()
		}
		f.items[key] = &copy
		if auditFn != nil {
			if err := auditFn(ctx, nil, old, &copy); err != nil {
				f.items = original
				return nil, err
			}
		}
		saved = append(saved, &copy)
	}
	if f.auditErr != nil {
		f.items = original
		return nil, f.auditErr
	}
	if key != "" {
		f.requests[key] = struct {
			hash  string
			facts []*repository.RetailStoreDayFact
		}{hash: hash, facts: saved}
	}
	return &repository.RetailStoreDayFactWriteResult{Facts: saved}, nil
}

func (f *fakeRetailStoreDayFactRepo) ListRetailStoreDayFactsPage(_ context.Context, legalEntityID, _, _ string, _ []string, pageSize, offset int) (*repository.RetailStoreDayFactsPage, error) {
	f.listTenant = legalEntityID
	f.pageSize, f.pageOffset = pageSize, offset
	if f.listErr != nil {
		return nil, f.listErr
	}
	total := f.pageTotal
	if total == 0 {
		total = len(f.listRows)
	}
	start, end := offset, offset+pageSize
	if start > len(f.listRows) {
		start = len(f.listRows)
	}
	if pageSize <= 0 || end > len(f.listRows) {
		end = len(f.listRows)
	}
	return &repository.RetailStoreDayFactsPage{Data: f.listRows[start:end], Total: total, PageSize: pageSize, Offset: offset, Returned: end - start}, nil
}

type fakeRetailStoreDayFactAuditor struct {
	calls     int
	tableName string
}

func (f *fakeRetailStoreDayFactAuditor) LogInTx(_ context.Context, _ repository.DBTX, tableName, _ string, _ string, _ interface{}, _ interface{}, _ string, _ *gin.Context) error {
	f.calls++
	f.tableName = tableName
	return nil
}

func validRetailStoreDayInput() retailStoreDayFactInput {
	revenue := 100.0
	return retailStoreDayFactInput{
		StoreID:            "c3d4e5f6-a7b8-9012-cdef-123456789012",
		BusinessDate:       "2026-08-01",
		Currency:           "CNY",
		Revenue:            &revenue,
		SourceSystem:       "pos",
		DataClassification: "production",
	}
}

func retailStoreDayTestContext(method, target string, payload any) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	var body io.Reader
	if payload != nil {
		encoded, _ := json.Marshal(payload)
		body = bytes.NewReader(encoded)
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, target, body)
	request.Header.Set("Content-Type", "application/json")
	c, _ := gin.CreateTestContext(recorder)
	c.Request = request
	c.Set("legal_entity_id", "legal-entity-a")
	c.Set("user_id", "user-a")
	return c, recorder
}

func TestValidateRetailStoreDayInputRequiresConsistentSimulationSource(t *testing.T) {
	simulatedWithoutVersion := validRetailStoreDayInput()
	simulatedWithoutVersion.DataClassification = "simulated"
	if _, err := validateRetailStoreDayInput(simulatedWithoutVersion); !strings.Contains(err, "simulation_dataset_version") {
		t.Fatalf("simulated fact without dataset version error = %q", err)
	}

	datasetVersion := "retail-seed-v1"
	productionWithVersion := validRetailStoreDayInput()
	productionWithVersion.SimulationDatasetVersion = &datasetVersion
	if _, err := validateRetailStoreDayInput(productionWithVersion); !strings.Contains(err, "must be absent") {
		t.Fatalf("production fact with dataset version error = %q", err)
	}
	emptyDatasetVersion := ""
	productionWithEmptyVersion := validRetailStoreDayInput()
	productionWithEmptyVersion.SimulationDatasetVersion = &emptyDatasetVersion
	if fact, err := validateRetailStoreDayInput(productionWithEmptyVersion); err != "" || fact.SimulationDatasetVersion != nil {
		t.Fatalf("production fact with empty dataset version rejected or retained: fact=%+v error=%q", fact, err)
	}

	validSimulated := validRetailStoreDayInput()
	validSimulated.DataClassification = "simulated"
	validSimulated.SimulationDatasetVersion = &datasetVersion
	if fact, err := validateRetailStoreDayInput(validSimulated); err != "" || fact.SimulationDatasetVersion == nil {
		t.Fatalf("valid simulated fact rejected: fact=%+v error=%q", fact, err)
	}
}

func TestValidateRetailStoreDayInputRejectsInvalidDatesAndNegativeMeasures(t *testing.T) {
	invalidDate := validRetailStoreDayInput()
	invalidDate.BusinessDate = "2026-02-30"
	if _, err := validateRetailStoreDayInput(invalidDate); !strings.Contains(err, "business_date") {
		t.Fatalf("invalid date error = %q", err)
	}

	negative := validRetailStoreDayInput()
	value := -1.0
	negative.Transactions = &value
	if _, err := validateRetailStoreDayInput(negative); !strings.Contains(err, "transactions") {
		t.Fatalf("negative measure error = %q", err)
	}
}

func TestRetailStoreDayFactsHandlerRejectsOversizedBatch(t *testing.T) {
	items := make([]retailStoreDayFactInput, maxRetailStoreDayBatch+1)
	c, recorder := retailStoreDayTestContext(http.MethodPost, "/api/v1/retail/operating-facts/store-days", retailStoreDayFactRequest{Items: items})
	h := NewRetailStoreDayFactsHandler(&fakeRetailStoreDayFactRepo{}, &fakeRetailStoreDayFactAuditor{})
	h.Upsert(c)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusRequestEntityTooLarge, recorder.Body.String())
	}
}

func TestRetailStoreDayFactsHandlerUpsertUsesBusinessIdempotencyAndAuditsEveryWrite(t *testing.T) {
	input := validRetailStoreDayInput()
	input.DataClassification = "simulated"
	datasetVersion := "retail-seed-v1"
	input.SimulationDatasetVersion = &datasetVersion
	repo := &fakeRetailStoreDayFactRepo{}
	auditor := &fakeRetailStoreDayFactAuditor{}
	h := NewRetailStoreDayFactsHandler(repo, auditor)
	c, recorder := retailStoreDayTestContext(http.MethodPost, "/api/v1/retail/operating-facts/store-days", retailStoreDayFactRequest{Items: []retailStoreDayFactInput{input, input}})
	h.Upsert(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	if len(repo.items) != 1 {
		t.Fatalf("stored fact count = %d, want 1", len(repo.items))
	}
	if repo.upsertCall != 2 || auditor.calls != 2 {
		t.Fatalf("upsert calls/audit calls = %d/%d, want 2/2", repo.upsertCall, auditor.calls)
	}
	if auditor.tableName != "retail_store_day_facts" {
		t.Fatalf("audit table = %q", auditor.tableName)
	}
	var envelope map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["data_classification"] != "simulated" {
		t.Fatalf("classification = %v, want simulated", envelope["data_classification"])
	}
}

func TestRetailStoreDayFactsHandlerListPassesTenantAndReturnsMixedEnvelope(t *testing.T) {
	firstAsOf := time.Date(2026, 8, 1, 3, 0, 0, 0, time.UTC)
	secondAsOf := firstAsOf.Add(time.Hour)
	version := "retail-seed-v1"
	repo := &fakeRetailStoreDayFactRepo{listRows: []*repository.RetailStoreDayFact{
		{ID: uuid.NewString(), StoreID: "store-a", BusinessDate: "2026-08-01", DataClassification: "production", AsOfAt: firstAsOf},
		{ID: uuid.NewString(), StoreID: "store-b", BusinessDate: "2026-08-01", DataClassification: "simulated", SimulationDatasetVersion: &version, AsOfAt: secondAsOf},
	}}
	h := NewRetailStoreDayFactsHandler(repo, nil)
	c, recorder := retailStoreDayTestContext(http.MethodGet, "/api/v1/retail/operating-facts/store-days?date_from=2026-08-01&date_to=2026-08-31", nil)
	h.List(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	if repo.listTenant != "legal-entity-a" {
		t.Fatalf("tenant passed to repository = %q", repo.listTenant)
	}
	var envelope map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["data_classification"] != "mixed" {
		t.Fatalf("classification = %v, want mixed", envelope["data_classification"])
	}
	if envelope["basis"] != "Working" {
		t.Fatalf("basis = %v, want Working", envelope["basis"])
	}
	if envelope["as_of"] != secondAsOf.Format(time.RFC3339Nano) {
		t.Fatalf("as_of = %v, want %s", envelope["as_of"], secondAsOf.Format(time.RFC3339Nano))
	}
}

func TestRetailStoreDayFactsHandlerProductionOnlyEnvelope(t *testing.T) {
	repo := &fakeRetailStoreDayFactRepo{listRows: []*repository.RetailStoreDayFact{{
		ID: uuid.NewString(), StoreID: "store-a", BusinessDate: "2026-08-01", DataClassification: "production", AsOfAt: time.Now().UTC(),
	}}}
	h := NewRetailStoreDayFactsHandler(repo, nil)
	c, recorder := retailStoreDayTestContext(http.MethodGet, "/api/v1/retail/operating-facts/store-days?date_from=2026-08-01&date_to=2026-08-01", nil)
	h.List(c)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["data_classification"] != "production" {
		t.Fatalf("classification = %v, want production", envelope["data_classification"])
	}
	versions, ok := envelope["simulation_dataset_versions"].([]any)
	if !ok || len(versions) != 0 {
		t.Fatalf("production simulation versions = %v, want empty", envelope["simulation_dataset_versions"])
	}
}

func TestRetailStoreDayFactsHandlerRejectsInvalidAndOverlongRanges(t *testing.T) {
	h := NewRetailStoreDayFactsHandler(&fakeRetailStoreDayFactRepo{}, nil)
	tests := []struct {
		name   string
		target string
	}{
		{name: "reverse", target: "/api/v1/retail/operating-facts/store-days?date_from=2026-08-02&date_to=2026-08-01"},
		{name: "overlong", target: "/api/v1/retail/operating-facts/store-days?date_from=2026-01-01&date_to=2027-01-02"},
		{name: "invalid", target: "/api/v1/retail/operating-facts/store-days?date_from=2026-02-30&date_to=2026-03-01"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, recorder := retailStoreDayTestContext(http.MethodGet, tt.target, nil)
			h.List(c)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestRetailStoreDayFactsHandlerIdempotencyReplayAndConflict(t *testing.T) {
	input := validRetailStoreDayInput()
	repo := &fakeRetailStoreDayFactRepo{}
	auditor := &fakeRetailStoreDayFactAuditor{}
	h := NewRetailStoreDayFactsHandler(repo, auditor)
	firstContext, firstRecorder := retailStoreDayTestContext(http.MethodPost, "/api/v1/retail/operating-facts/store-days", retailStoreDayFactRequest{Items: []retailStoreDayFactInput{input}})
	firstContext.Request.Header.Set("Idempotency-Key", "handler-idem")
	h.Upsert(firstContext)
	if firstRecorder.Code != http.StatusOK || auditor.calls != 1 || repo.upsertCall != 1 {
		t.Fatalf("first idempotent request status=%d calls=%d/%d body=%s", firstRecorder.Code, repo.upsertCall, auditor.calls, firstRecorder.Body.String())
	}
	replayContext, replayRecorder := retailStoreDayTestContext(http.MethodPost, "/api/v1/retail/operating-facts/store-days", retailStoreDayFactRequest{Items: []retailStoreDayFactInput{input}})
	replayContext.Request.Header.Set("Idempotency-Key", "handler-idem")
	h.Upsert(replayContext)
	if replayRecorder.Code != http.StatusOK || auditor.calls != 1 || repo.upsertCall != 1 {
		t.Fatalf("replay status=%d calls=%d/%d body=%s", replayRecorder.Code, repo.upsertCall, auditor.calls, replayRecorder.Body.String())
	}
	var replayEnvelope map[string]any
	if err := json.Unmarshal(replayRecorder.Body.Bytes(), &replayEnvelope); err != nil {
		t.Fatal(err)
	}
	if replayEnvelope["idempotent_replay"] != true {
		t.Fatalf("replay envelope = %v", replayEnvelope["idempotent_replay"])
	}
	conflict := input
	changedRevenue := 101.0
	conflict.Revenue = &changedRevenue
	conflictContext, conflictRecorder := retailStoreDayTestContext(http.MethodPost, "/api/v1/retail/operating-facts/store-days", retailStoreDayFactRequest{Items: []retailStoreDayFactInput{conflict}})
	conflictContext.Request.Header.Set("Idempotency-Key", "handler-idem")
	h.Upsert(conflictContext)
	if conflictRecorder.Code != http.StatusConflict {
		t.Fatalf("conflict status=%d, want 409; body=%s", conflictRecorder.Code, conflictRecorder.Body.String())
	}
}

func TestRetailStoreDayFactsHandlerPaginationExposesReliableTotalAndTruncation(t *testing.T) {
	rows := []*repository.RetailStoreDayFact{
		{ID: uuid.NewString(), StoreID: "store-a", BusinessDate: "2026-08-01", DataClassification: "production", AsOfAt: time.Now().UTC()},
		{ID: uuid.NewString(), StoreID: "store-b", BusinessDate: "2026-08-01", DataClassification: "production", AsOfAt: time.Now().UTC()},
	}
	repo := &fakeRetailStoreDayFactRepo{listRows: rows, pageTotal: 50001}
	h := NewRetailStoreDayFactsHandler(repo, nil)
	c, recorder := retailStoreDayTestContext(http.MethodGet, "/api/v1/retail/operating-facts/store-days?date_from=2026-08-01&date_to=2026-08-01&page=1&page_size=2", nil)
	h.List(c)
	if recorder.Code != http.StatusOK || repo.pageSize != 2 || repo.pageOffset != 0 {
		t.Fatalf("status=%d page=%d/%d body=%s", recorder.Code, repo.pageSize, repo.pageOffset, recorder.Body.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["total"] != float64(50001) || envelope["returned_count"] != float64(2) {
		t.Fatalf("pagination envelope total/returned = %v/%v", envelope["total"], envelope["returned_count"])
	}
	pagination, ok := envelope["pagination"].(map[string]any)
	if !ok || pagination["truncated"] != true || pagination["has_more"] != true {
		t.Fatalf("pagination envelope = %v", envelope["pagination"])
	}
}
