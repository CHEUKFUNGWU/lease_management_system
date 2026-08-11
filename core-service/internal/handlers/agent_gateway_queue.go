package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
)

type agentRunClaimRequest struct {
	WorkerID     string `json:"worker_id"`
	LeaseSeconds int    `json:"lease_seconds,omitempty"`
}

type agentRunLeaseRequest struct {
	WorkerID     string `json:"worker_id"`
	LeaseToken   string `json:"lease_token"`
	LeaseSeconds int    `json:"lease_seconds,omitempty"`
	Requeue      bool   `json:"requeue,omitempty"`
}

func (h *AgentGatewayHandler) ClaimRun(c *gin.Context) {
	if h == nil || h.queue == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent run queue unavailable"})
		return
	}
	ctx, status, err := h.workerContext(c, "agent-run-claim")
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	var request agentRunClaimRequest
	if err := decodeStrictJSON(c, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid worker claim request"})
		return
	}
	request.WorkerID = strings.TrimSpace(request.WorkerID)
	if request.WorkerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "worker_id is required"})
		return
	}
	duration := workerLeaseDuration(request.LeaseSeconds)
	run, leaseToken, err := h.queue.ClaimQueuedRun(ctx, request.WorkerID, duration)
	if err == repository.ErrNoQueuedAgentRun {
		c.JSON(http.StatusNoContent, nil)
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to claim agent run"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"run": run, "lease_token": leaseToken, "worker_id": request.WorkerID,
		"lease_seconds": int(duration / time.Second),
	})
}

func (h *AgentGatewayHandler) HeartbeatRunLease(c *gin.Context) {
	if h == nil || h.queue == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent run queue unavailable"})
		return
	}
	ctx, status, err := h.workerContext(c, "agent-run-heartbeat")
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	var request agentRunLeaseRequest
	if err := decodeStrictJSON(c, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid worker heartbeat request"})
		return
	}
	if strings.TrimSpace(request.WorkerID) == "" || strings.TrimSpace(request.LeaseToken) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "worker_id and lease_token are required"})
		return
	}
	if err := h.queue.HeartbeatRunLease(ctx, c.Param("id"), request.WorkerID, request.LeaseToken, workerLeaseDuration(request.LeaseSeconds)); err != nil {
		if err == repository.ErrAgentRunLeaseLost {
			c.JSON(http.StatusConflict, gin.H{"error": "agent run lease is no longer owned by this worker"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to heartbeat agent run lease"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"accepted": true, "run_id": c.Param("id")})
}

func (h *AgentGatewayHandler) ReleaseRunLease(c *gin.Context) {
	if h == nil || h.queue == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent run queue unavailable"})
		return
	}
	ctx, status, err := h.workerContext(c, "agent-run-release")
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	var request agentRunLeaseRequest
	if err := decodeStrictJSON(c, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid worker release request"})
		return
	}
	if strings.TrimSpace(request.WorkerID) == "" || strings.TrimSpace(request.LeaseToken) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "worker_id and lease_token are required"})
		return
	}
	if err := h.queue.ReleaseRunLease(ctx, c.Param("id"), request.WorkerID, request.LeaseToken, request.Requeue); err != nil {
		if err == repository.ErrAgentRunLeaseLost {
			c.JSON(http.StatusConflict, gin.H{"error": "agent run lease is no longer owned by this worker"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to release agent run lease"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"accepted": true, "run_id": c.Param("id"), "requeued": request.Requeue})
}

func (h *AgentGatewayHandler) RecoverRunLeases(c *gin.Context) {
	if h == nil || h.queue == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent run queue unavailable"})
		return
	}
	ctx, status, err := h.workerContext(c, "agent-run-recover")
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	count, err := h.queue.RecoverExpiredRunLeases(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to recover agent run leases"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"recovered": count})
}

func (h *AgentGatewayHandler) workerContext(c *gin.Context, traceID string) (context.Context, int, error) {
	ctx, status, err := h.gatewayContext(c, "agent-worker", traceID)
	if err != nil {
		return nil, status, err
	}
	execution, ok := agenttools.ExecutionContextFromContext(ctx)
	if !ok || (!middleware.HasPermission(execution.Principal.Permissions, "agent_runtime", "worker") &&
		!middleware.HasPermission(execution.Principal.Permissions, "*", "*")) {
		return nil, http.StatusForbidden, errors.New("agent worker permission is required")
	}
	return ctx, http.StatusOK, nil
}

func workerLeaseDuration(seconds int) time.Duration {
	if seconds <= 0 {
		seconds = 60
	}
	if seconds > 3600 {
		seconds = 3600
	}
	return time.Duration(seconds) * time.Second
}
