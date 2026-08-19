package adapter

import (
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

func pf(v float64) *float64 { return &v }

func factDay(store string, revenue, labor float64) retailkpi.DailyFact {
	f := retailkpi.DailyFact{
		StoreID: store, Currency: "CNY", SourceSystem: "pos-a", DataClassification: "production",
		Version: 2, BusinessDate: mustDate("2026-07-10"),
	}
	if revenue >= 0 {
		f.Revenue = pf(revenue)
	}
	if labor >= 0 {
		f.LaborCost = pf(labor)
	}
	return f
}

func mustDate(value string) (out time.Time) {
	out, _ = time.Parse("2006-01-02", value)
	return out
}

func TestAggregateMonthFactsSumsAndCoverage(t *testing.T) {
	set := &repository.RetailKPIFactSet{
		ExpectedStoreCount: 2,
		ExpectedStores: []retailkpi.StorePopulation{
			{StoreID: "S1"}, {StoreID: "S2"},
		},
		Facts: []retailkpi.DailyFact{
			factDay("S1", 100, 30), factDay("S1", 50, 20), factDay("S2", 200, 60),
		},
	}
	out := AggregateMonthFacts("2026-07", set)
	if out.Revenue == nil || *out.Revenue != 350 || out.LaborCost == nil || *out.LaborCost != 110 {
		t.Fatalf("sums wrong: revenue=%v labor=%v", out.Revenue, out.LaborCost)
	}
	if out.GrossProfit != nil {
		t.Fatalf("fields without contributing rows must stay nil, got %v", *out.GrossProfit)
	}
	if !out.DecisionReady {
		t.Fatalf("full store coverage must be decision-ready: %q", out.DecisionReadyReason)
	}
	if out.DataClassification != "production" {
		t.Fatalf("classification = %q", out.DataClassification)
	}
}

func TestAggregateMonthFactsDegrades(t *testing.T) {
	// 一家门店无事实 → 覆盖不足；混币种 → 同因诚实降级。
	partial := &repository.RetailKPIFactSet{
		ExpectedStoreCount: 2,
		Facts:              []retailkpi.DailyFact{factDay("S1", 100, 30)},
	}
	out := AggregateMonthFacts("2026-07", partial)
	if out.DecisionReady || out.DecisionReadyReason == "" {
		t.Fatalf("partial coverage must degrade with a reason: %+v", out)
	}

	mixed := &repository.RetailKPIFactSet{
		ExpectedStoreCount: 2,
		Facts: []retailkpi.DailyFact{
			func() retailkpi.DailyFact {
				f := factDay("S1", 100, 30)
				return f
			}(), func() retailkpi.DailyFact {
				f := factDay("S2", 100, 30)
				f.Currency = "USD"
				return f
			}(),
		},
	}
	mixedOut := AggregateMonthFacts("2026-07", mixed)
	if mixedOut.DecisionReady {
		t.Fatalf("mixed currencies must degrade: %+v", mixedOut)
	}
}
