package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/access"
)

type fakeContractScopeStore struct {
	attributes access.ContractAttributes
}

func (s fakeContractScopeStore) GetContractAttributes(context.Context, string) (access.ContractAttributes, bool, error) {
	return s.attributes, true, nil
}

func TestContractScopeMiddlewareHidesContractOutsideNarrowingScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		scope := access.Scope{LegalEntityID: "le-001", StoreIDs: []string{"store-1"}}
		c.Set("access_scope", scope)
		c.Next()
	})
	router.GET("/contracts/:id",
		RequireContractScope(fakeContractScopeStore{attributes: access.ContractAttributes{
			LegalEntityID: "le-001",
			StoreID:       "store-2",
		}}, "id"),
		func(c *gin.Context) { c.Status(http.StatusNoContent) },
	)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/contracts/contract-1", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, recorder.Code)
	}
}
