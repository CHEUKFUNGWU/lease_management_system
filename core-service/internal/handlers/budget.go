package handlers

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/budget"
	contractsvc "github.com/lease-management-system/core-service/internal/services/contracts"
	ifrs16svc "github.com/lease-management-system/core-service/internal/services/ifrs16"
)

type BudgetHandler struct {
	budgetRepo   *repository.BudgetRepository
	contractRepo *repository.ContractRepository
	psRepo       *repository.PaymentScheduleRepository
	settingRepo  *repository.SystemSettingRepository
}

func NewBudgetHandler(
	budgetRepo *repository.BudgetRepository,
	contractRepo *repository.ContractRepository,
	psRepo *repository.PaymentScheduleRepository,
	settingRepo *repository.SystemSettingRepository,
) *BudgetHandler {
	return &BudgetHandler{budgetRepo: budgetRepo, contractRepo: contractRepo, psRepo: psRepo, settingRepo: settingRepo}
}

type createBudgetVersionRequest struct {
	Name          string `json:"name" binding:"required"`
	VersionType   string `json:"version_type"`
	Source        string `json:"source"`
	CoverageScope string `json:"coverage_scope"`
	IsOfficial    bool   `json:"is_official"`
	FromPeriod    string `json:"from_period" binding:"required"`
	ToPeriod      string `json:"to_period" binding:"required"`
}

type varianceActionItem struct {
	ContractID  string `json:"contract_id" binding:"required"`
	Explanation string `json:"explanation"`
	OwnerName   string `json:"owner_name"`
	DueDate     string `json:"due_date"`
	Status      string `json:"status"`
}

type upsertVarianceActionsRequest struct {
	Period string               `json:"period" binding:"required"`
	Items  []varianceActionItem `json:"items" binding:"required"`
}

// CreateVersion freezes today's measured forward schedule as a budget.
//
// The budget is the measurement itself rather than a separately keyed plan:
// what the engine projects today is exactly what "as planned" means, so the
// later comparison is like-for-like.
func (h *BudgetHandler) CreateVersion(c *gin.Context) {
	var req createBudgetVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写预算版本名称与起止期间"})
		return
	}
	from, err := time.Parse("2006-01", strings.TrimSpace(req.FromPeriod))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "起始期间格式应为 YYYY-MM"})
		return
	}
	to, err := time.Parse("2006-01", strings.TrimSpace(req.ToPeriod))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "结束期间格式应为 YYYY-MM"})
		return
	}
	if to.Before(from) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "结束期间不能早于起始期间"})
		return
	}
	versionType := strings.TrimSpace(req.VersionType)
	if versionType == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供版本类型：budget、forecast 或 scenario"})
		return
	}
	if versionType != "budget" && versionType != "forecast" && versionType != "scenario" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "版本类型只能是 budget、forecast 或 scenario"})
		return
	}
	source := strings.TrimSpace(req.Source)
	if source == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供版本来源"})
		return
	}

	ctx := c.Request.Context()
	legalEntityID := middleware.GetTenantID(c)
	contracts, err := h.contractRepo.GetByStatuses(ctx, []string{"approved"}, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	globalRate := h.settingRepo.GetFloat64(ctx, "global_discount_rate", 0)

	lines := make([]repository.BudgetLine, 0)
	skipped := 0
	for _, contract := range contracts {
		schedules, err := h.psRepo.GetByContractID(ctx, contract.ID)
		if err != nil || len(schedules) == 0 {
			skipped++
			continue
		}
		rate, _, err := contractsvc.ResolveDiscountRateValues(0, globalRate, contract.DiscountRateValue, contract.LeaseScope)
		if err != nil {
			skipped++
			continue
		}
		payments := repository.ToIFRS16Payments(schedules)
		result, err := ifrs16svc.Calculate(ifrs16svc.LeaseCalculation{
			CommencementDate: contract.CommencementDate,
			LeaseEndDate:     contract.LeaseEndDate,
			LeaseScope:       contract.LeaseScope,
			DiscountRate:     rate,
			Payments:         payments,
			PrepaidRent: ifrs16svc.CalculatePrepaidRent(ifrs16svc.LeaseCalculation{
				CommencementDate: contract.CommencementDate, Payments: payments,
			}),
		})
		if err != nil {
			skipped++
			continue
		}
		for _, monthly := range result.MonthlySummary {
			period := time.Date(monthly.Year, time.Month(monthly.Month), 1, 0, 0, 0, 0, time.UTC)
			if period.Before(from) || period.After(to) {
				continue
			}
			lines = append(lines, repository.BudgetLine{
				ContractID:       contract.ID,
				AccountingPeriod: period.Format("2006-01"),
				Currency:         contract.Currency,
				InterestExpense:  monthly.InterestExpense,
				Depreciation:     monthly.Depreciation,
				TotalPayment:     monthly.TotalPayments,
				ClosingLiability: monthly.ClosingLiability,
			})
		}
	}

	uid, _ := c.Get("user_id")
	uidStr, _ := uid.(string)
	var createdBy, entityID *string
	if uidStr != "" {
		createdBy = &uidStr
	}
	if legalEntityID != "" {
		entityID = &legalEntityID
	}

	version, err := h.budgetRepo.CreateVersion(ctx, &repository.BudgetVersion{
		Name: req.Name, LegalEntityID: entityID, VersionType: versionType,
		Source: source, CoverageScope: strings.TrimSpace(req.CoverageScope), IsOfficial: req.IsOfficial,
		AsOfPeriod: time.Now().Format("2006-01"),
		FromPeriod: from.Format("2006-01"), ToPeriod: to.Format("2006-01"),
		CreatedBy: createdBy,
	}, lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "预算版本已固化", "data": version,
		"line_count": len(lines), "skipped_contracts": skipped,
	})
}

func (h *BudgetHandler) ListVersions(c *gin.Context) {
	versions, err := h.budgetRepo.ListVersions(c.Request.Context(), middleware.GetTenantID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": versions, "total": len(versions)})
}

// Variance compares a budget version with the period's actuals and explains the
// difference by cause.
func (h *BudgetHandler) Variance(c *gin.Context) {
	versionID := c.Param("id")
	period := strings.TrimSpace(c.Query("period"))
	if period == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供会计期间(period=YYYY-MM)"})
		return
	}

	ctx := c.Request.Context()
	legalEntityID := middleware.GetTenantID(c)
	version, err := h.budgetRepo.GetVersion(ctx, versionID, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if version == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "budget version not found"})
		return
	}

	budgetLines, err := h.budgetRepo.LinesForPeriod(ctx, versionID, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	actuals, err := h.budgetRepo.ActualsForPeriod(ctx, legalEntityID, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if _, err := requireSingleCurrency(budgetLines, actuals); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	events, err := h.budgetRepo.EventTypesByContract(ctx, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	fx, err := h.budgetRepo.FXByContract(ctx, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	materialityThreshold := h.budgetPolicySetting(ctx, "budget_variance_materiality_threshold")
	tieOutTolerance := h.budgetPolicySetting(ctx, "budget_tie_out_tolerance")
	result := budget.Explain(budget.Input{
		Period:               period,
		Budget:               toContractPeriods(budgetLines),
		Actual:               toContractPeriods(actuals),
		MaterialityThreshold: materialityThreshold,
		TieOutTolerance:      tieOutTolerance,
		EventsByContract:     events,
		FXByContract:         fx,
	})
	actions, err := h.budgetRepo.ListVarianceActions(ctx, versionID, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	today := time.Now().UTC()
	for i := range result.ByContract {
		row := &result.ByContract[i]
		action, exists := actions[row.ContractID]
		if !exists {
			row.ActionStatus = "open"
			if math.Abs(row.Variance) > materialityThreshold {
				result.OpenActionAmount += math.Abs(row.Variance)
				result.OpenActionCount++
			}
			continue
		}
		row.Explanation = action.Explanation
		row.OwnerName = action.OwnerName
		row.ActionStatus = action.Status
		if action.DueDate != nil {
			row.DueDate = action.DueDate.Format("2006-01-02")
			row.IsOverdue = action.DueDate.Before(today) && action.Status != "resolved" && action.Status != "accepted"
		}
		if strings.TrimSpace(action.Explanation) != "" && math.Abs(row.Variance) > materialityThreshold {
			result.ExplainedCount++
		}
		if action.Status != "resolved" && action.Status != "accepted" && math.Abs(row.Variance) > materialityThreshold {
			result.OpenActionAmount += math.Abs(row.Variance)
			result.OpenActionCount++
		}
	}
	if result.VarianceCount > 0 {
		result.ExplanationCoverage = math.Round(float64(result.ExplainedCount)/float64(result.VarianceCount)*10000) / 10000
	}
	c.JSON(http.StatusOK, gin.H{
		"version":      version,
		"actual_basis": "measurement_results",
		"result":       result,
	})
}

func (h *BudgetHandler) budgetPolicySetting(ctx context.Context, key string) float64 {
	if h.settingRepo == nil {
		return 0
	}
	return h.settingRepo.GetFloat64(ctx, key, 0)
}

// VarianceActions saves the human explanation layer without changing the
// calculated bridge.
func (h *BudgetHandler) VarianceActions(c *gin.Context) {
	versionID := c.Param("id")
	var req upsertVarianceActionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供期间和差异行动项"})
		return
	}
	if _, err := time.Parse("2006-01", strings.TrimSpace(req.Period)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "期间格式应为 YYYY-MM"})
		return
	}
	if len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请至少提供一条差异行动项"})
		return
	}
	if version, err := h.budgetRepo.GetVersion(c.Request.Context(), versionID, middleware.GetTenantID(c)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	} else if version == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "budget version not found"})
		return
	}

	items := make([]repository.VarianceAction, 0, len(req.Items))
	for _, item := range req.Items {
		allowed, err := h.budgetRepo.ContractAllowedForVersion(c.Request.Context(), versionID, item.ContractID, middleware.GetTenantID(c))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !allowed {
			c.JSON(http.StatusForbidden, gin.H{"error": "差异行动项包含不在当前版本权限范围内的合同"})
			return
		}
		action := repository.VarianceAction{
			ContractID: item.ContractID, Explanation: strings.TrimSpace(item.Explanation),
			OwnerName: strings.TrimSpace(item.OwnerName), Status: strings.TrimSpace(item.Status),
		}
		if action.Status == "" {
			action.Status = "open"
		}
		if action.Status != "open" && action.Status != "in_progress" && action.Status != "resolved" && action.Status != "accepted" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "差异状态只能是 open、in_progress、resolved 或 accepted"})
			return
		}
		if strings.TrimSpace(item.DueDate) != "" {
			due, err := time.Parse("2006-01-02", strings.TrimSpace(item.DueDate))
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "截止日期格式应为 YYYY-MM-DD"})
				return
			}
			action.DueDate = &due
		}
		items = append(items, action)
	}
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	saved, err := h.budgetRepo.UpsertVarianceActions(c.Request.Context(), versionID, req.Period, userIDStr, items)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": saved, "saved_count": len(saved)})
}

// CompareVersions compares two plan-like versions or a plan against Actual.
func (h *BudgetHandler) CompareVersions(c *gin.Context) {
	leftID := strings.TrimSpace(c.Query("left_id"))
	rightID := strings.TrimSpace(c.Query("right_id"))
	period := strings.TrimSpace(c.Query("period"))
	if leftID == "" || rightID == "" || period == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 left_id、right_id 和 period=YYYY-MM"})
		return
	}
	ctx := c.Request.Context()
	legalEntityID := middleware.GetTenantID(c)
	leftVersion, err := h.budgetRepo.GetVersion(ctx, leftID, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if leftVersion == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "left budget version not found"})
		return
	}
	leftLines, err := h.budgetRepo.LinesForPeriod(ctx, leftID, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	left := toContractPeriods(leftLines)
	leftLabel := leftVersion.Name
	var right []budget.ContractPeriod
	var rightLines []repository.BudgetContractPeriod
	rightBasis := gin.H{"kind": "version"}
	rightLabel := "Actual"
	if rightID == "actual" {
		actuals, err := h.budgetRepo.ActualsForPeriod(ctx, legalEntityID, period)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		rightLines = actuals
		right = toContractPeriods(actuals)
		rightBasis = gin.H{"kind": "actual", "source": "measurement_results"}
	} else {
		rightVersion, err := h.budgetRepo.GetVersion(ctx, rightID, legalEntityID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if rightVersion == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "right budget version not found"})
			return
		}
		rightLines, err = h.budgetRepo.LinesForPeriod(ctx, rightID, period)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		right = toContractPeriods(rightLines)
		rightLabel = rightVersion.Name
		rightBasis = gin.H{"kind": "version", "version": rightVersion}
	}
	currency, err := requireSingleCurrency(leftLines, rightLines)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	comparison := budget.CompareVersions(
		period,
		leftLabel,
		rightLabel,
		left,
		right,
		h.budgetPolicySetting(ctx, "budget_tie_out_tolerance"),
	)
	c.JSON(http.StatusOK, gin.H{
		"period":      period,
		"currency":    currency,
		"left_basis":  gin.H{"kind": "version", "version": leftVersion},
		"right_basis": rightBasis,
		"comparison":  comparison,
	})
}

// ManagementBrief returns the three management views together, so a reader
// does not have to manually run three separate comparisons to see plan drift.
func (h *BudgetHandler) ManagementBrief(c *gin.Context) {
	budgetID := strings.TrimSpace(c.Query("budget_id"))
	forecastID := strings.TrimSpace(c.Query("forecast_id"))
	period := strings.TrimSpace(c.Query("period"))
	if budgetID == "" || forecastID == "" || period == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 budget_id、forecast_id 和 period=YYYY-MM"})
		return
	}
	ctx := c.Request.Context()
	legalEntityID := middleware.GetTenantID(c)
	budgetVersion, err := h.budgetRepo.GetVersion(ctx, budgetID, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	forecastVersion, err := h.budgetRepo.GetVersion(ctx, forecastID, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if budgetVersion == nil || forecastVersion == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "budget or forecast version not found"})
		return
	}
	budgetLines, err := h.budgetRepo.LinesForPeriod(ctx, budgetID, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	forecastLines, err := h.budgetRepo.LinesForPeriod(ctx, forecastID, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	actuals, err := h.budgetRepo.ActualsForPeriod(ctx, legalEntityID, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	currency, err := requireSingleCurrency(budgetLines, forecastLines, actuals)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	budgetTotal := sumLeaseCost(toContractPeriods(budgetLines))
	forecastTotal := sumLeaseCost(toContractPeriods(forecastLines))
	actualTotal := sumLeaseCost(toContractPeriods(actuals))
	c.JSON(http.StatusOK, gin.H{
		"period":             period,
		"currency":           currency,
		"budget":             gin.H{"version": budgetVersion, "total": budgetTotal},
		"forecast":           gin.H{"version": forecastVersion, "total": forecastTotal},
		"actual":             gin.H{"source": "measurement_results", "total": actualTotal},
		"forecast_vs_budget": math.Round((forecastTotal-budgetTotal)*100) / 100,
		"actual_vs_budget":   math.Round((actualTotal-budgetTotal)*100) / 100,
		"actual_vs_forecast": math.Round((actualTotal-forecastTotal)*100) / 100,
	})
}

func sumLeaseCost(rows []budget.ContractPeriod) float64 {
	var total float64
	for _, row := range rows {
		total += row.LeaseCost
	}
	return math.Round(total*100) / 100
}

func requireSingleCurrency(groups ...[]repository.BudgetContractPeriod) (string, error) {
	currency := ""
	for _, rows := range groups {
		for _, row := range rows {
			rowCurrency := strings.TrimSpace(row.Currency)
			if rowCurrency == "" {
				return "", fmt.Errorf("比较范围存在缺失币种，不能进行金额汇总")
			}
			if currency == "" {
				currency = rowCurrency
				continue
			}
			if currency != rowCurrency {
				return "", fmt.Errorf("比较范围包含多个币种（%s、%s），请先执行明确的汇率换算", currency, rowCurrency)
			}
		}
	}
	return currency, nil
}

func toContractPeriods(rows []repository.BudgetContractPeriod) []budget.ContractPeriod {
	items := make([]budget.ContractPeriod, 0, len(rows))
	for _, row := range rows {
		items = append(items, budget.ContractPeriod{
			ContractID: row.ContractID, ContractNumber: row.ContractNumber,
			ContractName: row.ContractName, Currency: row.Currency, LeaseCost: row.LeaseCost,
			TotalPayment: row.TotalPayment,
		})
	}
	return items
}
