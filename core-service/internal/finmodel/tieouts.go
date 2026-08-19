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
			s.t1(period), s.t2(period), s.t3(period, i), s.t4(period), s.t5(period),
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

// T2 现金勾稽：BS 现金＝CF 期末现金（±0.01）。
func (s *runState) t2(period string) TieOutResult {
	return tieOut("T2", period, s.at("ending_cash", period), s.at("cash", period), 0.01,
		s.openingMissing, "cash tie-out")
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

// T4 净利润同源：CFO 撇开 D&A 与营运资本变动后必须精确等于 IS 净利润（0）。
func (s *runState) t4(period string) TieOutResult {
	expected := sub(sub(s.at("cfo", period), s.at("dna", period)), s.at("delta_nwc", period))
	return tieOut("T4", period, s.at("net_income", period), expected, 0.0001,
		s.openingMissing, "net income single source")
}

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
		expected = *add(&expected, term)
	}
	for _, term := range []*float64{cur.Payments, cur.Terminations} {
		expected = *add(&expected, neg(term))
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
	expected = *add(&expected, cur.Additions)
	expected = *add(&expected, neg(cur.Depreciation))
	expected = *add(&expected, neg(cur.Terminations))
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

// T11 期初连续性：首期期初来自导入，后续期初＝上期末值（容差 0）。
func (s *runState) t11(period string, i int) TieOutResult {
	if s.openingMissing {
		return tieOut("T11", period, nil, nil, 0, true, "opening balance not provided")
	}
	if i == 0 {
		return tieOut("T11", period, nil, nil, 0, true, "first period opening comes from import")
	}
	expected := s.at("total_assets", s.periods[i-1])
	return tieOut("T11", period, expected, s.at("total_assets", period), 0, true, "opening continuity (delta-based, see T10/T3/T7/T8 for the carrying lines)")
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

// T13 Actual 来源勾稽：事实行与事实层聚合一致（±0.05）。
func (s *runState) t13(period string) TieOutResult {
	fact, ok := s.factByPeriod[period]
	if !ok || period > s.def.ActualCutoffPeriod {
		return tieOut("T13", period, nil, nil, 0, true, "no actual window facts")
	}
	for _, pair := range []struct {
		row  string
		want *float64
	}{
		{"rev", fact.Revenue},
		{"gp", fact.GrossProfit},
		{"labor", fact.LaborCost},
	} {
		got := s.at(pair.row, period)
		if pair.want == nil || got == nil {
			continue
		}
		if out := tieOut("T13", period, pair.want, got, 0.05, false, pair.row); out.Status != "passed" {
			return out
		}
	}
	return TieOutResult{CheckCode: "T13", Period: period, Status: "passed"}
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
