package varianceattribution

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

// RH5 利润差异归因（Spec D-R5，模块设计 §7）。
//
// 方法：连环替代法（cumulative chain replacement）。利润函数
//
//	π(f, c, t, m, l, o, x) = f·c·t·m − l − o − x
//
// 其中 f 客流、c 转化率、t 客单价、m 毛利率、l 人工、o 经营占用、x 其他可控。
// 从基期向量出发按声明顺序逐项替换为当期值，每步贡献 = 替换后利润 − 替换前利润，
// 并回显该步替换后的中间利润（复核锚点，也是守恒构造断言的对象）。
//
// 数学事实（对实现者与复核者都重要）：精确连环替代下各步贡献之和恒等于
// 总差异（望远镜和），残差只承载浮点噪声，按构造 ≈ 0。因子之间的交互作用
// 不是消失在残差里，而是被吸收进替代顺序靠后的因子的贡献值——这正是顺序
// 必须回显的原因。User Story 30「残差=未归因交叉项」描述的是逐项隔离分解
// （isolated decomposition）那一族方法；本实现选精确连环，因为验收同时要求
// 「换顺序数字会变」，而隔离分解的主要效应与顺序无关，两者不可兼得。
// 该张力已在交付报告登记（Planner 裁决前文案按工单定稿渲染）。

// Factor 参与分解的七因子。
type Factor string

const (
	FactorFootfall   Factor = "footfall"
	FactorConversion Factor = "conversion_rate"
	FactorAvgTicket  Factor = "average_transaction_value"
	FactorMarginRate Factor = "gross_margin_rate"
	FactorLabor      Factor = "labor_cost"
	FactorOccupancy  Factor = "occupancy_cost"
	FactorOther      Factor = "other_controllable_cost"
)

// DefaultOrder 固定替代顺序（D-R5 钉死）：客流 → 转化率 → 客单价 → 毛利率 → 人工 → 经营占用 → 其他可控。
var DefaultOrder = []Factor{FactorFootfall, FactorConversion, FactorAvgTicket, FactorMarginRate, FactorLabor, FactorOccupancy, FactorOther}

// ResidualMaterialThreshold 残差占总差异的比例超过该值时 ResidualMaterial = true。
const ResidualMaterialThreshold = 0.05

// PeriodFacts 一个期间的门店事实。指针字段：nil 就是缺失，不得当 0 用
// （retail-kpi-v1 同一纪律）。OccupancyCost 是经营占用现金成本口径
// （固定租金 + 变动租金 + 非租赁成本），不是 IFRS 16 折旧利息。
type PeriodFacts struct {
	Footfall              *float64
	Transactions          *float64
	Revenue               *float64
	GrossProfit           *float64
	LaborCost             *float64
	OccupancyCost         *float64
	OtherControllableCost *float64
}

type FactorContribution struct {
	Factor             string  `json:"factor"`
	Base               float64 `json:"base"`                // 该因子基期取值（派生率为算出的比率）
	Current            float64 `json:"current"`             // 该因子当期取值
	Effect             float64 `json:"effect"`              // 本步替换的贡献 = 中间利润差
	IntermediateProfit float64 `json:"intermediate_profit"` // 该步替换后的利润（复核锚点）
}

type Result struct {
	Currency           string               `json:"currency,omitempty"`
	BaseProfit         float64              `json:"base_profit"`
	CurrentProfit      float64              `json:"current_profit"`
	TotalVariance      float64              `json:"total_variance"`
	Factors            []FactorContribution `json:"factors"`
	Residual           float64              `json:"residual"`
	ResidualMaterial   bool                 `json:"residual_material"`
	DecompositionOrder []string             `json:"decomposition_order"` // 必填回显：换个顺序数字会变，不声明顺序的归因数字无法复核
	Status             string               `json:"status"`              // complete | unavailable
	MissingFacts       []string             `json:"missing_facts,omitempty"`
}

func IsResidualMaterial(residual, totalVariance float64) bool {
	denom := math.Abs(totalVariance)
	if denom < 1e-9 {
		// 总差异本身≈0 时谈占比没有意义；非零残差直接视为材料性
		return math.Abs(residual) > 1e-9
	}
	return math.Abs(residual)/denom > ResidualMaterialThreshold
}

// Attribute 连环替代归因。order 为空用 DefaultOrder；非空必须是七因子的一个排列。
// 任一必需事实缺失或出现零分母比率 → 整体 unavailable + MissingFacts，不做部分归因：
// 七个因子里少两个，剩下五个的贡献值是错的，不是「不完整」。
func Attribute(base, current PeriodFacts, currency string, order []Factor) (Result, error) {
	res := Result{Currency: currency, Status: "unavailable"}

	effectiveOrder := DefaultOrder
	if len(order) > 0 {
		if err := validateOrder(order); err != nil {
			return res, err
		}
		effectiveOrder = order
	}
	for _, f := range effectiveOrder {
		res.DecompositionOrder = append(res.DecompositionOrder, string(f))
	}

	// 缺失检查：任一必需事实在任一期间为 nil 即整体不可用
	missing := make([]string, 0)
	raw := map[string][2]*float64{
		"footfall":                {base.Footfall, current.Footfall},
		"transactions":            {base.Transactions, current.Transactions},
		"revenue":                 {base.Revenue, current.Revenue},
		"gross_profit":            {base.GrossProfit, current.GrossProfit},
		"labor_cost":              {base.LaborCost, current.LaborCost},
		"occupancy_cost":          {base.OccupancyCost, current.OccupancyCost},
		"other_controllable_cost": {base.OtherControllableCost, current.OtherControllableCost},
	}
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		pair := raw[k]
		if pair[0] == nil {
			missing = append(missing, fmt.Sprintf("base.%s", k))
		}
		if pair[1] == nil {
			missing = append(missing, fmt.Sprintf("current.%s", k))
		}
	}
	if len(missing) > 0 {
		res.MissingFacts = missing
		return res, nil
	}

	bv, err := deriveVector(base)
	if err != nil {
		res.MissingFacts = append([]string(nil), err.Error())
		return res, nil
	}
	cv, err := deriveVector(current)
	if err != nil {
		res.MissingFacts = append([]string(nil), err.Error())
		return res, nil
	}

	res.BaseProfit = round2(profit(bv))
	res.CurrentProfit = round2(profit(cv))
	res.TotalVariance = round2(profit(cv) - profit(bv))

	var vec [7]float64
	copy(vec[:], bv[:])
	factorIndex := map[Factor]int{
		FactorFootfall: 0, FactorConversion: 1, FactorAvgTicket: 2, FactorMarginRate: 3,
		FactorLabor: 4, FactorOccupancy: 5, FactorOther: 6,
	}
	prev := profit(vec)
	baseVals := map[Factor]float64{}
	curVals := map[Factor]float64{}
	for _, f := range effectiveOrder {
		baseVals[f] = bv[factorIndex[f]]
		curVals[f] = cv[factorIndex[f]]
	}
	for _, f := range effectiveOrder {
		vec[factorIndex[f]] = cv[factorIndex[f]]
		now := profit(vec)
		contrib := FactorContribution{
			Factor:             string(f),
			Base:               round2(baseVals[f]),
			Current:            round2(curVals[f]),
			Effect:             round2(now - prev),
			IntermediateProfit: round2(now),
		}
		res.Factors = append(res.Factors, contrib)
		prev = now
	}

	var sum float64
	for _, fc := range res.Factors {
		sum += fc.Effect
	}
	res.Residual = round2(profit(cv) - profit(bv) - sum)
	res.ResidualMaterial = IsResidualMaterial(res.Residual, res.TotalVariance)
	res.Status = "complete"
	return res, nil
}

// deriveVector 把原始事实转成利润函数的自变量向量。
// 零分母（客流/交易数/销售额为零）使比率无定义，整体不可用——
// 不许拿 0 顶替比率继续分解。
func deriveVector(p PeriodFacts) ([7]float64, error) {
	var v [7]float64
	if *p.Footfall == 0 {
		return v, errors.New("footfall(=0)")
	}
	if *p.Transactions == 0 {
		return v, errors.New("transactions(=0)")
	}
	if *p.Revenue == 0 {
		return v, errors.New("revenue(=0)")
	}
	v[0] = *p.Footfall
	v[1] = *p.Transactions / *p.Footfall // 转化率
	v[2] = *p.Revenue / *p.Transactions  // 客单价
	v[3] = *p.GrossProfit / *p.Revenue   // 毛利率
	v[4] = *p.LaborCost
	v[5] = *p.OccupancyCost
	v[6] = *p.OtherControllableCost
	return v, nil
}

func profit(v [7]float64) float64 {
	return v[0]*v[1]*v[2]*v[3] - v[4] - v[5] - v[6]
}

func validateOrder(order []Factor) error {
	seen := map[Factor]bool{}
	for _, f := range order {
		switch f {
		case FactorFootfall, FactorConversion, FactorAvgTicket, FactorMarginRate, FactorLabor, FactorOccupancy, FactorOther:
			if seen[f] {
				return fmt.Errorf("varianceattribution: duplicate factor %q in order", f)
			}
			seen[f] = true
		default:
			return fmt.Errorf("varianceattribution: unknown factor %q", f)
		}
	}
	if len(seen) != 7 {
		return fmt.Errorf("varianceattribution: order must be a permutation of all seven factors, got %d", len(seen))
	}
	return nil
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// AggregateWindow folds one window's store-day facts into PeriodFacts.
// 缺失传播：任一门店日在任一字段上为 nil，该字段的期间聚合即为缺失（nil），
// 由 Attribute 整体降级——不许拿 0 补齐后继续分解（retail-kpi-v1）。
// 返回的 issues 是窗口级数据问题（如 currency_conflict / no_facts），由调用方
// 并入 MissingFacts 回显。单一真相源：HTTP 归因端点与 Agent 诊断链共用。
func AggregateWindow(facts []retailkpi.DailyFact) (PeriodFacts, []string) {
	out := PeriodFacts{}
	if len(facts) == 0 {
		return out, []string{"no_facts"}
	}
	currencies := map[string]bool{}
	type acc struct {
		allPresent bool
		sum        float64
	}
	fields := map[string]*acc{
		"footfall":                {allPresent: true},
		"transactions":            {allPresent: true},
		"revenue":                 {allPresent: true},
		"gross_profit":            {allPresent: true},
		"labor_cost":              {allPresent: true},
		"occupancy_cost":          {allPresent: true},
		"other_controllable_cost": {allPresent: true},
	}
	add := func(name string, v *float64) {
		a := fields[name]
		if v == nil {
			a.allPresent = false
			return
		}
		a.sum += *v
	}
	for _, f := range facts {
		currencies[f.Currency] = true
		add("footfall", f.Footfall)
		add("transactions", f.Transactions)
		add("revenue", f.Revenue)
		add("gross_profit", f.GrossProfit)
		add("labor_cost", f.LaborCost)
		occ := sumNonNil(f.FixedRent, f.VariableRent, f.NonLeaseCost)
		if f.FixedRent == nil || f.VariableRent == nil || f.NonLeaseCost == nil {
			fields["occupancy_cost"].allPresent = false
		} else {
			fields["occupancy_cost"].sum += occ
		}
		add("other_controllable_cost", f.OtherControllableCost)
	}
	issues := make([]string, 0)
	if len(currencies) > 1 {
		issues = append(issues, "currency_conflict")
	}
	get := func(name string) *float64 {
		a := fields[name]
		if !a.allPresent {
			return nil
		}
		v := a.sum
		return &v
	}
	out.Footfall = get("footfall")
	out.Transactions = nil
	if tx := get("transactions"); tx != nil {
		txInt := *tx
		out.Transactions = &txInt
	}
	out.Revenue = get("revenue")
	out.GrossProfit = get("gross_profit")
	out.LaborCost = get("labor_cost")
	out.OccupancyCost = get("occupancy_cost")
	out.OtherControllableCost = get("other_controllable_cost")

	if len(issues) > 0 || out.Footfall == nil || out.Transactions == nil || out.Revenue == nil ||
		out.GrossProfit == nil || out.LaborCost == nil || out.OccupancyCost == nil || out.OtherControllableCost == nil {
		return out, issues
	}
	return out, issues
}

func sumNonNil(vs ...*float64) float64 {
	var s float64
	for _, v := range vs {
		if v != nil {
			s += *v
		}
	}
	return s
}
