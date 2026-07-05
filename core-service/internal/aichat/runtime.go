package aichat

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/repository"
)

const defaultRunTimeout = 4 * time.Minute

type Runtime[T any] struct {
	store    store
	planner  Planner
	executor Executor[T]
	project  Projector[T]
	dispatch func(func())
	timeout  time.Duration
	now      func() time.Time
}

type preparedRun struct {
	session *repository.AIChatSession
	run     *repository.AIChatRun
	message *repository.AIChatMessage
	input   Input
	plan    Plan
}

func NewRuntime[T any](persistence *repository.AIChatRuntimeRepository, planner Planner, executor Executor[T], project Projector[T], options Options) *Runtime[T] {
	return newRuntime[T](persistence, planner, executor, project, options)
}

func newRuntime[T any](persistence store, planner Planner, executor Executor[T], project Projector[T], options Options) *Runtime[T] {
	dispatch := options.Dispatch
	if dispatch == nil {
		dispatch = func(task func()) { go task() }
	}
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = defaultRunTimeout
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &Runtime[T]{
		store: persistence, planner: planner, executor: executor, project: project,
		dispatch: dispatch, timeout: timeout, now: now,
	}
}

func (r *Runtime[T]) OpenSession(ctx context.Context, command SessionCommand) (*repository.AIChatSession, error) {
	if strings.TrimSpace(command.UserID) == "" {
		return nil, errors.New("AI chat session requires a user")
	}
	session := &repository.AIChatSession{
		UserID: command.UserID, Title: strings.TrimSpace(command.Title),
		ContextSnapshot: marshalJSON(command.ContextSnapshot),
	}
	if command.LegalEntityID != "" {
		session.LegalEntityID = &command.LegalEntityID
	}
	if command.BoundContractID != "" {
		session.BoundContractID = &command.BoundContractID
	}
	if err := r.store.CreateSession(ctx, session); err != nil {
		return nil, fmt.Errorf("create AI chat session: %w", err)
	}
	return session, nil
}

func (r *Runtime[T]) Start(ctx context.Context, input Input) (*Started, error) {
	prepared, err := r.prepare(ctx, input, nil, nil)
	if err != nil {
		return nil, err
	}
	started := r.started(prepared, nil)
	r.dispatch(func() {
		runCtx, cancel := context.WithTimeout(context.Background(), r.timeout)
		defer cancel()
		r.executeDispatched(runCtx, prepared)
	})
	return started, nil
}

func (r *Runtime[T]) Run(ctx context.Context, input Input) (*Completed[T], error) {
	prepared, err := r.prepare(ctx, input, nil, nil)
	if err != nil {
		return nil, err
	}
	response, executionErr, persistenceErr := r.execute(ctx, prepared)
	if persistenceErr != nil {
		_ = r.markPersistenceFailure(ctx, prepared, persistenceErr)
		return nil, persistenceErr
	}
	return &Completed[T]{
		Started: r.started(prepared, nil), Response: response, ExecutionError: executionErr,
	}, nil
}

func (r *Runtime[T]) executeDispatched(ctx context.Context, prepared *preparedRun) {
	_, _, persistenceErr := r.execute(ctx, prepared)
	if persistenceErr != nil {
		_ = r.markPersistenceFailure(ctx, prepared, persistenceErr)
	}
}

func (r *Runtime[T]) Inspect(ctx context.Context, userID, runID string, afterSequence int) (*Inspection, error) {
	run, err := r.store.GetRunByID(ctx, runID, userID)
	if err != nil {
		return nil, fmt.Errorf("load AI agent run: %w", err)
	}
	events, err := r.store.ListRunEvents(ctx, runID, afterSequence, 200)
	if err != nil {
		return nil, fmt.Errorf("load AI agent run events: %w", err)
	}
	return &Inspection{Run: run, Events: events}, nil
}

func (r *Runtime[T]) prepare(ctx context.Context, input Input, session *repository.AIChatSession, sourceRun *repository.AIChatRun) (*preparedRun, error) {
	input.Message = strings.TrimSpace(input.Message)
	if input.UserID == "" {
		return nil, errors.New("AI agent run requires a user")
	}
	if input.Message == "" {
		return nil, errors.New("AI agent run requires a message")
	}
	if input.Language == "" {
		input.Language = "zh-CN"
	}

	var err error
	if session == nil {
		if input.SessionID != "" {
			session, err = r.store.GetSessionByID(ctx, input.SessionID, input.UserID)
			if err != nil {
				return nil, fmt.Errorf("load AI chat session: %w", err)
			}
		} else {
			session = &repository.AIChatSession{
				UserID: input.UserID, Title: summarizeTitle(input.Message),
				ContextSnapshot: marshalJSON(input.PageContext),
			}
			if input.LegalEntityID != "" {
				session.LegalEntityID = &input.LegalEntityID
			}
			if input.ContractID != "" {
				session.BoundContractID = &input.ContractID
			}
			if err := r.store.CreateSession(ctx, session); err != nil {
				return nil, fmt.Errorf("create AI chat session: %w", err)
			}
			input.SessionID = session.ID
		}
	}
	if input.ContractID == "" && session.BoundContractID != nil {
		input.ContractID = *session.BoundContractID
	}
	input.SessionID = session.ID

	plan := r.planner.Plan(input, sourceRun)
	if input.AgentMode != nil {
		plan.AgentMode = *input.AgentMode
	}
	if input.SkillID != "" {
		plan.SkillID = input.SkillID
	}
	if input.SkillVersion != "" {
		plan.SkillVersion = input.SkillVersion
	}

	run := &repository.AIChatRun{
		SessionID: session.ID, Status: "queued", AgentMode: plan.AgentMode,
		PageContext: marshalJSON(input.PageContext), ReviewRequired: plan.ReviewRequired,
		CreatedBy: &input.UserID,
	}
	if input.ParentRunID != "" {
		run.ParentRunID = &input.ParentRunID
	}
	if plan.SkillID != "" {
		run.SkillID = &plan.SkillID
	}
	if plan.SkillVersion != "" {
		run.SkillVersion = &plan.SkillVersion
	}
	if err := r.store.CreateRun(ctx, run); err != nil {
		return nil, fmt.Errorf("create AI agent run: %w", err)
	}

	nextSequence, err := r.store.GetNextMessageSequence(ctx, session.ID)
	if err != nil {
		return nil, fmt.Errorf("allocate AI chat message sequence: %w", err)
	}
	message := &repository.AIChatMessage{
		SessionID: session.ID, RunID: &run.ID, Role: "user", MessageType: "text",
		SequenceNo: nextSequence, Content: input.Message, CreatedBy: &input.UserID,
		Attachments: marshalJSON(attachmentReferences(input)),
	}
	if err := r.store.CreateMessage(ctx, message); err != nil {
		return nil, fmt.Errorf("persist AI chat trigger message: %w", err)
	}
	if err := r.store.LinkRunTriggerMessage(ctx, run.ID, message.ID); err != nil {
		return nil, fmt.Errorf("link AI agent trigger message: %w", err)
	}

	prepared := &preparedRun{session: session, run: run, message: message, input: input, plan: plan}
	if err := r.appendEvent(ctx, prepared, "message_start", map[string]any{
		"message_id": message.ID, "role": "user", "content": input.Message,
		"has_file": input.FileID != "", "contract_id": input.ContractID,
		"page_context": input.PageContext,
	}, false); err != nil {
		return nil, err
	}
	if input.FileID != "" {
		attachment := &repository.AIChatAttachment{
			SessionID: session.ID, MessageID: &message.ID, RunID: &run.ID,
			FileID: input.FileID, OriginalName: input.ObjectName, ContentType: input.ContentType,
			CreatedBy: &input.UserID, MinioObjectKey: &input.ObjectName, ParseStatus: "processing",
		}
		if err := r.store.CreateAttachment(ctx, attachment); err != nil {
			return nil, fmt.Errorf("persist AI chat attachment: %w", err)
		}
	}
	startedAt := r.now()
	if err := r.store.UpdateRunStatus(ctx, run.ID, "running", run.ReviewRequired, nil, nil, &startedAt, nil); err != nil {
		return nil, fmt.Errorf("start AI agent run: %w", err)
	}
	run.Status = "running"
	run.StartedAt = &startedAt
	if err := r.appendEvent(ctx, prepared, "run_status", map[string]any{
		"status": "running", "skill_id": run.SkillID, "agent_mode": run.AgentMode,
		"review_required": run.ReviewRequired,
	}, false); err != nil {
		return nil, err
	}
	return prepared, nil
}

func (r *Runtime[T]) execute(ctx context.Context, prepared *preparedRun) (T, error, error) {
	response, executionErr := r.executor.Execute(ctx, Execution{
		Input: prepared.input, Session: prepared.session, Run: prepared.run, Plan: prepared.plan,
		Emit: func(eventCtx context.Context, eventType string, payload any) error {
			return r.appendEvent(eventCtx, prepared, eventType, payload, false)
		},
	})
	result := r.project(response)
	if executionErr != nil {
		return response, executionErr, r.fail(ctx, prepared, result, executionErr)
	}
	return response, nil, r.complete(ctx, prepared, result)
}

func (r *Runtime[T]) complete(ctx context.Context, prepared *preparedRun, result Result) error {
	if err := r.persistAssistantMessage(ctx, prepared, result); err != nil {
		return err
	}
	if !emptyValue(result.ToolCalls) {
		if err := r.appendEvent(ctx, prepared, "tool_end", result.ToolCalls, false); err != nil {
			return err
		}
	}
	if !emptyValue(result.ReviewPrompts) {
		if err := r.appendEvent(ctx, prepared, "review_prompt", result.ReviewPrompts, false); err != nil {
			return err
		}
	}
	for _, draft := range result.Artifacts {
		artifact := &repository.AIChatArtifact{
			SessionID: prepared.session.ID, RunID: prepared.run.ID, ArtifactType: draft.Type,
			Title: draft.Title, Status: "ready", Data: marshalJSON(draft.Data),
			ReviewRequired: draft.ReviewRequired, CreatedBy: &prepared.input.UserID,
		}
		if err := r.store.CreateArtifact(ctx, artifact); err != nil {
			return fmt.Errorf("persist AI agent artifact: %w", err)
		}
		if err := r.appendEvent(ctx, prepared, "artifact_ready", map[string]any{
			"artifact_id": artifact.ID, "artifact_type": artifact.ArtifactType,
			"title": artifact.Title, "data": draft.Data,
		}, false); err != nil {
			return err
		}
	}
	status := "completed"
	if result.ReviewRequired {
		status = "waiting_review"
	}
	completedAt := r.now()
	summary := result.Answer
	if err := r.store.UpdateRunStatus(ctx, prepared.run.ID, status, result.ReviewRequired, &summary, nil, nil, &completedAt); err != nil {
		return fmt.Errorf("complete AI agent run: %w", err)
	}
	return r.appendEvent(ctx, prepared, "run_end", map[string]any{
		"status": status, "review_required": result.ReviewRequired,
		"artifact_present": len(result.Artifacts) > 0,
	}, true)
}

func (r *Runtime[T]) fail(ctx context.Context, prepared *preparedRun, result Result, executionErr error) error {
	if err := r.persistAssistantMessage(ctx, prepared, result); err != nil {
		return err
	}
	errorText := executionErr.Error()
	completedAt := r.now()
	summary := result.Answer
	if err := r.store.UpdateRunStatus(ctx, prepared.run.ID, "failed", result.ReviewRequired, &summary, &errorText, nil, &completedAt); err != nil {
		return fmt.Errorf("fail AI agent run: %w", err)
	}
	return r.appendEvent(ctx, prepared, "run_error", map[string]any{
		"error": errorText, "model": result.Model,
	}, true)
}

func (r *Runtime[T]) markPersistenceFailure(ctx context.Context, prepared *preparedRun, persistenceErr error) error {
	errorText := persistenceErr.Error()
	summary := "AI agent runtime persistence failed"
	completedAt := r.now()
	if err := r.store.UpdateRunStatus(ctx, prepared.run.ID, "failed", prepared.run.ReviewRequired, &summary, &errorText, nil, &completedAt); err != nil {
		return fmt.Errorf("mark AI agent persistence failure: %w", err)
	}
	return r.appendEvent(ctx, prepared, "run_error", map[string]any{
		"error": errorText, "stage": "persistence",
	}, true)
}

func (r *Runtime[T]) persistAssistantMessage(ctx context.Context, prepared *preparedRun, result Result) error {
	nextSequence, err := r.store.GetNextMessageSequence(ctx, prepared.session.ID)
	if err != nil {
		return fmt.Errorf("allocate AI assistant message sequence: %w", err)
	}
	message := &repository.AIChatMessage{
		SessionID: prepared.session.ID, RunID: &prepared.run.ID, Role: "assistant",
		MessageType: "text", SequenceNo: nextSequence, Content: result.Answer,
		Model: &result.Model, CreatedBy: &prepared.input.UserID, Sources: marshalJSON(result.Sources),
	}
	if err := r.store.CreateMessage(ctx, message); err != nil {
		return fmt.Errorf("persist AI assistant message: %w", err)
	}
	return r.appendEvent(ctx, prepared, "message_end", map[string]any{
		"message_id": message.ID, "role": "assistant", "content": result.Answer,
		"model": result.Model, "sources": result.Sources,
	}, false)
}

func (r *Runtime[T]) appendEvent(ctx context.Context, prepared *preparedRun, eventType string, payload any, terminal bool) error {
	sequence, err := r.store.GetNextRunEventSequence(ctx, prepared.run.ID)
	if err != nil {
		return fmt.Errorf("allocate AI agent event sequence: %w", err)
	}
	if err := r.store.AppendRunEvent(ctx, &repository.AIChatRunEvent{
		RunID: prepared.run.ID, SessionID: prepared.session.ID, SequenceNo: sequence,
		EventType: eventType, Payload: marshalJSON(payload), IsTerminal: terminal,
	}); err != nil {
		return fmt.Errorf("append AI agent %s event: %w", eventType, err)
	}
	return nil
}

func (r *Runtime[T]) started(prepared *preparedRun, continuation *Continuation) *Started {
	return &Started{
		Run: prepared.run, TriggerMessage: prepared.message, Plan: prepared.plan,
		StreamPath:   fmt.Sprintf("/api/v1/ai/chat/runs/%s/stream", prepared.run.ID),
		Continuation: continuation,
	}
}

func attachmentReferences(input Input) []map[string]string {
	if input.FileID == "" {
		return nil
	}
	return []map[string]string{{
		"file_id": input.FileID, "object_name": input.ObjectName, "content_type": input.ContentType,
	}}
}

func summarizeTitle(message string) string {
	runes := []rune(strings.TrimSpace(message))
	if len(runes) == 0 {
		return "新会话"
	}
	if len(runes) > 20 {
		return string(runes[:20]) + "..."
	}
	return string(runes)
}

func emptyValue(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Interface, reflect.Pointer:
		return v.IsNil()
	default:
		return false
	}
}
