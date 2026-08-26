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
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools/tools"
	"github.com/lease-management-system/core-service/internal/services/leasescenario"
	"github.com/lease-management-system/core-service/internal/workingpaper"
	finpaper "github.com/lease-management-system/core-service/internal/workingpaper/finmodel"
	retailpaper "github.com/lease-management-system/core-service/internal/workingpaper/retail"
	s1 "github.com/lease-management-system/core-service/internal/workingpaper/s1"
)

//go:embed testdata/agent-invariants.v1.json
var casesJSON []byte

// InvariantCase is one deterministic L1 case.
type InvariantCase struct {
	ID          string `json:"id"`
	Category    string `json:"category"` // provenance | triage_refusal | s1_engine_consistency
	Description string `json:"description,omitempty"`

	// provenance category
	WorkingPaper       *workingpaper.Paper `json:"working_paper,omitempty"`
	CompletedToolCalls []string            `json:"completed_tool_calls,omitempty"`
	ExpectedViolations []string            `json:"expected_violations,omitempty"` // violation codes, order-insensitive; absent means "must pass"

	// triage_refusal category
	Triage           *TriageInput `json:"triage,omitempty"`
	ExpectedDocClass string       `json:"expected_doc_class,omitempty"`

	// s1_engine_consistency category (CORR-1 deterministic half): the paper
	// builder's cells must equal direct engine outputs.
	S1Input *s1.Input `json:"s1_input,omitempty"`

	// retail_paper category: the retail paper builder must preserve engine
	// values 1:1, skip nil values, name its gaps, pass the lint and carry no
	// exploratory cells.
	RetailPaper *retailpaper.Input `json:"retail_paper,omitempty"`

	// finmodel_paper category: the model working paper passes values 1:1,
	// skips nil lines, flags failed tie-outs, passes the lint with its
	// anchored call and carries zero exploratory cells.
	FinPaper *finpaper.Input `json:"finmodel_paper,omitempty"`
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
		case "s1_engine_consistency":
			result.Passed, result.Detail = evaluateS1Consistency(c)
		case "retail_paper":
			result.Passed, result.Detail = evaluateRetailPaper(c)
		case "finmodel_paper":
			result.Passed, result.Detail = evaluateFinModelPaper(c)
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

// evaluateS1Consistency is CORR-1's deterministic half: every engine-derived
// paper cell must equal the engine's direct output, the paper must pass the
// fail-closed lint with its own audited call, and no cell may be exploratory.
func evaluateS1Consistency(c InvariantCase) (bool, string) {
	if c.S1Input == nil {
		return false, "s1_input is required for s1_engine_consistency cases"
	}
	in := *c.S1Input
	if in.ToolCallID == "" {
		in.ToolCallID = "eval-s1-call"
	}
	paper, err := s1.Build(in)
	if err != nil {
		return false, fmt.Sprintf("s1.Build failed: %v", err)
	}
	built := workingpaper.Build(paper, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	rep := workingpaper.Lint(built, makeAuditSet([]string{in.ToolCallID}))
	if !rep.OK {
		return false, fmt.Sprintf("paper failed lint: %+v", rep.Violations)
	}
	if refs := built.ExploratoryRefs(); len(refs) != 0 {
		return false, fmt.Sprintf("S1 paper must have no exploratory cells, got %v", refs)
	}

	direct, err := leasescenario.Build(in.Draft)
	if err != nil {
		return false, fmt.Sprintf("direct predeal build failed: %v", err)
	}
	byRef := map[string]workingpaper.Cell{}
	for _, cc := range built.AllCells() {
		byRef[cc.Ref] = cc
	}
	checks := []struct {
		ref  string
		want any
	}{
		{"IF-1", direct.BalanceSheet.InitialLiability},
		{"IF-2", direct.BalanceSheet.InitialROU},
		{"IF-3", direct.DiscountRate},
	}
	for _, chk := range checks {
		got, ok := byRef[chk.ref]
		if !ok {
			return false, fmt.Sprintf("missing cell %s", chk.ref)
		}
		if got.Value != chk.want {
			return false, fmt.Sprintf("cell %s = %v, engine says %v", chk.ref, got.Value, chk.want)
		}
	}
	for _, y := range direct.Yearly {
		got, ok := byRef["IF-"+fmt.Sprint(y.Year)+"-interest"]
		if !ok || got.Value != y.Interest {
			return false, fmt.Sprintf("yearly interest cell for year %d diverges: paper=%v engine=%v", y.Year, got.Value, y.Interest)
		}
	}
	if len(in.Offers) >= 2 {
		comparison, err := leasescenario.Compare(leasescenario.CompareInput{DiscountRate: in.Draft.DiscountRate, Currency: in.Draft.Currency, Offers: in.Offers})
		if err != nil {
			return false, fmt.Sprintf("direct compare failed: %v", err)
		}
		for _, o := range comparison.Offers {
			got, ok := byRef["DC-"+o.Name+"-pv"]
			if !ok || got.Value != o.PresentValue {
				return false, fmt.Sprintf("compare cell for %s diverges: paper=%v engine=%v", o.Name, got.Value, o.PresentValue)
			}
		}
	}
	for _, shock := range in.ShocksPercent {
		variant := in.Draft
		variant.DiscountRate = in.Draft.DiscountRate * (1 + shock)
		shocked, err := leasescenario.Build(variant)
		if err != nil {
			return false, fmt.Sprintf("shock build failed: %v", err)
		}
		got, ok := byRef["SE-"+fmt.Sprint(shock)+"-liability"]
		if !ok || got.Value != shocked.BalanceSheet.InitialLiability {
			return false, fmt.Sprintf("shock %v cell diverges: paper=%v engine=%v", shock, got.Value, shocked.BalanceSheet.InitialLiability)
		}
	}
	return true, ""
}

// evaluateRetailPaper runs the retail paper builder on a fixture and asserts
// the retail-paper sanctity rules: engine values preserved verbatim, nil
// values produce no cells, honesty gaps are named, the fail-closed lint
// passes with the anchored audited call, and zero exploratory cells exist.
func evaluateRetailPaper(c InvariantCase) (bool, string) {
	if c.RetailPaper == nil {
		return false, "retail_paper is required for retail_paper cases"
	}
	in := *c.RetailPaper
	if in.ToolCallID == "" {
		in.ToolCallID = "eval-retail-call"
	}
	paper, err := retailpaper.Build(in)
	if err != nil {
		return false, fmt.Sprintf("retail paper build failed: %v", err)
	}
	built := workingpaper.Build(paper, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	rep := workingpaper.Lint(built, makeAuditSet([]string{in.ToolCallID}))
	if !rep.OK {
		return false, fmt.Sprintf("retail paper failed lint: %+v", rep.Violations)
	}
	if refs := built.ExploratoryRefs(); len(refs) != 0 {
		return false, fmt.Sprintf("retail paper must have no exploratory cells, got %v", refs)
	}
	byRef := map[string]workingpaper.Cell{}
	for _, cc := range built.AllCells() {
		if cc.Value == nil {
			return false, fmt.Sprintf("cell %s must never carry a nil value (missing must be skipped)", cc.Ref)
		}
		byRef[cc.Ref] = cc
	}
	// The deliberately nil KPI in the fixture must not appear as a cell.
	if _, present := byRef["P-footfall-current"]; present {
		return false, "nil KPI footfall must be skipped, not zero-filled"
	}
	// Engine value preserved verbatim.
	if got := byRef["P-revenue-current"].Value.(float64); got != 9876543.21 {
		return false, fmt.Sprintf("cell P-revenue-current = %v, engine says 9876543.21", got)
	}
	// Honesty gaps must be named.
	joined := ""
	for _, g := range built.DataGaps {
		joined += g + "|"
	}
	for _, want := range []string{"多币种", "模拟（SIMULATED）", "被抑制"} {
		if !strings.Contains(joined, want) {
			return false, fmt.Sprintf("gap %q missing; gaps=%v", want, built.DataGaps)
		}
	}
	return true, ""
}

// evaluateFinModelPaper runs the finmodel paper builder on a fixture: values
// preserved 1:1, nil lines skipped, tie-out failures flagged — and, under the
// P1-3 fail-closed contract (D-S5 second point), a fixture carrying a failed
// tie-out must be REFUSED by lint (tie_out_unpassed), never exported.
func evaluateFinModelPaper(c InvariantCase) (bool, string) {
	if c.FinPaper == nil {
		return false, "finmodel_paper is required for finmodel_paper cases"
	}
	in := *c.FinPaper
	if in.ToolCallID == "" {
		in.ToolCallID = "eval-finpaper-call"
	}
	paper, err := finpaper.Build(in)
	if err != nil {
		return false, fmt.Sprintf("finmodel paper build failed: %v", err)
	}
	built := workingpaper.Build(paper, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	rep := workingpaper.Lint(built, makeAuditSet([]string{in.ToolCallID}))
	if refs := built.ExploratoryRefs(); len(refs) != 0 {
		return false, fmt.Sprintf("finmodel paper must have no exploratory cells, got %v", refs)
	}
	byRef := map[string]workingpaper.Cell{}
	for _, cc := range built.AllCells() {
		if cc.Value == nil {
			return false, fmt.Sprintf("cell %s must never carry a nil value", cc.Ref)
		}
		byRef[cc.Ref] = cc
	}
	if got := byRef["rev@2026-01"].Value.(float64); got != 987654.32 {
		return false, fmt.Sprintf("cell rev@2026-01 = %v, run says 987654.32 (1:1 保值)", got)
	}
	if _, present := byRef["labor@2026-01"]; present {
		return false, "nil line labor@2026-01 must be skipped, never zero-filled"
	}
	flagged := false
	for _, sec := range built.Sections {
		if sec.ID == "tie_outs" && strings.Contains(sec.Narrative, "T1") {
			flagged = true
		}
	}
	if !flagged {
		return false, "failed tie-outs must be flagged in the check section"
	}
	// P1-3: 失败 run 的底稿必须被 lint 拒绝（fail-closed），不得导出。
	// 有一处 failed tie-out → lint 必须非 OK 且带 tie_out_unpassed。
	hasFailed := false
	lintRefused := false
	for _, out := range in.TieOuts {
		if out.Status == "failed" {
			hasFailed = true
		}
	}
	for _, v := range rep.Violations {
		if v.Code == "tie_out_unpassed" {
			lintRefused = true
		}
	}
	if hasFailed {
		if rep.OK || !lintRefused {
			return false, "a failed-tie-out run's paper must be refused by lint (tie_out_unpassed, D-S5 fail-closed)"
		}
		return true, ""
	}
	if !rep.OK {
		return false, fmt.Sprintf("an all-green run's paper must pass lint, got %+v", rep.Violations)
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
