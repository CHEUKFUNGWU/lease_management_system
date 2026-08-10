package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

func (h *AgentGatewayHandler) ListTerminalAlerts(c *gin.Context) {
	if h == nil || h.alerts == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent terminal alert store unavailable"})
		return
	}
	ctx, status, err := h.gatewayContext(c, "agent-terminal-alerts", "")
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	execution, ok := agenttools.ExecutionContextFromContext(ctx)
	if !ok || (!execution.Principal.HasPermission(agenttools.Permission{Resource: "ai_chat", Action: "use"}) &&
		!execution.Principal.HasPermission(agenttools.Permission{Resource: "audit_logs", Action: "read"})) {
		c.JSON(http.StatusForbidden, gin.H{"error": "agent terminal alert permission is required"})
		return
	}
	limit := 100
	if value, parseErr := strconv.Atoi(c.Query("limit")); parseErr == nil && value > 0 {
		limit = value
	}
	alerts, err := h.alerts.ListTerminalAlerts(ctx, execution.Principal.UserID, strings.TrimSpace(c.Query("status")), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load agent terminal alerts"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"alerts": alerts})
}

func (h *AgentGatewayHandler) AcknowledgeTerminalAlert(c *gin.Context) {
	if h == nil || h.alerts == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent terminal alert store unavailable"})
		return
	}
	ctx, status, err := h.gatewayContext(c, "agent-terminal-alert-ack", "")
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	execution, ok := agenttools.ExecutionContextFromContext(ctx)
	if !ok || (!execution.Principal.HasPermission(agenttools.Permission{Resource: "ai_chat", Action: "use"}) &&
		!execution.Principal.HasPermission(agenttools.Permission{Resource: "audit_logs", Action: "read"})) {
		c.JSON(http.StatusForbidden, gin.H{"error": "agent terminal alert permission is required"})
		return
	}
	alertID := strings.TrimSpace(c.Param("id"))
	if alertID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "alert ID is required"})
		return
	}
	if err := h.alerts.AcknowledgeTerminalAlert(ctx, alertID, execution.Principal.UserID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent terminal alert not found"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"acknowledged": true, "alert_id": alertID})
}
