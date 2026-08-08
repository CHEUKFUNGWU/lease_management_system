package handlers

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
	"github.com/lease-management-system/core-service/internal/services/renttosales"
)

type StoreMetricsHandler struct {
	repo              *repository.StoreMetricsRepository
	auditLogger       *audit.Logger
	systemSettingRepo *repository.SystemSettingRepository
}

func NewStoreMetricsHandler(repo *repository.StoreMetricsRepository, auditLogger *audit.Logger, settingRepos ...*repository.SystemSettingRepository) *StoreMetricsHandler {
	var settings *repository.SystemSettingRepository
	if len(settingRepos) > 0 {
		settings = settingRepos[0]
	}
	return &StoreMetricsHandler{repo: repo, auditLogger: auditLogger, systemSettingRepo: settings}
}

var periodPattern = regexp.MustCompile(`^\d{4}-\d{2}$`)

type StoreMetricItem struct {
	StoreID     string   `json:"store_id" binding:"required,uuid"`
	Period      string   `json:"period" binding:"required"`
	PeriodBasis string   `json:"period_basis"`
	Revenue     float64  `json:"revenue"`
	GrossProfit *float64 `json:"gross_profit"`
	Currency    string   `json:"currency"`
	Version     int      `json:"version"`
	Source      string   `json:"source"`
	Note        *string  `json:"note"`
}

type UpsertStoreMetricsRequest struct {
	Items []StoreMetricItem `json:"items" binding:"required"`
}

// Upsert records store revenue. It is idempotent on store, period and version,
// which is what lets a scheduled push from the customer's BI retry without
// double-counting. A restatement arrives as a new version and the earlier one
// is kept, so a report can still say which vintage it was based on.
//
// POST /store-metrics
func (h *StoreMetricsHandler) Upsert(c *gin.Context) {
	var req UpsertStoreMetricsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if len(req.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请至少提供一条营收数据"})
		return
	}

	userID, _ := c.Get("user_id")
	userIDStr, _ := userID.(string)
	ctx := c.Request.Context()

	saved := make([]*repository.StoreMetric, 0, len(req.Items))
	failures := make([]gin.H, 0)

	for index, item := range req.Items {
		if !periodPattern.MatchString(item.Period) {
			failures = append(failures, gin.H{"index": index, "error": "期间格式应为 YYYY-MM"})
			continue
		}
		if item.Revenue < 0 {
			failures = append(failures, gin.H{"index": index, "error": "营收不能为负数"})
			continue
		}
		if strings.TrimSpace(item.PeriodBasis) == "" || strings.TrimSpace(item.Currency) == "" || strings.TrimSpace(item.Source) == "" {
			failures = append(failures, gin.H{"index": index, "error": "期间口径、币种和来源均为必填项"})
			continue
		}

		metric := &repository.StoreMetric{
			StoreID: item.StoreID, Period: item.Period, PeriodBasis: strings.TrimSpace(item.PeriodBasis),
			Revenue: item.Revenue, GrossProfit: item.GrossProfit, Currency: strings.TrimSpace(item.Currency),
			Version: item.Version, Source: strings.TrimSpace(item.Source), Note: item.Note,
		}
		if userIDStr != "" {
			metric.CreatedBy = &userIDStr
		}

		result, err := h.repo.Upsert(ctx, metric)
		if err != nil {
			failures = append(failures, gin.H{"index": index, "error": err.Error()})
			continue
		}
		saved = append(saved, result)
		if h.auditLogger != nil {
			h.auditLogger.Log(ctx, "store_metrics", result.ID, "upsert", nil, result, userIDStr, c)
		}
	}

	status := http.StatusOK
	if len(saved) == 0 {
		status = http.StatusUnprocessableEntity
	}
	c.JSON(status, gin.H{
		"saved_count":  len(saved),
		"failed_count": len(failures),
		"failures":     failures,
		"data":         saved,
	})
}

// List returns the reported figures.
// GET /store-metrics
func (h *StoreMetricsHandler) List(c *gin.Context) {
	metrics, err := h.repo.List(c.Request.Context(), middleware.GetTenantID(c), c.Query("period"), c.Query("store_id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": metrics, "total": len(metrics)})
}

// RentToSales reports the ratio for a period.
// GET /reports/rent-to-sales?period=YYYY-MM
func (h *StoreMetricsHandler) RentToSales(c *gin.Context) {
	period := c.Query("period")
	if !periodPattern.MatchString(period) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请提供期间，格式 YYYY-MM"})
		return
	}

	rows, err := h.repo.RentToSales(c.Request.Context(), middleware.GetTenantID(c), period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	stores := make([]renttosales.StoreInput, 0, len(rows))
	for _, row := range rows {
		stores = append(stores, renttosales.StoreInput{
			StoreID: row.StoreID, StoreCode: row.StoreCode, StoreName: row.StoreName,
			Brand: row.Brand, Region: row.Region,
			CashRent: row.CashRent, RentCurrency: row.RentCurrency,
			Revenue: row.Revenue, RevenueCurrency: row.RevenueCurrency,
			RevenueVersion: row.RevenueVersion, RevenueSource: row.RevenueSource,
			AreaSqm: row.AreaSqm,
		})
	}

	healthy, _ := strconv.ParseFloat(c.Query("healthy_ceiling"), 64)
	warning, _ := strconv.ParseFloat(c.Query("warning_ceiling"), 64)
	if h.systemSettingRepo != nil {
		if healthy <= 0 {
			healthy = h.systemSettingRepo.GetFloat64(c.Request.Context(), "rent_to_sales_healthy_ceiling", 0)
		}
		if warning <= 0 {
			warning = h.systemSettingRepo.GetFloat64(c.Request.Context(), "rent_to_sales_warning_ceiling", 0)
		}
	}

	result, err := renttosales.Calculate(renttosales.Input{
		Period: period, HealthyCeiling: healthy, WarningCeiling: warning, Stores: stores,
	})
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
