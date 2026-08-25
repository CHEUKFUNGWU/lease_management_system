package governance

// RT1-D-2 双路径错误码一致性锚（先落地并绿，再删 agentcore/hooks）。
//
// 删除旧链后全仓只剩两条工具执行路径：已汇流平面（chat/gateway）走本包
// governance.Assembly 九控制，base runtime（in-agent 平面、无 guard 构造）
// 回落 agenttools.Evaluate。本锚用同一 registry 两台真实 Runtime.Execute
// 断言：同一调用在两条路径下产生相同的 Status 与错误码——RC1 的码表契约
// 在合并后不得退化。
//
// ┌─ 适用范围（比「九项等价」窄，刻意写明）────────────────────────────────┐
// │ 1) 本锚只证两边都实际接线的那组控制：TenantScope / CapabilityCheck /      │
// │ IdempotencyGuard(写键要求) / ReviewGate 的判定与码。Evaluate 里没有      │
// │ ProtectedMeasure，也没有 BudgetGuard（policy.go 零命中）——这两项不在     │
// │ 等价声明内。今天没有回归（两条路径的生产 Deps 里它们本来就是 nil），      │
// │ 但若将来某个平面接上 MeasureResolver 或 Budget，「双路径一致」不覆盖      │
// │ 它们——那两项的语义只存在于链上。                                       │
// │ 2) TenantScope 的身份完整性判定两边刻意不同：链要求 user + 法人(或       │
// │ global)，Evaluate 只要求认证上下文（见 TestDualPathKnownAsymmetry…）。   │
// └───────────────────────────────────────────────────────────────────────┘
//
// 错误消息不做跨路径断言：RC1 收敛的是类别化文案，两条路径的映射器
// （publicPolicyError vs publicGuardError）在个别类别措辞不同（如
// unauthenticated），码才是契约面。

import (
	"context"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
	picoclawagent "github.com/lease-management-system/core-service/internal/agentkernel/third_party/picoclaw/agent"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

// rc1ChainGuard adapts a mounted Assembly into agenttools.ExecutionGuard,
// translating decisions exactly as the production adapters do
// (handlers.governedGatewayGuard): Continue passes, Respond carries a
// derived short-circuit, deny/abort keeps the sink-authored code.
type rc1ChainGuard struct {
	manager *picoclawagent.HookManager
}

func (g *rc1ChainGuard) Before(ctx context.Context, call agenttools.ToolCall, descriptor agenttools.ToolDescriptor, principal agenttools.Principal) (agenttools.GuardResult, error) {
	sink := &RejectSink{}
	ctx = WithRejectSink(ctx, sink)
	request := &picoclawagent.ToolCallHookRequest{
		Meta:      picoclawagent.HookMeta{TurnID: call.RunID},
		Tool:      call.ToolName,
		Arguments: map[string]any{},
	}
	_, decision := g.manager.BeforeTool(ctx, request)
	switch decision.Action {
	case "", picoclawagent.HookActionContinue, picoclawagent.HookActionModify:
		return agenttools.GuardResult{}, nil
	case picoclawagent.HookActionRespond:
		return agenttools.GuardResult{Short: deriveRespondShort(descriptor, call)}, nil
	default: // deny_tool / abort_turn / hard_abort
		return agenttools.GuardResult{Block: true, Reason: decision.Reason, Code: sink.Code()}, nil
	}
}

func (g *rc1ChainGuard) After(context.Context, agenttools.ToolCall, agenttools.ToolDescriptor, *agenttools.ToolResult, agenttools.Principal) error {
	return nil // after controls are inert under these Deps (Audit/Metrics/Sink all nil)
}

// deriveRespondShort mirrors the production derivation (chatexec
// deriveShortCircuit / handlers.reviewShortCircuitFor): only ReviewGate can
// Respond under these Deps (replay store nil), and its short-circuit is the
// command-level needs_review.
func deriveRespondShort(descriptor agenttools.ToolDescriptor, call agenttools.ToolCall) *agenttools.ToolResult {
	if !agenttools.RequiresReviewDecision(descriptor, agenttools.Policy{RequireDraftReview: true}) ||
		descriptor.Level != agenttools.LevelCommand {
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

type rc1Scenario struct {
	name string
	// allowCommand opens command level on BOTH sides (mirrors AR5c's liveness harness).
	allowCommand bool
	principal    agenttools.Principal
	withIdentity bool
	call         func(callID string) agenttools.ToolCall

	wantStatus     agenttools.ToolStatus
	wantCode       agenttools.ErrorCode // "" asserts no error
	wantReviewReq  bool
	wantConfirmAct string // non-empty asserts Review.Actions contains it
}

func rc1DualPathRegistry(t *testing.T) *agenttools.Registry {
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
			Name: "probe.read", Version: "v1", Description: "dualpath read probe", Level: agenttools.LevelRead,
			ReadOnly: true, Permissions: []agenttools.Permission{{Resource: "probe", Action: "read"}},
		},
		Handler: func(context.Context, agenttools.ToolCall) (agenttools.ToolResult, error) {
			return agenttools.ToolResult{Status: agenttools.StatusCompleted}, nil
		},
	})
	mustRegister(agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "probe.draft", Version: "v1", Description: "dualpath draft probe", Level: agenttools.LevelDraft,
			Permissions:         []agenttools.Permission{{Resource: "probe", Action: "draft"}},
			SupportsIdempotency: true,
			Review:              agenttools.ReviewPolicy{Required: true, Reasons: []string{"draft review"}, ConfirmAction: "confirm_draft"},
		},
		Handler: func(context.Context, agenttools.ToolCall) (agenttools.ToolResult, error) {
			return agenttools.ToolResult{Status: agenttools.StatusCompleted}, nil
		},
	})
	mustRegister(agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "probe.command", Version: "v1", Description: "dualpath command probe", Level: agenttools.LevelCommand,
			Permissions:         []agenttools.Permission{{Resource: "monthly_closing", Action: "lock"}},
			SupportsIdempotency: true,
			Review:              agenttools.ReviewPolicy{Required: true, Reasons: []string{"lock_period_review"}, ConfirmAction: "confirm_lock"},
		},
		Handler: func(context.Context, agenttools.ToolCall) (agenttools.ToolResult, error) {
			return agenttools.ToolResult{Status: agenttools.StatusCompleted}, nil
		},
	})
	return registry
}

func rc1DualPathScenarios() []rc1Scenario {
	fullPerms := []string{"probe:read", "probe:draft", "monthly_closing:lock"}
	return []rc1Scenario{
		{
			name:         "missing execution context",
			withIdentity: false,
			call: func(id string) agenttools.ToolCall {
				return agenttools.ToolCall{ToolName: "probe.read", ToolVersion: "v1"}
			},
			wantStatus: agenttools.StatusRejected, wantCode: agenttools.ErrorUnauthenticated,
		},
		{
			name:         "command level disabled by default policy",
			withIdentity: true,
			principal:    agenttools.Principal{UserID: "u1", Permissions: fullPerms, Scope: access.Scope{LegalEntityID: "entity-a"}},
			call: func(id string) agenttools.ToolCall {
				return agenttools.ToolCall{ToolName: "probe.command", ToolVersion: "v1", IdempotencyKey: id}
			},
			wantStatus: agenttools.StatusRejected, wantCode: agenttools.ErrorCapabilityDenied,
		},
		{
			name:         "capability narrowing denies non-granted tool",
			withIdentity: true,
			principal: agenttools.Principal{
				UserID: "u1", Permissions: fullPerms, Scope: access.Scope{LegalEntityID: "entity-a"},
				CapabilityActive: true, CapabilityGrants: []string{"probe.read"},
			},
			call: func(id string) agenttools.ToolCall {
				return agenttools.ToolCall{ToolName: "probe.draft", ToolVersion: "v1", IdempotencyKey: id}
			},
			wantStatus: agenttools.StatusRejected, wantCode: agenttools.ErrorCapabilityDenied,
		},
		{
			name:         "permission not granted",
			withIdentity: true,
			principal:    agenttools.Principal{UserID: "u1", Permissions: []string{"other:thing"}, Scope: access.Scope{LegalEntityID: "entity-a"}},
			call: func(id string) agenttools.ToolCall {
				return agenttools.ToolCall{ToolName: "probe.read", ToolVersion: "v1"}
			},
			wantStatus: agenttools.StatusRejected, wantCode: agenttools.ErrorPermissionDenied,
		},
		{
			name:         "write tool without idempotency key",
			withIdentity: true,
			principal:    agenttools.Principal{UserID: "u1", Permissions: fullPerms, Scope: access.Scope{LegalEntityID: "entity-a"}},
			call: func(id string) agenttools.ToolCall {
				return agenttools.ToolCall{ToolName: "probe.draft", ToolVersion: "v1"} // no key
			},
			wantStatus: agenttools.StatusRejected, wantCode: agenttools.ErrorInvalidArguments,
		},
		{
			name:         "dry_run unsupported",
			withIdentity: true,
			principal:    agenttools.Principal{UserID: "u1", Permissions: fullPerms, Scope: access.Scope{LegalEntityID: "entity-a"}},
			call: func(id string) agenttools.ToolCall {
				return agenttools.ToolCall{ToolName: "probe.read", ToolVersion: "v1", DryRun: true}
			},
			wantStatus: agenttools.StatusRejected, wantCode: agenttools.ErrorInvalidArguments,
		},
		{
			name:         "command requiring review responds needs_review",
			allowCommand: true,
			withIdentity: true,
			principal: agenttools.Principal{
				UserID: "u1", Permissions: []string{"monthly_closing:lock"}, Scope: access.Scope{LegalEntityID: "entity-a"},
				CapabilityActive: true, CapabilityGrants: []string{"probe.command"},
			},
			call: func(id string) agenttools.ToolCall {
				return agenttools.ToolCall{ToolName: "probe.command", ToolVersion: "v1", IdempotencyKey: id}
			},
			wantStatus: agenttools.StatusNeedsReview, wantCode: "",
			wantReviewReq: true, wantConfirmAct: "confirm_lock",
		},
		{
			name:         "draft executes then forces needs_review at tail",
			withIdentity: true,
			principal:    agenttools.Principal{UserID: "u1", Permissions: fullPerms, Scope: access.Scope{LegalEntityID: "entity-a"}},
			call: func(id string) agenttools.ToolCall {
				return agenttools.ToolCall{ToolName: "probe.draft", ToolVersion: "v1", IdempotencyKey: id}
			},
			wantStatus: agenttools.StatusNeedsReview, wantCode: "",
			wantReviewReq: true, wantConfirmAct: "confirm_draft",
		},
	}
}

func TestDualPathCodeFidelityChainMatchesEvaluate(t *testing.T) {
	registry := rc1DualPathRegistry(t)

	for _, tc := range rc1DualPathScenarios() {
		t.Run(tc.name, func(t *testing.T) {
			chainResult, bareResult := runBothSides(t, registry, tc)
			if chainResult.Status != bareResult.Status || chainResult.Status != tc.wantStatus {
				t.Fatalf("status diverged: chain=%v(%s) evaluate=%v(%s) want=%v",
					chainResult.Status, chainMessage(chainResult), bareResult.Status, chainMessage(bareResult), tc.wantStatus)
			}
			chainCode, bareCode := resultCode(chainResult), resultCode(bareResult)
			if chainCode != bareCode || chainCode != tc.wantCode {
				t.Fatalf("code diverged: chain=%q evaluate=%q want=%q", chainCode, bareCode, tc.wantCode)
			}
			if chainResult.Review.Required != bareResult.Review.Required || chainResult.Review.Required != tc.wantReviewReq {
				t.Fatalf("review-required diverged: chain=%v evaluate=%v want=%v",
					chainResult.Review.Required, bareResult.Review.Required, tc.wantReviewReq)
			}
			if tc.wantConfirmAct != "" && len(chainResult.Review.Actions) == 0 {
				t.Fatalf("needs_review must carry confirm actions, got %+v", chainResult.Review)
			}
		})
	}
}

// 已知不对称（刻意保留，非本锟缺陷）：链侧身份完整性严于 Evaluate。
// identityComplete 要求 user_id 非空且（Scope.Global 或法人非空）；Evaluate 的
// RequireExecutionContext 只要求认证上下文。AR5d 注释记了原因：admin 账号的
// 法人可为空，仓库层范围过滤才是隔离权威（底线 1）。旧 agentcore 链在这条上
// 行为同 Evaluate（只查认证上下文），所以 RT1-D-2 删除旧链、base 回落 Evaluate
// 在此维度零漂移；不对称存在于新链与 base 之间，自 AR5d 起即如此。
func TestDualPathKnownAsymmetryTenantScopeCompleteness(t *testing.T) {
	registry := agenttools.NewRegistry()
	if err := registry.Register(agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "probe.read", Version: "v1", Description: "asymmetry probe", Level: agenttools.LevelRead,
			ReadOnly: true, Permissions: []agenttools.Permission{{Resource: "probe", Action: "read"}},
		},
		Handler: func(context.Context, agenttools.ToolCall) (agenttools.ToolResult, error) {
			return agenttools.ToolResult{Status: agenttools.StatusCompleted}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	call := agenttools.ToolCall{CallID: "dp-asym", RunID: "run-dp", ToolName: "probe.read", ToolVersion: "v1"}
	principal := agenttools.Principal{UserID: "u1", Permissions: []string{"probe:read"}} // 有用户无法人、非 global

	controls, _ := Assembly(Deps{
		Policy:             agenttools.DefaultPolicy(),
		Facts:              &staticFacts{facts: CallFacts{Call: call, Principal: principal, Descriptor: mustResolve(t, registry, call)}},
		RequireDraftReview: true,
	})
	manager := picoclawagent.NewHookManager(nil)
	for i, control := range controls {
		if err := manager.Mount(picoclawagent.HookRegistration{
			Name: control.Name, Priority: (i + 1) * 10, Hook: control.Hook,
		}); err != nil {
			t.Fatal(err)
		}
	}
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{Principal: principal})

	chain, _ := agenttools.NewRuntime(registry, agenttools.RuntimeOptions{Guard: &rc1ChainGuard{manager: manager}}).Execute(ctx, call)
	bare, _ := agenttools.NewRuntime(registry, agenttools.RuntimeOptions{Policy: agenttools.DefaultPolicy()}).Execute(ctx, call)

	if resultCode(chain) != agenttools.ErrorUnauthenticated {
		t.Fatalf("chain must keep requiring entity-or-global identity (AR5d decision), got %+v", chain)
	}
	if bare.Status != agenttools.StatusCompleted {
		t.Fatalf("evaluate must keep accepting authenticated context without entity (legacy-chain parity), got %+v", bare)
	}
}

func runBothSides(t *testing.T, registry *agenttools.Registry, tc rc1Scenario) (chain, bare agenttools.ToolResult) {
	t.Helper()
	policy := agenttools.DefaultPolicy()
	if tc.allowCommand {
		policy.AllowCommand = true
		policy.AllowedLevels[agenttools.LevelCommand] = true
	}
	callID := "dp-" + tc.name
	call := tc.call(callID)
	call.CallID = callID // Validate requires call_id + run_id before any policy runs
	call.RunID = "run-dp"

	// 链侧：Assembly + HookManager + 生产同构适配器。
	var frame CallFacts
	if tc.withIdentity {
		frame = CallFacts{
			Call: call, Principal: tc.principal,
			Descriptor: mustResolve(t, registry, call),
		}
	} else {
		frame = CallFacts{Call: call, Descriptor: mustResolve(t, registry, call)}
	}
	controls, _ := Assembly(Deps{Policy: policy, Facts: &staticFacts{facts: frame}, RequireDraftReview: true})
	manager := picoclawagent.NewHookManager(nil)
	for i, control := range controls {
		if err := manager.Mount(picoclawagent.HookRegistration{
			Name: control.Name, Priority: (i + 1) * 10, Hook: control.Hook,
		}); err != nil {
			t.Fatal(err)
		}
	}
	chainRuntime := agenttools.NewRuntime(registry, agenttools.RuntimeOptions{Guard: &rc1ChainGuard{manager: manager}})

	// 裸侧：同一 registry、同一 policy，无 guard —— Evaluate 内联策略。
	bareRuntime := agenttools.NewRuntime(registry, agenttools.RuntimeOptions{Policy: policy})

	ctx := context.Background()
	if tc.withIdentity {
		ctx = agenttools.WithExecutionContext(ctx, agenttools.ExecutionContext{Principal: tc.principal})
	}

	chain, chainErr := chainRuntime.Execute(ctx, call)
	bare, bareErr := bareRuntime.Execute(ctx, call)
	if (chainErr != nil) != (bareErr != nil) {
		t.Fatalf("execute-error diverged: chain=%v evaluate=%v", chainErr, bareErr)
	}
	return chain, bare
}

func mustResolve(t *testing.T, registry *agenttools.Registry, call agenttools.ToolCall) agenttools.ToolDescriptor {
	t.Helper()
	definition, found := registry.Resolve(call.ToolName, call.ToolVersion)
	if !found {
		t.Fatalf("tool %s@%s not registered", call.ToolName, call.ToolVersion)
	}
	return definition.Descriptor
}

func chainMessage(result agenttools.ToolResult) string {
	if result.Error == nil {
		return ""
	}
	return result.Error.Message
}

func resultCode(result agenttools.ToolResult) agenttools.ErrorCode {
	if result.Error == nil {
		return ""
	}
	return result.Error.Code
}
