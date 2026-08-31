package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/agenttools"
)

func TestDeterministicTriageRoutes(t *testing.T) {
	cases := []struct {
		name string
		req  TriageRequest
		want DocClass
	}{
		{"rent schedule", TriageRequest{ObjectName: "2026-07租金表.xlsx"}, DocRentSchedule},
		{"rent schedule en", TriageRequest{ObjectName: "rent-schedule.xlsx", UserMessage: "这是租金表"}, DocRentSchedule},
		{"ledger", TriageRequest{ObjectName: "合同台账.xlsx", UserMessage: "批量导入合同"}, DocContractLedger},
		{"amendment", TriageRequest{ObjectName: "补充协议.pdf"}, DocAmendment},
		{"invoice", TriageRequest{ObjectName: "发票_202607.pdf"}, DocInvoice},
		{"financial", TriageRequest{ObjectName: "利润表.xlsx"}, DocFinancialStatement},
		{"trial balance", TriageRequest{ObjectName: "GL试算平衡表_2026-07.xlsx"}, DocTrialBalance},
		{"budget plan", TriageRequest{ObjectName: "门店预算版本_2026.xlsx"}, DocBudgetPlan},
		{"minutes", TriageRequest{ObjectName: "会议纪要.docx"}, DocMeetingMinutes},
		{"operating", TriageRequest{ObjectName: "门店销售数据.xlsx"}, DocOperatingData},
		{"contract", TriageRequest{ObjectName: "租赁合同扫描件.pdf"}, DocLeaseContract},
	}
	for _, c := range cases {
		got := DeterministicTriage(c.req)
		if got.DocClass != c.want {
			t.Fatalf("%s: DeterministicTriage = %s, want %s", c.name, got.DocClass, c.want)
		}
	}
}

// CORR-6's core failure mode: out-of-domain files must resolve to unknown,
// never to lease_contract.
func TestDeterministicTriageNeverDefaultsToContract(t *testing.T) {
	for _, name := range []string{"劳动合同.pdf", "宣传册.pdf", "README.md", "random-file.bin", "培训材料.docx"} {
		got := DeterministicTriage(TriageRequest{ObjectName: name})
		if got.DocClass != DocUnknown {
			t.Fatalf("%s must triage to unknown, got %s", name, got.DocClass)
		}
		if len(got.Candidates) == 0 {
			t.Fatalf("%s: unknown result must carry candidates", name)
		}
		if got.GapCode != "doc_class_unresolved" {
			t.Fatalf("%s: unknown result gap_code = %q", name, got.GapCode)
		}
	}
}

func TestTriageToolDefinition(t *testing.T) {
	def := NewDocTriageDefinition(nil)
	if def.Descriptor.Name != "lease.file.triage" || def.Descriptor.Level != agenttools.LevelRead || !def.Descriptor.ReadOnly {
		t.Fatalf("triage tool descriptor wrong: %+v", def.Descriptor)
	}
	call := agenttools.ToolCall{
		CallID:    "c1",
		ToolName:  "lease.file.triage",
		Arguments: json.RawMessage(`{"file_id":"f1","object_name":"发票.pdf","content_type":"application/pdf"}`),
	}
	result, err := def.Handler(context.Background(), call)
	if err != nil {
		t.Fatal(err)
	}
	data, ok := result.Data.(TriageResult)
	if !ok || data.DocClass != DocInvoice {
		t.Fatalf("triage tool must return DocInvoice for an invoice, got %+v", result.Data)
	}

	// Strict schema: unknown fields must fail.
	bad := agenttools.ToolCall{Arguments: json.RawMessage(`{"file_id":"f1","object_name":"x","content_type":"y","surprise":1}`)}
	if _, err := def.Handler(context.Background(), bad); err == nil || !strings.Contains(err.Error(), "invalid triage arguments") {
		t.Fatalf("unknown fields must be rejected, got %v", err)
	}
}
