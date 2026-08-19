// Package retail builds the retail operating working paper from the three
// deterministic retail engines (pulse / store360 / scenario). It is a pure
// mapper: every numeric cell equals the engine response field verbatim, nil
// values are skipped rather than zero-filled, and every honesty signal
// (coverage gaps, suppressed attention, currency conflicts, simulation flags)
// becomes a named gap. No computation lives here — retail-kpi-v1 owns the
// math, the builder only carries it into provably-traced cells.
package retail

import (
	"errors"
	"strings"

	"github.com/lease-management-system/core-service/internal/services/retailpulse"
	"github.com/lease-management-system/core-service/internal/services/retailscenario"
	"github.com/lease-management-system/core-service/internal/services/retailstore360"
	"github.com/lease-management-system/core-service/internal/workingpaper"
)

// Engine version fallbacks for minimal fixtures that omit version fields.
const (
	fallbackPulseVersion    = "retail-pulse-v1"
	fallbackDiagVersion     = "retail-store-diagnostics-v1"
	fallbackScenarioVersion = "retail-store-scenario-v1"
)

// Input is everything the paper needs. Assumptions must be human-confirmed;
// ToolCallID anchors every certified cell to the audited paper-tool call (I2).
type Input struct {
	Pulse          *retailpulse.Response      `json:"pulse"`
	Diagnostics    *retailstore360.Response   `json:"diagnostics,omitempty"`
	Scenario       *retailscenario.Response   `json:"scenario,omitempty"`
	Assumptions    retailscenario.Assumptions `json:"assumptions,omitempty"`
	ConfirmedBy    string                     `json:"confirmed_by,omitempty"`
	ConfirmedAt    string                     `json:"confirmed_at,omitempty"`
	ToolCallID     string                     `json:"tool_call_id"`
	AttentionLimit int                        `json:"attention_limit,omitempty"`
}

// Build assembles the paper; it fails only on structural contract breaks,
// never on data gaps — those are recorded, not covered up.
func Build(in Input) (workingpaper.Paper, error) {
	if in.Pulse == nil {
		return workingpaper.Paper{}, errors.New("retail: pulse response is required")
	}
	if strings.TrimSpace(in.ToolCallID) == "" {
		return workingpaper.Paper{}, errors.New("retail: tool_call_id is required for certified provenance (I2)")
	}
	if in.Scenario != nil && strings.TrimSpace(in.ConfirmedBy) == "" {
		return workingpaper.Paper{}, errors.New("retail: scenario assumptions must be confirmed by a human (confirmed_by is empty)")
	}

	pulseEngine := in.Pulse.PulseVersion
	if pulseEngine == "" {
		pulseEngine = fallbackPulseVersion
	}
	diagEngine := fallbackDiagVersion
	if in.Diagnostics != nil && in.Diagnostics.DiagnosticsVersion != "" {
		diagEngine = in.Diagnostics.DiagnosticsVersion
	}
	scenarioEngine := fallbackScenarioVersion
	if in.Scenario != nil && in.Scenario.ScenarioVersion != "" {
		scenarioEngine = in.Scenario.ScenarioVersion
	}

	var gaps []string
	gaps = append(gaps, scopeGaps(in)...)
	gaps = append(gaps, pulseGaps(in)...)
	gaps = append(gaps, diagGaps(in)...)
	gaps = append(gaps, scenarioGaps(in)...)

	sections := []workingpaper.Section{
		scopeSection(in),
		pulseSection(in, pulseEngine),
	}
	if sssg := sssgSection(in, pulseEngine); sssg.ID != "" {
		sections = append(sections, sssg)
	}
	if att := attentionSection(in, pulseEngine); att.ID != "" {
		sections = append(sections, att)
	}
	if in.Diagnostics != nil {
		sections = append(sections, diagnosticsSection(in, diagEngine))
	}
	if in.Scenario != nil {
		sections = append(sections, scenarioSection(in, scenarioEngine))
		sections = append(sections, assumptionsSection(in))
	}

	period := ""
	if in.Pulse.PeriodLabel != "" {
		period = in.Pulse.PeriodLabel
	} else if in.Pulse.CurrentCoverage.RequestedDateFrom != "" {
		period = in.Pulse.CurrentCoverage.RequestedDateFrom + " ~ " + in.Pulse.CurrentCoverage.RequestedDateTo
	}

	legalEntity := ""
	if v, ok := in.Pulse.RequestedScope["legal_entity_id"].(string); ok {
		legalEntity = v
	}

	return workingpaper.Paper{
		Title:               "零售门店经营底稿",
		Period:              period,
		LegalEntityScope:    legalEntity,
		ReviewState:         workingpaper.ReviewNeedsReview,
		DataVersion:         in.Pulse.DatasetVersion,
		AssumptionVersion:   in.Pulse.DatasetVersion,
		GeneratedBy:         "retail.working_paper.store.generate",
		DataGaps:            gaps,
		UnexplainedResidual: residualSummary(in),
		OpenQuestions:       []string{"门店诊断与情景均需 store_id；组合级底稿不含这两节"},
		Sections:            sections,
	}, nil
}

const sourceFactsTable = "store_operating_facts"

func systemFact(in Input) workingpaper.Provenance {
	return workingpaper.Provenance{
		Basis:       workingpaper.BasisSystemFact,
		SourceTable: sourceFactsTable,
		DataVersion: in.Pulse.DatasetVersion,
	}
}

func certified(callID, engine string) workingpaper.Provenance {
	return workingpaper.Provenance{
		Basis:         workingpaper.BasisCertified,
		ToolCallID:    callID,
		EngineVersion: engine,
	}
}

func human(in Input) workingpaper.Provenance {
	return workingpaper.Provenance{
		Basis:       workingpaper.BasisHumanInput,
		ConfirmedBy: in.ConfirmedBy,
		ConfirmedAt: in.ConfirmedAt,
	}
}

// numCell emits a numeric cell only when the value is present — missing stays
// missing, never zero (AGENTS.md: 不用 0 填补缺失).
func numCell(ref, label, measureID string, value *float64, unit, currency string, p workingpaper.Provenance) (workingpaper.Cell, bool) {
	if value == nil {
		return workingpaper.Cell{}, false
	}
	return workingpaper.Cell{
		Ref: ref, Label: label, MeasureID: measureID,
		Value: *value, Unit: unit, Currency: currency, Provenance: p,
	}, true
}

func strCell(ref, label string, value string, p workingpaper.Provenance) workingpaper.Cell {
	return workingpaper.Cell{Ref: ref, Label: label, Value: value, Provenance: p}
}

// usableCurrency returns the ISO currency only when the engine asserts a
// single known currency; unknown/conflict must leave it empty.
func usableCurrency(iso, status string) string {
	if status == "unknown" || status == "conflict" || iso == "" {
		return ""
	}
	return iso
}
