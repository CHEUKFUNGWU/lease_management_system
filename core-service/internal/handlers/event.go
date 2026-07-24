package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
	contractsvc "github.com/lease-management-system/core-service/internal/services/contracts"
	"github.com/lease-management-system/core-service/internal/services/eventaccounting"
)

type EventHandler struct {
	eventRepo         *repository.EventRepository
	contractRepo      *repository.ContractRepository
	mcRepo            *repository.MonthlyClosingRepository
	psRepo            *repository.PaymentScheduleRepository
	systemSettingRepo *repository.SystemSettingRepository
	eventPersistence  *eventaccounting.PersistenceService
	auditLogger       *audit.Logger
}

func NewEventHandler(
	eventRepo *repository.EventRepository,
	contractRepo *repository.ContractRepository,
	mcRepo *repository.MonthlyClosingRepository,
	psRepo *repository.PaymentScheduleRepository,
	systemSettingRepo *repository.SystemSettingRepository,
	eventPersistence *eventaccounting.PersistenceService,
	auditLogger *audit.Logger,
) *EventHandler {
	return &EventHandler{
		eventRepo: eventRepo, contractRepo: contractRepo, mcRepo: mcRepo,
		psRepo: psRepo, systemSettingRepo: systemSettingRepo,
		eventPersistence: eventPersistence, auditLogger: auditLogger,
	}
}

type CreateEventRequest struct {
	ContractID    string  `json:"contract_id" binding:"required,uuid"`
	EventType     string  `json:"event_type" binding:"required"`
	EffectiveDate string  `json:"effective_date" binding:"required"`
	OriginalValue *string `json:"original_value"`
	NewValue      *string `json:"new_value"`
	ChangeReason  string  `json:"change_reason" binding:"required"`
	JudgmentBasis string  `json:"judgment_basis"`
}

func (h *EventHandler) Create(c *gin.Context) {
	var req CreateEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ContractID != c.Param("id") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "contract_id must match route contract"})
		return
	}

	ed, _ := time.Parse("2006-01-02", req.EffectiveDate)
	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	event := &repository.LeaseEvent{
		ContractID:    req.ContractID,
		EventType:     req.EventType,
		EffectiveDate: ed,
		OriginalValue: req.OriginalValue,
		NewValue:      req.NewValue,
		ChangeReason:  &req.ChangeReason,
		JudgmentBasis: &req.JudgmentBasis,
		CreatedBy:     &userIDStr,
	}

	result, err := h.eventRepo.Create(c.Request.Context(), event)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Audit log: event created
	if h.auditLogger != nil {
		h.auditLogger.Log(c.Request.Context(), "lease_events", result.ID, "create", nil, result, userIDStr, c)
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
	if !requireEventOnRouteContract(c, event) {
		return
	}

	if err := h.eventRepo.SubmitForReview(ctx, eventID); err != nil {
		writeWorkflowMutationError(c, "submit event", err)
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

func requireEventOnRouteContract(c *gin.Context, event *repository.LeaseEvent) bool {
	if event.ContractID == c.Param("id") {
		return true
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "event not found"})
	return false
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
	if !requireEventOnRouteContract(c, event) {
		return
	}

	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	if err := h.eventRepo.Review(ctx, eventID, userIDStr, req.Approved, req.Reason); err != nil {
		writeWorkflowMutationError(c, "review event", err)
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
	if !requireEventOnRouteContract(c, event) {
		return
	}

	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	// Auto-classify event for IFRS 16 treatment
	treatment := eventaccounting.Classify(event.EventType)

	if err := h.eventRepo.Approve(ctx, eventID, userIDStr, treatment); err != nil {
		writeWorkflowMutationError(c, "approve event", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"event_id":  eventID,
		"status":    "approved",
		"treatment": treatment,
		"message":   "事件已审批通过",
	})

	// Audit log: event approved
	if h.auditLogger != nil {
		h.auditLogger.Log(ctx, "lease_events", eventID, "approve", nil, approvalAuditValues(c, map[string]interface{}{"event": event}), userIDStr, c)
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
	if !requireEventOnRouteContract(c, event) {
		return
	}

	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)

	if err := h.eventRepo.Reject(ctx, eventID, userIDStr, req.Reason); err != nil {
		writeWorkflowMutationError(c, "reject event", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"event_id": eventID,
		"status":   "rejected",
		"message":  "事件已驳回",
	})
	if h.auditLogger != nil {
		h.auditLogger.Log(ctx, "lease_events", eventID, "reject", nil, approvalAuditValues(c, map[string]interface{}{"reason": req.Reason}), userIDStr, c)
	}
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

	// Get payment schedules
	schedules, err := h.psRepo.GetByContractID(ctx, contractID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get payment schedules: " + err.Error()})
		return
	}
	payments := repository.ToIFRS16Payments(schedules)
	discountRate, _, err := contractsvc.ResolveDiscountRate(ctx, 0, h.systemSettingRepo, contract)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "discount_rate_missing": true})
		return
	}

	accountingResult, err := eventaccounting.Calculate(eventaccounting.Input{
		EventID: eventID, ContractID: contractID, EventType: event.EventType,
		EffectiveDate: event.EffectiveDate, CommencementDate: contract.CommencementDate,
		LeaseEndDate: contract.LeaseEndDate, NewValue: event.NewValue,
		Currency: contract.Currency, DiscountRate: discountRate, Payments: payments,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "recalculation failed: " + err.Error()})
		return
	}
	treatment := accountingResult.Treatment
	accountingAdjustment := accountingResult.Adjustment
	uid, _ := c.Get("user_id")
	uidStr, _ := uid.(string)
	adjustment, err := h.eventPersistence.Commit(ctx, accountingResult, audit.MetadataFromGin(uidStr, c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist recalculation: " + err.Error()})
		return
	}

	effectivePeriod := event.EffectiveDate.Format("2006-01")
	c.JSON(http.StatusOK, gin.H{
		"message":          "Event recalculated successfully",
		"event_id":         eventID,
		"adjustment":       adjustment,
		"effective_date":   event.EffectiveDate.Format("2006-01-02"),
		"effective_period": effectivePeriod,
		"treatment":        treatment,
		"liability_delta":  accountingAdjustment.LiabilityAdjustment,
		"rou_delta":        accountingAdjustment.ROUAdjustment,
		"pnl_gain":         accountingAdjustment.PnLGain,
		"pnl_loss":         accountingAdjustment.PnLLoss,
		"forward_periods":  len(accountingResult.ForwardSchedule),
	})
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

	discountRate, _, err := contractsvc.ResolveDiscountRate(ctx, 0, h.systemSettingRepo, contract)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "discount_rate_missing": true})
		return
	}
	result, err := eventaccounting.Calculate(eventaccounting.Input{
		EventID: eventID, ContractID: contractID, EventType: event.EventType,
		EffectiveDate: event.EffectiveDate, CommencementDate: contract.CommencementDate,
		LeaseEndDate: contract.LeaseEndDate, NewValue: event.NewValue,
		Currency: contract.Currency, DiscountRate: discountRate, Payments: payments,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "preview calculation failed: " + err.Error()})
		return
	}
	adjustment := result.Adjustment

	c.JSON(http.StatusOK, gin.H{
		"event_id":         eventID,
		"contract_id":      contractID,
		"event_type":       event.EventType,
		"treatment":        result.Treatment,
		"effective_date":   event.EffectiveDate.Format("2006-01-02"),
		"liability_before": adjustment.LiabilityBefore,
		"liability_after":  adjustment.LiabilityAfter,
		"liability_delta":  adjustment.LiabilityAdjustment,
		"rou_before":       adjustment.ROUBefore,
		"rou_after":        adjustment.ROUAfter,
		"rou_delta":        adjustment.ROUAdjustment,
		"pnl_gain":         adjustment.PnLGain,
		"pnl_loss":         adjustment.PnLLoss,
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
