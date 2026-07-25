package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
)

type WorkQueueHandler struct {
	repo *repository.WorkQueueRepository
}

func NewWorkQueueHandler(repo *repository.WorkQueueRepository) *WorkQueueHandler {
	return &WorkQueueHandler{repo: repo}
}

// Get returns everything currently waiting on the caller, across contracts,
// events, journal entries and critical dates.
//
// The list is scoped by the caller's data scope rather than by role: a user sees
// the whole backlog they own, and the actions they may take on each item are
// still gated by the endpoints behind them.
func (h *WorkQueueHandler) Get(c *gin.Context) {
	windowDays := 30
	if raw := c.Query("critical_date_days"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			windowDays = parsed
		}
	}

	queue, err := h.repo.Load(c.Request.Context(), middleware.GetTenantID(c), windowDays)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, queue)
}
