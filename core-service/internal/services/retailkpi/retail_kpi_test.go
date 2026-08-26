package retailkpi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/services/retailsimulation"
)

type goldenMetricSet struct {
	FactCount         int                `json:"fact_count"`
	ExpectedStoreDays int                `json:"expected_store_days"`
	CoverageRate      float64            `json:"coverage_rate"`
	KPIs              map[string]float64 `json:"kpis"`
}
type goldenAnomaly struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	DateFrom string             `json:"date_from"`
	DateTo   string             `json:"date_to"`
	KPIs     map[string]float64 `json:"kpis"`
}
type goldenKPIValue struct {
	Value  *float64 `json:"value"`
	Status string   `json:"status"`
	Reason string   `json:"reason"`
}
type goldenDocument struct {
	FormulaVersion  string                     `json:"formula_version"`
	DefaultOverall  goldenMetricSet            `json:"default_overall"`
	ControlStore001 goldenMetricSet            `json:"control_store_001"`
	Anomalies       []goldenAnomaly            `json:"anomalies"`
	ZeroDenominator map[string]goldenKPIValue  `json:"zero_denominator"`
	MissingField    map[string]json.RawMessage `json:"missing_field"`
}

func ptr(v float64) *float64 { return &v }

func TestAggregateStrictNullZeroDenominatorAndAreaSemantics(t *testing.T) {
	facts := []DailyFact{
		{StoreID: "s1", StoreCode: "S1", StoreName: "One", Brand: "A", Region: "N", Currency: "CNY", BusinessDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Revenue: ptr(100), GrossProfit: ptr(40), Transactions: ptr(10), Footfall: ptr(20), AreaSqm: ptr(50), LaborCost: ptr(10), FixedRent: ptr(5), VariableRent: ptr(2), NonLeaseCost: ptr(3), OtherControllableCost: ptr(4), MappingStatus: "mapped"},
		{StoreID: "s1", StoreCode: "S1", StoreName: "One", Brand: "A", Region: "N", Currency: "CNY", BusinessDate: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), Revenue: ptr(100), GrossProfit: ptr(40), Transactions: ptr(0), Footfall: ptr(0), AreaSqm: ptr(50), LaborCost: ptr(10), FixedRent: ptr(5), VariableRent: ptr(2), NonLeaseCost: ptr(3), OtherControllableCost: ptr(4), MappingStatus: "mapped"},
	}
	rows, coverage, err := AggregateFacts(facts, Request{DateFrom: facts[0].BusinessDate, DateTo: facts[1].BusinessDate, RequestedDateFrom: "2026-01-01", RequestedDateTo: "2026-01-02", GroupBy: "total", ExpectedStoreCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if coverage.ObservedStoreDays != 2 || coverage.ExpectedStoreDays != 2 || coverage.CoverageRate == nil || *coverage.CoverageRate != 100 {
		t.Fatalf("coverage = %+v", coverage)
	}
	got := rows[0]
	if got.DistinctBusinessDays != 2 || got.AverageDailyAreaSqm == nil || *got.AverageDailyAreaSqm != 50 {
		t.Fatalf("area semantics = %+v", got)
	}
	if got.KPIs["revenue"].Value == nil || *got.KPIs["revenue"].Value != 200 {
		t.Fatalf("revenue = %+v", got.KPIs["revenue"])
	}
	if got.KPIs["conversion_rate"].Value == nil || *got.KPIs["conversion_rate"].Value != 50 {
		t.Fatalf("conversion with one zero day = %+v", got.KPIs["conversion_rate"])
	}
	zeroRows, _, _ := AggregateFacts([]DailyFact{{StoreID: "zero", Currency: "CNY", BusinessDate: facts[0].BusinessDate, Revenue: ptr(0), GrossProfit: ptr(0), Transactions: ptr(0), Footfall: ptr(0), AreaSqm: ptr(10), LaborCost: ptr(0), FixedRent: ptr(0), VariableRent: ptr(0), NonLeaseCost: ptr(0), OtherControllableCost: ptr(0), MappingStatus: "mapped"}}, Request{DateFrom: facts[0].BusinessDate, DateTo: facts[0].BusinessDate, RequestedDateFrom: "2026-01-01", RequestedDateTo: "2026-01-01", GroupBy: "total", ExpectedStoreCount: 1})
	if zeroRows[0].KPIs["conversion_rate"].Value != nil || zeroRows[0].KPIs["conversion_rate"].Reason != "zero_denominator" || zeroRows[0].KPIs["conversion_rate"].Status != StatusUnavailable {
		t.Fatalf("zero denominator = %+v", zeroRows[0].KPIs["conversion_rate"])
	}
	if got.KPIs["occupancy_cash_cost"].Value == nil || *got.KPIs["occupancy_cash_cost"].Value != 20 {
		t.Fatalf("occupancy cash = %+v", got.KPIs["occupancy_cash_cost"])
	}
	if got.KPIs["store_contribution"].Value == nil || *got.KPIs["store_contribution"].Value != 32 {
		t.Fatalf("contribution = %+v", got.KPIs["store_contribution"])
	}
	if !got.DecisionReady {
		t.Fatal("complete valid facts should be decision ready")
	}
	partialRows, partialCoverage, err := AggregateFacts(facts[:1], Request{DateFrom: facts[0].BusinessDate, DateTo: facts[1].BusinessDate, RequestedDateFrom: "2026-01-01", RequestedDateTo: "2026-01-02", GroupBy: "store", ExpectedStoreCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if partialCoverage.CoverageRate == nil || *partialCoverage.CoverageRate != 50 || partialRows[0].DecisionReady || !containsIssue(partialRows[0].DataQualityIssues, "incomplete_store_day_coverage") {
		t.Fatalf("incomplete coverage was decision ready: coverage=%+v row=%+v", partialCoverage, partialRows[0])
	}

	facts[1].GrossProfit = nil
	rows, _, err = AggregateFacts(facts, Request{DateFrom: facts[0].BusinessDate, DateTo: facts[1].BusinessDate, RequestedDateFrom: "2026-01-01", RequestedDateTo: "2026-01-02", GroupBy: "total", ExpectedStoreCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].KPIs["gross_profit"].Value != nil || rows[0].KPIs["gross_profit"].Status != StatusPartial || rows[0].DecisionReady {
		t.Fatalf("missing input was not partial: %+v decision=%v", rows[0].KPIs["gross_profit"], rows[0].DecisionReady)
	}
}

func containsIssue(issues []string, target string) bool {
	for _, issue := range issues {
		if issue == target {
			return true
		}
	}
	return false
}

func TestAggregateGroupingAndCurrencySeparation(t *testing.T) {
	date := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	facts := []DailyFact{
		{StoreID: "s1", StoreCode: "S1", StoreName: "One", Brand: "A", Region: "North", Currency: "CNY", BusinessDate: date, Revenue: ptr(100), GrossProfit: ptr(50), Transactions: ptr(10), Footfall: ptr(20), AreaSqm: ptr(10), LaborCost: ptr(10), FixedRent: ptr(5), VariableRent: ptr(1), NonLeaseCost: ptr(1), OtherControllableCost: ptr(1), MappingStatus: "mapped"},
		{StoreID: "s2", StoreCode: "S2", StoreName: "Two", Brand: "B", Region: "South", Currency: "USD", BusinessDate: date, Revenue: ptr(200), GrossProfit: ptr(100), Transactions: ptr(20), Footfall: ptr(40), AreaSqm: ptr(20), LaborCost: ptr(20), FixedRent: ptr(10), VariableRent: ptr(2), NonLeaseCost: ptr(2), OtherControllableCost: ptr(2), MappingStatus: "mapped"},
	}
	rows, _, err := AggregateFacts(facts, Request{DateFrom: date, DateTo: date, RequestedDateFrom: "2026-01-01", RequestedDateTo: "2026-01-01", GroupBy: "region", ExpectedStoreCount: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Currency == rows[1].Currency {
		t.Fatalf("currency rows were summed: %+v", rows)
	}
	storeRows, _, err := AggregateFacts(facts, Request{DateFrom: date, DateTo: date, RequestedDateFrom: "2026-01-01", RequestedDateTo: "2026-01-01", GroupBy: "store", ExpectedStoreCount: 2})
	if err != nil || len(storeRows) != 2 {
		t.Fatalf("store grouping rows=%d err=%v", len(storeRows), err)
	}
}

func planFacts(plan *retailsimulation.Plan) []DailyFact {
	stores := make(map[string]retailsimulation.StorePlan, len(plan.Stores))
	for _, store := range plan.Stores {
		stores[store.Code] = store
	}
	result := make([]DailyFact, 0, len(plan.Facts))
	for _, f := range plan.Facts {
		date, _ := time.Parse("2006-01-02", f.BusinessDate)
		store := stores[f.StoreCode]
		result = append(result, DailyFact{StoreID: f.StoreCode, StoreCode: f.StoreCode, StoreName: store.Name, Brand: store.Brand, Region: store.Region, BusinessDate: date, Currency: f.Currency, Revenue: ptr(f.Revenue), GrossProfit: ptr(f.GrossProfit), Transactions: ptr(f.Transactions), Footfall: ptr(f.Footfall), AreaSqm: ptr(f.AreaSqm), LaborCost: ptr(f.LaborCost), FixedRent: ptr(f.FixedRent), VariableRent: ptr(f.VariableRent), NonLeaseCost: ptr(f.NonLeaseCost), OtherControllableCost: ptr(f.OtherControllableCost), MappingStatus: "mapped", DataQualityStatus: "valid", DataClassification: "simulated"})
	}
	return result
}

func TestDefaultGoldenValues(t *testing.T) {
	plan, err := retailsimulation.Build("entity-a", retailsimulation.Input{})
	if err != nil {
		t.Fatal(err)
	}
	facts := planFacts(plan)
	from, _ := time.Parse("2006-01-02", plan.DateFrom)
	to, _ := time.Parse("2006-01-02", plan.DateTo)
	rows, coverage, err := AggregateFacts(facts, Request{DateFrom: from, DateTo: to, RequestedDateFrom: plan.DateFrom, RequestedDateTo: plan.DateTo, GroupBy: "total", ExpectedStoreCount: plan.StoreCount})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("dataset=%s coverage=%+v", plan.DatasetVersion, coverage)
	for _, code := range []string{"revenue", "gross_profit", "footfall", "transactions", "labor_cost", "fixed_rent", "variable_rent", "non_lease_cost", "other_controllable_cost", "occupancy_cash_cost", "store_contribution", "gross_margin_rate", "conversion_rate", "average_transaction_value", "labor_cost_rate", "rent_to_sales_rate", "occupancy_cash_cost_rate", "store_contribution_margin", "average_daily_area_sqm", "sales_per_sqm", "revenue_per_store_day"} {
		k := rows[0].KPIs[code]
		if k.Value == nil {
			t.Logf("%s=null status=%s", code, k.Status)
		} else {
			t.Logf("%s=%.2f", code, *k.Value)
		}
	}
	if !rows[0].DecisionReady || coverage.ObservedStoreDays != 10860 || coverage.ExpectedStoreDays != 10860 {
		t.Fatalf("default not complete: decision=%v coverage=%+v", rows[0].DecisionReady, coverage)
	}
	for _, anomaly := range plan.Anomalies {
		var window []DailyFact
		for _, f := range facts {
			if f.StoreCode == anomaly.StoreCode && f.BusinessDate.Format("2006-01-02") >= anomaly.DateFrom && f.BusinessDate.Format("2006-01-02") <= anomaly.DateTo {
				window = append(window, f)
			}
		}
		start, _ := time.Parse("2006-01-02", anomaly.DateFrom)
		end, _ := time.Parse("2006-01-02", anomaly.DateTo)
		rows, _, _ := AggregateFacts(window, Request{DateFrom: start, DateTo: end, RequestedDateFrom: anomaly.DateFrom, RequestedDateTo: anomaly.DateTo, GroupBy: "store", ExpectedStoreCount: 1})
		for _, code := range []string{"revenue", "gross_profit", "footfall", "transactions", "store_contribution", "gross_margin_rate", "conversion_rate", "average_transaction_value", "occupancy_cash_cost_rate"} {
			k := rows[0].KPIs[code]
			if k.Value != nil {
				t.Logf("anomaly=%s type=%s %s=%.2f", anomaly.ID, anomaly.Type, code, *k.Value)
			}
		}
	}
	control := facts[:0]
	for _, f := range facts {
		if f.StoreCode == plan.Stores[0].Code {
			control = append(control, f)
		}
	}
	rows, _, _ = AggregateFacts(control, Request{DateFrom: from, DateTo: to, RequestedDateFrom: plan.DateFrom, RequestedDateTo: plan.DateTo, GroupBy: "store", ExpectedStoreCount: 1})
	for _, code := range []string{"revenue", "gross_profit", "footfall", "transactions", "store_contribution", "gross_margin_rate", "conversion_rate", "average_transaction_value"} {
		k := rows[0].KPIs[code]
		if k.Value != nil {
			t.Logf("control %s=%.2f", code, *k.Value)
		}
	}
}

func TestCommittedGoldenConstants(t *testing.T) {
	path := filepath.Join("testdata", "retail_kpi_v1_golden.json")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var golden goldenDocument
	if err := json.Unmarshal(content, &golden); err != nil {
		t.Fatal(err)
	}
	if golden.FormulaVersion != FormulaVersion {
		t.Fatalf("golden formula version=%s", golden.FormulaVersion)
	}
	plan, err := retailsimulation.Build("entity-a", retailsimulation.Input{})
	if err != nil {
		t.Fatal(err)
	}
	facts := planFacts(plan)
	from, _ := time.Parse("2006-01-02", plan.DateFrom)
	to, _ := time.Parse("2006-01-02", plan.DateTo)
	rows, coverage, err := AggregateFacts(facts, Request{DateFrom: from, DateTo: to, RequestedDateFrom: plan.DateFrom, RequestedDateTo: plan.DateTo, GroupBy: "total", ExpectedStoreCount: plan.StoreCount})
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != golden.DefaultOverall.FactCount || coverage.ExpectedStoreDays != golden.DefaultOverall.ExpectedStoreDays || coverage.CoverageRate == nil || *coverage.CoverageRate != golden.DefaultOverall.CoverageRate {
		t.Fatalf("default coverage mismatch: got=%+v", coverage)
	}
	assertGoldenMetrics(t, rows[0], golden.DefaultOverall.KPIs)
	var control []DailyFact
	for _, f := range facts {
		if f.StoreCode == plan.Stores[0].Code {
			control = append(control, f)
		}
	}
	controlRows, _, err := AggregateFacts(control, Request{DateFrom: from, DateTo: to, RequestedDateFrom: plan.DateFrom, RequestedDateTo: plan.DateTo, GroupBy: "store", ExpectedStoreCount: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(control) != golden.ControlStore001.FactCount {
		t.Fatalf("control fact count=%d", len(control))
	}
	assertGoldenMetrics(t, controlRows[0], golden.ControlStore001.KPIs)
	if len(plan.Anomalies) != len(golden.Anomalies) {
		t.Fatalf("anomaly golden count=%d plan=%d", len(golden.Anomalies), len(plan.Anomalies))
	}
	for i, anomaly := range plan.Anomalies {
		want := golden.Anomalies[i]
		if anomaly.ID != want.ID || anomaly.Type != want.Type || anomaly.DateFrom != want.DateFrom || anomaly.DateTo != want.DateTo {
			t.Fatalf("anomaly[%d] metadata got=%+v want=%+v", i, anomaly, want)
		}
		var window []DailyFact
		for _, f := range facts {
			date := f.BusinessDate.Format("2006-01-02")
			if f.StoreCode == anomaly.StoreCode && date >= anomaly.DateFrom && date <= anomaly.DateTo {
				window = append(window, f)
			}
		}
		start, _ := time.Parse("2006-01-02", anomaly.DateFrom)
		end, _ := time.Parse("2006-01-02", anomaly.DateTo)
		windowRows, _, err := AggregateFacts(window, Request{DateFrom: start, DateTo: end, RequestedDateFrom: anomaly.DateFrom, RequestedDateTo: anomaly.DateTo, GroupBy: "store", ExpectedStoreCount: 1})
		if err != nil {
			t.Fatal(err)
		}
		assertGoldenMetrics(t, windowRows[0], want.KPIs)
	}
	zeroFacts := []DailyFact{{StoreID: "zero", StoreCode: "ZERO", Currency: "CNY", BusinessDate: from, Revenue: ptr(0), GrossProfit: ptr(0), Transactions: ptr(0), Footfall: ptr(0), AreaSqm: ptr(0), LaborCost: ptr(0), FixedRent: ptr(0), VariableRent: ptr(0), NonLeaseCost: ptr(0), OtherControllableCost: ptr(0), MappingStatus: "mapped"}}
	zeroRows, _, _ := AggregateFacts(zeroFacts, Request{DateFrom: from, DateTo: from, RequestedDateFrom: plan.DateFrom, RequestedDateTo: plan.DateFrom, GroupBy: "total", ExpectedStoreCount: 1})
	for code, want := range golden.ZeroDenominator {
		got := zeroRows[0].KPIs[code]
		if got.Status != KPIStatus(want.Status) || got.Reason != want.Reason || got.Value != nil {
			t.Fatalf("zero denominator %s got=%+v want=%+v", code, got, want)
		}
	}
	missing := zeroFacts[0]
	missing.GrossProfit = nil
	missingRows, _, _ := AggregateFacts([]DailyFact{missing}, Request{DateFrom: from, DateTo: from, RequestedDateFrom: plan.DateFrom, RequestedDateTo: plan.DateFrom, GroupBy: "total", ExpectedStoreCount: 1})
	var missingWant struct {
		Value          *float64 `json:"value"`
		Status, Reason string
	}
	if err := json.Unmarshal(golden.MissingField["gross_profit"], &missingWant); err != nil {
		t.Fatal(err)
	}
	gotMissing := missingRows[0].KPIs["gross_profit"]
	if gotMissing.Value != nil || string(gotMissing.Status) != missingWant.Status || gotMissing.Reason != missingWant.Reason || missingRows[0].DecisionReady {
		t.Fatalf("missing golden mismatch got=%+v", gotMissing)
	}
}

func assertGoldenMetrics(t *testing.T, row Aggregate, expected map[string]float64) {
	t.Helper()
	for code, want := range expected {
		got := row.KPIs[code].Value
		if got == nil {
			t.Fatalf("golden %s is null", code)
		}
		delta := *got - want
		if delta < -0.011 || delta > 0.011 {
			t.Fatalf("golden %s expected=%.2f actual=%.2f delta=%.4f", code, want, *got, delta)
		}
	}
}

// K3: the single Fact Coverage verdict. Over-coverage (observed > expected)
// is not incomplete — the store-360 engine previously used `!=` and wiped
// the peer benchmark on any over-coverage; every read now shares this rule.
func TestCoverageIncompleteSingleVerdict(t *testing.T) {
	cases := []struct {
		name     string
		coverage Coverage
		want     bool
	}{
		{"under coverage", Coverage{ObservedStoreDays: 5, ExpectedStoreDays: 7}, true},
		{"exact coverage", Coverage{ObservedStoreDays: 7, ExpectedStoreDays: 7}, false},
		{"over coverage", Coverage{ObservedStoreDays: 10, ExpectedStoreDays: 7}, false},
		{"unknown expected", Coverage{ObservedStoreDays: 7, ExpectedStoreDays: 0}, false},
		{"empty", Coverage{}, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := CoverageIncomplete(testCase.coverage); got != testCase.want {
				t.Fatalf("CoverageIncomplete(%+v) = %v, want %v", testCase.coverage, got, testCase.want)
			}
			// Complete means expected known and fully observed; with an
			// unknown expected population neither verdict fires.
			wantComplete := testCase.coverage.ExpectedStoreDays > 0 && !testCase.want
			if got := CoverageComplete(testCase.coverage); got != wantComplete {
				t.Fatalf("CoverageComplete(%+v) = %v, want %v", testCase.coverage, got, wantComplete)
			}
		})
	}
}

// M1 / P0-3: the percent change denominator takes |comparison| so a negative
// base keeps a stable direction. This is the GUARD-001 rule-body proof that
// the replacement formula is live: improving from -100 to -50 must read as
// +50 (improvement), which the retired (c/p-1) form reported as -50.
func TestChangeRateNegativeBaseDirection(t *testing.T) {
	current, comparison := -50.0, -100.0
	got, reason := ChangeRate(&current, &comparison, "percent")
	if reason != "" || got == nil || *got != 50 {
		t.Fatalf("ChangeRate(-50, -100, percent) = (%+v, %q), want (+50, \"\")", got, reason)
	}
	// Worsening on a negative base reads as negative: -100 back to -150.
	current, comparison = -150.0, -100.0
	got, reason = ChangeRate(&current, &comparison, "percent")
	if reason != "" || got == nil || *got != -50 {
		t.Fatalf("ChangeRate(-150, -100, percent) = (%+v, %q), want (-50, \"\")", got, reason)
	}
	// Positive bases keep the classic ratio result: 120 vs 80 stays +50.
	current, comparison = 120.0, 80.0
	got, reason = ChangeRate(&current, &comparison, "percent")
	if reason != "" || got == nil || *got != 50 {
		t.Fatalf("ChangeRate(120, 80, percent) = (%+v, %q), want (+50, \"\")", got, reason)
	}
	// Percentage-point changes remain plain differences on any base sign.
	current, comparison = 20.0, 35.0
	got, reason = ChangeRate(&current, &comparison, "percentage_point")
	if reason != "" || got == nil || *got != -15 {
		t.Fatalf("ChangeRate(20, 35, percentage_point) = (%+v, %q), want (-15, \"\")", got, reason)
	}
}

// P0-8: duplicated (store, business_date) rows are over-coverage with
// polluted sums — the alarm is the symmetric counterpart of the incomplete
// verdict, and it downgrades decision readiness instead of hiding behind a
// >100% coverage rate.
func TestAggregateDuplicateStoreDayRowsAlarm(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newFact := func(day int) DailyFact {
		return DailyFact{StoreID: "s1", StoreCode: "S1", StoreName: "One", Brand: "A", Region: "N", Currency: "CNY", BusinessDate: base.AddDate(0, 0, day), Revenue: ptr(100), GrossProfit: ptr(40), Transactions: ptr(10), Footfall: ptr(20), AreaSqm: ptr(50), LaborCost: ptr(10), FixedRent: ptr(5), VariableRent: ptr(2), NonLeaseCost: ptr(3), OtherControllableCost: ptr(4), MappingStatus: "mapped"}
	}
	req := Request{DateFrom: base, DateTo: base.AddDate(0, 0, 6), RequestedDateFrom: "2026-01-01", RequestedDateTo: "2026-01-07", GroupBy: "total", ExpectedStoreCount: 1}
	clean := make([]DailyFact, 0, 8)
	for day := 0; day < 7; day++ {
		clean = append(clean, newFact(day))
	}
	rows, coverage, err := AggregateFacts(clean, req)
	if err != nil {
		t.Fatal(err)
	}
	if coverage.DuplicateStoreDays != 0 || !rows[0].DecisionReady {
		t.Fatalf("clean read should stay ready: coverage=%+v ready=%v", coverage, rows[0].DecisionReady)
	}
	duplicated := append(append([]DailyFact{}, clean...), newFact(3))
	rows, coverage, err = AggregateFacts(duplicated, req)
	if err != nil {
		t.Fatal(err)
	}
	if coverage.DuplicateStoreDays != 1 || coverage.ObservedStoreDays != 8 {
		t.Fatalf("duplicate accounting = %+v", coverage)
	}
	if rows[0].DecisionReady {
		t.Fatal("duplicated store-day rows must downgrade decision readiness")
	}
	found := false
	for _, issue := range rows[0].DataQualityIssues {
		if issue == "duplicate_store_day_rows" {
			found = true
		}
	}
	if !found {
		t.Fatalf("duplicate alarm missing from issues: %+v", rows[0].DataQualityIssues)
	}
}

func TestMeasureKindStockVersusFlow(t *testing.T) {
	// Guard N4 / GUARD-001: stock measures (average_daily_area_sqm) average across
	// distinct business days and are never summed, while flow measures (revenue) sum.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	facts := []DailyFact{
		{StoreID: "s1", StoreCode: "S1", StoreName: "Store 1", Currency: "CNY", BusinessDate: base, Revenue: ptr(500), GrossProfit: ptr(200), Transactions: ptr(50), Footfall: ptr(100), AreaSqm: ptr(120), LaborCost: ptr(50), FixedRent: ptr(30), VariableRent: ptr(10), NonLeaseCost: ptr(5), OtherControllableCost: ptr(10), MappingStatus: "mapped"},
		{StoreID: "s1", StoreCode: "S1", StoreName: "Store 1", Currency: "CNY", BusinessDate: base.AddDate(0, 0, 1), Revenue: ptr(700), GrossProfit: ptr(280), Transactions: ptr(70), Footfall: ptr(140), AreaSqm: ptr(120), LaborCost: ptr(50), FixedRent: ptr(30), VariableRent: ptr(14), NonLeaseCost: ptr(5), OtherControllableCost: ptr(10), MappingStatus: "mapped"},
	}

	rows, _, err := AggregateFacts(facts, Request{DateFrom: base, DateTo: base.AddDate(0, 0, 1), RequestedDateFrom: "2026-01-01", RequestedDateTo: "2026-01-02", GroupBy: "total", ExpectedStoreCount: 1})
	if err != nil {
		t.Fatal(err)
	}

	kpis := rows[0].KPIs
	// Stock: average_daily_area_sqm is 120 (not 240)
	if kpis["average_daily_area_sqm"].Value == nil || *kpis["average_daily_area_sqm"].Value != 120 {
		t.Fatalf("expected stock average 120 sqm, got %v", kpis["average_daily_area_sqm"].Value)
	}
	// Flow: revenue is 500 + 700 = 1200
	if kpis["revenue"].Value == nil || *kpis["revenue"].Value != 1200 {
		t.Fatalf("expected flow sum 1200, got %v", kpis["revenue"].Value)
	}
	// Sales per sqm: 1200 / 120 = 10
	if kpis["sales_per_sqm"].Value == nil || *kpis["sales_per_sqm"].Value != 10 {
		t.Fatalf("expected sales_per_sqm 10, got %v", kpis["sales_per_sqm"].Value)
	}

	// Verify Definition MeasureKind properties
	areaDef := findDefinition("average_daily_area_sqm")
	if areaDef == nil || areaDef.MeasureKind != MeasureKindStock {
		t.Fatalf("average_daily_area_sqm must be MeasureKindStock, got %+v", areaDef)
	}
	revDef := findDefinition("revenue")
	if revDef == nil || revDef.MeasureKind != MeasureKindFlow {
		t.Fatalf("revenue must be MeasureKindFlow, got %+v", revDef)
	}
}

func TestLaborProductivityMetrics(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// Case 1: Complete labor hours facts
	factsWithLabor := []DailyFact{
		{StoreID: "s1", StoreCode: "S1", StoreName: "Store 1", Currency: "CNY", BusinessDate: base, Revenue: ptr(1000), GrossProfit: ptr(400), Transactions: ptr(50), Footfall: ptr(100), AreaSqm: ptr(100), LaborCost: ptr(200), FixedRent: ptr(50), VariableRent: ptr(20), NonLeaseCost: ptr(10), OtherControllableCost: ptr(20), LaborHours: ptr(25), Headcount: ptr(4), MappingStatus: "mapped"},
	}

	rows, _, err := AggregateFacts(factsWithLabor, Request{DateFrom: base, DateTo: base, RequestedDateFrom: "2026-01-01", RequestedDateTo: "2026-01-01", GroupBy: "total", ExpectedStoreCount: 1})
	if err != nil {
		t.Fatal(err)
	}

	kpis := rows[0].KPIs
	// Sales per labor hour: 1000 / 25 = 40
	if kpis["sales_per_labor_hour"].Value == nil || *kpis["sales_per_labor_hour"].Value != 40 || kpis["sales_per_labor_hour"].Status != StatusComplete {
		t.Fatalf("expected sales_per_labor_hour 40 (complete), got %+v", kpis["sales_per_labor_hour"])
	}
	// Labor hours per transaction: 25 / 50 = 0.5
	if kpis["labor_hours_per_transaction"].Value == nil || *kpis["labor_hours_per_transaction"].Value != 0.5 || kpis["labor_hours_per_transaction"].Status != StatusComplete {
		t.Fatalf("expected labor_hours_per_transaction 0.5 (complete), got %+v", kpis["labor_hours_per_transaction"])
	}

	// Case 2: Missing labor hours should return StatusPartial with missing_required_field without failing core DecisionReady
	factsWithoutLabor := []DailyFact{
		{StoreID: "s1", StoreCode: "S1", StoreName: "Store 1", Currency: "CNY", BusinessDate: base, Revenue: ptr(1000), GrossProfit: ptr(400), Transactions: ptr(50), Footfall: ptr(100), AreaSqm: ptr(100), LaborCost: ptr(200), FixedRent: ptr(50), VariableRent: ptr(20), NonLeaseCost: ptr(10), OtherControllableCost: ptr(20), LaborHours: nil, Headcount: nil, MappingStatus: "mapped"},
	}

	rowsMissing, _, err := AggregateFacts(factsWithoutLabor, Request{DateFrom: base, DateTo: base, RequestedDateFrom: "2026-01-01", RequestedDateTo: "2026-01-01", GroupBy: "total", ExpectedStoreCount: 1})
	if err != nil {
		t.Fatal(err)
	}

	kpisMissing := rowsMissing[0].KPIs
	if kpisMissing["sales_per_labor_hour"].Status != StatusPartial || kpisMissing["sales_per_labor_hour"].Reason != "missing_required_field" || kpisMissing["sales_per_labor_hour"].Value != nil {
		t.Fatalf("missing labor_hours should yield partial with missing_required_field, got %+v", kpisMissing["sales_per_labor_hour"])
	}
	// Core decision readiness should remain true even when optional labor hours are missing
	if !rowsMissing[0].DecisionReady {
		t.Fatalf("missing optional labor_hours must not break core DecisionReady status")
	}

	// Case 3: Zero labor hours / Zero transactions denominator check
	factsZero := []DailyFact{
		{StoreID: "s1", StoreCode: "S1", StoreName: "Store 1", Currency: "CNY", BusinessDate: base, Revenue: ptr(1000), GrossProfit: ptr(400), Transactions: ptr(0), Footfall: ptr(100), AreaSqm: ptr(100), LaborCost: ptr(200), FixedRent: ptr(50), VariableRent: ptr(20), NonLeaseCost: ptr(10), OtherControllableCost: ptr(20), LaborHours: ptr(0), Headcount: ptr(0), MappingStatus: "mapped"},
	}

	rowsZero, _, _ := AggregateFacts(factsZero, Request{DateFrom: base, DateTo: base, RequestedDateFrom: "2026-01-01", RequestedDateTo: "2026-01-01", GroupBy: "total", ExpectedStoreCount: 1})
	kpisZero := rowsZero[0].KPIs
	if kpisZero["sales_per_labor_hour"].Status != StatusUnavailable || kpisZero["sales_per_labor_hour"].Reason != "zero_denominator" {
		t.Fatalf("zero labor hours should yield zero_denominator, got %+v", kpisZero["sales_per_labor_hour"])
	}
	if kpisZero["labor_hours_per_transaction"].Status != StatusUnavailable || kpisZero["labor_hours_per_transaction"].Reason != "zero_denominator" {
		t.Fatalf("zero transactions should yield zero_denominator, got %+v", kpisZero["labor_hours_per_transaction"])
	}
}
