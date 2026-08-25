package handlers

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agentkernel/governance"
	"github.com/lease-management-system/core-service/internal/agenttools"
	picoclawagent "github.com/lease-management-system/core-service/internal/agentkernel/third_party/picoclaw/agent"
)

// RT1-D-1: ACORE-2 九控制的变异在 gateway 装配下重跑。
//
// 与 chatexec 的 mutation_production_test.go 同构，但通过 gateway 的实际执行
// 缝（agenttools.Runtime.Execute + governedGatewayGuard）——不是 chat 平面的
// 结论复用，是在真正上了这条路径的链上重新成立。

// gwMutationWorld builds a runtime guarded by the NEW chain, driven through
// Runtime.Execute (the seam the gateway's /agent/tools/execute crosses).
type gwMutationWorld struct {
	runtime  *agenttools.Runtime
	guard    *governedGatewayGuard
	registry *agenttools.Registry
}

func newGwMutationWorld(t *testing.T, audit agenttools.AuditRecorder) *gwMutationWorld {
	t.Helper()
	registry := agenttools.NewRegistry()
	mustRegister := func(def agenttools.ToolDefinition) {
		t.Helper()
		if err := registry.Register(def); err != nil {
			t.Fatal(err)
		}
	}
	mustRegister(agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "s1.read", Version: "v1", Description: "gateway mutation probe", Level: agenttools.LevelRead,
			ReadOnly: true, Permissions: []agenttools.Permission{{Resource: "s1", Action: "read"}},
		},
		Handler: func(context.Context, agenttools.ToolCall) (agenttools.ToolResult, error) {
			return agenttools.ToolResult{Status: agenttools.StatusCompleted}, nil
		},
	})
	mustRegister(agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "s1.draft", Version: "v1", Description: "gateway write probe", Level: agenttools.LevelDraft,
			Permissions:         []agenttools.Permission{{Resource: "s1", Action: "draft"}},
			SupportsIdempotency: true,
			Review:              agenttools.ReviewPolicy{Required: true, Reasons: []string{"draft review"}, ConfirmAction: "confirm"},
		},
		Handler: func(context.Context, agenttools.ToolCall) (agenttools.ToolResult, error) {
			return agenttools.ToolResult{Status: agenttools.StatusCompleted}, nil
		},
	})
	guard := newGovernedGatewayGuard()
	base := agenttools.NewRuntime(registry, agenttools.RuntimeOptions{})
	runtime := base.WithGuard(guard)
	if audit != nil {
		// RT1-D-1 复验：审计只在 runtime 层（与真实 gateway 路径的
		// runtimeForRequest().WithAudit 一致），链上 AuditRecorder 控制留 nil。
		runtime = runtime.WithAudit(audit)
	}
	return &gwMutationWorld{runtime: runtime, guard: guard, registry: registry}
}

func gwCall(w *gwMutationWorld, ctx context.Context, name string, call agenttools.ToolCall) agenttools.ToolResult {
	call.CallID = "gw-" + name
	call.RunID = "run-gw"
	call.ToolName = name
	call.ToolVersion = "v1"
	result, err := w.runtime.Execute(ctx, call)
	if err != nil {
		return agenttools.ToolResult{CallID: call.CallID, Status: agenttools.StatusFailed,
			Error: &agenttools.ToolError{Code: "test", Message: err.Error()}}
	}
	return result
}

func gwIdentity() context.Context {
	return agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID: "user-1", SubjectType: "agent_gateway",
			Scope:       access.Scope{LegalEntityID: "entity-a"},
			Permissions: []string{"s1:read", "s1:draft"},
		},
	})
}

func gwNoIdentity() context.Context {
	return agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "user-1", Permissions: []string{"s1:read", "s1:draft"}},
	})
}

func (w *gwMutationWorld) unmount(t *testing.T, name string) {
	t.Helper()
	w.guard.hooks.Unmount(name)
}

func (w *gwMutationWorld) remount(t *testing.T, name string) {
	t.Helper()
	for i, control := range w.guard.controls {
		if control.Name == name {
			if err := w.guard.hooks.Mount(picoclawagent.HookRegistration{
				Name: control.Name, Priority: (i + 1) * 10, Hook: control.Hook,
			}); err != nil {
				t.Fatalf("remount %s: %v", name, err)
			}
			return
		}
	}
	t.Fatalf("control %q not found in gateway assembly", name)
}

func gwRejectedWith(result agenttools.ToolResult, code agenttools.ErrorCode) bool {
	return result.Status == agenttools.StatusRejected &&
		result.Error != nil && result.Error.Code == code
}

// ── 1 TenantScope ────────────────────────────────────────────────────────────

func TestGatewayGovernanceMutationTenantScope(t *testing.T) {
	w := newGwMutationWorld(t, nil)

	if got := gwCall(w, gwNoIdentity(), "s1.read", agenttools.ToolCall{}); !gwRejectedWith(got, agenttools.ErrorUnauthenticated) {
		t.Fatalf("full assembly must reject incomplete identity, got %+v", got)
	}
	w.unmount(t, "TenantScope")
	if got := gwCall(w, gwNoIdentity(), "s1.read", agenttools.ToolCall{}); got.Status != agenttools.StatusCompleted {
		t.Fatalf("removing TenantScope must let the incomplete-identity call through, got %+v", got)
	}
	w.remount(t, "TenantScope")
	if got := gwCall(w, gwNoIdentity(), "s1.read", agenttools.ToolCall{}); !gwRejectedWith(got, agenttools.ErrorUnauthenticated) {
		t.Fatalf("restored assembly must reject again, got %+v", got)
	}
}

// ── 2 CapabilityCheck ────────────────────────────────────────────────────────

func TestGatewayGovernanceMutationCapabilityCheck(t *testing.T) {
	w := newGwMutationWorld(t, nil)
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID: "user-1", Scope: access.Scope{LegalEntityID: "entity-a"},
			Permissions: []string{"other:thing"}, // s1:* absent
		},
	})

	if got := gwCall(w, ctx, "s1.read", agenttools.ToolCall{}); !gwRejectedWith(got, agenttools.ErrorPermissionDenied) {
		t.Fatalf("full assembly must reject a call lacking the tool permission, got %+v", got)
	}
	w.unmount(t, "CapabilityCheck")
	if got := gwCall(w, ctx, "s1.read", agenttools.ToolCall{}); got.Status != agenttools.StatusCompleted {
		t.Fatalf("removing CapabilityCheck must let the unpermitted call through, got %+v", got)
	}
	w.remount(t, "CapabilityCheck")
	if got := gwCall(w, ctx, "s1.read", agenttools.ToolCall{}); !gwRejectedWith(got, agenttools.ErrorPermissionDenied) {
		t.Fatalf("restored assembly must reject again, got %+v", got)
	}
}

// ── 3 ProtectedMeasure（生产 parity：nil resolver 惰性）──────────────────────

func TestGatewayGovernanceParityProtectedMeasureDormant(t *testing.T) {
	w := newGwMutationWorld(t, nil)
	if got := gwCall(w, gwIdentity(), "s1.read", agenttools.ToolCall{}); got.Status != agenttools.StatusCompleted {
		t.Fatalf("with production deps the measure control must stay inert, got %+v", got)
	}
}

// ── 4 BudgetGuard（gateway Deps 决定：nil——单调用端点无「轮」可计数）───────────

func TestGatewayGovernanceBudgetGuardDisabledForSingleCallEndpoint(t *testing.T) {
	w := newGwMutationWorld(t, nil)
	// 无轮预算：反复调用永不 rate_limited（网关一次请求一次 Execute，
	// 外层限流由 M6.3/usage store 承担）。
	for i := 0; i < 3; i++ {
		if got := gwCall(w, gwIdentity(), "s1.read", agenttools.ToolCall{}); got.Status != agenttools.StatusCompleted {
			t.Fatalf("single-call endpoint must never rate_limited on a turn budget, got %+v", got)
		}
	}
}

// ── 5 IdempotencyGuard（key 要求 live；replay store nil 故无重放）────────────

func TestGatewayGovernanceMutationIdempotencyGuard(t *testing.T) {
	w := newGwMutationWorld(t, nil)
	draftCall := func() agenttools.ToolResult {
		return gwCall(w, gwIdentity(), "s1.draft", agenttools.ToolCall{})
	}

	if got := draftCall(); !gwRejectedWith(got, agenttools.ErrorInvalidArguments) {
		t.Fatalf("full assembly must require idempotency key for write tools, got %+v", got)
	}
	w.unmount(t, "IdempotencyGuard")
	got := draftCall()
	if got.Status == agenttools.StatusRejected && strings.Contains(got.Error.Message, "idempotency_key") {
		t.Fatalf("removing IdempotencyGuard must drop the key requirement, got %+v", got)
	}
	if got.Status != agenttools.StatusNeedsReview && got.Status != agenttools.StatusCompleted {
		t.Fatalf("unmounted flow must reach execution/review forcing, got %+v", got)
	}
	w.remount(t, "IdempotencyGuard")
	if got := draftCall(); !gwRejectedWith(got, agenttools.ErrorInvalidArguments) {
		t.Fatalf("restored assembly must reject again, got %+v", got)
	}
}

// ── 6 ReviewGate ───────────────────────────────────────────────────────
// RT1-D-1 复验退回补：生产策略（DefaultPolicy）下 command 级被禁，ReviewGate
// 排在 CapabilityCheck 之后作纵深防御——与 chat 平面同构。需要两条测试：
//
//   - ORDER（经 Runtime.Execute）：command 工具在生产策略下被 CapabilityCheck
//     的级别门拦下（ErrorCapabilityDenied）——证明 CapabilityCheck 先答，
//     ReviewGate 在 gateway 生产路径下轮不到。
//   - LIVENESS（直接驱动 HookManager）：在放开 command 的变体装配下，门确实
//     Respond needs_review；卸掉门链上不再有 Respond。必须绕开 Runtime.Execute
//     ——runtime 尾部的 review 事后强制会在门被删光的情况下照样给出
//     needs_review（复验实测：mounted 与 unmounted 结果一字不差），经 Execute
//     断言证不了第 6 项。

func TestGatewayGovernanceOrderCommandToolsGatedByCapabilityCheck(t *testing.T) {
	w := newGwMutationWorld(t, nil)

	commandIdentity := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID: "user-1", SubjectType: "agent_gateway",
			Scope:       access.Scope{LegalEntityID: "entity-a"},
			Permissions: []string{"monthly_closing:lock"},
		},
	})
	if err := w.registry.Register(agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "s1.command", Version: "v1", Description: "gateway command probe", Level: agenttools.LevelCommand,
			Permissions:         []agenttools.Permission{{Resource: "monthly_closing", Action: "lock"}},
			SupportsIdempotency: true,
			Review:              agenttools.ReviewPolicy{Required: true, Reasons: []string{"lock_period_review"}, ConfirmAction: "confirm_lock"},
		},
		Handler: func(context.Context, agenttools.ToolCall) (agenttools.ToolResult, error) {
			return agenttools.ToolResult{Status: agenttools.StatusCompleted}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	result, err := w.runtime.Execute(commandIdentity, agenttools.ToolCall{
		CallID: "gw-cmd", RunID: "run-gw", ToolName: "s1.command", ToolVersion: "v1", IdempotencyKey: "k",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !gwRejectedWith(result, agenttools.ErrorCapabilityDenied) {
		t.Fatalf("under production policy CapabilityCheck must deny command level before any later control answers, got %+v", result)
	}
}

func TestGatewayGovernanceLoadedReviewGateRespondsNeedsReview(t *testing.T) {
	w := newGwMutationWorld(t, nil)
	commandDescriptor := agenttools.ToolDescriptor{
		Name: "lease.month.close.lock", Version: "v1", Description: "lock", Level: agenttools.LevelCommand,
		Permissions:         []agenttools.Permission{{Resource: "monthly_closing", Action: "lock"}},
		SupportsIdempotency: true,
		Review:              agenttools.ReviewPolicy{Required: true, Reasons: []string{"lock_period_review"}, ConfirmAction: "confirm_lock"},
	}
	policy := agenttools.DefaultPolicy()
	policy.AllowCommand = true
	policy.AllowedLevels[agenttools.LevelCommand] = true

	// 变体装配：挂载路径与优先级同 guard 构造（(i+1)*10），Deps 取 gateway
	// 生产值（Facts=ctx resolver、Replay=no-op、RequireDraftReview=true、
	// Budget/Audit/Metrics/Sink=nil），仅放开 command 级——证明 Respond 能力
	// 属于 ReviewGate 本身，且走的是 gateway 自己的挂点。
	controls, _ := governance.Assembly(governance.Deps{
		Policy: policy, Facts: gatewayFactsResolver{},
		Replay: &gatewayReplayStore{}, RequireDraftReview: true,
	})
	mountChain := func(skip string) *picoclawagent.HookManager {
		t.Helper()
		manager := picoclawagent.NewHookManager(nil)
		for i, control := range controls {
			if control.Name == skip {
				continue
			}
			if err := manager.Mount(picoclawagent.HookRegistration{
				Name: control.Name, Priority: (i + 1) * 10, Hook: control.Hook,
			}); err != nil {
				t.Fatalf("mount %s: %v", control.Name, err)
			}
		}
		return manager
	}

	principal := agenttools.Principal{
		UserID: "user-1", SubjectType: "agent_gateway",
		Scope:            access.Scope{LegalEntityID: "entity-a"},
		CapabilityActive: true,
		CapabilityGrants: []string{"lease.month.close.lock"},
		Permissions:      []string{"monthly_closing:lock"},
	}
	call := agenttools.ToolCall{
		CallID: "gw-gate", RunID: "run-gw", ToolName: "lease.month.close.lock",
		ToolVersion: "v1", IdempotencyKey: "lock-k1",
	}
	runScenario := func(mgr *picoclawagent.HookManager) picoclawagent.HookDecision {
		ctx := withGatewayFacts(governance.WithRejectSink(context.Background(), &governance.RejectSink{}),
			gatewayFacts{descriptor: commandDescriptor, call: call, principal: principal})
		request := &picoclawagent.ToolCallHookRequest{
			Meta: gatewayMeta(call), Tool: call.ToolName, Arguments: map[string]any{},
		}
		_, decision := mgr.BeforeTool(ctx, request)
		return decision
	}

	// 门在：链产生 needs_review Respond。
	if decision := runScenario(mountChain("")); decision.Action != picoclawagent.HookActionRespond {
		t.Fatalf("mounted gate must author the chain Respond (needs_review), got %+v", decision)
	}
	// 派生与链同答（footer order parity）：guard 的 deriveShortCircuit 重建出
	// 带确认动作的 short-circuit。
	short := w.guard.deriveShortCircuit(
		withGatewayFacts(context.Background(), gatewayFacts{descriptor: commandDescriptor, call: call, principal: principal}),
		commandDescriptor, call)
	if short == nil || short.Status != agenttools.StatusNeedsReview || len(short.Review.Actions) == 0 {
		t.Fatalf("derivation must reconstruct the gate-authored short-circuit with confirm actions, got %+v", short)
	}
	// 卸掉门：链不再产生任何 Respond。（runtime 层的事后 review 强制仍会拦住
	// 需评审的 command——纵深防御——但门亲笔的带 Actions 的 short-circuit 只在
	// 门挂载时存在。）
	if decision := runScenario(mountChain("ReviewGate")); decision.Action == picoclawagent.HookActionRespond {
		t.Fatalf("unmounting ReviewGate must remove the chain-authored Respond, got %+v", decision)
	}
}

// ── 9 MetricsRecorder（生产 parity：控制不接，runtime 层唯一计数）──────────────

func TestGatewayGovernanceParityMetricsObservedExactlyOnce(t *testing.T) {
	w := newGwMutationWorld(t, nil)
	before := w.runtime.Metrics().Snapshot(time.Now()).TotalExecutions
	if got := gwCall(w, gwIdentity(), "s1.read", agenttools.ToolCall{}); got.Status != agenttools.StatusCompleted {
		t.Fatalf("call must complete, got %+v", got)
	}
	after := w.runtime.Metrics().Snapshot(time.Now()).TotalExecutions
	if delta := after - before; delta != 1 {
		t.Fatalf("TotalExecutions advanced by %d, want exactly 1 (runtime-owned observation, no chain double-count)", delta)
	}
}

// ── 7 AuditRecorder（RT1-D-1 复验：链上留 nil，runtime 层唯一所有者）──────────
// 双写防回归：一次工具调用必须恰好产生一条审计。runtime 层 WithAudit 与链上
// AuditRecorder 控制各写一条就是两条——全量单测当时是绿的，因为没有任何测试
// 数过条数。若有人把 recorder 重新接回 Deps.Audit，本测试必须变红。

type countingAudit struct {
	mu      sync.Mutex
	records []agenttools.ToolExecutionAudit
}

func (c *countingAudit) RecordToolExecution(_ context.Context, record agenttools.ToolExecutionAudit) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = append(c.records, record)
	return nil
}

func (c *countingAudit) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.records)
}

func TestGatewayGovernanceParityAuditExactlyOnePerCall(t *testing.T) {
	counter := &countingAudit{}
	w := newGwMutationWorld(t, counter)

	if got := gwCall(w, gwIdentity(), "s1.read", agenttools.ToolCall{}); got.Status != agenttools.StatusCompleted {
		t.Fatalf("call must complete, got %+v", got)
	}
	if n := counter.count(); n != 1 {
		t.Fatalf("one tool call must produce exactly one audit record, got %d — the chain-level AuditRecorder control is wired in addition to the runtime layer (double-write)", n)
	}

	if got := gwCall(w, gwIdentity(), "s1.read", agenttools.ToolCall{}); got.Status != agenttools.StatusCompleted {
		t.Fatalf("second call must complete, got %+v", got)
	}
	if n := counter.count(); n != 2 {
		t.Fatalf("two tool calls must produce exactly two audit records (per-call emission), got %d", n)
	}
}

// ── 机器断言 + 反向对照（照 AR5-G1 / RT1-B SessionOwnerKind 模式）────────────
// AgentGovernedToolRuntime 的 guard 是新链适配器；legacy AgentToolRuntime
// 的 guard 是旧链（agentcorehooks.ExecutionGuard）或空（inline policy）。
// 判别器必须能区分，否则断言恒真。

// AgentGovernedToolRuntime 的 guard 是新链适配器；同一 registry 上不换 guard
// 的 legacy runtime 报告空串。走过真实生产方法（handler 持有 toolRuntime），
// 判别器必须能区分，否则断言恒真（反向对照）。
func TestAgentGovernedToolRuntimeCrossesNewChain(t *testing.T) {
	registry := agenttools.NewRegistry()
	if err := registry.Register(agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "s1.read", Version: "v1", Description: "machine discriminator probe", Level: agenttools.LevelRead,
			ReadOnly: true, Permissions: []agenttools.Permission{{Resource: "s1", Action: "read"}},
		},
		Handler: func(context.Context, agenttools.ToolCall) (agenttools.ToolResult, error) {
			return agenttools.ToolResult{Status: agenttools.StatusCompleted}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	base := agenttools.NewRuntime(registry, agenttools.RuntimeOptions{})
	handler := &AIChatHandler{toolRuntime: base}

	governed := handler.AgentGovernedToolRuntime()
	if governed == nil {
		t.Fatal("AgentGovernedToolRuntime returned nil")
	}
	rt, ok := governed.(*agenttools.Runtime)
	if !ok {
		t.Fatalf("governed runtime type = %T; want *agenttools.Runtime", governed)
	}
	const newGuard = "*handlers.governedGatewayGuard"
	if got := rt.GuardKind(); got != newGuard {
		t.Fatalf("governed runtime guard = %q; want %q — the RT1-D-1 gateway wiring was reverted or bypassed", got, newGuard)
	}
	// 反向对照：同一 base 不换 guard → 不报告新适配器（legacy path）。
	if got := handler.AgentToolRuntime().(*agenttools.Runtime).GuardKind(); got == newGuard {
		t.Fatalf("legacy runtime reports the new guard — discriminator cannot distinguish old from new")
	}
}

// ── parity：capability 收窄（RT1-D-1 用户要求专门测试）───────────────────────
// gateway 的 capability token 把 Principal 收窄：CapabilityActive=true +
// CapabilityGrants 限定工具集合。新链 CapabilityCheck 的 HasCapability 判定
// 必须：未授予的工具 → ErrorCapabilityDenied；授予的 → 放行。

func TestGatewayGovernanceCapabilityNarrowing(t *testing.T) {
	w := newGwMutationWorld(t, nil)

	// Capability 授予了 s1.read，但 principal 的权限集完整（模拟真实收窄：
	// 权限与工具集都收窄到 token 名单）。
	granted := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID: "user-1", SubjectType: "agent_gateway",
			Scope:            access.Scope{LegalEntityID: "entity-a"},
			Permissions:      []string{"s1:read", "s1:draft"},
			CapabilityActive: true,
			CapabilityGrants: []string{"s1.read"},
		},
	})

	// 授予的工具：放行。
	if got := gwCall(w, granted, "s1.read", agenttools.ToolCall{}); got.Status != agenttools.StatusCompleted {
		t.Fatalf("capability-granted tool must pass, got %+v", got)
	}
	// 未授予的 write 工具：ErrorCapabilityDenied（即使 principal 有 s1:draft 权限）
	got := gwCall(w, granted, "s1.draft", agenttools.ToolCall{})
	if !gwRejectedWith(got, agenttools.ErrorCapabilityDenied) {
		t.Fatalf("capability narrowing must deny a non-granted tool, got %+v", got)
	}

	// 反向：CapabilityActive=false（无 capability token）→ 权限面判定放行。
	noCap := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID: "user-1", Scope: access.Scope{LegalEntityID: "entity-a"},
			Permissions: []string{"s1:read", "s1:draft"},
		},
	})
	if got := gwCall(w, noCap, "s1.draft", agenttools.ToolCall{IdempotencyKey: "k"}); gwRejectedWith(got, agenttools.ErrorCapabilityDenied) {
		t.Fatalf("without capability active the permission path must govern, got %+v", got)
	}
}
