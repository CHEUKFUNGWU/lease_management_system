package promotionattribution

import (
	"math"
	"testing"
)

// RH3（R2-1）四条边界，先红后绿。
//
// 自检句：把 Breakeven 的 PromoMarginRate<=0 分支删掉（让它除出巨大数字
// 返回 achievable），前两条测试必红；把 unachievable 时的指针赋值加回来，
// 第二条的红字会指出金额字段应为 nil。

func TestBreakevenAchievable(t *testing.T) {
	res := Breakeven(BreakevenInput{
		Currency:           "CNY",
		EventDays:          7,
		Baseline:           RunRate{DailyRevenue: 10000, DailyGrossProfit: 3000},
		BaselineMarginRate: 0.30,
		PromoMarginRate:    0.22,
		FixedMarketingCost: 5000,
	})
	if res.Status != "achievable" {
		t.Fatalf("status = %q (%s), want achievable", res.Status, res.UnachievableReason)
	}
	// baselineRevenue = 10000 × 7 = 70000
	if math.Abs(res.BaselineRevenue-70000) > 0.01 {
		t.Fatalf("baseline revenue = %v, want 70000（与 Attribute 同一算式：日基线 × 天数）", res.BaselineRevenue)
	}
	// sacrifice = 70000 × (0.30 − 0.22) = 5600
	if math.Abs(res.MarginSacrifice-5600) > 0.01 {
		t.Fatalf("margin sacrifice = %v, want 5600", res.MarginSacrifice)
	}
	// required = (5000 + 5600) / 0.22 = 48181.82
	if res.RequiredIncrementalRevenue == nil || math.Abs(*res.RequiredIncrementalRevenue-48181.82) > 0.01 {
		t.Fatalf("required incremental revenue = %v, want 48181.82", res.RequiredIncrementalRevenue)
	}
	// uplift = 48181.82 / 70000 ≈ 0.6883
	if res.RequiredUpliftRate == nil || math.Abs(*res.RequiredUpliftRate-0.6883) > 0.0001 {
		t.Fatalf("required uplift rate = %v, want ~0.6883", res.RequiredUpliftRate)
	}
}

func TestBreakevenZeroPromoMarginIsUnachievable(t *testing.T) {
	res := Breakeven(BreakevenInput{
		Currency: "CNY", EventDays: 7,
		Baseline:           RunRate{DailyRevenue: 10000},
		BaselineMarginRate: 0.30,
		PromoMarginRate:    0, // 打到白送：每多卖一元不赚钱
		FixedMarketingCost: 5000,
	})
	if res.Status != "unachievable" {
		t.Fatalf("promo_margin_rate=0 must be unachievable, got %q", res.Status)
	}
	if res.RequiredIncrementalRevenue != nil || res.RequiredUpliftRate != nil {
		t.Fatalf("unachievable must carry nil amounts (no fake solution), got revenue=%v uplift=%v", res.RequiredIncrementalRevenue, res.RequiredUpliftRate)
	}
	if res.UnachievableReason == "" {
		t.Fatal("unachievable must explain why")
	}
}

func TestBreakevenNegativePromoMarginIsUnachievable(t *testing.T) {
	res := Breakeven(BreakevenInput{
		Currency: "CNY", EventDays: 7,
		Baseline:           RunRate{DailyRevenue: 10000},
		BaselineMarginRate: 0.30,
		PromoMarginRate:    -0.05, // 折后毛利为负：卖得越多亏得越多
		FixedMarketingCost: 5000,
	})
	if res.Status != "unachievable" {
		t.Fatalf("promo_margin_rate<0 must be unachievable, got %q", res.Status)
	}
	if res.RequiredIncrementalRevenue != nil || res.RequiredUpliftRate != nil {
		t.Fatalf("unachievable must carry nil amounts, got %v / %v", res.RequiredIncrementalRevenue, res.RequiredUpliftRate)
	}
}

func TestBreakevenZeroBaselineRevenue(t *testing.T) {
	// 基线为零（新店无历史）：保本额 = 固定投入 ÷ 折后毛利率；uplift 无从谈起 → nil
	res := Breakeven(BreakevenInput{
		Currency: "CNY", EventDays: 7,
		Baseline:           RunRate{DailyRevenue: 0},
		BaselineMarginRate: 0.30,
		PromoMarginRate:    0.22,
		FixedMarketingCost: 5000,
	})
	if res.Status != "achievable" {
		t.Fatalf("zero baseline with positive promo margin is still computable, got %q (%s)", res.Status, res.UnachievableReason)
	}
	if res.RequiredIncrementalRevenue == nil || math.Abs(*res.RequiredIncrementalRevenue-22727.27) > 0.01 {
		t.Fatalf("required = 5000/0.22 = 22727.27, got %v", res.RequiredIncrementalRevenue)
	}
	if res.RequiredUpliftRate != nil {
		t.Fatalf("uplift is undefined against a zero baseline, must be nil, got %v", *res.RequiredUpliftRate)
	}
}

func TestBreakevenInvalidInput(t *testing.T) {
	cases := []struct {
		name string
		in   BreakevenInput
	}{
		{"negative baseline daily revenue", BreakevenInput{Baseline: RunRate{DailyRevenue: -1}, EventDays: 7, BaselineMarginRate: 0.2, PromoMarginRate: 0.2}},
		{"negative fixed cost", BreakevenInput{Baseline: RunRate{DailyRevenue: 100}, EventDays: 7, FixedMarketingCost: -5, BaselineMarginRate: 0.2, PromoMarginRate: 0.2}},
		{"baseline margin > 1", BreakevenInput{Baseline: RunRate{DailyRevenue: 100}, EventDays: 7, BaselineMarginRate: 1.2, PromoMarginRate: 0.2}},
		{"negative event days", BreakevenInput{Baseline: RunRate{DailyRevenue: 100}, EventDays: -3, BaselineMarginRate: 0.2, PromoMarginRate: 0.2}},
	}
	for _, tc := range cases {
		res := Breakeven(tc.in)
		if res.Status != "invalid_input" {
			t.Fatalf("%s: status = %q, want invalid_input", tc.name, res.Status)
		}
		if res.RequiredIncrementalRevenue != nil {
			t.Fatalf("%s: must not produce numbers on invalid input", tc.name)
		}
	}
}

// 与投后复盘同源的物理证明：同一个 RunRate 喂进两个函数，
// Breakeven 的 BaselineRevenue 必须等于 Attribute 的 BaselineRevenue。
func TestBreakevenBaselineMatchesAttribute(t *testing.T) {
	rr := RunRate{DailyRevenue: 12000, DailyGrossProfit: 3600, DailyTransactions: 400}
	days := 7

	attr := Attribute(Promotion{PromoCode: "P1", StartDate: "2026-07-01", EndDate: "2026-07-07"}, nil, nil, rr, nil)
	br := Breakeven(BreakevenInput{Currency: "CNY", EventDays: days, Baseline: rr, BaselineMarginRate: 0.3, PromoMarginRate: 0.25})

	if attr.EventDays != days {
		t.Fatalf("attribute event days = %d, want %d", attr.EventDays, days)
	}
	if math.Abs(attr.BaselineRevenue-br.BaselineRevenue) > 0.01 {
		t.Fatalf("投前投后基线必须逐字相同：Attribute=%v Breakeven=%v", attr.BaselineRevenue, br.BaselineRevenue)
	}
}
