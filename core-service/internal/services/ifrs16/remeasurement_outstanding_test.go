package ifrs16

import (
	"math"
	"testing"
	"time"
)

// A remeasured liability is the present value of what is left to pay. Payments
// already made must not be priced again.
//
// This was wrong in a way that inflated rather than merely misstated the
// liability: calculateInitialLiability discounts by days from its commencement
// date, and for a remeasurement that date is the effective date. A payment in
// the past therefore had a negative day count, which compounds the amount up
// instead of discounting it down. A mid-term rent review on a year-long lease
// roughly doubled the liability.
func TestRecalculateFromDate_PricesOnlyOutstandingPayments(t *testing.T) {
	effective := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	// A full year of level rent; six months of it has already been paid.
	var payments []LeasePayment
	for month := 1; month <= 12; month++ {
		monthEnd := time.Date(2026, time.Month(month), 1, 0, 0, 0, 0, time.UTC).
			AddDate(0, 1, 0).AddDate(0, 0, -1)
		payments = append(payments, LeasePayment{
			Date: monthEnd, Amount: 10000, Timing: "postpaid", Type: "fixed",
		})
	}

	output, err := RecalculateFromDate(59161.43, 58935.32, RemeasurementInput{
		EffectiveDate:       effective,
		LeaseEndDate:        end,
		RevisedDiscountRate: 0.05,
		RevisedPayments:     payments,
	})
	if err != nil {
		t.Fatalf("RecalculateFromDate: %v", err)
	}

	// Six payments of 10,000 remain, so the liability is just under 60,000.
	if output.NewLiability > 60000 {
		t.Errorf("liability %.2f exceeds the 60,000 still to be paid; past payments are being priced",
			output.NewLiability)
	}
	if output.NewLiability < 57000 {
		t.Errorf("liability %.2f is too low for six remaining payments of 10,000", output.NewLiability)
	}

	// Nothing about the lease changed, so the remeasurement should barely move.
	if math.Abs(output.LiabilityDelta) > 100 {
		t.Errorf("an unchanged schedule moved the liability by %.2f", output.LiabilityDelta)
	}
}

// A rent increase must move the liability by the present value of the increase,
// not by a multiple of it.
func TestRecalculateFromDate_RentIncreaseMovesLiabilityByItsPresentValue(t *testing.T) {
	effective := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)

	var payments []LeasePayment
	for month := 1; month <= 12; month++ {
		monthEnd := time.Date(2026, time.Month(month), 1, 0, 0, 0, 0, time.UTC).
			AddDate(0, 1, 0).AddDate(0, 0, -1)
		amount := 10000.0
		if month >= 7 {
			amount = 10200 // a 2% review from July
		}
		payments = append(payments, LeasePayment{
			Date: monthEnd, Amount: amount, Timing: "postpaid", Type: "fixed",
		})
	}

	output, err := RecalculateFromDate(59161.43, 58935.32, RemeasurementInput{
		EffectiveDate:       effective,
		LeaseEndDate:        end,
		RevisedDiscountRate: 0.05,
		RevisedPayments:     payments,
	})
	if err != nil {
		t.Fatalf("RecalculateFromDate: %v", err)
	}

	// Six payments rise by 200 each: 1,200 undiscounted, a little less in
	// present value. Anything far above that means past payments crept in.
	if output.LiabilityDelta <= 0 || output.LiabilityDelta > 1200 {
		t.Errorf("liability moved by %.2f, want a positive amount no greater than the 1,200 of extra rent",
			output.LiabilityDelta)
	}
}

func TestPaymentsAfter(t *testing.T) {
	on := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	payments := []LeasePayment{
		{Date: time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC), Timing: "postpaid"},
		// Falling exactly on the date: a postpaid payment covers the period that
		// just ended and is settled; a prepaid one is for the period starting now.
		{Date: on, Timing: "postpaid"},
		{Date: on, Timing: "prepaid"},
		{Date: time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC), Timing: "postpaid"},
	}

	outstanding := paymentsAfter(payments, on)
	if len(outstanding) != 2 {
		t.Fatalf("expected 2 outstanding payments, got %d: %+v", len(outstanding), outstanding)
	}
	if outstanding[0].Timing != "prepaid" || !outstanding[0].Date.Equal(on) {
		t.Errorf("a prepaid payment falling on the date is still due: %+v", outstanding[0])
	}
}
