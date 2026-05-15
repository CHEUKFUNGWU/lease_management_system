package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/ifrs16/core-service/internal/repository"
)

type AuditHandler struct {
	auditRepo *repository.AuditRepository
}

func NewAuditHandler(auditRepo *repository.AuditRepository) *AuditHandler {
	return &AuditHandler{auditRepo: auditRepo}
}

func (h *AuditHandler) List(c *gin.Context) {
	filter := repository.AuditLogFilter{
		TableName: c.Query("table_name"),
		RecordID:  c.Query("record_id"),
		Action:    c.Query("action"),
		ChangedBy: c.Query("changed_by"),
		StartDate: c.Query("start_date"),
		EndDate:   c.Query("end_date"),
		Limit:     50,
		Offset:    0,
	}
	if limit, err := strconv.Atoi(c.Query("limit")); err == nil && limit > 0 {
		filter.Limit = limit
	}
	if offset, err := strconv.Atoi(c.Query("offset")); err == nil && offset >= 0 {
		filter.Offset = offset
	}

	logs, total, err := h.auditRepo.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": logs, "total": total})
}
