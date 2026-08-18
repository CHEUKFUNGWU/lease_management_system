package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/middleware"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/inventorymetrics"
)

type InventoryMetricsHandler struct {
	repo    *repository.InventoryRepository
	kpiRepo *repository.RetailKPIRepository
}

func NewInventoryMetricsHandler(repo *repository.InventoryRepository, kpiRepo *repository.RetailKPIRepository) *InventoryMetricsHandler {
	return &InventoryMetricsHandler{repo: repo, kpiRepo: kpiRepo}
}

func (h *InventoryMetricsHandler) GetInventorySummary(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	storeID := c.Query("store_id")
	fromDate := c.Query("from_date")
	toDate := c.Query("to_date")
	currency := c.DefaultQuery("currency", "CNY")

	annualRateStr := c.DefaultQuery("annual_rate", "0.08")
	annualRate, _ := strconv.ParseFloat(annualRateStr, 64)

	facts, err := h.repo.ListInventoryFacts(c.Request.Context(), tenantID, storeID, fromDate, toDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var latestStockCost, latestStockQty, latestTransitCost, latestTransitQty float64
	var days int = 30
	if fromDate != "" && toDate != "" {
		st, _ := time.Parse("2006-01-02", fromDate)
		et, _ := time.Parse("2006-01-02", toDate)
		if !et.Before(st) {
			days = int(et.Sub(st).Hours()/24) + 1
		}
	}

	if len(facts) > 0 {
		// Take latest fact as ending stock
		latest := facts[0]
		latestStockCost = latest.StockCost
		latestStockQty = latest.StockQty
		latestTransitCost = latest.InTransitCost
		latestTransitQty = latest.InTransitQty
	}

	// Fetch COGS / Revenue from store day facts to compute true COGS
	var cogs float64 = 0
	if h.kpiRepo != nil && storeID != "" {
		kpiFacts, _ := h.kpiRepo.QueryFacts(c.Request.Context(), tenantID, storeID, "production", "", fromDate, toDate, []string{storeID})
		if kpiFacts != nil {
			for _, f := range kpiFacts.Facts {
				if f.Revenue != nil && f.GrossProfit != nil {
					cogs += (*f.Revenue - *f.GrossProfit)
				}
			}
		}
	}
	if cogs <= 0 && latestStockCost > 0 {
		cogs = latestStockCost * 1.5 // fallback baseline estimate
	}

	summary := inventorymetrics.SummarizeInventory(
		latestStockCost,
		latestStockQty,
		latestTransitCost,
		latestTransitQty,
		cogs,
		days,
		annualRate,
		currency,
	)

	c.JSON(http.StatusOK, summary)
}

type upsertInventoryFactReq struct {
	StoreID         string   `json:"store_id" binding:"required"`
	BusinessDate    string   `json:"business_date" binding:"required"`
	Currency        string   `json:"currency"`
	CategoryCode    *string  `json:"category_code"`
	SKUCode         *string  `json:"sku_code"`
	StockQty        float64  `json:"stock_qty"`
	StockCost       float64  `json:"stock_cost"`
	InTransitQty    float64  `json:"in_transit_qty"`
	InTransitCost   float64  `json:"in_transit_cost"`
	DaysOfInventory *float64 `json:"days_of_inventory"`
}

func (h *InventoryMetricsHandler) UpsertInventoryFact(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	var req upsertInventoryFactReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Currency == "" {
		req.Currency = "CNY"
	}

	fact := &repository.StoreDayInventoryFact{
		LegalEntityID:   tenantID,
		StoreID:         req.StoreID,
		BusinessDate:    req.BusinessDate,
		Currency:        req.Currency,
		CategoryCode:    req.CategoryCode,
		SKUCode:         req.SKUCode,
		StockQty:        req.StockQty,
		StockCost:       req.StockCost,
		InTransitQty:    req.InTransitQty,
		InTransitCost:   req.InTransitCost,
		DaysOfInventory: req.DaysOfInventory,
	}

	if err := h.repo.UpsertInventoryFact(c.Request.Context(), fact); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, fact)
}
