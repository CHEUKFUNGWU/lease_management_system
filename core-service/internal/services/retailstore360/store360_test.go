package retailstore360

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/services/retailsimulation"
)

type fakeReader struct {
	set   *repository.RetailKPIFactSet
	calls int
}

func (r *fakeReader) QueryFacts(context.Context, string, string, string, string, string, string, []string) (*repository.RetailKPIFactSet, error) {
	r.calls++
	return r.set, nil
}

func float(v float64) *float64 { return &v }

func fixture() *repository.RetailKPIFactSet {
	stores := make([]retailkpi.StorePopulation, 0, 5)
	facts := make([]retailkpi.DailyFact, 0, 5*14)
	for i := 0; i < 5; i++ {
		id := "store-" + string(rune('a'+i))
		stores = append(stores, retailkpi.StorePopulation{StoreID: id, StoreCode: "S00" + string(rune('2'+i)), StoreName: "门店" + string(rune('2'+i)), Brand: "品牌A", Region: "华东"})
		for d := 0; d < 14; d++ {
			date := time.Date(2026, 6, 1+d, 0, 0, 0, 0, time.UTC)
			base := float64(1000 + i*100 + d*10)
			facts = append(facts, retailkpi.DailyFact{StoreID: id, StoreCode: stores[i].StoreCode, StoreName: stores[i].StoreName, Brand: "品牌A", Region: "华东", BusinessDate: date, AsOfAt: date.Add(24 * time.Hour), Currency: "CNY", SourceSystem: "retail_simulator", Version: 1, Revenue: float(base), GrossProfit: float(base * .3), Transactions: float(float64(100 + i)), Footfall: float(float64(500 + i)), AreaSqm: float(100), LaborCost: float(100), FixedRent: float(80), VariableRent: float(10), NonLeaseCost: float(10), OtherControllableCost: float(20), DataQualityStatus: "valid", MappingStatus: "mapped", DataClassification: "simulated"})
		}
	}
	dataset := "planA-v1"
	return &repository.RetailKPIFactSet{Facts: facts, ExpectedStoreCount: len(stores), ExpectedStores: stores, SourceSystems: []string{"retail_simulator"}, DatasetVersions: []string{dataset}, MinFactVersion: 1, MaxFactVersion: 1}
}

func TestBuildUsesOneQueryAndConservesBridges(t *testing.T) {
	reader := &fakeReader{set: fixture()}
	svc := NewService(reader)
	result, err := svc.Build(context.Background(), Query{LegalEntityID: "entity-a", StoreID: "store-a", AsOf: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC), WindowDays: 7, Classification: "simulated", DatasetVersion: "planA-v1"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if reader.calls != 1 {
		t.Fatalf("QueryFacts calls = %d, want 1", reader.calls)
	}
	if result.Current.DateFrom != "2026-06-08" || result.Current.DateTo != "2026-06-14" || result.Comparison.DateTo != "2026-06-07" {
		t.Fatalf("non-overlapping periods: %+v %+v", result.Current, result.Comparison)
	}
	if len(result.PeerBenchmark) == 0 || result.PeerBenchmark[0].PeerCount != 4 {
		t.Fatalf("peer exclusion/count failed: %+v", result.PeerBenchmark[0])
	}
	references := map[string]bool{}
	for _, bridge := range result.Bridges {
		if bridge.Status != "complete" {
			t.Fatalf("bridge unavailable: %+v", bridge)
		}
		if residual := bridgeConservationForTest(bridge); residual == nil || *residual > 0.01 || *residual < -0.01 {
			t.Fatalf("bridge not conserved: %+v residual=%v", bridge, residual)
		}
		for _, observation := range result.Observations {
			references[observation.Reference] = true
			if strings.Contains(observation.Statement, "根因") || strings.Contains(observation.Statement, "因果") {
				t.Fatalf("forbidden causal language: %q", observation.Statement)
			}
		}
	}
	if !references["summary:revenue"] || !references["benchmark:revenue"] || !references["revenue"] {
		t.Fatalf("observations did not consume summary/benchmark/bridge: %+v", references)
	}
}

func bridgeConservationForTest(bridge Bridge) *float64 {
	if bridge.TotalChange == nil {
		return nil
	}
	v := -(*bridge.TotalChange)
	for _, item := range bridge.Items {
		if item.Contribution != nil {
			v += *item.Contribution
		}
	}
	if bridge.RoundingResidual != nil {
		v += *bridge.RoundingResidual
	}
	return &v
}

func TestBuildTenantAndQueryValidation(t *testing.T) {
	svc := NewService(&fakeReader{set: fixture()})
	_, err := svc.Build(context.Background(), Query{LegalEntityID: "entity-a", StoreID: "missing", AsOf: time.Now(), WindowDays: 7, Classification: "simulated", DatasetVersion: "planA-v1"})
	if !errors.Is(err, ErrStoreNotFound) {
		t.Fatalf("missing store err = %v", err)
	}
	// M2 range contract: 10 is a valid custom window now; 366 is out of range.
	_, err = svc.Build(context.Background(), Query{LegalEntityID: "entity-a", StoreID: "store-a", AsOf: time.Now(), WindowDays: 366, Classification: "simulated", DatasetVersion: "planA-v1"})
	if !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("invalid window err = %v", err)
	}
}

func TestBuildSupportsAllFixedWindowsWithoutOverlap(t *testing.T) {
	for _, days := range []int{7, 14, 28} {
		reader := &fakeReader{set: fixture()}
		result, err := NewService(reader).Build(context.Background(), Query{LegalEntityID: "entity-a", StoreID: "store-a", AsOf: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC), WindowDays: days, Classification: "simulated", DatasetVersion: "planA-v1"})
		if err != nil {
			t.Fatalf("window %d: %v", days, err)
		}
		if result.Current.DateTo >= result.Comparison.DateTo && result.Current.DateFrom <= result.Comparison.DateTo {
			t.Fatalf("window %d overlaps: %+v %+v", days, result.Current, result.Comparison)
		}
		if len(result.DailyTrend) != days {
			t.Fatalf("window %d trend len=%d", days, len(result.DailyTrend))
		}
	}
}

func TestQuantileTieIsDeterministic(t *testing.T) {
	values := []float64{1, 1, 2, 3}
	if got := quantile(values, .5); got != 1.5 {
		t.Fatalf("median = %v", got)
	}
	rank := retailkpi.PercentileRank(values, 1)
	if rank == nil || *rank != 25 {
		t.Fatalf("tie rank = %v", rank)
	}
}

func completeKPI(value float64, unit string) retailkpi.KPIValue {
	return retailkpi.KPIValue{Value: float(value), Unit: unit, Status: retailkpi.StatusComplete, FormulaVersion: retailkpi.FormulaVersion}
}

func TestSummaryUsesRelativePercentAndStableZeroComparisonReason(t *testing.T) {
	current := &retailkpi.Aggregate{KPIs: map[string]retailkpi.KPIValue{"revenue": completeKPI(120, "currency"), "gross_margin_rate": completeKPI(30, "percent")}}
	comparison := &retailkpi.Aggregate{KPIs: map[string]retailkpi.KPIValue{"revenue": completeKPI(80, "currency"), "gross_margin_rate": completeKPI(20, "percent")}}
	summary := makeSummary(current, comparison)
	if summary["revenue"].ChangeValue == nil || *summary["revenue"].ChangeValue != 50 || summary["revenue"].ChangeType != "percent" {
		t.Fatalf("revenue change=%+v, want 50 percent", summary["revenue"])
	}
	if summary["gross_margin_rate"].ChangeValue == nil || *summary["gross_margin_rate"].ChangeValue != 10 || summary["gross_margin_rate"].ChangeType != "percentage_point" {
		t.Fatalf("rate change=%+v, want 10pp", summary["gross_margin_rate"])
	}
	current.KPIs["revenue"] = completeKPI(5, "currency")
	comparison.KPIs["revenue"] = completeKPI(0, "currency")
	zero := makeSummary(current, comparison)["revenue"]
	if zero.ChangeValue != nil || zero.Reason != "zero_comparison" {
		t.Fatalf("zero comparison=%+v", zero)
	}
}

func TestGrossProfitShapleyAveragesBothPermutations(t *testing.T) {
	summary := map[string]SummaryMetric{
		"gross_profit":      {Current: completeKPI(80, "currency"), Comparison: completeKPI(20, "currency")},
		"revenue":           {Current: completeKPI(200, "currency"), Comparison: completeKPI(100, "currency")},
		"gross_margin_rate": {Current: completeKPI(40, "percent"), Comparison: completeKPI(20, "percent")},
	}
	bridge := grossProfitBridge(summary)
	if bridge.Status != "complete" || bridge.Items[0].Contribution == nil || bridge.Items[1].Contribution == nil || math.Abs(*bridge.Items[0].Contribution-30) > 0.001 || math.Abs(*bridge.Items[1].Contribution-30) > 0.001 {
		t.Fatalf("gross profit Shapley=%+v, want 30/30", bridge)
	}
}

func TestMixedCurrencyPeerIsExcludedAndInsufficientPeersAreExplicit(t *testing.T) {
	set := fixture()
	for _, fact := range set.Facts {
		if fact.StoreID == "store-b" && fact.BusinessDate.Format("2006-01-02") == "2026-06-10" {
			mixed := fact
			mixed.Currency = "USD"
			set.Facts = append(set.Facts, mixed)
			break
		}
	}
	result, err := NewService(&fakeReader{set: set}).Build(context.Background(), Query{LegalEntityID: "entity-a", StoreID: "store-a", AsOf: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC), WindowDays: 7, Classification: "simulated", DatasetVersion: "planA-v1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.PeerBenchmark[0].PeerCount != 3 {
		t.Fatalf("mixed currency peer was included: %+v", result.PeerBenchmark[0])
	}
	set.ExpectedStores = set.ExpectedStores[:2]
	result, err = NewService(&fakeReader{set: set}).Build(context.Background(), Query{LegalEntityID: "entity-a", StoreID: "store-a", AsOf: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC), WindowDays: 7, Classification: "simulated", DatasetVersion: "planA-v1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.PeerBenchmark[0].Status != "insufficient_peers" || result.PeerBenchmark[0].Median != nil || result.PeerBenchmark[0].Reason != "peer_count_below_minimum" {
		t.Fatalf("insufficient peers=%+v", result.PeerBenchmark[0])
	}
	for _, row := range result.DailyTrend {
		if row.PeerCount["revenue"] != 0 || row.PeerMedian["revenue"] != nil {
			t.Fatalf("insufficient daily peers must retain count and suppress median: %+v", row)
		}
	}
}

func TestZeroDenominatorMakesBridgeUnavailableWithoutZeroFill(t *testing.T) {
	set := fixture()
	for i := range set.Facts {
		if set.Facts[i].StoreID == "store-a" && !set.Facts[i].BusinessDate.Before(time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC)) {
			set.Facts[i].Revenue = float(0)
			set.Facts[i].Transactions = float(0)
			set.Facts[i].GrossProfit = float(0)
		}
	}
	result, err := NewService(&fakeReader{set: set}).Build(context.Background(), Query{LegalEntityID: "entity-a", StoreID: "store-a", AsOf: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC), WindowDays: 7, Classification: "simulated", DatasetVersion: "planA-v1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary["gross_margin_rate"].Current.Reason != "zero_denominator" || result.Bridges[0].Status != "unavailable" || result.Bridges[0].Items[0].Contribution != nil {
		t.Fatalf("zero denominator result=%+v bridge=%+v", result.Summary["gross_margin_rate"], result.Bridges[0])
	}
}

func TestPartialAndUnavailableFactsDowngradeWithoutZeroFill(t *testing.T) {
	set := fixture()
	partial := set.Facts[:0]
	for _, fact := range set.Facts {
		if fact.StoreID == "store-a" && fact.BusinessDate.Format("2006-01-02") == "2026-06-10" {
			fact.Revenue = nil
		}
		partial = append(partial, fact)
	}
	set.Facts = partial
	partialResult, err := NewService(&fakeReader{set: set}).Build(context.Background(), Query{LegalEntityID: "entity-a", StoreID: "store-a", AsOf: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC), WindowDays: 7, Classification: "simulated", DatasetVersion: "planA-v1"})
	if err != nil {
		t.Fatal(err)
	}
	if partialResult.DecisionReady || partialResult.Summary["revenue"].Current.Status != retailkpi.StatusPartial || partialResult.Bridges[0].Status != "unavailable" {
		t.Fatalf("partial result=%+v", partialResult)
	}
	assertQualityOnlyObservations(t, partialResult.Observations)
	noFacts := fixture()
	filtered := noFacts.Facts[:0]
	for _, fact := range noFacts.Facts {
		if fact.StoreID != "store-a" {
			filtered = append(filtered, fact)
		}
	}
	noFacts.Facts = filtered
	noFactsResult, err := NewService(&fakeReader{set: noFacts}).Build(context.Background(), Query{LegalEntityID: "entity-a", StoreID: "store-a", AsOf: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC), WindowDays: 7, Classification: "simulated", DatasetVersion: "planA-v1"})
	if err != nil {
		t.Fatal(err)
	}
	if noFactsResult.Summary["revenue"].Current.Status != retailkpi.StatusUnavailable || noFactsResult.Bridges[0].Status != "unavailable" || noFactsResult.TargetCoverage.ObservedStoreDays != 0 {
		t.Fatalf("unavailable result=%+v", noFactsResult)
	}
	assertQualityOnlyObservations(t, noFactsResult.Observations)
}

func assertQualityOnlyObservations(t *testing.T, observations []Observation) {
	t.Helper()
	if len(observations) == 0 {
		t.Fatal("decision-not-ready response must retain a data-quality observation")
	}
	for _, observation := range observations {
		if observation.Reference != "evidence" || observation.Status != "unavailable" {
			t.Fatalf("non-quality observation leaked: %+v", observation)
		}
	}
}

func TestCurrencyConflictKeepsObservationsQualityOnly(t *testing.T) {
	set := fixture()
	for i := range set.Facts {
		if set.Facts[i].StoreID == "store-a" && set.Facts[i].BusinessDate.Format("2006-01-02") == "2026-06-10" {
			set.Facts[i].Currency = "USD"
			break
		}
	}
	result, err := NewService(&fakeReader{set: set}).Build(context.Background(), Query{LegalEntityID: "entity-a", StoreID: "store-a", AsOf: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC), WindowDays: 7, Classification: "simulated", DatasetVersion: "planA-v1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.DecisionReady || result.CurrencyStatus != "conflict" {
		t.Fatalf("currency conflict envelope=%+v", result)
	}
	assertQualityOnlyObservations(t, result.Observations)
}

func TestEvidenceFactVersionIsTargetScoped(t *testing.T) {
	set := fixture()
	set.MinFactVersion, set.MaxFactVersion = 1, 9
	for i := range set.Facts {
		if set.Facts[i].StoreID == "store-b" {
			set.Facts[i].Version = 9
		}
	}
	result, err := NewService(&fakeReader{set: set}).Build(context.Background(), Query{LegalEntityID: "entity-a", StoreID: "store-a", AsOf: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC), WindowDays: 7, Classification: "simulated", DatasetVersion: "planA-v1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.FactVersionMax != 1 || result.Evidence.FactVersionMax != 1 {
		t.Fatalf("target evidence leaked peer version: top=%d evidence=%d", result.FactVersionMax, result.Evidence.FactVersionMax)
	}
}

func TestObservationsAreDeterministicAndConsumeSummaryPeerBridge(t *testing.T) {
	query := Query{LegalEntityID: "entity-a", StoreID: "store-a", AsOf: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC), WindowDays: 7, Classification: "simulated", DatasetVersion: "planA-v1"}
	first, err := NewService(&fakeReader{set: fixture()}).Build(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewService(&fakeReader{set: fixture()}).Build(context.Background(), query)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Observations, second.Observations) {
		t.Fatalf("observations are not deterministic: first=%+v second=%+v", first.Observations, second.Observations)
	}
	refs := map[string]bool{}
	for _, observation := range first.Observations {
		refs[observation.Reference] = true
		for _, forbidden := range []string{"根因", "因果", "导致", "证明"} {
			if strings.Contains(observation.Statement, forbidden) {
				t.Fatalf("forbidden observation language %q", observation.Statement)
			}
		}
	}
	for _, reference := range []string{"summary:revenue", "benchmark:revenue", "revenue"} {
		if !refs[reference] {
			t.Fatalf("missing observation reference %s: %+v", reference, refs)
		}
	}
}

func TestDailyTrendGapKeepsValidPeerValues(t *testing.T) {
	set := fixture()
	filtered := set.Facts[:0]
	for _, fact := range set.Facts {
		if fact.StoreID == "store-a" && fact.BusinessDate.Format("2006-01-02") == "2026-06-10" {
			continue
		}
		filtered = append(filtered, fact)
	}
	set.Facts = filtered
	result, err := NewService(&fakeReader{set: set}).Build(context.Background(), Query{LegalEntityID: "entity-a", StoreID: "store-a", AsOf: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC), WindowDays: 7, Classification: "simulated", DatasetVersion: "planA-v1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range result.DailyTrend {
		if row.Gap && (row.PeerMedian["revenue"] == nil || row.PeerCount["revenue"] != 4) {
			t.Fatalf("gap row should keep valid peers: %+v", row)
		}
	}
}

func TestDailyTrendSuppressesMedianWhenDailyPeersAreInsufficient(t *testing.T) {
	set := fixture()
	filtered := set.Facts[:0]
	for _, fact := range set.Facts {
		if fact.BusinessDate.Format("2006-01-02") == "2026-06-10" && (fact.StoreID == "store-b" || fact.StoreID == "store-c") {
			continue
		}
		filtered = append(filtered, fact)
	}
	set.Facts = filtered
	result, err := NewService(&fakeReader{set: set}).Build(context.Background(), Query{LegalEntityID: "entity-a", StoreID: "store-a", AsOf: time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC), WindowDays: 7, Classification: "simulated", DatasetVersion: "planA-v1"})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range result.DailyTrend {
		if row.Date == "2026-06-10" {
			if row.Gap || row.PeerCount["revenue"] != 2 || row.PeerMedian["revenue"] != nil {
				t.Fatalf("daily insufficient peers must retain count and suppress median: %+v", row)
			}
			return
		}
	}
	t.Fatal("daily trend date not found")
}

func TestFixedSeedStore002To007BridgeDirections(t *testing.T) {
	plan, err := retailsimulation.Build("entity-a", retailsimulation.Input{})
	if err != nil {
		t.Fatal(err)
	}
	set := generatedFactSet(plan)
	reader := &fakeReader{set: set}
	service := NewService(reader)
	want := []struct {
		storeIndex                        int
		anomalyType, bridgeCode, itemCode string
	}{
		{1, "footfall_continuous_decline", "revenue", "footfall"},
		{2, "conversion_rate_drop", "revenue", "conversion_rate"},
		{3, "average_ticket_drop", "revenue", "average_transaction_value"},
		{4, "gross_margin_compression", "gross_profit", "gross_margin_rate"},
		{5, "labor_cost_spike", "store_contribution", "labor_cost"},
		{6, "occupancy_cost_burden", "store_contribution", "occupancy_cash_cost"},
	}
	for _, scenario := range want {
		var anomaly retailsimulation.Anomaly
		for _, candidate := range plan.Anomalies {
			if candidate.Type == scenario.anomalyType {
				anomaly = candidate
				break
			}
		}
		if anomaly.Type == "" {
			t.Fatalf("missing anomaly %s", scenario.anomalyType)
		}
		asOf, _ := time.Parse("2006-01-02", anomaly.DateTo)
		result, buildErr := service.Build(context.Background(), Query{LegalEntityID: "entity-a", StoreID: "store-" + string(rune('a'+scenario.storeIndex)), AsOf: asOf, WindowDays: 7, Classification: "simulated", DatasetVersion: plan.DatasetVersion})
		if buildErr != nil {
			t.Fatalf("store %d: %v", scenario.storeIndex+1, buildErr)
		}
		var found *float64
		for _, bridge := range result.Bridges {
			if bridge.Code == scenario.bridgeCode {
				for _, item := range bridge.Items {
					if item.Code == scenario.itemCode {
						found = item.Contribution
					}
				}
			}
		}
		if found == nil || *found >= 0 {
			t.Fatalf("store %d expected negative %s contribution, got %v", scenario.storeIndex+1, scenario.itemCode, found)
		}
		t.Logf("Store %03d %s bridge %s contribution=%.2f (expected direction: negative)", scenario.storeIndex+1, scenario.anomalyType, scenario.itemCode, *found)
	}
}

func generatedFactSet(plan *retailsimulation.Plan) *repository.RetailKPIFactSet {
	stores := make([]retailkpi.StorePopulation, 0, len(plan.Stores))
	for _, store := range plan.Stores {
		stores = append(stores, retailkpi.StorePopulation{StoreID: "store-" + string(rune('a'+store.Index)), StoreCode: store.Code, StoreName: store.Name, Brand: store.Brand, Region: store.Region})
	}
	dataset := plan.DatasetVersion
	facts := make([]retailkpi.DailyFact, 0, len(plan.Facts))
	for _, fact := range plan.Facts {
		store := plan.Stores[fact.StoreIndex]
		date, _ := time.Parse("2006-01-02", fact.BusinessDate)
		facts = append(facts, retailkpi.DailyFact{StoreID: "store-" + string(rune('a'+fact.StoreIndex)), StoreCode: store.Code, StoreName: store.Name, Brand: store.Brand, Region: store.Region, BusinessDate: date, AsOfAt: date.Add(24 * time.Hour), Currency: fact.Currency, SourceSystem: "retail_simulator", Version: 1, Revenue: float(fact.Revenue), GrossProfit: float(fact.GrossProfit), Transactions: float(fact.Transactions), Footfall: float(fact.Footfall), AreaSqm: float(fact.AreaSqm), LaborCost: float(fact.LaborCost), FixedRent: float(fact.FixedRent), VariableRent: float(fact.VariableRent), NonLeaseCost: float(fact.NonLeaseCost), OtherControllableCost: float(fact.OtherControllableCost), DataQualityStatus: "valid", MappingStatus: "mapped", DataClassification: "simulated", SimulationDatasetVersion: &dataset})
	}
	return &repository.RetailKPIFactSet{Facts: facts, ExpectedStoreCount: len(stores), ExpectedStores: stores, SourceSystems: []string{"retail_simulator"}, DatasetVersions: []string{dataset}, MinFactVersion: 1, MaxFactVersion: 1}
}
