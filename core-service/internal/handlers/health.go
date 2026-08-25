package handlers

// RT1-L3-B: honest health surface. The previous /health answered 200 with
// "status":"ok" even when the database probe failed and pasted the raw error
// into the body — a control that existed but could never be false (risk red
// line 12, the runtime sibling of the 2026-08-23 anydoc SRI guard).
//
// Contract:
//   - GET /live  — liveness. The process answers; no dependency is consulted.
//     Restarting the process cannot heal postgres, so liveness must not fail
//     on dependency outages.
//   - GET /ready — readiness. Gating dependencies (postgres, minio) are
//     probed; any down → 503 with "status":"unavailable". MinIO absent by
//     configuration is reported as not_configured and does NOT gate.
//   - GET /health — kept for existing callers as an alias of readiness.
//
// Public bodies carry dependency names and states only; underlying error text
// goes to the server log, never the response (probes carry no credentials).
// LLM provider state comes from llm.LastCallSignal (real-call outcomes; the
// spec forbids active provider probing — paid calls). It is reported
// informationally and never gates readiness.

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/llm"
)

type LLMState string

const (
	LLMUnknown  LLMState = "unknown"  // no real call since process start
	LLMOk       LLMState = "ok"       // last real call succeeded
	LLMDegraded LLMState = "degraded" // last real call failed
)

const probeTimeout = 5 * time.Second

type HealthHandler struct {
	postgres func(ctx context.Context) error // gating
	minio    func(ctx context.Context) error // gating when configured; nil = not configured
	llm      func() LLMState                 // informational only
}

// NewHealthHandler builds the handler around the postgres probe (the only
// mandatory gating dependency).
func NewHealthHandler(postgres func(ctx context.Context) error) *HealthHandler {
	return &HealthHandler{
		postgres: postgres,
		llm:      defaultLLMState,
	}
}

// WithMinioProbe wires the MinIO probe. A nil probe means "not configured in
// this deployment" and is reported honestly instead of guessed.
func (h *HealthHandler) WithMinioProbe(probe func(ctx context.Context) error) *HealthHandler {
	h.minio = probe
	return h
}

// WithLLMState overrides the llm signal reader (tests; production uses the
// default built from llm.LastCallSignal).
func (h *HealthHandler) WithLLMState(state LLMState) *HealthHandler {
	h.llm = func() LLMState { return state }
	return h
}

func defaultLLMState() LLMState {
	signal := llm.LastCallSignal()
	switch {
	case signal.LastFailure.After(signal.LastSuccess):
		return LLMDegraded
	case !signal.LastSuccess.IsZero():
		return LLMOk
	default:
		return LLMUnknown
	}
}

func (h *HealthHandler) probe(ctx context.Context, probe func(context.Context) error) (state string, err error) {
	if probe == nil {
		return "not_configured", nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	if err := probe(probeCtx); err != nil {
		return "down", err
	}
	return "ok", nil
}

// Live reports process liveness. Deliberately dependency-blind: restarting
// the pod cannot heal an external database, so failing live on postgres would
// turn a dependency outage into self-inflicted restart loops.
func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "core-service"})
}

// Ready gates traffic-facing readiness on the infrastructure dependencies.
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx := c.Request.Context()

	pgState, pgErr := h.probe(ctx, h.postgres)
	minioState, minioErr := h.probe(ctx, h.minio) // nil probe → not_configured, no error

	deps := gin.H{
		"postgres":     pgState,
		"minio":        minioState,
		"llm_provider": string(h.llm()),
	}
	status := http.StatusOK
	overall := "ok"
	// Gating failures reach the log via c.Error; the response body carries only
	// the per-dependency state words.
	if pgErr != nil {
		status = http.StatusServiceUnavailable
		overall = "unavailable"
		c.Error(pgErr)
	}
	if minioErr != nil {
		status = http.StatusServiceUnavailable
		overall = "unavailable"
		c.Error(minioErr)
	}

	c.JSON(status, gin.H{
		"status":       overall,
		"service":      "core-service",
		"version":      "0.1.0",
		"dependencies": deps,
	})
}
