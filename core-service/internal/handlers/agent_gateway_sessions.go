package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/aichat"
	"github.com/lease-management-system/core-service/internal/errcontract"
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
	// RT1-B: 会话创建走 sessionmanager（与 chat 平面同一 AR2 权威）。gateway
	// 入口被 agentrunner 唯一调用（http_gateway.go 的 CreateSession），接好
	// 这里即覆盖 runner 平面。HoldLease=false：纯创建不持租约。
	if h.sessionOwner != nil {
		session, _, err := h.sessionOwner.ResolveSession(ctx, aichat.SessionIntent{
			UserID:          execution.Principal.UserID,
			Title:           strings.TrimSpace(request.Title),
			ContractID:      strings.TrimSpace(request.BoundContractID),
			ContextSnapshot: request.ContextSnapshot,
			HoldLease:       false,
		})
		if err != nil {
			if errcontract.CodeOf(err) == errcontract.CodeScopeDenied {
				writeCodedError(c, http.StatusForbidden, errcontract.CodeScopeDenied,
					errcontract.SafeMessage(err), nil)
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create agent session"})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"session": session})
		return
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
