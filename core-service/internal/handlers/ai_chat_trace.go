package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/errcontract"
	"github.com/lease-management-system/core-service/internal/repository"
)

// AgentRunTrace is the canonical read model shared by Web, CLI and external
// Runner consumers. Every collection is owner-filtered through the same Run
// lookup before it is returned.
type AgentRunTrace struct {
	Run           any `json:"run"`
	Summary       any `json:"summary,omitempty"`
	Events        any `json:"events"`
	Artifacts     any `json:"artifacts"`
	ReviewActions any `json:"review_actions"`
	ToolAudits    any `json:"tool_audits"`
	AuditLinks    any `json:"audit_links"`
	Checkpoints   any `json:"checkpoint_audits"`
	AuditTotal    int `json:"audit_total"`
}

func (h *AIChatHandler) GetAgentRunTrace(c *gin.Context) {
	if h == nil || h.runtimeRepo == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ai chat runtime unavailable"})
		return
	}
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return
	}
	userIDString, _ := userID.(string)
	runID := strings.TrimSpace(c.Param("id"))
	if runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "run ID is required"})
		return
	}
	entity, entityOK := tenantEntity(c)
	if !entityOK {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}
	run, err := h.runtimeRepo.GetRunByID(c.Request.Context(), runID, userIDString, entity)
	if err != nil || run == nil {
		if err != nil && errcontract.CodeOf(err) == errcontract.CodeScopeDenied {
			writeCodedError(c, http.StatusForbidden, errcontract.CodeScopeDenied,
				errcontract.SafeMessage(err), nil)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "agent run not found"})
		return
	}
	events, err := h.runtimeRepo.ListRunEvents(c.Request.Context(), runID, 0, 1000, entity, userIDString)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load agent run events"})
		return
	}
	artifacts, err := h.runtimeRepo.ListArtifactsBySession(c.Request.Context(), run.SessionID, userIDString, entity, 1000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load agent artifacts"})
		return
	}
	filteredArtifacts := make([]any, 0, len(artifacts))
	for _, artifact := range artifacts {
		if artifact != nil && artifact.RunID == runID {
			filteredArtifacts = append(filteredArtifacts, artifact)
		}
	}
	actions, err := h.runtimeRepo.ListReviewActionsBySession(c.Request.Context(), run.SessionID, userIDString, entity, 1000)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load review actions"})
		return
	}
	filteredActions := make([]any, 0, len(actions))
	for _, action := range actions {
		if action != nil && action.RunID != nil && *action.RunID == runID {
			filteredActions = append(filteredActions, action)
		}
	}

	var audits any = []any{}
	var summary any
	if summaryReader, ok := h.runtimeRepo.(AgentRunAuditSummaryReader); ok {
		loadedSummary, summaryErr := summaryReader.GetRunAuditSummary(c.Request.Context(), runID)
		if summaryErr == nil {
			summary = loadedSummary
		}
	}
	auditTotal := 0
	var auditLinks any = []any{}
	var checkpointAudits any = []any{}
	if linkReader, ok := h.runtimeRepo.(AgentRunAuditLinkReader); ok {
		if links, linkErr := linkReader.ListRunAuditLinks(c.Request.Context(), runID, userIDString); linkErr == nil {
			auditLinks = links
		}
		if audits, auditErr := linkReader.ListRunCheckpointAudits(c.Request.Context(), runID, userIDString); auditErr == nil {
			checkpointAudits = audits
		}
	}
	if h.auditRepo != nil {
		rows, total, auditErr := h.auditRepo.List(c.Request.Context(), repository.AuditLogFilter{
			RunID: runID, Limit: 1000, Offset: 0,
		})
		if auditErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load tool audit trace"})
			return
		}
		audits = rows
		auditTotal = total
	}

	c.JSON(http.StatusOK, AgentRunTrace{
		Run: run, Summary: summary, Events: events, Artifacts: filteredArtifacts, ReviewActions: filteredActions,
		ToolAudits: audits, AuditLinks: auditLinks, Checkpoints: checkpointAudits, AuditTotal: auditTotal,
	})
}
