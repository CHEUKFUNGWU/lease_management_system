package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
)

type ContractHandler struct {
	contractRepo *repository.ContractRepository
	auditLogger  *audit.Logger
}

func NewContractHandler(contractRepo *repository.ContractRepository, auditLogger *audit.Logger) *ContractHandler {
	return &ContractHandler{contractRepo: contractRepo, auditLogger: auditLogger}
}

type ContractRequest struct {
	ContractNumber               string   `json:"contract_number" binding:"required"`
	ContractName                 string   `json:"contract_name" binding:"required"`
	LegalEntityID                *string  `json:"legal_entity_id,omitempty"`
	StoreID                      *string  `json:"store_id,omitempty"`
	LandlordID                   *string  `json:"landlord_id,omitempty"`
	LesseeName                   string   `json:"lessee_name"`
	LessorName                   string   `json:"lessor_name"`
	StoreName                    string   `json:"store_name"`
	StoreAddress                 string   `json:"store_address"`
	Tags                         string   `json:"tags"`
	Currency                     string   `json:"currency" binding:"required"`
	AssetType                    string   `json:"asset_type"`
	CommencementDate             string   `json:"commencement_date" binding:"required"`
	LeaseStartDate               string   `json:"lease_start_date" binding:"required"`
	LeaseEndDate                 string   `json:"lease_end_date" binding:"required"`
	AssetCategory                *string  `json:"asset_category"`
	PropertyCategory             *string  `json:"property_category"`
	SigningDate                  *string  `json:"signing_date"`
	RenewalOptionDescription     *string  `json:"renewal_option_description"`
	TerminationOptionDescription *string  `json:"termination_option_description"`
	RenewalAssessment            *bool    `json:"renewal_assessment"`
	TerminationAssessment        *bool    `json:"termination_assessment"`
	DiscountRateType             *string  `json:"discount_rate_type"`
	DiscountRateVersion          *string  `json:"discount_rate_version"`
	DiscountRateValue            *float64 `json:"discount_rate_value"`
	LeaseScope                   string   `json:"lease_scope"`
	ExemptionReason              *string  `json:"exemption_reason"`
	ScopeSource                  *string  `json:"scope_source"`
	ScopeConfidence              *float64 `json:"scope_confidence"`
}

// UpdateContractRequest represents the editable fields for updating a contract.
type UpdateContractRequest struct {
	ContractNumber      string   `json:"contract_number"`
	ContractName        string   `json:"contract_name"`
	LegalEntityID       *string  `json:"legal_entity_id,omitempty"`
	StoreID             *string  `json:"store_id,omitempty"`
	LandlordID          *string  `json:"landlord_id,omitempty"`
	LesseeName          string   `json:"lessee_name"`
	LessorName          string   `json:"lessor_name"`
	StoreName           string   `json:"store_name"`
	StoreAddress        string   `json:"store_address"`
	Tags                string   `json:"tags"`
	Currency            string   `json:"currency"`
	AssetType           string   `json:"asset_type"`
	SigningDate         *string  `json:"signing_date"`
	CommencementDate    string   `json:"commencement_date"`
	LeaseStartDate      string   `json:"lease_start_date"`
	LeaseEndDate        string   `json:"lease_end_date"`
	DiscountRateType    *string  `json:"discount_rate_type"`
	DiscountRateVersion *string  `json:"discount_rate_version"`
	DiscountRateValue   *float64 `json:"discount_rate_value"`
	LeaseScope          string   `json:"lease_scope"`
	ExemptionReason     *string  `json:"exemption_reason"`
	ScopeSource         *string  `json:"scope_source"`
	ScopeConfidence     *float64 `json:"scope_confidence"`
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

func normalizeLeaseScope(scope string) string {
	switch scope {
	case "in_scope", "short_term_exempt", "low_value_exempt", "not_a_lease":
		return scope
	default:
		return "in_scope"
	}
}

func normalizeAssetType(assetType string) string {
	switch assetType {
	case "real_estate", "vehicle", "it_equipment", "machinery", "other":
		return assetType
	default:
		return "real_estate"
	}
}

func (h *ContractHandler) resolveContractMasterData(c *gin.Context, contract *repository.Contract, legalEntityHint string) error {
	if contract.LegalEntityID == nil || *contract.LegalEntityID == "" {
		resolved, err := h.contractRepo.ResolveLegalEntityID(c.Request.Context(), strings.TrimSpace(legalEntityHint), contract.Currency)
		if err != nil {
			return err
		}
		contract.LegalEntityID = resolved
	}

	if contract.StoreID == nil || *contract.StoreID == "" {
		resolved, err := h.contractRepo.ResolveOrCreateStoreID(c.Request.Context(), strings.TrimSpace(contract.StoreName), strings.TrimSpace(contract.StoreAddress), contract.LegalEntityID)
		if err != nil {
			return err
		}
		contract.StoreID = resolved
	}

	if contract.LandlordID == nil || *contract.LandlordID == "" {
		resolved, err := h.contractRepo.ResolveOrCreateLandlordID(c.Request.Context(), strings.TrimSpace(contract.LessorName))
		if err != nil {
			return err
		}
		contract.LandlordID = resolved
	}

	return nil
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
	now := time.Now()
	normalizedDiscountRateValue := normalizeDiscountRateValue(req.DiscountRateValue)

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
		AssetType:                    normalizeAssetType(req.AssetType),
		AssetCategory:                req.AssetCategory,
		PropertyCategory:             req.PropertyCategory,
		SigningDate:                  signingDate,
		RenewalOptionDescription:     req.RenewalOptionDescription,
		TerminationOptionDescription: req.TerminationOptionDescription,
		RenewalAssessment:            req.RenewalAssessment,
		TerminationAssessment:        req.TerminationAssessment,
		DiscountRateType:             req.DiscountRateType,
		DiscountRateVersion:          req.DiscountRateVersion,
		DiscountRateValue:            normalizedDiscountRateValue,
		DiscountRateMissing:          normalizedDiscountRateValue == nil,
		LeaseScope:                   normalizeLeaseScope(req.LeaseScope),
		ExemptionReason:              req.ExemptionReason,
		ScopeSource:                  req.ScopeSource,
		ScopeConfidence:              req.ScopeConfidence,
		CreatedBy:                    createdBy,
	}
	if contract.ScopeSource == nil {
		source := "manual"
		contract.ScopeSource = &source
	}
	if createdBy != nil {
		contract.ScopeClassifiedBy = createdBy
		contract.ScopeClassifiedAt = &now
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
	if err := h.resolveContractMasterData(c, contract, req.LesseeName); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to resolve master data: " + err.Error()})
		return
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

// CreateBatch creates multiple contracts from AI parsed draft data
func (h *ContractHandler) CreateBatch(c *gin.Context) {
	type BatchContractRequest struct {
		Contracts []ContractRequest `json:"contracts" binding:"required"`
	}

	var req BatchContractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	var createdBy *string
	if id, ok := userID.(string); ok {
		createdBy = &id
	}
	now := time.Now()

	// Auto-fill legal_entity_id from JWT tenant context
	jwtLegalEntityID := middleware.GetTenantID(c)

	createdContracts := make([]*repository.Contract, 0)
	failedContracts := make([]map[string]interface{}, 0)

	for i, contractReq := range req.Contracts {
		commencementDate, err := time.Parse("2006-01-02", contractReq.CommencementDate)
		if err != nil {
			failedContracts = append(failedContracts, map[string]interface{}{
				"index":  i,
				"number": contractReq.ContractNumber,
				"error":  "invalid commencement_date: " + err.Error(),
			})
			continue
		}
		leaseStartDate, err := time.Parse("2006-01-02", contractReq.LeaseStartDate)
		if err != nil {
			failedContracts = append(failedContracts, map[string]interface{}{
				"index":  i,
				"number": contractReq.ContractNumber,
				"error":  "invalid lease_start_date: " + err.Error(),
			})
			continue
		}
		leaseEndDate, err := time.Parse("2006-01-02", contractReq.LeaseEndDate)
		if err != nil {
			failedContracts = append(failedContracts, map[string]interface{}{
				"index":  i,
				"number": contractReq.ContractNumber,
				"error":  "invalid lease_end_date: " + err.Error(),
			})
			continue
		}

		var signingDate *time.Time
		if contractReq.SigningDate != nil {
			sd, _ := time.Parse("2006-01-02", *contractReq.SigningDate)
			signingDate = &sd
		}
		normalizedDiscountRateValue := normalizeDiscountRateValue(contractReq.DiscountRateValue)

		contract := &repository.Contract{
			ContractNumber:               contractReq.ContractNumber,
			ContractName:                 contractReq.ContractName,
			Currency:                     contractReq.Currency,
			CommencementDate:             commencementDate,
			LeaseStartDate:               leaseStartDate,
			LeaseEndDate:                 leaseEndDate,
			LesseeName:                   contractReq.LesseeName,
			LessorName:                   contractReq.LessorName,
			StoreName:                    contractReq.StoreName,
			StoreAddress:                 contractReq.StoreAddress,
			Tags:                         normalizeTags(contractReq.Tags),
			AssetType:                    normalizeAssetType(contractReq.AssetType),
			AssetCategory:                contractReq.AssetCategory,
			PropertyCategory:             contractReq.PropertyCategory,
			SigningDate:                  signingDate,
			RenewalOptionDescription:     contractReq.RenewalOptionDescription,
			TerminationOptionDescription: contractReq.TerminationOptionDescription,
			RenewalAssessment:            contractReq.RenewalAssessment,
			TerminationAssessment:        contractReq.TerminationAssessment,
			DiscountRateType:             contractReq.DiscountRateType,
			DiscountRateVersion:          contractReq.DiscountRateVersion,
			DiscountRateValue:            normalizedDiscountRateValue,
			DiscountRateMissing:          normalizedDiscountRateValue == nil,
			LeaseScope:                   normalizeLeaseScope(contractReq.LeaseScope),
			ExemptionReason:              contractReq.ExemptionReason,
			ScopeSource:                  contractReq.ScopeSource,
			ScopeConfidence:              contractReq.ScopeConfidence,
			CreatedBy:                    createdBy,
		}
		if contract.ScopeSource == nil {
			source := "manual"
			contract.ScopeSource = &source
		}
		if createdBy != nil {
			contract.ScopeClassifiedBy = createdBy
			contract.ScopeClassifiedAt = &now
		}

		// Auto-fill legal_entity_id from JWT tenant context if not provided
		if contractReq.LegalEntityID != nil && *contractReq.LegalEntityID != "" {
			contract.LegalEntityID = contractReq.LegalEntityID
		} else if jwtLegalEntityID != "" {
			contract.LegalEntityID = &jwtLegalEntityID
		}
		if contractReq.StoreID != nil && *contractReq.StoreID != "" {
			contract.StoreID = contractReq.StoreID
		}
		if contractReq.LandlordID != nil && *contractReq.LandlordID != "" {
			contract.LandlordID = contractReq.LandlordID
		}
		if err := h.resolveContractMasterData(c, contract, contractReq.LesseeName); err != nil {
			failedContracts = append(failedContracts, map[string]interface{}{
				"index":  i,
				"number": contractReq.ContractNumber,
				"error":  "failed to resolve master data: " + err.Error(),
			})
			continue
		}

		created, err := h.contractRepo.Create(c.Request.Context(), contract)
		if err != nil {
			failedContracts = append(failedContracts, map[string]interface{}{
				"index":  i,
				"number": contractReq.ContractNumber,
				"error":  "failed to create: " + err.Error(),
			})
			continue
		}

		createdContracts = append(createdContracts, created)

		// Audit log: contract created
		if h.auditLogger != nil {
			uid, _ := c.Get("user_id")
			uidStr, _ := uid.(string)
			h.auditLogger.Log(c.Request.Context(), "lease_contracts", created.ID, "create", nil, created, uidStr, c)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":           true,
		"created_count":     len(createdContracts),
		"failed_count":      len(failedContracts),
		"created_contracts": createdContracts,
		"failed_contracts":  failedContracts,
	})
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
	now := time.Now()
	normalizedDiscountRateValue := normalizeDiscountRateValue(req.DiscountRateValue)

	// Build update contract
	leaseScope := existing.LeaseScope
	if req.LeaseScope != "" {
		leaseScope = normalizeLeaseScope(req.LeaseScope)
	}
	assetType := existing.AssetType
	if req.AssetType != "" {
		assetType = normalizeAssetType(req.AssetType)
	}
	contract := &repository.Contract{
		ID:                  id,
		ContractNumber:      req.ContractNumber,
		ContractName:        req.ContractName,
		LegalEntityID:       req.LegalEntityID,
		StoreID:             req.StoreID,
		LandlordID:          req.LandlordID,
		LesseeName:          req.LesseeName,
		LessorName:          req.LessorName,
		StoreName:           req.StoreName,
		StoreAddress:        req.StoreAddress,
		Tags:                normalizeTags(req.Tags),
		Currency:            req.Currency,
		AssetType:           assetType,
		SigningDate:         signingDate,
		CommencementDate:    commencementDate,
		LeaseStartDate:      leaseStartDate,
		LeaseEndDate:        leaseEndDate,
		DiscountRateType:    req.DiscountRateType,
		DiscountRateVersion: req.DiscountRateVersion,
		DiscountRateValue:   normalizedDiscountRateValue,
		DiscountRateMissing: normalizedDiscountRateValue == nil,
		LeaseScope:          leaseScope,
		ExemptionReason:     req.ExemptionReason,
		ScopeSource:         req.ScopeSource,
		ScopeConfidence:     req.ScopeConfidence,
		Status:              existing.Status,
	}
	if contract.ScopeSource == nil {
		source := "manual"
		contract.ScopeSource = &source
	}
	if updatedBy != "" {
		contract.ScopeClassifiedBy = &updatedBy
		contract.ScopeClassifiedAt = &now
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
