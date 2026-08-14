package reporting

import (
	"math"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/money"
	"github.com/lease-management-system/core-service/internal/repository"
)

func TestProjectCashflowClassifiesAndBucketsSnapshotPayments(t *testing.T) {
	tax := 5.0
	dueDate := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	snapshot := &Snapshot{
		ID: "snapshot-1", PolicyVersion: "report-snapshot-v1", Mode: Official,
		IsOfficial: true, GeneratedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Contracts: []ContractFact{{
			Contract: &repository.Contract{
				ID: "contract-1", ContractNumber: "LC-001", ContractName: "Flagship",
				StoreName: "Central", Currency: "CNY",
			},
			PaymentSchedules: []*repository.PaymentSchedule{
				{DueDate: dueDate, Amount: 100, IsFixed: true, TaxAmount: &tax},
				{DueDate: dueDate, Amount: 20, IsVariable: true},
			},
		}},
	}

	result, err := Project(snapshot, ProjectionRequest{
		Kind: KindCashflow, View: ViewSummary, Granularity: GranularityMonth,
		StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("project cashflow: %v", err)
	}

	rows, ok := result.Payload["data"].([]CashflowRow)
	if !ok || len(rows) != 1 {
		t.Fatalf("cashflow rows = %#v", result.Payload["data"])
	}
	row := rows[0]
	if row.GroupKey != "summary" || row.PeriodKey != "2026-01" ||
		!row.FixedRent.Equal(money.NewFromInt64(100)) || !row.VariableRent.Equal(money.NewFromInt64(20)) ||
		!row.TaxAmount.Equal(money.NewFromInt64(5)) || !row.TotalCashOut.Equal(money.NewFromInt64(125)) || row.PaymentCount != 2 {
		t.Fatalf("cashflow row = %#v", row)
	}
	if result.Payload["snapshot_id"] != "snapshot-1" || result.Payload["total"] != 1 {
		t.Fatalf("projection metadata = %#v", result.Payload)
	}
}

func TestProjectLiabilityRollingReusesSnapshotRowsForJSONAndCSV(t *testing.T) {
	legalEntityID := "entity-1"
	snapshot := &Snapshot{
		ID: "snapshot-2", PolicyVersion: "report-snapshot-v1", Mode: Official,
		IsOfficial: true, GeneratedAt: time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC),
		Contracts: []ContractFact{{Contract: &repository.Contract{
			ID: "contract-1", ContractNumber: "LC-001", ContractName: "Flagship",
			LegalEntityID: &legalEntityID, ApprovalStatus: "approved", IsOfficialVersion: true,
			ReportMode: "official", Currency: "CNY",
			CommencementDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			LeaseEndDate:     time.Date(2028, 12, 31, 0, 0, 0, 0, time.UTC),
		}}},
	}

	jsonResult, err := Project(snapshot, ProjectionRequest{Kind: KindLiabilityRolling})
	if err != nil {
		t.Fatalf("project liability JSON: %v", err)
	}
	rows, ok := jsonResult.Payload["data"].([]LiabilityRollingRow)
	if !ok || len(rows) != 1 || rows[0].ContractNumber != "LC-001" || rows[0].CommencementDate != "2026-01-01" {
		t.Fatalf("liability rows = %#v", jsonResult.Payload["data"])
	}

	csvResult, err := Project(snapshot, ProjectionRequest{Kind: KindLiabilityRolling, Format: FormatCSV, Language: "en"})
	if err != nil {
		t.Fatalf("project liability CSV: %v", err)
	}
	if csvResult.CSV == nil || csvResult.CSV.Filename != "IFRS16_LiabilityRolling_OFFICIAL_20260203_040506.csv" {
		t.Fatalf("CSV metadata = %#v", csvResult.CSV)
	}
	if len(csvResult.CSV.Records) != 2 || csvResult.CSV.Records[1][0] != "LC-001" || csvResult.CSV.Records[1][5] != "entity-1" {
		t.Fatalf("CSV records = %#v", csvResult.CSV.Records)
	}
}

func TestProjectPortfolioAppliesDefaultsAndAccountingClassification(t *testing.T) {
	snapshot := &Snapshot{
		GeneratedAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		Contracts: []ContractFact{{
			Contract: &repository.Contract{
				ID: "contract-1", ApprovalStatus: "approved", DiscountRateMissing: true,
				CommencementDate: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				LeaseEndDate:     time.Date(2027, 12, 31, 0, 0, 0, 0, time.UTC),
			},
			PaymentSchedules: []*repository.PaymentSchedule{
				{Amount: 100, IsFixed: true, IsLeaseComponent: true},
				{Amount: 20, IsVariable: true},
				{Amount: 5, IsNonLeaseComponent: true},
			},
		}},
	}

	result, err := Project(snapshot, ProjectionRequest{Kind: KindPortfolio})
	if err != nil {
		t.Fatalf("project portfolio: %v", err)
	}
	rows, ok := result.Payload["data"].([]PortfolioRow)
	if !ok || len(rows) != 1 {
		t.Fatalf("portfolio rows = %#v", result.Payload["data"])
	}
	row := rows[0]
	if row.AssetType != "" || row.LeaseScope != "" || row.Currency != "" || row.ActiveContractCount != 1 || row.MissingDiscountRateCount != 1 ||
		!row.FixedLeaseCommitment.Equal(money.NewFromInt64(100)) || !row.VariableRentExposure.Equal(money.NewFromInt64(20)) ||
		!row.NonLeaseComponentAmount.Equal(money.NewFromInt64(5)) || row.PaymentCount != 3 {
		t.Fatalf("portfolio row = %#v", row)
	}
}

func TestProjectSensitivityUsesControlledRateAndIFRSCalculation(t *testing.T) {
	commencement := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	snapshot := &Snapshot{
		GeneratedAt: commencement,
		Contracts: []ContractFact{{
			Contract: &repository.Contract{
				ID: "contract-1", ContractNumber: "LC-001", ContractName: "Flagship",
				Currency: "CNY", LeaseScope: "in_scope", CommencementDate: commencement,
				LeaseEndDate: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
			},
			DiscountRate: 0.05,
			PaymentSchedules: []*repository.PaymentSchedule{
				{DueDate: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), Amount: 1200, IsFixed: true, IsLeaseComponent: true, IncludedInLiabilityPV: true, PaymentTiming: "postpaid"},
			},
		}},
	}

	result, err := Project(snapshot, ProjectionRequest{
		Kind: KindSensitivity, ContractID: "contract-1", Shocks: []float64{0, 0.01},
	})
	if err != nil {
		t.Fatalf("project sensitivity: %v", err)
	}
	rows, ok := result.Payload["data"].([]SensitivityRow)
	if !ok || len(rows) != 2 {
		t.Fatalf("sensitivity rows = %#v", result.Payload["data"])
	}
	if result.Payload["base_rate"] != 0.05 || rows[0].DiscountRate != 0.05 || !rows[0].LiabilityDelta.IsZero() ||
		math.Abs(rows[1].DiscountRate-0.06) > 0.0000001 ||
		rows[1].InitialLiability.Cmp(rows[0].InitialLiability) >= 0 || !rows[1].LiabilityDelta.Decimal().IsNegative() {
		t.Fatalf("sensitivity projection = %#v payload=%#v", rows, result.Payload)
	}
}

func TestProjectStandardComparisonPreservesExemptLeaseMeasurement(t *testing.T) {
	commencement := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	snapshot := &Snapshot{
		GeneratedAt: commencement,
		Contracts: []ContractFact{{
			Contract: &repository.Contract{
				ID: "contract-1", ContractNumber: "LC-001", ContractName: "Short lease",
				LeaseScope: "short_term_exempt", CommencementDate: commencement,
				LeaseEndDate: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
			},
			DiscountRate: 0.05,
			PaymentSchedules: []*repository.PaymentSchedule{
				{DueDate: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), Amount: 1200, IsFixed: true, IsLeaseComponent: true, IncludedInLiabilityPV: true, PaymentTiming: "postpaid"},
			},
		}},
	}

	result, err := Project(snapshot, ProjectionRequest{Kind: KindStandardComparison, ContractID: "contract-1"})
	if err != nil {
		t.Fatalf("project standards: %v", err)
	}
	rows, ok := result.Payload["data"].([]StandardComparisonRow)
	if !ok || len(rows) != 4 {
		t.Fatalf("standard rows = %#v", result.Payload["data"])
	}
	for _, row := range rows {
		if row.MeasurementBasis != "straight_line_expense" || !row.InitialLiability.IsZero() || !row.InitialROUAsset.IsZero() ||
			!row.FirstPeriodExpense.Decimal().IsPositive() || !row.TotalRecognizedCost.Equal(money.NewFromInt64(1200)) {
			t.Fatalf("exempt standard row = %#v", row)
		}
	}
}

func TestProjectAmortizationCombinesIFRSRowsAndSnapshotAdjustments(t *testing.T) {
	commencement := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	snapshot := &Snapshot{
		GeneratedAt: commencement,
		Contracts: []ContractFact{{
			Contract: &repository.Contract{
				ID: "contract-1", ContractNumber: "LC-001", ContractName: "Flagship",
				Currency: "CNY", LeaseScope: "in_scope", CommencementDate: commencement,
				LeaseEndDate: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC),
			},
			DiscountRate: 0.05,
			PaymentSchedules: []*repository.PaymentSchedule{
				{DueDate: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), Amount: 1200, IsFixed: true, IsLeaseComponent: true, IncludedInLiabilityPV: true, PaymentTiming: "postpaid"},
			},
			EventAdjustments: []*repository.EventAdjustment{
				{EffectiveDate: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC), AdjustmentType: "impairment", ROUAdjustment: -25, PnLLoss: 25},
			},
		}},
	}

	result, err := Project(snapshot, ProjectionRequest{
		Kind: KindAmortization, View: ViewSummary, Granularity: GranularityMonth,
		StartDate: commencement, EndDate: time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
		ReportCurrency: "USD", ExchangeRate: 2,
	})
	if err != nil {
		t.Fatalf("project amortization: %v", err)
	}
	rows, ok := result.Payload["data"].([]AmortizationRow)
	if !ok || len(rows) != 1 {
		t.Fatalf("amortization rows = %#v", result.Payload["data"])
	}
	if rows[0].Currency != "USD" || !rows[0].Impairment.Equal(money.NewFromInt64(50)) ||
		!rows[0].PnLAdjustment.Equal(money.NewFromInt64(-50)) || !rows[0].OpeningLiability.Decimal().IsPositive() {
		t.Fatalf("amortization row = %#v", rows[0])
	}
}

func TestProjectTagsNormalizesAndSummarizesContracts(t *testing.T) {
	snapshot := &Snapshot{Contracts: []ContractFact{
		{Contract: &repository.Contract{ID: "c1", ContractNumber: "LC-001", ContractName: "One", Tags: "beta, alpha,alpha"}},
		{Contract: &repository.Contract{ID: "c2", ContractNumber: "LC-002", ContractName: "Two", Tags: "alpha；gamma"}},
		{Contract: &repository.Contract{ID: "c3", ContractNumber: "LC-003", ContractName: "Three"}},
	}}

	tagsResult, err := Project(snapshot, ProjectionRequest{Kind: KindTags})
	if err != nil {
		t.Fatalf("project tags: %v", err)
	}
	tags, ok := tagsResult.Payload["data"].([]string)
	if !ok || len(tags) != 3 || tags[0] != "alpha" || tags[1] != "beta" || tags[2] != "gamma" {
		t.Fatalf("tags = %#v", tagsResult.Payload["data"])
	}

	summaryResult, err := Project(snapshot, ProjectionRequest{Kind: KindTagSummary})
	if err != nil {
		t.Fatalf("project tag summary: %v", err)
	}
	rows, ok := summaryResult.Payload["data"].([]TagSummaryRow)
	if !ok || len(rows) != 3 || rows[0].Tag != "alpha" || rows[0].ContractCount != 2 || len(rows[0].ContractIDs) != 2 {
		t.Fatalf("tag summary = %#v", summaryResult.Payload["data"])
	}
}

func TestProjectContractSummaryCountsPendingWorkflowStates(t *testing.T) {
	snapshot := &Snapshot{Contracts: []ContractFact{
		{Contract: &repository.Contract{ApprovalStatus: "approved"}},
		{Contract: &repository.Contract{ApprovalStatus: "draft"}},
		{Contract: &repository.Contract{ApprovalStatus: "submitted"}},
		{Contract: &repository.Contract{ApprovalStatus: "reviewed"}},
	}}

	result, err := Project(snapshot, ProjectionRequest{Kind: KindContractSummary})
	if err != nil {
		t.Fatalf("project contract summary: %v", err)
	}
	if result.Payload["total_contracts"] != 4 || result.Payload["approved_count"] != 1 || result.Payload["draft_count"] != 1 || result.Payload["pending_count"] != 2 {
		t.Fatalf("contract summary = %#v", result.Payload)
	}
}
