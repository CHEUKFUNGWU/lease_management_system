// Package sitepnl 是单站利润表投影（模块深化 EM4）：一张确定性报表，不是一组 KPI 卡。
//
// 纯函数：一切输入经 Readers 注入；子合计由子行推导，下钻与父行不可能不一致；
// 缺失行 nil 不填 0。经营口径与会计口径是响应类型上的两个块、basis 标签上在块级，
// 两口径永不互换——会计收入行的唯一来源是 GL reader，本包没有任何收入确认计算，
// 这是 R-E3-5 的工程落点。
package sitepnl

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/services/ecomfact"
	"github.com/lease-management-system/core-service/internal/services/unitecon"
)

// RowKeys 固定行序：GMV → 净收入 → 落地成本 → 履约 → 支付通道费 → CM1 →
// 广告(实付) → CM2 → 分摊固定费 → 经营利润（R-E3-1）。
const (
	RowGMV             = "gmv"
	RowNetRevenue      = "net_revenue"
	RowLandedCost      = "landed_cost"
	RowFulfillment     = "fulfillment_cost"
	RowPaymentFee      = "payment_fee"
	RowCM1             = "cm1"
	RowAdSpendPaid     = "ad_spend_paid"
	RowCM2             = "cm2"
	RowFixedCost       = "allocated_fixed_cost"
	RowOperatingProfit = "operating_profit"

	// 辅助构成行（不进主行序，作为 Components 挂在父行下）
	CompDiscounts        = "discounts"
	CompRefunds          = "refunds"
	CompChargebackLosses = "chargeback_losses"
)

// Basis 块级口径标签。经营口径 operating；会计口径 gl（只读来自总账导入）。
type Basis string

const (
	BasisOperating Basis = "operating"
	BasisGL        Basis = "gl"
)

// PeriodKind 月度 | 周度。
type PeriodKind string

const (
	PeriodMonthly PeriodKind = "monthly"
	PeriodWeekly  PeriodKind = "weekly"
)

// Period 月度用 Month（YYYY-MM），周度用 [From, To]。
type Period struct {
	Kind PeriodKind `json:"kind"`
	Month string    `json:"month,omitempty"`
	From  time.Time `json:"from,omitempty"`
	To    time.Time `json:"to,omitempty"`
}

func (p Period) window() ecomfact.Window {
	if p.Kind == PeriodWeekly {
		return ecomfact.Window{From: p.From, To: p.To}
	}
	return ecomfact.Window{} // monthly 由调用方换算成窗口后经 Facts 取数
}

// Breakdown 下钻维：none | channel | campaign | sku。
type Breakdown string

const (
	BreakdownNone     Breakdown = "none"
	BreakdownChannel  Breakdown = "channel"
	BreakdownCampaign Breakdown = "campaign"
	BreakdownSKU      Breakdown = "sku"
)

// Valid 拒绝未知维度。
func (b Breakdown) Valid() bool {
	switch b {
	case BreakdownNone, BreakdownChannel, BreakdownCampaign, BreakdownSKU:
		return true
	}
	return false
}

// SitePnlRequest 投影请求。TargetProfit 参与保本测算（缺省按 0 处理）。
type SitePnlRequest struct {
	Storefront   ecomfact.StorefrontRef `json:"storefront"`
	Currency     string                 `json:"currency"` // 目标币种分区；空 = 全部分区各出一块
	Period       Period                 `json:"period"`
	Breakdown    Breakdown              `json:"breakdown"`
	TargetProfit *float64               `json:"target_profit,omitempty"`
}

// GLRevenueReader 会计收入行的唯一来源端口（只读来自 GL 导入）。
type GLRevenueReader interface {
	GLRevenue(ctx context.Context, ref ecomfact.StorefrontRef, period Period) (*GLRevenue, error)
}

// GLRevenue 总账导出的会计收入（含退货准备计提的口径由 GL 决定，本包不做任何调整计算）。
type GLRevenue struct {
	Amount         *float64  `json:"amount"`
	Currency       string    `json:"currency"`
	SourceSystem   string    `json:"source_system"`
	ImportBatchID  string    `json:"import_batch_id"`
	FactVersion    int       `json:"fact_version"`
	AsOfAt         time.Time `json:"as_of_at"`
}

// FixedCostReader 分摊固定费端口。未接线 ⇒ 具名 Gap，经营利润 nil，绝不补 0。
type FixedCostReader interface {
	FixedCost(ctx context.Context, ref ecomfact.StorefrontRef, period Period) (*FixedCost, error)
}

// FixedCost 一个期间一个币种的分摊固定费。
type FixedCost struct {
	Amount   *float64 `json:"amount"`
	Currency string   `json:"currency"`
	SourceSystem string `json:"source_system"`
}

// Readers 一切外部数据经此注入。
type Readers struct {
	Facts ecomfact.FactReader
	GL    GLRevenueReader
	Fixed FixedCostReader
}

// Component 行内构成拆分：三击到来源的第一击（R-E3-2）。
type Component struct {
	Key   string   `json:"key"`
	Label string   `json:"label"`
	Value *float64 `json:"value"`
}

// Row 利润表行。Kind: line | subtotal。子合计行以 Children 声明推导来源（构造性质，
// 不做「加起来等于自己」的事后检查——红线 12 同款纪律）。
type Row struct {
	Key        string      `json:"key"`
	Label      string      `json:"label"`
	Kind       string      `json:"kind"`
	Sign       int         `json:"sign"`
	Value      *float64    `json:"value"`
	Children   []string    `json:"children,omitempty"`
	Components []Component `json:"components,omitempty"`
}

// BreakdownRow 下钻行：按渠道 / campaign / SKU 展开的净收入与贡献。
type BreakdownRow struct {
	Dimension  string   `json:"dimension"`
	Key        string   `json:"key"`
	NetRevenue *float64 `json:"net_revenue"`
	CM1        *float64 `json:"cm1"`
	AdSpend    *float64 `json:"ad_spend_paid,omitempty"`
}

// AccountingBlock 会计口径块：basis=gl，只读呈现，永不与经营块互换。
type AccountingBlock struct {
	Basis        Basis    `json:"basis"` // 恒为 "gl"
	Revenue      *float64 `json:"revenue"`
	Currency     string   `json:"currency"`
	SourceSystem string   `json:"source_system,omitempty"`
	ImportBatchID string  `json:"import_batch_id,omitempty"`
	FactVersion  int      `json:"fact_version,omitempty"`
	AsOfAt       *time.Time `json:"as_of_at,omitempty"`
	Gap          string   `json:"gap,omitempty"` // gl_unavailable 等，非 error
}

// CurrencyBlock 单一币种分区的完整利润表（默认视图即分区视图，R-E2-5）。
type CurrencyBlock struct {
	Currency    string           `json:"currency"`
	Basis       Basis            `json:"basis"`
	Rows        []Row            `json:"rows"`
	BreakEven   unitecon.BreakEvenResult `json:"break_even"`
	Accounting  AccountingBlock  `json:"accounting"`
	Breakdown   []BreakdownRow   `json:"breakdown,omitempty"`
	RestatedDays []string        `json:"restated_days,omitempty"` // 被重述期间的修订标记
}

// Statement 投影结果顶层。
type Statement struct {
	Storefront ecomfact.StorefrontRef `json:"storefront"`
	Period     Period                 `json:"period"`
	BreakdownDimension Breakdown      `json:"breakdown_dimension"`
	Blocks     []CurrencyBlock        `json:"blocks"`
	Gaps       []string               `json:"gaps"`
}

var rowLabels = map[string]string{
	RowGMV:             "GMV",
	RowNetRevenue:      "净收入（经营口径）",
	RowLandedCost:      "落地成本",
	RowFulfillment:     "履约成本",
	RowPaymentFee:      "支付通道费",
	RowCM1:             "订单贡献 CM1",
	RowAdSpendPaid:     "广告费（实付）",
	RowCM2:             "广告后贡献 CM2",
	RowFixedCost:       "分摊固定费",
	RowOperatingProfit: "经营利润",
	CompDiscounts:      "其中：折扣",
	CompRefunds:        "其中：退款退货",
	CompChargebackLosses: "其中：拒付损失",
}

// Project 单站利润表投影。Facts 未接线返回错误；GL / Fixed 未接线或无数据 ⇒
// 对应块/行降级为具名 Gap，其余行照常产出（部分可用不是整体拒绝）。
func Project(ctx context.Context, req SitePnlRequest, readers Readers) (*Statement, error) {
	if readers.Facts == nil {
		return nil, fmt.Errorf("sitepnl: facts reader 未接线")
	}
	if req.Storefront.LegalEntityID == "" || req.Storefront.StorefrontID == "" {
		return nil, fmt.Errorf("sitepnl: 缺少法人或站点定位——纯函数不做越权兜底（底线 1）")
	}
	if req.Breakdown == "" {
		req.Breakdown = BreakdownNone
	}
	if !req.Breakdown.Valid() {
		return nil, fmt.Errorf("sitepnl: 未知下钻维度 %q", req.Breakdown)
	}
	entity, entityErr := access.EntityFilterFor(req.Storefront.LegalEntityID)
	if entityErr != nil {
		return nil, fmt.Errorf("sitepnl: %w", entityErr)
	}

	win, err := windowOf(req.Period)
	if err != nil {
		return nil, err
	}
	facts, err := readers.Facts.StorefrontDays(ctx, ecomfact.StorefrontFilter{
		Entity:        entity,
		StorefrontIDs: []string{req.Storefront.StorefrontID},
	}, win)
	if err != nil {
		return nil, fmt.Errorf("sitepnl: 读取站点日事实: %w", err)
	}
	ads, err := readers.Facts.CampaignDays(ctx, ecomfact.StorefrontFilter{
		Entity:        entity,
		StorefrontIDs: []string{req.Storefront.StorefrontID},
	}, win, ecomfact.AdBasisPaid)
	if err != nil {
		return nil, fmt.Errorf("sitepnl: 读取 campaign 日事实: %w", err)
	}

	highest := ecomfact.HighestStorefrontDays(facts)
	gaps := make([]string, 0, 4)
	if len(highest) < len(facts) {
		gaps = append(gaps, "restatement_resolved") // 有旧版本被重述取代——记录谱系事件而非错误
	}

	// 分摊固定费：端口未接线或期间无值 ⇒ 具名 Gap，固定费行与经营利润行 nil（不补 0）。
	var fixed *FixedCost
	fixedGap := ""
	if readers.Fixed != nil {
		fc, fcErr := readers.Fixed.FixedCost(ctx, req.Storefront, req.Period)
		switch {
		case fcErr != nil:
			fixedGap = "fixed_cost_read_failed：" + fcErr.Error()
		case fc == nil || fc.Amount == nil:
			fixedGap = "fixed_cost_unavailable：该期间未导入分摊固定费"
		default:
			fixed = fc
		}
	} else {
		fixedGap = "fixed_cost_port_unwired：分摊固定费端口未接线"
	}
	if fixedGap != "" {
		gaps = append(gaps, fixedGap)
	}

	currencies := currenciesIn(highest, req.Currency)
	stmt := &Statement{Storefront: req.Storefront, Period: req.Period, BreakdownDimension: req.Breakdown}

	for _, cur := range currencies {
		block := buildBlock(cur, highest, ads, req, fixed)
		if readers.GL != nil {
			gl, glErr := readers.GL.GLRevenue(ctx, req.Storefront, req.Period)
			switch {
			case glErr != nil:
				block.Accounting = AccountingBlock{Basis: BasisGL, Currency: cur, Gap: "gl_read_failed"}
				gaps = append(gaps, "gl_read_failed："+glErr.Error())
			case gl == nil || gl.Amount == nil:
				block.Accounting = AccountingBlock{Basis: BasisGL, Currency: cur, Gap: "gl_unavailable"}
				gaps = append(gaps, "gl_unavailable：总账收入未导入该期间")
			default:
				block.Accounting = AccountingBlock{Basis: BasisGL, Revenue: gl.Amount, Currency: cur,
					SourceSystem: gl.SourceSystem, ImportBatchID: gl.ImportBatchID,
					FactVersion: gl.FactVersion, AsOfAt: &gl.AsOfAt}
			}
		} else {
			block.Accounting = AccountingBlock{Basis: BasisGL, Currency: cur, Gap: "gl_port_unwired"}
			gaps = append(gaps, "gl_port_unwired：GL 收入端口未接线")
		}
		stmt.Blocks = append(stmt.Blocks, block)
	}

	sort.Slice(stmt.Blocks, func(i, j int) bool { return stmt.Blocks[i].Currency < stmt.Blocks[j].Currency })
	stmt.Gaps = gaps
	return stmt, nil
}

func buildBlock(currency string, facts []ecomfact.StorefrontDayFact, ads []ecomfact.CampaignDayFact, req SitePnlRequest, fixed *FixedCost) CurrencyBlock {
	cur := filterByCurrency(facts, currency)
	sums := sumMeasures(cur)

	block := CurrencyBlock{Currency: currency, Basis: BasisOperating}

	var fixedValue *float64
	if fixed != nil && fixed.Currency == currency {
		fixedValue = fixed.Amount
	}

	gmv := addAll(sums.gmv, neg(sums.discount)) // GMV = 商品金额 − 折扣；折扣缺失 ⇒ 整体 nil
	netRevenue := nilIfAny(sums.gmv, sums.discount, sums.refund, sums.chargeback)
	if netRevenue != nil {
		v := round2(*sums.gmv - *sums.discount - *sums.refund - *sums.chargeback)
		netRevenue = &v
	}
	cm1Parts := []*float64{netRevenue, sums.landed, sums.fulfillment, sums.paymentFee}
	cm1 := deriveSubtotal(cm1Parts, func(vals []*float64) float64 {
		return *vals[0] - *vals[1] - *vals[2] - *vals[3]
	})
	adSpend := paidSpend(ads, currency)
	cm2 := nilIf(cm1, adSpend)
	if cm2 != nil {
		v := round2(*cm1 - *adSpend)
		cm2 = &v
	}
	operatingProfit := deriveSubtotal([]*float64{cm2, fixedValue}, func(vals []*float64) float64 {
		return *vals[0] - *vals[1]
	})

	block.Rows = []Row{
		{Key: RowGMV, Label: rowLabels[RowGMV], Kind: "line", Sign: 1, Value: gmv,
			Components: []Component{{Key: CompDiscounts, Label: rowLabels[CompDiscounts], Value: sums.discount}}},
		{Key: RowNetRevenue, Label: rowLabels[RowNetRevenue], Kind: "subtotal", Sign: 1, Value: netRevenue,
			Children: []string{"gmv_gross", CompRefunds, CompChargebackLosses},
			Components: []Component{
				{Key: CompRefunds, Label: rowLabels[CompRefunds], Value: sums.refund},
				{Key: CompChargebackLosses, Label: rowLabels[CompChargebackLosses], Value: sums.chargeback},
			}},
		{Key: RowLandedCost, Label: rowLabels[RowLandedCost], Kind: "line", Sign: -1, Value: sums.landed},
		{Key: RowFulfillment, Label: rowLabels[RowFulfillment], Kind: "line", Sign: -1, Value: sums.fulfillment},
		{Key: RowPaymentFee, Label: rowLabels[RowPaymentFee], Kind: "line", Sign: -1, Value: sums.paymentFee},
		{Key: RowCM1, Label: rowLabels[RowCM1], Kind: "subtotal", Sign: 1, Value: cm1,
			Children: []string{RowNetRevenue, RowLandedCost, RowFulfillment, RowPaymentFee}},
		{Key: RowAdSpendPaid, Label: rowLabels[RowAdSpendPaid], Kind: "line", Sign: -1, Value: adSpend},
		{Key: RowCM2, Label: rowLabels[RowCM2], Kind: "subtotal", Sign: 1, Value: cm2,
			Children: []string{RowCM1, RowAdSpendPaid}},
		{Key: RowFixedCost, Label: rowLabels[RowFixedCost], Kind: "line", Sign: -1, Value: fixedValue},
		{Key: RowOperatingProfit, Label: rowLabels[RowOperatingProfit], Kind: "subtotal", Sign: 1, Value: operatingProfit,
			Children: []string{RowCM2, RowFixedCost}},
	}

	// 保本：基于显式的固定/变动拆分一键计算（R-E3-3）。CM1 率 ≤ 0 由 unitecon 报 unachievable。
	cm1Rate := cm1RateOf(netRevenue, cm1)
	target := 0.0
	if req.TargetProfit != nil {
		target = *req.TargetProfit
	}
	if fixedValue != nil {
		block.BreakEven = unitecon.BreakEven(cm1Rate, *fixedValue, target)
	} else {
		block.BreakEven = unitecon.BreakEvenResult{Status: unitecon.StatusUnachievable, Reason: "fixed_cost_missing"}
	}

	if req.Breakdown == BreakdownCampaign {
		block.Breakdown = breakdownCampaign(cur, ads, currency)
	} else if req.Breakdown != BreakdownNone {
		block.Breakdown = breakdownDimension(cur, string(req.Breakdown))
	}
	return block
}

// Setters 用于把 FixedCostReader 的结果注入块内（保持 buildBlock 纯函数性）。
// 这里选择显式两段式而不是闭包，让「固定费缺失」在行与保本两处一致地降级。

func cm1RateOf(netRevenue, cm1 *float64) float64 {
	if netRevenue == nil || cm1 == nil || *netRevenue == 0 {
		return 0
	}
	return *cm1 / *netRevenue
}

func breakdownDimension(facts []ecomfact.StorefrontDayFact, dimension string) []BreakdownRow {
	type agg struct {
		nrParts []*float64
	}
	groups := map[string]*agg{}
	order := []string{}
	for _, f := range facts {
		key := ""
		switch dimension {
		case "channel":
			key = f.Channel
		case "sku":
			key = f.SKU
		default:
			continue
		}
		a, ok := groups[key]
		if !ok {
			a = &agg{}
			groups[key] = a
			order = append(order, key)
		}
		if f.GMVAmount != nil && f.DiscountAmount != nil && f.RefundAmount != nil && f.ChargebackLoss != nil {
			nr := *f.GMVAmount - *f.DiscountAmount - *f.RefundAmount - *f.ChargebackLoss
			a.nrParts = append(a.nrParts, &nr)
		} else {
			a.nrParts = append(a.nrParts, nil)
		}
	}
	sort.Strings(order)
	rows := make([]BreakdownRow, 0, len(order))
	for _, key := range order {
		total := 0.0
		ok := len(groups[key].nrParts) > 0
		for _, p := range groups[key].nrParts {
			if p == nil {
				ok = false
				break
			}
			total += *p
		}
		row := BreakdownRow{Dimension: dimension, Key: key}
		if ok {
			v := round2(total)
			row.NetRevenue = &v
		}
		rows = append(rows, row)
	}
	return rows
}

func breakdownCampaign(facts []ecomfact.StorefrontDayFact, ads []ecomfact.CampaignDayFact, currency string) []BreakdownRow {
	// campaign 维的净收入无法从订单侧归属到具体 campaign（独立站订单没有可信的广告归因字段时
	// 不许编造），所以净收入留 nil、只有实付花销有数——这是诚实降级，不是缺功能。
	spends := map[string]float64{}
	keys := []string{}
	for _, ad := range ecomfact.HighestCampaignDays(ads) {
		if ad.Currency != currency {
			continue
		}
		if _, ok := spends[ad.CampaignID]; !ok {
			keys = append(keys, ad.CampaignID)
		}
		spends[ad.CampaignID] += ad.SpendAmount
	}
	sort.Strings(keys)
	rows := make([]BreakdownRow, 0, len(keys))
	for _, k := range keys {
		v := round2(spends[k])
		sv := v
		rows = append(rows, BreakdownRow{Dimension: "campaign", Key: k, AdSpend: &sv})
	}
	return rows
}

type measures struct {
	gmv, discount, refund, chargeback *float64
	landed, fulfillment, paymentFee, tax *float64
	orders, newOrders *int
}

// sumMeasures 聚合度量族：任一天任一字段缺失 ⇒ 该度量整体 nil（严格 null，不补 0）。
func sumMeasures(facts []ecomfact.StorefrontDayFact) measures {
	m := measures{}
	var gmv, disc, rf, cb, lc, ff, pf, tax float64
	okG, okD, okR, okC, okL, okF, okP, okT := true, true, true, true, true, true, true, true
	var orders, newOrders float64
	okO, okN := true, true
	for _, f := range facts {
		if f.GMVAmount == nil {
			okG = false
		} else {
			gmv += *f.GMVAmount
		}
		if f.DiscountAmount == nil {
			okD = false
		} else {
			disc += *f.DiscountAmount
		}
		if f.RefundAmount == nil {
			okR = false
		} else {
			rf += *f.RefundAmount
		}
		if f.ChargebackLoss == nil {
			okC = false
		} else {
			cb += *f.ChargebackLoss
		}
		if f.LandedCostAmount == nil {
			okL = false
		} else {
			lc += *f.LandedCostAmount
		}
		if f.FulfillmentAmount == nil {
			okF = false
		} else {
			ff += *f.FulfillmentAmount
		}
		if f.PaymentFeeAmount == nil {
			okP = false
		} else {
			pf += *f.PaymentFeeAmount
		}
		if f.TaxCollectedAmount == nil {
			okT = false
		} else {
			tax += *f.TaxCollectedAmount
		}
		if f.OrderCount == nil {
			okO = false
		} else {
			orders += float64(*f.OrderCount)
		}
		if f.NewCustomerOrders == nil {
			okN = false
		} else {
			newOrders += float64(*f.NewCustomerOrders)
		}
	}
	if len(facts) == 0 {
		okG, okD, okR, okC, okL, okF, okP, okT, okO, okN = false, false, false, false, false, false, false, false, false, false
	}
	if okG {
		m.gmv = ptr(round2(gmv))
	}
	if okD {
		m.discount = ptr(round2(disc))
	}
	if okR {
		m.refund = ptr(round2(rf))
	}
	if okC {
		m.chargeback = ptr(round2(cb))
	}
	if okL {
		m.landed = ptr(round2(lc))
	}
	if okF {
		m.fulfillment = ptr(round2(ff))
	}
	if okP {
		m.paymentFee = ptr(round2(pf))
	}
	if okT {
		m.tax = ptr(round2(tax))
	}
	if okO {
		io := int(orders)
		m.orders = &io
	}
	if okN {
		no := int(newOrders)
		m.newOrders = &no
	}
	return m
}

func currenciesIn(facts []ecomfact.StorefrontDayFact, only string) []string {
	set := map[string]bool{}
	for _, f := range facts {
		set[f.Currency] = true
	}
	out := make([]string, 0, len(set))
	for c := range set {
		if only == "" || c == only {
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}

func filterByCurrency(facts []ecomfact.StorefrontDayFact, currency string) []ecomfact.StorefrontDayFact {
	out := make([]ecomfact.StorefrontDayFact, 0, len(facts))
	for _, f := range facts {
		if f.Currency == currency {
			out = append(out, f)
		}
	}
	return out
}

func paidSpend(ads []ecomfact.CampaignDayFact, currency string) *float64 {
	total := 0.0
	seen := false
	for _, ad := range ecomfact.HighestCampaignDays(ads) {
		if ad.Currency != currency {
			continue
		}
		total += ad.SpendAmount
		seen = true
	}
	if !seen {
		return nil
	}
	v := round2(total)
	return &v
}

func windowOf(p Period) (ecomfact.Window, error) {
	switch p.Kind {
	case PeriodWeekly:
		if p.From.IsZero() || p.To.IsZero() {
			return ecomfact.Window{}, fmt.Errorf("sitepnl: 周度期间需要 from/to")
		}
		return ecomfact.Window{From: p.From, To: p.To}, nil
	case PeriodMonthly, "":
		if len(p.Month) != 7 {
			return ecomfact.Window{}, fmt.Errorf("sitepnl: 月度期间需要 month=YYYY-MM")
		}
		start, err := time.Parse("2006-01", p.Month)
		if err != nil {
			return ecomfact.Window{}, fmt.Errorf("sitepnl: 非法月份 %q", p.Month)
		}
		end := start.AddDate(0, 1, 0).AddDate(0, 0, -1)
		return ecomfact.Window{From: start, To: end}, nil
	default:
		return ecomfact.Window{}, fmt.Errorf("sitepnl: 未知期间粒度 %q", p.Kind)
	}
}

func nilIfAny(vs ...*float64) *float64 {
	for _, v := range vs {
		if v == nil {
			return nil
		}
	}
	return vs[0]
}

func nilIf(a, b *float64) *float64 {
	if a == nil || b == nil {
		return nil
	}
	return a
}

// deriveSubtotal 子合计推导：任一子行 nil ⇒ 合计 nil（构造性质，不做事后恒等检查）。
func deriveSubtotal(parts []*float64, combine func([]*float64) float64) *float64 {
	vals := make([]*float64, len(parts))
	for i, p := range parts {
		if p == nil {
			return nil
		}
		vals[i] = p
	}
	v := round2(combine(vals))
	return &v
}

func addAll(a, b *float64) *float64 {
	if a == nil || b == nil {
		return nil
	}
	v := round2(*a + *b)
	return &v
}

func neg(v *float64) *float64 {
	if v == nil {
		return nil
	}
	n := -*v
	return &n
}

func ptr(v float64) *float64 { return &v }

func round2(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return v
	}
	return math.Round(v*100) / 100
}
