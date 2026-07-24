package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/aichat"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
)

type CreateAIChatSessionRequest struct {
	Title           string                 `json:"title"`
	BoundContractID string                 `json:"bound_contract_id"`
	ContextSnapshot map[string]interface{} `json:"context_snapshot"`
}

type CreateAIChatRunRequest struct {
	Message      string        `json:"message" binding:"required"`
	ParentRunID  string        `json:"parent_run_id,omitempty"`
	ContractID   string        `json:"contract_id,omitempty"`
	History      []ChatMessage `json:"history,omitempty"`
	FileID       string        `json:"file_id,omitempty"`
	ObjectName   string        `json:"object_name,omitempty"`
	ContentType  string        `json:"content_type,omitempty"`
	Language     string        `json:"language,omitempty"`
	PageContext  *PageContext  `json:"page_context,omitempty"`
	AgentMode    *bool         `json:"agent_mode,omitempty"`
	SkillID      string        `json:"skill_id,omitempty"`
	SkillVersion string        `json:"skill_version,omitempty"`
}

type CreateAIChatReviewActionRequest struct {
	ActionType    string                 `json:"action_type" binding:"required"`
	ActionPayload map[string]interface{} `json:"action_payload,omitempty"`
	Comment       string                 `json:"comment,omitempty"`
	FollowUp      *AIChatFollowUpRequest `json:"follow_up,omitempty"`
}

func aiReviewActionPermission(actionType string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(actionType)) {
	case "confirm", "import", "create_draft":
		return "confirm", true
	case "reject", "skip":
		return "review", true
	default:
		return "", false
	}
}

type AIChatContinuationTarget struct {
	Type string `json:"type" binding:"required"`
	ID   string `json:"id" binding:"required"`
}

type CreateAIChatContinuationRequest struct {
	Target      AIChatContinuationTarget `json:"target" binding:"required"`
	Instruction string                   `json:"instruction,omitempty"`
	ContractID  string                   `json:"contract_id,omitempty"`
	Language    string                   `json:"language,omitempty"`
	PageContext *PageContext             `json:"page_context,omitempty"`
}

type AIChatFollowUpRequest struct {
	Message     string       `json:"message,omitempty"`
	ContractID  string       `json:"contract_id,omitempty"`
	Language    string       `json:"language,omitempty"`
	PageContext *PageContext `json:"page_context,omitempty"`
}

func reverseMessages(messages []*repository.AIChatMessage) []*repository.AIChatMessage {
	reversed := append([]*repository.AIChatMessage(nil), messages...)
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	return reversed
}

func writeSSEEvent(c *gin.Context, event string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(c.Writer, "event: %s\n", event); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(c.Writer, "data: %s\n\n", data); err != nil {
		return err
	}
	c.Writer.Flush()
	return nil
}

func (h *AIChatHandler) CreateSession(c *gin.Context) {
	var req CreateAIChatSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return
	}
	userIDStr, _ := userID.(string)
	legalEntityID := middleware.GetTenantID(c)

	session, err := h.agentRuntime.OpenSession(c.Request.Context(), aichat.SessionCommand{
		UserID: userIDStr, LegalEntityID: legalEntityID, Title: req.Title,
		BoundContractID: req.BoundContractID, ContextSnapshot: req.ContextSnapshot,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create ai chat session: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"session": session})
}

func (h *AIChatHandler) ListSessions(c *gin.Context) {
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return
	}
	userIDStr, _ := userID.(string)

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	status := c.Query("status")
	legalEntityID := c.Query("legal_entity_id")
	if legalEntityID == "" {
		legalEntityID = middleware.GetTenantID(c)
	}

	sessions, err := h.runtimeRepo.ListSessions(c.Request.Context(), repository.AIChatSessionFilter{
		UserID:        userIDStr,
		LegalEntityID: legalEntityID,
		Status:        status,
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list ai chat sessions: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

func (h *AIChatHandler) GetSession(c *gin.Context) {
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return
	}
	userIDStr, _ := userID.(string)
	sessionID := c.Param("id")

	session, err := h.runtimeRepo.GetSessionByID(c.Request.Context(), sessionID, userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ai chat session not found"})
		return
	}

	messages, err := h.runtimeRepo.ListMessagesBySession(c.Request.Context(), sessionID, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list ai chat messages: " + err.Error()})
		return
	}
	runs, err := h.runtimeRepo.ListRunsBySession(c.Request.Context(), sessionID, 50, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list ai chat runs: " + err.Error()})
		return
	}
	artifacts, err := h.runtimeRepo.ListArtifactsBySession(c.Request.Context(), sessionID, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list ai chat artifacts: " + err.Error()})
		return
	}
	reviewActions, err := h.runtimeRepo.ListReviewActionsBySession(c.Request.Context(), sessionID, 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list ai chat review actions: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session":        session,
		"messages":       reverseMessages(messages),
		"runs":           runs,
		"artifacts":      artifacts,
		"review_actions": reviewActions,
	})
}

func (h *AIChatHandler) CreateRun(c *gin.Context) {
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return
	}
	userIDStr, _ := userID.(string)
	sessionID := c.Param("id")

	_, err := h.runtimeRepo.GetSessionByID(c.Request.Context(), sessionID, userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ai chat session not found"})
		return
	}

	var req CreateAIChatRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Language == "" {
		req.Language = "zh-CN"
	}

	input := runtimeInput(c, AIChatRequest{
		SessionID: sessionID, Message: req.Message, ContractID: req.ContractID,
		History: req.History, FileID: req.FileID, ObjectName: req.ObjectName,
		ContentType: req.ContentType, PageContext: req.PageContext, Language: req.Language,
	})
	input.ParentRunID = req.ParentRunID
	input.AgentMode = req.AgentMode
	input.SkillID = req.SkillID
	input.SkillVersion = req.SkillVersion
	started, err := h.agentRuntime.Start(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start AI agent run: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"run": started.Run, "trigger_message": started.TriggerMessage,
		"agent_plan": started.Plan.AgentPlan, "tool_calls": started.Plan.ToolCalls,
		"review_prompts": started.Plan.ReviewPrompts, "stream_url": started.StreamPath,
	})
}

func (h *AIChatHandler) ListRuns(c *gin.Context) {
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return
	}
	userIDStr, _ := userID.(string)
	sessionID := c.Param("id")

	if _, err := h.runtimeRepo.GetSessionByID(c.Request.Context(), sessionID, userIDStr); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ai chat session not found"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	runs, err := h.runtimeRepo.ListRunsBySession(c.Request.Context(), sessionID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list ai chat runs: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"runs": runs})
}

func (h *AIChatHandler) ListRunEvents(c *gin.Context) {
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return
	}
	userIDStr, _ := userID.(string)
	runID := c.Param("id")

	run, err := h.runtimeRepo.GetRunByID(c.Request.Context(), runID, userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ai chat run not found"})
		return
	}

	afterSequence, _ := strconv.Atoi(c.DefaultQuery("after_sequence", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	events, err := h.runtimeRepo.ListRunEvents(c.Request.Context(), runID, afterSequence, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list ai chat run events: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"run":    run,
		"events": events,
	})
}

func (h *AIChatHandler) StreamRunEvents(c *gin.Context) {
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return
	}
	userIDStr, _ := userID.(string)
	runID := c.Param("id")

	run, err := h.runtimeRepo.GetRunByID(c.Request.Context(), runID, userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ai chat run not found"})
		return
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming unsupported"})
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	afterSequence, _ := strconv.Atoi(c.DefaultQuery("after_sequence", "0"))
	_ = writeSSEEvent(c, "run_meta", gin.H{
		"run": run,
	})
	flusher.Flush()

	ticker := time.NewTicker(750 * time.Millisecond)
	defer ticker.Stop()

	for {
		events, err := h.runtimeRepo.ListRunEvents(c.Request.Context(), runID, afterSequence, 200)
		if err != nil {
			_ = writeSSEEvent(c, "error", gin.H{"error": "failed to list ai chat run events"})
			return
		}

		terminalSeen := false
		for _, event := range events {
			afterSequence = event.SequenceNo
			payload := gin.H{
				"event": event,
			}
			if err := writeSSEEvent(c, "run_event", payload); err != nil {
				return
			}
			if event.IsTerminal {
				terminalSeen = true
			}
		}
		flusher.Flush()

		if terminalSeen {
			_ = writeSSEEvent(c, "complete", gin.H{"run_id": runID, "last_sequence": afterSequence})
			return
		}

		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
		}
	}
}

func (h *AIChatHandler) CreateContinuation(c *gin.Context) {
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return
	}
	userIDStr, _ := userID.(string)

	var req CreateAIChatContinuationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	role, _ := c.Get("role")
	roleString, _ := role.(string)
	started, err := h.agentRuntime.Continue(c.Request.Context(), aichat.ContinueCommand{
		Target:      aichat.Target{Type: req.Target.Type, ID: req.Target.ID},
		Instruction: req.Instruction, ContractID: req.ContractID,
		Language: req.Language, PageContext: req.PageContext,
		UserID: userIDStr, LegalEntityID: middleware.GetTenantID(c),
		Role: roleString, AuthHeader: c.GetHeader("Authorization"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create ai chat continuation: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"run": started.Run, "trigger_message": started.TriggerMessage,
		"agent_plan": started.Plan.AgentPlan, "tool_calls": started.Plan.ToolCalls,
		"review_prompts": started.Plan.ReviewPrompts, "stream_url": started.StreamPath,
		"continuation": started.Continuation,
	})
}

func (h *AIChatHandler) CreateReviewAction(c *gin.Context) {
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return
	}
	userIDStr, _ := userID.(string)
	var req CreateAIChatReviewActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	requiredAction, supported := aiReviewActionPermission(req.ActionType)
	if !supported {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported AI draft review action"})
		return
	}
	permissionValue, _ := c.Get("permissions")
	permissions, _ := permissionValue.([]string)
	if !middleware.HasPermission(permissions, "ai_drafts", requiredAction) {
		c.JSON(http.StatusForbidden, gin.H{
			"error":    "insufficient permissions",
			"required": "ai_drafts:" + requiredAction,
		})
		return
	}

	command := aichat.ReviewCommand{
		ArtifactID: c.Param("id"), ActionType: req.ActionType,
		ActionPayload: req.ActionPayload, Comment: req.Comment, UserID: userIDStr,
	}
	if req.FollowUp != nil {
		role, _ := c.Get("role")
		roleString, _ := role.(string)
		command.FollowUp = &aichat.ContinueCommand{
			Instruction: req.FollowUp.Message, ContractID: req.FollowUp.ContractID,
			Language: req.FollowUp.Language, PageContext: req.FollowUp.PageContext,
			UserID: userIDStr, LegalEntityID: middleware.GetTenantID(c),
			Role: roleString, AuthHeader: c.GetHeader("Authorization"),
		}
	}
	result, err := h.agentRuntime.Review(c.Request.Context(), command)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record ai chat review action: " + err.Error()})
		return
	}
	response := gin.H{
		"action":   result.Action,
		"artifact": gin.H{"id": result.ArtifactID, "status": result.ArtifactStatus},
	}
	if result.FollowUp != nil {
		response["run"] = result.FollowUp.Run
		response["trigger_message"] = result.FollowUp.TriggerMessage
		response["agent_plan"] = result.FollowUp.Plan.AgentPlan
		response["tool_calls"] = result.FollowUp.Plan.ToolCalls
		response["review_prompts"] = result.FollowUp.Plan.ReviewPrompts
		response["stream_url"] = result.FollowUp.StreamPath
		response["continuation"] = result.FollowUp.Continuation
	}
	c.JSON(http.StatusCreated, response)
}
