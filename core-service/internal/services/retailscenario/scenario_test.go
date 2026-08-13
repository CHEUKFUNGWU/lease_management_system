package retailscenario

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

type fakeScenarioReader struct {
	set   *repository.RetailKPIFactSet
	calls int
}

func (r *fakeScenarioReader) QueryFacts(context.Context, string, string, string, string, string, string, []string) (*repository.RetailKPIFactSet, error) {
	r.calls++
	return r.set, nil
}

func scenarioAssumptions() Assumptions {
	return Assumptions{RevenueChangePct: 10, GrossMarginRateChangePP: 2, LaborCostChangePct: -5, FixedRentChangePct: -10}
}

func TestCalculatorManualGolden(t *testing.T) {
	base := baseValues{Revenue: 30000, GrossProfit: 9000, GrossMarginRate: 30, LaborCost: 3000, FixedRent: 2400, VariableRent: 300, VariableRentRate: 1, NonLeaseCost: 300, OtherCost: 600, Occupancy: 3000, Contribution: 2400, ContributionMargin: 8}
	result := calculateScenario(ScenarioInput{Key: "plan", Name: "Plan", Assumptions: scenarioAssumptions()}, base, 12)
	want := map[string]float64{"revenue": 33000, "gross_profit": 10560, "labor_cost": 2850, "fixed_rent": 2160, "variable_rent": 330, "non_lease_cost": 300, "other_controllable_cost": 600, "occupancy_cash_cost": 2790, "store_contribution": 4320}
	for code, expected := range want {
		metric := result.Metrics[code]
		if metric.Result == nil || *metric.Result != expected {
			t.Fatalf("%s=%v want %.2f", code, metric.Result, expected)
		}
	}
	if result.MonthlyContributionChange == nil || *result.MonthlyContributionChange != 1920 || result.HorizonContributionChange == nil || *result.HorizonContributionChange != 23040 {
		t.Fatalf("contribution changes=%+v", result)
	}
	if result.Metrics["gross_margin_rate"].Result == nil || *result.Metrics["gross_margin_rate"].Result != 32 {
		t.Fatalf("gross margin precision=%v", result.Metrics["gross_margin_rate"])
	}
	if result.Bridge.RoundingResidual == nil || math.Abs(*result.Bridge.RoundingResidual) > .01 {
		t.Fatalf("bridge residual=%+v", result.Bridge)
	}
	wantBridge := map[string]float64{"gross_profit": 1560, "labor_cost": 150, "fixed_rent": 240, "variable_rent": -30, "non_lease_cost": 0, "other_controllable_cost": 0}
	for _, item := range result.Bridge.Items {
		if item.Contribution == nil || math.Abs(*item.Contribution-wantBridge[item.Code]) > .01 {
			t.Fatalf("bridge %s=%v want %.2f", item.Code, item.Contribution, wantBridge[item.Code])
		}
	}
}

func TestCalculatorIdentityAndBoundaryValidation(t *testing.T) {
	base := baseValues{Revenue: 30000, GrossProfit: 9000, GrossMarginRate: 30, LaborCost: 3000, FixedRent: 2400, VariableRent: 300, VariableRentRate: 1, NonLeaseCost: 300, OtherCost: 600, Occupancy: 3000, Contribution: 2400, ContributionMargin: 8}
	result := calculateScenario(ScenarioInput{Key: "baseline", Name: "Baseline"}, base, 12)
	if result.MonthlyContributionChange == nil || *result.MonthlyContributionChange != 0 {
		t.Fatalf("identity change=%v", result.MonthlyContributionChange)
	}
	if err := validateAssumptions(Assumptions{RevenueChangePct: -100}); err != nil {
		t.Fatalf("-100 should be accepted: %v", err)
	}
	if err := validateAssumptions(Assumptions{RevenueChangePct: 300}); err != nil {
		t.Fatalf("300 should be accepted: %v", err)
	}
	if err := validateAssumptions(Assumptions{RevenueChangePct: math.NaN()}); err == nil {
		t.Fatal("NaN must be rejected")
	}
	zero := calculateScenario(ScenarioInput{Key: "zero", Name: "Zero", Assumptions: Assumptions{RevenueChangePct: -100}}, base, 12)
	if zero.Metrics["store_contribution_margin"].Result != nil || zero.Metrics["store_contribution_margin"].Reason != "zero_denominator" {
		t.Fatalf("zero revenue margin=%+v", zero.Metrics["store_contribution_margin"])
	}
	if err := validateAssumptions(Assumptions{GrossMarginRateChangePP: 100}); err != nil {
		t.Fatal(err)
	}
	if err := validateResultingRates(Assumptions{GrossMarginRateChangePP: 71}, base); err == nil {
		t.Fatal("resulting rate boundary must be rejected")
	}
}

func TestEvaluateUsesOneQueryAndRejectsPartial(t *testing.T) {
	date := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	value := func(v float64) *float64 { return &v }
	facts := make([]retailkpi.DailyFact, 0, 7)
	for i := 0; i < 7; i++ {
		d := date.AddDate(0, 0, i)
		facts = append(facts, retailkpi.DailyFact{StoreID: "store-a", StoreCode: "S001", StoreName: "A", Brand: "B", Region: "R", BusinessDate: d, Currency: "CNY", SourceSystem: "retail_simulator", Version: 1, Revenue: value(1000), GrossProfit: value(300), Transactions: value(100), Footfall: value(500), AreaSqm: value(100), LaborCost: value(100), FixedRent: value(80), VariableRent: value(10), NonLeaseCost: value(10), OtherControllableCost: value(20), DataQualityStatus: "valid", MappingStatus: "mapped"})
	}
	reader := &fakeScenarioReader{set: &repository.RetailKPIFactSet{Facts: facts, ExpectedStores: []retailkpi.StorePopulation{{StoreID: "store-a", StoreCode: "S001", StoreName: "A", Brand: "B", Region: "R"}}}}
	service := NewService(reader)
	request := EvaluateRequest{HorizonMonths: 12, Scenarios: []ScenarioInput{{Key: "baseline", Name: "Baseline"}, {Key: "plan", Name: "Plan", Assumptions: Assumptions{LaborCostChangePct: -10}}}}
	response, err := service.Evaluate(context.Background(), Query{LegalEntityID: "entity-a", StoreID: "store-a", AsOf: date.AddDate(0, 0, 6), WindowDays: 7, Classification: "production"}, request)
	if err != nil || response == nil || reader.calls != 1 || response.Basis != "Scenario" || response.SideEffects || response.IFRS16Impact {
		t.Fatalf("response=%+v err=%v calls=%d", response, err, reader.calls)
	}
	facts[0].Revenue = nil
	if _, err := NewService(&fakeScenarioReader{set: reader.set}).Evaluate(context.Background(), Query{LegalEntityID: "entity-a", StoreID: "store-a", AsOf: date.AddDate(0, 0, 6), WindowDays: 7, Classification: "production"}, request); err == nil {
		t.Fatal("partial facts must be rejected")
	}
}

func TestStoreScenarioDirections(t *testing.T) {
	base := baseValues{Revenue: 30000, GrossProfit: 9000, GrossMarginRate: 30, LaborCost: 3000, FixedRent: 2400, VariableRent: 300, VariableRentRate: 1, NonLeaseCost: 300, OtherCost: 600, Occupancy: 3000, Contribution: 2400, ContributionMargin: 8}
	cases := []struct {
		name   string
		a      Assumptions
		bridge string
	}{{"Store005 gross margin", Assumptions{GrossMarginRateChangePP: 2}, "gross_profit"}, {"Store006 labor", Assumptions{LaborCostChangePct: -10}, "labor_cost"}, {"Store007 rent", Assumptions{FixedRentChangePct: -10}, "fixed_rent"}}
	for _, test := range cases {
		result := calculateScenario(ScenarioInput{Key: "plan", Name: test.name, Assumptions: test.a}, base, 12)
		if result.Bridge.Items == nil {
			t.Fatal(test.name)
		}
		var contribution float64
		for _, item := range result.Bridge.Items {
			if item.Code == test.bridge && item.Contribution != nil {
				contribution = *item.Contribution
			}
		}
		if contribution <= 0 {
			t.Fatalf("%s bridge contribution=%.2f", test.name, contribution)
		}
	}
}

func TestDecisionReadyAndUnavailableEvidenceReasons(t *testing.T) {
	date := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	value := func(v float64) *float64 { return &v }
	makeFacts := func() []retailkpi.DailyFact {
		facts := make([]retailkpi.DailyFact, 0, 7)
		for i := 0; i < 7; i++ {
			facts = append(facts, retailkpi.DailyFact{StoreID: "store-a", StoreCode: "S001", StoreName: "A", Brand: "B", Region: "R", BusinessDate: date.AddDate(0, 0, i), Currency: "CNY", SourceSystem: "source-a", Version: 1, Revenue: value(1000), GrossProfit: value(300), Transactions: value(100), Footfall: value(500), AreaSqm: value(100), LaborCost: value(100), FixedRent: value(80), VariableRent: value(10), NonLeaseCost: value(10), OtherControllableCost: value(20), DataQualityStatus: "valid", MappingStatus: "mapped"})
		}
		return facts
	}
	request := EvaluateRequest{HorizonMonths: 3, Scenarios: []ScenarioInput{{Key: "baseline", Name: "Baseline"}, {Key: "plan", Name: "Plan"}}}
	query := Query{LegalEntityID: "entity-a", StoreID: "store-a", AsOf: date.AddDate(0, 0, 6), WindowDays: 7, Classification: "production", SourceSystem: "source-a"}
	population := []retailkpi.StorePopulation{{StoreID: "store-a", StoreCode: "S001", StoreName: "A", Brand: "B", Region: "R"}}
	for _, test := range []struct {
		name   string
		mutate func([]retailkpi.DailyFact) []retailkpi.DailyFact
		reason string
	}{
		{name: "partial coverage", mutate: func(facts []retailkpi.DailyFact) []retailkpi.DailyFact { return facts[:6] }, reason: "incomplete_store_day_coverage"},
		{name: "invalid fact", mutate: func(facts []retailkpi.DailyFact) []retailkpi.DailyFact {
			facts[0].DataQualityStatus = "invalid"
			return facts
		}, reason: "data_quality_invalid"},
		{name: "mapping failure", mutate: func(facts []retailkpi.DailyFact) []retailkpi.DailyFact {
			facts[0].MappingStatus = "unmapped"
			return facts
		}, reason: "mapping_unmapped"},
		{name: "missing core", mutate: func(facts []retailkpi.DailyFact) []retailkpi.DailyFact { facts[0].GrossProfit = nil; return facts }, reason: "missing_required_field"},
		{name: "missing auxiliary", mutate: func(facts []retailkpi.DailyFact) []retailkpi.DailyFact { facts[0].Footfall = nil; return facts }, reason: "decision_not_ready"},
		{name: "zero denominator", mutate: func(facts []retailkpi.DailyFact) []retailkpi.DailyFact {
			for _, fact := range facts {
				*fact.Revenue = 0
			}
			return facts
		}, reason: "zero_denominator"},
		{name: "currency conflict", mutate: func(facts []retailkpi.DailyFact) []retailkpi.DailyFact { facts[0].Currency = "USD"; return facts }, reason: "currency_conflict"},
		{name: "no facts", mutate: func([]retailkpi.DailyFact) []retailkpi.DailyFact { return nil }, reason: "no_facts"},
	} {
		t.Run(test.name, func(t *testing.T) {
			reader := &fakeScenarioReader{set: &repository.RetailKPIFactSet{Facts: test.mutate(makeFacts()), ExpectedStores: population}}
			_, err := NewService(reader).Evaluate(context.Background(), query, request)
			var evidenceErr *ScenarioEvidenceError
			if !errors.As(err, &evidenceErr) || evidenceErr.Reason != test.reason || evidenceErr.Evidence.ExpectedStoreDays != 7 || evidenceErr.Evidence.KPIDrilldownURL == "" {
				t.Fatalf("error=%v evidence=%+v", err, evidenceErr)
			}
		})
	}
}

func TestRequestFingerprintIncludesFactScope(t *testing.T) {
	request := EvaluateRequest{HorizonMonths: 12, Scenarios: []ScenarioInput{{Key: "baseline", Name: "Baseline"}, {Key: "plan", Name: "Plan"}}}
	left := Query{LegalEntityID: "entity-a", StoreID: "store-a", AsOf: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC), WindowDays: 28, Classification: "simulated", DatasetVersion: "plan-a", SourceSystem: "retail_simulator"}
	right := left
	right.StoreID = "store-b"
	if RequestFingerprint(left, request, "title", "action", "", "", "2026-08") == RequestFingerprint(right, request, "title", "action", "", "", "2026-08") {
		t.Fatal("idempotency fingerprint must include store/data scope")
	}
}
