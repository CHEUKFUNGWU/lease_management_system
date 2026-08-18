package promotionattribution

import (
	"fmt"
	"math"
	"time"

	"github.com/lease-management-system/core-service/internal/services/periodutil"
)

type AttributionStatus string

const (
	StatusSeparable    AttributionStatus = "separable"
	StatusNonSeparable AttributionStatus = "non_separable"
)

type Promotion struct {
	ID             string    `json:"id"`
	LegalEntityID  string    `json:"legal_entity_id"`
	PromoCode      string    `json:"promo_code"`
	Name           string    `json:"name"`
	PromoType      string    `json:"promo_type"`
	StartDate      string    `json:"start_date"` // YYYY-MM-DD
	EndDate        string    `json:"end_date"`   // YYYY-MM-DD
	TargetScope    string    `json:"target_scope"`
	ScopeValues    []string  `json:"scope_values"`
	Currency       string    `json:"currency"`
	BudgetAmount   float64   `json:"budget_amount"`
	ApprovalStatus string    `json:"approval_status"`
	Owner          string    `json:"owner,omitempty"`
	Description    string    `json:"description,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type PromotionCost struct {
	Category string  `json:"cost_category"` // subsidy, materials, labor, marketing, other
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Period   string  `json:"period"`
}

type DailyFact struct {
	StoreID      string
	BusinessDate string // YYYY-MM-DD
	Currency     string
	Revenue      float64
	GrossProfit  float64
	Transactions int
}

type RunRate struct {
	DailyRevenue      float64 `json:"daily_revenue"`
	DailyGrossProfit  float64 `json:"daily_gross_profit"`
	DailyTransactions float64 `json:"daily_transactions"`
}

type AttributionResult struct {
	PromoCode              string            `json:"promo_code"`
	Name                   string            `json:"name"`
	Currency               string            `json:"currency"`
	EventDays              int               `json:"event_days"`
	ActualRevenue          float64           `json:"actual_revenue"`
	ActualGrossProfit      float64           `json:"actual_gross_profit"`
	BaselineRevenue        float64           `json:"baseline_revenue"`
	BaselineGrossProfit    float64           `json:"baseline_gross_profit"`
	IncrementalRevenue     float64           `json:"incremental_revenue"`
	IncrementalGrossProfit float64           `json:"incremental_gross_profit"`
	TotalCost              float64           `json:"total_cost"`
	BudgetAmount           float64           `json:"budget_amount"`
	CostBreakdown          map[string]float64 `json:"cost_breakdown"`
	ROI                    *float64          `json:"roi,omitempty"` // Incremental GP / Total Cost
	Status                 AttributionStatus `json:"status"`
	IsSeparable            bool              `json:"is_separable"`
	OverlapWarnings        []string          `json:"overlap_warnings"`
	Disclaimers            []string          `json:"disclaimers"`
}

// Attribute executes pure promotion ROI attribution with overlap non-separable degradation.
func Attribute(promo Promotion, costs []PromotionCost, actual []DailyFact, baseline RunRate, overlaps []Promotion) AttributionResult {
	// 1. Calculate actual totals during promotion
	var actualRev, actualGP float64
	daySet := make(map[string]struct{})
	for _, f := range actual {
		actualRev += f.Revenue
		actualGP += f.GrossProfit
		daySet[f.BusinessDate] = struct{}{}
	}

	eventDays := len(daySet)
	if eventDays == 0 {
		// Estimate days from date range if no actual facts yet
		st, err1 := time.Parse("2006-01-02", promo.StartDate)
		et, err2 := time.Parse("2006-01-02", promo.EndDate)
		if err1 == nil && err2 == nil && !et.Before(st) {
			eventDays = int(et.Sub(st).Hours()/24) + 1
		}
	}

	// 2. Calculate baseline totals using run rate
	baselineRev := baseline.DailyRevenue * float64(eventDays)
	baselineGP := baseline.DailyGrossProfit * float64(eventDays)

	// 3. Incremental amounts
	incRev := actualRev - baselineRev
	incGP := actualGP - baselineGP

	// 4. Calculate total cost and category breakdown
	costMap := make(map[string]float64)
	var totalCost float64
	for _, c := range costs {
		costMap[c.Category] += c.Amount
		totalCost += c.Amount
	}

	// 5. Calculate ROI: Incremental GP / Total Cost
	var roi *float64
	if totalCost > 0.005 {
		val := round4(incGP / totalCost)
		roi = &val
	}

	// 6. Overlap detection & degradation
	var overlapWarnings []string
	isSeparable := true
	status := StatusSeparable

	for _, ov := range overlaps {
		if ov.PromoCode == promo.PromoCode {
			continue
		}
		// Check date overlap
		if periodutil.DatesOverlap(promo.StartDate, promo.EndDate, ov.StartDate, ov.EndDate) {
			isSeparable = false
			status = StatusNonSeparable
			overlapWarnings = append(overlapWarnings, fmt.Sprintf(
				"同期存在重叠活动「%s」(编码: %s, 期间: %s~%s)，活动归因与增量贡献不可完全分离",
				ov.Name, ov.PromoCode, ov.StartDate, ov.EndDate,
			))
		}
	}

	disclaimers := []string{
		"本测算基于活动前同期基线运行率 (Run-Rate) 进行关联分析，不构成完全排他的因果性证明。",
	}
	if !isSeparable {
		disclaimers = append(disclaimers, "存在重叠活动或多因素并发，增量销售与毛利数据已降级为不可分离状态。")
	}

	return AttributionResult{
		PromoCode:              promo.PromoCode,
		Name:                   promo.Name,
		Currency:               promo.Currency,
		EventDays:              eventDays,
		ActualRevenue:          round2(actualRev),
		ActualGrossProfit:      round2(actualGP),
		BaselineRevenue:        round2(baselineRev),
		BaselineGrossProfit:    round2(baselineGP),
		IncrementalRevenue:     round2(incRev),
		IncrementalGrossProfit: round2(incGP),
		TotalCost:              round2(totalCost),
		BudgetAmount:           round2(promo.BudgetAmount),
		CostBreakdown:          costMap,
		ROI:                    roi,
		Status:                 status,
		IsSeparable:            isSeparable,
		OverlapWarnings:        overlapWarnings,
		Disclaimers:            disclaimers,
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}
