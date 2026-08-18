package categoryreconciliation

import (
	"fmt"
	"math"
)

type ReconciliationStatus string

const (
	StatusTie             ReconciliationStatus = "tie"
	StatusWithinTolerance ReconciliationStatus = "within_tolerance"
	StatusMismatch        ReconciliationStatus = "mismatch"
	StatusNoDetail        ReconciliationStatus = "no_detail"
)

type CategoryFact struct {
	StoreID      string
	BusinessDate string // YYYY-MM-DD
	Currency     string
	CategoryCode string
	Revenue      float64
	GrossProfit  float64
	Transactions int
	Units        float64
}

type DailySummaryFact struct {
	StoreID      string
	BusinessDate string // YYYY-MM-DD
	Currency     string
	Revenue      float64
	GrossProfit  float64
}

type Tolerance struct {
	AbsoluteRevenue float64
	AbsoluteGP      float64
	Ratio           float64 // e.g. 0.0001 (0.01%)
}

func DefaultTolerance() Tolerance {
	return Tolerance{
		AbsoluteRevenue: 1.0, // <= 1.0 currency unit rounding tolerance
		AbsoluteGP:      1.0,
		Ratio:           0.001, // 0.1% tolerance
	}
}

type DayStoreResult struct {
	StoreID        string               `json:"store_id"`
	BusinessDate   string               `json:"business_date"`
	Currency       string               `json:"currency"`
	SummaryRevenue float64              `json:"summary_revenue"`
	DetailRevenue  float64              `json:"detail_revenue"`
	RevenueDiff    float64              `json:"revenue_diff"`
	SummaryGP      float64              `json:"summary_gross_profit"`
	DetailGP       float64              `json:"detail_gross_profit"`
	GPDiff         float64              `json:"gross_profit_diff"`
	Status         ReconciliationStatus `json:"status"`
	Reason         string               `json:"reason,omitempty"`
}

type ReconciliationResult struct {
	TotalStoreDays         int              `json:"total_store_days"`
	TieCount               int              `json:"tie_count"`
	WithinToleranceCount   int              `json:"within_tolerance_count"`
	MismatchCount          int              `json:"mismatch_count"`
	NoDetailCount          int              `json:"no_detail_count"`
	StoreDayResults        []DayStoreResult `json:"store_day_results"`
	Mismatches             []DayStoreResult `json:"mismatches"`
	OverallStatus          ReconciliationStatus `json:"overall_status"`
}

// Reconcile performs pure store-day detail vs summary reconciliation.
// Critical rule: DO NOT auto-balance or force equality when mismatch occurs.
func Reconcile(details []CategoryFact, summaries []DailySummaryFact, tol Tolerance) ReconciliationResult {
	if tol.AbsoluteRevenue <= 0 && tol.Ratio <= 0 {
		tol = DefaultTolerance()
	}

	type key struct {
		storeID string
		date    string
		curr    string
	}

	// 1. Group category details by (store, date, currency)
	detailSums := make(map[key]*struct {
		rev   float64
		gp    float64
		count int
	})

	for _, d := range details {
		k := key{storeID: d.StoreID, date: d.BusinessDate, curr: d.Currency}
		entry := detailSums[k]
		if entry == nil {
			entry = &struct {
				rev   float64
				gp    float64
				count int
			}{}
			detailSums[k] = entry
		}
		entry.rev += d.Revenue
		entry.gp += d.GrossProfit
		entry.count++
	}

	// 2. Iterate summaries
	results := make([]DayStoreResult, 0, len(summaries))
	mismatches := make([]DayStoreResult, 0)
	tieCount := 0
	withinTolCount := 0
	mismatchCount := 0
	noDetailCount := 0

	for _, s := range summaries {
		k := key{storeID: s.StoreID, date: s.BusinessDate, curr: s.Currency}
		dEntry := detailSums[k]

		r := DayStoreResult{
			StoreID:        s.StoreID,
			BusinessDate:   s.BusinessDate,
			Currency:       s.Currency,
			SummaryRevenue: round2(s.Revenue),
			SummaryGP:      round2(s.GrossProfit),
		}

		if dEntry == nil || dEntry.count == 0 {
			r.DetailRevenue = 0
			r.DetailGP = 0
			r.RevenueDiff = round2(-s.Revenue)
			r.GPDiff = round2(-s.GrossProfit)
			r.Status = StatusNoDetail
			r.Reason = "no category detail facts found for this store-day"
			noDetailCount++
		} else {
			r.DetailRevenue = round2(dEntry.rev)
			r.DetailGP = round2(dEntry.gp)
			r.RevenueDiff = round2(dEntry.rev - s.Revenue)
			r.GPDiff = round2(dEntry.gp - s.GrossProfit)

			revDiffAbs := math.Abs(r.RevenueDiff)
			gpDiffAbs := math.Abs(r.GPDiff)

			if revDiffAbs < 0.005 && gpDiffAbs < 0.005 {
				r.Status = StatusTie
				tieCount++
			} else {
				// Check tolerance
				revRatio := 0.0
				if math.Abs(s.Revenue) > 0.005 {
					revRatio = revDiffAbs / math.Abs(s.Revenue)
				}
				gpRatio := 0.0
				if math.Abs(s.GrossProfit) > 0.005 {
					gpRatio = gpDiffAbs / math.Abs(s.GrossProfit)
				}

				revOk := revDiffAbs <= tol.AbsoluteRevenue || revRatio <= tol.Ratio
				gpOk := gpDiffAbs <= tol.AbsoluteGP || gpRatio <= tol.Ratio

				if revOk && gpOk {
					r.Status = StatusWithinTolerance
					r.Reason = fmt.Sprintf("variance within tolerance (revDiff: %.2f, gpDiff: %.2f)", r.RevenueDiff, r.GPDiff)
					withinTolCount++
				} else {
					r.Status = StatusMismatch
					r.Reason = fmt.Sprintf("reconciliation mismatch (revDiff: %.2f, gpDiff: %.2f)", r.RevenueDiff, r.GPDiff)
					mismatchCount++
					mismatches = append(mismatches, r)
				}
			}
		}

		results = append(results, r)
	}

	overall := StatusTie
	if mismatchCount > 0 {
		overall = StatusMismatch
	} else if withinTolCount > 0 {
		overall = StatusWithinTolerance
	} else if noDetailCount > 0 && tieCount == 0 {
		overall = StatusNoDetail
	}

	return ReconciliationResult{
		TotalStoreDays:       len(summaries),
		TieCount:             tieCount,
		WithinToleranceCount: withinTolCount,
		MismatchCount:        mismatchCount,
		NoDetailCount:        noDetailCount,
		StoreDayResults:      results,
		Mismatches:           mismatches,
		OverallStatus:        overall,
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
