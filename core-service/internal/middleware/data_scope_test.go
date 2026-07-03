package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/access"
)

func TestDataScopeMiddlewareBuildsNarrowingScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("legal_entity_id", "le-001")
		c.Set("permissions", []string{"contracts:read"})
		c.Set("data_scopes", map[string][]string{
			"store":  {"store-1"},
			"region": {"east"},
			"brand":  {"brand-a"},
		})
		c.Next()
	})
	router.Use(DataScopeMiddleware())
	router.GET("/check", func(c *gin.Context) {
		scope, ok := GetAccessScope(c)
		if !ok {
			c.Status(http.StatusInternalServerError)
			return
		}
		if !scope.AllowsContract(access.ContractAttributes{
			LegalEntityID: "le-001",
			StoreID:       "store-1",
			Region:        "east",
			Brand:         "brand-a",
		}) {
			c.Status(http.StatusForbidden)
			return
		}
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/check", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, recorder.Code)
	}
}

func TestLegalEntityWideOperationRejectsNarrowingScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("access_scope", access.Scope{LegalEntityID: "le-001", StoreIDs: []string{"store-1"}})
		c.Next()
	})
	router.POST("/lock", RequireLegalEntityWideScope(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/lock", nil))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected narrowed scope to return %d, got %d", http.StatusForbidden, recorder.Code)
	}
}

func TestLegalEntityWideOperationAllowsFullEntityScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("access_scope", access.Scope{LegalEntityID: "le-001"})
		c.Next()
	})
	router.POST("/lock", RequireLegalEntityWideScope(), func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/lock", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected full legal entity scope to return %d, got %d", http.StatusNoContent, recorder.Code)
	}
}
