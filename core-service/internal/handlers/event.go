package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
	contractsvc "github.com/lease-management-system/core-service/internal/services/contracts"
	"github.com/lease-management-system/core-service/internal/services/eventaccounting"
	"github.com/lease-management-system/core-service/internal/services/ifrs16"
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
	// RevisionParameters states the rent clause as the landlord's notice puts
	// it, so the revised payment schedule is derived rather than retyped.
	RevisionParameters *ifrs16.PaymentRevision `json:"revision_parameters"`
}

// encodeRevision validates the clause before it is stored. A clause that cannot
// produce a schedule is rejected at the door rather than at month end.
func encodeRevision(revision *ifrs16.PaymentRevision) ([]byte, error) {
	if revision == nil {
		return nil, nil
	}
	// Deriving against an empty schedule exercises every validation the clause
	// itself can fail, without needing the contract's payments.
	if _, err := ifrs16.DeriveRevisedPayments(nil, *revision, time.Time{}); err != nil {
		return nil, err
	}
	return json.Marshal(revision)
}

// clauseError states, in the language the rest of the product speaks, that the
// terms cannot produce a schedule. The engine's own wording is appended because
// it names which part is wrong, which is what the person fixing it needs.
func clauseError(err error) string {
	reasons := map[string]string{
		"both index readings are required and must be positive":          "指数联动条款需要基期与现期两个指数，且均须为正数",
		"a stepped revision needs at least one step":                     "阶梯租金条款至少需要一级阶梯",
		"every step needs a start date":                                  "每一级阶梯都需要填写起始日",
		"step amount cannot be negative":                                 "阶梯租金不能为负数",
		"revised rent cannot be negative":                                "调整后租金不能为负数",
		"the stated index movement would take the rent to zero or below": "按所填指数计算，租金将降至零或以下",
	}
	if translated, known := reasons[err.Error()]; known {
		return translated
	}
	if strings.HasPrefix(err.Error(), "unknown revision kind") {
		return "条款类型无法识别"
	}
	if strings.Contains(err.Error(), "would take the rent to zero or below") {
		return "降幅过大，租金将降至零或以下"
	}
	if strings.Contains(err.Error(), "before it starts") {
		return "条款结束日早于起始日"
	}
	return "条款参数无法推导付款流：" + err.Error()
}

// decodeRevision reads a stored clause back. An event recorded before clauses
// existed has none, and must keep calculating from its free-text value.
func decodeRevision(raw []byte) (*ifrs16.PaymentRevision, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	revision := &ifrs16.PaymentRevision{}
	if err := json.Unmarshal(raw, revision); err != nil {
		return nil, err
	}
	return revision, nil
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

	clause, err := encodeRevision(req.RevisionParameters)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": clauseError(err)})
		return
	}

	event := &repository.LeaseEvent{
		ContractID:         req.ContractID,
		EventType:          req.EventType,
		EffectiveDate:      ed,
		OriginalValue:      req.OriginalValue,
		NewValue:           req.NewValue,
		ChangeReason:       &req.ChangeReason,
		JudgmentBasis:      &req.JudgmentBasis,
		CreatedBy:          &userIDStr,
		RevisionParameters: clause,
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

	revision, err := decodeRevision(event.RevisionParameters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "stored clause is unreadable: " + err.Error()})
		return
	}
	accountingResult, err := eventaccounting.Calculate(eventaccounting.Input{
		EventID: eventID, ContractID: contractID, EventType: event.EventType,
		EffectiveDate: event.EffectiveDate, CommencementDate: contract.CommencementDate,
		LeaseEndDate: contract.LeaseEndDate, NewValue: event.NewValue, Revision: revision,
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
type PreviewPaymentsRequest struct {
	EffectiveDate      string                 `json:"effective_date" binding:"required"`
	RevisionParameters ifrs16.PaymentRevision `json:"revision_parameters" binding:"required"`
}

// PreviewRevisedPayments derives the payment schedule a clause implies, without
// writing anything. It exists so the revised schedule can be read and agreed
// before it is committed, which is what replaces editing the schedule by hand
// and then hoping the event recorded alongside it says the same thing.
//
// POST /contracts/:id/events/preview-payments
func (h *EventHandler) PreviewRevisedPayments(c *gin.Context) {
	contractID := c.Param("id")
	ctx := c.Request.Context()

	var req PreviewPaymentsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	effectiveDate, err := time.Parse("2006-01-02", req.EffectiveDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "生效日期格式应为 YYYY-MM-DD"})
		return
	}

	contract, err := h.contractRepo.GetByID(ctx, contractID, middleware.GetTenantID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify contract: " + err.Error()})
		return
	}
	if contract == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
		return
	}

	schedules, err := h.psRepo.GetByContractID(ctx, contractID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get payment schedules: " + err.Error()})
		return
	}

	draft, err := ifrs16.DeriveRevisedPayments(
		repository.ToIFRS16Payments(schedules), req.RevisionParameters, effectiveDate)
	if err != nil {
		// The clause itself does not work; this is the caller's mistake to fix,
		// not a server fault. The message is wrapped the same way the create
		// path wraps it, so the two routes tell the user the same thing.
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": clauseError(err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"contract_id":    contractID,
		"currency":       contract.Currency,
		"effective_date": effectiveDate.Format("2006-01-02"),
		"draft":          draft,
	})
}

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
	revision, err := decodeRevision(event.RevisionParameters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "stored clause is unreadable: " + err.Error()})
		return
	}
	result, err := eventaccounting.Calculate(eventaccounting.Input{
		EventID: eventID, ContractID: contractID, EventType: event.EventType,
		EffectiveDate: event.EffectiveDate, CommencementDate: contract.CommencementDate,
		LeaseEndDate: contract.LeaseEndDate, NewValue: event.NewValue, Revision: revision,
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
