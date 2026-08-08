package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
)

type PaymentScheduleHandler struct {
	psRepo       *repository.PaymentScheduleRepository
	contractRepo *repository.ContractRepository
}

func NewPaymentScheduleHandler(psRepo *repository.PaymentScheduleRepository, contractRepo *repository.ContractRepository) *PaymentScheduleHandler {
	return &PaymentScheduleHandler{psRepo: psRepo, contractRepo: contractRepo}
}

type CreatePaymentScheduleRequest struct {
	ContractID            string   `json:"contract_id" binding:"required,uuid"`
	EffectiveStartDate    string   `json:"effective_start_date" binding:"required"`
	EffectiveEndDate      string   `json:"effective_end_date" binding:"required"`
	CoverageStartDate     string   `json:"coverage_start_date" binding:"required"`
	CoverageEndDate       string   `json:"coverage_end_date" binding:"required"`
	DueDate               string   `json:"due_date" binding:"required"`
	ActualPaymentDate     *string  `json:"actual_payment_date"`
	PaymentTiming         string   `json:"payment_timing" binding:"required,oneof=prepaid postpaid"`
	Amount                float64  `json:"amount" binding:"required,gt=0"`
	Currency              string   `json:"currency"`
	TaxAmount             *float64 `json:"tax_amount"`
	AmountType            string   `json:"amount_type" binding:"required"`
	IsFixed               bool     `json:"is_fixed"`
	IsVariable            bool     `json:"is_variable"`
	IsIndexAdjusted       bool     `json:"is_index_adjusted"`
	IsLeaseComponent      bool     `json:"is_lease_component"`
	IsNonLeaseComponent   bool     `json:"is_non_lease_component"`
	IncludedInLiabilityPV bool     `json:"included_in_liability_pv"`
}

func (h *PaymentScheduleHandler) Create(c *gin.Context) {
	var req CreatePaymentScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()
	contract, err := h.contractRepo.GetByID(ctx, req.ContractID, middleware.GetTenantID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify contract: " + err.Error()})
		return
	}
	if contract == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
		return
	}
	contractCurrency := strings.TrimSpace(contract.Currency)
	requestedCurrency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if contractCurrency == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "合同缺少币种，无法创建付款计划"})
		return
	}
	if requestedCurrency == "" {
		requestedCurrency = strings.ToUpper(contractCurrency)
	}
	if requestedCurrency != strings.ToUpper(contractCurrency) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "付款计划币种必须与合同币种一致"})
		return
	}

	esd, _ := time.Parse("2006-01-02", req.EffectiveStartDate)
	eed, _ := time.Parse("2006-01-02", req.EffectiveEndDate)
	csd, _ := time.Parse("2006-01-02", req.CoverageStartDate)
	ced, _ := time.Parse("2006-01-02", req.CoverageEndDate)
	dd, _ := time.Parse("2006-01-02", req.DueDate)

	var apd *time.Time
	if req.ActualPaymentDate != nil {
		t, _ := time.Parse("2006-01-02", *req.ActualPaymentDate)
		apd = &t
	}

	ps := &repository.PaymentSchedule{
		ContractID:            req.ContractID,
		EffectiveStartDate:    esd,
		EffectiveEndDate:      eed,
		CoverageStartDate:     csd,
		CoverageEndDate:       ced,
		DueDate:               dd,
		ActualPaymentDate:     apd,
		PaymentTiming:         req.PaymentTiming,
		Amount:                req.Amount,
		Currency:              requestedCurrency,
		TaxAmount:             req.TaxAmount,
		AmountType:            req.AmountType,
		IsFixed:               req.IsFixed,
		IsVariable:            req.IsVariable,
		IsIndexAdjusted:       req.IsIndexAdjusted,
		IsLeaseComponent:      req.IsLeaseComponent,
		IsNonLeaseComponent:   req.IsNonLeaseComponent,
		IncludedInLiabilityPV: req.IncludedInLiabilityPV,
	}
	result, err := h.psRepo.Create(ctx, ps)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *PaymentScheduleHandler) ListByContract(c *gin.Context) {
	contractID := c.Param("id")
	ctx := c.Request.Context()
	legalEntityID := middleware.GetTenantID(c)

	// Verify contract belongs to tenant
	contract, err := h.contractRepo.GetByID(ctx, contractID, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify contract: " + err.Error()})
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

	c.JSON(http.StatusOK, gin.H{
		"data":  schedules,
		"total": len(schedules),
	})
}
