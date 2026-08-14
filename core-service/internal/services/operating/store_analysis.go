package operating

import (
	"sort"
	"strings"

	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

// StoreBenchmark is a deterministic peer comparison. The Peer Cohort follows
// the retail-kpi-v1 rule (CONTEXT.md): stores sharing the target's brand,
// region and currency, decision-ready, excluding the target; a cohort below
// the minimum sample yields no benchmark rather than a weak one.
type StoreBenchmark struct {
	StoreID     string   `json:"store_id"`
	StoreCode   string   `json:"store_code"`
	Region      string   `json:"region"`
	Brand       string   `json:"brand"`
	Currency    string   `json:"currency,omitempty"`
	Cohort      string   `json:"cohort"`
	Metric      string   `json:"metric"`
	Value       *float64 `json:"value,omitempty"`
	PeerAverage *float64 `json:"peer_average,omitempty"`
	PeerCount   int      `json:"peer_count"`
	Percentile  *float64 `json:"percentile,omitempty"`
	DataReady   bool     `json:"data_ready"`
}

func BenchmarkStores(facts []*repository.StoreOperatingFact) []StoreBenchmark {
	// Peer membership: same brand + region + currency, decision-ready, with a
	// measurable metric. The target itself is excluded from its own cohort.
	type peerMember struct {
		storeID string
		ebitda  float64
		ready   bool
	}
	cohorts := make(map[string][]peerMember)
	for _, fact := range facts {
		if fact == nil {
			continue
		}
		metric := CalculateFourWall(*fact)
		if metric.FourWallEBITDA == nil {
			continue
		}
		key := fact.Region + "\x00" + fact.Brand + "\x00" + strings.ToUpper(fact.Currency)
		cohorts[key] = append(cohorts[key], peerMember{storeID: fact.StoreID, ebitda: *metric.FourWallEBITDA, ready: metric.DataReady})
	}
	result := make([]StoreBenchmark, 0, len(facts))
	for _, fact := range facts {
		if fact == nil {
			continue
		}
		metric := CalculateFourWall(*fact)
		key := fact.Region + "\x00" + fact.Brand + "\x00" + strings.ToUpper(fact.Currency)
		peers := make([]float64, 0, len(cohorts[key]))
		for _, member := range cohorts[key] {
			if member.storeID == fact.StoreID {
				continue // the target is not part of its own cohort
			}
			peers = append(peers, member.ebitda)
		}
		average, percentile := (*float64)(nil), (*float64)(nil)
		if len(peers) >= retailkpi.MinimumPeerCount {
			sum := 0.0
			for _, value := range peers {
				sum += value
			}
			avg := round2(sum / float64(len(peers)))
			average = &avg
			percentile = retailkpi.PercentileRank(peers, *metric.FourWallEBITDA)
		}
		cohort := fact.CohortCode
		if cohort == "" && fact.StoreAgeMonths != nil {
			switch {
			case *fact.StoreAgeMonths < 12:
				cohort = "new_0_12m"
			case *fact.StoreAgeMonths < 36:
				cohort = "ramp_12_36m"
			default:
				cohort = "mature_36m_plus"
			}
		}
		result = append(result, StoreBenchmark{StoreID: fact.StoreID, StoreCode: fact.StoreCode, Region: fact.Region, Brand: fact.Brand, Currency: fact.Currency, Cohort: cohort, Metric: "four_wall_ebitda", Value: metric.FourWallEBITDA, PeerAverage: average, PeerCount: len(peers), Percentile: percentile, DataReady: metric.DataReady})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].StoreCode < result[j].StoreCode })
	return result
}

type StoreCohort struct {
	Cohort         string   `json:"cohort"`
	StoreCount     int      `json:"store_count"`
	Revenue        float64  `json:"revenue"`
	FourWallEBITDA *float64 `json:"four_wall_ebitda,omitempty"`
	DataReadyCount int      `json:"data_ready_count"`
}

func SummarizeStoreCohorts(facts []*repository.StoreOperatingFact) []StoreCohort {
	type aggregate struct {
		count, ready    int
		revenue, ebitda float64
		ebitdaCount     int
	}
	groups := map[string]*aggregate{}
	for _, fact := range facts {
		if fact == nil {
			continue
		}
		cohort := fact.CohortCode
		if cohort == "" && fact.StoreAgeMonths != nil {
			if *fact.StoreAgeMonths < 12 {
				cohort = "new_0_12m"
			} else if *fact.StoreAgeMonths < 36 {
				cohort = "ramp_12_36m"
			} else {
				cohort = "mature_36m_plus"
			}
		}
		if cohort == "" {
			cohort = "unclassified"
		}
		group := groups[cohort]
		if group == nil {
			group = &aggregate{}
			groups[cohort] = group
		}
		group.count++
		group.revenue += fact.Revenue
		metric := CalculateFourWall(*fact)
		if metric.DataReady {
			group.ready++
		}
		if metric.FourWallEBITDA != nil {
			group.ebitda += *metric.FourWallEBITDA
			group.ebitdaCount++
		}
	}
	result := make([]StoreCohort, 0, len(groups))
	for cohort, group := range groups {
		item := StoreCohort{Cohort: cohort, StoreCount: group.count, Revenue: round2(group.revenue), DataReadyCount: group.ready}
		if group.ebitdaCount > 0 {
			avg := round2(group.ebitda / float64(group.ebitdaCount))
			item.FourWallEBITDA = &avg
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Cohort < result[j].Cohort })
	return result
}

type PromotionROIInput struct {
	Currency        string  `json:"currency"`
	BaselineSales   float64 `json:"baseline_sales"`
	PromotedSales   float64 `json:"promoted_sales"`
	GrossMarginPct  float64 `json:"gross_margin_pct"`
	PromotionCost   float64 `json:"promotion_cost"`
	TurnoverRentPct float64 `json:"turnover_rent_pct"`
}

type PromotionROIResult struct {
	Currency                string   `json:"currency"`
	IncrementalSales        float64  `json:"incremental_sales"`
	IncrementalGrossProfit  float64  `json:"incremental_gross_profit"`
	IncrementalTurnoverRent float64  `json:"incremental_turnover_rent"`
	NetBenefit              float64  `json:"net_benefit"`
	ROI                     *float64 `json:"roi,omitempty"`
	ReviewRequired          bool     `json:"review_required"`
}

func EvaluatePromotionROI(input PromotionROIInput) PromotionROIResult {
	incrementalSales := input.PromotedSales - input.BaselineSales
	grossProfit := incrementalSales * input.GrossMarginPct / 100
	turnoverRent := incrementalSales * input.TurnoverRentPct / 100
	net := grossProfit - turnoverRent - input.PromotionCost
	result := PromotionROIResult{Currency: input.Currency, IncrementalSales: round2(incrementalSales), IncrementalGrossProfit: round2(grossProfit), IncrementalTurnoverRent: round2(turnoverRent), NetBenefit: round2(net), ReviewRequired: true}
	if input.PromotionCost > 0 {
		value := round2(net / input.PromotionCost * 100)
		result.ROI = &value
	}
	return result
}
