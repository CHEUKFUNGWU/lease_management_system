package operating

import (
	"fmt"
	"math"

	"github.com/lease-management-system/core-service/internal/repository"
)

// FourWall is the governed store contribution view. Nil inputs stay nil in
// the response: missing operating facts are not silently converted to zero.
type FourWall struct {
	StoreID               string   `json:"store_id"`
	StoreCode             string   `json:"store_code"`
	StoreName             string   `json:"store_name"`
	Brand                 string   `json:"brand"`
	Region                string   `json:"region"`
	Period                string   `json:"period"`
	Currency              string   `json:"currency"`
	Revenue               float64  `json:"revenue"`
	GrossProfit           *float64 `json:"gross_profit,omitempty"`
	LaborCost             *float64 `json:"labor_cost,omitempty"`
	FixedRent             *float64 `json:"fixed_rent,omitempty"`
	VariableRent          *float64 `json:"variable_rent,omitempty"`
	NonLeaseCost          *float64 `json:"non_lease_cost,omitempty"`
	OtherControllableCost *float64 `json:"other_controllable_cost,omitempty"`
	FourWallEBITDA        *float64 `json:"four_wall_ebitda,omitempty"`
	ContributionMargin    *float64 `json:"contribution_margin,omitempty"`
	RentToSales           *float64 `json:"rent_to_sales,omitempty"`
	OccupancyCostRatio    *float64 `json:"occupancy_cost_ratio,omitempty"`
	SalesPerSqm           *float64 `json:"sales_per_sqm,omitempty"`
	BreakEvenSales        *float64 `json:"break_even_sales,omitempty"`
	DataReady             bool     `json:"data_ready"`
	DataGaps              []string `json:"data_gaps,omitempty"`
	SourceSystem          string   `json:"source_system"`
	Version               int      `json:"version"`
	ReconciliationStatus  string   `json:"reconciliation_status"`
}

func CalculateFourWall(f repository.StoreOperatingFact) FourWall {
	result := FourWall{
		StoreID: f.StoreID, StoreCode: f.StoreCode, StoreName: f.StoreName, Brand: f.Brand, Region: f.Region,
		Period: f.Period, Currency: f.Currency, Revenue: round2(f.Revenue), GrossProfit: f.GrossProfit,
		LaborCost: f.LaborCost, FixedRent: f.FixedRent, VariableRent: f.VariableRent,
		NonLeaseCost: f.NonLeaseCost, OtherControllableCost: f.OtherControllableCost,
		DataReady: f.MappingStatus == "mapped" && f.ReconciliationStatus == "matched",
		DataGaps:  []string{}, SourceSystem: f.SourceSystem, Version: f.Version, ReconciliationStatus: f.ReconciliationStatus,
	}
	if f.GrossProfit == nil {
		result.DataGaps = append(result.DataGaps, "gross_profit")
	}
	if f.LaborCost == nil {
		result.DataGaps = append(result.DataGaps, "labor_cost")
	}
	if f.FixedRent == nil {
		result.DataGaps = append(result.DataGaps, "fixed_rent")
	}
	if f.VariableRent == nil {
		result.DataGaps = append(result.DataGaps, "variable_rent")
	}
	if f.NonLeaseCost == nil {
		result.DataGaps = append(result.DataGaps, "non_lease_cost")
	}
	if f.AreaSqm == nil || *f.AreaSqm <= 0 {
		result.DataGaps = append(result.DataGaps, "area_sqm")
	}
	// A matched import with missing mandatory four-wall inputs remains visible
	// but is not decision-ready. This prevents missing evidence from looking
	// like a healthy zero or a complete store result.
	if len(result.DataGaps) > 0 {
		result.DataReady = false
	}
	if f.GrossProfit != nil && f.LaborCost != nil && f.FixedRent != nil && f.VariableRent != nil && f.NonLeaseCost != nil {
		value := *f.GrossProfit - *f.LaborCost - *f.FixedRent - *f.VariableRent - *f.NonLeaseCost
		if f.OtherControllableCost != nil {
			value -= *f.OtherControllableCost
		}
		result.FourWallEBITDA = ptr(round2(value))
		result.ContributionMargin = ptr(round2(value / max(f.Revenue, 1) * 100))
	}
	if f.Revenue > 0 && f.FixedRent != nil {
		result.RentToSales = ptr(round2(*f.FixedRent / f.Revenue * 100))
	}
	if f.Revenue > 0 && f.FixedRent != nil && f.VariableRent != nil {
		result.OccupancyCostRatio = ptr(round2((*f.FixedRent + *f.VariableRent) / f.Revenue * 100))
	}
	if f.AreaSqm != nil && *f.AreaSqm > 0 {
		result.SalesPerSqm = ptr(round2(f.Revenue / *f.AreaSqm))
	}
	if f.GrossProfit != nil && f.LaborCost != nil && f.FixedRent != nil && f.VariableRent != nil && f.NonLeaseCost != nil {
		marginRate := *f.GrossProfit / max(f.Revenue, 1)
		variableRate := *f.VariableRent / max(f.Revenue, 1)
		fixedCosts := *f.LaborCost + *f.FixedRent + *f.NonLeaseCost
		if f.OtherControllableCost != nil {
			fixedCosts += *f.OtherControllableCost
		}
		if marginRate > variableRate {
			result.BreakEvenSales = ptr(round2(fixedCosts / (marginRate - variableRate)))
		}
	}
	return result
}

type CostBridge struct {
	Period             string  `json:"period"`
	Currency           string  `json:"currency"`
	StandardCost       float64 `json:"standard_cost"`
	ActualCost         float64 `json:"actual_cost"`
	Variance           float64 `json:"variance"`
	PurchasePrice      float64 `json:"purchase_price"`
	PurchasePriceVariance float64 `json:"purchase_price_variance"`
	MaterialUsage      float64 `json:"material_usage"`
	LaborEfficiency    float64 `json:"labor_efficiency"`
	YieldScrap         float64 `json:"yield_scrap"`
	Energy             float64 `json:"energy"`
	OverheadAbsorption float64 `json:"overhead_absorption"`
	Residual           float64 `json:"residual"`
	TiesOut            bool    `json:"ties_out"`
}

func CalculateCostBridge(f repository.EquipmentOperatingFact) (CostBridge, error) {
	if f.StandardCost == nil || f.ActualCost == nil {
		return CostBridge{}, fmt.Errorf("standard_cost and actual_cost are required")
	}
	result := CostBridge{Period: f.Period, Currency: f.Currency, StandardCost: round2(*f.StandardCost), ActualCost: round2(*f.ActualCost)}
	result.Variance = round2(result.ActualCost - result.StandardCost)
	if f.PurchasePrice != nil { result.PurchasePrice = round2(*f.PurchasePrice) }
	if f.PurchasePriceVariance != nil { result.PurchasePriceVariance = round2(*f.PurchasePriceVariance) }
	if f.MaterialUsageCost != nil {
		result.MaterialUsage = round2(*f.MaterialUsageCost)
	}
	if f.LaborCost != nil {
		result.LaborEfficiency = round2(*f.LaborCost)
	}
	if f.YieldPct != nil && f.ScrapQty != nil {
		result.YieldScrap = round2(*f.ScrapQty * (1 - *f.YieldPct/100))
	}
	if f.EnergyCost != nil {
		result.Energy = round2(*f.EnergyCost)
	}
	if f.OverheadAbsorption != nil {
		result.OverheadAbsorption = round2(*f.OverheadAbsorption)
	}
	result.Residual = round2(result.Variance - result.PurchasePriceVariance - result.MaterialUsage - result.LaborEfficiency - result.YieldScrap - result.Energy - result.OverheadAbsorption)
	result.TiesOut = math.Abs(result.Variance-(result.PurchasePriceVariance+result.MaterialUsage+result.LaborEfficiency+result.YieldScrap+result.Energy+result.OverheadAbsorption+result.Residual)) < 0.01
	return result, nil
}

func ptr(v float64) *float64 { return &v }
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
func round2(v float64) float64 { return math.Round(v*100) / 100 }
