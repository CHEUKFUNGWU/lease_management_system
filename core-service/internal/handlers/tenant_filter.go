package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/middleware"
)

// tenantEntity returns the caller's legal-entity filter for the repository
// methods that take an access.EntityFilter. The RequireTenant guard in the
// middleware chain guarantees the scope exists and is either global or carries
// a legal entity, so the false branch is a middleware regression and handlers
// answer it with 403 rather than falling back to an unfiltered query.
func tenantEntity(c *gin.Context) (access.EntityFilter, bool) {
	return middleware.EntityFilterFromRequest(c)
}
