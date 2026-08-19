package workingpaper

import (
	"testing"
	"time"
)

type memAudits map[string]bool

func (m memAudits) CompletedToolCall(callID string) bool { return m[callID] }

// clonePaper deep-copies sections and cells so tests can mutate one case
// without leaking into the shared sample.
func clonePaper(p Paper) Paper {
	secs := make([]Section, len(p.Sections))
	for i, sec := range p.Sections {
		secs[i] = sec
		secs[i].Cells = append([]Cell(nil), sec.Cells...)
	}
	p.Sections = secs
	p.DataGaps = append([]string(nil), p.DataGaps...)
	return p
}

func samplePaper() Paper {
	p := Paper{
		Title:            "S1 签约前决策底稿",
		Period:           "2026-08",
		LegalEntityScope: "LE-1",
		ReviewState:      ReviewDraft,
		Sections: []Section{
			{
				ID:    "overview",
				Title: "概览",
				Kind:  KindTable,
				Cells: []Cell{
					{
						Ref: "A1", Label: "年租金", Value: 1200000, Currency: "CNY",
						Provenance: Provenance{Basis: BasisSystemFact, SourceTable: "contracts", SourceRecordID: "C-1"},
					},
					{
						Ref: "A2", Label: "租赁负债", MeasureID: "lease_liability", Value: 3255676.79, Currency: "CNY",
						Provenance: Provenance{Basis: BasisCertified, ToolCallID: "call-1", EngineVersion: "ifrs16@v2"},
					},
					{
						Ref: "A3", Label: "客流量预测", Value: "约 3% 增长",
						Provenance: Provenance{Basis: BasisExploratory, SandboxRunID: "run-1", CodeHash: "abc", ImageDigest: "img:1"},
					},
					{
						Ref: "A4", Label: "折现率（人工确认）", Value: 0.05,
						Provenance: Provenance{Basis: BasisHumanInput, ConfirmedBy: "bp-zhang", ConfirmedAt: "2026-08-01T10:00:00Z"},
					},
				},
			},
		},
		DataGaps: []string{"门店 2 客流缺失"},
	}
	return Build(p, time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC))
}

func TestBuildComputesCover(t *testing.T) {
	p := samplePaper()
	c := p.Cover
	if c.CertifiedCount != 1 || c.ExploratoryCount != 1 || c.SystemFactCount != 1 || c.HumanInputCount != 1 {
		t.Fatalf("cover counts wrong: %+v", c)
	}
	if c.DataGapCount != 1 || c.ReviewState != ReviewDraft {
		t.Fatalf("cover fields wrong: %+v", c)
	}
	if c.GeneratedAt == "" {
		t.Fatal("cover must carry generation time")
	}
}

func TestLintCleanPaperPasses(t *testing.T) {
	p := samplePaper()
	rep := Lint(p, memAudits{"call-1": true})
	if !rep.OK {
		t.Fatalf("clean paper must pass lint, got %+v", rep.Violations)
	}
}

// PROV-1 (I1): a cell without provenance must fail the lint, and nothing may
// be rendered.
func TestLintRejectsMissingProvenance(t *testing.T) {
	p := clonePaper(samplePaper())
	p.Sections[0].Cells[0].Provenance = Provenance{}
	rep := Lint(p, nil)
	if rep.OK {
		t.Fatal("missing provenance must fail lint")
	}
	if !hasViolation(rep, "provenance_missing", "A1") {
		t.Fatalf("expected provenance_missing on A1, got %+v", rep.Violations)
	}
}

func TestLintRejectsInvalidBasis(t *testing.T) {
	p := clonePaper(samplePaper())
	p.Sections[0].Cells[0].Provenance = Provenance{Basis: Basis("AI")}
	rep := Lint(p, nil)
	if rep.OK || !hasViolation(rep, "provenance_invalid_basis", "A1") {
		t.Fatalf("invalid basis must fail lint, got %+v", rep.Violations)
	}
}

// PROV-2 (I2): certified cells must trace to a completed tool call.
func TestLintCertifiedToolCallCrossCheck(t *testing.T) {
	// Missing tool_call_id.
	broken := clonePaper(samplePaper())
	broken.Sections[0].Cells[1].Provenance = Provenance{Basis: BasisCertified, EngineVersion: "ifrs16@v2"}
	if rep := Lint(broken, memAudits{}); rep.OK || !hasViolation(rep, "certified_missing_tool_call", "A2") {
		t.Fatalf("certified cell without call id must fail, got %+v", rep.Violations)
	}
	// Unknown tool call.
	p := clonePaper(samplePaper())
	if rep := Lint(p, memAudits{"other": true}); rep.OK || !hasViolation(rep, "certified_tool_call_unverified", "A2") {
		t.Fatalf("certified cell with unknown call id must fail, got %+v", rep.Violations)
	}
	// Known completed call passes.
	if rep := Lint(p, memAudits{"call-1": true}); !rep.OK {
		t.Fatalf("known completed call must pass, got %+v", rep.Violations)
	}
}

// I3: a protected measure must never carry an exploratory basis.
func TestLintRejectsProtectedExploratory(t *testing.T) {
	p := clonePaper(samplePaper())
	p.Sections[0].Cells[1].Provenance = Provenance{Basis: BasisExploratory, SandboxRunID: "run-9"}
	rep := Lint(p, nil)
	if rep.OK || !hasViolation(rep, "protected_measure_exploratory", "A2") {
		t.Fatalf("protected exploratory must fail lint, got %+v", rep.Violations)
	}
}

// Lexical probe fallback: no measure_id but the label mentions a protected
// measure.
func TestLintRejectsLexicalProbeExploratory(t *testing.T) {
	p := clonePaper(samplePaper())
	p.Sections[0].Cells = append(p.Sections[0].Cells, Cell{
		Ref: "B1", Label: "使用权资产余额", Value: 100,
		Provenance: Provenance{Basis: BasisExploratory, SandboxRunID: "run-9"},
	})
	p = Build(p, time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC))
	rep := Lint(p, nil)
	if rep.OK || !hasViolation(rep, "lexical_probe_exploratory", "B1") {
		t.Fatalf("lexical probe must catch unlabelled exploratory cell, got %+v", rep.Violations)
	}
}

// PROV-6 (I6): tampering with the cover statistics must fail the lint.
func TestLintRejectsCoverMismatch(t *testing.T) {
	p := clonePaper(samplePaper())
	p.Cover.CertifiedCount++
	rep := Lint(p, nil)
	if rep.OK || !hasViolation(rep, "cover_mismatch", "") {
		t.Fatalf("tampered cover must fail lint, got %+v", rep.Violations)
	}
}

func TestExploratoryRefs(t *testing.T) {
	p := samplePaper()
	refs := p.ExploratoryRefs()
	if len(refs) != 1 || refs[0] != "A3" {
		t.Fatalf("ExploratoryRefs must list A3, got %v", refs)
	}
}

func hasViolation(rep LintReport, code, ref string) bool {
	for _, v := range rep.Violations {
		if v.Code == code && v.CellRef == ref {
			return true
		}
	}
	return false
}
