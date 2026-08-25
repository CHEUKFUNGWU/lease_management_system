package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/aichat"
	"github.com/lease-management-system/core-service/internal/errcontract"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/draftapp"
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

type RetryAIChatDraftBatchRequest struct {
	ArtifactID    string                 `json:"artifact_id" binding:"required"`
	ActionPayload map[string]interface{} `json:"action_payload,omitempty"`
	Comment       string                 `json:"comment,omitempty"`
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

// writeSessionAccessError maps a session-load refusal: cross-entity / wrong
// owner stays 403 with the scope_denied code preserved (never softened to
// 404); a genuinely absent row stays 404.
func writeSessionAccessError(c *gin.Context, err error) {
	if errcontract.CodeOf(err) == errcontract.CodeScopeDenied {
		writeCodedError(c, http.StatusForbidden, errcontract.CodeScopeDenied,
			errcontract.SafeMessage(err), nil)
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "ai chat session not found"})
}

// writeRunAccessError maps a run/artifact/message/action-load refusal the
// same way: scope_denied stays 403 with the code preserved, never softened
// into 404 (SI2 直读路径)。
func writeRunAccessError(c *gin.Context, err error) {
	if errcontract.CodeOf(err) == errcontract.CodeScopeDenied {
		writeCodedError(c, http.StatusForbidden, errcontract.CodeScopeDenied,
			errcontract.SafeMessage(err), nil)
		return
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "ai chat run not found"})
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
	entity, ok := tenantEntity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}

	// CHAT-001: the user-facing session list defaults to user-initiated
	// sessions only. System-initiated sessions (home brief auto-runs) stay
	// fully recorded for audit but do not crowd the sidebar; pass
	// ?include_system=true to see them.
	excludeInitiator := "system"
	if c.Query("include_system") == "true" {
		excludeInitiator = ""
	}

	sessions, err := h.runtimeRepo.ListSessions(c.Request.Context(), repository.AIChatSessionFilter{
		UserID:           userIDStr,
		Entity:           entity,
		Status:           status,
		ExcludeInitiator: excludeInitiator,
		Limit:            limit,
		Offset:           offset,
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

	entity, ok := tenantEntity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}
	session, err := h.runtimeRepo.GetSessionByID(c.Request.Context(), sessionID, userIDStr, entity)
	if err != nil {
		writeSessionAccessError(c, err)
		return
	}

	messages, err := h.runtimeRepo.ListMessagesBySession(c.Request.Context(), sessionID, userIDStr, entity, 100)
	if err != nil {
		writeSessionAccessError(c, err)
		return
	}
	runs, err := h.runtimeRepo.ListRunsBySession(c.Request.Context(), sessionID, userIDStr, entity, 50, 0)
	if err != nil {
		writeSessionAccessError(c, err)
		return
	}
	artifacts, err := h.runtimeRepo.ListArtifactsBySession(c.Request.Context(), sessionID, userIDStr, entity, 100)
	if err != nil {
		writeSessionAccessError(c, err)
		return
	}
	reviewActions, err := h.runtimeRepo.ListReviewActionsBySession(c.Request.Context(), sessionID, userIDStr, entity, 200)
	if err != nil {
		writeSessionAccessError(c, err)
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

	entity, ok := tenantEntity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}
	_, err := h.runtimeRepo.GetSessionByID(c.Request.Context(), sessionID, userIDStr, entity)
	if err != nil {
		writeSessionAccessError(c, err)
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

	entity, ok := tenantEntity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}
	if _, err := h.runtimeRepo.GetSessionByID(c.Request.Context(), sessionID, userIDStr, entity); err != nil {
		writeSessionAccessError(c, err)
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	runs, err := h.runtimeRepo.ListRunsBySession(c.Request.Context(), sessionID, userIDStr, entity, limit, offset)
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
	entity, ok := tenantEntity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}

	run, err := h.runtimeRepo.GetRunByID(c.Request.Context(), runID, userIDStr, entity)
	if err != nil {
		writeRunAccessError(c, err)
		return
	}

	afterSequence, _ := strconv.Atoi(c.DefaultQuery("after_sequence", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	events, err := h.runtimeRepo.ListRunEvents(c.Request.Context(), runID, afterSequence, limit, entity, userIDStr)
	if err != nil {
		writeRunAccessError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"run":    run,
		"events": events,
	})
}

func (h *AIChatHandler) StreamRunEvents(c *gin.Context) {
	runID := c.Param("id")
	workerID, leaseToken, isWorker, workerErr := workerLeaseHeaders(c)
	if workerErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": workerErr.Error()})
		return
	}
	var run *repository.AIChatRun
	var err error
	var listEvents func(context.Context, int, int) ([]*repository.AIChatRunEvent, error)
	if isWorker {
		permissionsValue, _ := c.Get("permissions")
		permissions, _ := permissionsValue.([]string)
		if !middleware.HasPermission(permissions, "agent_runtime", "worker") && !middleware.HasPermission(permissions, "*", "*") {
			c.JSON(http.StatusForbidden, gin.H{"error": "agent worker permission is required"})
			return
		}
		if h == nil || h.workerRuns == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent worker run store unavailable"})
			return
		}
		run, err = h.workerRuns.GetClaimedRun(c.Request.Context(), runID, workerID, leaseToken)
		listEvents = func(ctx context.Context, after, limit int) ([]*repository.AIChatRunEvent, error) {
			return h.workerRuns.ListClaimedRunEvents(ctx, runID, workerID, leaseToken, after, limit)
		}
	} else {
		userID, ok := c.Get("user_id")
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
			return
		}
		userIDStr, _ := userID.(string)
		// SI2: 建流前做边界检查（chat 平面）。RT1-C 订正登记：
		// （1）存在一个受连接寿命限制的暴露窗口——listEvents 闭包在建流那一刻
		// 捕获 entity，scope 中途被收回的用户会继续收到本会话的事件直到重连。
		// 方向安全（不会跨到别人的法人，窗口只覆盖已建流会话自身的后续事件）、
		// 窗口有界，但把它说成「不构成问题」是在断言一件没被证明的事——收窄
		// 需单独提案，不在此顺手改。
		// （2）listEvents 每帧都把 entity 传给 ListRunEvents——是每帧都校验，
		// 不是「每帧不重复检查」。
		entity, entityOK := tenantEntity(c)
		if !entityOK {
			c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
			return
		}
		run, err = h.runtimeRepo.GetRunByID(c.Request.Context(), runID, userIDStr, entity)
		if err != nil {
			writeRunAccessError(c, err)
			return
		}
		listEvents = func(ctx context.Context, after, limit int) ([]*repository.AIChatRunEvent, error) {
			return h.runtimeRepo.ListRunEvents(ctx, runID, after, limit, entity, userIDStr)
		}
	}
	if err != nil || run == nil {
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
		events, err := listEvents(c.Request.Context(), afterSequence, 200)
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
	permissionValue, _ := c.Get("permissions")
	permissions, _ := permissionValue.([]string)
	started, err := h.agentRuntime.Continue(c.Request.Context(), aichat.ContinueCommand{
		Target:      aichat.Target{Type: req.Target.Type, ID: req.Target.ID},
		Instruction: req.Instruction, ContractID: req.ContractID,
		Language: req.Language, PageContext: req.PageContext,
		UserID: userIDStr, LegalEntityID: middleware.GetTenantID(c),
		Role: roleString, Permissions: append([]string(nil), permissions...), AuthHeader: c.GetHeader("Authorization"),
	})
	if err != nil {
		if errcontract.CodeOf(err) == errcontract.CodeScopeDenied {
			writeCodedError(c, http.StatusForbidden, errcontract.CodeScopeDenied,
				errcontract.SafeMessage(err), nil)
			return
		}
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
	// SI2 写路径：审批动作前必须确认 artifact 归属调用方的法人。预检先于
	// Review 的 Runtime 内检查，scope_denied 保持 403 不软化为 not_found。
	entity, entityOK := tenantEntity(c)
	if !entityOK {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}
	_, artifactErr := h.runtimeRepo.GetArtifactByID(c.Request.Context(), command.ArtifactID, userIDStr, entity)
	if artifactErr != nil {
		writeRunAccessError(c, artifactErr)
		return
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
	var draftResult *draftapp.BatchResult
	if committed, ok := result.CommitResult.(*draftapp.BatchResult); ok {
		draftResult = committed
	}
	response := gin.H{
		"action":   result.Action,
		"artifact": gin.H{"id": result.ArtifactID, "status": result.ArtifactStatus},
	}
	if draftResult != nil {
		response["draft_result"] = draftResult
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

// RetryDraftBatch replays only the selected items of an existing partial
// batch. The artifact remains the review/audit anchor; idempotency in the
// application service prevents already-created drafts from being duplicated.
func (h *AIChatHandler) RetryDraftBatch(c *gin.Context) {
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return
	}
	userIDStr, _ := userID.(string)
	permissionsValue, _ := c.Get("permissions")
	permissions, _ := permissionsValue.([]string)
	if !middleware.HasPermission(permissions, "ai_drafts", "confirm") {
		c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions", "required": "ai_drafts:confirm"})
		return
	}
	var req RetryAIChatDraftBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	entity, entityOK := tenantEntity(c)
	if !entityOK {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}
	artifact, err := h.runtimeRepo.GetArtifactByID(c.Request.Context(), req.ArtifactID, userIDStr, entity)
	if err != nil {
		writeRunAccessError(c, err)
		return
	}
	if req.ActionPayload == nil {
		req.ActionPayload = map[string]interface{}{}
	}
	if batchID := strings.TrimSpace(c.Param("id")); batchID != "" {
		req.ActionPayload["batch_id"] = batchID
	}
	actionType := "import"
	if artifact.ArtifactType == "contract_draft" {
		actionType = "create_draft"
	}
	if artifact.ArtifactType != "contract_draft" && artifact.ArtifactType != "payment_schedule_draft" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "artifact is not a retryable draft"})
		return
	}
	reviewResult, err := h.agentRuntime.Review(c.Request.Context(), aichat.ReviewCommand{
		ArtifactID: artifact.ID, ActionType: actionType, ActionPayload: req.ActionPayload,
		Comment: req.Comment, UserID: userIDStr,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record AI draft retry: " + err.Error()})
		return
	}
	var draftResult *draftapp.BatchResult
	if committed, ok := reviewResult.CommitResult.(*draftapp.BatchResult); ok {
		draftResult = committed
	}
	c.JSON(http.StatusCreated, gin.H{
		"action": reviewResult.Action, "artifact": gin.H{"id": reviewResult.ArtifactID, "status": reviewResult.ArtifactStatus},
		"draft_result": draftResult,
	})
}

type aiChatRunEventWriter interface {
	GetNextRunEventSequence(context.Context, string) (int, error)
	AppendRunEvent(context.Context, *repository.AIChatRunEvent) error
}

func (h *AIChatHandler) appendDraftBatchEvent(ctx context.Context, artifact *repository.AIChatArtifact, action string, result *draftapp.BatchResult) error {
	if h == nil || artifact == nil || result == nil {
		return nil
	}
	writer, ok := h.runtimeRepo.(aiChatRunEventWriter)
	if !ok || strings.TrimSpace(artifact.RunID) == "" {
		return nil
	}
	return appendDraftBatchEventToWriter(ctx, writer, artifact, action, result)
}

func appendDraftBatchEventToWriter(ctx context.Context, writer aiChatRunEventWriter, artifact *repository.AIChatArtifact, action string, result *draftapp.BatchResult) error {
	if writer == nil || artifact == nil || result == nil {
		return nil
	}
	sequence, err := writer.GetNextRunEventSequence(ctx, artifact.RunID)
	if err != nil {
		return err
	}
	payload := map[string]interface{}{
		"action": action, "artifact_id": artifact.ID, "batch_id": result.BatchID,
		"operation": result.Operation, "status": result.Status,
		"created_count": result.CreatedCount, "replayed_count": result.ReplayedCount,
		"failed_count": result.FailedCount,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return writer.AppendRunEvent(ctx, &repository.AIChatRunEvent{
		RunID: artifact.RunID, SessionID: artifact.SessionID, SequenceNo: sequence,
		EventType: "draft_batch", Payload: raw, IsTerminal: false,
	})
}
