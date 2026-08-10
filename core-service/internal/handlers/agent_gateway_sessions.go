package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
)

type createAgentSessionRequest struct {
	Title           string          `json:"title,omitempty"`
	BoundContractID string          `json:"bound_contract_id,omitempty"`
	ContextSnapshot json.RawMessage `json:"context_snapshot,omitempty"`
}

func (h *AgentGatewayHandler) CreateSession(c *gin.Context) {
	if h == nil || h.sessions == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent session store unavailable"})
		return
	}
	var request createAgentSessionRequest
	if err := decodeStrictJSON(c, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent session request"})
		return
	}
	ctx, status, err := h.gatewayContext(c, "agent-session-create", "agent-session-create")
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	execution, ok := agenttools.ExecutionContextFromContext(ctx)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "authenticated agent context is required"})
		return
	}
	if len(request.ContextSnapshot) == 0 {
		request.ContextSnapshot = json.RawMessage("null")
	}
	if contractID := strings.TrimSpace(request.BoundContractID); contractID != "" {
		if h.contracts == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent contract scope reader unavailable"})
			return
		}
		attributes, found, err := h.contracts.GetContractAttributes(ctx, contractID)
		if err != nil || !found || !execution.Principal.Scope.AllowsContract(attributes) {
			c.JSON(http.StatusNotFound, gin.H{"error": "bound contract not found in the authenticated scope"})
			return
		}
	}
	session := &repository.AIChatSession{
		UserID: execution.Principal.UserID, Title: strings.TrimSpace(request.Title),
		BoundContractID: optionalAgentString(request.BoundContractID),
		ContextSnapshot: request.ContextSnapshot,
	}
	if legalEntityID := strings.TrimSpace(execution.Principal.Scope.LegalEntityID); legalEntityID != "" {
		session.LegalEntityID = &legalEntityID
	}
	if err := h.sessions.CreateSession(ctx, session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create agent session"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"session": session})
}

func optionalAgentString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
