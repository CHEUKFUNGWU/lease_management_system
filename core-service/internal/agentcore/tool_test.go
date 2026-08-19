package agentcore

import (
	"context"
	"testing"

	"github.com/lease-management-system/core-service/internal/agenttools"
)

func testDefinition(name string, level agenttools.ToolLevel, handler agenttools.ToolHandler) agenttools.ToolDefinition {
	return agenttools.ToolDefinition{
		Descriptor: agenttools.ToolDescriptor{Name: name, Version: "v1", Level: level},
		Handler:    handler,
	}
}

func TestFromDefinitionRejectsParallelNonRead(t *testing.T) {
	def := testDefinition("lease.contract.create", agenttools.LevelDraft, func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
		return agenttools.ToolResult{}, nil
	})
	if _, err := FromDefinition(def, Parallel); err == nil {
		t.Fatal("expected registration failure for non-read tool declared Parallel")
	}
	if _, err := FromDefinition(def, Sequential); err != nil {
		t.Fatalf("Sequential non-read tool must register: %v", err)
	}
}

func TestFromDefinitionAllowsParallelRead(t *testing.T) {
	def := testDefinition("retail.operating_pulse.read", agenttools.LevelRead, func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
		return agenttools.ToolResult{}, nil
	})
	if _, err := FromDefinition(def, Parallel); err != nil {
		t.Fatalf("read tool may be Parallel: %v", err)
	}
}

func TestValidateToolsRejectsDuplicates(t *testing.T) {
	def := testDefinition("lease.contract.get", agenttools.LevelRead, func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
		return agenttools.ToolResult{}, nil
	})
	t1, err := FromDefinition(def, Sequential)
	if err != nil {
		t.Fatal(err)
	}
	t2, err := FromDefinition(def, Sequential)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateTools([]Tool{t1, t2}); err == nil {
		t.Fatal("expected duplicate detection")
	}
	if err := ValidateTools([]Tool{t1}); err != nil {
		t.Fatalf("single tool must validate: %v", err)
	}
}

func TestValidateToolsRejectsParallelWriteInSet(t *testing.T) {
	write := testDefinition("lease.contract.draft.create", agenttools.LevelDraft, func(ctx context.Context, call agenttools.ToolCall) (agenttools.ToolResult, error) {
		return agenttools.ToolResult{}, nil
	})
	// A definitionTool with mode Parallel is not producible through
	// FromDefinition; ValidateTools must still catch it.
	bad := definitionTool{def: write, mode: Parallel}
	if err := ValidateTools([]Tool{bad}); err == nil {
		t.Fatal("expected ValidateTools to reject Parallel write tool")
	}
}
