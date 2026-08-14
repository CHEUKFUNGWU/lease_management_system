package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/errcontract"
	"github.com/lease-management-system/core-service/internal/repository"
)

func decodeBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v body=%s", err, recorder.Body.String())
	}
	return body
}

// The same requests as the pre-contract tests keep their HTTP status codes;
// the contract adds the `code` without moving the status. R5 parity is proven
// by these assertions plus the untouched status checks in the existing
// retail_*_test.go files.
func TestErrorContractCarriesCodesWithoutChangingStatuses(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// 400 validation -> invalid_arguments
	handler := NewRetailPulseHandler(pulseHandlerReader{set: &repository.RetailKPIFactSet{}})
	router := gin.New()
	router.GET("/pulse", func(c *gin.Context) { c.Set("legal_entity_id", "entity-a"); handler.OperatingPulse(c) })
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/pulse?as_of=2026-01-31&data_classification=production&window_days=99", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("validation status=%d", recorder.Code)
	}
	body := decodeBody(t, recorder)
	if body["code"] != "invalid_arguments" {
		t.Fatalf("validation code=%v body=%s", body["code"], recorder.Body.String())
	}

	// 409 source conflict -> conflict + details.reason
	conflictHandler := NewRetailPulseHandler(pulseHandlerReader{err: repository.ErrRetailKPISourceConflict})
	conflictRouter := gin.New()
	conflictRouter.GET("/pulse", func(c *gin.Context) { c.Set("legal_entity_id", "entity-a"); conflictHandler.OperatingPulse(c) })
	conflictRecorder := httptest.NewRecorder()
	conflictRouter.ServeHTTP(conflictRecorder, httptest.NewRequest(http.MethodGet, "/pulse?as_of=2026-01-31&data_classification=production", nil))
	if conflictRecorder.Code != http.StatusConflict {
		t.Fatalf("conflict status=%d", conflictRecorder.Code)
	}
	conflictBody := decodeBody(t, conflictRecorder)
	details, _ := conflictBody["details"].(map[string]any)
	if conflictBody["code"] != "conflict" || details["reason"] != "source_conflict" {
		t.Fatalf("conflict code=%v details=%v", conflictBody["code"], conflictBody["details"])
	}
}

// R6: an unclassified database error must be sanitized — no SQL fragment or
// internal path may reach the response body, and the code is system_failure.
func TestErrorContractRedactsInternalFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeRetailStoreDayFactRepo{listErr: fmt.Errorf(`ERROR: relation "retail_store_day_facts" does not exist (SQLSTATE 42P01) at /Users/cheukfungwu/ifrs16_management_system/core-service/internal/repository/retail_store_day_facts.go:336`)}
	handler := NewRetailStoreDayFactsHandler(repo, nil)
	c, recorder := retailStoreDayTestContext(http.MethodGet, "/retail/operating-facts/store-days?date_from=2026-08-01&date_to=2026-08-02", nil)
	handler.List(c)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("internal failure status=%d", recorder.Code)
	}
	raw := recorder.Body.String()
	for _, leak := range []string{"relation", "SQLSTATE", "/Users/", "retail_store_day_facts.go"} {
		if strings.Contains(raw, leak) {
			t.Fatalf("internal detail leaked into response: %q in %s", leak, raw)
		}
	}
	body := decodeBody(t, recorder)
	if body["code"] != "system_failure" || body["error"] != "internal server error" {
		t.Fatalf("sanitized body=%v", body)
	}
}

// Unit-level pass-through: a contract scope_denied error from the repository
// keeps its code and message on the way out (the end-to-end proof against a
// real database is TestRetailStoreDayFactsUpsertScopeDeniedEndToEnd).
func TestErrorContractPassesScopeDeniedThroughUpsert(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeRetailStoreDayFactRepo{upsertErr: errcontract.New(errcontract.CodeScopeDenied, "retail store is outside the caller data scope")}
	handler := NewRetailStoreDayFactsHandler(repo, nil)
	c, recorder := retailStoreDayTestContext(http.MethodPost, "/retail/operating-facts/store-days", retailStoreDayFactRequest{Items: []retailStoreDayFactInput{validRetailStoreDayInput()}})
	handler.Upsert(c)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("scope denied status=%d", recorder.Code)
	}
	body := decodeBody(t, recorder)
	if body["code"] != "scope_denied" {
		t.Fatalf("scope denied code=%v body=%s", body["code"], recorder.Body.String())
	}
}

// R4: end-to-end scope_denied against a real PostgreSQL — a tenant-A scoped
// caller writing facts for a tenant-B store gets code scope_denied in the
// response body, straight from the repository through the HTTP adapter.
func TestRetailStoreDayFactsUpsertScopeDeniedEndToEnd(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect to Postgres: %v", err)
	}
	// Registered before the data cleanup so LIFO runs the deletes first:
	// a deferred pool.Close() would run before t.Cleanup and silence them.
	t.Cleanup(pool.Close)
	ctx := context.Background()

	suffixA, suffixB := uuid.NewString()[:8], uuid.NewString()[:8]
	var entityA, entityB, storeA, storeB string
	if err := pool.QueryRow(ctx, `INSERT INTO legal_entities (code, name, country, currency, is_active) VALUES ($1,$2,'CN','CNY',true) RETURNING id`, "EC-LE-A-"+suffixA, "Contract tenant A").Scan(&entityA); err != nil {
		t.Fatalf("seed legal entity A: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO legal_entities (code, name, country, currency, is_active) VALUES ($1,$2,'CN','CNY',true) RETURNING id`, "EC-LE-B-"+suffixB, "Contract tenant B").Scan(&entityB); err != nil {
		t.Fatalf("seed legal entity B: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO stores (code, name, legal_entity_id, brand, region, is_active) VALUES ($1,$2,$3,$4,$5,true) RETURNING id`, "EC-ST-A-"+suffixA, "Contract store A", entityA, "Brand-A", "Region-A").Scan(&storeA); err != nil {
		t.Fatalf("seed store A: %v", err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO stores (code, name, legal_entity_id, brand, region, is_active) VALUES ($1,$2,$3,$4,$5,true) RETURNING id`, "EC-ST-B-"+suffixB, "Contract store B", entityB, "Brand-B", "Region-B").Scan(&storeB); err != nil {
		t.Fatalf("seed store B: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM retail_store_day_fact_requests WHERE legal_entity_id IN ($1,$2)`, entityA, entityB)
		_, _ = pool.Exec(context.Background(), `DELETE FROM retail_store_day_facts WHERE store_id IN ($1,$2)`, storeA, storeB)
		_, _ = pool.Exec(context.Background(), `DELETE FROM stores WHERE id IN ($1,$2)`, storeA, storeB)
		_, _ = pool.Exec(context.Background(), `DELETE FROM legal_entities WHERE id IN ($1,$2)`, entityA, entityB)
	})

	gin.SetMode(gin.TestMode)
	handler := NewRetailStoreDayFactsHandler(repository.NewOperatingFactsRepository(pool), nil)

	revenue := 100.0
	payload, _ := json.Marshal(retailStoreDayFactRequest{Items: []retailStoreDayFactInput{{
		StoreID: storeB, BusinessDate: "2026-08-01", Currency: "CNY", Revenue: &revenue,
		SourceSystem: "pos", DataClassification: "production",
	}}})

	// Tenant-A scope writing facts for tenant-B's store -> scope_denied.
	deniedRecorder := httptest.NewRecorder()
	deniedRequest := httptest.NewRequest(http.MethodPost, "/retail/operating-facts/store-days", bytes.NewReader(payload))
	deniedRequest.Header.Set("Content-Type", "application/json")
	deniedCtx, _ := gin.CreateTestContext(deniedRecorder)
	deniedCtx.Request = deniedRequest
	deniedCtx.Set("legal_entity_id", entityA)
	deniedCtx.Set("access_scope", access.Scope{LegalEntityID: entityA})
	// No user_id: the handler writes created_by as NULL, matching the
	// repository integration tests (created_by is a users FK).
	handler.Upsert(deniedCtx)
	if deniedRecorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("out-of-scope status=%d body=%s", deniedRecorder.Code, deniedRecorder.Body.String())
	}
	deniedBody := decodeBody(t, deniedRecorder)
	if deniedBody["code"] != "scope_denied" {
		t.Fatalf("out-of-scope code=%v body=%s", deniedBody["code"], deniedRecorder.Body.String())
	}

	// Positive control: the same caller writing facts for its own store.
	ownPayload, _ := json.Marshal(retailStoreDayFactRequest{Items: []retailStoreDayFactInput{{
		StoreID: storeA, BusinessDate: "2026-08-01", Currency: "CNY", Revenue: &revenue,
		SourceSystem: "pos", DataClassification: "production",
	}}})
	ownRecorder := httptest.NewRecorder()
	ownRequest := httptest.NewRequest(http.MethodPost, "/retail/operating-facts/store-days", bytes.NewReader(ownPayload))
	ownRequest.Header.Set("Content-Type", "application/json")
	ownCtx, _ := gin.CreateTestContext(ownRecorder)
	ownCtx.Request = ownRequest
	ownCtx.Set("legal_entity_id", entityA)
	ownCtx.Set("access_scope", access.Scope{LegalEntityID: entityA})
	handler.Upsert(ownCtx)
	if ownRecorder.Code != http.StatusOK {
		t.Fatalf("in-scope status=%d body=%s", ownRecorder.Code, ownRecorder.Body.String())
	}
}
