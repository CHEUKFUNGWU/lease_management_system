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

type AgentUsageReader interface {
	SummarizePlannerUsage(context.Context, repository.AgentUsageQuery) (*repository.AgentUsageSummary, error)
}

// Usage exposes persisted planner usage separately from the process-local
// Tool metrics. It returns no run IDs or business values and derives the
// tenant boundary from the authenticated Agent Gateway principal.
func (h *AgentGatewayHandler) Usage(c *gin.Context) {
	ctx, status, err := h.gatewayContext(c, "agent-usage", "")
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	if h == nil || h.usage == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent usage unavailable"})
		return
	}
	execution, ok := agenttools.ExecutionContextFromContext(ctx)
	if !ok || (!middleware.HasPermission(execution.Principal.Permissions, "agent_runtime", "metrics") &&
		!middleware.HasPermission(execution.Principal.Permissions, "audit_logs", "read") &&
		!middleware.HasPermission(execution.Principal.Permissions, "*", "*")) {
		c.JSON(http.StatusForbidden, gin.H{"error": "agent metrics permission is required"})
		return
	}
	from, to, err := usageTimeRange(c.Query("from"), c.Query("to"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	scope := execution.Principal.Scope
	summary, err := h.usage.SummarizePlannerUsage(ctx, repository.AgentUsageQuery{
		UserID: execution.Principal.UserID, LegalEntityID: scope.LegalEntityID,
		Global: scope.Global, From: from, To: to,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to summarize agent usage"})
		return
	}
	c.JSON(http.StatusOK, summary)
}

func usageTimeRange(fromValue, toValue string) (time.Time, time.Time, error) {
	now := time.Now().UTC()
	from := now.Add(-24 * time.Hour)
	to := now
	var err error
	if strings.TrimSpace(fromValue) != "" {
		from, err = time.Parse(time.RFC3339, strings.TrimSpace(fromValue))
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("from must be an RFC3339 timestamp")
		}
	}
	if strings.TrimSpace(toValue) != "" {
		to, err = time.Parse(time.RFC3339, strings.TrimSpace(toValue))
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("to must be an RFC3339 timestamp")
		}
	}
	if !to.After(from) {
		return time.Time{}, time.Time{}, errors.New("to must be after from")
	}
	if to.Sub(from) > 31*24*time.Hour {
		return time.Time{}, time.Time{}, errors.New("usage range cannot exceed 31 days")
	}
	return from, to, nil
}
