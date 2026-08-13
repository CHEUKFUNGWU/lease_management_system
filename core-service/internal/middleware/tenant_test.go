package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/repository"
)

// roleRepoFunc lets tests fix the role/permission/data-scope answers per case
// without duplicating the LoadUserPermissions store shape.
type roleRepoFunc struct {
	roleCodes   []string
	permissions []*repository.Permission
	dataScopes  []*repository.DataScope
}

func (s roleRepoFunc) GetUserRoleCodes(context.Context, string) ([]string, error) {
	return s.roleCodes, nil
}

func (s roleRepoFunc) GetUserPermissions(context.Context, string) ([]*repository.Permission, error) {
	return s.permissions, nil
}

func (s roleRepoFunc) GetUserDataScopes(context.Context, string) ([]*repository.DataScope, error) {
	return s.dataScopes, nil
}

// buildTenantGuardRouter assembles the exact authenticated chain used by
// cmd/api/main.go (JWTAuth → LoadUserPermissions → DataScopeMiddleware →
// TenantMiddleware → RequireTenant) and registers one guarded handler per
// domain route.
func buildTenantGuardRouter(t *testing.T, roleRepo roleRepoFunc, token string) (*gin.Engine, *bool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	entered := false
	router := gin.New()
	router.Use(JWTAuth("test-secret"))
	router.Use(LoadUserPermissions(roleRepo))
	router.Use(DataScopeMiddleware())
	router.Use(TenantMiddleware())
	router.Use(RequireTenant())
	handler := func(c *gin.Context) {
		entered = true
		c.Status(http.StatusOK)
	}
	// One FP&A, one retail, one contract route: the guard must be global to
	// the chain, not a per-route patch.
	router.GET("/api/v1/performance/decision-memos", handler)
	router.GET("/api/v1/retail/operating-pulse", handler)
	router.GET("/api/v1/contracts", handler)
	return router, &entered
}

func requestGuarded(t *testing.T, router *gin.Engine, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	router.ServeHTTP(recorder, request)
	return recorder
}

func mustToken(t *testing.T, userID, legalEntityID string, roles []string) string {
	t.Helper()
	token, err := GenerateTokenWithRoles(userID, "user", "user", roles, legalEntityID, "test-secret")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	return token
}

// T1: a non-global user with an empty legal entity must get 403 on every
// protected route, and the handler must never run.
func TestRequireTenantRejectsNonGlobalUserWithoutLegalEntityAcrossDomains(t *testing.T) {
	token := mustToken(t, "user-1", "", []string{"editor"})
	roleRepo := roleRepoFunc{
		roleCodes:   []string{"editor"},
		permissions: []*repository.Permission{{Resource: "contracts", Action: "read"}},
	}
	router, entered := buildTenantGuardRouter(t, roleRepo, token)

	for _, path := range []string{
		"/api/v1/performance/decision-memos",
		"/api/v1/retail/operating-pulse",
		"/api/v1/contracts",
	} {
		recorder := requestGuarded(t, router, path, token)
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("%s: expected 403 for non-global user without legal entity, got %d", path, recorder.Code)
		}
	}
	if *entered {
		t.Fatal("guarded handler ran for a non-global user without legal entity")
	}
}

// T2: a global (admin) user without a legal entity must pass through exactly
// as before the guard.
func TestRequireTenantAllowsGlobalUserWithoutLegalEntity(t *testing.T) {
	token := mustToken(t, "admin-1", "", []string{"admin"})
	roleRepo := roleRepoFunc{
		roleCodes:   []string{"admin"},
		permissions: []*repository.Permission{{Resource: "*", Action: "*"}},
	}
	router, entered := buildTenantGuardRouter(t, roleRepo, token)

	recorder := requestGuarded(t, router, "/api/v1/contracts", token)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected global user to pass, got %d", recorder.Code)
	}
	if !*entered {
		t.Fatal("guarded handler did not run for global user")
	}
}

// T3: an ordinary user with a legal entity must pass through exactly as
// before the guard.
func TestRequireTenantAllowsUserWithLegalEntity(t *testing.T) {
	token := mustToken(t, "user-1", "11111111-1111-1111-1111-111111111111", []string{"editor"})
	roleRepo := roleRepoFunc{
		roleCodes:   []string{"editor"},
		permissions: []*repository.Permission{{Resource: "contracts", Action: "read"}},
	}
	router, entered := buildTenantGuardRouter(t, roleRepo, token)

	recorder := requestGuarded(t, router, "/api/v1/contracts", token)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected scoped user to pass, got %d", recorder.Code)
	}
	if !*entered {
		t.Fatal("guarded handler did not run for scoped user")
	}
}

// T4: a whitespace-only legal entity is the same as missing; the guard must
// trim before judging.
func TestRequireTenantRejectsWhitespaceLegalEntity(t *testing.T) {
	token := mustToken(t, "user-1", "   ", []string{"editor"})
	roleRepo := roleRepoFunc{
		roleCodes:   []string{"editor"},
		permissions: []*repository.Permission{{Resource: "contracts", Action: "read"}},
	}
	router, entered := buildTenantGuardRouter(t, roleRepo, token)

	recorder := requestGuarded(t, router, "/api/v1/contracts", token)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for whitespace legal entity, got %d", recorder.Code)
	}
	if *entered {
		t.Fatal("guarded handler ran for whitespace legal entity")
	}
}

// The guard fails closed when access_scope is absent from the context, even
// though every authenticated chain writes it.
func TestRequireTenantRejectsMissingAccessScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequireTenant())
	router.GET("/check", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/check", nil)
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403 when access_scope is missing, got %d", recorder.Code)
	}
}
