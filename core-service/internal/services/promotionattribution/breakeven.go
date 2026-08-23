package promotionattribution

import (
	"fmt"
)

// RH3 促销投前保本测算（Spec D-R3，模块设计 §5，D-R12）。
//
// 与投后 Attribute 同包的物理理由：两者必须共用同一 run-rate 基线口径，
// 否则投前保本和投后增量两个数对不上（User Story 19）。
// 一致性靠类型共享——BreakevenInput 直接持有 RunRate（attribution.go:52），
// 不另定义「投前基线」类型。

// BreakevenInput 投前测算输入。Baseline 就是投后复盘用的同一个 RunRate；
// EventDays 与 Attribute 的活动天数同源（同一日期跨度推导）。
type BreakevenInput struct {
	Currency           string
	EventDays          int
	Baseline           RunRate // 活动前同期运行率（与投后复盘同源）
	BaselineMarginRate float64 // 基线综合毛利率（0-1）
	PromoMarginRate    float64 // 折后毛利 / 折后销售额（0-1）
	FixedMarketingCost float64 // 推广费、物料、平台服务费等固定投入
}

// BreakevenResult 保本测算结果。
// PromoMarginRate <= 0 时 Status = unachievable 且两个金额指针为 nil——
// 让利后边际毛利非正意味着卖得越多亏得越多，不存在保本点。
// 返回一个巨大的数字假装有解是本模块最容易犯的错。
type BreakevenResult struct {
	Currency                   string   `json:"currency"`
	EventDays                  int      `json:"event_days"`
	BaselineRevenue            float64  `json:"baseline_revenue"` // Baseline.DailyRevenue × EventDays，与投后的基线算法逐字相同
	RequiredIncrementalRevenue *float64 `json:"required_incremental_revenue,omitempty"`
	RequiredUpliftRate         *float64 `json:"required_uplift_rate,omitempty"` // 相对基线的增幅
	MarginSacrifice            float64  `json:"margin_sacrifice"`               // 原有销量因让利少赚的毛利
	Status                     string   `json:"status"`                         // achievable | unachievable | invalid_input
	UnachievableReason         string   `json:"unachievable_reason,omitempty"`
}

// Breakeven computes pre-promotion break-even incremental revenue.
//
// 公式（D-R3 钉死，勿自行推导）：
//
//	保本增量销售额 = ( 固定投入 + 基线销售额 × (基线毛利率 − 折后毛利率) ) ÷ 折后毛利率
//	其中分子第二项即「原有销量因让利少赚的毛利」（MarginSacrifice）。
func Breakeven(in BreakevenInput) BreakevenResult {
	res := BreakevenResult{Currency: in.Currency, EventDays: in.EventDays}

	// 输入有效性：负值与超出概率语义的比率直接拒绝，不算数。
	if in.EventDays < 0 || in.Baseline.DailyRevenue < 0 ||
		in.FixedMarketingCost < 0 || in.BaselineMarginRate < 0 || in.BaselineMarginRate > 1 || in.PromoMarginRate > 1 {
		res.Status = "invalid_input"
		res.UnachievableReason = "输入无效：天数、金额不能为负，毛利率必须在 0 与 1 之间"
		return res
	}

	baselineRevenue := round2(in.Baseline.DailyRevenue * float64(in.EventDays)) // 与 Attribute 的 baselineRev 同一算式
	res.BaselineRevenue = baselineRevenue

	sacrifice := baselineRevenue * (in.BaselineMarginRate - in.PromoMarginRate)

	if in.PromoMarginRate <= 0 {
		res.Status = "unachievable"
		res.UnachievableReason = fmt.Sprintf("折后每多卖一元也不赚钱（折后毛利率 %s ≤ 0），不存在保本点，卖得越多亏得越多。要么降推广费，要么把折扣力度收回来。", trimPct(in.PromoMarginRate))
		res.MarginSacrifice = round2(sacrifice)
		return res
	}

	required := (in.FixedMarketingCost + sacrifice) / in.PromoMarginRate
	requiredPtr := round2(required)
	res.RequiredIncrementalRevenue = &requiredPtr
	if baselineRevenue > 0 {
		uplift := round4(required / baselineRevenue)
		res.RequiredUpliftRate = &uplift
	}
	res.MarginSacrifice = round2(sacrifice)
	res.Status = "achievable"
	return res
}

func trimPct(v float64) string {
	return fmt.Sprintf("%.2f%%", v*100)
}
