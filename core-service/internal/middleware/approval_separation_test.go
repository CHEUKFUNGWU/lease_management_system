package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/access"
)

type fakeApprovalParticipantStore struct {
	participants access.ApprovalParticipants
}

func (s fakeApprovalParticipantStore) GetApprovalParticipants(context.Context, string, string) (access.ApprovalParticipants, bool, error) {
	return s.participants, true, nil
}

func TestApprovalSeparationRejectsCreatorApprovingOwnRecord(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Set("permissions", []string{"contracts:approve"})
		c.Next()
	})
	router.POST("/contracts/:id/approve",
		RequireApprovalSeparation(fakeApprovalParticipantStore{participants: access.ApprovalParticipants{
			CreatorID: "user-1",
		}}, "contract", "id"),
		func(c *gin.Context) { c.Status(http.StatusNoContent) },
	)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/contracts/contract-1/approve", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}
}

func TestAdministrativeOverrideRequiresAndPropagatesReason(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "admin-1")
		c.Set("permissions", []string{"*:*"})
		c.Next()
	})
	router.POST("/contracts/:id/approve",
		RequireApprovalSeparation(fakeApprovalParticipantStore{participants: access.ApprovalParticipants{
			CreatorID: "admin-1",
		}}, "contract", "id"),
		func(c *gin.Context) {
			reason, ok := GetAdminOverrideReason(c)
			if !ok || reason != "emergency month-end close" {
				c.Status(http.StatusInternalServerError)
				return
			}
			c.Status(http.StatusNoContent)
		},
	)

	withoutReason := httptest.NewRecorder()
	router.ServeHTTP(withoutReason, httptest.NewRequest(http.MethodPost, "/contracts/contract-1/approve", nil))
	if withoutReason.Code != http.StatusForbidden {
		t.Fatalf("expected missing override reason to return %d, got %d", http.StatusForbidden, withoutReason.Code)
	}

	withReason := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/contracts/contract-1/approve", nil)
	request.Header.Set("X-Admin-Override-Reason", "emergency month-end close")
	router.ServeHTTP(withReason, request)
	if withReason.Code != http.StatusNoContent {
		t.Fatalf("expected reasoned override to return %d, got %d", http.StatusNoContent, withReason.Code)
	}
}
