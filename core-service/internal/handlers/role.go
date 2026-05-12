package handlers

import (
	"net/http"
	
	"github.com/gin-gonic/gin"
	"github.com/ifrs16/core-service/internal/repository"
)

func ListRoles(roleRepo *repository.RoleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		
		// For now, return static roles. In production, query from DB
		roles := []gin.H{
			{"id": "11111111-1111-1111-1111-111111111111", "name": "System Admin", "description": "系统管理员"},
			{"id": "22222222-2222-2222-2222-222222222222", "name": "Finance Editor", "description": "财务录入员"},
			{"id": "33333333-3333-3333-3333-333333333333", "name": "Finance Reviewer", "description": "财务复核员"},
			{"id": "44444444-4444-4444-4444-444444444444", "name": "Finance Approver", "description": "财务审批员"},
			{"id": "55555555-5555-5555-5555-555555555555", "name": "Auditor Readonly", "description": "审计只读"},
			{"id": "66666666-6666-6666-6666-666666666666", "name": "Business Readonly", "description": "业务只读"},
		}
		
		c.JSON(http.StatusOK, gin.H{"data": roles})
	}
}

func GetMyPermissions(roleRepo *repository.RoleRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		userIDStr, _ := userID.(string)
		
		perms, err := roleRepo.GetUserPermissions(c.Request.Context(), userIDStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get permissions"})
			return
		}
		
		scopes, err := roleRepo.GetUserDataScopes(c.Request.Context(), userIDStr)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get data scopes"})
			return
		}
		
		c.JSON(http.StatusOK, gin.H{
			"permissions": perms,
			"data_scopes": scopes,
		})
	}
}
