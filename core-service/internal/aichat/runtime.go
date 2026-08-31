package aichat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agentartifact"
	"github.com/lease-management-system/core-service/internal/repository"
)

const defaultRunTimeout = 4 * time.Minute

type Runtime[T any] struct {
	store        store
	planner      Planner
	executor     Executor[T]
	project      Projector[T]
	dispatch     func(func())
	timeout      time.Duration
	now          func() time.Time
	reviewCommit ReviewCommitFunc
	sessionOwner SessionOwner
}

type preparedRun struct {
	session *repository.AIChatSession
	run     *repository.AIChatRun
	message *repository.AIChatMessage
	input   Input
	plan    Plan
	// releaseSession releases the exclusive per-session lease acquired by the
	// SessionOwner for this run. nil means no lease is held (legacy path or
	// create-only). It fires exactly once after the run finishes.
	releaseSession func()
}

func NewRuntime[T any](persistence *repository.AIChatRuntimeRepository, planner Planner, executor Executor[T], project Projector[T], options Options) *Runtime[T] {
	return newRuntime[T](persistence, planner, executor, project, options)
}

// ExecutorKind reports the concrete type of the wired executor. Diagnostic
// seam for AR5-G1: the convergence assertion must prove which execution plane
// a production runtime instance carries without reaching into unexported
// fields — G1 stayed open for months because no such machine check existed.
func (r *Runtime[T]) ExecutorKind() string {
	if r == nil || r.executor == nil {
		return ""
	}
	return fmt.Sprintf("%T", r.executor)
}

// SessionOwnerKind reports the concrete type of the wired session lifecycle
// owner. Diagnostic seam for SI1 Part B (AR5-G1 pattern): the convergence
// assertion proves production chat session create/load really flows through
// the sessionmanager adapter — empty when nothing is wired (legacy path).
func (r *Runtime[T]) SessionOwnerKind() string {
	if r == nil || r.sessionOwner == nil {
		return ""
	}
	return fmt.Sprintf("%T", r.sessionOwner)
}

// SessionOwner returns the wired AR2 lifecycle seam (nil when the legacy
// store path is in use). Exposed so sibling planes (gateway) can share the
// same session-management authority (RT1-B).
func (r *Runtime[T]) SessionOwner() SessionOwner {
	if r == nil {
		return nil
	}
	return r.sessionOwner
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
		dispatch: dispatch, timeout: timeout, now: now, reviewCommit: options.ReviewCommit,
	}
}

// WithSessionOwner attaches the AR2 lifecycle seam (SI1 Part B). Production
// wiring calls it after construction; tests may attach a fake or leave the
// legacy path. Nil removes the seam.
func (r *Runtime[T]) WithSessionOwner(owner SessionOwner) *Runtime[T] {
	if r == nil {
		return r
	}
	r.sessionOwner = owner
	return r
}

func (r *Runtime[T]) OpenSession(ctx context.Context, command SessionCommand) (*repository.AIChatSession, error) {
	if strings.TrimSpace(command.UserID) == "" {
		return nil, errors.New("AI chat session requires a user")
	}
	if r.sessionOwner != nil {
		// AR2 seam：显式创建也经过 sessionmanager（D3：创建即释放，不持租约，
		// 会话未被 run 占用时不该把自己锁死）。标题语义：显式 command.Title
		// 优先于模块默认。
		session, release, err := r.sessionOwner.ResolveSession(ctx, SessionIntent{
			UserID:          command.UserID,
			LegalEntityID:   command.LegalEntityID,
			Title:           strings.TrimSpace(command.Title),
			ContractID:      command.BoundContractID,
			ContextSnapshot: marshalJSON(command.ContextSnapshot),
			Initiator:       command.Initiator,
			HoldLease:       false,
		})
		if err != nil {
			return nil, fmt.Errorf("create AI chat session: %w", err)
		}
		if release != nil {
			release() // 创建即释放（D3）；防防御性调用（HoldLease=false 本应 nil）
		}
		return session, nil
	}
	session := &repository.AIChatSession{
		UserID: command.UserID, Title: strings.TrimSpace(command.Title),
		ContextSnapshot: marshalJSON(command.ContextSnapshot), Initiator: command.Initiator,
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
	if !prepared.plan.QueueForWorker {
		r.dispatch(func() {
			// Keep authenticated request values (notably the access.Scope installed
			// by DataScopeMiddleware) while detaching the background run from the
			// HTTP request cancellation. The old Background() call silently dropped
			// the scope before asynchronous Agent execution.
			runCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.timeout)
			defer cancel()
			defer releaseAfterRun(prepared)
			r.executeDispatched(runCtx, prepared)
		})
	} else {
		// G1 bridge: the worker plane executes this run outside the chat
		// process (gateway/runner stay under G9). The in-process lease must
		// not hang on a run nobody here finishes; release at dispatch.
		releaseAfterRun(prepared)
	}
	return started, nil
}

func (r *Runtime[T]) Run(ctx context.Context, input Input) (*Completed[T], error) {
	prepared, err := r.prepare(ctx, input, nil, nil)
	if err != nil {
		return nil, err
	}
	defer releaseAfterRun(prepared)
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
	boundary, boundaryErr := entityBoundary(ctx)
	if boundaryErr != nil {
		return nil, fmt.Errorf("resolve AI run inspection boundary: %w", boundaryErr)
	}
	run, err := r.store.GetRunByID(ctx, runID, userID, boundary)
	if err != nil {
		return nil, fmt.Errorf("load AI agent run: %w", err)
	}
	events, err := r.store.ListRunEvents(ctx, runID, afterSequence, 200, boundary, userID)
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
	var releaseSession func()
	if session == nil && r.sessionOwner != nil {
		// AR2 seam: get-or-create AND hold the exclusive per-session lease for
		// the duration of this run. Content dual-read happens inside the owner
		// with the same boundary; the lease is released after execution.
		session, releaseSession, err = r.sessionOwner.ResolveSession(ctx, sessionIntentFromPrepare(input))
		if err != nil {
			return nil, fmt.Errorf("resolve AI chat session: %w", err)
		}
		input.SessionID = session.ID
	} else if session == nil {
		if input.SessionID != "" {
			boundary, boundaryErr := entityBoundary(ctx)
			if boundaryErr != nil {
				return nil, fmt.Errorf("resolve AI chat session boundary: %w", boundaryErr)
			}
			session, err = r.store.GetSessionByID(ctx, input.SessionID, input.UserID, boundary)
			if err != nil {
				return nil, fmt.Errorf("load AI chat session: %w", err)
			}
		} else {
			session = &repository.AIChatSession{
				UserID: input.UserID, Title: summarizeTitle(input.Message),
				ContextSnapshot: marshalJSON(input.PageContext), Initiator: input.Initiator,
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

	prepared := &preparedRun{session: session, run: run, message: message, input: input, plan: plan, releaseSession: releaseSession}
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
	if plan.QueueForWorker {
		// G1 bridge: the run stays queued for the Gateway worker. No
		// in-process execution, no assistant message — the worker's events
		// (run_started → tool_* → run_finished) become the conversation.
		if err := r.appendEvent(ctx, prepared, "run_dispatched", map[string]any{
			"run_id": run.ID, "skill_id": run.SkillID, "status": "queued",
		}, false); err != nil {
			return nil, err
		}
		return prepared, nil
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
		data := marshalJSON(draft.Data)
		artifactID := ""
		if draft.Type == string(agentartifact.ArtifactPageFill) {
			artifactID = uuid.NewString()
			var err error
			data, err = bindPageFillArtifactID(data, artifactID)
			if err != nil {
				return fmt.Errorf("bind page-fill artifact deep link: %w", err)
			}
		}
		artifactProtocol, err := agentartifact.Normalize(agentartifact.Artifact{
			SchemaVersion: draft.SchemaVersion, ArtifactType: agentartifact.ArtifactType(draft.Type),
			Title: draft.Title, Status: "ready", Data: data, EvidenceRefs: draft.EvidenceRefs,
			EvidenceComplete: draft.EvidenceComplete, ReviewRequired: draft.ReviewRequired,
			ReviewReasons: draft.ReviewReasons, ModelVersion: draft.ModelVersion, RuleVersion: draft.RuleVersion,
		})
		if err != nil {
			return fmt.Errorf("validate AI agent artifact: %w", err)
		}
		modelVersion := artifactProtocol.ModelVersion
		ruleVersion := artifactProtocol.RuleVersion
		artifact := &repository.AIChatArtifact{
			ID:        artifactID,
			SessionID: prepared.session.ID, RunID: prepared.run.ID, ArtifactType: draft.Type,
			Title: draft.Title, Status: "ready", Data: data,
			SchemaVersion: artifactProtocol.SchemaVersion, EvidenceRefs: marshalJSON(artifactProtocol.EvidenceRefs),
			EvidenceComplete: artifactProtocol.EvidenceComplete, ReviewReasons: marshalJSON(artifactProtocol.ReviewReasons),
			ModelVersion: stringPointerOrNil(modelVersion), RuleVersion: stringPointerOrNil(ruleVersion),
			ReviewRequired: draft.ReviewRequired, CreatedBy: &prepared.input.UserID,
		}
		if err := r.store.CreateArtifact(ctx, artifact); err != nil {
			return fmt.Errorf("persist AI agent artifact: %w", err)
		}
		if err := r.appendEvent(ctx, prepared, "artifact_ready", map[string]any{
			"artifact_id": artifact.ID, "artifact_type": artifact.ArtifactType,
			"title": artifact.Title, "status": artifact.Status, "schema_version": artifact.SchemaVersion,
			"data": data, "evidence_refs": artifactProtocol.EvidenceRefs,
			"evidence_complete": artifactProtocol.EvidenceComplete, "review_required": artifactProtocol.ReviewRequired,
			"review_reasons": artifactProtocol.ReviewReasons, "model_version": artifactProtocol.ModelVersion,
			"rule_version": artifactProtocol.RuleVersion,
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

// bindPageFillArtifactID makes the deep link address the persisted artifact,
// not the earlier tool-call id used while the fill was being built.
func bindPageFillArtifactID(data json.RawMessage, artifactID string) (json.RawMessage, error) {
	var fill map[string]any
	if err := json.Unmarshal(data, &fill); err != nil {
		return nil, err
	}
	deepLink, ok := fill["deep_link"].(string)
	if !ok || strings.TrimSpace(deepLink) == "" {
		return nil, errors.New("deep_link is required")
	}
	parsed, err := url.Parse(deepLink)
	if err != nil {
		return nil, err
	}
	query := parsed.Query()
	bound := false
	for _, key := range []string{"fill", "schedule_fill", "tb_fill", "plan_fill"} {
		if query.Has(key) {
			query.Set(key, artifactID)
			bound = true
		}
	}
	if !bound {
		return nil, errors.New("deep_link has no supported artifact query parameter")
	}
	parsed.RawQuery = query.Encode()
	fill["deep_link"] = parsed.String()
	return json.Marshal(fill)
}

func stringPointerOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
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
		Confidence: result.Confidence, ConfidenceReason: result.ConfidenceReason,
		MeasuredTokens: result.MeasuredTokens,
	}
	if err := r.store.CreateMessage(ctx, message); err != nil {
		return fmt.Errorf("persist AI assistant message: %w", err)
	}
	return r.appendEvent(ctx, prepared, "message_end", map[string]any{
		"message_id": message.ID, "role": "assistant", "content": result.Answer,
		"model": result.Model, "sources": result.Sources,
		"confidence": result.Confidence, "confidence_reason": result.ConfidenceReason,
		"measured_tokens": result.MeasuredTokens,
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

// entityBoundary resolves the caller's legal-entity boundary for session
// loads from the resolved access.Scope installed in the request context
// (DataScopeMiddleware for the chat plane, gatewayContext for the gateway
// plane) — the same resolver that produced the JWT (D-C4: 该值由与 JWT 同一
// 个解析器产出, 不接受调用方传入). No scope in context fails closed: an
// unresolved identity can never degrade into "no filtering" (SI1).
func entityBoundary(ctx context.Context) (access.EntityFilter, error) {
	scope, ok := access.ScopeFromContext(ctx)
	if !ok {
		return access.EntityFilter{}, fmt.Errorf("no resolved access scope in request context")
	}
	filter, err := access.FromScope(scope)
	if err != nil {
		return access.EntityFilter{}, fmt.Errorf("resolve entity boundary from scope: %w", err)
	}
	return filter, nil
}

// sessionIntentFromPrepare maps a chat run's Input onto the AR2 session
// intent. HoldLease is always true for run paths: the lease spans execution
// so two messages of one conversation serialize (SI1 Part B, D-C4).
func sessionIntentFromPrepare(input Input) SessionIntent {
	return SessionIntent{
		UserID:          input.UserID,
		LegalEntityID:   input.LegalEntityID,
		SessionID:       input.SessionID,
		Title:           summarizeTitle(input.Message),
		ContractID:      input.ContractID,
		ContextSnapshot: marshalJSON(input.PageContext),
		Initiator:       input.Initiator,
		HoldLease:       true,
	}
}

// releaseAfterRun fires the exclusive session lease exactly once after the
// run's execution settles. Safe when nil (legacy path / create-only).
func releaseAfterRun(prepared *preparedRun) {
	if prepared != nil && prepared.releaseSession != nil {
		prepared.releaseSession()
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
