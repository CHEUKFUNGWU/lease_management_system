// Package agentseval holds the L1 invariant cases of the evaluation harness:
// working-paper lint (provenance invariants I1/I2/I3) and deterministic file
// triage. The cases live in testdata/agent-invariants.v1.json, embedded here
// so the harness and the dataset ship together; the evaluation is fully
// deterministic (no LLM calls), which is what makes it CI-safe.
package agentseval

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools/tools"
	"github.com/lease-management-system/core-service/internal/workingpaper"
)

//go:embed testdata/agent-invariants.v1.json
var casesJSON []byte

// InvariantCase is one deterministic L1 case.
type InvariantCase struct {
	ID          string `json:"id"`
	Category    string `json:"category"` // provenance | triage_refusal
	Description string `json:"description,omitempty"`

	// provenance category
	WorkingPaper       *workingpaper.Paper `json:"working_paper,omitempty"`
	CompletedToolCalls []string            `json:"completed_tool_calls,omitempty"`
	ExpectedViolations []string            `json:"expected_violations,omitempty"` // violation codes, order-insensitive; absent means "must pass"

	// triage_refusal category
	Triage           *TriageInput `json:"triage,omitempty"`
	ExpectedDocClass string       `json:"expected_doc_class,omitempty"`
}

// TriageInput is the deterministic triage request of a case.
type TriageInput struct {
	ObjectName  string `json:"object_name"`
	ContentType string `json:"content_type"`
	UserMessage string `json:"user_message"`
}

// InvariantResult is the outcome of one case.
type InvariantResult struct {
	CaseID   string `json:"case_id"`
	Category string `json:"category"`
	Passed   bool   `json:"passed"`
	Detail   string `json:"detail,omitempty"`
}

// InvariantReport aggregates the invariant cases.
type InvariantReport struct {
	Version string            `json:"version"`
	Total   int               `json:"total"`
	Passed  int               `json:"passed"`
	Failed  int               `json:"failed"`
	Results []InvariantResult `json:"results"`
}

// ProductionInvariantCases decodes the embedded dataset.
func ProductionInvariantCases() ([]InvariantCase, error) {
	var cases []InvariantCase
	if err := json.Unmarshal(casesJSON, &cases); err != nil {
		return nil, fmt.Errorf("agent-invariants.v1.json: %w", err)
	}
	return cases, nil
}

// EvaluateInvariantCases runs every case and reports pass/fail. It never
// mutates the inputs.
func EvaluateInvariantCases(cases []InvariantCase) InvariantReport {
	report := InvariantReport{Version: "agent-invariants.v1"}
	for _, c := range cases {
		result := InvariantResult{CaseID: c.ID, Category: c.Category, Passed: true}
		switch c.Category {
		case "provenance":
			result.Passed, result.Detail = evaluateProvenance(c)
		case "triage_refusal":
			result.Passed, result.Detail = evaluateTriage(c)
		default:
			result.Passed = false
			result.Detail = fmt.Sprintf("unknown category %q", c.Category)
		}
		report.Total++
		if result.Passed {
			report.Passed++
		} else {
			report.Failed++
		}
		report.Results = append(report.Results, result)
	}
	return report
}

func evaluateProvenance(c InvariantCase) (bool, string) {
	if c.WorkingPaper == nil {
		return false, "working_paper is required for provenance cases"
	}
	// Rebuild the cover from the case's cells — a hand-crafted cover in the
	// dataset cannot silently pass I6 because Build recomputes it.
	paper := workingpaper.Build(*c.WorkingPaper, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	audits := makeAuditSet(c.CompletedToolCalls)
	rep := workingpaper.Lint(paper, audits)

	got := make(map[string]bool, len(rep.Violations))
	for _, v := range rep.Violations {
		got[v.Code] = true
	}
	want := make(map[string]bool, len(c.ExpectedViolations))
	for _, code := range c.ExpectedViolations {
		want[code] = true
	}
	if len(got) != len(want) {
		return false, fmt.Sprintf("violation sets differ: got %v want %v", keys(got), keys(want))
	}
	for code := range want {
		if !got[code] {
			return false, fmt.Sprintf("missing expected violation %q", code)
		}
	}
	return true, ""
}

func evaluateTriage(c InvariantCase) (bool, string) {
	if c.Triage == nil {
		return false, "triage is required for triage_refusal cases"
	}
	result := tools.DeterministicTriage(tools.TriageRequest{
		ObjectName:  c.Triage.ObjectName,
		ContentType: c.Triage.ContentType,
		UserMessage: c.Triage.UserMessage,
	})
	if string(result.DocClass) != c.ExpectedDocClass {
		return false, fmt.Sprintf("triage got %s, want %s", result.DocClass, c.ExpectedDocClass)
	}
	return true, ""
}

type auditSet map[string]bool

func (a auditSet) CompletedToolCall(callID string) bool { return a[callID] }

func makeAuditSet(completed []string) auditSet {
	a := make(auditSet, len(completed))
	for _, id := range completed {
		a[id] = true
	}
	return a
}

func keys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
