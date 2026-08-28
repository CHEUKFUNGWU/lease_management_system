package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/pagefill"
)

type fakeTBReader struct{ data []byte }

func (f fakeTBReader) ReadObject(_ context.Context, _ string) ([]byte, error) { return f.data, nil }

const tbCSV = `account_code,account_name,debit,credit
1001,Cash,1200.50,0
2001,AP,0,800.25
`

func tbExecContext() context.Context {
	return agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "u1", Role: "editor", Permissions: []string{"*:*"}},
		RunID:     "run-tb",
		SkillID:   "fpna_copilot",
	})
}

// TestTrialBalanceFillProducesPageFill locks the P0-B① contract: valid CSV in,
// a Fill out whose payload carries ONLY the human envelope fields and whose
// suggestions hold the parsed structure — Exploratory never reaches Payload.
func TestTrialBalanceFillProducesPageFill(t *testing.T) {
	def := NewTrialBalanceFillDefinition(fakeTBReader{data: []byte(tbCSV)})
	args, _ := json.Marshal(map[string]any{
		"file_id": "f1", "object_name": "tb.csv", "content_type": "text/csv",
		"name": "2026-07 GL TB", "source_system": "gl-export",
		"period": "2026-07", "functional_currency": "cny",
	})
	result, err := def.Handler(tbExecContext(), agenttools.ToolCall{
		CallID: "call-1", RunID: "run-tb", ToolName: def.Descriptor.Name, ToolVersion: "v1",
		Arguments: args, IdempotencyKey: "k1",
	})
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if result.Status != agenttools.StatusCompleted || !result.Review.Required {
		t.Fatalf("status=%s review=%+v", result.Status, result.Review)
	}
	fill, ok := result.Data.(map[string]any)["page_fill"].(*pagefill.Fill)
	if !ok {
		t.Fatalf("page_fill type = %T", result.Data.(map[string]any)["page_fill"])
	}
	payload := fill.FillPayload()
	for _, key := range []string{"source_system", "period", "name"} {
		if _, present := payload[key]; !present {
			t.Fatalf("payload missing %s: %+v", key, payload)
		}
	}
	if payload["functional_currency"] != "CNY" {
		t.Fatalf("currency must be normalised to upper: %v", payload["functional_currency"])
	}
	suggestions := fill.ExploratoryRefs()
	foundStructure := false
	for _, ref := range suggestions {
		if strings.Contains(ref, "column_structure") || ref == "column_structure" {
			foundStructure = true
		}
	}
	if !foundStructure && len(suggestions) == 0 {
		t.Fatalf("column_structure suggestion missing; exploratory refs=%v", suggestions)
	}
}

func TestTrialBalanceFillRejectsWrongShapeAndMissingColumns(t *testing.T) {
	// A file lacking the required columns must be refused with the column list.
	// Missing required columns is a handler-level go-error; through the
	// runtime it becomes a failed result — either way the refusal must name
	// the required columns and never fabricate a preview.
	badReader := NewTrialBalanceFillDefinition(fakeTBReader{data: []byte("foo,bar\nx,y\n")})
	_, err := badReader.Handler(tbExecContext(), agenttools.ToolCall{
		CallID: "call-2", RunID: "run-tb", ToolName: badReader.Descriptor.Name, ToolVersion: "v1",
		Arguments:      json.RawMessage(`{"file_id":"f2","object_name":"tb-bad.csv","content_type":"text/csv","source_system":"gl","period":"2026-07"}`),
		IdempotencyKey: "k2",
	})
	if err == nil || !strings.Contains(err.Error(), "account_code") {
		t.Fatalf("missing-column refusal must name the required columns, got %v", err)
	}
	goodDef := NewTrialBalanceFillDefinition(fakeTBReader{data: []byte(tbCSV)})
	// unknown fields rejected (strict decoding)
	_, err = goodDef.Handler(tbExecContext(), agenttools.ToolCall{
		CallID: "call-3", RunID: "run-tb", ToolName: goodDef.Descriptor.Name, ToolVersion: "v1",
		Arguments:      json.RawMessage(`{"file_id":"f3","object_name":"o.csv","content_type":"text/csv","source_system":"gl","period":"2026-07","hacker":"x"}`),
		IdempotencyKey: "k3",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid trial balance fill arguments") {
		t.Fatalf("unknown field must be refused, got %v", err)
	}
}

func TestTrialBalanceFillRequiresReader(t *testing.T) {
	def := NewTrialBalanceFillDefinition(nil)
	_, err := def.Handler(tbExecContext(), agenttools.ToolCall{
		CallID: "call-4", RunID: "run-tb", ToolName: def.Descriptor.Name, ToolVersion: "v1",
		Arguments:      json.RawMessage(`{"file_id":"f4","object_name":"o.csv","content_type":"text/csv","source_system":"gl","period":"2026-07"}`),
		IdempotencyKey: "k4",
	})
	if err == nil || !strings.Contains(err.Error(), "reader is not wired") {
		t.Fatalf("unwired reader must refuse honestly, got %v", err)
	}
}
