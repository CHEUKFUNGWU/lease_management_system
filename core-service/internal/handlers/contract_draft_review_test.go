package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/draftreview"
)

// draftReviewStubReader hands back canned rows; error shaping lives in the
// service, so this layer only pins the HTTP envelope and status mapping.
type draftReviewStubReader struct {
	rows      []repository.DraftReviewRow
	getErr    error
	listErr   error
	updateErr error
}

func (s *draftReviewStubReader) ListDraftsForReview(_ context.Context, _ access.EntityFilter, _ string, _ int) ([]repository.DraftReviewRow, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	out := make([]repository.DraftReviewRow, 0)
	for _, row := range s.rows {
		row := row
		out = append(out, row)
	}
	return out, nil
}

func (s *draftReviewStubReader) GetDraftForReview(_ context.Context, entity access.EntityFilter, id string) (repository.DraftReviewRow, error) {
	if s.getErr != nil {
		return repository.DraftReviewRow{}, s.getErr
	}
	// 与真实 SQL 同语义：法人严格相等才可见，否则 ErrNoRows。
	scopedID, err := entity.LegalEntityID()
	if err != nil || entity.IsGlobal() {
		return repository.DraftReviewRow{}, pgx.ErrNoRows
	}
	for _, row := range s.rows {
		if row.ID == id && row.LegalEntityID != nil && *row.LegalEntityID == scopedID {
			return row, nil
		}
	}
	return repository.DraftReviewRow{}, pgx.ErrNoRows
}

func (s *draftReviewStubReader) UpdateDraftReview(_ context.Context, _ string, _ repository.UpdateDraftReviewInput) error {
	return s.updateErr
}

func (s *draftReviewStubReader) ResolveOrCreateStoreID(_ context.Context, _, _ string, _ *string) (*string, error) {
	id := "00000000-0000-0000-0000-00000000store"
	return &id, nil
}

func (s *draftReviewStubReader) ResolveOrCreateLandlordID(_ context.Context, _ string) (*string, error) {
	id := "00000000-0000-0000-0000-00000landlord"
	return &id, nil
}

func (s *draftReviewStubReader) SaveDraftEdits(_ context.Context, _ string, _ json.RawMessage) error { return nil }

// failingUOW rejects any approval write — the tests that reach it expect the
// confidence gate to fail first.
type failingUOW struct{}

func (failingUOW) Execute(_ context.Context, _ func(draftreview.ContractStore) error) error {
	return context.DeadlineExceeded
}

func draftReviewRouter(t *testing.T, reader *draftReviewStubReader, entity string, global bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	handler := NewContractDraftReviewHandler(reader, failingUOW{})
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "00000000-0000-0000-0000-00000000abcd")
		scope := access.Scope{LegalEntityID: entity, Global: global}
		c.Request = c.Request.WithContext(access.WithScope(c.Request.Context(), scope))
	})
	router.GET("/contracts/drafts", handler.ListDrafts)
	router.GET("/contracts/drafts/:id", handler.GetDraft)
	router.PUT("/contracts/drafts/:id", handler.ReviseDraft)
	router.POST("/contracts/drafts/decide", handler.DecideDrafts)
	return router
}

func doJSON(t *testing.T, router *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func TestDraftReviewListEnvelope(t *testing.T) {
	entityA := "11111111-1111-1111-1111-111111111111"
	reader := &draftReviewStubReader{rows: []repository.DraftReviewRow{{
		ID: "d-1", Status: "pending", LegalEntityID: &entityA,
		ContractData: json.RawMessage(`{"contract_number":"CT-1"}`),
	}}}
	recorder := doJSON(t, draftReviewRouter(t, reader, entityA, false), http.MethodGet, "/contracts/drafts", "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	body := decodeBody(t, recorder)
	data, ok := body["data"].([]any)
	if !ok || len(data) != 1 {
		t.Fatalf("list envelope missing data array: %s", recorder.Body.String())
	}
}

// 异法人与不存在：同一状态码、同一错误码、同一文案 —— 无存在性泄漏。
func TestDraftReviewGetForeignAndMissingSameShape(t *testing.T) {
	entityB := "22222222-2222-2222-2222-222222222222"
	foreign := "33333333-3333-3333-3333-333333333333"
	reader := &draftReviewStubReader{rows: []repository.DraftReviewRow{{
		ID: foreign, Status: "pending", LegalEntityID: &entityB,
		ContractData: json.RawMessage(`{}`),
	}}}
	router := draftReviewRouter(t, reader, "11111111-1111-1111-1111-111111111111", false)

	foreignRec := doJSON(t, router, http.MethodGet, "/contracts/drafts/"+foreign, "")
	missingRec := doJSON(t, router, http.MethodGet, "/contracts/drafts/44444444-4444-4444-4444-444444444444", "")

	for _, rec := range []*httptest.ResponseRecorder{foreignRec, missingRec} {
		if rec.Code != http.StatusNotFound {
			t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
		}
		body := decodeBody(t, rec)
		if body["code"] != "scope_denied" {
			t.Fatalf("code=%v body=%s", body["code"], rec.Body.String())
		}
		if body["error"] != "scope_denied: contract draft outside caller legal entity" {
			t.Fatalf("softened wording: %v", body["error"])
		}
	}
	if foreignRec.Body.String() != missingRec.Body.String() {
		t.Fatalf("existence leak: foreign %s vs missing %s",
			foreignRec.Body.String(), missingRec.Body.String())
	}
}

func TestDraftReviewGetForGlobalAdminIsDeniedNotEverything(t *testing.T) {
	reader := &draftReviewStubReader{}
	recorder := doJSON(t, draftReviewRouter(t, reader, "", true), http.MethodGet, "/contracts/drafts/whatever", "")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("global admin got status=%d", recorder.Code)
	}
	if decodeBody(t, recorder)["code"] != "scope_denied" {
		t.Fatalf("global admin miss not coded scope_denied: %s", recorder.Body.String())
	}
}

func TestDraftReviewDecideLowConfidenceFailsServerSide(t *testing.T) {
	entityA := "11111111-1111-1111-1111-111111111111"
	reader := &draftReviewStubReader{rows: []repository.DraftReviewRow{{
		ID: "d-low", Status: "pending", LegalEntityID: &entityA,
		ContractData:   json.RawMessage(`{"contract_number":"CT-2","lessee_name":"甲","lessor_name":"乙","currency":"CNY","commencement_date":"2026-01-01","lease_start_date":"2026-01-01","lease_end_date":"2027-12-31"}`),
		ConfidenceScores: json.RawMessage(`{"lessee_name":0.42}`),
	}}}
	recorder := doJSON(t, draftReviewRouter(t, reader, entityA, false), http.MethodPost, "/contracts/drafts/decide",
		`{"decisions":[{"draft_id":"d-low","approve":true}]}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	body := decodeBody(t, recorder)
	items, _ := body["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("outcome items missing: %s", recorder.Body.String())
	}
	item, _ := items[0].(map[string]any)
	if item["verdict"] != "failed" || !strings.Contains(item["error"].(string), "low_confidence_fields_unconfirmed") {
		t.Fatalf("gate did not fail the item server-side: %v", item)
	}
}

func TestDraftReviewReviseMalformedBodyIsInvalidArguments(t *testing.T) {
	entityA := "11111111-1111-1111-1111-111111111111"
	recorder := doJSON(t, draftReviewRouter(t, &draftReviewStubReader{}, entityA, false),
		http.MethodPut, "/contracts/drafts/d-1", `{"edits": "not-an-array"}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", recorder.Code)
	}
	if decodeBody(t, recorder)["code"] != "invalid_arguments" {
		t.Fatalf("body=%s", recorder.Body.String())
	}
}
