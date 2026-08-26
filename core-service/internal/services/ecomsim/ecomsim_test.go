package ecomsim

import (
	"context"
	"testing"
)

// R-E5-1/2/3 的 golden：funnel 逐格、保本判定、输出顶层 simulated 标识。

func TestEvaluateBFCMFunnel(t *testing.T) {
	in := BFCMInput{
		AdBudget: 100000, CPM: 10, CPC: 1.0, CVR: 0.02, AOV: 50,
		CM1Rate: 0.25, FixedCost: 100000, TargetProfit: 20000, Currency: "USD",
	}
	out := EvaluateBFCM(context.Background(), in, nil)
	if out.DataClassification != ClassificationSimulated {
		t.Fatalf("输出顶层必须 simulated：%q", out.DataClassification)
	}
	if out.Impressions == nil || *out.Impressions != 10000000 {
		t.Fatalf("impressions = 100000/10*1000 = 10,000,000：%v", out.Impressions)
	}
	if out.Clicks == nil || *out.Clicks != 100000 {
		t.Fatalf("clicks = 100000/1.0 = 100,000：%v", out.Clicks)
	}
	if out.Orders == nil || *out.Orders != 2000 {
		t.Fatalf("orders = 100000*0.02 = 2,000：%v", out.Orders)
	}
	if out.GMV == nil || *out.GMV != 2000*50 {
		t.Fatalf("GMV = 2000*50 = 100,000：%v", out.GMV)
	}
	if out.MER == nil || *out.MER != 1.0 {
		t.Fatalf("MER = 100000/100000 = 1.0：%v", out.MER)
	}
	if out.CM1 == nil || *out.CM1 != 25000 {
		t.Fatalf("CM1 = 100000*0.25 = 25,000：%v", out.CM1)
	}
	if out.BreakEvenStatus != "achieved" || out.BreakEvenMER == nil || *out.BreakEvenMER != 480000 {
		t.Fatalf("保本 MER = (100000+20000)/0.25 = 480,000：%+v", out)
	}
	if out.BreakEvenROAS == nil || *out.BreakEvenROAS != 4 {
		t.Fatalf("保本 ROAS = 4：%+v", out)
	}
}

func TestEvaluateBFCMNegativeCM1Unachievable(t *testing.T) {
	in := BFCMInput{AdBudget: 1000, CPC: 1, CVR: 0.02, AOV: 50, CM1Rate: -0.1}
	out := EvaluateBFCM(context.Background(), in, nil)
	if out.BreakEvenStatus != "unachievable" || out.BreakEvenMER != nil || out.BreakEvenROAS != nil {
		t.Fatalf("CM1 率 ≤ 0 必须 unachievable 且无数值：%+v", out)
	}
	if out.CM1 != nil {
		t.Fatalf("负 CM1 率下不得给出 CM1：%+v", out)
	}
}

func TestEvaluateBFCMZeroBudgetArguesWarnings(t *testing.T) {
	out := EvaluateBFCM(context.Background(), BFCMInput{AdBudget: 0, CM1Rate: 0.1}, nil)
	if out.BreakEvenStatus != "unachievable" {
		t.Fatalf("预算为 0 无意义：%+v", out)
	}
	if len(out.Warnings) == 0 {
		t.Fatalf("预算为 0 必须有 warning：%+v", out)
	}
}

func TestEvaluateBFCMCashGapStaticFormula(t *testing.T) {
	inv := 50000.0
	in := BFCMInput{AdBudget: 100000, CPC: 1, CVR: 0.02, AOV: 50, CM1Rate: 0.25,
		InventoryOutlay: &inv, PayoutLagDays: 5, ReserveHoldPct: 0.1}
	out := EvaluateBFCM(context.Background(), in, nil)
	if out.CashGap == nil {
		t.Fatalf("备货+回款滞后输入必须出 cash gap 提示：%+v", out)
	}
	// 滞后 ≥ 窗口 ⇒ 窗口内回款按 0（静态公式版语义：T+N 滞后 ≥ 大促窗口是常态）
	if out.CashGap.Gap != 150000 {
		t.Fatalf("gap = 备货50000 + 广告100000 − 0 = 150,000：%+v", out.CashGap)
	}
	if out.CashGap.BasisNote == "" {
		t.Fatalf("现金提示必须声明静态公式口径（非预测模型）：%+v", out.CashGap)
	}
	if out.DataClassification != "simulated" {
		t.Fatalf("现金提示随输出一起 simulated：%+v", out)
	}
}

func TestPriceSensitivityUnitsConstantByDefault(t *testing.T) {
	base := CMBase{AOV: 50, UnitLandedCost: 20, FulfillmentPerOrder: 5, PaymentFeePct: 0.03,
		AdSpendPerOrder: 10, CurrentUnits: 1000, Currency: "USD"}
	priceUp := 0.1
	out := PriceSensitivity(PriceDelta{PricePct: &priceUp}, base)
	if out.DataClassification != "simulated" {
		t.Fatalf("敏感度输出必须 simulated")
	}
	if out.UnitsAssumption != "units_constant" {
		t.Fatalf("无弹性时销量不变假设必须显式声明：%s", out.UnitsAssumption)
	}
	// 原单位 CM1 = 50−20−5−50*0.03−10 = 13.5
	if out.BaseUnitCM1 != 13.5 {
		t.Fatalf("基线单位 CM1 应 13.5：%v", out.BaseUnitCM1)
	}
	// 新价 55：55−20−5−55*0.03−10 = 18.35
	if out.NewUnitCM1 != 18.35 {
		t.Fatalf("提价 10pct 后单位 CM1 应 18.35：%v", out.NewUnitCM1)
	}
	if out.BaseTotalCM1 != 13500 || out.NewTotalCM1 != 18350 {
		t.Fatalf("总量 CM1 计算错误：%+v", out)
	}
}

func TestPriceSensitivityElasticityApprox(t *testing.T) {
	base := CMBase{AOV: 50, UnitLandedCost: 20, FulfillmentPerOrder: 5, PaymentFeePct: 0.03,
		AdSpendPerOrder: 10, CurrentUnits: 1000, Currency: "USD"}
	priceUp := 0.1
	elasticity := 1.5
	out := PriceSensitivity(PriceDelta{PricePct: &priceUp, Elasticity: &elasticity}, base)
	if out.UnitsAssumption != "constant_elasticity_approx" {
		t.Fatalf("带弹性时必须显式声明近似假设：%s", out.UnitsAssumption)
	}
	// effectiveBase = 55, effectiveNew = 60 → changeRate = 5/55 ≈ 0.0909
	// units = 1000*(1−1.5*0.0909) ≈ 863.6 → 864
	if out.ProjectedUnits >= out.BaseUnits {
		t.Fatalf("提价 + 弹性>1 必须缩量：%d → %d", out.BaseUnits, out.ProjectedUnits)
	}
}
