package retail

import (
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/services/retailcohort"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/services/retailpulse"
	"github.com/lease-management-system/core-service/internal/services/retailscenario"
	"github.com/lease-management-system/core-service/internal/services/retailstore360"
	"github.com/lease-management-system/core-service/internal/workingpaper"
)

func f(v float64) *float64 { return &v }

// sampleInput builds a realistic multi-section paper input with honesty
// signals sprinkled in: a nil KPI, suppressed attention, a currency conflict
// on diagnostics, and a bridge rounding residual.
func sampleInput() Input {
	pulse := &retailpulse.Response{
		PulseVersion:       "retail-pulse-v1",
		FormulaVersion:     "retail-kpi-v1",
		DataClassification: "production",
		Currency:           "CNY",
		CurrencyStatus:     "known",
		RequestedScope:     map[string]any{"legal_entity_id": "LE-1"},
		PeriodLabel:        "2026-08",
		DecisionReady:      true,
		FactVersionMin:     12,
		FactVersionMax:     14,
		SourceSystems:      []string{"pos-a"},
		CurrentCoverage: retailkpi.Coverage{
			RequestedDateFrom: "2026-08-12", RequestedDateTo: "2026-08-18",
			ObservedStoreDays: 139, ExpectedStoreDays: 140,
			MissingFields: []string{"footfall"},
		},
		ComparisonCoverage: retailkpi.Coverage{
			RequestedDateFrom: "2026-08-05", RequestedDateTo: "2026-08-11",
			ObservedStoreDays: 140, ExpectedStoreDays: 140,
		},
		Summary: map[string]retailpulse.SummaryMetric{
			"revenue": {
				Current:     retailkpi.KPIValue{Value: f(1234567.89), Unit: "currency", Status: "complete"},
				Comparison:  retailkpi.KPIValue{Value: f(1180000), Unit: "currency", Status: "complete"},
				ChangeValue: f(54567.89), ChangeType: "percent",
			},
			// Deliberately nil-valued: must be SKIPPED, never zero-filled.
			"footfall": {
				Current: retailkpi.KPIValue{Value: nil, Unit: "count", Status: "unavailable"},
				Status:  "unavailable",
			},
			"store_contribution": {
				Current:        retailkpi.KPIValue{Value: f(320000), Unit: "currency"},
				ChangeMarginPP: f(-2.1),
			},
		},
		SSSG: &retailcohort.SSSGResult{
			SSSG:            f(3.4),
			CurrentRevenue:  f(1000000),
			BaselineRevenue: f(967000),
			DecisionReady:   true,
			Cohort: retailcohort.CohortResult{
				Policy: retailcohort.ComparabilityPolicy{RampUpMonths: 3, RequireContinuousOperation: true, RequireSameFormat: true},
			},
		},
		Attention: []retailpulse.Attention{
			{
				Rank: 1, StoreID: "S-1", StoreName: "旗舰店", Brand: "A", Currency: "CNY", Score: 8,
				ObservedSignals: []retailpulse.Signal{
					{SignalCode: "revenue_trend_drop", ObservedChange: f(-12.5), Unit: "percent"},
				},
			},
		},
		SuppressedAttention: []retailpulse.SuppressedAttention{
			{StoreID: "S-2", StoreName: "新开店", Reasons: []string{"no_facts_in_requested_range"}},
		},
	}

	diag := &retailstore360.Response{
		DiagnosticsVersion: "retail-store-diagnostics-v1",
		Currency:           "CNY",
		CurrencyStatus:     "known",
		Store:              retailstore360.StoreIdentity{StoreID: "S-1", StoreCode: "ST001", StoreName: "旗舰店", Brand: "A", Region: "华东"},
		DecisionReady:      true,
		PeerDefinition:     "同品牌同业态",
		MinimumPeerCount:   3,
		Summary: map[string]retailstore360.SummaryMetric{
			"revenue": {
				Current:    retailkpi.KPIValue{Value: f(300000), Unit: "currency"},
				Comparison: retailkpi.KPIValue{Value: f(290000), Unit: "currency"},
			},
		},
		PeerBenchmark: []retailstore360.PeerBenchmark{
			{Code: "sales_per_sqm", Unit: "currency_per_sqm", Target: f(900), Median: f(820), P25: f(750), P75: f(950), PeerCount: 6, Status: "complete"},
			{Code: "labor_cost_rate", Status: "insufficient_peers", Reason: "only 2 comparable stores"},
		},
		Bridges: []retailstore360.Bridge{
			{
				Code: "revenueShapley", Status: "complete",
				Current: f(300000), Comparison: f(290000), TotalChange: f(10000),
				Items: []retailstore360.BridgeItem{
					{Label: "客流量", Contribution: f(6000), Unit: "currency"},
					{Label: "客单价", Contribution: f(4000), Unit: "currency"},
				},
				RoundingResidual: f(0.01),
			},
		},
		Observations: []retailstore360.Observation{
			{Code: "obs-1", Statement: "客流量驱动本期增长，客单价持平。", Status: "complete"},
		},
	}

	scenario := &retailscenario.Response{
		ScenarioVersion: "retail-store-scenario-v1",
		Currency:        "CNY",
		HorizonMonths:   12,
		ReviewRequired:  true,
		OfficialImpact:  false,
		IFRS16Impact:    false,
		Evidence:        retailscenario.Evidence{CoverageRate: f(100)},
		Baseline: retailscenario.ScenarioResult{
			Key: "baseline", Name: "Baseline",
			Metrics: map[string]retailscenario.Metric{
				"store_contribution": {Baseline: f(39000), Result: f(39000), Unit: "currency", Status: "complete"},
			},
		},
		Scenarios: []retailscenario.ScenarioResult{
			{
				Key: "plan", Name: "Plan",
				Metrics: map[string]retailscenario.Metric{
					"store_contribution": {Baseline: f(39000), Result: f(45200), Delta: f(6200), Unit: "currency", Status: "complete"},
				},
				MonthlyContributionChange: f(6200), HorizonContributionChange: f(74400),
				Bridge: retailscenario.Bridge{
					Items: []retailscenario.BridgeItem{
						{Label: "毛利率", Contribution: f(3200), Unit: "currency"},
					},
					TotalChange:      f(6200),
					RoundingResidual: f(0.005),
					Status:           "complete",
				},
			},
		},
	}

	return Input{
		Pulse: pulse, Diagnostics: diag, Scenario: scenario,
		Assumptions:    retailscenario.Assumptions{GrossMarginRateChangePP: 1.5, FixedRentChangePct: -5},
		ConfirmedBy:    "bp-zhang",
		ConfirmedAt:    "2026-08-19T10:00:00Z",
		ToolCallID:     "call-retail-1",
		AttentionLimit: 3,
	}
}

func TestBuildPreservesEngineValuesOneToOne(t *testing.T) {
	paper, err := Build(sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	byRef := map[string]workingpaper.Cell{}
	for _, c := range paper.AllCells() {
		byRef[c.Ref] = c
	}
	check := func(ref string, want float64) {
		t.Helper()
		c, ok := byRef[ref]
		if !ok {
			t.Fatalf("missing cell %s", ref)
		}
		if c.Value.(float64) != want {
			t.Fatalf("cell %s = %v, engine says %v", ref, c.Value, want)
		}
		if c.Provenance.Basis != workingpaper.BasisCertified {
			t.Fatalf("cell %s must be Certified, got %s", ref, c.Provenance.Basis)
		}
	}
	check("P-revenue-current", 1234567.89)
	check("P-revenue-comparison", 1180000)
	check("P-store_contribution-margin-pp", -2.1)
	check("SG-1", 3.4)
	check("AT-S-1-score", 8)
	check("AT-S-1-revenue_trend_drop", -12.5)
	check("D-revenue-current", 300000)
	check("PB-sales_per_sqm-median", 820)
	check("BR-revenueShapley-total", 10000)
	check("BR-revenueShapley-客流量", 6000)
	check("BR-revenueShapley-residual", 0.01)
	check("S-plan-store_contribution-delta", 6200)
	check("S-plan-horizon", 74400)
	check("S-plan-bridge-毛利率", 3200)
	check("S-plan-bridge-residual", 0.005)

	// Assumptions are human-confirmed inputs.
	c := byRef["ASU-2"]
	if c.Value.(float64) != 1.5 || c.Provenance.Basis != workingpaper.BasisHumanInput || c.Provenance.ConfirmedBy != "bp-zhang" {
		t.Fatalf("assumption cell wrong: %+v", c)
	}
}

// Missing values stay missing: no zero-filled cells for unavailable KPIs.
func TestBuildSkipsNilValues(t *testing.T) {
	paper, err := Build(sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range paper.AllCells() {
		if c.Ref == "P-footfall-current" || c.Ref == "P-footfall-comparison" {
			t.Fatalf("nil KPI must not produce a cell, got %+v", c)
		}
	}
}

func TestBuildAssemblesHonestGaps(t *testing.T) {
	paper, err := Build(sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, g := range paper.DataGaps {
		joined += g + "|"
	}
	for _, want := range []string{
		"覆盖率不足：139/140 门店日",
		"footfall",
		"新开店 的关注信号被抑制",
		"基准 labor_cost_rate 同群样本不足",
	} {
		if !contains(joined, want) {
			t.Fatalf("gap %q missing; gaps=%v", want, paper.DataGaps)
		}
	}
	if !contains(paper.UnexplainedResidual, "revenueShapley") || !contains(paper.UnexplainedResidual, "0.01") {
		t.Fatalf("bridge residual must be kept explicit, got %q", paper.UnexplainedResidual)
	}
}

// I1/I2/I3/I6: the assembled paper passes the fail-closed lint with the
// audited call known completed, and carries zero exploratory cells.
func TestBuildPaperPassesLint(t *testing.T) {
	paper, err := Build(sampleInput())
	if err != nil {
		t.Fatal(err)
	}
	paper = workingpaper.Build(paper, time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC))
	rep := workingpaper.Lint(paper, auditSet{"call-retail-1": true})
	if !rep.OK {
		t.Fatalf("retail paper must pass lint, got %+v", rep.Violations)
	}
	if refs := paper.ExploratoryRefs(); len(refs) != 0 {
		t.Fatalf("retail paper is pure engine output: no exploratory cells, got %v", refs)
	}
}

func TestBuildMultiCurrencyAndSimulationGaps(t *testing.T) {
	in := sampleInput()
	in.Pulse.MultiCurrency = true
	in.Pulse.MixedCurrencyStores = 4
	in.Pulse.DataClassification = "simulated"
	in.Pulse.DatasetVersion = "ds-7"
	in.Pulse.SimulationDatasetVersions = []string{"ds-7"}
	paper, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	joined := ""
	for _, g := range paper.DataGaps {
		joined += g + "|"
	}
	if !contains(joined, "多币种数据") {
		t.Fatalf("multi-currency gap missing: %v", paper.DataGaps)
	}
	if !contains(joined, "模拟（SIMULATED）") {
		t.Fatalf("simulation gap missing: %v", paper.DataGaps)
	}
}

func TestBuildRequiresI2Anchor(t *testing.T) {
	in := sampleInput()
	in.ToolCallID = ""
	if _, err := Build(in); err == nil {
		t.Fatal("missing tool_call_id must fail")
	}
	in = sampleInput()
	in.ConfirmedBy = ""
	if _, err := Build(in); err == nil {
		t.Fatal("scenario assumptions need human confirmation")
	}
}

func TestBuildScenarioSkippedIsRecorded(t *testing.T) {
	in := sampleInput()
	in.Scenario = nil
	paper, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, g := range paper.DataGaps {
		if contains(g, "情景测算缺失") {
			found = true
		}
	}
	if !found {
		t.Fatalf("skipped scenario must be recorded as a gap: %v", paper.DataGaps)
	}
}

type auditSet map[string]bool

func (a auditSet) CompletedToolCall(callID string) bool { return a[callID] }

func contains(s, sub string) bool {
	return len(sub) == 0 || len(s) >= len(sub) && (s == sub || len(s) > 0 && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
