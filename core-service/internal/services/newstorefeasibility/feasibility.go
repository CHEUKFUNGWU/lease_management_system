package newstorefeasibility

import (
	"context"
	"math"
)

// RH4 新店可行性测算（Spec D-R4，模块设计 §6，D-R13/D-R14）。
//
// 形状照 finmodel：纯函数 Evaluate(in, ports)，一切外部数据经 Ports 注入。
// 三条硬约束：
//   1. 本包禁止 import ifrs16——租赁侧数字经 LeaseProjectionReader 从
//      measurement_results 只读投影取（守卫 importguard_test.go 遍历全部子包）；
//   2. 折现率缺失是部分降级：IRR/NPV/动态回本期 nil + Gap discount_rate_missing，
//      静态回本期与盈亏平衡销售额照常返回（D-R14），不许用任何默认折现率；
//   3. 端口未接线是具名 Gap，不 panic、不填 0。

// Gap 具名缺口：值为什么算不出来。是一等输出，不是错误。
type Gap struct {
	Kind string `json:"kind"`
}

const (
	GapDiscountRateMissing    = "discount_rate_missing"
	GapLeaseProjectionUnwired = "lease_projection_unwired"
)

// BusinessDrivers 业务侧输入。销售额是推导出来的（User Story 21），不是直接填：
//
//	月销售额 = 商圈日均客流 × 营业天数 × 进店率 × 转化率 × 客单价
type BusinessDrivers struct {
	DailyAreaFootfall float64 `json:"daily_area_footfall"`
	OperatingDays     int     `json:"operating_days"`
	EntryRate         float64 `json:"entry_rate"`     // 0-1
	ConversionRate    float64 `json:"conversion_rate"` // 0-1
	AvgTicket         float64 `json:"avg_ticket"`
	GrossMarginRate   float64 `json:"gross_margin_rate"` // 0-1
}

// InvestmentPlan 投资侧输入。
type InvestmentPlan struct {
	FitoutAndEquipment float64   `json:"fitout_and_equipment"`
	InitialInventory   float64   `json:"initial_inventory"`
	RampMonths         int       `json:"ramp_months"`
	RampFactors        []float64 `json:"ramp_factors"` // 逐月爬坡系数，长度须 ≥ RampMonths；缺失月份按 1
}

// LeaseTerms 租约侧引用。金额一律不在输入里——经端口从计量结果投影取，
// 与月结报表同数（lease_source 定稿文案的承诺）。
type LeaseTerms struct {
	ContractID string `json:"contract_id"`
}

// Input 唯一入口的输入。DiscountRate 为月折现率，nil = 未确定（fail-closed）。
type Input struct {
	Currency     string          `json:"currency"`
	StartMonth   string          `json:"start_month"` // YYYY-MM
	Horizon      int             `json:"horizon"`     // 评估月数
	Business     BusinessDrivers `json:"business"`
	Investment   InvestmentPlan  `json:"investment"`
	Lease        LeaseTerms      `json:"lease"`
	DiscountRate *float64        `json:"discount_rate"`
}

// LeaseMonth 端口返回的一个月租赁投影（只读）。
type LeaseMonth struct {
	Month           string  `json:"month"` // YYYY-MM
	LeaseExpense    float64 `json:"lease_expense"`
	ROUDepreciation float64 `json:"rou_depreciation"`
	InterestExpense float64 `json:"interest_expense"`
	TotalPayment    float64 `json:"total_payment"`
}

// LeaseProjectionReader 租赁投影端口：生产薄绑定 measurement_results /
// 测试内存桩。全系统只有一套租赁计算，本包绝不自算 ROU 或利息
// （风险红线 14 的对应物）。
type LeaseProjectionReader interface {
	MonthlyProjection(ctx context.Context, contractID, fromMonth string, months int) ([]LeaseMonth, error)
}

type Ports struct {
	LeaseProjection LeaseProjectionReader
}

type MonthlyCashFlow struct {
	Month        string   `json:"month"`
	RampFactor   float64  `json:"ramp_factor"`
	Revenue      *float64 `json:"revenue"`
	GrossProfit  *float64 `json:"gross_profit"`
	LeaseCost    *float64 `json:"lease_cost"`     // 端口未接线时为 nil
	NetCashFlow  *float64 `json:"net_cash_flow"`  // 毛利 − 租赁成本
}

type Result struct {
	Currency        string            `json:"currency,omitempty"`
	MonthlyCashFlows []MonthlyCashFlow `json:"monthly_cash_flows"`
	StaticPayback   *float64          `json:"static_payback_months,omitempty"`   // 月；不依赖折现率
	DynamicPayback  *float64          `json:"dynamic_payback_months,omitempty"`  // 月；依赖折现率
	IRR             *float64          `json:"irr_monthly,omitempty"`             // 月度内部收益率；依赖折现率语义下的求解
	NPV             *float64          `json:"npv,omitempty"`                     // 期初投入 + 贴现月现金流
	BreakEvenSales  *float64          `json:"break_even_sales,omitempty"`        // 月度盈亏平衡销售额；不依赖折现率
	Gaps            []Gap             `json:"gaps"`
	Status          string            `json:"status"`                            // complete | partial | unavailable
}

// Evaluate 纯函数入口。IO 一律经 ports。
func Evaluate(ctx context.Context, in Input, ports Ports) Result {
	res := Result{Currency: in.Currency}
	gaps := map[string]bool{}
	addGap := func(kind string) { gaps[kind] = true }

	// 输入有效性：负值与越界比率直接整体不可用（invalid 不算 Gap，是拒绝）。
	if !inputsValid(in) {
		res.Status = "unavailable"
		res.Gaps = []Gap{{Kind: "invalid_input"}}
		return res
	}

	leaseMissing := ports.LeaseProjection == nil || in.Lease.ContractID == ""
	if ports.LeaseProjection == nil {
		addGap(GapLeaseProjectionUnwired)
	}

	rateMissing := in.DiscountRate == nil
	if rateMissing {
		addGap(GapDiscountRateMissing)
	}

	// 租赁投影：端口未接线时不造数，各月 LeaseCost 保持 nil。
	var leaseByMonth map[string]float64
	if !leaseMissing {
		rows, err := ports.LeaseProjection.MonthlyProjection(ctx, in.Lease.ContractID, in.StartMonth, in.Horizon)
		if err != nil {
			addGap(GapLeaseProjectionUnwired)
			leaseMissing = true
		} else {
			// 投影行数不足评估期（合同不存在于该租户视角、计量未跑满、
			// 或租约短于评估期）：与端口未接线同语义——租赁侧整体缺席，
			// 缺席月份的 LeaseCost 保持 nil，绝不填 0 继续算。
			if len(rows) < in.Horizon {
				addGap(GapLeaseProjectionUnwired)
				leaseMissing = true
			} else {
				leaseByMonth = map[string]float64{}
				for _, r := range rows {
					leaseByMonth[r.Month] = r.LeaseExpense
				}
			}
		}
	}

	months := monthLabels(in.StartMonth, in.Horizon)
	res.MonthlyCashFlows = make([]MonthlyCashFlow, 0, len(months))
	var cumulative, discCumulative float64
	staticPayback := math.NaN()
	dynamicPayback := math.NaN()
	var npv float64

	for i, month := range months {
		row := MonthlyCashFlow{Month: month, RampFactor: rampFactor(in.Investment, i)}
		revenue := monthlyRevenue(in.Business) * row.RampFactor
		gp := revenue * in.Business.GrossMarginRate
		rv, gv := round2(revenue), round2(gp)
		row.Revenue = &rv
		row.GrossProfit = &gv

		if !leaseMissing {
			cost := round2(leaseByMonth[month])
			row.LeaseCost = &cost
			ncf := round2(gp - cost)
			row.NetCashFlow = &ncf
		}
		res.MonthlyCashFlows = append(res.MonthlyCashFlows, row)

		if row.NetCashFlow != nil {
			ncf := *row.NetCashFlow
			initial := initialInvestment(in.Investment)
			cumulative += ncf
			if math.IsNaN(staticPayback) && cumulative >= initial {
				staticPayback = float64(i + 1)
			}
			if !rateMissing {
				df := math.Pow(1+*in.DiscountRate, float64(i+1))
				pv := ncf / df
				discCumulative += pv
				npv += pv
				if math.IsNaN(dynamicPayback) && discCumulative >= initial {
					dynamicPayback = float64(i + 1)
				}
			}
		}
	}

	initial := initialInvestment(in.Investment)
	npv -= initial

	if !math.IsNaN(staticPayback) {
		v := staticPayback
		res.StaticPayback = &v
	}
	if !rateMissing && !math.IsNaN(dynamicPayback) {
		v := dynamicPayback
		res.DynamicPayback = &v
	}
	if !rateMissing {
		monthlyNCF := make([]float64, 0, len(res.MonthlyCashFlows))
		complete := true
		for _, r := range res.MonthlyCashFlows {
			if r.NetCashFlow == nil {
				complete = false
				break
			}
			monthlyNCF = append(monthlyNCF, *r.NetCashFlow)
		}
		if complete {
			irr := irrMonthly(monthlyNCF, initial)
			if !math.IsNaN(irr) {
				// IRR 是率不是金额：保留全精度（roundPtr 会破坏「代回报零」的复核性质）
				v := irr
				res.IRR = &v
			}
			v := round2(npv)
			res.NPV = &v
		}
	}
	if !leaseMissing && in.Business.GrossMarginRate > 0 {
		totalLease, counted := 0.0, 0
		for _, r := range res.MonthlyCashFlows {
			if r.LeaseCost != nil {
				totalLease += *r.LeaseCost
				counted++
			}
		}
		if counted > 0 {
			be := totalLease / float64(counted) / in.Business.GrossMarginRate
			res.BreakEvenSales = roundPtr(be)
		}
	}

	for _, k := range sortedKeys(gaps) {
		res.Gaps = append(res.Gaps, Gap{Kind: k})
	}
	switch {
	case len(gaps) == 0:
		res.Status = "complete"
	case res.StaticPayback != nil || res.BreakEvenSales != nil:
		res.Status = "partial"
	default:
		res.Status = "unavailable"
	}
	return res
}
