package finmodel

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"testing"

	"github.com/lease-management-system/core-service/internal/finmodel/opening"
	"github.com/lease-management-system/core-service/internal/finmodel/template"
)

// ── memory adapters (replacement, not accumulation) ─────────────────────

type memFacts struct{ byPeriod map[string]OperatingFacts }

func (m memFacts) Operating(_ context.Context, _, period string) (OperatingFacts, error) {
	if m.byPeriod == nil {
		return OperatingFacts{}, nil
	}
	return m.byPeriod[period], nil
}

type memLease struct{ byPeriod map[string]LeaseMonth }

func (m memLease) Monthly(_ context.Context, _, period string) (LeaseMonth, error) {
	if m.byPeriod == nil {
		return LeaseMonth{}, nil
	}
	return m.byPeriod[period], nil
}

type memSched struct{ byPeriod map[string]ScheduleFanout }

func (m memSched) Monthly(_ context.Context, _, period string) (ScheduleFanout, error) {
	if m.byPeriod == nil {
		return ScheduleFanout{}, nil
	}
	return m.byPeriod[period], nil
}

type memAssumptions struct {
	byKey     map[string]float64
	missingOK bool
}

func (m memAssumptions) Value(_ context.Context, _, key, _ string) (json.RawMessage, error) {
	v, ok := m.byKey[key]
	if !ok {
		return nil, nil
	}
	return json.Marshal(v)
}

type memOpening struct {
	bal    *opening.OpeningBalance
	ref    []opening.ContractBalance
	engine []opening.ContractBalance
	policy opening.MergePolicy
}

func (m memOpening) Get(_ context.Context, _ string) (*opening.OpeningBalance, []opening.ContractBalance, []opening.ContractBalance, opening.MergePolicy, error) {
	return m.bal, m.ref, m.engine, m.policy, nil
}

// ── fixtures ─────────────────────────────────────────────────────────────

func f(v float64) *float64 { return &v }

func goldenTemplate(t *testing.T) *template.Template {
	t.Helper()
	tmpl, err := template.DefaultStatementTemplate()
	if err != nil {
		t.Fatal(err)
	}
	return tmpl
}

func goldenDef(t *testing.T) ModelDef {
	return ModelDef{
		Name: "golden", LegalEntityID: "LE-1", Currency: "CNY",
		Template:    goldenTemplate(t),
		PeriodStart: "2026-01", HistoricalMonths: 2, ForecastMonths: 2,
		ActualCutoffPeriod: "2026-02",
		Policy:             ModelPolicy{Version: "p1", InterestCashFlowPresentation: "financing"},
	}
}

func goldenInputs() ModelInputs {
	assumps := memAssumptions{byKey: map[string]float64{
		"gross_margin_rate": 0.4, "borrow_interest_rate": 0.05, "tax_rate": 0.25,
		"dso": 10, "dio": 20, "dpo": 15, "days": 30, "dividend_payout_rate": 0,
		"sssg": 0.02, "labor_cost_growth": 0.01, "fixed_rent_growth": 0.02,
		"variable_rent_growth": 0.02, "non_lease_cost_growth": 0.01,
		"other_controllable_cost_growth": 0.02, "borrowings": 200,
	}}
	facts := memFacts{byPeriod: map[string]OperatingFacts{
		"2026-01": {Revenue: f(1000), GrossProfit: f(400), LaborCost: f(200), FixedRent: f(150), VariableRent: f(50), NonLeaseCost: f(60), OtherControllableCost: f(40), DecisionReady: true, DataClassification: "production"},
		"2026-02": {Revenue: f(1100), GrossProfit: f(440), LaborCost: f(220), FixedRent: f(165), VariableRent: f(55), NonLeaseCost: f(66), OtherControllableCost: f(44), DecisionReady: true, DataClassification: "production"},
	}}
	lease := memLease{byPeriod: map[string]LeaseMonth{
		"2026-01": {ROUAsset: f(1150), LeaseLiability: f(910), Interest: f(10), Depreciation: f(50), Payments: f(100), Principal: f(90), Additions: f(0), Remeasurements: f(0), Terminations: f(0)},
		"2026-02": {ROUAsset: f(1100), LeaseLiability: f(820), Interest: f(10), Depreciation: f(50), Payments: f(100), Principal: f(90), Additions: f(0), Remeasurements: f(0), Terminations: f(0)},
		"2026-03": {ROUAsset: f(1050), LeaseLiability: f(730), Interest: f(10), Depreciation: f(50), Payments: f(100), Principal: f(90), Additions: f(0), Remeasurements: f(0), Terminations: f(0)},
		"2026-04": {ROUAsset: f(1000), LeaseLiability: f(640), Interest: f(10), Depreciation: f(50), Payments: f(100), Principal: f(90), Additions: f(0), Remeasurements: f(0), Terminations: f(0)},
	}}
	sched := memSched{byPeriod: map[string]ScheduleFanout{
		"2026-01": {Capex: f(50), OtherDepreciation: f(20), ShareCapital: f(500), ServiceFee: f(0), Borrowings: f(200)},
		"2026-02": {Capex: f(50), OtherDepreciation: f(20), ShareCapital: f(500), ServiceFee: f(0), Borrowings: f(200)},
		"2026-03": {Capex: f(50), OtherDepreciation: f(20), ShareCapital: f(500), ServiceFee: f(0), Borrowings: f(200)},
		"2026-04": {Capex: f(50), OtherDepreciation: f(20), ShareCapital: f(500), ServiceFee: f(0), Borrowings: f(200)},
	}}
	balance := &opening.OpeningBalance{
		LegalEntityID: "LE-1", Currency: "CNY",
		Periods: []opening.PeriodBalance{{
			Period:  "2026-01",
			Lines:   map[string]float64{"cash": 500, "ar": 100, "inventory": 60, "ap": 40, "ppe": 300, "rou_asset": 1200, "lease_liability": 1000, "borrowings": 200, "share_capital": 500, "retained_earnings": 420},
			Mapping: map[string]string{"1101": "cash"},
		}},
	}
	return ModelInputs{
		Facts: facts, Lease: lease, Schedules: sched, Assumptions: assumps,
		Opening: memOpening{bal: balance,
			ref:    []opening.ContractBalance{{ContractID: "C1", LeaseLiability: 1000, ROUAsset: 1200}},
			engine: []opening.ContractBalance{{ContractID: "C1", LeaseLiability: 1000, ROUAsset: 1200}},
			policy: opening.MergePolicy{Version: "v1"}},
		Versions:           VersionSet{Data: "ds-1", Assumption: "as-1", ExchangeRate: "fx-1", MetricDefinition: "md-1", ModelDefinition: "v1"},
		DataClassification: "production",
	}
}

func lineValue(t *testing.T, result *RunResult, row, period string) *float64 {
	t.Helper()
	for _, line := range result.Lines {
		if line.RowKey == row && line.Period == period {
			return line.Value
		}
	}
	t.Fatalf("missing line %s@%s", row, period)
	return nil
}

func closeTo(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

// ── Golden 模型对数 ─────────────────────────────────────────────────────

func TestGoldenModel(t *testing.T) {
	result, err := Run(context.Background(), goldenDef(t), goldenInputs())
	if err != nil {
		t.Fatal(err)
	}
	if got := lineValue(t, result, "rev", "2026-01"); got == nil || !closeTo(*got, 1000, 0.001) {
		t.Fatalf("rev@01 = %v, want 1000", got)
	}
	if got := lineValue(t, result, "net_income", "2026-01"); got == nil || !closeTo(*got, 7.5, 0.01) {
		t.Fatalf("net_income@01 = %v, want 7.5", got)
	}
	if got := lineValue(t, result, "ending_cash", "2026-01"); got == nil || !closeTo(*got, 124.1667, 0.02) {
		t.Fatalf("ending_cash@01 = %v, want ~124.17", got)
	}
	// Forecast revenue drives from SSSG on the run-rate.
	if got := lineValue(t, result, "rev", "2026-03"); got == nil || !closeTo(*got, 1122, 0.01) {
		t.Fatalf("rev@03 = %v, want 1122 (1100 × 1.02)", got)
	}
	if result.TieOutStatus != "passed" {
		t.Fatalf("golden tie-outs must pass, got %s: %+v", result.TieOutStatus, result.TieOuts)
	}
}

// 相同输入重跑必须得到相同结果（复演性）。
func TestRunDeterminism(t *testing.T) {
	def := goldenDef(t)
	in := goldenInputs()
	r1, err := Run(context.Background(), def, in)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := Run(context.Background(), def, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(r1.Lines) != len(r2.Lines) || r1.TieOutStatus != r2.TieOutStatus {
		t.Fatalf("rerun diverged: %d vs %d lines, %s vs %s", len(r1.Lines), len(r2.Lines), r1.TieOutStatus, r2.TieOutStatus)
	}
	for i := range r1.Lines {
		a, b := r1.Lines[i], r2.Lines[i]
		if (a.Value == nil) != (b.Value == nil) || (a.Value != nil && *a.Value != *b.Value) {
			t.Fatalf("line %d diverged: %+v vs %+v", i, a, b)
		}
	}
}

// ── 降级：缺期初 BS ─────────────────────────────────────────────────────

func TestWithoutOpeningBalanceDegradesHonestly(t *testing.T) {
	def := goldenDef(t)
	in := goldenInputs()
	in.Opening = nil
	result, err := Run(context.Background(), def, in)
	if err != nil {
		t.Fatal(err)
	}
	// IS still computes its formula chain where inputs exist.
	if got := lineValue(t, result, "gp", "2026-01"); got == nil || !closeTo(*got, 400, 0.01) {
		t.Fatalf("IS must be unaffected, gp@01 = %v", got)
	}
	// 缺期初的诚实降级必须留下原因说明（不是无声返回空）。
	foundOpeningGap := false
	for _, g := range result.Gaps {
		if g.Kind == "opening_missing" {
			foundOpeningGap = true
		}
	}
	if !foundOpeningGap {
		t.Fatalf("缺期初必须以 gap 说明降级原因，got gaps=%+v", result.Gaps)
	}
	// 无期初时首期留存收益/营运资本/长期资产滚动必须 not_applicable，不允许假平衡。
	for _, out := range result.TieOuts {
		if out.CheckCode == "T3" || out.CheckCode == "T6" || out.CheckCode == "T10" {
			if out.Period == "2026-01" && out.Status != "not_applicable" {
				t.Fatalf("%s@01 must be not_applicable without opening, got %s", out.CheckCode, out.Status)
			}
		}
	}
}

// ── 勾稽反向测试：先证明破坏后必红 ────────────────────────────────────

func prepareState(t *testing.T, mutate func(s *runState)) *runState {
	t.Helper()
	def := goldenDef(t)
	in := goldenInputs()
	state, err := newRunState(def, in)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.loadOpening(context.Background()); err != nil {
		t.Fatal(err)
	}
	state.loadPeriods(context.Background())
	mutate(state)
	return state
}

func statusOf(t *testing.T, outs []TieOutResult, code, period string) string {
	t.Helper()
	for _, out := range outs {
		if out.CheckCode == code && out.Period == period {
			return out.Status
		}
	}
	t.Fatalf("missing tie-out %s@%s", code, period)
	return ""
}

func requireGreen(t *testing.T, state *runState, code, period string) {
	if got := statusOf(t, state.evaluateTieOuts(), code, period); got != "passed" {
		t.Fatalf("%s@%s must be passed on the intact model, got %s", code, period, got)
	}
}

func requireRed(t *testing.T, state *runState, code, period string) {
	if got := statusOf(t, state.evaluateTieOuts(), code, period); got != "failed" {
		t.Fatalf("%s@%s must flip to failed after corruption, got %s", code, period, got)
	}
}

func TestTieOutReverseT1(t *testing.T) {
	s := prepareState(t, func(s *runState) {})
	requireGreen(t, s, "T1", "2026-01")
	s = prepareState(t, func(s *runState) { *s.values["total_assets"]["2026-01"] += 10 })
	requireRed(t, s, "T1", "2026-01")
}

func TestTieOutReverseT3AndT5(t *testing.T) {
	s := prepareState(t, func(s *runState) { *s.values["retained_earnings"]["2026-01"] += 5 })
	requireRed(t, s, "T3", "2026-01")
	s = prepareState(t, func(s *runState) { *s.values["dna"]["2026-01"] += 1 })
	requireRed(t, s, "T5", "2026-01")
}

func TestTieOutReverseT7EngineOnly(t *testing.T) {
	s := prepareState(t, func(s *runState) {})
	requireGreen(t, s, "T7", "2026-01")
	// Cut the engine cache: model-side corruption of the lease projection.
	s = prepareState(t, func(s *runState) {
		m := s.leaseByPeriod["2026-01"]
		m.LeaseLiability = f(999)
		s.leaseByPeriod["2026-01"] = m
	})
	requireRed(t, s, "T7", "2026-01")
}

func TestTieOutReverseT9AndT10(t *testing.T) {
	s := prepareState(t, func(s *runState) { *s.values["lease_payments"]["2026-01"] += 2 })
	requireRed(t, s, "T9", "2026-01")
	s = prepareState(t, func(s *runState) { *s.values["ppe"]["2026-01"] += 3 })
	requireRed(t, s, "T10", "2026-01")
}

func TestTieOutReverseT12Subtotal(t *testing.T) {
	s := prepareState(t, func(s *runState) { *s.values["opex"]["2026-01"] += 7 })
	// opex corruption propagates into operating_ebitda? No — operating_ebitda
	// is a formula relying on opex, so T12 on opex itself flips.
	requireRed(t, s, "T12", "2026-01")
}

func TestTieOutReverseT13ActualSource(t *testing.T) {
	s := prepareState(t, func(s *runState) { *s.values["gp"]["2026-01"] += 20 })
	requireRed(t, s, "T13", "2026-01")
}

// P1-1：T13 的占用成本对照——固定或变量租金一侧被改，对照必红。
func TestTieOutReverseT13OccupancyRow(t *testing.T) {
	s := prepareState(t, func(s *runState) {})
	requireGreen(t, s, "T13", "2026-01")
	// 事实侧不变、模型侧固定租金被改 → 占用成本两条路径不一致。
	s = prepareState(t, func(s *runState) { *s.values["fixed_rent"]["2026-01"] += 33 })
	requireRed(t, s, "T13", "2026-01")
}

func TestTieOutReverseT14Currency(t *testing.T) {
	def := goldenDef(t)
	in := goldenInputs()
	s, err := newRunState(def, in)
	if err != nil {
		t.Fatal(err)
	}
	s.def.Currency = "" // 未声明本位币
	if got := statusOf(t, s.evaluateTieOuts(), "T14", "2026-01"); got != "failed" {
		t.Fatalf("T14 must fail without a currency, got %s", got)
	}
}

func TestTieOutReverseT16Classification(t *testing.T) {
	def := goldenDef(t)
	in := goldenInputs()
	in.DataClassification = ""
	s, err := newRunState(def, in)
	if err != nil {
		t.Fatal(err)
	}
	if got := statusOf(t, s.evaluateTieOuts(), "T16", "2026-01"); got != "failed" {
		t.Fatalf("T16 must fail without classification, got %s", got)
	}
}

// ── 反向测试补齐（P0-4）：T2 / T4 / T6 / T8 / T11 / T15 ───────────────────

func TestTieOutReverseT2Cash(t *testing.T) {
	s := prepareState(t, func(s *runState) {})
	requireGreen(t, s, "T2", "2026-01")
	// 现金净变动被改 → 滚动恒等式破裂。
	s = prepareState(t, func(s *runState) { *s.values["net_cash_flow"]["2026-01"] += 30 })
	requireRed(t, s, "T2", "2026-01")
	// BS 现金行与 CF 脱钩（被改成独立值）→ 构造断言破裂。
	s = prepareState(t, func(s *runState) { *s.values["cash"]["2026-01"] = *s.values["cash"]["2026-01"] - 99 })
	requireRed(t, s, "T2", "2026-01")
}

func TestTieOutReverseT4NetIncomeSource(t *testing.T) {
	s := prepareState(t, func(s *runState) {})
	requireGreen(t, s, "T4", "2026-01")
	// 数值侧：CF 起点行（cfo）被私改 → 加回利息还原后 ≠ 净利润，T4 红。
	s = prepareState(t, func(s *runState) { *s.values["cfo"]["2026-01"] += 7 })
	requireRed(t, s, "T4", "2026-01")
}

// cfAnchorIsNetIncome 的结构断言直接测试：CF 起点引用单独的“净利润副本行”
// （数值上可能与 IS 净利润相同）也必须被判为结构违约。
func TestT4StructureRejectsSeparateCFIncomeRow(t *testing.T) {
	defRows := func(cfoFormula string) *template.Template {
		tmpl, err := template.Parse(template.TemplateDef{Name: "t4-structure", Version: 1, Rows: []template.RowDef{
			{Key: "net_income", Label: "IS 净利润", Kind: template.RowInput, Basis: template.BasisShared},
			{Key: "cf_ni", Label: "CF 起点行（副本）", Kind: template.RowInput, Basis: template.BasisShared},
			{Key: "dna", Label: "D&A", Kind: template.RowInput, Basis: template.BasisShared},
			{Key: "delta_nwc", Label: "ΔNWC", Kind: template.RowInput, Basis: template.BasisShared},
			{Key: "cfo", Label: "CFO", Kind: template.RowFormula, Basis: template.BasisShared, Formula: cfoFormula},
		}})
		if err != nil {
			t.Fatal(err)
		}
		return tmpl
	}
	base := func(key string) ModelDef {
		return ModelDef{Name: "t4", LegalEntityID: "LE-1", Currency: "CNY",
			Template: defRows(key), PeriodStart: "2026-01", HistoricalMonths: 1, ForecastMonths: 1,
			ActualCutoffPeriod: "2026-01",
			Policy:             ModelPolicy{Version: "p1", InterestCashFlowPresentation: "operating"}}
	}
	good, err := newRunState(base("rows.net_income + rows.dna + rows.delta_nwc"), ModelInputs{DataClassification: "production"})
	if err != nil {
		t.Fatal(err)
	}
	if !good.cfAnchorIsNetIncome() {
		t.Fatal("CF 引用 IS net_income 行必须通过结构断言")
	}
	bad, err := newRunState(base("rows.cf_ni + rows.dna + rows.delta_nwc"), ModelInputs{DataClassification: "production"})
	if err != nil {
		t.Fatal(err)
	}
	if bad.cfAnchorIsNetIncome() {
		t.Fatal("CF 引用独立净利润副本行必须被判结构违约（同一 run 同一行源）")
	}
}

func TestTieOutReverseT6NWC(t *testing.T) {
	s := prepareState(t, func(s *runState) {})
	requireGreen(t, s, "T6", "2026-01")
	s = prepareState(t, func(s *runState) { *s.values["delta_nwc"]["2026-01"] += 5 })
	requireRed(t, s, "T6", "2026-01")
}

func TestTieOutReverseT8ROURoll(t *testing.T) {
	s := prepareState(t, func(s *runState) {})
	requireGreen(t, s, "T8", "2026-01")
	s = prepareState(t, func(s *runState) {
		m := s.leaseByPeriod["2026-01"]
		m.ROUAsset = f(9999)
		s.leaseByPeriod["2026-01"] = m
	})
	requireRed(t, s, "T8", "2026-01")
}

func TestTieOutReverseT11OpeningContinuity(t *testing.T) {
	s := prepareState(t, func(s *runState) {})
	requireGreen(t, s, "T11", "2026-02")
	// 上一个期间的期末值被事后改动（版本切换/私改前值）→ 本期 carry 基值
	// 与新物化的前值不一致，T11 必须红。
	s = prepareState(t, func(s *runState) { *s.values["ppe"]["2026-01"] += 9 })
	requireRed(t, s, "T11", "2026-02")
}

func TestTieOutReverseT15BasisIsolation(t *testing.T) {
	s := prepareState(t, func(s *runState) {})
	requireGreen(t, s, "T15", "2026-01")
	// 把一个 shared 子行改成 IFRS 16 口径 → 经营口径小计路径混行，T15 红。
	s = prepareState(t, func(s *runState) {
		for i := range s.def.Template.Rows {
			if s.def.Template.Rows[i].Key == "labor" {
				s.def.Template.Rows[i].Basis = template.BasisIFRS16
			}
		}
	})
	requireRed(t, s, "T15", "2026-01")
}

// 政策开关缺失：run 拒绝启动（T9）。
func TestRunRefusesMissingPolicySwitch(t *testing.T) {
	def := goldenDef(t)
	def.Policy.InterestCashFlowPresentation = ""
	_, err := Run(context.Background(), def, goldenInputs())
	if err == nil {
		t.Fatal("missing interest presentation policy must refuse the run")
	}
}

func TestForecastCombinedDriversRampAndStoreGrowth(t *testing.T) {
	// 预测收入 = run-rate × (1+sssg) × (1+ramp_factor) × (1+store_count_growth)：
	// 100 × 1.02 × 1.05 × 1.10 = 117.81。
	tmpl := goldenTemplate(t)
	def := ModelDef{
		Name: "drivers", LegalEntityID: "LE-1", Currency: "CNY",
		Template: tmpl, PeriodStart: "2026-01", HistoricalMonths: 1, ForecastMonths: 2,
		ActualCutoffPeriod: "2026-01",
		Policy:             ModelPolicy{Version: "p1", InterestCashFlowPresentation: "financing"},
	}
	facts := memFacts{byPeriod: map[string]OperatingFacts{
		"2026-01": {Revenue: f(100), DecisionReady: true, DataClassification: "production"},
	}}
	assumps := memAssumptions{byKey: map[string]float64{
		"sssg": 0.02, "ramp_factor": 0.05, "store_count_growth": 0.10,
	}}
	inputs := ModelInputs{
		Assumptions: assumps, Versions: VersionSet{}, DataClassification: "production",
		Facts: facts, Lease: nil, Schedules: nil, Opening: nil,
	}
	result, err := Run(context.Background(), def, inputs)
	if err != nil {
		t.Fatal(err)
	}
	want := 100.0 * 1.02 * 1.05 * 1.10
	for _, line := range result.Lines {
		if line.RowKey == "rev" && line.Period == "2026-02" {
			if line.Value == nil || *line.Value < want-0.01 || *line.Value > want+0.01 {
				t.Fatalf("combined-driver revenue = %v, want %v", line.Value, want)
			}
			return
		}
	}
	t.Fatal("forecast revenue row missing")
}

func TestForecastCombinedDriversAreNeutralWhenUnregistered(t *testing.T) {
	// 未登记 ramp/growth：与仅 SSSG 的旧语义一致——驱动是假设，不是内置常数。
	tmpl := goldenTemplate(t)
	def := ModelDef{
		Name: "neutral", LegalEntityID: "LE-1", Currency: "CNY",
		Template: tmpl, PeriodStart: "2026-01", HistoricalMonths: 1, ForecastMonths: 2,
		ActualCutoffPeriod: "2026-01",
		Policy:             ModelPolicy{Version: "p1", InterestCashFlowPresentation: "financing"},
	}
	inputs := ModelInputs{
		Assumptions: memAssumptions{byKey: map[string]float64{"sssg": 0.02}},
		Versions:    VersionSet{}, DataClassification: "production",
		Facts: memFacts{byPeriod: map[string]OperatingFacts{
			"2026-01": {Revenue: f(100), DecisionReady: true, DataClassification: "production"},
		}},
	}
	result, err := Run(context.Background(), def, inputs)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range result.Lines {
		if line.RowKey == "rev" && line.Period == "2026-02" {
			if line.Value == nil || *line.Value < 101.99 || *line.Value > 102.01 {
				t.Fatalf("neutral drivers must preserve the sssg-only value: %v", line.Value)
			}
			return
		}
	}
	t.Fatal("forecast revenue row missing")
}

// ── P0-3：Actual 冻结线左侧只读事实，右侧才用驱动公式 ─────────────────────

func TestActualWindowReadsFactsNotAssumptions(t *testing.T) {
	def := goldenDef(t)
	in := goldenInputs()
	// 事实毛利 350 ≠ 收入 1000 × 假设毛利率 0.4：Actual 期必须输出事实值，
	// 不允许用假设推导出 400。
	facts := memFacts{byPeriod: map[string]OperatingFacts{
		"2026-01": {Revenue: f(1000), GrossProfit: f(350), LaborCost: f(200), FixedRent: f(150), VariableRent: f(50), NonLeaseCost: f(60), OtherControllableCost: f(40), DecisionReady: true, DataClassification: "production"},
		"2026-02": {Revenue: f(1100), GrossProfit: f(385), LaborCost: f(220), FixedRent: f(165), VariableRent: f(55), NonLeaseCost: f(66), OtherControllableCost: f(44), DecisionReady: true, DataClassification: "production"},
	}}
	in.Facts = facts
	result, err := Run(context.Background(), def, in)
	if err != nil {
		t.Fatal(err)
	}
	if got := lineValue(t, result, "gp", "2026-01"); got == nil || *got != 350 {
		t.Fatalf("Actual 毛利必须等于事实 350，而非假设推导 400，got %v", got)
	}
	if got := lineValue(t, result, "gp", "2026-02"); got == nil || *got != 385 {
		t.Fatalf("Actual 毛利必须等于事实 385，而非假设推导，got %v", got)
	}
	// 预测期仍由毛利率假设驱动：rev@03 = 1100×1.02 = 1122，gp@03 = 1122×0.4。
	rev03 := lineValue(t, result, "rev", "2026-03")
	gp03 := lineValue(t, result, "gp", "2026-03")
	if rev03 == nil || gp03 == nil || *gp03-*rev03*0.4 > 0.001 {
		t.Fatalf("预测期毛利由假设驱动：rev@03=%v gp@03=%v", rev03, gp03)
	}
	// T13 在 Actual 期对真实事实聚合通过。
	s, err := newRunState(def, in)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.loadOpening(context.Background()); err != nil {
		t.Fatal(err)
	}
	s.loadPeriods(context.Background())
	requireGreen(t, s, "T13", "2026-01")
}

// ── P0-5：期初三道闸失败阻止运行（已提供但带病的期初 ≠ 未提供）─────────

func TestRunRejectsSickOpeningGateFail(t *testing.T) {
	def := goldenDef(t)
	in := goldenInputs()
	// 破坏闸 1：期初自身不平（把现金减成让资产 ≠ 负债+权益）。
	broken := *in.Opening.(memOpening).bal
	broken.Periods = []opening.PeriodBalance{{
		Period: "2026-01",
		Lines:  map[string]float64{"cash": 0, "ar": 100, "inventory": 60, "ap": 40, "ppe": 300, "rou_asset": 1200, "lease_liability": 1000, "borrowings": 200, "share_capital": 500, "retained_earnings": 420},
	}}
	in.Opening = memOpening{bal: &broken, ref: goldenInputs().Opening.(memOpening).ref, engine: goldenInputs().Opening.(memOpening).engine, policy: opening.MergePolicy{Version: "v1"}}
	_, err := Run(context.Background(), def, in)
	if err == nil {
		t.Fatal("a sick (gate-failing) opening must reject the run, not degrade")
	}
	if !errors.Is(err, ErrOpeningRejected) {
		t.Fatalf("rejection must carry ErrOpeningRejected, got %v", err)
	}
	// 对照：未提供期初仍走降级而非拒绝。
	in2 := goldenInputs()
	in2.Opening = nil
	if _, err := Run(context.Background(), goldenDef(t), in2); err != nil {
		t.Fatalf("absent opening must degrade, not error: %v", err)
	}
}

// ── P0-6：利息列报政策两个分支都让 CFO/CFE 生效且现金守恒 ────────────────

func TestInterestPresentationBranchesConserveCash(t *testing.T) {
	defF := goldenDef(t) // financing（CAS 实务）
	defF.Policy.InterestCashFlowPresentation = "financing"
	defO := goldenDef(t) // operating（IAS 7 选项）
	defO.Policy.InterestCashFlowPresentation = "operating"

	runF, err := Run(context.Background(), defF, goldenInputs())
	if err != nil {
		t.Fatal(err)
	}
	runO, err := Run(context.Background(), defO, goldenInputs())
	if err != nil {
		t.Fatal(err)
	}

	// financing 下利息从 CFO 移出（备筹资列报）：两分支 CFO/CFE 必须不同。
	cfoF, cfoO := lineValue(t, runF, "cfo", "2026-01"), lineValue(t, runO, "cfo", "2026-01")
	cfeF, cfeO := lineValue(t, runF, "cfe", "2026-01"), lineValue(t, runO, "cfe", "2026-01")
	if cfoF == nil || cfoO == nil || cfeF == nil || cfeO == nil {
		t.Fatalf("cfo/cfe must compute in both branches: F=%v/%v O=%v/%v", cfoF, cfeF, cfoO, cfeO)
	}
	if *cfoF == *cfoO || *cfeF == *cfeO {
		t.Fatalf("the policy switch must change CFO/CFE: F cfo=%v cfe=%v, O cfo=%v cfe=%v", *cfoF, *cfeF, *cfoO, *cfeO)
	}
	// 利息总额不变 → 期末现金两分支一致（T2 现金守恒在两个分支下都成立）。
	if gotF, gotO := lineValue(t, runF, "ending_cash", "2026-01"), lineValue(t, runO, "ending_cash", "2026-01"); gotF == nil || gotO == nil || *gotF != *gotO {
		t.Fatalf("ending cash must be policy-invariant: F=%v O=%v", gotF, gotO)
	}
	// T1/T2 双双通过。
	sF, _ := newRunState(defF, goldenInputs())
	_ = sF.loadOpening(context.Background())
	sF.loadPeriods(context.Background())
	requireGreen(t, sF, "T1", "2026-01")
	requireGreen(t, sF, "T2", "2026-01")
	sO, _ := newRunState(defO, goldenInputs())
	_ = sO.loadOpening(context.Background())
	sO.loadPeriods(context.Background())
	requireGreen(t, sO, "T1", "2026-01")
	requireGreen(t, sO, "T2", "2026-01")
}
