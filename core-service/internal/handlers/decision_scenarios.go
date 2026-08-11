package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/draftapp"
	"github.com/lease-management-system/core-service/internal/services/operating"
)

type DecisionScenarioHandler struct{ draftService *draftapp.Service }

func NewDecisionScenarioHandler(draftServices ...*draftapp.Service) *DecisionScenarioHandler {
	var service *draftapp.Service
	if len(draftServices) > 0 {
		service = draftServices[0]
	}
	return &DecisionScenarioHandler{draftService: service}
}

type storeScenarioRequest struct {
	Scenarios []operating.StoreDecisionScenario `json:"scenarios" binding:"required,min=2"`
}

func (h *DecisionScenarioHandler) Store(c *gin.Context) {
	var req storeScenarioRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := operating.EvaluateStoreScenarios(req.Scenarios)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "review_required": true})
		return
	}
	c.JSON(http.StatusOK, gin.H{"basis": "Scenario", "data": result, "total": len(result), "review_required": true, "side_effects": false})
}

type equipmentScenarioRequest struct {
	Scenarios []operating.EquipmentDecisionScenario `json:"scenarios" binding:"required,min=2"`
}

func (h *DecisionScenarioHandler) Equipment(c *gin.Context) {
	var req equipmentScenarioRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := operating.EvaluateEquipmentScenarios(req.Scenarios)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error(), "review_required": true})
		return
	}
	c.JSON(http.StatusOK, gin.H{"basis": "Scenario", "data": result, "total": len(result), "review_required": true, "side_effects": false})
}

// StoreDecisionEventDraft is the only bridge from a reviewed commercial
// decision into lease accounting. It deliberately creates a draft event via
// draftapp; approval and recalculation remain separate workflow steps.
func (h *DecisionScenarioHandler) StoreDecisionEventDraft(c *gin.Context) {
	if h.draftService == nil {
		c.JSON(http.StatusNotImplemented, gin.H{"error": "draft service is not configured"})
		return
	}
	var req struct {
		ContractID    string         `json:"contract_id" binding:"required"`
		Approved      bool           `json:"approved"`
		EventType     string         `json:"event_type"`
		EffectiveDate string         `json:"effective_date" binding:"required"`
		ChangeReason  string         `json:"change_reason" binding:"required"`
		Decision      string         `json:"decision" binding:"required"`
		ScenarioName  string         `json:"scenario_name"`
		OriginalValue string         `json:"original_value"`
		NewValue      string         `json:"new_value"`
		EvidenceRef   map[string]any `json:"evidence_ref" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !req.Approved {
		c.JSON(http.StatusConflict, gin.H{"error": "only an approved decision can create a lease event draft", "review_required": true})
		return
	}
	effective, err := time.Parse("2006-01-02", req.EffectiveDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "effective_date must be YYYY-MM-DD"})
		return
	}
	eventType := strings.TrimSpace(req.EventType)
	if eventType == "" {
		eventType = "lease_modification"
	}
	reason := fmt.Sprintf("%s decision: %s", strings.TrimSpace(req.Decision), strings.TrimSpace(req.ChangeReason))
	event := &repository.LeaseEvent{ContractID: req.ContractID, EventType: eventType, EffectiveDate: effective, ChangeReason: &reason, OriginalValue: optionalString(req.OriginalValue), NewValue: optionalString(req.NewValue), JudgmentBasis: optionalString("approved_store_decision"), SourceReferenceLocator: req.EvidenceRef}
	result := h.draftService.CreateEventDraft(c.Request.Context(), draftapp.EventDraftCommand{IdempotencyKey: c.GetHeader("Idempotency-Key"), ActorID: userIDFromContext(c), Event: event, EvidenceRef: req.EvidenceRef, RequireEvidence: true})
	if result.Status == draftapp.ItemFailed {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"data": result, "review_required": true})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": result, "basis": "Scenario", "review_required": true, "formal_event_created": false})
}
