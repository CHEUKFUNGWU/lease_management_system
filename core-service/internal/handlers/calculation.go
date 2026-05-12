package handlers

import (
	"net/http"
	"time"
	
	"github.com/gin-gonic/gin"
	"github.com/ifrs16/core-service/internal/repository"
	"github.com/ifrs16/core-service/internal/services/ifrs16"
)

type CalculationHandler struct {
	contractRepo *repository.ContractRepository
}

func NewCalculationHandler(contractRepo *repository.ContractRepository) *CalculationHandler {
	return &CalculationHandler{contractRepo: contractRepo}
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
	MonthlySummary   []ifrs16.MonthlyEntry         `json:"monthly_summary"`
}

func (h *CalculationHandler) Calculate(c *gin.Context) {
	var req CalculateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	// Get contract
	contract, err := h.contractRepo.GetByID(c.Request.Context(), req.ContractID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get contract: " + err.Error()})
		return
	}
	if contract == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
		return
	}
	
	// For MVP, create sample payments based on contract
	// In production, this would come from lease_payment_schedules table
	payments := []ifrs16.LeasePayment{
		{
			Date:   contract.CommencementDate,
			Amount: 100000,
			Timing: "postpaid",
			Type:   "fixed",
		},
	}
	
	// Add monthly payments
	currentDate := contract.CommencementDate.AddDate(0, 1, 0)
	for currentDate.Before(contract.LeaseEndDate) {
		payments = append(payments, ifrs16.LeasePayment{
			Date:   currentDate,
			Amount: 100000,
			Timing: "postpaid",
			Type:   "fixed",
		})
		currentDate = currentDate.AddDate(0, 1, 0)
	}
	
	calculation := ifrs16.LeaseCalculation{
		CommencementDate: contract.CommencementDate,
		LeaseEndDate:     contract.LeaseEndDate,
		DiscountRate:     req.DiscountRate,
		Payments:         payments,
	}
	
	result, err := ifrs16.Calculate(calculation)
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
	
	contract, err := h.contractRepo.GetByID(c.Request.Context(), contractID)
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
