package retailpulse

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/services/retailsimulation"
)

type fakeReader struct {
	calls int
	set   *repository.RetailKPIFactSet
	err   error
}

func (f *fakeReader) QueryFacts(context.Context, string, string, string, string, string, string, []string) (*repository.RetailKPIFactSet, error) {
	f.calls++
	return f.set, f.err
}

func TestFixedSeedPulseSignalsAndSingleRead(t *testing.T) {
	plan, err := retailsimulation.Build("entity-a", retailsimulation.Input{})
	if err != nil {
		t.Fatal(err)
	}
	reader := &fakeReader{set: &repository.RetailKPIFactSet{Facts: simulationFacts(plan), ExpectedStoreCount: 60, SourceSystems: []string{"retail_simulator"}, DatasetVersions: []string{plan.DatasetVersion}}}
	service := NewService(reader)
	cases := []struct{ asOf, storeCode, signal string }{
		{"2026-01-31", plan.Stores[1].Code, "footfall_decline"},
		{"2026-02-25", plan.Stores[2].Code, "conversion_drop"},
		{"2026-03-22", plan.Stores[3].Code, "average_ticket_drop"},
		{"2026-04-16", plan.Stores[4].Code, "gross_margin_compression"},
		{"2026-05-11", plan.Stores[5].Code, "labor_cost_rate_spike"},
		{"2026-06-05", plan.Stores[6].Code, "occupancy_cost_rate_spike"},
	}
	for _, testCase := range cases {
		asOf, _ := time.Parse("2006-01-02", testCase.asOf)
		response, err := service.Build(context.Background(), Query{LegalEntityID: "entity-a", AsOf: asOf, Classification: "simulated", DatasetVersion: plan.DatasetVersion, AttentionLimit: 50})
		if err != nil {
			t.Fatalf("%s: %v", testCase.asOf, err)
		}
		if reader.calls != 1 {
			t.Fatalf("%s made %d fact reads", testCase.asOf, reader.calls)
		}
		reader.calls = 0
		if response.Currency != "CNY" || response.MultiCurrency || !response.DecisionReady {
			t.Fatalf("%s envelope=%+v", testCase.asOf, response)
		}
		if !strings.Contains(response.KPIDrilldownURL, "store_id={store_id}") || !strings.Contains(response.KPIDrilldownURL, "data_classification=simulated") || !strings.Contains(response.KPIDrilldownURL, "simulation_dataset_version=") || !strings.Contains(response.KPIDrilldownURL, "source_system=") {
			t.Fatalf("%s drilldown template is not replaceable/complete: %s", testCase.asOf, response.KPIDrilldownURL)
		}
		found := false
		for _, attention := range response.Attention {
			if attention.StoreCode == testCase.storeCode {
				for _, signal := range attention.ObservedSignals {
					if signal.SignalCode == testCase.signal {
						found = true
						if attention.Drilldown["data_classification"] != "simulated" || attention.Drilldown["simulation_dataset_version"] != plan.DatasetVersion || attention.Drilldown["source_system"] != "retail_simulator" || attention.Drilldown["store_id"] != attention.StoreID || !strings.Contains(attention.Drilldown["current_url"], "data_classification=simulated") || !strings.Contains(attention.Drilldown["current_url"], "simulation_dataset_version="+plan.DatasetVersion) || !strings.Contains(attention.Drilldown["current_url"], "source_system=retail_simulator") || !strings.Contains(attention.Drilldown["current_url"], "store_id="+attention.StoreID) || !strings.Contains(attention.Drilldown["current_url"], "date_from="+attention.Drilldown["current_date_from"]) || !strings.Contains(attention.Drilldown["comparison_url"], "date_from="+attention.Drilldown["comparison_date_from"]) {
							t.Fatalf("%s incomplete drilldown=%+v", testCase.asOf, attention.Drilldown)
						}
						t.Logf("%s %s score=%.2f change=%.2f threshold=%.2f", testCase.asOf, attention.StoreCode, attention.Score, valueOrZero(signal.ObservedChange), signal.Threshold)
					}
				}
			}
		}
		if !found {
			t.Fatalf("%s did not identify %s/%s; attention=%+v", testCase.asOf, testCase.storeCode, testCase.signal, response.Attention)
		}
		for _, code := range []string{"revenue", "gross_margin_rate", "footfall", "conversion_rate", "average_transaction_value", "labor_cost_rate", "occupancy_cash_cost_rate", "store_contribution"} {
			metric := response.Summary[code]
			t.Logf("%s summary %s current=%.2f comparison=%.2f change=%.2f", testCase.asOf, code, valueOrZero(metric.Current.Value), valueOrZero(metric.Comparison.Value), valueOrZero(metric.ChangeValue))
		}
		if len(response.DailyTrend) != 7 || response.DailyTrend[0].Date != response.Current.DateFrom || response.DailyTrend[6].Date != response.Current.DateTo {
			t.Fatalf("%s trend=%+v", testCase.asOf, response.DailyTrend)
		}
	}
}

func TestPulseSummaryDrilldownPreservesExplicitStoreScope(t *testing.T) {
	plan, err := retailsimulation.Build("entity-a", retailsimulation.Input{})
	if err != nil {
		t.Fatal(err)
	}
	storeIDs := []string{"store-a", "store-b"}
	reader := &fakeReader{set: &repository.RetailKPIFactSet{Facts: simulationFacts(plan), ExpectedStoreCount: 60, SourceSystems: []string{"retail_simulator"}, DatasetVersions: []string{plan.DatasetVersion}}}
	asOf, _ := time.Parse("2006-01-02", "2026-06-05")
	response, err := NewService(reader).Build(context.Background(), Query{LegalEntityID: "entity-a", AsOf: asOf, Classification: "simulated", DatasetVersion: plan.DatasetVersion, StoreIDs: storeIDs})
	if err != nil {
		t.Fatal(err)
	}
	for _, storeID := range storeIDs {
		if !strings.Contains(response.CurrentKPIDrilldownURL, "store_id="+storeID) || !strings.Contains(response.ComparisonKPIDrilldownURL, "store_id="+storeID) {
			t.Fatalf("summary drilldown dropped explicit store scope store=%s current=%s comparison=%s", storeID, response.CurrentKPIDrilldownURL, response.ComparisonKPIDrilldownURL)
		}
	}
}

func TestPulseResponseCarriesRequestedStoreMetadataForEmptyScope(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	set := &repository.RetailKPIFactSet{
		ExpectedStoreCount: 1,
		ExpectedStores:     []retailkpi.StorePopulation{{StoreID: "store-1", StoreCode: "S001", StoreName: "店一", Brand: "Brand A", Region: "North"}},
	}
	response, err := NewService(&fakeReader{set: set}).Build(context.Background(), Query{LegalEntityID: "entity-a", AsOf: from.AddDate(0, 0, 13), Classification: "production"})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.RequestedStores) != 1 || response.RequestedStores[0].StoreCode != "S001" || response.RequestedStores[0].Brand != "Brand A" {
		t.Fatalf("requested store metadata=%+v", response.RequestedStores)
	}
}

func TestPulseSuppressedAttentionOrderingAndSeverityBoundaries(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	facts := make([]retailkpi.DailyFact, 0, 14)
	for _, store := range []struct{ id, code string }{{"s2", "S2"}, {"s1", "S1"}} {
		for i := 0; i < 7; i++ {
			facts = append(facts, dailyFact(store.id, store.code, "CNY", from.AddDate(0, 0, i), 100))
		}
	}
	response, err := NewService(&fakeReader{set: &repository.RetailKPIFactSet{Facts: facts, ExpectedStoreCount: 2}}).Build(context.Background(), Query{LegalEntityID: "entity-a", AsOf: from.AddDate(0, 0, 13), Classification: "production", AttentionLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.SuppressedAttention) != 2 || response.SuppressedAttention[0].StoreCode != "S1" || response.SuppressedAttention[1].StoreCode != "S2" {
		t.Fatalf("suppressed attention order=%+v", response.SuppressedAttention)
	}
	for score, want := range map[float64]string{1: "medium", 3: "high", 6: "critical"} {
		if got := severity(score); got != want {
			t.Fatalf("severity(%v)=%s want=%s", score, got, want)
		}
	}
}

type pulseGolden struct {
	PulseVersion                     string              `json:"pulse_version"`
	FormulaVersion                   string              `json:"formula_version"`
	WindowDays                       int                 `json:"window_days"`
	Cases                            []pulseGoldenCase   `json:"cases"`
	ControlStoreIndex                int                 `json:"control_store_index"`
	CoverageSuppressionReason        string              `json:"coverage_suppression_reason"`
	EmptyPopulationSuppressionReason string              `json:"empty_population_suppression_reason"`
	SeverityThresholds               map[string]float64  `json:"severity_thresholds"`
	ContributionTurnsNegative        pulseNegativeGolden `json:"contribution_turns_negative"`
}
type pulseCoverageGolden struct {
	ObservedStoreDays int     `json:"observed_store_days"`
	ExpectedStoreDays int     `json:"expected_store_days"`
	CoverageRate      float64 `json:"coverage_rate"`
}
type pulseGoldenCase struct {
	AsOf               string              `json:"as_of"`
	StoreIndex         int                 `json:"store_index"`
	SignalCode         string              `json:"signal_code"`
	Rank               int                 `json:"rank"`
	Severity           string              `json:"severity"`
	Score              float64             `json:"score"`
	ObservedChange     float64             `json:"observed_change"`
	Threshold          float64             `json:"threshold"`
	CurrentCoverage    pulseCoverageGolden `json:"current_coverage"`
	ComparisonCoverage pulseCoverageGolden `json:"comparison_coverage"`
	Summary            map[string]float64  `json:"summary"`
}
type pulseNegativeGolden struct {
	Threshold         float64 `json:"threshold"`
	ScoreContribution float64 `json:"score_contribution"`
	Severity          string  `json:"severity"`
}

func TestCommittedPulseGolden(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("testdata", "retail_pulse_v1_golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var golden pulseGolden
	if err := json.Unmarshal(content, &golden); err != nil {
		t.Fatal(err)
	}
	if golden.PulseVersion != PulseVersion || golden.FormulaVersion != retailkpi.FormulaVersion || golden.WindowDays != 7 {
		t.Fatalf("golden header=%+v", golden)
	}
	for name, expected := range map[string]float64{"critical": 6, "high": 3, "medium": 1} {
		if golden.SeverityThresholds[name] != expected {
			t.Fatalf("severity threshold %s=%v want=%v", name, golden.SeverityThresholds[name], expected)
		}
	}
	if golden.ContributionTurnsNegative.Threshold != 0 || golden.ContributionTurnsNegative.ScoreContribution != 1 || golden.ContributionTurnsNegative.Severity != "medium" {
		t.Fatalf("negative contribution golden=%+v", golden.ContributionTurnsNegative)
	}
	plan, err := retailsimulation.Build("entity-a", retailsimulation.Input{})
	if err != nil {
		t.Fatal(err)
	}
	reader := &fakeReader{set: &repository.RetailKPIFactSet{Facts: simulationFacts(plan), ExpectedStoreCount: 60, SourceSystems: []string{"retail_simulator"}, DatasetVersions: []string{plan.DatasetVersion}}}
	service := NewService(reader)
	for _, want := range golden.Cases {
		asOf, _ := time.Parse("2006-01-02", want.AsOf)
		response, err := service.Build(context.Background(), Query{LegalEntityID: "entity-a", AsOf: asOf, WindowDays: golden.WindowDays, Classification: "simulated", DatasetVersion: plan.DatasetVersion, AttentionLimit: 50})
		if err != nil {
			t.Fatal(err)
		}
		if len(response.Attention) == 0 {
			t.Fatalf("%s has no attention", want.AsOf)
		}
		var found *Attention
		for i := range response.Attention {
			if response.Attention[i].StoreCode == plan.Stores[want.StoreIndex].Code {
				found = &response.Attention[i]
				break
			}
		}
		if found == nil {
			t.Fatalf("%s missing store %s", want.AsOf, plan.Stores[want.StoreIndex].Code)
		}
		if math.Abs(found.Score-want.Score) > 0.011 {
			t.Fatalf("%s score expected %.2f actual %.2f", want.AsOf, want.Score, found.Score)
		}
		if found.Rank != want.Rank || found.Severity != want.Severity {
			t.Fatalf("%s rank/severity expected=%d/%s actual=%d/%s", want.AsOf, want.Rank, want.Severity, found.Rank, found.Severity)
		}
		assertGoldenCoverage(t, want.AsOf+" current", response.CurrentCoverage, want.CurrentCoverage)
		assertGoldenCoverage(t, want.AsOf+" comparison", response.ComparisonCoverage, want.ComparisonCoverage)
		var signal *Signal
		for i := range found.ObservedSignals {
			if found.ObservedSignals[i].SignalCode == want.SignalCode {
				signal = &found.ObservedSignals[i]
				break
			}
		}
		if signal == nil || signal.ObservedChange == nil || math.Abs(*signal.ObservedChange-want.ObservedChange) > 0.011 || math.Abs(signal.Threshold-want.Threshold) > 0.011 {
			t.Fatalf("%s signal=%+v want=%+v", want.AsOf, signal, want)
		}
		for key, expected := range want.Summary {
			var actual float64
			switch key {
			case "revenue_current":
				actual = valueOrZero(response.Summary["revenue"].Current.Value)
			case "revenue_comparison":
				actual = valueOrZero(response.Summary["revenue"].Comparison.Value)
			case "store_contribution_current":
				actual = valueOrZero(response.Summary["store_contribution"].Current.Value)
			case "store_contribution_comparison":
				actual = valueOrZero(response.Summary["store_contribution"].Comparison.Value)
			}
			if math.Abs(actual-expected) > 0.011 {
				t.Fatalf("%s summary %s expected %.2f actual %.2f", want.AsOf, key, expected, actual)
			}
		}
		for _, attention := range response.Attention {
			if attention.StoreCode == plan.Stores[golden.ControlStoreIndex].Code {
				t.Fatalf("control store unexpectedly ranked for %s", want.AsOf)
			}
		}
	}
	for _, days := range []int{7, 8, 14, 21, 28} {
		asOf, _ := time.Parse("2006-01-02", "2026-06-05")
		response, err := service.Build(context.Background(), Query{LegalEntityID: "entity-a", AsOf: asOf, WindowDays: days, Classification: "simulated", DatasetVersion: plan.DatasetVersion, AttentionLimit: 10})
		if err != nil {
			t.Fatalf("window %d: %v", days, err)
		}
		if response.Comparison.DateTo >= response.Current.DateFrom || response.Current.DateFrom > response.Current.DateTo {
			t.Fatalf("window %d date boundaries current=%+v comparison=%+v", days, response.Current, response.Comparison)
		}
	}
}

func assertGoldenCoverage(t *testing.T, label string, actual retailkpi.Coverage, expected pulseCoverageGolden) {
	t.Helper()
	if actual.ObservedStoreDays != expected.ObservedStoreDays || actual.ExpectedStoreDays != expected.ExpectedStoreDays || actual.CoverageRate == nil || math.Abs(*actual.CoverageRate-expected.CoverageRate) > 0.001 {
		t.Fatalf("%s coverage expected=%+v actual=%+v", label, expected, actual)
	}
}

func TestPulseCoverageSuppressionAndMultiCurrency(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	facts := []retailkpi.DailyFact{dailyFact("s1", "S1", "CNY", from, 100), dailyFact("s1", "S1", "CNY", from.AddDate(0, 0, 1), 100), dailyFact("s1", "S1", "CNY", from.AddDate(0, 0, 2), 100), dailyFact("s1", "S1", "CNY", from.AddDate(0, 0, 3), 100), dailyFact("s1", "S1", "CNY", from.AddDate(0, 0, 4), 100), dailyFact("s1", "S1", "CNY", from.AddDate(0, 0, 5), 100), dailyFact("s1", "S1", "CNY", from.AddDate(0, 0, 6), 100), dailyFact("s1", "S1", "CNY", from.AddDate(0, 0, 7), 100), dailyFact("s1", "S1", "USD", from, 100), dailyFact("s1", "S1", "USD", from.AddDate(0, 0, 1), 100), dailyFact("s1", "S1", "USD", from.AddDate(0, 0, 2), 100), dailyFact("s1", "S1", "USD", from.AddDate(0, 0, 3), 100), dailyFact("s1", "S1", "USD", from.AddDate(0, 0, 4), 100), dailyFact("s1", "S1", "USD", from.AddDate(0, 0, 5), 100), dailyFact("s1", "S1", "USD", from.AddDate(0, 0, 6), 100)}
	reader := &fakeReader{set: &repository.RetailKPIFactSet{Facts: facts, ExpectedStoreCount: 1}}
	service := NewService(reader)
	response, err := service.Build(context.Background(), Query{LegalEntityID: "entity-a", AsOf: from.AddDate(0, 0, 7), Classification: "production", AttentionLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if !response.MultiCurrency || len(response.Partitions) != 2 || response.Summary != nil {
		t.Fatalf("multi currency response=%+v", response)
	}
	if response.Partitions[0].DecisionReady {
		t.Fatal("missing comparison facts should suppress decision readiness")
	}
	if len(response.Partitions[0].SuppressedAttention) == 0 {
		t.Fatal("missing comparison facts were not suppressed")
	}

	reader.set = &repository.RetailKPIFactSet{Facts: facts[:7], ExpectedStoreCount: 1}
	response, err = service.Build(context.Background(), Query{LegalEntityID: "entity-a", AsOf: from.AddDate(0, 0, 7), Classification: "production", AttentionLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if response.DecisionReady {
		t.Fatal("incomplete current/comparison coverage marked ready")
	}
	reader.set = &repository.RetailKPIFactSet{ExpectedStoreCount: 1, ExpectedStores: []retailkpi.StorePopulation{{StoreID: "missing", StoreCode: "MISSING-001", StoreName: "Missing", Brand: "Brand", Region: "Region"}}}
	response, err = service.Build(context.Background(), Query{LegalEntityID: "entity-a", AsOf: from.AddDate(0, 0, 55), WindowDays: 28, Classification: "production", AttentionLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if response.DecisionReady || len(response.SuppressedAttention) != 1 || response.SuppressedAttention[0].Reason != "no_facts_in_requested_range" || response.CurrentCoverage.ObservedStoreDays != 0 || response.CurrentCoverage.ExpectedStoreDays != 28 || response.ComparisonCoverage.ExpectedStoreDays != 28 {
		t.Fatalf("empty authorized population suppression=%+v current=%+v comparison=%+v", response.SuppressedAttention, response.CurrentCoverage, response.ComparisonCoverage)
	}
	reader.set = &repository.RetailKPIFactSet{ExpectedStoreCount: 1}
	response, err = service.Build(context.Background(), Query{LegalEntityID: "entity-a", AsOf: from.AddDate(0, 0, 6), WindowDays: 7, Classification: "production", AttentionLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if response.DecisionReady || len(response.SuppressedAttention) != 1 || response.SuppressedAttention[0].Reason != "no_facts_in_requested_range" || response.SuppressedAttention[0].Currency != "" || response.SuppressedAttention[0].CurrencyStatus != UnknownCurrencyStatus || len(response.DailyTrend) != 7 {
		t.Fatalf("count-only empty population suppression contract=%+v", response.SuppressedAttention)
	}
	reader.set = &repository.RetailKPIFactSet{}
	response, err = service.Build(context.Background(), Query{LegalEntityID: "entity-a", AsOf: from.AddDate(0, 0, 6), WindowDays: 7, Classification: "production", AttentionLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if response.DecisionReady || len(response.SuppressedAttention) != 0 || response.Currency != "" || response.CurrencyStatus != UnknownCurrencyStatus || response.CurrentCoverage.RequestedDateFrom != "2026-01-01" || response.CurrentCoverage.RequestedDateTo != "2026-01-07" || len(response.DailyTrend) != 7 {
		t.Fatalf("zero-authorized empty envelope=%+v trend=%d", response, len(response.DailyTrend))
	}
	for _, row := range response.DailyTrend {
		if !row.Gap || row.Currency != "" || row.CurrencyStatus != UnknownCurrencyStatus || row.Coverage.CoverageRate == nil || *row.Coverage.CoverageRate != 0 {
			t.Fatalf("zero-authorized trend row=%+v", row)
		}
	}
}

func TestPulseZeroComparisonDoesNotCreateInfiniteChange(t *testing.T) {
	zero := 0.0
	current := 100.0
	if value, reason := change(&current, &zero, "percent"); value != nil || reason != "zero_comparison" {
		t.Fatalf("zero comparison value=%v reason=%s", value, reason)
	}
	if value, reason := change(&current, nil, "percent"); value != nil || reason != "missing_value" {
		t.Fatalf("missing comparison value=%v reason=%s", value, reason)
	}
}

func TestPulseInvalidFactsAreSuppressed(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	facts := make([]retailkpi.DailyFact, 0, 14)
	for i := 0; i < 14; i++ {
		fact := dailyFact("s1", "S1", "CNY", from.AddDate(0, 0, i), 100)
		fact.DataQualityStatus = "invalid"
		facts = append(facts, fact)
	}
	service := NewService(&fakeReader{set: &repository.RetailKPIFactSet{Facts: facts, ExpectedStoreCount: 1}})
	response, err := service.Build(context.Background(), Query{LegalEntityID: "entity-a", AsOf: from.AddDate(0, 0, 13), Classification: "production", AttentionLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if response.DecisionReady || len(response.Attention) != 0 || len(response.SuppressedAttention) == 0 || response.SuppressedAttention[0].Reason != "data_quality_invalid" {
		t.Fatalf("invalid facts were not suppressed: %+v", response)
	}
}

func TestPulseSuppressionReasons(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	build := func(facts []retailkpi.DailyFact) *Response {
		response, err := NewService(&fakeReader{set: &repository.RetailKPIFactSet{Facts: facts, ExpectedStoreCount: 1}}).Build(context.Background(), Query{LegalEntityID: "entity-a", AsOf: from.AddDate(0, 0, 13), Classification: "production", AttentionLimit: 10})
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	base := make([]retailkpi.DailyFact, 0, 14)
	for i := 0; i < 14; i++ {
		base = append(base, dailyFact("s1", "S1", "CNY", from.AddDate(0, 0, i), 100))
	}
	invalid := append([]retailkpi.DailyFact(nil), base...)
	invalid[0].DataQualityStatus = "invalid"
	if got := build(invalid).SuppressedAttention[0].Reason; got != "data_quality_invalid" {
		t.Fatalf("invalid reason=%s", got)
	}
	mapping := append([]retailkpi.DailyFact(nil), base...)
	mapping[0].MappingStatus = "unmapped"
	if got := build(mapping).SuppressedAttention[0].Reason; got != "mapping_unmapped" {
		t.Fatalf("mapping reason=%s", got)
	}
	partial := append([]retailkpi.DailyFact(nil), base...)
	partial[0].Revenue = nil
	if got := build(partial).SuppressedAttention[0].Reason; got != "partial_or_unavailable_kpi" {
		t.Fatalf("partial reason=%s", got)
	}
	coverage := append([]retailkpi.DailyFact(nil), base[:13]...)
	if got := build(coverage).SuppressedAttention[0].Reason; got != "incomplete_store_day_coverage" {
		t.Fatalf("coverage reason=%s", got)
	}
	missingPeriod := append([]retailkpi.DailyFact(nil), base[7:]...)
	if got := build(missingPeriod).SuppressedAttention[0].Reason; got != "missing_period_facts" {
		t.Fatalf("missing period reason=%s", got)
	}
}

func TestPulseEvidenceUsesOnlyStoreFacts(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	facts := make([]retailkpi.DailyFact, 0, 28)
	datasetA, datasetB := "dataset-a", "dataset-b"
	for i := 0; i < 14; i++ {
		storeA := dailyFact("s1", "S1", "CNY", from.AddDate(0, 0, i), 100)
		storeA.SourceSystem = "source-a"
		storeA.SimulationDatasetVersion = &datasetA
		if i >= 7 {
			storeA.Revenue = ptr(80)
		}
		storeB := dailyFact("s2", "S2", "CNY", from.AddDate(0, 0, i), 100)
		storeB.SourceSystem = "source-b"
		storeB.SimulationDatasetVersion = &datasetB
		facts = append(facts, storeA, storeB)
	}
	response, err := NewService(&fakeReader{set: &repository.RetailKPIFactSet{Facts: facts, ExpectedStoreCount: 2}}).Build(context.Background(), Query{LegalEntityID: "entity-a", AsOf: from.AddDate(0, 0, 13), Classification: "production", AttentionLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	for _, attention := range response.Attention {
		if attention.StoreCode == "S1" {
			if len(attention.Evidence.SourceSystems) != 1 || attention.Evidence.SourceSystems[0] != "source-a" || len(attention.Evidence.DatasetVersions) != 1 || attention.Evidence.DatasetVersions[0] != "dataset-a" {
				t.Fatalf("store evidence leaked other-store provenance: %+v", attention.Evidence)
			}
			return
		}
	}
	t.Fatal("S1 was not ranked for revenue decline")
}

func TestContributionTurnsNegativeUsesFixedOneScoreException(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	facts := make([]retailkpi.DailyFact, 0, 14)
	for i := 0; i < 14; i++ {
		fact := dailyFact("s1", "S1", "CNY", from.AddDate(0, 0, i), 100)
		if i >= 7 {
			fact.OtherControllableCost = ptr(100)
		}
		facts = append(facts, fact)
	}
	response, err := NewService(&fakeReader{set: &repository.RetailKPIFactSet{Facts: facts, ExpectedStoreCount: 1}}).Build(context.Background(), Query{LegalEntityID: "entity-a", AsOf: from.AddDate(0, 0, 13), Classification: "production", AttentionLimit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Attention) != 1 || response.Attention[0].Severity != "medium" {
		t.Fatalf("negative contribution attention=%+v", response.Attention)
	}
	for _, signal := range response.Attention[0].ObservedSignals {
		if signal.SignalCode == "contribution_turns_negative" {
			if signal.Threshold != 0 || signal.ScoreContribution != 1 || signal.ObservedChange == nil || *signal.ObservedChange >= 0 {
				t.Fatalf("negative contribution signal=%+v", signal)
			}
			return
		}
	}
	t.Fatalf("negative contribution signal missing: %+v", response.Attention[0].ObservedSignals)
}

func simulationFacts(plan *retailsimulation.Plan) []retailkpi.DailyFact {
	stores := map[string]retailsimulation.StorePlan{}
	for _, store := range plan.Stores {
		stores[store.Code] = store
	}
	result := make([]retailkpi.DailyFact, 0, len(plan.Facts))
	for _, fact := range plan.Facts {
		store := stores[fact.StoreCode]
		date, _ := time.Parse("2006-01-02", fact.BusinessDate)
		result = append(result, retailkpi.DailyFact{StoreID: fact.StoreCode, StoreCode: fact.StoreCode, StoreName: store.Name, Brand: store.Brand, Region: store.Region, BusinessDate: date, Currency: fact.Currency, SourceSystem: "retail_simulator", AsOfAt: date, Version: 1, Revenue: ptr(fact.Revenue), GrossProfit: ptr(fact.GrossProfit), Transactions: ptr(fact.Transactions), Footfall: ptr(fact.Footfall), AreaSqm: ptr(fact.AreaSqm), LaborCost: ptr(fact.LaborCost), FixedRent: ptr(fact.FixedRent), VariableRent: ptr(fact.VariableRent), NonLeaseCost: ptr(fact.NonLeaseCost), OtherControllableCost: ptr(fact.OtherControllableCost), MappingStatus: "mapped", DataQualityStatus: "valid", DataClassification: "simulated", SimulationDatasetVersion: &plan.DatasetVersion})
	}
	return result
}

func dailyFact(storeID, storeCode, currency string, date time.Time, revenue float64) retailkpi.DailyFact {
	return retailkpi.DailyFact{StoreID: storeID, StoreCode: storeCode, StoreName: storeCode, Brand: "Brand", Region: "Region", BusinessDate: date, Currency: currency, SourceSystem: "unit_source", AsOfAt: date, Version: 1, Revenue: ptr(revenue), GrossProfit: ptr(revenue * 0.3), Transactions: ptr(10), Footfall: ptr(20), AreaSqm: ptr(10), LaborCost: ptr(revenue * 0.1), FixedRent: ptr(1), VariableRent: ptr(1), NonLeaseCost: ptr(1), OtherControllableCost: ptr(1), MappingStatus: "mapped", DataQualityStatus: "valid", DataClassification: "production"}
}
func valueOrZero(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}
