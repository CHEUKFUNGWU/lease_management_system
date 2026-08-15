package reporting

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/lease-management-system/core-service/internal/money"
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
	ContractID          string                          `json:"contract_id"`
	ContractNumber      string                          `json:"contract_number"`
	ContractName        string                          `json:"contract_name"`
	StoreName           string                          `json:"store_name,omitempty"`
	AssetType           string                          `json:"asset_type"`
	Currency            string                          `json:"currency"`
	LeaseEndDate        string                          `json:"lease_end_date"`
	DiscountRate        float64                         `json:"discount_rate"`
	Bands               [MaturityBandCount]money.Amount `json:"bands"`
	TotalUndiscounted   money.Amount                    `json:"total_undiscounted"`
	CarryingLiability   money.Amount                    `json:"carrying_liability"`
	UnearnedFinanceCost money.Amount                    `json:"unearned_finance_cost"`
}

// ROUReconciliationRow is the right-of-use asset roll-forward for one asset class.
type ROUReconciliationRow struct {
	AssetType        string       `json:"asset_type"`
	ContractCount    int          `json:"contract_count"`
	Opening          money.Amount `json:"opening"`
	Additions        money.Amount `json:"additions"`
	Depreciation     money.Amount `json:"depreciation"`
	Remeasurement    money.Amount `json:"remeasurement"`
	Impairment       money.Amount `json:"impairment"`
	OtherAdjustments money.Amount `json:"other_adjustments"`
	Closing          money.Amount `json:"closing"`
}

// LiabilityRollforward is the lease liability roll-forward for the period.
type LiabilityRollforward struct {
	Opening          money.Amount `json:"opening"`
	Additions        money.Amount `json:"additions"`
	Interest         money.Amount `json:"interest"`
	Payments         money.Amount `json:"payments"`
	Remeasurement    money.Amount `json:"remeasurement"`
	OtherAdjustments money.Amount `json:"other_adjustments"`
	Closing          money.Amount `json:"closing"`
}

// ExpenseBreakdown decomposes lease-related profit or loss for the period.
type ExpenseBreakdown struct {
	Depreciation    money.Amount `json:"depreciation"`
	Interest        money.Amount `json:"interest"`
	ShortTermExempt money.Amount `json:"short_term_exempt"`
	LowValueExempt  money.Amount `json:"low_value_exempt"`
	VariableRent    money.Amount `json:"variable_rent"`
	NonLease        money.Amount `json:"non_lease"`
	Total           money.Amount `json:"total"`
}

// CashOutflowSummary is the total cash outflow for leases in the period (IFRS 16.53(g)).
type CashOutflowSummary struct {
	FixedPayments    money.Amount `json:"fixed_payments"`
	PrepaidPayments  money.Amount `json:"prepaid_payments"`
	VariablePayments money.Amount `json:"variable_payments"`
	NonLeasePayments money.Amount `json:"non_lease_payments"`
	Total            money.Amount `json:"total"`
}

// AuditWorkpaperRow is the contract-level evidence row behind the aggregated
// disclosure sections. It is intentionally built from the same disclosureFact
// so the UI and an auditor never receive a second accounting calculation.
type AuditWorkpaperRow struct {
	ContractID                string       `json:"contract_id"`
	ContractNumber            string       `json:"contract_number"`
	ContractName              string       `json:"contract_name"`
	LegalEntityID             string       `json:"legal_entity_id,omitempty"`
	StoreName                 string       `json:"store_name,omitempty"`
	AssetType                 string       `json:"asset_type"`
	Currency                  string       `json:"currency"`
	LeaseScope                string       `json:"lease_scope"`
	ApprovalStatus            string       `json:"approval_status"`
	ReportMode                string       `json:"report_mode"`
	CommencementDate          string       `json:"commencement_date"`
	LeaseEndDate              string       `json:"lease_end_date"`
	DiscountRate              float64      `json:"discount_rate"`
	DiscountRateType          string       `json:"discount_rate_type,omitempty"`
	DiscountRateVersion       string       `json:"discount_rate_version,omitempty"`
	DiscountRateSource        string       `json:"discount_rate_source,omitempty"`
	DiscountRateConfirmedAt   string       `json:"discount_rate_confirmed_at,omitempty"`
	PaymentScheduleCount      int          `json:"payment_schedule_count"`
	EventAdjustmentCount      int          `json:"event_adjustment_count"`
	InitialLiability          money.Amount `json:"initial_liability"`
	InitialROUAsset           money.Amount `json:"initial_rou_asset"`
	OpeningLiability          money.Amount `json:"opening_liability"`
	Additions                 money.Amount `json:"additions"`
	Interest                  money.Amount `json:"interest"`
	Payments                  money.Amount `json:"payments"`
	LiabilityRemeasurement    money.Amount `json:"liability_remeasurement"`
	LiabilityOtherAdjustments money.Amount `json:"liability_other_adjustments"`
	ClosingLiability          money.Amount `json:"closing_liability"`
	OpeningROU                money.Amount `json:"opening_rou"`
	ROUAdditions              money.Amount `json:"rou_additions"`
	Depreciation              money.Amount `json:"depreciation"`
	ROURemeasurement          money.Amount `json:"rou_remeasurement"`
	Impairment                money.Amount `json:"impairment"`
	ROUOtherAdjustments       money.Amount `json:"rou_other_adjustments"`
	ClosingROU                money.Amount `json:"closing_rou"`
	LiabilityTieOut           money.Amount `json:"liability_tie_out"`
	ROUTieOut                 money.Amount `json:"rou_tie_out"`
}

type AuditWorkpaperTotals struct {
	RowCount         int `json:"row_count"`
	CapitalizedCount int `json:"capitalized_count"`
	ExemptCount      int `json:"exempt_count"`
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
		"report_basis": map[string]any{
			"snapshot_id": snapshot.ID, "policy_version": snapshot.PolicyVersion,
			"mode": snapshot.Mode, "is_official": snapshot.IsOfficial, "generated_at": snapshot.GeneratedAt,
			"approval_status": func() string {
				if snapshot.IsOfficial {
					return "approved"
				}
				return "mixed_working_statuses"
			}(),
			"period_start": request.StartDate.Format("2006-01-02"), "period_end": request.EndDate.Format("2006-01-02"),
			"as_of": asOf.Format("2006-01-02"), "population_count": len(snapshot.Contracts),
			"computed_contract_count": len(facts), "skipped_contract_count": skipped,
			"excluded_not_a_lease_count": countNotALease(snapshot),
			"approval_status_policy":     approvalStatusPolicy(snapshot.Mode),
		},
		"maturity_analysis":     map[string]any{"rows": maturityRows, "totals": maturityTotals},
		"rou_reconciliation":    map[string]any{"rows": rouRows, "totals": rouTotals},
		"liability_rollforward": buildLiabilityRollforward(facts, request.StartDate, request.EndDate),
		"expense_breakdown":     buildExpenseBreakdown(facts, request.StartDate, request.EndDate),
		"cash_outflow":          buildCashOutflow(facts, request.StartDate, request.EndDate),
		"audit_workpaper": func() map[string]any {
			rows, totals := buildAuditWorkpaper(facts, request.StartDate, request.EndDate, snapshot.Mode)
			return map[string]any{"rows": rows, "totals": totals}
		}(),
	})}, nil
}

func approvalStatusPolicy(mode Mode) string {
	if mode == Official {
		return "approved_only"
	}
	return "working_statuses"
}

func countNotALease(snapshot *Snapshot) int {
	count := 0
	for _, fact := range snapshot.Contracts {
		if ifrs16.NormalizeLeaseScope(fact.Contract.LeaseScope) == ifrs16.LeaseScopeNotALease {
			count++
		}
	}
	return count
}

func buildAuditWorkpaper(facts []disclosureFact, periodStart, periodEnd time.Time, mode Mode) ([]AuditWorkpaperRow, AuditWorkpaperTotals) {
	rows := make([]AuditWorkpaperRow, 0, len(facts))
	totals := AuditWorkpaperTotals{}
	for _, fact := range facts {
		contract := fact.contract
		row := AuditWorkpaperRow{
			ContractID: contract.ID, ContractNumber: contract.ContractNumber, ContractName: contract.ContractName,
			StoreName: contract.StoreName, AssetType: contract.AssetType, Currency: contract.Currency,
			LeaseScope: ifrs16.NormalizeLeaseScope(contract.LeaseScope), ApprovalStatus: contract.ApprovalStatus,
			ReportMode: string(mode), CommencementDate: contract.CommencementDate.Format("2006-01-02"),
			LeaseEndDate: contract.LeaseEndDate.Format("2006-01-02"), DiscountRate: roundRate(fact.rate),
			PaymentScheduleCount: len(fact.payments), EventAdjustmentCount: len(fact.adjustments),
			InitialLiability: fact.calculation.InitialLiability, InitialROUAsset: fact.calculation.InitialROUAsset,
		}
		if contract.LegalEntityID != nil {
			row.LegalEntityID = *contract.LegalEntityID
		}
		if contract.DiscountRateType != nil {
			row.DiscountRateType = *contract.DiscountRateType
		}
		if contract.DiscountRateVersion != nil {
			row.DiscountRateVersion = *contract.DiscountRateVersion
		}
		if contract.DiscountRateSource != nil {
			row.DiscountRateSource = *contract.DiscountRateSource
		}
		if contract.DiscountRateConfirmedAt != nil {
			row.DiscountRateConfirmedAt = contract.DiscountRateConfirmedAt.Format(time.RFC3339)
		}

		row.OpeningLiability, row.OpeningROU = carryingAmountsAt(fact.calculation.DailyAmortization, periodStart.AddDate(0, 0, -1))
		row.ClosingLiability, row.ClosingROU = carryingAmountsAt(fact.calculation.DailyAmortization, periodEnd)
		if !contract.CommencementDate.Before(periodStart) && !contract.CommencementDate.After(periodEnd) {
			row.Additions = fact.calculation.InitialLiability
			row.ROUAdditions = fact.calculation.InitialROUAsset
		}
		for _, entry := range fact.calculation.DailyAmortization {
			if entry.Date.Before(periodStart) || entry.Date.After(periodEnd) {
				continue
			}
			row.Interest = row.Interest.Add(entry.InterestExpense)
			row.Payments = row.Payments.Add(entry.Payment)
			row.LiabilityOtherAdjustments = row.LiabilityOtherAdjustments.Add(entry.LiabilityAdjustment)
			row.Depreciation = row.Depreciation.Add(entry.Depreciation)
			row.ROUOtherAdjustments = row.ROUOtherAdjustments.Add(entry.ROUAdjustment)
		}
		for _, adjustment := range fact.adjustments {
			if adjustment.EffectiveDate.Before(periodStart) || adjustment.EffectiveDate.After(periodEnd) {
				continue
			}
			// Event adjustment amounts are stored decimals read as float64;
			// the conversion happens once, at this seam.
			if adjustment.AdjustmentType == "impairment" {
				written := money.NewFromFloat(math.Abs(adjustment.ROUAdjustment))
				if written.IsZero() {
					written = money.NewFromFloat(adjustment.PnLLoss)
				}
				row.Impairment = row.Impairment.Add(written)
				continue
			}
			row.LiabilityRemeasurement = row.LiabilityRemeasurement.Add(money.NewFromFloat(adjustment.LiabilityAdjustment))
			row.ROURemeasurement = row.ROURemeasurement.Add(money.NewFromFloat(adjustment.ROUAdjustment))
		}
		row.LiabilityTieOut = row.OpeningLiability.Add(row.Additions).Add(row.Interest).Sub(row.Payments).
			Add(row.LiabilityRemeasurement).Add(row.LiabilityOtherAdjustments).Sub(row.ClosingLiability)
		row.ROUTieOut = row.OpeningROU.Add(row.ROUAdditions).Sub(row.Depreciation).
			Add(row.ROURemeasurement).Sub(row.Impairment).Add(row.ROUOtherAdjustments).Sub(row.ClosingROU)

		// The workpaper is what an auditor reads: every disclosed amount is
		// quantised once, at the contract currency's precision.
		round := func(amount money.Amount) money.Amount { return amount.Round(row.Currency) }
		row.InitialLiability = round(row.InitialLiability)
		row.InitialROUAsset = round(row.InitialROUAsset)
		row.OpeningLiability = round(row.OpeningLiability)
		row.Additions = round(row.Additions)
		row.Interest = round(row.Interest)
		row.Payments = round(row.Payments)
		row.LiabilityRemeasurement = round(row.LiabilityRemeasurement)
		row.LiabilityOtherAdjustments = round(row.LiabilityOtherAdjustments)
		row.ClosingLiability = round(row.ClosingLiability)
		row.OpeningROU = round(row.OpeningROU)
		row.ROUAdditions = round(row.ROUAdditions)
		row.Depreciation = round(row.Depreciation)
		row.ROURemeasurement = round(row.ROURemeasurement)
		row.Impairment = round(row.Impairment)
		row.ROUOtherAdjustments = round(row.ROUOtherAdjustments)
		row.ClosingROU = round(row.ClosingROU)
		rows = append(rows, row)
		totals.RowCount++
		if fact.calculation.MeasurementBasis == "capitalized" {
			totals.CapitalizedCount++
		} else {
			totals.ExemptCount++
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ContractNumber < rows[j].ContractNumber })
	return rows, totals
}

// MaturityBandLabels names the bands in the order they are indexed. The cash
// flow ladder and the disclosure note must agree on where a payment falls, so
// both read the boundaries from here rather than each defining their own.
var MaturityBandLabels = [MaturityBandCount]string{
	"1 年内", "1-2 年", "2-3 年", "3-4 年", "4-5 年", "5 年以上",
}

// MaturityBandIndex assigns a payment date to a maturity band relative to asOf.
func MaturityBandIndex(paymentDate, asOf time.Time) int {
	return maturityBandIndex(paymentDate, asOf)
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
func carryingAmountsAt(entries []ifrs16.DailyEntry, date time.Time) (liability, rou money.Amount) {
	liability = money.NewFromInt64(0)
	rou = money.NewFromInt64(0)
	for _, entry := range entries {
		if entry.Date.After(date) {
			break
		}
		liability = entry.ClosingLiability
		rou = entry.ClosingROUAsset
	}
	return liability, rou
}

// roundRate quantises the discount-rate display at the same two decimal places
// the pre-migration workpaper emitted. It is a rate, not a money amount; the
// rounding is display precision, preserved so the wire value does not change.
func roundRate(value float64) float64 {
	return math.Round(value*100) / 100
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
			row.Bands[maturityBandIndex(payment.Date, asOf)] = row.Bands[maturityBandIndex(payment.Date, asOf)].Add(payment.Amount)
			row.TotalUndiscounted = row.TotalUndiscounted.Add(payment.Amount)
		}

		liability, _ := carryingAmountsAt(fact.calculation.DailyAmortization, asOf)
		row.CarryingLiability = liability
		row.UnearnedFinanceCost = row.TotalUndiscounted.Sub(liability)

		if row.TotalUndiscounted.IsZero() && row.CarryingLiability.IsZero() {
			continue
		}

		// Each disclosed figure is quantised once, at the contract currency's
		// precision (the maturity analysis is per contract, so each row knows
		// its own currency).
		for band := range row.Bands {
			row.Bands[band] = row.Bands[band].Round(row.Currency)
			totals.Bands[band] = totals.Bands[band].Add(row.Bands[band])
		}
		row.TotalUndiscounted = row.TotalUndiscounted.Round(row.Currency)
		row.CarryingLiability = row.CarryingLiability.Round(row.Currency)
		row.UnearnedFinanceCost = row.UnearnedFinanceCost.Round(row.Currency)
		totals.TotalUndiscounted = totals.TotalUndiscounted.Add(row.TotalUndiscounted)
		totals.CarryingLiability = totals.CarryingLiability.Add(row.CarryingLiability)
		totals.UnearnedFinanceCost = totals.UnearnedFinanceCost.Add(row.UnearnedFinanceCost)
		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].ContractNumber < rows[j].ContractNumber })

	// The totals row spans contracts that may use different currencies; it
	// quantises at the default two-decimal scale, as the pre-migration report
	// did.
	totals.TotalUndiscounted = totals.TotalUndiscounted.RoundTo(2)
	totals.CarryingLiability = totals.CarryingLiability.RoundTo(2)
	totals.UnearnedFinanceCost = totals.UnearnedFinanceCost.RoundTo(2)
	for band := range totals.Bands {
		totals.Bands[band] = totals.Bands[band].RoundTo(2)
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

		var additions money.Amount
		if !contract.CommencementDate.Before(periodStart) && !contract.CommencementDate.After(periodEnd) {
			additions = fact.calculation.InitialROUAsset
		}

		var depreciation, engineAdjustment money.Amount
		for _, entry := range fact.calculation.DailyAmortization {
			if entry.Date.Before(periodStart) || entry.Date.After(periodEnd) {
				continue
			}
			depreciation = depreciation.Add(entry.Depreciation)
			engineAdjustment = engineAdjustment.Add(entry.ROUAdjustment)
		}

		var remeasurement, impairment money.Amount
		for _, adjustment := range fact.adjustments {
			if adjustment.EffectiveDate.Before(periodStart) || adjustment.EffectiveDate.After(periodEnd) {
				continue
			}
			// Event adjustment amounts are stored decimals read as float64;
			// the conversion happens once, at this seam.
			if adjustment.AdjustmentType == "impairment" {
				written := money.NewFromFloat(math.Abs(adjustment.ROUAdjustment))
				if written.IsZero() {
					written = money.NewFromFloat(adjustment.PnLLoss)
				}
				impairment = impairment.Add(written)
				continue
			}
			remeasurement = remeasurement.Add(money.NewFromFloat(adjustment.ROUAdjustment))
		}

		if opening.IsZero() && closing.IsZero() && additions.IsZero() && depreciation.IsZero() && remeasurement.IsZero() && impairment.IsZero() {
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
		row.Opening = row.Opening.Add(opening)
		row.Additions = row.Additions.Add(additions)
		row.Depreciation = row.Depreciation.Add(depreciation)
		row.Remeasurement = row.Remeasurement.Add(remeasurement)
		row.Impairment = row.Impairment.Add(impairment)
		row.OtherAdjustments = row.OtherAdjustments.Add(engineAdjustment)
		row.Closing = row.Closing.Add(closing)
	}

	rows := make([]ROUReconciliationRow, 0, len(byAssetType))
	totals := ROUReconciliationRow{AssetType: "total"}
	for _, row := range byAssetType {
		// An asset-class row may span contracts in different currencies; it
		// quantises at the default two-decimal scale, as the pre-migration
		// report did.
		row.Opening = row.Opening.RoundTo(2)
		row.Additions = row.Additions.RoundTo(2)
		row.Depreciation = row.Depreciation.RoundTo(2)
		row.Remeasurement = row.Remeasurement.RoundTo(2)
		row.Impairment = row.Impairment.RoundTo(2)
		row.OtherAdjustments = row.OtherAdjustments.RoundTo(2)
		row.Closing = row.Closing.RoundTo(2)
		rows = append(rows, *row)

		totals.ContractCount += row.ContractCount
		totals.Opening = totals.Opening.Add(row.Opening)
		totals.Additions = totals.Additions.Add(row.Additions)
		totals.Depreciation = totals.Depreciation.Add(row.Depreciation)
		totals.Remeasurement = totals.Remeasurement.Add(row.Remeasurement)
		totals.Impairment = totals.Impairment.Add(row.Impairment)
		totals.OtherAdjustments = totals.OtherAdjustments.Add(row.OtherAdjustments)
		totals.Closing = totals.Closing.Add(row.Closing)
	}
	sortROUReconciliationRows(rows)

	totals.Opening = totals.Opening.RoundTo(2)
	totals.Additions = totals.Additions.RoundTo(2)
	totals.Depreciation = totals.Depreciation.RoundTo(2)
	totals.Remeasurement = totals.Remeasurement.RoundTo(2)
	totals.Impairment = totals.Impairment.RoundTo(2)
	totals.OtherAdjustments = totals.OtherAdjustments.RoundTo(2)
	totals.Closing = totals.Closing.RoundTo(2)
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
		rollforward.Opening = rollforward.Opening.Add(opening)
		rollforward.Closing = rollforward.Closing.Add(closing)

		if !contract.CommencementDate.Before(periodStart) && !contract.CommencementDate.After(periodEnd) {
			rollforward.Additions = rollforward.Additions.Add(fact.calculation.InitialLiability)
		}

		for _, entry := range fact.calculation.DailyAmortization {
			if entry.Date.Before(periodStart) || entry.Date.After(periodEnd) {
				continue
			}
			rollforward.Interest = rollforward.Interest.Add(entry.InterestExpense)
			rollforward.Payments = rollforward.Payments.Add(entry.Payment)
			rollforward.OtherAdjustments = rollforward.OtherAdjustments.Add(entry.LiabilityAdjustment)
		}

		for _, adjustment := range fact.adjustments {
			if adjustment.EffectiveDate.Before(periodStart) || adjustment.EffectiveDate.After(periodEnd) {
				continue
			}
			if adjustment.AdjustmentType != "impairment" {
				// Event adjustment amounts are stored decimals read as float64;
				// the conversion happens once, at this seam.
				rollforward.Remeasurement = rollforward.Remeasurement.Add(money.NewFromFloat(adjustment.LiabilityAdjustment))
			}
		}
	}

	// The roll-forward spans contracts that may use different currencies; it
	// quantises at the default two-decimal scale, as the pre-migration report
	// did.
	rollforward.Opening = rollforward.Opening.RoundTo(2)
	rollforward.Additions = rollforward.Additions.RoundTo(2)
	rollforward.Interest = rollforward.Interest.RoundTo(2)
	rollforward.Payments = rollforward.Payments.RoundTo(2)
	rollforward.Remeasurement = rollforward.Remeasurement.RoundTo(2)
	rollforward.OtherAdjustments = rollforward.OtherAdjustments.RoundTo(2)
	rollforward.Closing = rollforward.Closing.RoundTo(2)
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
			breakdown.Depreciation = breakdown.Depreciation.Add(entry.Depreciation)
			breakdown.Interest = breakdown.Interest.Add(entry.InterestExpense)
			breakdown.VariableRent = breakdown.VariableRent.Add(entry.VariableRentExpense)
			breakdown.NonLease = breakdown.NonLease.Add(entry.NonLeaseExpense)
			switch scope {
			case ifrs16.LeaseScopeShortTermExempt:
				breakdown.ShortTermExempt = breakdown.ShortTermExempt.Add(entry.ExemptLeaseExpense)
			case ifrs16.LeaseScopeLowValueExempt:
				breakdown.LowValueExempt = breakdown.LowValueExempt.Add(entry.ExemptLeaseExpense)
			}
		}
	}

	// The breakdown spans contracts that may use different currencies; it
	// quantises at the default two-decimal scale, as the pre-migration report
	// did.
	breakdown.Depreciation = breakdown.Depreciation.RoundTo(2)
	breakdown.Interest = breakdown.Interest.RoundTo(2)
	breakdown.ShortTermExempt = breakdown.ShortTermExempt.RoundTo(2)
	breakdown.LowValueExempt = breakdown.LowValueExempt.RoundTo(2)
	breakdown.VariableRent = breakdown.VariableRent.RoundTo(2)
	breakdown.NonLease = breakdown.NonLease.RoundTo(2)
	// Non-lease components are disclosed separately and stay out of the lease total.
	breakdown.Total = breakdown.Depreciation.Add(breakdown.Interest).
		Add(breakdown.ShortTermExempt).Add(breakdown.LowValueExempt).Add(breakdown.VariableRent).RoundTo(2)
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
				summary.VariablePayments = summary.VariablePayments.Add(payment.Amount)
			case "non_lease":
				summary.NonLeasePayments = summary.NonLeasePayments.Add(payment.Amount)
			default:
				if payment.Timing == "prepaid" && !payment.Date.After(fact.contract.CommencementDate) {
					summary.PrepaidPayments = summary.PrepaidPayments.Add(payment.Amount)
				} else {
					summary.FixedPayments = summary.FixedPayments.Add(payment.Amount)
				}
			}
		}
	}

	// The outflow spans contracts that may use different currencies; it
	// quantises at the default two-decimal scale, as the pre-migration report
	// did.
	summary.FixedPayments = summary.FixedPayments.RoundTo(2)
	summary.PrepaidPayments = summary.PrepaidPayments.RoundTo(2)
	summary.VariablePayments = summary.VariablePayments.RoundTo(2)
	summary.NonLeasePayments = summary.NonLeasePayments.RoundTo(2)
	summary.Total = summary.FixedPayments.Add(summary.PrepaidPayments).
		Add(summary.VariablePayments).Add(summary.NonLeasePayments).RoundTo(2)
	return summary
}
