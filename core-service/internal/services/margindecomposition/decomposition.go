package margindecomposition

import (
	"math"
)

type CategoryPeriodData struct {
	CategoryCode string  `json:"category_code"`
	CategoryName string  `json:"category_name"`
	Revenue      float64 `json:"revenue"`
	GrossProfit  float64 `json:"gross_profit"`
}

type DecompositionRequest struct {
	Currency string               `json:"currency"`
	Base     []CategoryPeriodData `json:"base"`
	Current  []CategoryPeriodData `json:"current"`
}

type CategoryAttribution struct {
	CategoryCode      string  `json:"category_code"`
	CategoryName      string  `json:"category_name"`
	BaseRevenue       float64 `json:"base_revenue"`
	CurrentRevenue    float64 `json:"current_revenue"`
	BaseMarginRate    float64 `json:"base_margin_rate"`
	CurrentMarginRate float64 `json:"current_margin_rate"`
	BaseGP            float64 `json:"base_gross_profit"`
	CurrentGP         float64 `json:"current_gross_profit"`
	GPVariance        float64 `json:"gross_profit_variance"`
	VolumeEffect      float64 `json:"volume_effect"`
	MixEffect         float64 `json:"mix_effect"`
	RateEffect        float64 `json:"rate_effect"`
}

type DecompositionResult struct {
	Currency            string                `json:"currency"`
	BaseTotalRevenue    float64               `json:"base_total_revenue"`
	CurrentTotalRevenue float64               `json:"current_total_revenue"`
	BaseTotalGP         float64               `json:"base_total_gross_profit"`
	CurrentTotalGP      float64               `json:"current_total_gross_profit"`
	BaseMarginRate      float64               `json:"base_margin_rate"`
	CurrentMarginRate   float64               `json:"current_margin_rate"`
	TotalGPVariance     float64               `json:"total_gross_profit_variance"`
	VolumeEffect        float64               `json:"volume_effect"`
	MixEffect           float64               `json:"mix_effect"`
	RateEffect          float64               `json:"rate_effect"`
	RoundingResidual    float64               `json:"rounding_residual"`
	IsConserved         bool                  `json:"is_conserved"`
	Categories          []CategoryAttribution `json:"categories"`
}

// Decompose decomposes Gross Profit Variance into Volume Effect, Mix Effect, Rate Effect, and Residual.
// Pure function, strictly conserved with explicit rounding residual.
func Decompose(req DecompositionRequest) DecompositionResult {
	var baseTotalRev, baseTotalGP, currTotalRev, currTotalGP float64

	baseMap := make(map[string]CategoryPeriodData)
	for _, b := range req.Base {
		baseTotalRev += b.Revenue
		baseTotalGP += b.GrossProfit
		baseMap[b.CategoryCode] = b
	}

	currMap := make(map[string]CategoryPeriodData)
	for _, c := range req.Current {
		currTotalRev += c.Revenue
		currTotalGP += c.GrossProfit
		currMap[c.CategoryCode] = c
	}

	baseMarginRate := 0.0
	if math.Abs(baseTotalRev) > 0.0001 {
		baseMarginRate = baseTotalGP / baseTotalRev
	}

	currMarginRate := 0.0
	if math.Abs(currTotalRev) > 0.0001 {
		currMarginRate = currTotalGP / currTotalRev
	}

	totalGPVariance := currTotalGP - baseTotalGP

	// 1. Overall Volume Effect: (R1 - R0) * M0
	volumeEffect := (currTotalRev - baseTotalRev) * baseMarginRate

	// Union of all categories
	allCategories := make(map[string]string)
	for code, b := range baseMap {
		allCategories[code] = b.CategoryName
	}
	for code, c := range currMap {
		if allCategories[code] == "" {
			allCategories[code] = c.CategoryName
		}
	}

	var totalMixEffect, totalRateEffect float64
	categoriesAttribution := make([]CategoryAttribution, 0, len(allCategories))

	for code, name := range allCategories {
		b := baseMap[code]
		c := currMap[code]

		bRev := b.Revenue
		bGP := b.GrossProfit
		cRev := c.Revenue
		cGP := c.GrossProfit

		bRate := 0.0
		if math.Abs(bRev) > 0.0001 {
			bRate = bGP / bRev
		}

		cRate := 0.0
		if math.Abs(cRev) > 0.0001 {
			cRate = cGP / cRev
		}

		bShare := 0.0
		if math.Abs(baseTotalRev) > 0.0001 {
			bShare = bRev / baseTotalRev
		}

		cShare := 0.0
		if math.Abs(currTotalRev) > 0.0001 {
			cShare = cRev / currTotalRev
		}

		// Category Rate Effect: cRev * (cRate - bRate)
		catRateEff := cRev * (cRate - bRate)

		// Category Mix Effect: currTotalRev * (cShare - bShare) * (bRate - baseMarginRate)
		catMixEff := currTotalRev * (cShare - bShare) * (bRate - baseMarginRate)

		// Category Volume Effect: (cRev - bRev) * baseMarginRate (category proportional)
		catVolEff := (cRev - bRev) * baseMarginRate

		totalMixEffect += catMixEff
		totalRateEffect += catRateEff

		categoriesAttribution = append(categoriesAttribution, CategoryAttribution{
			CategoryCode:      code,
			CategoryName:      name,
			BaseRevenue:       round2(bRev),
			CurrentRevenue:    round2(cRev),
			BaseMarginRate:    round4(bRate),
			CurrentMarginRate: round4(cRate),
			BaseGP:            round2(bGP),
			CurrentGP:         round2(cGP),
			GPVariance:        round2(cGP - bGP),
			VolumeEffect:      round2(catVolEff),
			MixEffect:         round2(catMixEff),
			RateEffect:        round2(catRateEff),
		})
	}

	sumParts := volumeEffect + totalMixEffect + totalRateEffect
	residual := totalGPVariance - sumParts
	isConserved := math.Abs(residual) < 1.0

	return DecompositionResult{
		Currency:            req.Currency,
		BaseTotalRevenue:    round2(baseTotalRev),
		CurrentTotalRevenue: round2(currTotalRev),
		BaseTotalGP:         round2(baseTotalGP),
		CurrentTotalGP:      round2(currTotalGP),
		BaseMarginRate:      round4(baseMarginRate),
		CurrentMarginRate:   round4(currMarginRate),
		TotalGPVariance:     round2(totalGPVariance),
		VolumeEffect:        round2(volumeEffect),
		MixEffect:           round2(totalMixEffect),
		RateEffect:          round2(totalRateEffect),
		RoundingResidual:    round2(residual),
		IsConserved:         isConserved,
		Categories:          categoriesAttribution,
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}
