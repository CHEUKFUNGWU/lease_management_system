package middleware

import (
	"context"
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

		legalEntityID, _ := c.Get("legal_entity_id")
		legalEntityIDStr, _ := legalEntityID.(string)
		permissions, _ := c.Get("permissions")
		permissionStrings, _ := permissions.([]string)
		scope := access.Scope{
			Global:        HasPermission(permissionStrings, "*", "*"),
			LegalEntityID: legalEntityIDStr,
			StoreIDs:      append([]string(nil), scopes["store"]...),
			Regions:       append([]string(nil), scopes["region"]...),
			Brands:        append([]string(nil), scopes["brand"]...),
		}
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

// RequireLegalEntityWideScope prevents a store/region/brand-scoped actor from
// applying an operation, such as a period lock, to the entire legal entity.
func RequireLegalEntityWideScope() gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, ok := GetAccessScope(c)
		if !ok || (!scope.Global && (scope.LegalEntityID == "" || len(scope.StoreIDs) > 0 || len(scope.Regions) > 0 || len(scope.Brands) > 0)) {
			c.JSON(http.StatusForbidden, gin.H{"error": "operation requires legal-entity-wide access"})
			c.Abort()
			return
		}
		c.Next()
	}
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
		roles, err := roleRepo.GetUserRoleCodes(c.Request.Context(), userIDStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load role assignments"})
			c.Abort()
			return
		}
		c.Set("roles", roles)

		perms, err := roleRepo.GetUserPermissions(c.Request.Context(), userIDStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load permissions"})
			c.Abort()
			return
		}

		normalizedPerms := make([]string, 0, len(perms))
		for _, perm := range perms {
			normalizedPerms = append(normalizedPerms, normalizePermission(perm.Resource, perm.Action))
		}
		c.Set("permissions", normalizedPerms)

		scopes, err := roleRepo.GetUserDataScopes(c.Request.Context(), userIDStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load data scopes"})
			c.Abort()
			return
		}

		scopeMap := map[string][]string{}
		for _, scope := range scopes {
			dimension := strings.ToLower(scope.Dimension)
			scopeMap[dimension] = append(scopeMap[dimension], scope.TargetID)
		}
		c.Set("data_scopes", scopeMap)

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
