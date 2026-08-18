package competitorreference

import (
	"math"
	"time"
)

type Observation struct {
	ID               string    `json:"id"`
	LegalEntityID    string    `json:"legal_entity_id"`
	StoreID          string    `json:"store_id"`
	CompetitorName   string    `json:"competitor_name"`
	CompetitorBrand  string    `json:"competitor_brand,omitempty"`
	DistanceMeters   *int      `json:"distance_meters,omitempty"`
	ObservationDate  string    `json:"observation_date"` // YYYY-MM-DD
	PriceIndex       *float64  `json:"price_index,omitempty"`
	PromoIntensity   string    `json:"promo_intensity"` // low, medium, high, aggressive
	FootfallEstimate *int      `json:"footfall_estimate,omitempty"`
	Observer         string    `json:"observer,omitempty"`
	Notes            string    `json:"notes,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type StoreBenchmarkSummary struct {
	StoreID             string        `json:"store_id"`
	CompetitorCount     int           `json:"competitor_count"`
	AvgPriceIndex       *float64      `json:"avg_price_index,omitempty"`
	HighestPromoThreat  string        `json:"highest_promo_threat"`
	RecentObservations  []Observation `json:"recent_observations"`
	BenchmarkDisclaimer string        `json:"benchmark_disclaimer"`
}

func SummarizeStoreCompetitors(storeID string, obs []Observation) StoreBenchmarkSummary {
	if len(obs) == 0 {
		return StoreBenchmarkSummary{
			StoreID:             storeID,
			CompetitorCount:     0,
			HighestPromoThreat:  "none",
			RecentObservations:  []Observation{},
			BenchmarkDisclaimer: "竞品商圈观测仅供横向参考，物理隔离于财务核算与法定报表体系。",
		}
	}

	var sumPrice float64
	var priceCount int
	threatLevel := "low"

	for _, o := range obs {
		if o.PriceIndex != nil && *o.PriceIndex > 0 {
			sumPrice += *o.PriceIndex
			priceCount++
		}
		if o.PromoIntensity == "aggressive" {
			threatLevel = "aggressive"
		} else if o.PromoIntensity == "high" && threatLevel != "aggressive" {
			threatLevel = "high"
		} else if o.PromoIntensity == "medium" && threatLevel == "low" {
			threatLevel = "medium"
		}
	}

	var avgPrice *float64
	if priceCount > 0 {
		val := math.Round((sumPrice/float64(priceCount))*100) / 100
		avgPrice = &val
	}

	return StoreBenchmarkSummary{
		StoreID:             storeID,
		CompetitorCount:     len(obs),
		AvgPriceIndex:       avgPrice,
		HighestPromoThreat:  threatLevel,
		RecentObservations:  obs,
		BenchmarkDisclaimer: "竞品商圈观测仅供横向参考，物理隔离于财务核算与法定报表体系。",
	}
}
