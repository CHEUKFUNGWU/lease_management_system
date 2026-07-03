package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
)

func approvalAuditValues(c *gin.Context, values map[string]interface{}) map[string]interface{} {
	if values == nil {
		values = map[string]interface{}{}
	}
	if reason, ok := middleware.GetAdminOverrideReason(c); ok {
		values["administrative_override"] = true
		values["administrative_override_reason"] = reason
	}
	return values
}
