package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
)

// RBACMiddleware checks if user has required permission
type RBACConfig struct {
	Resource string
	Action   string
}

func RBACMiddleware(resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		permissions, exists := c.Get("permissions")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "no permissions found"})
			c.Abort()
			return
		}

		perms, ok := permissions.([]string)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid permissions format"})
			c.Abort()
			return
		}

		// Check for wildcard admin permission
		for _, p := range perms {
			if p == "*:*" || p == resource+":*" || p == resource+":"+action {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"error":    "insufficient permissions",
			"required": resource + ":" + action,
		})
		c.Abort()
	}
}

// HasPermission checks if permission list contains required permission
func HasPermission(permissions []string, resource, action string) bool {
	for _, p := range permissions {
		if p == "*:*" || p == resource+":*" || p == resource+":"+action {
			return true
		}
	}
	return false
}

// BuildAccessScope is the single Scope assembly point for the whole system:
// the JWT middleware chain (DataScopeMiddleware) and non-HTTP callers
// (channel identity resolution, internal/gateway) both call this one function
// instead of composing access.Scope literals themselves (ADR-0026 §3: the
// channel path delegates to the same resolver as JWT — the same function, not
// a copy of its logic).
func BuildAccessScope(permissions []string, dataScopes map[string][]string, legalEntityID string) access.Scope {
	return access.Scope{
		Global:          HasPermission(permissions, "*", "*"),
		LegalEntityID:   strings.TrimSpace(legalEntityID),
		StoreIDs:        append([]string(nil), dataScopes["store"]...),
		Regions:         append([]string(nil), dataScopes["region"]...),
		Brands:          append([]string(nil), dataScopes["brand"]...),
		Plants:          append([]string(nil), dataScopes["plant"]...),
		ProductionLines: append([]string(nil), dataScopes["production_line"]...),
		EquipmentIDs:    append([]string(nil), dataScopes["equipment"]...),
	}
}

// DataScopeMiddleware filters contracts by user's data scope
func DataScopeMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		scopes := map[string][]string{}
		if dataScopes, exists := c.Get("data_scopes"); exists {
			if loaded, ok := dataScopes.(map[string][]string); ok {
				scopes = loaded
			}
		}

		// Store scopes in context for downstream handlers to use
		c.Set("scope_legal_entity", scopes["legal_entity"])
		c.Set("scope_store", scopes["store"])
		c.Set("scope_region", scopes["region"])
		c.Set("scope_brand", scopes["brand"])
		c.Set("scope_plant", scopes["plant"])
		c.Set("scope_production_line", scopes["production_line"])
		c.Set("scope_equipment", scopes["equipment"])

		legalEntityID, _ := c.Get("legal_entity_id")
		legalEntityIDStr, _ := legalEntityID.(string)
		permissions, _ := c.Get("permissions")
		permissionStrings, _ := permissions.([]string)
		scope := BuildAccessScope(permissionStrings, scopes, legalEntityIDStr)
		c.Set("access_scope", scope)
		c.Request = c.Request.WithContext(access.WithScope(c.Request.Context(), scope))

		c.Next()
	}
}

func GetAccessScope(c *gin.Context) (access.Scope, bool) {
	value, exists := c.Get("access_scope")
	if !exists {
		return access.Scope{}, false
	}
	scope, ok := value.(access.Scope)
	return scope, ok
}

// EntityFilterFromRequest builds the caller's legal-entity filter from the
// request scope. The RequireTenant guard in the same chain guarantees the
// scope exists and is either global or carries a legal entity, so failure
// here means a middleware regression and should be answered with 403.
func EntityFilterFromRequest(c *gin.Context) (access.EntityFilter, bool) {
	scope, ok := GetAccessScope(c)
	if !ok {
		return access.EntityFilter{}, false
	}
	filter, err := access.FromScope(scope)
	if err != nil {
		return access.EntityFilter{}, false
	}
	return filter, true
}

// RequireLegalEntityWideScope prevents a store/region/brand-scoped actor from
// applying an operation, such as a period lock, to the entire legal entity.
func RequireLegalEntityWideScope() gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, ok := GetAccessScope(c)
		if !ok || (!scope.Global && (scope.LegalEntityID == "" || len(scope.StoreIDs) > 0 || len(scope.Regions) > 0 || len(scope.Brands) > 0 || len(scope.Plants) > 0 || len(scope.ProductionLines) > 0 || len(scope.EquipmentIDs) > 0)) {
			c.JSON(http.StatusForbidden, gin.H{"error": "operation requires legal-entity-wide access"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// UserAccessRepo is the DB face of the JWT permission chain. The channel
// identity resolver (internal/gateway) loads through the same interface, so
// both paths read the same tables with the same queries.
type UserAccessRepo interface {
	GetUserRoleCodes(ctx context.Context, userID string) ([]string, error)
	GetUserPermissions(ctx context.Context, userID string) ([]*repository.Permission, error)
	GetUserDataScopes(ctx context.Context, userID string) ([]*repository.DataScope, error)
}

// LoadUserAccess loads role codes, normalized permissions and the data-scope
// map exactly as the JWT middleware chain does. It is the shared loading half
// of the scope resolver; BuildAccessScope is the shared assembly half.
func LoadUserAccess(ctx context.Context, repo UserAccessRepo, userID string) (roles []string, permissions []string, dataScopes map[string][]string, err error) {
	roles, err = repo.GetUserRoleCodes(ctx, userID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load role assignments: %w", err)
	}

	perms, err := repo.GetUserPermissions(ctx, userID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load permissions: %w", err)
	}
	normalizedPerms := make([]string, 0, len(perms))
	for _, perm := range perms {
		normalizedPerms = append(normalizedPerms, normalizePermission(perm.Resource, perm.Action))
	}

	scopes, err := repo.GetUserDataScopes(ctx, userID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load data scopes: %w", err)
	}
	scopeMap := map[string][]string{}
	for _, scope := range scopes {
		dimension := strings.ToLower(scope.Dimension)
		scopeMap[dimension] = append(scopeMap[dimension], scope.TargetID)
	}
	return roles, normalizedPerms, scopeMap, nil
}

// LoadUserPermissions loads user's permissions into context
func LoadUserPermissions(roleRepo interface {
	GetUserRoleCodes(ctx context.Context, userID string) ([]string, error)
	GetUserPermissions(ctx context.Context, userID string) ([]*repository.Permission, error)
	GetUserDataScopes(ctx context.Context, userID string) ([]*repository.DataScope, error)
}) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}

		userIDStr, ok := userID.(string)
		if !ok {
			c.Next()
			return
		}

		roles, permissions, dataScopes, err := LoadUserAccess(c.Request.Context(), roleRepo, userIDStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			c.Abort()
			return
		}
		c.Set("roles", roles)
		c.Set("permissions", permissions)
		c.Set("data_scopes", dataScopes)

		c.Next()
	}
}

// ParsePermissions converts permission list to string format
func ParsePermissions(perms []interface{}) []string {
	var result []string
	for _, p := range perms {
		if s, ok := p.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func normalizePermission(resource, action string) string {
	return strings.ToLower(resource) + ":" + strings.ToLower(action)
}
