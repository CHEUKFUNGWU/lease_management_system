package handlers

import (
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/ifrs16"
)

// AmortizationReportRow is the response row for the amortization report.
type AmortizationReportRow struct {
	GroupKey            string  `json:"group_key"`
	GroupLabel          string  `json:"group_label"`
	ContractID          string  `json:"contract_id,omitempty"`
	ContractNumber      string  `json:"contract_number,omitempty"`
	ContractName        string  `json:"contract_name,omitempty"`
	StoreName           string  `json:"store_name,omitempty"`
	PeriodKey           string  `json:"period_key"`
	PeriodStart         string  `json:"period_start"`
	PeriodEnd           string  `json:"period_end"`
	OpeningLiability    float64 `json:"opening_liability"`
	InterestExpense     float64 `json:"interest_expense"`
	Payment             float64 `json:"payment"`
	PrepaidPayment      float64 `json:"prepaid_payment"`
	LiabilityAdjustment float64 `json:"liability_adjustment"`
	ClosingLiability    float64 `json:"closing_liability"`
	OpeningROUAsset     float64 `json:"opening_rou_asset"`
	Depreciation        float64 `json:"depreciation"`
	Impairment          float64 `json:"impairment"`
	ROUAdjustment       float64 `json:"rou_adjustment"`
	ClosingROUAsset     float64 `json:"closing_rou_asset"`
	VariableRentExpense float64 `json:"variable_rent_expense"`
	NonLeaseExpense     float64 `json:"non_lease_expense"`
	PnLAdjustment       float64 `json:"pnl_adjustment"`
	Currency            string  `json:"currency,omitempty"`
}

// contractBucketRow holds the aggregated daily data for a contract in one bucket.
type contractBucketRow struct {
	contractID     string
	contractNumber string
	contractName   string
	storeName      string
	currency       string
	periodKey      string
	periodStart    time.Time
	periodEnd      time.Time
	// Daily-derived values
	firstOpeningLiability float64
	firstOpeningROU       float64
	sumInterest           float64
	sumPayment            float64
	sumPrepaidPayment     float64
	lastClosingLiability  float64
	sumDepreciation       float64
	lastClosingROU        float64
	sumVariableRent       float64
	sumNonLease           float64
	// Event adjustment accumulators
	liabilityAdjustment float64
	impairment          float64
	rouAdjustment       float64
	pnlAdjustment       float64
	// Flag to indicate first/last day seen
	seen bool
}

// CashflowForecastRow is the response row for the cashflow forecast report.
type CashflowForecastRow struct {
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

// cashflowBucketRow holds aggregated cashflow data for one group in one bucket.
type cashflowBucketRow struct {
	groupKey       string
	groupLabel     string
	contractID     string
	contractNumber string
	contractName   string
	storeName      string
	currency       string
	periodKey      string
	periodStart    time.Time
	periodEnd      time.Time
	fixedRent      float64
	variableRent   float64
	nonLease       float64
	taxAmount      float64
	paymentCount   int
}

// TagSummaryRow is the response row for the tag summary endpoint.
type TagSummaryRow struct {
	Tag             string   `json:"tag"`
	ContractCount   int      `json:"contract_count"`
	ContractIDs     []string `json:"contract_ids"`
	ContractNumbers []string `json:"contract_numbers"`
	ContractNames   []string `json:"contract_names"`
}

// PortfolioSummaryRow summarizes lease population by asset class, IFRS 16 scope, and currency.
type PortfolioSummaryRow struct {
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

type SensitivityScenarioRow struct {
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

type ReportHandler struct {
	contractRepo      *repository.ContractRepository
	psRepo            *repository.PaymentScheduleRepository
	mcRepo            *repository.MonthlyClosingRepository
	systemSettingRepo *repository.SystemSettingRepository
}

func NewReportHandler(contractRepo *repository.ContractRepository, psRepo *repository.PaymentScheduleRepository, mcRepo *repository.MonthlyClosingRepository, systemSettingRepo *repository.SystemSettingRepository) *ReportHandler {
	return &ReportHandler{
		contractRepo:      contractRepo,
		psRepo:            psRepo,
		mcRepo:            mcRepo,
		systemSettingRepo: systemSettingRepo,
	}
}

// LiabilityRolling returns lease liability rolling schedule
// Query param: mode=working|official
func (h *ReportHandler) LiabilityRolling(c *gin.Context) {
	mode := c.Query("mode")
	if mode == "" {
		mode = "working"
	}
	legalEntityID := middleware.GetTenantID(c)

	// Build status filter based on report mode
	var statuses []string
	if mode == "official" {
		statuses = []string{"approved"}
	} else {
		// Working report includes all non-rejected statuses
		statuses = []string{"draft", "submitted", "reviewed", "pending_approval", "approved"}
	}

	// Query contracts with status filter and tenant isolation
	contracts, err := h.contractRepo.GetByStatuses(c.Request.Context(), statuses, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var results []gin.H
	for _, contract := range contracts {
		results = append(results, gin.H{
			"contract_id":           contract.ID,
			"contract_number":       contract.ContractNumber,
			"contract_name":         contract.ContractName,
			"approval_status":       contract.ApprovalStatus,
			"is_official_version":   contract.IsOfficialVersion,
			"report_mode":           contract.ReportMode,
			"commencement_date":     contract.CommencementDate.Format("2006-01-02"),
			"lease_end_date":        contract.LeaseEndDate.Format("2006-01-02"),
			"currency":              contract.Currency,
			"discount_rate_type":    contract.DiscountRateType,
			"discount_rate_missing": contract.DiscountRateMissing,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"mode":         mode,
		"is_official":  mode == "official",
		"data":         results,
		"total":        len(results),
		"generated_at": time.Now(),
	})
}

// ContractSummary returns summary statistics for the dashboard
func (h *ReportHandler) ContractSummary(c *gin.Context) {
	mode := c.Query("mode")
	if mode == "" {
		mode = "working"
	}
	legalEntityID := middleware.GetTenantID(c)

	var statuses []string
	if mode == "official" {
		statuses = []string{"approved"}
	} else {
		statuses = []string{"draft", "submitted", "reviewed", "pending_approval", "approved"}
	}

	contracts, err := h.contractRepo.GetByStatuses(c.Request.Context(), statuses, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var totalContracts, approvedContracts, draftContracts int
	for _, c := range contracts {
		totalContracts++
		switch c.ApprovalStatus {
		case "approved":
			approvedContracts++
		case "draft":
			draftContracts++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"mode":            mode,
		"total_contracts": totalContracts,
		"approved_count":  approvedContracts,
		"draft_count":     draftContracts,
		"pending_count":   totalContracts - approvedContracts - draftContracts,
	})
}

// PortfolioSummary returns portfolio-level operational and accounting exposure by asset type.
func (h *ReportHandler) PortfolioSummary(c *gin.Context) {
	mode := c.Query("mode")
	if mode == "" {
		mode = "working"
	}
	legalEntityID := middleware.GetTenantID(c)

	var statuses []string
	if mode == "official" {
		statuses = []string{"approved"}
	} else {
		statuses = []string{"draft", "submitted", "reviewed", "pending_approval", "approved"}
	}

	contracts, err := h.contractRepo.GetByStatuses(c.Request.Context(), statuses, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsByKey := make(map[string]*PortfolioSummaryRow)
	now := time.Now()
	for _, ct := range contracts {
		assetType := ct.AssetType
		if assetType == "" {
			assetType = "real_estate"
		}
		leaseScope := ct.LeaseScope
		if leaseScope == "" {
			leaseScope = "in_scope"
		}
		currency := ct.Currency
		if currency == "" {
			currency = "CNY"
		}
		key := assetType + "|" + leaseScope + "|" + currency
		row := rowsByKey[key]
		if row == nil {
			row = &PortfolioSummaryRow{
				AssetType:  assetType,
				LeaseScope: leaseScope,
				Currency:   currency,
			}
			rowsByKey[key] = row
		}

		row.ContractCount++
		if ct.ApprovalStatus == "approved" {
			row.ApprovedCount++
		}
		if !ct.LeaseEndDate.Before(now) {
			row.ActiveContractCount++
		}
		if ct.DiscountRateMissing {
			row.MissingDiscountRateCount++
		}
		if row.EarliestCommencementDate == "" || ct.CommencementDate.Format("2006-01-02") < row.EarliestCommencementDate {
			row.EarliestCommencementDate = ct.CommencementDate.Format("2006-01-02")
		}
		if row.LatestLeaseEndDate == "" || ct.LeaseEndDate.Format("2006-01-02") > row.LatestLeaseEndDate {
			row.LatestLeaseEndDate = ct.LeaseEndDate.Format("2006-01-02")
		}

		schedules, err := h.psRepo.GetByContractID(c.Request.Context(), ct.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		for _, ps := range schedules {
			row.PaymentCount++
			switch {
			case ps.IsVariable:
				row.VariableRentExposure += ps.Amount
			case ps.IsNonLeaseComponent:
				row.NonLeaseComponentAmount += ps.Amount
			case ps.IsLeaseComponent && ps.IsFixed:
				row.FixedLeaseCommitment += ps.Amount
			}
		}
	}

	rows := make([]*PortfolioSummaryRow, 0, len(rowsByKey))
	for _, row := range rowsByKey {
		rows = append(rows, row)
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

	c.JSON(http.StatusOK, gin.H{
		"mode":         mode,
		"is_official":  mode == "official",
		"data":         rows,
		"total":        len(rows),
		"generated_at": time.Now(),
	})
}

// SensitivityAnalysis returns discount-rate impact scenarios for one contract.
func (h *ReportHandler) SensitivityAnalysis(c *gin.Context) {
	contractID := c.Query("contract_id")
	if contractID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contract_id is required"})
		return
	}
	legalEntityID := middleware.GetTenantID(c)
	contract, err := h.contractRepo.GetByID(c.Request.Context(), contractID, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if contract == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
		return
	}

	schedules, err := h.psRepo.GetByContractID(c.Request.Context(), contractID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(schedules) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payment schedules are required for sensitivity analysis"})
		return
	}

	baseRate := 0.0
	if raw := c.Query("base_rate"); raw != "" {
		baseRate, _ = strconv.ParseFloat(raw, 64)
	}
	if baseRate <= 0 {
		baseRate = resolveGlobalDiscountRate(c.Request.Context(), h.systemSettingRepo)
	}
	if baseRate <= 0 && contract.DiscountRateValue != nil {
		baseRate = *contract.DiscountRateValue
	}
	if baseRate <= 0 {
		baseRate = 0.05
	}

	shockValues := []float64{-0.01, -0.005, 0, 0.005, 0.01}
	if raw := c.Query("shocks"); raw != "" {
		shockValues = []float64{}
		for _, part := range strings.Split(raw, ",") {
			value, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
			if err == nil {
				shockValues = append(shockValues, value)
			}
		}
		if len(shockValues) == 0 {
			shockValues = []float64{0}
		}
	}

	payments := repository.ToIFRS16Payments(schedules)
	var baseLiability float64
	rows := make([]SensitivityScenarioRow, 0, len(shockValues))
	for _, shock := range shockValues {
		rate := baseRate + shock
		if rate < 0 {
			rate = 0
		}
		result, err := ifrs16.Calculate(ifrs16.LeaseCalculation{
			CommencementDate: contract.CommencementDate,
			LeaseEndDate:     contract.LeaseEndDate,
			LeaseScope:       contract.LeaseScope,
			DiscountRate:     rate,
			Payments:         payments,
			PrepaidRent: ifrs16.CalculatePrepaidRent(ifrs16.LeaseCalculation{
				CommencementDate: contract.CommencementDate,
				Payments:         payments,
			}),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if math.Abs(shock) < 0.0000001 {
			baseLiability = result.InitialLiability
		}
		rows = append(rows, SensitivityScenarioRow{
			ScenarioName:     fmt.Sprintf("%+.2f%%", shock*100),
			DiscountRate:     rate,
			RateDelta:        shock,
			InitialLiability: result.InitialLiability,
			InitialROUAsset:  result.InitialROUAsset,
		})
	}
	if baseLiability == 0 && len(rows) > 0 {
		baseLiability = rows[0].InitialLiability
	}
	for i := range rows {
		rows[i].LiabilityDelta = rows[i].InitialLiability - baseLiability
		if baseLiability != 0 {
			rows[i].LiabilityDeltaPercent = rows[i].LiabilityDelta / baseLiability
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"contract_id":     contract.ID,
		"contract_number": contract.ContractNumber,
		"contract_name":   contract.ContractName,
		"base_rate":       baseRate,
		"lease_scope":     contract.LeaseScope,
		"currency":        contract.Currency,
		"generated_at":    time.Now(),
		"data":            rows,
		"total":           len(rows),
	})
}

// StandardComparison compares the current contract under selected lease accounting standards.
func (h *ReportHandler) StandardComparison(c *gin.Context) {
	contractID := c.Query("contract_id")
	if contractID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contract_id is required"})
		return
	}
	legalEntityID := middleware.GetTenantID(c)
	contract, err := h.contractRepo.GetByID(c.Request.Context(), contractID, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if contract == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
		return
	}

	schedules, err := h.psRepo.GetByContractID(c.Request.Context(), contractID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(schedules) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payment schedules are required for standard comparison"})
		return
	}

	discountRate := 0.0
	if raw := c.Query("discount_rate"); raw != "" {
		discountRate, _ = strconv.ParseFloat(raw, 64)
	}
	if discountRate <= 0 {
		discountRate = resolveGlobalDiscountRate(c.Request.Context(), h.systemSettingRepo)
	}
	if discountRate <= 0 && contract.DiscountRateValue != nil {
		discountRate = *contract.DiscountRateValue
	}
	if discountRate <= 0 {
		discountRate = 0.05
	}

	payments := repository.ToIFRS16Payments(schedules)
	input := ifrs16.LeaseCalculation{
		CommencementDate: contract.CommencementDate,
		LeaseEndDate:     contract.LeaseEndDate,
		LeaseScope:       contract.LeaseScope,
		DiscountRate:     discountRate,
		Payments:         payments,
		PrepaidRent: ifrs16.CalculatePrepaidRent(ifrs16.LeaseCalculation{
			CommencementDate: contract.CommencementDate,
			Payments:         payments,
		}),
	}
	result, err := ifrs16.Calculate(input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	financeExpense, totalCapitalizedCost := summarizeCapitalizedCost(result)
	straightLineExpense, totalFixedCost := summarizeStraightLineCost(result, payments)
	rows := []StandardComparisonRow{
		{
			Standard:              "ifrs16",
			StandardName:          "IFRS 16",
			Classification:        "single lessee model",
			MeasurementBasis:      result.MeasurementBasis,
			InitialLiability:      result.InitialLiability,
			InitialROUAsset:       result.InitialROUAsset,
			FirstPeriodExpense:    financeExpense,
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
			Standard:              "asc842_finance",
			StandardName:          "ASC 842 - Finance Lease View",
			Classification:        "finance lease",
			MeasurementBasis:      result.MeasurementBasis,
			InitialLiability:      result.InitialLiability,
			InitialROUAsset:       result.InitialROUAsset,
			FirstPeriodExpense:    financeExpense,
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
			Standard:              "asc842_operating",
			StandardName:          "ASC 842 - Operating Lease View",
			Classification:        "operating lease",
			MeasurementBasis:      result.MeasurementBasis,
			InitialLiability:      result.InitialLiability,
			InitialROUAsset:       result.InitialROUAsset,
			FirstPeriodExpense:    straightLineExpense,
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
			Standard:              "cas21",
			StandardName:          "中国企业会计准则第21号 - 租赁",
			Classification:        "new lease standard lessee model",
			MeasurementBasis:      result.MeasurementBasis,
			InitialLiability:      result.InitialLiability,
			InitialROUAsset:       result.InitialROUAsset,
			FirstPeriodExpense:    financeExpense,
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
	if result.MeasurementBasis == "straight_line_expense" {
		for i := range rows {
			rows[i].FirstPeriodExpense = straightLineExpense
			rows[i].TotalRecognizedCost = totalFixedCost
			rows[i].InitialLiability = 0
			rows[i].InitialROUAsset = 0
		}
	}
	if result.MeasurementBasis == "skipped" {
		for i := range rows {
			rows[i].FirstPeriodExpense = 0
			rows[i].TotalRecognizedCost = 0
			rows[i].InitialLiability = 0
			rows[i].InitialROUAsset = 0
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"contract_id":     contract.ID,
		"contract_number": contract.ContractNumber,
		"contract_name":   contract.ContractName,
		"lease_scope":     contract.LeaseScope,
		"discount_rate":   discountRate,
		"currency":        contract.Currency,
		"generated_at":    time.Now(),
		"data":            rows,
		"total":           len(rows),
	})
}

func summarizeCapitalizedCost(result *ifrs16.CalculationResult) (firstPeriodExpense float64, totalCost float64) {
	for idx, row := range result.MonthlySummary {
		periodCost := row.InterestExpense + row.Depreciation + row.VariableRentExpense + row.NonLeaseExpense
		if idx == 0 {
			firstPeriodExpense = periodCost
		}
		totalCost += periodCost
	}
	return firstPeriodExpense, totalCost
}

func summarizeStraightLineCost(result *ifrs16.CalculationResult, payments []ifrs16.LeasePayment) (firstPeriodExpense float64, totalCost float64) {
	for _, payment := range payments {
		if payment.Type == "fixed" || payment.Type == "" {
			totalCost += payment.Amount
		}
	}
	if len(result.MonthlySummary) > 0 {
		firstPeriodExpense = totalCost / float64(len(result.MonthlySummary))
	}
	return roundReport(firstPeriodExpense), roundReport(totalCost)
}

func roundReport(value float64) float64 {
	return math.Round(value*100) / 100
}

// ExportLiabilityRolling exports contract data as CSV
func (h *ReportHandler) ExportLiabilityRolling(c *gin.Context) {
	mode := c.Query("mode")
	if mode == "" {
		mode = "working"
	}

	lang := c.Query("language")
	if lang == "" {
		lang = "zh-CN"
	}

	var statuses []string
	if mode == "official" {
		statuses = []string{"approved"}
	} else {
		statuses = []string{"draft", "submitted", "reviewed", "pending_approval", "approved"}
	}

	legalEntityID := middleware.GetTenantID(c)
	contracts, err := h.contractRepo.GetByStatuses(c.Request.Context(), statuses, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Set headers for CSV download
	marker := ""
	if mode == "working" {
		marker = "_WORKING_DRAFT"
	} else {
		marker = "_OFFICIAL"
	}

	filename := fmt.Sprintf("IFRS16_LiabilityRolling%s_%s.csv", marker, time.Now().Format("20060102_150405"))
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	// BOM for Excel UTF-8
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})

	// Header
	var headers []string
	switch lang {
	case "en":
		headers = []string{
			"Contract Number", "Contract Name", "Approval Status", "Is Official Version", "Report Mode",
			"Legal Entity ID", "Store ID", "Landlord ID", "Currency",
			"Commencement Date", "Lease End Date", "Discount Rate Type", "Discount Rate Missing",
			"Created At",
		}
	case "zh-TW":
		headers = []string{
			"合同編號", "合同名稱", "審批狀態", "是否正式版", "報表模式",
			"法人主體ID", "門店ID", "出租方ID", "幣種",
			"租賃起始日", "租賃結束日", "折現率類型", "折現率缺失",
			"創建時間",
		}
	default: // zh-CN
		headers = []string{
			"合同编号", "合同名称", "审批状态", "是否正式版", "报表模式",
			"法人主体ID", "门店ID", "出租方ID", "币种",
			"租赁起始日", "租赁结束日", "折现率类型", "折现率缺失",
			"创建时间",
		}
	}
	writer.Write(headers)

	// Data rows
	for _, contract := range contracts {
		writer.Write([]string{
			contract.ContractNumber,
			contract.ContractName,
			contract.ApprovalStatus,
			fmt.Sprintf("%v", contract.IsOfficialVersion),
			contract.ReportMode,
			func() string {
				if contract.LegalEntityID != nil {
					return *contract.LegalEntityID
				}
				return ""
			}(),
			func() string {
				if contract.StoreID != nil {
					return *contract.StoreID
				}
				return ""
			}(),
			func() string {
				if contract.LandlordID != nil {
					return *contract.LandlordID
				}
				return ""
			}(),
			contract.Currency,
			contract.CommencementDate.Format("2006-01-02"),
			contract.LeaseEndDate.Format("2006-01-02"),
			func() string {
				if contract.DiscountRateType != nil {
					return *contract.DiscountRateType
				}
				return ""
			}(),
			fmt.Sprintf("%v", contract.DiscountRateMissing),
			contract.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
}

// Amortization returns a real-time daily amortization report.
// GET /api/v1/reports/amortization
// Query params:
//
//	mode=working|official (default working)
//	view=contract|store|summary|tag (default summary)
//	granularity=day|month|quarter|half_year|year (default month)
//	start_date=YYYY-MM-DD (required)
//	end_date=YYYY-MM-DD (required)
//	contract_id (optional)
//	store (optional)
//	tag (optional)
//	discount_rate_override (optional, float, e.g. 0.05 or 5)
//	report_currency (optional, string, e.g. CNY/USD/HKD)
//	exchange_rate (optional, float > 0)
func (h *ReportHandler) Amortization(c *gin.Context) {
	mode := c.Query("mode")
	if mode == "" {
		mode = "working"
	}
	view := c.Query("view")
	if view == "" {
		view = "summary"
	}
	granularity := c.Query("granularity")
	if granularity == "" {
		granularity = "month"
	}

	// Validate granularity and view early before expensive computation
	switch granularity {
	case "day", "month", "quarter", "half_year", "year":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid granularity, must be day|month|quarter|half_year|year"})
		return
	}
	switch view {
	case "contract", "store", "summary", "tag":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid view, must be contract|store|summary|tag"})
		return
	}

	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	if startDateStr == "" || endDateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date and end_date are required"})
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid start_date: %v", err)})
		return
	}
	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid end_date: %v", err)})
		return
	}
	if endDate.Before(startDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end_date must be >= start_date"})
		return
	}

	contractIDFilter := c.Query("contract_id")
	storeFilter := c.Query("store")
	discountRateOverrideStr := c.Query("discount_rate_override")
	reportCurrency := c.Query("report_currency")
	exchangeRateStr := c.Query("exchange_rate")

	// Collect tag filters from tags[] (repeated) and tag (comma-separated)
	var tagFilters []string
	tagFilters = append(tagFilters, c.QueryArray("tags")...)
	if singleTag := c.Query("tag"); singleTag != "" {
		// Support comma-separated values in the single tag param
		for _, t := range strings.Split(singleTag, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tagFilters = append(tagFilters, t)
			}
		}
	}
	// Normalize tag filter tokens (trim whitespace; preserve # if present; do not auto-prefix)
	normalizedTagFilters := normalizeTokens(tagFilters)

	// Parse and normalize discount rate override
	var discountRateOverride *float64
	var normalizedOverride *float64 // for response display
	if discountRateOverrideStr != "" {
		val, err := parseFloat(discountRateOverrideStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid discount_rate_override: %v", err)})
			return
		}
		if val > 1 {
			val = val / 100
		}
		discountRateOverride = &val
		normalizedOverride = &val
	}

	// Parse exchange rate
	var exchangeRate float64
	if exchangeRateStr != "" {
		exchangeRate, err = parseFloat(exchangeRateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid exchange_rate: %v", err)})
			return
		}
		if exchangeRate <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "exchange_rate must be > 0"})
			return
		}
	}

	legalEntityID := middleware.GetTenantID(c)

	// Build status filter
	var statuses []string
	if mode == "official" {
		statuses = []string{"approved"}
	} else {
		statuses = []string{"draft", "submitted", "reviewed", "pending_approval", "approved"}
	}

	contracts, err := h.contractRepo.GetByStatuses(c.Request.Context(), statuses, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Filter contracts in-memory
	var filtered []*repository.Contract
	for _, ct := range contracts {
		if contractIDFilter != "" && ct.ID != contractIDFilter {
			continue
		}
		if storeFilter != "" {
			storeID := ""
			if ct.StoreID != nil {
				storeID = *ct.StoreID
			}
			if !strings.Contains(ct.StoreName, storeFilter) && !strings.Contains(storeID, storeFilter) {
				continue
			}
		}
		if len(normalizedTagFilters) > 0 {
			ctTags := splitTags(ct.Tags)
			matched := false
			for _, ft := range normalizedTagFilters {
				for _, ctTag := range ctTags {
					if ctTag == ft {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if !matched {
				continue
			}
		}
		filtered = append(filtered, ct)
	}

	// Build contractID -> tags map for tag view aggregation
	contractTags := make(map[string][]string)
	for _, ct := range filtered {
		contractTags[ct.ID] = splitTags(ct.Tags)
	}

	// Build contract-level bucket rows
	var allContractRows []contractBucketRow

	for _, ct := range filtered {
		// 1) Determine discount rate
		var discountRate float64
		if discountRateOverride != nil {
			discountRate = *discountRateOverride
		} else {
			discountRate = getDiscountRate(c.Request.Context(), h.mcRepo, h.systemSettingRepo, ct)
		}

		// 2) Load payment schedules and convert to IFRS16 payments
		schedules, err := h.psRepo.GetByContractID(c.Request.Context(), ct.ID)
		if err != nil {
			// Skip contracts with errors loading schedules
			continue
		}
		payments := repository.ToIFRS16Payments(schedules)

		// 3) Calculate daily amortization
		calcInput := ifrs16.LeaseCalculation{
			CommencementDate: ct.CommencementDate,
			LeaseEndDate:     ct.LeaseEndDate,
			LeaseScope:       ct.LeaseScope,
			DiscountRate:     discountRate,
			Payments:         payments,
			PrepaidRent: ifrs16.CalculatePrepaidRent(ifrs16.LeaseCalculation{
				CommencementDate: ct.CommencementDate,
				Payments:         payments,
			}),
		}
		result, err := ifrs16.Calculate(calcInput)
		if err != nil {
			continue
		}

		// 4) Filter daily rows within [startDate, endDate]
		// Then group into buckets by granularity
		bucketMap := make(map[string]*contractBucketRow)

		for _, entry := range result.DailyAmortization {
			if entry.Date.Before(startDate) || entry.Date.After(endDate) {
				continue
			}
			key := bucketKey(entry.Date, granularity)
			bucket, ok := bucketMap[key]
			if !ok {
				periodStart, periodEnd := bucketRange(entry.Date, granularity)
				bucket = &contractBucketRow{
					contractID:     ct.ID,
					contractNumber: ct.ContractNumber,
					contractName:   ct.ContractName,
					storeName:      ct.StoreName,
					currency:       ct.Currency,
					periodKey:      key,
					periodStart:    periodStart,
					periodEnd:      periodEnd,
				}
				bucketMap[key] = bucket
			}

			if !bucket.seen {
				bucket.firstOpeningLiability = entry.OpeningLiability
				bucket.firstOpeningROU = entry.OpeningROUAsset
				bucket.seen = true
			}
			bucket.sumInterest += entry.InterestExpense
			bucket.sumPayment += entry.Payment
			bucket.sumPrepaidPayment += entry.PrepaidPayment
			bucket.liabilityAdjustment += entry.LiabilityAdjustment
			bucket.lastClosingLiability = entry.ClosingLiability
			bucket.sumDepreciation += entry.Depreciation
			bucket.rouAdjustment += entry.ROUAdjustment
			bucket.lastClosingROU = entry.ClosingROUAsset
			bucket.sumVariableRent += entry.VariableRentExpense
			bucket.sumNonLease += entry.NonLeaseExpense
		}

		// 5) Event adjustments
		adjustments, err := h.mcRepo.GetEventAdjustmentsForContract(c.Request.Context(), ct.ID)
		if err == nil {
			for _, adj := range adjustments {
				if adj.EffectiveDate.Before(startDate) || adj.EffectiveDate.After(endDate) {
					continue
				}
				key := bucketKey(adj.EffectiveDate, granularity)
				bucket, ok := bucketMap[key]
				if !ok {
					periodStart, periodEnd := bucketRange(adj.EffectiveDate, granularity)
					bucket = &contractBucketRow{
						contractID:     ct.ID,
						contractNumber: ct.ContractNumber,
						contractName:   ct.ContractName,
						storeName:      ct.StoreName,
						periodKey:      key,
						periodStart:    periodStart,
						periodEnd:      periodEnd,
					}
					bucketMap[key] = bucket
				}
				if adj.AdjustmentType == "impairment" {
					imp := math.Abs(adj.ROUAdjustment)
					if imp == 0 {
						imp = adj.PnLLoss
					}
					bucket.impairment += imp
					bucket.pnlAdjustment += adj.PnLGain - adj.PnLLoss
				} else {
					bucket.liabilityAdjustment += adj.LiabilityAdjustment
					bucket.rouAdjustment += adj.ROUAdjustment
					bucket.pnlAdjustment += adj.PnLGain - adj.PnLLoss
				}
			}
		}

		// Flatten bucket map
		for _, bucket := range bucketMap {
			allContractRows = append(allContractRows, *bucket)
		}
	}

	// 6) Aggregate by view
	var responseRows []AmortizationReportRow
	switch view {
	case "contract":
		for _, cr := range allContractRows {
			row := contractBucketRowToResponse(cr)
			row.GroupKey = cr.contractID
			row.GroupLabel = cr.contractNumber + " - " + cr.contractName
			row.ContractID = cr.contractID
			row.ContractNumber = cr.contractNumber
			row.ContractName = cr.contractName
			row.StoreName = cr.storeName
			if reportCurrency != "" {
				row.Currency = reportCurrency
			} else {
				row.Currency = cr.currency
			}
			responseRows = append(responseRows, row)
		}
	case "store":
		storeMap := make(map[string]*AmortizationReportRow)
		for _, cr := range allContractRows {
			storeName := resolveStoreName(cr)
			aggKey := storeName + "|" + cr.periodKey
			agg, ok := storeMap[aggKey]
			if !ok {
				agg = &AmortizationReportRow{
					GroupKey:    storeName,
					GroupLabel:  storeName,
					StoreName:   storeName,
					PeriodKey:   cr.periodKey,
					PeriodStart: cr.periodStart.Format("2006-01-02"),
					PeriodEnd:   cr.periodEnd.Format("2006-01-02"),
					Currency:    resolveAggCurrency(reportCurrency),
				}
				storeMap[aggKey] = agg
			}
			addToAggregate(agg, cr)
		}
		for _, agg := range storeMap {
			responseRows = append(responseRows, *agg)
		}
	case "summary":
		summaryMap := make(map[string]*AmortizationReportRow)
		for _, cr := range allContractRows {
			agg, ok := summaryMap[cr.periodKey]
			if !ok {
				agg = &AmortizationReportRow{
					GroupKey:    "summary",
					GroupLabel:  "汇总",
					PeriodKey:   cr.periodKey,
					PeriodStart: cr.periodStart.Format("2006-01-02"),
					PeriodEnd:   cr.periodEnd.Format("2006-01-02"),
					Currency:    resolveAggCurrency(reportCurrency),
				}
				summaryMap[cr.periodKey] = agg
			}
			addToAggregate(agg, cr)
		}
		for _, agg := range summaryMap {
			responseRows = append(responseRows, *agg)
		}
	case "tag":
		tagMap := make(map[string]*AmortizationReportRow)
		for _, cr := range allContractRows {
			tags := contractTags[cr.contractID]
			for _, tag := range tags {
				aggKey := tag + "|" + cr.periodKey
				agg, ok := tagMap[aggKey]
				if !ok {
					agg = &AmortizationReportRow{
						GroupKey:    tag,
						GroupLabel:  tag,
						PeriodKey:   cr.periodKey,
						PeriodStart: cr.periodStart.Format("2006-01-02"),
						PeriodEnd:   cr.periodEnd.Format("2006-01-02"),
						Currency:    resolveAggCurrency(reportCurrency),
					}
					tagMap[aggKey] = agg
				}
				addToAggregate(agg, cr)
			}
		}
		for _, agg := range tagMap {
			responseRows = append(responseRows, *agg)
		}
	}

	// Sort: by group key, then period_start ascending
	sort.Slice(responseRows, func(i, j int) bool {
		if responseRows[i].GroupKey != responseRows[j].GroupKey {
			return responseRows[i].GroupKey < responseRows[j].GroupKey
		}
		return responseRows[i].PeriodStart < responseRows[j].PeriodStart
	})

	// Apply exchange rate conversion if requested
	if exchangeRate > 0 {
		for i := range responseRows {
			applyExchangeRate(&responseRows[i], exchangeRate)
		}
	}

	// Build discount_rate_override for response
	var discountRateOverrideResp interface{}
	if normalizedOverride != nil {
		discountRateOverrideResp = *normalizedOverride
	}

	c.JSON(http.StatusOK, gin.H{
		"mode":                   mode,
		"view":                   view,
		"granularity":            granularity,
		"start_date":             startDateStr,
		"end_date":               endDateStr,
		"report_currency":        reportCurrency,
		"exchange_rate":          exchangeRate,
		"discount_rate_override": discountRateOverrideResp,
		"data":                   responseRows,
		"total":                  len(responseRows),
	})
}

// Tags returns all unique normalized tags across tenant-scoped working-mode contracts.
// GET /api/v1/reports/tags
func (h *ReportHandler) Tags(c *gin.Context) {
	legalEntityID := middleware.GetTenantID(c)
	statuses := []string{"draft", "submitted", "reviewed", "pending_approval", "approved"}

	contracts, err := h.contractRepo.GetByStatuses(c.Request.Context(), statuses, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	tagSet := make(map[string]bool)
	for _, ct := range contracts {
		for _, tag := range splitTags(ct.Tags) {
			if tag != "未打标签" {
				tagSet[tag] = true
			}
		}
	}

	var tags []string
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	c.JSON(http.StatusOK, gin.H{
		"data":  tags,
		"total": len(tags),
	})
}

// TagSummary returns per-tag contract counts and details for the tag manager settings page.
// GET /api/v1/reports/tags/summary
func (h *ReportHandler) TagSummary(c *gin.Context) {
	legalEntityID := middleware.GetTenantID(c)
	statuses := []string{"draft", "submitted", "reviewed", "pending_approval", "approved"}

	contracts, err := h.contractRepo.GetByStatuses(c.Request.Context(), statuses, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// tag -> set of contract IDs
	tagContracts := make(map[string]map[string]*repository.Contract)
	for _, ct := range contracts {
		for _, tag := range splitTags(ct.Tags) {
			if tag == "未打标签" {
				continue
			}
			if tagContracts[tag] == nil {
				tagContracts[tag] = make(map[string]*repository.Contract)
			}
			tagContracts[tag][ct.ID] = ct
		}
	}

	var rows []TagSummaryRow
	for tag, ctMap := range tagContracts {
		var ids, numbers, names []string
		for _, ct := range ctMap {
			ids = append(ids, ct.ID)
			numbers = append(numbers, ct.ContractNumber)
			names = append(names, ct.ContractName)
		}
		// Sort sub-arrays for consistent output
		sort.Strings(ids)
		sort.Strings(numbers)
		sort.Strings(names)

		rows = append(rows, TagSummaryRow{
			Tag:             tag,
			ContractCount:   len(ctMap),
			ContractIDs:     ids,
			ContractNumbers: numbers,
			ContractNames:   names,
		})
	}

	// Sort: contract_count DESC, then tag ASC
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ContractCount != rows[j].ContractCount {
			return rows[i].ContractCount > rows[j].ContractCount
		}
		return rows[i].Tag < rows[j].Tag
	})

	c.JSON(http.StatusOK, gin.H{
		"data":  rows,
		"total": len(rows),
	})
}

// CashflowForecast returns a future rent cash flow forecast.
// GET /api/v1/reports/cashflow-forecast
// Query params:
//
//	mode=working|official (default working)
//	view=contract|store|summary (default summary)
//	granularity=month|quarter|year (default month)
//	start_date=YYYY-MM-DD (required)
//	end_date=YYYY-MM-DD (required)
//	contract_id (optional)
//	store (optional)
//	tag (optional single string, backward compatible)
//	tags (optional repeated query params for multi-tag any-match)
func (h *ReportHandler) CashflowForecast(c *gin.Context) {
	mode := c.Query("mode")
	if mode == "" {
		mode = "working"
	}
	view := c.Query("view")
	if view == "" {
		view = "summary"
	}
	granularity := c.Query("granularity")
	if granularity == "" {
		granularity = "month"
	}

	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	if startDateStr == "" || endDateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date and end_date are required"})
		return
	}

	startDate, err := time.Parse("2006-01-02", startDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid start_date: %v", err)})
		return
	}
	endDate, err := time.Parse("2006-01-02", endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid end_date: %v", err)})
		return
	}
	if endDate.Before(startDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "end_date must be >= start_date"})
		return
	}

	contractIDFilter := c.Query("contract_id")
	storeFilter := c.Query("store")

	// Collect tag filters from tags[] (repeated) and tag (comma-separated)
	var tagFilters []string
	tagFilters = append(tagFilters, c.QueryArray("tags")...)
	if singleTag := c.Query("tag"); singleTag != "" {
		for _, t := range strings.Split(singleTag, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				tagFilters = append(tagFilters, t)
			}
		}
	}
	normalizedTagFilters := normalizeTokens(tagFilters)

	legalEntityID := middleware.GetTenantID(c)

	// Build status filter based on report mode
	var statuses []string
	if mode == "official" {
		statuses = []string{"approved"}
	} else {
		statuses = []string{"draft", "submitted", "reviewed", "pending_approval", "approved"}
	}

	contracts, err := h.contractRepo.GetByStatuses(c.Request.Context(), statuses, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Filter contracts in-memory
	var filtered []*repository.Contract
	for _, ct := range contracts {
		if contractIDFilter != "" && ct.ID != contractIDFilter {
			continue
		}
		if storeFilter != "" {
			storeID := ""
			if ct.StoreID != nil {
				storeID = *ct.StoreID
			}
			if !strings.Contains(ct.StoreName, storeFilter) && !strings.Contains(storeID, storeFilter) {
				continue
			}
		}
		if len(normalizedTagFilters) > 0 {
			ctTags := splitTags(ct.Tags)
			matched := false
			for _, ft := range normalizedTagFilters {
				for _, ctTag := range ctTags {
					if ctTag == ft {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if !matched {
				continue
			}
		}
		filtered = append(filtered, ct)
	}

	// Validate granularity
	switch granularity {
	case "month", "quarter", "year":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid granularity, must be month|quarter|year"})
		return
	}

	// Validate view
	switch view {
	case "contract", "store", "summary":
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid view, must be contract|store|summary"})
		return
	}

	// Build bucketed cashflow rows
	var allBucketRows []cashflowBucketRow

	for _, ct := range filtered {
		schedules, err := h.psRepo.GetByContractID(c.Request.Context(), ct.ID)
		if err != nil || len(schedules) == 0 {
			continue
		}

		// Filter schedules by due_date range
		for _, s := range schedules {
			if s.DueDate.Before(startDate) || s.DueDate.After(endDate) {
				continue
			}

			key := bucketKey(s.DueDate, granularity)
			periodStart, periodEnd := bucketRange(s.DueDate, granularity)

			cbr := cashflowBucketRow{
				contractID:     ct.ID,
				contractNumber: ct.ContractNumber,
				contractName:   ct.ContractName,
				storeName:      ct.StoreName,
				currency:       ct.Currency,
				periodKey:      key,
				periodStart:    periodStart,
				periodEnd:      periodEnd,
				paymentCount:   1,
			}

			amount := s.Amount
			var tax float64
			if s.TaxAmount != nil {
				tax = *s.TaxAmount
			}
			totalCash := amount + tax

			// Classify: variable first, then non-lease, then fixed
			if s.IsVariable || s.AmountType == "turnover_rent" {
				cbr.variableRent = amount
			} else if s.IsNonLeaseComponent || s.AmountType == "cam" || s.AmountType == "service_fee" {
				cbr.nonLease = amount
			} else {
				cbr.fixedRent = amount
			}
			cbr.taxAmount = tax
			_ = totalCash // used below via sum of classified + tax

			allBucketRows = append(allBucketRows, cbr)
		}
	}

	// Aggregate raw rows into grouped bucketed map
	type aggKey struct {
		groupKey  string
		periodKey string
	}

	grouped := make(map[aggKey]*CashflowForecastRow)

	for _, cr := range allBucketRows {
		gKey := ""
		gLabel := ""
		currency := cr.currency
		contractID := ""
		contractNumber := ""
		contractName := ""
		storeName := ""

		switch view {
		case "contract":
			gKey = cr.contractID
			gLabel = cr.contractNumber + " - " + cr.contractName
			contractID = cr.contractID
			contractNumber = cr.contractNumber
			contractName = cr.contractName
			storeName = cr.storeName
		case "store":
			gKey = fallbackStoreName(cr.storeName)
			gLabel = gKey
			currency = resolveAggCurrency("")
		case "summary":
			gKey = "summary"
			gLabel = "汇总"
			currency = resolveAggCurrency("")
		}

		ak := aggKey{groupKey: gKey, periodKey: cr.periodKey}
		row, ok := grouped[ak]
		if !ok {
			row = &CashflowForecastRow{
				GroupKey:       gKey,
				GroupLabel:     gLabel,
				ContractID:     contractID,
				ContractNumber: contractNumber,
				ContractName:   contractName,
				StoreName:      storeName,
				Currency:       currency,
				PeriodKey:      cr.periodKey,
				PeriodStart:    cr.periodStart.Format("2006-01-02"),
				PeriodEnd:      cr.periodEnd.Format("2006-01-02"),
			}
			grouped[ak] = row
		}
		row.FixedRent += cr.fixedRent
		row.VariableRent += cr.variableRent
		row.NonLeaseExpense += cr.nonLease
		row.TaxAmount += cr.taxAmount
		row.TotalCashOut += cr.fixedRent + cr.variableRent + cr.nonLease + cr.taxAmount
		row.PaymentCount += cr.paymentCount
	}

	// Flatten to slice
	var responseRows []CashflowForecastRow
	for _, row := range grouped {
		responseRows = append(responseRows, *row)
	}

	// Sort: by group key, then period_start asc
	sort.Slice(responseRows, func(i, j int) bool {
		if responseRows[i].GroupKey != responseRows[j].GroupKey {
			return responseRows[i].GroupKey < responseRows[j].GroupKey
		}
		return responseRows[i].PeriodStart < responseRows[j].PeriodStart
	})

	c.JSON(http.StatusOK, gin.H{
		"mode":        mode,
		"view":        view,
		"granularity": granularity,
		"start_date":  startDateStr,
		"end_date":    endDateStr,
		"data":        responseRows,
		"total":       len(responseRows),
	})
}

func fallbackStoreName(name string) string {
	if name != "" {
		return name
	}
	return "未分配门店"
}

func getDiscountRate(ctx context.Context, mcRepo *repository.MonthlyClosingRepository, systemSettingRepo *repository.SystemSettingRepository, contract *repository.Contract) float64 {
	globalRate := resolveGlobalDiscountRate(ctx, systemSettingRepo)
	if globalRate > 0 {
		return globalRate
	}
	if contract.DiscountRateValue != nil && *contract.DiscountRateValue > 0 {
		return *contract.DiscountRateValue
	}
	results, err := mcRepo.GetMeasurementResults(ctx, contract.ID, "")
	if err != nil || len(results) == 0 {
		return 0.05
	}
	latest := results[len(results)-1]
	if latest.DiscountRate == 0 {
		return 0.05
	}
	return latest.DiscountRate
}

func resolveGlobalDiscountRate(ctx context.Context, repo *repository.SystemSettingRepository) float64 {
	if repo == nil {
		return 0.0
	}
	return repo.GetFloat64(ctx, "global_discount_rate", 0.0)
}

// bucketKey returns the grouping key for a given date and granularity.
func bucketKey(date time.Time, granularity string) string {
	y := date.Year()
	m := int(date.Month())
	switch granularity {
	case "day":
		return date.Format("2006-01-02")
	case "month":
		return fmt.Sprintf("%04d-%02d", y, m)
	case "quarter":
		q := (m-1)/3 + 1
		return fmt.Sprintf("%04d-Q%d", y, q)
	case "half_year":
		if m <= 6 {
			return fmt.Sprintf("%04d-H1", y)
		}
		return fmt.Sprintf("%04d-H2", y)
	case "year":
		return fmt.Sprintf("%04d", y)
	default:
		return date.Format("2006-01")
	}
}

// bucketRange returns the calendar start and end dates for a bucket.
func bucketRange(date time.Time, granularity string) (time.Time, time.Time) {
	y := date.Year()
	m := int(date.Month())
	switch granularity {
	case "day":
		return date, date
	case "month":
		start := time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, -1)
		return start, end
	case "quarter":
		q := (m-1)/3 + 1
		qm := (q-1)*3 + 1
		start := time.Date(y, time.Month(qm), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 3, -1)
		return start, end
	case "half_year":
		if m <= 6 {
			start := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC)
			end := time.Date(y, 6, 30, 0, 0, 0, 0, time.UTC)
			return start, end
		}
		start := time.Date(y, 7, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(y, 12, 31, 0, 0, 0, 0, time.UTC)
		return start, end
	case "year":
		start := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(y, 12, 31, 0, 0, 0, 0, time.UTC)
		return start, end
	default:
		start := time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, -1)
		return start, end
	}
}

func resolveStoreName(cr contractBucketRow) string {
	return fallbackStoreName(cr.storeName)
}

func addToAggregate(agg *AmortizationReportRow, cr contractBucketRow) {
	agg.OpeningLiability += cr.firstOpeningLiability
	agg.OpeningROUAsset += cr.firstOpeningROU
	agg.InterestExpense += cr.sumInterest
	agg.Payment += cr.sumPayment
	agg.PrepaidPayment += cr.sumPrepaidPayment
	agg.LiabilityAdjustment += cr.liabilityAdjustment
	agg.ClosingLiability += cr.lastClosingLiability
	agg.Depreciation += cr.sumDepreciation
	agg.Impairment += cr.impairment
	agg.ROUAdjustment += cr.rouAdjustment
	agg.ClosingROUAsset += cr.lastClosingROU
	agg.VariableRentExpense += cr.sumVariableRent
	agg.NonLeaseExpense += cr.sumNonLease
	agg.PnLAdjustment += cr.pnlAdjustment
}

func contractBucketRowToResponse(cr contractBucketRow) AmortizationReportRow {
	return AmortizationReportRow{
		PeriodKey:           cr.periodKey,
		PeriodStart:         cr.periodStart.Format("2006-01-02"),
		PeriodEnd:           cr.periodEnd.Format("2006-01-02"),
		OpeningLiability:    cr.firstOpeningLiability,
		InterestExpense:     cr.sumInterest,
		Payment:             cr.sumPayment,
		PrepaidPayment:      cr.sumPrepaidPayment,
		LiabilityAdjustment: cr.liabilityAdjustment,
		ClosingLiability:    cr.lastClosingLiability,
		OpeningROUAsset:     cr.firstOpeningROU,
		Depreciation:        cr.sumDepreciation,
		Impairment:          cr.impairment,
		ROUAdjustment:       cr.rouAdjustment,
		ClosingROUAsset:     cr.lastClosingROU,
		VariableRentExpense: cr.sumVariableRent,
		NonLeaseExpense:     cr.sumNonLease,
		PnLAdjustment:       cr.pnlAdjustment,
	}
}

// parseFloat parses a string to float64 with support for both period and comma decimal separators.
func parseFloat(s string) (float64, error) {
	s = strings.ReplaceAll(s, ",", ".")
	return strconv.ParseFloat(s, 64)
}

// resolveAggCurrency returns the display currency for aggregated (store/summary) rows.
func resolveAggCurrency(reportCurrency string) string {
	if reportCurrency != "" {
		return reportCurrency
	}
	return "CNY"
}

// applyExchangeRate multiplies all monetary fields in a row by the given exchange rate.
func applyExchangeRate(row *AmortizationReportRow, rate float64) {
	row.OpeningLiability *= rate
	row.InterestExpense *= rate
	row.Payment *= rate
	row.PrepaidPayment *= rate
	row.LiabilityAdjustment *= rate
	row.ClosingLiability *= rate
	row.OpeningROUAsset *= rate
	row.Depreciation *= rate
	row.Impairment *= rate
	row.ROUAdjustment *= rate
	row.ClosingROUAsset *= rate
	row.VariableRentExpense *= rate
	row.NonLeaseExpense *= rate
	row.PnLAdjustment *= rate
}

// splitTags splits a raw tags string into normalized individual tags.
// Supported separators: comma (,), full-width comma (，), semicolon (;),
// Chinese semicolon (；), pipe (|), whitespace/newline.
// Empty input returns ["未打标签"].
func splitTags(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{"未打标签"}
	}

	// Normalize separators to a single delimiter
	replacer := strings.NewReplacer(
		"，", "|",
		";", "|",
		"；", "|",
		",", "|",
		"\n", "|",
		"\r", "|",
		" ", "|",
		"\t", "|",
	)
	normalized := replacer.Replace(raw)

	parts := strings.Split(normalized, "|")
	var result []string
	seen := make(map[string]bool)

	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag == "" {
			continue
		}
		// Deduplicate within a single contract
		if seen[tag] {
			continue
		}
		seen[tag] = true
		result = append(result, tag)
	}

	if len(result) == 0 {
		return []string{"未打标签"}
	}
	return result
}

// normalizeTokens trims whitespace from each token, preserves leading # if present,
// and removes empty/duplicate entries. Does not auto-prefix #.
func normalizeTokens(tokens []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, t := range tokens {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if seen[t] {
			continue
		}
		seen[t] = true
		result = append(result, t)
	}
	return result
}
