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

// AuditWorkpaperRow is the contract-level evidence row behind the aggregated
// disclosure sections. It is intentionally built from the same disclosureFact
// so the UI and an auditor never receive a second accounting calculation.
type AuditWorkpaperRow struct {
	ContractID                string  `json:"contract_id"`
	ContractNumber            string  `json:"contract_number"`
	ContractName              string  `json:"contract_name"`
	LegalEntityID             string  `json:"legal_entity_id,omitempty"`
	StoreName                 string  `json:"store_name,omitempty"`
	AssetType                 string  `json:"asset_type"`
	Currency                  string  `json:"currency"`
	LeaseScope                string  `json:"lease_scope"`
	ApprovalStatus            string  `json:"approval_status"`
	ReportMode                string  `json:"report_mode"`
	CommencementDate          string  `json:"commencement_date"`
	LeaseEndDate              string  `json:"lease_end_date"`
	DiscountRate              float64 `json:"discount_rate"`
	DiscountRateType          string  `json:"discount_rate_type,omitempty"`
	DiscountRateVersion       string  `json:"discount_rate_version,omitempty"`
	DiscountRateSource        string  `json:"discount_rate_source,omitempty"`
	DiscountRateConfirmedAt   string  `json:"discount_rate_confirmed_at,omitempty"`
	PaymentScheduleCount      int     `json:"payment_schedule_count"`
	EventAdjustmentCount      int     `json:"event_adjustment_count"`
	InitialLiability          float64 `json:"initial_liability"`
	InitialROUAsset           float64 `json:"initial_rou_asset"`
	OpeningLiability          float64 `json:"opening_liability"`
	Additions                 float64 `json:"additions"`
	Interest                  float64 `json:"interest"`
	Payments                  float64 `json:"payments"`
	LiabilityRemeasurement    float64 `json:"liability_remeasurement"`
	LiabilityOtherAdjustments float64 `json:"liability_other_adjustments"`
	ClosingLiability          float64 `json:"closing_liability"`
	OpeningROU                float64 `json:"opening_rou"`
	ROUAdditions              float64 `json:"rou_additions"`
	Depreciation              float64 `json:"depreciation"`
	ROURemeasurement          float64 `json:"rou_remeasurement"`
	Impairment                float64 `json:"impairment"`
	ROUOtherAdjustments       float64 `json:"rou_other_adjustments"`
	ClosingROU                float64 `json:"closing_rou"`
	LiabilityTieOut           float64 `json:"liability_tie_out"`
	ROUTieOut                 float64 `json:"rou_tie_out"`
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
			LeaseEndDate: contract.LeaseEndDate.Format("2006-01-02"), DiscountRate: roundProjection(fact.rate),
			PaymentScheduleCount: len(fact.payments), EventAdjustmentCount: len(fact.adjustments),
			InitialLiability: roundProjection(fact.calculation.InitialLiability.Float64()), InitialROUAsset: roundProjection(fact.calculation.InitialROUAsset.Float64()),
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
			row.Additions = fact.calculation.InitialLiability.Float64()
			row.ROUAdditions = fact.calculation.InitialROUAsset.Float64()
		}
		for _, entry := range fact.calculation.DailyAmortization {
			if entry.Date.Before(periodStart) || entry.Date.After(periodEnd) {
				continue
			}
			row.Interest += entry.InterestExpense.Float64()
			row.Payments += entry.Payment.Float64()
			row.LiabilityOtherAdjustments += entry.LiabilityAdjustment.Float64()
			row.Depreciation += entry.Depreciation.Float64()
			row.ROUOtherAdjustments += entry.ROUAdjustment.Float64()
		}
		for _, adjustment := range fact.adjustments {
			if adjustment.EffectiveDate.Before(periodStart) || adjustment.EffectiveDate.After(periodEnd) {
				continue
			}
			if adjustment.AdjustmentType == "impairment" {
				written := math.Abs(adjustment.ROUAdjustment)
				if written == 0 {
					written = adjustment.PnLLoss
				}
				row.Impairment += written
				continue
			}
			row.LiabilityRemeasurement += adjustment.LiabilityAdjustment
			row.ROURemeasurement += adjustment.ROUAdjustment
		}
		row.LiabilityTieOut = roundProjection(row.OpeningLiability + row.Additions + row.Interest - row.Payments + row.LiabilityRemeasurement + row.LiabilityOtherAdjustments - row.ClosingLiability)
		row.ROUTieOut = roundProjection(row.OpeningROU + row.ROUAdditions - row.Depreciation + row.ROURemeasurement - row.Impairment + row.ROUOtherAdjustments - row.ClosingROU)
		row.InitialLiability = roundProjection(row.InitialLiability)
		row.InitialROUAsset = roundProjection(row.InitialROUAsset)
		row.OpeningLiability = roundProjection(row.OpeningLiability)
		row.Additions = roundProjection(row.Additions)
		row.Interest = roundProjection(row.Interest)
		row.Payments = roundProjection(row.Payments)
		row.LiabilityRemeasurement = roundProjection(row.LiabilityRemeasurement)
		row.LiabilityOtherAdjustments = roundProjection(row.LiabilityOtherAdjustments)
		row.ClosingLiability = roundProjection(row.ClosingLiability)
		row.OpeningROU = roundProjection(row.OpeningROU)
		row.ROUAdditions = roundProjection(row.ROUAdditions)
		row.Depreciation = roundProjection(row.Depreciation)
		row.ROURemeasurement = roundProjection(row.ROURemeasurement)
		row.Impairment = roundProjection(row.Impairment)
		row.ROUOtherAdjustments = roundProjection(row.ROUOtherAdjustments)
		row.ClosingROU = roundProjection(row.ClosingROU)
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
func carryingAmountsAt(entries []ifrs16.DailyEntry, date time.Time) (liability, rou float64) {
	for _, entry := range entries {
		if entry.Date.After(date) {
			break
		}
		liability = entry.ClosingLiability.Float64()
		rou = entry.ClosingROUAsset.Float64()
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
			row.Bands[maturityBandIndex(payment.Date, asOf)] += payment.Amount.Float64()
			row.TotalUndiscounted += payment.Amount.Float64()
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
			additions = fact.calculation.InitialROUAsset.Float64()
		}

		var depreciation, engineAdjustment float64
		for _, entry := range fact.calculation.DailyAmortization {
			if entry.Date.Before(periodStart) || entry.Date.After(periodEnd) {
				continue
			}
			depreciation += entry.Depreciation.Float64()
			engineAdjustment += entry.ROUAdjustment.Float64()
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
			rollforward.Additions += fact.calculation.InitialLiability.Float64()
		}

		for _, entry := range fact.calculation.DailyAmortization {
			if entry.Date.Before(periodStart) || entry.Date.After(periodEnd) {
				continue
			}
			rollforward.Interest += entry.InterestExpense.Float64()
			rollforward.Payments += entry.Payment.Float64()
			rollforward.OtherAdjustments += entry.LiabilityAdjustment.Float64()
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
			breakdown.Depreciation += entry.Depreciation.Float64()
			breakdown.Interest += entry.InterestExpense.Float64()
			breakdown.VariableRent += entry.VariableRentExpense.Float64()
			breakdown.NonLease += entry.NonLeaseExpense.Float64()
			switch scope {
			case ifrs16.LeaseScopeShortTermExempt:
				breakdown.ShortTermExempt += entry.ExemptLeaseExpense.Float64()
			case ifrs16.LeaseScopeLowValueExempt:
				breakdown.LowValueExempt += entry.ExemptLeaseExpense.Float64()
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
				summary.VariablePayments += payment.Amount.Float64()
			case "non_lease":
				summary.NonLeasePayments += payment.Amount.Float64()
			default:
				if payment.Timing == "prepaid" && !payment.Date.After(fact.contract.CommencementDate) {
					summary.PrepaidPayments += payment.Amount.Float64()
				} else {
					summary.FixedPayments += payment.Amount.Float64()
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
