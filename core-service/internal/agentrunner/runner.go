// Package agentrunner provides a Pi-like Agent adapter that can plan and
// execute work through the Agent Gateway. It has no database, MinIO or
// business-repository dependency.
package agentrunner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

var (
	ErrRunIDRequired      = errors.New("agent runner run_id is required")
	ErrPlanNotAllowed     = errors.New("agent runner plan contains a tool outside the Skill allowlist")
	ErrBudgetExceeded     = errors.New("agent runner budget exceeded")
	ErrCapabilityIssue    = errors.New("agent runner could not issue capability")
	ErrPlannerUnavailable = errors.New("agent runner planner is unavailable")
	ErrRunnerGateway      = errors.New("agent runner gateway failure")
	ErrRunAlreadyActive   = errors.New("agent runner run is already active")
	ErrRunNotActive       = errors.New("agent runner run is not active")
)

type Gateway interface {
	Describe(context.Context, agenttools.ToolFilter, string) ([]agenttools.ToolDescriptor, error)
	IssueCapability(context.Context, CapabilityRequest) (string, error)
	Execute(context.Context, agenttools.ToolCall, string) (agenttools.ToolResult, error)
}

// CapabilityRevoker is optional so lightweight test gateways and non-HTTP
// adapters can participate without implementing lifecycle management. The
// production HTTP Gateway implements it and revokes the run grant after the
// Runner reaches a terminal state.
type CapabilityRevoker interface {
	RevokeCapability(context.Context, string) error
}

type CapabilityRequest struct {
	SessionID    string   `json:"session_id,omitempty"`
	RunID        string   `json:"run_id"`
	SkillID      string   `json:"skill_id,omitempty"`
	SkillVersion string   `json:"skill_version,omitempty"`
	AllowedTools []string `json:"allowed_tools"`
}

type PlanRequest struct {
	RunID            string
	SessionID        string
	Message          string
	SkillID          string
	SkillVersion     string
	Tools            []agenttools.ToolDescriptor
	CompletedResults []agenttools.ToolResult
	SteerInstruction string
}

type Planner interface {
	Plan(context.Context, PlanRequest) ([]agenttools.ToolCall, error)
}

// PlannerWithUsage is an optional extension implemented by the HTTP model
// adapter. The base Planner interface stays compatible with deterministic and
// test planners, while production planners can expose measured token usage.
type PlannerWithUsage interface {
	PlanWithUsage(context.Context, PlanRequest) ([]agenttools.ToolCall, *PlannerUsage, error)
}

// PlannerUsage is operational model metadata returned by the AI Service.
// Costs are optional and are never interpreted as accounting amounts by Core.
// A usage record is persisted as a Run event so operators can reconcile the
// provider response and the pricing-book version used to calculate it.
type PlannerUsage struct {
	SchemaVersion  string `json:"schema_version,omitempty"`
	Provider       string `json:"provider,omitempty"`
	Model          string `json:"model,omitempty"`
	InputTokens    *int64 `json:"input_tokens,omitempty"`
	OutputTokens   *int64 `json:"output_tokens,omitempty"`
	TotalTokens    *int64 `json:"total_tokens,omitempty"`
	CostMicros     *int64 `json:"cost_micros,omitempty"`
	CostCurrency   string `json:"cost_currency,omitempty"`
	CostStatus     string `json:"cost_status,omitempty"`
	PricingVersion string `json:"pricing_version,omitempty"`
	PricingSource  string `json:"pricing_source,omitempty"`
}

type PlannerFunc func(context.Context, PlanRequest) ([]agenttools.ToolCall, error)

func (f PlannerFunc) Plan(ctx context.Context, request PlanRequest) ([]agenttools.ToolCall, error) {
	return f(ctx, request)
}

type Limits struct {
	MaxToolCalls   int
	MaxRetries     int
	MaxResultBytes int
	MaxModelTokens int64
	Deadline       time.Duration
}

type Request struct {
	RunID        string
	SessionID    string
	Message      string
	SkillID      string
	SkillVersion string
	Limits       Limits
	Resume       bool
	WorkerID     string
	LeaseToken   string
	LeaseSeconds int
}

type Event struct {
	Type    string `json:"type"`
	RunID   string `json:"run_id"`
	CallID  string `json:"call_id,omitempty"`
	Payload any    `json:"payload,omitempty"`
}

// EventRecorder is an optional durable event sink. Emit remains available for
// local observers, while the recorder lets a Runner append the same protocol
// events to Core's Run event stream without exposing a database to the Agent.
type EventRecorder interface {
	Record(context.Context, Event) error
}

// RunEventReader is optional. When the Gateway supports durable Run events,
// the Runner polls the control stream so a steer submitted through Core's HTTP
// API reaches the active process instead of becoming an orphaned audit event.
type RunEventReader interface {
	ListRunEvents(context.Context, string, int, int) (RunEventPage, error)
}

// RunEventSubscription is the live event transport used by a production
// worker. The subscription is intentionally narrower than an HTTP response:
// the Runner only receives the versioned Run event stream and never receives
// a database handle or an arbitrary Core endpoint.
type RunEventSubscription struct {
	Events <-chan RunEvent
	Errors <-chan error
	Close  func()
}

// RunEventSubscriber is optional. Gateways that do not provide SSE continue
// to use RunEventReader polling, which keeps local and older adapters
// compatible while the worker deployment uses a real live stream.
type RunEventSubscriber interface {
	SubscribeRunEvents(context.Context, string, int) (*RunEventSubscription, error)
}

// RunLeaseManager is optional. A production worker supplies the lease token
// returned by Core's atomic claim endpoint; local/embedded runners can omit it.
type RunLeaseManager interface {
	HeartbeatRunLease(context.Context, string, string, string, int) error
	ReleaseRunLease(context.Context, string, string, string, bool) error
}

type Result struct {
	RunID       string
	Status      string
	ToolResults []agenttools.ToolResult
	Events      []Event
	CallCount   int
	RetryCount  int
	ModelTokens int64
	Error       string
}

type Runner struct {
	Gateway       Gateway
	Planner       Planner
	Emit          func(Event)
	EventRecorder EventRecorder
	Now           func() time.Time
	Checkpoints   CheckpointStore

	stateMu sync.Mutex
	active  map[string]bool
	steers  map[string][]string
}

// Steer queues a human or supervisor instruction for the next planning
// boundary of an active run. The Runner will re-plan with the instruction and
// the verified Tool results already collected in that run.
func (r *Runner) Steer(runID, instruction string) error {
	if r == nil {
		return ErrRunNotActive
	}
	runID = strings.TrimSpace(runID)
	instruction = strings.TrimSpace(instruction)
	if runID == "" || instruction == "" {
		return ErrRunNotActive
	}
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	if !r.active[runID] {
		return ErrRunNotActive
	}
	r.steers[runID] = append(r.steers[runID], instruction)
	return nil
}

func (r *Runner) beginRun(runID string) error {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	if r.active == nil {
		r.active = make(map[string]bool)
	}
	if r.steers == nil {
		r.steers = make(map[string][]string)
	}
	if r.active[runID] {
		return ErrRunAlreadyActive
	}
	r.active[runID] = true
	return nil
}

func (r *Runner) endRun(runID string) {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	delete(r.active, runID)
	delete(r.steers, runID)
}

func (r *Runner) takeSteer(runID string) string {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	queued := r.steers[runID]
	if len(queued) == 0 {
		return ""
	}
	delete(r.steers, runID)
	return strings.Join(queued, "\n")
}

func (r *Runner) readRemoteControl(ctx context.Context, runID string, afterSequence int) (string, bool, int, error) {
	reader, ok := r.Gateway.(RunEventReader)
	if !ok {
		return "", false, afterSequence, nil
	}
	page, err := reader.ListRunEvents(ctx, runID, afterSequence, 200)
	if err != nil {
		return "", false, afterSequence, err
	}
	nextSequence := afterSequence
	cancelled := false
	instructions := make([]string, 0)
	for _, event := range page.Events {
		if event.SequenceNo > nextSequence {
			nextSequence = event.SequenceNo
		}
		if event.EventType == "run_cancelled" {
			cancelled = true
		}
		if event.EventType != "run_steer" && event.EventType != "run_follow_up" {
			continue
		}
		var envelope struct {
			Payload json.RawMessage `json:"payload"`
		}
		if json.Unmarshal(event.Payload, &envelope) != nil {
			continue
		}
		controlPayload := envelope.Payload
		if len(controlPayload) == 0 {
			controlPayload = event.Payload
		}
		var control struct {
			Instruction string `json:"instruction"`
		}
		if json.Unmarshal(controlPayload, &control) == nil && strings.TrimSpace(control.Instruction) != "" {
			instructions = append(instructions, strings.TrimSpace(control.Instruction))
		}
	}
	return strings.Join(instructions, "\n"), cancelled, nextSequence, nil
}

func readRemoteControlStream(ctx context.Context, subscription *RunEventSubscription, afterSequence int) (string, bool, int, error) {
	if subscription == nil {
		return "", false, afterSequence, nil
	}
	nextSequence := afterSequence
	cancelled := false
	instructions := make([]string, 0)
	events := subscription.Events
	errorsChannel := subscription.Errors
	for events != nil || errorsChannel != nil {
		select {
		case <-ctx.Done():
			return "", false, nextSequence, ctx.Err()
		case err, ok := <-errorsChannel:
			if !ok {
				errorsChannel = nil
				continue
			}
			if err != nil {
				return "", false, nextSequence, err
			}
		case event, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			if event.SequenceNo <= nextSequence {
				continue
			}
			nextSequence = event.SequenceNo
			if event.EventType == "run_cancelled" {
				cancelled = true
			}
			if event.EventType != "run_steer" && event.EventType != "run_follow_up" {
				continue
			}
			var envelope struct {
				Payload json.RawMessage `json:"payload"`
			}
			if json.Unmarshal(event.Payload, &envelope) != nil {
				continue
			}
			controlPayload := envelope.Payload
			if len(controlPayload) == 0 {
				controlPayload = event.Payload
			}
			var control struct {
				Instruction string `json:"instruction"`
			}
			if json.Unmarshal(controlPayload, &control) == nil && strings.TrimSpace(control.Instruction) != "" {
				instructions = append(instructions, strings.TrimSpace(control.Instruction))
			}
		default:
			return strings.Join(instructions, "\n"), cancelled, nextSequence, nil
		}
	}
	return strings.Join(instructions, "\n"), cancelled, nextSequence, nil
}

func (r *Runner) Run(ctx context.Context, request Request) (Result, error) {
	result := Result{RunID: strings.TrimSpace(request.RunID), Status: "queued"}
	if result.RunID == "" {
		return result, ErrRunIDRequired
	}
	if r == nil || r.Gateway == nil {
		return result, ErrRunnerGateway
	}
	if r.Planner == nil {
		return result, ErrPlannerUnavailable
	}
	if err := r.beginRun(result.RunID); err != nil {
		return result, err
	}
	defer r.endRun(result.RunID)
	if leaseManager, ok := r.Gateway.(RunLeaseManager); ok && strings.TrimSpace(request.WorkerID) != "" && strings.TrimSpace(request.LeaseToken) != "" {
		leaseSeconds := request.LeaseSeconds
		if leaseSeconds <= 0 {
			leaseSeconds = 60
		}
		leaseCtx, cancelLease := context.WithCancel(ctx)
		ctx = leaseCtx
		go r.heartbeatLease(leaseCtx, cancelLease, leaseManager, result.RunID, request.WorkerID, request.LeaseToken, leaseSeconds)
		defer func() {
			cancelLease()
			_ = leaseManager.ReleaseRunLease(context.WithoutCancel(ctx), result.RunID, request.WorkerID, request.LeaseToken, false)
		}()
	}
	limits := request.Limits
	if limits.MaxToolCalls <= 0 {
		limits.MaxToolCalls = 12
	}
	if limits.MaxRetries < 0 {
		limits.MaxRetries = 0
	}
	if limits.MaxResultBytes <= 0 {
		limits.MaxResultBytes = 2 << 20
	}
	if limits.Deadline > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, limits.Deadline)
		defer cancel()
	}
	r.emit(Event{Type: "run_started", RunID: result.RunID, Payload: request})

	descriptors, err := r.Gateway.Describe(ctx, agenttools.ToolFilter{SkillID: request.SkillID, IncludeSchema: true}, result.RunID)
	if err != nil {
		return r.fail(result, err)
	}
	allowed := make(map[string]agenttools.ToolDescriptor, len(descriptors))
	allowedNames := make([]string, 0, len(descriptors))
	for _, descriptor := range descriptors {
		allowed[descriptor.Name] = descriptor
		allowedNames = append(allowedNames, descriptor.Name)
	}
	if len(allowedNames) == 0 {
		return r.fail(result, fmt.Errorf("%w: no discoverable tools for skill %q", ErrRunnerGateway, request.SkillID))
	}
	capability, err := r.Gateway.IssueCapability(ctx, CapabilityRequest{SessionID: request.SessionID, RunID: result.RunID, SkillID: request.SkillID, SkillVersion: request.SkillVersion, AllowedTools: allowedNames})
	if err != nil {
		return r.fail(result, fmt.Errorf("%w: %v", ErrCapabilityIssue, err))
	}
	defer func() {
		if revoker, ok := r.Gateway.(CapabilityRevoker); ok {
			_ = revoker.RevokeCapability(context.WithoutCancel(ctx), result.RunID)
		}
	}()
	var plan []agenttools.ToolCall
	var plannerUsage *PlannerUsage
	startIndex := 0
	if request.Resume && r.Checkpoints != nil {
		checkpoint, checkpointErr := r.Checkpoints.Load(ctx, result.RunID)
		if checkpointErr == nil && checkpoint.SkillID == request.SkillID && checkpoint.SkillVersion == request.SkillVersion {
			plan = checkpoint.Plan
			startIndex = checkpoint.NextIndex
			result.ToolResults = append(result.ToolResults, checkpoint.ToolResults...)
			r.emit(Event{Type: "run_resumed", RunID: result.RunID, Payload: map[string]any{"next_index": startIndex}})
		}
	}
	if plan == nil {
		var planErr error
		plan, plannerUsage, planErr = r.plan(ctx, PlanRequest{RunID: result.RunID, SessionID: request.SessionID, Message: request.Message, SkillID: request.SkillID, SkillVersion: request.SkillVersion, Tools: descriptors, CompletedResults: append([]agenttools.ToolResult(nil), result.ToolResults...)})
		err = planErr
		if err != nil {
			return r.fail(result, fmt.Errorf("plan: %w", err))
		}
		if err := applyPlannerUsage(&result, plannerUsage, limits.MaxModelTokens); err != nil {
			return r.fail(result, err)
		}
	}
	r.saveCheckpoint(ctx, request, plan, startIndex, result)
	result.Status = "running"
	index := startIndex
	eventSequence := 0
	var subscription *RunEventSubscription
	if subscriber, ok := r.Gateway.(RunEventSubscriber); ok {
		subscription, err = subscriber.SubscribeRunEvents(ctx, result.RunID, eventSequence)
		if err != nil {
			return r.fail(result, fmt.Errorf("subscribe run events: %w", err))
		}
		if subscription != nil && subscription.Close != nil {
			defer subscription.Close()
		}
	}
	readControls := func(readCtx context.Context, after int) (string, bool, int, error) {
		if subscription != nil {
			return readRemoteControlStream(readCtx, subscription, after)
		}
		return r.readRemoteControl(readCtx, result.RunID, after)
	}
	replan := func(instruction string) error {
		steeredPlan, usage, planErr := r.plan(ctx, PlanRequest{
			RunID: result.RunID, SessionID: request.SessionID, Message: request.Message,
			SkillID: request.SkillID, SkillVersion: request.SkillVersion, Tools: descriptors,
			CompletedResults: append([]agenttools.ToolResult(nil), result.ToolResults...), SteerInstruction: instruction,
		})
		if planErr != nil {
			return fmt.Errorf("steer plan: %w", planErr)
		}
		if err := applyPlannerUsage(&result, usage, limits.MaxModelTokens); err != nil {
			return err
		}
		plan = steeredPlan
		index = 0
		r.emit(Event{Type: "run_steered", RunID: result.RunID, Payload: map[string]any{"instruction": instruction}})
		r.saveCheckpoint(ctx, request, plan, index, result)
		return nil
	}
	for index < len(plan) {
		if instruction, cancelled, nextSequence, pollErr := readControls(ctx, eventSequence); pollErr != nil {
			return r.fail(result, fmt.Errorf("read run control events: %w", pollErr))
		} else {
			eventSequence = nextSequence
			if cancelled {
				result.Status = "cancelled"
				result.Error = "cancelled by user"
				r.saveCheckpoint(ctx, request, plan, index, result)
				r.emit(Event{Type: "run_cancelled", RunID: result.RunID, Payload: result.Error})
				return result, context.Canceled
			}
			if instruction != "" {
				if err := replan(instruction); err != nil {
					return r.fail(result, err)
				}
				continue
			}
		}
		planned := plan[index]
		if err := ctx.Err(); err != nil {
			result.Status = "cancelled"
			result.Error = err.Error()
			r.saveCheckpoint(ctx, request, plan, index, result)
			r.emit(Event{Type: "run_cancelled", RunID: result.RunID, Payload: result.Error})
			return result, err
		}
		if result.CallCount >= limits.MaxToolCalls {
			return r.fail(result, ErrBudgetExceeded)
		}
		descriptor, ok := allowed[planned.ToolName]
		if !ok {
			return r.fail(result, fmt.Errorf("%w: %s", ErrPlanNotAllowed, planned.ToolName))
		}
		planned.RunID = result.RunID
		planned.SkillID = request.SkillID
		planned.SkillVersion = request.SkillVersion
		if strings.TrimSpace(planned.CallID) == "" {
			planned.CallID = uuid.NewString()
		}
		if strings.TrimSpace(planned.ToolVersion) == "" {
			planned.ToolVersion = descriptor.Version
		}
		if descriptor.Level != agenttools.LevelRead && strings.TrimSpace(planned.IdempotencyKey) == "" {
			return r.fail(result, fmt.Errorf("%w: write Tool %s requires idempotency_key", ErrPlanNotAllowed, planned.ToolName))
		}
		attempts := 0
		replannedRun := false
		for {
			attempts++
			result.CallCount++
			r.emit(Event{Type: "tool_started", RunID: result.RunID, CallID: planned.CallID, Payload: planned.ToolName})
			toolResult, executionErr := r.Gateway.Execute(ctx, planned, capability)
			if executionErr != nil {
				if attempts <= limits.MaxRetries+1 && ctx.Err() == nil {
					result.RetryCount++
					continue
				}
				return r.fail(result, fmt.Errorf("%w: %v", ErrRunnerGateway, executionErr))
			}
			if encoded, marshalErr := json.Marshal(toolResult.Data); marshalErr == nil && len(encoded) > limits.MaxResultBytes {
				return r.fail(result, fmt.Errorf("%w: result size for %s exceeds limit", ErrBudgetExceeded, planned.ToolName))
			}
			result.ToolResults = append(result.ToolResults, toolResult)
			r.saveCheckpoint(ctx, request, plan, index+1, result)
			r.emit(Event{Type: "tool_completed", RunID: result.RunID, CallID: planned.CallID, Payload: toolResult})
			if toolResult.Error != nil && toolResult.Error.Retryable && attempts <= limits.MaxRetries {
				result.RetryCount++
				continue
			}
			if toolResult.Status == agenttools.StatusFailed || toolResult.Status == agenttools.StatusRejected {
				result.Status = "failed"
				if toolResult.Error != nil {
					result.Error = toolResult.Error.Message
				}
				r.saveCheckpoint(ctx, request, plan, index+1, result)
				r.emit(Event{Type: "run_failed", RunID: result.RunID, CallID: planned.CallID, Payload: result.Error})
				return result, nil
			}
			if toolResult.Status == agenttools.StatusNeedsReview {
				result.Status = "waiting_review"
			}
			instruction := r.takeSteer(result.RunID)
			if instruction == "" {
				var pollErr error
				var cancelled bool
				instruction, cancelled, eventSequence, pollErr = readControls(ctx, eventSequence)
				if pollErr != nil {
					return r.fail(result, fmt.Errorf("read run control events: %w", pollErr))
				}
				if cancelled {
					result.Status = "cancelled"
					result.Error = "cancelled by user"
					r.saveCheckpoint(ctx, request, plan, index+1, result)
					r.emit(Event{Type: "run_cancelled", RunID: result.RunID, Payload: result.Error})
					return result, context.Canceled
				}
			}
			if instruction != "" {
				if err := replan(instruction); err != nil {
					return r.fail(result, err)
				}
				replannedRun = true
				break
			}
			break
		}
		if replannedRun {
			continue
		}
		index++
	}
	if result.Status == "running" {
		result.Status = "completed"
	}
	r.emit(Event{Type: "run_finished", RunID: result.RunID, Payload: result.Status})
	r.saveCheckpoint(ctx, request, plan, len(plan), result)
	return result, nil
}

func (r *Runner) saveCheckpoint(ctx context.Context, request Request, plan []agenttools.ToolCall, nextIndex int, result Result) {
	if r == nil || r.Checkpoints == nil {
		return
	}
	_ = r.Checkpoints.Save(ctx, Checkpoint{
		SchemaVersion: CheckpointSchemaVersion, RunID: result.RunID, SessionID: request.SessionID, SkillID: request.SkillID,
		SkillVersion: request.SkillVersion, Message: request.Message, Plan: plan,
		NextIndex: nextIndex, ToolResults: result.ToolResults, Status: result.Status,
		UpdatedAt: time.Now().UTC(),
	})
}

func (r *Runner) plan(ctx context.Context, request PlanRequest) ([]agenttools.ToolCall, *PlannerUsage, error) {
	if planner, ok := r.Planner.(PlannerWithUsage); ok {
		calls, usage, err := planner.PlanWithUsage(ctx, request)
		return calls, usage, err
	}
	calls, err := r.Planner.Plan(ctx, request)
	return calls, nil, err
}

func applyPlannerUsage(result *Result, usage *PlannerUsage, limit int64) error {
	if result == nil || usage == nil {
		return nil
	}
	tokens := int64(0)
	if usage.TotalTokens != nil {
		tokens = *usage.TotalTokens
	} else if usage.InputTokens != nil && usage.OutputTokens != nil {
		tokens = *usage.InputTokens + *usage.OutputTokens
	}
	if tokens < 0 {
		return fmt.Errorf("%w: planner reported negative token usage", ErrBudgetExceeded)
	}
	result.ModelTokens += tokens
	if limit > 0 && result.ModelTokens > limit {
		return fmt.Errorf("%w: model token usage %d exceeds limit %d", ErrBudgetExceeded, result.ModelTokens, limit)
	}
	return nil
}

func (r *Runner) fail(result Result, err error) (Result, error) {
	result.Status = "failed"
	result.Error = err.Error()
	r.emit(Event{Type: "run_failed", RunID: result.RunID, Payload: result.Error})
	return result, err
}

func (r *Runner) heartbeatLease(ctx context.Context, cancel context.CancelFunc, manager RunLeaseManager, runID, workerID, leaseToken string, leaseSeconds int) {
	interval := time.Duration(leaseSeconds/3) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := manager.HeartbeatRunLease(ctx, runID, workerID, leaseToken, leaseSeconds); err != nil {
				cancel()
				return
			}
		}
	}
}

func (r *Runner) emit(event Event) {
	if r != nil && r.EventRecorder != nil {
		_ = r.EventRecorder.Record(context.WithoutCancel(context.Background()), event)
	}
	if r != nil && r.Emit != nil {
		r.Emit(event)
	}
}

var _ Planner = PlannerFunc(nil)
