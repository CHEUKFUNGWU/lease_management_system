package sourceenvelope

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

func day(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func fact(storeID string, date time.Time, version int, classification string) retailkpi.DailyFact {
	simulated := classification == "simulated"
	var dataset *string
	if simulated {
		value := "seed-v1"
		dataset = &value
	}
	return retailkpi.DailyFact{
		StoreID: storeID, BusinessDate: date, AsOfAt: date.Add(12 * time.Hour),
		Currency: "CNY", SourceSystem: "pos", DataClassification: classification,
		SimulationDatasetVersion: dataset, Version: version,
	}
}

// P1: the coverage rate stays null when expected store-days are unknown —
// a missing signal is never zero-filled (AGENTS.md: 不用 0 填补缺失).
func TestBuildCoverageRateIsNullWhenExpectedUnknown(t *testing.T) {
	env := Build([]retailkpi.DailyFact{fact("s1", day(2026, 1, 1), 1, "production")}, Spec{
		Classification: "production",
		Current:        PeriodSpec{From: day(2026, 1, 1), To: day(2026, 1, 7)}, // ExpectedStoreDays 0
	})
	if env.CurrentCoverage.CoverageRate != nil {
		t.Fatalf("rate must be null when expected store-days are unknown, got %v", *env.CurrentCoverage.CoverageRate)
	}
	if env.CurrentCoverage.ObservedStoreDays != 1 {
		t.Fatalf("observed store-days = %d, want 1", env.CurrentCoverage.ObservedStoreDays)
	}
}

func TestBuildCoverageRateIsComputedWhenExpectedKnown(t *testing.T) {
	facts := []retailkpi.DailyFact{
		fact("s1", day(2026, 1, 1), 1, "production"),
		fact("s1", day(2026, 1, 2), 1, "production"),
	}
	env := Build(facts, Spec{
		Classification: "production",
		Current:        PeriodSpec{From: day(2026, 1, 1), To: day(2026, 1, 7), ExpectedStoreDays: 7},
	})
	if env.CurrentCoverage.CoverageRate == nil || *env.CurrentCoverage.CoverageRate != float64(2)/float64(7)*100 {
		t.Fatalf("rate = %v, want 2/7*100", env.CurrentCoverage.CoverageRate)
	}
}

// CONTEXT.md: a read spanning both production and simulated facts reports
// `mixed` at the envelope level.
func TestBuildClassificationMixed(t *testing.T) {
	env := Build([]retailkpi.DailyFact{
		fact("s1", day(2026, 1, 1), 1, "production"),
		fact("s2", day(2026, 1, 1), 1, "simulated"),
	}, Spec{Classification: "production"})
	if env.DataClassification != "mixed" {
		t.Fatalf("classification = %q, want mixed", env.DataClassification)
	}
}

func TestBuildProvenanceRollup(t *testing.T) {
	simulated := "seed-b"
	facts := []retailkpi.DailyFact{
		{StoreID: "s1", BusinessDate: day(2026, 1, 1), AsOfAt: day(2026, 1, 1).Add(8 * time.Hour), SourceSystem: "pos", DataClassification: "simulated", SimulationDatasetVersion: &simulated, Version: 2},
		{StoreID: "s1", BusinessDate: day(2026, 1, 2), AsOfAt: day(2026, 1, 2).Add(9 * time.Hour), SourceSystem: "retail_simulator", DataClassification: "simulated", SimulationDatasetVersion: &simulated, Version: 1},
	}
	env := Build(facts, Spec{
		Classification: "simulated",
		FormulaVersion: "retail-kpi-v1",
		PulseVersion:   "retail-pulse-v1",
		GeneratedAt:    day(2026, 1, 3),
	})
	if env.FactVersionMin != 1 || env.FactVersionMax != 2 {
		t.Fatalf("version range = %d-%d, want 1-2", env.FactVersionMin, env.FactVersionMax)
	}
	if len(env.SourceSystems) != 2 || env.SourceSystems[0] != "pos" || env.SourceSystems[1] != "retail_simulator" {
		t.Fatalf("source systems = %v", env.SourceSystems)
	}
	if len(env.DatasetVersions) != 1 || env.DatasetVersions[0] != "seed-b" {
		t.Fatalf("dataset versions = %v", env.DatasetVersions)
	}
	if env.HighestAsOf == nil || !env.HighestAsOf.Equal(day(2026, 1, 2).Add(9*time.Hour)) {
		t.Fatalf("highest as-of = %v", env.HighestAsOf)
	}
	if env.SemanticVersion != SemanticVersion || env.FormulaVersion != "retail-kpi-v1" || env.PulseVersion != "retail-pulse-v1" {
		t.Fatalf("versions = %+v", env)
	}
}

func TestBuildZeroComparisonPeriodYieldsZeroCoverage(t *testing.T) {
	env := Build([]retailkpi.DailyFact{fact("s1", day(2026, 1, 1), 1, "production")}, Spec{
		Classification: "production",
		Current:        PeriodSpec{From: day(2026, 1, 1), To: day(2026, 1, 7), ExpectedStoreDays: 7},
	})
	if env.ComparisonCoverage.CoverageRate != nil || env.ComparisonCoverage.ExpectedStoreDays != 0 {
		t.Fatalf("comparison coverage = %+v, want zero", env.ComparisonCoverage)
	}
}

// The envelope JSON shape is the one contract the frontend and the agent
// consume; pin the exact key set so an accidental rename breaks here rather
// than silently on a page.
func TestEnvelopeJSONShapeIsStable(t *testing.T) {
	env := Build([]retailkpi.DailyFact{fact("s1", day(2026, 1, 1), 1, "production")}, Spec{
		Classification: "production",
		Current:        PeriodSpec{From: day(2026, 1, 1), To: day(2026, 1, 7), ExpectedStoreDays: 7},
		DecisionReady:  true, DecisionReadyReason: "ready",
		GeneratedAt: day(2026, 1, 8),
	})
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatal(err)
	}
	want := []string{"data_classification", "source_systems", "dataset_versions", "fact_version_min", "fact_version_max", "highest_as_of", "current_coverage", "comparison_coverage", "decision_ready", "decision_ready_reason", "formula_version", "pulse_version", "semantic_version", "generated_at"}
	if len(object) != len(want) {
		t.Fatalf("envelope keys = %d, want %d", len(object), len(want))
	}
	for _, key := range want {
		if _, ok := object[key]; !ok {
			t.Fatalf("envelope missing key %q in %v", key, object)
		}
	}
}
