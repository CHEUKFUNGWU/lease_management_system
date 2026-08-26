// Package unitecon 是保本与单位经济小包（模块深化 EM6）。
//
// 三个消费方（sitepnl 页面、ecomsim、agent tools）都要保本语义，但不需要整张利润表——
// 独立小包让 ecomsim 不必拖起投影。纯函数无端口。
//
// 「不可达成」是有效结论不是失败：CM1 率 ≤ 0 时返回具名 Unachievable，
// 非 error、非巨大数字（D-E7；沿用 Required Incremental Revenue 的纪律）。
package unitecon

import "math"

// BreakEvenStatus 保本判定的封闭两值：achieved（可计算）| unachievable（不可达成）。
type BreakEvenStatus string

const (
	StatusAchieved     BreakEvenStatus = "achieved"
	StatusUnachievable BreakEvenStatus = "unachievable"
)

// ReasonUnachievableZero / ReasonUnachievableNegative 具名降级原因：
// CM1 率为零或为负时，保本 MER/ROAS 在数学上无意义（负率下「卖越多亏越多」）。
const (
	ReasonUnachievableZero     = "cm1_rate_is_zero"
	ReasonUnachievableNegative = "cm1_rate_is_negative"
)

// BreakEvenResult 盈亏平衡结果。Unachievable 时 MER/ROAS 为 nil——调用方必须把
// 「unachievable」当一等公民展示，不得把 nil 渲染成 0 或 —。
type BreakEvenResult struct {
	Status        BreakEvenStatus `json:"status"`
	Reason        string          `json:"reason,omitempty"`
	BreakEvenMER  *float64        `json:"break_even_mer,omitempty"`
	BreakEvenROAS *float64        `json:"break_even_roas,omitempty"`
	RequiredRevenue *float64      `json:"required_revenue,omitempty"`
}

// BreakEven 盈亏平衡测算（口径见 PRD §4.1）：
//   - Break-even MER = (固定费用 + 目标利润) ÷ CM1 率
//   - Break-even ROAS = 1 ÷ CM1 率
//
// CM1 率 ≤ 0 ⇒ Status=Unachievable 且不给任何数值。输入 NaN/Inf 视为缺失，同样拒绝。
func BreakEven(cm1Rate float64, fixedCost float64, targetProfit float64) BreakEvenResult {
	if !isFinite(cm1Rate) || !isFinite(fixedCost) || !isFinite(targetProfit) {
		return BreakEvenResult{Status: StatusUnachievable, Reason: "invalid_input"}
	}
	if cm1Rate == 0 {
		return BreakEvenResult{Status: StatusUnachievable, Reason: ReasonUnachievableZero}
	}
	if cm1Rate < 0 {
		return BreakEvenResult{Status: StatusUnachievable, Reason: ReasonUnachievableNegative}
	}
	mer := round2((fixedCost + targetProfit) / cm1Rate)
	roas := round2(1 / cm1Rate)
	req := round2((fixedCost + targetProfit) / cm1Rate)
	return BreakEvenResult{Status: StatusAchieved, BreakEvenMER: &mer, BreakEvenROAS: &roas, RequiredRevenue: &req}
}

// CACInput 分子分母显式的 CAC 输入。指针可空——缺就是缺。
type CACInput struct {
	AdSpendPaid    *float64 `json:"ad_spend_paid"`     // 分子来源：广告费·实付
	PayingNewCustomers *int `json:"paying_new_customers"` // 付费新客分母
	TotalOrders    *int     `json:"total_orders"`         // 混合 CAC 分母
}

// CACFigure 单个 CAC 口径：分子分母在响应中标明（R-E3-4），零分母 unavailable。
type CACFigure struct {
	Value       *float64 `json:"value"`
	Numerator   string   `json:"numerator"`
	Denominator string   `json:"denominator"`
	NumValue    *float64 `json:"numerator_value,omitempty"`
	DenValue    *float64 `json:"denominator_value,omitempty"`
	Status      string   `json:"status"`
	Reason      string   `json:"reason,omitempty"`
}

// CACReport 付费新客 CAC 与混合 CAC 并列展示。
type CACReport struct {
	Paid    CACFigure `json:"paid"`
	Blended CACFigure `json:"blended"`
}

// CACView 计算两个口径的 CAC。分子固定为广告费·实付口径；分母分别是付费新客数与全部订单数。
func CACView(in CACInput) CACReport {
	report := CACReport{
		Paid:    CACFigure{Numerator: "ad_spend_paid", Denominator: "paying_new_customers", Status: "unavailable", Reason: "missing_required_field"},
		Blended: CACFigure{Numerator: "ad_spend_paid", Denominator: "order_count", Status: "unavailable", Reason: "missing_required_field"},
	}
	if in.AdSpendPaid == nil || !isFinite(*in.AdSpendPaid) {
		return report
	}
	spend := round2(*in.AdSpendPaid)
	if in.PayingNewCustomers != nil {
		den := float64(*in.PayingNewCustomers)
		report.Paid.NumValue = &spend
		report.Paid.DenValue = &den
		if den > 0 {
			v := round2(spend / den)
			report.Paid.Value, report.Paid.Status = &v, "complete"
			report.Paid.Reason = ""
		} else {
			report.Paid.Reason = "zero_denominator"
		}
	}
	if in.TotalOrders != nil {
		den := float64(*in.TotalOrders)
		report.Blended.NumValue = &spend
		report.Blended.DenValue = &den
		if den > 0 {
			v := round2(spend / den)
			report.Blended.Value, report.Blended.Status = &v, "complete"
			report.Blended.Reason = ""
		} else {
			report.Blended.Reason = "zero_denominator"
		}
	}
	return report
}

func isFinite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func round2(v float64) float64 { return math.Round(v*100) / 100 }
