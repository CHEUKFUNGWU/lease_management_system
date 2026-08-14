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
