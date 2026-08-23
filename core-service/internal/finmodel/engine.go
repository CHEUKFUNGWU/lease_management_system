package finmodel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/finmodel/opening"
	"github.com/lease-management-system/core-service/internal/finmodel/template"
)

// ── result types ─────────────────────────────────────────────────────────

// Provenance traces one result cell (bottom line 3).
type Provenance struct {
	SourceType         string `json:"source_type"` // fact_aggregate | ifrs16_engine | contract_schedule | assumption | formula | opening_balance | human_input
	Ref                string `json:"ref,omitempty"`
	DataVersion        string `json:"data_version,omitempty"`
	DataClassification string `json:"data_classification,omitempty"`
}

// LineValue is one result cell; Value is nil when missing (D-S4).
type LineValue struct {
	RowKey     string     `json:"row_key"`
	Period     string     `json:"period"`
	Value      *float64   `json:"value"`
	Provenance Provenance `json:"provenance"`
}

// TieOutResult is one executed check (value, not a log — D-S5).
type TieOutResult struct {
	CheckCode string   `json:"check_code"`
	Period    string   `json:"period"`
	Expected  *float64 `json:"expected"`
	Actual    *float64 `json:"actual"`
	Diff      *float64 `json:"diff"`
	Status    string   `json:"status"` // passed | failed | not_applicable
}

// DataGap is one honest degradation record.
type DataGap struct {
	Kind   string `json:"kind"`
	Period string `json:"period,omitempty"`
	Detail string `json:"detail"`
}

// VersionSet carries the five version lines (bottom line 3).
type VersionSet struct {
	Data             string `json:"data_version"`
	Assumption       string `json:"assumption_version"`
	ExchangeRate     string `json:"exchange_rate_version"`
	MetricDefinition string `json:"metric_definition_version"`
	ModelDefinition  string `json:"model_definition_version"`
}

// RunResult is the whole deterministic output of one model run.
type RunResult struct {
	Periods      []string          `json:"periods"`
	Lines        []LineValue       `json:"lines"`
	TieOuts      []TieOutResult    `json:"tie_outs"`
	Gaps         []DataGap         `json:"gaps,omitempty"`
	Versions     VersionSet        `json:"versions"`
	TieOutStatus string            `json:"tie_out_status"` // passed | failed | degraded
	Types        map[string]string `json:"-"`
}

// ModelInputs injects every reader (all pure ports).
type ModelInputs struct {
	Facts              FactReader
	Lease              LeaseRollforwardReader
	Schedules          ScheduleReader
	Assumptions        AssumptionReader
	Opening            OpeningBalanceReader
	HumanZeros map[string]HumanZero
	Versions           VersionSet
	DataClassification string
}

// HumanZero is a user-asserted zero (F1 D-F2): 「我的业务没有这一项」。它与
// 缺失在数据层是不同的值——0 + human_input provenance vs nil + 具名 Gap。
type HumanZero struct {
	ConfirmedBy string `json:"confirmed_by"`
	ConfirmedAt string `json:"confirmed_at,omitempty"`
}

// ── Run ──────────────────────────────────────────────────────────────────

// Run is the pure model engine: every input is injected, every output is a
// value (D-S2). AI evaluate, golden tests and the formal run path share this
// function, so "试算的数字" and "正式发布的数字" cannot diverge.
func Run(ctx context.Context, def ModelDef, in ModelInputs) (*RunResult, error) {
	if def.Template == nil {
		return nil, errors.New("finmodel: a parsed template is required")
	}
	// F1 D-F1：保留键是 T1–T16 的读取点。缺键在这里拦（机械后果发生地），
	// 错误列出全部缺失键与结构原因，绝不以权限问题措辞。
	rowKeys := make([]string, 0, len(def.Template.Rows))
	for _, row := range def.Template.Rows {
		rowKeys = append(rowKeys, row.Key)
	}
	if missing := template.MissingReservedSheetKeys(rowKeys); len(missing) > 0 {
		return nil, &template.ErrMissingReservedKeys{Missing: missing}
	}
	if def.Policy.InterestCashFlowPresentation == "" {
		return nil, errors.New("finmodel: policy.interest_cash_flow_presentation is required (T9 — the run refuses to start without the switch)")
	}
	if def.Policy.InterestMethod != "" && def.Policy.InterestMethod != "opening_balance" {
		return nil, fmt.Errorf("finmodel: unsupported interest method %q (only opening_balance in this release)", def.Policy.InterestMethod)
	}
	state, err := newRunState(def, in)
	if err != nil {
		return nil, err
	}
	periods := state.periods

	if err := state.loadOpening(ctx); err != nil {
		return nil, err
	}
	state.loadPeriods(ctx)

	result := &RunResult{Periods: periods, Versions: in.Versions}
	for _, row := range def.Template.Rows {
		for _, period := range periods {
			result.Lines = append(result.Lines, LineValue{
				RowKey: row.Key, Period: period,
				Value:      state.values[row.Key][period],
				Provenance: state.prov[row.Key+"\x00"+period],
			})
		}
	}
	result.TieOuts = state.evaluateTieOuts()
	result.Gaps = state.gaps
	result.TieOutStatus = classifyTieOuts(result.TieOuts)
	return result, nil
}

// classifyTieOuts: any failed check fails the run; only not_applicable (no
// applied checks) is degraded.
func classifyTieOuts(outs []TieOutResult) string {
	failed, applied := false, false
	for _, out := range outs {
		switch out.Status {
		case "failed":
			failed = true
		case "passed":
			applied = true
		}
	}
	if failed {
		return "failed"
	}
	if !applied {
		return "degraded"
	}
	return "passed"
}

// newRunState expands periods and initializes the value/provenance tables.
// Tests reuse it to reach the tie-out layer with a corrupted value table —
// every tie-out needs a break-it-and-see-red path.
func newRunState(def ModelDef, in ModelInputs) (*runState, error) {
	periods, cutoffIdx, err := expandPeriods(def)
	if err != nil {
		return nil, err
	}
	state := &runState{
		def: def, in: in, periods: periods, cutoffIdx: cutoffIdx,
		values:        map[string]map[string]*float64{},
		prov:          map[string]Provenance{},
		leaseByPeriod: map[string]LeaseMonth{},
		factByPeriod:  map[string]OperatingFacts{},
		openingValues: map[string]*float64{},
		lagBasis:      map[string]map[string]*float64{},
		lagUsed:       map[string]map[string]bool{},
	}
	for _, row := range def.Template.Rows {
		state.values[row.Key] = map[string]*float64{}
	}
	return state, nil
}

// ── run state ────────────────────────────────────────────────────────────

type runState struct {
	def            ModelDef
	in             ModelInputs
	periods        []string
	cutoffIdx      int // -1: no actuals
	values         map[string]map[string]*float64
	prov           map[string]Provenance
	gaps           []DataGap
	opening        *opening.OpeningBalance
	openingMissing bool
	openingValues  map[string]*float64             // synthetic prior period for lag across the model start
	lagBasis       map[string]map[string]*float64  // period → row → the lag-1 carry basis the engine used (T11)
	lagUsed        map[string]map[string]bool      // period → row → whether a lag-1 reference was resolved (T11, incl. nil)
	leaseByPeriod  map[string]LeaseMonth     // engine projection cache (T7/T8/T9)
	factByPeriod   map[string]OperatingFacts // fact cache (T13)
}

func expandPeriods(def ModelDef) ([]string, int, error) {
	start, err := time.Parse("2006-01", def.PeriodStart)
	if err != nil || def.HistoricalMonths < 0 || def.ForecastMonths <= 0 {
		return nil, 0, fmt.Errorf("finmodel: invalid period range (start=%q hist=%d fc=%d)", def.PeriodStart, def.HistoricalMonths, def.ForecastMonths)
	}
	months := def.HistoricalMonths + def.ForecastMonths
	periods := make([]string, 0, months)
	cutoff := -1
	for i := 0; i < months; i++ {
		m := start.AddDate(0, i, 0)
		p := m.Format("2006-01")
		if def.ActualCutoffPeriod != "" && p <= def.ActualCutoffPeriod {
			cutoff = i
		}
		periods = append(periods, p)
	}
	return periods, cutoff, nil
}

func (s *runState) setValue(rowKey, period string, value *float64, provenance Provenance) {
	s.values[rowKey][period] = value
	s.prov[rowKey+"\x00"+period] = provenance
}

// ErrOpeningRejected is the S2-3 fail-closed: an opening balance that was
// PROVIDED but fails the three gates must stop the run — a sick opening makes
// every later period wrong. It is distinct from "no opening provided", which
// degrades BS/CF honestly instead of stopping (D-S7 / PRD S2-3).
var ErrOpeningRejected = errors.New("finmodel: opening balance rejected by the three gates — BS/CF refused (期初带病，阻止运行)")

// OpeningRejectedError carries the structured gate failures behind
// ErrOpeningRejected so a caller can render or queue them (P1-1: 期初三道闸
// 不符进 fpna_data_quality_items).
type OpeningRejectedError struct {
	Failures []opening.GateFailure
	wrapped  error
}

func (e *OpeningRejectedError) Error() string { return e.wrapped.Error() }
func (e *OpeningRejectedError) Unwrap() error { return e.wrapped }

func (s *runState) loadOpening(ctx context.Context) error {
	if s.in.Opening == nil {
		s.openingMissing = true
		s.gaps = append(s.gaps, DataGap{Kind: "opening_missing", Detail: "未提供期初 BS：BS 与间接法 CF 整体降级为不可判定（PRD S2-3），IS 不受影响"})
		return nil
	}
	balance, ref, engine, policy, err := s.in.Opening.Get(ctx, s.def.LegalEntityID)
	if err != nil {
		return fmt.Errorf("finmodel: load opening balance: %w", err)
	}
	if balance == nil {
		s.openingMissing = true
		s.gaps = append(s.gaps, DataGap{Kind: "opening_missing", Detail: "期初端口返回空：未提供期初 BS（PRD S2-3 降级，不编造平衡表）"})
		return nil
	}
	failures := opening.Validate(opening.ValidateInput{Balance: *balance, LeaseRef: ref, Engine: engine, Policy: policy})
	if len(failures) > 0 {
		// 期初带病：闸失败 ≠ 未提供期初。前者阻止运行（每期都会错），
		// 后者才允许 BS/CF 降级。失败明细结构化携带，供渲染与数据质量队列。
		details := make([]string, 0, len(failures))
		for _, f := range failures {
			details = append(details, fmt.Sprintf("gate %s%s: %s", f.Gate, periodSuffix(f.Period), f.Detail))
		}
		return &OpeningRejectedError{
			Failures: failures,
			wrapped:  fmt.Errorf("%w (%s)", ErrOpeningRejected, strings.Join(details, "; ")),
		}
	}
	s.opening = balance
	// 期初余额法的机械落点：首个 lag 跨界时，先看向导入的期初值。
	if len(balance.Periods) > 0 {
		first := balance.Periods[0]
		s.openingValues = map[string]*float64{
			"cash":              ptr(first.Lines["cash"]),
			"ar":                ptr(first.Lines["ar"]),
			"inventory":         ptr(first.Lines["inventory"]),
			"ap":                ptr(first.Lines["ap"]),
			"ppe":               ptr(first.Lines["ppe"]),
			"rou_asset":         ptr(first.Lines["rou_asset"]),
			"lease_liability":   ptr(first.Lines["lease_liability"]),
			"borrowings":        ptr(first.Lines["borrowings"]),
			"share_capital":     ptr(first.Lines["share_capital"]),
			"retained_earnings": ptr(first.Lines["retained_earnings"]),
			"ending_cash":       ptr(first.Lines["cash"]),
			"nwc":               ptr(first.Lines["ar"] + first.Lines["inventory"] - first.Lines["ap"]),
		}
	}
	return nil
}

func ptr(v float64) *float64 { return &v }

func periodSuffix(period string) string {
	if period == "" {
		return ""
	}
	return " @" + period
}

// recordLagBasis marks that a stock row resolved a lag-1 reference for the
// period and copies the carry basis actually used — the "opening" of that row.
// T11 compares the recorded basis against the materialized prior closing to
// catch post-computation drift in the carry chain. The value is copied, never
// aliased, so a later value-table mutation cannot hide from the check.
func (s *runState) recordLagBasis(period, ref string, v *float64) {
	if s.lagUsed[period] == nil {
		s.lagUsed[period] = map[string]bool{}
	}
	s.lagUsed[period][ref] = true
	if v == nil {
		return
	}
	copied := *v
	if s.lagBasis[period] == nil {
		s.lagBasis[period] = map[string]*float64{}
	}
	s.lagBasis[period][ref] = &copied
}

// loadPeriods evaluates every row for every period in order.
func (s *runState) loadPeriods(ctx context.Context) {
	// link rows + input rows first, then formulas in dependency order, then
	// subtotals (which depend on everything they sum).
	var formulas []string
	var subtotals []string
	rowByKey := map[string]template.Row{}
	for _, row := range s.def.Template.Rows {
		rowByKey[row.Key] = row
		switch row.Kind {
		case template.RowFormula, template.RowCheck:
			formulas = append(formulas, row.Key)
		case template.RowSubtotal:
			subtotals = append(subtotals, row.Key)
		}
	}
	ordered := topoOrder(rowByKey, formulas)

	for i, period := range s.periods {
		_ = i
		// Links and inputs.
		for _, row := range s.def.Template.Rows {
			switch row.Kind {
			case template.RowLink:
				s.loadLink(ctx, row, period)
			case template.RowInput:
				s.loadInput(ctx, row, period)
			}
		}
		// Formulas/checks. `ordered` is a superset (dependency traversal also
		// visits input/link rows); only formula-bearing rows evaluate.
		for _, key := range ordered {
			row := rowByKey[key]
			if row.Formula == nil {
				continue
			}
			// PRD C7 / T13: a formula row declaring an actual_source reads the
			// fact on the Actual side of the freeze line — the Actual 区永不来自
			// 假设推导. The driver formula applies only from the forecast side.
			if row.ActualSource != "" && s.cutoffIdx >= 0 && period <= s.def.ActualCutoffPeriod {
				s.loadActualOverride(ctx, row, period)
				continue
			}
			value := row.Formula.Eval(
				func(ref string) *float64 { return s.values[ref][period] },
				func(ref string, n int) *float64 {
					if i-n >= 0 {
						v := s.values[ref][s.periods[i-n]]
						if n == 1 {
							s.recordLagBasis(period, ref, v)
						}
						return v
					}
					// 首期 lag 跨界：期初余额法下看导入的期初值；无期初即缺失。
					if n == 1 && !s.openingMissing {
						v := s.openingValues[ref]
						if v != nil {
							s.recordLagBasis(period, ref, v)
						}
						return v
					}
					return nil
				},
			)
			value = s.applyInterestPresentation(row.Key, period, value)
			s.setValue(row.Key, period, value, Provenance{SourceType: "formula", DataClassification: s.in.DataClassification})
		}
		// Subtotals.
		for _, key := range subtotals {
			row := rowByKey[key]
			s.evalSubtotal(row, period)
		}
	}
	// Alias resolution: link rows bound to computed rows. Subtotals that
	// consume aliased rows must then be re-evaluated.
	s.resolveCfAliases()
	for _, period := range s.periods {
		for _, key := range subtotals {
			s.evalSubtotal(rowByKey[key], period)
		}
	}
}

func (s *runState) loadLink(ctx context.Context, row template.Row, period string) {
	var value *float64
	prov := Provenance{DataClassification: s.in.DataClassification}
	switch {
	case strings.HasPrefix(row.Source, "fact."):
		if s.cutoffIdx >= 0 && period > s.def.ActualCutoffPeriod {
			s.forecastFact(ctx, row, period)
			return
		}
		if s.in.Facts == nil {
			s.gaps = append(s.gaps, DataGap{Kind: "fact_port_unavailable", Period: period, Detail: "事实端口未接线，行 " + row.Key + " 缺失而非 0"})
			s.setValue(row.Key, period, nil, prov)
			return
		}
		facts, err := s.in.Facts.Operating(ctx, s.def.LegalEntityID, period)
		if err != nil {
			s.setValue(row.Key, period, nil, prov)
			return
		}
		s.factByPeriod[period] = facts
		value, prov.SourceType = factField(facts, row.Source)
		// 值表写入前复制事实：值表与事实缓存不得共享同一底层地址，否则
		// 值表被改（勾稽反向测试即这样破坏）会连带改掉事实缓存，T13 恒真。
		if value != nil {
			copied := *value
			value = &copied
		}
		if value != nil {
			prov.Ref = row.Source
			prov.DataVersion = facts.DatasetVersion
			prov.DataClassification = facts.DataClassification
			if !facts.DecisionReady {
				s.gaps = append(s.gaps, DataGap{Kind: "fact_coverage", Period: period, Detail: facts.DecisionReadyReason})
			}
		} else {
			s.gaps = append(s.gaps, DataGap{Kind: "fact_missing", Period: period, Detail: row.Key + " 缺事实，缺失不显示为 0"})
		}
	case strings.HasPrefix(row.Source, "ifrs16."):
		if s.in.Lease == nil {
			s.gaps = append(s.gaps, DataGap{Kind: "lease_port_unavailable", Period: period, Detail: "租赁投影端口未接线，行 " + row.Key + " 缺失而非 0"})
			s.setValue(row.Key, period, nil, prov)
			return
		}
		lease, err := s.in.Lease.Monthly(ctx, s.def.LegalEntityID, period)
		if err != nil {
			s.setValue(row.Key, period, nil, prov)
			return
		}
		s.leaseByPeriod[period] = lease
		value, prov.Ref = leaseField(lease, row.Source)
		prov.SourceType = "ifrs16_engine"
	case strings.HasPrefix(row.Source, "sched."):
		if s.in.Schedules == nil {
			s.gaps = append(s.gaps, DataGap{Kind: "schedule_port_unavailable", Period: period, Detail: "付款计划端口未接线，行 " + row.Key + " 缺失而非 0"})
			s.setValue(row.Key, period, nil, prov)
			return
		}
		sched, err := s.in.Schedules.Monthly(ctx, s.def.LegalEntityID, period)
		if err != nil {
			s.setValue(row.Key, period, nil, prov)
			return
		}
		value, prov.Ref = schedField(sched, row.Source)
		prov.SourceType = "contract_schedule"
	case strings.HasPrefix(row.Source, "cf.") || strings.HasPrefix(row.Source, "contract.") || strings.HasPrefix(row.Source, "assumption."):
		// cf.* handled by resolvetCfAliases after computation; contract.* and
		// assumption.* resolve through the assumption reader.
		if strings.HasPrefix(row.Source, "assumption.") || strings.HasPrefix(row.Source, "contract.") {
			if s.in.Assumptions == nil {
				s.gaps = append(s.gaps, DataGap{Kind: "assumption_port_unavailable", Period: period, Detail: "假设端口未接线，行 " + row.Key + " 缺失而非 0"})
				return
			}
			raw, err := s.in.Assumptions.Value(ctx, s.def.LegalEntityID, sourceAssumptionKey(row.Source), period)
			if err == nil && raw != nil {
				if v, ok := jsonNumber(raw); ok {
					value = &v
					prov.SourceType = "assumption"
					prov.Ref = row.Source
				}
			} else if err == nil {
				s.gaps = append(s.gaps, DataGap{Kind: "assumption_missing", Period: period, Detail: "假设 " + row.Source + " 未登记（approved），缺失而非 0"})
			}
		}
	default:
		s.gaps = append(s.gaps, DataGap{Kind: "unregistered_source", Period: period, Detail: "模板行 " + row.Key + " 绑定未登记数据源 " + row.Source})
	}
	s.setValue(row.Key, period, value, prov)
}

// forecastFact drives the forecast window. Revenue uses the SSSG assumption
// (run-rate × (1+sssg)); cost rows use their growth-rate assumptions
// (S2-4's 增长率法). The driver assumption is a versioned input, never a
// literal in a formula; a missing driver leaves the cell missing with a gap.
func (s *runState) forecastFact(ctx context.Context, row template.Row, period string) {
	drivers := map[string]string{
		"fact.revenue":                 "sssg",
		"fact.labor_cost":              "labor_cost_growth",
		"fact.fixed_rent":              "fixed_rent_growth",
		"fact.variable_rent":           "variable_rent_growth",
		"fact.non_lease_cost":          "non_lease_cost_growth",
		"fact.other_controllable_cost": "other_controllable_cost_growth",
	}
	key, ok := drivers[row.Source]
	if !ok {
		s.setValue(row.Key, period, nil, Provenance{DataClassification: s.in.DataClassification})
		s.gaps = append(s.gaps, DataGap{Kind: "forecast_driver_missing", Period: period, Detail: "预测期无可用驱动假设：" + row.Source + " 缺失"})
		return
	}
	idx := s.idxOf(period)
	if idx == 0 {
		s.setValue(row.Key, period, nil, Provenance{DataClassification: s.in.DataClassification})
		s.gaps = append(s.gaps, DataGap{Kind: "forecast_driver_missing", Period: period, Detail: "预测首期无 run-rate 基期：" + row.Source})
		return
	}
	prev := s.values[row.Key][s.periods[idx-1]]
	if prev == nil {
		s.setValue(row.Key, period, nil, Provenance{DataClassification: s.in.DataClassification})
		s.gaps = append(s.gaps, DataGap{Kind: "forecast_driver_missing", Period: period, Detail: row.Source + " 基期缺失，预测无法驱动"})
		return
	}
	rate := 0.0
	if s.in.Assumptions != nil {
		if raw, err := s.in.Assumptions.Value(ctx, s.def.LegalEntityID, key, period); err == nil && raw != nil {
			if v, ok := jsonNumber(raw); ok {
				rate = v
			}
		}
	}
	// S2-4 组合驱动：新店爬坡因子与门店增减计划以批准假设显式登记，未登记
	// 即中性 1.0（驱动的取值是有来源、有版本、有责任人的假设，绝不内置常数）。
	factor := 1.0
	activeDrivers := []string{}
	for _, driver := range []string{"ramp_factor", "store_count_growth"} {
		if s.in.Assumptions == nil {
			continue
		}
		if raw, err := s.in.Assumptions.Value(ctx, s.def.LegalEntityID, driver, period); err == nil && raw != nil {
			if v, ok := jsonNumber(raw); ok {
				factor *= 1 + v
				activeDrivers = append(activeDrivers, driver+"="+fmt.Sprintf("%.4f", v))
			}
		}
	}
	value := *prev * (1 + rate) * factor
	s.setValue(row.Key, period, &value, Provenance{SourceType: "assumption", Ref: "driver." + key, DataClassification: s.in.DataClassification})
	if row.Source == "fact.revenue" {
		detail := "预测收入以 SSSG 驱动（run-rate × (1+sssg)）"
		if len(activeDrivers) > 0 {
			detail += "；组合驱动：" + strings.Join(activeDrivers, "、")
		}
		s.gaps = append(s.gaps, DataGap{Kind: "revenue_driver_note", Period: period, Detail: detail})
	}
}

func (s *runState) idxOf(period string) int {
	for i, p := range s.periods {
		if p == period {
			return i
		}
	}
	return -1
}

func (s *runState) loadInput(ctx context.Context, row template.Row, period string) {
	// F1 D-F2：用户主张的零是数据层的不同值——0 + human_input provenance，
	// 优先于假设读取；缺失仍走 nil + 具名 Gap。两者绝不混同。
	if hz, asserted := s.in.HumanZeros[row.Key]; asserted {
		confirmedBy := hz.ConfirmedBy
		s.setValue(row.Key, period, func() *float64 { v := 0.0; return &v }(), Provenance{
			SourceType: "human_input", Ref: "confirmed_by:" + confirmedBy, DataClassification: s.in.DataClassification,
		})
		return
	}
	prov := Provenance{SourceType: "assumption", Ref: row.Key, DataClassification: s.in.DataClassification}
	if s.in.Assumptions == nil {
		s.setValue(row.Key, period, nil, prov)
		return
	}
	raw, err := s.in.Assumptions.Value(ctx, s.def.LegalEntityID, row.Key, period)
	var value *float64
	if err == nil && raw != nil {
		if v, ok := jsonNumber(raw); ok {
			value = &v
		}
	}
	if value == nil {
		s.setValue(row.Key, period, nil, prov)
		s.gaps = append(s.gaps, DataGap{Kind: "assumption_missing", Period: period, Detail: "假设 " + row.Key + " 未登记"})
		return
	}
	s.setValue(row.Key, period, value, prov)
}

// loadActualOverride resolves a formula row's Actual-window value from the
// FactReader instead of the driver formula (PRD C7 / T13). The Actual 冻结线
// 左侧永不来自假设推导——同一输入的 run，真实 Actual 的毛利就是事实毛利。
func (s *runState) loadActualOverride(ctx context.Context, row template.Row, period string) {
	prov := Provenance{DataClassification: s.in.DataClassification}
	if s.in.Facts == nil {
		s.gaps = append(s.gaps, DataGap{Kind: "fact_port_unavailable", Period: period, Detail: "事实端口未接线，行 " + row.Key + " 的 Actual 来源缺失而非 0"})
		s.setValue(row.Key, period, nil, prov)
		return
	}
	facts, err := s.in.Facts.Operating(ctx, s.def.LegalEntityID, period)
	if err != nil {
		s.setValue(row.Key, period, nil, prov)
		return
	}
	s.factByPeriod[period] = facts
	value, src := factField(facts, row.ActualSource)
	// 值表写入前复制事实：与 loadLink 同一条纪律——值表与事实缓存不得
	// 共享底层地址，否则勾稽反向测试对值表的破坏会反向改写事实缓存。
	if value != nil {
		copied := *value
		value = &copied
	}
	if value != nil {
		prov.SourceType = "fact_aggregate"
		prov.Ref = src
		prov.DataVersion = facts.DatasetVersion
		prov.DataClassification = facts.DataClassification
		if !facts.DecisionReady {
			s.gaps = append(s.gaps, DataGap{Kind: "fact_coverage", Period: period, Detail: facts.DecisionReadyReason})
		}
	} else {
		s.gaps = append(s.gaps, DataGap{Kind: "fact_missing", Period: period, Detail: row.Key + " 的 Actual 来源 " + row.ActualSource + " 缺事实，缺失不显示为 0"})
	}
	s.setValue(row.Key, period, value, prov)
}

// applyInterestPresentation is the T9 policy switch made real (PRD S2-3
// Step 3 / appendix B T9). Under "financing" (CAS practice) the cash interest
// is moved out of CFO and shown separately inside CFE「分配股利、利润或偿付
// 利息支付的现金」; under "operating" (IAS 7 option) interest stays inside
// CFO. The two branches conserve total cash (net_cash_flow is unchanged), so
// T1/T2 hold under both. Applied at the cfo/cfe rows so every dependent row
// (net_cash_flow, ending_cash) sees the presented numbers.
func (s *runState) applyInterestPresentation(rowKey, period string, value *float64) *float64 {
	if s.def.Policy.InterestCashFlowPresentation != "financing" || value == nil {
		return value
	}
	if rowKey != "cfo" && rowKey != "cfe" {
		return value
	}
	// 现金支付的利息 = 租赁利息 + 借款利息（管理口径按费用近似现金支付）。
	lease := s.at("lease_interest", period)
	borrow := s.at("borrow_interest", period)
	if lease == nil && borrow == nil {
		s.gaps = append(s.gaps, DataGap{Kind: "interest_presentation_missing", Period: period, Detail: "financing 列报要求把利息移出 CFO，但本期利息行缺失，无法调整"})
		return value
	}
	interest := 0.0
	if lease != nil {
		interest += *lease
	}
	if borrow != nil {
		interest += *borrow
	}
	out := *value
	if rowKey == "cfo" {
		out -= interest
	} else {
		out += interest
	}
	return &out
}

func (s *runState) evalSubtotal(row template.Row, period string) {
	var sum float64
	complete := true
	for _, child := range row.Children {
		v := s.values[child][period]
		if v == nil {
			complete = false
			break
		}
		sum += *v * row.ChildSign(child)
	}
	if !complete {
		s.setValue(row.Key, period, nil, Provenance{SourceType: "formula", DataClassification: s.in.DataClassification})
		s.gaps = append(s.gaps, DataGap{Kind: "subtotal_partial", Period: period, Detail: "小计 " + row.Key + " 遇缺失子行，整体降级 partial"})
		return
	}
	s.setValue(row.Key, period, &sum, Provenance{SourceType: "formula", DataClassification: s.in.DataClassification})
}

// resolveCfAliases copies computed CF rows into link rows bound to "cf.*".
func (s *runState) resolveCfAliases() {
	for _, row := range s.def.Template.Rows {
		if row.Kind != template.RowLink || !strings.HasPrefix(row.Source, "cf.") {
			continue
		}
		target := strings.TrimPrefix(row.Source, "cf.")
		for _, period := range s.periods {
			s.setValue(row.Key, period, s.values[target][period], Provenance{SourceType: "formula", Ref: "cf." + target, DataClassification: s.in.DataClassification})
		}
	}
}

// topoOrder orders formula keys so dependencies evaluate first.
func topoOrder(rowByKey map[string]template.Row, keys []string) []string {
	deps := map[string][]string{}
	for _, key := range keys {
		if rowByKey[key].Formula != nil {
			for _, dep := range rowByKey[key].Formula.Deps() {
				if _, ok := rowByKey[dep]; ok {
					deps[key] = append(deps[key], dep)
				}
			}
		}
	}
	visited := map[string]int{}
	var order []string
	var visit func(k string)
	visit = func(k string) {
		if visited[k] == 2 {
			return
		}
		visited[k] = 1 // cycles already rejected at Parse time
		for _, dep := range deps[k] {
			visit(dep)
		}
		visited[k] = 2
		order = append(order, k)
	}
	for _, key := range keys {
		visit(key)
	}
	return order
}

// ── reader field mapping ─────────────────────────────────────────────────

func factField(f OperatingFacts, source string) (*float64, string) {
	switch source {
	case "fact.revenue":
		return f.Revenue, "fact.revenue"
	case "fact.gross_profit":
		return f.GrossProfit, "fact.gross_profit"
	case "fact.labor_cost":
		return f.LaborCost, "fact.labor_cost"
	case "fact.fixed_rent":
		return f.FixedRent, "fact.fixed_rent"
	case "fact.variable_rent":
		return f.VariableRent, "fact.variable_rent"
	case "fact.non_lease_cost":
		return f.NonLeaseCost, "fact.non_lease_cost"
	case "fact.other_controllable_cost":
		return f.OtherControllableCost, "fact.other_controllable_cost"
	case "fact.four_wall_ebitda":
		return f.FourWallEBITDA, "fact.four_wall_ebitda"
	case "fact.break_even_sales":
		return f.BreakEvenSales, "fact.break_even_sales"
	}
	return nil, ""
}

func leaseField(lease LeaseMonth, source string) (*float64, string) {
	switch source {
	case "ifrs16.rou_asset":
		return lease.ROUAsset, "ifrs16.rou_asset"
	case "ifrs16.lease_liability":
		return lease.LeaseLiability, "ifrs16.lease_liability"
	case "ifrs16.lease_interest":
		return lease.Interest, "ifrs16.lease_interest"
	case "ifrs16.rou_depreciation":
		return lease.Depreciation, "ifrs16.rou_depreciation"
	case "ifrs16.lease_payments":
		return lease.Payments, "ifrs16.lease_payments"
	case "ifrs16.lease_principal":
		return lease.Principal, "ifrs16.lease_principal"
	case "ifrs16.lease_additions":
		return lease.Additions, "ifrs16.lease_additions"
	case "ifrs16.lease_remeasurements":
		return lease.Remeasurements, "ifrs16.lease_remeasurements"
	case "ifrs16.lease_terminations":
		return lease.Terminations, "ifrs16.lease_terminations"
	}
	return nil, ""
}

func schedField(s ScheduleFanout, source string) (*float64, string) {
	switch source {
	case "sched.capex":
		return s.Capex, "sched.capex"
	case "sched.other_depreciation":
		return s.OtherDepreciation, "sched.other_depreciation"
	case "sched.share_capital":
		return s.ShareCapital, "sched.share_capital"
	case "contract.service_fee":
		return s.ServiceFee, "contract.service_fee"
	case "sched.borrowings":
		return s.Borrowings, "sched.borrowings"
	}
	return nil, ""
}

func sourceAssumptionKey(source string) string {
	switch source {
	case "assumption.marketing":
		return "marketing"
	case "contract.service_fee":
		return "service_fee"
	default:
		return strings.TrimPrefix(source, "assumption.")
	}
}

func jsonNumber(raw json.RawMessage) (float64, bool) {
	var v float64
	if err := json.Unmarshal(raw, &v); err == nil {
		return v, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}
