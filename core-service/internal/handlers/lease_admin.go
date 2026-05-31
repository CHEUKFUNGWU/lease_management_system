package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
)

type LeaseAdminHandler struct {
	repo         *repository.LeaseAdminRepository
	contractRepo *repository.ContractRepository
	auditLogger  *audit.Logger
}

func NewLeaseAdminHandler(repo *repository.LeaseAdminRepository, contractRepo *repository.ContractRepository, auditLogger *audit.Logger) *LeaseAdminHandler {
	return &LeaseAdminHandler{repo: repo, contractRepo: contractRepo, auditLogger: auditLogger}
}

type CreateCriticalDateRequest struct {
	DateType          string  `json:"date_type" binding:"required"`
	TargetDate        string  `json:"target_date" binding:"required"`
	ReminderDays      int     `json:"reminder_days"`
	ResponsibleUserID *string `json:"responsible_user_id"`
	Title             string  `json:"title" binding:"required"`
	Description       *string `json:"description"`
	Source            string  `json:"source"`
}

func (h *LeaseAdminHandler) CreateCriticalDate(c *gin.Context) {
	contractID := c.Param("id")
	if !h.verifyContract(c, contractID) {
		return
	}

	var req CreateCriticalDateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	targetDate, err := time.Parse("2006-01-02", req.TargetDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid target_date, expected YYYY-MM-DD"})
		return
	}
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	item := &repository.CriticalDate{
		ContractID:        contractID,
		DateType:          req.DateType,
		TargetDate:        targetDate,
		ReminderDays:      req.ReminderDays,
		ResponsibleUserID: req.ResponsibleUserID,
		Title:             req.Title,
		Description:       req.Description,
		Source:            req.Source,
		CreatedBy:         &userIDStr,
	}
	created, err := h.repo.CreateCriticalDate(c.Request.Context(), item)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), "critical_dates", created.ID, "create", nil, created, userIDStr, c)
	}
	c.JSON(http.StatusCreated, created)
}

func (h *LeaseAdminHandler) ListCriticalDates(c *gin.Context) {
	contractID := c.Param("id")
	if !h.verifyContract(c, contractID) {
		return
	}
	items, err := h.repo.ListCriticalDates(c.Request.Context(), contractID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": len(items)})
}

func (h *LeaseAdminHandler) ListUpcomingCriticalDates(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "90"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	legalEntityID := middleware.GetTenantID(c)
	items, err := h.repo.ListUpcomingCriticalDates(c.Request.Context(), legalEntityID, days, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": len(items), "days": days})
}

func (h *LeaseAdminHandler) UpdateCriticalDateStatus(c *gin.Context) {
	contractID := c.Param("id")
	if !h.verifyContract(c, contractID) {
		return
	}
	var req struct {
		Status string `json:"status" binding:"required,oneof=open snoozed completed cancelled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	if err := h.repo.UpdateCriticalDateStatus(c.Request.Context(), c.Param("dateId"), req.Status, userIDStr); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": req.Status})
}

type CreateDocumentRequest struct {
	DocumentType    string  `json:"document_type"`
	FileName        string  `json:"file_name" binding:"required"`
	FileType        *string `json:"file_type"`
	FileSize        *int64  `json:"file_size"`
	MinIOBucket     *string `json:"minio_bucket"`
	MinIOObjectKey  *string `json:"minio_object_key"`
	DocumentVersion *string `json:"document_version"`
	FileHash        *string `json:"file_hash"`
	SourcePage      *int    `json:"source_page"`
	Notes           *string `json:"notes"`
}

type CreateObligationRequest struct {
	ObligationType   string                 `json:"obligation_type" binding:"required,oneof=maintenance cam insurance index_adjustment restoration security_deposit notice other"`
	ResponsibleParty string                 `json:"responsible_party" binding:"required,oneof=lessee lessor shared third_party"`
	Title            string                 `json:"title" binding:"required"`
	Description      *string                `json:"description"`
	StructuredValue  map[string]interface{} `json:"structured_value"`
	SourceClause     *string                `json:"source_clause"`
	SourcePage       *int                   `json:"source_page"`
}

func (h *LeaseAdminHandler) CreateDocument(c *gin.Context) {
	contractID := c.Param("id")
	if !h.verifyContract(c, contractID) {
		return
	}
	var req CreateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	doc := &repository.LeaseDocument{
		ContractID:      contractID,
		DocumentType:    req.DocumentType,
		FileName:        req.FileName,
		FileType:        req.FileType,
		FileSize:        req.FileSize,
		MinIOBucket:     req.MinIOBucket,
		MinIOObjectKey:  req.MinIOObjectKey,
		DocumentVersion: req.DocumentVersion,
		FileHash:        req.FileHash,
		SourcePage:      req.SourcePage,
		Notes:           req.Notes,
		UploadedBy:      &userIDStr,
	}
	created, err := h.repo.CreateDocument(c.Request.Context(), doc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), "lease_documents", created.ID, "create", nil, created, userIDStr, c)
	}
	c.JSON(http.StatusCreated, created)
}

func (h *LeaseAdminHandler) ListDocuments(c *gin.Context) {
	contractID := c.Param("id")
	if !h.verifyContract(c, contractID) {
		return
	}
	docs, err := h.repo.ListDocuments(c.Request.Context(), contractID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": docs, "total": len(docs)})
}

func (h *LeaseAdminHandler) CreateObligation(c *gin.Context) {
	contractID := c.Param("id")
	if !h.verifyContract(c, contractID) {
		return
	}
	var req CreateObligationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	item := &repository.LeaseObligation{
		ContractID:       contractID,
		ObligationType:   req.ObligationType,
		ResponsibleParty: req.ResponsibleParty,
		Title:            req.Title,
		Description:      req.Description,
		StructuredValue:  req.StructuredValue,
		SourceClause:     req.SourceClause,
		SourcePage:       req.SourcePage,
		CreatedBy:        &userIDStr,
	}
	created, err := h.repo.CreateObligation(c.Request.Context(), item)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), "lease_obligations", created.ID, "create", nil, created, userIDStr, c)
	}
	c.JSON(http.StatusCreated, created)
}

func (h *LeaseAdminHandler) ListObligations(c *gin.Context) {
	contractID := c.Param("id")
	if !h.verifyContract(c, contractID) {
		return
	}
	items, err := h.repo.ListObligations(c.Request.Context(), contractID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": items, "total": len(items)})
}

func (h *LeaseAdminHandler) UpdateObligationStatus(c *gin.Context) {
	contractID := c.Param("id")
	if !h.verifyContract(c, contractID) {
		return
	}
	var req struct {
		Status string `json:"status" binding:"required,oneof=active completed waived cancelled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.repo.UpdateObligationStatus(c.Request.Context(), c.Param("obligationId"), req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), "lease_obligations", c.Param("obligationId"), "status_update", nil, gin.H{"status": req.Status}, userIDStr, c)
	}
	c.JSON(http.StatusOK, gin.H{"status": req.Status})
}

func (h *LeaseAdminHandler) verifyContract(c *gin.Context, contractID string) bool {
	legalEntityID := middleware.GetTenantID(c)
	contract, err := h.contractRepo.GetByID(c.Request.Context(), contractID, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify contract: " + err.Error()})
		return false
	}
	if contract == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
		return false
	}
	return true
}
