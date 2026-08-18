package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
	"github.com/lease-management-system/core-service/internal/services/dealcompare"
	"github.com/lease-management-system/core-service/internal/services/renewaldecision"
	"github.com/lease-management-system/core-service/internal/services/renttosales"
)

// RenewalCardHandler turns a critical date from a reminder into a decision.
// Knowing a lease expires in ninety days is not the hard part; deciding whether
// to renew it, and on what terms, is.
type RenewalCardHandler struct {
	contractRepo      *repository.ContractRepository
	psRepo            *repository.PaymentScheduleRepository
	storeMetricsRepo  *repository.StoreMetricsRepository
	measurementRepo   *repository.MonthlyClosingRepository
	decisionRepo      *repository.RenewalDecisionRepository
	auditLogger       *audit.Logger
	systemSettingRepo *repository.SystemSettingRepository
}

func NewRenewalCardHandler(
	contractRepo *repository.ContractRepository,
	psRepo *repository.PaymentScheduleRepository,
	storeMetricsRepo *repository.StoreMetricsRepository,
	measurementRepo *repository.MonthlyClosingRepository,
	decisionRepo *repository.RenewalDecisionRepository,
	auditLogger *audit.Logger,
	settingRepos ...*repository.SystemSettingRepository,
) *RenewalCardHandler {
	var settings *repository.SystemSettingRepository
	if len(settingRepos) > 0 {
		settings = settingRepos[0]
	}
	return &RenewalCardHandler{
		contractRepo: contractRepo, psRepo: psRepo, storeMetricsRepo: storeMetricsRepo,
		measurementRepo: measurementRepo, decisionRepo: decisionRepo, auditLogger: auditLogger, systemSettingRepo: settings,
	}
}

type renewalDecisionRequest struct {
	DecisionDate    string                     `json:"decision_date" binding:"required"`
	DiscountRate    float64                    `json:"discount_rate"`
	OwnerName       string                     `json:"owner_name"`
	BusinessOpinion string                     `json:"business_opinion"`
	Evidence        string                     `json:"evidence"`
	Scenarios       []renewaldecision.Scenario `json:"scenarios" binding:"required"`
}

// Card returns everything needed to decide a renewal.
// GET /contracts/:id/renewal-card?renewal_term_months=<months>&uplift_percent=<percent>&discount_rate=<rate>
func (h *RenewalCardHandler) Card(c *gin.Context) {
	contractID := c.Param("id")
	ctx := c.Request.Context()

	contract, err := h.contractRepo.GetByID(ctx, contractID, middleware.GetTenantID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if contract == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
		return
	}

	schedules, err := h.psRepo.GetByContractID(ctx, contractID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	payments := repository.ToIFRS16Payments(schedules)

	asOf := time.Now()
	var remainingCommitment, lastRent float64
	for _, payment := range payments {
		if payment.Date.Before(asOf) || payment.Type == "variable" {
			continue
		}
		remainingCommitment += payment.Amount.Float64()
		if payment.Amount.Float64() > 0 {
			lastRent = payment.Amount.Float64()
		}
	}

	renewalMonths, hasTerm := requiredIntQuery(c, "renewal_term_months")
	uplift, hasUplift := requiredFloatQuery(c, "uplift_percent")
	rentFreeMonths, hasRentFree := requiredIntQuery(c, "rent_free_months")
	escalationPercent, hasEscalation := requiredFloatQuery(c, "annual_escalation_percent")
	exitPenaltyMonths, hasExitPenalty := requiredFloatQuery(c, "early_exit_penalty_months")
	if !hasTerm || renewalMonths <= 0 || !hasUplift || !hasRentFree || rentFreeMonths < 0 || !hasEscalation || !hasExitPenalty || exitPenaltyMonths < 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "续租情景必须明确提供租期、涨幅、免租期、年递增和退出罚金假设"})
		return
	}
	discountRate := floatQuery(c, "discount_rate", 0)
	if discountRate <= 0 && contract.DiscountRateValue != nil {
		discountRate = *contract.DiscountRateValue
	}
	if discountRate <= 0 {
		// The comparison is a present-value comparison, so a rate is required
		// rather than assumed — the same rule the rest of the product follows.
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":                 "该合同尚未确认折现率，无法进行续租现值对比",
			"discount_rate_missing": true,
		})
		return
	}

	response := gin.H{
		"contract_id":          contract.ID,
		"contract_number":      contract.ContractNumber,
		"contract_name":        contract.ContractName,
		"currency":             contract.Currency,
		"lease_end_date":       contract.LeaseEndDate.Format("2006-01-02"),
		"days_to_expiry":       int(contract.LeaseEndDate.Sub(asOf).Hours() / 24),
		"remaining_commitment": round2Handler(remainingCommitment),
		"discount_rate":        discountRate,
	}

	if decisionInput, err := h.buildDecisionInput(ctx, contract, asOf, discountRate, defaultDecisionScenarios(lastRent, renewalMonths, uplift, rentFreeMonths, escalationPercent, exitPenaltyMonths)); err == nil {
		if decision, evalErr := renewaldecision.Evaluate(decisionInput); evalErr == nil {
			response["decision_scenarios"] = decision
		}
	}

	// The comparison worth putting in front of a negotiator is not "renew
	// versus walk away" — walking away always costs less in rent, which tells
	// nobody anything. It is what the landlord's asking uplift costs against
	// holding the current rent, because that is the number being negotiated.
	if lastRent > 0 && renewalMonths > 0 {
		comparison, err := dealcompare.Compare(dealcompare.Input{
			DiscountRate: discountRate,
			Currency:     contract.Currency,
			Offers: []dealcompare.Offer{
				{
					Name: "按现租金续租", TermMonths: renewalMonths,
					BaseMonthlyRent: lastRent,
					AreaSqm:         valueOrZero(contract.AreaSqm),
				},
				{
					Name: fmt.Sprintf("按上浮 %.1f%% 续租", uplift), TermMonths: renewalMonths,
					BaseMonthlyRent: lastRent * (1 + uplift/100),
					AreaSqm:         valueOrZero(contract.AreaSqm),
				},
			},
		})
		if err == nil {
			response["renewal_comparison"] = comparison
			response["current_monthly_rent"] = round2Handler(lastRent)
			response["assumed_renewal_rent"] = round2Handler(lastRent * (1 + uplift/100))
			// What the uplift costs over the renewal term, in cash.
			response["uplift_cost_over_term"] = round2Handler(
				lastRent * (uplift / 100) * float64(renewalMonths))
			response["renewal_term_months"] = renewalMonths
			response["uplift_percent"] = uplift
		}
	}

	// The store's trading is what makes the decision a business one rather than
	// an accounting one. It is optional: many contracts have no store, and many
	// stores have no revenue reported.
	if contract.StoreID != nil {
		if health := h.storeHealth(ctx, *contract.StoreID, contract, lastRent); health != nil {
			response["store_health"] = health
		}
	}

	c.JSON(http.StatusOK, response)
}

// CreateDecision evaluates and stores an immutable decision snapshot. The
// snapshot is evidence for the business decision, not a write to the lease
// ledger.
func (h *RenewalCardHandler) CreateDecision(c *gin.Context) {
	contractID := c.Param("id")
	ctx := c.Request.Context()
	contract, err := h.contractRepo.GetByID(ctx, contractID, middleware.GetTenantID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if contract == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
		return
	}
	var req renewalDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供决策日期和至少两种情景"})
		return
	}
	decisionDate, err := time.Parse("2006-01-02", req.DecisionDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "决策日期格式应为 YYYY-MM-DD"})
		return
	}
	discountRate := req.DiscountRate
	if discountRate <= 0 && contract.DiscountRateValue != nil {
		discountRate = *contract.DiscountRateValue
	}
	input, err := h.buildDecisionInput(ctx, contract, decisionDate, discountRate, req.Scenarios)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "discount_rate_missing": discountRate <= 0})
		return
	}
	decision, err := renewaldecision.Evaluate(input)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	snapshotBytes, _ := json.Marshal(decision)
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	var entityID *string
	if legalEntityID := middleware.GetTenantID(c); legalEntityID != "" {
		entityID = &legalEntityID
	}
	snapshot, err := h.decisionRepo.Create(ctx, &repository.RenewalDecisionSnapshot{
		ContractID: contractID, LegalEntityID: entityID, DecisionDate: decisionDate,
		OwnerName: req.OwnerName, BusinessOpinion: req.BusinessOpinion, Evidence: req.Evidence,
		Snapshot: snapshotBytes, CreatedBy: stringPtr(userIDStr),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.auditLogger != nil {
		_ = h.auditLogger.Log(ctx, "renewal_decision_snapshots", snapshot.ID, "create", nil, snapshot, userIDStr, c)
	}
	c.JSON(http.StatusOK, gin.H{"data": snapshot, "decision": decision, "source": "scenario_assumption"})
}

func (h *RenewalCardHandler) ListDecisions(c *gin.Context) {
	entity, ok := tenantEntity(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required"})
		return
	}
	items, err := h.decisionRepo.List(c.Request.Context(), c.Param("id"), entity)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": len(items)})
}

func (h *RenewalCardHandler) buildDecisionInput(ctx context.Context, contract *repository.Contract, decisionDate time.Time, discountRate float64, scenarios []renewaldecision.Scenario) (renewaldecision.Input, error) {
	if discountRate <= 0 {
		return renewaldecision.Input{}, fmt.Errorf("该合同尚未确认折现率，无法进行续租现值、ROU和负债对比")
	}
	schedules, err := h.psRepo.GetByContractID(ctx, contract.ID)
	if err != nil {
		return renewaldecision.Input{}, fmt.Errorf("加载付款计划失败：%w", err)
	}
	payments := repository.ToIFRS16Payments(schedules)
	var remainingCommitment, lastRent float64
	for _, payment := range payments {
		if payment.Date.Before(decisionDate) || payment.Type == "variable" {
			continue
		}
		remainingCommitment += payment.Amount.Float64()
		if payment.Amount.Float64() > 0 {
			lastRent = payment.Amount.Float64()
		}
	}
	var currentLiability, currentROU float64
	if h.measurementRepo != nil {
		results, measurementErr := h.measurementRepo.GetMeasurementResults(ctx, contract.ID, "")
		if measurementErr != nil {
			return renewaldecision.Input{}, fmt.Errorf("加载当前计量结果失败：%w", measurementErr)
		}
		if len(results) > 0 {
			latest := results[len(results)-1]
			currentLiability = latest.ClosingLiability.Float64()
			currentROU = latest.ClosingROUAsset.Float64()
		}
	}
	remainingTermMonths := int(math.Ceil(contract.LeaseEndDate.Sub(decisionDate).Hours() / (24 * 30.4375)))
	if remainingTermMonths < 0 {
		remainingTermMonths = 0
	}
	return renewaldecision.Input{
		DecisionDate: decisionDate, Currency: contract.Currency, DiscountRate: discountRate,
		CurrentMonthlyRent: lastRent, RemainingCommitment: remainingCommitment,
		CurrentLiability: currentLiability, CurrentROU: currentROU, RemainingTermMonths: remainingTermMonths, Scenarios: scenarios,
	}, nil
}

func defaultDecisionScenarios(currentRent float64, termMonths int, uplift float64, rentFreeMonths int, escalationPercent float64, exitPenaltyMonths float64) []renewaldecision.Scenario {
	return []renewaldecision.Scenario{
		{Name: "renew_current_terms", Decision: "renew", TermMonths: termMonths, MonthlyRent: currentRent, RentFreeMonths: rentFreeMonths, AnnualEscalationPercent: escalationPercent},
		{Name: "renegotiate_terms", Decision: "renegotiate", TermMonths: termMonths, MonthlyRent: currentRent * (1 + uplift/100), RentFreeMonths: rentFreeMonths, AnnualEscalationPercent: escalationPercent},
		{Name: "terminate_no_renewal", Decision: "terminate", EarlyExitPenaltyMonths: exitPenaltyMonths},
	}
}

func stringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// storeHealth pairs the store's most recent reported revenue with its rent.
func (h *RenewalCardHandler) storeHealth(ctx context.Context, storeID string, contract *repository.Contract, monthlyRent float64) gin.H {
	metrics, err := h.storeMetricsRepo.List(ctx, "", "", storeID)
	if err != nil || len(metrics) == 0 {
		return nil
	}
	latest := metrics[0]

	result, err := renttosales.Calculate(renttosales.Input{
		Period:         latest.Period,
		HealthyCeiling: h.rentToSalesThreshold(ctx, "rent_to_sales_healthy_ceiling"),
		WarningCeiling: h.rentToSalesThreshold(ctx, "rent_to_sales_warning_ceiling"),
Stores: []renttosales.StoreInput{{
				StoreID: storeID, StoreCode: latest.StoreCode, StoreName: latest.StoreName,
				CashRent: &monthlyRent, RentCurrency: contract.Currency,
				Revenue: &latest.Revenue, RevenueCurrency: latest.Currency,
				RevenueSource: latest.Source, AreaSqm: contract.AreaSqm,
			}},
	})
	if err != nil || len(result.Stores) == 0 {
		return nil
	}
	store := result.Stores[0]

	return gin.H{
		"store_name":            store.StoreName,
		"period":                latest.Period,
		"revenue":               latest.Revenue,
		"gross_profit":          latest.GrossProfit,
		"rent_to_sales_percent": store.RentToSales,
		"sales_per_sqm":         store.SalesPerSqm,
		"status":                store.Status,
		"status_reason":         store.StatusReason,
		"revenue_source":        latest.Source,
	}
}

func (h *RenewalCardHandler) rentToSalesThreshold(ctx context.Context, key string) float64 {
	if h.systemSettingRepo == nil {
		return 0
	}
	return h.systemSettingRepo.GetFloat64(ctx, key, 0)
}

func requiredIntQuery(c *gin.Context, name string) (int, bool) {
	value, err := strconv.Atoi(c.Query(name))
	return value, err == nil
}

func floatQuery(c *gin.Context, name string, fallback float64) float64 {
	if value, err := strconv.ParseFloat(c.Query(name), 64); err == nil {
		return value
	}
	return fallback
}

func requiredFloatQuery(c *gin.Context, name string) (float64, bool) {
	value, err := strconv.ParseFloat(c.Query(name), 64)
	return value, err == nil
}

func valueOrZero(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func round2Handler(value float64) float64 {
	return math.Round(value*100) / 100
}
