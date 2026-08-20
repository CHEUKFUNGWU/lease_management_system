package finmodel

import (
	"fmt"

	"github.com/lease-management-system/core-service/internal/finmodel/template"
)

// evaluateTieOuts executes T1–T16 for every period and returns them as
// values. Every check is an identity of the value table or the injected
// engine projections — the reverse tests mutate those and assert the flip.
func (s *runState) evaluateTieOuts() []TieOutResult {
	var outs []TieOutResult
	for i, period := range s.periods {
		outs = append(outs,
			s.t1(period), s.t2(period, i), s.t3(period, i), s.t4(period), s.t5(period),
			s.t6(period, i), s.t7(period, i), s.t8(period, i), s.t9(period),
			s.t10(period, i), s.t11(period, i), s.t12(period),
			s.t13(period), s.t14(period), s.t15(period), s.t16(period),
		)
	}
	return outs
}

func (s *runState) at(row, period string) *float64 { return s.values[row][period] }

// hasOpening reports a usable, gate-passed opening balance.
func (s *runState) hasOpening() bool { return s.opening != nil && !s.openingMissing }

func tieOut(code, period string, expected, actual *float64, tolerance float64, notApplicable bool, note string) TieOutResult {
	out := TieOutResult{CheckCode: code, Period: period, Expected: expected, Actual: actual}
	if notApplicable {
		out.Status = "not_applicable"
		return out
	}
	if expected == nil || actual == nil {
		out.Status = "failed"
		return out
	}
	diff := *actual - *expected
	out.Diff = &diff
	if diff > tolerance || diff < -tolerance {
		out.Status = "failed"
		return out
	}
	out.Status = "passed"
	return out
}

func add(a, b *float64) *float64 {
	if a == nil || b == nil {
		return nil
	}
	v := *a + *b
	return &v
}

func sub(a, b *float64) *float64 {
	if a == nil || b == nil {
		return nil
	}
	v := *a - *b
	return &v
}

// T1 资产负债表恒等：资产合计＝负债合计＋权益合计（±0.01）。
func (s *runState) t1(period string) TieOutResult {
	return tieOut("T1", period,
		add(s.at("total_liabilities", period), s.at("total_equity", period)),
		s.at("total_assets", period), 0.01,
		!s.hasOpening(), "balance sheet identity")
}

// T2 现金勾稽/构造：BS 货币资金必须由 CF 期末现金回填（D-S5，无 plug），
// 且期末现金按滚动恒等式成立 ending_cash_t = 期初现金_t + 现金净变动_t。
// 现金由 CF 回填是构造保证（PRD 附录 B 已表述为构造断言）——检查的作用是
// 抓住“值表里出现一个与 CF 脱钩的现金行 / 现金净变动被改”这类破坏。
func (s *runState) t2(period string, i int) TieOutResult {
	cash := s.at("cash", period)
	ending := s.at("ending_cash", period)
	// 第一重：BS 现金 = CF 期末现金（构造断言：现金由 CF 回填，无 plug）。
	if out := tieOut("T2", period, ending, cash, 0.01, false, "BS cash returns CF ending cash (D-S5)"); out.Status != "passed" {
		return out
	}
	// 第二重：滚动恒等式。期初现金_t = 期初导入（首期）或上期现金值。
	var prior *float64
	if i == 0 {
		if s.openingMissing {
			return tieOut("T2", period, nil, nil, 0, true, "opening balance not provided")
		}
		prior = s.openingValues["cash"]
	} else {
		prior = s.at("cash", s.periods[i-1])
	}
	expected := add(prior, s.at("net_cash_flow", period))
	return tieOut("T2", period, ending, expected, 0.01, false, "ending cash rolls from prior cash + net flow")
}

// T3 留存收益滚动：RE_t＝RE_{t-1}＋NI_t−股利_t（±0.01）。
func (s *runState) t3(period string, i int) TieOutResult {
	if !s.hasOpening() && i == 0 {
		return tieOut("T3", period, nil, nil, 0, true, "opening balance not provided")
	}
	var opening *float64
	if i == 0 {
		v := s.opening.Periods[0].Lines["retained_earnings"]
		opening = &v
	} else {
		opening = s.at("retained_earnings", s.periods[i-1])
	}
	expected := add(add(opening, s.at("net_income", period)), neg(s.at("dividends", period)))
	return tieOut("T3", period, expected, s.at("retained_earnings", period), 0.01,
		false, "retained earnings roll")
}

func neg(v *float64) *float64 {
	if v == nil {
		return nil
	}
	out := -*v
	return &out
}

// T4 净利润同源：CF 的起点行必须引用 IS 净利润行本身（行源引用断言，同一
// run 同一行源），且数值上 CFO 撇开 D&A、营运资本变动并把按列报政策移出
// 的利息加回后必须精确等于 IS 净利润（0）。financing 列报下 CFO 已扣除利息，
// 加回正是把它还原到净利润的“起点”位置。
func (s *runState) t4(period string) TieOutResult {
	if !s.cfAnchorIsNetIncome() {
		return tieOut("T4", period, nil, nil, 0, false, "CF anchor row must reference the IS net_income row (same row source)")
	}
	if s.openingMissing {
		// 无期初时间接法 CF 的 ΔNWC/起点不可判定：保持降级，不判 failed。
		return tieOut("T4", period, nil, nil, 0, true, "opening balance not provided")
	}
	expected := sub(sub(s.at("cfo", period), s.at("dna", period)), s.at("delta_nwc", period))
	expected = add(expected, s.interestPresented(period))
	return tieOut("T4", period, s.at("net_income", period), expected, 0.0001,
		false, "net income single source (CF anchor ≡ IS net_income)")
}

// cfAnchorIsNetIncome is T4's 行源引用断言: the CF starting row (cfo) must
// reference the exact IS net-income row key — a template where CF starts from
// a separately-computed income row fails here, whatever the numbers say.
func (s *runState) cfAnchorIsNetIncome() bool {
	for _, row := range s.def.Template.Rows {
		if row.Key != "cfo" || row.Formula == nil {
			continue
		}
		for _, dep := range row.Formula.Deps() {
			if dep == "net_income" {
				return true
			}
		}
	}
	return false
}

// interestPresented returns the cash interest the financing policy moved out
// of CFO (zero under operating) — the add-back T4 needs to reconcile the CF
// anchor to IS net income.
func (s *runState) interestPresented(period string) *float64 {
	if s.def.Policy.InterestCashFlowPresentation != "financing" {
		return &zero
	}
	return add(s.at("lease_interest", period), s.at("borrow_interest", period))
}

var zero = 0.0

// T5 折旧摊销同源：D&A＝其他折旧摊销＋ROU 折旧（±0.01）。
func (s *runState) t5(period string) TieOutResult {
	return tieOut("T5", period,
		add(s.at("dna_other", period), s.at("rou_dep", period)),
		s.at("dna", period), 0.01, false, "D&A single source")
}

// T6 营运资本勾稽：CF 变动＝BS 净营运资本减少额（±0.01）。
func (s *runState) t6(period string, i int) TieOutResult {
	if !s.hasOpening() && i == 0 {
		return tieOut("T6", period, nil, nil, 0, true, "opening balance not provided")
	}
	var opening *float64
	if i == 0 {
		v := s.opening.Periods[0].Lines["ar"] + s.opening.Periods[0].Lines["inventory"] - s.opening.Periods[0].Lines["ap"]
		opening = &v
	} else {
		opening = s.at("nwc", s.periods[i-1])
	}
	expected := sub(opening, s.at("nwc", period))
	return tieOut("T6", period, expected, s.at("delta_nwc", period), 0.01,
		s.openingMissing, "working capital delta")
}

// T7 租赁负债滚动：唯一来源为计量引擎 roll-forward（容差 0）。
func (s *runState) t7(period string, i int) TieOutResult {
	cur, okCur := s.leaseByPeriod[period]
	if !okCur {
		return tieOut("T7", period, nil, nil, 0, true, "no engine lease projection")
	}
	var opening float64
	if i == 0 {
		if !s.hasOpening() {
			return tieOut("T7", period, nil, nil, 0, true, "no opening lease reference")
		}
		opening = s.opening.Periods[0].Lines["lease_liability"]
	} else {
		prev, ok := s.leaseByPeriod[s.periods[i-1]]
		if !ok || prev.LeaseLiability == nil {
			return tieOut("T7", period, nil, nil, 0, true, "no prior engine lease projection")
		}
		opening = *prev.LeaseLiability
	}
	expected := opening
	for _, term := range []*float64{cur.Additions, cur.Interest, cur.Remeasurements} {
		updated := add(&expected, term)
		if updated == nil {
			return tieOut("T7", period, nil, nil, 0, true, "lease roll-forward term missing (additions/interest/remeasurements)")
		}
		expected = *updated
	}
	for _, term := range []*float64{cur.Payments, cur.Terminations} {
		updated := add(&expected, neg(term))
		if updated == nil {
			return tieOut("T7", period, nil, nil, 0, true, "lease roll-forward term missing (payments/terminations)")
		}
		expected = *updated
	}
	return tieOut("T7", period, &expected, cur.LeaseLiability, 0, false, "lease liability roll-forward (engine-only)")
}

// T8 ROU 滚动：唯一来源为计量引擎（容差 0）。
func (s *runState) t8(period string, i int) TieOutResult {
	cur, okCur := s.leaseByPeriod[period]
	if !okCur {
		return tieOut("T8", period, nil, nil, 0, true, "no engine lease projection")
	}
	var opening float64
	if i == 0 {
		if !s.hasOpening() {
			return tieOut("T8", period, nil, nil, 0, true, "no opening lease reference")
		}
		opening = s.opening.Periods[0].Lines["rou_asset"]
	} else {
		prev, ok := s.leaseByPeriod[s.periods[i-1]]
		if !ok || prev.ROUAsset == nil {
			return tieOut("T8", period, nil, nil, 0, true, "no prior engine lease projection")
		}
		opening = *prev.ROUAsset
	}
	expected := opening
	for _, term := range []*float64{cur.Additions, neg(cur.Depreciation), neg(cur.Terminations)} {
		updated := add(&expected, term)
		if updated == nil {
			return tieOut("T8", period, nil, nil, 0, true, "ROU roll-forward term missing")
		}
		expected = *updated
	}
	return tieOut("T8", period, &expected, cur.ROUAsset, 0, false, "ROU roll-forward (engine-only)")
}

// T9 租赁付款拆分：付款＝本金＋利息（±0.01）；政策开关缺失已在 Run 入口拒绝。
func (s *runState) t9(period string) TieOutResult {
	return tieOut("T9", period,
		add(s.at("lease_principal", period), s.at("lease_interest", period)),
		s.at("lease_payments", period), 0.01, false,
		fmt.Sprintf("lease payment split (interest presented in %s)", s.def.Policy.InterestCashFlowPresentation))
}

// T10 长期资产滚动：PPE_t＝PPE_{t-1}＋CAPEX−折旧（±0.01）。
func (s *runState) t10(period string, i int) TieOutResult {
	if !s.hasOpening() && i == 0 {
		return tieOut("T10", period, nil, nil, 0, true, "opening balance not provided")
	}
	var opening *float64
	if i == 0 {
		v := s.opening.Periods[0].Lines["ppe"]
		opening = &v
	} else {
		opening = s.at("ppe", s.periods[i-1])
	}
	expected := add(add(opening, s.at("capex", period)), neg(s.at("dna_other", period)))
	return tieOut("T10", period, expected, s.at("ppe", period), 0.01, false, "PP&E roll")
}

// T11 期初连续性：逐存量行的 opening_t = closing_{t−1}（容差 0）。这里比较
// 的是「本次 run 实际用作 carry 基值的 lag 输入」与「上期间已物化的期末值」
// ——如果一个上期末值在 run 后被改（版本切换 / 私改前值），本期的 carry 基值
// 与新物化的前值不再一致，T11 立即红。首期（Opening = 期初导入版本）验证
// 每根滚动行都从导入取得了基值。
func (s *runState) t11(period string, i int) TieOutResult {
	if s.openingMissing {
		return tieOut("T11", period, nil, nil, 0, true, "opening balance not provided")
	}
	if i == 0 {
		// 首期：期初导入必须为每根实际滚动的行提供基值。
		for ref := range s.lagUsed[period] {
			if s.openingValues[ref] == nil {
				return tieOut("T11", period, nil, nil, 0, false, "opening import missing carry basis for "+ref)
			}
		}
		return TieOutResult{CheckCode: "T11", Period: period, Status: "passed"}
	}
	priorPeriod := s.periods[i-1]
	for ref, basis := range s.lagBasis[period] {
		if basis == nil {
			continue
		}
		prior := s.at(ref, priorPeriod)
		if prior == nil {
			return tieOut("T11", period, prior, basis, 0, false, "prior closing missing for "+ref)
		}
		if out := tieOut("T11", period, prior, basis, 0, false, "opening "+ref+" = closing "+ref+" prior"); out.Status != "passed" {
			return out
		}
	}
	return TieOutResult{CheckCode: "T11", Period: period, Status: "passed"}
}

// T12 科目树合计守恒：每个 subtotal＝Σ(符号×子行)。
func (s *runState) t12(period string) TieOutResult {
	anyPassed := false
	for _, row := range s.def.Template.Rows {
		if row.Kind != template.RowSubtotal {
			continue
		}
		var sum float64
		complete := true
		for _, child := range row.Children {
			v := s.at(child, period)
			if v == nil {
				complete = false
				break
			}
			sum += *v * row.ChildSign(child)
		}
		if !complete {
			continue
		}
		anyPassed = true
		if out := tieOut("T12", period, &sum, s.at(row.Key, period), 0.01, false, "subtotal "+row.Key); out.Status != "passed" {
			return out
		}
	}
	if !anyPassed {
		return tieOut("T12", period, nil, nil, 0, true, "no complete subtotals")
	}
	return TieOutResult{CheckCode: "T12", Period: period, Status: "passed"}
}

// T13 Actual 来源勾稽：事实行与事实层聚合一致（±0.05）。覆盖收入 / 毛利 /
// 人工 / 占用成本（固定租金＋变量租金——经营口径的租金费用，PRD T13）。
func (s *runState) t13(period string) TieOutResult {
	fact, ok := s.factByPeriod[period]
	if !ok || period > s.def.ActualCutoffPeriod {
		return tieOut("T13", period, nil, nil, 0, true, "no actual window facts")
	}
	// 「占用成本 = 固定租金 + 变量租金」：事实侧是两个事实字段之和，模型侧
	// 是两行租金行之和——两条独立路径，任何一侧被改都必红。
	occWant := add(fact.FixedRent, fact.VariableRent)
	occGot := add(s.at("fixed_rent", period), s.at("variable_rent", period))
	pairs := []struct {
		row  string
		want *float64
		got  *float64
	}{
		{"rev", fact.Revenue, s.at("rev", period)},
		{"gp", fact.GrossProfit, s.at("gp", period)},
		{"labor", fact.LaborCost, s.at("labor", period)},
		{"occupancy_cost", occWant, occGot},
	}
	for _, pair := range pairs {
		if pair.want == nil || pair.got == nil {
			continue
		}
		if out := tieOut("T13", period, pair.want, pair.got, 0.05, false, rowLabelFor(s.def.Template, pair.row)); out.Status != "passed" {
			return out
		}
	}
	return TieOutResult{CheckCode: "T13", Period: period, Status: "passed"}
}

func rowLabelFor(tmpl *template.Template, key string) string {
	for _, row := range tmpl.Rows {
		if row.Key == key {
			return row.Label
		}
	}
	return key
}

func rowForSource(tmpl *template.Template, source string) string {
	for _, row := range tmpl.Rows {
		if row.Kind == template.RowLink && row.Source == source {
			return row.Key
		}
	}
	return ""
}

// T14 币种纪律：单币种模型必须声明本位币（无声明即失败）。
func (s *runState) t14(period string) TieOutResult {
	if s.def.Currency == "" {
		return TieOutResult{CheckCode: "T14", Period: period, Status: "failed"}
	}
	return TieOutResult{CheckCode: "T14", Period: period, Status: "passed"}
}

// T15 口径隔离：模板级结构保证（Parse 拒绝混行），运行时复核每个小计不跨口径。
func (s *runState) t15(period string) TieOutResult {
	for _, row := range s.def.Template.Rows {
		if row.Kind != template.RowSubtotal || row.Basis == template.BasisShared {
			continue
		}
		for _, child := range row.Children {
			childBasis := basisOfRow(s.def.Template, child)
			if childBasis != template.BasisShared && childBasis != row.Basis {
				return TieOutResult{CheckCode: "T15", Period: period, Status: "failed"}
			}
		}
	}
	return TieOutResult{CheckCode: "T15", Period: period, Status: "passed"}
}

func basisOfRow(tmpl *template.Template, key string) template.Basis {
	for _, row := range tmpl.Rows {
		if row.Key == key {
			return row.Basis
		}
	}
	return template.Basis("")
}

// T16 模拟标识贯穿：每个产出的 provenance 都带 data_classification，且取值合法。
func (s *runState) t16(period string) TieOutResult {
	if s.in.DataClassification != "production" && s.in.DataClassification != "simulated" && s.in.DataClassification != "mixed" {
		return TieOutResult{CheckCode: "T16", Period: period, Status: "failed"}
	}
	for _, row := range s.def.Template.Rows {
		prov, ok := s.prov[row.Key+"\x00"+period]
		if !ok || prov.DataClassification == "" {
			return TieOutResult{CheckCode: "T16", Period: period, Status: "failed"}
		}
		if s.values[row.Key][period] != nil && prov.DataClassification != s.in.DataClassification {
			return TieOutResult{CheckCode: "T16", Period: period, Status: "failed"}
		}
	}
	return TieOutResult{CheckCode: "T16", Period: period, Status: "passed"}
}
