package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// TenantMiddleware extracts legal_entity_id from JWT claims and enforces tenant isolation.
// It sets legal_entity_id in the context for downstream handlers to use.
// If the user has no legal_entity_id (e.g., admin with cross-tenant access),
// it still proceeds but downstream handlers should check if legal_entity_id is present.
func TenantMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		legalEntityID, exists := c.Get("legal_entity_id")
		if !exists {
			// No legal_entity_id in token, could be admin or legacy token
			// Set empty string for consistent handling
			c.Set("legal_entity_id", "")
		} else {
			// Ensure it's stored as string
			if id, ok := legalEntityID.(string); ok {
				c.Set("legal_entity_id", id)
			} else {
				c.Set("legal_entity_id", "")
			}
		}
		c.Next()
	}
}

// GetTenantID retrieves the legal_entity_id from the gin context.
// Returns empty string if not set.
func GetTenantID(c *gin.Context) string {
	if id, exists := c.Get("legal_entity_id"); exists {
		if strID, ok := id.(string); ok {
			return strID
		}
	}
	return ""
}

// RequireTenant guards legal-entity isolation at the single choke point of the
// authenticated middleware chain (see cmd/api/main.go). A user with no legal
// entity is legitimate only when they hold global (admin) access; any other
// user without a legal entity would otherwise fall through the
// `($N=” OR legal_entity_id::text=$N)` predicates and read every legal
// entity's data. That state is a configuration error, not a legal one, so it
// is rejected with 403 before any handler runs.
func RequireTenant() gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, ok := GetAccessScope(c)
		if !ok || (!scope.Global && strings.TrimSpace(scope.LegalEntityID) == "") {
			c.JSON(http.StatusForbidden, gin.H{"error": "legal entity scope is required for non-admin users"})
			c.Abort()
			return
		}
		c.Next()
	}
}
