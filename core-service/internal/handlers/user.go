package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func GetCurrentUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		username, _ := c.Get("username")
		role, _ := c.Get("role")
		roles, _ := c.Get("roles")
		legalEntityID, _ := c.Get("legal_entity_id")

		c.JSON(http.StatusOK, gin.H{
			"user_id":         userID,
			"username":        username,
			"role":            role,
			"roles":           roles,
			"legal_entity_id": legalEntityID,
		})
	}
}
