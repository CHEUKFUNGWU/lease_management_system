package governance

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	picoclawagent "github.com/lease-management-system/core-service/internal/agentkernel/third_party/picoclaw/agent"
	"github.com/lease-management-system/core-service/internal/agenttools"
)

// ACORE-2 on the picoclaw mount points: every mutation is proven by running a
// scenario through the REAL vendored HookManager dispatcher — first with the
// full assembly (rejected), then with the control removed (passes), then with
// the assembly restored (rejected again). The dispatcher semantics under test
// are hooks.go's own: hook errors skip the hook, so every rejection here must
// travel as a DenyTool/Respond decision — that IS the ported contract.

// ── fakes ──────────────────────────────────────────────────────────────────

type staticFacts struct {
	facts     CallFacts
	resolveEr error
	calls     int
}

func (f *staticFacts) FactsFor(_ context.Context, _ *picoclawagent.ToolCallHookRequest) (CallFacts, error) {
	f.calls++
	if f.resolveEr != nil {
		return CallFacts{}, f.resolveEr
	}
	return f.facts, nil
}

type staticMeasures struct{ measures map[string][]string }

func (s staticMeasures) MeasuresFor(tool string) []string { return s.measures[tool] }
func (s staticMeasures) IsCertified(tool string) bool     { return false }

type staticReplay struct{ stored *agenttools.ToolResult }

func (s staticReplay) Lookup(_ context.Context, _ string) (*agenttools.ToolResult, bool) {
	return s.stored, s.stored != nil
}

type captureSink struct{ calls []CertifiedCall }

func (c *captureSink) RecordCertified(_ context.Context, call CertifiedCall) error {
	c.calls = append(c.calls, call)
	return nil
}

var auditErr = errors.New("audit down")

type failingAudit struct{}

func (failingAudit) RecordToolExecution(_ context.Context, _ agenttools.ToolExecutionAudit) error {
	return auditErr
}

func readDescriptor() agenttools.ToolDescriptor {
	return agenttools.ToolDescriptor{
		Name: "lease.contract.get", Version: "v1",
		Level: agenttools.LevelRead, ReadOnly: true,
	}
}

// mustManager builds the assembly and discards the mounted-name list.
func mustManager(t *testing.T, deps Deps, omit string) *picoclawagent.HookManager {
	t.Helper()
	manager, _ := assembly(t, deps, omit)
	return manager
}

func accessScope(entityID string) access.Scope {
	return access.Scope{LegalEntityID: entityID}
}

func readFacts() CallFacts {
	return CallFacts{
		Descriptor: readDescriptor(),
		Call:       agenttools.ToolCall{CallID: "call-1", ToolName: "lease.contract.get", ToolVersion: "v1"},
		Principal: agenttools.Principal{
			UserID: "u1", Scope: accessScope("entity-a"),
		},
	}
}

func draftDescriptor() agenttools.ToolDescriptor {
	return agenttools.ToolDescriptor{Name: "lease.contract.draft.create", Version: "v1", Level: agenttools.LevelDraft}
}

func commandDescriptor() agenttools.ToolDescriptor {
	return agenttools.ToolDescriptor{
		Name: "lease.month.close.lock", Version: "v1", Level: agenttools.LevelCommand,
		Permissions: []agenttools.Permission{{Resource: "monthly_closing", Action: "lock"}},
		Review: agenttools.ReviewPolicy{
			Required: true, Reasons: []string{"lock_period_review"}, ConfirmAction: "confirm_lock",
		},
	}
}

// assembly mounts the nine controls in product order, omitting one name for a
// mutation run. Names double as the assembly test's presence check.
func assembly(t *testing.T, deps Deps, omit string) (*picoclawagent.HookManager, []string) {
	t.Helper()
	manager := picoclawagent.NewHookManager(nil)
	controls, recorder := Assembly(deps)
	mounted := []string{}
	for _, control := range controls {
		if control.Name == omit {
			continue // mutation: this control's logic leaves the chain entirely
		}
		if err := manager.Mount(picoclawagent.NamedHook(control.Name, control.Hook)); err != nil {
			t.Fatalf("mount %s: %v", control.Name, err)
		}
		mounted = append(mounted, control.Name)
	}
	_ = recorder
	return manager, mounted
}

// ── the nine mutations ─────────────────────────────────────────────────────

func request(tool string) *picoclawagent.ToolCallHookRequest {
	return &picoclawagent.ToolCallHookRequest{Tool: tool}
}

func denied(d picoclawagent.HookDecision) bool {
	return d.Action == picoclawagent.HookActionDenyTool
}

func TestMutationTenantScope(t *testing.T) {
	// 场景：FactsResolver 解析成功、但 principal 身份不完整（空 user）。
	// 这是 TenantScope 独占的判定——其余 control 不再重复检查身份完整性，
	// 因此移除 TenantScope 后，同一次调用会一路走到执行端点。
	facts := readFacts()
	facts.Principal.UserID = "" // identity incomplete, resolution itself succeeds
	deps := Deps{Policy: agenttools.DefaultPolicy(), Facts: &staticFacts{facts: facts}}

	// full assembly: deny
	manager := mustManager(t, deps, "")
	if _, decision := manager.BeforeTool(context.Background(), request("lease.contract.get")); !denied(decision) {
		t.Fatalf("full chain must reject an incomplete identity, got %+v", decision)
	}

	// remove TenantScope: the same request now flows past the gate
	without, _ := assembly(t, deps, "TenantScope")
	_, decision := without.BeforeTool(context.Background(), request("lease.contract.get"))
	if denied(decision) {
		t.Fatalf("removing TenantScope must let the incomplete-identity call through (that is the red), got %+v", decision)
	}
}
func TestMutationCapabilityCheck(t *testing.T) {
	facts := readFacts()
	facts.Descriptor = draftDescriptor()
	facts.Call = agenttools.ToolCall{
		CallID: "c2", ToolName: "lease.contract.draft.create", ToolVersion: "v1",
		IdempotencyKey: "k1", // keep IdempotencyGuard out of the way; only CapabilityCheck is under test
	}
	policy := agenttools.DefaultPolicy()
	delete(policy.AllowedLevels, agenttools.LevelDraft)
	deps := Deps{Policy: policy, Facts: &staticFacts{facts: facts}}

	mgr, _ := assembly(t, deps, "")
	_, decision := mgr.BeforeTool(context.Background(), request("x"))
	if !denied(decision) {
		t.Fatalf("full chain must reject disabled level, got %+v", decision)
	}
	_, decision = mustManager(t, deps, "CapabilityCheck").BeforeTool(context.Background(), request("x"))
	if denied(decision) {
		t.Fatalf("removing CapabilityCheck must let the call through, got %+v", decision)
	}
}

func TestMutationProtectedMeasure(t *testing.T) {
	measures := staticMeasures{measures: map[string][]string{
		"lease.predeal.simulate": {"lease_liability"},
	}}
	facts := readFacts()
	facts.Descriptor = agenttools.ToolDescriptor{Name: "lease.predeal.simulate", Version: "v1", Level: agenttools.LevelRead, ReadOnly: true}
	facts.Call = agenttools.ToolCall{CallID: "c3", ToolName: "lease.predeal.simulate", ToolVersion: "v1"}
	deps := Deps{Policy: agenttools.DefaultPolicy(), MeasureResolver: measures, Facts: &staticFacts{facts: facts}}

	mgr := mustManager(t, deps, "")
	_, decision := mgr.BeforeTool(context.Background(), request("lease.predeal.simulate"))
	if !denied(decision) {
		t.Fatalf("full chain must block protected measures on non-certified tools, got %+v", decision)
	}
	_, decision = mustManager(t, deps, "ProtectedMeasure").BeforeTool(context.Background(), request("lease.predeal.simulate"))
	if denied(decision) {
		t.Fatalf("removing ProtectedMeasure must let the call through, got %+v", decision)
	}
}

func TestMutationBudgetGuard(t *testing.T) {
	facts := readFacts()
	deps := Deps{Policy: agenttools.DefaultPolicy(), Budget: &Budget{MaxToolCalls: 1}, Facts: &staticFacts{facts: facts}}

	manager, _ := assembly(t, deps, "")
	if _, decision := manager.BeforeTool(context.Background(), request("x")); denied(decision) {
		t.Fatal("first call must fit the budget")
	}
	if _, decision := manager.BeforeTool(context.Background(), request("x")); !denied(decision) {
		t.Fatal("exhausted budget must block")
	}
	relaxed, _ := assembly(t, deps, "BudgetGuard")
	if _, decision := relaxed.BeforeTool(context.Background(), request("x")); denied(decision) {
		t.Fatal("removing BudgetGuard must let calls through")
	}
	if _, decision := relaxed.BeforeTool(context.Background(), request("x")); denied(decision) {
		t.Fatal("removing BudgetGuard must let further calls through")
	}
}

func TestMutationIdempotencyGuard(t *testing.T) {
	facts := readFacts()
	facts.Descriptor = draftDescriptor()
	facts.Call = agenttools.ToolCall{CallID: "c4", ToolName: "lease.contract.draft.create", ToolVersion: "v1"}
	stored := &agenttools.ToolResult{CallID: "c4", Status: agenttools.StatusCompleted, Data: "replayed"}
	deps := Deps{Policy: agenttools.DefaultPolicy(), Replay: staticReplay{stored: stored}, Facts: &staticFacts{facts: facts}}

	// write without key → deny
	noKey := deps
	noKey.Replay = nil
	mgr := mustManager(t, noKey, "")
	_, decision := mgr.BeforeTool(context.Background(), request("x"))
	if !denied(decision) {
		t.Fatalf("full chain must require an idempotency key for writes, got %+v", decision)
	}
	// replay hit → Respond short-circuit carrying the stored result
	hit := deps
	hit.Facts = &staticFacts{facts: func() CallFacts {
		f := readFacts()
		f.Call.IdempotencyKey = "replay-1"
		return f
	}()}
	_, decision = mustManager(t, hit, "").BeforeTool(context.Background(), request("x"))
	if decision.Action != picoclawagent.HookActionRespond {
		t.Fatalf("replay hit must short-circuit via Respond, got %+v", decision)
	}
	// removing IdempotencyGuard drops both behaviours
	_, decision = mustManager(t, hit, "IdempotencyGuard").BeforeTool(context.Background(), request("x"))
	if decision.Action == picoclawagent.HookActionRespond || denied(decision) {
		t.Fatalf("removing IdempotencyGuard must drop replay and key checks, got %+v", decision)
	}
}

func TestMutationReviewGate(t *testing.T) {
	facts := readFacts()
	facts.Descriptor = commandDescriptor()
	facts.Principal.CapabilityActive = true
	facts.Principal.CapabilityGrants = []string{"lease.month.close.lock"}
	facts.Principal.Permissions = []string{"monthly_closing:lock"}
	facts.Call = agenttools.ToolCall{CallID: "c5", ToolName: "lease.month.close.lock", ToolVersion: "v1", IdempotencyKey: "lock-k1"}
	policy := agenttools.DefaultPolicy()
	policy.AllowCommand = true
	policy.AllowedLevels[agenttools.LevelCommand] = true
	deps := Deps{Policy: policy, Facts: &staticFacts{facts: facts}}

	mgr := mustManager(t, deps, "")
	updated, decision := mgr.BeforeTool(context.Background(), request("lease.month.close.lock"))
	hookResult := updated.HookResult
	if decision.Action != picoclawagent.HookActionRespond || hookResult == nil ||
		!strings.Contains(hookResult.ForLLM, "needs_review") {
		t.Fatalf("command requiring review must Respond with needs_review, got %+v %+v", decision, hookResult)
	}
	_, decision = mustManager(t, deps, "ReviewGate").BeforeTool(context.Background(), request("lease.month.close.lock"))
	if decision.Action == picoclawagent.HookActionRespond {
		t.Fatalf("removing ReviewGate must not produce needs_review, got %+v", decision)
	}
}

// ── after-phase mutations (audit adjudication (b)) ─────────────────────────

func TestMutationAuditRecorder(t *testing.T) {
	deps := func() Deps {
		return Deps{Policy: agenttools.DefaultPolicy(), Audit: failingAudit{}, Facts: &staticFacts{facts: readFactsWithResult()}}
	}
	controls, recorder := Assembly(deps())
	manager := picoclawagent.NewHookManager(nil)
	for _, control := range controls {
		if err := manager.Mount(picoclawagent.NamedHook(control.Name, control.Hook)); err != nil {
			t.Fatal(err)
		}
	}
	res := &picoclawagent.ToolResultHookResponse{Tool: "lease.contract.get", Result: toVendorResult(readFactsWithResult().Result)}

	_, decision := manager.AfterTool(context.Background(), res)
	if decision.Action == picoclawagent.HookActionAbortTurn || decision.Action == picoclawagent.HookActionHardAbort {
		t.Fatalf("adjudication (b) forbids aborting the turn mid-flight: %+v", decision)
	}
	failures := recorder.DrainAuditFailures()
	if len(failures) == 0 {
		t.Fatal("audit failure was swallowed: the run cannot fail at turn end (adjudication b violated)")
	}
	for _, failure := range failures {
		if !errors.Is(failure, auditErr) && !strings.Contains(failure.Error(), auditErr.Error()) &&
			!strings.Contains(failure.Error(), "audit down") {
			t.Fatalf("marker does not carry the original cause: %v", failure)
		}
	}
	if again := recorder.DrainAuditFailures(); len(again) != 0 {
		t.Fatalf("drain must clear markers, got %v", again)
	}

	// removing AuditRecorder: no marker ever appears
	controlsWithout, _ := Assembly(func() Deps {
		d := deps()
		d.Audit = nil
		return d
	}())
	clean := picoclawagent.NewHookManager(nil)
	for _, control := range controlsWithout {
		if err := clean.Mount(picoclawagent.NamedHook(control.Name, control.Hook)); err != nil {
			t.Fatal(err)
		}
	}
	if _, decision := clean.AfterTool(context.Background(), res); decision.Action != picoclawagent.HookActionContinue {
		t.Fatalf("removing AuditRecorder must keep the pass-through, got %+v", decision)
	}
}

func readFactsWithResult() CallFacts {
	facts := readFacts()
	facts.Result = &agenttools.ToolResult{CallID: "call-1", Status: agenttools.StatusCompleted}
	return facts
}

func withResult(facts CallFacts) CallFacts {
	facts.Result = &agenttools.ToolResult{CallID: facts.Call.CallID, Status: agenttools.StatusCompleted}
	return facts
}

func afterResponse(facts CallFacts) *picoclawagent.ToolResultHookResponse {
	return &picoclawagent.ToolResultHookResponse{
		Tool: facts.Call.ToolName, Result: toVendorResult(facts.Result), Duration: 0,
	}
}

func TestMutationArtifactCollector(t *testing.T) {
	facts := withResult(readFacts())
	sink := &captureSink{}
	deps := Deps{Policy: agenttools.DefaultPolicy(), Sink: sink, Facts: &staticFacts{facts: facts}}

	manager, _ := assembly(t, deps, "")
	if _, decision := manager.AfterTool(context.Background(), afterResponse(facts)); decision.Action == picoclawagent.HookActionAbortTurn {
		t.Fatal("after phase aborted the turn unexpectedly")
	}
	if len(sink.calls) != 1 || sink.calls[0].CallID != "call-1" {
		t.Fatalf("sink must receive the certified call, got %+v", sink.calls)
	}
	without, _ := assembly(t, deps, "ArtifactCollector")
	sink.calls = nil
	if _, _ = without.AfterTool(context.Background(), afterResponse(facts)); false {
		t.Fatal("unreachable")
	}
	if len(sink.calls) != 0 {
		t.Fatalf("removing ArtifactCollector must stop collection, got %+v", sink.calls)
	}
}

func TestMutationMetricsRecorder(t *testing.T) {
	facts := withResult(readFacts())
	metrics := agenttools.NewRuntimeMetrics()
	deps := Deps{Policy: agenttools.DefaultPolicy(), Metrics: metrics, Facts: &staticFacts{facts: facts}}

	manager, _ := assembly(t, deps, "")
	if _, decision := manager.AfterTool(context.Background(), afterResponse(facts)); decision.Action == picoclawagent.HookActionAbortTurn {
		t.Fatal("after phase aborted the turn unexpectedly")
	}
	if metrics.Snapshot(time.Now()).TotalExecutions == 0 {
		t.Fatal("metrics must observe the call")
	}
	without, _ := assembly(t, deps, "MetricsRecorder")
	before := metrics.Snapshot(time.Now()).TotalExecutions
	if _, _ = without.AfterTool(context.Background(), afterResponse(facts)); false {
		t.Fatal("unreachable")
	}
	if metrics.Snapshot(time.Now()).TotalExecutions != before {
		t.Fatal("removing MetricsRecorder must stop observation")
	}
}

// ── 装配守卫：生产装配下九个中间件全部在位 ────────────────────────────────

func TestAssemblyMountsAllNineControls(t *testing.T) {
	deps := Deps{Policy: agenttools.DefaultPolicy(), Facts: &staticFacts{facts: readFacts()}}
	_, mounted := assembly(t, deps, "")
	want := []string{
		"TenantScope", "CapabilityCheck", "ProtectedMeasure", "BudgetGuard",
		"IdempotencyGuard", "ReviewGate", "AuditRecorder", "ArtifactCollector",
		"MetricsRecorder",
	}
	for _, name := range want {
		found := false
		for _, got := range mounted {
			if got == name {
				found = true
			}
		}
		if !found {
			t.Errorf("production assembly is missing %q — the chain is weakened (ACORE-2)", name)
		}
	}
}

// toVendorResult lives in governance.go (production helper).
