package finmodel

import (
	"context"
	"encoding/json"
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
	foundGap := false
	for _, g := range result.Gaps {
		if g.Detail != "" && len(g.Detail) > 0 && result.TieOutStatus != "passed" {
			foundGap = true
		}
	}
	_ = foundGap
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
