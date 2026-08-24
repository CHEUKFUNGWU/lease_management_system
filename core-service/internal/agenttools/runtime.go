package agenttools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const defaultToolTimeout = 30 * time.Second

// GuardResult is the outcome of a before-guard: block the call with a reason,
// or replace execution with a short-circuit result.
type GuardResult struct {
	Block  bool
	Reason string
	Short  *ToolResult
}

// ExecutionGuard is the seam the governance middleware chain (W2) crosses.
// Before runs after registry resolution and parameter validation; After runs
// after the handler returned. The runtime keeps this interface free of any
// agentcore dependency — the chain adapter lives in agentcore/hooks.
type ExecutionGuard interface {
	Before(ctx context.Context, call ToolCall, descriptor ToolDescriptor, principal Principal) (GuardResult, error)
	After(ctx context.Context, call ToolCall, descriptor ToolDescriptor, result *ToolResult, principal Principal) error
}

type RuntimeOptions struct {
	Policy  Policy
	Timeout time.Duration
	Audit   AuditRecorder
	Now     func() time.Time
	Metrics *RuntimeMetrics
	// Guard replaces the inline policy evaluation with the ordered governance
	// chain when set. Behaviour is equivalent; the chain is the W2 mount
	// point for future controls.
	Guard ExecutionGuard
}

type Runtime struct {
	registry *Registry
	policy   Policy
	timeout  time.Duration
	audit    AuditRecorder
	now      func() time.Time
	metrics  *RuntimeMetrics
	guard    ExecutionGuard
}

func NewRuntime(registry *Registry, options RuntimeOptions) *Runtime {
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = defaultToolTimeout
	}
	policy := options.Policy
	if policy.AllowedLevels == nil {
		policy = DefaultPolicy()
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	metrics := options.Metrics
	if metrics == nil {
		metrics = NewRuntimeMetrics()
	}
	return &Runtime{registry: registry, policy: policy, timeout: timeout, audit: options.Audit, now: now, metrics: metrics, guard: options.Guard}
}

// Metrics returns the shared process metrics sink. WithAudit keeps this sink
// shared so the Web Agent and standalone Gateway observe one aggregate.
func (r *Runtime) Metrics() *RuntimeMetrics {
	if r == nil {
		return nil
	}
	return r.metrics
}

// WithAudit returns a shallow Adapter of the same Runtime with a per-run
// recorder. The registry and policy remain shared; the Agent can therefore
// attach the current AI Chat event stream without mutating a concurrent
// production Runtime.
func (r *Runtime) WithAudit(audit AuditRecorder) *Runtime {
	if r == nil {
		return nil
	}
	copy := *r
	copy.audit = audit
	return &copy
}

// WithGuard returns a shallow Adapter of the same Runtime with a per-run
// governance guard. It is the AR5d convergence seam: the kernel executor
// derives a per-turn runtime whose guard is the vendored HookManager chain,
// while the shared base Runtime keeps its construction-time guard for the
// planes that have not converged yet.
func (r *Runtime) WithGuard(guard ExecutionGuard) *Runtime {
	if r == nil {
		return nil
	}
	copy := *r
	copy.guard = guard
	return &copy
}

func (r *Runtime) Describe(ctx context.Context, filter ToolFilter) ([]ToolDescriptor, error) {
	execution, err := RequireExecutionContext(ctx)
	if err != nil {
		return nil, err
	}
	if r == nil || r.registry == nil {
		return nil, fmt.Errorf("%w: registry is unavailable", ErrInvalidToolDescriptor)
	}

	definitions := r.registry.definitions(filter)
	result := make([]ToolDescriptor, 0, len(definitions))
	for _, definition := range definitions {
		descriptor := definition.Descriptor
		if !r.discoverable(execution.Principal, descriptor) {
			continue
		}
		if !filter.IncludeSchema {
			descriptor.InputSchema = nil
			descriptor.OutputSchema = nil
		}
		result = append(result, descriptor)
	}
	return result, nil
}

func (r *Runtime) Execute(ctx context.Context, call ToolCall) (ToolResult, error) {
	now := time.Now
	if r != nil && r.now != nil {
		now = r.now
	}
	startedAt := now()
	result, executionErr := r.execute(ctx, call)
	if auditErr := r.recordAudit(ctx, call, result, startedAt, now()); auditErr != nil {
		if result.Error == nil {
			result.Status = StatusFailed
			result.Error = &ToolError{Code: ErrorSystemFailure, Message: "tool execution audit failed", Retryable: true}
		}
		if executionErr == nil {
			executionErr = fmt.Errorf("record tool execution audit: %w", auditErr)
		}
	}
	return result, executionErr
}

func (r *Runtime) execute(ctx context.Context, call ToolCall) (ToolResult, error) {
	result := ToolResult{CallID: call.CallID}
	if err := call.Validate(); err != nil {
		return rejectedResult(call.CallID, ErrorInvalidArguments, err.Error(), false), nil
	}
	if r == nil || r.registry == nil {
		return rejectedResult(call.CallID, ErrorSystemFailure, "tool registry is unavailable", true), nil
	}
	if _, err := RequireExecutionContext(ctx); err != nil {
		return rejectedResult(call.CallID, ErrorUnauthenticated, "authenticated tool context is required", false), nil
	}
	definition, found := r.registry.Resolve(call.ToolName, call.ToolVersion)
	if !found {
		return rejectedResult(call.CallID, ErrorNotFound, "tool is not registered", false), nil
	}

	var execution ExecutionContext
	decision := PolicyDecision{Allowed: true}
	var err error
	if r.guard != nil {
		execution, _ = ExecutionContextFromContext(ctx)
		out, err := r.guard.Before(ctx, call, definition.Descriptor, execution.Principal)
		if err != nil {
			return rejectedResult(call.CallID, policyErrorCode(err), publicPolicyError(err), false), nil
		}
		if out.Block {
			return rejectedResult(call.CallID, ErrorScopeDenied, out.Reason, false), nil
		}
		if out.Short != nil {
			short := *out.Short
			if short.CallID == "" {
				short.CallID = call.CallID
			}
			return short, nil
		}
		decision = ReviewDecision(definition.Descriptor, r.policy)
	} else {
		var err error
		decision, err = Evaluate(ctx, definition.Descriptor, call, r.policy)
		if err != nil {
			return rejectedResult(call.CallID, policyErrorCode(err), publicPolicyError(err), false), nil
		}
	}
	result.Review = ReviewResult{
		Required: decision.RequiresReview,
		Reasons:  append([]string(nil), decision.Reasons...),
	}
	if decision.RequiresReview && definition.Descriptor.Level == LevelCommand {
		result.Status = StatusNeedsReview
		return result, nil
	}

	timeout := r.timeout
	if definition.Descriptor.TimeoutSeconds > 0 {
		timeout = time.Duration(definition.Descriptor.TimeoutSeconds) * time.Second
	}
	executionCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err = executeHandler(executionCtx, definition.Handler, call)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return failedResult(call.CallID, ErrorCancelled, "tool execution was cancelled", false), nil
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return failedResult(call.CallID, ErrorTimeout, "tool execution timed out", true), nil
		}
		return failedResult(call.CallID, ErrorBusinessFailure, "tool execution failed", false), err
	}
	result.CallID = call.CallID
	if result.Status == "" {
		result.Status = StatusCompleted
	}
	if r.guard != nil {
		if err := r.guard.After(ctx, call, definition.Descriptor, &result, execution.Principal); err != nil {
			result.Status = StatusFailed
			if result.Error == nil {
				result.Error = &ToolError{Code: ErrorSystemFailure, Message: "post-execution guard failed", Retryable: true}
			}
			return result, err
		}
	}
	if decision.RequiresReview {
		result.Status = StatusNeedsReview
	}
	result.Review = ReviewResult{
		Required: decision.RequiresReview,
		Reasons:  append([]string(nil), decision.Reasons...),
		Actions:  append([]string(nil), definition.Descriptor.Review.ConfirmAction),
	}
	return result, nil
}

func (r *Runtime) recordAudit(ctx context.Context, call ToolCall, result ToolResult, startedAt, completedAt time.Time) error {
	if r == nil {
		return nil
	}
	audit := ToolExecutionAudit{
		CallID:          call.CallID,
		RunID:           call.RunID,
		TraceID:         call.TraceID,
		ToolName:        call.ToolName,
		ToolVersion:     call.ToolVersion,
		Status:          result.Status,
		ArgumentsSHA256: argumentsFingerprint(call.Arguments),
		ReviewRequired:  result.Review.Required,
		StartedAt:       startedAt,
		CompletedAt:     completedAt,
		DurationMillis:  completedAt.Sub(startedAt).Milliseconds(),
	}
	if result.Error != nil {
		audit.ErrorCode = result.Error.Code
	}
	if execution, ok := ExecutionContextFromContext(ctx); ok {
		audit.UserID = execution.Principal.UserID
		audit.SubjectType = execution.Principal.SubjectType
		audit.LegalEntityID = execution.Principal.Scope.LegalEntityID
		audit.SkillID = execution.SkillID
		audit.SkillVersion = execution.SkillVersion
		if audit.RunID == "" {
			audit.RunID = execution.RunID
		}
		if audit.TraceID == "" {
			audit.TraceID = execution.TraceID
		}
	}
	if r.metrics != nil {
		r.metrics.Observe(audit)
	}
	if r.audit == nil {
		return nil
	}
	return r.audit.RecordToolExecution(context.WithoutCancel(ctx), audit)
}

func (r *Runtime) discoverable(principal Principal, descriptor ToolDescriptor) bool {
	if !r.policy.AllowedLevels[descriptor.Level] {
		return false
	}
	if principal.CapabilityActive && !principal.HasCapability(descriptor.Name) {
		return false
	}
	if descriptor.Level == LevelCommand && (!r.policy.AllowCommand || !principal.HasCapability(descriptor.Name)) {
		return false
	}
	for _, permission := range descriptor.Permissions {
		if !principal.HasPermission(permission) {
			return false
		}
	}
	return true
}

func executeHandler(ctx context.Context, handler ToolHandler, call ToolCall) (result ToolResult, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("tool handler panic: %v", recovered)
		}
	}()
	return handler(ctx, call)
}

func rejectedResult(callID string, code ErrorCode, message string, retryable bool) ToolResult {
	return ToolResult{
		CallID: callID,
		Status: StatusRejected,
		Error:  &ToolError{Code: code, Message: message, Retryable: retryable},
	}
}

func failedResult(callID string, code ErrorCode, message string, retryable bool) ToolResult {
	return ToolResult{
		CallID: callID,
		Status: StatusFailed,
		Error:  &ToolError{Code: code, Message: message, Retryable: retryable},
	}
}

func policyErrorCode(err error) ErrorCode {
	switch {
	case errors.Is(err, ErrExecutionContextRequired):
		return ErrorUnauthenticated
	case errors.Is(err, ErrToolCapabilityRequired):
		return ErrorCapabilityDenied
	case errors.Is(err, ErrToolNotPermitted):
		return ErrorPermissionDenied
	default:
		return ErrorInvalidArguments
	}
}

func publicPolicyError(err error) string {
	switch {
	case errors.Is(err, ErrToolCapabilityRequired):
		return "tool capability is not granted"
	case errors.Is(err, ErrToolNotPermitted):
		return "tool permission is not granted"
	case errors.Is(err, ErrInvalidToolCall):
		return "tool call arguments or identity are invalid"
	default:
		return strings.TrimSpace(err.Error())
	}
}

var _ ToolRuntime = (*Runtime)(nil)
