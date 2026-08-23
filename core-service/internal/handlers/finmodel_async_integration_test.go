package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/repository"
)

const asyncTemplateRows = `{"name":"ASYNC-TPL","version":1,"rows":[
	{"key":"rev","label":"营业收入","kind":"link","basis":"shared","source":"fact.revenue"},
	{"key":"gross_margin_rate","label":"毛利率假设","kind":"input","basis":"shared"},
	{"key":"gp","label":"毛利","kind":"formula","basis":"shared","formula":"rows.rev * rows.gross_margin_rate"},
	{"key":"total_gp","label":"毛利合计","kind":"subtotal","basis":"shared","children":["gp"]},
	{"key":"cash","label":"现金","kind":"input","basis":"ifrs16_basis","fold":"stock"},
	{"key":"ar","label":"应收账款","kind":"input","basis":"ifrs16_basis","fold":"stock"},
	{"key":"inventory","label":"存货","kind":"input","basis":"ifrs16_basis","fold":"stock"},
	{"key":"ppe","label":"固定资产","kind":"input","basis":"ifrs16_basis","fold":"stock"},
	{"key":"rou_asset","label":"使用权资产","kind":"input","basis":"ifrs16_basis","fold":"stock"},
	{"key":"total_assets","label":"资产合计","kind":"subtotal","basis":"ifrs16_basis","children":["cash","ar","inventory","ppe","rou_asset"]},
	{"key":"ap","label":"应付账款","kind":"input","basis":"ifrs16_basis","fold":"stock"},
	{"key":"lease_liability","label":"租赁负债","kind":"input","basis":"ifrs16_basis","fold":"stock"},
	{"key":"borrowings","label":"借款","kind":"input","basis":"ifrs16_basis","fold":"stock"},
	{"key":"total_liabilities","label":"负债合计","kind":"subtotal","basis":"ifrs16_basis","children":["ap","lease_liability","borrowings"]},
	{"key":"share_capital","label":"股本","kind":"input","basis":"ifrs16_basis","fold":"stock"},
	{"key":"retained_earnings","label":"留存收益","kind":"input","basis":"ifrs16_basis","fold":"stock"},
	{"key":"total_equity","label":"权益合计","kind":"subtotal","basis":"ifrs16_basis","children":["share_capital","retained_earnings"]},
	{"key":"nwc","label":"营运资本","kind":"input","basis":"ifrs16_basis","fold":"stock"},
	{"key":"borrowings_opening","label":"期初借款","kind":"input","basis":"ifrs16_basis","fold":"stock"},
	{"key":"ending_cash","label":"期末现金","kind":"input","basis":"ifrs16_basis","fold":"stock"}
]}`

// asyncFixture provisions entity/template/definition and a handler whose
// runs execute the real engine and persist through the real schema.
func asyncFixture(t *testing.T, pool *pgxpool.Pool) (string, string, string, *FinModelHandler) {
	t.Helper()
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("fixture exec: %v", err)
		}
	}
	suffix := uuid.NewString()[:8]
	entity := uuid.NewString()
	exec(`INSERT INTO legal_entities (id, code, name, country, currency) VALUES ($1,$2,$3,'CN','CNY')`,
		entity, "ASYNC-E-"+suffix, "Async "+suffix)
	userID := uuid.NewString()
	exec(`INSERT INTO users (id, username, email, password_hash, legal_entity_id)
		VALUES ($1,$2,$3,'integration-only',$4)`, userID, "async-user-"+suffix, "async-user-"+suffix+"@example.com", entity)
	tmplID := uuid.NewString()
	exec(`INSERT INTO fin_statement_templates (id, legal_entity_id, name, version, status, rows)
		VALUES ($1,$2,$3,1,'approved',$4::jsonb)`, tmplID, entity, "ASYNC-TPL-"+suffix, json.RawMessage(asyncTemplateRows))
	defID := uuid.NewString()
	exec(`INSERT INTO fin_model_definitions (id, legal_entity_id, name, version, template_id, policy, source_bindings)
		VALUES ($1,$2,$3,1,$4,'{}'::jsonb,'{}'::jsonb)`, defID, entity, "ASYNC-DEF-"+suffix, tmplID)
	return entity, defID, userID, NewFinModelHandler(repository.NewFinModelRepository(pool))
}

func ginServe(handler *FinModelHandler, entity, userID, method, path string, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	wrap := func(fn func(*gin.Context)) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Set("legal_entity_id", entity)
			c.Set("user_id", userID)
			fn(c)
		}
	}
	router.POST("/financial-model/definitions/:id/runs", wrap(handler.RunDefinition))
	router.GET("/financial-model/runs/:id", wrap(handler.GetRun))
	router.POST("/financial-model/runs/:id/cancel", wrap(handler.CancelRun))
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// TestAsyncRunEndsHonestlyAndReplaysPostgres locks the S2-5 lifecycle: an
// async request gets 202 with a queued run, the worker reaches a TERMINAL
// state — with the ports unwired the tie-out gate correctly refuses the
// publishable result and the failure_reason says why (fail-closed, never a
// zombie "running") — and a replayed request returns the same run instead
// of a new one.
func TestAsyncRunCompletesAndReplaysPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to Postgres: %v", err)
	}
	t.Cleanup(pool.Close)
	entity, defID, userID, handler := asyncFixture(t, pool)

	dispatch := ginServe(handler, entity, userID, http.MethodPost, "/financial-model/definitions/"+defID+"/runs",
		`{"definition_id":"`+defID+`","async":true,"idempotency_key":"async-k-1","assumptions":{"gross_margin_rate":0.4},"versions":{"data_version":"2026-08-18"}}`)
	if dispatch.Code != http.StatusAccepted {
		t.Fatalf("async dispatch status %d: %s", dispatch.Code, dispatch.Body.String())
	}
	var dispatched struct {
		RunID  string `json:"run_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(dispatch.Body.Bytes(), &dispatched); err != nil {
		t.Fatal(err)
	}
	if dispatched.RunID == "" || dispatched.Status != "queued" {
		t.Fatalf("dispatch body = %+v", dispatched)
	}

	// 轮询进度直至终态（引擎同步执行很快，但路径真实走后台 worker）。
	deadline := time.Now().Add(8 * time.Second)
	status := ""
	failure := ""
	lastBody := ""
	for time.Now().Before(deadline) {
		statusW := ginServe(handler, entity, userID, http.MethodGet, "/financial-model/runs/"+dispatched.RunID, "")
		var progress struct {
			Run       repository.FinModelRun `json:"run"`
			LineCount int                    `json:"line_count"`
		}
		if err := json.Unmarshal(statusW.Body.Bytes(), &progress); err != nil {
			t.Fatalf("progress body: %v (%s)", err, statusW.Body.String())
		}
		status = progress.Run.Status
		failure = ""
		if progress.Run.FailureReason != nil {
			failure = *progress.Run.FailureReason
		}
		lastBody = statusW.Body.String()
		if status == "completed" || status == "failed" || status == "cancelled" {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if status != "failed" {
		t.Fatalf("expected the fail-closed terminal state (ports unwired → tie-out gate refuses), got %q: %s", status, lastBody)
	}
	if !strings.Contains(failure, "tie-out") {
		t.Fatalf("failure_reason must name the gate, got %q", failure)
	}

	// 幂等重放：同一 idempotency_key 返回同一 run，不产生第二条记录。
	replay := ginServe(handler, entity, userID, http.MethodPost, "/financial-model/definitions/"+defID+"/runs",
		`{"definition_id":"`+defID+`","async":true,"idempotency_key":"async-k-1","assumptions":{"gross_margin_rate":0.4}}`)
	var replayed struct {
		RunID    string `json:"run_id"`
		Replayed bool   `json:"replayed"`
	}
	if err := json.Unmarshal(replay.Body.Bytes(), &replayed); err != nil {
		t.Fatal(err)
	}
	if !replayed.Replayed || replayed.RunID != dispatched.RunID {
		t.Fatalf("replay must return the existing run, got %+v (status %d)", replayed, replay.Code)
	}
}

// TestRunDefinitionRejectsCrossEntityDefinitionPostgres locks P0-1 (底线 1):
// a tenant-A caller presenting tenant-B's definition id must be refused with
// the unsoftened reason, while the same-entity caller passes the scope gate.
func TestRunDefinitionRejectsCrossEntityDefinitionPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to Postgres: %v", err)
	}
	t.Cleanup(pool.Close)
	entity, defID, userID, handler := asyncFixture(t, pool)

	// 跨法人：以另一法人身份请求本法人的 definition。
	otherEntity := uuid.NewString()
	denied := ginServe(handler, otherEntity, userID, http.MethodPost, "/financial-model/definitions/"+defID+"/runs",
		`{"definition_id":"`+defID+`","assumptions":{"gross_margin_rate":0.4}}`)
	if denied.Code != http.StatusNotFound {
		t.Fatalf("cross-entity run must be refused, got %d: %s", denied.Code, denied.Body.String())
	}
	if !strings.Contains(denied.Body.String(), "model definition not found") {
		t.Fatalf("cross-entity refusal must keep the reason, got %s", denied.Body.String())
	}

	// 法人内：同法人请求通过权限门（引擎跑通，不是 404）。
	own := ginServe(handler, entity, userID, http.MethodPost, "/financial-model/definitions/"+defID+"/runs",
		`{"definition_id":"`+defID+`","assumptions":{"gross_margin_rate":0.4}}`)
	if own.Code == http.StatusNotFound {
		t.Fatalf("same-entity run must pass the scope gate: %s", own.Body.String())
	}
}

// TestAsyncRunCancelPostgres holds a worker at the persist boundary through
// the test hook and proves cancellation lands as cancelled with zero lines.
func TestAsyncRunCancelPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to Postgres: %v", err)
	}
	t.Cleanup(pool.Close)
	entity, defID, userID, handler := asyncFixture(t, pool)

	gate := make(chan string, 1)
	release := make(chan struct{})
	oldHook := asyncRunHook
	asyncRunHook = func(runID string) {
		gate <- runID
		<-release
	}
	t.Cleanup(func() { asyncRunHook = oldHook })

	dispatch := ginServe(handler, entity, userID, http.MethodPost, "/financial-model/definitions/"+defID+"/runs",
		`{"definition_id":"`+defID+`","async":true,"assumptions":{"gross_margin_rate":0.4}}`)
	if dispatch.Code != http.StatusAccepted {
		t.Fatalf("dispatch status %d: %s", dispatch.Code, dispatch.Body.String())
	}
	var dispatched struct {
		RunID string `json:"run_id"`
	}
	_ = json.Unmarshal(dispatch.Body.Bytes(), &dispatched)

	select {
	case <-gate:
	case <-time.After(8 * time.Second):
		t.Fatal("worker never reached the persist boundary")
	}
	cancel := ginServe(handler, entity, userID, http.MethodPost, "/financial-model/runs/"+dispatched.RunID+"/cancel", "")
	if cancel.Code != http.StatusOK {
		t.Fatalf("cancel status %d: %s", cancel.Code, cancel.Body.String())
	}
	close(release)

	deadline := time.Now().Add(8 * time.Second)
	status := ""
	for time.Now().Before(deadline) {
		statusW := ginServe(handler, entity, userID, http.MethodGet, "/financial-model/runs/"+dispatched.RunID, "")
		var progress struct {
			Run       repository.FinModelRun `json:"run"`
			LineCount int                    `json:"line_count"`
		}
		if err := json.Unmarshal(statusW.Body.Bytes(), &progress); err != nil {
			t.Fatal(err)
		}
		if progress.Run.Status == "cancelled" {
			if progress.LineCount != 0 {
				t.Fatalf("cancelled run must not persist lines, got %d", progress.LineCount)
			}
			status = "cancelled"
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if status != "cancelled" {
		t.Fatalf("run did not land as cancelled, last %q", status)
	}
}
