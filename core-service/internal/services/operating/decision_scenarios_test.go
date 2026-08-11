package operating

import "testing"

func TestStoreScenariosRequireHumanConfirmedMaterialAssumptions(t *testing.T) {
	_, err := EvaluateStoreScenarios([]StoreDecisionScenario{{Name: "renew", HorizonMonths: 12, MonthlySales: 100, MonthlyRent: 10}, {Name: "close", Decision: "close", HorizonMonths: 12, MonthlySales: 100, MonthlyRent: 10}})
	if err == nil {
		t.Fatal("missing discount rate must block scenario calculation")
	}
}

func TestStoreScenariosReturnNegotiationMetrics(t *testing.T) {
	result, err := EvaluateStoreScenarios([]StoreDecisionScenario{
		{Name: "renew", Decision: "renew", Currency: "CNY", HorizonMonths: 12, DiscountRate: .12, MonthlySales: 100000, GrossMarginPct: 40, MonthlyLabor: 10000, MonthlyOtherCost: 5000, MonthlyRent: 12000, VariableRentPct: 2},
		{Name: "close", Decision: "close", Currency: "CNY", HorizonMonths: 12, DiscountRate: .12, MonthlySales: 100000, GrossMarginPct: 40, MonthlyLabor: 10000, MonthlyOtherCost: 5000, MonthlyRent: 12000, ExitCost: 20000},
	})
	if err != nil || len(result) != 2 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if result[0].TargetNegotiationHigh <= 0 || result[0].BreakEvenMonthlyRent <= 0 {
		t.Fatalf("result=%+v", result[0])
	}
}

func TestEquipmentScenariosRequireCapacityEvidence(t *testing.T) {
	_, err := EvaluateEquipmentScenarios([]EquipmentDecisionScenario{{Name: "buy", HorizonMonths: 12, DiscountRate: .1}, {Name: "lease", HorizonMonths: 12, DiscountRate: .1}})
	if err == nil {
		t.Fatal("missing capacity evidence must block recommendation")
	}
}
