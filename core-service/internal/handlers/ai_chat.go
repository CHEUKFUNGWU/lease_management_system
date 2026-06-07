package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
)

type aiChatRuntimeStore interface {
	CreateSession(ctx context.Context, session *repository.AIChatSession) error
	GetSessionByID(ctx context.Context, sessionID, userID string) (*repository.AIChatSession, error)
	ListSessions(ctx context.Context, filter repository.AIChatSessionFilter) ([]*repository.AIChatSession, error)
	CreateRun(ctx context.Context, run *repository.AIChatRun) error
	LinkRunTriggerMessage(ctx context.Context, runID, triggerMessageID string) error
	UpdateRunStatus(ctx context.Context, runID, status string, reviewRequired bool, summaryText, errorMessage *string, startedAt, completedAt *time.Time) error
	UpdateRunParent(ctx context.Context, runID, parentRunID string) error
	GetRunByID(ctx context.Context, runID, userID string) (*repository.AIChatRun, error)
	ListRunsBySession(ctx context.Context, sessionID string, limit, offset int) ([]*repository.AIChatRun, error)
	CreateMessage(ctx context.Context, message *repository.AIChatMessage) error
	GetNextMessageSequence(ctx context.Context, sessionID string) (int, error)
	ListMessagesBySession(ctx context.Context, sessionID string, limit int) ([]*repository.AIChatMessage, error)
	GetMessageByID(ctx context.Context, messageID, userID string) (*repository.AIChatMessage, error)
	AppendRunEvent(ctx context.Context, event *repository.AIChatRunEvent) error
	ListRunEvents(ctx context.Context, runID string, afterSequence, limit int) ([]*repository.AIChatRunEvent, error)
	GetNextRunEventSequence(ctx context.Context, runID string) (int, error)
	CreateArtifact(ctx context.Context, artifact *repository.AIChatArtifact) error
	GetArtifactByID(ctx context.Context, artifactID, userID string) (*repository.AIChatArtifact, error)
	ListArtifactsBySession(ctx context.Context, sessionID string, limit int) ([]*repository.AIChatArtifact, error)
	UpdateArtifactStatus(ctx context.Context, artifactID, status string) error
	RecordReviewAction(ctx context.Context, action *repository.AIChatReviewAction) error
	GetReviewActionByID(ctx context.Context, actionID, userID string) (*repository.AIChatReviewAction, error)
	ListReviewActionsBySession(ctx context.Context, sessionID string, limit int) ([]*repository.AIChatReviewAction, error)
	CreateAttachment(ctx context.Context, attachment *repository.AIChatAttachment) error
}

type AIChatHandler struct {
	contractRepo *repository.ContractRepository
	mcRepo       *repository.MonthlyClosingRepository
	eventRepo    *repository.EventRepository
	runtimeRepo  aiChatRuntimeStore
}

func NewAIChatHandler(contractRepo *repository.ContractRepository, mcRepo *repository.MonthlyClosingRepository, eventRepo *repository.EventRepository, runtimeRepo aiChatRuntimeStore) *AIChatHandler {
	return &AIChatHandler{contractRepo: contractRepo, mcRepo: mcRepo, eventRepo: eventRepo, runtimeRepo: runtimeRepo}
}

type PageContext struct {
	Page       string            `json:"page,omitempty"`
	Title      string            `json:"title,omitempty"`
	ContractID string            `json:"contract_id,omitempty"`
	Period     string            `json:"period,omitempty"`
	ReportView string            `json:"report_view,omitempty"`
	Filters    map[string]string `json:"filters,omitempty"`
	Summary    string            `json:"summary,omitempty"`
}

type AIChatRequest struct {
	SessionID   string        `json:"session_id,omitempty"`
	RunID       string        `json:"run_id,omitempty"`
	Message     string        `json:"message" binding:"required"`
	ContractID  string        `json:"contract_id,omitempty"`
	History     []ChatMessage `json:"history,omitempty"`
	FileID      string        `json:"file_id,omitempty"`
	ObjectName  string        `json:"object_name,omitempty"`
	ContentType string        `json:"content_type,omitempty"`
	PageContext *PageContext  `json:"page_context,omitempty"`
	Language    string        `json:"language,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type AIChatResponse struct {
	SessionID              string                       `json:"session_id,omitempty"`
	RunID                  string                       `json:"run_id,omitempty"`
	Answer                 string                       `json:"answer"`
	Sources                []Source                     `json:"sources"`
	Confidence             float64                      `json:"confidence"`
	IsOfficial             bool                         `json:"is_official"`
	Model                  string                       `json:"model,omitempty"`
	AgentMode              bool                         `json:"agent_mode,omitempty"`
	AgentPlan              []AgentPlanStep              `json:"agent_plan,omitempty"`
	ToolCalls              []AgentToolCall              `json:"tool_calls,omitempty"`
	ReviewPrompts          []AgentReviewPrompt          `json:"review_prompts,omitempty"`
	DraftContracts         []ContractDraftItem          `json:"draft_contracts,omitempty"`
	BatchSummary           *BatchParseSummary           `json:"batch_summary,omitempty"`
	DraftPaymentSchedules  []PaymentScheduleDraftItem   `json:"draft_payment_schedules,omitempty"`
	PaymentScheduleSummary *PaymentScheduleParseSummary `json:"payment_schedule_summary,omitempty"`
}

type AgentPlanStep struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"` // pending | running | completed | needs_review
}

type AgentToolCall struct {
	Tool           string `json:"tool"`
	Skill          string `json:"skill"`
	Status         string `json:"status"` // completed | failed | needs_review
	InputSummary   string `json:"input_summary"`
	OutputSummary  string `json:"output_summary"`
	RequiresReview bool   `json:"requires_review"`
}

// LLMToolCall represents a tool call returned by the LLM (OpenAI/DeepSeek format).
type LLMToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function map[string]interface{} `json:"function"`
}

// LLMChatResponse is the raw response from AI Service when using function calling.
type LLMChatResponse struct {
	Answer     string        `json:"answer"`
	Sources    []Source      `json:"sources"`
	Confidence float64       `json:"confidence"`
	Model      string        `json:"model"`
	ToolCalls  []LLMToolCall `json:"tool_calls,omitempty"`
}

type AgentReviewPrompt struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	Severity        string   `json:"severity"` // info | warning | critical
	Action          string   `json:"action"`
	ContractNumbers []string `json:"contract_numbers,omitempty"`
}

type AgentRunbook struct {
	SkillID               string
	SkillName             string
	RequiresEvidenceInput bool
	NeedsPortfolioContext bool
	AnswerPrefix          string
	AgentPlan             []AgentPlanStep
	ToolCalls             []AgentToolCall
	ReviewPrompts         []AgentReviewPrompt
}

type ContractDraftItem struct {
	ContractNumber    string   `json:"contract_number"`
	ContractName      string   `json:"contract_name"`
	Lessee            string   `json:"lessee"`
	Lessor            string   `json:"lessor"`
	StoreName         string   `json:"store_name"`
	StoreAddress      string   `json:"store_address"`
	CommencementDate  string   `json:"commencement_date"`
	LeaseStartDate    string   `json:"lease_start_date"`
	LeaseEndDate      string   `json:"lease_end_date"`
	Currency          string   `json:"currency"`
	AssetType         string   `json:"asset_type"`
	FixedRentAmount   float64  `json:"fixed_rent_amount"`
	PaymentFrequency  string   `json:"payment_frequency"`
	PaymentTiming     string   `json:"payment_timing"`
	RenewalOption     bool     `json:"renewal_option"`
	TerminationOption bool     `json:"termination_option"`
	CAMAmount         float64  `json:"cam_amount"`
	ServiceFee        float64  `json:"service_fee"`
	DiscountRateType  string   `json:"discount_rate_type"`
	DiscountRate      float64  `json:"discount_rate"`
	IsLease           bool     `json:"is_lease"`
	LeaseScope        string   `json:"lease_scope"`
	SuggestedScope    string   `json:"suggested_scope"`
	ExemptionReason   string   `json:"exemption_reason"`
	ScopeSource       string   `json:"scope_source"`
	ScopeConfidence   float64  `json:"scope_confidence"`
	Confidence        float64  `json:"confidence"`
	MissingFields     []string `json:"missing_fields"`
	Warnings          []string `json:"warnings"`
}

type BatchParseSummary struct {
	TotalCount           int      `json:"total_count"`
	OverallConfidence    float64  `json:"overall_confidence"`
	RequiresHumanConfirm bool     `json:"requires_human_confirmation"`
	MissingFields        []string `json:"missing_fields"`
	Warnings             []string `json:"warnings"`
}

type PaymentScheduleDraftItem struct {
	PeriodStart      string  `json:"period_start"`
	PeriodEnd        string  `json:"period_end"`
	DueDate          string  `json:"due_date"`
	Amount           float64 `json:"amount"`
	PaymentTiming    string  `json:"payment_timing"`
	IsFixed          bool    `json:"is_fixed"`
	IsLeaseComponent bool    `json:"is_lease_component"`
	AmountType       string  `json:"amount_type"`
	Currency         string  `json:"currency"`
	Confidence       float64 `json:"confidence"`
}

type PaymentScheduleParseSummary struct {
	TotalCount           int      `json:"total_count"`
	OverallConfidence    float64  `json:"overall_confidence"`
	RequiresHumanConfirm bool     `json:"requires_human_confirmation"`
	MissingFields        []string `json:"missing_fields"`
	Warnings             []string `json:"warnings"`
	CanImport            bool     `json:"can_import"`
	ContractID           string   `json:"contract_id,omitempty"`
}

type Source struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Title   string `json:"title"`
	Snippet string `json:"snippet"`
}

type aiChatRuntimeState struct {
	session     *repository.AIChatSession
	run         *repository.AIChatRun
	userMessage *repository.AIChatMessage
	userID      string
}

func marshalRuntimeJSON(v interface{}) json.RawMessage {
	if v == nil {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return data
}

func (h *AIChatHandler) beginRuntimeStateForSession(ctx context.Context, session *repository.AIChatSession, req *AIChatRequest, userID, effectiveContractID string, runbook *AgentRunbook) (*aiChatRuntimeState, error) {
	if h.runtimeRepo == nil || session == nil {
		return nil, nil
	}

	run := &repository.AIChatRun{
		SessionID:      session.ID,
		Status:         "running",
		PageContext:    marshalRuntimeJSON(req.PageContext),
		ReviewRequired: runbook != nil && len(runbook.ReviewPrompts) > 0,
		CreatedBy:      &userID,
		AgentMode:      runbook != nil,
	}
	if req.RunID != "" {
		run.ID = req.RunID
	}
	if runbook != nil {
		run.SkillID = &runbook.SkillID
	}
	now := time.Now()
	run.StartedAt = &now
	if err := h.runtimeRepo.CreateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("failed to create ai chat run: %w", err)
	}

	nextSeq, err := h.runtimeRepo.GetNextMessageSequence(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate ai chat message sequence: %w", err)
	}
	userMessage := &repository.AIChatMessage{
		SessionID:   session.ID,
		RunID:       &run.ID,
		Role:        "user",
		MessageType: "text",
		SequenceNo:  nextSeq,
		Content:     req.Message,
		CreatedBy:   &userID,
		Attachments: marshalRuntimeJSON(buildRuntimeAttachmentRefs(req)),
	}
	if err := h.runtimeRepo.CreateMessage(ctx, userMessage); err != nil {
		return nil, fmt.Errorf("failed to persist ai chat user message: %w", err)
	}
	if err := h.runtimeRepo.LinkRunTriggerMessage(ctx, run.ID, userMessage.ID); err != nil {
		return nil, fmt.Errorf("failed to link ai chat run trigger message: %w", err)
	}

	if req.FileID != "" {
		attachment := &repository.AIChatAttachment{
			SessionID:      session.ID,
			MessageID:      &userMessage.ID,
			RunID:          &run.ID,
			FileID:         req.FileID,
			OriginalName:   req.ObjectName,
			ContentType:    req.ContentType,
			CreatedBy:      &userID,
			MinioObjectKey: &req.ObjectName,
			ParseStatus:    "processing",
		}
		if err := h.runtimeRepo.CreateAttachment(ctx, attachment); err != nil {
			return nil, fmt.Errorf("failed to persist ai chat attachment: %w", err)
		}
	}

	state := &aiChatRuntimeState{
		session:     session,
		run:         run,
		userMessage: userMessage,
		userID:      userID,
	}

	if err := h.appendRuntimeEvent(ctx, state, "message_start", map[string]interface{}{
		"message_id":   userMessage.ID,
		"role":         "user",
		"content":      req.Message,
		"has_file":     req.FileID != "",
		"contract_id":  effectiveContractID,
		"page_context": req.PageContext,
	}, false); err != nil {
		return nil, err
	}
	if err := h.appendRuntimeEvent(ctx, state, "run_status", map[string]interface{}{
		"status":          "running",
		"skill_id":        run.SkillID,
		"agent_mode":      run.AgentMode,
		"review_required": run.ReviewRequired,
	}, false); err != nil {
		return nil, err
	}

	return state, nil
}

func (h *AIChatHandler) beginRuntimeState(c *gin.Context, req *AIChatRequest, legalEntityID, userID, effectiveContractID string, runbook *AgentRunbook) (*aiChatRuntimeState, error) {
	if h.runtimeRepo == nil {
		return nil, nil
	}

	ctx := c.Request.Context()
	var session *repository.AIChatSession
	var err error

	if req.SessionID != "" {
		session, err = h.runtimeRepo.GetSessionByID(ctx, req.SessionID, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to load ai chat session: %w", err)
		}
	} else {
		session = &repository.AIChatSession{
			UserID:          userID,
			Title:           summarizeSessionTitle(req.Message),
			ContextSnapshot: marshalRuntimeJSON(req.PageContext),
		}
		if legalEntityID != "" {
			session.LegalEntityID = &legalEntityID
		}
		if effectiveContractID != "" {
			session.BoundContractID = &effectiveContractID
		}
		if err := h.runtimeRepo.CreateSession(ctx, session); err != nil {
			return nil, fmt.Errorf("failed to create ai chat session: %w", err)
		}
	}

	return h.beginRuntimeStateForSession(ctx, session, req, userID, effectiveContractID, runbook)
}

func (h *AIChatHandler) appendRuntimeEvent(ctx context.Context, state *aiChatRuntimeState, eventType string, payload interface{}, isTerminal bool) error {
	if h.runtimeRepo == nil || state == nil || state.run == nil || state.session == nil {
		return nil
	}
	seq, err := h.runtimeRepo.GetNextRunEventSequence(ctx, state.run.ID)
	if err != nil {
		return err
	}
	return h.runtimeRepo.AppendRunEvent(ctx, &repository.AIChatRunEvent{
		RunID:      state.run.ID,
		SessionID:  state.session.ID,
		SequenceNo: seq,
		EventType:  eventType,
		Payload:    marshalRuntimeJSON(payload),
		IsTerminal: isTerminal,
	})
}

func (h *AIChatHandler) finalizeRuntimeSuccess(ctx context.Context, state *aiChatRuntimeState, resp *AIChatResponse) {
	if h.runtimeRepo == nil || state == nil || resp == nil {
		return
	}

	resp.SessionID = state.session.ID
	resp.RunID = state.run.ID

	nextSeq, err := h.runtimeRepo.GetNextMessageSequence(ctx, state.session.ID)
	if err == nil {
		model := resp.Model
		assistantMessage := &repository.AIChatMessage{
			SessionID:   state.session.ID,
			RunID:       &state.run.ID,
			Role:        "assistant",
			MessageType: "text",
			SequenceNo:  nextSeq,
			Content:     resp.Answer,
			Model:       &model,
			CreatedBy:   &state.userID,
			Sources:     marshalRuntimeJSON(resp.Sources),
		}
		_ = h.runtimeRepo.CreateMessage(ctx, assistantMessage)
		_ = h.appendRuntimeEvent(ctx, state, "message_end", map[string]interface{}{
			"message_id": assistantMessage.ID,
			"role":       "assistant",
			"content":    resp.Answer,
			"model":      resp.Model,
			"sources":    resp.Sources,
		}, false)
	}

	if len(resp.ToolCalls) > 0 {
		_ = h.appendRuntimeEvent(ctx, state, "tool_end", resp.ToolCalls, false)
	}
	if len(resp.ReviewPrompts) > 0 {
		_ = h.appendRuntimeEvent(ctx, state, "review_prompt", resp.ReviewPrompts, false)
	}

	h.persistRuntimeArtifacts(ctx, state, resp)

	now := time.Now()
	summary := resp.Answer
	_ = h.runtimeRepo.UpdateRunStatus(ctx, state.run.ID, statusForResponse(resp), len(resp.ReviewPrompts) > 0, &summary, nil, nil, &now)
	_ = h.appendRuntimeEvent(ctx, state, "run_end", map[string]interface{}{
		"status":           statusForResponse(resp),
		"review_required":  len(resp.ReviewPrompts) > 0,
		"artifact_present": resp.DraftContracts != nil || resp.DraftPaymentSchedules != nil,
	}, true)
}

func (h *AIChatHandler) finalizeRuntimeFailure(ctx context.Context, state *aiChatRuntimeState, answer, model string, reviewPrompts []AgentReviewPrompt, err error) {
	if h.runtimeRepo == nil || state == nil {
		return
	}

	nextSeq, seqErr := h.runtimeRepo.GetNextMessageSequence(ctx, state.session.ID)
	if seqErr == nil {
		assistantMessage := &repository.AIChatMessage{
			SessionID:   state.session.ID,
			RunID:       &state.run.ID,
			Role:        "assistant",
			MessageType: "text",
			SequenceNo:  nextSeq,
			Content:     answer,
			Model:       &model,
			CreatedBy:   &state.userID,
		}
		_ = h.runtimeRepo.CreateMessage(ctx, assistantMessage)
		_ = h.appendRuntimeEvent(ctx, state, "message_end", map[string]interface{}{
			"message_id": assistantMessage.ID,
			"role":       "assistant",
			"content":    answer,
			"model":      model,
		}, false)
	}

	errText := ""
	if err != nil {
		errText = err.Error()
	}
	now := time.Now()
	_ = h.runtimeRepo.UpdateRunStatus(ctx, state.run.ID, "failed", len(reviewPrompts) > 0, &answer, &errText, nil, &now)
	_ = h.appendRuntimeEvent(ctx, state, "run_error", map[string]interface{}{
		"error": errText,
		"model": model,
	}, true)
}

func (h *AIChatHandler) persistRuntimeArtifacts(ctx context.Context, state *aiChatRuntimeState, resp *AIChatResponse) {
	if h.runtimeRepo == nil || state == nil || resp == nil {
		return
	}

	if len(resp.DraftContracts) > 0 {
		artifact := &repository.AIChatArtifact{
			SessionID:      state.session.ID,
			RunID:          state.run.ID,
			ArtifactType:   "contract_draft",
			Title:          "合同草稿",
			Status:         "ready",
			Data:           marshalRuntimeJSON(map[string]interface{}{"contracts": resp.DraftContracts, "summary": resp.BatchSummary}),
			ReviewRequired: true,
			CreatedBy:      &state.userID,
		}
		if err := h.runtimeRepo.CreateArtifact(ctx, artifact); err == nil {
			_ = h.appendRuntimeEvent(ctx, state, "artifact_ready", map[string]interface{}{
				"artifact_id":   artifact.ID,
				"artifact_type": artifact.ArtifactType,
				"title":         artifact.Title,
				"data": map[string]interface{}{
					"contracts": resp.DraftContracts,
					"summary":   resp.BatchSummary,
				},
			}, false)
		}
	}

	if len(resp.DraftPaymentSchedules) > 0 {
		artifact := &repository.AIChatArtifact{
			SessionID:      state.session.ID,
			RunID:          state.run.ID,
			ArtifactType:   "payment_schedule_draft",
			Title:          "付款计划草稿",
			Status:         "ready",
			Data:           marshalRuntimeJSON(map[string]interface{}{"schedules": resp.DraftPaymentSchedules, "summary": resp.PaymentScheduleSummary}),
			ReviewRequired: true,
			CreatedBy:      &state.userID,
		}
		if err := h.runtimeRepo.CreateArtifact(ctx, artifact); err == nil {
			_ = h.appendRuntimeEvent(ctx, state, "artifact_ready", map[string]interface{}{
				"artifact_id":   artifact.ID,
				"artifact_type": artifact.ArtifactType,
				"title":         artifact.Title,
				"data": map[string]interface{}{
					"schedules": resp.DraftPaymentSchedules,
					"summary":   resp.PaymentScheduleSummary,
				},
			}, false)
		}
	}
}

func buildRuntimeAttachmentRefs(req *AIChatRequest) []map[string]string {
	if req == nil || req.FileID == "" {
		return nil
	}
	return []map[string]string{{
		"file_id":      req.FileID,
		"object_name":  req.ObjectName,
		"content_type": req.ContentType,
	}}
}

func summarizeSessionTitle(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return "新会话"
	}
	runes := []rune(message)
	if len(runes) > 20 {
		return string(runes[:20]) + "..."
	}
	return message
}

func statusForResponse(resp *AIChatResponse) string {
	if resp == nil {
		return "completed"
	}
	if len(resp.ReviewPrompts) > 0 {
		return "waiting_review"
	}
	return "completed"
}

func attachRuntimeIDs(resp *AIChatResponse, state *aiChatRuntimeState) {
	if resp == nil || state == nil || state.session == nil || state.run == nil {
		return
	}
	resp.SessionID = state.session.ID
	resp.RunID = state.run.ID
}

func effectiveContractIDFromRequest(req AIChatRequest) string {
	effectiveContractID := req.ContractID
	if effectiveContractID == "" && req.PageContext != nil && req.PageContext.ContractID != "" {
		effectiveContractID = req.PageContext.ContractID
	}
	return effectiveContractID
}

func (h *AIChatHandler) executeChatRequest(ctx context.Context, authHeader string, req AIChatRequest, legalEntityID, userIDStr, roleStr string, runtimeState *aiChatRuntimeState, agentRunbook *AgentRunbook) AIChatResponse {
	var sources []Source
	var contextData strings.Builder
	effectiveContractID := effectiveContractIDFromRequest(req)

	// 1. Handle file upload if present — use Function Calling to let LLM decide which tool to invoke
	if req.FileID != "" && req.ObjectName != "" {
		fileSystemPrompt := fmt.Sprintf(`用户上传了一个文件，文件名: %s，MIME类型: %s。
请根据用户的意图和文件类型，决定调用哪个解析工具：
- parse_contract_batch: 当文件是合同台账（Excel/PDF包含多份合同）时使用
- parse_payment_schedule: 当文件是租金表/付款计划时使用
- parse_contract: 当文件是单份合同（PDF/Word/图片）时使用

用户消息: %s`, req.ObjectName, req.ContentType, req.Message)

		_, modelName, toolCalls, err := h.callLLMWithTools(ctx, authHeader, fileSystemPrompt, req.Message, req.History, req.Language, fileParseTools)
		fallbackTool := detectFileParseTool(req.Message)
		selectedTool := fallbackTool
		toolExecutionChain := []AgentToolCall{}
		contractID := effectiveContractID

		if err == nil && len(toolCalls) > 0 && toolCalls[0].Tool != "" {
			selectedTool = toolCalls[0].Tool
			toolExecutionChain = toolCalls
			var funcArgs map[string]interface{}
			if toolCalls[0].InputSummary != "" {
				_ = json.Unmarshal([]byte(toolCalls[0].InputSummary), &funcArgs)
			}
			if cid, ok := funcArgs["contract_id"].(string); ok && cid != "" {
				contractID = cid
			}
		} else if fallbackTool != "" {
			modelName = "deterministic-router"
		}

		if selectedTool != "" {
			_ = h.appendRuntimeEvent(ctx, runtimeState, "tool_start", map[string]interface{}{
				"tool":        selectedTool,
				"status":      "running",
				"file_id":     req.FileID,
				"file_name":   req.ObjectName,
				"contract_id": contractID,
			}, false)
		}

		switch selectedTool {
		case "parse_contract_batch":
			batchResult, err := h.parseContractBatch(ctx, authHeader, req.FileID, req.ObjectName, req.ContentType)
			if err != nil {
				resp := AIChatResponse{
					Answer:     fmt.Sprintf("文件解析失败: %s", err.Error()),
					Sources:    sources,
					Confidence: 0.5,
					IsOfficial: false,
					Model:      "fallback",
				}
				attachRuntimeIDs(&resp, runtimeState)
				h.finalizeRuntimeFailure(ctx, runtimeState, resp.Answer, resp.Model, nil, err)
				return resp
			}
			resp := AIChatResponse{
				Answer:         batchResult.SummaryText,
				Sources:        batchResult.Sources,
				Confidence:     batchResult.Confidence,
				IsOfficial:     false,
				Model:          modelName,
				AgentMode:      true,
				AgentPlan:      batchResult.AgentPlan,
				ToolCalls:      append(toolExecutionChain, batchResult.ToolCalls...),
				ReviewPrompts:  batchResult.ReviewPrompts,
				DraftContracts: batchResult.Contracts,
				BatchSummary:   batchResult.Summary,
			}
			h.finalizeRuntimeSuccess(ctx, runtimeState, &resp)
			return resp

		case "parse_payment_schedule":
			scheduleResult, err := h.parsePaymentSchedule(ctx, authHeader, req.FileID, req.ObjectName, req.ContentType, contractID)
			if err != nil {
				resp := AIChatResponse{
					Answer:     fmt.Sprintf("租金表解析失败: %s", err.Error()),
					Sources:    sources,
					Confidence: 0.5,
					IsOfficial: false,
					Model:      "fallback",
				}
				attachRuntimeIDs(&resp, runtimeState)
				h.finalizeRuntimeFailure(ctx, runtimeState, resp.Answer, resp.Model, nil, err)
				return resp
			}
			resp := AIChatResponse{
				Answer:                 scheduleResult.SummaryText,
				Sources:                scheduleResult.Sources,
				Confidence:             scheduleResult.Confidence,
				IsOfficial:             false,
				Model:                  modelName,
				AgentMode:              true,
				AgentPlan:              scheduleResult.AgentPlan,
				ToolCalls:              append(toolExecutionChain, scheduleResult.ToolCalls...),
				ReviewPrompts:          scheduleResult.ReviewPrompts,
				DraftPaymentSchedules:  scheduleResult.Schedules,
				PaymentScheduleSummary: scheduleResult.Summary,
			}
			h.finalizeRuntimeSuccess(ctx, runtimeState, &resp)
			return resp

		default:
			parsed, err := h.parseFile(ctx, authHeader, req.FileID, req.ObjectName, req.ContentType)
			if err != nil {
				contextData.WriteString(fmt.Sprintf("\n## 文件解析失败\n错误: %s\n", err.Error()))
			} else {
				contextData.WriteString("\n## 上传文件解析结果\n")
				for k, v := range parsed {
					contextData.WriteString(fmt.Sprintf("- %s: %v\n", k, v))
				}
				sources = append(sources, Source{
					Type:    "file",
					ID:      req.FileID,
					Title:   "上传文件",
					Snippet: req.ObjectName,
				})
			}
		}
	}

	if agentRunbook != nil && agentRunbook.NeedsPortfolioContext {
		h.appendAgentPortfolioContext(ctx, legalEntityID, &contextData, &sources)
	}

	// 2. Page-context-aware data retrieval
	// 2a. Contract detail context: always retrieve full contract data regardless of message keywords
	if effectiveContractID != "" {
		h.retrieveContractDetail(ctx, effectiveContractID, legalEntityID, &contextData, &sources)
	}

	// 2b. Reports page context
	if req.PageContext != nil && req.PageContext.Page == "reports" {
		h.appendReportsContext(req.PageContext, &contextData)
	}

	// 2c. Monthly closing page context
	if req.PageContext != nil && req.PageContext.Page == "monthly-closing" {
		h.appendMonthlyClosingContext(ctx, req.PageContext, &contextData, &sources)
	}

	// 3. Keyword-based retrieval (backward compatible, works alongside page context)
	msgLower := strings.ToLower(req.Message)

	// Contract list queries (only if no specific contract ID is resolved)
	if effectiveContractID == "" && containsAny(msgLower, []string{"合同", "租赁", "门店", "承租", "出租", "lease", "contract"}) {
		contracts, err := h.contractRepo.GetAll(ctx, legalEntityID)
		if err == nil && len(contracts) > 0 {
			contextData.WriteString(fmt.Sprintf("\n## 合同数据（共 %d 份）\n", len(contracts)))
			for _, contract := range contracts {
				contextData.WriteString(fmt.Sprintf("- ID: %s, 编号: %s, 名称: %s, 状态: %s, 承租方: %s, 出租方: %s\n",
					contract.ID, contract.ContractNumber, contract.ContractName,
					contract.ApprovalStatus, contract.LesseeName, contract.LessorName))
				sources = append(sources, Source{
					Type:    "contract",
					ID:      contract.ID,
					Title:   contract.ContractName,
					Snippet: fmt.Sprintf("编号: %s, 状态: %s", contract.ContractNumber, contract.ApprovalStatus),
				})
			}
		}
	}

	// Measurement / liability / depreciation queries (only if contract detail context hasn't already loaded them)
	if effectiveContractID == "" && containsAny(msgLower, []string{"负债", "折旧", "利息", "摊销", "rou", "计量", "measurement", "depreciation", "liability"}) {
		if req.ContractID != "" {
			results, err := h.mcRepo.GetMeasurementResults(ctx, req.ContractID, "")
			if err == nil && len(results) > 0 {
				contextData.WriteString(fmt.Sprintf("\n## 计量结果（合同 %s，共 %d 期）\n", req.ContractID, len(results)))
				for _, r := range results {
					contextData.WriteString(fmt.Sprintf("- 期间: %s, 期初负债: %.2f, 期末负债: %.2f, 利息: %.2f, 本金偿还: %.2f, 折旧: %.2f, 期末ROU: %.2f\n",
						r.AccountingPeriod, r.OpeningLiability, r.ClosingLiability, r.InterestExpense,
						r.PrincipalRepayment, r.Depreciation, r.ClosingROUAsset))
				}
				sources = append(sources, Source{
					Type:    "measurement",
					ID:      req.ContractID,
					Title:   "计量结果",
					Snippet: fmt.Sprintf("共 %d 期", len(results)),
				})
			}
		}
	}

	// Journal entry queries (only if contract detail context hasn't already loaded them)
	if effectiveContractID == "" && containsAny(msgLower, []string{"分录", "journal", "会计", "过账", "post", "voucher", "entry", "凭证"}) {
		if req.ContractID != "" {
			entries, err := h.mcRepo.GetJournalEntries(ctx, req.ContractID, "", "")
			if err == nil && len(entries) > 0 {
				contextData.WriteString(fmt.Sprintf("\n## 会计分录（合同 %s，共 %d 条）\n", req.ContractID, len(entries)))
				for _, e := range entries {
					desc := ""
					if e.Description != nil {
						desc = *e.Description
					}
					contextData.WriteString(fmt.Sprintf("- 期间: %s, 类型: %s, 借方: %s, 贷方: %s, 金额: %.2f %s, 状态: %s, 描述: %s\n",
						e.AccountingPeriod, e.EntryType, e.DebitAccount, e.CreditAccount,
						e.Amount, e.Currency, e.PostingStatus, desc))
				}
				sources = append(sources, Source{
					Type:    "journal",
					ID:      req.ContractID,
					Title:   "会计分录",
					Snippet: fmt.Sprintf("共 %d 条", len(entries)),
				})
			}
		}
	}

	// Event queries (only if contract detail context hasn't already loaded them)
	if effectiveContractID == "" && containsAny(msgLower, []string{"事件", "变更", "modification", "reassessment", "impairment", "event", "change"}) {
		if req.ContractID != "" {
			events, err := h.eventRepo.GetByContractID(ctx, req.ContractID)
			if err == nil && len(events) > 0 {
				contextData.WriteString(fmt.Sprintf("\n## 事件数据（合同 %s，共 %d 条）\n", req.ContractID, len(events)))
				for _, ev := range events {
					reason := ""
					if ev.ChangeReason != nil {
						reason = *ev.ChangeReason
					}
					contextData.WriteString(fmt.Sprintf("- 类型: %s, 生效日: %s, 状态: %s, 审批状态: %s, 原因: %s\n",
						ev.EventType, ev.EffectiveDate.Format("2006-01-02"), ev.Status, ev.ApprovalStatus, reason))
				}
				sources = append(sources, Source{
					Type:    "event",
					ID:      req.ContractID,
					Title:   "变更事件",
					Snippet: fmt.Sprintf("共 %d 条", len(events)),
				})
			}
		}
	}

	// 4. If no context was built and no file was parsed, provide helpful guidance
	if contextData.Len() == 0 {
		contextData.WriteString(`
## 系统信息
当前系统暂无匹配的数据。建议用户：
- 指定具体的合同 ID 以查询计量结果和分录
- 询问合同列表（如"有哪些合同"）
- 上传合同文件或租金表进行 AI 解析
`)
	}

	if agentRunbook != nil && agentRunbook.RequiresEvidenceInput && req.FileID == "" && effectiveContractID == "" {
		resp := AIChatResponse{
			Answer:        agentRunbook.AnswerPrefix,
			Sources:       sources,
			Confidence:    0.85,
			IsOfficial:    false,
			Model:         "lease-agent",
			AgentMode:     true,
			AgentPlan:     agentRunbook.AgentPlan,
			ToolCalls:     agentRunbook.ToolCalls,
			ReviewPrompts: agentRunbook.ReviewPrompts,
		}
		h.finalizeRuntimeSuccess(ctx, runtimeState, &resp)
		return resp
	}

	// 5. Build system prompt
	isWorkingData := false
	if req.PageContext != nil && (req.PageContext.ReportView == "working" || req.PageContext.Page == "monthly-closing") {
		isWorkingData = true
	}
	systemPrompt := h.buildSystemPrompt(userIDStr, roleStr, legalEntityID, contextData.String(), isWorkingData, req.Language)

	// 6. Call AI Service
	_ = h.appendRuntimeEvent(ctx, runtimeState, "tool_start", map[string]interface{}{
		"tool":   "llm.chat_completion",
		"status": "running",
		"model":  "auto",
	}, false)
	answer, modelName, err := h.callLLM(ctx, authHeader, systemPrompt, req.Message, req.History, req.Language)
	if err != nil {
		// Fallback: return context data without LLM if AI Service is unavailable
		fallbackAnswer := fmt.Sprintf("（AI 服务暂不可用，以下为系统数据摘要）\n\n%s", contextData.String())
		if agentRunbook != nil {
			fallbackAnswer = agentRunbook.AnswerPrefix + "\n\n" + fallbackAnswer
		}
		resp := AIChatResponse{
			Answer:        fallbackAnswer,
			Sources:       sources,
			Confidence:    0.5,
			IsOfficial:    false,
			Model:         "fallback",
			AgentMode:     agentRunbook != nil,
			AgentPlan:     agentPlanFromRunbook(agentRunbook),
			ToolCalls:     toolCallsFromRunbook(agentRunbook),
			ReviewPrompts: reviewPromptsFromRunbook(agentRunbook),
		}
		attachRuntimeIDs(&resp, runtimeState)
		h.finalizeRuntimeFailure(ctx, runtimeState, resp.Answer, resp.Model, resp.ReviewPrompts, err)
		return resp
	}
	_ = h.appendRuntimeEvent(ctx, runtimeState, "tool_end", []map[string]interface{}{
		{
			"tool":   "llm.chat_completion",
			"status": "completed",
			"model":  modelName,
		},
	}, false)

	// 7. Extract sources from answer and merge with context sources.
	// If no source citations found in the answer, fall back to all known sources.
	extractedSources := extractSourcesFromAnswer(answer, sources)

	if agentRunbook != nil && agentRunbook.AnswerPrefix != "" {
		answer = agentRunbook.AnswerPrefix + "\n\n" + answer
	}

	resp := AIChatResponse{
		Answer:        answer,
		Sources:       extractedSources,
		Confidence:    0.9,
		IsOfficial:    false,
		Model:         modelName,
		AgentMode:     agentRunbook != nil,
		AgentPlan:     agentPlanFromRunbook(agentRunbook),
		ToolCalls:     toolCallsFromRunbook(agentRunbook),
		ReviewPrompts: reviewPromptsFromRunbook(agentRunbook),
	}
	h.finalizeRuntimeSuccess(ctx, runtimeState, &resp)
	return resp
}

func (h *AIChatHandler) Chat(c *gin.Context) {
	var req AIChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Language == "" {
		req.Language = "zh-CN"
	}

	ctx := c.Request.Context()
	legalEntityID := middleware.GetTenantID(c)
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	role, _ := c.Get("role")
	roleStr, _ := role.(string)
	effectiveContractID := effectiveContractIDFromRequest(req)
	agentRunbook := h.buildAgentRunbook(req, effectiveContractID)
	runtimeState, err := h.beginRuntimeState(c, &req, legalEntityID, userIDStr, effectiveContractID, agentRunbook)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to initialize ai chat runtime: " + err.Error()})
		return
	}

	resp := h.executeChatRequest(ctx, c.GetHeader("Authorization"), req, legalEntityID, userIDStr, roleStr, runtimeState, agentRunbook)
	c.JSON(http.StatusOK, resp)
}

func (h *AIChatHandler) buildAgentRunbook(req AIChatRequest, effectiveContractID string) *AgentRunbook {
	message := strings.ToLower(req.Message)
	hasFile := req.FileID != "" && req.ObjectName != ""
	hasContractContext := effectiveContractID != ""

	switch {
	case containsAny(message, []string{"审计包", "审计底稿", "审计工作底稿", "披露核对", "抽样", "audit pack", "audit package", "disclosure checklist"}):
		return buildAuditPackRunbook()
	case containsAny(message, []string{"租金表", "付款计划", "付款表", "rent schedule", "payment schedule", "rental schedule"}):
		return buildPaymentScheduleRunbook(hasFile, hasContractContext)
	case containsAny(message, []string{"合同复核", "合同审阅", "合同审核", "关键条款", "contract review", "review contract", "lease review"}):
		return buildContractReviewRunbook(hasFile, hasContractContext)
	case containsAny(message, []string{"excel 台账", "excel台账", "合同台账", "批量导入", "批量创建", "ledger import", "contract ledger"}):
		return buildContractLedgerRunbook(hasFile)
	default:
		return nil
	}
}

func buildContractLedgerRunbook(hasFile bool) *AgentRunbook {
	evidenceStatus := "needs_review"
	parserStatus := "pending"
	prefix := "Agent 已准备执行 Excel 台账导入技能。请上传合同台账文件后，我会先用 Office skill 展开工作簿，再由 AI Agent 进行语义理解并生成合同草稿；不会直接写入正式台账。"
	reviewPrompts := []AgentReviewPrompt{
		{ID: "upload_ledger", Title: "上传合同台账", Description: "需要 Excel、CSV 或可解析的台账文件作为证据输入。", Severity: "info", Action: "上传文件后再次发送导入指令。"},
	}
	if hasFile {
		evidenceStatus = "completed"
		parserStatus = "needs_review"
		prefix = "Agent 正在按 Excel 台账导入技能处理上传文件。"
		reviewPrompts = []AgentReviewPrompt{
			{ID: "ledger_review", Title: "复核台账草稿", Description: "上传文件已作为证据输入。下一步需要核对草稿字段、折现率和租赁范围判断。", Severity: "warning", Action: "确认字段无误后再创建 draft 合同。"},
		}
	}
	return &AgentRunbook{
		SkillID:               "excel_ledger",
		SkillName:             "Office Excel Ledger Skill",
		RequiresEvidenceInput: !hasFile,
		AnswerPrefix:          prefix,
		AgentPlan: []AgentPlanStep{
			{ID: "read_workbook", Title: "读取并展开 Office 工作簿", Status: evidenceStatus},
			{ID: "understand_columns", Title: "理解非标准字段、表头和合并单元格", Status: parserStatus},
			{ID: "human_review", Title: "人工确认缺失字段、折现率和租赁范围", Status: "needs_review"},
			{ID: "create_drafts", Title: "确认后创建 draft 合同", Status: "pending"},
		},
		ToolCalls: []AgentToolCall{
			{Tool: "office.excel_reader", Skill: "Office Excel Skill", Status: evidenceStatus, InputSummary: "等待或读取上传的 Excel 台账", OutputSummary: "将工作簿展开为带 sheet、行列坐标和单元格值的证据文本", RequiresReview: !hasFile},
			{Tool: "lease.contract_batch_parser", Skill: "Lease Intake Skill", Status: parserStatus, InputSummary: "从非标准台账中抽取合同、门店、出租方、日期、租金和范围判断", OutputSummary: "生成合同草稿和字段级复核清单", RequiresReview: true},
			{Tool: "lease.draft_contract_creator", Skill: "Core Service Draft Skill", Status: "pending", InputSummary: "等待用户确认草稿", OutputSummary: "只创建 draft 合同，正式入库仍走审批", RequiresReview: true},
		},
		ReviewPrompts: reviewPrompts,
	}
}

func buildContractReviewRunbook(hasFile, hasContractContext bool) *AgentRunbook {
	hasEvidence := hasFile || hasContractContext
	evidenceStatus := statusFromReview(!hasEvidence)
	reviewPrompts := []AgentReviewPrompt{
		{ID: "contract_evidence", Title: "提供合同证据", Description: "合同复核需要上传合同文件，或在合同详情页指定当前合同。", Severity: "info", Action: "上传 PDF/Word/Excel 合同文件，或进入某份合同详情后提问。"},
	}
	if hasEvidence {
		reviewPrompts = []AgentReviewPrompt{
			{ID: "contract_judgement", Title: "复核关键会计判断", Description: "已具备合同证据。请重点确认续租/终止选择权、非租赁成分、折现率来源和租赁范围。", Severity: "warning", Action: "根据 Agent 提取结果逐项确认判断依据。"},
		}
	}
	return &AgentRunbook{
		SkillID:               "contract_review",
		SkillName:             "Office Contract Review Skill",
		RequiresEvidenceInput: !hasEvidence,
		AnswerPrefix:          "Agent 已切换到合同复核技能。我会提取关键条款、识别 IFRS 16 风险点，并把需要人工判断的事项列为复核提示。",
		AgentPlan: []AgentPlanStep{
			{ID: "collect_evidence", Title: "读取合同文件或当前合同上下文", Status: evidenceStatus},
			{ID: "extract_terms", Title: "抽取租赁期限、付款、选择权和非租赁成分", Status: statusFromReview(!hasEvidence)},
			{ID: "accounting_review", Title: "生成 IFRS 16 复核事项", Status: "needs_review"},
			{ID: "prepare_notes", Title: "形成可追溯复核结论", Status: "pending"},
		},
		ToolCalls: []AgentToolCall{
			{Tool: "office.document_reader", Skill: "Office/PDF Document Skill", Status: evidenceStatus, InputSummary: "读取合同文件或当前合同详情", OutputSummary: "提取条款证据文本和可引用来源", RequiresReview: !hasEvidence},
			{Tool: "lease.term_reviewer", Skill: "Lease Review Skill", Status: statusFromReview(!hasEvidence), InputSummary: "识别期限、付款、续租/终止选择权、CAM/服务费、折现率线索", OutputSummary: "输出字段级风险和人工复核点", RequiresReview: true},
		},
		ReviewPrompts: reviewPrompts,
	}
}

func buildPaymentScheduleRunbook(hasFile, hasContractContext bool) *AgentRunbook {
	hasEvidence := hasFile || hasContractContext
	evidenceStatus := statusFromReview(!hasEvidence)
	reviewPrompts := []AgentReviewPrompt{
		{ID: "schedule_evidence", Title: "提供租金表证据", Description: "付款计划导入需要租金表、合同付款页或当前合同上下文。", Severity: "info", Action: "上传 Excel/PDF 租金表，或进入合同详情后提问。"},
	}
	if hasEvidence {
		reviewPrompts = []AgentReviewPrompt{
			{ID: "schedule_review", Title: "复核付款计划判断", Description: "已具备付款证据。请重点确认先付/后付、变量租金、非租赁成分和缺失期间。", Severity: "warning", Action: "确认后再导入付款计划草稿。"},
		}
	}
	return &AgentRunbook{
		SkillID:               "payment_schedule",
		SkillName:             "Office Rent Schedule Skill",
		RequiresEvidenceInput: !hasEvidence,
		AnswerPrefix:          "Agent 已切换到租金表技能。我会先理解付款表结构，再区分先付/后付、固定租金、变量租金和非租赁成分。",
		AgentPlan: []AgentPlanStep{
			{ID: "read_schedule", Title: "读取租金表或合同付款上下文", Status: evidenceStatus},
			{ID: "classify_payments", Title: "分类付款时点和会计属性", Status: statusFromReview(!hasEvidence)},
			{ID: "human_review", Title: "人工确认异常金额、缺失期间和变量租金", Status: "needs_review"},
			{ID: "import_schedule", Title: "确认后导入付款计划草稿", Status: "pending"},
		},
		ToolCalls: []AgentToolCall{
			{Tool: "office.sheet_reader", Skill: "Office Excel Skill", Status: evidenceStatus, InputSummary: "读取上传的租金表或合同付款上下文", OutputSummary: "展开期间、金额、币种、税额和备注字段", RequiresReview: !hasEvidence},
			{Tool: "lease.payment_schedule_parser", Skill: "Payment Schedule Skill", Status: statusFromReview(!hasEvidence), InputSummary: "抽取付款计划并判断资本化属性", OutputSummary: "生成付款计划草稿和异常清单", RequiresReview: true},
		},
		ReviewPrompts: reviewPrompts,
	}
}

func buildAuditPackRunbook() *AgentRunbook {
	return &AgentRunbook{
		SkillID:               "audit_pack",
		SkillName:             "Lease Audit Pack Skill",
		NeedsPortfolioContext: true,
		AnswerPrefix:          "Agent 已切换到审计包技能。我会基于权限范围内的系统数据生成审计资料清单、抽样建议和数据质量提示；正式审计结论仍需人工复核。",
		AgentPlan: []AgentPlanStep{
			{ID: "define_scope", Title: "确认审计期间、法人和报表口径", Status: "needs_review"},
			{ID: "collect_system_data", Title: "检索合同、计量、分录和审批状态", Status: "completed"},
			{ID: "quality_checks", Title: "识别缺失字段、未审批和口径差异", Status: "needs_review"},
			{ID: "prepare_pack", Title: "生成审计包目录和导出建议", Status: "pending"},
		},
		ToolCalls: []AgentToolCall{
			{Tool: "core.contract_repository", Skill: "Lease Data Retrieval Skill", Status: "completed", InputSummary: "读取当前用户权限范围内的合同组合", OutputSummary: "形成审计包覆盖范围和抽样基础", RequiresReview: false},
			{Tool: "lease.audit_pack_builder", Skill: "Audit Pack Skill", Status: "needs_review", InputSummary: "整理合同清单、计量结果、分录、审批和附件需求", OutputSummary: "等待用户确认审计期间和 Official/Working 口径", RequiresReview: true},
		},
		ReviewPrompts: []AgentReviewPrompt{
			{ID: "audit_scope", Title: "确认审计范围", Description: "生成审计包前需要确认法人、期间、Official/Working 口径和抽样规则。", Severity: "warning", Action: "指定期间和口径，例如：生成 2024-01 至 2024-12 Official 审计包。"},
		},
	}
}

func (h *AIChatHandler) appendAgentPortfolioContext(ctx context.Context, legalEntityID string, contextData *strings.Builder, sources *[]Source) {
	contracts, err := h.contractRepo.GetAll(ctx, legalEntityID)
	if err != nil || len(contracts) == 0 {
		contextData.WriteString("\n## Agent 组合数据\n当前权限范围内未检索到合同组合数据。\n")
		return
	}

	statusCounts := make(map[string]int)
	showCount := len(contracts)
	if showCount > 20 {
		showCount = 20
	}
	for _, contract := range contracts {
		statusCounts[contract.ApprovalStatus]++
	}

	contextData.WriteString(fmt.Sprintf("\n## Agent 组合数据（共 %d 份合同，显示前 %d 份）\n", len(contracts), showCount))
	contextData.WriteString("- 审批状态分布:")
	for status, count := range statusCounts {
		contextData.WriteString(fmt.Sprintf(" %s=%d", status, count))
	}
	contextData.WriteString("\n")
	for i, contract := range contracts {
		if i >= showCount {
			break
		}
		contextData.WriteString(fmt.Sprintf("- ID: %s, 编号: %s, 名称: %s, 审批状态: %s, 承租方: %s, 出租方: %s, 门店: %s\n",
			contract.ID, contract.ContractNumber, contract.ContractName, contract.ApprovalStatus, contract.LesseeName, contract.LessorName, contract.StoreName))
		*sources = append(*sources, Source{
			Type:    "contract",
			ID:      contract.ID,
			Title:   contract.ContractName,
			Snippet: fmt.Sprintf("编号: %s, 状态: %s", contract.ContractNumber, contract.ApprovalStatus),
		})
	}
}

func agentPlanFromRunbook(runbook *AgentRunbook) []AgentPlanStep {
	if runbook == nil {
		return nil
	}
	return runbook.AgentPlan
}

func toolCallsFromRunbook(runbook *AgentRunbook) []AgentToolCall {
	if runbook == nil {
		return nil
	}
	return runbook.ToolCalls
}

func reviewPromptsFromRunbook(runbook *AgentRunbook) []AgentReviewPrompt {
	if runbook == nil {
		return nil
	}
	return runbook.ReviewPrompts
}

// retrieveContractDetail fetches and appends contract basic info, latest measurements,
// events, and latest journal entries for a given contract ID.
func (h *AIChatHandler) retrieveContractDetail(ctx context.Context, contractID, legalEntityID string, contextData *strings.Builder, sources *[]Source) {
	// Contract basic info
	contract, err := h.contractRepo.GetByID(ctx, contractID, legalEntityID)
	if err == nil && contract != nil {
		contextData.WriteString(fmt.Sprintf("\n## 合同详情\n- ID: %s\n- 编号: %s\n- 名称: %s\n- 状态: %s\n- 审批状态: %s\n- 承租方: %s\n- 出租方: %s\n- 门店: %s\n- 币种: %s\n",
			contract.ID, contract.ContractNumber, contract.ContractName,
			contract.ApprovalStatus, contract.Status, contract.LesseeName,
			contract.LessorName, contract.StoreName, contract.Currency))
		if contract.CommencementDate.Year() > 1 {
			contextData.WriteString(fmt.Sprintf("- 租赁开始日: %s\n", contract.CommencementDate.Format("2006-01-02")))
		}
		if contract.LeaseEndDate.Year() > 1 {
			contextData.WriteString(fmt.Sprintf("- 租赁结束日: %s\n", contract.LeaseEndDate.Format("2006-01-02")))
		}
		if contract.DiscountRateValue != nil {
			contextData.WriteString(fmt.Sprintf("- 折现率: %.4f%%\n", *contract.DiscountRateValue*100))
		}
		if contract.DiscountRateMissing {
			contextData.WriteString("- ⚠️ 折现率缺失，需要人工补充\n")
		}
		*sources = append(*sources, Source{
			Type:    "contract",
			ID:      contract.ID,
			Title:   contract.ContractName,
			Snippet: fmt.Sprintf("编号: %s, 状态: %s", contract.ContractNumber, contract.ApprovalStatus),
		})
	}

	// Latest measurement results
	results, err := h.mcRepo.GetMeasurementResults(ctx, contractID, "")
	if err == nil && len(results) > 0 {
		contextData.WriteString(fmt.Sprintf("\n## 计量结果（合同 %s，共 %d 期）\n", contractID, len(results)))
		for _, r := range results {
			contextData.WriteString(fmt.Sprintf("- 期间: %s, 期初负债: %.2f, 期末负债: %.2f, 利息: %.2f, 本金偿还: %.2f, 折旧: %.2f, 期末ROU: %.2f\n",
				r.AccountingPeriod, r.OpeningLiability, r.ClosingLiability, r.InterestExpense,
				r.PrincipalRepayment, r.Depreciation, r.ClosingROUAsset))
		}
		*sources = append(*sources, Source{
			Type:    "measurement",
			ID:      contractID,
			Title:   "计量结果",
			Snippet: fmt.Sprintf("合同 %s 共 %d 期", contractID, len(results)),
		})
	}

	// Events for this contract
	events, err := h.eventRepo.GetByContractID(ctx, contractID)
	if err == nil && len(events) > 0 {
		contextData.WriteString(fmt.Sprintf("\n## 变更事件（合同 %s，共 %d 条）\n", contractID, len(events)))
		for _, ev := range events {
			reason := ""
			if ev.ChangeReason != nil {
				reason = *ev.ChangeReason
			}
			contextData.WriteString(fmt.Sprintf("- 类型: %s, 生效日: %s, 状态: %s, 审批状态: %s, 原因: %s\n",
				ev.EventType, ev.EffectiveDate.Format("2006-01-02"), ev.Status, ev.ApprovalStatus, reason))
		}
		*sources = append(*sources, Source{
			Type:    "event",
			ID:      contractID,
			Title:   "变更事件",
			Snippet: fmt.Sprintf("合同 %s 共 %d 条", contractID, len(events)),
		})
	}

	// Latest journal entries for this contract
	entries, err := h.mcRepo.GetJournalEntries(ctx, contractID, "", "")
	if err == nil && len(entries) > 0 {
		contextData.WriteString(fmt.Sprintf("\n## 会计分录（合同 %s，共 %d 条）\n", contractID, len(entries)))
		for _, e := range entries {
			desc := ""
			if e.Description != nil {
				desc = *e.Description
			}
			contextData.WriteString(fmt.Sprintf("- 期间: %s, 类型: %s, 借方: %s, 贷方: %s, 金额: %.2f %s, 状态: %s, 描述: %s\n",
				e.AccountingPeriod, e.EntryType, e.DebitAccount, e.CreditAccount,
				e.Amount, e.Currency, e.PostingStatus, desc))
		}
		*sources = append(*sources, Source{
			Type:    "journal",
			ID:      contractID,
			Title:   "会计分录",
			Snippet: fmt.Sprintf("合同 %s 共 %d 条", contractID, len(entries)),
		})
	}
}

// appendReportsContext appends report page context info from the frontend payload.
// It does not query the reports endpoint internally; it uses the provided context only.
func (h *AIChatHandler) appendReportsContext(pc *PageContext, contextData *strings.Builder) {
	contextData.WriteString("\n## 当前报表上下文\n")
	if pc.Title != "" {
		contextData.WriteString(fmt.Sprintf("- 报表标题: %s\n", pc.Title))
	}
	if pc.ReportView != "" {
		contextData.WriteString(fmt.Sprintf("- 报表口径: %s\n", pc.ReportView))
	}
	if pc.Period != "" {
		contextData.WriteString(fmt.Sprintf("- 覆盖期间: %s\n", pc.Period))
	}
	if len(pc.Filters) > 0 {
		contextData.WriteString("- 筛选条件:\n")
		for k, v := range pc.Filters {
			contextData.WriteString(fmt.Sprintf("  - %s: %s\n", k, v))
		}
	}
	if pc.Summary != "" {
		contextData.WriteString(fmt.Sprintf("- 摘要: %s\n", pc.Summary))
	}
}

// appendMonthlyClosingContext appends monthly closing page context and queries
// journal entries for the selected period if provided.
func (h *AIChatHandler) appendMonthlyClosingContext(ctx context.Context, pc *PageContext, contextData *strings.Builder, sources *[]Source) {
	contextData.WriteString("\n## 当前结账中心上下文\n")
	if pc.Period != "" {
		contextData.WriteString(fmt.Sprintf("- 选定期间: %s\n", pc.Period))
	}
	if pc.Summary != "" {
		contextData.WriteString(fmt.Sprintf("- 摘要: %s\n", pc.Summary))
	}

	if pc.Period != "" {
		entries, err := h.mcRepo.GetJournalEntries(ctx, "", pc.Period, "")
		if err == nil && len(entries) > 0 {
			showCount := len(entries)
			if showCount > 20 {
				showCount = 20
			}
			postedCount := 0
			for _, e := range entries {
				if e.PostingStatus == "posted" {
					postedCount++
				}
			}
			contextData.WriteString(fmt.Sprintf("\n### 期间 %s 的会计分录（共 %d 条，显示前 %d 条）\n", pc.Period, len(entries), showCount))
			for i, e := range entries {
				if i >= showCount {
					break
				}
				desc := ""
				if e.Description != nil {
					desc = *e.Description
				}
				contextData.WriteString(fmt.Sprintf("- 合同: %s, 类型: %s, 借方: %s, 贷方: %s, 金额: %.2f %s, 状态: %s, 描述: %s\n",
					e.ContractID, e.EntryType, e.DebitAccount, e.CreditAccount,
					e.Amount, e.Currency, e.PostingStatus, desc))
			}
			contextData.WriteString(fmt.Sprintf("\n汇总: 共 %d 条分录，其中 %d 条已过账\n", len(entries), postedCount))
			*sources = append(*sources, Source{
				Type:    "journal",
				ID:      pc.Period,
				Title:   fmt.Sprintf("期间 %s 会计分录", pc.Period),
				Snippet: fmt.Sprintf("共 %d 条", len(entries)),
			})
		}
	}
}

// buildSystemPrompt constructs the structured system prompt for the LLM.
func (h *AIChatHandler) buildSystemPrompt(userID, role, legalEntityID, contextData string, isWorkingData bool, language string) string {
	workingWarning := ""
	if isWorkingData {
		workingWarning = "\n⚠️ 注意：当前页面展示的是 Working/试算数据，这不是 Official 口径，请在回答中提醒用户。\n"
	}

	// Language instruction
	var langInstruction string
	switch language {
	case "en":
		langInstruction = "\n\nPlease answer in English."
	case "zh-TW":
		langInstruction = "\n\n請用繁體中文回答。"
	default: // zh-CN
		langInstruction = "\n\n请用简体中文回答。"
	}

	return fmt.Sprintf(`你是 IFRS 16 租赁管理系统的 AI Agent，协助会计师和审计人员工作。准确性、可追溯性和专业审慎比速度更重要。

当前用户: %s
角色: %s
可访问法人: %s
%s
%s
%s

## 核心原则

- 你不是普通聊天机器人。你应把用户目标拆解为可执行步骤，并说明需要调用或已经调用的系统工具/Office skill
- Office skill 只负责读取、展开或生成文件；字段语义、会计判断和风险提示由 AI Agent 完成；正式入库和计量由 Core Service 规则控制
- 只能基于以上系统数据回答问题，不得编造任何数字、日期、会计分录、准则引用或审计结论
- 严格区分：事实（系统数据）、假设（用户提供的参数）、计算结果（系统输出的计量数值）、会计估计（如折现率、租赁期限判断）、专业判断（如分类建议）
- 保留原始数值，不得擅自四舍五入、归并、重分类或净额列示（例如 ¥3,255,676.79 不得写成约 ¥326万）
- 当不确定性影响财务报告或合规判断时，优先保守解释
- 在做出重大假设前，先向用户确认

## 证据优先

- 每条结论必须可追溯到系统中的具体合同、计量结果、会计分录或付款计划
- 引用数据时标注：来源类型、编号/期间、版本状态（Approved/Draft/Pending）
- 汇总财务数据时，注明涵盖的合同范围、期间、币种和数据口径
- 发现以下情况时必须明确标注：
  - 数据缺失或不完整
  - 数值不一致或无法勾稽
  - 日期逻辑异常（如开始日晚于结束日）
  - 审批状态异常（如已过账但分录仍为草稿）
- 不确定时使用审慎措辞："根据当前系统数据..."、"未看到支持...的证据"、"这表明..."、"在得出结论前需要进一步核实..."

## IFRS 16 会计处理

- 在生成或解释会计分录前，先确认以下要素：
  - 交易/事件日期
  - 涉及的合同和法人主体
  - 金额和币种
  - 相关科目（租赁负债、使用权资产、利息费用、折旧费用等）
  - 适用的会计准则（IFRS 16 / CAS 21）
  - 支撑证据（合同条款、付款计划、事件记录）
- 会计分录需展示借贷方、金额、合计，并附简要说明
- 若科目分类不确定，列出现有选项并解释各选项的影响
- 不得代替用户审批、过账或确认分录

## 计量与计算

- 涉及重要计算时（租赁负债现值、利息摊销、折旧、重估调整），展示公式和中间步骤
- 尽可能将合计数与源头数据勾稽核对
- 发现差异时高亮显示，而非隐藏
- 若涉及会计估计（折现率、租赁期限、残值等），明确指出：输入值、方法、假设前提及敏感性

## 回答结构

严格遵循以下结构：

【结论】
用 1-3 句话给出直接回答。

【依据】
逐条列出支撑结论的事实，每条标注来源，格式为：
- ...（来源：合同 LEASE-XXXX-XXXX，状态：Approved）
- ...（来源：计量结果 2024-01）
- ...（来源：会计分录 2024-01，状态：Posted）
- ...（来源：AI 推断，非系统正式数据）← 如涉及推断必须标注

【数据质量提示】
如发现以下问题，在此列出：
- 缺失数据（缺少哪类数据）
- 不一致或异常（具体描述）
- Working/试算数据提醒（非 Official 口径）
如无问题，标注"数据完整，未发现异常"。

【建议动作】
根据数据和结论，给出具体可执行的建议操作，例如：
- 去补录折现率（合同 XXX 缺少折现率）
- 去执行 IFRS 16 重算（因事件变更尚未重算）
- 去查看摊销报表（核对利息和折旧）
- 去审批待处理事件（X 条事件待审批）
- 无需操作，数据正常

## 边界

- 不得签署、认证、审批或替代持牌专业人员的工作
- 不得代替管理层做决策
- 不得篡改原始数据
- 不得隐瞒错误、绕过控制或协助粉饰财务信息
- 如被要求执行误导性、不合规或无依据的操作，必须拒绝并建议合规替代方案`, userID, role, legalEntityID, workingWarning, langInstruction, contextData)
}

// fileParseTools defines the available tools for LLM function calling when a file is uploaded.
var fileParseTools = []map[string]interface{}{
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "parse_contract_batch",
			"description": "解析合同台账文件（Excel/PDF），批量提取多份合同草稿。当用户上传包含多份合同的台账文件，或提到'台账'、'批量'、'导入多份合同'、'ledger'时使用。",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_id":      map[string]interface{}{"type": "string", "description": "上传文件的ID"},
					"object_name":  map[string]interface{}{"type": "string", "description": "文件在MinIO中的对象名"},
					"content_type": map[string]interface{}{"type": "string", "description": "文件MIME类型"},
				},
				"required": []string{"file_id", "object_name", "content_type"},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "parse_payment_schedule",
			"description": "解析租金表/付款计划文件。当用户上传租金表、付款计划、rent schedule，或提到'租金'、'付款'、'payment schedule'时使用。",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_id":      map[string]interface{}{"type": "string", "description": "上传文件的ID"},
					"object_name":  map[string]interface{}{"type": "string", "description": "文件在MinIO中的对象名"},
					"content_type": map[string]interface{}{"type": "string", "description": "文件MIME类型"},
					"contract_id":  map[string]interface{}{"type": "string", "description": "关联的合同ID（可选）"},
				},
				"required": []string{"file_id", "object_name", "content_type"},
			},
		},
	},
	{
		"type": "function",
		"function": map[string]interface{}{
			"name":        "parse_contract",
			"description": "解析单个合同文件（PDF/Word/图片），提取合同字段生成草稿。当用户上传单份合同文件，且未明确提到'台账'或'批量'时使用。",
			"parameters": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_id":      map[string]interface{}{"type": "string", "description": "上传文件的ID"},
					"object_name":  map[string]interface{}{"type": "string", "description": "文件在MinIO中的对象名"},
					"content_type": map[string]interface{}{"type": "string", "description": "文件MIME类型"},
				},
				"required": []string{"file_id", "object_name", "content_type"},
			},
		},
	},
}

// callLLM sends the prompt and message history to the AI Service chat endpoint.
func (h *AIChatHandler) callLLM(ctx context.Context, authHeader, systemPrompt, userMessage string, history []ChatMessage, language string) (string, string, error) {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://ai-service:8000"
	}

	// Build messages from history + current message
	messages := []map[string]string{}
	for _, h := range history {
		messages = append(messages, map[string]string{"role": h.Role, "content": h.Content})
	}
	messages = append(messages, map[string]string{"role": "user", "content": userMessage})

	reqBody, err := json.Marshal(map[string]interface{}{
		"messages":      messages,
		"system_prompt": systemPrompt,
		"temperature":   0.3,
		"max_tokens":    2000,
		"language":      language,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", aiServiceURL+"/api/v1/chat", bytes.NewReader(reqBody))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq = httpReq.WithContext(ctx)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", "", fmt.Errorf("AI service unreachable: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("AI service returned %d: %s", resp.StatusCode, string(body))
	}

	var result AIChatResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Answer, result.Model, nil
}

// callLLMWithTools sends the prompt and message history to the AI Service chat endpoint with function calling support.
// It returns the LLM answer, model name, and any tool_calls if the LLM decided to invoke tools.
func (h *AIChatHandler) callLLMWithTools(ctx context.Context, authHeader, systemPrompt, userMessage string, history []ChatMessage, language string, tools []map[string]interface{}) (string, string, []AgentToolCall, error) {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://ai-service:8000"
	}

	// Build messages from history + current message
	messages := []map[string]string{}
	for _, h := range history {
		messages = append(messages, map[string]string{"role": h.Role, "content": h.Content})
	}
	messages = append(messages, map[string]string{"role": "user", "content": userMessage})

	reqBody, err := json.Marshal(map[string]interface{}{
		"messages":      messages,
		"system_prompt": systemPrompt,
		"temperature":   0.1,
		"max_tokens":    2000,
		"language":      language,
		"tools":         tools,
		"tool_choice":   "auto",
	})
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", aiServiceURL+"/api/v1/chat", bytes.NewReader(reqBody))
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq = httpReq.WithContext(ctx)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return "", "", nil, fmt.Errorf("AI service unreachable: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", "", nil, fmt.Errorf("AI service returned %d: %s", resp.StatusCode, string(body))
	}

	var result LLMChatResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert tool_calls to AgentToolCall format
	var agentToolCalls []AgentToolCall
	if result.ToolCalls != nil {
		for _, tc := range result.ToolCalls {
			funcName := ""
			funcArgs := ""
			if tc.Function != nil {
				if name, ok := tc.Function["name"].(string); ok {
					funcName = name
				}
				if args, ok := tc.Function["arguments"].(string); ok {
					funcArgs = args
				}
			}
			agentToolCalls = append(agentToolCalls, AgentToolCall{
				Tool:           funcName,
				Skill:          "LLM Function Calling",
				Status:         "completed",
				InputSummary:   funcArgs,
				OutputSummary:  "等待执行",
				RequiresReview: false,
			})
		}
	}

	return result.Answer, result.Model, agentToolCalls, nil
}

// extractSourcesFromAnswer parses the LLM answer for source citations and matches them
// against the known sources from context retrieval. It returns a deduplicated slice.
func extractSourcesFromAnswer(answer string, knownSources []Source) []Source {
	if len(knownSources) == 0 {
		return knownSources
	}

	// Find source citations in the answer, pattern: （来源：XXX）
	re := regexp.MustCompile(`（来源[：:]\s*([^）)]+)）`)
	matches := re.FindAllStringSubmatch(answer, -1)

	if len(matches) == 0 {
		return knownSources
	}

	// Build a set of cited source tokens
	citedTokens := make(map[string]bool)
	for _, m := range matches {
		if len(m) > 1 {
			token := strings.TrimSpace(strings.ToLower(m[1]))
			citedTokens[token] = true
		}
	}

	// Find which known sources were cited
	seen := make(map[string]bool)
	var cited []Source
	for _, s := range knownSources {
		// Check if any part of the source (id, title, snippet) matches a citation
		isCited := false
		sourceTokens := strings.ToLower(s.ID + " " + s.Title + " " + s.Snippet)
		for token := range citedTokens {
			if strings.Contains(sourceTokens, token) {
				isCited = true
				break
			}
		}
		if isCited && !seen[s.ID] {
			cited = append(cited, s)
			seen[s.ID] = true
		}
	}

	// If no sources matched citations but citations exist, return known sources
	if len(cited) == 0 && len(matches) > 0 {
		return knownSources
	}
	// If nothing was cited, return known sources (LLM used context data)
	if len(cited) == 0 {
		return knownSources
	}

	return cited
}

func (h *AIChatHandler) parseFile(ctx context.Context, authHeader, fileID, objectName, contentType string) (map[string]interface{}, error) {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://ai-service:8000"
	}

	reqBody, err := json.Marshal(map[string]interface{}{
		"file_id":      fileID,
		"object_name":  objectName,
		"content_type": contentType,
		"mode":         "assist",
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", aiServiceURL+"/api/v1/parse/contract", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI service returned %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	// Extract the extracted_data field
	if data, ok := result["extracted_data"].(map[string]interface{}); ok {
		return data, nil
	}

	return result, nil
}

type PaymentScheduleParseResult struct {
	SummaryText   string
	Sources       []Source
	Confidence    float64
	AgentPlan     []AgentPlanStep
	ToolCalls     []AgentToolCall
	ReviewPrompts []AgentReviewPrompt
	Schedules     []PaymentScheduleDraftItem
	Summary       *PaymentScheduleParseSummary
}

func (h *AIChatHandler) parsePaymentSchedule(ctx context.Context, authHeader, fileID, objectName, contentType, contractID string) (*PaymentScheduleParseResult, error) {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://ai-service:8000"
	}

	reqBody, err := json.Marshal(map[string]interface{}{
		"file_id":      fileID,
		"object_name":  objectName,
		"content_type": contentType,
		"mode":         "assist",
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", aiServiceURL+"/api/v1/parse/payment-schedule", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI service returned %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	schedulesRaw, _ := result["schedules"].([]interface{})
	schedules := make([]PaymentScheduleDraftItem, 0, len(schedulesRaw))
	for _, sr := range schedulesRaw {
		smap, ok := sr.(map[string]interface{})
		if !ok {
			continue
		}
		item := PaymentScheduleDraftItem{
			PeriodStart:      getString(smap, "period_start"),
			PeriodEnd:        getString(smap, "period_end"),
			DueDate:          getString(smap, "due_date"),
			Amount:           getFloat64(smap, "amount"),
			PaymentTiming:    getString(smap, "payment_timing"),
			IsFixed:          getBoolDefault(smap, "is_fixed", true),
			IsLeaseComponent: getBoolDefault(smap, "is_lease_component", true),
			AmountType:       getString(smap, "amount_type"),
			Currency:         getString(smap, "currency"),
			Confidence:       getFloat64(smap, "confidence"),
		}
		if item.PaymentTiming == "" {
			item.PaymentTiming = "postpaid"
		}
		if item.AmountType == "" {
			item.AmountType = "fixed_rent"
		}
		if item.Confidence == 0 {
			item.Confidence = 0.8
		}
		schedules = append(schedules, item)
	}

	confidenceScores, _ := result["confidence_scores"].(map[string]interface{})
	overallConf := getFloat64(confidenceScores, "overall")
	if overallConf == 0 {
		overallConf = 0.8
	}
	requiresHuman := getBool(result, "requires_human_confirmation")
	missingFields := getStringSlice(result, "missing_fields")
	warnings := getStringSlice(result, "warnings")
	canImport := contractID != "" && len(schedules) > 0

	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Agent 已完成租金表理解和付款计划草稿生成：识别出 **%d** 笔付款计划。\n\n", len(schedules)))
	if contractID == "" {
		summary.WriteString("⚠️ **尚未绑定合同**：付款计划必须挂到具体合同后才能入库。请在合同详情页进入 AI Chat，或在 URL 中带上 `contract_id` 后再确认导入。\n\n")
	}
	if requiresHuman || len(warnings) > 0 || len(missingFields) > 0 {
		summary.WriteString("⚠️ **需要人工确认**：请重点核对覆盖期间、应付日、金额、先付/后付、变量租金和非租赁成分。\n\n")
	} else {
		summary.WriteString("识别结果总体良好，仍需人工快速核对后才能导入。\n\n")
	}
	summary.WriteString("下方付款计划草稿只处于 Assist Mode。确认导入后也只写入当前合同付款计划，不会绕过合同审批、计量重算和月结控制。")

	planStatus := statusFromReview(requiresHuman || len(warnings) > 0 || len(missingFields) > 0)
	importStatus := "pending"
	if !canImport {
		importStatus = "needs_review"
	}
	toolCalls := []AgentToolCall{
		{
			Tool:           "office.sheet_reader",
			Skill:          "Office Excel/PDF Skill",
			Status:         "completed",
			InputSummary:   fmt.Sprintf("读取上传租金表 %s", objectName),
			OutputSummary:  "已将租金表展开为可供 LLM 理解的付款证据",
			RequiresReview: false,
		},
		{
			Tool:           "lease.payment_schedule_parser",
			Skill:          "Payment Schedule Skill",
			Status:         planStatus,
			InputSummary:   "识别覆盖期间、应付日、金额、先付/后付和会计属性",
			OutputSummary:  fmt.Sprintf("生成 %d 笔付款计划草稿，缺失字段 %d 类，警告 %d 条", len(schedules), len(missingFields), len(warnings)),
			RequiresReview: planStatus == "needs_review",
		},
		{
			Tool:           "lease.payment_schedule_importer",
			Skill:          "Core Service Payment Schedule Skill",
			Status:         importStatus,
			InputSummary:   "等待用户确认付款计划草稿",
			OutputSummary:  "确认后写入指定合同的付款计划",
			RequiresReview: true,
		},
	}
	agentPlan := []AgentPlanStep{
		{ID: "read_schedule", Title: "读取租金表并保留证据", Status: "completed"},
		{ID: "classify_payments", Title: "理解付款期间、金额和会计属性", Status: planStatus},
		{ID: "human_review", Title: "人工确认异常金额、缺失期间和变量租金", Status: "needs_review"},
		{ID: "import_schedule", Title: "确认后导入付款计划", Status: importStatus},
	}

	return &PaymentScheduleParseResult{
		SummaryText:   summary.String(),
		Sources:       []Source{{Type: "file", ID: fileID, Title: "租金表文件", Snippet: objectName}},
		Confidence:    overallConf,
		AgentPlan:     agentPlan,
		ToolCalls:     toolCalls,
		ReviewPrompts: buildPaymentScheduleReviewPrompts(schedules, missingFields, warnings, contractID),
		Schedules:     schedules,
		Summary: &PaymentScheduleParseSummary{
			TotalCount:           len(schedules),
			OverallConfidence:    overallConf,
			RequiresHumanConfirm: requiresHuman,
			MissingFields:        missingFields,
			Warnings:             warnings,
			CanImport:            canImport,
			ContractID:           contractID,
		},
	}, nil
}

type BatchParseResult struct {
	SummaryText   string
	Sources       []Source
	Confidence    float64
	AgentPlan     []AgentPlanStep
	ToolCalls     []AgentToolCall
	ReviewPrompts []AgentReviewPrompt
	Contracts     []ContractDraftItem
	Summary       *BatchParseSummary
}

func (h *AIChatHandler) parseContractBatch(ctx context.Context, authHeader, fileID, objectName, contentType string) (*BatchParseResult, error) {
	aiServiceURL := os.Getenv("AI_SERVICE_URL")
	if aiServiceURL == "" {
		aiServiceURL = "http://ai-service:8000"
	}

	reqBody, err := json.Marshal(map[string]interface{}{
		"file_id":      fileID,
		"object_name":  objectName,
		"content_type": contentType,
		"mode":         "assist",
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", aiServiceURL+"/api/v1/parse/contract-batch", bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 180 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AI service returned %d: %s", resp.StatusCode, string(body))
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	contractsRaw, _ := result["contracts"].([]interface{})
	contracts := make([]ContractDraftItem, 0, len(contractsRaw))
	for _, cr := range contractsRaw {
		cmap, ok := cr.(map[string]interface{})
		if !ok {
			continue
		}
		item := ContractDraftItem{
			ContractNumber:    getString(cmap, "contract_number"),
			ContractName:      getString(cmap, "contract_name"),
			Lessee:            getString(cmap, "lessee"),
			Lessor:            getString(cmap, "lessor"),
			StoreName:         getString(cmap, "store_name"),
			StoreAddress:      getString(cmap, "store_address"),
			CommencementDate:  getString(cmap, "commencement_date"),
			LeaseStartDate:    getString(cmap, "lease_start_date"),
			LeaseEndDate:      getString(cmap, "lease_end_date"),
			Currency:          getString(cmap, "currency"),
			AssetType:         getString(cmap, "asset_type"),
			PaymentFrequency:  getString(cmap, "payment_frequency"),
			PaymentTiming:     getString(cmap, "payment_timing"),
			DiscountRateType:  getString(cmap, "discount_rate_type"),
			RenewalOption:     getBool(cmap, "renewal_option"),
			TerminationOption: getBool(cmap, "termination_option"),
			FixedRentAmount:   getFloat64(cmap, "fixed_rent_amount"),
			CAMAmount:         getFloat64(cmap, "cam_amount"),
			ServiceFee:        getFloat64(cmap, "service_fee"),
			DiscountRate:      getFloat64(cmap, "discount_rate"),
			IsLease:           getBoolDefault(cmap, "is_lease", true),
			LeaseScope:        getString(cmap, "lease_scope"),
			SuggestedScope:    getString(cmap, "suggested_scope"),
			ExemptionReason:   getString(cmap, "exemption_reason"),
			ScopeSource:       getString(cmap, "scope_source"),
			ScopeConfidence:   getFloat64(cmap, "scope_confidence"),
			Confidence:        getFloat64(cmap, "confidence"),
			MissingFields:     getStringSlice(cmap, "missing_fields"),
			Warnings:          getStringSlice(cmap, "warnings"),
		}
		if item.AssetType == "" {
			item.AssetType = "real_estate"
		}
		if item.LeaseScope == "" {
			item.LeaseScope = item.SuggestedScope
		}
		if item.LeaseScope == "" {
			item.LeaseScope = "in_scope"
		}
		if item.SuggestedScope == "" {
			item.SuggestedScope = item.LeaseScope
		}
		if item.ScopeSource == "" {
			item.ScopeSource = "ai_suggested"
		}
		contracts = append(contracts, item)
	}

	totalCount := getInt(result, "total_count")
	overallConf := getFloat64(result, "overall_confidence")
	if overallConf == 0 {
		if scores, ok := result["confidence_scores"].(map[string]interface{}); ok {
			overallConf = getFloat64(scores, "overall")
		}
	}
	requiresHuman := getBool(result, "requires_human_confirmation")
	warnings := getStringSlice(result, "warnings")
	missingFields := getStringSlice(result, "missing_fields")

	// Build summary text
	var summary strings.Builder
	summary.WriteString(fmt.Sprintf("Agent 已完成文件理解和合同草稿生成：识别出 **%d** 份合同。\n\n", totalCount))
	if requiresHuman {
		summary.WriteString("⚠️ **需要人工确认**：部分合同存在缺失字段、低置信度识别结果或会计判断事项，请逐条核对后再确认入库。\n\n")
	} else {
		summary.WriteString("识别结果总体良好，建议快速核对后确认入库。\n\n")
	}
	if len(missingFields) > 0 {
		summary.WriteString(fmt.Sprintf("需要优先确认的字段：%s。\n\n", strings.Join(missingFields, ", ")))
	}
	summary.WriteString("请在下方草稿表格中逐条编辑、跳过或确认每份合同。确认后 Agent 只会创建 draft 合同，正式入库仍进入正常审批流程。")

	sources := []Source{{
		Type:    "file",
		ID:      fileID,
		Title:   "合同台账文件",
		Snippet: objectName,
	}}

	officeSkill := "Office Excel Skill"
	if !isExcelContentType(contentType) {
		officeSkill = "Office Document Skill"
	}
	toolCalls := []AgentToolCall{
		{
			Tool:           "office.file_reader",
			Skill:          officeSkill,
			Status:         "completed",
			InputSummary:   fmt.Sprintf("读取上传文件 %s", objectName),
			OutputSummary:  "已将文件展开为可供 LLM 理解的文本/表格证据",
			RequiresReview: false,
		},
		{
			Tool:           "lease.contract_batch_parser",
			Skill:          "Lease Intake Skill",
			Status:         statusFromReview(requiresHuman),
			InputSummary:   "基于文件证据语义识别合同台账字段、范围判定和缺失项",
			OutputSummary:  fmt.Sprintf("生成 %d 份合同草稿，缺失字段 %d 类，警告 %d 条", len(contracts), len(missingFields), len(warnings)),
			RequiresReview: requiresHuman,
		},
		{
			Tool:           "lease.draft_contract_creator",
			Skill:          "Core Service Draft Skill",
			Status:         "needs_review",
			InputSummary:   "等待用户确认草稿卡片",
			OutputSummary:  "确认后批量创建 draft 合同，不绕过审批",
			RequiresReview: true,
		},
	}
	agentPlan := []AgentPlanStep{
		{ID: "read_file", Title: "读取 Office 文件并保留证据", Status: "completed"},
		{ID: "understand_ledger", Title: "理解非标准台账并生成合同草稿", Status: statusFromReview(requiresHuman)},
		{ID: "human_review", Title: "人工确认缺失字段和会计判断", Status: "needs_review"},
		{ID: "create_draft", Title: "确认后创建 draft 合同并进入审批", Status: "pending"},
	}
	reviewPrompts := buildReviewPrompts(contracts, missingFields, warnings)

	return &BatchParseResult{
		SummaryText:   summary.String(),
		Sources:       sources,
		Confidence:    overallConf,
		AgentPlan:     agentPlan,
		ToolCalls:     toolCalls,
		ReviewPrompts: reviewPrompts,
		Contracts:     contracts,
		Summary: &BatchParseSummary{
			TotalCount:           totalCount,
			OverallConfidence:    overallConf,
			RequiresHumanConfirm: requiresHuman,
			MissingFields:        missingFields,
			Warnings:             warnings,
		},
	}, nil
}

func isExcelContentType(contentType string) bool {
	return contentType == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" ||
		contentType == "application/vnd.ms-excel"
}

func buildReviewPrompts(contracts []ContractDraftItem, missingFields []string, warnings []string) []AgentReviewPrompt {
	var prompts []AgentReviewPrompt

	if len(missingFields) > 0 {
		prompts = append(prompts, AgentReviewPrompt{
			ID:          "missing_fields",
			Title:       "补齐全局缺失字段",
			Description: fmt.Sprintf("本次解析发现 %d 类缺失字段：%s。缺失字段会影响正式台账完整性。", len(missingFields), strings.Join(missingFields, ", ")),
			Severity:    "warning",
			Action:      "在下方草稿表格中补齐字段，或明确跳过不适用字段后再确认入库。",
		})
	}

	discountRateContracts := make([]string, 0)
	lowConfidenceContracts := make([]string, 0)
	scopeReviewContracts := make([]string, 0)
	for _, contract := range contracts {
		if contract.DiscountRate == 0 || containsString(contract.MissingFields, "discount_rate") {
			discountRateContracts = append(discountRateContracts, contract.ContractNumber)
		}
		if contract.Confidence < 0.8 || contract.ScopeConfidence < 0.8 {
			lowConfidenceContracts = append(lowConfidenceContracts, contract.ContractNumber)
		}
		if contract.LeaseScope == "" || contract.LeaseScope != "in_scope" || contract.ScopeConfidence < 0.8 {
			scopeReviewContracts = append(scopeReviewContracts, contract.ContractNumber)
		}
	}

	if len(discountRateContracts) > 0 {
		prompts = append(prompts, AgentReviewPrompt{
			ID:              "discount_rate",
			Title:           "确认折现率",
			Description:     fmt.Sprintf("%d 份合同缺少折现率或折现率为 0。AI 不得猜测折现率。", len(discountRateContracts)),
			Severity:        "critical",
			Action:          "检查合同文本、系统利率政策或请人工选择适用 IBR 后再确认。",
			ContractNumbers: firstStrings(discountRateContracts, 8),
		})
	}

	if len(scopeReviewContracts) > 0 {
		prompts = append(prompts, AgentReviewPrompt{
			ID:              "lease_scope",
			Title:           "复核租赁范围判断",
			Description:     fmt.Sprintf("%d 份合同需要确认是否资本化、短期/低价值豁免或非租赁。", len(scopeReviewContracts)),
			Severity:        "warning",
			Action:          "复核 lease_scope、scope confidence 和豁免/排除原因，确认后才进入 draft 合同。",
			ContractNumbers: firstStrings(scopeReviewContracts, 8),
		})
	}

	if len(lowConfidenceContracts) > 0 {
		prompts = append(prompts, AgentReviewPrompt{
			ID:              "low_confidence",
			Title:           "核对低置信度草稿",
			Description:     fmt.Sprintf("%d 份合同存在整体或范围判断低置信度。", len(lowConfidenceContracts)),
			Severity:        "warning",
			Action:          "优先核对这些合同的日期、主体、租金、付款时点和范围判断。",
			ContractNumbers: firstStrings(lowConfidenceContracts, 8),
		})
	}

	if len(prompts) == 0 && len(warnings) == 0 && len(contracts) > 0 {
		prompts = append(prompts, AgentReviewPrompt{
			ID:          "quick_review",
			Title:       "快速核对后确认",
			Description: "未发现全局缺失字段或低置信度草稿，但 AI 结果仍需人工确认。",
			Severity:    "info",
			Action:      "抽查关键字段和原始文件证据，确认后创建 draft 合同。",
		})
	}

	return prompts
}

func buildPaymentScheduleReviewPrompts(schedules []PaymentScheduleDraftItem, missingFields []string, warnings []string, contractID string) []AgentReviewPrompt {
	var prompts []AgentReviewPrompt
	if contractID == "" {
		prompts = append(prompts, AgentReviewPrompt{
			ID:          "bind_contract",
			Title:       "绑定目标合同",
			Description: "付款计划必须挂到具体合同后才能入库，当前 AI Chat 请求没有合同上下文。",
			Severity:    "critical",
			Action:      "进入合同详情页后上传租金表，或在 AI Chat URL 中带上 contract_id。",
		})
	}
	if len(missingFields) > 0 {
		prompts = append(prompts, AgentReviewPrompt{
			ID:          "schedule_missing_fields",
			Title:       "补齐付款计划缺失字段",
			Description: fmt.Sprintf("本次解析发现 %d 类缺失字段：%s。", len(missingFields), strings.Join(missingFields, ", ")),
			Severity:    "warning",
			Action:      "核对租金表原文，补齐覆盖期间、应付日、金额或币种后再导入。",
		})
	}

	lowConfidence := 0
	variableOrNonLease := 0
	emptyCurrency := 0
	for _, schedule := range schedules {
		if schedule.Confidence < 0.8 {
			lowConfidence++
		}
		if !schedule.IsFixed || !schedule.IsLeaseComponent || schedule.AmountType == "turnover_rent" || schedule.AmountType == "cam" || schedule.AmountType == "service_fee" {
			variableOrNonLease++
		}
		if schedule.Currency == "" {
			emptyCurrency++
		}
	}
	if lowConfidence > 0 {
		prompts = append(prompts, AgentReviewPrompt{
			ID:          "schedule_low_confidence",
			Title:       "核对低置信度付款行",
			Description: fmt.Sprintf("%d 笔付款计划置信度低于 0.8。", lowConfidence),
			Severity:    "warning",
			Action:      "优先核对这些行的期间、金额、付款时点和会计属性。",
		})
	}
	if variableOrNonLease > 0 {
		prompts = append(prompts, AgentReviewPrompt{
			ID:          "schedule_accounting_attribute",
			Title:       "确认变量租金和非租赁成分",
			Description: fmt.Sprintf("%d 笔付款可能涉及变量租金或非租赁成分。", variableOrNonLease),
			Severity:    "warning",
			Action:      "确认 turnover rent、CAM、服务费等不得错误资本化计入租赁负债。",
		})
	}
	if emptyCurrency > 0 {
		prompts = append(prompts, AgentReviewPrompt{
			ID:          "schedule_currency",
			Title:       "确认币种",
			Description: fmt.Sprintf("%d 笔付款缺少币种。AI 不应猜测文件中未出现的币种。", emptyCurrency),
			Severity:    "warning",
			Action:      "根据合同或租金表原文确认币种后再导入。",
		})
	}
	if len(prompts) == 0 && len(warnings) == 0 && len(schedules) > 0 {
		prompts = append(prompts, AgentReviewPrompt{
			ID:          "schedule_quick_review",
			Title:       "快速核对后导入",
			Description: "未发现明显缺失字段或低置信度付款行，但付款计划仍需人工确认。",
			Severity:    "info",
			Action:      "抽查期间、金额、付款时点和会计属性，确认后导入付款计划。",
		})
	}
	return prompts
}

func statusFromReview(requiresHuman bool) string {
	if requiresHuman {
		return "needs_review"
	}
	return "completed"
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func firstStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	return values[:limit]
}

// Helper functions for safe type extraction
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func getBoolDefault(m map[string]interface{}, key string, fallback bool) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return fallback
}

func getFloat64(m map[string]interface{}, key string) float64 {
	switch v := m[key].(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return 0
}

func getInt(m map[string]interface{}, key string) int {
	switch v := m[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	case int64:
		return int(v)
	}
	return 0
}

func getStringSlice(m map[string]interface{}, key string) []string {
	if arr, ok := m[key].([]interface{}); ok {
		result := make([]string, 0, len(arr))
		for _, v := range arr {
			if s, ok := v.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return []string{}
}

func containsAny(msg string, targets []string) bool {
	for _, t := range targets {
		if strings.Contains(strings.ToLower(msg), strings.ToLower(t)) {
			return true
		}
	}
	return false
}

func detectFileParseTool(message string) string {
	msgLower := strings.ToLower(message)
	switch {
	case containsAny(msgLower, []string{"租金表", "付款计划", "付款表", "rent schedule", "payment schedule", "rental schedule"}):
		return "parse_payment_schedule"
	case containsAny(msgLower, []string{"录入", "导入", "录入台账", "批量创建", "创建合同", "导入台账", "import", "batch create", "create contracts", "ledger"}):
		return "parse_contract_batch"
	default:
		return "parse_contract"
	}
}
