package chatexec

import (
	"context"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agentkernel/governance"
	picoclawagent "github.com/lease-management-system/core-service/internal/agentkernel/third_party/picoclaw/agent"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

func accessScopeRC1() access.Scope { return access.Scope{LegalEntityID: "entity-a"} }

// ── RC1：治理链拒绝的错误码保真 ─────────────────────────────────────────────
//
// Core acceptance: for the SAME rejection scenario, the guarded path (the
// governance chain) and the unguarded path (policy.Evaluate) return the SAME
// error code. The fork went unnoticed for months because no test compared the
// two paths.
//
// Mutation check: reverting runtime.go's Block branch to a blanket
// scope_denied makes every scenario here red.

func rc1World(t *testing.T, opts func(*Deps)) *mutationWorld {
	t.Helper()
	return newMutationWorld(t, opts)
}

// unguarded builds the SAME registry under plain policy evaluation — no guard.
func unguarded(w *mutationWorld) *agenttools.Runtime {
	return agenttools.NewRuntime(w.registry, agenttools.RuntimeOptions{Policy: agenttools.DefaultPolicy()})
}

func registerCommandTool(t *testing.T, w *mutationWorld) {
	t.Helper()
	if err := w.registry.Register(agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "mut.command", Version: "v1", Description: "command probe", Level: agenttools.LevelCommand,
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
}

func assertCode(t *testing.T, label string, result agenttools.ToolResult, want agenttools.ErrorCode) {
	t.Helper()
	if result.Error == nil {
		t.Fatalf("%s: no error on rejected result %+v", label, result)
	}
	if result.Error.Code != want {
		t.Fatalf("%s: code = %q, want %q", label, result.Error.Code, want)
	}
}

// ── 1 能力不足：level 被禁用 → capability_denied（双路径对照） ───────────────

func TestRC1CapabilityLevelDisabledParity(t *testing.T) {
	w := rc1World(t, nil)
	registerCommandTool(t, w)

	guarded, err := w.kernel.Runtime.Execute(fullIdentity(), agenttools.ToolCall{
		CallID: "rc1-cap", RunID: "run-rc1", ToolName: "mut.command", ToolVersion: "v1",
		IdempotencyKey: "k",
	})
	if err != nil {
		t.Fatal(err)
	}
	plain := unguarded(w)
	legacy, err := plain.Execute(fullIdentity(), agenttools.ToolCall{
		CallID: "rc1-cap", RunID: "run-rc1", ToolName: "mut.command", ToolVersion: "v1",
		IdempotencyKey: "k",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertCode(t, "guarded", guarded, agenttools.ErrorCapabilityDenied)
	assertCode(t, "unguarded", legacy, agenttools.ErrorCapabilityDenied)
	// External text converges to the shared wording, not the raw sentinel.
	if guarded.Error.Message != "tool capability is not granted" {
		t.Errorf("guarded message = %q; want converged public wording", guarded.Error.Message)
	}
}

// ── 2 能力不足：capability 未授予 → capability_denied，且绝不是 scope_denied ─

func TestRC1CapabilityNotGrantedIsNotScopeDenied(t *testing.T) {
	w := rc1World(t, nil)

	identity := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID: "user-1", Scope: accessScopeRC1(),
			CapabilityActive: true, // grants deliberately absent
			Permissions:      []string{"mut:read"},
		},
	})
	result, err := w.kernel.Runtime.Execute(identity, agenttools.ToolCall{
		CallID: "rc1-nocap", RunID: "run-rc1", ToolName: "mut.draft", ToolVersion: "v1",
		IdempotencyKey: "k",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertCode(t, "capability not granted", result, agenttools.ErrorCapabilityDenied)
	// The negative half of the acceptance: an ability refusal must NOT wear
	// the tenant-isolation evidence code.
	if result.Error.Code == agenttools.ErrorScopeDenied {
		t.Fatal("capability refusal disguised as scope_denied — bottom-line-1 evidence corrupted")
	}
}

// ── 3 权限不足：permission 未授予 → permission_denied（双路径对照） ──────────

func TestRC1PermissionDeniedParity(t *testing.T) {
	w := rc1World(t, nil)

	identity := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID: "user-1", Scope: accessScopeRC1(),
			// mut:draft deliberately absent
			Permissions: []string{"mut:read"},
		},
	})
	guarded, err := w.kernel.Runtime.Execute(identity, agenttools.ToolCall{
		CallID: "rc1-perm", RunID: "run-rc1", ToolName: "mut.draft", ToolVersion: "v1",
		IdempotencyKey: "k",
	})
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := unguarded(w).Execute(identity, agenttools.ToolCall{
		CallID: "rc1-perm", RunID: "run-rc1", ToolName: "mut.draft", ToolVersion: "v1",
		IdempotencyKey: "k",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertCode(t, "guarded", guarded, agenttools.ErrorPermissionDenied)
	assertCode(t, "unguarded", legacy, agenttools.ErrorPermissionDenied)
}

// ── 4 dry_run 不支持 → invalid_arguments（双路径对照） ──────────────────────

func TestRC1DryRunUnsupportedParity(t *testing.T) {
	w := rc1World(t, nil)

	call := agenttools.ToolCall{CallID: "rc1-dry", RunID: "run-rc1", ToolName: "mut.read", ToolVersion: "v1", DryRun: true}
	guarded, err := w.kernel.Runtime.Execute(fullIdentity(), call)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := unguarded(w).Execute(fullIdentity(), call)
	if err != nil {
		t.Fatal(err)
	}
	assertCode(t, "guarded", guarded, agenttools.ErrorInvalidArguments)
	assertCode(t, "unguarded", legacy, agenttools.ErrorInvalidArguments)
}

// ── 5 写类工具缺幂等键 → invalid_arguments（双路径对照） ────────────────────

func TestRC1MissingIdempotencyKeyParity(t *testing.T) {
	w := rc1World(t, nil)

	call := agenttools.ToolCall{CallID: "rc1-idem", RunID: "run-rc1", ToolName: "mut.draft", ToolVersion: "v1"}
	guarded, err := w.kernel.Runtime.Execute(fullIdentity(), call)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := unguarded(w).Execute(fullIdentity(), call)
	if err != nil {
		t.Fatal(err)
	}
	assertCode(t, "guarded", guarded, agenttools.ErrorInvalidArguments)
	assertCode(t, "unguarded", legacy, agenttools.ErrorInvalidArguments)
}

// ── 6 预算耗尽 → rate_limited 且 Retryable=false ────────────────────────────

func TestRC1BudgetExhaustedRateLimitedNotRetryable(t *testing.T) {
	w := rc1World(t, func(deps *Deps) { deps.MaxToolCalls = 1 })

	first := w.call(fullIdentity(), "mut.read")
	if first.Status != agenttools.StatusCompleted {
		t.Fatalf("first call must fit: %+v", first)
	}
	second, err := w.kernel.Runtime.Execute(fullIdentity(), agenttools.ToolCall{
		CallID: "rc1-budget2", RunID: "run-rc1", ToolName: "mut.read", ToolVersion: "v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertCode(t, "budget exhausted", second, agenttools.ErrorRateLimited)
	// Pinned false BY TEST (adjudication): the per-turn budget cannot recover
	// within the turn, and agentrunner auto-retries retryable errors — true
	// here would burn MaxRetries calls guaranteed to fail again.
	if second.Error.Retryable {
		t.Fatal("budget exhaustion must not be marked retryable")
	}
}

// ── 7 受保护度量落在非认证工具 → business_failure ───────────────────────────

func TestRC1ProtectedMeasureBusinessFailure(t *testing.T) {
	w := rc1World(t, nil)
	// bindTurn deliberately leaves MeasureResolver unwired (no production
	// measure catalog yet), so remount the chain from an Assembly that HAS one
	// — same controls, same order, plus the resolver under test.
	controls, _ := governance.Assembly(governance.Deps{
		Policy:          agenttools.DefaultPolicy(),
		Facts:           w.kernel,
		Budget:          &governance.Budget{MaxToolCalls: 4},
		MeasureResolver: staticMeasureCatalog{"mut.read": []string{"lease_liability"}},
	})
	manager := picoclawagent.NewHookManager(nil)
	for i, control := range controls {
		if err := manager.Mount(picoclawagent.HookRegistration{
			Name: control.Name, Priority: (i + 1) * 10, Hook: control.Hook,
		}); err != nil {
			t.Fatal(err)
		}
	}
	w.kernel.Hooks = manager

	result, err := w.kernel.Runtime.Execute(fullIdentity(), agenttools.ToolCall{
		CallID: "rc1-measure", RunID: "run-rc1", ToolName: "mut.read", ToolVersion: "v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertCode(t, "protected measure reject", result, agenttools.ErrorBusinessFailure)
	if result.Error.Retryable {
		t.Fatal("domain-policy refusal must not be retryable")
	}
	// business_failure keeps its authored guidance instead of a generic line.
	if result.Error.Message == "" || result.Error.Message == "tool governance infrastructure failed" {
		t.Errorf("authored guidance lost: %q", result.Error.Message)
	}
}

// ── 身份不完整 → unauthenticated（双路径对照） ──────────────────────────────

func TestRC1MissingIdentityUnauthenticatedParity(t *testing.T) {
	w := rc1World(t, nil)

	bare := context.Background() // no execution context at all
	guarded, err := w.kernel.Runtime.Execute(bare, agenttools.ToolCall{
		CallID: "rc1-noauth", RunID: "run-rc1", ToolName: "mut.read", ToolVersion: "v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := unguarded(w).Execute(bare, agenttools.ToolCall{
		CallID: "rc1-noauth", RunID: "run-rc1", ToolName: "mut.read", ToolVersion: "v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertCode(t, "guarded", guarded, agenttools.ErrorUnauthenticated)
	assertCode(t, "unguarded", legacy, agenttools.ErrorUnauthenticated)
}

// ── FactsResolver 自身失败 → system_failure（裁决 3：四处统一） ─────────────

type failingResolver struct{}

func (failingResolver) FactsFor(context.Context, *picoclawagent.ToolCallHookRequest) (governance.CallFacts, error) {
	return governance.CallFacts{}, errFactsDown
}

var errFactsDown = errFactsDownError{}

type errFactsDownError struct{}

func (errFactsDownError) Error() string { return "facts resolver down" }

func TestRC1FactsResolverFailureIsSystemFailure(t *testing.T) {
	sink := &governance.RejectSink{}
	ctx := governance.WithRejectSink(context.Background(), sink)
	request := &picoclawagent.ToolCallHookRequest{Tool: "mut.read"}
	_, decision, err := governance.TenantScope{Facts: failingResolver{}}.BeforeTool(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != picoclawagent.HookActionDenyTool {
		t.Fatalf("resolver failure must still fail closed, got %+v", decision)
	}
	if got := sink.Code(); got != agenttools.ErrorSystemFailure {
		t.Fatalf("resolver failure code = %q; want system_failure (the cause is wrapper infrastructure, not identity)", got)
	}
}
