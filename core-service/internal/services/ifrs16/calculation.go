package ifrs16

import (
	"math"
	"time"
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
		return LeaseScopeInScope
	}
}

func IsCapitalizedScope(scope string) bool {
	return NormalizeLeaseScope(scope) == LeaseScopeInScope
}

// LeaseCalculation holds all inputs for IFRS 16 calculation
type LeaseCalculation struct {
	CommencementDate  time.Time
	LeaseEndDate      time.Time
	LeaseScope        string  // in_scope, short_term_exempt, low_value_exempt, not_a_lease
	DiscountRate      float64 // Annual discount rate (e.g., 0.05 for 5%)
	PaymentFrequency  string  // monthly, quarterly, yearly
	Payments          []LeasePayment
	InitialDirectCost float64
	PrepaidRent       float64 // Already paid at commencement
	IncentiveReceived float64
	RestorationCost   float64
}

type LeasePayment struct {
	Date   time.Time
	Amount float64 // Total payment amount
	Timing string  // prepaid or postpaid
	Type   string  // fixed, variable, non_lease
}

// CalculationResult holds all IFRS 16 outputs
type CalculationResult struct {
	LeaseScope        string
	MeasurementBasis  string // capitalized, straight_line_expense, skipped
	InitialLiability  float64
	InitialROUAsset   float64
	DailyAmortization []DailyEntry
	MonthlySummary    []MonthlyEntry
}

type DailyEntry struct {
	Date                time.Time
	OpeningLiability    float64
	InterestExpense     float64
	Payment             float64
	PrepaidPayment      float64 // Prepaid rent at/before commencement (capitalized into ROU, not reducing liability)
	LiabilityAdjustment float64 // Rounding/settlement adjustment to force liability to zero at lease end
	ClosingLiability    float64
	OpeningROUAsset     float64
	Depreciation        float64
	ROUAdjustment       float64 // Rounding adjustment to force ROU to zero at lease end
	ClosingROUAsset     float64
	ExemptLeaseExpense  float64
	VariableRentExpense float64
	NonLeaseExpense     float64
}

type MonthlyEntry struct {
	Year                int
	Month               int
	OpeningLiability    float64
	InterestExpense     float64
	TotalPayments       float64
	PrepaidPayment      float64 // Prepaid rent at/before commencement (capitalized into ROU)
	LiabilityAdjustment float64
	ClosingLiability    float64
	OpeningROUAsset     float64
	Depreciation        float64
	ROUAdjustment       float64
	ClosingROUAsset     float64
	ExemptLeaseExpense  float64
	VariableRentExpense float64
	NonLeaseExpense     float64
}

// RemeasurementInput holds inputs for remeasuring a lease after a modification or reassessment event
type RemeasurementInput struct {
	EffectiveDate       time.Time
	LeaseEndDate        time.Time
	RevisedDiscountRate float64
	RevisedPayments     []LeasePayment
	InitialDirectCost   float64
	LeaseIncentives     float64
}

// RemeasurementOutput holds the results of a lease remeasurement
type RemeasurementOutput struct {
	NewLiability    float64
	LiabilityDelta  float64
	ROUAdjustment   float64
	PnLGain         float64
	PnLLoss         float64
	NewROU          float64
	ForwardSchedule []DailyEntry
}

// CalculatePrepaidRent computes the total prepaid rent at or before commencement date.
// Only fixed lease payments (not variable or non-lease) with Timing == "prepaid"
// and Date <= CommencementDate are included.
func CalculatePrepaidRent(input LeaseCalculation) float64 {
	var prepaidRent float64
	for _, payment := range input.Payments {
		if payment.Type == "variable" || payment.Type == "non_lease" {
			continue
		}
		if payment.Timing == "prepaid" && !payment.Date.After(input.CommencementDate) {
			prepaidRent += payment.Amount
		}
	}
	return round(prepaidRent)
}

// Calculate performs full IFRS 16 calculation with daily granularity
func Calculate(input LeaseCalculation) (*CalculationResult, error) {
	scope := NormalizeLeaseScope(input.LeaseScope)
	switch scope {
	case LeaseScopeInScope:
		return calculateCapitalized(input, scope), nil
	case LeaseScopeShortTermExempt, LeaseScopeLowValueExempt:
		return calculateStraightLineExpense(input, scope), nil
	case LeaseScopeNotALease:
		return skipMeasurement(input, scope), nil
	default:
		return calculateCapitalized(input, LeaseScopeInScope), nil
	}
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
	if prepaidRent == 0 {
		prepaidRent = CalculatePrepaidRent(input)
	}
	result.InitialROUAsset = result.InitialLiability +
		input.InitialDirectCost +
		prepaidRent -
		input.IncentiveReceived +
		input.RestorationCost

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
func GetCarryingAmount(input LeaseCalculation, asOfDate time.Time) (liability, rou float64, err error) {
	result, err := Calculate(input)
	if err != nil {
		return 0, 0, err
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
func calculateInitialLiability(input LeaseCalculation) float64 {
	var liability float64

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

		liability += payment.Amount * discountFactor
	}

	return round(liability)
}

// generateDailySchedule creates daily-level amortization
func generateDailySchedule(input LeaseCalculation, initialLiability, initialROUAsset float64) []DailyEntry {
	return GenerateForwardSchedule(input.CommencementDate, input.LeaseEndDate, initialLiability, initialROUAsset, input.DiscountRate, input.Payments, input.CommencementDate)
}

func generateStraightLineSchedule(input LeaseCalculation) []DailyEntry {
	var schedule []DailyEntry

	leaseTermDays := int(input.LeaseEndDate.Sub(input.CommencementDate).Hours() / 24)
	if leaseTermDays <= 0 {
		return schedule
	}

	totalLeasePayments := 0.0
	for _, payment := range input.Payments {
		if payment.Type == "variable" || payment.Type == "non_lease" {
			continue
		}
		totalLeasePayments += payment.Amount
	}
	dailyExpense := totalLeasePayments / float64(leaseTermDays)

	for day := 0; day < leaseTermDays; day++ {
		currentDate := input.CommencementDate.Add(time.Duration(day) * 24 * time.Hour)

		variableRent := 0.0
		nonLeaseExpense := 0.0
		payment := 0.0
		prepaidPayment := 0.0
		for _, p := range input.Payments {
			if !isSameDay(p.Date, currentDate) {
				continue
			}
			switch p.Type {
			case "variable":
				variableRent += p.Amount
			case "non_lease":
				nonLeaseExpense += p.Amount
			default:
				if p.Timing == "prepaid" && !p.Date.After(input.CommencementDate) {
					prepaidPayment += p.Amount
				} else {
					payment += p.Amount
				}
			}
		}

		schedule = append(schedule, DailyEntry{
			Date:                currentDate,
			Payment:             round(payment),
			PrepaidPayment:      round(prepaidPayment),
			ExemptLeaseExpense:  round(dailyExpense),
			VariableRentExpense: round(variableRent),
			NonLeaseExpense:     round(nonLeaseExpense),
		})
	}

	return schedule
}

// GenerateForwardSchedule creates a daily amortization schedule from startDate to endDate.
// commencementDate is the original lease commencement (used to determine prepaid treatment).
// The schedule starts from startDate (e.g., the effective date of a modification) not commencementDate.
func GenerateForwardSchedule(startDate, endDate time.Time, initialLiability, initialROU, discountRate float64, payments []LeasePayment, commencementDate time.Time) []DailyEntry {
	var schedule []DailyEntry

	leaseTermDays := int(endDate.Sub(startDate).Hours() / 24)
	if leaseTermDays <= 0 {
		return schedule
	}

	dailyDepreciation := initialROU / float64(leaseTermDays)
	currentLiability := initialLiability
	currentROUAsset := initialROU
	dailyRate := math.Pow(1+discountRate, 1.0/365.0) - 1

	for day := 0; day < leaseTermDays; day++ {
		currentDate := startDate.Add(time.Duration(day) * 24 * time.Hour)

		openingLiability := currentLiability
		openingROUAsset := currentROUAsset

		interest := currentLiability * dailyRate

		payment := 0.0
		variableRent := 0.0
		nonLeaseExpense := 0.0
		prepaidAtCommencement := 0.0

		for _, p := range payments {
			if !isSameDay(p.Date, currentDate) {
				continue
			}
			switch p.Type {
			case "variable":
				variableRent += p.Amount
			case "non_lease":
				nonLeaseExpense += p.Amount
			default:
				if p.Timing == "prepaid" && !p.Date.After(commencementDate) {
					// Prepaid at/before commencement: capitalized into ROU, does not reduce liability
					prepaidAtCommencement += p.Amount
					continue
				}
				payment += p.Amount
			}
		}

		currentLiability = currentLiability + interest - payment
		depreciation := dailyDepreciation
		currentROUAsset = currentROUAsset - depreciation

		// Build the entry
		entry := DailyEntry{
			Date:                currentDate,
			OpeningLiability:    round(openingLiability),
			InterestExpense:     round(interest),
			Payment:             round(payment),
			PrepaidPayment:      round(prepaidAtCommencement),
			ClosingLiability:    round(currentLiability),
			OpeningROUAsset:     round(openingROUAsset),
			Depreciation:        round(depreciation),
			ClosingROUAsset:     round(currentROUAsset),
			VariableRentExpense: round(variableRent),
			NonLeaseExpense:     round(nonLeaseExpense),
		}

		// Force zero on the last day of the lease term.
		// The daily amortization accumulates floating-point drift over the lease term
		// because the PV calculation (fractional-day discounting) and daily compounding
		// (integer-day rate) don't perfectly reconcile. The residual is recorded as a
		// rounding adjustment so the formula balances:
		//   Closing = Opening + Interest - Payment + Adjustment = 0
		isLastDay := day == leaseTermDays-1
		if isLastDay {
			if currentLiability != 0 {
				entry.LiabilityAdjustment = round(-currentLiability)
				entry.ClosingLiability = 0
				currentLiability = 0
			}
			if currentROUAsset != 0 {
				entry.ROUAdjustment = round(-currentROUAsset)
				entry.ClosingROUAsset = 0
				currentROUAsset = 0
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
		m.InterestExpense += entry.InterestExpense
		m.TotalPayments += entry.Payment
		m.PrepaidPayment += entry.PrepaidPayment
		m.LiabilityAdjustment += entry.LiabilityAdjustment
		m.Depreciation += entry.Depreciation
		m.ROUAdjustment += entry.ROUAdjustment
		m.ClosingLiability = entry.ClosingLiability
		m.ClosingROUAsset = entry.ClosingROUAsset
		m.ExemptLeaseExpense += entry.ExemptLeaseExpense
		m.VariableRentExpense += entry.VariableRentExpense
		m.NonLeaseExpense += entry.NonLeaseExpense
	}

	// Convert map to sorted slice
	var result []MonthlyEntry
	for _, m := range monthMap {
		result = append(result, *m)
	}

	return result
}

func isSameDay(t1, t2 time.Time) bool {
	return t1.Year() == t2.Year() && t1.YearDay() == t2.YearDay()
}

func round(val float64) float64 {
	return math.Round(val*100) / 100
}

// RecalculateFromDate performs a lease remeasurement from a given carrying amount.
// It computes the new liability PV from the effective date using revised payments/discount rate,
// adjusts the ROU by the liability change, handles P&L recognition when ROU reduction exceeds
// the carrying amount, and generates a forward amortization schedule.
func RecalculateFromDate(carryingLiability, carryingROU float64, input RemeasurementInput) (*RemeasurementOutput, error) {
	output := &RemeasurementOutput{}

	// 1. Calculate new liability = PV of revised payments from effective date
	calcInput := LeaseCalculation{
		CommencementDate: input.EffectiveDate,
		LeaseEndDate:     input.LeaseEndDate,
		DiscountRate:     input.RevisedDiscountRate,
		Payments:         input.RevisedPayments,
	}
	output.NewLiability = calculateInitialLiability(calcInput)
	output.LiabilityDelta = output.NewLiability - carryingLiability

	// 2. Adjust ROU by change in liability (IFRS 16.46)
	output.ROUAdjustment = output.LiabilityDelta

	// 3. Check if ROU reduction exceeds carrying amount → P&L gain
	if output.ROUAdjustment < 0 && math.Abs(output.ROUAdjustment) > carryingROU {
		output.PnLGain = math.Abs(output.ROUAdjustment) - carryingROU
		output.ROUAdjustment = -carryingROU
	}

	// 4. Compute new ROU
	output.NewROU = carryingROU + output.ROUAdjustment + input.InitialDirectCost - input.LeaseIncentives
	if output.NewROU < 0 {
		output.NewROU = 0
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
