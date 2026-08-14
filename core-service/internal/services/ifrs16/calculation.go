package ifrs16

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/lease-management-system/core-service/internal/money"
	"github.com/shopspring/decimal"
)

const (
	LeaseScopeInScope         = "in_scope"
	LeaseScopeShortTermExempt = "short_term_exempt"
	LeaseScopeLowValueExempt  = "low_value_exempt"
	LeaseScopeNotALease       = "not_a_lease"
)

func NormalizeLeaseScope(scope string) string {
	switch scope {
	case LeaseScopeInScope, LeaseScopeShortTermExempt, LeaseScopeLowValueExempt, LeaseScopeNotALease:
		return scope
	default:
		return ""
	}
}

func IsCapitalizedScope(scope string) bool {
	return scope == LeaseScopeInScope
}

// LeaseCalculation holds all inputs for IFRS 16 calculation
type LeaseCalculation struct {
	CommencementDate  time.Time
	LeaseEndDate      time.Time
	LeaseScope        string  // in_scope, short_term_exempt, low_value_exempt, not_a_lease
	DiscountRate      float64 // Annual discount rate (e.g., 0.05 for 5%)
	PaymentFrequency  string  // monthly, quarterly, yearly
	Payments          []LeasePayment
	InitialDirectCost money.Amount
	PrepaidRent       money.Amount // Already paid at commencement
	IncentiveReceived money.Amount
	RestorationCost   money.Amount
}

type LeasePayment struct {
	Date   time.Time
	Amount money.Amount // Total payment amount
	Timing string       // prepaid or postpaid
	Type   string       // fixed, variable, non_lease
}

// CalculationResult holds all IFRS 16 outputs
type CalculationResult struct {
	LeaseScope        string
	MeasurementBasis  string // capitalized, straight_line_expense, skipped
	InitialLiability  money.Amount
	InitialROUAsset   money.Amount
	DailyAmortization []DailyEntry
	MonthlySummary    []MonthlyEntry
}

type DailyEntry struct {
	Date                time.Time
	OpeningLiability    money.Amount
	InterestExpense     money.Amount
	Payment             money.Amount
	PrepaidPayment      money.Amount // Prepaid rent at/before commencement (capitalized into ROU, not reducing liability)
	LiabilityAdjustment money.Amount // Rounding/settlement adjustment to force liability to zero at lease end
	ClosingLiability    money.Amount
	OpeningROUAsset     money.Amount
	Depreciation        money.Amount
	ROUAdjustment       money.Amount // Rounding adjustment to force ROU to zero at lease end
	ClosingROUAsset     money.Amount
	ExemptLeaseExpense  money.Amount
	VariableRentExpense money.Amount
	NonLeaseExpense     money.Amount
}

type MonthlyEntry struct {
	Year                int
	Month               int
	OpeningLiability    money.Amount
	InterestExpense     money.Amount
	TotalPayments       money.Amount
	PrepaidPayment      money.Amount // Prepaid rent at/before commencement (capitalized into ROU)
	LiabilityAdjustment money.Amount
	ClosingLiability    money.Amount
	OpeningROUAsset     money.Amount
	Depreciation        money.Amount
	ROUAdjustment       money.Amount
	ClosingROUAsset     money.Amount
	ExemptLeaseExpense  money.Amount
	VariableRentExpense money.Amount
	NonLeaseExpense     money.Amount
}

// RemeasurementInput holds inputs for remeasuring a lease after a modification or reassessment event
type RemeasurementInput struct {
	EffectiveDate       time.Time
	LeaseEndDate        time.Time
	RevisedDiscountRate float64
	RevisedPayments     []LeasePayment
	InitialDirectCost   money.Amount
	LeaseIncentives     money.Amount

	// ScopeDecreaseProportion is the share of the lease given up, between 0 and
	// 1, where 1 is a full termination. It selects IFRS 16.46(a): the ROU asset
	// is written down in proportion to the scope surrendered and the difference
	// against the liability released goes to profit or loss. Leaving it at zero
	// keeps 46(b), where the whole liability movement is absorbed by the ROU
	// asset and no gain or loss arises.
	//
	// The distinction is what makes a loss possible at all. Under 46(b) the ROU
	// asset moves with the liability, so a shortened lease can only ever produce
	// a gain, which is not what happens when a lease is walked away from while
	// the asset still carries initial direct costs or a prepayment.
	ScopeDecreaseProportion float64
}

// RemeasurementOutput holds the results of a lease remeasurement
type RemeasurementOutput struct {
	NewLiability    money.Amount
	LiabilityDelta  money.Amount
	ROUAdjustment   money.Amount
	PnLGain         money.Amount
	PnLLoss         money.Amount
	NewROU          money.Amount
	ForwardSchedule []DailyEntry
}

// CalculatePrepaidRent computes the total prepaid rent at or before commencement date.
// Only fixed lease payments (not variable or non-lease) with Timing == "prepaid"
// and Date <= CommencementDate are included.
func CalculatePrepaidRent(input LeaseCalculation) money.Amount {
	prepaidRent := money.NewFromInt64(0)
	for _, payment := range input.Payments {
		if payment.Type == "variable" || payment.Type == "non_lease" {
			continue
		}
		if payment.Timing == "prepaid" && !payment.Date.After(input.CommencementDate) {
			prepaidRent = prepaidRent.Add(payment.Amount)
		}
	}
	return prepaidRent.Round("CNY")
}

// Calculate performs full IFRS 16 calculation with daily granularity
func Calculate(input LeaseCalculation) (*CalculationResult, error) {
	scope := NormalizeLeaseScope(input.LeaseScope)
	if scope == "" {
		return nil, fmt.Errorf("lease scope is required and must be one of in_scope, short_term_exempt, low_value_exempt, not_a_lease")
	}
	switch scope {
	case LeaseScopeInScope:
		return calculateCapitalized(input, scope), nil
	case LeaseScopeShortTermExempt, LeaseScopeLowValueExempt:
		return calculateStraightLineExpense(input, scope), nil
	case LeaseScopeNotALease:
		return skipMeasurement(input, scope), nil
	}
	return nil, fmt.Errorf("unsupported lease scope %q", input.LeaseScope)
}

func calculateCapitalized(input LeaseCalculation, scope string) *CalculationResult {
	result := &CalculationResult{
		LeaseScope:       scope,
		MeasurementBasis: "capitalized",
	}

	// 1. Calculate initial lease liability (PV of lease payments)
	result.InitialLiability = calculateInitialLiability(input)

	// 2. Calculate initial ROU asset
	// If PrepaidRent is not explicitly set, compute it from payments
	prepaidRent := input.PrepaidRent
	if !prepaidRent.IsSet() || prepaidRent.IsZero() {
		prepaidRent = CalculatePrepaidRent(input)
	}
	result.InitialROUAsset = result.InitialLiability.
		Add(input.InitialDirectCost).
		Add(prepaidRent).
		Sub(input.IncentiveReceived).
		Add(input.RestorationCost)

	// 3. Generate daily amortization schedule
	result.DailyAmortization = generateDailySchedule(input, result.InitialLiability, result.InitialROUAsset)

	// 4. Aggregate to monthly
	result.MonthlySummary = aggregateMonthly(result.DailyAmortization)

	return result
}

func calculateStraightLineExpense(input LeaseCalculation, scope string) *CalculationResult {
	result := &CalculationResult{
		LeaseScope:       scope,
		MeasurementBasis: "straight_line_expense",
	}
	result.DailyAmortization = generateStraightLineSchedule(input)
	result.MonthlySummary = aggregateMonthly(result.DailyAmortization)
	return result
}

func skipMeasurement(input LeaseCalculation, scope string) *CalculationResult {
	return &CalculationResult{
		LeaseScope:       scope,
		MeasurementBasis: "skipped",
	}
}

// GetCarryingAmount returns the lease liability and ROU asset carrying amounts as of the day before the given date.
// It runs a full Calculate and finds the latest state on or before (asOfDate - 1 day).
func GetCarryingAmount(input LeaseCalculation, asOfDate time.Time) (liability, rou money.Amount, err error) {
	result, err := Calculate(input)
	if err != nil {
		return money.Amount{}, money.Amount{}, err
	}

	// Find the state on the day BEFORE asOfDate
	targetDate := asOfDate.Add(-24 * time.Hour)

	for _, entry := range result.DailyAmortization {
		if !entry.Date.After(targetDate) {
			liability = entry.ClosingLiability
			rou = entry.ClosingROUAsset
		}
	}
	return liability, rou, nil
}

// calculateInitialLiability calculates PV of all lease payments
// paymentsAfter keeps the payments that are still due on or after a date. A
// prepaid payment falling exactly on the date is still due; a postpaid one
// covers the period that just ended and has been settled.
func paymentsAfter(payments []LeasePayment, from time.Time) []LeasePayment {
	outstanding := make([]LeasePayment, 0, len(payments))
	for _, payment := range payments {
		if payment.Date.After(from) || (payment.Date.Equal(from) && payment.Timing == "prepaid") {
			outstanding = append(outstanding, payment)
		}
	}
	return outstanding
}

func calculateInitialLiability(input LeaseCalculation) money.Amount {
	liability := decimal.Zero

	for _, payment := range input.Payments {
		// Skip variable and non-lease payments for liability calculation
		if payment.Type == "variable" || payment.Type == "non_lease" {
			continue
		}

		// Skip payments before commencement date (prepaid)
		if !payment.Date.After(input.CommencementDate) && payment.Timing == "prepaid" {
			continue
		}

		daysFromCommencement := payment.Date.Sub(input.CommencementDate).Hours() / 24
		dailyRate := math.Pow(1+input.DiscountRate, 1.0/365.0) - 1
		discountFactor := math.Pow(1+dailyRate, -daysFromCommencement)

		// The discount factor is a float64 rate computation; the payment is
		// exact money. The product is carried at full decimal precision and
		// rounded once at the boundary (the result field serialises to the
		// API) — never per step.
		liability = liability.Add(payment.Amount.Decimal().Mul(decimal.NewFromFloat(discountFactor)))
	}

	return money.New(liability).Round("CNY")
}

// generateDailySchedule creates daily-level amortization
func generateDailySchedule(input LeaseCalculation, initialLiability, initialROUAsset money.Amount) []DailyEntry {
	return GenerateForwardSchedule(input.CommencementDate, input.LeaseEndDate, initialLiability, initialROUAsset, input.DiscountRate, input.Payments, input.CommencementDate)
}

func generateStraightLineSchedule(input LeaseCalculation) []DailyEntry {
	var schedule []DailyEntry

	leaseTermDays := int(input.LeaseEndDate.Sub(input.CommencementDate).Hours() / 24)
	if leaseTermDays <= 0 {
		return schedule
	}

	totalLeasePayments := money.NewFromInt64(0)
	for _, payment := range input.Payments {
		if payment.Type == "variable" || payment.Type == "non_lease" {
			continue
		}
		totalLeasePayments = totalLeasePayments.Add(payment.Amount)
	}
	dailyExpense := totalLeasePayments.Decimal().Div(decimal.NewFromInt(int64(leaseTermDays)))

	for day := 0; day < leaseTermDays; day++ {
		currentDate := input.CommencementDate.Add(time.Duration(day) * 24 * time.Hour)

		variableRent := money.NewFromInt64(0)
		nonLeaseExpense := money.NewFromInt64(0)
		payment := money.NewFromInt64(0)
		prepaidPayment := money.NewFromInt64(0)
		for _, p := range input.Payments {
			if !isSameDay(p.Date, currentDate) {
				continue
			}
			switch p.Type {
			case "variable":
				variableRent = variableRent.Add(p.Amount)
			case "non_lease":
				nonLeaseExpense = nonLeaseExpense.Add(p.Amount)
			default:
				if p.Timing == "prepaid" && !p.Date.After(input.CommencementDate) {
					prepaidPayment = prepaidPayment.Add(p.Amount)
				} else {
					payment = payment.Add(p.Amount)
				}
			}
		}

		schedule = append(schedule, DailyEntry{
			Date:                currentDate,
			Payment:             payment.Round("CNY"),
			PrepaidPayment:      prepaidPayment.Round("CNY"),
			ExemptLeaseExpense:  money.New(dailyExpense).Round("CNY"),
			VariableRentExpense: variableRent.Round("CNY"),
			NonLeaseExpense:     nonLeaseExpense.Round("CNY"),
		})
	}

	return schedule
}

// GenerateForwardSchedule creates a daily amortization schedule from startDate to endDate.
// commencementDate is the original lease commencement (used to determine prepaid treatment).
// The schedule starts from startDate (e.g., the effective date of a modification) not commencementDate.
func GenerateForwardSchedule(startDate, endDate time.Time, initialLiability, initialROU money.Amount, discountRate float64, payments []LeasePayment, commencementDate time.Time) []DailyEntry {
	var schedule []DailyEntry

	leaseTermDays := int(endDate.Sub(startDate).Hours() / 24)
	if leaseTermDays <= 0 {
		return schedule
	}

	dailyDepreciation := initialROU.Decimal().Div(decimal.NewFromInt(int64(leaseTermDays)))
	currentLiability := initialLiability.Decimal()
	currentROUAsset := initialROU.Decimal()
	dailyRate := math.Pow(1+discountRate, 1.0/365.0) - 1

	for day := 0; day < leaseTermDays; day++ {
		currentDate := startDate.Add(time.Duration(day) * 24 * time.Hour)

		openingLiability := currentLiability
		openingROUAsset := currentROUAsset

		interest := currentLiability.Mul(decimal.NewFromFloat(dailyRate))

		payment := decimal.Zero
		variableRent := decimal.Zero
		nonLeaseExpense := decimal.Zero
		prepaidAtCommencement := decimal.Zero

		for _, p := range payments {
			if !isSameDay(p.Date, currentDate) {
				continue
			}
			switch p.Type {
			case "variable":
				variableRent = variableRent.Add(p.Amount.Decimal())
			case "non_lease":
				nonLeaseExpense = nonLeaseExpense.Add(p.Amount.Decimal())
			default:
				if p.Timing == "prepaid" && !p.Date.After(commencementDate) {
					// Prepaid at/before commencement: capitalized into ROU, does not reduce liability
					prepaidAtCommencement = prepaidAtCommencement.Add(p.Amount.Decimal())
					continue
				}
				payment = payment.Add(p.Amount.Decimal())
			}
		}

		currentLiability = currentLiability.Add(interest).Sub(payment)
		depreciation := dailyDepreciation
		currentROUAsset = currentROUAsset.Sub(depreciation)

		// Build the entry. Every field is quantised at this boundary — the
		// schedule is what the API serialises and what the ledger persists;
		// the carry (currentLiability/currentROUAsset) stays at full
		// precision between periods.
		entry := DailyEntry{
			Date:                currentDate,
			OpeningLiability:    money.New(openingLiability).Round("CNY"),
			InterestExpense:     money.New(interest).Round("CNY"),
			Payment:             money.New(payment).Round("CNY"),
			PrepaidPayment:      money.New(prepaidAtCommencement).Round("CNY"),
			ClosingLiability:    money.New(currentLiability).Round("CNY"),
			OpeningROUAsset:     money.New(openingROUAsset).Round("CNY"),
			Depreciation:        money.New(depreciation).Round("CNY"),
			ClosingROUAsset:     money.New(currentROUAsset).Round("CNY"),
			VariableRentExpense: money.New(variableRent).Round("CNY"),
			NonLeaseExpense:     money.New(nonLeaseExpense).Round("CNY"),
		}

		// Force zero on the last day of the lease term.
		// The daily amortization accumulates floating-point drift over the lease term
		// because the PV calculation (fractional-day discounting) and daily compounding
		// (integer-day rate) don't perfectly reconcile. The residual is recorded as a
		// rounding adjustment so the formula balances:
		//   Closing = Opening + Interest - Payment + Adjustment = 0
		isLastDay := day == leaseTermDays-1
		if isLastDay {
			if !currentLiability.IsZero() {
				entry.LiabilityAdjustment = money.New(currentLiability.Neg()).Round("CNY")
				entry.ClosingLiability = money.NewFromInt64(0)
				currentLiability = decimal.Zero
			}
			if !currentROUAsset.IsZero() {
				entry.ROUAdjustment = money.New(currentROUAsset.Neg()).Round("CNY")
				entry.ClosingROUAsset = money.NewFromInt64(0)
				currentROUAsset = decimal.Zero
			}
		}

		schedule = append(schedule, entry)
	}

	return schedule
}

// aggregateMonthly aggregates daily entries to monthly
func aggregateMonthly(dailyEntries []DailyEntry) []MonthlyEntry {
	if len(dailyEntries) == 0 {
		return nil
	}

	monthMap := make(map[string]*MonthlyEntry)

	for _, entry := range dailyEntries {
		key := entry.Date.Format("2006-01")

		if _, exists := monthMap[key]; !exists {
			monthMap[key] = &MonthlyEntry{
				Year:             entry.Date.Year(),
				Month:            int(entry.Date.Month()),
				OpeningLiability: entry.OpeningLiability,
				OpeningROUAsset:  entry.OpeningROUAsset,
			}
		}

		m := monthMap[key]
		m.InterestExpense = m.InterestExpense.Add(entry.InterestExpense)
		m.TotalPayments = m.TotalPayments.Add(entry.Payment)
		m.PrepaidPayment = m.PrepaidPayment.Add(entry.PrepaidPayment)
		m.LiabilityAdjustment = m.LiabilityAdjustment.Add(entry.LiabilityAdjustment)
		m.Depreciation = m.Depreciation.Add(entry.Depreciation)
		m.ROUAdjustment = m.ROUAdjustment.Add(entry.ROUAdjustment)
		m.ClosingLiability = entry.ClosingLiability
		m.ClosingROUAsset = entry.ClosingROUAsset
		m.ExemptLeaseExpense = m.ExemptLeaseExpense.Add(entry.ExemptLeaseExpense)
		m.VariableRentExpense = m.VariableRentExpense.Add(entry.VariableRentExpense)
		m.NonLeaseExpense = m.NonLeaseExpense.Add(entry.NonLeaseExpense)
	}

	// Convert map to sorted slice. The wire output must be deterministic:
	// a random month order would make API responses and the regression
	// report churn on every run.
	var result []MonthlyEntry
	for _, m := range monthMap {
		result = append(result, *m)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Year != result[j].Year {
			return result[i].Year < result[j].Year
		}
		return result[i].Month < result[j].Month
	})

	return result
}

func isSameDay(t1, t2 time.Time) bool {
	return t1.Year() == t2.Year() && t1.YearDay() == t2.YearDay()
}

// RecalculateFromDate performs a lease remeasurement from a given carrying amount.
// It computes the new liability PV from the effective date using revised payments/discount rate,
// adjusts the ROU by the liability change, handles P&L recognition when ROU reduction exceeds
// the carrying amount, and generates a forward amortization schedule.
func RecalculateFromDate(carryingLiability, carryingROU money.Amount, input RemeasurementInput) (*RemeasurementOutput, error) {
	output := &RemeasurementOutput{}

	// 1. Calculate new liability = PV of revised payments from effective date.
	//
	// Only payments still outstanding are priced. A remeasured liability is the
	// present value of what is left to pay, and a payment already made is not.
	// The filter matters because calculateInitialLiability discounts by days
	// from its commencement date: a past payment yields a negative day count and
	// is compounded up rather than discounted, so leaving it in would inflate
	// the liability by more than the payment itself.
	outstanding := paymentsAfter(input.RevisedPayments, input.EffectiveDate)
	calcInput := LeaseCalculation{
		CommencementDate: input.EffectiveDate,
		LeaseEndDate:     input.LeaseEndDate,
		DiscountRate:     input.RevisedDiscountRate,
		Payments:         outstanding,
	}
	output.NewLiability = calculateInitialLiability(calcInput)
	output.LiabilityDelta = output.NewLiability.Sub(carryingLiability)

	if input.ScopeDecreaseProportion > 0 {
		// IFRS 16.46(a): scope decrease. The ROU asset is written down by the
		// share of the lease surrendered, and the difference between that
		// write-down and the liability released is a gain or a loss.
		proportion := math.Min(input.ScopeDecreaseProportion, 1)
		rouRemoved := carryingROU.Decimal().Mul(decimal.NewFromFloat(proportion))
		liabilityReleased := output.LiabilityDelta.Neg()

		output.ROUAdjustment = money.New(rouRemoved.Neg())
		difference := liabilityReleased.Decimal().Sub(rouRemoved)
		if difference.IsPositive() || difference.IsZero() {
			output.PnLGain = money.New(difference).Round("CNY")
		} else {
			// More asset written off than liability released — this is the loss
			// the engine previously had no way to express.
			output.PnLLoss = money.New(difference.Neg()).Round("CNY")
		}
	} else {
		// IFRS 16.46(b): every other remeasurement. The ROU asset absorbs the
		// liability movement, so no gain or loss arises.
		output.ROUAdjustment = output.LiabilityDelta

		// An ROU asset cannot be driven below zero; whatever the liability
		// reduction cannot absorb is a gain.
		if output.ROUAdjustment.Decimal().IsNegative() && output.ROUAdjustment.Neg().Decimal().Cmp(carryingROU.Decimal()) > 0 {
			output.PnLGain = output.ROUAdjustment.Neg().Sub(carryingROU).Round("CNY")
			output.ROUAdjustment = carryingROU.Neg()
		}
	}

	// 4. Compute new ROU
	output.NewROU = carryingROU.Add(output.ROUAdjustment).Add(input.InitialDirectCost).Sub(input.LeaseIncentives)
	if output.NewROU.Decimal().IsNegative() {
		output.NewROU = money.NewFromInt64(0)
	}

	// 5. Generate forward schedule from effective date
	output.ForwardSchedule = GenerateForwardSchedule(
		input.EffectiveDate,
		input.LeaseEndDate,
		output.NewLiability,
		output.NewROU,
		input.RevisedDiscountRate,
		input.RevisedPayments,
		input.EffectiveDate, // effective date is also the "commencement" for prepaid logic in the forward period
	)

	return output, nil
}
