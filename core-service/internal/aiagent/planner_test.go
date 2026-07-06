package aiagent

import (
	"testing"

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
