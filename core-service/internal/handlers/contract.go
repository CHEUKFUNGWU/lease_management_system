package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
	contractsvc "github.com/lease-management-system/core-service/internal/services/contracts"
)

type ContractHandler struct {
	contractRepo *repository.ContractRepository
	auditLogger  *audit.Logger
}

func NewContractHandler(contractRepo *repository.ContractRepository, auditLogger *audit.Logger) *ContractHandler {
	return &ContractHandler{contractRepo: contractRepo, auditLogger: auditLogger}
}

type ContractRequest = contractsvc.CreateInput

// UpdateContractRequest represents the editable fields for updating a contract.
type UpdateContractRequest = contractsvc.UpdateInput

func (h *ContractHandler) Create(c *gin.Context) {
	var req ContractRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	var createdBy *string
	if id, ok := userID.(string); ok {
		createdBy = &id
	}
	contract, err := contractsvc.BuildForCreate(req, middleware.GetTenantID(c), createdBy, time.Now())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := contractsvc.ResolveMasterData(c.Request.Context(), h.contractRepo, contract, req.LesseeName); err != nil {
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

	createdContracts := make([]*repository.Contract, 0)
	failedContracts := make([]map[string]interface{}, 0)

	for i, contractReq := range req.Contracts {
		contract, err := contractsvc.BuildForCreate(contractReq, middleware.GetTenantID(c), createdBy, time.Now())
		if err != nil {
			failedContracts = append(failedContracts, map[string]interface{}{
				"index":  i,
				"number": contractReq.ContractNumber,
				"error":  err.Error(),
			})
			continue
		}
		if err := contractsvc.ResolveMasterData(c.Request.Context(), h.contractRepo, contract, contractReq.LesseeName); err != nil {
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

	// Get user_id from context
	userID, _ := c.Get("user_id")
	var updatedBy *string
	if uid, ok := userID.(string); ok {
		updatedBy = &uid
	}
	contract, err := contractsvc.BuildForUpdate(id, existing, req, updatedBy, time.Now())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updateActor := ""
	if updatedBy != nil {
		updateActor = *updatedBy
	}
	if err := h.contractRepo.Update(c.Request.Context(), contract, legalEntityID, updateActor); err != nil {
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
