package agentskill

import (
	_ "embed"
	"encoding/json"
	"testing"
)

//go:embed testdata/skill-cases.json
var skillRoutingCases []byte

func TestSkillRoutingEvaluationCases(t *testing.T) {
	var cases []struct {
		ID             string `json:"id"`
		Message        string `json:"message"`
		Role           string `json:"role"`
		RequestedSkill string `json:"requested_skill"`
		ExpectedSkill  string `json:"expected_skill"`
	}
	if err := json.Unmarshal(skillRoutingCases, &cases); err != nil {
		t.Fatal(err)
	}
	registry := ProductionRegistry()
	for _, testCase := range cases {
		t.Run(testCase.ID, func(t *testing.T) {
			definition, selected := registry.Select(Intent{
				Message:          testCase.Message,
				Role:             testCase.Role,
				RequestedSkillID: testCase.RequestedSkill,
			})
			if !selected {
				if testCase.ExpectedSkill != "" {
					t.Fatalf("skill not selected, want %q", testCase.ExpectedSkill)
				}
				return
			}
			if definition.ID != testCase.ExpectedSkill {
				t.Fatalf("skill=%q, want %q", definition.ID, testCase.ExpectedSkill)
			}
		})
	}
}

func TestProductionEvaluationGatePasses(t *testing.T) {
	cases, err := ProductionEvaluationCases()
	if err != nil {
		t.Fatal(err)
	}
	report := Evaluate(ProductionRegistry(), cases)
	if report.Total != len(cases) || report.Failed != 0 {
		t.Fatalf("evaluation report=%+v\n%s", report, report.Markdown())
	}
}

func TestEvaluationDetectsHighRiskToolLeak(t *testing.T) {
	report := Evaluate(ProductionRegistry(), []EvaluationCase{{
		ID: "leak", Category: "test", Message: "生成审计包", Role: "auditor",
		ExpectedSkill: "audit_pack", ForbiddenTools: []string{"lease.contract.get"},
	}})
	if report.Failed != 1 || report.Results[0].Passed {
		t.Fatalf("report=%+v", report)
	}
}
