package agentskill

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// EvaluationCase is a deterministic control case. It does not pretend to
// measure LLM prose quality; it protects the server-owned routing, role,
// Review Gate and high-risk refusal invariants that must not regress when a
// model, Prompt or Skill changes.
type EvaluationCase struct {
	ID                  string   `json:"id"`
	Category            string   `json:"category"`
	Message             string   `json:"message"`
	Role                string   `json:"role"`
	RequestedSkill      string   `json:"requested_skill,omitempty"`
	ExpectedSkill       string   `json:"expected_skill,omitempty"`
	ExpectedBlocking    []string `json:"expected_blocking,omitempty"`
	ForbiddenTools      []string `json:"forbidden_tools,omitempty"`
	ExpectedNoSelection bool     `json:"expected_no_selection,omitempty"`
}

type EvaluationResult struct {
	ID            string `json:"id"`
	Category      string `json:"category"`
	Passed        bool   `json:"passed"`
	ExpectedSkill string `json:"expected_skill,omitempty"`
	ActualSkill   string `json:"actual_skill,omitempty"`
	FailureReason string `json:"failure_reason,omitempty"`
}

type EvaluationReport struct {
	Version string             `json:"version"`
	Total   int                `json:"total"`
	Passed  int                `json:"passed"`
	Failed  int                `json:"failed"`
	Results []EvaluationResult `json:"results"`
}

// ContractEvaluationReport is the CI-facing result for the version-pinned
// Skill contract. Keeping it separate from intent evaluation makes a routing
// improvement unable to mask a replay compatibility regression.
type ContractEvaluationReport struct {
	Version string `json:"version"`
	Passed  bool   `json:"passed"`
	Error   string `json:"error,omitempty"`
}

func EvaluateProductionSkillContract() ContractEvaluationReport {
	report := ContractEvaluationReport{Version: "skill-contract-replay.v1"}
	fixture, err := ProductionSkillContractFixture()
	if err != nil {
		report.Error = err.Error()
		return report
	}
	if err := ValidateSkillContractFixture(ProductionRegistry(), fixture); err != nil {
		report.Error = err.Error()
		return report
	}
	report.Passed = true
	return report
}

//go:embed testdata/agent-evaluation.v1.json
var productionEvaluationData []byte

func ProductionEvaluationCases() ([]EvaluationCase, error) {
	return decodeEvaluationCases(productionEvaluationData)
}

func decodeEvaluationCases(raw []byte) ([]EvaluationCase, error) {
	var cases []EvaluationCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		return nil, fmt.Errorf("decode agent evaluation cases: %w", err)
	}
	for index, item := range cases {
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Category) == "" {
			return nil, fmt.Errorf("evaluation case %d requires id and category", index)
		}
	}
	return cases, nil
}

func Evaluate(registry *Registry, cases []EvaluationCase) EvaluationReport {
	report := EvaluationReport{Version: "agent-evaluation.v1", Results: make([]EvaluationResult, 0, len(cases))}
	for _, testCase := range cases {
		result := EvaluationResult{ID: testCase.ID, Category: testCase.Category, ExpectedSkill: testCase.ExpectedSkill}
		definition, selected := registry.Select(Intent{
			Message: testCase.Message, Role: testCase.Role,
			RequestedSkillID: testCase.RequestedSkill,
		})
		if selected {
			result.ActualSkill = definition.ID
		}
		result.Passed, result.FailureReason = evaluationCasePasses(testCase, definition, selected)
		if result.Passed {
			report.Passed++
		} else {
			report.Failed++
		}
		report.Results = append(report.Results, result)
	}
	report.Total = len(report.Results)
	return report
}

func evaluationCasePasses(testCase EvaluationCase, definition Definition, selected bool) (bool, string) {
	expected := strings.TrimSpace(testCase.ExpectedSkill)
	if testCase.ExpectedNoSelection || expected == "" {
		if selected {
			return false, "expected Skill selection to be refused"
		}
		return true, ""
	}
	if !selected || definition.ID != expected {
		return false, fmt.Sprintf("selected Skill=%q, want %q", definition.ID, expected)
	}
	blocking := make(map[string]struct{}, len(definition.Review.Blocking))
	for _, reason := range definition.Review.Blocking {
		blocking[strings.ToLower(strings.TrimSpace(reason))] = struct{}{}
	}
	for _, reason := range testCase.ExpectedBlocking {
		if _, ok := blocking[strings.ToLower(strings.TrimSpace(reason))]; !ok {
			return false, fmt.Sprintf("Skill %q lacks Review blocker %q", definition.ID, reason)
		}
	}
	allowed := make(map[string]struct{}, len(definition.AllowedTools))
	for _, tool := range definition.AllowedTools {
		allowed[strings.ToLower(strings.TrimSpace(tool))] = struct{}{}
	}
	for _, forbidden := range testCase.ForbiddenTools {
		if _, ok := allowed[strings.ToLower(strings.TrimSpace(forbidden))]; ok {
			return false, fmt.Sprintf("Skill %q exposes forbidden Tool %q", definition.ID, forbidden)
		}
	}
	return true, ""
}

func (r EvaluationReport) SortedResults() []EvaluationResult {
	results := append([]EvaluationResult(nil), r.Results...)
	sort.Slice(results, func(i, j int) bool { return results[i].ID < results[j].ID })
	return results
}

func (r EvaluationReport) Markdown() string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "# Agent Evaluation %s\n\n", r.Version)
	fmt.Fprintf(&builder, "- Total: %d\n- Passed: %d\n- Failed: %d\n\n", r.Total, r.Passed, r.Failed)
	builder.WriteString("| Case | Category | Expected Skill | Actual Skill | Result | Failure |\n|---|---|---|---|---|---|\n")
	for _, item := range r.SortedResults() {
		status := "PASS"
		if !item.Passed {
			status = "FAIL"
		}
		fmt.Fprintf(&builder, "| %s | %s | %s | %s | %s | %s |\n", item.ID, item.Category, item.ExpectedSkill, item.ActualSkill, status, item.FailureReason)
	}
	return builder.String()
}
