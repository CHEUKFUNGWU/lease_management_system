package inventorymetrics

import (
	"math"
)

type InventorySummary struct {
	Currency             string   `json:"currency"`
	Days                 int      `json:"days"`
	EndingStockCost      float64  `json:"ending_stock_cost"`
	EndingStockQty       float64  `json:"ending_stock_qty"`
	InTransitCost        float64  `json:"in_transit_cost"`
	InTransitQty         float64  `json:"in_transit_qty"`
	COGS                 float64  `json:"cogs"`
	DOI                  *float64 `json:"doi,omitempty"`           // Days of Inventory: (Ending Stock / COGS) * Days
	TurnoverRate         *float64 `json:"turnover_rate,omitempty"` // Turnover: COGS / Avg Stock
	StockCarryingCost    float64  `json:"stock_carrying_cost"`     // Stock Cost * Annual Rate * (Days/365)
	TransitCarryingCost  float64  `json:"transit_carrying_cost"`   // Transit Cost * Annual Rate * (Days/365)
	TotalCarryingCost    float64  `json:"total_carrying_cost"`
	CarryingCostRate     float64  `json:"carrying_cost_rate"`      // e.g. 0.08 (8% per annum)
}

func CalculateDOI(endingStockCost, cogs float64, days int) *float64 {
	if cogs <= 0.0001 || days <= 0 {
		return nil
	}
	doi := (endingStockCost / cogs) * float64(days)
	val := math.Round(doi*100) / 100
	return &val
}

func CalculateTurnover(cogs, avgStockCost float64) *float64 {
	if avgStockCost <= 0.0001 {
		return nil
	}
	to := cogs / avgStockCost
	val := math.Round(to*100) / 100
	return &val
}

func CalculateCarryingCost(stockCost, transitCost, annualRate float64, days int) (stockCarrying, transitCarrying, totalCarrying float64) {
	if annualRate <= 0 || days <= 0 {
		return 0, 0, 0
	}
	fraction := float64(days) / 365.0
	stockCarrying = math.Round(stockCost*annualRate*fraction*100) / 100
	transitCarrying = math.Round(transitCost*annualRate*fraction*100) / 100
	totalCarrying = math.Round((stockCarrying+transitCarrying)*100) / 100
	return stockCarrying, transitCarrying, totalCarrying
}

func SummarizeInventory(endingStockCost, endingStockQty, inTransitCost, inTransitQty, cogs float64, days int, annualRate float64, currency string) InventorySummary {
	if annualRate <= 0 {
		annualRate = 0.08 // Default 8% annual inventory carrying cost rate
	}
	doi := CalculateDOI(endingStockCost, cogs, days)
	turnover := CalculateTurnover(cogs, endingStockCost)
	sc, tc, totalC := CalculateCarryingCost(endingStockCost, inTransitCost, annualRate, days)

	return InventorySummary{
		Currency:            currency,
		Days:                days,
		EndingStockCost:     math.Round(endingStockCost*100) / 100,
		EndingStockQty:      math.Round(endingStockQty*100) / 100,
		InTransitCost:       math.Round(inTransitCost*100) / 100,
		InTransitQty:        math.Round(inTransitQty*100) / 100,
		COGS:                math.Round(cogs*100) / 100,
		DOI:                 doi,
		TurnoverRate:        turnover,
		StockCarryingCost:   sc,
		TransitCarryingCost: tc,
		TotalCarryingCost:   totalC,
		CarryingCostRate:    annualRate,
	}
}
