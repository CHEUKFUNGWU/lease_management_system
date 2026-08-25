package handlers

// RT1-L3-B 健康检查。先写测试后写实现：本文件的核心断言在旧实现下全绿——
// 那正是它要修正的谎报（DB 探测失败仍 200 + "status":"ok"，错误原文进响应体）。
// 三条不恒真的测试：断 DB 必须不报健康、内部错误原文不得出响应体、liveness 与
// readiness 在「进程活 + DB 断」下结论不同。把判断改回恒 200 必须变红。

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/llm"
)

func newHealthRouter(h *HealthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/live", h.Live)
	r.GET("/ready", h.Ready)
	return r
}

const dbErrorMarker = "pq: root cause detail marker XJ42"

func failingPostgres() func(context.Context) error {
	return func(context.Context) error { return errors.New(dbErrorMarker) }
}

func healthyPostgres() func(context.Context) error {
	return func(context.Context) error { return nil }
}

// ── 核心反向测试：断 DB 必须不报健康 ─────────────────────────────────────────

func TestReadyReportsUnavailableWhenDatabaseDown(t *testing.T) {
	h := NewHealthHandler(failingPostgres())
	w := httptest.NewRecorder()
	newHealthRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness must answer 503 when postgres is down (the constant-200 is exactly the lie this endpoint used to tell), got %d", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, `"status":"ok"`) || strings.Contains(body, `"status": "ok"`) {
		t.Fatalf("body must not claim overall ok while a gating dependency is down: %s", body)
	}
}

func TestReadyResponseOmitsInternalErrorDetail(t *testing.T) {
	h := NewHealthHandler(failingPostgres())
	w := httptest.NewRecorder()
	newHealthRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("precondition: readiness must be 503, got %d", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, dbErrorMarker) || strings.Contains(body, "XJ42") {
		t.Fatalf("internal error detail must not reach the public response body (probes carry no credentials): %s", body)
	}
}

// ── liveness / readiness 分离 ────────────────────────────────────────────────

func TestLivenessPassesWhenDatabaseDown(t *testing.T) {
	h := NewHealthHandler(failingPostgres())
	w := httptest.NewRecorder()
	newHealthRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/live", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("liveness must pass when only the database is down (restarting the process cannot heal postgres), got %d", w.Code)
	}
}

func TestLivenessAndReadinessDivergeOnProcessAliveDbDown(t *testing.T) {
	h := NewHealthHandler(failingPostgres())
	r := newHealthRouter(h)

	live := httptest.NewRecorder()
	r.ServeHTTP(live, httptest.NewRequest(http.MethodGet, "/live", nil))
	ready := httptest.NewRecorder()
	r.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if live.Code != http.StatusOK || ready.Code != http.StatusServiceUnavailable {
		t.Fatalf("process-alive+db-down must yield live=200 ready=503, got live=%d ready=%d", live.Code, ready.Code)
	}
}

// ── 依赖清单语义 ─────────────────────────────────────────────────────────────

func TestReadyOkWhenGatingDependenciesHealthy(t *testing.T) {
	h := NewHealthHandler(healthyPostgres()).WithMinioProbe(func(context.Context) error { return nil }).WithLLMState(LLMDegraded)
	w := httptest.NewRecorder()
	newHealthRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("healthy infra must answer 200 even when the llm signal is degraded (paid external service, not a gate), got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"postgres":"ok"`) || !strings.Contains(w.Body.String(), `"minio":"ok"`) {
		t.Fatalf("per-dependency states must be reported: %s", w.Body.String())
	}
}

func TestMinioNotConfiguredDoesNotGateReadiness(t *testing.T) {
	// miniostore.New 对空 endpoint 返回 nil client 是合法部署形态——如实报告
	// not_configured，不算 down。
	h := NewHealthHandler(healthyPostgres()).WithMinioProbe(nil)
	w := httptest.NewRecorder()
	newHealthRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("absent optional dependency must not fail readiness, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"minio":"not_configured"`) {
		t.Fatalf("absent minio must be reported honestly as not_configured: %s", w.Body.String())
	}
}

func TestMinioDownFailsReadiness(t *testing.T) {
	h := NewHealthHandler(healthyPostgres()).WithMinioProbe(func(context.Context) error {
		return errors.New("minio unreachable")
	})
	w := httptest.NewRecorder()
	newHealthRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ready", nil))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("configured-but-down minio must fail readiness, got %d", w.Code)
	}
}

// LLM 信号三态只展示、不闸门；unknown（进程启动后尚无真实调用）同样不挡 readiness。
func TestLLMSignalReportedButNeverGates(t *testing.T) {
	for _, state := range []LLMState{LLMUnknown, LLMOk, LLMDegraded} {
		h := NewHealthHandler(healthyPostgres()).WithLLMState(state)
		w := httptest.NewRecorder()
		newHealthRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ready", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("llm state %q must not gate readiness, got %d", state, w.Code)
		}
		if !strings.Contains(w.Body.String(), `"llm_provider":"`+string(state)+`"`) {
			t.Fatalf("llm state %q must appear in the dependency report: %s", state, w.Body.String())
		}
	}
}

// 变异自检锚（人工执行）：把 Ready 改回恒 200，本文件前两条测试必须红。

// ── 复验缺陷回归（RT1-L3-B review）：defaultLLMState 的 unknown 曾不可达 ────
// 根因：LastCallSignal 用 time.Unix(0, nanos) 构造，未调用时返回 1970-01-01
// 而非零值时间，IsZero 恒假 → 无任何调用也报 ok。刚部署或 provider 配错一次
// 都没通过的服务在健康面上谎报——恰在最需要真话的时刻说假话。修在源头
// （nanos==0 → 真零值），本测试直接调 defaultLLMState 钉住 unknown 可达；
// 变异自检：去掉 signal.go 的零值转换，本条必须变红。
func TestDefaultLLMStateUnknownBeforeAnyRealCall(t *testing.T) {
	llm.ResetCallSignal()
	if got := defaultLLMState(); got != LLMUnknown {
		t.Fatalf("no real call since boot must report unknown — a fresh deployment must not claim llm_provider ok, got %q", got)
	}
}
