package handlers

// RT1-D-1: the gateway plane's tool execution crosses the NEW governance
// chain (agentkernel/governance — the same nine controls the chat plane runs
// through chatexec). This is the second convergence (第一次是 AR5d chat 平面);
// the gateway's Deps are decided per-surface, not copied from chatexec.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	picoclawagent "github.com/lease-management-system/core-service/internal/agentkernel/third_party/picoclaw/agent"
	"github.com/lease-management-system/core-service/internal/agentkernel/third_party/picoclaw/tools"
	"github.com/lease-management-system/core-service/internal/agentkernel/governance"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

// gatewayFactsKey carries one call's facts frame through the context. The
// chatexec turnKernel stores the frame in struct fields (safe: one goroutine
// per turn); the gateway shares ONE runtime across concurrent requests, so
// the frame must ride the request context, never shared mutable state.
type gatewayFactsKey struct{}

type gatewayFacts struct {
	descriptor agenttools.ToolDescriptor
	call       agenttools.ToolCall
	principal  agenttools.Principal
	result     *agenttools.ToolResult
}

func withGatewayFacts(ctx context.Context, facts gatewayFacts) context.Context {
	return context.WithValue(ctx, gatewayFactsKey{}, &facts)
}

func gatewayFactsFromContext(ctx context.Context) (*gatewayFacts, bool) {
	facts, ok := ctx.Value(gatewayFactsKey{}).(*gatewayFacts)
	return facts, ok
}

// gatewayFactsResolver reads the per-request facts frame the guard captured.
type gatewayFactsResolver struct{}

func (gatewayFactsResolver) FactsFor(ctx context.Context, _ *picoclawagent.ToolCallHookRequest) (governance.CallFacts, error) {
	facts, ok := gatewayFactsFromContext(ctx)
	if !ok || facts == nil {
		return governance.CallFacts{}, fmt.Errorf("gateway governance call facts were not captured")
	}
	return governance.CallFacts{
		Descriptor: facts.descriptor, Call: facts.call, Principal: facts.principal, Result: facts.result,
	}, nil
}

// governedGatewayGuard implements agenttools.ExecutionGuard by running the
// ACORE-2 nine controls (governance.Assembly) through a vendored HookManager.
// It is the NEW chain — same as chatexec's chat plane — mounted once at
// construction and driven per request through the context-carried facts frame.
//
// footer order (RT1-D-1): the gateway's runtime performs its own post-guard
// review forcing on Continue; the two Respond-capable controls (Idempotency
// replay / ReviewGate) reconstruct their short-circuit result here so the
// chain decision and the execute path answer identically, mirroring chatexec's
// deriveShortCircuit order (governance.ShortCircuitOrder).
type governedGatewayGuard struct {
	hooks    *picoclawagent.HookManager
	controls []governance.NamedControl
	recorder *governance.AuditRecorder

	replay            *gatewayReplayStore
	requireDraftReview bool
}

// gatewayReplayStore is the nil-replay stand-in (unwired): Lookup never hits.
type gatewayReplayStore struct{}

func (gatewayReplayStore) Lookup(context.Context, string) (*agenttools.ToolResult, bool) { return nil, false }

// newGovernedGatewayGuard assembles the nine controls with the gateway's Deps.
// Deps decisions (per-surface, NOT a copy of chatexec):
//
//   - Policy: DefaultPolicy (shared product policy, same as chat)
//   - Facts: ctx-carried resolver (concurrency-safe for the shared instance)
//   - MeasureResolver: nil — no production measure catalog exists yet (same
//     parity as the pre-convergence wiring; registered open item)
//   - Budget: nil — the gateway is structurally one tool call per HTTP
//     request, not a turn loop; a per-turn budget belongs to the chat/runner
//     turn lifecycle. Per-client rate limiting is the gateway's outer bound
//     (M6.3/usage store), not this control's job.
//   - Replay: gatewayReplayStore (no-op) — no turn-level replay store; the
//     idempotency key requirement STILL applies (IdempotencyGuard is live),
//     replay hits cannot occur with a nil store (documented, same as chat
//     pre-convergence).
//   - RequireDraftReview: true (Assist Mode default, same as chat)
//   - Audit: nil ON THE CHAIN — the runtime layer stays the single owner of
//     tool_execution emission and its failure semantics (chatexec comment,
//     same rule). RT1-D-1 复验修正：把同一个 recorder 同时接进链上
//     AuditRecorder 控制与 runtime 层 WithAudit 会让一次调用写两条审计；且
//     gateway 是共享实例，链上 failures slice 无人 drain（全仓只有 chatexec
//     调），失败永不浮现且 slice 无界增长。路线 A：与 chat 平面一致，链上
//     Audit=nil；gateway 单调用端点下「审计写失败→调用标 StatusFailed」由
//     runtime 层直接承担，与 adjudication (b) 的终点等价，无需 drain 中转。
//   - Sink: nil — no provenance sink (same control-list policy as chat)
//   - Metrics: nil on the CONTROL — the runtime layer owns metric observation
//     (its own Observe in execute); wiring the control would double count.
func newGovernedGatewayGuard() *governedGatewayGuard {
	g := &governedGatewayGuard{
		replay:            &gatewayReplayStore{},
		requireDraftReview: true,
	}
	deps := governance.Deps{
		Policy:             agenttools.DefaultPolicy(),
		Facts:              gatewayFactsResolver{},
		Replay:             g.replay,
		RequireDraftReview: g.requireDraftReview,
		// Audit 留 nil：runtime 层是审计唯一所有者（见上）。
	}
	g.controls, g.recorder = governance.Assembly(deps)
	hooks := picoclawagent.NewHookManager(nil)
	for i, control := range g.controls {
		// Explicit priorities mirror chatexec: equal priorities make
		// HookManager.rebuildOrdered fall back to name ordering, which is NOT
		// the product order (TenantScope must gate first, audit before
		// metrics, ...).
		if err := hooks.Mount(picoclawagent.HookRegistration{
			Name: control.Name, Priority: (i + 1) * 10, Hook: control.Hook,
		}); err != nil {
			panic(fmt.Sprintf("gateway governance: mounting %s: %v", control.Name, err))
		}
	}
	g.hooks = hooks
	return g
}

// kind is the machine-assertion discriminator (RT1-D-1): the gateway runtime
// constructed through this guard reports as a governed runtime; the reverse
// control (legacy agentcore chain) reports differently.
func (g *governedGatewayGuard) kind() string {
	if g == nil || g.hooks == nil {
		return ""
	}
	return fmt.Sprintf("%T", g)
}

// Before implements agenttools.ExecutionGuard: capture the frame into ctx,
// derive any short-circuit, then run the nine controls through the hook
// manager. Rejections keep their explicit code via governance.RejectSink.
func (g *governedGatewayGuard) Before(ctx context.Context, call agenttools.ToolCall, descriptor agenttools.ToolDescriptor, principal agenttools.Principal) (agenttools.GuardResult, error) {
	ctx = withGatewayFacts(ctx, gatewayFacts{descriptor: descriptor, call: call, principal: principal})
	candidate := g.deriveShortCircuit(ctx, descriptor, call)

	sink := &governance.RejectSink{}
	ctx = governance.WithRejectSink(ctx, sink)

	request := &picoclawagent.ToolCallHookRequest{
		Meta:      gatewayMeta(call),
		Tool:      call.ToolName,
		Arguments: argumentsMapForGateway(call.Arguments),
	}
	_, decision := g.hooks.BeforeTool(ctx, request)
	switch decision.Action {
	case "", picoclawagent.HookActionContinue, picoclawagent.HookActionModify:
		return agenttools.GuardResult{}, nil
	case picoclawagent.HookActionRespond:
		// Respond bypasses ApproveTool; only ReviewGate can produce one here
		// (replay store is nil, so IdempotencyGuard never responds). The
		// derivation reconstructs it faithfully from the captured frame.
		if candidate != nil {
			return agenttools.GuardResult{Short: candidate}, nil
		}
		return blockedGateway("tool short-circuit carried no derivable first-party payload"), nil
	default: // deny_tool / abort_turn / hard_abort — reason + code preserved
		return agenttools.GuardResult{Block: true, Reason: decision.Reason, Code: sink.Code()}, nil
	}
}

// After implements the after half of agenttools.ExecutionGuard.
func (g *governedGatewayGuard) After(ctx context.Context, call agenttools.ToolCall, descriptor agenttools.ToolDescriptor, result *agenttools.ToolResult, principal agenttools.Principal) error {
	ctx = withGatewayFacts(ctx, gatewayFacts{descriptor: descriptor, call: call, principal: principal, result: result})
	response := &picoclawagent.ToolResultHookResponse{
		Meta:      gatewayMeta(call),
		Tool:      call.ToolName,
		Arguments: argumentsMapForGateway(call.Arguments),
		Result:    gatewayAfterResult(result),
	}
	_, _ = g.hooks.AfterTool(ctx, response)
	return nil
}

// deriveShortCircuit mirrors chatexec's: walk governance.ShortCircuitOrder so
// the chain and this derivation answer the same order. Only ReviewGate can
// respond (replay store nil), but the order walk stays for parity.
func (g *governedGatewayGuard) deriveShortCircuit(ctx context.Context, descriptor agenttools.ToolDescriptor, call agenttools.ToolCall) *agenttools.ToolResult {
	predicates := map[string]func(context.Context) *agenttools.ToolResult{
		"IdempotencyGuard": func(c context.Context) *agenttools.ToolResult {
			return g.replayShortCircuit(c, call)
		},
		"ReviewGate": func(context.Context) *agenttools.ToolResult {
			return reviewShortCircuitFor(descriptor, call, g.requireDraftReview)
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

func (g *governedGatewayGuard) replayShortCircuit(ctx context.Context, call agenttools.ToolCall) *agenttools.ToolResult {
	if g.replay != nil && strings.TrimSpace(call.IdempotencyKey) != "" {
		if stored, hit := g.replay.Lookup(ctx, call.IdempotencyKey); hit && stored != nil {
			short := *stored
			if strings.TrimSpace(short.CallID) == "" {
				short.CallID = call.CallID
			}
			return &short
		}
	}
	return nil
}

// reviewShortCircuitFor mirrors ReviewGate's Respond branch (shared logic
// extracted so the gateway derivation and the control agree).
func reviewShortCircuitFor(descriptor agenttools.ToolDescriptor, call agenttools.ToolCall, requireDraftReview bool) *agenttools.ToolResult {
	policy := agenttools.Policy{RequireDraftReview: requireDraftReview}
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

func gatewayMeta(call agenttools.ToolCall) picoclawagent.HookMeta {
	return picoclawagent.HookMeta{
		TurnID: call.RunID, SessionKey: call.SkillID, TracePath: call.TraceID,
		Source: "agent-gateway",
	}
}

func argumentsMapForGateway(raw json.RawMessage) map[string]any {
	arguments := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &arguments)
	}
	return arguments
}

func gatewayAfterResult(result *agenttools.ToolResult) *tools.ToolResult {
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

func blockedGateway(reason string) agenttools.GuardResult {
	return agenttools.GuardResult{Block: true, Reason: reason}
}
