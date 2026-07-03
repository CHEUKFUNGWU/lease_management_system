package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/repository"
)

type fakeRoleAuthorityStore struct{}

func (fakeRoleAuthorityStore) GetUserRoleCodes(context.Context, string) ([]string, error) {
	return []string{"editor", "reviewer"}, nil
}

func (fakeRoleAuthorityStore) GetUserPermissions(context.Context, string) ([]*repository.Permission, error) {
	return []*repository.Permission{{Resource: "contracts", Action: "read"}}, nil
}

func (fakeRoleAuthorityStore) GetUserDataScopes(context.Context, string) ([]*repository.DataScope, error) {
	return nil, nil
}

func TestPermissionLoaderReplacesTokenRolesWithAuthoritativeAssignments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("user_id", "user-1")
		c.Set("roles", []string{"readonly"})
		c.Next()
	})
	router.Use(LoadUserPermissions(fakeRoleAuthorityStore{}))
	router.GET("/me", func(c *gin.Context) {
		roles, _ := c.Get("roles")
		if !reflect.DeepEqual(roles, []string{"editor", "reviewer"}) {
			c.Status(http.StatusConflict)
			return
		}
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/me", nil))
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected authoritative roles in request context, got status %d", recorder.Code)
	}
}
