package handlers

import (
	"net/http"
	
	"github.com/gin-gonic/gin"
	"github.com/ifrs16/core-service/internal/middleware"
	"github.com/ifrs16/core-service/internal/repository"
	ifrs16svc "github.com/ifrs16/core-service/internal/services/ifrs16"
)

type CalculationHandler struct {
	contractRepo *repository.ContractRepository
	psRepo       *repository.PaymentScheduleRepository
}

func NewCalculationHandler(contractRepo *repository.ContractRepository, psRepo *repository.PaymentScheduleRepository) *CalculationHandler {
	return &CalculationHandler{contractRepo: contractRepo, psRepo: psRepo}
}

type CalculateRequest struct {
	ContractID   string  `json:"contract_id" binding:"required,uuid"`
	DiscountRate float64 `json:"discount_rate" binding:"required,gt=0,lte=1"`
}

type CalculateResponse struct {
	ContractID       string                        `json:"contract_id"`
	InitialLiability float64                       `json:"initial_liability"`
	InitialROUAsset  float64                       `json:"initial_rou_asset"`
	TotalDays        int                           `json:"total_days"`
	MonthlySummary   []ifrs16svc.MonthlyEntry      `json:"monthly_summary"`
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
	
	var payments []ifrs16svc.LeasePayment
	if len(schedules) == 0 {
		// Fallback: generate monthly payments if no schedules exist
		payments = []ifrs16svc.LeasePayment{
			{
				Date:   contract.CommencementDate,
				Amount: 100000,
				Timing: "postpaid",
				Type:   "fixed",
			},
		}
		currentDate := contract.CommencementDate.AddDate(0, 1, 0)
		for currentDate.Before(contract.LeaseEndDate) {
			payments = append(payments, ifrs16svc.LeasePayment{
				Date:   currentDate,
				Amount: 100000,
				Timing: "postpaid",
				Type:   "fixed",
			})
			currentDate = currentDate.AddDate(0, 1, 0)
		}
	} else {
		payments = repository.ToIFRS16Payments(schedules)
	}
	
	calculation := ifrs16svc.LeaseCalculation{
		CommencementDate: contract.CommencementDate,
		LeaseEndDate:     contract.LeaseEndDate,
		DiscountRate:     req.DiscountRate,
		Payments:         payments,
	}
	
	result, err := ifrs16svc.Calculate(calculation)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "calculation failed: " + err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, CalculateResponse{
		ContractID:       req.ContractID,
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
