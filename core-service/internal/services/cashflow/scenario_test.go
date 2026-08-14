package cashflow

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/money"
	"github.com/lease-management-system/core-service/internal/services/ifrs16"
)

func day(value string) time.Time {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		panic(err)
	}
	return parsed
}

// leaseEndingIn builds a lease paying `rent` monthly until it expires.
func leaseEndingIn(id string, start time.Time, months int, rent float64) Lease {
	payments := make([]ifrs16.LeasePayment, 0, months)
	firstOfMonth := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location())
	for month := 0; month < months; month++ {
		monthEnd := firstOfMonth.AddDate(0, month+1, 0).AddDate(0, 0, -1)
		payments = append(payments, ifrs16.LeasePayment{
			Date: monthEnd, Amount: money.NewFromFloat(rent), Timing: "postpaid", Type: "fixed",
		})
	}
	return Lease{
		ContractID: id, ContractNumber: id, ContractName: id + " 租约",
		Currency: "CNY", LeaseEndDate: payments[len(payments)-1].Date, Payments: payments,
	}
}

func baseInput() Input {
	return Input{
		AsOf: day("2026-01-01"), Currency: "CNY", HorizonMonths: 60,
		Leases: []Lease{leaseEndingIn("A", day("2026-01-01"), 24, 10000)},
	}
}

// With no estates plan the projection is just the signed commitment. That is
// the baseline every scenario is read against.
func TestProject_NoScenarioIsJustTheCommitment(t *testing.T) {
	result, err := Project(baseInput())
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if result.Committed != 240000 {
		t.Errorf("24 months at 10,000 is 240,000, got %.2f", result.Committed)
	}
	if result.Renewal != 0 || result.Closure != 0 {
		t.Errorf("no plan means no assumed cash: renewal %.2f closure %.2f", result.Renewal, result.Closure)
	}
	if result.Total != result.Committed {
		t.Errorf("total %.2f should equal committed %.2f", result.Total, result.Committed)
	}
}

func TestProject_LadderTiesToTheTotal(t *testing.T) {
	input := baseInput()
	input.Scenario = Scenario{
		RenewalRate: 0.5, RenewalTermMonths: 12, RenewalUpliftPercent: 10,
		ClosureRate: 0.2, ClosureCostMonths: 3,
	}
	result, err := Project(input)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	// Every projected outflow lands in exactly one band, so the ladder is the
	// same money as the total viewed a different way.
	if math.Abs(result.Ladder.Total-result.Total) > 0.05 {
		t.Errorf("ladder %.2f does not tie to total %.2f", result.Ladder.Total, result.Total)
	}
	if result.Ladder.Labels[0] != "1 年内" || result.Ladder.Labels[5] != "5 年以上" {
		t.Errorf("the ladder must use the shared disclosure bands, got %v", result.Ladder.Labels)
	}
}

// The rates are weights over the expiring population, not over the portfolio.
func TestProject_RatesApplyOnlyToLeasesThatExpireInTheHorizon(t *testing.T) {
	input := baseInput()
	// A second lease running well past the horizon: it is not up for decision.
	input.Leases = append(input.Leases, leaseEndingIn("LONG", day("2026-01-01"), 120, 10000))
	input.Scenario = Scenario{RenewalRate: 1, RenewalTermMonths: 12}

	result, err := Project(input)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if result.ExpiringLeases != 1 {
		t.Errorf("only the 24-month lease expires inside the horizon, got %d", result.ExpiringLeases)
	}
	// Twelve months of renewal on one lease at its last rent.
	if math.Abs(result.Renewal-120000) > 0.01 {
		t.Errorf("renewal cash = %.2f, want twelve months of 10,000", result.Renewal)
	}
}

func TestProject_RenewalUpliftRaisesTheAssumedRent(t *testing.T) {
	input := baseInput()
	input.Scenario = Scenario{RenewalRate: 1, RenewalTermMonths: 12, RenewalUpliftPercent: 20}

	result, err := Project(input)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if math.Abs(result.Renewal-144000) > 0.01 {
		t.Errorf("renewal at +20%% = %.2f, want twelve months of 12,000", result.Renewal)
	}
}

func TestProject_ClosureCostLandsWhenTheLeaseEnds(t *testing.T) {
	input := baseInput()
	input.Scenario = Scenario{ClosureRate: 0.5, ClosureCostMonths: 3}

	result, err := Project(input)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	// Half of one lease closing, at three months of 10,000.
	if math.Abs(result.Closure-15000) > 0.01 {
		t.Errorf("closure cost = %.2f, want 15,000", result.Closure)
	}
	// The lease ends 2027-12-31, so the cost belongs in the second year band.
	if result.Ladder.Values[1] < 15000 {
		t.Errorf("the closure cost should fall in the 1-2 year band, ladder = %v", result.Ladder.Values)
	}
}

// Renewals and closures come out of the same population, so together they
// cannot exceed it. Accepting 80% renewing and 80% closing would silently
// project 160% of the estate.
func TestProject_RejectsSharesThatExceedThePopulation(t *testing.T) {
	input := baseInput()
	input.Scenario = Scenario{RenewalRate: 0.8, ClosureRate: 0.8, RenewalTermMonths: 12}
	if _, err := Project(input); err == nil {
		t.Error("renewal plus closure above 100% must be rejected")
	}
}

func TestProject_RejectsInputItCannotProject(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Input)
	}{
		{"no as-of date", func(i *Input) { i.AsOf = time.Time{} }},
		{"negative renewal rate", func(i *Input) { i.Scenario.RenewalRate = -0.1 }},
		{"renewal rate above one", func(i *Input) { i.Scenario.RenewalRate = 1.5 }},
		{"closure rate above one", func(i *Input) { i.Scenario.ClosureRate = 1.5 }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			input := baseInput()
			testCase.mutate(&input)
			if _, err := Project(input); err == nil {
				t.Error("expected the input to be rejected")
			}
		})
	}
}

func TestProject_MonthlySeriesIsChronologicalAndAddsUp(t *testing.T) {
	input := baseInput()
	input.Scenario = Scenario{RenewalRate: 0.5, RenewalTermMonths: 12}

	result, err := Project(input)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	var sum float64
	for i, month := range result.Monthly {
		if i > 0 && month.Period <= result.Monthly[i-1].Period {
			t.Fatalf("months out of order at %d: %s after %s", i, month.Period, result.Monthly[i-1].Period)
		}
		if math.Abs(month.Total-(month.Committed+month.Renewal+month.ClosureCost)) > 0.01 {
			t.Errorf("month %s does not add up: %+v", month.Period, month)
		}
		sum += month.Total
	}
	if math.Abs(sum-result.Total) > 0.05 {
		t.Errorf("monthly series sums to %.2f, total says %.2f", sum, result.Total)
	}
}

// A projection that mixes signed commitments with assumptions has to say which
// is which, or it will be read as a forecast of facts.
func TestProject_CaveatSeparatesCommitmentFromAssumption(t *testing.T) {
	input := baseInput()
	input.Scenario = Scenario{RenewalRate: 0.3, RenewalTermMonths: 12, ClosureRate: 0.1, ClosureCostMonths: 2}

	result, err := Project(input)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	for _, phrase := range []string{"1 份租约到期", "30% 续租", "10% 关店", "不是已签约承诺"} {
		if !strings.Contains(result.Caveat, phrase) {
			t.Errorf("caveat %q is missing %q", result.Caveat, phrase)
		}
	}
}

func TestProject_VariableRentIsNotProjectedAsCash(t *testing.T) {
	lease := leaseEndingIn("V", day("2026-01-01"), 12, 10000)
	lease.Payments = append(lease.Payments, ifrs16.LeasePayment{
		Date: day("2026-06-30"), Amount: money.NewFromInt64(50000), Timing: "postpaid", Type: "variable",
	})
	input := baseInput()
	input.Leases = []Lease{lease}

	result, err := Project(input)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	// Turnover rent depends on trading nobody has forecast here; projecting it
	// as certain cash would overstate the outflow.
	if result.Committed != 120000 {
		t.Errorf("committed = %.2f, want only the fixed rent 120,000", result.Committed)
	}
}
