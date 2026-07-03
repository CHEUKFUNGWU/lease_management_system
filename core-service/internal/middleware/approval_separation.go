package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/access"
)

const adminOverrideReasonKey = "admin_override_reason"

type ApprovalParticipantStore interface {
	GetApprovalParticipants(ctx context.Context, recordType, recordID string) (access.ApprovalParticipants, bool, error)
}

func RequireApprovalSeparation(store ApprovalParticipantStore, recordType, parameterName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, _ := c.Get("user_id")
		actorID, _ := actor.(string)
		participants, found, err := store.GetApprovalParticipants(c.Request.Context(), recordType, c.Param(parameterName))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify segregation of duties"})
			c.Abort()
			return
		}
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "approval record not found"})
			c.Abort()
			return
		}
		if actorID == "" {
			c.JSON(http.StatusForbidden, gin.H{"error": "approver must differ from creator and reviewer"})
			c.Abort()
			return
		}
		isSelfApproval := actorID == participants.CreatorID || actorID == participants.ReviewerID
		if isSelfApproval {
			permissions, _ := c.Get("permissions")
			permissionStrings, _ := permissions.([]string)
			reason := strings.TrimSpace(c.GetHeader("X-Admin-Override-Reason"))
			if !HasPermission(permissionStrings, "*", "*") || reason == "" {
				c.JSON(http.StatusForbidden, gin.H{"error": "approver must differ from creator and reviewer; admin override requires a reason"})
				c.Abort()
				return
			}
			c.Set(adminOverrideReasonKey, reason)
		}
		c.Next()
	}
}

func GetAdminOverrideReason(c *gin.Context) (string, bool) {
	value, exists := c.Get(adminOverrideReasonKey)
	if !exists {
		return "", false
	}
	reason, ok := value.(string)
	return reason, ok && reason != ""
}
