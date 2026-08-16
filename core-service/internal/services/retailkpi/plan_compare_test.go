package retailkpi

import (
	"strings"
	"testing"
	"time"
)

func planPtr(value float64) *float64 { return &value }

func monthFacts(storeID string, days int, revenue float64) []DailyFact {
	facts := make([]DailyFact, 0, days)
	for day := 1; day <= days; day++ {
		facts = append(facts, DailyFact{
			StoreID: storeID, Currency: "CNY", BusinessDate: time.Date(2026, 7, day, 0, 0, 0, 0, time.UTC),
			Revenue: planPtr(revenue), GrossProfit: planPtr(revenue * 0.4), LaborCost: planPtr(100),
			FixedRent: planPtr(50), VariableRent: planPtr(10), NonLeaseCost: planPtr(20), OtherControllableCost: planPtr(5),
		})
	}
	return facts
}

func TestComparePlanBaselineVarianceAndAttainment(t *testing.T) {
	actual := append(monthFacts("s1", 10, 110), monthFacts("s2", 10, 90)...)
	plan := []PlanFact{
		{StoreID: "s1", Period: "2026-07", Currency: "CNY", Revenue: planPtr(1000), GrossProfit: planPtr(400), LaborCost: planPtr(1000), FixedRent: planPtr(500), VariableRent: planPtr(100), NonLeaseCost: planPtr(200)},
		{StoreID: "s2", Period: "2026-07", Currency: "CNY", Revenue: planPtr(1000), GrossProfit: planPtr(400), LaborCost: planPtr(1000), FixedRent: planPtr(500), VariableRent: planPtr(100), NonLeaseCost: planPtr(200)},
	}
	comparison, err := ComparePlan(actual, plan, ComparePlanRequest{Period: "2026-07", ExpectedStoreCount: 2, MaterialityThresholdPct: 5})
	if err != nil {
		t.Fatal(err)
	}
	if !comparison.DecisionReady {
		t.Fatalf("downgraded: %+v", comparison)
	}
	revenue := comparison.Variances[0]
	if revenue.KPI != "revenue" || revenue.Actual == nil || *revenue.Actual != 2000 || *revenue.Plan != 2000 {
		t.Fatalf("revenue=%+v", revenue)
	}
	if revenue.Variance == nil || *revenue.Variance != 0 || revenue.AttainmentPct == nil || *revenue.AttainmentPct != 100 {
		t.Fatalf("revenue variance=%+v", revenue)
	}
	// Store contribution: actual = gross(800) - labor(2000) - occupancy(1600)
	// - other(100) = -2900; plan (no other_controllable) = -2800.
	contribution := comparison.Variances[len(comparison.Variances)-1]
	if contribution.Actual == nil || *contribution.Actual != -2900 || contribution.Plan == nil || *contribution.Plan != -2800 || contribution.Variance == nil || *contribution.Variance != -100 {
		t.Fatalf("contribution=%+v", contribution)
	}
}

func TestComparePlanMissingSideDowngradesWithoutZeroing(t *testing.T) {
	actual := monthFacts("s1", 10, 100)
	plan := []PlanFact{{StoreID: "s1", Period: "2026-07", Currency: "CNY", Revenue: planPtr(1000)}}
	// s2 planned but no actual facts → actual_missing; expected 2 → coverage down
	plan = append(plan, PlanFact{StoreID: "s2", Period: "2026-07", Currency: "CNY", Revenue: planPtr(900)})
	comparison, err := ComparePlan(actual, plan, ComparePlanRequest{Period: "2026-07", ExpectedStoreCount: 2, MaterialityThresholdPct: 5})
	if err != nil {
		t.Fatal(err)
	}
	if comparison.DecisionReady {
		t.Fatalf("should downgrade: %+v", comparison)
	}
	revenue := comparison.Variances[0]
	if !strings.Contains(revenue.DowngradeReason, "actual_missing_for_1_stores") {
		t.Fatalf("revenue reason=%q", revenue.DowngradeReason)
	}
	// Intersection sum ignores s2 rather than treating it as zero.
	if *revenue.Actual != 1000 || *revenue.Plan != 1000 {
		t.Fatalf("revenue totals=%+v", revenue)
	}
}

func TestComparePlanMixedCurrencyDegrades(t *testing.T) {
	actual := monthFacts("s1", 10, 100)
	actual = append(actual, DailyFact{StoreID: "s2", Currency: "USD", BusinessDate: time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC), Revenue: planPtr(50)})
	plan := []PlanFact{{StoreID: "s1", Period: "2026-07", Currency: "CNY", Revenue: planPtr(1000)}}
	comparison, err := ComparePlan(actual, plan, ComparePlanRequest{Period: "2026-07", ExpectedStoreCount: 2, MaterialityThresholdPct: 5})
	if err != nil {
		t.Fatal(err)
	}
	if comparison.DecisionReady || comparison.DowngradeReason != "mixed_currency" {
		t.Fatalf("mixed currency=%+v", comparison)
	}
	for _, variance := range comparison.Variances {
		if variance.Actual != nil || variance.Plan != nil {
			t.Fatalf("mixed currency must not sum across currencies: %+v", variance)
		}
	}
}

func TestComparePlanZeroPlanRefusesAttainment(t *testing.T) {
	actual := monthFacts("s1", 10, 100)
	plan := []PlanFact{{StoreID: "s1", Period: "2026-07", Currency: "CNY", Revenue: planPtr(0)}}
	comparison, err := ComparePlan(actual, plan, ComparePlanRequest{Period: "2026-07", ExpectedStoreCount: 1, MaterialityThresholdPct: 5})
	if err != nil {
		t.Fatal(err)
	}
	revenue := comparison.Variances[0]
	if revenue.AttainmentPct != nil || revenue.VariancePct != nil {
		t.Fatalf("zero plan fabricated rates: %+v", revenue)
	}
	if !strings.Contains(revenue.DowngradeReason, "zero_plan") {
		t.Fatalf("reason=%q", revenue.DowngradeReason)
	}
}

func TestComparePlanMaterialityUsesNegativeBaseDirection(t *testing.T) {
	actual := monthFacts("s1", 10, 50)
	plan := []PlanFact{{StoreID: "s1", Period: "2026-07", Currency: "CNY", Revenue: planPtr(1000)}}
	comparison, err := ComparePlan(actual, plan, ComparePlanRequest{Period: "2026-07", ExpectedStoreCount: 1, MaterialityThresholdPct: 5})
	if err != nil {
		t.Fatal(err)
	}
	revenue := comparison.Variances[0]
	if revenue.VariancePct == nil || *revenue.VariancePct != -50 {
		t.Fatalf("variance pct=%+v", revenue)
	}
	if !revenue.MaterialityExceeded || *revenue.AttainmentPct != 50 {
		t.Fatalf("materiality/attainment=%+v", revenue)
	}
}

// c1: a store with only half the month of facts against a full-month plan is
// a coverage mismatch — the comparison downgrades instead of reporting a
// "variance" that is really missing days.
func TestComparePlanDayCoverageDowngrades(t *testing.T) {
	actual := monthFacts("s1", 15, 100) // half of July only
	plan := []PlanFact{{StoreID: "s1", Period: "2026-07", Currency: "CNY", Revenue: planPtr(3100)}}
	comparison, err := ComparePlan(actual, plan, ComparePlanRequest{Period: "2026-07", ExpectedStoreCount: 1, ExpectedDaysInMonth: 31, MaterialityThresholdPct: 5})
	if err != nil {
		t.Fatal(err)
	}
	if comparison.DecisionReady {
		t.Fatalf("half-month actual against full-month plan must downgrade: %+v", comparison)
	}
	if !strings.Contains(comparison.DowngradeReason, "actual_day_coverage_insufficient") {
		t.Fatalf("reason=%q", comparison.DowngradeReason)
	}
	// Full month coverage clears the day gate (other KPI downgrades here are
	// legitimate — the plan only carries revenue — so assert the absence of
	// the day-coverage reason specifically).
	full := monthFacts("s1", 31, 100)
	ready, err := ComparePlan(full, plan, ComparePlanRequest{Period: "2026-07", ExpectedStoreCount: 1, ExpectedDaysInMonth: 31, MaterialityThresholdPct: 5})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ready.DowngradeReason, "actual_day_coverage_insufficient") {
		t.Fatalf("full-month coverage still day-downgraded: %q", ready.DowngradeReason)
	}
	// Zero gate keeps the legacy behaviour (no day check).
	legacy, err := ComparePlan(actual, plan, ComparePlanRequest{Period: "2026-07", ExpectedStoreCount: 1, MaterialityThresholdPct: 5})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(legacy.DowngradeReason, "actual_day_coverage_insufficient") {
		t.Fatalf("zero gate introduced a day downgrade: %q", legacy.DowngradeReason)
	}
}
