// Package sourceenvelope is the single producer of the Source Envelope
// (CONTEXT.md, Retail Operations): the provenance carried by every retail
// read — Data Classification, source systems, dataset versions, Fact Version
// range, as-of, Fact Coverage, Decision Ready and its reason, and the formula
// / pulse / semantic versions in force.
//
// Before this module existed, every read path hand-rolled its own envelope
// (source/dataset dedup, version range, as-of, coverage) in different shapes.
// All retail reads now call Build and return the Envelope as-is; the shape
// below is the one contract the frontend and the agent consume.
package sourceenvelope

import (
	"sort"
	"time"

	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

// SemanticVersion is the version of the envelope contract itself. It is
// bumped only when the shape or semantics of the envelope change, and is
// read from here — never hardcoded at a call site.
const SemanticVersion = "retail-envelope-v1"

// PeriodSpec describes one evaluation period of the read. The zero value
// means "no such period" and yields an all-zero coverage (rate nil).
type PeriodSpec struct {
	From, To          time.Time
	ExpectedStoreDays int
}

// Spec carries the parts of the envelope that are not derivable from the
// fact slice alone: the requested classification and versions, the expected
// store-day counts per period, and the caller's decision-ready verdict.
type Spec struct {
	Classification      string
	FormulaVersion      string
	PulseVersion        string
	Current             PeriodSpec
	Comparison          PeriodSpec
	DecisionReady       bool
	DecisionReadyReason string
	GeneratedAt         time.Time
}

// Envelope is the canonical provenance shape returned by every retail read.
// The same struct is embedded (as `envelope`) in the pulse, store-360,
// scenario, KPI, store-day facts and agent tool responses.
type Envelope struct {
	DataClassification  string             `json:"data_classification"`
	SourceSystems       []string           `json:"source_systems"`
	DatasetVersions     []string           `json:"dataset_versions"`
	FactVersionMin      int                `json:"fact_version_min"`
	FactVersionMax      int                `json:"fact_version_max"`
	HighestAsOf         *time.Time         `json:"highest_as_of,omitempty"`
	CurrentCoverage     retailkpi.Coverage `json:"current_coverage"`
	ComparisonCoverage  retailkpi.Coverage `json:"comparison_coverage"`
	DecisionReady       bool               `json:"decision_ready"`
	DecisionReadyReason string             `json:"decision_ready_reason,omitempty"`
	FormulaVersion      string             `json:"formula_version"`
	PulseVersion        string             `json:"pulse_version"`
	SemanticVersion     string             `json:"semantic_version"`
	GeneratedAt         time.Time          `json:"generated_at"`
}

// Build is the single envelope producer. It derives everything it can from
// the fact slice (classification, sources, dataset versions, version range,
// as-of, per-period coverage) and takes the rest from the spec.
func Build(facts []retailkpi.DailyFact, spec Spec) Envelope {
	versionMin, versionMax := FactVersionRange(facts)
	env := Envelope{
		DataClassification:  classify(facts, spec.Classification),
		SourceSystems:       SourceSystems(facts),
		DatasetVersions:     DatasetVersions(facts),
		FactVersionMin:      versionMin,
		FactVersionMax:      versionMax,
		CurrentCoverage:     coverageFor(facts, spec.Current),
		ComparisonCoverage:  coverageFor(facts, spec.Comparison),
		DecisionReady:       spec.DecisionReady,
		DecisionReadyReason: spec.DecisionReadyReason,
		FormulaVersion:      spec.FormulaVersion,
		PulseVersion:        spec.PulseVersion,
		SemanticVersion:     SemanticVersion,
		GeneratedAt:         spec.GeneratedAt,
	}
	if highest := HighestAsOf(facts); !highest.IsZero() {
		env.HighestAsOf = &highest
	}
	return env
}

// classify reports the single classification of the facts, `mixed` when the
// read spans both production and simulated facts, and the requested
// classification when no fact carries one (CONTEXT.md: Data Classification).
func classify(facts []retailkpi.DailyFact, fallback string) string {
	seen := map[string]bool{}
	for _, fact := range facts {
		if fact.DataClassification != "" {
			seen[fact.DataClassification] = true
		}
	}
	switch len(seen) {
	case 0:
		return fallback
	case 1:
		for classification := range seen {
			return classification
		}
	}
	return "mixed"
}

// FactVersionRange returns the min/max Fact Version of the facts.
func FactVersionRange(facts []retailkpi.DailyFact) (min, max int) {
	for _, fact := range facts {
		if fact.Version > 0 && (min == 0 || fact.Version < min) {
			min = fact.Version
		}
		if fact.Version > max {
			max = fact.Version
		}
	}
	return min, max
}

// HighestAsOf returns the newest as-of timestamp of the facts.
func HighestAsOf(facts []retailkpi.DailyFact) time.Time {
	var highest time.Time
	for _, fact := range facts {
		if fact.AsOfAt.After(highest) {
			highest = fact.AsOfAt
		}
	}
	return highest
}

// SourceSystems returns the distinct source systems of the facts. Exported
// so fact-set rebuilds (e.g. the agent's scoped reader) reuse the same dedup
// instead of a fourth copy.
func SourceSystems(facts []retailkpi.DailyFact) []string {
	seen := map[string]bool{}
	for _, fact := range facts {
		if fact.SourceSystem != "" {
			seen[fact.SourceSystem] = true
		}
	}
	return sortedKeys(seen)
}

// DatasetVersions returns the distinct simulated dataset versions of the
// facts, in sorted order.
func DatasetVersions(facts []retailkpi.DailyFact) []string {
	seen := map[string]bool{}
	for _, fact := range facts {
		if fact.SimulationDatasetVersion != nil && *fact.SimulationDatasetVersion != "" {
			seen[*fact.SimulationDatasetVersion] = true
		}
	}
	return sortedKeys(seen)
}

func sortedKeys(seen map[string]bool) []string {
	result := make([]string, 0, len(seen))
	for key := range seen {
		result = append(result, key)
	}
	sort.Strings(result)
	return result
}

// coverageFor builds the Fact Coverage for one period: observed store-days
// counted from the fact slice, expected from the spec. The rate is null
// whenever expected is unknown (ExpectedStoreDays == 0) — a missing signal
// is never zero-filled.
func coverageFor(facts []retailkpi.DailyFact, period PeriodSpec) retailkpi.Coverage {
	if period.From.IsZero() {
		return retailkpi.Coverage{}
	}
	coverage := retailkpi.Coverage{
		RequestedDateFrom: period.From.Format("2006-01-02"),
		RequestedDateTo:   period.To.Format("2006-01-02"),
		ExpectedStoreDays: period.ExpectedStoreDays,
	}
	for _, fact := range facts {
		if !fact.BusinessDate.Before(period.From) && !fact.BusinessDate.After(period.To) {
			coverage.ObservedStoreDays++
		}
	}
	if coverage.ExpectedStoreDays > 0 {
		value := float64(coverage.ObservedStoreDays) / float64(coverage.ExpectedStoreDays) * 100
		coverage.CoverageRate = &value
	}
	return coverage
}
