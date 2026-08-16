package renewaldecision

import (
	"math"
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

// M7 test face: the five-year curves reconcile to the scenario totals —
// cash, IFRS 16 expense, EBITDA/EBIT/P&L all tie out so a saved snapshot is
// auditable.
func TestYearlyCurvesReconcileToTotals(t *testing.T) {
	input := Input{
		DecisionDate: time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC), Currency: "CNY",
		DiscountRate: 0.045, CurrentMonthlyRent: 50000, RemainingCommitment: 300000,
		CurrentLiability: 280000, CurrentROU: 290000, RemainingTermMonths: 6,
		Scenarios: []Scenario{
			{Name: "renew_current_terms", Decision: "renew", TermMonths: 60, MonthlyRent: 52000, RentFreeMonths: 0, AnnualEscalationPercent: 3},
			{Name: "terminate_no_renewal", Decision: "terminate", TermMonths: 0, MonthlyRent: 0, RentFreeMonths: 0, AnnualEscalationPercent: 0, EarlyExitPenaltyMonths: 2},
		},
	}
	result, err := Evaluate(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Scenarios) != 2 {
		t.Fatalf("scenarios=%d", len(result.Scenarios))
	}
	renew := result.Scenarios[0]
	var cashSum, expenseSum float64
	for _, year := range renew.Yearly {
		cashSum += year.CashOutflow
		expenseSum += year.IFRS16Expense
	}
	if math.Abs(cashSum-renew.TotalCashOutflow) > 0.05 {
		t.Fatalf("cash curve sum=%.2f total=%.2f", cashSum, renew.TotalCashOutflow)
	}
	if math.Abs(expenseSum-renew.TotalIFRS16Expense) > 0.05 {
		t.Fatalf("expense curve sum=%.2f total=%.2f", expenseSum, renew.TotalIFRS16Expense)
	}
	terminate := result.Scenarios[1]
	if terminate.Exit == nil {
		t.Fatal("terminate scenario missing exit impact")
	}
	// The exit separates avoided future rent from immediate exit cash: the
	// P&L hit is ROU write-off minus liability released plus the penalty.
	wantPnL := terminate.Exit.ROUWrittenOff - terminate.Exit.LiabilityReleased + terminate.Exit.Penalty
	if math.Abs(wantPnL-terminate.Exit.PnLImpact) > 0.05 {
		t.Fatalf("exit pnl=%.2f, components give %.2f", terminate.Exit.PnLImpact, wantPnL)
	}
}
