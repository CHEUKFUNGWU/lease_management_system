package aiagent

import (
	"testing"

	"github.com/lease-management-system/core-service/internal/agentartifact"
	"github.com/lease-management-system/core-service/internal/aichat"
	"github.com/lease-management-system/core-service/internal/repository"
)

func TestPlannerInheritsSourceRunSkill(t *testing.T) {
	agent := &Agent{}
	skillID := "payment_schedule"

	plan := agent.Plan(aichat.Input{Message: "继续"}, &repository.AIChatRun{SkillID: &skillID})

	if got, want := plan.SkillID, skillID; got != want {
		t.Fatalf("plan skill = %q, want %q", got, want)
	}
	if !plan.AgentMode {
		t.Fatalf("inherited skill should enable agent mode")
	}
}

func TestProjectResultCreatesEvidenceBackedAuditPackArtifact(t *testing.T) {
	result := ProjectResult(Response{
		Answer: "审计摘要", Model: "test-model",
		Sources:       []Source{{Type: "contract", ID: "contract-1", Title: "合同", Snippet: "审批状态 approved"}},
		ReviewPrompts: []AgentReviewPrompt{{ID: "scope", Title: "确认范围"}},
		AuditPack:     &AuditPackData{Basis: "official", Scope: "le-001", Answer: "审计摘要", SourceCount: 1, SourceIDs: []string{"contract-1"}},
		EvidenceRefs:  []agentartifact.EvidenceReference{{ReferenceID: "contract-1", Complete: true, Locators: []agentartifact.EvidenceLocator{{Field: "record", Source: "system:contract"}}}},
	})
	if len(result.Artifacts) != 1 || result.Artifacts[0].Type != string(agentartifact.ArtifactAuditPack) || !result.Artifacts[0].EvidenceComplete {
		t.Fatalf("audit artifact=%+v", result.Artifacts)
	}
}

func TestProjectResultCreatesDataQualityArtifactForIntakeWarnings(t *testing.T) {
	result := ProjectResult(Response{
		Answer: "已生成合同草稿", Model: "test-model",
		BatchSummary: &BatchParseSummary{
			MissingFields: []string{"discount_rate"}, Warnings: []string{"scope confidence is low"},
			ReviewReasons: []string{"discount_rate_missing"}, IntakeID: "intake-1",
		},
	})
	if len(result.Artifacts) != 1 || result.Artifacts[0].Type != string(agentartifact.ArtifactDataQualityIssues) {
		t.Fatalf("artifacts=%+v", result.Artifacts)
	}
	artifact := result.Artifacts[0]
	if artifact.EvidenceComplete || !artifact.ReviewRequired || len(artifact.ReviewReasons) == 0 {
		t.Fatalf("quality artifact=%+v", artifact)
	}
	data, ok := artifact.Data.(map[string]any)
	if !ok || data["source"] != "contract_batch" {
		t.Fatalf("quality data=%#v", artifact.Data)
	}
}

func TestProjectResultCreatesReportExplanationArtifact(t *testing.T) {
	result := ProjectResult(Response{
		Answer: "Working 报表显示本期负债上升。", Model: "test-model",
		Sources:           []Source{{Type: "report", ID: "report-1", Title: "负债滚动表", Snippet: "2026-01"}},
		ReportExplanation: &ReportExplanationData{Page: "reports", Period: "2026-01", Basis: "working", Answer: "Working 报表显示本期负债上升.", SourceIDs: []string{"report-1"}},
	})
	if len(result.Artifacts) != 1 || result.Artifacts[0].Type != "report_explanation" || !result.Artifacts[0].ReviewRequired {
		t.Fatalf("report artifact=%+v", result.Artifacts)
	}
	if !result.Artifacts[0].EvidenceComplete || len(result.Artifacts[0].EvidenceRefs) != 1 {
		t.Fatalf("report evidence=%+v", result.Artifacts[0])
	}
}

func TestPlannerPrefersCurrentIntent(t *testing.T) {
	agent := &Agent{}
	sourceSkillID := "payment_schedule"

	plan := agent.Plan(
		aichat.Input{Message: "请生成审计包，包含披露核对清单"},
		&repository.AIChatRun{SkillID: &sourceSkillID},
	)

	if got, want := plan.SkillID, "audit_pack"; got != want {
		t.Fatalf("plan skill = %q, want %q", got, want)
	}
}

func TestPlannerDoesNotBypassSkillRoleRestriction(t *testing.T) {
	agent := NewWithOperationalReadersAndGovernanceAndRetail(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	plan := agent.Plan(aichat.Input{Message: "生成审计包", Role: "editor", SkillID: "audit_pack", SkillVersion: "v1"}, nil)
	if plan.AgentMode || plan.SkillID != "" {
		t.Fatalf("restricted explicit skill should not be selected: %+v", plan)
	}
}
