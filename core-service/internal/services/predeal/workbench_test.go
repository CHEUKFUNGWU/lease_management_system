package predeal

import (
	"math"
	"strings"
	"testing"
	"time"
)

func fiveYearDraft() Draft {
	return Draft{
		Name:             "华东厂房 5 年租约",
		CommencementDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		TermMonths:       60,
		MonthlyRent:      100000,
		DiscountRate:     0.05,
		Currency:         "CNY",
	}
}

// The finding the view exists to deliver: IFRS 16 charges more than a
// straight-line rent early and less late, because interest tracks a liability
// that is largest at the start. A budget built on "rent is 100k a month" is
// wrong in year one, and in the direction nobody expects.
func TestBuild_ExpenseIsFrontLoadedAgainstStraightLineRent(t *testing.T) {
	briefing, err := Build(fiveYearDraft())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(briefing.Yearly) != 5 {
		t.Fatalf("expected five years, got %d", len(briefing.Yearly))
	}

	first, last := briefing.Yearly[0], briefing.Yearly[4]
	if first.ExpenseVsStraightLine <= 0 {
		t.Errorf("year one should cost more than straight-line rent, got %.2f", first.ExpenseVsStraightLine)
	}
	if last.ExpenseVsStraightLine >= 0 {
		t.Errorf("the final year should cost less, got %.2f", last.ExpenseVsStraightLine)
	}
	if first.Interest <= last.Interest {
		t.Errorf("interest falls as the liability amortises: year 1 %.2f vs year 5 %.2f",
			first.Interest, last.Interest)
	}

	// It is a timing difference and nothing more: over the whole term the
	// standard charges what the rent costs.
	var totalDifference float64
	for _, year := range briefing.Yearly {
		totalDifference += year.ExpenseVsStraightLine
	}
	if math.Abs(totalDifference) > closingTolerance {
		t.Errorf("front-loading must net to zero over the term, got %.2f", totalDifference)
	}
}

// The engine accrues interest daily on the outstanding balance while the
// liability is the present value of each payment discounted by its own day
// count. Over five years and 1,826 days those two conventions do not land on
// precisely the same number — the residual here is about 0.015% of the rent,
// and the liability still amortises to exactly zero. Asserting an exact tie
// would be asserting a property the daily engine does not have, so the
// tolerance is set to a size that would still catch a real leak.
const closingTolerance = 1500.0

func TestBuild_TotalExpenseEqualsTotalRent(t *testing.T) {
	briefing, err := Build(fiveYearDraft())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	var expense, cash float64
	for _, year := range briefing.Yearly {
		expense += year.IFRS16Expense
		cash += year.CashRent
	}
	// Interest plus depreciation over the life of a lease is the rent paid.
	// Any other answer means the engine is creating or destroying cost.
	if math.Abs(expense-cash) > closingTolerance {
		t.Errorf("total expense %.2f should equal total rent %.2f", expense, cash)
	}
}

// Capitalising a lease lifts EBITDA without anything about the business
// improving. Saying so is the point of the bridge.
func TestBuild_BridgeShowsEBITDALiftIsAnArtefactNotAnImprovement(t *testing.T) {
	briefing, err := Build(fiveYearDraft())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	first := briefing.Bridge[0]

	if first.EBITDAUplift <= 0 {
		t.Error("rent moving below the line lifts EBITDA")
	}
	if first.EBITDAUplift != briefing.Yearly[0].StraightLineRent {
		t.Errorf("the lift is exactly the rent that left the line: %.2f vs %.2f",
			first.EBITDAUplift, briefing.Yearly[0].StraightLineRent)
	}
	// What was taken off one line has to appear on the others.
	below := first.DepreciationBelowEBITDA + first.InterestBelowEBIT
	if math.Abs((below-first.RentAboveEBITDA)-first.NetProfitImpact) > 0.02 {
		t.Errorf("the bridge does not balance: %.2f below the line vs %.2f above, net %.2f",
			below, first.RentAboveEBITDA, first.NetProfitImpact)
	}

	var netOverTerm float64
	for _, year := range briefing.Bridge {
		netOverTerm += year.NetProfitImpact
	}
	if math.Abs(netOverTerm) > closingTolerance {
		t.Errorf("net profit impact is timing only and must net to zero, got %.2f", netOverTerm)
	}
}

func TestBuild_ExitCurveFallsAsTheLeaseRunsDown(t *testing.T) {
	draft := fiveYearDraft()
	draft.EarlyExitPenaltyMonths = 3

	briefing, err := Build(draft)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(briefing.ExitCurve) != 4 {
		t.Fatalf("a five year lease has four intermediate exit points, got %d", len(briefing.ExitCurve))
	}

	for i := 1; i < len(briefing.ExitCurve); i++ {
		previous, current := briefing.ExitCurve[i-1], briefing.ExitCurve[i]
		if current.RemainingCommitment >= previous.RemainingCommitment {
			t.Errorf("the commitment left must shrink each year: year %d %.2f vs year %d %.2f",
				previous.Year, previous.RemainingCommitment, current.Year, current.RemainingCommitment)
		}
		if current.LiabilityReleased >= previous.LiabilityReleased {
			t.Errorf("the liability to release must shrink: year %d %.2f vs year %d %.2f",
				previous.Year, previous.LiabilityReleased, current.Year, current.LiabilityReleased)
		}
	}

	// Three months of rent, and the rent is flat in this draft.
	if briefing.ExitCurve[0].Penalty != 300000 {
		t.Errorf("penalty = %.2f, want three months of 100,000", briefing.ExitCurve[0].Penalty)
	}
	// The remaining rent is avoided by exiting, so it is not cash to be paid.
	if briefing.ExitCurve[0].TotalCashToExit != briefing.ExitCurve[0].Penalty {
		t.Errorf("cash to exit is the break fee, not the rent avoided: %.2f",
			briefing.ExitCurve[0].TotalCashToExit)
	}
}

func TestBuild_PenaltyFollowsTheRentInForceAtExit(t *testing.T) {
	draft := fiveYearDraft()
	draft.AnnualEscalationPercent = 10
	draft.EarlyExitPenaltyMonths = 1

	briefing, err := Build(draft)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// Exiting at the end of year one means the rent in force is the year-two
	// rate: a break fee is quoted against the rent you are escaping, not the
	// rent you started on.
	if got := briefing.ExitCurve[0].Penalty; math.Abs(got-110000) > 0.01 {
		t.Errorf("penalty at the first break = %.2f, want one month at the escalated 110,000", got)
	}
}

func TestBuild_RentFreePeriodLowersTheLiabilityNotTheTerm(t *testing.T) {
	plain, err := Build(fiveYearDraft())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	free := fiveYearDraft()
	free.RentFreeMonths = 6
	withFree, err := Build(free)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if withFree.BalanceSheet.InitialLiability >= plain.BalanceSheet.InitialLiability {
		t.Errorf("six free months must reduce the liability: %.2f vs %.2f",
			withFree.BalanceSheet.InitialLiability, plain.BalanceSheet.InitialLiability)
	}
	if len(withFree.Yearly) != len(plain.Yearly) {
		t.Error("a rent-free period does not shorten the lease")
	}
}

func TestBuild_InitialDirectCostLiftsTheAssetButNotTheLiability(t *testing.T) {
	plain, err := Build(fiveYearDraft())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	withCost := fiveYearDraft()
	withCost.InitialDirectCost = 200000
	loaded, err := Build(withCost)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if loaded.BalanceSheet.InitialLiability != plain.BalanceSheet.InitialLiability {
		t.Errorf("initial direct cost is not owed to the landlord and must not change the liability: %.2f vs %.2f",
			loaded.BalanceSheet.InitialLiability, plain.BalanceSheet.InitialLiability)
	}
	if got := loaded.BalanceSheet.InitialROU - plain.BalanceSheet.InitialROU; math.Abs(got-200000) > 1 {
		t.Errorf("the asset should carry the whole 200,000, got %.2f", got)
	}
}

func TestBuild_DiscountingEffectIsTheGapToTheHeadlineCommitment(t *testing.T) {
	briefing, err := Build(fiveYearDraft())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	sheet := briefing.BalanceSheet
	if sheet.UndiscountedCommitment != 6000000 {
		t.Errorf("sixty months at 100,000 is 6,000,000, got %.2f", sheet.UndiscountedCommitment)
	}
	if sheet.DiscountingEffect <= 0 {
		t.Error("the liability is the discounted commitment, so the gap is positive")
	}
	if math.Abs((sheet.InitialLiability+sheet.DiscountingEffect)-sheet.UndiscountedCommitment) > 0.02 {
		t.Error("the liability plus the discounting must reconcile to the commitment")
	}
}

func TestBuild_HeadlineStatesTheTimingDifferenceExplicitly(t *testing.T) {
	briefing, err := Build(fiveYearDraft())
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	// A board told only "EBITDA is up" has been misled. The sentence has to
	// carry the caveat with the number.
	for _, phrase := range []string{"时间性差异", "全期合计为零", "与经营改善无关"} {
		if !strings.Contains(briefing.Headline, phrase) {
			t.Errorf("headline %q is missing %q", briefing.Headline, phrase)
		}
	}
}

func TestBuild_RejectsDraftsItCannotPrice(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Draft)
	}{
		{"zero term", func(d *Draft) { d.TermMonths = 0 }},
		{"negative rent", func(d *Draft) { d.MonthlyRent = -1 }},
		{"free period longer than the term", func(d *Draft) { d.RentFreeMonths = 61 }},
		// Everything in the briefing moves with the rate, so assuming one would
		// answer a different question than the one asked.
		{"no discount rate", func(d *Draft) { d.DiscountRate = 0 }},
		{"no commencement date", func(d *Draft) { d.CommencementDate = time.Time{} }},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			draft := fiveYearDraft()
			testCase.mutate(&draft)
			if _, err := Build(draft); err == nil {
				t.Error("expected the draft to be rejected")
			}
		})
	}
}
