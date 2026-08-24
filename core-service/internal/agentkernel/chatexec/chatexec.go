// Package chatexec is the AR5d convergence adapter: the aichat.Executor that
// puts the production chat path onto the vendored agent kernel (ADR-0028 §5).
//
// What it owns, and why:
//
//   - Per-turn governance assembly. Every turn binds a fresh HookManager with
//     the ACORE-2 nine controls mounted (agentkernel/governance.Assembly) and
//     derives a per-turn tool runtime whose ExecutionGuard IS that manager.
//     Nothing per-turn survives the call — the Executor itself is stateless
//     (shape A, D-C19; guarded by TestExecutorHoldsNoPerRunMutableState).
//   - Event bridging. Tool executions stream into the AI Chat timeline via the
//     same "tool_execution" event contract the previous executor installed.
//   - Audit-failure visibility. Adjudication (b): chain-side audit failures are
//     recorded by the mounted control and drained at turn end; non-empty drain
//     fails the whole run instead of passing silently.
//
// What it deliberately does NOT own:
//
//   - Domain execution. Skill routing, prompt building, file triage and the
//     deterministic paper paths stay in aiagent (ADR-0028 §7: first-party
//     domain wiring). The adapter delegates through aiagent.Agent's
//     ExecuteWithRuntime seam with the governed runtime injected.
//   - Persistence, planning and projection. They stay at the aichat.Runtime
//     injection points exactly as before (AR5a §4 disposition table): T remains
//     aiagent.Response, so aiagent.ProjectResult remains the projector.
package chatexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/lease-management-system/core-service/internal/agentkernel/governance"
	picoclawagent "github.com/lease-management-system/core-service/internal/agentkernel/third_party/picoclaw/agent"
	"github.com/lease-management-system/core-service/internal/agentkernel/third_party/picoclaw/tools"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/aiagent"
	"github.com/lease-management-system/core-service/internal/aichat"
)

// DefaultChatToolBudget is the per-turn tool-call budget wired in production.
// The pre-convergence chain ran with no budget (nil); 48 sits far above every
// current chat flow (context gathering + one LLM function-calling round + the
// deterministic paper sequences) while making BudgetGuard a live protection on
// production traffic instead of an inert mount. Reviewers signing off AR5d
// sign off this number.
const DefaultChatToolBudget = 48

// DomainRunner is the domain-execution seam. *aiagent.Agent satisfies it;
// tests substitute a stub to drive the executor without repositories or an
// LLM client.
type DomainRunner interface {
	ExecuteWithRuntime(ctx context.Context, execution aichat.Execution, toolRuntime *agenttools.Runtime) (aiagent.Response, error)
}

// Deps assembles the executor. Everything injected; no DB, no network.
type Deps struct {
	// Domain executes the turn's domain logic with a caller-supplied tool
	// runtime. Production wiring passes the same *aiagent.Agent that also
	// serves as the planner.
	Domain DomainRunner
	// Tools is the shared base tool runtime (registry owner). The executor
	// derives per-turn governed copies; the base itself is never mutated.
	Tools *agenttools.Runtime
	// MaxToolCalls caps governed tool calls per turn; <= 0 means unlimited.
	// Production wiring passes DefaultChatToolBudget.
	MaxToolCalls int
	// ChainAudit loads the chain-level audit control (adjudication b).
	// Production leaves it nil: the runtime layer stays the single owner of
	// "tool_execution" emission and its failure semantics, exactly as before
	// convergence. Setting it routes chain-side audit writes into the
	// turn-end drain — Execute fails the run when any write failed.
	ChainAudit agenttools.AuditRecorder
	// Replay loads the idempotency replay store. Production leaves it nil
	// (parity): the key-required rule stays active, replay short-circuits
	// stay off until a store is productised.
	Replay governance.ReplayStore
	// Sink loads the provenance sink behind ArtifactCollector. Production
	// leaves it nil (no certified-call sink exists yet).
	Sink governance.ArtifactSink
	// Metrics loads the chain-level metrics recorder. Production leaves it
	// nil: the runtime layer observes metrics for every executed call, and
	// wiring both would double-count.
	Metrics *agenttools.RuntimeMetrics
}

// Executor implements aichat.Executor[aiagent.Response] on the kernel.
//
// Shape A (D-C19 / AR5-G3): every field below is a long-lived shared
// dependency or a scalar. There is deliberately no map, channel, mutex,
// mutable slice or per-run/per-account state on this struct — all turn state
// lives in bindTurn's call-local turnKernel. The architecture test enforces
// this mechanically.
type Executor struct {
	domain       DomainRunner
	tools        *agenttools.Runtime
	maxToolCalls int
	chainAudit   agenttools.AuditRecorder
	replay       governance.ReplayStore
	sink         governance.ArtifactSink
	metrics      *agenttools.RuntimeMetrics
}

// New constructs the production chat executor.
func New(deps Deps) *Executor {
	return &Executor{
		domain: deps.Domain, tools: deps.Tools, maxToolCalls: deps.MaxToolCalls,
		chainAudit: deps.ChainAudit, replay: deps.Replay, sink: deps.Sink, metrics: deps.Metrics,
	}
}

// Execute runs one chat turn across the kernel-governed tool plane.
func (e *Executor) Execute(ctx context.Context, exec aichat.Execution) (aiagent.Response, error) {
	if e == nil || e.domain == nil {
		return aiagent.Response{}, errors.New("chat executor has no domain runner")
	}
	kernel := e.bindTurn(exec)
	defer kernel.Hooks.Close()

	response, err := e.domain.ExecuteWithRuntime(ctx, exec, kernel.Runtime)

	// Adjudication (b): audit failures never abort mid-flight, but they must
	// fail the run at turn end. Empty drain = every audit write landed.
	if failures := kernel.recorder.DrainAuditFailures(); len(failures) > 0 {
		joined := failures[0]
		for _, failure := range failures[1:] {
			joined = fmt.Errorf("%v; %w", joined, failure)
		}
		return response, fmt.Errorf("tool execution audit incomplete: %w", joined)
	}
	return response, err
}

// ── per-turn binding (shape A: all mutable state lives here, not on Executor) ──

// turnKernel is one turn's governance binding: the HookManager with the nine
// controls mounted, the derived governed tool runtime, and the per-call facts
// slot the controls read through governance.FactsResolver. The agenttools
// guard frames (Before/After) are sequential within a single Runtime.execute,
// so the slot needs no synchronisation.
type turnKernel struct {
	Hooks    *picoclawagent.HookManager
	Runtime  *agenttools.Runtime
	controls []governance.NamedControl
	recorder *governance.AuditRecorder

	replay             governance.ReplayStore
	requireDraftReview bool

	// per-call slot, written by the guard frames before dispatch
	captured   bool
	call       agenttools.ToolCall
	descriptor agenttools.ToolDescriptor
	principal  agenttools.Principal
	result     *agenttools.ToolResult
}

// bindTurn assembles this turn's kernel binding. It mirrors what the previous
// wiring did in one place — attach the emit bridge and evaluate policy — but
// evaluation now happens inside the vendored HookManager dispatcher.
func (e *Executor) bindTurn(exec aichat.Execution) *turnKernel {
	kernel := &turnKernel{requireDraftReview: true}

	emit := exec.Emit
	controls, recorder := governance.Assembly(governance.Deps{
		Policy: agenttools.DefaultPolicy(),
		Facts:  kernel,
		Budget: &governance.Budget{MaxToolCalls: e.maxToolCalls},
		// Loading points for the two ported controls whose product decisions
		// predate convergence are explicit Deps knobs (see Deps.ChainAudit /
		// Deps.Replay). MeasureResolver, Sink and Metrics stay unwired: the
		// runtime layer owns metrics, and no production measure catalog or
		// provenance sink exists yet — same as the pre-convergence wiring.
		Replay:             e.replay,
		Audit:              e.chainAudit,
		Sink:               e.sink,
		Metrics:            e.metrics,
		RequireDraftReview: kernel.requireDraftReview,
	})
	kernel.controls = controls
	kernel.recorder = recorder
	kernel.replay = e.replay

	hooks := picoclawagent.NewHookManager(nil)
	for i, control := range controls {
		// Explicit priorities are mandatory: equal priorities make
		// HookManager.rebuildOrdered fall back to name ordering, which is NOT
		// the product order (TenantScope must gate first, audit before
		// metrics, ...).
		if err := hooks.Mount(picoclawagent.HookRegistration{
			Name: control.Name, Priority: (i + 1) * 10, Hook: control.Hook,
		}); err != nil {
			// Mount fails only on empty names or nil hooks — both are
			// construction bugs in Assembly itself, not runtime conditions.
			panic(fmt.Sprintf("chatexec: mounting governance control %s: %v", control.Name, err))
		}
	}
	kernel.Hooks = hooks

	runtime := e.tools
	if runtime != nil {
		runtime = runtime.WithAudit(agenttools.AuditRecorderFunc(func(auditCtx context.Context, audit agenttools.ToolExecutionAudit) error {
			if emit == nil {
				return nil
			}
			return emit(auditCtx, "tool_execution", audit)
		}))
		runtime = runtime.WithGuard(kernel)
	}
	kernel.Runtime = runtime
	return kernel
}

// FactsFor implements governance.FactsResolver from the captured guard frame.
func (k *turnKernel) FactsFor(_ context.Context, _ *picoclawagent.ToolCallHookRequest) (governance.CallFacts, error) {
	if !k.captured {
		return governance.CallFacts{}, errors.New("governance call facts were not captured for this hook frame")
	}
	return governance.CallFacts{
		Descriptor: k.descriptor, Call: k.call, Principal: k.principal, Result: k.result,
	}, nil
}

// Before implements agenttools.ExecutionGuard: capture the frame, then let the
// vendored dispatcher run the nine controls.
func (k *turnKernel) Before(ctx context.Context, call agenttools.ToolCall, descriptor agenttools.ToolDescriptor, principal agenttools.Principal) (agenttools.GuardResult, error) {
	k.capture(call, descriptor, principal, nil)
	candidate := k.deriveShortCircuit(ctx, descriptor, call)

	// RC1: the chain writes each rejection's explicit code into this per-call
	// sink; deny sites own the code, this adapter only transports it.
	sink := &governance.RejectSink{}
	ctx = governance.WithRejectSink(ctx, sink)

	request := &picoclawagent.ToolCallHookRequest{
		Meta:      k.meta(call),
		Tool:      call.ToolName,
		Arguments: argumentsMap(call.Arguments),
	}
	_, decision := k.Hooks.BeforeTool(ctx, request)
	switch decision.Action {
	case "", picoclawagent.HookActionContinue, picoclawagent.HookActionModify:
		return agenttools.GuardResult{}, nil
	case picoclawagent.HookActionRespond:
		// Respond bypasses ApproveTool by upstream design (hooks.go SECURITY
		// note). At this seam only two sources exist — ReviewGate and replay
		// — and deriveShortCircuit reconstructs both faithfully from the
		// captured frame without parsing vendor blobs. Anything else is
		// fail-closed: an unaccountable short-circuit must not reach tools.
		if candidate != nil {
			return agenttools.GuardResult{Short: candidate}, nil
		}
		return blocked("tool short-circuit carried no derivable first-party payload"), nil
	default: // deny_tool / abort_turn / hard_abort — rejection reason preserved verbatim
		return agenttools.GuardResult{Block: true, Reason: decision.Reason, Code: sink.Code()}, nil
	}
}

// After implements the after half of agenttools.ExecutionGuard. Hook errors
// are swallowed by the vendored dispatcher by design; audit visibility travels
// through the adjudication-(b) markers drained in Execute.
func (k *turnKernel) After(ctx context.Context, call agenttools.ToolCall, descriptor agenttools.ToolDescriptor, result *agenttools.ToolResult, principal agenttools.Principal) error {
	k.capture(call, descriptor, principal, result)
	response := &picoclawagent.ToolResultHookResponse{
		Meta:      k.meta(call),
		Tool:      call.ToolName,
		Arguments: argumentsMap(call.Arguments),
		Result:    vendorAfterResult(result),
	}
	_, _ = k.Hooks.AfterTool(ctx, response)
	return nil
}

func (k *turnKernel) capture(call agenttools.ToolCall, descriptor agenttools.ToolDescriptor, principal agenttools.Principal, result *agenttools.ToolResult) {
	k.captured = true
	k.call = call
	k.descriptor = descriptor
	k.principal = principal
	k.result = result
}

func (k *turnKernel) meta(call agenttools.ToolCall) picoclawagent.HookMeta {
	return picoclawagent.HookMeta{
		TurnID: call.RunID, SessionKey: call.SkillID, TracePath: call.TraceID,
		Source: "aichat",
	}
}

// deriveShortCircuit rebuilds the first-party result a Respond decision
// stands for. The predicate ORDER comes from governance.ShortCircuitOrder —
// the same sequence Assembly mounts the controls in (AF3-b): when several
// short-circuits apply, chain and derivation must answer identically, so the
// order is written down exactly once.
func (k *turnKernel) deriveShortCircuit(ctx context.Context, descriptor agenttools.ToolDescriptor, call agenttools.ToolCall) *agenttools.ToolResult {
	predicates := map[string]func(context.Context) *agenttools.ToolResult{
		"IdempotencyGuard": func(c context.Context) *agenttools.ToolResult {
			return k.replayShortCircuit(c, call)
		},
		"ReviewGate": func(context.Context) *agenttools.ToolResult {
			return k.reviewShortCircuit(descriptor, call)
		},
	}
	for _, name := range governance.ShortCircuitOrder {
		if predicate, ok := predicates[name]; ok {
			if result := predicate(ctx); result != nil {
				return result
			}
		}
	}
	return nil
}

// replayShortCircuit mirrors IdempotencyGuard's Respond branch.
func (k *turnKernel) replayShortCircuit(ctx context.Context, call agenttools.ToolCall) *agenttools.ToolResult {
	if k.replay != nil && strings.TrimSpace(call.IdempotencyKey) != "" {
		if stored, hit := k.replay.Lookup(ctx, call.IdempotencyKey); hit && stored != nil {
			short := *stored
			if strings.TrimSpace(short.CallID) == "" {
				short.CallID = call.CallID
			}
			return &short
		}
	}
	return nil
}

// reviewShortCircuit mirrors ReviewGate's Respond branch.
func (k *turnKernel) reviewShortCircuit(descriptor agenttools.ToolDescriptor, call agenttools.ToolCall) *agenttools.ToolResult {
	policy := agenttools.Policy{RequireDraftReview: k.requireDraftReview}
	if !agenttools.RequiresReviewDecision(descriptor, policy) || descriptor.Level != agenttools.LevelCommand {
		return nil
	}
	reasons := append([]string(nil), descriptor.Review.Reasons...)
	if len(reasons) == 0 {
		reasons = []string{"tool policy requires human review"}
	}
	return &agenttools.ToolResult{
		CallID: call.CallID,
		Status: agenttools.StatusNeedsReview,
		Review: agenttools.ReviewResult{
			Required: true,
			Reasons:  reasons,
			Actions:  append([]string(nil), descriptor.Review.ConfirmAction),
		},
	}
}

func blocked(reason string) agenttools.GuardResult {
	return agenttools.GuardResult{Block: true, Reason: reason}
}

func argumentsMap(raw json.RawMessage) map[string]any {
	arguments := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &arguments)
	}
	return arguments
}

// vendorAfterResult projects a first-party result into the vendored shape for
// the after phase. The mounted after controls read the canonical status from
// CallFacts.Result (first-party); the vendor blob only carries debug framing.
func vendorAfterResult(result *agenttools.ToolResult) *tools.ToolResult {
	out := &tools.ToolResult{}
	if result == nil {
		return out
	}
	out.ForLLM = string(result.Status)
	if result.Error != nil {
		out.Err = fmt.Errorf("%s", result.Error.Code)
		out.IsError = true
	}
	return out
}
