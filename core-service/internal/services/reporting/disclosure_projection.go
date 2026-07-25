package reporting

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/ifrs16"
)

// MaturityBandCount is the number of maturity bands the disclosure note reports:
// <=1y, 1-2y, 2-3y, 3-4y, 4-5y, >5y. Fine bands aggregate losslessly into the
// coarse "1-5y" IFRS 16 presentation and also satisfy the ASC 842 requirement to
// disclose each of the next five years separately.
const MaturityBandCount = 6

// MaturityRow is one contract's undiscounted lease commitment by maturity band,
// with the reconciliation to the discounted carrying liability. Keeping the row at
// contract level is what makes every disclosed figure drillable in the workpaper.
type MaturityRow struct {
	ContractID          string                     `json:"contract_id"`
	ContractNumber      string                     `json:"contract_number"`
	ContractName        string                     `json:"contract_name"`
	StoreName           string                     `json:"store_name,omitempty"`
	AssetType           string                     `json:"asset_type"`
	Currency            string                     `json:"currency"`
	LeaseEndDate        string                     `json:"lease_end_date"`
	DiscountRate        float64                    `json:"discount_rate"`
	Bands               [MaturityBandCount]float64 `json:"bands"`
	TotalUndiscounted   float64                    `json:"total_undiscounted"`
	CarryingLiability   float64                    `json:"carrying_liability"`
	UnearnedFinanceCost float64                    `json:"unearned_finance_cost"`
}

// ROUReconciliationRow is the right-of-use asset roll-forward for one asset class.
type ROUReconciliationRow struct {
	AssetType        string  `json:"asset_type"`
	ContractCount    int     `json:"contract_count"`
	Opening          float64 `json:"opening"`
	Additions        float64 `json:"additions"`
	Depreciation     float64 `json:"depreciation"`
	Remeasurement    float64 `json:"remeasurement"`
	Impairment       float64 `json:"impairment"`
	OtherAdjustments float64 `json:"other_adjustments"`
	Closing          float64 `json:"closing"`
}

// LiabilityRollforward is the lease liability roll-forward for the period.
type LiabilityRollforward struct {
	Opening          float64 `json:"opening"`
	Additions        float64 `json:"additions"`
	Interest         float64 `json:"interest"`
	Payments         float64 `json:"payments"`
	Remeasurement    float64 `json:"remeasurement"`
	OtherAdjustments float64 `json:"other_adjustments"`
	Closing          float64 `json:"closing"`
}

// ExpenseBreakdown decomposes lease-related profit or loss for the period.
type ExpenseBreakdown struct {
	Depreciation    float64 `json:"depreciation"`
	Interest        float64 `json:"interest"`
	ShortTermExempt float64 `json:"short_term_exempt"`
	LowValueExempt  float64 `json:"low_value_exempt"`
	VariableRent    float64 `json:"variable_rent"`
	NonLease        float64 `json:"non_lease"`
	Total           float64 `json:"total"`
}

// CashOutflowSummary is the total cash outflow for leases in the period (IFRS 16.53(g)).
type CashOutflowSummary struct {
	FixedPayments    float64 `json:"fixed_payments"`
	PrepaidPayments  float64 `json:"prepaid_payments"`
	VariablePayments float64 `json:"variable_payments"`
	NonLeasePayments float64 `json:"non_lease_payments"`
	Total            float64 `json:"total"`
}

// disclosureFact is one contract's computed state, shared across the five tables
// so the engine runs once per contract rather than once per table.
type disclosureFact struct {
	contract    *repository.Contract
	rate        float64
	payments    []ifrs16.LeasePayment
	calculation *ifrs16.CalculationResult
	adjustments []*repository.EventAdjustment
}

// projectDisclosure produces the IFRS 16 disclosure note package: maturity
// analysis, ROU reconciliation, liability roll-forward, expense breakdown, and
// total cash outflow. StartDate/EndDate bound the reporting period; EndDate is
// also the as-of date for the maturity analysis.
func projectDisclosure(snapshot *Snapshot, request ProjectionRequest) (ProjectionResult, error) {
	if request.EndDate.Before(request.StartDate) {
		return ProjectionResult{}, fmt.Errorf("end date must not be before start date")
	}
	asOf := request.EndDate

	facts := make([]disclosureFact, 0, len(snapshot.Contracts))
	currencies := make([]string, 0)
	seenCurrency := make(map[string]bool)
	skipped := 0

	for index := range snapshot.Contracts {
		fact := &snapshot.Contracts[index]
		contract := fact.Contract
		if ifrs16.NormalizeLeaseScope(contract.LeaseScope) == ifrs16.LeaseScopeNotALease {
			continue
		}
		payments := repository.ToIFRS16Payments(fact.PaymentSchedules)
		calculation, err := calculateContract(fact, payments, fact.DiscountRate)
		if err != nil {
			skipped++
			continue
		}
		if contract.Currency != "" && !seenCurrency[contract.Currency] {
			seenCurrency[contract.Currency] = true
			currencies = append(currencies, contract.Currency)
		}
		facts = append(facts, disclosureFact{
			contract:    contract,
			rate:        fact.DiscountRate,
			payments:    payments,
			calculation: calculation,
			adjustments: fact.EventAdjustments,
		})
	}
	sort.Strings(currencies)

	maturityRows, maturityTotals := buildMaturityAnalysis(facts, asOf)
	rouRows, rouTotals := buildROUReconciliation(facts, request.StartDate, request.EndDate)

	return ProjectionResult{Payload: projectionPayload(snapshot, nil, map[string]any{
		"period_start":          request.StartDate.Format("2006-01-02"),
		"period_end":            request.EndDate.Format("2006-01-02"),
		"as_of":                 asOf.Format("2006-01-02"),
		"band_labels":           []string{"<=1y", "1-2y", "2-3y", "3-4y", "4-5y", ">5y"},
		"currencies":            currencies,
		"multi_currency_caveat": len(currencies) > 1,
		"skipped_contracts":     skipped,
		"maturity_analysis":     map[string]any{"rows": maturityRows, "totals": maturityTotals},
		"rou_reconciliation":    map[string]any{"rows": rouRows, "totals": rouTotals},
		"liability_rollforward": buildLiabilityRollforward(facts, request.StartDate, request.EndDate),
		"expense_breakdown":     buildExpenseBreakdown(facts, request.StartDate, request.EndDate),
		"cash_outflow":          buildCashOutflow(facts, request.StartDate, request.EndDate),
	})}, nil
}

// maturityBandIndex assigns a payment date to a maturity band relative to asOf.
func maturityBandIndex(paymentDate, asOf time.Time) int {
	for years := 1; years <= 5; years++ {
		if !paymentDate.After(asOf.AddDate(years, 0, 0)) {
			return years - 1
		}
	}
	return MaturityBandCount - 1
}

// carryingAmountsAt returns the closing liability and ROU from the daily schedule
// as of the given date, i.e. the latest daily entry on or before it. Both are zero
// when the lease has not commenced by that date.
func carryingAmountsAt(entries []ifrs16.DailyEntry, date time.Time) (liability, rou float64) {
	for _, entry := range entries {
		if entry.Date.After(date) {
			break
		}
		liability = entry.ClosingLiability
		rou = entry.ClosingROUAsset
	}
	return liability, rou
}

func buildMaturityAnalysis(facts []disclosureFact, asOf time.Time) ([]MaturityRow, MaturityRow) {
	rows := make([]MaturityRow, 0, len(facts))
	totals := MaturityRow{ContractName: "total"}

	for _, fact := range facts {
		contract := fact.contract
		if fact.calculation.MeasurementBasis != "capitalized" {
			continue
		}
		if contract.CommencementDate.After(asOf) {
			// Not yet commenced: there is no recognized liability to analyze.
			continue
		}

		row := MaturityRow{
			ContractID:     contract.ID,
			ContractNumber: contract.ContractNumber,
			ContractName:   contract.ContractName,
			StoreName:      contract.StoreName,
			AssetType:      contract.AssetType,
			Currency:       contract.Currency,
			LeaseEndDate:   contract.LeaseEndDate.Format("2006-01-02"),
			DiscountRate:   fact.rate,
		}

		for _, payment := range fact.payments {
			if payment.Type == "variable" || payment.Type == "non_lease" {
				continue
			}
			if payment.Timing == "prepaid" && !payment.Date.After(contract.CommencementDate) {
				continue
			}
			if !payment.Date.After(asOf) {
				continue
			}
			row.Bands[maturityBandIndex(payment.Date, asOf)] += payment.Amount
			row.TotalUndiscounted += payment.Amount
		}

		liability, _ := carryingAmountsAt(fact.calculation.DailyAmortization, asOf)
		row.CarryingLiability = liability
		row.UnearnedFinanceCost = row.TotalUndiscounted - liability

		if row.TotalUndiscounted == 0 && row.CarryingLiability == 0 {
			continue
		}

		row.TotalUndiscounted = roundProjection(row.TotalUndiscounted)
		row.CarryingLiability = roundProjection(row.CarryingLiability)
		row.UnearnedFinanceCost = roundProjection(row.UnearnedFinanceCost)
		for band := range row.Bands {
			row.Bands[band] = roundProjection(row.Bands[band])
			totals.Bands[band] += row.Bands[band]
		}
		totals.TotalUndiscounted += row.TotalUndiscounted
		totals.CarryingLiability += row.CarryingLiability
		totals.UnearnedFinanceCost += row.UnearnedFinanceCost
		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].ContractNumber < rows[j].ContractNumber })

	totals.TotalUndiscounted = roundProjection(totals.TotalUndiscounted)
	totals.CarryingLiability = roundProjection(totals.CarryingLiability)
	totals.UnearnedFinanceCost = roundProjection(totals.UnearnedFinanceCost)
	for band := range totals.Bands {
		totals.Bands[band] = roundProjection(totals.Bands[band])
	}
	return rows, totals
}

func buildROUReconciliation(facts []disclosureFact, periodStart, periodEnd time.Time) ([]ROUReconciliationRow, ROUReconciliationRow) {
	byAssetType := make(map[string]*ROUReconciliationRow)
	dayBeforeStart := periodStart.AddDate(0, 0, -1)

	for _, fact := range facts {
		contract := fact.contract
		if fact.calculation.MeasurementBasis != "capitalized" {
			continue
		}

		_, opening := carryingAmountsAt(fact.calculation.DailyAmortization, dayBeforeStart)
		_, closing := carryingAmountsAt(fact.calculation.DailyAmortization, periodEnd)

		var additions float64
		if !contract.CommencementDate.Before(periodStart) && !contract.CommencementDate.After(periodEnd) {
			additions = fact.calculation.InitialROUAsset
		}

		var depreciation, engineAdjustment float64
		for _, entry := range fact.calculation.DailyAmortization {
			if entry.Date.Before(periodStart) || entry.Date.After(periodEnd) {
				continue
			}
			depreciation += entry.Depreciation
			engineAdjustment += entry.ROUAdjustment
		}

		var remeasurement, impairment float64
		for _, adjustment := range fact.adjustments {
			if adjustment.EffectiveDate.Before(periodStart) || adjustment.EffectiveDate.After(periodEnd) {
				continue
			}
			if adjustment.AdjustmentType == "impairment" {
				written := math.Abs(adjustment.ROUAdjustment)
				if written == 0 {
					written = adjustment.PnLLoss
				}
				impairment += written
				continue
			}
			remeasurement += adjustment.ROUAdjustment
		}

		if opening == 0 && closing == 0 && additions == 0 && depreciation == 0 && remeasurement == 0 && impairment == 0 {
			continue
		}

		assetType := contract.AssetType
		if assetType == "" {
			assetType = "real_estate"
		}
		row, exists := byAssetType[assetType]
		if !exists {
			row = &ROUReconciliationRow{AssetType: assetType}
			byAssetType[assetType] = row
		}
		row.ContractCount++
		row.Opening += opening
		row.Additions += additions
		row.Depreciation += depreciation
		row.Remeasurement += remeasurement
		row.Impairment += impairment
		row.OtherAdjustments += engineAdjustment
		row.Closing += closing
	}

	rows := make([]ROUReconciliationRow, 0, len(byAssetType))
	totals := ROUReconciliationRow{AssetType: "total"}
	for _, row := range byAssetType {
		row.Opening = roundProjection(row.Opening)
		row.Additions = roundProjection(row.Additions)
		row.Depreciation = roundProjection(row.Depreciation)
		row.Remeasurement = roundProjection(row.Remeasurement)
		row.Impairment = roundProjection(row.Impairment)
		row.OtherAdjustments = roundProjection(row.OtherAdjustments)
		row.Closing = roundProjection(row.Closing)
		rows = append(rows, *row)

		totals.ContractCount += row.ContractCount
		totals.Opening += row.Opening
		totals.Additions += row.Additions
		totals.Depreciation += row.Depreciation
		totals.Remeasurement += row.Remeasurement
		totals.Impairment += row.Impairment
		totals.OtherAdjustments += row.OtherAdjustments
		totals.Closing += row.Closing
	}
	sortROUReconciliationRows(rows)

	totals.Opening = roundProjection(totals.Opening)
	totals.Additions = roundProjection(totals.Additions)
	totals.Depreciation = roundProjection(totals.Depreciation)
	totals.Remeasurement = roundProjection(totals.Remeasurement)
	totals.Impairment = roundProjection(totals.Impairment)
	totals.OtherAdjustments = roundProjection(totals.OtherAdjustments)
	totals.Closing = roundProjection(totals.Closing)
	return rows, totals
}

// sortROUReconciliationRows presents asset classes in the order finance reports
// them, with any unrecognized class last.
func sortROUReconciliationRows(rows []ROUReconciliationRow) {
	presentation := map[string]int{"real_estate": 0, "vehicle": 1, "it_equipment": 2, "machinery": 3}
	rank := func(assetType string) int {
		if order, known := presentation[assetType]; known {
			return order
		}
		return len(presentation)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rank(rows[i].AssetType) != rank(rows[j].AssetType) {
			return rank(rows[i].AssetType) < rank(rows[j].AssetType)
		}
		return rows[i].AssetType < rows[j].AssetType
	})
}

func buildLiabilityRollforward(facts []disclosureFact, periodStart, periodEnd time.Time) LiabilityRollforward {
	var rollforward LiabilityRollforward
	dayBeforeStart := periodStart.AddDate(0, 0, -1)

	for _, fact := range facts {
		contract := fact.contract
		if fact.calculation.MeasurementBasis != "capitalized" {
			continue
		}

		opening, _ := carryingAmountsAt(fact.calculation.DailyAmortization, dayBeforeStart)
		closing, _ := carryingAmountsAt(fact.calculation.DailyAmortization, periodEnd)
		rollforward.Opening += opening
		rollforward.Closing += closing

		if !contract.CommencementDate.Before(periodStart) && !contract.CommencementDate.After(periodEnd) {
			rollforward.Additions += fact.calculation.InitialLiability
		}

		for _, entry := range fact.calculation.DailyAmortization {
			if entry.Date.Before(periodStart) || entry.Date.After(periodEnd) {
				continue
			}
			rollforward.Interest += entry.InterestExpense
			rollforward.Payments += entry.Payment
			rollforward.OtherAdjustments += entry.LiabilityAdjustment
		}

		for _, adjustment := range fact.adjustments {
			if adjustment.EffectiveDate.Before(periodStart) || adjustment.EffectiveDate.After(periodEnd) {
				continue
			}
			if adjustment.AdjustmentType != "impairment" {
				rollforward.Remeasurement += adjustment.LiabilityAdjustment
			}
		}
	}

	rollforward.Opening = roundProjection(rollforward.Opening)
	rollforward.Additions = roundProjection(rollforward.Additions)
	rollforward.Interest = roundProjection(rollforward.Interest)
	rollforward.Payments = roundProjection(rollforward.Payments)
	rollforward.Remeasurement = roundProjection(rollforward.Remeasurement)
	rollforward.OtherAdjustments = roundProjection(rollforward.OtherAdjustments)
	rollforward.Closing = roundProjection(rollforward.Closing)
	return rollforward
}

func buildExpenseBreakdown(facts []disclosureFact, periodStart, periodEnd time.Time) ExpenseBreakdown {
	var breakdown ExpenseBreakdown

	for _, fact := range facts {
		scope := ifrs16.NormalizeLeaseScope(fact.contract.LeaseScope)
		for _, entry := range fact.calculation.DailyAmortization {
			if entry.Date.Before(periodStart) || entry.Date.After(periodEnd) {
				continue
			}
			breakdown.Depreciation += entry.Depreciation
			breakdown.Interest += entry.InterestExpense
			breakdown.VariableRent += entry.VariableRentExpense
			breakdown.NonLease += entry.NonLeaseExpense
			switch scope {
			case ifrs16.LeaseScopeShortTermExempt:
				breakdown.ShortTermExempt += entry.ExemptLeaseExpense
			case ifrs16.LeaseScopeLowValueExempt:
				breakdown.LowValueExempt += entry.ExemptLeaseExpense
			}
		}
	}

	breakdown.Depreciation = roundProjection(breakdown.Depreciation)
	breakdown.Interest = roundProjection(breakdown.Interest)
	breakdown.ShortTermExempt = roundProjection(breakdown.ShortTermExempt)
	breakdown.LowValueExempt = roundProjection(breakdown.LowValueExempt)
	breakdown.VariableRent = roundProjection(breakdown.VariableRent)
	breakdown.NonLease = roundProjection(breakdown.NonLease)
	// Non-lease components are disclosed separately and stay out of the lease total.
	breakdown.Total = roundProjection(breakdown.Depreciation + breakdown.Interest +
		breakdown.ShortTermExempt + breakdown.LowValueExempt + breakdown.VariableRent)
	return breakdown
}

func buildCashOutflow(facts []disclosureFact, periodStart, periodEnd time.Time) CashOutflowSummary {
	var summary CashOutflowSummary

	for _, fact := range facts {
		for _, payment := range fact.payments {
			if payment.Date.Before(periodStart) || payment.Date.After(periodEnd) {
				continue
			}
			switch payment.Type {
			case "variable":
				summary.VariablePayments += payment.Amount
			case "non_lease":
				summary.NonLeasePayments += payment.Amount
			default:
				if payment.Timing == "prepaid" && !payment.Date.After(fact.contract.CommencementDate) {
					summary.PrepaidPayments += payment.Amount
				} else {
					summary.FixedPayments += payment.Amount
				}
			}
		}
	}

	summary.FixedPayments = roundProjection(summary.FixedPayments)
	summary.PrepaidPayments = roundProjection(summary.PrepaidPayments)
	summary.VariablePayments = roundProjection(summary.VariablePayments)
	summary.NonLeasePayments = roundProjection(summary.NonLeasePayments)
	summary.Total = roundProjection(summary.FixedPayments + summary.PrepaidPayments +
		summary.VariablePayments + summary.NonLeasePayments)
	return summary
}
