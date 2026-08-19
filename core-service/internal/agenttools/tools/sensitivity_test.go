package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/services/reporting"
)

type fakeSensitivityReader struct {
	calls    int
	contract string
	shocks   []float64
	payload  reporting.ProjectionResult
}

func (f *fakeSensitivityReader) Sensitivity(ctx context.Context, contractID string, baseRate *float64, shocks []float64) (reporting.ProjectionResult, error) {
	f.calls++
	f.contract = contractID
	f.shocks = shocks
	if f.payload.Payload == nil {
		return reporting.ProjectionResult{Payload: map[string]any{"meta": "fake"}}, nil
	}
	return f.payload, nil
}

func TestSensitivityToolDefinition(t *testing.T) {
	reader := &fakeSensitivityReader{}
	def := NewSensitivityDefinition(reader)
	if def.Descriptor.Name != "lease.report.sensitivity" || def.Descriptor.Level != agenttools.LevelRead || !def.Descriptor.ReadOnly {
		t.Fatalf("descriptor wrong: %+v", def.Descriptor)
	}

	call := agenttools.ToolCall{
		CallID:    "c1",
		ToolName:  "lease.report.sensitivity",
		Arguments: json.RawMessage(`{"contract_id":"C-1","shocks":[0.005,-0.005]}`),
	}
	result, err := def.Handler(context.Background(), call)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != agenttools.StatusCompleted {
		t.Fatalf("expected completed, got %s", result.Status)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["basis"] != "Certified" {
		t.Fatalf("sensitivity data must carry Certified basis, got %+v", result.Data)
	}
	if reader.calls != 1 || reader.contract != "C-1" || len(reader.shocks) != 2 {
		t.Fatalf("reader invocation wrong: calls=%d contract=%s shocks=%v", reader.calls, reader.contract, reader.shocks)
	}
}

func TestSensitivityToolValidation(t *testing.T) {
	def := NewSensitivityDefinition(&fakeSensitivityReader{})
	// Missing contract_id.
	if _, err := def.Handler(context.Background(), agenttools.ToolCall{Arguments: json.RawMessage(`{"shocks":[0.01]}`)}); err == nil {
		t.Fatal("missing contract_id must fail")
	}
	// Empty shocks.
	if _, err := def.Handler(context.Background(), agenttools.ToolCall{Arguments: json.RawMessage(`{"contract_id":"C-1","shocks":[]}`)}); err == nil {
		t.Fatal("empty shocks must fail")
	}
	// Unknown fields.
	if _, err := def.Handler(context.Background(), agenttools.ToolCall{Arguments: json.RawMessage(`{"contract_id":"C-1","shocks":[0.01],"surprise":1}`)}); err == nil {
		t.Fatal("unknown fields must fail")
	}
}

func TestSensitivityToolWithoutReader(t *testing.T) {
	def := NewSensitivityDefinition(nil)
	if _, err := def.Handler(context.Background(), agenttools.ToolCall{Arguments: json.RawMessage(`{"contract_id":"C-1","shocks":[0.01]}`)}); err == nil {
		t.Fatal("missing reader must fail")
	}
}
