package fpna

import (
	"context"
	"testing"

	"github.com/lease-management-system/core-service/internal/repository"
)

func strPtr(s string) *string {
	return &s
}

func TestCompose_PureComposition(t *testing.T) {
	baselineLines := []*repository.FPnAPlanLine{
		{ID: "b1", PlanVersionID: "v-base", Period: "2026-01", Revenue: floatPtr(100), Currency: "CNY", StoreID: strPtr("store-1")},
		{ID: "b2", PlanVersionID: "v-base", Period: "2026-02", Revenue: floatPtr(110), Currency: "CNY", StoreID: strPtr("store-1")},
		{ID: "b3", PlanVersionID: "v-base", Period: "2026-03", Revenue: floatPtr(120), Currency: "CNY", StoreID: strPtr("store-1")},
		{ID: "b4", PlanVersionID: "v-base", Period: "2026-04", Revenue: floatPtr(130), Currency: "CNY", StoreID: strPtr("store-1")},
	}

	actualLines := []*repository.FPnAPlanLine{
		{ID: "a1", PlanVersionID: "v-act", Period: "2026-01", Revenue: floatPtr(95), Currency: "CNY", StoreID: strPtr("store-1")},
		{ID: "a2", PlanVersionID: "v-act", Period: "2026-02", Revenue: floatPtr(105), Currency: "CNY", StoreID: strPtr("store-1")},
	}

	req := ComposeRequest{
		Name:               "Q1 2026 Rolling Forecast",
		BaselineID:         "v-base",
		ActualID:           "v-act",
		ActualCutoffPeriod: "2026-02",
		ScenarioType:       "baseline",
		Currency:           "CNY",
	}

	proposed, err := Compose(baselineLines, actualLines, req)
	if err != nil {
		t.Fatalf("Compose failed: %v", err)
	}

	if proposed.Name != "Q1 2026 Rolling Forecast" {
		t.Errorf("expected name %s, got %s", req.Name, proposed.Name)
	}
	if proposed.FromPeriod != "2026-01" || proposed.ToPeriod != "2026-04" {
		t.Errorf("expected range 2026-01..2026-04, got %s..%s", proposed.FromPeriod, proposed.ToPeriod)
	}
	if len(proposed.Lines) != 4 {
		t.Fatalf("expected 4 blended lines, got %d", len(proposed.Lines))
	}

	// 2026-01 and 2026-02 should be replaced by actuals
	if *proposed.Lines[0].Revenue != 95 || !proposed.Lines[0].ActualFlag {
		t.Errorf("expected line 0 revenue 95 (actual), got %v", *proposed.Lines[0].Revenue)
	}
	if *proposed.Lines[1].Revenue != 105 || !proposed.Lines[1].ActualFlag {
		t.Errorf("expected line 1 revenue 105 (actual), got %v", *proposed.Lines[1].Revenue)
	}
	// 2026-03 and 2026-04 should remain baseline forecast
	if *proposed.Lines[2].Revenue != 120 || proposed.Lines[2].ActualFlag {
		t.Errorf("expected line 2 revenue 120 (forecast), got %v", *proposed.Lines[2].Revenue)
	}
	if *proposed.Lines[3].Revenue != 130 || proposed.Lines[3].ActualFlag {
		t.Errorf("expected line 3 revenue 130 (forecast), got %v", *proposed.Lines[3].Revenue)
	}

	// Check period blends summary
	if len(proposed.PeriodBlends) != 4 {
		t.Fatalf("expected 4 blend summaries, got %d", len(proposed.PeriodBlends))
	}
	if !proposed.PeriodBlends[0].Replaced || !proposed.PeriodBlends[1].Replaced {
		t.Errorf("expected periods 01 and 02 to be replaced")
	}
	if proposed.PeriodBlends[2].Replaced || proposed.PeriodBlends[3].Replaced {
		t.Errorf("expected periods 03 and 04 NOT to be replaced")
	}
}

func TestMemoryPlanVersionWriter_SingleDraftForecastInvariant(t *testing.T) {
	ctx := context.Background()
	writer := NewMemoryPlanVersionWriter()
	entity := "ent-1"

	proposed := &ProposedForecast{
		Name:               "Forecast 2026-03 v1",
		BaselineID:         "base-1",
		ActualID:           "act-1",
		ActualCutoffPeriod: "2026-03",
		AsOfPeriod:         "2026-03",
		FromPeriod:         "2026-01",
		ToPeriod:           "2026-12",
		Lines: []*repository.FPnAPlanLine{
			{Period: "2026-01", Revenue: floatPtr(100)},
		},
	}

	// First commit should succeed
	v1, err := writer.Commit(ctx, proposed, &entity, "user-1", "key-1")
	if err != nil {
		t.Fatalf("first commit failed: %v", err)
	}
	if v1.ID == "" || v1.Status != "draft" {
		t.Errorf("unexpected version attributes: %+v", v1)
	}

	// Second commit for same period & entity while v1 is in draft MUST fail
	proposed2 := &ProposedForecast{
		Name:               "Forecast 2026-03 v2 conflict",
		BaselineID:         "base-1",
		ActualID:           "act-1",
		ActualCutoffPeriod: "2026-03",
		AsOfPeriod:         "2026-03",
		FromPeriod:         "2026-01",
		ToPeriod:           "2026-12",
		Lines: []*repository.FPnAPlanLine{
			{Period: "2026-01", Revenue: floatPtr(120)},
		},
	}

	_, err = writer.Commit(ctx, proposed2, &entity, "user-2", "key-2")
	if err == nil {
		t.Fatalf("expected second draft forecast commit for same period to fail, but succeeded")
	}
}

func TestEvaluateAccuracyTrend_SystemicBiasDetection(t *testing.T) {
	// 3 consecutive periods where actual is lower than forecast (variance < 0 -> overestimation)
	points := []AccuracyTrendPoint{
		{Period: "2026-01", Forecast: 100, Actual: 90, Variance: -10, Bias: -10},
		{Period: "2026-02", Forecast: 100, Actual: 85, Variance: -15, Bias: -15},
		{Period: "2026-03", Forecast: 100, Actual: 88, Variance: -12, Bias: -12},
	}

	res := EvaluateAccuracyTrend(points)
	if !res.HasSystemicBias {
		t.Errorf("expected systemic bias to be flagged for 3 consecutive negative variance periods")
	}
	if res.SystemicDirection != "overestimation" {
		t.Errorf("expected systemic direction 'overestimation', got %s", res.SystemicDirection)
	}
	if res.ConsecutiveBiasCount != 3 {
		t.Errorf("expected consecutive bias count 3, got %d", res.ConsecutiveBiasCount)
	}
	if res.TotalBias != -37 {
		t.Errorf("expected total bias -37, got %v", res.TotalBias)
	}

	// Intermittent variance should not flag systemic bias
	intermittentPoints := []AccuracyTrendPoint{
		{Period: "2026-01", Forecast: 100, Actual: 90, Variance: -10, Bias: -10},
		{Period: "2026-02", Forecast: 100, Actual: 110, Variance: 10, Bias: 10},
		{Period: "2026-03", Forecast: 100, Actual: 88, Variance: -12, Bias: -12},
	}
	res2 := EvaluateAccuracyTrend(intermittentPoints)
	if res2.HasSystemicBias {
		t.Errorf("did not expect systemic bias for alternating points")
	}
}
