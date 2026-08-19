package retail

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/services/retailpulse"
	"github.com/lease-management-system/core-service/internal/services/retailstore360"
	"github.com/lease-management-system/core-service/internal/workingpaper"
)

func scopeSection(in Input) workingpaper.Section {
	sys := systemFact(in)
	var cells []workingpaper.Cell
	p := in.Pulse
	cells = append(cells, strCell("SC-1", "数据分类", p.DataClassification, sys))
	if p.DatasetVersion != "" {
		cells = append(cells, strCell("SC-2", "数据集版本", p.DatasetVersion, sys))
	}
	if len(p.SourceSystems) > 0 {
		cells = append(cells, strCell("SC-3", "来源系统", strings.Join(p.SourceSystems, "、"), sys))
	}
	cells = append(cells,
		strCell("SC-4", "事实版本范围", fmt.Sprintf("%d ~ %d", p.FactVersionMin, p.FactVersionMax), sys),
	)
	if p.HighestAsOf != nil {
		cells = append(cells, strCell("SC-5", "事实 as-of 最高时点", p.HighestAsOf.Format("2006-01-02"), sys))
	}
	cells = append(cells,
		strCell("SC-6", "币种", visibleCurrency(p.Currency, p.CurrencyStatus), sys),
	)
	if p.MultiCurrency {
		cells = append(cells, strCell("SC-7", "多币种提示", fmt.Sprintf("true（%d 家门店币种混杂）", p.MixedCurrencyStores), sys))
	}

	narrative := "数据边界与质量。decision_ready=" + strconv.FormatBool(p.DecisionReady)
	if p.DecisionReadyReason != "" {
		narrative += "（" + p.DecisionReadyReason + "）"
	}
	if in.Diagnostics != nil && in.Diagnostics.Store.StoreID != "" {
		s := in.Diagnostics.Store
		parts := []string{s.StoreCode, s.StoreName}
		if s.Brand != "" {
			parts = append(parts, s.Brand)
		}
		if s.Region != "" {
			parts = append(parts, s.Region)
		}
		narrative += "\n门店：" + strings.Join(parts, " / ")
	} else if len(p.RequestedStores) > 0 {
		var names []string
		for _, st := range p.RequestedStores {
			names = append(names, st.StoreCode+" "+st.StoreName)
		}
		narrative += "\n覆盖门店：" + strings.Join(names, "、")
	}
	return workingpaper.Section{ID: "scope", Title: "数据边界与质量", Kind: workingpaper.KindTable, Cells: cells, Narrative: narrative}
}

func pulseSection(in Input, engine string) workingpaper.Section {
	p := in.Pulse
	prov := certified(in.ToolCallID, engine)
	currency := usableCurrency(p.Currency, p.CurrencyStatus)

	// Multi-currency responses keep their numbers inside Partitions; the
	// paper refuses to aggregate across currencies and says so.
	var kpis []workingpaper.Cell
	kpis = append(kpis, summaryCells("P-", "组合", p.Summary, currency, prov)...)
	for _, part := range p.Partitions {
		kpis = append(kpis, summaryCells("P-"+part.Currency+"-", "分区 "+visibleCurrency(part.Currency, part.CurrencyStatus), part.Summary, usableCurrency(part.Currency, part.CurrencyStatus), prov)...)
	}
	return workingpaper.Section{ID: "pulse", Title: "经营脉搏（零售 KPI，retail-kpi-v1 语义）", Kind: workingpaper.KindTable, Cells: kpis}
}

func summaryCells(prefix, scopeName string, summary map[string]retailpulse.SummaryMetric, currency string, prov workingpaper.Provenance) []workingpaper.Cell {
	var cells []workingpaper.Cell
	for code, m := range summary {
		if c, ok := numCell(prefix+code+"-current", scopeName+" "+code+"（当期）", code, m.Current.Value, m.Current.Unit, currency, prov); ok {
			cells = append(cells, c)
		}
		if c, ok := numCell(prefix+code+"-comparison", scopeName+" "+code+"（对比期）", code, m.Comparison.Value, m.Comparison.Unit, currency, prov); ok {
			cells = append(cells, c)
		}
		if c, ok := numCell(prefix+code+"-change", scopeName+" "+code+"（变动）", code, m.ChangeValue, m.Current.Unit, currency, prov); ok {
			cells = append(cells, c)
		}
		if code == "store_contribution" {
			if c, ok := numCell(prefix+code+"-margin-pp", scopeName+" 门店贡献率变动（百分点）", code, m.ChangeMarginPP, "percentage_point", "", prov); ok {
				cells = append(cells, c)
			}
		}
	}
	return cells
}

func sssgSection(in Input, engine string) workingpaper.Section {
	if in.Pulse.SSSG == nil {
		return workingpaper.Section{}
	}
	s := in.Pulse.SSSG
	prov := certified(in.ToolCallID, engine)
	currency := usableCurrency(in.Pulse.Currency, in.Pulse.CurrencyStatus)
	var cells []workingpaper.Cell
	if c, ok := numCell("SG-1", "同店销售增长（SSSG）", "sssg", s.SSSG, "percent", "", prov); ok {
		cells = append(cells, c)
	}
	if c, ok := numCell("SG-2", "当期营收（同店口径）", "revenue", s.CurrentRevenue, "currency", currency, prov); ok {
		cells = append(cells, c)
	}
	if c, ok := numCell("SG-3", "基线营收（同店口径）", "revenue", s.BaselineRevenue, "currency", currency, prov); ok {
		cells = append(cells, c)
	}
	narrative := fmt.Sprintf("可比口径：爬坡期 %d 个月，要求连续经营=%v、同业态=%v",
		s.Cohort.Policy.RampUpMonths, s.Cohort.Policy.RequireContinuousOperation, s.Cohort.Policy.RequireSameFormat)
	if !s.DecisionReady && s.Reason != "" {
		narrative += "\n未就绪：" + s.Reason
	}
	return workingpaper.Section{ID: "sssg", Title: "同店销售增长", Kind: workingpaper.KindTable, Cells: cells, Narrative: narrative}
}

func attentionSection(in Input, engine string) workingpaper.Section {
	if len(in.Pulse.Attention) == 0 {
		return workingpaper.Section{}
	}
	prov := certified(in.ToolCallID, engine)
	limit := in.AttentionLimit
	if limit <= 0 || limit > len(in.Pulse.Attention) {
		limit = len(in.Pulse.Attention)
	}
	var cells []workingpaper.Cell
	for _, a := range in.Pulse.Attention[:limit] {
		label := fmt.Sprintf("#%d %s（%s，%s）", a.Rank, a.StoreName, a.Brand, a.Currency)
		if c, ok := numCell("AT-"+a.StoreID+"-score", label+" 关注度得分", "attention_score", &a.Score, "", a.Currency, prov); ok {
			cells = append(cells, c)
		}
		for _, sig := range a.ObservedSignals {
			ref := "AT-" + a.StoreID + "-" + sig.SignalCode
			if c, ok := numCell(ref, label+" "+sig.SignalCode+" 变动", sig.SignalCode, sig.ObservedChange, sig.Unit, a.Currency, prov); ok {
				cells = append(cells, c)
			}
		}
	}
	return workingpaper.Section{ID: "attention", Title: "关注门店（异常信号）", Kind: workingpaper.KindTable, Cells: cells}
}

func diagnosticsSection(in Input, engine string) workingpaper.Section {
	d := in.Diagnostics
	prov := certified(in.ToolCallID, engine)
	currency := usableCurrency(d.Currency, d.CurrencyStatus)
	var cells []workingpaper.Cell
	cells = append(cells, summaryCells("D-", "门店", toPulseSummary(d.Summary), currency, prov)...)

	for _, b := range d.PeerBenchmark {
		if b.Status != "complete" {
			continue
		}
		label := "同行基准 " + b.Code + "（" + d.PeerDefinition + "，n=" + strconv.Itoa(b.PeerCount) + "）"
		if c, ok := numCell("PB-"+b.Code+"-target", label+" 本店", b.Code, b.Target, b.Unit, currency, prov); ok {
			cells = append(cells, c)
		}
		if c, ok := numCell("PB-"+b.Code+"-median", label+" 同行中位", b.Code, b.Median, b.Unit, currency, prov); ok {
			cells = append(cells, c)
		}
		if c, ok := numCell("PB-"+b.Code+"-p25", label+" 同行 P25", b.Code, b.P25, b.Unit, currency, prov); ok {
			cells = append(cells, c)
		}
		if c, ok := numCell("PB-"+b.Code+"-p75", label+" 同行 P75", b.Code, b.P75, b.Unit, currency, prov); ok {
			cells = append(cells, c)
		}
	}

	for _, br := range d.Bridges {
		if br.Status == "unavailable" {
			continue
		}
		if c, ok := numCell("BR-"+br.Code+"-current", "变化桥 "+br.Code+" 当期", br.Code, br.Current, "currency", currency, prov); ok {
			cells = append(cells, c)
		}
		if c, ok := numCell("BR-"+br.Code+"-comparison", "变化桥 "+br.Code+" 对比期", br.Code, br.Comparison, "currency", currency, prov); ok {
			cells = append(cells, c)
		}
		if c, ok := numCell("BR-"+br.Code+"-total", "变化桥 "+br.Code+" 总变化", br.Code, br.TotalChange, "currency", currency, prov); ok {
			cells = append(cells, c)
		}
		for _, item := range br.Items {
			if c, ok := numCell("BR-"+br.Code+"-"+item.Label, "变化桥 "+br.Code+" · "+item.Label, br.Code, item.Contribution, item.Unit, currency, prov); ok {
				cells = append(cells, c)
			}
		}
		if c, ok := numCell("BR-"+br.Code+"-residual", "变化桥 "+br.Code+" 舍入残差", br.Code, br.RoundingResidual, "currency", currency, prov); ok {
			cells = append(cells, c)
		}
	}

	var statements []string
	for _, o := range d.Observations {
		statements = append(statements, "· "+o.Statement)
	}
	return workingpaper.Section{
		ID: "diagnostics", Title: "门店 360 诊断", Kind: workingpaper.KindTable, Cells: cells,
		Narrative: strings.Join(statements, "\n"),
	}
}

func scenarioSection(in Input, engine string) workingpaper.Section {
	s := in.Scenario
	prov := certified(in.ToolCallID, engine)
	currency := usableCurrency(s.Currency, "")
	var cells []workingpaper.Cell

	for code, m := range s.Baseline.Metrics {
		if c, ok := numCell("SBL-"+code, "基线 "+code, code, m.Result, m.Unit, currency, prov); ok {
			cells = append(cells, c)
		}
	}
	for _, sc := range s.Scenarios {
		for code, m := range sc.Metrics {
			if c, ok := numCell("S-"+sc.Key+"-"+code+"-result", sc.Name+" "+code+"（结果）", code, m.Result, m.Unit, currency, prov); ok {
				cells = append(cells, c)
			}
			if c, ok := numCell("S-"+sc.Key+"-"+code+"-delta", sc.Name+" "+code+"（Δ）", code, m.Delta, m.Unit, currency, prov); ok {
				cells = append(cells, c)
			}
		}
		if c, ok := numCell("S-"+sc.Key+"-monthly", sc.Name+" 月度门店贡献变化", "store_contribution", sc.MonthlyContributionChange, "currency", currency, prov); ok {
			cells = append(cells, c)
		}
		if c, ok := numCell("S-"+sc.Key+"-horizon", sc.Name+" 期内门店贡献变化（"+strconv.Itoa(s.HorizonMonths)+" 月）", "store_contribution", sc.HorizonContributionChange, "currency", currency, prov); ok {
			cells = append(cells, c)
		}
		for _, item := range sc.Bridge.Items {
			if c, ok := numCell("S-"+sc.Key+"-bridge-"+item.Label, sc.Name+" 贡献桥 · "+item.Label, item.Label, item.Contribution, item.Unit, currency, prov); ok {
				cells = append(cells, c)
			}
		}
		if c, ok := numCell("S-"+sc.Key+"-bridge-total", sc.Name+" 贡献桥总变化", "store_contribution", sc.Bridge.TotalChange, "currency", currency, prov); ok {
			cells = append(cells, c)
		}
		if c, ok := numCell("S-"+sc.Key+"-bridge-residual", sc.Name+" 贡献桥舍入残差", "store_contribution", sc.Bridge.RoundingResidual, "currency", currency, prov); ok {
			cells = append(cells, c)
		}
	}

	narrative := fmt.Sprintf("情景口径：%s。review_required=%v official_impact=%v ifrs16_impact=%v——均不触达正式过账与 IFRS 16 计量（红线承诺）。",
		s.ScenarioVersion, s.ReviewRequired, s.OfficialImpact, s.IFRS16Impact)
	return workingpaper.Section{ID: "scenario", Title: "经营情景测算（确定性引擎）", Kind: workingpaper.KindTable, Cells: cells, Narrative: narrative}
}

func assumptionsSection(in Input) workingpaper.Section {
	prov := human(in)
	var cells []workingpaper.Cell
	a := in.Assumptions
	items := []struct {
		ref, label string
		value      float64
		unit       string
	}{
		{"ASU-1", "营收变动 %", a.RevenueChangePct, "percent"},
		{"ASU-2", "毛利率变动（百分点）", a.GrossMarginRateChangePP, "percentage_point"},
		{"ASU-3", "人工成本变动 %", a.LaborCostChangePct, "percent"},
		{"ASU-4", "固定租金变动 %", a.FixedRentChangePct, "percent"},
		{"ASU-5", "变动租金率变动（百分点）", a.VariableRentRateChangePP, "percentage_point"},
		{"ASU-6", "非租赁成本变动 %", a.NonLeaseCostChangePct, "percent"},
		{"ASU-7", "其他可控成本变动 %", a.OtherControllableCostChangePct, "percent"},
	}
	for _, item := range items {
		// Zero = "未提供/无变化"，仅列示非零的人为确认假设；省略项即零。
		if item.value != 0 {
			cells = append(cells, workingpaper.Cell{
				Ref: item.ref, Label: item.label, Value: item.value, Unit: item.unit, Provenance: prov,
			})
		}
	}
	return workingpaper.Section{
		ID: "assumptions", Title: "情景假设（人工确认）", Kind: workingpaper.KindAssumptionList, Cells: cells,
		Narrative: "以上仅列示非零假设；未列示项均为 0（无变化）。这些是经营口径假设，与 IFRS 16 计量无关。",
	}
}

func visibleCurrency(iso, status string) string {
	if status == "unknown" {
		return "未知"
	}
	if status == "conflict" {
		return "币种冲突"
	}
	if iso == "" {
		return "未声明"
	}
	return iso
}

// ---- honesty signals → gaps ----

func scopeGaps(in Input) []string {
	var gaps []string
	p := in.Pulse
	if p.MultiCurrency {
		gaps = append(gaps, fmt.Sprintf("多币种数据（%d 家门店币种混杂）：底稿不跨币种加总，分区数字见经营脉搏节", p.MixedCurrencyStores))
	}
	if p.DataClassification == "simulated" {
		gaps = append(gaps, "数据标记为模拟（SIMULATED）：本底稿不得用作正式结论或对外材料")
	}
	if len(in.Pulse.SimulationDatasetVersions) > 0 {
		gaps = append(gaps, "模拟数据集版本："+strings.Join(in.Pulse.SimulationDatasetVersions, "、"))
	}
	return gaps
}

func pulseGaps(in Input) []string {
	var gaps []string
	p := in.Pulse
	if !p.DecisionReady {
		gaps = append(gaps, "经营脉搏未达 decision_ready："+p.DecisionReadyReason)
	}
	for _, c := range []struct {
		name string
		cov  retailkpi.Coverage
	}{{"当期", p.CurrentCoverage}, {"对比期", p.ComparisonCoverage}} {
		if c.cov.ExpectedStoreDays > 0 && c.cov.ObservedStoreDays < c.cov.ExpectedStoreDays {
			gaps = append(gaps, fmt.Sprintf("%s覆盖率不足：%d/%d 门店日；缺失字段：%s",
				c.name, c.cov.ObservedStoreDays, c.cov.ExpectedStoreDays, strings.Join(c.cov.MissingFields, "、")))
		}
	}
	for _, s := range p.SuppressedAttention {
		gaps = append(gaps, fmt.Sprintf("门店 %s 的关注信号被抑制：%s", s.StoreName, strings.Join(s.Reasons, "、")))
	}
	if p.SSSG == nil {
		gaps = append(gaps, "同店销售增长（SSSG）缺失：无同店可比生命周期数据")
	}
	return gaps
}

func diagGaps(in Input) []string {
	var gaps []string
	d := in.Diagnostics
	if d == nil {
		return gaps
	}
	if !d.DecisionReady {
		gaps = append(gaps, "门店诊断未达 decision_ready："+d.DecisionReadyReason)
	}
	for _, issue := range d.DataQualityIssues {
		gaps = append(gaps, "数据质量："+issue)
	}
	for _, b := range d.PeerBenchmark {
		if b.Status == "insufficient_peers" {
			gaps = append(gaps, "基准 "+b.Code+" 同群样本不足（最低 3 家）："+b.Reason)
		}
	}
	return gaps
}

func scenarioGaps(in Input) []string {
	var gaps []string
	if in.Scenario == nil {
		gaps = append(gaps, "情景测算缺失：经营脉搏或门店诊断未达 decision_ready，按确定性链路未执行情景（与聊天路径同一阻断语义）")
		return gaps
	}
	s := in.Scenario
	if s.Evidence.CoverageRate != nil && *s.Evidence.CoverageRate < 100 {
		gaps = append(gaps, fmt.Sprintf("情景证据覆盖率 %.1f%% < 100%%", *s.Evidence.CoverageRate))
	}
	return gaps
}

func residualSummary(in Input) string {
	var parts []string
	if in.Diagnostics != nil {
		for _, br := range in.Diagnostics.Bridges {
			if br.RoundingResidual != nil && *br.RoundingResidual != 0 {
				parts = append(parts, fmt.Sprintf("诊断桥 %s 舍入残差 %.2f（显式保留，不摊回任何驱动项）", br.Code, *br.RoundingResidual))
			}
		}
	}
	if in.Scenario != nil {
		for _, sc := range in.Scenario.Scenarios {
			if sc.Bridge.RoundingResidual != nil && *sc.Bridge.RoundingResidual != 0 {
				parts = append(parts, fmt.Sprintf("情景 %s 贡献桥舍入残差 %.2f（显式保留，不摊回）", sc.Name, *sc.Bridge.RoundingResidual))
			}
		}
	}
	return strings.Join(parts, "；")
}

// toPulseSummary adapts the store360 summary metric shape to the pulse shape
// so both sections share one mapping loop.
func toPulseSummary(in map[string]retailstore360.SummaryMetric) map[string]retailpulse.SummaryMetric {
	out := make(map[string]retailpulse.SummaryMetric, len(in))
	for k, v := range in {
		out[k] = retailpulse.SummaryMetric{
			Current: v.Current, Comparison: v.Comparison,
			ChangeValue: v.ChangeValue, ChangeType: v.ChangeType,
			Status: v.Status, Reason: v.Reason,
		}
	}
	return out
}
