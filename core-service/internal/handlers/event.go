package handlers

import (
	"net/http"
	"time"
	
	"github.com/gin-gonic/gin"
	"github.com/ifrs16/core-service/internal/middleware"
	"github.com/ifrs16/core-service/internal/repository"
)

type EventHandler struct {
	eventRepo    *repository.EventRepository
	contractRepo *repository.ContractRepository
}

func NewEventHandler(eventRepo *repository.EventRepository, contractRepo *repository.ContractRepository) *EventHandler {
	return &EventHandler{eventRepo: eventRepo, contractRepo: contractRepo}
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
