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
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/errcontract"
	"github.com/lease-management-system/core-service/internal/repository"
)

// SI2 红态测试：run / artifact 直读路径 + 写路径的跨法人拒绝。
//
// 判据同 SI1（订正后口径）：测试断言 CORRECT 行为——法人 B 上下文取法人 A
// 的 run / artifact / 对其执行审批必须拒绝，且 scope_denied 不软化。因此
// 今天的代码（仅按 user 校验、直接返回成功）跑这些测试就是红的；修完变绿。
//
// 全部跑 make test-integration 实跑；skip 不算证据。

func si2Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect to Postgres: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// si2Fixture seeds one user spanning entities A and B, with a session + run +
// artifact belonging to A (the cross-tenant probe target).
func si2Fixture(t *testing.T, pool *pgxpool.Pool) (entityA, entityB, userID, runID, artifactID string) {
	t.Helper()
	ctx := context.Background()
	exec := func(sql string, args ...any) string {
		t.Helper()
		row := pool.QueryRow(ctx, sql, args...)
		var id string
		if err := row.Scan(&id); err != nil {
			t.Fatalf("fixture exec: %v", err)
		}
		return id
	}
	suffix := uuid.NewString()[:8]
	entityA = exec(`INSERT INTO legal_entities (code, name, country, currency, is_active)
		VALUES ($1,$2,'CN','CNY',true) RETURNING id`, "SI2-A-"+suffix, "SI2 A")
	entityB = exec(`INSERT INTO legal_entities (code, name, country, currency, is_active)
		VALUES ($1,$2,'CN','CNY',true) RETURNING id`, "SI2-B-"+suffix, "SI2 B")
	userID = exec(`INSERT INTO users (username, email, password_hash)
		VALUES ($1,$2,'integration-only') RETURNING id`, "si2-"+suffix, "si2-"+suffix+"@example.com")
	sessionID := exec(`INSERT INTO ai_chat_sessions (user_id, legal_entity_id, title)
		VALUES ($1,$2,'A 的会话') RETURNING id`, userID, entityA)
	runID = exec(`INSERT INTO ai_chat_runs (session_id, status, agent_mode)
		VALUES ($1,'completed',true) RETURNING id`, sessionID)
	artifactID = exec(`INSERT INTO ai_chat_artifacts (session_id, run_id, artifact_type, title, data)
		VALUES ($1,$2,'contract_draft','A 的草稿','{}'::jsonb) RETURNING id`, sessionID, runID)
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM ai_chat_sessions WHERE user_id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM legal_entities WHERE id = ANY($1::uuid[])`,
			[]string{entityA, entityB})
	})
	return entityA, entityB, userID, runID, artifactID
}

// si2Router mounts the given handler method behind a gin context that carries
// entity B's scope (the "caller context") — matching how DataScopeMiddleware
// installs access scope in production.
func si2Router(register func(*gin.Engine, *AIChatHandler), entityB, userID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	scope := access.Scope{LegalEntityID: entityB}
	router.Use(func(c *gin.Context) {
		c.Set("user_id", userID)
		c.Set("legal_entity_id", entityB)
		c.Set("access_scope", scope)
		c.Set("permissions", []string{"ai_chat:use", "ai_drafts:confirm", "ai_drafts:review"})
		c.Request = c.Request.WithContext(access.WithScope(c.Request.Context(), scope))
		c.Next()
	})
	handler := &AIChatHandler{runtimeRepo: repository.NewAIChatRuntimeRepository(poolOf)}
	register(router, handler)
	return router
}

// poolOf is replaced by the concrete pool passed through si2Serve.
var poolOf *pgxpool.Pool

func si2Serve(t *testing.T, pool *pgxpool.Pool, entityB, userID, method, path string, register func(*gin.Engine, *AIChatHandler)) *httptest.ResponseRecorder {
	t.Helper()
	poolOf = pool
	router := si2Router(register, entityB, userID)
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// si2ServeJSON is si2Serve for handlers that bind a JSON body (POST write
// paths): cross-entity refusal must happen BEFORE the body matters, so the
// body here is a minimal valid one.
func si2ServeJSON(t *testing.T, pool *pgxpool.Pool, entityB, userID, method, path, body string, register func(*gin.Engine, *AIChatHandler)) *httptest.ResponseRecorder {
	t.Helper()
	poolOf = pool
	router := si2Router(register, entityB, userID)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// assertScopeDeniedResponse checks the endpoint refused with 403 + scope_denied.
func assertScopeDeniedResponse(t *testing.T, w *httptest.ResponseRecorder, endpoint string) {
	t.Helper()
	if w.Code != http.StatusForbidden {
		t.Fatalf("%s: cross-entity read returned %d; want %d (scope_denied, never softened) body=%s",
			endpoint, w.Code, http.StatusForbidden, w.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("%s: body not json: %v", endpoint, err)
	}
	code, _ := body["code"].(string)
	if code != string(errcontract.CodeScopeDenied) {
		t.Fatalf("%s: refusal code = %q; want %q", endpoint, code, errcontract.CodeScopeDenied)
	}
}

// ── 读路径：暴露端点 ─────────────────────────────────────────────────────

func TestListRunEventsRefusesCrossEntity(t *testing.T) {
	pool := si2Pool(t)
	_, entityB, userID, runID, _ := si2Fixture(t, pool)

	w := si2Serve(t, pool, entityB, userID, http.MethodGet,
		"/ai/chat/runs/"+runID+"/events", func(r *gin.Engine, h *AIChatHandler) {
			r.GET("/ai/chat/runs/:id/events", h.ListRunEvents)
		})
	assertScopeDeniedResponse(t, w, "GET runs/:id/events")
}

func TestGetAgentRunTraceRefusesCrossEntity(t *testing.T) {
	pool := si2Pool(t)
	_, entityB, userID, runID, _ := si2Fixture(t, pool)

	w := si2Serve(t, pool, entityB, userID, http.MethodGet,
		"/ai/chat/runs/"+runID+"/trace", func(r *gin.Engine, h *AIChatHandler) {
			r.GET("/ai/chat/runs/:id/trace", h.GetAgentRunTrace)
		})
	assertScopeDeniedResponse(t, w, "GET runs/:id/trace")
}

func TestStreamRunEventsRefusesCrossEntity(t *testing.T) {
	pool := si2Pool(t)
	_, entityB, userID, runID, _ := si2Fixture(t, pool)

	// streaming 在修复前会进入 SSE 长循环（无终止事件）；用 1 秒后取消的
	// 请求上下文保证今天红的形态下测试也能退出，而不是挂死。修复后 403
	// 在建流前返回，本判据不变。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(1*time.Second, cancel)

	poolOf = pool
	router := si2Router(func(r *gin.Engine, h *AIChatHandler) {
		r.GET("/ai/chat/runs/:id/stream", h.StreamRunEvents)
	}, entityB, userID)
	req := httptest.NewRequest(http.MethodGet, "/ai/chat/runs/"+runID+"/stream", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	// streaming 以 run_meta 事件开头；跨法人必须根本不开流。
	if w.Code == http.StatusOK {
		t.Fatalf("stream: cross-entity run started streaming; want 403 scope_denied body=%s", w.Body.String())
	}
}

func TestGetArtifactRefusesCrossEntity(t *testing.T) {
	pool := si2Pool(t)
	_, entityB, userID, _, artifactID := si2Fixture(t, pool)

	w := si2Serve(t, pool, entityB, userID, http.MethodGet,
		"/ai/chat/artifacts/"+artifactID, func(r *gin.Engine, h *AIChatHandler) {
			r.GET("/ai/chat/artifacts/:id", h.GetArtifact)
		})
	assertScopeDeniedResponse(t, w, "GET artifacts/:id")
}

func TestExportArtifactRefusesCrossEntity(t *testing.T) {
	pool := si2Pool(t)
	_, entityB, userID, _, artifactID := si2Fixture(t, pool)

	w := si2Serve(t, pool, entityB, userID, http.MethodGet,
		"/ai/chat/artifacts/"+artifactID+"/export", func(r *gin.Engine, h *AIChatHandler) {
			r.GET("/ai/chat/artifacts/:id/export", h.ExportArtifact)
		})
	assertScopeDeniedResponse(t, w, "GET artifacts/:id/export")
}
