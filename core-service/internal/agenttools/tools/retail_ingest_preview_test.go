package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/pagefill"
	"github.com/lease-management-system/core-service/internal/workingpaper"
)

type fakeIngestFileReader struct {
	content []byte
	err     error
	calls   int
}

func (r *fakeIngestFileReader) ReadObject(ctx context.Context, objectName string) ([]byte, error) {
	r.calls++
	return r.content, r.err
}

// previewTemplate produces a minimal store-day sheet in the controlled
// template shape the deterministic mapper understands.
const previewTemplate = `store_code,store_name,business_date,revenue,gross_profit,transactions,footfall,area_sqm,labor_cost,fixed_rent,variable_rent,non_lease_cost,other_controllable_cost,currency
ST001,旗舰店,2026-08-01,12000,6000,300,900,80,3000,2000,400,500,600,CNY
ST001,旗舰店,2026-08-02,12500,6300,310,920,80,3000,2000,410,500,600,CNY
`

func previewExecutionContext() context.Context {
	return agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "bp-zhang", Scope: access.Scope{LegalEntityID: "LE-1"}, Permissions: []string{"*:*"}},
		RunID:     "run-fill-1",
	})
}

func previewCall(t *testing.T) agenttools.ToolCall {
	t.Helper()
	return agenttools.ToolCall{
		CallID:    "call-fill-1",
		ToolName:  "retail.store_days.import.preview",
		Arguments: json.RawMessage(`{"file_id":"f1","object_name":"stores.csv","content_type":"text/csv","source_system":"pos-a","as_of":"2026-08-18"}`),
	}
}

func TestRetailIngestPreviewToolProducesPageFill(t *testing.T) {
	reader := &fakeIngestFileReader{content: []byte(previewTemplate)}
	def := NewRetailIngestPreviewDefinition(reader)
	result, err := def.Handler(previewExecutionContext(), previewCall(t))
	if err != nil {
		t.Fatal(err)
	}
	data, ok := result.Data.(map[string]any)
	if !ok || data["side_effects"] != false {
		t.Fatalf("data wrong: %+v", result.Data)
	}
	fill, ok := data["page_fill"].(*pagefill.Fill)
	if !ok {
		t.Fatalf("page_fill missing: %+v", result.Data)
	}
	if fill.TargetPage != "retail-data-import" {
		t.Fatalf("target page wrong: %+v", fill)
	}
	if err := fill.AssertNoExploratoryInPayload(); err != nil {
		t.Fatalf("ACORE-12: payload must be clean at assembly: %v", err)
	}
	// Envelope fields are human-provided; mapping is an unconfirmed rule
	// suggestion — Exploratory, structurally barred from the payload.
	if v, ok := fill.Payload["source_system"]; !ok || v.Provenance.Basis != workingpaper.BasisHumanInput {
		t.Fatalf("source_system must be human-confirmed payload, got %+v", fill.Payload)
	}
	sugg, ok := fill.Suggestions["mapping"]
	if !ok || sugg.Provenance.Basis != workingpaper.BasisExploratory {
		t.Fatalf("mapping must be an Exploratory suggestion, got %+v", fill.Suggestions)
	}
	if refs := fill.ExploratoryRefs(); len(refs) != 1 || refs[0] != "mapping" {
		t.Fatalf("exploratory refs wrong: %v", refs)
	}
	if err := fill.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRetailIngestPreviewToolHonestWithoutReader(t *testing.T) {
	def := NewRetailIngestPreviewDefinition(nil)
	_, err := def.Handler(previewExecutionContext(), previewCall(t))
	if err == nil || !strings.Contains(err.Error(), "not wired") {
		t.Fatalf("missing reader must refuse honestly, got %v", err)
	}
}

func TestRetailIngestPreviewToolValidation(t *testing.T) {
	reader := &fakeIngestFileReader{content: []byte(previewTemplate)}
	def := NewRetailIngestPreviewDefinition(reader)
	bad := agenttools.ToolCall{Arguments: json.RawMessage(`{"file_id":"f1","object_name":"x.csv","content_type":"text/csv"}`)}
	if _, err := def.Handler(previewExecutionContext(), bad); err == nil {
		t.Fatal("missing source_system must fail")
	}
	broken := previewCall(t)
	broken.Arguments = json.RawMessage(`{"file_id":"f1","object_name":"x.csv","content_type":"text/csv","source_system":"s","surprise":1}`)
	if _, err := def.Handler(previewExecutionContext(), broken); err == nil {
		t.Fatal("unknown fields must fail the strict decoder")
	}
}
