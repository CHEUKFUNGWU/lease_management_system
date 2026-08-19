// Package workingpaper is the working-paper product layer: the cell-level
// provenance protocol, the fail-closed lint that enforces invariants I1/I2/I3/
// I6, the cover page, and the xlsx/docx renderers. Every number in a paper
// must be able to answer "who computed it" — that is the invariant the lint
// enforces before anything may be rendered or exported.
package workingpaper

import "time"

// Basis is the provenance class of a cell value. The four classes follow the
// requirements list §11: system fact, certified engine, exploratory (AI
// inference) and human input.
type Basis string

const (
	BasisSystemFact  Basis = "SystemFact"
	BasisCertified   Basis = "Certified"
	BasisExploratory Basis = "Exploratory"
	BasisHumanInput  Basis = "HumanInput"
)

// Valid reports whether the basis is one of the four classes.
func (b Basis) Valid() bool {
	switch b {
	case BasisSystemFact, BasisCertified, BasisExploratory, BasisHumanInput:
		return true
	}
	return false
}

// Provenance answers "who computed this cell". Exactly one class applies; the
// other fields are populated per class (Certified: tool call, engine version,
// input hash; Exploratory: sandbox run, code hash, image digest; HumanInput:
// confirmed by/at; SystemFact: source table/record/data version).
type Provenance struct {
	Basis          Basis  `json:"basis"`
	ToolCallID     string `json:"tool_call_id,omitempty"`
	EngineVersion  string `json:"engine_version,omitempty"`
	InputHash      string `json:"input_hash,omitempty"`
	SandboxRunID   string `json:"sandbox_run_id,omitempty"`
	CodeHash       string `json:"code_hash,omitempty"`
	ImageDigest    string `json:"image_digest,omitempty"`
	SourceTable    string `json:"source_table,omitempty"`
	SourceRecordID string `json:"source_record_id,omitempty"`
	DataVersion    string `json:"data_version,omitempty"`
	ConfirmedBy    string `json:"confirmed_by,omitempty"`
	ConfirmedAt    string `json:"confirmed_at,omitempty"`
}

// SectionKind is the rendering kind of a section.
type SectionKind string

const (
	KindTable          SectionKind = "table"
	KindNarrative      SectionKind = "narrative"
	KindChart          SectionKind = "chart"
	KindAssumptionList SectionKind = "assumption_list"
)

// Cell is one number or fact with its provenance.
type Cell struct {
	Ref        string     `json:"ref"`
	Label      string     `json:"label"`
	MeasureID  string     `json:"measure_id,omitempty"`
	Value      any        `json:"value"`
	Unit       string     `json:"unit,omitempty"`
	Currency   string     `json:"currency,omitempty"`
	Provenance Provenance `json:"provenance"`
}

// Section groups cells under one heading.
type Section struct {
	ID        string      `json:"id"`
	Title     string      `json:"title"`
	Kind      SectionKind `json:"kind"`
	Cells     []Cell      `json:"cells,omitempty"`
	Narrative string      `json:"narrative,omitempty"`
}

// ReviewState mirrors the artifact review lifecycle.
type ReviewState string

const (
	ReviewDraft       ReviewState = "draft"
	ReviewNeedsReview ReviewState = "needs_review"
	ReviewConfirmed   ReviewState = "confirmed"
)

// CoverPage is computed by Build — callers cannot hand-fill it (I6).
type CoverPage struct {
	GeneratedAt      string      `json:"generated_at"`
	CertifiedCount   int         `json:"certified_count"`
	ExploratoryCount int         `json:"exploratory_count"`
	SystemFactCount  int         `json:"system_fact_count"`
	HumanInputCount  int         `json:"human_input_count"`
	DataGapCount     int         `json:"data_gap_count"`
	ReviewState      ReviewState `json:"review_state"`
}

// Paper is a working paper: sections of provenance-carrying cells plus the
// honesty fields the cover page surfaces (data gaps, unexplained residual,
// open questions).
type Paper struct {
	Title               string      `json:"title"`
	Period              string      `json:"period"`
	LegalEntityScope    string      `json:"legal_entity_scope"`
	Sections            []Section   `json:"sections"`
	DataGaps            []string    `json:"data_gaps,omitempty"`
	UnexplainedResidual string      `json:"unexplained_residual,omitempty"`
	OpenQuestions       []string    `json:"open_questions,omitempty"`
	ReviewState         ReviewState `json:"review_state"`
	DataVersion         string      `json:"data_version,omitempty"`
	AssumptionVersion   string      `json:"assumption_version,omitempty"`
	EngineVersion       string      `json:"engine_version,omitempty"`
	SandboxImageDigest  string      `json:"sandbox_image_digest,omitempty"`
	GeneratedBy         string      `json:"generated_by,omitempty"`
	Cover               CoverPage   `json:"cover,omitempty"`
}

// Build normalizes the paper and computes its cover page. It is the only
// entry that produces a cover — callers cannot supply one, which is what
// makes I6 checkable.
func Build(p Paper, now time.Time) Paper {
	if p.ReviewState == "" {
		p.ReviewState = ReviewDraft
	}
	p.Cover = computeCover(p, now)
	return p
}

// AllCells flattens every cell of the paper in order.
func (p Paper) AllCells() []Cell {
	var out []Cell
	for _, sec := range p.Sections {
		out = append(out, sec.Cells...)
	}
	return out
}

// ExploratoryRefs returns the refs of every cell whose basis is Exploratory.
// Write paths must refuse to commit these values (invariant I5 / ACORE-12).
func (p Paper) ExploratoryRefs() []string {
	var out []string
	for _, c := range p.AllCells() {
		if c.Provenance.Basis == BasisExploratory {
			out = append(out, c.Ref)
		}
	}
	return out
}

func computeCover(p Paper, now time.Time) CoverPage {
	cov := CoverPage{
		GeneratedAt:  now.UTC().Format(time.RFC3339),
		DataGapCount: len(p.DataGaps),
		ReviewState:  p.ReviewState,
	}
	for _, c := range p.AllCells() {
		switch c.Provenance.Basis {
		case BasisCertified:
			cov.CertifiedCount++
		case BasisExploratory:
			cov.ExploratoryCount++
		case BasisSystemFact:
			cov.SystemFactCount++
		case BasisHumanInput:
			cov.HumanInputCount++
		}
	}
	return cov
}
