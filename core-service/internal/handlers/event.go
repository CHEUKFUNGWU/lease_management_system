package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ifrs16/core-service/internal/middleware"
	"github.com/ifrs16/core-service/internal/repository"
	"github.com/ifrs16/core-service/internal/services/audit"
	ifrs16svc "github.com/ifrs16/core-service/internal/services/ifrs16"
)

type EventHandler struct {
	eventRepo    *repository.EventRepository
	contractRepo *repository.ContractRepository
	mcRepo       *repository.MonthlyClosingRepository
	psRepo       *repository.PaymentScheduleRepository
	auditLogger  *audit.Logger
}

func NewEventHandler(eventRepo *repository.EventRepository, contractRepo *repository.ContractRepository, mcRepo *repository.MonthlyClosingRepository, psRepo *repository.PaymentScheduleRepository, auditLogger *audit.Logger) *EventHandler {
	return &EventHandler{eventRepo: eventRepo, contractRepo: contractRepo, mcRepo: mcRepo, psRepo: psRepo, auditLogger: auditLogger}
}

type CreateEventRequest struct {
	ContractID       string  `json:"contract_id" binding:"required,uuid"`
	EventType        string  `json:"event_type" binding:"required"`
	EffectiveDate    string  `json:"effective_date" binding:"required"`
	OriginalValue    *string `json:"original_value"`
	NewValue         *string `json:"new_value"`
	ChangeReason     string  `json:"change_reason" binding:"required"`
	JudgmentBasis    string  `json:"judgment_basis"`
}

func (h *EventHandler) Create(c *gin.Context) {
	var req CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	ed, _ := time.Parse("2006-01-02", req.EffectiveDate)
	
	event := &repository.LeaseEvent{
		ContractID:    req.ContractID,
		EventType:     req.EventType,
		EffectiveDate: ed,
		OriginalValue: req.OriginalValue,
		NewValue:      req.NewValue,
		ChangeReason:  &req.ChangeReason,
		JudgmentBasis: &req.JudgmentBasis,
	}
	
	result, err := h.eventRepo.Create(c.Request.Context(), event)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Audit log: event created
	if h.auditLogger != nil {
		uid, _ := c.Get("user_id")
		uidStr, _ := uid.(string)
		h.auditLogger.Log(c.Request.Context(), "lease_events", result.ID, "create", nil, result, uidStr, c)
	}
	
	c.JSON(http.StatusOK, result)
}

func (h *EventHandler) ListByContract(c *gin.Context) {
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
	
	events, err := h.eventRepo.GetByContractID(ctx, contractID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"data":  events,
		"total": len(events),
	})
}

func (h *EventHandler) SubmitForReview(c *gin.Context) {
	eventID := c.Param("eventId")
	
	ctx := c.Request.Context()
	event, err := h.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify event: " + err.Error()})
		return
	}
	if event == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "event not found"})
		return
	}
	
	if err := h.eventRepo.SubmitForReview(ctx, eventID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to submit: " + err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"event_id": eventID,
		"status":   "submitted",
		"message":  "事件已提交复核",
	})
}

type EventReviewRequest struct {
	Approved bool   `json:"approved"`
	Reason   string `json:"reason"`
}

func (h *EventHandler) Review(c *gin.Context) {
	eventID := c.Param("eventId")
	
	var req EventReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	ctx := c.Request.Context()
	event, err := h.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify event: " + err.Error()})
		return
	}
	if event == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "event not found"})
		return
	}
	
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	
	if err := h.eventRepo.Review(ctx, eventID, userIDStr, req.Approved, req.Reason); err != nil {
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
		"event_id": eventID,
		"status":   status,
		"message":  message,
	})
}

func (h *EventHandler) Approve(c *gin.Context) {
	eventID := c.Param("eventId")
	
	ctx := c.Request.Context()
	event, err := h.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify event: " + err.Error()})
		return
	}
	if event == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "event not found"})
		return
	}
	
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	
	// Auto-classify event for IFRS 16 treatment
	treatment := repository.ClassifyEventType(event.EventType)
	
	if err := h.eventRepo.Approve(ctx, eventID, userIDStr, treatment); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to approve: " + err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"event_id": eventID,
		"status":   "approved",
		"treatment": treatment,
		"message":  "事件已审批通过",
	})

	// Audit log: event approved
	if h.auditLogger != nil {
		h.auditLogger.Log(ctx, "lease_events", eventID, "approve", nil, event, userIDStr, c)
	}
}

func (h *EventHandler) Reject(c *gin.Context) {
	eventID := c.Param("eventId")
	
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	ctx := c.Request.Context()
	event, err := h.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify event: " + err.Error()})
		return
	}
	if event == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "event not found"})
		return
	}
	
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	
	if err := h.eventRepo.Reject(ctx, eventID, userIDStr, req.Reason); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reject: " + err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"event_id": eventID,
		"status":   "rejected",
		"message":  "事件已驳回",
	})
}

// RecalculateEvent performs full IFRS 16 recalculation for an approved event.
// POST /contracts/:id/events/:eventId/recalculate
func (h *EventHandler) RecalculateEvent(c *gin.Context) {
	contractID := c.Param("id")
	eventID := c.Param("eventId")
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

	// Get the event
	event, err := h.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get event: " + err.Error()})
		return
	}
	if event == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "event not found"})
		return
	}
	if event.ApprovalStatus != "approved" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "event must be approved before recalculation"})
		return
	}
	if event.ContractID != contractID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "event does not belong to this contract"})
		return
	}

	// Check if already recalculated
	existingAdjustment, _ := h.mcRepo.GetEventAdjustmentByEventID(ctx, eventID)
	if existingAdjustment != nil {
		c.JSON(http.StatusOK, gin.H{
			"message":    "Event has already been recalculated",
			"adjustment": existingAdjustment,
		})
		return
	}

	// Get payment schedules
	schedules, err := h.psRepo.GetByContractID(ctx, contractID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get payment schedules: " + err.Error()})
		return
	}
	payments := repository.ToIFRS16Payments(schedules)

	// Determine discount rate: use contract's discount rate if confirmed, otherwise default to 0.05
	discountRate := 0.05
	if contract.DiscountRateType != nil {
		// For MVP, use a default. The DiscountRateType is a type string, not a numeric value.
		// We'll use a reasonable default or extract from the existing calculation.
		discountRate = 0.05
	}

	// Get lease end date, possibly revised by event
	leaseEndDate := contract.LeaseEndDate
	if event.EventType == "early_termination" && event.NewValue != nil {
		if parsedDate, err := time.Parse("2006-01-02", *event.NewValue); err == nil {
			leaseEndDate = parsedDate
		}
	}
	if event.EventType == "renewal" && event.NewValue != nil {
		if parsedDate, err := time.Parse("2006-01-02", *event.NewValue); err == nil {
			leaseEndDate = parsedDate
		}
	}

	// Calculate carrying amounts as of the day before effective date
	calcInput := ifrs16svc.LeaseCalculation{
		CommencementDate: contract.CommencementDate,
		LeaseEndDate:     leaseEndDate,
		DiscountRate:     discountRate,
		Payments:         payments,
	}

	carryingLiability, carryingROU, err := ifrs16svc.GetCarryingAmount(calcInput, event.EffectiveDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get carrying amount: " + err.Error()})
		return
	}

	// Build revised payments for remeasurement
	// For MVP: keep existing payments; future enhancement: modify per event type
	revisedPayments := payments

	// Determine treatment classification
	treatment := repository.ClassifyEventType(event.EventType)

	// Perform remeasurement
	remeasurementInput := ifrs16svc.RemeasurementInput{
		EffectiveDate:       event.EffectiveDate,
		LeaseEndDate:        leaseEndDate,
		RevisedDiscountRate: discountRate,
		RevisedPayments:     revisedPayments,
	}
	remeasurementResult, err := ifrs16svc.RecalculateFromDate(carryingLiability, carryingROU, remeasurementInput)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "recalculation failed: " + err.Error()})
		return
	}

	// Create event_adjustments record
	adjustment := &repository.EventAdjustment{
		EventID:             eventID,
		ContractID:          contractID,
		AdjustmentType:      treatment,
		EffectiveDate:       event.EffectiveDate,
		LiabilityBefore:     carryingLiability,
		LiabilityAfter:      remeasurementResult.NewLiability,
		LiabilityAdjustment: remeasurementResult.LiabilityDelta,
		ROUBefore:           carryingROU,
		ROUAfter:            remeasurementResult.NewROU,
		ROUAdjustment:       remeasurementResult.ROUAdjustment,
		PnLGain:             remeasurementResult.PnLGain,
		PnLLoss:             remeasurementResult.PnLLoss,
		RevisedDiscountRate: discountRate,
	}

	adjustment, err = h.mcRepo.CreateEventAdjustment(ctx, adjustment)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create adjustment: " + err.Error()})
		return
	}

	// Generate forward measurement results from effective date
	effectivePeriod := event.EffectiveDate.Format("2006-01")
	now := time.Now()
	for _, dailyEntry := range remeasurementResult.ForwardSchedule {
		period := dailyEntry.Date.Format("2006-01")
		periodStart, _ := time.Parse("2006-01", period)
		periodEnd := periodStart.AddDate(0, 1, -1)

		mr := &repository.MeasurementResult{
			ContractID:          contractID,
			AccountingPeriod:    period,
			PeriodStartDate:     periodStart,
			PeriodEndDate:       periodEnd,
			OpeningLiability:    dailyEntry.OpeningLiability,
			InterestExpense:     dailyEntry.InterestExpense,
			TotalPayment:        dailyEntry.Payment,
			ClosingLiability:    dailyEntry.ClosingLiability,
			OpeningROUAsset:     dailyEntry.OpeningROUAsset,
			Depreciation:        dailyEntry.Depreciation,
			ClosingROUAsset:     dailyEntry.ClosingROUAsset,
			DiscountRate:        discountRate,
			IsCalculated:        true,
			CalculationBatchID:  adjustment.CalculationBatchID,
			CalculatedAt:        &now,
		}
		if err := h.mcRepo.SaveMeasurementResult(ctx, mr); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save measurement result for period " + period + ": " + err.Error()})
			return
		}
	}

	// Create journal entries for the adjustment
	// Modification/Reassessment entry: adjust ROU and liability
	if treatment == "modification" || treatment == "reassessment" {
		// Entry for liability and ROU adjustment
		var description string
		var entryType string
		if treatment == "modification" {
			entryType = "modification"
			description = fmt.Sprintf("租赁修改调整 - 事件 %s", eventID[:8])
		} else {
			entryType = "reassessment"
			description = fmt.Sprintf("租赁重新评估 - 事件 %s", eventID[:8])
		}

		// Debit/Credit based on whether liability increased or decreased
		liabilityDelta := remeasurementResult.LiabilityDelta

		effectiveDateEnd := event.EffectiveDate.AddDate(0, 1, -1)
		effPeriod := event.EffectiveDate.Format("2006-01")

		if liabilityDelta > 0.01 {
			// Liability increased: Dr ROU, Cr Liability
			je := &repository.JournalEntry{
				ContractID:       contractID,
				AccountingPeriod: effPeriod,
				EntryDate:        effectiveDateEnd,
				EntryType:        entryType,
				DebitAccount:     "1701-使用权资产",
				CreditAccount:    "2801-租赁负债",
				Amount:           liabilityDelta,
				Currency:         contract.Currency,
				Description:      &description,
				PostingStatus:    "draft",
			}
			if err := h.mcRepo.CreateJournalEntry(ctx, je); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create liability adjustment entry: " + err.Error()})
				return
			}
		} else if liabilityDelta < -0.01 {
			// Liability decreased: Dr Liability, Cr ROU
			desc := description + " (负债减少)"
			je := &repository.JournalEntry{
				ContractID:       contractID,
				AccountingPeriod: effPeriod,
				EntryDate:        effectiveDateEnd,
				EntryType:        entryType,
				DebitAccount:     "2801-租赁负债",
				CreditAccount:    "1701-使用权资产",
				Amount:           -liabilityDelta,
				Currency:         contract.Currency,
				Description:      &desc,
				PostingStatus:    "draft",
			}
			if err := h.mcRepo.CreateJournalEntry(ctx, je); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create liability reduction entry: " + err.Error()})
				return
			}
		}

		// P&L gain entry if ROU reduction exceeds carrying amount
		if remeasurementResult.PnLGain > 0.01 {
			desc := description + " (处置收益)"
			je := &repository.JournalEntry{
				ContractID:       contractID,
				AccountingPeriod: effPeriod,
				EntryDate:        effectiveDateEnd,
				EntryType:        entryType,
				DebitAccount:     "2801-租赁负债",
				CreditAccount:    "6301-资产处置收益",
				Amount:           remeasurementResult.PnLGain,
				Currency:         contract.Currency,
				Description:      &desc,
				PostingStatus:    "draft",
			}
			if err := h.mcRepo.CreateJournalEntry(ctx, je); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create P&L gain entry: " + err.Error()})
				return
			}
		}

		// P&L loss entry if applicable
		if remeasurementResult.PnLLoss > 0.01 {
			desc := description + " (处置损失)"
			je := &repository.JournalEntry{
				ContractID:       contractID,
				AccountingPeriod: effPeriod,
				EntryDate:        effectiveDateEnd,
				EntryType:        entryType,
				DebitAccount:     "6711-资产处置损失",
				CreditAccount:    "2801-租赁负债",
				Amount:           remeasurementResult.PnLLoss,
				Currency:         contract.Currency,
				Description:      &desc,
				PostingStatus:    "draft",
			}
			if err := h.mcRepo.CreateJournalEntry(ctx, je); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create P&L loss entry: " + err.Error()})
				return
			}
		}
	}

	// Impairment entry
	if treatment == "impairment" {
		impairmentAmount := carryingROU - remeasurementResult.NewROU
		if impairmentAmount > 0.01 {
			desc := fmt.Sprintf("使用权资产减值 - 事件 %s", eventID[:8])
			effectiveDateEnd := event.EffectiveDate.AddDate(0, 1, -1)
			effPeriod := event.EffectiveDate.Format("2006-01")
			je := &repository.JournalEntry{
				ContractID:       contractID,
				AccountingPeriod: effPeriod,
				EntryDate:        effectiveDateEnd,
				EntryType:        "impairment",
				DebitAccount:     "6701-资产减值损失",
				CreditAccount:    "1702-使用权资产减值准备",
				Amount:           impairmentAmount,
				Currency:         contract.Currency,
				Description:      &desc,
				PostingStatus:    "draft",
			}
			if err := h.mcRepo.CreateJournalEntry(ctx, je); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create impairment entry: " + err.Error()})
				return
			}
		}
	}

	// Link recalculation batch to event (use adjustment ID as batch reference)
	if err := h.eventRepo.LinkRecalculationBatch(ctx, eventID, adjustment.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to link batch: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":          "Event recalculated successfully",
		"event_id":         eventID,
		"adjustment":       adjustment,
		"effective_date":   event.EffectiveDate.Format("2006-01-02"),
		"effective_period": effectivePeriod,
		"treatment":        treatment,
		"liability_delta":  remeasurementResult.LiabilityDelta,
		"rou_delta":        remeasurementResult.ROUAdjustment,
		"pnl_gain":         remeasurementResult.PnLGain,
		"pnl_loss":         remeasurementResult.PnLLoss,
		"forward_periods":  len(remeasurementResult.ForwardSchedule),
	})

	// Audit log: event recalculated
	if h.auditLogger != nil {
		uid, _ := c.Get("user_id")
		uidStr, _ := uid.(string)
		h.auditLogger.Log(ctx, "lease_events", eventID, "recalculate", nil, adjustment, uidStr, c)
	}
}

// PreviewEventAdjustment performs calculation without persisting.
// POST /contracts/:id/events/:eventId/preview
func (h *EventHandler) PreviewEventAdjustment(c *gin.Context) {
	contractID := c.Param("id")
	eventID := c.Param("eventId")
	ctx := c.Request.Context()
	legalEntityID := middleware.GetTenantID(c)

	contract, err := h.contractRepo.GetByID(ctx, contractID, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify contract: " + err.Error()})
		return
	}
	if contract == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
		return
	}

	event, err := h.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get event: " + err.Error()})
		return
	}
	if event == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "event not found"})
		return
	}
	if event.ContractID != contractID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "event does not belong to this contract"})
		return
	}

	schedules, err := h.psRepo.GetByContractID(ctx, contractID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get payment schedules: " + err.Error()})
		return
	}
	payments := repository.ToIFRS16Payments(schedules)

	discountRate := 0.05
	leaseEndDate := contract.LeaseEndDate
	if event.EventType == "early_termination" && event.NewValue != nil {
		if parsedDate, err := time.Parse("2006-01-02", *event.NewValue); err == nil {
			leaseEndDate = parsedDate
		}
	}
	if event.EventType == "renewal" && event.NewValue != nil {
		if parsedDate, err := time.Parse("2006-01-02", *event.NewValue); err == nil {
			leaseEndDate = parsedDate
		}
	}

	calcInput := ifrs16svc.LeaseCalculation{
		CommencementDate: contract.CommencementDate,
		LeaseEndDate:     leaseEndDate,
		DiscountRate:     discountRate,
		Payments:         payments,
	}

	carryingLiability, carryingROU, err := ifrs16svc.GetCarryingAmount(calcInput, event.EffectiveDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get carrying amount: " + err.Error()})
		return
	}

	remeasurementInput := ifrs16svc.RemeasurementInput{
		EffectiveDate:       event.EffectiveDate,
		LeaseEndDate:        leaseEndDate,
		RevisedDiscountRate: discountRate,
		RevisedPayments:     payments,
	}
	result, err := ifrs16svc.RecalculateFromDate(carryingLiability, carryingROU, remeasurementInput)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "preview calculation failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"event_id":         eventID,
		"contract_id":      contractID,
		"event_type":       event.EventType,
		"treatment":        repository.ClassifyEventType(event.EventType),
		"effective_date":   event.EffectiveDate.Format("2006-01-02"),
		"liability_before": carryingLiability,
		"liability_after":  result.NewLiability,
		"liability_delta":  result.LiabilityDelta,
		"rou_before":       carryingROU,
		"rou_after":        result.NewROU,
		"rou_delta":        result.ROUAdjustment,
		"pnl_gain":         result.PnLGain,
		"pnl_loss":         result.PnLLoss,
		"forward_periods":  len(result.ForwardSchedule),
	})
}

// GetEventAdjustment returns the event_adjustments record for a given event.
// GET /contracts/:id/events/:eventId/adjustment
func (h *EventHandler) GetEventAdjustment(c *gin.Context) {
	contractID := c.Param("id")
	eventID := c.Param("eventId")
	ctx := c.Request.Context()
	legalEntityID := middleware.GetTenantID(c)

	contract, err := h.contractRepo.GetByID(ctx, contractID, legalEntityID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify contract: " + err.Error()})
		return
	}
	if contract == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
		return
	}

	event, err := h.eventRepo.GetByID(ctx, eventID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get event: " + err.Error()})
		return
	}
	if event == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "event not found"})
		return
	}
	if event.ContractID != contractID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "event does not belong to this contract"})
		return
	}

	adjustment, err := h.mcRepo.GetEventAdjustmentByEventID(ctx, eventID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get adjustment: " + err.Error()})
		return
	}
	if adjustment == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no adjustment found for this event"})
		return
	}

	c.JSON(http.StatusOK, adjustment)
}
