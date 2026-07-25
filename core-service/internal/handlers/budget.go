package handlers

import (
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
	Name       string `json:"name" binding:"required"`
	FromPeriod string `json:"from_period" binding:"required"`
	ToPeriod   string `json:"to_period" binding:"required"`
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
		Name: req.Name, LegalEntityID: entityID,
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

	result := budget.Explain(budget.Input{
		Period:           period,
		Budget:           toContractPeriods(budgetLines),
		Actual:           toContractPeriods(actuals),
		EventsByContract: events,
		FXByContract:     fx,
	})
	c.JSON(http.StatusOK, result)
}

func toContractPeriods(rows []repository.BudgetContractPeriod) []budget.ContractPeriod {
	items := make([]budget.ContractPeriod, 0, len(rows))
	for _, row := range rows {
		items = append(items, budget.ContractPeriod{
			ContractID: row.ContractID, ContractNumber: row.ContractNumber,
			ContractName: row.ContractName, Currency: row.Currency, LeaseCost: row.LeaseCost,
		})
	}
	return items
}
