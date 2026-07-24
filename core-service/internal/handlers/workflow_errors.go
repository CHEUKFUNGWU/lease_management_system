package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/repository"
)

func writeWorkflowMutationError(c *gin.Context, action string, err error) {
	if errors.Is(err, repository.ErrInvalidWorkflowTransition) {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "workflow state changed or action is not allowed",
			"action":  action,
			"details": err.Error(),
		})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to " + action + ": " + err.Error()})
}
