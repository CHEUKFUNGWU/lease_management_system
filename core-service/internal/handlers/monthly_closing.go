package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
	ifrs16svc "github.com/lease-management-system/core-service/internal/services/ifrs16"
)

type MonthlyClosingHandler struct {
	mcRepo            *repository.MonthlyClosingRepository
	contractRepo      *repository.ContractRepository
	psRepo            *repository.PaymentScheduleRepository
	systemSettingRepo *repository.SystemSettingRepository
	auditLogger       *audit.Logger
}

func NewMonthlyClosingHandler(mcRepo *repository.MonthlyClosingRepository, contractRepo *repository.ContractRepository, psRepo *repository.PaymentScheduleRepository, systemSettingRepo *repository.SystemSettingRepository, auditLogger *audit.Logger) *MonthlyClosingHandler {
	return &MonthlyClosingHandler{mcRepo: mcRepo, contractRepo: contractRepo, psRepo: psRepo, systemSettingRepo: systemSettingRepo, auditLogger: auditLogger}
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

// Generate creates measurement results and journal entries for a given period
func (h *MonthlyClosingHandler) Generate(c *gin.Context) {
	var req GenerateMonthlyClosingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	legalEntityID := middleware.GetTenantID(c)

	// Check if period is locked
	isLocked, err := h.mcRepo.IsPeriodLocked(ctx, req.AccountingPeriod, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check period lock: " + err.Error()})
		return
	}
	if isLocked {
		c.JSON(http.StatusBadRequest, gin.H{"error": "期间已锁账，无法重新生成分录"})
		return
	}

	// Create batch record
	batch, err := h.mcRepo.CreateBatch(ctx, &repository.MonthlyClosingBatch{
		BatchNumber:      fmt.Sprintf("BATCH-%s-%s", req.AccountingPeriod, time.Now().Format("20060102-150405")),
		AccountingPeriod: req.AccountingPeriod,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create batch: " + err.Error()})
		return
	}

	// Get contracts to process
	var contracts []*repository.Contract
	if req.ContractID != "" {
		contract, err := h.contractRepo.GetByID(ctx, req.ContractID, legalEntityID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if contract != nil {
			contracts = append(contracts, contract)
		}
	} else {
		contracts, err = h.contractRepo.GetByStatuses(ctx, []string{"approved"}, legalEntityID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	processed := 0
	failed := 0
	totalEntries := 0
	batchID := batch.ID

	// Parse period to get start/end dates
	periodStart, _ := time.Parse("2006-01", req.AccountingPeriod)
	periodEnd := periodStart.AddDate(0, 1, -1)

	for _, contract := range contracts {
		// Skip if contract hasn't started yet or already ended before period
		if contract.LeaseEndDate.Before(periodStart) || contract.CommencementDate.After(periodEnd) {
			continue
		}

		// Get payment schedules
		schedules, err := h.psRepo.GetByContractID(ctx, contract.ID)
		if err != nil {
			failed++
			continue
		}

		var payments []ifrs16svc.LeasePayment
		if len(schedules) == 0 {
			// Fallback mock payments
			payments = generateMockPayments(contract)
		} else {
			payments = repository.ToIFRS16Payments(schedules)
		}

		// Determine discount rate priority:
		// 1) request override
		// 2) global system setting
		// 3) contract value
		// 4) fallback 0.05
		discountRate := req.DiscountRate
		if discountRate <= 0 {
			discountRate = resolveGlobalDiscountRate(ctx, h.systemSettingRepo)
		}
		if discountRate <= 0 && contract.DiscountRateValue != nil && *contract.DiscountRateValue > 0 {
			discountRate = *contract.DiscountRateValue
		}
		if discountRate <= 0 {
			discountRate = 0.05
		}

		// Run IFRS 16 calculation
		calculation := ifrs16svc.LeaseCalculation{
			CommencementDate: contract.CommencementDate,
			LeaseEndDate:     contract.LeaseEndDate,
			LeaseScope:       contract.LeaseScope,
			DiscountRate:     discountRate,
			Payments:         payments,
			PrepaidRent: ifrs16svc.CalculatePrepaidRent(ifrs16svc.LeaseCalculation{
				CommencementDate: contract.CommencementDate,
				Payments:         payments,
			}),
		}

		result, err := ifrs16svc.Calculate(calculation)
		if err != nil {
			failed++
			continue
		}
		if result.MeasurementBasis == "skipped" {
			processed++
			continue
		}

		// Find the monthly entry for this period
		var monthlyEntry *ifrs16svc.MonthlyEntry
		for _, m := range result.MonthlySummary {
			key := fmt.Sprintf("%04d-%02d", m.Year, m.Month)
			if key == req.AccountingPeriod {
				monthlyEntry = &m
				break
			}
		}

		if monthlyEntry == nil {
			// Period not in contract term - skip
			continue
		}

		now := time.Now()

		// Save measurement result
		mr := &repository.MeasurementResult{
			ContractID:          contract.ID,
			AccountingPeriod:    req.AccountingPeriod,
			PeriodStartDate:     periodStart,
			PeriodEndDate:       periodEnd,
			OpeningLiability:    monthlyEntry.OpeningLiability,
			InterestExpense:     monthlyEntry.InterestExpense,
			TotalPayment:        monthlyEntry.TotalPayments,
			ClosingLiability:    monthlyEntry.ClosingLiability,
			OpeningROUAsset:     monthlyEntry.OpeningROUAsset,
			Depreciation:        monthlyEntry.Depreciation,
			ClosingROUAsset:     monthlyEntry.ClosingROUAsset,
			VariableRentExpense: monthlyEntry.VariableRentExpense,
			NonLeaseExpense:     monthlyEntry.NonLeaseExpense,
			DiscountRate:        discountRate,
			IsCalculated:        true,
			CalculationBatchID:  &batchID,
			CalculatedAt:        &now,
		}

		if err := h.mcRepo.SaveMeasurementResult(ctx, mr); err != nil {
			failed++
			continue
		}

		// Generate journal entries for this period
		entries := generateJournalEntries(contract.ID, req.AccountingPeriod, periodEnd, monthlyEntry, batchID, result.MeasurementBasis)
		for _, entry := range entries {
			if err := h.mcRepo.CreateJournalEntry(ctx, entry); err == nil {
				totalEntries++
			}
		}

		processed++
	}

	// Link existing event adjustment journal entries to this batch
	for _, contract := range contracts {
		entries, err := h.mcRepo.GetJournalEntries(ctx, contract.ID, req.AccountingPeriod, "draft")
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.BatchID == nil && (entry.EntryType == "modification" || entry.EntryType == "reassessment" || entry.EntryType == "impairment") {
				if err := h.mcRepo.CreateJournalEntry(ctx, &repository.JournalEntry{
					ContractID:       entry.ContractID,
					AccountingPeriod: entry.AccountingPeriod,
					EntryDate:        entry.EntryDate,
					EntryType:        entry.EntryType,
					DebitAccount:     entry.DebitAccount,
					CreditAccount:    entry.CreditAccount,
					Amount:           entry.Amount,
					Currency:         entry.Currency,
					Description:      entry.Description,
					PostingStatus:    "draft",
					BatchID:          &batchID,
				}); err == nil {
					totalEntries++
				}
			}
		}
	}

	// Update batch status
	status := "completed"
	if failed > 0 && processed == 0 {
		status = "failed"
	} else if failed > 0 {
		status = "completed_with_errors"
	}

	h.mcRepo.UpdateBatchStatus(ctx, batch.ID, status, processed, failed, totalEntries, 0)

	c.JSON(http.StatusOK, gin.H{
		"batch_id":            batch.ID,
		"batch_number":        batch.BatchNumber,
		"accounting_period":   req.AccountingPeriod,
		"status":              status,
		"total_contracts":     len(contracts),
		"processed_contracts": processed,
		"failed_contracts":    failed,
		"total_entries":       totalEntries,
	})

	// Audit log: monthly closing generated
	if h.auditLogger != nil {
		uid, _ := c.Get("user_id")
		uidStr, _ := uid.(string)
		h.auditLogger.Log(ctx, "monthly_closing_batches", batch.ID, "generate", nil, map[string]interface{}{
			"batch_id":          batch.ID,
			"accounting_period": req.AccountingPeriod,
			"status":            status,
			"processed":         processed,
			"total_entries":     totalEntries,
		}, uidStr, c)
	}
}

func generateMockPayments(contract *repository.Contract) []ifrs16svc.LeasePayment {
	var payments []ifrs16svc.LeasePayment
	payments = append(payments, ifrs16svc.LeasePayment{
		Date:   contract.CommencementDate,
		Amount: 100000,
		Timing: "postpaid",
		Type:   "fixed",
	})
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
	return payments
}

func generateJournalEntries(contractID, period string, entryDate time.Time, monthly *ifrs16svc.MonthlyEntry, batchID string, measurementBasis string) []*repository.JournalEntry {
	var entries []*repository.JournalEntry

	if measurementBasis == "straight_line_expense" {
		if monthly.ExemptLeaseExpense > 0.01 {
			entries = append(entries, &repository.JournalEntry{
				ContractID:       contractID,
				AccountingPeriod: period,
				EntryDate:        entryDate,
				EntryType:        "lease_expense",
				DebitAccount:     "6603-租赁费用-豁免租赁",
				CreditAccount:    "2202-应付账款",
				Amount:           monthly.ExemptLeaseExpense,
				Currency:         "CNY",
				Description:      strPtr(fmt.Sprintf("短期/低价值租赁直线法费用 %s", period)),
				PostingStatus:    "draft",
				BatchID:          &batchID,
			})
		}
		if monthly.VariableRentExpense > 0.01 {
			entries = append(entries, &repository.JournalEntry{
				ContractID:       contractID,
				AccountingPeriod: period,
				EntryDate:        entryDate,
				EntryType:        "variable_rent",
				DebitAccount:     "6603-租赁费用-变量租金",
				CreditAccount:    "2202-应付账款",
				Amount:           monthly.VariableRentExpense,
				Currency:         "CNY",
				Description:      strPtr(fmt.Sprintf("变量租金 %s", period)),
				PostingStatus:    "draft",
				BatchID:          &batchID,
			})
		}
		if monthly.NonLeaseExpense > 0.01 {
			entries = append(entries, &repository.JournalEntry{
				ContractID:       contractID,
				AccountingPeriod: period,
				EntryDate:        entryDate,
				EntryType:        "non_lease",
				DebitAccount:     "6604-租赁费用-非租赁成分",
				CreditAccount:    "2202-应付账款",
				Amount:           monthly.NonLeaseExpense,
				Currency:         "CNY",
				Description:      strPtr(fmt.Sprintf("非租赁成分 %s", period)),
				PostingStatus:    "draft",
				BatchID:          &batchID,
			})
		}
		return entries
	}

	// Entry 1: Interest expense
	if monthly.InterestExpense > 0.01 {
		entries = append(entries, &repository.JournalEntry{
			ContractID:       contractID,
			AccountingPeriod: period,
			EntryDate:        entryDate,
			EntryType:        "interest",
			DebitAccount:     "6601-租赁利息费用",
			CreditAccount:    "2801-租赁负债",
			Amount:           monthly.InterestExpense,
			Currency:         "CNY",
			Description:      strPtr(fmt.Sprintf("租赁利息费用 %s", period)),
			PostingStatus:    "draft",
			BatchID:          &batchID,
		})
	}

	// Entry 2: Depreciation
	if monthly.Depreciation > 0.01 {
		entries = append(entries, &repository.JournalEntry{
			ContractID:       contractID,
			AccountingPeriod: period,
			EntryDate:        entryDate,
			EntryType:        "depreciation",
			DebitAccount:     "6602-使用权资产折旧",
			CreditAccount:    "1701-使用权资产累计折旧",
			Amount:           monthly.Depreciation,
			Currency:         "CNY",
			Description:      strPtr(fmt.Sprintf("使用权资产折旧 %s", period)),
			PostingStatus:    "draft",
			BatchID:          &batchID,
		})
	}

	// Entry 3: Payment
	if monthly.TotalPayments > 0.01 {
		entries = append(entries, &repository.JournalEntry{
			ContractID:       contractID,
			AccountingPeriod: period,
			EntryDate:        entryDate,
			EntryType:        "payment",
			DebitAccount:     "2801-租赁负债",
			CreditAccount:    "1002-银行存款",
			Amount:           monthly.TotalPayments,
			Currency:         "CNY",
			Description:      strPtr(fmt.Sprintf("租赁付款 %s", period)),
			PostingStatus:    "draft",
			BatchID:          &batchID,
		})
	}

	// Entry 4: Variable rent expense (turnover rent, sales-based rent)
	if monthly.VariableRentExpense > 0.01 {
		entries = append(entries, &repository.JournalEntry{
			ContractID:       contractID,
			AccountingPeriod: period,
			EntryDate:        entryDate,
			EntryType:        "variable_rent",
			DebitAccount:     "6603-租赁费用-变量租金",
			CreditAccount:    "2202-应付账款",
			Amount:           monthly.VariableRentExpense,
			Currency:         "CNY",
			Description:      strPtr(fmt.Sprintf("变量租金 %s", period)),
			PostingStatus:    "draft",
			BatchID:          &batchID,
		})
	}

	// Entry 5: Non-lease expense (CAM, service charge, management fee, etc.)
	if monthly.NonLeaseExpense > 0.01 {
		entries = append(entries, &repository.JournalEntry{
			ContractID:       contractID,
			AccountingPeriod: period,
			EntryDate:        entryDate,
			EntryType:        "non_lease",
			DebitAccount:     "6604-租赁费用-非租赁成分",
			CreditAccount:    "2202-应付账款",
			Amount:           monthly.NonLeaseExpense,
			Currency:         "CNY",
			Description:      strPtr(fmt.Sprintf("非租赁成分 %s", period)),
			PostingStatus:    "draft",
			BatchID:          &batchID,
		})
	}

	return entries
}

func strPtr(s string) *string { return &s }

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
		h.auditLogger.Log(c.Request.Context(), "journal_entries", entryID, "approve", nil, nil, userIDStr, c)
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
	template := c.DefaultQuery("template", "kingdee")
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

// GetJournalEntries returns journal entries
func (h *MonthlyClosingHandler) GetJournalEntries(c *gin.Context) {
	contractID := c.Query("contract_id")
	period := c.Query("period")
	status := c.Query("status")
	ctx := c.Request.Context()
	legalEntityID := middleware.GetTenantID(c)

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

	entries, err := h.mcRepo.GetJournalEntries(ctx, contractID, period, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  entries,
		"total": len(entries),
	})
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
