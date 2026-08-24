package chatexec

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agentkernel/governance"
	picoclawagent "github.com/lease-management-system/core-service/internal/agentkernel/third_party/picoclaw/agent"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/aiagent"
	"github.com/lease-management-system/core-service/internal/aichat"
)

// ACORE-2 under the PRODUCTION assembly (AR5d acceptance 3).
//
// AR5c proved the nine mutations against the offline chain (hand-built Deps,
// direct HookManager calls). This suite proves them against what production
// actually builds: Executor.bindTurn — the same constructor wired at
// handlers/ai_chat.go — with a real agenttools Registry/Runtime underneath,
// driven through Runtime.Execute (the seam every chat tool call crosses).
//
// Controls whose production deps are deliberately unwired (parity with the
// pre-convergence wiring) get two assertions each: dormant-under-production
// (the switch changed nothing) and live-once-loaded (the ported logic works
// through this exact mounting path).

const mutationBudget = 100

type mutationWorld struct {
	executor *Executor
	kernel   *turnKernel
	registry *agenttools.Registry
}

func newMutationWorld(t *testing.T, opts func(*Deps)) *mutationWorld {
	t.Helper()
	registry := agenttools.NewRegistry()
	if err := registry.Register(agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "mut.read", Version: "v1", Description: "mutation probe", Level: agenttools.LevelRead,
			ReadOnly: true, Permissions: []agenttools.Permission{{Resource: "mut", Action: "read"}},
		},
		Handler: func(context.Context, agenttools.ToolCall) (agenttools.ToolResult, error) {
			return agenttools.ToolResult{Status: agenttools.StatusCompleted}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "mut.draft", Version: "v1", Description: "mutation write probe", Level: agenttools.LevelDraft,
			Permissions:         []agenttools.Permission{{Resource: "mut", Action: "draft"}},
			SupportsIdempotency: true,
			Review:              agenttools.ReviewPolicy{Required: true, Reasons: []string{"draft review"}, ConfirmAction: "confirm"},
		},
		Handler: func(context.Context, agenttools.ToolCall) (agenttools.ToolResult, error) {
			return agenttools.ToolResult{Status: agenttools.StatusCompleted}, nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	base := agenttools.NewRuntime(registry, agenttools.RuntimeOptions{})
	deps := Deps{Tools: base, MaxToolCalls: mutationBudget, Domain: &countingDomain{}}
	if opts != nil {
		opts(&deps)
	}
	world := &mutationWorld{registry: registry}
	world.executor = New(deps)
	kernel := world.executor.bindTurn(aichat.Execution{})
	t.Cleanup(kernel.Hooks.Close)
	world.kernel = kernel
	return world
}

type countingDomain struct{}

func (c *countingDomain) ExecuteWithRuntime(_ context.Context, _ aichat.Execution, _ *agenttools.Runtime) (aiagent.Response, error) {
	return aiagent.Response{}, nil
}

func (w *mutationWorld) call(ctx context.Context, name string) agenttools.ToolResult {
	result, err := w.kernel.Runtime.Execute(ctx, agenttools.ToolCall{
		CallID: "mc-" + name, RunID: "run-mut", ToolName: name, ToolVersion: "v1",
	})
	if err != nil {
		return agenttools.ToolResult{CallID: "mc-" + name, Status: agenttools.StatusFailed,
			Error: &agenttools.ToolError{Code: "test", Message: err.Error()}}
	}
	return result
}

func fullIdentity() context.Context {
	return agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID: "user-1", SubjectType: "web_ai_agent",
			Scope:       access.Scope{LegalEntityID: "entity-a"},
			Permissions: []string{"mut:read", "mut:draft"},
		},
	})
}

func noIdentity() context.Context {
	return agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		// permissions are complete on purpose — identity incompleteness must
		// be the ONLY failing gate, so the mutation isolates TenantScope.
		Principal: agenttools.Principal{UserID: "user-1", Permissions: []string{"mut:read", "mut:draft"}},
	})
}

// unmount removes one control from the production-assembled manager;
// remount restores it with its original priority.
func (w *mutationWorld) unmount(t *testing.T, name string) {
	t.Helper()
	w.kernel.Hooks.Unmount(name)
}

func (w *mutationWorld) remount(t *testing.T, name string) {
	t.Helper()
	for i, control := range w.kernel.controls {
		if control.Name == name {
			if err := w.kernel.Hooks.Mount(picoclawagent.HookRegistration{
				Name: control.Name, Priority: (i + 1) * 10, Hook: control.Hook,
			}); err != nil {
				t.Fatalf("remount %s: %v", name, err)
			}
			return
		}
	}
	t.Fatalf("control %q not found in production assembly", name)
}

func rejectedWith(result agenttools.ToolResult, reasonPart string) bool {
	return result.Status == agenttools.StatusRejected &&
		result.Error != nil && strings.Contains(result.Error.Message, reasonPart)
}

// ── 1 TenantScope ────────────────────────────────────────────────────────────

func TestProductionMutationTenantScope(t *testing.T) {
	w := newMutationWorld(t, nil)

	if got := w.call(noIdentity(), "mut.read"); !rejectedWith(got, "missing execution context") {
		t.Fatalf("full assembly must reject an incomplete identity, got %+v", got)
	}
	w.unmount(t, "TenantScope")
	if got := w.call(noIdentity(), "mut.read"); got.Status != agenttools.StatusCompleted {
		t.Fatalf("removing TenantScope must let the incomplete-identity call through (that is the red), got %+v", got)
	}
	w.remount(t, "TenantScope")
	if got := w.call(noIdentity(), "mut.read"); !rejectedWith(got, "missing execution context") {
		t.Fatalf("restored assembly must reject again, got %+v", got)
	}
}

// ── 2 CapabilityCheck ────────────────────────────────────────────────────────

func noPermissionIdentity() context.Context {
	return agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID: "user-1", Scope: access.Scope{LegalEntityID: "entity-a"},
			Permissions: []string{"something:else"}, // mut:* deliberately absent
		},
	})
}

func TestProductionMutationCapabilityCheck(t *testing.T) {
	w := newMutationWorld(t, nil)

	if got := w.call(noPermissionIdentity(), "mut.read"); !rejectedWith(got, "not permitted") {
		t.Fatalf("full assembly must reject a call lacking the tool permission, got %+v", got)
	}
	w.unmount(t, "CapabilityCheck")
	if got := w.call(noPermissionIdentity(), "mut.read"); got.Status != agenttools.StatusCompleted {
		t.Fatalf("removing CapabilityCheck must let the unpermitted call through, got %+v", got)
	}
	w.remount(t, "CapabilityCheck")
	if got := w.call(noPermissionIdentity(), "mut.read"); !rejectedWith(got, "not permitted") {
		t.Fatalf("restored assembly must reject again, got %+v", got)
	}
}

// ── 3 ProtectedMeasure ──────────────────────────────────────────────────────

// Production parity: no MeasureResolver is wired (none existed before
// convergence either), so the control is inert — a call is NOT blocked on
// measure grounds. Liveness is proven once the dep is loaded, through the
// same mounting path.
func TestProductionParityProtectedMeasureDormant(t *testing.T) {
	w := newMutationWorld(t, nil)
	if got := w.call(fullIdentity(), "mut.read"); got.Status != agenttools.StatusCompleted {
		t.Fatalf("with production deps the measure control must stay inert (parity with pre-convergence wiring), got %+v", got)
	}
}

func TestProductionLoadedProtectedMeasureLive(t *testing.T) {
	measures := staticMeasureCatalog{"mut.read": {"lease_liability"}}
	w := newMutationWorld(t, nil)
	controls, _ := governance.Assembly(governance.Deps{
		Policy:          agenttools.DefaultPolicy(),
		Facts:           w.kernel,
		Budget:          &governance.Budget{MaxToolCalls: mutationBudget},
		MeasureResolver: measures,
	})
	manager := picoclawagent.NewHookManager(nil)
	for i, control := range controls {
		if err := manager.Mount(picoclawagent.HookRegistration{
			Name: control.Name, Priority: (i + 1) * 10, Hook: control.Hook,
		}); err != nil {
			t.Fatal(err)
		}
	}
	w.kernel.capture(
		agenttools.ToolCall{CallID: "m1", RunID: "r", ToolName: "mut.read", ToolVersion: "v1"},
		agenttools.ToolDescriptor{Name: "mut.read", Version: "v1", Level: agenttools.LevelRead},
		fullPrincipal(),
		nil,
	)
	_, decision := manager.BeforeTool(context.Background(), &picoclawagent.ToolCallHookRequest{Tool: "mut.read"})
	if decision.Action != picoclawagent.HookActionDenyTool {
		t.Fatalf("loaded MeasureResolver must block protected measures on non-certified tools, got %+v", decision)
	}
}

type staticMeasureCatalog map[string][]string

func (s staticMeasureCatalog) MeasuresFor(tool string) []string { return s[tool] }
func (s staticMeasureCatalog) IsCertified(string) bool          { return false }

func fullPrincipal() agenttools.Principal {
	return agenttools.Principal{
		UserID: "user-1", Scope: access.Scope{LegalEntityID: "entity-a"},
		Permissions: []string{"mut:read", "mut:draft"},
	}
}

// ── 4 BudgetGuard ─────────────────────────────────────────────────────────────

func TestProductionMutationBudgetGuard(t *testing.T) {
	w := newMutationWorld(t, func(deps *Deps) { deps.MaxToolCalls = 2 })

	if got := w.call(fullIdentity(), "mut.read"); got.Status != agenttools.StatusCompleted {
		t.Fatalf("first call must fit the budget, got %+v", got)
	}
	_ = w.call(fullIdentity(), "mut.read")
	if got := w.call(fullIdentity(), "mut.read"); !rejectedWith(got, "budget exhausted") {
		t.Fatalf("exhausted budget must block the third call, got %+v", got)
	}
	w.unmount(t, "BudgetGuard")
	if got := w.call(fullIdentity(), "mut.read"); got.Status != agenttools.StatusCompleted {
		t.Fatalf("removing BudgetGuard must let further calls through, got %+v", got)
	}
	w.remount(t, "BudgetGuard")
	if got := w.call(fullIdentity(), "mut.read"); !rejectedWith(got, "budget exhausted") {
		t.Fatalf("restored assembly must block again (counter survives within the turn), got %+v", got)
	}
}

// ── 5 IdempotencyGuard ────────────────────────────────────────────────────────

func TestProductionMutationIdempotencyGuard(t *testing.T) {
	w := newMutationWorld(t, nil)

	draftCall := func(ctx context.Context) agenttools.ToolResult {
		result, err := w.kernel.Runtime.Execute(ctx, agenttools.ToolCall{
			CallID: "mc-draft", RunID: "run-mut", ToolName: "mut.draft", ToolVersion: "v1",
			// IdempotencyKey deliberately empty
		})
		if err != nil {
			return agenttools.ToolResult{Status: agenttools.StatusFailed,
				Error: &agenttools.ToolError{Code: "test", Message: err.Error()}}
		}
		return result
	}

	if got := draftCall(fullIdentity()); !rejectedWith(got, "requires idempotency_key") {
		t.Fatalf("full assembly must require an idempotency key for write-capable tools, got %+v", got)
	}
	w.unmount(t, "IdempotencyGuard")
	got := draftCall(fullIdentity())
	if got.Status == agenttools.StatusRejected && strings.Contains(got.Error.Message, "idempotency_key") {
		t.Fatalf("removing IdempotencyGuard must drop the key requirement, got %+v", got)
	}
	// needs_review here is the runtime's post-guard review forcing on a passed
	// call — proof the flow crossed the guard instead of being rejected.
	if got.Status != agenttools.StatusNeedsReview && got.Status != agenttools.StatusCompleted {
		t.Fatalf("unmounted flow must reach execution/review forcing, got %+v", got)
	}
	w.remount(t, "IdempotencyGuard")
	if got := draftCall(fullIdentity()); !rejectedWith(got, "requires idempotency_key") {
		t.Fatalf("restored assembly must reject again, got %+v", got)
	}
}

// Replay loading point under the guard seam: a stored result short-circuits
// with the stored payload; removing IdempotencyGuard drops the replay.
func TestProductionLoadedReplayShortCircuits(t *testing.T) {
	stored := &agenttools.ToolResult{Status: agenttools.StatusCompleted, Data: "replayed"}
	w := newMutationWorld(t, func(deps *Deps) {
		deps.Replay = staticLookup{stored: stored}
	})

	ctx := fullIdentity()
	result, err := w.kernel.Runtime.Execute(ctx, agenttools.ToolCall{
		CallID: "mc-replay", RunID: "run-mut", ToolName: "mut.read", ToolVersion: "v1",
		IdempotencyKey: "replay-1",
	})
	if err != nil {
		t.Fatalf("governed execute: %v", err)
	}
	if result.Data != "replayed" || result.CallID != "mc-replay" {
		t.Fatalf("replay hit must return the stored payload with normalised CallID, got %+v", result)
	}

	w.unmount(t, "IdempotencyGuard")
	result, err = w.kernel.Runtime.Execute(ctx, agenttools.ToolCall{
		CallID: "mc-replay-2", RunID: "run-mut", ToolName: "mut.read", ToolVersion: "v1",
		IdempotencyKey: "replay-1",
	})
	if err != nil {
		t.Fatalf("governed execute: %v", err)
	}
	if result.Data == "replayed" {
		t.Fatal("removing IdempotencyGuard must drop the replay short-circuit")
	}
}

type staticLookup struct{ stored *agenttools.ToolResult }

func (s staticLookup) Lookup(context.Context, string) (*agenttools.ToolResult, bool) {
	return s.stored, s.stored != nil
}

// ── 6 ReviewGate ───────────────────────────────────────────────────────────────

// Production chat policy keeps command-level tools disabled (DefaultPolicy),
// so ReviewGate sits behind CapabilityCheck as defense-in-depth — exactly as
// in the pre-convergence chain. Two assertions:
//
//   - ORDER (through Runtime.Execute): a command tool under production policy
//     is rejected by CapabilityCheck's level gate — proof that CapabilityCheck
//     dispatches before anything downstream would answer differently.
//   - LIVENESS (mounted chain, command-allowed policy mirroring AR5c's own
//     harness): the gate Responds needs_review; unmounting it drops that.
func TestProductionOrderCommandToolsGatedByCapabilityCheck(t *testing.T) {
	w := newMutationWorld(t, nil)

	commandIdentity := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID: "user-1", Scope: access.Scope{LegalEntityID: "entity-a"},
			Permissions: []string{"monthly_closing:lock"},
		},
	})
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
	result, err := w.kernel.Runtime.Execute(commandIdentity, agenttools.ToolCall{
		CallID: "mc-cmd", RunID: "run-mut", ToolName: "mut.command", ToolVersion: "v1", IdempotencyKey: "k",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !rejectedWith(result, "level command is disabled") {
		t.Fatalf("under production policy CapabilityCheck must deny command level before any later control answers, got %+v", result)
	}
}

func TestProductionLoadedReviewGateRespondsNeedsReview(t *testing.T) {
	commandDescriptor := agenttools.ToolDescriptor{
		Name: "lease.month.close.lock", Version: "v1", Description: "lock", Level: agenttools.LevelCommand,
		Permissions:         []agenttools.Permission{{Resource: "monthly_closing", Action: "lock"}},
		SupportsIdempotency: true,
		Review:              agenttools.ReviewPolicy{Required: true, Reasons: []string{"lock_period_review"}, ConfirmAction: "confirm_lock"},
	}
	policy := agenttools.DefaultPolicy()
	policy.AllowCommand = true
	policy.AllowedLevels[agenttools.LevelCommand] = true

	w := newMutationWorld(t, nil)
	controls, _ := governance.Assembly(governance.Deps{
		Policy: policy, Facts: w.kernel,
		Budget: &governance.Budget{MaxToolCalls: mutationBudget},
	})
	manager := picoclawagent.NewHookManager(nil)
	for i, control := range controls {
		if err := manager.Mount(picoclawagent.HookRegistration{
			Name: control.Name, Priority: (i + 1) * 10, Hook: control.Hook,
		}); err != nil {
			t.Fatal(err)
		}
	}

	principal := fullPrincipal()
	// Mirror AR5c's own ReviewGate harness: command tools additionally require
	// an active capability grant and the tool permission on the principal.
	principal.CapabilityActive = true
	principal.CapabilityGrants = []string{"lease.month.close.lock"}
	principal.Permissions = []string{"monthly_closing:lock"}
	call := agenttools.ToolCall{
		CallID: "mc-gate", RunID: "run-mut", ToolName: "lease.month.close.lock",
		ToolVersion: "v1", IdempotencyKey: "lock-k1",
	}
	runScenario := func(mgr *picoclawagent.HookManager) (picoclawagent.HookDecision, *agenttools.ToolResult) {
		short := w.kernel.deriveShortCircuit(context.Background(), commandDescriptor, call)
		w.kernel.capture(call, commandDescriptor, principal, nil)
		request := &picoclawagent.ToolCallHookRequest{
			Meta: w.kernel.meta(call), Tool: call.ToolName, Arguments: map[string]any{},
		}
		_, decision := mgr.BeforeTool(context.Background(), request)
		return decision, short
	}

	decision, short := runScenario(manager)
	if decision.Action != picoclawagent.HookActionRespond || short == nil ||
		short.Status != agenttools.StatusNeedsReview || len(short.Review.Actions) == 0 {
		t.Fatalf("gate must Respond needs_review with confirm actions, got decision=%+v short=%+v", decision, short)
	}

	// Unmount the gate: no Respond is produced by the chain anymore. (The
	// runtime's own post-guard forcing still blocks review-required commands —
	// defence in depth — but the gate-authored short-circuit with its Actions
	// list exists only when the gate is mounted.)
	var without *picoclawagent.HookManager
	without = picoclawagent.NewHookManager(nil)
	for i, control := range controls {
		if control.Name == "ReviewGate" {
			continue
		}
		if err := without.Mount(picoclawagent.HookRegistration{
			Name: control.Name, Priority: (i + 1) * 10, Hook: control.Hook,
		}); err != nil {
			t.Fatal(err)
		}
	}
	decision, _ = runScenario(without)
	if decision.Action == picoclawagent.HookActionRespond {
		t.Fatalf("removing ReviewGate must not produce a chain-authored Respond, got %+v", decision)
	}
}

// ── 7 AuditRecorder ──────────────────────────────────────────────────────────

func TestProductionMutationAuditRecorder(t *testing.T) {
	w := newMutationWorld(t, func(deps *Deps) { deps.ChainAudit = failingChainAudit{} })
	ctx := fullIdentity()

	// Full assembly: the failing audit write lands as an adjudication-(b)
	// marker; the call itself CONTINUES (adjudication (b) forbids aborting
	// mid-flight) — visibility comes from the turn-end drain.
	if got := w.call(ctx, "mut.read"); got.Status != agenttools.StatusCompleted {
		t.Fatalf("audit failure must not abort mid-flight, got %+v", got)
	}
	failures := w.kernel.recorder.DrainAuditFailures()
	if len(failures) == 0 {
		t.Fatal("audit failure was swallowed: no marker for the turn-end drain (adjudication b violated)")
	}
	for _, failure := range failures {
		if !strings.Contains(failure.Error(), "audit down") {
			t.Fatalf("marker must carry the original cause, got %v", failure)
		}
	}
	if again := w.kernel.recorder.DrainAuditFailures(); len(again) != 0 {
		t.Fatalf("drain must clear markers, got %v", again)
	}

	// Mutation: remove AuditRecorder — no marker ever appears; the failure
	// becomes invisible.
	w.unmount(t, "AuditRecorder")
	if got := w.call(ctx, "mut.read"); got.Status != agenttools.StatusCompleted {
		t.Fatalf("post-unmount call must complete, got %+v", got)
	}
	if failures := w.kernel.recorder.DrainAuditFailures(); len(failures) != 0 {
		t.Fatalf("unmounted audit control must record nothing, got %v", failures)
	}
	w.remount(t, "AuditRecorder")
	_ = w.call(ctx, "mut.read")
	if failures := w.kernel.recorder.DrainAuditFailures(); len(failures) == 0 {
		t.Fatal("restored control must record markers again")
	}
}

// Production parity: with chain audit unwired (runtime owns emission, exactly
// as before convergence), a normal call leaves no markers and succeeds.
func TestProductionParityAuditRuntimeOwned(t *testing.T) {
	w := newMutationWorld(t, nil)
	if got := w.call(fullIdentity(), "mut.read"); got.Status != agenttools.StatusCompleted {
		t.Fatalf("parity case must complete cleanly, got %+v", got)
	}
	if failures := w.kernel.recorder.DrainAuditFailures(); len(failures) != 0 {
		t.Fatalf("chain-side drain must stay empty while the runtime owns audit emission, got %v", failures)
	}
}

// ── 8 ArtifactCollector ───────────────────────────────────────────────────────

type captureSink struct{ calls []governance.CertifiedCall }

func (c *captureSink) RecordCertified(_ context.Context, call governance.CertifiedCall) error {
	c.calls = append(c.calls, call)
	return nil
}

func TestProductionLoadedArtifactCollectorCollects(t *testing.T) {
	var sink *captureSink
	w := newMutationWorld(t, func(deps *Deps) {
		sink = &captureSink{}
		deps.Sink = sink
	})

	if got := w.call(fullIdentity(), "mut.read"); got.Status != agenttools.StatusCompleted {
		t.Fatalf("collector scenario must complete, got %+v", got)
	}
	if len(sink.calls) != 1 || sink.calls[0].ToolName != "mut.read" {
		t.Fatalf("sink must receive the certified call, got %+v", sink.calls)
	}

	w.unmount(t, "ArtifactCollector")
	if got := w.call(fullIdentity(), "mut.read"); got.Status != agenttools.StatusCompleted {
		t.Fatalf("post-unmount call must still complete, got %+v", got)
	}
	if len(sink.calls) != 1 {
		t.Fatalf("removing ArtifactCollector must stop collection, got %+v", sink.calls)
	}
}

// Production parity: no provenance sink is wired yet (same as pre-convergence);
// calls complete and nothing is collected.
func TestProductionParityArtifactSinkUnwired(t *testing.T) {
	w := newMutationWorld(t, nil)
	if got := w.call(fullIdentity(), "mut.read"); got.Status != agenttools.StatusCompleted {
		t.Fatalf("parity case must complete cleanly, got %+v", got)
	}
}

// ── 9 MetricsRecorder ─────────────────────────────────────────────────────────

// Production parity AND anti-double-count: the runtime layer observes metrics
// for every executed call; the chain-level MetricsRecorder stays unwired. A
// successful call therefore advances TotalExecutions by EXACTLY one — if the
// switch had left both layers observing, this would read two.
func TestProductionParityMetricsObservedExactlyOnce(t *testing.T) {
	w := newMutationWorld(t, nil)
	before := w.executor.tools.Metrics().Snapshot(time.Now()).TotalExecutions
	if got := w.call(fullIdentity(), "mut.read"); got.Status != agenttools.StatusCompleted {
		t.Fatalf("call must complete, got %+v", got)
	}
	after := w.executor.tools.Metrics().Snapshot(time.Now()).TotalExecutions
	if delta := after - before; delta != 1 {
		t.Fatalf("TotalExecutions advanced by %d, want exactly 1 (runtime-owned observation, no chain double-count)", delta)
	}
}

func TestProductionLoadedMetricsRecorderObserves(t *testing.T) {
	metrics := agenttools.NewRuntimeMetrics()
	w := newMutationWorld(t, func(deps *Deps) { deps.Metrics = metrics })

	// The loaded recorder observes through the production-assembled manager:
	// capture a frame, then run the after phase exactly as the guard does.
	before := metrics.Snapshot(time.Now()).TotalExecutions
	w.kernel.capture(
		agenttools.ToolCall{CallID: "m9", RunID: "r", ToolName: "mut.read", ToolVersion: "v1"},
		agenttools.ToolDescriptor{Name: "mut.read", Version: "v1", Level: agenttools.LevelRead},
		fullPrincipal(),
		&agenttools.ToolResult{CallID: "m9", Status: agenttools.StatusCompleted},
	)
	response := &picoclawagent.ToolResultHookResponse{
		Meta: w.kernel.meta(agenttools.ToolCall{RunID: "r"}), Tool: "mut.read",
	}
	if _, decision := w.kernel.Hooks.AfterTool(context.Background(), response); decision.Action == picoclawagent.HookActionAbortTurn {
		t.Fatal("after phase aborted unexpectedly")
	}
	if after := metrics.Snapshot(time.Now()).TotalExecutions; after != before+1 {
		t.Fatalf("loaded MetricsRecorder must observe the call (%d -> %d)", before, after)
	}
}
