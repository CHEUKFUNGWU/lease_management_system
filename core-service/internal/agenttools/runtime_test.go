package agenttools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
)

func readTool(name, permission string, handler ToolHandler) ToolDefinition {
	return ToolDefinition{
		Descriptor: ToolDescriptor{
			Name:        name,
			Version:     "v1",
			Description: "test read tool",
			Level:       LevelRead,
			ReadOnly:    true,
			Permissions: []Permission{{Resource: permission, Action: "read"}},
		},
		Handler: handler,
	}
}

func draftTool(name, permission string, handler ToolHandler) ToolDefinition {
	return ToolDefinition{
		Descriptor: ToolDescriptor{
			Name:                name,
			Version:             "v1",
			Description:         "test draft tool",
			Level:               LevelDraft,
			Permissions:         []Permission{{Resource: permission, Action: "create"}},
			Review:              ReviewPolicy{Required: true, Reasons: []string{"human confirmation"}},
			SupportsIdempotency: true,
		},
		Handler: handler,
	}
}

func runtimeContext() context.Context {
	return WithExecutionContext(context.Background(), ExecutionContext{
		Principal: Principal{
			UserID:      "user-1",
			Permissions: []string{"contracts:read", "contracts:create"},
			Scope:       access.Scope{LegalEntityID: "le-001"},
		},
		RunID: "run-1", TraceID: "trace-1", SkillID: "contract_review", SkillVersion: "v1",
	})
}

func TestRegistryRejectsDuplicateVersion(t *testing.T) {
	registry := NewRegistry()
	definition := readTool("lease.contract.get", "contracts", func(context.Context, ToolCall) (ToolResult, error) {
		return ToolResult{Data: map[string]any{"ok": true}}, nil
	})
	if err := registry.Register(definition); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := registry.Register(definition); !errors.Is(err, ErrInvalidToolDescriptor) {
		t.Fatalf("duplicate register error = %v", err)
	}
}

func TestRuntimeDescribesOnlyPermittedToolsAndStripsSchemasByDefault(t *testing.T) {
	registry := NewRegistry()
	read := readTool("lease.contract.get", "contracts", func(context.Context, ToolCall) (ToolResult, error) {
		return ToolResult{}, nil
	})
	read.Descriptor.InputSchema = json.RawMessage(`{"type":"object"}`)
	if err := registry.Register(read); err != nil {
		t.Fatalf("register read: %v", err)
	}
	if err := registry.Register(readTool("lease.audit.list", "audit_logs", func(context.Context, ToolCall) (ToolResult, error) {
		return ToolResult{}, nil
	})); err != nil {
		t.Fatalf("register audit: %v", err)
	}

	descriptors, err := NewRuntime(registry, RuntimeOptions{}).Describe(runtimeContext(), ToolFilter{})
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if len(descriptors) != 1 || descriptors[0].Name != "lease.contract.get" {
		t.Fatalf("descriptors = %#v, want only permitted contract tool", descriptors)
	}
	if len(descriptors[0].InputSchema) != 0 {
		t.Fatal("expected schema to be omitted unless explicitly requested")
	}
}

func TestRuntimeExecutesRegisteredToolWithAuthenticatedScope(t *testing.T) {
	registry := NewRegistry()
	var observed access.Scope
	if err := registry.Register(readTool("lease.contract.get", "contracts", func(ctx context.Context, call ToolCall) (ToolResult, error) {
		observed, _ = access.ScopeFromContext(ctx)
		return ToolResult{Data: map[string]any{"contract_id": call.Arguments}}, nil
	})); err != nil {
		t.Fatalf("register: %v", err)
	}

	result, err := NewRuntime(registry, RuntimeOptions{}).Execute(runtimeContext(), ToolCall{
		CallID: "call-1", RunID: "run-1", ToolName: "lease.contract.get", ToolVersion: "v1",
		Arguments: json.RawMessage(`{"contract_id":"contract-1"}`),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Status != StatusCompleted || result.Error != nil {
		t.Fatalf("result = %#v", result)
	}
	if observed.LegalEntityID != "le-001" {
		t.Fatalf("handler observed scope = %+v", observed)
	}
}

func TestRuntimeReturnsStructuredPermissionRejection(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(readTool("lease.event.list", "events", func(context.Context, ToolCall) (ToolResult, error) {
		t.Fatal("handler must not run after permission rejection")
		return ToolResult{}, nil
	})); err != nil {
		t.Fatalf("register: %v", err)
	}
	result, err := NewRuntime(registry, RuntimeOptions{}).Execute(runtimeContext(), ToolCall{
		CallID: "call-1", RunID: "run-1", ToolName: "lease.event.list", ToolVersion: "v1",
		Arguments: json.RawMessage(`{"contract_id":"foreign"}`),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Status != StatusRejected || result.Error == nil || result.Error.Code != ErrorPermissionDenied {
		t.Fatalf("permission result = %#v", result)
	}
}

func TestRuntimeDoesNotExecuteCommandBeforeReview(t *testing.T) {
	registry := NewRegistry()
	called := false
	definition := draftTool("lease.contract.create_draft", "contracts", func(context.Context, ToolCall) (ToolResult, error) {
		called = true
		return ToolResult{Data: map[string]any{"draft": true}}, nil
	})
	definition.Descriptor.Level = LevelCommand
	definition.Descriptor.Description = "test command"
	if err := registry.Register(definition); err != nil {
		t.Fatalf("register: %v", err)
	}

	policy := DefaultPolicy()
	policy.AllowedLevels[LevelCommand] = true
	policy.AllowCommand = true
	ctx := WithExecutionContext(runtimeContext(), ExecutionContext{
		Principal: Principal{
			UserID:           "user-1",
			Permissions:      []string{"contracts:create"},
			CapabilityGrants: []string{"lease.contract.create_draft"},
		},
		RunID: "run-1",
	})
	result, err := NewRuntime(registry, RuntimeOptions{Policy: policy}).Execute(ctx, ToolCall{
		CallID: "call-1", RunID: "run-1", ToolName: definition.Descriptor.Name, ToolVersion: "v1",
		Arguments: json.RawMessage(`{"contract_name":"draft"}`), IdempotencyKey: "idem-1",
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if called || result.Status != StatusNeedsReview || !result.Review.Required {
		t.Fatalf("command result = %#v, called=%v", result, called)
	}
}

func TestRuntimeRecordsStructuredAuditWithoutRawArguments(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(readTool("lease.contract.get", "contracts", func(context.Context, ToolCall) (ToolResult, error) {
		return ToolResult{Data: map[string]any{"ok": true}}, nil
	})); err != nil {
		t.Fatalf("register: %v", err)
	}
	var recorded ToolExecutionAudit
	clock := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	runtime := NewRuntime(registry, RuntimeOptions{
		Now: func() time.Time {
			return clock
		},
		Audit: AuditRecorderFunc(func(_ context.Context, audit ToolExecutionAudit) error {
			recorded = audit
			return nil
		}),
	})
	ctx := runtimeContext()
	result, err := runtime.Execute(ctx, ToolCall{
		CallID: "call-audit", RunID: "run-audit", TraceID: "trace-audit",
		ToolName: "lease.contract.get", ToolVersion: "v1",
		Arguments: json.RawMessage(`{"contract_id":"contract-1","amount":999999}`),
	})
	if err != nil || result.Status != StatusCompleted {
		t.Fatalf("Execute result=%#v err=%v", result, err)
	}
	if recorded.CallID != "call-audit" || recorded.UserID != "user-1" || recorded.LegalEntityID != "le-001" || recorded.SkillID != "contract_review" || recorded.SkillVersion != "v1" {
		t.Fatalf("audit identity = %+v", recorded)
	}
	if len(recorded.ArgumentsSHA256) != 64 || recorded.ArgumentsSHA256 == "contract-1" {
		t.Fatalf("audit argument fingerprint = %q", recorded.ArgumentsSHA256)
	}
	if recorded.Status != StatusCompleted || recorded.CompletedAt.Before(recorded.StartedAt) {
		t.Fatalf("audit outcome = %+v", recorded)
	}
}

func TestRuntimeRecordsLowCardinalityMetrics(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(readTool("lease.test.metrics", "contracts", func(context.Context, ToolCall) (ToolResult, error) {
		return ToolResult{Data: map[string]any{"ok": true}}, nil
	})); err != nil {
		t.Fatalf("register: %v", err)
	}
	metrics := NewRuntimeMetrics()
	runtime := NewRuntime(registry, RuntimeOptions{Metrics: metrics})
	if _, err := runtime.Execute(runtimeContext(), ToolCall{
		CallID: "call-metrics", RunID: "run-metrics", ToolName: "lease.test.metrics", ToolVersion: "v1", Arguments: []byte(`{}`),
	}); err != nil {
		t.Fatal(err)
	}
	snapshot := metrics.Snapshot(time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC))
	if snapshot.TotalExecutions != 1 || len(snapshot.ToolMetrics) != 1 {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	prometheus := metrics.Prometheus(time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC))
	if !strings.Contains(prometheus, `lease_agent_tool_executions_total{tool="lease.test.metrics",status="completed"} 1`) {
		t.Fatalf("prometheus output=%s", prometheus)
	}
}

func TestRuntimeSurfacesAuditFailureAsSystemFailure(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(readTool("lease.contract.get", "contracts", func(context.Context, ToolCall) (ToolResult, error) {
		return ToolResult{Data: map[string]any{"ok": true}}, nil
	})); err != nil {
		t.Fatalf("register: %v", err)
	}
	runtime := NewRuntime(registry, RuntimeOptions{
		Audit: AuditRecorderFunc(func(context.Context, ToolExecutionAudit) error {
			return errors.New("audit store unavailable")
		}),
	})
	result, err := runtime.Execute(runtimeContext(), ToolCall{
		CallID: "call-audit", RunID: "run-audit", ToolName: "lease.contract.get", ToolVersion: "v1",
		Arguments: json.RawMessage(`{"contract_id":"contract-1"}`),
	})
	if err == nil || result.Status != StatusFailed || result.Error == nil || result.Error.Code != ErrorSystemFailure {
		t.Fatalf("audit failure result=%#v err=%v", result, err)
	}
}

// ── RC1 fail-closed fallback ───────────────────────────────────────────────
//
// A guard that blocks without claiming a code is a wiring bug. The runtime
// must fail closed to system_failure and must NOT reach for scope_denied —
// that code is bottom-line-1 evidence and may only appear for a real tenant
// violation. The guard's own reason rides along, because this is precisely
// the case someone will have to diagnose.

type codelessBlockGuard struct{ reason string }

func (g codelessBlockGuard) Before(context.Context, ToolCall, ToolDescriptor, Principal) (GuardResult, error) {
	return GuardResult{Block: true, Reason: g.reason}, nil
}

func (codelessBlockGuard) After(context.Context, ToolCall, ToolDescriptor, *ToolResult, Principal) error {
	return nil
}

func TestRuntimeCodelessGuardBlockFailsClosedAndKeepsReason(t *testing.T) {
	registry := NewRegistry()
	if err := registry.Register(readTool("lease.contract.get", "contracts",
		func(context.Context, ToolCall) (ToolResult, error) {
			return ToolResult{Data: map[string]any{"ok": true}}, nil
		})); err != nil {
		t.Fatalf("register: %v", err)
	}
	runtime := NewRuntime(registry, RuntimeOptions{Guard: codelessBlockGuard{reason: "hook wiring lost its code"}})

	result, err := runtime.Execute(runtimeContext(), ToolCall{
		CallID: "call-1", RunID: "run-1", ToolName: "lease.contract.get",
		ToolVersion: "v1", Arguments: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if result.Error == nil {
		t.Fatalf("codeless block must reject, got %+v", result)
	}
	if result.Error.Code == ErrorScopeDenied {
		t.Fatal("codeless block defaulted to scope_denied — that code is reserved for real tenant violations")
	}
	if result.Error.Code != ErrorSystemFailure {
		t.Fatalf("code = %q, want %q", result.Error.Code, ErrorSystemFailure)
	}
	if !strings.Contains(result.Error.Message, "hook wiring lost its code") {
		t.Fatalf("message = %q, want the guard's reason preserved for diagnosis", result.Error.Message)
	}
}
