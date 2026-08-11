package fpna

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/repository"
)

func TestHybridForecastReplacesElapsedPeriodsOnly(t *testing.T) {
	period := "2026-01"
	forecast := []*repository.FPnAPlanLine{{Period: period, Grain: "store", StoreID: stringPtr("s1"), Revenue: floatPtr(100), ForecastFlag: true}, {Period: "2026-03", Grain: "store", StoreID: stringPtr("s1"), Revenue: floatPtr(300), ForecastFlag: true}}
	actual := []*repository.FPnAPlanLine{{Period: period, Grain: "store", StoreID: stringPtr("s1"), Revenue: floatPtr(120), ActualFlag: true}}
	rows, err := HybridForecast(forecast, actual, period)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Revenue == nil || *rows[0].Revenue != 120 {
		t.Fatalf("unexpected hybrid rows: %#v", rows)
	}
	if !rows[0].ActualFlag || rows[0].ForecastFlag {
		t.Fatalf("elapsed row was not marked actual: %#v", rows[0])
	}
}

func TestComparePlanLinesKeepsCoverageAndTieOut(t *testing.T) {
	left := []*repository.FPnAPlanLine{{Period: "2026-01", Grain: "group", Revenue: floatPtr(100)}}
	right := []*repository.FPnAPlanLine{{Period: "2026-01", Grain: "group", Revenue: floatPtr(120)}}
	result := ComparePlanLines("2026-01", "Budget", "Actual", "CNY", "v1", left, right, 0.01)
	if result.Variance != 20 || !result.TiesOut || !result.Coverage.Complete {
		t.Fatalf("unexpected comparison: %#v", result)
	}
}

func TestComparePlanLinesMarksMissingSideIncomplete(t *testing.T) {
	left := []*repository.FPnAPlanLine{{Period: "2026-01", Grain: "store", StoreID: stringPtr("s1"), Revenue: floatPtr(100)}}
	right := []*repository.FPnAPlanLine{{Period: "2026-01", Grain: "store", StoreID: stringPtr("s2"), Revenue: floatPtr(120)}}
	result := ComparePlanLines("2026-01", "Budget", "Actual", "CNY", "v1", left, right, 0.01)
	if result.Coverage.Complete || result.Coverage.Observed != 0 || result.Coverage.Expected != 2 {
		t.Fatalf("missing side must remain visible in coverage: %#v", result.Coverage)
	}
}

func TestForecastAccuracyReportsBiasAndMissingCoverage(t *testing.T) {
	forecast := []*repository.FPnAPlanLine{{Period: "2026-01", Grain: "group", Revenue: floatPtr(100)}}
	actual := []*repository.FPnAPlanLine{{Period: "2026-01", Grain: "group", Revenue: floatPtr(120)}, {Period: "2026-02", Grain: "group", Revenue: floatPtr(50)}}
	result := ForecastAccuracy(forecast, actual)
	if result.Bias != 70 || result.Coverage.Complete {
		t.Fatalf("unexpected accuracy: %#v", result)
	}
}

func TestVerifyRealizationDoesNotTreatMissingActualAsSuccess(t *testing.T) {
	status, amount := VerifyRealization(floatPtr(10), floatPtr(10), nil, 0.01)
	if status != "pending" || amount != nil {
		t.Fatalf("missing actual should remain pending")
	}
	status, amount = VerifyRealization(floatPtr(10), floatPtr(10), floatPtr(0), 0.01)
	if status != "verified" || amount == nil || *amount != 10 {
		t.Fatalf("unexpected realization: %s %#v", status, amount)
	}
}

func TestRankActionPrioritizesOverdueMaterialItem(t *testing.T) {
	now := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	due := now.Add(-24 * time.Hour)
	amount := 100000.0
	rank := RankAction(repository.FPnAActionItem{ID: "a1", Severity: "critical", ImpactAmount: &amount, DueDate: &due, VerificationStatus: "pending"}, now)
	if rank.Score < 150 || len(rank.Reasons) < 3 {
		t.Fatalf("unexpected rank: %#v", rank)
	}
}

func TestRankActionUsesExplicitEvidencePriorityDimensions(t *testing.T) {
	now := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	evidence, _ := json.Marshal(map[string]any{"control_risk_score": 20, "recurrence_count": 2, "fixability_score": 80})
	rank := RankAction(repository.FPnAActionItem{ID: "a2", Severity: "low", Evidence: evidence}, now)
	if rank.Score < 50 || len(rank.Reasons) != 3 {
		t.Fatalf("explicit evidence dimensions were not ranked: %#v", rank)
	}
}

func floatPtr(value float64) *float64 { return &value }
func stringPtr(value string) *string  { return &value }
