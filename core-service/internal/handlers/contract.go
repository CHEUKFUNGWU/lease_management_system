package handlers

import (
	"net/http"
	"strings"
	"time"
	
	"github.com/gin-gonic/gin"
	"github.com/ifrs16/core-service/internal/middleware"
	"github.com/ifrs16/core-service/internal/repository"
	"github.com/ifrs16/core-service/internal/services/audit"
)

type ContractHandler struct {
	contractRepo *repository.ContractRepository
	auditLogger  *audit.Logger
}

func NewContractHandler(contractRepo *repository.ContractRepository, auditLogger *audit.Logger) *ContractHandler {
	return &ContractHandler{contractRepo: contractRepo, auditLogger: auditLogger}
}

type ContractRequest struct {
	ContractNumber               string  `json:"contract_number" binding:"required"`
	ContractName                 string  `json:"contract_name" binding:"required"`
	LegalEntityID                *string `json:"legal_entity_id,omitempty"`
	StoreID                      *string `json:"store_id,omitempty"`
	LandlordID                   *string `json:"landlord_id,omitempty"`
	LesseeName                   string  `json:"lessee_name"`
	LessorName                   string  `json:"lessor_name"`
	StoreName                    string  `json:"store_name"`
	StoreAddress                 string  `json:"store_address"`
	Tags                         string  `json:"tags"`
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
	DiscountRateValue            *float64 `json:"discount_rate_value"`
}

// UpdateContractRequest represents the editable fields for updating a contract.
type UpdateContractRequest struct {
	ContractNumber      string  `json:"contract_number"`
	ContractName        string  `json:"contract_name"`
	LegalEntityID       *string `json:"legal_entity_id,omitempty"`
	StoreID             *string `json:"store_id,omitempty"`
	LandlordID          *string `json:"landlord_id,omitempty"`
	LesseeName          string  `json:"lessee_name"`
	LessorName          string  `json:"lessor_name"`
	StoreName           string  `json:"store_name"`
	StoreAddress        string  `json:"store_address"`
	Tags                string  `json:"tags"`
	Currency            string  `json:"currency"`
	SigningDate         *string `json:"signing_date"`
	CommencementDate    string  `json:"commencement_date"`
	LeaseStartDate      string  `json:"lease_start_date"`
	LeaseEndDate        string  `json:"lease_end_date"`
	DiscountRateType    *string `json:"discount_rate_type"`
	DiscountRateVersion *string `json:"discount_rate_version"`
	DiscountRateValue   *float64 `json:"discount_rate_value"`
}

func normalizeTags(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	replacer := strings.NewReplacer(",", " ", "，", " ", ";", " ", "；", " ", "|", " ", "\n", " ", "\t", " ")
	normalized := replacer.Replace(raw)
	parts := strings.Fields(normalized)
	if len(parts) == 0 {
		return ""
	}
	seen := make(map[string]struct{})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag == "" {
			continue
		}
		if !strings.HasPrefix(tag, "#") {
			tag = "#" + tag
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		result = append(result, tag)
	}
	return strings.Join(result, ", ")
}

func normalizeDiscountRateValue(v *float64) *float64 {
	if v == nil {
		return nil
	}
	normalized := *v
	if normalized > 1 {
		normalized = normalized / 100
	}
	return &normalized
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
		Currency:                     req.Currency,
		CommencementDate:             commencementDate,
		LeaseStartDate:               leaseStartDate,
		LeaseEndDate:                 leaseEndDate,
		LesseeName:                   req.LesseeName,
		LessorName:                   req.LessorName,
		StoreName:                    req.StoreName,
		StoreAddress:                 req.StoreAddress,
		Tags:                         normalizeTags(req.Tags),
		AssetCategory:                req.AssetCategory,
		PropertyCategory:             req.PropertyCategory,
		SigningDate:                  signingDate,
		RenewalOptionDescription:     req.RenewalOptionDescription,
		TerminationOptionDescription: req.TerminationOptionDescription,
		RenewalAssessment:            req.RenewalAssessment,
		TerminationAssessment:        req.TerminationAssessment,
		DiscountRateType:             req.DiscountRateType,
		DiscountRateVersion:          req.DiscountRateVersion,
		DiscountRateValue:            normalizeDiscountRateValue(req.DiscountRateValue),
		CreatedBy:                    createdBy,
	}

	// Auto-fill legal_entity_id from JWT tenant context if not provided
	if req.LegalEntityID != nil && *req.LegalEntityID != "" {
		contract.LegalEntityID = req.LegalEntityID
	} else {
		// Try to get from JWT context
		if jwtLegalEntityID := middleware.GetTenantID(c); jwtLegalEntityID != "" {
			contract.LegalEntityID = &jwtLegalEntityID
		}
	}
	if req.StoreID != nil && *req.StoreID != "" {
		contract.StoreID = req.StoreID
	}
	if req.LandlordID != nil && *req.LandlordID != "" {
		contract.LandlordID = req.LandlordID
	}
	
	created, err := h.contractRepo.Create(c.Request.Context(), contract)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create contract: " + err.Error()})
		return
	}

	// Audit log: contract created
	if h.auditLogger != nil {
		uid, _ := c.Get("user_id")
		uidStr, _ := uid.(string)
		h.auditLogger.Log(c.Request.Context(), "lease_contracts", created.ID, "create", nil, created, uidStr, c)
	}
	
	c.JSON(http.StatusCreated, created)
}

func (h *ContractHandler) GetAll(c *gin.Context) {
	legalEntityID := middleware.GetTenantID(c)
	
	filter := repository.ListContractsFilter{
		Search:    c.Query("search"),
		Status:    c.Query("status"),
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
	}
	
	contracts, err := h.contractRepo.List(c.Request.Context(), legalEntityID, filter)
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

func (h *ContractHandler) Update(c *gin.Context) {
	id := c.Param("id")
	legalEntityID := middleware.GetTenantID(c)

	// Get current contract to verify it exists and check status
	existing, err := h.contractRepo.GetByID(c.Request.Context(), id, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get contract: " + err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
		return
	}

	// Only allow editing if draft or rejected
	if existing.ApprovalStatus != "draft" && existing.ApprovalStatus != "rejected" {
		c.JSON(http.StatusForbidden, gin.H{"error": "contract cannot be edited in '" + existing.ApprovalStatus + "' status, only draft or rejected"})
		return
	}

	var req UpdateContractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse dates
	if req.CommencementDate == "" || req.LeaseStartDate == "" || req.LeaseEndDate == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "commencement_date, lease_start_date, lease_end_date are required"})
		return
	}
	commencementDate, err := time.Parse("2006-01-02", req.CommencementDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid commencement_date format, expected YYYY-MM-DD"})
		return
	}
	leaseStartDate, err := time.Parse("2006-01-02", req.LeaseStartDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lease_start_date format, expected YYYY-MM-DD"})
		return
	}
	leaseEndDate, err := time.Parse("2006-01-02", req.LeaseEndDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lease_end_date format, expected YYYY-MM-DD"})
		return
	}

	var signingDate *time.Time
	if req.SigningDate != nil && *req.SigningDate != "" {
		sd, err := time.Parse("2006-01-02", *req.SigningDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid signing_date format"})
			return
		}
		signingDate = &sd
	}

	// Get user_id from context
	userID, _ := c.Get("user_id")
	var updatedBy string
	if uid, ok := userID.(string); ok {
		updatedBy = uid
	}

	// Build update contract
	contract := &repository.Contract{
		ID:                    id,
		ContractNumber:        req.ContractNumber,
		ContractName:          req.ContractName,
		LegalEntityID:         req.LegalEntityID,
		StoreID:               req.StoreID,
		LandlordID:            req.LandlordID,
		LesseeName:            req.LesseeName,
		LessorName:            req.LessorName,
		StoreName:             req.StoreName,
		StoreAddress:          req.StoreAddress,
		Tags:                  normalizeTags(req.Tags),
		Currency:              req.Currency,
		SigningDate:           signingDate,
		CommencementDate:      commencementDate,
		LeaseStartDate:        leaseStartDate,
		LeaseEndDate:          leaseEndDate,
		DiscountRateType:      req.DiscountRateType,
		DiscountRateVersion:   req.DiscountRateVersion,
		DiscountRateValue:     normalizeDiscountRateValue(req.DiscountRateValue),
		Status:                existing.Status,
	}

	if err := h.contractRepo.Update(c.Request.Context(), contract, legalEntityID, updatedBy); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update contract: " + err.Error()})
		return
	}

	// Return updated contract
	updated, _ := h.contractRepo.GetByID(c.Request.Context(), id, legalEntityID)
	if updated == nil {
		c.JSON(http.StatusOK, gin.H{"message": "contract updated successfully"})
		return
	}

	// Audit log: contract updated
	if h.auditLogger != nil {
		uid, _ := c.Get("user_id")
		uidStr, _ := uid.(string)
		h.auditLogger.Log(c.Request.Context(), "lease_contracts", id, "update", existing, updated, uidStr, c)
	}

	c.JSON(http.StatusOK, updated)
}
