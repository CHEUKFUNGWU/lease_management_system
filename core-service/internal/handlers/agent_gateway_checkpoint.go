package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

type agentRunCheckpointRequest struct {
	Checkpoint json.RawMessage `json:"checkpoint"`
}

// SaveRunCheckpoint persists a Runner checkpoint only against an owned Core
// Run. The Gateway never interprets the plan or tool results as permission;
// they are resume metadata and every resumed ToolCall is re-authorized.
func (h *AgentGatewayHandler) SaveRunCheckpoint(c *gin.Context) {
	if h == nil || h.checkpoints == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent checkpoint store unavailable"})
		return
	}
	run, ctx, _, ok := h.loadRun(c, true)
	if !ok {
		return
	}
	var request agentRunCheckpointRequest
	if err := decodeStrictJSON(c, &request); err != nil || len(request.Checkpoint) == 0 || !json.Valid(request.Checkpoint) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "checkpoint must be valid JSON"})
		return
	}
	if len(request.Checkpoint) > 4<<20 {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "checkpoint exceeds 4 MiB"})
		return
	}
	var metadata struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(request.Checkpoint, &metadata); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "checkpoint must be a JSON object"})
		return
	}
	if metadata.RunID != "" && strings.TrimSpace(metadata.RunID) != run.ID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "checkpoint run_id does not match route"})
		return
	}
	workerID, leaseToken, isWorker, workerErr := workerLeaseHeaders(c)
	if workerErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": workerErr.Error()})
		return
	}
	if isWorker {
		if err := h.workerRuns.SaveClaimedRunCheckpoint(ctx, run.ID, workerID, leaseToken, request.Checkpoint); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save agent checkpoint"})
			return
		}
	} else if execution, exists := agenttools.ExecutionContextFromContext(ctx); !exists || execution.Principal.UserID == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "authenticated agent context is required"})
		return
	} else if err := h.checkpoints.SaveRunCheckpoint(ctx, run.ID, execution.Principal.UserID, request.Checkpoint); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save agent checkpoint"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"saved": true, "run_id": run.ID})
}

func (h *AgentGatewayHandler) GetRunCheckpoint(c *gin.Context) {
	if h == nil || h.checkpoints == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent checkpoint store unavailable"})
		return
	}
	_, ctx, _, ok := h.loadRun(c, true)
	if !ok {
		return
	}
	runID := strings.TrimSpace(c.Param("id"))
	workerID, leaseToken, isWorker, workerErr := workerLeaseHeaders(c)
	if workerErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": workerErr.Error()})
		return
	}
	var checkpoint json.RawMessage
	var err error
	if isWorker {
		checkpoint, err = h.workerRuns.GetClaimedRunCheckpoint(ctx, runID, workerID, leaseToken)
	} else {
		execution, exists := agenttools.ExecutionContextFromContext(ctx)
		if !exists || execution.Principal.UserID == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "authenticated agent context is required"})
			return
		}
		checkpoint, err = h.checkpoints.GetRunCheckpoint(ctx, runID, execution.Principal.UserID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load agent checkpoint"})
		return
	}
	if len(checkpoint) == 0 || string(checkpoint) == "null" {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent checkpoint not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"run_id": runID, "checkpoint": json.RawMessage(checkpoint)})
}
