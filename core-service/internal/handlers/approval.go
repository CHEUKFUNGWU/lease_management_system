package handlers

import (
	"net/http"
	
	"github.com/gin-gonic/gin"
	"github.com/ifrs16/core-service/internal/middleware"
	"github.com/ifrs16/core-service/internal/repository"
)

type ApprovalHandler struct {
	approvalRepo *repository.ApprovalRepository
	contractRepo *repository.ContractRepository
}

func NewApprovalHandler(approvalRepo *repository.ApprovalRepository, contractRepo *repository.ContractRepository) *ApprovalHandler {
	return &ApprovalHandler{approvalRepo: approvalRepo, contractRepo: contractRepo}
}

type SubmitRequest struct {
	ContractID string `json:"contract_id" binding:"required,uuid"`
}

type ReviewRequest struct {
	ContractID string `json:"contract_id" binding:"required,uuid"`
	Approved   bool   `json:"approved"`
	Reason     string `json:"reason"`
}

type ApproveRequest struct {
	ContractID string `json:"contract_id" binding:"required,uuid"`
}

type ApprovalResponse struct {
	ContractID        string `json:"contract_id"`
	ApprovalStatus    string `json:"approval_status"`
	IsOfficialVersion bool   `json:"is_official_version"`
}

func (h *ApprovalHandler) SubmitForReview(c *gin.Context) {
	var req SubmitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	ctx := c.Request.Context()
	legalEntityID := middleware.GetTenantID(c)
	
	// Verify contract belongs to tenant
	contract, err := h.contractRepo.GetByID(ctx, req.ContractID, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify contract: " + err.Error()})
		return
	}
	if contract == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
		return
	}
	
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	
	if err := h.approvalRepo.SubmitForReview(ctx, req.ContractID, userIDStr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to submit: " + err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"contract_id": req.ContractID,
		"status": "submitted",
		"message": "合同已提交复核",
	})
}

func (h *ApprovalHandler) Review(c *gin.Context) {
	var req ReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	ctx := c.Request.Context()
	legalEntityID := middleware.GetTenantID(c)
	
	// Verify contract belongs to tenant
	contract, err := h.contractRepo.GetByID(ctx, req.ContractID, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify contract: " + err.Error()})
		return
	}
	if contract == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
		return
	}
	
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	
	if err := h.approvalRepo.Review(ctx, req.ContractID, userIDStr, req.Approved, req.Reason); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to review: " + err.Error()})
		return
	}
	
	status := "reviewed"
	message := "复核通过，已送审"
	if !req.Approved {
		status = "returned_to_editor"
		message = "已退回编辑"
	}
	
	c.JSON(http.StatusOK, gin.H{
		"contract_id": req.ContractID,
		"status": status,
		"message": message,
	})
}

func (h *ApprovalHandler) Approve(c *gin.Context) {
	var req ApproveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	ctx := c.Request.Context()
	legalEntityID := middleware.GetTenantID(c)
	
	// Verify contract belongs to tenant
	contract, err := h.contractRepo.GetByID(ctx, req.ContractID, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify contract: " + err.Error()})
		return
	}
	if contract == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
		return
	}
	
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	
	if err := h.approvalRepo.Approve(ctx, req.ContractID, userIDStr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to approve: " + err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"contract_id": req.ContractID,
		"status": "approved",
		"message": "合同已审批通过",
	})
}

func (h *ApprovalHandler) Reject(c *gin.Context) {
	var req ReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	ctx := c.Request.Context()
	legalEntityID := middleware.GetTenantID(c)
	
	// Verify contract belongs to tenant
	contract, err := h.contractRepo.GetByID(ctx, req.ContractID, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify contract: " + err.Error()})
		return
	}
	if contract == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
		return
	}
	
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	
	if err := h.approvalRepo.Reject(ctx, req.ContractID, userIDStr, req.Reason); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reject: " + err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"contract_id": req.ContractID,
		"status": "rejected",
		"message": "合同已驳回",
	})
}

func (h *ApprovalHandler) GetStatus(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()
	legalEntityID := middleware.GetTenantID(c)
	
	// Verify contract belongs to tenant
	contract, err := h.contractRepo.GetByID(ctx, id, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify contract: " + err.Error()})
		return
	}
	if contract == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
		return
	}
	
	status, err := h.approvalRepo.GetApprovalStatus(ctx, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get status: " + err.Error()})
		return
	}
	if status == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
		return
	}
	
	c.JSON(http.StatusOK, status)
}

func (h *ApprovalHandler) ListByStatus(c *gin.Context) {
	status := c.Query("status")
	officialOnly := c.Query("official_only") == "true"
	legalEntityID := middleware.GetTenantID(c)
	
	contracts, err := h.approvalRepo.ListByStatus(c.Request.Context(), status, officialOnly)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list: " + err.Error()})
		return
	}
	
	// Filter by tenant if legalEntityID is set (admin users have empty string = cross-tenant)
	if legalEntityID != "" {
		var filtered []*repository.Contract
		for _, c := range contracts {
			if c.LegalEntityID == legalEntityID {
				filtered = append(filtered, c)
			}
		}
		contracts = filtered
	}
	
	c.JSON(http.StatusOK, gin.H{"data": contracts, "total": len(contracts)})
}
