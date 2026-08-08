package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/repository"
)

type SettingsHandler struct {
	systemSettingRepo *repository.SystemSettingRepository
}

func NewSettingsHandler(systemSettingRepo *repository.SystemSettingRepository) *SettingsHandler {
	return &SettingsHandler{systemSettingRepo: systemSettingRepo}
}

type GlobalSettingsResponse struct {
	GlobalDiscountRate                 float64 `json:"global_discount_rate"`
	RentToSalesHealthyCeiling          float64 `json:"rent_to_sales_healthy_ceiling"`
	RentToSalesWarningCeiling          float64 `json:"rent_to_sales_warning_ceiling"`
	BudgetVarianceMaterialityThreshold float64 `json:"budget_variance_materiality_threshold"`
	BudgetTieOutTolerance              float64 `json:"budget_tie_out_tolerance"`
	JournalEntryMaterialityThreshold   float64 `json:"journal_entry_materiality_threshold"`
}

type UpdateGlobalSettingsRequest struct {
	GlobalDiscountRate                 *float64 `json:"global_discount_rate"`
	RentToSalesHealthyCeiling          *float64 `json:"rent_to_sales_healthy_ceiling"`
	RentToSalesWarningCeiling          *float64 `json:"rent_to_sales_warning_ceiling"`
	BudgetVarianceMaterialityThreshold *float64 `json:"budget_variance_materiality_threshold"`
	BudgetTieOutTolerance              *float64 `json:"budget_tie_out_tolerance"`
	JournalEntryMaterialityThreshold   *float64 `json:"journal_entry_materiality_threshold"`
}

// GET /api/v1/settings/global
func (h *SettingsHandler) GetGlobal(c *gin.Context) {
	rate := 0.0
	healthy := 0.0
	warning := 0.0
	materiality := 0.0
	tieOut := 0.0
	journalMateriality := 0.0
	if h.systemSettingRepo != nil {
		rate = h.systemSettingRepo.GetFloat64(c.Request.Context(), "global_discount_rate", 0)
		healthy = h.systemSettingRepo.GetFloat64(c.Request.Context(), "rent_to_sales_healthy_ceiling", 0)
		warning = h.systemSettingRepo.GetFloat64(c.Request.Context(), "rent_to_sales_warning_ceiling", 0)
		materiality = h.systemSettingRepo.GetFloat64(c.Request.Context(), "budget_variance_materiality_threshold", 0)
		tieOut = h.systemSettingRepo.GetFloat64(c.Request.Context(), "budget_tie_out_tolerance", 0)
		journalMateriality = h.systemSettingRepo.GetFloat64(c.Request.Context(), "journal_entry_materiality_threshold", 0)
	}
	c.JSON(http.StatusOK, GlobalSettingsResponse{
		GlobalDiscountRate:                 rate,
		RentToSalesHealthyCeiling:          healthy,
		RentToSalesWarningCeiling:          warning,
		BudgetVarianceMaterialityThreshold: materiality,
		BudgetTieOutTolerance:              tieOut,
		JournalEntryMaterialityThreshold:   journalMateriality,
	})
}

// PUT /api/v1/settings/global
func (h *SettingsHandler) UpdateGlobal(c *gin.Context) {
	var req UpdateGlobalSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	var updatedBy *string
	if uid, exists := c.Get("user_id"); exists {
		if uidStr, ok := uid.(string); ok {
			updatedBy = &uidStr
		}
	}

	updates := []struct {
		key   string
		value *float64
		min   float64
		max   float64
	}{
		{key: "global_discount_rate", value: req.GlobalDiscountRate, min: 0, max: 1},
		{key: "rent_to_sales_healthy_ceiling", value: req.RentToSalesHealthyCeiling, min: 0, max: 100},
		{key: "rent_to_sales_warning_ceiling", value: req.RentToSalesWarningCeiling, min: 0, max: 100},
		{key: "budget_variance_materiality_threshold", value: req.BudgetVarianceMaterialityThreshold, min: 0, max: 1e15},
		{key: "budget_tie_out_tolerance", value: req.BudgetTieOutTolerance, min: 0, max: 1e15},
		{key: "journal_entry_materiality_threshold", value: req.JournalEntryMaterialityThreshold, min: 0, max: 1e15},
	}
	for _, update := range updates {
		if update.value == nil {
			continue
		}
		value := *update.value
		if update.key == "global_discount_rate" && value > 1 {
			value /= 100
		}
		if value <= update.min || value > update.max {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s must be greater than zero and within its configured range", update.key)})
			return
		}
		if err := h.systemSettingRepo.Upsert(c.Request.Context(), &repository.SystemSetting{
			SettingKey: update.key, SettingValue: fmt.Sprintf("%.6f", value), UpdatedBy: updatedBy,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save setting: " + err.Error()})
			return
		}
	}

	h.GetGlobal(c)
}
