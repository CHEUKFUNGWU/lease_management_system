package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestProtectedRouteRejectsUserWithoutRequiredPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api/v1")
	group.Use(func(c *gin.Context) {
		c.Set("permissions", []string{"contracts:read"})
		c.Next()
	})

	protected := NewProtectedRouter(group)
	protected.Handle(http.MethodPost, "/contracts", Permission{
		Resource: "contracts",
		Action:   "create",
	}, func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/contracts", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, recorder.Code)
	}
}

func TestProtectedRouteAllowsPermissionGrantedByAnyAssignedRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	group := router.Group("/api/v1")
	group.Use(func(c *gin.Context) {
		c.Set("permissions", []string{"contracts:update", "contracts:review"})
		c.Next()
	})

	protected := NewProtectedRouter(group)
	protected.Handle(http.MethodPost, "/contracts/contract-1/review", Permission{
		Resource: "contracts",
		Action:   "review",
	}, func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/contract-1/review", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}
}
