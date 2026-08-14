package ifrs16

import (
	"math"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/money"
)

func day(value string) time.Time {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		panic(err)
	}
	return parsed
}

// monthlySchedule builds level rent falling on each month end, starting with
// the month of start. Month ends are computed from the first of the following
// month so that adding months never drifts the way "31 January + 1 month" does.
func monthlySchedule(start time.Time, months int, amount float64) []LeasePayment {
	firstOfMonth := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location())
	payments := make([]LeasePayment, 0, months)
	for i := 0; i < months; i++ {
		monthEnd := firstOfMonth.AddDate(0, i+1, 0).AddDate(0, 0, -1)
		payments = append(payments, LeasePayment{
			Date:   monthEnd,
			Amount: money.NewFromFloat(amount),
			Timing: "postpaid",
			Type:   "fixed",
		})
	}
	return payments
}

func amountsOn(t *testing.T, schedule RevisedSchedule, dates ...string) []float64 {
	t.Helper()
	byDate := map[string]float64{}
	for _, change := range schedule.Changes {
		byDate[change.Date.Format("2006-01-02")] = change.RevisedAmount.Round("CNY").Float64()
	}
	result := make([]float64, 0, len(dates))
	for _, date := range dates {
		value, present := byDate[date]
		if !present {
			t.Fatalf("no payment on %s in the derived schedule", date)
		}
		result = append(result, value)
	}
	return result
}

func TestDerive_FixedEscalationLeavesEarlierPaymentsAlone(t *testing.T) {
	original := monthlySchedule(day("2026-01-31"), 12, 10000)

	schedule, err := DeriveRevisedPayments(original, PaymentRevision{
		Kind:       RevisionPercentage,
		Percentage: 5,
	}, day("2026-07-01"))
	if err != nil {
		t.Fatalf("derive: %v", err)
	}

	got := amountsOn(t, schedule, "2026-06-30", "2026-07-31", "2026-12-31")
	if got[0] != 10000 {
		t.Errorf("a payment before the effective date was changed: %.2f", got[0])
	}
	if got[1] != 10500 || got[2] != 10500 {
		t.Errorf("payments from the effective date should be 10500, got %.2f and %.2f", got[1], got[2])
	}
	// Six payments fall on or after 1 July.
	if schedule.ChangedCount != 6 {
		t.Errorf("expected 6 changed payments, got %d", schedule.ChangedCount)
	}
	if schedule.Delta.Round("CNY").Float64() != 3000 {
		t.Errorf("expected the revision to add 3000 over the remaining term, got %.2f", schedule.Delta.Round("CNY").Float64())
	}
}

func TestDerive_IndexClauseUsesTheRatioOfReadings(t *testing.T) {
	original := monthlySchedule(day("2026-01-31"), 12, 10000)

	// CPI from 102.4 to 105.1 is a 2.637% movement.
	schedule, err := DeriveRevisedPayments(original, PaymentRevision{
		Kind:      RevisionIndex,
		BaseIndex: 102.4,
		NewIndex:  105.1,
	}, day("2026-01-01"))
	if err != nil {
		t.Fatalf("derive: %v", err)
	}

	want := 10000 * (105.1 / 102.4)
	got := amountsOn(t, schedule, "2026-01-31")[0]
	if math.Abs(got-moneyRound(money.NewFromFloat(want))) > 0.01 {
		t.Errorf("indexed payment: want %.2f, got %.2f", moneyRound(money.NewFromFloat(want)), got)
	}
	if schedule.CapApplied || schedule.FloorApplied {
		t.Error("no cap or floor was stated, so neither should be reported as applied")
	}
}

func TestDerive_CapBoundsAnIndexMovement(t *testing.T) {
	original := monthlySchedule(day("2026-01-31"), 12, 10000)

	// The clause gives 2.637% but the lease caps the increase at 2%.
	schedule, err := DeriveRevisedPayments(original, PaymentRevision{
		Kind:          RevisionIndex,
		BaseIndex:     102.4,
		NewIndex:      105.1,
		CapPercentage: 2,
	}, day("2026-01-01"))
	if err != nil {
		t.Fatalf("derive: %v", err)
	}

	if got := amountsOn(t, schedule, "2026-01-31")[0]; got != 10200 {
		t.Errorf("with a 2%% cap the payment should be 10200, got %.2f", got)
	}
	if !schedule.CapApplied {
		t.Error("the cap changed the outcome, so it must be reported as applied")
	}
	if math.Abs(schedule.AppliedFactor-1.02) > 1e-9 {
		t.Errorf("applied factor should be the capped 1.02, got %v", schedule.AppliedFactor)
	}
}

func TestDerive_FloorHoldsUpAFallingIndex(t *testing.T) {
	original := monthlySchedule(day("2026-01-31"), 12, 10000)

	// Deflation: the index falls, but the lease floors the movement at 0%.
	schedule, err := DeriveRevisedPayments(original, PaymentRevision{
		Kind:            RevisionIndex,
		BaseIndex:       105,
		NewIndex:        103,
		FloorPercentage: -0.0001, // a floor of "no reduction at all"
	}, day("2026-01-01"))
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if !schedule.FloorApplied {
		t.Fatal("the floor should have bound a falling index")
	}
	if got := amountsOn(t, schedule, "2026-01-31")[0]; got >= 10000 {
		// The floor is just below zero, so the rent barely moves.
		t.Logf("floored payment %.2f", got)
	}
}

func TestDerive_SteppedLadderTakesTheRungInForce(t *testing.T) {
	original := monthlySchedule(day("2026-01-31"), 24, 10000)

	schedule, err := DeriveRevisedPayments(original, PaymentRevision{
		Kind: RevisionStepped,
		Steps: []StepChange{
			// Deliberately out of order: the derivation sorts them.
			{FromDate: day("2027-01-01"), Amount: money.NewFromFloat(13000)},
			{FromDate: day("2026-07-01"), Amount: money.NewFromFloat(12000)},
		},
	}, day("2026-01-01"))
	if err != nil {
		t.Fatalf("derive: %v", err)
	}

	got := amountsOn(t, schedule, "2026-06-30", "2026-07-31", "2027-01-31")
	if got[0] != 10000 {
		t.Errorf("before the first rung the rent is unchanged, got %.2f", got[0])
	}
	if got[1] != 12000 {
		t.Errorf("first rung should give 12000, got %.2f", got[1])
	}
	if got[2] != 13000 {
		t.Errorf("second rung should give 13000, got %.2f", got[2])
	}
}

func TestDerive_LeavesVariableAndServiceChargesAlone(t *testing.T) {
	base := day("2026-06-30")
	original := []LeasePayment{
		{Date: base, Amount: money.NewFromFloat(10000), Timing: "postpaid", Type: "fixed"},
		{Date: base, Amount: money.NewFromFloat(3000), Timing: "postpaid", Type: "variable"},
		{Date: base, Amount: money.NewFromFloat(1500), Timing: "postpaid", Type: "non_lease"},
	}

	schedule, err := DeriveRevisedPayments(original, PaymentRevision{
		Kind:       RevisionPercentage,
		Percentage: 10,
	}, day("2026-01-01"))
	if err != nil {
		t.Fatalf("derive: %v", err)
	}

	byType := map[string]float64{}
	for _, change := range schedule.Changes {
		byType[change.Type] = change.RevisedAmount.Round("CNY").Float64()
	}
	if byType["fixed"] != 11000 {
		t.Errorf("fixed rent should escalate to 11000, got %.2f", byType["fixed"])
	}
	// An escalation clause restates the rent. Inflating turnover rent or the
	// service charge alongside it would overstate the liability.
	if byType["variable"] != 3000 {
		t.Errorf("variable rent must not be escalated, got %.2f", byType["variable"])
	}
	if byType["non_lease"] != 1500 {
		t.Errorf("non-lease component must not be escalated, got %.2f", byType["non_lease"])
	}
}

func TestDerive_ConcessionWindowEndsWhenTheClauseSays(t *testing.T) {
	original := monthlySchedule(day("2026-01-31"), 12, 10000)

	// Three months of half rent during a refit, then back to normal.
	schedule, err := DeriveRevisedPayments(original, PaymentRevision{
		Kind:        RevisionPercentage,
		Percentage:  -50,
		AppliesFrom: day("2026-03-01"),
		AppliesTo:   day("2026-05-31"),
	}, day("2026-03-01"))
	if err != nil {
		t.Fatalf("derive: %v", err)
	}

	got := amountsOn(t, schedule, "2026-02-28", "2026-03-31", "2026-05-31", "2026-06-30")
	if got[0] != 10000 {
		t.Errorf("before the concession the rent is unchanged, got %.2f", got[0])
	}
	if got[1] != 5000 || got[2] != 5000 {
		t.Errorf("during the concession the rent is halved, got %.2f and %.2f", got[1], got[2])
	}
	if got[3] != 10000 {
		t.Errorf("after the concession the rent returns to 10000, got %.2f", got[3])
	}
}

func TestDerive_SetAmountReplacesTheRent(t *testing.T) {
	original := monthlySchedule(day("2026-01-31"), 12, 10000)

	schedule, err := DeriveRevisedPayments(original, PaymentRevision{
		Kind:   RevisionSetAmount,
		Amount: money.NewFromFloat(8800),
	}, day("2026-07-01"))
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if got := amountsOn(t, schedule, "2026-07-31")[0]; got != 8800 {
		t.Errorf("want 8800, got %.2f", got)
	}
	if schedule.Delta.Round("CNY").Float64() != -7200 {
		t.Errorf("six payments falling by 1200 should total -7200, got %.2f", schedule.Delta.Round("CNY").Float64())
	}
}

func TestDerive_RejectsTermsThatCannotBeMeant(t *testing.T) {
	original := monthlySchedule(day("2026-01-31"), 12, 10000)

	cases := []struct {
		name     string
		revision PaymentRevision
	}{
		{"unknown kind", PaymentRevision{Kind: "guesswork"}},
		{"reduction past zero", PaymentRevision{Kind: RevisionPercentage, Percentage: -100}},
		{"index without readings", PaymentRevision{Kind: RevisionIndex, BaseIndex: 100}},
		{"negative index reading", PaymentRevision{Kind: RevisionIndex, BaseIndex: -1, NewIndex: 100}},
		{"negative rent", PaymentRevision{Kind: RevisionSetAmount, Amount: money.NewFromFloat(-1)}},
		{"stepped with no steps", PaymentRevision{Kind: RevisionStepped}},
		{"step without a date", PaymentRevision{Kind: RevisionStepped, Steps: []StepChange{{Amount: money.NewFromFloat(100)}}}},
		{"window ends before it starts", PaymentRevision{
			Kind: RevisionPercentage, Percentage: 5,
			AppliesFrom: day("2026-06-01"), AppliesTo: day("2026-03-01"),
		}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := DeriveRevisedPayments(original, testCase.revision, day("2026-01-01")); err == nil {
				t.Error("expected the terms to be rejected, but they were accepted")
			}
		})
	}
}

// The draft feeds the measurement engine, so the two must agree on shape.
func TestDerive_PaymentsRoundTripIntoTheEngine(t *testing.T) {
	original := monthlySchedule(day("2026-01-31"), 12, 10000)

	schedule, err := DeriveRevisedPayments(original, PaymentRevision{
		Kind: RevisionPercentage, Percentage: 5,
	}, day("2026-07-01"))
	if err != nil {
		t.Fatalf("derive: %v", err)
	}

	payments := schedule.Payments()
	if len(payments) != len(original) {
		t.Fatalf("the draft should keep every payment: want %d, got %d", len(original), len(payments))
	}
	for i, payment := range payments {
		if payment.Type != original[i].Type || payment.Timing != original[i].Timing {
			t.Errorf("payment %d lost its type or timing", i)
		}
		if !payment.Date.Equal(original[i].Date) {
			t.Errorf("payment %d moved date", i)
		}
	}
}
