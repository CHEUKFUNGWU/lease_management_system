package ifrs16

import (
	"math"
	"time"
)

// LeaseCalculation holds all inputs for IFRS 16 calculation
type LeaseCalculation struct {
	CommencementDate  time.Time
	LeaseEndDate      time.Time
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
	InitialLiability    float64
	InitialROUAsset     float64
	DailyAmortization   []DailyEntry
	MonthlySummary      []MonthlyEntry
}

type DailyEntry struct {
	Date              time.Time
	OpeningLiability  float64
	InterestExpense   float64
	Payment           float64
	ClosingLiability  float64
	OpeningROUAsset   float64
	Depreciation      float64
	ClosingROUAsset   float64
}

type MonthlyEntry struct {
	Year             int
	Month            int
	OpeningLiability float64
	InterestExpense  float64
	TotalPayments    float64
	ClosingLiability float64
	OpeningROUAsset  float64
	Depreciation     float64
	ClosingROUAsset  float64
}

// Calculate performs full IFRS 16 calculation with daily granularity
func Calculate(input LeaseCalculation) (*CalculationResult, error) {
	result := &CalculationResult{}
	
	// 1. Calculate initial lease liability (PV of lease payments)
	result.InitialLiability = calculateInitialLiability(input)
	
	// 2. Calculate initial ROU asset
	result.InitialROUAsset = result.InitialLiability + 
		input.InitialDirectCost + 
		input.PrepaidRent - 
		input.IncentiveReceived +
		input.RestorationCost
	
	// 3. Generate daily amortization schedule
	result.DailyAmortization = generateDailySchedule(input, result.InitialLiability, result.InitialROUAsset)
	
	// 4. Aggregate to monthly
	result.MonthlySummary = aggregateMonthly(result.DailyAmortization)
	
	return result, nil
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
	var schedule []DailyEntry
	
	// Lease term in days
	leaseTermDays := int(input.LeaseEndDate.Sub(input.CommencementDate).Hours() / 24)
	if leaseTermDays <= 0 {
		return schedule
	}
	
	// Daily depreciation
	dailyDepreciation := initialROUAsset / float64(leaseTermDays)
	
	currentLiability := initialLiability
	currentROUAsset := initialROUAsset
	dailyRate := math.Pow(1+input.DiscountRate, 1.0/365.0) - 1
	
	for day := 0; day < leaseTermDays; day++ {
		currentDate := input.CommencementDate.Add(time.Duration(day) * 24 * time.Hour)
		
		openingLiability := currentLiability
		openingROUAsset := currentROUAsset
		
		// Calculate interest for this day
		interest := currentLiability * dailyRate
		
		// Check for payment on this day
		payment := 0.0
		for _, p := range input.Payments {
			if isSameDay(p.Date, currentDate) && p.Type != "variable" && p.Type != "non_lease" {
				payment += p.Amount
			}
		}
		
		// Update liability
		currentLiability = currentLiability + interest - payment
		
		// Depreciation
		depreciation := dailyDepreciation
		currentROUAsset = currentROUAsset - depreciation
		
		// Ensure we don't go below zero
		if currentROUAsset < 0.01 {
			currentROUAsset = 0
		}
		if currentLiability < 0.01 {
			currentLiability = 0
		}
		
		schedule = append(schedule, DailyEntry{
			Date:             currentDate,
			OpeningLiability: round(openingLiability),
			InterestExpense:  round(interest),
			Payment:          round(payment),
			ClosingLiability: round(currentLiability),
			OpeningROUAsset:  round(openingROUAsset),
			Depreciation:     round(depreciation),
			ClosingROUAsset:  round(currentROUAsset),
		})
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
		m.Depreciation += entry.Depreciation
		m.ClosingLiability = entry.ClosingLiability
		m.ClosingROUAsset = entry.ClosingROUAsset
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
