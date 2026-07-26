package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/cashflow"
)

type CashflowScenarioHandler struct {
	contractRepo *repository.ContractRepository
	psRepo       *repository.PaymentScheduleRepository
}

func NewCashflowScenarioHandler(contractRepo *repository.ContractRepository, psRepo *repository.PaymentScheduleRepository) *CashflowScenarioHandler {
	return &CashflowScenarioHandler{contractRepo: contractRepo, psRepo: psRepo}
}

type CashflowScenarioRequest struct {
	AsOf          string `json:"as_of"`
	HorizonMonths int    `json:"horizon_months"`
	// Scenarios are run against the same portfolio so they can be read side by
	// side; comparing a plan against the do-nothing case is the whole point.
	Scenarios []cashflow.Scenario `json:"scenarios" binding:"required"`
}

// Scenario projects portfolio cash outflow under one or more estates plans.
// POST /reports/cashflow-scenario
func (h *CashflowScenarioHandler) Scenario(c *gin.Context) {
	var req CashflowScenarioRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Scenarios) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请至少提供一个情景"})
		return
	}

	asOf := time.Now()
	if req.AsOf != "" {
		parsed, err := time.Parse("2006-01-02", req.AsOf)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "基准日格式应为 YYYY-MM-DD"})
			return
		}
		asOf = parsed
	}

	ctx := c.Request.Context()
	contracts, err := h.contractRepo.List(ctx, middleware.GetTenantID(c), repository.ListContractsFilter{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	leases := make([]cashflow.Lease, 0, len(contracts))
	currency := ""
	mixedCurrency := false
	for _, contract := range contracts {
		schedules, err := h.psRepo.GetByContractID(ctx, contract.ID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if currency == "" {
			currency = contract.Currency
		} else if currency != contract.Currency {
			mixedCurrency = true
		}
		leases = append(leases, cashflow.Lease{
			ContractID: contract.ID, ContractNumber: contract.ContractNumber,
			ContractName: contract.ContractName, Currency: contract.Currency,
			LeaseEndDate: contract.LeaseEndDate,
			Payments:     repository.ToIFRS16Payments(schedules),
		})
	}

	results := make([]cashflow.Result, 0, len(req.Scenarios))
	for _, scenario := range req.Scenarios {
		projected, err := cashflow.Project(cashflow.Input{
			AsOf: asOf, Currency: currency, HorizonMonths: req.HorizonMonths,
			Leases: leases, Scenario: scenario,
		})
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		results = append(results, projected)
	}

	response := gin.H{"as_of": asOf.Format("2006-01-02"), "results": results}
	if mixedCurrency {
		// Adding rents in different currencies produces a number in none of
		// them. The projection is still useful per scenario, but the caller
		// needs to know the totals are not a single currency.
		response["currency_warning"] = "组合内存在多种币种，合计金额不代表任何单一币种，请按币种分别查看"
	}
	c.JSON(http.StatusOK, response)
}
