package middleware

import (
	"net/http"
	"strings"
	
	"github.com/gin-gonic/gin"
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
			"error": "insufficient permissions",
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
		dataScopes, exists := c.Get("data_scopes")
		if !exists {
			c.Next()
			return
		}
		
		scopes, ok := dataScopes.(map[string][]string)
		if !ok || len(scopes) == 0 {
			c.Next()
			return
		}
		
		// Store scopes in context for downstream handlers to use
		c.Set("scope_legal_entity", scopes["legal_entity"])
		c.Set("scope_store", scopes["store"])
		c.Set("scope_region", scopes["region"])
		c.Set("scope_brand", scopes["brand"])
		
		c.Next()
	}
}

// LoadUserPermissions loads user's permissions into context
func LoadUserPermissions(roleRepo interface {
	GetUserPermissions(ctx interface{}, userID string) (interface{}, error)
	GetUserDataScopes(ctx interface{}, userID string) (interface{}, error)
}) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := c.Get("user_id")
		if !exists {
			c.Next()
			return
		}
		
		_, ok := userID.(string)
		if !ok {
			c.Next()
			return
		}
		
		// TODO: Load permissions from database and set in context
		// For now, skip to avoid circular dependency
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
