package hooks

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/agentcore"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

// mutation harness: build the before chain omitting one hook, then run a
// scenario that only that hook guards. ACORE-2: every omission must be
// detectable by at least one failing case.

func beforeChain(d Deps, omit string) agentcore.BeforeToolCall {
	var hooks []agentcore.BeforeToolCall
	if omit != "TenantScope" {
		hooks = append(hooks, TenantScope())
	}
	if omit != "CapabilityCheck" {
		hooks = append(hooks, CapabilityCheck(d.Policy))
	}
	if omit != "ProtectedMeasure" {
		hooks = append(hooks, ProtectedMeasure(d.MeasureResolver))
	}
	if omit != "BudgetGuard" {
		hooks = append(hooks, BudgetGuard(d.Budget))
	}
	if omit != "IdempotencyGuard" {
		hooks = append(hooks, IdempotencyGuard(d.Replay))
	}
	if omit != "ReviewGate" {
		hooks = append(hooks, ReviewGate(d.RequireDraftReview))
	}
	return agentcore.ChainBefore(hooks...)
}

func readCall() agenttools.ToolCall {
	return agenttools.ToolCall{
		CallID: "call-1", ToolName: "lease.contract.get", ToolVersion: "v1",
		Arguments: json.RawMessage(`{}`),
	}
}

func readDescriptor() agenttools.ToolDescriptor {
	return agenttools.ToolDescriptor{Name: "lease.contract.get", Version: "v1", Level: agenttools.LevelRead, ReadOnly: true}
}

func withCtx(principal agenttools.Principal) context.Context {
	return agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: principal,
		RunID:     "run-1",
	})
}

// mkBC builds a BeforeContext carrying the principal the hooks inspect.
func mkBC(call agenttools.ToolCall, desc agenttools.ToolDescriptor, principal agenttools.Principal) agentcore.BeforeContext {
	return agentcore.BeforeContext{Call: call, Descriptor: desc, Principal: principal}
}

func TestMutationTenantScope(t *testing.T) {
	// Missing execution context: only TenantScope rejects it.
	bc := agentcore.BeforeContext{Call: readCall(), Descriptor: readDescriptor()}
	if _, err := beforeChain(Deps{Policy: agenttools.DefaultPolicy()}, "")(context.Background(), bc); err == nil {
		t.Fatal("full chain must reject a call without execution context")
	}
	if _, err := beforeChain(Deps{Policy: agenttools.DefaultPolicy()}, "TenantScope")(context.Background(), bc); err != nil {
		t.Fatalf("removing TenantScope must let the call through, got %v", err)
	}
}

func TestMutationCapabilityCheck(t *testing.T) {
	policy := agenttools.DefaultPolicy()
	delete(policy.AllowedLevels, agenttools.LevelDraft)
	draftCall := agenttools.ToolCall{
		CallID: "c2", ToolName: "lease.contract.draft.create", ToolVersion: "v1",
		Arguments: json.RawMessage(`{}`), IdempotencyKey: "k1",
	}
	draftDesc := agenttools.ToolDescriptor{Name: "lease.contract.draft.create", Version: "v1", Level: agenttools.LevelDraft}
	principal := agenttools.Principal{UserID: "u1"}
	ctx := withCtx(principal)
	bc := mkBC(draftCall, draftDesc, principal)
	if _, err := beforeChain(Deps{Policy: policy}, "")(ctx, bc); !errors.Is(err, agenttools.ErrToolCapabilityRequired) {
		t.Fatalf("full chain must reject disabled level, got %v", err)
	}
	if _, err := beforeChain(Deps{Policy: policy}, "CapabilityCheck")(ctx, bc); err != nil {
		t.Fatalf("removing CapabilityCheck must let the call through, got %v", err)
	}
}

func TestMutationProtectedMeasure(t *testing.T) {
	resolver := staticResolver{measures: map[string][]string{"lease.predeal.simulate": {"lease_liability"}}}
	call := agenttools.ToolCall{CallID: "c3", ToolName: "lease.predeal.simulate", ToolVersion: "v1", Arguments: json.RawMessage(`{}`)}
	desc := agenttools.ToolDescriptor{Name: "lease.predeal.simulate", Version: "v1", Level: agenttools.LevelRead, ReadOnly: true}
	principal := agenttools.Principal{UserID: "u1"}
	ctx := withCtx(principal)
	bc := mkBC(call, desc, principal)
	deps := Deps{Policy: agenttools.DefaultPolicy(), MeasureResolver: resolver}
	if res, err := beforeChain(deps, "")(ctx, bc); err != nil || !res.Block {
		t.Fatalf("full chain must block protected measures on non-certified tools, got %+v %v", res, err)
	}
	if res, err := beforeChain(deps, "ProtectedMeasure")(ctx, bc); err != nil || res.Block {
		t.Fatalf("removing ProtectedMeasure must let the call through, got %+v %v", res, err)
	}
}

func TestMutationBudgetGuard(t *testing.T) {
	principal := agenttools.Principal{UserID: "u1"}
	ctx := withCtx(principal)
	bc := mkBC(readCall(), readDescriptor(), principal)
	full := Deps{Policy: agenttools.DefaultPolicy(), Budget: &Budget{MaxToolCalls: 1}}
	// First call consumes the single slot, the second must block.
	if res, err := beforeChain(full, "")(ctx, bc); err != nil || res.Block {
		t.Fatalf("first call must fit the budget, got %+v %v", res, err)
	}
	if res, err := beforeChain(full, "")(ctx, bc); err != nil || !res.Block {
		t.Fatalf("exhausted budget must block, got %+v %v", res, err)
	}
	relaxed := Deps{Policy: agenttools.DefaultPolicy(), Budget: &Budget{MaxToolCalls: 1}}
	if res, err := beforeChain(relaxed, "BudgetGuard")(ctx, bc); err != nil || res.Block {
		t.Fatalf("removing BudgetGuard must let the call through, got %+v %v", res, err)
	}
}

func TestMutationIdempotencyGuard(t *testing.T) {
	principal := agenttools.Principal{UserID: "u1"}
	ctx := withCtx(principal)
	draftCall := agenttools.ToolCall{CallID: "c4", ToolName: "lease.contract.draft.create", ToolVersion: "v1", Arguments: json.RawMessage(`{}`)}
	draftDesc := agenttools.ToolDescriptor{Name: "lease.contract.draft.create", Version: "v1", Level: agenttools.LevelDraft}
	bc := mkBC(draftCall, draftDesc, principal)
	deps := Deps{Policy: agenttools.DefaultPolicy()}
	if _, err := beforeChain(deps, "")(ctx, bc); !errors.Is(err, agenttools.ErrInvalidToolCall) {
		t.Fatalf("full chain must require an idempotency key for writes, got %v", err)
	}
	if _, err := beforeChain(deps, "IdempotencyGuard")(ctx, bc); err != nil {
		t.Fatalf("removing IdempotencyGuard must let the call through, got %v", err)
	}

	// Replay short-circuit: the stored result replaces execution.
	stored := &agenttools.ToolResult{CallID: "c4", Status: agenttools.StatusCompleted, Data: "replayed"}
	replayCall := draftCall
	replayCall.IdempotencyKey = "replay-1"
	replayBC := mkBC(replayCall, draftDesc, principal)
	replayDeps := Deps{Policy: agenttools.DefaultPolicy(), Replay: staticReplay{key: "replay-1", result: stored}}
	if res, err := beforeChain(replayDeps, "")(ctx, replayBC); err != nil || res.Short == nil || res.Short.Data != "replayed" {
		t.Fatalf("replay hit must short-circuit, got %+v %v", res, err)
	}
	if res, err := beforeChain(replayDeps, "IdempotencyGuard")(ctx, replayBC); err != nil || res.Short != nil {
		t.Fatalf("removing IdempotencyGuard must skip the replay, got %+v %v", res, err)
	}
}

func TestMutationReviewGate(t *testing.T) {
	policy := agenttools.DefaultPolicy()
	policy.AllowCommand = true
	policy.AllowedLevels[agenttools.LevelCommand] = true
	executed := false
	_ = executed
	cmdCall := agenttools.ToolCall{CallID: "c5", ToolName: "lease.month.close.lock", ToolVersion: "v1", Arguments: json.RawMessage(`{}`), IdempotencyKey: "k"}
	cmdDesc := agenttools.ToolDescriptor{
		Name: "lease.month.close.lock", Version: "v1", Level: agenttools.LevelCommand,
		Review: agenttools.ReviewPolicy{Required: true, Reasons: []string{"lock_period_review"}, ConfirmAction: "confirm_lock"},
	}
	principal := agenttools.Principal{UserID: "u1", CapabilityActive: true, CapabilityGrants: []string{"lease.month.close.lock"}}
	ctx := withCtx(principal)
	bc := mkBC(cmdCall, cmdDesc, principal)
	deps := Deps{Policy: policy}
	if res, err := beforeChain(deps, "")(ctx, bc); err != nil || res.Short == nil || res.Short.Status != agenttools.StatusNeedsReview {
		t.Fatalf("command with review required must short-circuit to needs_review, got %+v %v", res, err)
	}
	if res, err := beforeChain(deps, "ReviewGate")(ctx, bc); err != nil || res.Short != nil {
		t.Fatalf("removing ReviewGate must let the command execute, got %+v %v", res, err)
	}
}

func TestMutationAfterHooks(t *testing.T) {
	ac := agentcore.AfterContext{
		Call:       readCall(),
		Descriptor: readDescriptor(),
		Result:     &agenttools.ToolResult{CallID: "call-1", Status: agenttools.StatusCompleted},
		StartedAt:  time.Now().Add(-time.Second), CompletedAt: time.Now(),
	}

	auditErr := errors.New("audit down")
	recorder := agenttools.AuditRecorderFunc(func(ctx context.Context, audit agenttools.ToolExecutionAudit) error { return auditErr })

	// AuditRecorder: its failure must surface through the chain.
	if _, err := afterChain(Deps{Audit: recorder}, "")(context.Background(), ac); !errors.Is(err, auditErr) {
		t.Fatalf("audit failure must propagate, got %v", err)
	}
	if _, err := afterChain(Deps{Audit: recorder}, "AuditRecorder")(context.Background(), ac); err != nil {
		t.Fatalf("removing AuditRecorder must drop the failure, got %v", err)
	}

	// ArtifactCollector: completed calls reach the sink.
	sink := &captureSink{}
	if _, err := afterChain(Deps{Sink: sink}, "")(context.Background(), ac); err != nil {
		t.Fatal(err)
	}
	if len(sink.calls) != 1 || sink.calls[0].CallID != "call-1" {
		t.Fatalf("sink must receive the certified call, got %+v", sink.calls)
	}
	sink.calls = nil
	if _, err := afterChain(Deps{Sink: sink}, "ArtifactCollector")(context.Background(), ac); err != nil {
		t.Fatal(err)
	}
	if len(sink.calls) != 0 {
		t.Fatalf("removing ArtifactCollector must stop collection, got %+v", sink.calls)
	}

	// MetricsRecorder: the shared metrics sink observes the call.
	metrics := agenttools.NewRuntimeMetrics()
	if _, err := afterChain(Deps{Metrics: metrics}, "")(context.Background(), ac); err != nil {
		t.Fatal(err)
	}
	if snap := metrics.Snapshot(time.Now()); snap.TotalExecutions != 1 {
		t.Fatalf("metrics must observe the call, got %d", snap.TotalExecutions)
	}
	metrics2 := agenttools.NewRuntimeMetrics()
	if _, err := afterChain(Deps{Metrics: metrics2}, "MetricsRecorder")(context.Background(), ac); err != nil {
		t.Fatal(err)
	}
	if snap := metrics2.Snapshot(time.Now()); snap.TotalExecutions != 0 {
		t.Fatalf("removing MetricsRecorder must stop observation, got %d", snap.TotalExecutions)
	}
}

// afterChain builds the after chain omitting one hook.
func afterChain(d Deps, omit string) agentcore.AfterToolCall {
	var hooks []agentcore.AfterToolCall
	if omit != "AuditRecorder" {
		hooks = append(hooks, AuditRecorder(d.Audit))
	}
	if omit != "ArtifactCollector" {
		hooks = append(hooks, ArtifactCollector(d.Sink))
	}
	if omit != "MetricsRecorder" {
		hooks = append(hooks, MetricsRecorder(d.Metrics))
	}
	return agentcore.ChainAfter(hooks...)
}

type staticResolver struct{ measures map[string][]string }

func (s staticResolver) MeasuresFor(toolName string) []string { return s.measures[toolName] }
func (s staticResolver) IsCertified(toolName string) bool     { return false }

type staticReplay struct {
	key    string
	result *agenttools.ToolResult
}

func (s staticReplay) Lookup(ctx context.Context, key string) (*agenttools.ToolResult, bool) {
	if key == s.key {
		return s.result, true
	}
	return nil, false
}

type captureSink struct{ calls []CertifiedCall }

func (s *captureSink) RecordCertified(ctx context.Context, call CertifiedCall) error {
	s.calls = append(s.calls, call)
	return nil
}

// Guard against the chain accidentally short-circuiting when it must not.
func TestGovernanceAssemblyOrder(t *testing.T) {
	before, after := Governance(Deps{Policy: agenttools.DefaultPolicy()})
	if before == nil || after == nil {
		t.Fatal("Governance must assemble both chains")
	}
	// A clean read call passes the whole before chain.
	principal := agenttools.Principal{UserID: "u1", Permissions: []string{"*:*"}}
	ctx := withCtx(principal)
	if res, err := before(ctx, mkBC(readCall(), readDescriptor(), principal)); err != nil || res.Block || res.Short != nil {
		t.Fatalf("clean read call must pass governance, got %+v %v", res, err)
	}
}
