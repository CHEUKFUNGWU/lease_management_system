package handlers

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
	"github.com/lease-management-system/core-service/internal/services/closereadiness"
	"github.com/lease-management-system/core-service/internal/services/monthend"
)

type MonthlyClosingHandler struct {
	mcRepo           *repository.MonthlyClosingRepository
	contractRepo     *repository.ContractRepository
	closeService     *monthend.Service
	readinessService *closereadiness.Service
	auditLogger      *audit.Logger
}

func NewMonthlyClosingHandler(mcRepo *repository.MonthlyClosingRepository, contractRepo *repository.ContractRepository, closeService *monthend.Service, readinessService *closereadiness.Service, auditLogger *audit.Logger) *MonthlyClosingHandler {
	return &MonthlyClosingHandler{mcRepo: mcRepo, contractRepo: contractRepo, closeService: closeService, readinessService: readinessService, auditLogger: auditLogger}
}

type GenerateMonthlyClosingRequest struct {
	AccountingPeriod string  `json:"accounting_period" binding:"required"`
	ContractID       string  `json:"contract_id" binding:"omitempty,uuid"`
	LegalEntityID    string  `json:"legal_entity_id" binding:"omitempty,uuid"`
	DiscountRate     float64 `json:"discount_rate" binding:"omitempty,gt=0,lte=1"`
}

type ERPWritebackItem struct {
	EntryID       string `json:"entry_id" binding:"required,uuid"`
	ERPReference  string `json:"erp_reference"`
	VoucherNumber string `json:"voucher_number"`
}

type ERPWritebackRequest struct {
	Items []ERPWritebackItem `json:"items" binding:"required"`
}

// Generate runs the month-end close for a period and returns the batch result.
// All close behavior — lock validation, eligibility, measurement, entry
// production, idempotency, transactionality, and the audit result — lives behind
// the close service; the handler only parses the request and shapes the response.
func (h *MonthlyClosingHandler) Generate(c *gin.Context) {
	var req GenerateMonthlyClosingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid, _ := c.Get("user_id")
	uidStr, _ := uid.(string)

	result, err := h.closeService.Close(c.Request.Context(), monthend.Command{
		AccountingPeriod:     req.AccountingPeriod,
		ContractID:           req.ContractID,
		LegalEntityID:        middleware.GetTenantID(c),
		DiscountRateOverride: req.DiscountRate,
		Actor:                audit.MetadataFromGin(uidStr, c),
	})
	if err != nil {
		if errors.Is(err, monthend.ErrPeriodLocked) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "期间已锁账，无法重新生成分录"})
			return
		}
		if errors.Is(err, monthend.ErrCloseAlreadyFinalized) {
			c.JSON(http.StatusConflict, gin.H{"error": "月结分录已进入审批或过账流程，请先执行冲销后再重新生成"})
			return
		}
		if errors.Is(err, monthend.ErrContractNotApproved) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "仅已审批合同可生成月结分录"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// Readiness returns a deterministic, read-only preflight for the selected
// period. It does not authorize close, posting or period lock; those decisions
// remain behind the existing month-end workflow.
func (h *MonthlyClosingHandler) Readiness(c *gin.Context) {
	period := strings.TrimSpace(c.Query("period"))
	if _, err := time.Parse("2006-01", period); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "会计期间格式应为 YYYY-MM"})
		return
	}
	if h.readinessService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "月结准备度服务不可用"})
		return
	}

	scope, scopeAvailable := middleware.GetAccessScope(c)
	scopeComplete := scopeAvailable && (scope.Global ||
		(scope.LegalEntityID != "" && len(scope.StoreIDs) == 0 && len(scope.Regions) == 0 && len(scope.Brands) == 0))

	result, err := h.readinessService.Evaluate(c.Request.Context(), closereadiness.Command{
		AccountingPeriod: period,
		LegalEntityID:    middleware.GetTenantID(c),
		ScopeComplete:    scopeComplete,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

type RejectEntryRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// RejectEntry returns an approved journal entry to draft.
func (h *MonthlyClosingHandler) RejectEntry(c *gin.Context) {
	var req RejectEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写驳回理由"})
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写驳回理由"})
		return
	}

	entryID := c.Param("id")
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	if err := h.mcRepo.RejectJournalEntry(c.Request.Context(), entryID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "仅已审批且未过账的分录可驳回"})
		return
	}
	// The reason only survives in the audit trail, so it is recorded with the
	// status change rather than alongside it.
	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), "journal_entries", entryID, "reject",
			map[string]interface{}{"posting_status": "approved"},
			approvalAuditValues(c, map[string]interface{}{
				"posting_status": "draft",
				"reason":         reason,
			}), userIDStr, c)
	}
	c.JSON(http.StatusOK, gin.H{"message": "分录已驳回，状态回到草稿", "entry_id": entryID})
}

type ReverseEntryRequest struct {
	Reason string `json:"reason" binding:"required"`
	// AccountingPeriod places the reversing entry in an open period. It is only
	// needed when the original entry's own period has since been locked.
	AccountingPeriod string `json:"accounting_period"`
}

// ReverseEntry cancels a posted journal entry with an opposite entry.
func (h *MonthlyClosingHandler) ReverseEntry(c *gin.Context) {
	var req ReverseEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写红冲原因"})
		return
	}

	uid, _ := c.Get("user_id")
	uidStr, _ := uid.(string)

	result, err := h.closeService.Reverse(c.Request.Context(), monthend.ReversalCommand{
		EntryID:          c.Param("id"),
		AccountingPeriod: req.AccountingPeriod,
		Reason:           req.Reason,
		LegalEntityID:    middleware.GetTenantID(c),
		Actor:            audit.MetadataFromGin(uidStr, c),
	})
	if err != nil {
		switch {
		case errors.Is(err, monthend.ErrEntryNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "分录不存在"})
		case errors.Is(err, monthend.ErrEntryAlreadyReversed):
			c.JSON(http.StatusConflict, gin.H{"error": "该分录已被红冲，不能重复红冲"})
		case errors.Is(err, monthend.ErrEntryNotPosted):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "仅已过账分录可红冲；草稿或待审批分录请驳回或重新生成"})
		case errors.Is(err, monthend.ErrReversalPeriodLocked):
			c.JSON(http.StatusBadRequest, gin.H{"error": "目标期间已锁账，请指定一个未锁定的期间入账红冲分录"})
		case errors.Is(err, monthend.ErrReversalReasonRequired):
			c.JSON(http.StatusBadRequest, gin.H{"error": "请填写红冲原因"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "红冲成功", "result": result})
}

// ApproveEntry approves a single journal entry
func (h *MonthlyClosingHandler) ApproveEntry(c *gin.Context) {
	entryID := c.Param("id")
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	if err := h.mcRepo.ApproveJournalEntry(c.Request.Context(), entryID, userIDStr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Audit log
	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), "journal_entries", entryID, "approve", nil, approvalAuditValues(c, nil), userIDStr, c)
	}
	c.JSON(http.StatusOK, gin.H{"message": "分录审批成功", "entry_id": entryID})
}

// PostEntry posts a single journal entry
func (h *MonthlyClosingHandler) PostEntry(c *gin.Context) {
	entryID := c.Param("id")
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	var req struct {
		ERPReference string `json:"erp_reference"`
	}
	_ = c.ShouldBindJSON(&req)

	if err := h.mcRepo.PostJournalEntry(c.Request.Context(), entryID, userIDStr, req.ERPReference); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Audit log
	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), "journal_entries", entryID, "post", nil, nil, userIDStr, c)
	}
	c.JSON(http.StatusOK, gin.H{"message": "分录过账成功", "entry_id": entryID})
}

// ApproveBatch approves all draft entries in a batch
func (h *MonthlyClosingHandler) ApproveBatch(c *gin.Context) {
	batchID := c.Param("id")
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	count, err := h.mcRepo.ApproveBatchEntries(c.Request.Context(), batchID, userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), "monthly_closing_batches", batchID, "approve", nil, approvalAuditValues(c, map[string]interface{}{"approved_count": count}), userIDStr, c)
	}
	c.JSON(http.StatusOK, gin.H{"message": "批次审批成功", "approved_count": count})
}

// PostBatch posts all approved entries in a batch
func (h *MonthlyClosingHandler) PostBatch(c *gin.Context) {
	batchID := c.Param("id")
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	count, err := h.mcRepo.PostBatchEntries(c.Request.Context(), batchID, userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "批次过账成功", "posted_count": count})
}

func (h *MonthlyClosingHandler) ExportJournalEntries(c *gin.Context) {
	period := c.Query("period")
	status := c.DefaultQuery("status", "approved")
	template := strings.TrimSpace(c.Query("template"))
	if template == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 ERP 导出模板标识"})
		return
	}
	if strings.ContainsAny(template, `/\\:\"'`) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ERP 导出模板标识包含非法字符"})
		return
	}
	legalEntityID := middleware.GetTenantID(c)

	entries, err := h.mcRepo.GetJournalEntriesForExport(c.Request.Context(), legalEntityID, period, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	filename := fmt.Sprintf("Lease_GL_%s_%s.csv", period, template)
	if period == "" {
		filename = fmt.Sprintf("Lease_GL_%s.csv", template)
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	writer := csv.NewWriter(c.Writer)
	_ = writer.Write([]string{
		"entry_id", "template", "period", "entry_date", "entry_type",
		"debit_account", "credit_account", "amount", "currency", "description",
		"contract_id", "posting_status",
	})
	for _, entry := range entries {
		desc := ""
		if entry.Description != nil {
			desc = *entry.Description
		}
		_ = writer.Write([]string{
			entry.ID,
			template,
			entry.AccountingPeriod,
			entry.EntryDate.Format("2006-01-02"),
			entry.EntryType,
			entry.DebitAccount,
			entry.CreditAccount,
			strconv.FormatFloat(entry.Amount, 'f', 2, 64),
			entry.Currency,
			desc,
			entry.ContractID,
			entry.PostingStatus,
		})
	}
	writer.Flush()
}

func (h *MonthlyClosingHandler) ApplyERPWriteback(c *gin.Context) {
	var req ERPWritebackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	applied := 0
	failures := make([]gin.H, 0)
	for _, item := range req.Items {
		if err := h.mcRepo.ApplyERPWriteback(c.Request.Context(), item.EntryID, userIDStr, item.ERPReference, item.VoucherNumber); err != nil {
			failures = append(failures, gin.H{"entry_id": item.EntryID, "error": err.Error()})
			continue
		}
		applied++
		if h.auditLogger != nil {
			h.auditLogger.Log(c.Request.Context(), "journal_entries", item.EntryID, "erp_writeback", nil, item, userIDStr, c)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"applied_count": applied,
		"failed_count":  len(failures),
		"failures":      failures,
	})
}

// LockPeriod locks an accounting period
func (h *MonthlyClosingHandler) LockPeriod(c *gin.Context) {
	period := c.Param("period")
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	legalEntityID := middleware.GetTenantID(c)

	if err := h.mcRepo.LockPeriod(c.Request.Context(), period, legalEntityID, userIDStr); err != nil {
		if errors.Is(err, repository.ErrUnresolvedBlockingExceptions) {
			c.JSON(http.StatusConflict, gin.H{"error": "本期间仍有未解决的阻塞性异常，完成解决或受控豁免后才能锁账", "blocking_exceptions": true})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Audit log
	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), "period_locks", period, "lock", nil, nil, userIDStr, c)
	}
	c.JSON(http.StatusOK, gin.H{"message": "期间锁账成功", "period": period})
}

// UnlockPeriod unlocks an accounting period
func (h *MonthlyClosingHandler) UnlockPeriod(c *gin.Context) {
	period := c.Param("period")
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	legalEntityID := middleware.GetTenantID(c)

	if err := h.mcRepo.UnlockPeriod(c.Request.Context(), period, legalEntityID, userIDStr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Audit log
	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), "period_locks", period, "unlock", nil, nil, userIDStr, c)
	}
	c.JSON(http.StatusOK, gin.H{"message": "期间解锁成功", "period": period})
}

// GetPeriodLockStatus returns the lock status for a period
func (h *MonthlyClosingHandler) GetPeriodLockStatus(c *gin.Context) {
	period := c.Param("period")
	legalEntityID := middleware.GetTenantID(c)

	isLocked, err := h.mcRepo.IsPeriodLocked(c.Request.Context(), period, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"period": period, "is_locked": isLocked})
}

// ListBatches returns all monthly closing batches
func (h *MonthlyClosingHandler) ListBatches(c *gin.Context) {
	period := c.Query("period")
	legalEntityID := middleware.GetTenantID(c)

	batches, err := h.mcRepo.GetBatches(c.Request.Context(), period, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  batches,
		"total": len(batches),
	})
}

// GetJournalEntries queries the ledger by period, independently of any close.
// A period can therefore be reviewed, reconciled or exported long after it was
// closed, without regenerating anything.
func (h *MonthlyClosingHandler) GetJournalEntries(c *gin.Context) {
	contractID := c.Query("contract_id")
	ctx := c.Request.Context()
	legalEntityID := middleware.GetTenantID(c)

	status, valid := repository.NormalizeEntryStatus(c.Query("status"))
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "分录状态无效，可选：draft、approved、posted、reversed"})
		return
	}

	// If filtering by contract, verify contract belongs to tenant
	if contractID != "" {
		contract, err := h.contractRepo.GetByID(ctx, contractID, legalEntityID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify contract: " + err.Error()})
			return
		}
		if contract == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
			return
		}
	}

	requestedPage, _ := strconv.Atoi(c.Query("page"))
	requestedSize, _ := strconv.Atoi(c.Query("page_size"))
	page, pageSize := repository.NormalizeEntryPaging(requestedPage, requestedSize)

	entries, summary, err := h.mcRepo.ListJournalEntries(ctx, repository.JournalEntryQuery{
		LegalEntityID: legalEntityID,
		Period:        c.Query("period"),
		ContractID:    contractID,
		Status:        status,
		EntryType:     c.Query("entry_type"),
		Page:          page,
		PageSize:      pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": entries,
		// total counts the whole filtered period, not this page, so the caller
		// can paginate without a second request to learn how far it goes.
		"total":     summary.Total,
		"page":      page,
		"page_size": pageSize,
		"summary":   summary,
	})
}

// ListEntryPeriods returns the periods the ledger actually holds entries for,
// so a period can be chosen from what exists rather than typed from memory.
func (h *MonthlyClosingHandler) ListEntryPeriods(c *gin.Context) {
	limit, _ := strconv.Atoi(c.Query("limit"))

	periods, err := h.mcRepo.ListEntryPeriods(c.Request.Context(), middleware.GetTenantID(c), limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": periods, "total": len(periods)})
}

// GetMeasurementResults returns measurement results
func (h *MonthlyClosingHandler) GetMeasurementResults(c *gin.Context) {
	contractID := c.Param("id")
	period := c.Query("period")
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

	results, err := h.mcRepo.GetMeasurementResults(ctx, contractID, period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  results,
		"total": len(results),
	})
}
