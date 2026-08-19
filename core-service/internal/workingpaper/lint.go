package workingpaper

import (
	"fmt"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools"
)

// Violation is one failed invariant with the offending cell reference.
type Violation struct {
	Code    string `json:"code"`
	CellRef string `json:"cell_ref,omitempty"`
	Detail  string `json:"detail"`
}

// LintReport is the fail-closed result: any violation means the paper must
// not be rendered or exported.
type LintReport struct {
	OK         bool        `json:"ok"`
	Violations []Violation `json:"violations,omitempty"`
}

// AuditLookup cross-checks certified cells against the run's tool audit
// records (invariant I2). Callers implement it over agent_tool_audit or the
// future agentcore ArtifactCollector.
type AuditLookup interface {
	CompletedToolCall(callID string) bool
}

// Lint enforces the invariants in order:
//
//	I1  every cell has a non-empty provenance with a valid basis
//	I2  certified cells carry a tool_call_id that exists in this run's audit
//	    records with completed status
//	I3  protected measures (by id or lexical probe) never carry an
//	    exploratory basis
//	I6  the cover statistics match the actual cell statistics
//
// Lint is the single gate every export path must pass. It never mutates the
// paper and never downgrades: violations are reported, not smoothed over.
func Lint(p Paper, audits AuditLookup) LintReport {
	rep := LintReport{}
	for _, sec := range p.Sections {
		for _, c := range sec.Cells {
			rep.Violations = append(rep.Violations, lintCell(c, audits)...)
		}
	}

	recomputed := computeCover(p, time.Time{})
	if p.Cover.CertifiedCount != recomputed.CertifiedCount ||
		p.Cover.ExploratoryCount != recomputed.ExploratoryCount ||
		p.Cover.SystemFactCount != recomputed.SystemFactCount ||
		p.Cover.HumanInputCount != recomputed.HumanInputCount ||
		p.Cover.DataGapCount != recomputed.DataGapCount ||
		p.Cover.ReviewState != recomputed.ReviewState {
		rep.Violations = append(rep.Violations, Violation{
			Code:   "cover_mismatch",
			Detail: fmt.Sprintf("cover stats differ from recomputed stats: cover=%+v recomputed=%+v", p.Cover, recomputed),
		})
	}

	rep.OK = len(rep.Violations) == 0
	return rep
}

func lintCell(c Cell, audits AuditLookup) []Violation {
	var out []Violation
	ref := c.Ref

	// I1 — provenance completeness.
	if c.Provenance.Basis == "" {
		out = append(out, Violation{Code: "provenance_missing", CellRef: ref, Detail: "cell has no provenance basis"})
		return out
	}
	if !c.Provenance.Basis.Valid() {
		out = append(out, Violation{Code: "provenance_invalid_basis", CellRef: ref, Detail: fmt.Sprintf("unknown basis %q", c.Provenance.Basis)})
		return out
	}

	// I2 — certified cells must trace to a completed tool call.
	if c.Provenance.Basis == BasisCertified {
		if c.Provenance.ToolCallID == "" {
			out = append(out, Violation{Code: "certified_missing_tool_call", CellRef: ref, Detail: "certified cell has no tool_call_id"})
		} else if audits != nil && !audits.CompletedToolCall(c.Provenance.ToolCallID) {
			out = append(out, Violation{Code: "certified_tool_call_unverified", CellRef: ref, Detail: fmt.Sprintf("tool_call_id %s not found completed in audit records", c.Provenance.ToolCallID)})
		}
	}

	// I3 + lexical probe fallback — protected measures stay certified.
	for _, code := range agenttools.LintCell(c.MeasureID, c.Label, string(c.Provenance.Basis)) {
		out = append(out, Violation{Code: code, CellRef: ref, Detail: fmt.Sprintf("label %q / measure_id %q must not be exploratory", c.Label, c.MeasureID)})
	}
	return out
}
