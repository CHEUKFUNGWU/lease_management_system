package promotionattribution

import (
	"testing"
)

func fp(v float64) *float64 { return &v }

func TestAttribute_CleanSeparableROI(t *testing.T) {
	promo := Promotion{
		PromoCode:    "PROMO_2026_SUMMER",
		Name:         "夏季清凉会员日",
		PromoType:    "discount",
		StartDate:    "2026-06-01",
		EndDate:      "2026-06-03",
		Currency:     "CNY",
		BudgetAmount: 5000.0,
	}

	costs := []PromotionCost{
		{Category: "subsidy", Amount: 3000.0, Currency: "CNY"},
		{Category: "materials", Amount: 1000.0, Currency: "CNY"},
	} // Total cost = 4000.0

	// 3 event days
	actual := []DailyFact{
		{BusinessDate: "2026-06-01", Revenue: fp(20000.0), GrossProfit: fp(8000.0)},
		{BusinessDate: "2026-06-02", Revenue: fp(25000.0), GrossProfit: fp(10000.0)},
		{BusinessDate: "2026-06-03", Revenue: fp(25000.0), GrossProfit: fp(10000.0)},
	} // Actual Rev = 70,000, GP = 28,000

	baseline := RunRate{
		DailyRevenue:     15000.0, // 3 days * 15,000 = 45,000
		DailyGrossProfit: 6000.0,  // 3 days * 6,000 = 18,000
	}

	overlaps := []Promotion{} // No overlaps

	res := Attribute(promo, costs, actual, baseline, overlaps)

	if !res.IsSeparable || res.Status != StatusSeparable {
		t.Fatalf("expected separable status, got %s", res.Status)
	}

	// Incremental Sales = 70,000 - 45,000 = 25,000
	if res.IncrementalRevenue != 25000.0 {
		t.Fatalf("expected incRev 25000, got %.2f", res.IncrementalRevenue)
	}

	// Incremental GP = 28,000 - 18,000 = 10,000
	if res.IncrementalGrossProfit != 10000.0 {
		t.Fatalf("expected incGP 10000, got %.2f", res.IncrementalGrossProfit)
	}

	// Total Cost = 4,000
	if res.TotalCost != 4000.0 {
		t.Fatalf("expected cost 4000, got %.2f", res.TotalCost)
	}

	// ROI = 10,000 / 4,000 = 2.5
	if res.ROI == nil || *res.ROI != 2.5 {
		t.Fatalf("expected ROI 2.5, got %v", res.ROI)
	}

	if len(res.OverlapWarnings) != 0 {
		t.Fatalf("expected 0 overlap warnings, got %d", len(res.OverlapWarnings))
	}
}

func TestAttribute_OverlappingDegradation(t *testing.T) {
	promo := Promotion{
		PromoCode: "PROMO_MAIN",
		Name:      "店庆大促",
		StartDate: "2026-06-01",
		EndDate:   "2026-06-07",
		Currency:  "CNY",
	}

	overlaps := []Promotion{
		{
			PromoCode: "PROMO_CONCURRENT",
			Name:      "商圈联名满减",
			StartDate: "2026-06-03",
			EndDate:   "2026-06-05",
		},
	}

	res := Attribute(promo, nil, nil, RunRate{DailyRevenue: 1000, DailyGrossProfit: 400}, overlaps)

	if res.IsSeparable || res.Status != StatusNonSeparable {
		t.Fatalf("expected non_separable status on overlap, got %s", res.Status)
	}

	if len(res.OverlapWarnings) != 1 {
		t.Fatalf("expected 1 overlap warning, got %d", len(res.OverlapWarnings))
	}
}

func TestAttribute_MissingRevenueDaysAreNotZero(t *testing.T) {
	// 缺失的营收/毛利不得当作 0 拉低实际值，也不得用 0 编造增量。
	promo := Promotion{PromoCode: "P", StartDate: "2026-06-01", EndDate: "2026-06-02", Currency: "CNY"}
	actual := []DailyFact{
		{BusinessDate: "2026-06-01", Revenue: fp(1000.0), GrossProfit: fp(400.0)},
		{BusinessDate: "2026-06-02", Revenue: nil, GrossProfit: fp(300.0)}, // 营收缺失
	}
	baseline := RunRate{DailyRevenue: 500, DailyGrossProfit: 200}

	res := Attribute(promo, nil, actual, baseline, nil)

	if res.ActualRevenue != 1000.0 {
		t.Fatalf("expected actual revenue 1000 (missing day excluded, not 0), got %.2f", res.ActualRevenue)
	}
	if res.EventDays != 2 {
		t.Fatalf("expected 2 event days (missing day still counts as an event day), got %d", res.EventDays)
	}
	if res.ROI != nil {
		t.Fatalf("expected nil ROI (no costs -> no ROI), got %v", *res.ROI)
	}
	hasMissingDisclaimer := false
	for _, d := range res.Disclaimers {
		if len(d) > 0 && d != "本测算基于活动前同期基线运行率 (Run-Rate) 进行关联分析，不构成完全排他的因果性证明。" {
			hasMissingDisclaimer = true
		}
	}
	if !hasMissingDisclaimer {
		t.Fatalf("expected a disclaimer about missing fact days, got %v", res.Disclaimers)
	}
}
