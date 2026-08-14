package reporting

import (
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/money"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/ifrs16"
	"github.com/shopspring/decimal"
)

// monthlyRentSchedules builds `months` postpaid fixed payments of `amount`,
// each due on the day before the anniversary of commencement.
func monthlyRentSchedules(commencement time.Time, months int, amount float64, timing string) []*repository.PaymentSchedule {
	schedules := make([]*repository.PaymentSchedule, 0, months)
	for month := 1; month <= months; month++ {
		due := commencement.AddDate(0, month, 0).AddDate(0, 0, -1)
		if timing == "prepaid" {
			due = commencement.AddDate(0, month-1, 0)
		}
		schedules = append(schedules, &repository.PaymentSchedule{
			DueDate: due, Amount: amount, PaymentTiming: timing, IsFixed: true,
		})
	}
	return schedules
}

// capitalizedLeaseSnapshot is a 36-month in-scope lease at 10,000/month, 5% IBR.
func capitalizedLeaseSnapshot() *Snapshot {
	commencement := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	return &Snapshot{
		ID: "snapshot-disclosure", PolicyVersion: policyVersion, Mode: Working,
		GeneratedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Contracts: []ContractFact{{
			Contract: &repository.Contract{
				ID: "contract-1", ContractNumber: "LC-001", ContractName: "旗舰店租约",
				StoreName: "南京东路旗舰店", AssetType: "real_estate", Currency: "CNY",
				CommencementDate: commencement,
				LeaseEndDate:     time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC),
				LeaseScope:       ifrs16.LeaseScopeInScope,
			},
			PaymentSchedules: monthlyRentSchedules(commencement, 36, 10000, "postpaid"),
			DiscountRate:     0.05,
		}},
	}
}

func disclosurePayload(t *testing.T, snapshot *Snapshot, start, end time.Time) map[string]any {
	t.Helper()
	result, err := Project(snapshot, ProjectionRequest{
		Kind: KindDisclosure, StartDate: start, EndDate: end,
	})
	if err != nil {
		t.Fatalf("project disclosure: %v", err)
	}
	return result.Payload
}

func TestDisclosureMaturityBandsReconcileToCarryingLiability(t *testing.T) {
	asOf := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	payload := disclosurePayload(t, capitalizedLeaseSnapshot(),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), asOf)

	analysis := payload["maturity_analysis"].(map[string]any)
	rows := analysis["rows"].([]MaturityRow)
	if len(rows) != 1 {
		t.Fatalf("maturity rows = %d, want 1", len(rows))
	}
	row := rows[0]

	// 24 of 36 payments remain after 2025-12-31.
	if !row.TotalUndiscounted.Equal(money.NewFromInt64(240000)) {
		t.Errorf("total undiscounted = %v, want 240000", row.TotalUndiscounted)
	}
	// 12 fall within one year, 12 in the 1-2y band, none beyond.
	if !row.Bands[0].Equal(money.NewFromInt64(120000)) || !row.Bands[1].Equal(money.NewFromInt64(120000)) {
		t.Errorf("bands = %v, want [120000 120000 0 0 0 0]", row.Bands)
	}
	var bandSum money.Amount
	for _, band := range row.Bands {
		bandSum = bandSum.Add(band)
	}
	if !bandSum.Equal(row.TotalUndiscounted) {
		t.Errorf("band sum %v != total %v", bandSum, row.TotalUndiscounted)
	}

	// The disclosure only holds up if undiscounted - unearned finance cost ties
	// back to the carrying liability the ledger reports.
	if !row.TotalUndiscounted.Sub(row.UnearnedFinanceCost).Equal(row.CarryingLiability) {
		t.Errorf("reconciliation does not tie: %v - %v != %v",
			row.TotalUndiscounted, row.UnearnedFinanceCost, row.CarryingLiability)
	}
	if row.CarryingLiability.IsZero() || row.CarryingLiability.Decimal().IsNegative() || row.CarryingLiability.Cmp(row.TotalUndiscounted) >= 0 {
		t.Errorf("carrying liability %v outside (0, %v)", row.CarryingLiability, row.TotalUndiscounted)
	}

	totals := analysis["totals"].(MaturityRow)
	if !totals.TotalUndiscounted.Equal(row.TotalUndiscounted) {
		t.Errorf("totals %v != single row %v", totals.TotalUndiscounted, row.TotalUndiscounted)
	}
}

func TestDisclosureLiabilityRollforwardTies(t *testing.T) {
	payload := disclosurePayload(t, capitalizedLeaseSnapshot(),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC))

	roll := payload["liability_rollforward"].(LiabilityRollforward)
	if !roll.Opening.IsZero() {
		t.Errorf("opening = %v, want 0 for a lease commencing in period", roll.Opening)
	}
	if !roll.Additions.Decimal().IsPositive() {
		t.Errorf("additions = %v, want > 0", roll.Additions)
	}
	derived := roll.Opening.Add(roll.Additions).Add(roll.Interest).Sub(roll.Payments).
		Add(roll.Remeasurement).Add(roll.OtherAdjustments)
	// The daily schedule quantises each entry at emission, so the sum of the
	// rounded daily fields can sit a small spread from the closing carry. The
	// spread is sub-currency-unit by construction; anything larger is a real
	// break of the roll-forward.
	if derived.Sub(roll.Closing).Abs().Cmp(money.NewFromInt64(1)) > 0 {
		t.Errorf("rollforward does not tie: derived %v vs closing %v", derived, roll.Closing)
	}
}

func TestDisclosureROUReconciliationTies(t *testing.T) {
	payload := disclosurePayload(t, capitalizedLeaseSnapshot(),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC))

	reconciliation := payload["rou_reconciliation"].(map[string]any)
	rows := reconciliation["rows"].([]ROUReconciliationRow)
	if len(rows) != 1 || rows[0].AssetType != "real_estate" {
		t.Fatalf("rou rows = %#v", rows)
	}
	row := rows[0]
	derived := row.Opening.Add(row.Additions).Sub(row.Depreciation).Add(row.Remeasurement).
		Sub(row.Impairment).Add(row.OtherAdjustments)
	// See the roll-forward tie: per-entry quantisation leaves a sub-currency-
	// unit spread between summed entries and the closing carry.
	if derived.Sub(row.Closing).Abs().Cmp(money.NewFromInt64(1)) > 0 {
		t.Errorf("ROU reconciliation does not tie: derived %v vs closing %v", derived, row.Closing)
	}
	if totals := reconciliation["totals"].(ROUReconciliationRow); !totals.Closing.Equal(row.Closing) {
		t.Errorf("totals closing %v != row closing %v", totals.Closing, row.Closing)
	}
}

func TestDisclosureExpenseAndCashOutflow(t *testing.T) {
	payload := disclosurePayload(t, capitalizedLeaseSnapshot(),
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC))

	expenses := payload["expense_breakdown"].(ExpenseBreakdown)
	if !expenses.Depreciation.Decimal().IsPositive() || !expenses.Interest.Decimal().IsPositive() {
		t.Errorf("depreciation %v / interest %v, want both > 0", expenses.Depreciation, expenses.Interest)
	}
	if !expenses.ShortTermExempt.IsZero() || !expenses.LowValueExempt.IsZero() {
		t.Errorf("an in-scope lease must not report exempt expense: %#v", expenses)
	}

	cash := payload["cash_outflow"].(CashOutflowSummary)
	if !cash.FixedPayments.Equal(money.NewFromInt64(120000)) || !cash.Total.Equal(money.NewFromInt64(120000)) {
		t.Errorf("cash outflow = %#v, want 120000 fixed", cash)
	}
}

func TestDisclosureIncludesReportBasisAndContractAuditWorkpaper(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	payload := disclosurePayload(t, capitalizedLeaseSnapshot(), start, end)

	basis, ok := payload["report_basis"].(map[string]any)
	if !ok {
		t.Fatalf("report basis type = %T", payload["report_basis"])
	}
	if basis["population_count"] != 1 || basis["computed_contract_count"] != 1 || basis["approval_status_policy"] != "working_statuses" {
		t.Fatalf("report basis = %#v", basis)
	}
	workpaper, ok := payload["audit_workpaper"].(map[string]any)
	if !ok {
		t.Fatalf("audit workpaper type = %T", payload["audit_workpaper"])
	}
	rows, ok := workpaper["rows"].([]AuditWorkpaperRow)
	if !ok || len(rows) != 1 {
		t.Fatalf("audit workpaper rows = %#v", workpaper["rows"])
	}
	row := rows[0]
	if row.ContractNumber != "LC-001" || row.PaymentScheduleCount != 36 || row.DiscountRate != 0.05 {
		t.Fatalf("audit workpaper row = %#v", row)
	}
	if row.LiabilityTieOut.Abs().Cmp(money.NewFromInt64(1)) > 0 || row.ROUTieOut.Abs().Cmp(money.NewFromInt64(1)) > 0 {
		t.Fatalf("audit workpaper tie-outs = liability %v rou %v", row.LiabilityTieOut, row.ROUTieOut)
	}
}

func TestDisclosureClassifiesExemptLeasesOutsideTheBalanceSheet(t *testing.T) {
	commencement := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	snapshot := &Snapshot{
		ID: "snapshot-exempt", PolicyVersion: policyVersion, Mode: Working,
		GeneratedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Contracts: []ContractFact{{
			Contract: &repository.Contract{
				ID: "contract-2", ContractNumber: "LC-002", ContractName: "短期仓库",
				AssetType: "real_estate", Currency: "CNY",
				CommencementDate: commencement,
				LeaseEndDate:     time.Date(2025, 9, 1, 0, 0, 0, 0, time.UTC),
				LeaseScope:       ifrs16.LeaseScopeShortTermExempt,
			},
			PaymentSchedules: monthlyRentSchedules(commencement, 6, 3000, "postpaid"),
			DiscountRate:     0.05,
		}},
	}

	payload := disclosurePayload(t, snapshot,
		time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC))

	expenses := payload["expense_breakdown"].(ExpenseBreakdown)
	// The straight-line daily expense is quantised per day, so the six-month
	// sum can sit a cent or two either side of the nominal 18,000.
	if expenses.ShortTermExempt.Sub(money.NewFromInt64(18000)).Abs().Decimal().Cmp(decimal.NewFromFloat(1)) > 0 {
		t.Errorf("short-term exempt expense = %v, want ~18000", expenses.ShortTermExempt)
	}
	if !expenses.Depreciation.IsZero() || !expenses.Interest.IsZero() {
		t.Errorf("an exempt lease must not produce depreciation or interest: %#v", expenses)
	}

	// Exempt leases carry no liability, so they stay out of the maturity analysis.
	analysis := payload["maturity_analysis"].(map[string]any)
	if rows := analysis["rows"].([]MaturityRow); len(rows) != 0 {
		t.Errorf("exempt lease appeared in maturity analysis: %#v", rows)
	}
}
