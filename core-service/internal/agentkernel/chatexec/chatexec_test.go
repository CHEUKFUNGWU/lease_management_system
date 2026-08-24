package chatexec

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/aiagent"
	"github.com/lease-management-system/core-service/internal/aichat"
)

// ── AR5-G3：形状 A 守卫 ─────────────────────────────────────────────────────

// shapeBFixture is the reverse-injection fixture: exactly the shape D-C19
// forbids. It must trip every detector check below; if it stops tripping
// them, the guard has gone blind and the real assertion proves nothing.
type shapeBFixture struct {
	states map[string]*turnKernel // map field — forbidden
	events chan int               // channel field — forbidden
	mu     sync.Mutex             // locker field — forbidden
	order  []string               // mutable slice field — forbidden
}

func TestExecutorHoldsNoPerRunMutableState(t *testing.T) {
	if violations := mutableStateFields(reflect.TypeOf(Executor{})); len(violations) != 0 {
		t.Fatalf("the production chat executor holds per-run mutable state (shape A violated):\n  %s",
			strings.Join(violations, "\n  "))
	}
}

func TestMutableStateDetectorCatchesTheForbiddenShape(t *testing.T) {
	violations := mutableStateFields(reflect.TypeOf(shapeBFixture{}))
	for _, want := range []string{"states", "events", "mu", "order"} {
		found := false
		for _, violation := range violations {
			if strings.Contains(violation, want) {
				found = true
			}
		}
		if !found {
			t.Fatalf("detector failed to flag %q — the shape A guard is blind; violations were:\n  %s",
				want, strings.Join(violations, "\n  "))
		}
	}
}

// ── 生产入口集成：Executor.Execute 驱动内核治理链 ───────────────────────────

type recordedEvents []struct {
	eventType string
	payload   any
}

type emitCollector struct{ events recordedEvents }

func (c *emitCollector) emit(_ context.Context, eventType string, payload any) error {
	c.events = append(c.events, struct {
		eventType string
		payload   any
	}{eventType, payload})
	return nil
}

// scriptedDomain plays the part of aiagent.Agent inside the delegation
// window: it executes its scripted probe calls against the runtime it was
// handed WHILE the turn's kernel chain is live. Probing after Executor.Execute
// returns would prove nothing — the per-turn HookManager closes with the turn
// (shape A: nothing survives the call).
type probeCall struct {
	ctx context.Context
	c   agenttools.ToolCall
}

type scriptedDomain struct {
	calls    []probeCall
	received *agenttools.Runtime
	results  []agenttools.ToolResult
}

func (s *scriptedDomain) ExecuteWithRuntime(ctx context.Context, exec aichat.Execution, toolRuntime *agenttools.Runtime) (aiagent.Response, error) {
	s.received = toolRuntime
	for _, probe := range s.calls {
		callCtx := probe.ctx
		if callCtx == nil {
			callCtx = ctx
		}
		result, err := toolRuntime.Execute(callCtx, probe.c)
		if err != nil {
			return aiagent.Response{}, err
		}
		s.results = append(s.results, result)
	}
	return aiagent.Response{Answer: "done"}, nil
}

func stubReadTool(t *testing.T, registry *agenttools.Registry, invoked *int) {
	t.Helper()
	err := registry.Register(agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{
			Name: "stub.read", Version: "v1", Description: "stub read tool", Level: agenttools.LevelRead,
			ReadOnly: true, Permissions: []agenttools.Permission{{Resource: "stub", Action: "read"}},
		},
		Handler: func(context.Context, agenttools.ToolCall) (agenttools.ToolResult, error) {
			*invoked++
			return agenttools.ToolResult{Status: agenttools.StatusCompleted, Data: map[string]string{"ok": "yes"}}, nil
		},
	})
	if err != nil {
		t.Fatalf("register stub.read: %v", err)
	}
}

func executionContext(entityID, userID string) context.Context {
	return agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID: userID, SubjectType: "web_ai_agent",
			Scope: accessScopeFor(entityID), Permissions: []string{"stub:read"},
		},
	})
}

func accessScopeFor(entityID string) access.Scope {
	return access.Scope{LegalEntityID: entityID}
}

// The production entry wired at handlers/ai_chat.go must route every governed
// tool call across the vendored HookManager. Evidence here is behavioural:
// with the full production assembly, an incomplete identity never reaches a
// registered handler; with a complete one it does, and the event bridge fires.
func TestExecuteRoutesGovernedToolCallsThroughKernelChain(t *testing.T) {
	registry := agenttools.NewRegistry()
	invoked := 0
	stubReadTool(t, registry, &invoked)
	base := agenttools.NewRuntime(registry, agenttools.RuntimeOptions{})
	emits := &emitCollector{}
	domain := &scriptedDomain{calls: []probeCall{{
		ctx: executionContext("entity-a", "user-1"),
		c:   agenttools.ToolCall{CallID: "c1", RunID: "run-1", ToolName: "stub.read", ToolVersion: "v1"},
	}}}
	executor := New(Deps{Domain: domain, Tools: base, MaxToolCalls: 4})

	_, err := executor.Execute(executionContext("entity-a", "user-1"), aichat.Execution{
		Input: aichat.Input{UserID: "user-1", LegalEntityID: "entity-a", Message: "hi"},
		Emit:  emits.emit,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if domain.received == nil {
		t.Fatal("domain runner received no tool runtime")
	}
	result := domain.results[0]
	if invoked != 1 {
		t.Fatalf("handler invocations = %d, want 1", invoked)
	}
	if result.Status != agenttools.StatusCompleted {
		t.Fatalf("status = %q, want completed", result.Status)
	}
	toolExecutionEvents := 0
	for _, event := range emits.events {
		if event.eventType == "tool_execution" {
			toolExecutionEvents++
		}
	}
	if toolExecutionEvents != 1 {
		t.Fatalf("tool_execution events = %d, want 1 (the emit bridge must stay attached)", toolExecutionEvents)
	}

	// Kernel evidence: under the SAME production assembly an incomplete
	// identity is denied by TenantScope before the handler runs.
	domain = &scriptedDomain{calls: []probeCall{{
		ctx: executionContext("", "user-2"),
		c:   agenttools.ToolCall{CallID: "c2", RunID: "run-2", ToolName: "stub.read", ToolVersion: "v1"},
	}}}
	executor = New(Deps{Domain: domain, Tools: base, MaxToolCalls: 4})
	_, err = executor.Execute(context.Background(), aichat.Execution{
		Input: aichat.Input{UserID: "user-2", Message: "hi"},
	})
	if err != nil {
		t.Fatalf("execute without entity: %v", err)
	}
	result = domain.results[0]
	if invoked != 1 {
		t.Fatal("the denied call incremented handler invocations")
	}
	// RC1: identity incompleteness is unauthenticated — NOT scope_denied,
	// which is bottom-line-1 evidence and only appears for real tenant
	// violations. The external wording converges like publicPolicyError.
	if result.Status != agenttools.StatusRejected || result.Error == nil ||
		result.Error.Code != agenttools.ErrorUnauthenticated {
		t.Fatalf("rejection = %+v, want rejected with unauthenticated", result)
	}
}

// Adjudication (b), end to end through the production entry: when the loaded
// chain-side audit recorder fails, the whole run fails at turn end carrying
// the original cause.
func TestExecuteFailsRunWhenChainAuditWriteFails(t *testing.T) {
	registry := agenttools.NewRegistry()
	invoked := 0
	stubReadTool(t, registry, &invoked)
	base := agenttools.NewRuntime(registry, agenttools.RuntimeOptions{})
	domain := &scriptedDomain{calls: []probeCall{{
		ctx: executionContext("entity-a", "user-1"),
		c:   agenttools.ToolCall{CallID: "c1", RunID: "run-1", ToolName: "stub.read", ToolVersion: "v1"},
	}}}
	executor := New(Deps{
		Domain: domain, Tools: base, MaxToolCalls: 4,
		ChainAudit: failingChainAudit{},
	})

	_, err := executor.Execute(executionContext("entity-a", "user-1"), aichat.Execution{
		Input: aichat.Input{UserID: "user-1", LegalEntityID: "entity-a", Message: "hi"},
	})
	if err == nil || !strings.Contains(err.Error(), "audit down") {
		t.Fatalf("run error = %v, want the adjudication-(b) failure carrying the original cause", err)
	}
	if invoked != 1 {
		t.Fatalf("handler invocations = %d, want 1 (audit failure must not abort mid-flight)", invoked)
	}
}

type failingChainAudit struct{}

func (failingChainAudit) RecordToolExecution(context.Context, agenttools.ToolExecutionAudit) error {
	return errors.New("audit down")
}

// Replay loading point: a stored result short-circuits the call as Respond
// before the handler runs.
func TestExecuteReplaysStoredResultWithoutExecuting(t *testing.T) {
	registry := agenttools.NewRegistry()
	invoked := 0
	stubReadTool(t, registry, &invoked)
	base := agenttools.NewRuntime(registry, agenttools.RuntimeOptions{})
	domain := &scriptedDomain{calls: []probeCall{{
		ctx: executionContext("entity-a", "user-1"),
		c:   agenttools.ToolCall{CallID: "c9", RunID: "run-9", ToolName: "stub.read", ToolVersion: "v1", IdempotencyKey: "k1"},
	}}}
	executor := New(Deps{
		Domain: domain, Tools: base, MaxToolCalls: 4,
		Replay: staticReplayStore{stored: &agenttools.ToolResult{
			Status: agenttools.StatusCompleted, Data: "replayed",
		}},
	})

	if _, err := executor.Execute(executionContext("entity-a", "user-1"), aichat.Execution{
		Input: aichat.Input{UserID: "user-1", LegalEntityID: "entity-a", Message: "hi"},
	}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	result := domain.results[0]
	if invoked != 0 {
		t.Fatal("replayed call executed again — replay short-circuit did not fire")
	}
	if result.Data != "replayed" || result.CallID != "c9" {
		t.Fatalf("replay result = %+v, want stored payload with normalised CallID", result)
	}
}

type staticReplayStore struct{ stored *agenttools.ToolResult }

func (s staticReplayStore) Lookup(context.Context, string) (*agenttools.ToolResult, bool) {
	return s.stored, s.stored != nil
}

// mutableStateFields reports every direct field of a struct type that is a
// map, channel, slice, array or sync.Locker — the per-run/per-account state a
// long-lived shared executor must never hold (D-C19 shape A). Value-type
// struct fields are walked one level deep because wrapping a map in a struct
// does not purify it. Pointer fields are NOT followed: shared dependencies
// (registry owner, domain wiring) are legitimate, and their internals are
// governed by their own packages.
func mutableStateFields(structType reflect.Type) []string {
	if structType.Kind() != reflect.Struct {
		return nil
	}
	var violations []string
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		kind := classifyMutable(field.Type)
		switch kind {
		case "map", "chan", "slice", "array", "locker":
			violations = append(violations, field.Name+" ("+field.Type.String()+")")
		case "":
			// Value-struct fields are walked one level deep: wrapping a map in
			// a struct does not purify it. Pointer fields stay unopened.
			if field.Type.Kind() != reflect.Struct {
				continue
			}
			for j := 0; j < field.Type.NumField(); j++ {
				inner := field.Type.Field(j)
				if innerKind := classifyMutable(inner.Type); innerKind != "" && innerKind != "struct" {
					violations = append(violations,
						field.Name+"."+inner.Name+" ("+inner.Type.String()+")")
				}
			}
		}
	}
	return violations
}

func classifyMutable(t reflect.Type) string {
	lockerType := reflect.TypeOf((*sync.Locker)(nil)).Elem()
	isPointer := false
	for t.Kind() == reflect.Pointer {
		isPointer = true
		t = t.Elem()
	}
	if isPointer {
		// Pointer fields are NOT followed (see mutableStateFields): shared
		// dependencies may legitimately contain locks or registries.
		return ""
	}
	// A value-struct field whose pointer implements sync.Locker IS a lock
	// (sync.Mutex declares pointer receivers).
	if t.Kind() == reflect.Struct && reflect.PointerTo(t).Implements(lockerType) {
		return "locker"
	}
	switch t.Kind() {
	case reflect.Map:
		return "map"
	case reflect.Chan:
		return "chan"
	case reflect.Slice:
		return "slice"
	case reflect.Array:
		return "array"
	default:
		return ""
	}
}
