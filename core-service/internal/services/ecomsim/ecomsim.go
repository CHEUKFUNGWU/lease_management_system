// Package ecomsim 是大促与定价情景模拟（模块深化 EM7）。
//
// 纯函数评估器：只评估不落正式表；输出顶层强制 data_classification=simulated
// （响应类型字段，不是约定——底线 2 靠类型不靠自觉，D-E8）。
// 现金缺口提示（备货 + 广告预充 + 回款滞后）P0 只做静态公式版。
// 不做：预测模型、补货优化（PRD §7）。
package ecomsim

import (
	"context"
	"math"
)

// ClassificationSimulated 情景输出的唯一合法分类值。
const ClassificationSimulated = "simulated"

// CMReader 基线贡献率端口（可选）：nil 时输出省略基线对比并给具名 warning。
type CMReader interface {
	HistoricalCM1(ctx context.Context, storefrontID string) (*float64, error)
}

// BFCMInput 大促输入：广告预算 + 预期 CPM/CPC/CVR/AOV（R-E5-1）。
// 全部为显式输入；缺项（nil/0）走具名降级，不猜。
type BFCMInput struct {
	StorefrontID string   `json:"storefront_id"`
	Currency     string   `json:"currency"`
	AdBudget     float64  `json:"ad_budget"`
	CPM          float64  `json:"cpm"` // 千次曝光成本
	CPC          float64  `json:"cpc"` // 单次点击成本
	CVR          float64  `json:"cvr"` // 转化率（0-1）
	AOV          float64  `json:"aov"` // 客单价
	CM1Rate      float64  `json:"cm1_rate"`
	FixedCost    float64  `json:"fixed_cost"`
	TargetProfit float64  `json:"target_profit"`

	// 现金缺口静态公式输入（可空）
	InventoryOutlay *float64 `json:"inventory_outlay,omitempty"` // 备货现金占用
	PayoutLagDays   int      `json:"payout_lag_days"`            // 回款滞后（T+N）
	ReserveHoldPct  float64  `json:"reserve_hold_pct"`           // 准备金冻结比例（0-1）
}

// CashGapHint 静态公式版现金提示：备货占用 + 广告预充 − 大促期内可回款。
type CashGapHint struct {
	InventoryOutlay  *float64 `json:"inventory_outlay,omitempty"`
	AdPrepay         float64  `json:"ad_prepay"`
	ExpectedCollect  float64  `json:"expected_collect_in_window"`
	Gap              float64  `json:"gap"`  // >0 表示需要垫资
	BasisNote        string   `json:"basis_note"`
}

// ScenarioOutput 情景输出。Classification 字段在类型上强制 simulated。
type ScenarioOutput struct {
	DataClassification string   `json:"data_classification"` // 恒 "simulated"
	Currency           string   `json:"currency"`
	Impressions        *float64 `json:"impressions"`
	Clicks             *float64 `json:"clicks"`
	Orders             *float64 `json:"orders"`
	GMV                *float64 `json:"gmv"`
	CM1                *float64 `json:"cm1"`
	MER                *float64 `json:"mer"` // 净收入 ÷ 广告预算
	BreakEvenMER       *float64 `json:"break_even_mer,omitempty"`
	BreakEvenROAS      *float64 `json:"break_even_roas,omitempty"`
	BreakEvenStatus    string   `json:"break_even_status"`
	BreakEvenReason    string   `json:"break_even_reason,omitempty"`
	BaseCM1Rate        *float64 `json:"base_cm1_rate,omitempty"`
	Warnings           []string `json:"warnings,omitempty"`
	CashGap            *CashGapHint `json:"cash_gap_hint,omitempty"`
}

// EvaluateBFCM 大促保本与预期测算。funnel：
//
//	impressions = budget ÷ CPM × 1000 → clicks = budget ÷ CPC → orders = clicks × CVR → GMV = orders × AOV
//
// 任一环节输入非法（≤0 或 CM1 率 < 0）⇒ 该环节及下游 nil + 具名 warning；
// 保本判定复用 unitecon（CM1 率 ≤ 0 ⇒ unachievable 具名值）。
func EvaluateBFCM(ctx context.Context, in BFCMInput, cm CMReader) *ScenarioOutput {
	out := &ScenarioOutput{DataClassification: ClassificationSimulated, Currency: in.Currency}
	warn := func(msg string) { out.Warnings = append(out.Warnings, msg) }

	if in.AdBudget <= 0 {
		out.BreakEvenStatus = "unachievable"
		out.BreakEvenReason = "ad_budget_must_be_positive"
		warn("ad_budget_must_be_positive")
		return out
	}
	clicks := 0.0
	if in.CPC > 0 {
		c := in.AdBudget / in.CPC
		out.Clicks = ptr(round2(c))
		clicks = c
	} else {
		warn("cpc_missing_clicks_unknown")
	}
	if in.CPM > 0 {
		imp := in.AdBudget / in.CPM * 1000
		out.Impressions = ptr(round2(imp))
	} else {
		warn("cpm_missing_impressions_unknown")
	}
	orders := 0.0
	if clicks > 0 && in.CVR > 0 {
		orders = clicks * in.CVR
		out.Orders = ptr(round2(orders))
	} else {
		warn("cvr_missing_orders_unknown")
	}
	gmv := 0.0
	if orders > 0 && in.AOV > 0 {
		gmv = orders * in.AOV
		out.GMV = ptr(round2(gmv))
	} else {
		warn("aov_missing_gmv_unknown")
	}
	if in.CM1Rate >= 0 && out.GMV != nil {
		cm1 := gmv * in.CM1Rate - 0 // CM1 = GMV × CM1 率（情景口径）
		out.CM1 = ptr(round2(cm1))
		if in.AdBudget > 0 {
			out.MER = ptr(round2(gmv / in.AdBudget))
		}
		be := breakEven(in.CM1Rate, in.FixedCost, in.TargetProfit)
		out.BreakEvenStatus = string(be.Status)
		out.BreakEvenMER, out.BreakEvenROAS, out.BreakEvenReason = be.MER, be.ROAS, be.Reason
	} else {
		out.BreakEvenStatus = "unachievable"
		out.BreakEvenReason = "cm1_rate_invalid_or_gmv_unknown"
		warn("cm1_rate_invalid")
	}
	if in.CM1Rate < 0 {
		out.BreakEvenStatus = "unachievable"
		out.BreakEvenReason = "cm1_rate_is_negative"
	}

	if cm != nil {
		base, err := cm.HistoricalCM1(ctx, in.StorefrontID)
		switch {
		case err != nil:
			warn("baseline_cm1_read_failed")
		case base == nil:
			warn("baseline_cm1_unavailable")
		default:
			out.BaseCM1Rate = base
		}
	} else {
		warn("baseline_cm1_port_unwired")
	}

	// 现金缺口静态公式版：备货 + 广告预充 − 窗口内回款（GMV × (1−准备金比例) 的滞后近似按 0 计入，
	// 因为 T+N 滞后 ≥ 大促窗口是常态；这里只提示、不预测）。
	if in.InventoryOutlay != nil || in.PayoutLagDays > 0 || in.ReserveHoldPct > 0 {
		hint := &CashGapHint{
			AdPrepay: round2(in.AdBudget),
			BasisNote: "static_formula_v0：备货+广告预充−窗口内回款(按滞后≥窗口期计 0)；非预测模型",
		}
		if in.InventoryOutlay != nil {
			hint.InventoryOutlay = ptr(round2(*in.InventoryOutlay))
		}
		collected := 0.0
		if in.PayoutLagDays == 0 {
			collected = gmv * (1 - clamp01(in.ReserveHoldPct))
		}
		hint.ExpectedCollect = round2(collected)
		totalOut := hint.AdPrepay
		if hint.InventoryOutlay != nil {
			totalOut += *hint.InventoryOutlay
		}
		hint.Gap = round2(totalOut - collected)
		out.CashGap = hint
	}
	return out
}

// PriceDelta 改价 / 运费 / 折扣的敏感度输入（R-E5-2）。百分比用小数（0.05 = +5%）。
type PriceDelta struct {
	PricePct     *float64 `json:"price_pct,omitempty"`      // 售价变动
	ShippingPct  *float64 `json:"shipping_pct,omitempty"`   // 运费变动
	DiscountPct  *float64 `json:"discount_pct,omitempty"`   // 折扣力度变动
	Elasticity   *float64 `json:"elasticity,omitempty"`     // 需求弹性（可空；空 = 销量不变假设）
}

// CMBase 单位经济基线：全部必填（缺失由调用方先降级，本函数不猜）。
type CMBase struct {
	AOV                float64 `json:"aov"`
	UnitLandedCost     float64 `json:"unit_landed_cost"`
	FulfillmentPerOrder float64 `json:"fulfillment_per_order"`
	PaymentFeePct      float64 `json:"payment_fee_pct"`
	AdSpendPerOrder    float64 `json:"ad_spend_per_order"`
	CurrentUnits       int     `json:"current_units"`
	Currency           string  `json:"currency"`
}

// PriceScenarioOutput 定价敏感度输出：单位经济前后对照。
type PriceScenarioOutput struct {
	DataClassification string   `json:"data_classification"`
	Currency           string   `json:"currency"`
	BaseUnitPrice      float64  `json:"base_unit_price"`
	NewUnitPrice       float64  `json:"new_unit_price"`
	BaseUnitCM1        float64  `json:"base_unit_cm1"`
	NewUnitCM1         float64  `json:"new_unit_cm1"`
	BaseUnits          int      `json:"base_units"`
	ProjectedUnits     int      `json:"projected_units"`
	BaseTotalCM1       float64  `json:"base_total_cm1"`
	NewTotalCM1        float64  `json:"new_total_cm1"`
	UnitsAssumption    string   `json:"units_assumption"`
	Warnings           []string `json:"warnings,omitempty"`
}

// PriceSensitivity 改价 / 运费 / 折扣敏感度：单位 CM1 = 价 − 落地 − 履约 − 支付费 − 广告/单。
// 弹性缺省时销量不变（显式声明 UnitsAssumption，绝不假装知道需求曲线）。
func PriceSensitivity(delta PriceDelta, base CMBase) *PriceScenarioOutput {
	out := &PriceScenarioOutput{
		DataClassification: ClassificationSimulated,
		Currency:           base.Currency,
		BaseUnitPrice:      round2(base.AOV),
		BaseUnits:          base.CurrentUnits,
		UnitsAssumption:    "units_constant",
	}
	pct := func(p *float64) float64 {
		if p == nil {
			return 0
		}
		return *p
	}
	newPrice := base.AOV * (1 + pct(delta.PricePct))
	newShip := base.FulfillmentPerOrder * (1 + pct(delta.ShippingPct))
	unitCM := func(price, ship float64) float64 {
		fee := price * base.PaymentFeePct
		return price - base.UnitLandedCost - ship - fee - base.AdSpendPerOrder
	}
	out.NewUnitPrice = round2(newPrice)
	out.BaseUnitCM1 = round2(unitCM(base.AOV, base.FulfillmentPerOrder))
	out.NewUnitCM1 = round2(unitCM(newPrice, newShip))
	units := float64(base.CurrentUnits)
	if delta.Elasticity != nil {
		// 近似：销量变化 ≈ −弹性 × 加权价格变化（价+运+折合成有效单价变化率）
		effectiveBase := base.AOV + base.FulfillmentPerOrder
		effectiveNew := newPrice + newShip
		if effectiveBase > 0 {
			changeRate := (effectiveNew - effectiveBase) / effectiveBase
			units = units * (1 - *delta.Elasticity*changeRate)
			if units < 0 {
				units = 0
			}
			out.UnitsAssumption = "constant_elasticity_approx"
		}
	}
	out.ProjectedUnits = int(math.Round(units))
	out.BaseTotalCM1 = round2(float64(base.CurrentUnits) * out.BaseUnitCM1)
	out.NewTotalCM1 = round2(float64(out.ProjectedUnits) * out.NewUnitCM1)
	if base.PaymentFeePct < 0 || base.PaymentFeePct > 1 {
		out.Warnings = append(out.Warnings, "payment_fee_pct_out_of_range")
	}
	return out
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

type beResult struct {
	Status string
	MER    *float64
	ROAS   *float64
	Reason string
}

func breakEven(cm1Rate, fixedCost, targetProfit float64) beResult {
	if cm1Rate <= 0 || !isFinite(cm1Rate) {
		reason := "cm1_rate_is_zero"
		if cm1Rate < 0 {
			reason = "cm1_rate_is_negative"
		}
		if !isFinite(cm1Rate) {
			reason = "invalid_input"
		}
		return beResult{Status: "unachievable", Reason: reason}
	}
	mer := round2((fixedCost + targetProfit) / cm1Rate)
	roas := round2(1 / cm1Rate)
	return beResult{Status: "achieved", MER: &mer, ROAS: &roas}
}

func isFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func ptr(v float64) *float64 { return &v }

func round2(v float64) float64 { return math.Round(v*100) / 100 }
