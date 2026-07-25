package reporting

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/repository"
	contractsvc "github.com/lease-management-system/core-service/internal/services/contracts"
	"github.com/lease-management-system/core-service/internal/services/ifrs16"
)

type ProjectionKind string

const (
	KindLiabilityRolling   ProjectionKind = "liability_rolling"
	KindContractSummary    ProjectionKind = "contract_summary"
	KindPortfolio          ProjectionKind = "portfolio"
	KindSensitivity        ProjectionKind = "sensitivity"
	KindStandardComparison ProjectionKind = "standard_comparison"
	KindAmortization       ProjectionKind = "amortization"
	KindTags               ProjectionKind = "tags"
	KindTagSummary         ProjectionKind = "tag_summary"
	KindCashflow           ProjectionKind = "cashflow"
	KindDisclosure         ProjectionKind = "disclosure"
)

var (
	ErrContractNotFound         = errors.New("contract not found")
	ErrPaymentSchedulesRequired = errors.New("payment schedules are required")
)

type ProjectionFormat string

const (
	FormatJSON ProjectionFormat = "json"
	FormatCSV  ProjectionFormat = "csv"
)

const (
	ViewContract = "contract"
	ViewStore    = "store"
	ViewSummary  = "summary"
	ViewTag      = "tag"
)

const (
	GranularityDay      = "day"
	GranularityMonth    = "month"
	GranularityQuarter  = "quarter"
	GranularityHalfYear = "half_year"
	GranularityYear     = "year"
)

type ProjectionRequest struct {
	Kind           ProjectionKind
	View           string
	Granularity    string
	StartDate      time.Time
	EndDate        time.Time
	ContractID     string
	Store          string
	Tags           []string
	Format         ProjectionFormat
	Language       string
	Rate           *float64
	Shocks         []float64
	ReportCurrency string
	ExchangeRate   float64
}

type ProjectionResult struct {
	Payload map[string]any
	CSV     *CSVProjection
}

type CSVProjection struct {
	Filename string
	Records  [][]string
}

type LiabilityRollingRow struct {
	ContractID          string  `json:"contract_id"`
	ContractNumber      string  `json:"contract_number"`
	ContractName        string  `json:"contract_name"`
	ApprovalStatus      string  `json:"approval_status"`
	IsOfficialVersion   bool    `json:"is_official_version"`
	ReportMode          string  `json:"report_mode"`
	CommencementDate    string  `json:"commencement_date"`
	LeaseEndDate        string  `json:"lease_end_date"`
	Currency            string  `json:"currency"`
	DiscountRateType    *string `json:"discount_rate_type"`
	DiscountRateMissing bool    `json:"discount_rate_missing"`
}

type CashflowRow struct {
	GroupKey        string  `json:"group_key"`
	GroupLabel      string  `json:"group_label"`
	ContractID      string  `json:"contract_id,omitempty"`
	ContractNumber  string  `json:"contract_number,omitempty"`
	ContractName    string  `json:"contract_name,omitempty"`
	StoreName       string  `json:"store_name,omitempty"`
	Currency        string  `json:"currency,omitempty"`
	PeriodKey       string  `json:"period_key"`
	PeriodStart     string  `json:"period_start"`
	PeriodEnd       string  `json:"period_end"`
	FixedRent       float64 `json:"fixed_rent"`
	VariableRent    float64 `json:"variable_rent"`
	NonLeaseExpense float64 `json:"non_lease_expense"`
	TaxAmount       float64 `json:"tax_amount"`
	TotalCashOut    float64 `json:"total_cash_out"`
	PaymentCount    int     `json:"payment_count"`
}

type PortfolioRow struct {
	AssetType                string  `json:"asset_type"`
	LeaseScope               string  `json:"lease_scope"`
	Currency                 string  `json:"currency"`
	ContractCount            int     `json:"contract_count"`
	ApprovedCount            int     `json:"approved_count"`
	ActiveContractCount      int     `json:"active_contract_count"`
	MissingDiscountRateCount int     `json:"missing_discount_rate_count"`
	FixedLeaseCommitment     float64 `json:"fixed_lease_commitment"`
	VariableRentExposure     float64 `json:"variable_rent_exposure"`
	NonLeaseComponentAmount  float64 `json:"non_lease_component_amount"`
	PaymentCount             int     `json:"payment_count"`
	EarliestCommencementDate string  `json:"earliest_commencement_date,omitempty"`
	LatestLeaseEndDate       string  `json:"latest_lease_end_date,omitempty"`
}

type SensitivityRow struct {
	ScenarioName          string  `json:"scenario_name"`
	DiscountRate          float64 `json:"discount_rate"`
	RateDelta             float64 `json:"rate_delta"`
	InitialLiability      float64 `json:"initial_liability"`
	InitialROUAsset       float64 `json:"initial_rou_asset"`
	LiabilityDelta        float64 `json:"liability_delta"`
	LiabilityDeltaPercent float64 `json:"liability_delta_percent"`
}

type StandardComparisonRow struct {
	Standard              string   `json:"standard"`
	StandardName          string   `json:"standard_name"`
	Classification        string   `json:"classification"`
	MeasurementBasis      string   `json:"measurement_basis"`
	InitialLiability      float64  `json:"initial_liability"`
	InitialROUAsset       float64  `json:"initial_rou_asset"`
	FirstPeriodExpense    float64  `json:"first_period_expense"`
	TotalRecognizedCost   float64  `json:"total_recognized_cost"`
	BalanceSheetTreatment string   `json:"balance_sheet_treatment"`
	PnLPattern            string   `json:"pnl_pattern"`
	KeyDifferences        []string `json:"key_differences"`
}

type TagSummaryRow struct {
	Tag             string   `json:"tag"`
	ContractCount   int      `json:"contract_count"`
	ContractIDs     []string `json:"contract_ids"`
	ContractNumbers []string `json:"contract_numbers"`
	ContractNames   []string `json:"contract_names"`
}

func Project(snapshot *Snapshot, request ProjectionRequest) (ProjectionResult, error) {
	if snapshot == nil {
		return ProjectionResult{}, fmt.Errorf("report snapshot is required")
	}
	switch request.Kind {
	case KindLiabilityRolling:
		return projectLiabilityRolling(snapshot, request), nil
	case KindContractSummary:
		return projectContractSummary(snapshot), nil
	case KindPortfolio:
		return projectPortfolio(snapshot), nil
	case KindSensitivity:
		return projectSensitivity(snapshot, request)
	case KindStandardComparison:
		return projectStandardComparison(snapshot, request)
	case KindAmortization:
		return projectAmortization(snapshot, request)
	case KindTags:
		return projectTags(snapshot), nil
	case KindTagSummary:
		return projectTagSummary(snapshot), nil
	case KindCashflow:
		return projectCashflow(snapshot, request)
	case KindDisclosure:
		return projectDisclosure(snapshot, request)
	default:
		return ProjectionResult{}, fmt.Errorf("unsupported report projection %q", request.Kind)
	}
}

func projectContractSummary(snapshot *Snapshot) ProjectionResult {
	approved, draft := 0, 0
	for _, fact := range snapshot.Contracts {
		switch fact.Contract.ApprovalStatus {
		case "approved":
			approved++
		case "draft":
			draft++
		}
	}
	total := len(snapshot.Contracts)
	payload := projectionPayload(snapshot, nil, map[string]any{
		"total_contracts": total, "approved_count": approved,
		"draft_count": draft, "pending_count": total - approved - draft,
	})
	delete(payload, "data")
	return ProjectionResult{Payload: payload}
}

func projectTags(snapshot *Snapshot) ProjectionResult {
	tagSet := make(map[string]bool)
	for _, fact := range snapshot.Contracts {
		for _, tag := range splitProjectionTags(fact.Contract.Tags) {
			if tag != "未打标签" {
				tagSet[tag] = true
			}
		}
	}
	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return ProjectionResult{Payload: projectionPayload(snapshot, tags, nil)}
}

func projectTagSummary(snapshot *Snapshot) ProjectionResult {
	tagContracts := make(map[string]map[string]*repository.Contract)
	for _, fact := range snapshot.Contracts {
		contract := fact.Contract
		for _, tag := range splitProjectionTags(contract.Tags) {
			if tag == "未打标签" {
				continue
			}
			if tagContracts[tag] == nil {
				tagContracts[tag] = make(map[string]*repository.Contract)
			}
			tagContracts[tag][contract.ID] = contract
		}
	}
	rows := make([]TagSummaryRow, 0, len(tagContracts))
	for tag, contracts := range tagContracts {
		row := TagSummaryRow{Tag: tag, ContractCount: len(contracts)}
		for _, contract := range contracts {
			row.ContractIDs = append(row.ContractIDs, contract.ID)
			row.ContractNumbers = append(row.ContractNumbers, contract.ContractNumber)
			row.ContractNames = append(row.ContractNames, contract.ContractName)
		}
		sort.Strings(row.ContractIDs)
		sort.Strings(row.ContractNumbers)
		sort.Strings(row.ContractNames)
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ContractCount != rows[j].ContractCount {
			return rows[i].ContractCount > rows[j].ContractCount
		}
		return rows[i].Tag < rows[j].Tag
	})
	return ProjectionResult{Payload: projectionPayload(snapshot, rows, nil)}
}

func projectStandardComparison(snapshot *Snapshot, request ProjectionRequest) (ProjectionResult, error) {
	fact := findContractFact(snapshot, request.ContractID)
	if fact == nil {
		return ProjectionResult{}, ErrContractNotFound
	}
	if len(fact.PaymentSchedules) == 0 {
		return ProjectionResult{}, fmt.Errorf("%w for standard comparison", ErrPaymentSchedulesRequired)
	}
	discountRate := fact.DiscountRate
	if request.Rate != nil && *request.Rate > 0 {
		discountRate = *request.Rate
	}
	if discountRate <= 0 {
		return ProjectionResult{}, contractsvc.ErrDiscountRateRequired
	}
	payments := repository.ToIFRS16Payments(fact.PaymentSchedules)
	calculation, err := calculateContract(fact, payments, discountRate)
	if err != nil {
		return ProjectionResult{}, err
	}
	financeExpense, totalCapitalizedCost := summarizeCapitalizedCost(calculation)
	straightLineExpense, totalFixedCost := summarizeStraightLineCost(calculation, payments)
	rows := []StandardComparisonRow{
		{
			Standard: "ifrs16", StandardName: "IFRS 16", Classification: "single lessee model",
			MeasurementBasis: calculation.MeasurementBasis, InitialLiability: calculation.InitialLiability,
			InitialROUAsset: calculation.InitialROUAsset, FirstPeriodExpense: financeExpense,
			TotalRecognizedCost:   totalCapitalizedCost,
			BalanceSheetTreatment: "Recognize lease liability and right-of-use asset unless exempt or not a lease.",
			PnLPattern:            "Front-loaded finance cost plus straight-line depreciation for capitalized leases.",
			KeyDifferences: []string{
				"Single lessee accounting model for in-scope leases.",
				"Short-term and low-value exemptions are supported when elected and documented.",
				"Variable and non-lease components remain outside lease liability measurement.",
			},
		},
		{
			Standard: "asc842_finance", StandardName: "ASC 842 - Finance Lease View", Classification: "finance lease",
			MeasurementBasis: calculation.MeasurementBasis, InitialLiability: calculation.InitialLiability,
			InitialROUAsset: calculation.InitialROUAsset, FirstPeriodExpense: financeExpense,
			TotalRecognizedCost:   totalCapitalizedCost,
			BalanceSheetTreatment: "Recognize lease liability and ROU asset; classification remains finance lease.",
			PnLPattern:            "Interest and amortization are presented separately; expense pattern is generally front-loaded.",
			KeyDifferences: []string{
				"ASC 842 keeps finance and operating lease classification for lessees.",
				"Finance lease economics are directionally aligned with IFRS 16 in this comparison.",
				"Presentation and disclosure differ from IFRS 16 even when measurements are close.",
			},
		},
		{
			Standard: "asc842_operating", StandardName: "ASC 842 - Operating Lease View", Classification: "operating lease",
			MeasurementBasis: calculation.MeasurementBasis, InitialLiability: calculation.InitialLiability,
			InitialROUAsset: calculation.InitialROUAsset, FirstPeriodExpense: straightLineExpense,
			TotalRecognizedCost:   totalFixedCost,
			BalanceSheetTreatment: "Recognize lease liability and ROU asset for most operating leases.",
			PnLPattern:            "Single lease cost is generally recognized on a straight-line basis.",
			KeyDifferences: []string{
				"Operating lease P&L pattern differs from IFRS 16 finance-cost plus depreciation pattern.",
				"ASC 842 does not provide an IFRS-style low-value asset exemption.",
				"This row is a policy comparison view, not a substitute for a full ASC 842 subledger.",
			},
		},
		{
			Standard: "cas21", StandardName: "中国企业会计准则第21号 - 租赁", Classification: "new lease standard lessee model",
			MeasurementBasis: calculation.MeasurementBasis, InitialLiability: calculation.InitialLiability,
			InitialROUAsset: calculation.InitialROUAsset, FirstPeriodExpense: financeExpense,
			TotalRecognizedCost:   totalCapitalizedCost,
			BalanceSheetTreatment: "Recognize lease liability and right-of-use asset unless exempt or outside lease scope.",
			PnLPattern:            "Generally aligned with IFRS 16 style lessee accounting for in-scope leases.",
			KeyDifferences: []string{
				"Useful for local reporting bridge where company policy maps CAS 21 to IFRS 16 controls.",
				"Chart of accounts, tax, and statutory presentation may differ from group IFRS reporting.",
				"Use the IFRS engine result as the controlled baseline, then document local-policy adjustments.",
			},
		},
	}
	if calculation.MeasurementBasis == "straight_line_expense" {
		for index := range rows {
			rows[index].FirstPeriodExpense = straightLineExpense
			rows[index].TotalRecognizedCost = totalFixedCost
			rows[index].InitialLiability = 0
			rows[index].InitialROUAsset = 0
		}
	}
	if calculation.MeasurementBasis == "skipped" {
		for index := range rows {
			rows[index].FirstPeriodExpense = 0
			rows[index].TotalRecognizedCost = 0
			rows[index].InitialLiability = 0
			rows[index].InitialROUAsset = 0
		}
	}
	contract := fact.Contract
	return ProjectionResult{Payload: projectionPayload(snapshot, rows, map[string]any{
		"contract_id": contract.ID, "contract_number": contract.ContractNumber,
		"contract_name": contract.ContractName, "lease_scope": contract.LeaseScope,
		"discount_rate": discountRate, "currency": contract.Currency,
	})}, nil
}

func summarizeCapitalizedCost(result *ifrs16.CalculationResult) (float64, float64) {
	firstPeriodExpense, totalCost := 0.0, 0.0
	for index, row := range result.MonthlySummary {
		periodCost := row.InterestExpense + row.Depreciation + row.VariableRentExpense + row.NonLeaseExpense
		if index == 0 {
			firstPeriodExpense = periodCost
		}
		totalCost += periodCost
	}
	return firstPeriodExpense, totalCost
}

func summarizeStraightLineCost(result *ifrs16.CalculationResult, payments []ifrs16.LeasePayment) (float64, float64) {
	totalCost := 0.0
	for _, payment := range payments {
		if payment.Type == "fixed" || payment.Type == "" {
			totalCost += payment.Amount
		}
	}
	firstPeriodExpense := 0.0
	if len(result.MonthlySummary) > 0 {
		firstPeriodExpense = totalCost / float64(len(result.MonthlySummary))
	}
	return roundProjection(firstPeriodExpense), roundProjection(totalCost)
}

func roundProjection(value float64) float64 {
	return math.Round(value*100) / 100
}

func projectSensitivity(snapshot *Snapshot, request ProjectionRequest) (ProjectionResult, error) {
	fact := findContractFact(snapshot, request.ContractID)
	if fact == nil {
		return ProjectionResult{}, ErrContractNotFound
	}
	if len(fact.PaymentSchedules) == 0 {
		return ProjectionResult{}, fmt.Errorf("%w for sensitivity analysis", ErrPaymentSchedulesRequired)
	}
	baseRate := fact.DiscountRate
	if request.Rate != nil && *request.Rate > 0 {
		baseRate = *request.Rate
	}
	if baseRate <= 0 {
		return ProjectionResult{}, contractsvc.ErrDiscountRateRequired
	}
	shocks := request.Shocks
	if len(shocks) == 0 {
		shocks = []float64{-0.01, -0.005, 0, 0.005, 0.01}
	}
	payments := repository.ToIFRS16Payments(fact.PaymentSchedules)
	rows := make([]SensitivityRow, 0, len(shocks))
	baseLiability := 0.0
	for _, shock := range shocks {
		rate := baseRate + shock
		if rate < 0 {
			rate = 0
		}
		result, err := calculateContract(fact, payments, rate)
		if err != nil {
			return ProjectionResult{}, err
		}
		if math.Abs(shock) < 0.0000001 {
			baseLiability = result.InitialLiability
		}
		rows = append(rows, SensitivityRow{
			ScenarioName: fmt.Sprintf("%+.2f%%", shock*100), DiscountRate: rate, RateDelta: shock,
			InitialLiability: result.InitialLiability, InitialROUAsset: result.InitialROUAsset,
		})
	}
	if baseLiability == 0 && len(rows) > 0 {
		baseLiability = rows[0].InitialLiability
	}
	for index := range rows {
		rows[index].LiabilityDelta = rows[index].InitialLiability - baseLiability
		if baseLiability != 0 {
			rows[index].LiabilityDeltaPercent = rows[index].LiabilityDelta / baseLiability
		}
	}
	contract := fact.Contract
	return ProjectionResult{Payload: projectionPayload(snapshot, rows, map[string]any{
		"contract_id": contract.ID, "contract_number": contract.ContractNumber,
		"contract_name": contract.ContractName, "base_rate": baseRate,
		"lease_scope": contract.LeaseScope, "currency": contract.Currency,
	})}, nil
}

func findContractFact(snapshot *Snapshot, contractID string) *ContractFact {
	for index := range snapshot.Contracts {
		if snapshot.Contracts[index].Contract.ID == contractID {
			return &snapshot.Contracts[index]
		}
	}
	return nil
}

func calculateContract(fact *ContractFact, payments []ifrs16.LeasePayment, rate float64) (*ifrs16.CalculationResult, error) {
	return ifrs16.Calculate(ifrs16.LeaseCalculation{
		CommencementDate: fact.Contract.CommencementDate,
		LeaseEndDate:     fact.Contract.LeaseEndDate,
		LeaseScope:       fact.Contract.LeaseScope,
		DiscountRate:     rate,
		Payments:         payments,
		PrepaidRent: ifrs16.CalculatePrepaidRent(ifrs16.LeaseCalculation{
			CommencementDate: fact.Contract.CommencementDate,
			Payments:         payments,
		}),
	})
}

func projectPortfolio(snapshot *Snapshot) ProjectionResult {
	rowsByKey := make(map[string]*PortfolioRow)
	for _, fact := range snapshot.Contracts {
		contract := fact.Contract
		assetType := contract.AssetType
		if assetType == "" {
			assetType = "real_estate"
		}
		leaseScope := contract.LeaseScope
		if leaseScope == "" {
			leaseScope = "in_scope"
		}
		currency := contract.Currency
		if currency == "" {
			currency = "CNY"
		}
		key := assetType + "|" + leaseScope + "|" + currency
		row := rowsByKey[key]
		if row == nil {
			row = &PortfolioRow{AssetType: assetType, LeaseScope: leaseScope, Currency: currency}
			rowsByKey[key] = row
		}
		row.ContractCount++
		if contract.ApprovalStatus == "approved" {
			row.ApprovedCount++
		}
		if !contract.LeaseEndDate.Before(snapshot.GeneratedAt) {
			row.ActiveContractCount++
		}
		if contract.DiscountRateMissing {
			row.MissingDiscountRateCount++
		}
		commencement := contract.CommencementDate.Format("2006-01-02")
		if row.EarliestCommencementDate == "" || commencement < row.EarliestCommencementDate {
			row.EarliestCommencementDate = commencement
		}
		leaseEnd := contract.LeaseEndDate.Format("2006-01-02")
		if row.LatestLeaseEndDate == "" || leaseEnd > row.LatestLeaseEndDate {
			row.LatestLeaseEndDate = leaseEnd
		}
		for _, schedule := range fact.PaymentSchedules {
			row.PaymentCount++
			switch {
			case schedule.IsVariable:
				row.VariableRentExposure += schedule.Amount
			case schedule.IsNonLeaseComponent:
				row.NonLeaseComponentAmount += schedule.Amount
			case schedule.IsLeaseComponent && schedule.IsFixed:
				row.FixedLeaseCommitment += schedule.Amount
			}
		}
	}
	rows := make([]PortfolioRow, 0, len(rowsByKey))
	for _, row := range rowsByKey {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].AssetType != rows[j].AssetType {
			return rows[i].AssetType < rows[j].AssetType
		}
		if rows[i].LeaseScope != rows[j].LeaseScope {
			return rows[i].LeaseScope < rows[j].LeaseScope
		}
		return rows[i].Currency < rows[j].Currency
	})
	return ProjectionResult{Payload: projectionPayload(snapshot, rows, nil)}
}

func projectLiabilityRolling(snapshot *Snapshot, request ProjectionRequest) ProjectionResult {
	rows := make([]LiabilityRollingRow, 0, len(snapshot.Contracts))
	for _, fact := range snapshot.Contracts {
		contract := fact.Contract
		rows = append(rows, LiabilityRollingRow{
			ContractID: contract.ID, ContractNumber: contract.ContractNumber, ContractName: contract.ContractName,
			ApprovalStatus: contract.ApprovalStatus, IsOfficialVersion: contract.IsOfficialVersion,
			ReportMode: contract.ReportMode, CommencementDate: contract.CommencementDate.Format("2006-01-02"),
			LeaseEndDate: contract.LeaseEndDate.Format("2006-01-02"), Currency: contract.Currency,
			DiscountRateType: contract.DiscountRateType, DiscountRateMissing: contract.DiscountRateMissing,
		})
	}
	result := ProjectionResult{Payload: projectionPayload(snapshot, rows, nil)}
	if request.Format != FormatCSV {
		return result
	}
	marker := "_OFFICIAL"
	if snapshot.Mode == Working {
		marker = "_WORKING_DRAFT"
	}
	result.CSV = &CSVProjection{
		Filename: fmt.Sprintf("IFRS16_LiabilityRolling%s_%s.csv", marker, snapshot.GeneratedAt.Format("20060102_150405")),
		Records:  liabilityCSVRecords(snapshot, request.Language),
	}
	return result
}

func liabilityCSVRecords(snapshot *Snapshot, language string) [][]string {
	headers := []string{
		"合同编号", "合同名称", "审批状态", "是否正式版", "报表模式",
		"法人主体ID", "门店ID", "出租方ID", "币种", "租赁起始日", "租赁结束日",
		"折现率类型", "折现率缺失", "创建时间",
	}
	switch language {
	case "en":
		headers = []string{
			"Contract Number", "Contract Name", "Approval Status", "Is Official Version", "Report Mode",
			"Legal Entity ID", "Store ID", "Landlord ID", "Currency", "Commencement Date", "Lease End Date",
			"Discount Rate Type", "Discount Rate Missing", "Created At",
		}
	case "zh-TW":
		headers = []string{
			"合同編號", "合同名稱", "審批狀態", "是否正式版", "報表模式",
			"法人主體ID", "門店ID", "出租方ID", "幣種", "租賃起始日", "租賃結束日",
			"折現率類型", "折現率缺失", "創建時間",
		}
	}
	records := make([][]string, 0, len(snapshot.Contracts)+1)
	records = append(records, headers)
	for _, fact := range snapshot.Contracts {
		contract := fact.Contract
		records = append(records, []string{
			contract.ContractNumber, contract.ContractName, contract.ApprovalStatus,
			fmt.Sprintf("%v", contract.IsOfficialVersion), contract.ReportMode,
			stringValue(contract.LegalEntityID), stringValue(contract.StoreID), stringValue(contract.LandlordID),
			contract.Currency, contract.CommencementDate.Format("2006-01-02"), contract.LeaseEndDate.Format("2006-01-02"),
			stringValue(contract.DiscountRateType), fmt.Sprintf("%v", contract.DiscountRateMissing),
			contract.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	return records
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func projectCashflow(snapshot *Snapshot, request ProjectionRequest) (ProjectionResult, error) {
	if request.View == "" {
		request.View = ViewSummary
	}
	if request.Granularity == "" {
		request.Granularity = GranularityMonth
	}
	if request.EndDate.Before(request.StartDate) {
		return ProjectionResult{}, fmt.Errorf("end date must not be before start date")
	}
	if request.View != ViewContract && request.View != ViewStore && request.View != ViewSummary {
		return ProjectionResult{}, fmt.Errorf("invalid cashflow view %q", request.View)
	}
	if request.Granularity != GranularityMonth && request.Granularity != GranularityQuarter && request.Granularity != GranularityYear {
		return ProjectionResult{}, fmt.Errorf("invalid cashflow granularity %q", request.Granularity)
	}

	type groupKey struct {
		group  string
		period string
	}
	rows := make(map[groupKey]*CashflowRow)
	tagFilters := normalizeProjectionTokens(request.Tags)
	for _, fact := range snapshot.Contracts {
		contract := fact.Contract
		if request.ContractID != "" && contract.ID != request.ContractID {
			continue
		}
		if request.Store != "" {
			storeID := ""
			if contract.StoreID != nil {
				storeID = *contract.StoreID
			}
			if !strings.Contains(contract.StoreName, request.Store) && !strings.Contains(storeID, request.Store) {
				continue
			}
		}
		if len(tagFilters) > 0 && !matchesAnyTag(splitProjectionTags(contract.Tags), tagFilters) {
			continue
		}

		for _, schedule := range fact.PaymentSchedules {
			if schedule.DueDate.Before(request.StartDate) || schedule.DueDate.After(request.EndDate) {
				continue
			}
			periodKey := projectionBucketKey(schedule.DueDate, request.Granularity)
			periodStart, periodEnd := projectionBucketRange(schedule.DueDate, request.Granularity)
			group, label, currency := "summary", "汇总", "CNY"
			contractID, contractNumber, contractName, storeName := "", "", "", ""
			switch request.View {
			case ViewContract:
				group = contract.ID
				label = contract.ContractNumber + " - " + contract.ContractName
				currency = contract.Currency
				contractID, contractNumber, contractName, storeName = contract.ID, contract.ContractNumber, contract.ContractName, contract.StoreName
			case ViewStore:
				group = fallbackProjectionStore(contract.StoreName)
				label = group
			}
			key := groupKey{group: group, period: periodKey}
			row := rows[key]
			if row == nil {
				row = &CashflowRow{
					GroupKey: group, GroupLabel: label, Currency: currency,
					ContractID: contractID, ContractNumber: contractNumber,
					ContractName: contractName, StoreName: storeName,
					PeriodKey: periodKey, PeriodStart: periodStart.Format("2006-01-02"), PeriodEnd: periodEnd.Format("2006-01-02"),
				}
				rows[key] = row
			}
			amount := schedule.Amount
			switch {
			case schedule.IsVariable || schedule.AmountType == "turnover_rent":
				row.VariableRent += amount
			case schedule.IsNonLeaseComponent || schedule.AmountType == "cam" || schedule.AmountType == "service_fee":
				row.NonLeaseExpense += amount
			default:
				row.FixedRent += amount
			}
			if schedule.TaxAmount != nil {
				row.TaxAmount += *schedule.TaxAmount
			}
			row.TotalCashOut = row.FixedRent + row.VariableRent + row.NonLeaseExpense + row.TaxAmount
			row.PaymentCount++
		}
	}

	data := make([]CashflowRow, 0, len(rows))
	for _, row := range rows {
		data = append(data, *row)
	}
	sort.Slice(data, func(i, j int) bool {
		if data[i].GroupKey != data[j].GroupKey {
			return data[i].GroupKey < data[j].GroupKey
		}
		return data[i].PeriodStart < data[j].PeriodStart
	})
	return ProjectionResult{Payload: projectionPayload(snapshot, data, map[string]any{
		"view": request.View, "granularity": request.Granularity,
		"start_date": request.StartDate.Format("2006-01-02"), "end_date": request.EndDate.Format("2006-01-02"),
	})}, nil
}

func projectionPayload(snapshot *Snapshot, data any, fields map[string]any) map[string]any {
	payload := map[string]any{
		"snapshot_id": snapshot.ID, "policy_version": snapshot.PolicyVersion,
		"mode": snapshot.Mode, "is_official": snapshot.IsOfficial,
		"generated_at": snapshot.GeneratedAt, "data": data,
	}
	if rows, ok := data.([]CashflowRow); ok {
		payload["total"] = len(rows)
	}
	if rows, ok := data.([]LiabilityRollingRow); ok {
		payload["total"] = len(rows)
	}
	if rows, ok := data.([]PortfolioRow); ok {
		payload["total"] = len(rows)
	}
	if rows, ok := data.([]SensitivityRow); ok {
		payload["total"] = len(rows)
	}
	if rows, ok := data.([]StandardComparisonRow); ok {
		payload["total"] = len(rows)
	}
	if rows, ok := data.([]AmortizationRow); ok {
		payload["total"] = len(rows)
	}
	if rows, ok := data.([]TagSummaryRow); ok {
		payload["total"] = len(rows)
	}
	if rows, ok := data.([]string); ok {
		payload["total"] = len(rows)
	}
	for key, value := range fields {
		payload[key] = value
	}
	return payload
}

func projectionBucketKey(date time.Time, granularity string) string {
	year, month := date.Year(), int(date.Month())
	switch granularity {
	case GranularityDay:
		return date.Format("2006-01-02")
	case GranularityQuarter:
		return fmt.Sprintf("%04d-Q%d", year, (month-1)/3+1)
	case GranularityHalfYear:
		if month <= 6 {
			return fmt.Sprintf("%04d-H1", year)
		}
		return fmt.Sprintf("%04d-H2", year)
	case GranularityYear:
		return fmt.Sprintf("%04d", year)
	default:
		return fmt.Sprintf("%04d-%02d", year, month)
	}
}

func projectionBucketRange(date time.Time, granularity string) (time.Time, time.Time) {
	year, month := date.Year(), int(date.Month())
	switch granularity {
	case GranularityDay:
		return date, date
	case GranularityQuarter:
		start := time.Date(year, time.Month(((month-1)/3)*3+1), 1, 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 3, -1)
	case GranularityHalfYear:
		if month <= 6 {
			return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(year, 6, 30, 0, 0, 0, 0, time.UTC)
		}
		return time.Date(year, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)
	case GranularityYear:
		return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)
	default:
		start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		return start, start.AddDate(0, 1, -1)
	}
}

func fallbackProjectionStore(name string) string {
	if name != "" {
		return name
	}
	return "未分配门店"
}

func matchesAnyTag(contractTags, filters []string) bool {
	for _, filter := range filters {
		for _, tag := range contractTags {
			if tag == filter {
				return true
			}
		}
	}
	return false
}

func normalizeProjectionTokens(tokens []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" || seen[token] {
			continue
		}
		seen[token] = true
		result = append(result, token)
	}
	return result
}

func splitProjectionTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{"未打标签"}
	}
	replacer := strings.NewReplacer("，", "|", ";", "|", "；", "|", ",", "|", "\n", "|", "\r", "|", " ", "|", "\t", "|")
	tags := normalizeProjectionTokens(strings.Split(replacer.Replace(raw), "|"))
	if len(tags) == 0 {
		return []string{"未打标签"}
	}
	return tags
}
