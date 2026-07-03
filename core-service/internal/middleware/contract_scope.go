package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/access"
)

type ContractScopeStore interface {
	GetContractAttributes(ctx context.Context, contractID string) (access.ContractAttributes, bool, error)
}

func RequireContractScope(store ContractScopeStore, parameterName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		scope, ok := GetAccessScope(c)
		if !ok {
			c.JSON(http.StatusForbidden, gin.H{"error": "access scope is unavailable"})
			c.Abort()
			return
		}
		attributes, found, err := store.GetContractAttributes(c.Request.Context(), c.Param(parameterName))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify contract access"})
			c.Abort()
			return
		}
		if !found || !scope.AllowsContract(attributes) {
			c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
			c.Abort()
			return
		}
		c.Next()
	}
}
