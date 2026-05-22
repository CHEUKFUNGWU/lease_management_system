package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ifrs16/core-service/internal/repository"
)

type SettingsHandler struct {
	systemSettingRepo *repository.SystemSettingRepository
}

func NewSettingsHandler(systemSettingRepo *repository.SystemSettingRepository) *SettingsHandler {
	return &SettingsHandler{systemSettingRepo: systemSettingRepo}
}

type GlobalSettingsResponse struct {
	GlobalDiscountRate float64 `json:"global_discount_rate"`
}

type UpdateGlobalSettingsRequest struct {
	GlobalDiscountRate float64 `json:"global_discount_rate"`
}

// GET /api/v1/settings/global
func (h *SettingsHandler) GetGlobal(c *gin.Context) {
	rate := 0.05
	if h.systemSettingRepo != nil {
		rate = h.systemSettingRepo.GetFloat64(c.Request.Context(), "global_discount_rate", 0.05)
	}
	c.JSON(http.StatusOK, GlobalSettingsResponse{
		GlobalDiscountRate: rate,
	})
}

// PUT /api/v1/settings/global
func (h *SettingsHandler) UpdateGlobal(c *gin.Context) {
	var req UpdateGlobalSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	// Normalize: if > 1, treat as percentage (e.g. 5 => 0.05)
	if req.GlobalDiscountRate > 1 {
		req.GlobalDiscountRate = req.GlobalDiscountRate / 100.0
	}

	if req.GlobalDiscountRate <= 0 || req.GlobalDiscountRate > 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "global_discount_rate must be > 0 and <= 1 (or > 0 and <= 100 as percentage)"})
		return
	}

	valueStr := fmt.Sprintf("%.6f", req.GlobalDiscountRate)

	var updatedBy *string
	if uid, exists := c.Get("user_id"); exists {
		if uidStr, ok := uid.(string); ok {
			updatedBy = &uidStr
		}
	}

	setting := &repository.SystemSetting{
		SettingKey:   "global_discount_rate",
		SettingValue: valueStr,
		UpdatedBy:    updatedBy,
	}

	if err := h.systemSettingRepo.Upsert(c.Request.Context(), setting); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save setting: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, GlobalSettingsResponse{
		GlobalDiscountRate: req.GlobalDiscountRate,
	})
}
