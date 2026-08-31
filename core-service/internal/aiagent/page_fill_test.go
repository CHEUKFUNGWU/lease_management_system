package aiagent

import (
	"context"
	"strings"
	"testing"

	"github.com/lease-management-system/core-service/internal/agenttools"
	agenttooldefs "github.com/lease-management-system/core-service/internal/agenttools/tools"
	"github.com/lease-management-system/core-service/internal/pagefill"
)

func TestProjectResultPageFillWithoutEvidenceStaysIncomplete(t *testing.T) {
	projected := ProjectResult(Response{
		Model:    "deterministic-router",
		PageFill: pagefill.New("retail-data-import", "POST /retail/import", "/retail-data-import?fill=artifact"),
	})
	if len(projected.Artifacts) != 1 {
		t.Fatalf("page-fill artifacts = %d, want 1", len(projected.Artifacts))
	}
	artifact := projected.Artifacts[0]
	if artifact.EvidenceComplete {
		t.Fatal("page-fill without evidence references must not claim complete evidence")
	}
	if len(artifact.EvidenceRefs) != 0 {
		t.Fatalf("evidence refs = %d, want none", len(artifact.EvidenceRefs))
	}
	if len(artifact.ReviewReasons) != 2 || artifact.ReviewReasons[1] != "evidence_incomplete" {
		t.Fatalf("review reasons = %v, want evidence_incomplete", artifact.ReviewReasons)
	}
}

func TestExtractSourceSystem(t *testing.T) {
	if got := extractSourceSystem("我把门店销售数据传上来了，来源系统 pos-a"); got != "pos-a" {
		t.Fatalf("got %q", got)
	}
	if got := extractSourceSystem("导入一下 source system ERP-X，谢谢"); got != "ERP-X" {
		t.Fatalf("got %q", got)
	}
	if got := extractSourceSystem("没有来源"); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

// The fill seam is registered with no file reader (D-D2): the agent must
// refuse honestly and guide the user to the import page, never fabricate a
// preview.
func TestExecuteRetailIngestFillHonestWithoutReader(t *testing.T) {
	reader := &agentRetailReader{set: agentRetailSet()}
	agent := NewAgent(AgentPorts{RetailKPI: reader})
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "bp-zhang", Permissions: []string{"*:*"}},
		RunID:     "run-fill-1",
	})
	triage := agenttooldefs.TriageResult{DocClass: agenttooldefs.DocOperatingData, Confidence: 0.8}
	resp, err := agent.executeRetailIngestFill(ctx, Request{
		Message: "导入经营数据，来源系统 pos-a",
		FileID:  "f1",
	}, triage, func(ctx context.Context, eventType string, payload any) error { return nil }, agent.toolRuntime)
	if err != nil {
		t.Fatal(err)
	}
	if resp.PageFill != nil {
		t.Fatal("no reader means no preview — PageFill must stay nil")
	}
	if !strings.Contains(resp.Answer, "零售数据导入") {
		t.Fatalf("answer must guide to the import page, got %q", resp.Answer)
	}
}

func TestExecuteRetailIngestFillRequiresSourceSystem(t *testing.T) {
	reader := &agentRetailReader{set: agentRetailSet()}
	agent := NewAgent(AgentPorts{RetailKPI: reader})
	ctx := agenttools.WithExecutionContext(context.Background(), agenttools.ExecutionContext{
		Principal: agenttools.Principal{UserID: "bp-zhang", Permissions: []string{"*:*"}},
		RunID:     "run-fill-2",
	})
	resp, err := agent.executeRetailIngestFill(ctx, Request{Message: "导入经营数据"}, agenttooldefs.TriageResult{DocClass: agenttooldefs.DocOperatingData}, func(ctx context.Context, eventType string, payload any) error { return nil }, agent.toolRuntime)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Answer, "来源系统") {
		t.Fatalf("missing source system must be asked for, got %q", resp.Answer)
	}
}
