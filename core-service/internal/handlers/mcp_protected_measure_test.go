package handlers

// RT1-L3-D review point ⑤: MCP tools are NEVER certified engines. When a
// measure catalog exists and a tool's name is bound to protected measures,
// the call must land in the Reject tier — registration provides no
// certification path, and the assertion pins that guarantee through the real
// governance chain rather than leaving it as inference.
//
// Production currently wires no MeasureResolver (the catalog is an open item),
// so this test arms a resolver itself and drives a real runtime guarded by
// the assembly — the same shape the gateway mutation tests use.

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agentkernel/governance"
	picoclawagent "github.com/lease-management-system/core-service/internal/agentkernel/third_party/picoclaw/agent"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/mcp"
)

func TestMCPToolProtectedMeasuresLandsInRejectTier(t *testing.T) {
	registry := agenttools.NewRegistry()
	definition, err := mcp.BuildDefinition("analytics", mcp.ToolEntry{
		Name:        "store_revenue",
		Description: "external revenue read",
		Permissions: []mcp.Permission{{Resource: "mcp", Action: "revenue"}},
		InputSchema: json.RawMessage(`{"type":"object","properties":{"store_id":{"type":"string"}},"required":["store_id"]}`),
	}, func(context.Context, string, json.RawMessage) (mcp.ToolCallResult, error) {
		return mcp.ToolCallResult{Text: "looked up"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(definition); err != nil {
		t.Fatal(err)
	}

	// Measure catalog binds the tool's name to a protected measure. The
	// resolver never marks it certified — and nothing in the mcp package has
	// any way to mark it so.
	measures := &staticMeasuresGuard{measures: map[string][]string{"mcp.analytics.store_revenue": {"lease_liability"}}}
	controls, _ := governance.Assembly(governance.Deps{
		Policy:             agenttools.DefaultPolicy(),
		Facts:              staticFactsResolver{},
		MeasureResolver:    measures,
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
	runtime := agenttools.NewRuntime(registry, agenttools.RuntimeOptions{Guard: &rc1ChainGuardAdapter{manager: manager}})

	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{
			UserID: "u1", SubjectType: "agent_gateway",
			Scope:       access.Scope{LegalEntityID: "entity-a"},
			Permissions: []string{"mcp:revenue"},
		},
	})
	result, err := runtime.Execute(ctx, agenttools.ToolCall{
		CallID: "mcp-pm", RunID: "run-pm", ToolName: "mcp.analytics.store_revenue", ToolVersion: "v1",
		Arguments: json.RawMessage(`{"store_id":"s1"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != agenttools.StatusRejected || result.Error == nil || result.Error.Code != agenttools.ErrorBusinessFailure {
		t.Fatalf("an uncertified MCP tool requesting protected measures must be REJECTED by the chain (business_failure), got %+v", result)
	}
}

type staticMeasuresGuard struct {
	measures map[string][]string
}

func (s *staticMeasuresGuard) MeasuresFor(tool string) []string { return s.measures[tool] }
func (s *staticMeasuresGuard) IsCertified(tool string) bool     { return false }

// staticFactsResolver reads the per-call facts frame carried in ctx — the
// same mechanism as the production gatewayFactsResolver (concurrency-safe for
// a shared instance).
type staticFactsKeyType struct{}

var staticFactsKeyValue = staticFactsKeyType{}

type staticFactsResolver struct{}

func (staticFactsResolver) FactsFor(ctx context.Context, _ *picoclawagent.ToolCallHookRequest) (governance.CallFacts, error) {
	facts, ok := ctx.Value(staticFactsKeyValue).(*governance.CallFacts)
	if !ok || facts == nil {
		return governance.CallFacts{}, fmt.Errorf("facts not captured")
	}
	return *facts, nil
}

// rc1ChainGuardAdapter mirrors handlers.governedGatewayGuard's decision
// translation (Continue passes / Respond short-circuits / deny carries the
// sink code) — minimal adapter so this test drives the REAL runtime seam.
type rc1ChainGuardAdapter struct {
	manager *picoclawagent.HookManager
}

func (g *rc1ChainGuardAdapter) Before(ctx context.Context, call agenttools.ToolCall, descriptor agenttools.ToolDescriptor, principal agenttools.Principal) (agenttools.GuardResult, error) {
	facts := governance.CallFacts{Call: call, Descriptor: descriptor, Principal: principal}
	factoredCtx := context.WithValue(ctx, staticFactsKeyValue, &facts)
	sink := &governance.RejectSink{}
	factoredCtx = governance.WithRejectSink(factoredCtx, sink)
	request := &picoclawagent.ToolCallHookRequest{
		Meta: gatewayMeta(call), Tool: call.ToolName, Arguments: map[string]any{},
	}
	_, decision := g.manager.BeforeTool(factoredCtx, request)
	switch decision.Action {
	case "", picoclawagent.HookActionContinue, picoclawagent.HookActionModify:
		return agenttools.GuardResult{}, nil
	default:
		return agenttools.GuardResult{Block: true, Reason: decision.Reason, Code: sink.Code()}, nil
	}
}

func (g *rc1ChainGuardAdapter) After(context.Context, agenttools.ToolCall, agenttools.ToolDescriptor, *agenttools.ToolResult, agenttools.Principal) error {
	return nil
}
