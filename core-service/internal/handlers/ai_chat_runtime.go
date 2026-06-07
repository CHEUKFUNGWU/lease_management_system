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

type continuationSeed struct {
	session             *repository.AIChatSession
	parentRunID         string
	sourceRun           *repository.AIChatRun
	effectiveContractID string
	pageContext         *PageContext
	history             []ChatMessage
	instruction         string
	targetType          string
	targetID            string
	compressionStrategy string
}

func marshalOptionalJSON(v interface{}) json.RawMessage {
	if v == nil {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return data
}

func reverseMessages(messages []*repository.AIChatMessage) []*repository.AIChatMessage {
	reversed := append([]*repository.AIChatMessage(nil), messages...)
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	return reversed
}

func toAscendingChatHistory(messages []*repository.AIChatMessage) []ChatMessage {
	history := make([]ChatMessage, 0, len(messages))
	for _, message := range reverseMessages(messages) {
		if message.Role != "user" && message.Role != "assistant" {
			continue
		}
		history = append(history, ChatMessage{
			Role:    message.Role,
			Content: message.Content,
		})
	}
	return history
}

func buildCompressedHistory(messages []*repository.AIChatMessage, anchorMessageID string) ([]ChatMessage, string) {
	history := toAscendingChatHistory(messages)
	if len(history) == 0 {
		return nil, "empty"
	}
	if len(history) <= 12 {
		return history, "full_session"
	}
	if anchorMessageID == "" {
		return history[len(history)-12:], "session_tail"
	}

	anchorIndex := -1
	historyIndex := -1
	for _, message := range reverseMessages(messages) {
		if message.Role != "user" && message.Role != "assistant" {
			continue
		}
		historyIndex++
		if message.ID == anchorMessageID {
			anchorIndex = historyIndex
			break
		}
	}
	if anchorIndex == -1 {
		return history[len(history)-12:], "session_tail"
	}

	start := anchorIndex - 8
	if start < 0 {
		start = 0
	}
	end := anchorIndex + 4
	if end > len(history) {
		end = len(history)
	}
	if end-start < 12 {
		if end == len(history) {
			start = len(history) - 12
			if start < 0 {
				start = 0
			}
		} else if start == 0 {
			end = 12
			if end > len(history) {
				end = len(history)
			}
		}
	}
	return history[start:end], "anchor_window"
}

func parsePageContext(raw json.RawMessage) *PageContext {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var pageContext PageContext
	if err := json.Unmarshal(raw, &pageContext); err == nil {
		if pageContext.Page != "" || pageContext.Title != "" || pageContext.ContractID != "" || pageContext.Period != "" || pageContext.ReportView != "" || pageContext.Summary != "" || len(pageContext.Filters) > 0 {
			return &pageContext
		}
	}
	return nil
}

func truncateSummary(text string, limit int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "..."
}

func buildContinuationInstruction(targetType, summary string) string {
	base := "请基于当前会话上下文继续推进，先概括当前状态，再给出下一步建议；如果仍需人工确认，请明确指出。"
	if summary == "" {
		return base
	}
	return fmt.Sprintf("请承接刚才的%s继续推进。当前锚点信息：%s。%s", targetType, summary, base)
}

func summarizeRun(run *repository.AIChatRun) string {
	if run == nil {
		return ""
	}
	parts := []string{}
	if run.SkillID != nil && *run.SkillID != "" {
		parts = append(parts, fmt.Sprintf("skill=%s", *run.SkillID))
	}
	if run.Status != "" {
		parts = append(parts, fmt.Sprintf("status=%s", run.Status))
	}
	if run.SummaryText != nil && *run.SummaryText != "" {
		parts = append(parts, fmt.Sprintf("summary=%s", truncateSummary(*run.SummaryText, 120)))
	}
	return strings.Join(parts, ", ")
}

func summarizeArtifact(artifact *repository.AIChatArtifact) string {
	if artifact == nil {
		return ""
	}
	parts := []string{fmt.Sprintf("artifact_type=%s", artifact.ArtifactType)}
	if artifact.Title != "" {
		parts = append(parts, fmt.Sprintf("title=%s", artifact.Title))
	}
	if artifact.Status != "" {
		parts = append(parts, fmt.Sprintf("status=%s", artifact.Status))
	}
	var data map[string]interface{}
	if err := json.Unmarshal(artifact.Data, &data); err == nil {
		if contracts, ok := data["contracts"].([]interface{}); ok {
			parts = append(parts, fmt.Sprintf("contracts=%d", len(contracts)))
		}
		if schedules, ok := data["schedules"].([]interface{}); ok {
			parts = append(parts, fmt.Sprintf("schedules=%d", len(schedules)))
		}
	}
	return strings.Join(parts, ", ")
}

func summarizeReviewAction(action *repository.AIChatReviewAction, artifact *repository.AIChatArtifact) string {
	if action == nil {
		return ""
	}
	parts := []string{fmt.Sprintf("action=%s", action.ActionType)}
	if artifact != nil {
		parts = append(parts, summarizeArtifact(artifact))
	}
	if action.Comment != nil && *action.Comment != "" {
		parts = append(parts, fmt.Sprintf("comment=%s", truncateSummary(*action.Comment, 80)))
	}
	return strings.Join(parts, ", ")
}

func runbookForSkillID(skillID string, req AIChatRequest, effectiveContractID string) *AgentRunbook {
	hasFile := req.FileID != "" && req.ObjectName != ""
	hasContractContext := effectiveContractID != ""
	switch skillID {
	case "excel_ledger":
		return buildContractLedgerRunbook(hasFile)
	case "contract_review":
		return buildContractReviewRunbook(hasFile, hasContractContext)
	case "payment_schedule":
		return buildPaymentScheduleRunbook(hasFile, hasContractContext)
	case "audit_pack":
		return buildAuditPackRunbook()
	default:
		return nil
	}
}

func (h *AIChatHandler) buildContinuationRunbook(req AIChatRequest, effectiveContractID string, sourceRun *repository.AIChatRun) *AgentRunbook {
	runbook := h.buildAgentRunbook(req, effectiveContractID)
	if runbook != nil {
		return runbook
	}
	if sourceRun != nil && sourceRun.SkillID != nil {
		return runbookForSkillID(*sourceRun.SkillID, req, effectiveContractID)
	}
	return nil
}

func mergePageContext(primary, fallback *PageContext) *PageContext {
	if primary != nil {
		return primary
	}
	return fallback
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

	session := &repository.AIChatSession{
		UserID:          userIDStr,
		Title:           req.Title,
		ContextSnapshot: marshalOptionalJSON(req.ContextSnapshot),
	}
	if legalEntityID != "" {
		session.LegalEntityID = &legalEntityID
	}
	if req.BoundContractID != "" {
		session.BoundContractID = &req.BoundContractID
	}

	if err := h.runtimeRepo.CreateSession(c.Request.Context(), session); err != nil {
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

	session, err := h.runtimeRepo.GetSessionByID(c.Request.Context(), sessionID, userIDStr)
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

	effectiveContractID := req.ContractID
	if effectiveContractID == "" && session.BoundContractID != nil {
		effectiveContractID = *session.BoundContractID
	}
	runbook := h.buildAgentRunbook(AIChatRequest{
		Message:     req.Message,
		ContractID:  effectiveContractID,
		FileID:      req.FileID,
		ObjectName:  req.ObjectName,
		ContentType: req.ContentType,
		PageContext: req.PageContext,
	}, effectiveContractID)

	agentMode := runbook != nil
	if req.AgentMode != nil {
		agentMode = *req.AgentMode
	}

	run := &repository.AIChatRun{
		SessionID:      sessionID,
		ParentRunID:    nil,
		Status:         "queued",
		AgentMode:      agentMode,
		PageContext:    marshalOptionalJSON(req.PageContext),
		ReviewRequired: runbook != nil && len(runbook.ReviewPrompts) > 0,
		CreatedBy:      &userIDStr,
	}
	if req.ParentRunID != "" {
		run.ParentRunID = &req.ParentRunID
	}
	if req.SkillID != "" {
		run.SkillID = &req.SkillID
	} else if runbook != nil {
		run.SkillID = &runbook.SkillID
	}
	if req.SkillVersion != "" {
		run.SkillVersion = &req.SkillVersion
	}

	if err := h.runtimeRepo.CreateRun(c.Request.Context(), run); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create ai chat run: " + err.Error()})
		return
	}

	nextSeq, err := h.runtimeRepo.GetNextMessageSequence(c.Request.Context(), sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to allocate message sequence: " + err.Error()})
		return
	}

	userMessage := &repository.AIChatMessage{
		SessionID:   sessionID,
		RunID:       &run.ID,
		Role:        "user",
		MessageType: "text",
		SequenceNo:  nextSeq,
		Content:     req.Message,
		CreatedBy:   &userIDStr,
		Attachments: marshalOptionalJSON(buildRuntimeAttachmentRefs(&AIChatRequest{
			FileID:      req.FileID,
			ObjectName:  req.ObjectName,
			ContentType: req.ContentType,
		})),
	}
	if err := h.runtimeRepo.CreateMessage(c.Request.Context(), userMessage); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist ai chat message: " + err.Error()})
		return
	}
	if err := h.runtimeRepo.LinkRunTriggerMessage(c.Request.Context(), run.ID, userMessage.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to link trigger message: " + err.Error()})
		return
	}

	initialPayload := map[string]interface{}{
		"status":             run.Status,
		"trigger_message_id": userMessage.ID,
		"agent_mode":         run.AgentMode,
		"skill_id":           run.SkillID,
		"review_required":    run.ReviewRequired,
	}
	if err := h.runtimeRepo.AppendRunEvent(c.Request.Context(), &repository.AIChatRunEvent{
		RunID:      run.ID,
		SessionID:  sessionID,
		SequenceNo: 1,
		EventType:  "run_status",
		Payload:    marshalOptionalJSON(initialPayload),
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to append initial run event: " + err.Error()})
		return
	}

	if req.FileID != "" {
		attachment := &repository.AIChatAttachment{
			SessionID:      session.ID,
			MessageID:      &userMessage.ID,
			RunID:          &run.ID,
			FileID:         req.FileID,
			OriginalName:   req.ObjectName,
			ContentType:    req.ContentType,
			CreatedBy:      &userIDStr,
			MinioObjectKey: &req.ObjectName,
			ParseStatus:    "processing",
		}
		if err := h.runtimeRepo.CreateAttachment(c.Request.Context(), attachment); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist ai chat attachment: " + err.Error()})
			return
		}
	}

	now := time.Now()
	_ = h.runtimeRepo.UpdateRunStatus(c.Request.Context(), run.ID, "running", run.ReviewRequired, nil, nil, &now, nil)
	run.Status = "running"
	run.StartedAt = &now

	runtimeState := &aiChatRuntimeState{
		session:     session,
		run:         run,
		userMessage: userMessage,
		userID:      userIDStr,
	}
	role, _ := c.Get("role")
	roleStr, _ := role.(string)
	legalEntityID := middleware.GetTenantID(c)
	authHeader := c.GetHeader("Authorization")
	go h.executeRuntimeRun(runtimeState, AIChatRequest{
		SessionID:   session.ID,
		RunID:       run.ID,
		Message:     req.Message,
		ContractID:  effectiveContractID,
		History:     req.History,
		FileID:      req.FileID,
		ObjectName:  req.ObjectName,
		ContentType: req.ContentType,
		PageContext: req.PageContext,
		Language:    req.Language,
	}, legalEntityID, roleStr, authHeader, runbook)

	c.JSON(http.StatusCreated, gin.H{
		"run":             run,
		"trigger_message": userMessage,
		"agent_plan":      agentPlanFromRunbook(runbook),
		"tool_calls":      toolCallsFromRunbook(runbook),
		"review_prompts":  reviewPromptsFromRunbook(runbook),
		"stream_url":      fmt.Sprintf("/api/v1/ai/chat/runs/%s/stream", run.ID),
	})
}

func (h *AIChatHandler) executeRuntimeRun(state *aiChatRuntimeState, req AIChatRequest, legalEntityID, roleStr, authHeader string, runbook *AgentRunbook) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	h.executeChatRequest(ctx, authHeader, req, legalEntityID, state.userID, roleStr, state, runbook)
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

func artifactStatusForAction(actionType string) string {
	switch actionType {
	case "confirm", "import", "create_draft":
		return "confirmed"
	case "reject":
		return "rejected"
	case "skip":
		return "archived"
	default:
		return "draft"
	}
}

func contractIDFromPayload(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	if value, ok := payload["contract_id"].(string); ok {
		return value
	}
	return ""
}

func resolveContractID(explicit string, pageContext *PageContext, session *repository.AIChatSession, fallbacks ...string) string {
	if explicit != "" {
		return explicit
	}
	if pageContext != nil && pageContext.ContractID != "" {
		return pageContext.ContractID
	}
	if session != nil && session.BoundContractID != nil && *session.BoundContractID != "" {
		return *session.BoundContractID
	}
	for _, candidate := range fallbacks {
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func (h *AIChatHandler) resolveContinuationSeed(ctx context.Context, req *CreateAIChatContinuationRequest, userIDStr string) (*continuationSeed, error) {
	if req == nil {
		return nil, fmt.Errorf("missing continuation request")
	}
	targetType := strings.TrimSpace(strings.ToLower(req.Target.Type))
	targetID := strings.TrimSpace(req.Target.ID)
	if targetType == "" || targetID == "" {
		return nil, fmt.Errorf("continuation target is required")
	}

	if req.Language == "" {
		req.Language = "zh-CN"
	}

	var (
		session         *repository.AIChatSession
		sourceRun       *repository.AIChatRun
		parentRunID     string
		pageContext     *PageContext
		anchorMessageID string
		instruction     string
		fallbackIDs     []string
	)

	switch targetType {
	case "run":
		run, err := h.runtimeRepo.GetRunByID(ctx, targetID, userIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to load continuation run: %w", err)
		}
		session, err = h.runtimeRepo.GetSessionByID(ctx, run.SessionID, userIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to load continuation session: %w", err)
		}
		sourceRun = run
		parentRunID = run.ID
		pageContext = parsePageContext(run.PageContext)
		if run.TriggerMessageID != nil {
			anchorMessageID = *run.TriggerMessageID
		}
		instruction = buildContinuationInstruction("run", summarizeRun(run))
	case "message":
		message, err := h.runtimeRepo.GetMessageByID(ctx, targetID, userIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to load continuation message: %w", err)
		}
		session, err = h.runtimeRepo.GetSessionByID(ctx, message.SessionID, userIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to load continuation session: %w", err)
		}
		anchorMessageID = message.ID
		if message.RunID != nil && *message.RunID != "" {
			parentRunID = *message.RunID
			sourceRun, _ = h.runtimeRepo.GetRunByID(ctx, parentRunID, userIDStr)
			if sourceRun != nil {
				pageContext = parsePageContext(sourceRun.PageContext)
			}
		}
		instruction = buildContinuationInstruction("message", fmt.Sprintf("role=%s, content=%s", message.Role, truncateSummary(message.Content, 120)))
	case "artifact":
		artifact, err := h.runtimeRepo.GetArtifactByID(ctx, targetID, userIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to load continuation artifact: %w", err)
		}
		session, err = h.runtimeRepo.GetSessionByID(ctx, artifact.SessionID, userIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to load continuation session: %w", err)
		}
		parentRunID = artifact.RunID
		if parentRunID != "" {
			sourceRun, _ = h.runtimeRepo.GetRunByID(ctx, parentRunID, userIDStr)
			if sourceRun != nil {
				pageContext = parsePageContext(sourceRun.PageContext)
				if sourceRun.TriggerMessageID != nil {
					anchorMessageID = *sourceRun.TriggerMessageID
				}
			}
		}
		instruction = buildContinuationInstruction("artifact", summarizeArtifact(artifact))
	case "action":
		action, err := h.runtimeRepo.GetReviewActionByID(ctx, targetID, userIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to load continuation action: %w", err)
		}
		session, err = h.runtimeRepo.GetSessionByID(ctx, action.SessionID, userIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to load continuation session: %w", err)
		}
		var artifact *repository.AIChatArtifact
		if action.ArtifactID != nil && *action.ArtifactID != "" {
			artifact, _ = h.runtimeRepo.GetArtifactByID(ctx, *action.ArtifactID, userIDStr)
			if artifact != nil && artifact.RunID != "" {
				parentRunID = artifact.RunID
			}
			if artifact != nil {
				fallbackIDs = append(fallbackIDs, contractIDFromPayload(artifact.Data))
			}
		}
		if parentRunID == "" && action.RunID != nil && *action.RunID != "" {
			parentRunID = *action.RunID
		}
		if parentRunID != "" {
			sourceRun, _ = h.runtimeRepo.GetRunByID(ctx, parentRunID, userIDStr)
			if sourceRun != nil {
				pageContext = parsePageContext(sourceRun.PageContext)
				if sourceRun.TriggerMessageID != nil {
					anchorMessageID = *sourceRun.TriggerMessageID
				}
			}
		}
		fallbackIDs = append(fallbackIDs, contractIDFromPayload(action.ActionPayload))
		instruction = buildContinuationInstruction("action", summarizeReviewAction(action, artifact))
	default:
		return nil, fmt.Errorf("unsupported continuation target type: %s", req.Target.Type)
	}

	messages, err := h.runtimeRepo.ListMessagesBySession(ctx, session.ID, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to load session history for continuation: %w", err)
	}

	history, compressionStrategy := buildCompressedHistory(messages, anchorMessageID)
	pageContext = mergePageContext(req.PageContext, pageContext)
	effectiveContractID := resolveContractID(req.ContractID, pageContext, session, fallbackIDs...)

	if strings.TrimSpace(req.Instruction) != "" {
		instruction = strings.TrimSpace(req.Instruction)
	}

	return &continuationSeed{
		session:             session,
		parentRunID:         parentRunID,
		sourceRun:           sourceRun,
		effectiveContractID: effectiveContractID,
		pageContext:         pageContext,
		history:             history,
		instruction:         instruction,
		targetType:          targetType,
		targetID:            targetID,
		compressionStrategy: compressionStrategy,
	}, nil
}

func (h *AIChatHandler) createContinuationRuntimeRun(c *gin.Context, req *CreateAIChatContinuationRequest, userIDStr string) (*aiChatRuntimeState, *AgentRunbook, *AIChatRequest, *continuationSeed, error) {
	seed, err := h.resolveContinuationSeed(c.Request.Context(), req, userIDStr)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	runtimeReq := &AIChatRequest{
		SessionID:   seed.session.ID,
		Message:     seed.instruction,
		ContractID:  seed.effectiveContractID,
		History:     seed.history,
		Language:    req.Language,
		PageContext: seed.pageContext,
	}
	agentRunbook := h.buildContinuationRunbook(*runtimeReq, effectiveContractIDFromRequest(*runtimeReq), seed.sourceRun)
	state, err := h.beginRuntimeStateForSession(c.Request.Context(), seed.session, runtimeReq, userIDStr, effectiveContractIDFromRequest(*runtimeReq), agentRunbook)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to initialize continuation runtime: %w", err)
	}
	if seed.parentRunID != "" {
		state.run.ParentRunID = &seed.parentRunID
		_ = h.runtimeRepo.UpdateRunParent(c.Request.Context(), state.run.ID, seed.parentRunID)
	}
	return state, agentRunbook, runtimeReq, seed, nil
}

func continuationResponse(state *aiChatRuntimeState, runbook *AgentRunbook, seed *continuationSeed) gin.H {
	response := gin.H{
		"run":             state.run,
		"trigger_message": state.userMessage,
		"agent_plan":      agentPlanFromRunbook(runbook),
		"tool_calls":      toolCallsFromRunbook(runbook),
		"review_prompts":  reviewPromptsFromRunbook(runbook),
		"stream_url":      fmt.Sprintf("/api/v1/ai/chat/runs/%s/stream", state.run.ID),
		"continuation": gin.H{
			"target": gin.H{
				"type": seed.targetType,
				"id":   seed.targetID,
			},
			"parent_run_id":         seed.parentRunID,
			"resolved_instruction":  seed.instruction,
			"effective_contract_id": seed.effectiveContractID,
			"resolved_skill_id": func() string {
				if runbook == nil {
					return ""
				}
				return runbook.SkillID
			}(),
			"history_message_count": len(seed.history),
			"compression_strategy":  seed.compressionStrategy,
		},
	}
	return response
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

	state, runbook, runtimeReq, seed, err := h.createContinuationRuntimeRun(c, &req, userIDStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create ai chat continuation: " + err.Error()})
		return
	}

	role, _ := c.Get("role")
	roleStr, _ := role.(string)
	legalEntityID := middleware.GetTenantID(c)
	authHeader := c.GetHeader("Authorization")
	go h.executeRuntimeRun(state, *runtimeReq, legalEntityID, roleStr, authHeader, runbook)

	c.JSON(http.StatusCreated, continuationResponse(state, runbook, seed))
}

func (h *AIChatHandler) CreateReviewAction(c *gin.Context) {
	userID, ok := c.Get("user_id")
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user context"})
		return
	}
	userIDStr, _ := userID.(string)
	artifactID := c.Param("id")

	artifact, err := h.runtimeRepo.GetArtifactByID(c.Request.Context(), artifactID, userIDStr)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ai chat artifact not found"})
		return
	}

	var req CreateAIChatReviewActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	action := &repository.AIChatReviewAction{
		SessionID:     artifact.SessionID,
		RunID:         &artifact.RunID,
		ArtifactID:    &artifact.ID,
		ActionType:    req.ActionType,
		ActionPayload: marshalOptionalJSON(req.ActionPayload),
		ActedBy:       userIDStr,
	}
	if req.Comment != "" {
		action.Comment = &req.Comment
	}
	if err := h.runtimeRepo.RecordReviewAction(c.Request.Context(), action); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to record ai chat review action: " + err.Error()})
		return
	}
	if err := h.runtimeRepo.UpdateArtifactStatus(c.Request.Context(), artifact.ID, artifactStatusForAction(req.ActionType)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update ai chat artifact status: " + err.Error()})
		return
	}

	response := gin.H{
		"action": action,
		"artifact": gin.H{
			"id":     artifact.ID,
			"status": artifactStatusForAction(req.ActionType),
		},
	}
	if req.FollowUp != nil {
		continuationReq := &CreateAIChatContinuationRequest{
			Target: AIChatContinuationTarget{
				Type: "action",
				ID:   action.ID,
			},
			Instruction: req.FollowUp.Message,
			ContractID:  req.FollowUp.ContractID,
			Language:    req.FollowUp.Language,
			PageContext: req.FollowUp.PageContext,
		}
		followUpState, followUpRunbook, runtimeReq, seed, err := h.createContinuationRuntimeRun(c, continuationReq, userIDStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create follow-up ai chat continuation: " + err.Error()})
			return
		}

		role, _ := c.Get("role")
		roleStr, _ := role.(string)
		legalEntityID := middleware.GetTenantID(c)
		authHeader := c.GetHeader("Authorization")
		go h.executeRuntimeRun(followUpState, *runtimeReq, legalEntityID, roleStr, authHeader, followUpRunbook)

		response["run"] = followUpState.run
		response["trigger_message"] = followUpState.userMessage
		response["agent_plan"] = agentPlanFromRunbook(followUpRunbook)
		response["tool_calls"] = toolCallsFromRunbook(followUpRunbook)
		response["review_prompts"] = reviewPromptsFromRunbook(followUpRunbook)
		response["stream_url"] = fmt.Sprintf("/api/v1/ai/chat/runs/%s/stream", followUpState.run.ID)
		response["continuation"] = continuationResponse(followUpState, followUpRunbook, seed)["continuation"]
	}

	c.JSON(http.StatusCreated, response)
}
