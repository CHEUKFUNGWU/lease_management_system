package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	contractsvc "github.com/lease-management-system/core-service/internal/services/contracts"
	ifrs16svc "github.com/lease-management-system/core-service/internal/services/ifrs16"
)

type CalculationHandler struct {
	contractRepo      *repository.ContractRepository
	psRepo            *repository.PaymentScheduleRepository
	systemSettingRepo *repository.SystemSettingRepository
}

func NewCalculationHandler(contractRepo *repository.ContractRepository, psRepo *repository.PaymentScheduleRepository, systemSettingRepo *repository.SystemSettingRepository) *CalculationHandler {
	return &CalculationHandler{contractRepo: contractRepo, psRepo: psRepo, systemSettingRepo: systemSettingRepo}
}

type CalculateRequest struct {
	ContractID   string  `json:"contract_id" binding:"required,uuid"`
	DiscountRate float64 `json:"discount_rate" binding:"omitempty,gt=0,lte=1"`
}

type CalculateResponse struct {
	ContractID       string                   `json:"contract_id"`
	LeaseScope       string                   `json:"lease_scope"`
	MeasurementBasis string                   `json:"measurement_basis"`
	InitialLiability float64                  `json:"initial_liability"`
	InitialROUAsset  float64                  `json:"initial_rou_asset"`
	TotalDays        int                      `json:"total_days"`
	MonthlySummary   []ifrs16svc.MonthlyEntry `json:"monthly_summary"`
}

func (h *CalculationHandler) Calculate(c *gin.Context) {
	var req CalculateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	legalEntityID := middleware.GetTenantID(c)

	// Get contract with tenant isolation
	contract, err := h.contractRepo.GetByID(ctx, req.ContractID, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get contract: " + err.Error()})
		return
	}
	if contract == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
		return
	}

	// Load payment schedules from database
	schedules, err := h.psRepo.GetByContractID(c.Request.Context(), req.ContractID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get payment schedules: " + err.Error()})
		return
	}

	if len(schedules) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "payment schedules are required for calculation"})
		return
	}
	payments := repository.ToIFRS16Payments(schedules)

	discountRate, _, err := contractsvc.ResolveDiscountRate(ctx, req.DiscountRate, h.systemSettingRepo, contract)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "discount_rate_missing": true})
		return
	}

	calculation := ifrs16svc.LeaseCalculation{
		CommencementDate: contract.CommencementDate,
		LeaseEndDate:     contract.LeaseEndDate,
		LeaseScope:       contract.LeaseScope,
		DiscountRate:     discountRate,
		Payments:         payments,
		PrepaidRent: ifrs16svc.CalculatePrepaidRent(ifrs16svc.LeaseCalculation{
			CommencementDate: contract.CommencementDate,
			Payments:         payments,
		}),
	}

	result, err := ifrs16svc.Calculate(calculation)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "calculation failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, CalculateResponse{
		ContractID:       req.ContractID,
		LeaseScope:       result.LeaseScope,
		MeasurementBasis: result.MeasurementBasis,
		InitialLiability: result.InitialLiability,
		InitialROUAsset:  result.InitialROUAsset,
		TotalDays:        len(result.DailyAmortization),
		MonthlySummary:   result.MonthlySummary,
	})
}

func (h *CalculationHandler) GetAmortizationSchedule(c *gin.Context) {
	contractID := c.Param("id")
	legalEntityID := middleware.GetTenantID(c)

	contract, err := h.contractRepo.GetByID(c.Request.Context(), contractID, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get contract: " + err.Error()})
		return
	}
	if contract == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
		return
	}

	// Return contract dates for now
	// In production, would return full schedule from database
	c.JSON(http.StatusOK, gin.H{
		"contract_id":       contractID,
		"commencement_date": contract.CommencementDate.Format("2006-01-02"),
		"lease_end_date":    contract.LeaseEndDate.Format("2006-01-02"),
		"message":           "Full amortization schedule would be returned here",
	})
}
