// Package ecomkpi 是电商指标语义层（模块深化 EM3）：平行于 retailkpi 复刻其纪律，
// 不共享 Definitions map——事实族（站点日 ≠ store-day）与指标族（MER/CM/ROAS ≠ 坪效/人效）
// 都不同，共享的只有「严格 null / 零分母 / 覆盖门槛 / Surface 启动校验」这套形状
// （spec 待决问题②定案）。
//
// 中文名只有本包一张真相源；消费包不得再建第二张 labels map。
// 每个指标定义挂 Metric Definition Version；改口径必须出新版本，测试钉住。
package ecomkpi

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/lease-management-system/core-service/internal/services/ecomfact"
)

// FormulaVersion 是语义契约自身的版本（类比 retail-kpi-v1）。
const FormulaVersion = "ecom-kpi-v1"

// MetricDefinitionVersion 是当前口径字典版本；改任何 Definition 的口径必须递增它。
const MetricDefinitionVersion = "ecom-metric-def-v1"

// Status 严格三值：complete / partial / unavailable。没有第四种「大概齐」。
type Status string

const (
	StatusComplete    Status = "complete"
	StatusPartial     Status = "partial"
	StatusUnavailable Status = "unavailable"
)

// Definition 口径字典条目。Formula 是给人看的文字描述；真正的计算在 evaluate 分派里，
// 两者不一致时以测试锁定的行为为准（golden）。
type Definition struct {
	Code                    string `json:"code"`
	NameZH                  string `json:"name_zh"`
	Unit                    string `json:"unit"`
	Formula                 string `json:"formula"`
	MetricDefinitionVersion string `json:"metric_definition_version"`
	RequiredFields          []string `json:"required_fields"`
	NullRule                string   `json:"null_rule"`
	DenominatorRule         string   `json:"denominator_rule,omitempty"`
	Description             string   `json:"description"`
}

var definitions = []Definition{
	{Code: "gmv", NameZH: "GMV", Unit: "currency", Formula: "下单商品金额 − 折扣（不含税、不含运费）",
		RequiredFields: []string{"gmv_amount"}, NullRule: "any_missing_null",
		Description: "GMV = 下单商品金额 − 订单级/商品级折扣"},
	{Code: "discounts", NameZH: "折扣金额", Unit: "currency", Formula: "sum(discount_amount)",
		RequiredFields: []string{"discount_amount"}, NullRule: "any_missing_null"},
	{Code: "refunds", NameZH: "退款退货", Unit: "currency", Formula: "sum(refund_amount)",
		RequiredFields: []string{"refund_amount"}, NullRule: "any_missing_null"},
	{Code: "chargeback_losses", NameZH: "拒付损失", Unit: "currency", Formula: "sum(chargeback_loss_amount)",
		RequiredFields: []string{"chargeback_loss_amount"}, NullRule: "any_missing_null"},
	{Code: "net_revenue", NameZH: "净收入（经营口径）", Unit: "currency",
		Formula: "GMV − 退款 − 退货 − 拒付损失", RequiredFields: []string{"gmv_amount", "refund_amount", "chargeback_loss_amount"},
		NullRule: "any_missing_null", Description: "经营口径净收入，与会计口径收入并列、永不互换"},
	{Code: "order_count", NameZH: "订单数", Unit: "count", Formula: "sum(order_count)",
		RequiredFields: []string{"order_count"}, NullRule: "any_missing_null"},
	{Code: "new_customer_orders", NameZH: "新客首单数", Unit: "count", Formula: "sum(new_customer_orders)",
		RequiredFields: []string{"new_customer_orders"}, NullRule: "any_missing_null"},
	{Code: "aov", NameZH: "客单价 AOV", Unit: "currency", Formula: "net_revenue ÷ order_count",
		RequiredFields: []string{"gmv_amount", "refund_amount", "chargeback_loss_amount", "order_count"},
		NullRule: "any_missing_null", DenominatorRule: "zero_denominator_unavailable"},
	{Code: "landed_cost", NameZH: "落地成本", Unit: "currency", Formula: "采购价 + 头程 + 关税 + 入仓（移动平均）",
		RequiredFields: []string{"landed_cost_amount"}, NullRule: "any_missing_null"},
	{Code: "fulfillment_cost", NameZH: "履约成本", Unit: "currency", Formula: "包材+拣配+头程+尾程+退货运费（3PL 账单口径）",
		RequiredFields: []string{"fulfillment_amount"}, NullRule: "any_missing_null"},
	{Code: "payment_fee", NameZH: "支付通道费", Unit: "currency", Formula: "收单账单实扣（含汇兑费）",
		RequiredFields: []string{"payment_fee_amount"}, NullRule: "any_missing_null"},
	{Code: "cm1", NameZH: "订单贡献 CM1", Unit: "currency", Formula: "净收入 − 落地成本 − 履约 − 支付通道费",
		RequiredFields: []string{"gmv_amount", "refund_amount", "chargeback_loss_amount", "landed_cost_amount", "fulfillment_amount", "payment_fee_amount"},
		NullRule: "any_missing_null"},
	{Code: "cm1_rate", NameZH: "CM1 率", Unit: "ratio", Formula: "CM1 ÷ 净收入",
		RequiredFields: []string{"gmv_amount", "refund_amount", "chargeback_loss_amount", "landed_cost_amount", "fulfillment_amount", "payment_fee_amount"},
		NullRule: "any_missing_null", DenominatorRule: "zero_denominator_unavailable"},
	{Code: "tax_collected", NameZH: "代收销售税/VAT", Unit: "currency", Formula: "sum(tax_collected_amount)",
		RequiredFields: []string{"tax_collected_amount"}, NullRule: "any_missing_null",
		Description: "代收代缴项目，不进收入行，单独挂负债并参与对账（R-E2-6）"},
	{Code: "ad_spend_booked", NameZH: "广告费·账面", Unit: "currency", Formula: "平台后台 spend 合计",
		MetricDefinitionVersion: MetricDefinitionVersion,
		RequiredFields: []string{"ad_spend"}, NullRule: "any_missing_null"},
	{Code: "ad_spend_paid", NameZH: "广告费·实付", Unit: "currency", Formula: "代理发票额合计（返点冲减下期）",
		MetricDefinitionVersion: MetricDefinitionVersion,
		RequiredFields: []string{"ad_spend"}, NullRule: "any_missing_null"},
	{Code: "mer", NameZH: "营销效率比 MER", Unit: "ratio", Formula: "净收入 ÷ 广告费（实付）",
		RequiredFields: []string{"gmv_amount", "refund_amount", "chargeback_loss_amount", "ad_spend"},
		NullRule: "any_missing_null", DenominatorRule: "zero_denominator_unavailable"},
	{Code: "roas", NameZH: "广告回报 ROAS", Unit: "ratio", Formula: "净收入 ÷ 广告费（实付，campaign 归属）",
		RequiredFields: []string{"gmv_amount", "refund_amount", "chargeback_loss_amount", "ad_spend"},
		NullRule: "any_missing_null", DenominatorRule: "zero_denominator_unavailable"},
	{Code: "cm2", NameZH: "广告后贡献 CM2", Unit: "currency", Formula: "CM1 − 广告费（实付）",
		RequiredFields: []string{"gmv_amount", "refund_amount", "chargeback_loss_amount", "landed_cost_amount", "fulfillment_amount", "payment_fee_amount", "ad_spend"},
		NullRule: "any_missing_null"},
	{Code: "cm2_rate", NameZH: "CM2 率", Unit: "ratio", Formula: "CM2 ÷ 净收入",
		RequiredFields: []string{"gmv_amount", "refund_amount", "chargeback_loss_amount", "landed_cost_amount", "fulfillment_amount", "payment_fee_amount", "ad_spend"},
		NullRule: "any_missing_null", DenominatorRule: "zero_denominator_unavailable"},
	{Code: "refund_rate", NameZH: "退款率", Unit: "ratio", Formula: "(退款+拒付损失) ÷ GMV",
		RequiredFields: []string{"gmv_amount", "refund_amount", "chargeback_loss_amount"},
		NullRule: "any_missing_null", DenominatorRule: "zero_denominator_unavailable"},
	{Code: "cac_paid", NameZH: "付费新客 CAC", Unit: "currency", Formula: "广告费（实付）÷ 付费新客数",
		RequiredFields: []string{"ad_spend", "new_customer_orders"}, NullRule: "any_missing_null",
		DenominatorRule: "zero_denominator_unavailable"},
	{Code: "cac_blended", NameZH: "混合 CAC", Unit: "currency", Formula: "广告费（实付）÷ 全部订单数",
		RequiredFields: []string{"ad_spend", "order_count"}, NullRule: "any_missing_null",
		DenominatorRule: "zero_denominator_unavailable"},
}

var chineseNames = map[string]string{}

func init() {
	for _, d := range definitions {
		chineseNames[d.Code] = d.NameZH
	}
}

// Definitions 返回口径字典深拷贝（消费方改不动内部切片）。
func Definitions() []Definition {
	out := make([]Definition, len(definitions))
	copy(out, definitions)
	for i := range out {
		if out[i].MetricDefinitionVersion == "" {
			out[i].MetricDefinitionVersion = MetricDefinitionVersion
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

// FindDefinition 按 code 查找口径定义。
func FindDefinition(code string) (Definition, bool) {
	for _, d := range definitions {
		if d.Code == code {
			return d, true
		}
	}
	return Definition{}, false
}

// DisplayName 中文名唯一出口：未登记的 code 返回原样 code，调用方不得自建第二张映射。
func DisplayName(code string) string {
	if n, ok := chineseNames[code]; ok {
		return n
	}
	return code
}

// Label 返回（名，是否存在）；不存在时调用方应显示「未识别指标」而不是把 code 当名字贴出去。
func Label(code string) (string, bool) {
	n, ok := chineseNames[code]
	return n, ok
}

// SurfaceEntry 哪些指标露出到哪个页面（受校验清单）。
type SurfaceEntry struct {
	Code string `json:"code"`
	Page string `json:"page"`
}

// Surface 全量露出清单：启动校验引用完整性，未知 code 让进程起不来。
var Surface = []SurfaceEntry{
	{Code: "gmv", Page: "site-pulse"},
	{Code: "net_revenue", Page: "site-pulse"},
	{Code: "cm1_rate", Page: "site-pulse"},
	{Code: "mer", Page: "site-pulse"},
	{Code: "refund_rate", Page: "site-pulse"},
	{Code: "ad_spend_paid", Page: "site-pulse"},
	{Code: "net_revenue", Page: "site-360"},
	{Code: "cm1", Page: "site-360"},
	{Code: "cm2", Page: "site-360"},
	{Code: "cac_paid", Page: "site-360"},
	{Code: "cac_blended", Page: "site-360"},
	{Code: "aov", Page: "site-360"},
	{Code: "roas", Page: "site-360"},
	{Code: "gmv", Page: "site-pnl"},
	{Code: "net_revenue", Page: "site-pnl"},
	{Code: "landed_cost", Page: "site-pnl"},
	{Code: "fulfillment_cost", Page: "site-pnl"},
	{Code: "payment_fee", Page: "site-pnl"},
	{Code: "cm1", Page: "site-pnl"},
	{Code: "ad_spend_paid", Page: "site-pnl"},
	{Code: "cm2", Page: "site-pnl"},
	{Code: "tax_collected", Page: "settlement-workbench"},
}

// ValidateSurface 与 retailkpi 同款启动守卫：清单里的 code 必须存在，
// 一次报出全部未知 code，否则启动失败。
func ValidateSurface(entries []SurfaceEntry) error {
	var unknown []string
	pages := map[string][]string{}
	for _, e := range entries {
		if _, ok := FindDefinition(e.Code); !ok {
			unknown = append(unknown, e.Code)
		}
		pages[e.Code] = append(pages[e.Code], e.Page)
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fmt.Errorf("ecomkpi surface references undefined metric codes: %s", strings.Join(unique(unknown), ", "))
	}
	return nil
}

// Value 一个指标的求值结果：严格 null——Value 为 nil 就是不可用，永不补 0。
type Value struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Currency   string  `json:"currency"`
	Value      *float64 `json:"value"`
	Status     Status  `json:"status"`
	Reason     string  `json:"reason,omitempty"`
	Unit       string  `json:"unit"`
	Numerator  string  `json:"numerator,omitempty"`
	Denominator string `json:"denominator,omitempty"`
}

// Coverage 覆盖率与 Decision Ready 判定（沿用零售语义：缺数据降级，不编造）。
type Coverage struct {
	ExpectedDays int     `json:"expected_days"`
	ObservedDays int     `json:"observed_days"`
	CoverageRate *float64 `json:"coverage_rate"`
}

func buildCoverage(w ecomfact.Window, days map[string]bool) Coverage {
	cov := Coverage{ExpectedDays: w.Days(), ObservedDays: len(days)}
	if cov.ExpectedDays > 0 {
		rate := float64(cov.ObservedDays) / float64(cov.ExpectedDays)
		cov.CoverageRate = roundPtr(rate)
	}
	return cov
}

// CoverageIncomplete 覆盖不足 ⇒ Decision Ready=false 的判定点。
func CoverageIncomplete(c Coverage) bool { return c.ExpectedDays > 0 && c.ObservedDays < c.ExpectedDays }

// CurrencyPartitionResult 币种分区的求值结果：多币种永不跨币种聚合（R-E2-5 默认视图）。
type CurrencyPartitionResult struct {
	Currency      string            `json:"currency"`
	KPIs          map[string]Value  `json:"kpis"`
	Issues        []string          `json:"issues,omitempty"`
	DecisionReady bool              `json:"decision_ready"`
}

// AggregateWindow 把站点日事实按币种聚合为逐日行集（先做 Highest 解析是调用方的责任；
// 这里对同键重复行保守去重：保留 fact_version 最高的一行）。
type dailyAgg struct {
	currency string
	gmv, discount, refund, chargeback *float64
	landed, fulfillment, paymentFee, tax *float64
	orders, newOrders *int
	day string
}

// EvaluateByCurrency 对窗口内事实按指标 code 求值，按币种分区返回。
// ads 提供广告口径事实（basis 已由读侧过滤）；两者任一为空不报错——覆盖门槛负责降级。
func EvaluateByCurrency(codes []string, facts []ecomfact.StorefrontDayFact, ads []ecomfact.CampaignDayFact, w ecomfact.Window) ([]CurrencyPartitionResult, Coverage) {
	highest := ecomfact.HighestStorefrontDays(facts)
	days := map[string]bool{}
	byCur := map[string][]*dailyAgg{}
	for i := range highest {
		f := highest[i]
		day := f.BusinessDate.UTC().Format("2006-01-02")
		days[day] = true
		key := f.Currency
		byCur[key] = append(byCur[key], &dailyAgg{
			currency: f.Currency, day: day,
			gmv: f.GMVAmount, discount: f.DiscountAmount, refund: f.RefundAmount, chargeback: f.ChargebackLoss,
			landed: f.LandedCostAmount, fulfillment: f.FulfillmentAmount, paymentFee: f.PaymentFeeAmount,
			tax: f.TaxCollectedAmount, orders: f.OrderCount, newOrders: f.NewCustomerOrders,
		})
	}
	coverage := buildCoverage(w, days)

	adSpendByCur := map[string]*float64{}
	for _, ad := range ecomfact.HighestCampaignDays(ads) {
		cur := adSpendByCur[ad.Currency]
		v := ad.SpendAmount
		if cur == nil {
			adSpendByCur[ad.Currency] = &v
		} else {
			nv := *cur + v
			adSpendByCur[ad.Currency] = &nv
		}
	}

	currencies := make([]string, 0, len(byCur))
	for cur := range byCur {
		currencies = append(currencies, cur)
	}
	sort.Strings(currencies)

	results := make([]CurrencyPartitionResult, 0, len(currencies))
	for _, cur := range currencies {
		rows := byCur[cur]
		res := CurrencyPartitionResult{Currency: cur, KPIs: map[string]Value{}, DecisionReady: !CoverageIncomplete(coverage)}
		for _, code := range codes {
			res.KPIs[code] = evaluateOne(code, rows, adSpendByCur[cur], cur)
		}
		results = append(results, res)
	}
	if len(results) == 0 {
		// 无事实：返回一个空分区让上层显式降级，而不是静默消失。
		results = append(results, CurrencyPartitionResult{KPIs: map[string]Value{}, DecisionReady: false,
			Issues: []string{"no_facts_in_window"}})
	}
	if CoverageIncomplete(coverage) {
		for i := range results {
			results[i].DecisionReady = false
			results[i].Issues = append(results[i].Issues, "incomplete_storefront_day_coverage")
		}
	}
	return results, coverage
}

func evaluateOne(code string, rows []*dailyAgg, adSpend *float64, currency string) Value {
	base := Value{Code: code, Currency: currency, Status: StatusComplete, Unit: unitFor(code)}
	name, ok := Label(code)
	if !ok {
		base.Status = StatusUnavailable
		base.Reason = "unknown_metric_code"
		return base
	}
	base.Name = name
	sumF := func(pick func(*dailyAgg) *float64) (*float64, bool) {
		total := 0.0
		for _, r := range rows {
			v := pick(r)
			if v == nil {
				return nil, false
			}
			total += *v
		}
		t := round(total)
		return &t, true
	}
	sumI := func(pick func(*dailyAgg) *int) (*float64, bool) {
		total := 0.0
		for _, r := range rows {
			v := pick(r)
			if v == nil {
				return nil, false
			}
			total += float64(*v)
		}
		t := round(total)
		return &t, true
	}
	unavailable := func(reason string) Value {
		v := base
		v.Value, v.Status, v.Reason = nil, StatusUnavailable, reason
		return v
	}
	ratio := func(num *float64, den *float64, numText, denText string) Value {
		v := base
		if num == nil || den == nil {
			v.Value, v.Status, v.Reason = nil, StatusUnavailable, "missing_required_field"
			return v
		}
		if *den == 0 {
			v.Value, v.Status, v.Reason = nil, StatusUnavailable, "zero_denominator"
			return v
		}
		r := round(*num / *den)
		v.Value, v.Numerator, v.Denominator = &r, numText, denText
		return v
	}

	switch code {
	case "gmv":
		v, ok := sumF(func(r *dailyAgg) *float64 { return sub(r.gmv, r.discount) })
		if !ok {
			return unavailable("missing_required_field")
		}
		base.Value = v
	case "discounts":
		v, ok := sumF(func(r *dailyAgg) *float64 { return r.discount })
		if !ok {
			return unavailable("missing_required_field")
		}
		base.Value = v
	case "refunds":
		v, ok := sumF(func(r *dailyAgg) *float64 { return r.refund })
		if !ok {
			return unavailable("missing_required_field")
		}
		base.Value = v
	case "chargeback_losses":
		v, ok := sumF(func(r *dailyAgg) *float64 { return r.chargeback })
		if !ok {
			return unavailable("missing_required_field")
		}
		base.Value = v
	case "net_revenue":
		g, ok1 := sumF(func(r *dailyAgg) *float64 { return sub(r.gmv, r.discount) })
		rf, ok2 := sumF(func(r *dailyAgg) *float64 { return r.refund })
		cb, ok3 := sumF(func(r *dailyAgg) *float64 { return r.chargeback })
		if !ok1 || !ok2 || !ok3 {
			return unavailable("missing_required_field")
		}
		nr := round(*g - *rf - *cb)
		base.Value = &nr
	case "landed_cost":
		v, ok := sumF(func(r *dailyAgg) *float64 { return r.landed })
		if !ok {
			return unavailable("missing_required_field")
		}
		base.Value = v
	case "fulfillment_cost":
		v, ok := sumF(func(r *dailyAgg) *float64 { return r.fulfillment })
		if !ok {
			return unavailable("missing_required_field")
		}
		base.Value = v
	case "payment_fee":
		v, ok := sumF(func(r *dailyAgg) *float64 { return r.paymentFee })
		if !ok {
			return unavailable("missing_required_field")
		}
		base.Value = v
	case "tax_collected":
		v, ok := sumF(func(r *dailyAgg) *float64 { return r.tax })
		if !ok {
			return unavailable("missing_required_field")
		}
		base.Value = v
	case "order_count":
		v, ok := sumI(func(r *dailyAgg) *int { return r.orders })
		if !ok {
			return unavailable("missing_required_field")
		}
		base.Value = v
	case "new_customer_orders":
		v, ok := sumI(func(r *dailyAgg) *int { return r.newOrders })
		if !ok {
			return unavailable("missing_required_field")
		}
		base.Value = v
	case "aov":
		nrV := evaluateOne("net_revenue", rows, adSpend, currency)
		oc := evaluateOne("order_count", rows, adSpend, currency)
		if nrV.Value == nil || oc.Value == nil {
			return unavailable("missing_required_field")
		}
		return ratio(nrV.Value, oc.Value, "net_revenue", "order_count")
	case "cm1":
		nrV := evaluateOne("net_revenue", rows, adSpend, currency)
		lc, _ := sumF(func(r *dailyAgg) *float64 { return r.landed })
		ff, _ := sumF(func(r *dailyAgg) *float64 { return r.fulfillment })
		pf, _ := sumF(func(r *dailyAgg) *float64 { return r.paymentFee })
		if nrV.Value == nil || lc == nil || ff == nil || pf == nil {
			return unavailable("missing_required_field")
		}
		cm := round(*nrV.Value - *lc - *ff - *pf)
		base.Value = &cm
	case "cm1_rate":
		cm1 := evaluateOne("cm1", rows, adSpend, currency)
		nr := evaluateOne("net_revenue", rows, adSpend, currency)
		return ratio(cm1.Value, nr.Value, "cm1", "net_revenue")
	case "ad_spend_booked", "ad_spend_paid":
		if adSpend == nil {
			return unavailable("missing_required_field")
		}
		v := round(*adSpend)
		base.Value = &v
	case "mer", "roas":
		nr := evaluateOne("net_revenue", rows, adSpend, currency)
		if nr.Value == nil || adSpend == nil {
			return unavailable("missing_required_field")
		}
		sp := round(*adSpend)
		return ratio(nr.Value, &sp, "net_revenue", "ad_spend_paid")
	case "cm2":
		cm1 := evaluateOne("cm1", rows, adSpend, currency)
		if cm1.Value == nil || adSpend == nil {
			return unavailable("missing_required_field")
		}
		cm2 := round(*cm1.Value - *adSpend)
		base.Value = &cm2
	case "cm2_rate":
		cm2 := evaluateOne("cm2", rows, adSpend, currency)
		nr := evaluateOne("net_revenue", rows, adSpend, currency)
		return ratio(cm2.Value, nr.Value, "cm2", "net_revenue")
	case "refund_rate":
		rf, _ := sumF(func(r *dailyAgg) *float64 { return add(r.refund, r.chargeback) })
		gmv := evaluateOne("gmv", rows, adSpend, currency)
		return ratio(rf, gmv.Value, "refunds_plus_chargebacks", "gmv")
	case "cac_paid":
		if adSpend == nil {
			return unavailable("missing_required_field")
		}
		sp := round(*adSpend)
		nc, _ := sumI(func(r *dailyAgg) *int { return r.newOrders })
		if nc == nil {
			return unavailable("missing_required_field")
		}
		return ratio(&sp, nc, "ad_spend_paid", "paying_new_customers")
	case "cac_blended":
		if adSpend == nil {
			return unavailable("missing_required_field")
		}
		sp := round(*adSpend)
		oc, _ := sumI(func(r *dailyAgg) *int { return r.orders })
		if oc == nil {
			return unavailable("missing_required_field")
		}
		return ratio(&sp, oc, "ad_spend_paid", "order_count")
	default:
		return unavailable("unknown_metric_code")
	}
	return base
}

func unitFor(code string) string {
	if d, ok := FindDefinition(code); ok {
		return d.Unit
	}
	return ""
}

func sub(a, b *float64) *float64 {
	if a == nil || b == nil {
		return nil
	}
	v := *a - *b
	return &v
}

func add(a, b *float64) *float64 {
	if a == nil || b == nil {
		return nil
	}
	v := *a + *b
	return &v
}

func round(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return v
	}
	return math.Round(v*100) / 100
}

func roundPtr(v float64) *float64 { r := round(v); return &r }

func unique(in []string) []string {
	seen := map[string]bool{}
	out := in[:0]
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
