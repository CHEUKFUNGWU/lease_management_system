package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/repository"
)

type createAgentRunRequest struct {
	SessionID    string          `json:"session_id"`
	Message      string          `json:"message"`
	SkillID      string          `json:"skill_id,omitempty"`
	SkillVersion string          `json:"skill_version,omitempty"`
	PageContext  json.RawMessage `json:"page_context,omitempty"`
}

type agentRunEventRequest struct {
	Type    string `json:"type"`
	CallID  string `json:"call_id,omitempty"`
	Payload any    `json:"payload,omitempty"`
}

type agentRunControlRequest struct {
	Instruction string `json:"instruction"`
}

type agentRunBranchRequest struct {
	Message string `json:"message"`
}

func (h *AgentGatewayHandler) CreateRun(c *gin.Context) {
	if h == nil || h.runs == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent run store unavailable"})
		return
	}
	var request createAgentRunRequest
	if err := decodeStrictJSON(c, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid agent run request"})
		return
	}
	request.SessionID = strings.TrimSpace(request.SessionID)
	request.Message = strings.TrimSpace(request.Message)
	request.SkillID = strings.TrimSpace(request.SkillID)
	request.SkillVersion = strings.TrimSpace(request.SkillVersion)
	if request.SessionID == "" || request.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id and message are required"})
		return
	}
	ctx, status, err := h.gatewayContext(c, "agent-run-create", "agent-run-create")
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	execution, ok := agenttools.ExecutionContextFromContext(ctx)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "authenticated agent context is required"})
		return
	}
	session, err := h.runs.GetSessionByID(ctx, request.SessionID, execution.Principal.UserID)
	if err != nil || session == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent session not found"})
		return
	}
	if request.SkillID != "" {
		if h.skills == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent skill registry unavailable"})
			return
		}
		definition, found := h.skills.Resolve(request.SkillID, request.SkillVersion)
		if !found || !skillRoleAllowed(definition.AllowedRoles, execution.Principal.Role) {
			c.JSON(http.StatusForbidden, gin.H{"error": "requested Skill is not available to this principal"})
			return
		}
		request.SkillID = definition.ID
		request.SkillVersion = definition.Version
	}
	if len(request.PageContext) == 0 {
		request.PageContext = json.RawMessage("null")
	}
	run := &repository.AIChatRun{
		SessionID: session.ID, Status: "queued", AgentMode: true,
		PageContext: request.PageContext, CreatedBy: agentStringPointer(execution.Principal.UserID),
	}
	if request.SkillID != "" {
		run.SkillID = stringPointer(request.SkillID)
		run.SkillVersion = stringPointer(request.SkillVersion)
	}
	if err := h.runs.CreateRun(ctx, run); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create agent run"})
		return
	}
	if err := h.appendRunEvent(ctx, run, "message_start", map[string]any{
		"message": request.Message, "skill_id": request.SkillID, "skill_version": request.SkillVersion,
	}, false); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist agent run event"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"run": run})
}

func (h *AgentGatewayHandler) ListRunEvents(c *gin.Context) {
	run, ctx, status, ok := h.loadRun(c, true)
	if !ok {
		return
	}
	after, err := parseNonNegativeInt(c.Query("after_sequence"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "after_sequence must be a non-negative integer"})
		return
	}
	limit, err := parseNonNegativeInt(c.Query("limit"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be a non-negative integer"})
		return
	}
	if limit <= 0 {
		limit = 200
	}
	workerID, leaseToken, isWorker, err := workerLeaseHeaders(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var events []*repository.AIChatRunEvent
	if isWorker {
		events, err = h.workerRuns.ListClaimedRunEvents(ctx, run.ID, workerID, leaseToken, after, limit)
	} else {
		events, err = h.runs.ListRunEvents(ctx, run.ID, after, limit)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list agent run events"})
		return
	}
	c.JSON(status, gin.H{"run": run, "events": events})
}

func (h *AgentGatewayHandler) AppendRunEvent(c *gin.Context) {
	run, ctx, _, ok := h.loadRun(c, true)
	if !ok {
		return
	}
	var request agentRunEventRequest
	if err := decodeStrictJSON(c, &request); err != nil || strings.TrimSpace(request.Type) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "event type is required"})
		return
	}
	eventType := strings.TrimSpace(request.Type)
	terminal := isTerminalAgentEvent(eventType)
	workerID, leaseToken, isWorker, err := workerLeaseHeaders(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	payload := map[string]any{
		"call_id": request.CallID, "payload": request.Payload,
	}
	if isWorker {
		runEvent := &repository.AIChatRunEvent{RunID: run.ID, SessionID: run.SessionID, EventType: eventType, Payload: mustMarshalAgentPayload(payload), IsTerminal: terminal}
		err = h.workerRuns.AppendClaimedRunEvent(ctx, run.ID, workerID, leaseToken, runEvent)
	} else {
		err = h.appendRunEvent(ctx, run, eventType, payload, terminal)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to append agent run event"})
		return
	}
	if statusValue, summary, errorMessage := agentRunStatusUpdate(eventType, request.Payload); statusValue != "" {
		startedAt := (*time.Time)(nil)
		if strings.EqualFold(eventType, "run_started") {
			now := time.Now().UTC()
			startedAt = &now
		}
		completedAt := (*time.Time)(nil)
		if terminal {
			now := time.Now().UTC()
			completedAt = &now
		}
		if isWorker {
			err = h.workerRuns.UpdateClaimedRunStatus(ctx, run.ID, workerID, leaseToken, statusValue, run.ReviewRequired, summary, errorMessage, startedAt, completedAt)
		} else {
			err = h.runs.UpdateRunStatus(ctx, run.ID, statusValue, run.ReviewRequired, summary, errorMessage, startedAt, completedAt)
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update agent run status"})
			return
		}
	}
	c.JSON(http.StatusAccepted, gin.H{"accepted": true, "run_id": run.ID, "event_type": eventType, "terminal": terminal})
}

func (h *AgentGatewayHandler) CancelRun(c *gin.Context) {
	run, ctx, _, ok := h.loadOwnedRun(c)
	if !ok {
		return
	}
	if isTerminalRunStatus(run.Status) {
		c.JSON(http.StatusConflict, gin.H{"error": "agent run is already terminal", "status": run.Status})
		return
	}
	errorText := "cancelled by user"
	if err := h.runs.UpdateRunStatus(ctx, run.ID, "cancelled", run.ReviewRequired, nil, &errorText, nil, timePointer(time.Now().UTC())); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to cancel agent run"})
		return
	}
	if err := h.appendRunEvent(ctx, run, "run_cancelled", map[string]any{"reason": errorText}, true); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist cancellation event"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"cancelled": true, "run_id": run.ID})
}

func (h *AgentGatewayHandler) SteerRun(c *gin.Context) {
	h.appendControlEvent(c, "run_steer")
}

func (h *AgentGatewayHandler) FollowUpRun(c *gin.Context) {
	run, ctx, _, ok := h.loadOwnedRun(c)
	if !ok {
		return
	}
	var request agentRunControlRequest
	if err := decodeStrictJSON(c, &request); err != nil || strings.TrimSpace(request.Instruction) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "instruction is required"})
		return
	}
	if !isTerminalRunStatus(run.Status) {
		c.JSON(http.StatusConflict, gin.H{"error": "follow-up requires a terminal agent run", "status": run.Status})
		return
	}
	execution, ok := agenttools.ExecutionContextFromContext(ctx)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "authenticated agent context is required"})
		return
	}
	child := &repository.AIChatRun{
		SessionID: run.SessionID, ParentRunID: agentStringPointer(run.ID), Status: "queued", AgentMode: true,
		PageContext: run.PageContext, CreatedBy: agentStringPointer(execution.Principal.UserID),
		SkillID: run.SkillID, SkillVersion: run.SkillVersion,
	}
	if err := h.runs.CreateRun(ctx, child); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create follow-up agent run"})
		return
	}
	if err := h.appendRunEvent(ctx, run, "run_follow_up", map[string]any{
		"instruction": strings.TrimSpace(request.Instruction), "child_run_id": child.ID,
	}, false); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist follow-up event"})
		return
	}
	if err := h.appendRunEvent(ctx, child, "message_start", map[string]any{
		"message": strings.TrimSpace(request.Instruction), "parent_run_id": run.ID,
	}, false); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist follow-up start event"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"run": child, "parent_run_id": run.ID, "accepted": true})
}

// BranchRun creates a new queued Run from an owned checkpoint. The checkpoint
// is copied as part of the child Run insert so a failed branch cannot expose a
// child without its recovery state. Execution still requires a Runner to
// explicitly resume the returned child Run.
func (h *AgentGatewayHandler) BranchRun(c *gin.Context) {
	run, ctx, _, ok := h.loadOwnedRun(c)
	if !ok {
		return
	}
	if h.checkpoints == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent checkpoint store unavailable"})
		return
	}
	var request agentRunBranchRequest
	if err := decodeStrictJSON(c, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch request"})
		return
	}
	request.Message = strings.TrimSpace(request.Message)
	if request.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message is required"})
		return
	}
	execution, ok := agenttools.ExecutionContextFromContext(ctx)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "authenticated agent context is required"})
		return
	}
	checkpoint, err := h.checkpoints.GetRunCheckpoint(ctx, run.ID, execution.Principal.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load agent checkpoint"})
		return
	}
	if len(checkpoint) == 0 || string(checkpoint) == "null" {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent checkpoint not found"})
		return
	}
	var checkpointObject map[string]any
	if err := json.Unmarshal(checkpoint, &checkpointObject); err != nil || checkpointObject == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "agent checkpoint is not a JSON object"})
		return
	}
	child := &repository.AIChatRun{
		SessionID: run.SessionID, ParentRunID: agentStringPointer(run.ID), Status: "queued", AgentMode: true,
		PageContext: run.PageContext, ReviewRequired: run.ReviewRequired, CreatedBy: agentStringPointer(execution.Principal.UserID),
		SkillID: run.SkillID, SkillVersion: run.SkillVersion, Checkpoint: append(json.RawMessage(nil), checkpoint...),
	}
	if err := h.runs.CreateRun(ctx, child); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create checkpoint branch"})
		return
	}
	if err := h.appendRunEvent(ctx, run, "run_branch_created", map[string]any{
		"child_run_id": child.ID, "message": request.Message, "checkpoint": true,
	}, false); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist branch event"})
		return
	}
	if err := h.appendRunEvent(ctx, child, "checkpoint_restored", map[string]any{
		"parent_run_id": run.ID, "message": request.Message,
	}, false); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist branch start event"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"run": child, "parent_run_id": run.ID, "checkpoint_restored": true, "accepted": true})
}
func (h *AgentGatewayHandler) appendControlEvent(c *gin.Context, eventType string) {
	run, ctx, _, ok := h.loadOwnedRun(c)
	if !ok {
		return
	}
	var request agentRunControlRequest
	if err := decodeStrictJSON(c, &request); err != nil || strings.TrimSpace(request.Instruction) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "instruction is required"})
		return
	}
	if isTerminalRunStatus(run.Status) {
		c.JSON(http.StatusConflict, gin.H{"error": "agent run is already terminal", "status": run.Status})
		return
	}
	if err := h.appendRunEvent(ctx, run, eventType, map[string]any{"instruction": strings.TrimSpace(request.Instruction)}, false); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist agent control event"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"accepted": true, "run_id": run.ID, "event_type": eventType})
}

func (h *AgentGatewayHandler) loadOwnedRun(c *gin.Context) (*repository.AIChatRun, context.Context, int, bool) {
	return h.loadRun(c, false)
}

func (h *AgentGatewayHandler) loadRun(c *gin.Context, allowWorker bool) (*repository.AIChatRun, context.Context, int, bool) {
	if h == nil || h.runs == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent run store unavailable"})
		return nil, nil, 0, false
	}
	runID := strings.TrimSpace(c.Param("id"))
	if runID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "run ID is required"})
		return nil, nil, 0, false
	}
	ctx, status, err := h.gatewayContext(c, runID, runID)
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return nil, nil, 0, false
	}
	execution, ok := agenttools.ExecutionContextFromContext(ctx)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "authenticated agent context is required"})
		return nil, nil, 0, false
	}
	workerID, leaseToken, isWorker, workerErr := workerLeaseHeaders(c)
	if workerErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": workerErr.Error()})
		return nil, nil, 0, false
	}
	if isWorker {
		workerCtx, workerStatus, workerContextErr := h.workerContext(c, runID)
		if workerContextErr != nil {
			c.JSON(workerStatus, gin.H{"error": workerContextErr.Error()})
			return nil, nil, 0, false
		}
		ctx = workerCtx
	}
	if isWorker && !allowWorker {
		c.JSON(http.StatusForbidden, gin.H{"error": "worker lease cannot control an agent run"})
		return nil, nil, 0, false
	}
	var run *repository.AIChatRun
	if isWorker {
		if h.workerRuns == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent worker run store unavailable"})
			return nil, nil, 0, false
		}
		run, err = h.workerRuns.GetClaimedRun(ctx, runID, workerID, leaseToken)
	} else {
		run, err = h.runs.GetRunByID(ctx, runID, execution.Principal.UserID)
	}
	if err != nil || run == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "agent run not found"})
		return nil, nil, 0, false
	}
	return run, ctx, http.StatusOK, true
}

func workerLeaseHeaders(c *gin.Context) (string, string, bool, error) {
	workerID := strings.TrimSpace(c.GetHeader("X-Agent-Worker-ID"))
	leaseToken := strings.TrimSpace(c.GetHeader("X-Agent-Lease-Token"))
	if workerID == "" && leaseToken == "" {
		return "", "", false, nil
	}
	if workerID == "" || leaseToken == "" {
		return "", "", false, errors.New("X-Agent-Worker-ID and X-Agent-Lease-Token are required together")
	}
	return workerID, leaseToken, true, nil
}

func mustMarshalAgentPayload(payload any) json.RawMessage {
	raw, err := json.Marshal(payload)
	if err != nil {
		return json.RawMessage(`{"payload_encoding_error":true}`)
	}
	return raw
}

func (h *AgentGatewayHandler) appendRunEvent(ctx context.Context, run *repository.AIChatRun, eventType string, payload any, terminal bool) error {
	if h == nil || h.runs == nil || run == nil {
		return errors.New("agent run store unavailable")
	}
	sequence, err := h.runs.GetNextRunEventSequence(ctx, run.ID)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return h.runs.AppendRunEvent(ctx, &repository.AIChatRunEvent{
		RunID: run.ID, SessionID: run.SessionID, SequenceNo: sequence,
		EventType: eventType, Payload: raw, IsTerminal: terminal,
	})
}

func isTerminalAgentEvent(eventType string) bool {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "run_finished", "run_failed", "run_cancelled", "run_error":
		return true
	default:
		return false
	}
}

func agentRunStatusUpdate(eventType string, payload any) (string, *string, *string) {
	status := ""
	summary := (*string)(nil)
	errorMessage := (*string)(nil)
	if raw, ok := payload.(string); ok && strings.TrimSpace(raw) != "" {
		value := strings.TrimSpace(raw)
		switch strings.ToLower(strings.TrimSpace(eventType)) {
		case "run_failed", "run_error", "run_cancelled":
			errorMessage = &value
		}
	}
	if mapValue, ok := payload.(map[string]any); ok {
		if raw, ok := mapValue["status"].(string); ok {
			status = strings.TrimSpace(raw)
		}
		if raw, ok := mapValue["error"].(string); ok && strings.TrimSpace(raw) != "" {
			value := strings.TrimSpace(raw)
			errorMessage = &value
		}
	}
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "run_started":
		return "running", summary, errorMessage
	case "run_finished":
		if !validAgentRunStatus(status) {
			status = "completed"
		}
		return status, summary, errorMessage
	case "run_failed", "run_error":
		return "failed", summary, errorMessage
	case "run_cancelled":
		return "cancelled", summary, errorMessage
	default:
		return "", nil, nil
	}
}

func validAgentRunStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued", "running", "completed", "waiting_review", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func isTerminalRunStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed", "waiting_review", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func agentStringPointer(value string) *string {
	return &value
}

func timePointer(value time.Time) *time.Time {
	return &value
}

func parseNonNegativeInt(value string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 0 {
		return 0, errors.New("value must be a non-negative integer")
	}
	return parsed, nil
}
