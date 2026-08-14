package ifrs16

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/money"
	"github.com/shopspring/decimal"
)

// The forward amortization carries a balance from one period to the next for
// 36 or more periods (calculation.go). This test runs an independent
// full-precision oracle of the documented recurrence — carry_next =
// carry·(1+r_d) − payment, payments matched by day — and asserts that every
// emitted schedule field equals the oracle's value rounded once at the cent,
// for every day of the term. Any drift in the carry (per-step rounding,
// accumulated float error) shows up as a cent-level mismatch somewhere in the
// term; a representation change that stays honest must match exactly.
//
// The oracle is written from the recurrence, not from the engine's code, and
// deliberately starts from the engine's rounded initial liability: the point
// under test is the 36-period amortization, not the PV formula, which the
// regression suite already pins to the golden values.
func TestForwardCarryConservesExactlyOverTheTerm(t *testing.T) {
	for _, months := range []int{36, 60} {
		t.Run(fmt.Sprintf("%02d months", months), func(t *testing.T) {
			start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
			end := start.AddDate(0, months, 0).AddDate(0, 0, -1)

			var payments []LeasePayment
			for month := 0; month < months; month++ {
				monthEnd := start.AddDate(0, month+1, 0).AddDate(0, 0, -1)
				payments = append(payments, LeasePayment{
					Date: monthEnd, Amount: money.NewFromInt64(10000), Timing: "postpaid", Type: "fixed",
				})
			}

			result, err := Calculate(LeaseCalculation{
				CommencementDate: start, LeaseEndDate: end,
				LeaseScope: LeaseScopeInScope, DiscountRate: 0.05, Payments: payments,
			})
			if err != nil {
				t.Fatalf("Calculate: %v", err)
			}

			schedule := result.DailyAmortization
			if len(schedule) == 0 {
				t.Fatal("empty daily schedule")
			}

			dailyRate := math.Pow(1.05, 1.0/365.0) - 1
			dailyDepreciation := result.InitialROUAsset.Decimal().Div(decimal.NewFromInt(int64(len(schedule))))
			carry := result.InitialLiability.Decimal()
			rouCarry := result.InitialROUAsset.Decimal()

			totalInterest := decimal.Zero
			totalPayments := decimal.Zero
			totalDepreciation := decimal.Zero

			for day, entry := range schedule {
				wantOpening := carry
				wantOpeningROU := rouCarry

				interest := carry.Mul(decimal.NewFromFloat(dailyRate))
				payment := decimal.Zero
				for _, p := range payments {
					if !isSameDay(p.Date, entry.Date) {
						continue
					}
					payment = payment.Add(p.Amount.Decimal())
				}
				carry = carry.Add(interest).Sub(payment)
				rouCarry = rouCarry.Sub(dailyDepreciation)
				totalInterest = totalInterest.Add(interest)
				totalPayments = totalPayments.Add(payment)
				totalDepreciation = totalDepreciation.Add(dailyDepreciation)

				// C4: 期初 − 付款 + 利息 = 期末, exactly, at full precision.
				if !carry.Equal(wantOpening.Add(interest).Sub(payment)) {
					t.Fatalf("day %d: recurrence does not conserve", day)
				}
				if !rouCarry.Equal(wantOpeningROU.Sub(dailyDepreciation)) {
					t.Fatalf("day %d: ROU recurrence does not conserve", day)
				}

				// Every emitted field is the oracle value rounded once — a
				// mismatch means the carried balance drifted.
				isLastDay := day == len(schedule)-1
				assertAmount(t, day, "opening_liability", entry.OpeningLiability, wantOpening)
				assertAmount(t, day, "interest_expense", entry.InterestExpense, interest)
				assertAmount(t, day, "payment", entry.Payment, payment)
				assertAmount(t, day, "opening_rou_asset", entry.OpeningROUAsset, wantOpeningROU)
				assertAmount(t, day, "depreciation", entry.Depreciation, dailyDepreciation)
				if isLastDay {
					// C5: the engine forces the balances to zero on the last
					// day and books the remainder as the settlement
					// adjustment — a postpaid payment due exactly at lease
					// end falls on the day after the schedule and is settled
					// here. The adjustment must be exactly the oracle's
					// remainder, rounded once.
					if !entry.ClosingLiability.IsZero() || !entry.ClosingROUAsset.IsZero() {
						t.Errorf("day %d: final balance must be zero, got liability %v ROU %v",
							day, entry.ClosingLiability, entry.ClosingROUAsset)
					}
					assertAmount(t, day, "liability_adjustment", entry.LiabilityAdjustment, carry.Neg())
					assertAmount(t, day, "rou_adjustment", entry.ROUAdjustment, rouCarry.Neg())
				} else {
					assertAmount(t, day, "closing_liability", entry.ClosingLiability, carry)
					assertAmount(t, day, "closing_rou_asset", entry.ClosingROUAsset, rouCarry)
				}
			}

			// C5, term level: after 36+ periods the recurrence plus the
			// settlement adjustment balances exactly against the initial
			// liability and the payments —
			//   initial + Σinterest − Σpayments + adjustment = 0.
			// The oracle's adjustment is exactly what the last day's entry
			// carried, so this is the same conservation statement the daily
			// loop already proved, closed over the whole term.
			adjustment := carry.Neg()
			if !totalInterest.Add(adjustment).Equal(totalPayments.Sub(result.InitialLiability.Decimal())) {
				t.Errorf("interest + adjustment = %v, want payments − initial = %v",
					totalInterest.Add(adjustment), totalPayments.Sub(result.InitialLiability.Decimal()))
			}
			if !totalDepreciation.Add(rouCarry).Equal(result.InitialROUAsset.Decimal()) {
				t.Errorf("depreciation + residual = %v, want initial ROU = %v",
					totalDepreciation.Add(rouCarry), result.InitialROUAsset.Decimal())
			}

			// The monthly aggregation is a plain sum of the daily entries: no
			// month may disagree with its own days.
			monthlyTotal := money.NewFromInt64(0)
			monthlyInterest := money.NewFromInt64(0)
			for _, month := range result.MonthlySummary {
				monthlyTotal = monthlyTotal.Add(month.TotalPayments)
				monthlyInterest = monthlyInterest.Add(month.InterestExpense)
			}
			var dayTotal, dayInterest money.Amount
			for _, entry := range schedule {
				dayTotal = dayTotal.Add(entry.Payment)
				dayInterest = dayInterest.Add(entry.InterestExpense)
			}
			if !monthlyTotal.Equal(dayTotal) || !monthlyInterest.Equal(dayInterest) {
				t.Errorf("monthly sums disagree with daily sums: payments %v/%v interest %v/%v",
					monthlyTotal, dayTotal, monthlyInterest, dayInterest)
			}
		})
	}
}

func assertAmount(t *testing.T, day int, name string, got money.Amount, wantDecimal decimal.Decimal) {
	t.Helper()
	want := money.New(wantDecimal).Round("CNY")
	if !got.Equal(want) {
		t.Fatalf("day %d: %s = %v, want %v (oracle rounded)", day, name, got, want)
	}
}
