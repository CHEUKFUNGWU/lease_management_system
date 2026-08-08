package renewaldecision

import (
	"testing"
	"time"
)

func TestEvaluateReturnsRenewRenegotiateAndTerminate(t *testing.T) {
	result, err := Evaluate(Input{
		DecisionDate:        time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		Currency:            "CNY",
		DiscountRate:        0.05,
		CurrentMonthlyRent:  100000,
		RemainingCommitment: 2400000,
		CurrentLiability:    1800000,
		CurrentROU:          1700000,
		RemainingTermMonths: 36,
		Scenarios: []Scenario{
			{Name: "renew", Decision: "renew", TermMonths: 60, MonthlyRent: 100000},
			{Name: "renegotiate", Decision: "renegotiate", TermMonths: 60, MonthlyRent: 105000, RentFreeMonths: 3, AnnualEscalationPercent: 3},
			{Name: "terminate", Decision: "terminate", EarlyExitPenaltyMonths: 3},
		},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(result.Scenarios) != 3 {
		t.Fatalf("scenarios = %d, want 3", len(result.Scenarios))
	}
	if len(result.Scenarios[0].Yearly) != 5 {
		t.Fatalf("renewal yearly curve = %d, want 5 years", len(result.Scenarios[0].Yearly))
	}
	if result.Scenarios[1].Yearly[0].CashOutflow >= result.Scenarios[1].TotalCashOutflow {
		t.Error("the yearly curve must be a slice of total cash, not the total repeated")
	}
	exit := result.Scenarios[2]
	if exit.Exit == nil || exit.Exit.TotalCashToExit != 300000 {
		t.Fatalf("termination exit = %#v, want a 300,000 penalty", exit.Exit)
	}
	if len(exit.ExitCurve) != 3 {
		t.Fatalf("termination exit curve = %d points, want 3", len(exit.ExitCurve))
	}
}

func TestEvaluateRejectsMissingRate(t *testing.T) {
	_, err := Evaluate(Input{
		DecisionDate: time.Now(),
		Scenarios: []Scenario{
			{Name: "renew", Decision: "renew", TermMonths: 12, MonthlyRent: 100},
			{Name: "terminate", Decision: "terminate"},
		},
	})
	if err == nil {
		t.Fatal("missing discount rate must be rejected")
	}
}
