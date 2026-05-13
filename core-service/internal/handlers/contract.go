package handlers

import (
	"net/http"
	"time"
	
	"github.com/gin-gonic/gin"
	"github.com/ifrs16/core-service/internal/middleware"
	"github.com/ifrs16/core-service/internal/repository"
)

type ContractHandler struct {
	contractRepo *repository.ContractRepository
}

func NewContractHandler(contractRepo *repository.ContractRepository) *ContractHandler {
	return &ContractHandler{contractRepo: contractRepo}
}

type ContractRequest struct {
	ContractNumber               string  `json:"contract_number" binding:"required"`
	ContractName                 string  `json:"contract_name" binding:"required"`
	LegalEntityID                string  `json:"legal_entity_id" binding:"required,uuid"`
	StoreID                      string  `json:"store_id" binding:"required,uuid"`
	LandlordID                   string  `json:"landlord_id" binding:"required,uuid"`
	Currency                     string  `json:"currency" binding:"required"`
	CommencementDate             string  `json:"commencement_date" binding:"required"`
	LeaseStartDate               string  `json:"lease_start_date" binding:"required"`
	LeaseEndDate                 string  `json:"lease_end_date" binding:"required"`
	AssetCategory                *string `json:"asset_category"`
	PropertyCategory             *string `json:"property_category"`
	SigningDate                  *string `json:"signing_date"`
	RenewalOptionDescription     *string `json:"renewal_option_description"`
	TerminationOptionDescription *string `json:"termination_option_description"`
	RenewalAssessment            *bool   `json:"renewal_assessment"`
	TerminationAssessment        *bool   `json:"termination_assessment"`
	DiscountRateType             *string `json:"discount_rate_type"`
	DiscountRateVersion          *string `json:"discount_rate_version"`
}

func (h *ContractHandler) Create(c *gin.Context) {
	var req ContractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	commencementDate, _ := time.Parse("2006-01-02", req.CommencementDate)
	leaseStartDate, _ := time.Parse("2006-01-02", req.LeaseStartDate)
	leaseEndDate, _ := time.Parse("2006-01-02", req.LeaseEndDate)
	
	var signingDate *time.Time
	if req.SigningDate != nil {
		sd, _ := time.Parse("2006-01-02", *req.SigningDate)
		signingDate = &sd
	}
	
	userID, _ := c.Get("user_id")
	var createdBy *string
	if id, ok := userID.(string); ok {
		createdBy = &id
	}
	
	contract := &repository.Contract{
		ContractNumber:               req.ContractNumber,
		ContractName:                 req.ContractName,
		LegalEntityID:                req.LegalEntityID,
		StoreID:                      req.StoreID,
		LandlordID:                   req.LandlordID,
		Currency:                     req.Currency,
		CommencementDate:             commencementDate,
		LeaseStartDate:               leaseStartDate,
		LeaseEndDate:                 leaseEndDate,
		AssetCategory:                req.AssetCategory,
		PropertyCategory:             req.PropertyCategory,
		SigningDate:                  signingDate,
		RenewalOptionDescription:     req.RenewalOptionDescription,
		TerminationOptionDescription: req.TerminationOptionDescription,
		RenewalAssessment:            req.RenewalAssessment,
		TerminationAssessment:        req.TerminationAssessment,
		DiscountRateType:             req.DiscountRateType,
		DiscountRateVersion:          req.DiscountRateVersion,
		CreatedBy:                    createdBy,
	}
	
	created, err := h.contractRepo.Create(c.Request.Context(), contract)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create contract: " + err.Error()})
		return
	}
	
	c.JSON(http.StatusCreated, created)
}

func (h *ContractHandler) GetAll(c *gin.Context) {
	legalEntityID := middleware.GetTenantID(c)
	contracts, err := h.contractRepo.GetAll(c.Request.Context(), legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list contracts: " + err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"data": contracts, "total": len(contracts)})
}

func (h *ContractHandler) GetByID(c *gin.Context) {
	id := c.Param("id")
	legalEntityID := middleware.GetTenantID(c)
	contract, err := h.contractRepo.GetByID(c.Request.Context(), id, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get contract: " + err.Error()})
		return
	}
	if contract == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
		return
	}
	c.JSON(http.StatusOK, contract)
}
