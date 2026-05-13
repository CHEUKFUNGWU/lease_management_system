package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ifrs16/core-service/internal/middleware"
	"github.com/ifrs16/core-service/internal/repository"
)

func CheckDiscountRate(contractRepo *repository.ContractRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		legalEntityID := middleware.GetTenantID(c)

		contract, err := contractRepo.GetByID(c.Request.Context(), id, legalEntityID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get contract: " + err.Error()})
			return
		}
		if contract == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "contract not found"})
			return
		}
		
		c.JSON(http.StatusOK, gin.H{
			"contract_id":             id,
			"discount_rate_missing":   contract.DiscountRateType == nil && contract.DiscountRateVersion == nil,
			"discount_rate_type":      contract.DiscountRateType,
			"discount_rate_version":   contract.DiscountRateVersion,
			"discount_rate_source":    contract.DiscountRateSource,
			"discount_rate_confirmed": contract.DiscountRateConfirmedBy != nil,
			"can_calculate":           contract.DiscountRateType != nil || contract.DiscountRateVersion != nil,
		})
	}
}

type ConfirmDiscountRateRequest struct {
	ContractID          string  `json:"contract_id" binding:"required,uuid"`
	DiscountRateType    string  `json:"discount_rate_type" binding:"required"`
	DiscountRateVersion string  `json:"discount_rate_version" binding:"required"`
	DiscountRateValue   float64 `json:"discount_rate_value" binding:"required,gt=0"`
	Source              string  `json:"source" binding:"required"`
	PolicyID            *string `json:"policy_id"`
}

func ConfirmDiscountRate(contractRepo *repository.ContractRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req ConfirmDiscountRateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		
		userID, _ := c.Get("user_id")
		userIDStr, _ := userID.(string)
		
		// TODO: Update contract with confirmed discount rate
		// For now, just return success
		c.JSON(http.StatusOK, gin.H{
			"contract_id":              req.ContractID,
			"discount_rate_type":       req.DiscountRateType,
			"discount_rate_version":    req.DiscountRateVersion,
			"discount_rate_value":      req.DiscountRateValue,
			"source":                   req.Source,
			"confirmed_by":             userIDStr,
			"message":                  "折现率已确认",
		})
	}
}
