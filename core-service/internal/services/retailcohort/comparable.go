package retailcohort

import (
	"fmt"
	"time"

	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

type LifecycleStatus string

const (
	LifecyclePreOpening LifecycleStatus = "pre_opening"
	LifecycleRampUp     LifecycleStatus = "ramp_up"
	LifecycleMature     LifecycleStatus = "mature"
	LifecycleClosed     LifecycleStatus = "closed"
	LifecycleUndecided  LifecycleStatus = "undecided"
)

type StoreLifecycle struct {
	StoreID     string     `json:"store_id"`
	StoreCode   string     `json:"store_code"`
	StoreName   string     `json:"store_name"`
	Brand       string     `json:"brand"`
	Region      string     `json:"region"`
	StoreFormat string     `json:"store_format"`
	OpeningDate *time.Time `json:"opening_date,omitempty"`
	ClosingDate *time.Time `json:"closing_date,omitempty"`
	IsActive    bool       `json:"is_active"`
}

type PeriodPair struct {
	CurrentStart  time.Time `json:"current_start"`
	CurrentEnd    time.Time `json:"current_end"`
	BaselineStart time.Time `json:"baseline_start"`
	BaselineEnd   time.Time `json:"baseline_end"`
}

type ComparabilityPolicy struct {
	RampUpMonths               int  `json:"ramp_up_months"`
	RequireContinuousOperation bool `json:"require_continuous_operation"`
	RequireSameFormat          bool `json:"require_same_format"`
}

func DefaultPolicy() ComparabilityPolicy {
	return ComparabilityPolicy{
		RampUpMonths:               12,
		RequireContinuousOperation: true,
		RequireSameFormat:          true,
	}
}

type ExclusionReason string

const (
	ExclusionTooNew        ExclusionReason = "too_new"
	ExclusionClosed        ExclusionReason = "closed"
	ExclusionUndecidable   ExclusionReason = "missing_lifecycle_data"
	ExclusionFormatChanged ExclusionReason = "format_changed"
	ExclusionRenovating    ExclusionReason = "renovating"
)

type Exclusion struct {
	StoreID   string          `json:"store_id"`
	StoreCode string          `json:"store_code"`
	StoreName string          `json:"store_name"`
	Reason    ExclusionReason `json:"reason"`
	Detail    string          `json:"detail"`
}

type CohortResult struct {
	Policy        ComparabilityPolicy `json:"policy"`
	Included      []StoreLifecycle    `json:"included"`
	Excluded      []Exclusion         `json:"excluded"`
	Undecidable   []Exclusion         `json:"undecidable"`
	TotalStores   int                 `json:"total_stores"`
	IncludedCount int                 `json:"included_count"`
}

// CalculateLifecycleStatus evaluates the lifecycle state of a store at a given
// reference point without mutating persistent state.
func CalculateLifecycleStatus(opening, closing *time.Time, asOf time.Time, rampMonths int) LifecycleStatus {
	if rampMonths <= 0 {
		rampMonths = 12
	}
	if opening == nil {
		return LifecycleUndecided
	}
	if closing != nil && !asOf.Before(*closing) {
		return LifecycleClosed
	}
	if asOf.Before(*opening) {
		return LifecyclePreOpening
	}
	matureDate := opening.AddDate(0, rampMonths, 0)
	if asOf.Before(matureDate) {
		return LifecycleRampUp
	}
	return LifecycleMature
}

// EvaluateComparableCohort separates stores into Included, Excluded, and Undecidable
// groups based on the provided period window and comparability policy.
func EvaluateComparableCohort(stores []StoreLifecycle, window PeriodPair, policy ComparabilityPolicy) CohortResult {
	if policy.RampUpMonths <= 0 {
		policy.RampUpMonths = 12
	}

	result := CohortResult{
		Policy:      policy,
		Included:    make([]StoreLifecycle, 0),
		Excluded:    make([]Exclusion, 0),
		Undecidable: make([]Exclusion, 0),
		TotalStores: len(stores),
	}

	for _, s := range stores {
		if s.OpeningDate == nil {
			result.Undecidable = append(result.Undecidable, Exclusion{
				StoreID:   s.StoreID,
				StoreCode: s.StoreCode,
				StoreName: s.StoreName,
				Reason:    ExclusionUndecidable,
				Detail:    "missing opening_date; cannot determine mature comparability",
			})
			continue
		}

		// Store must have finished its ramp-up period before baseline start to be comparable
		matureDate := s.OpeningDate.AddDate(0, policy.RampUpMonths, 0)
		if matureDate.After(window.BaselineStart) {
			result.Excluded = append(result.Excluded, Exclusion{
				StoreID:   s.StoreID,
				StoreCode: s.StoreCode,
				StoreName: s.StoreName,
				Reason:    ExclusionTooNew,
				Detail:    fmt.Sprintf("opened on %s; ramp-up (%d months) completes on %s after baseline start %s", s.OpeningDate.Format("2006-01-02"), policy.RampUpMonths, matureDate.Format("2006-01-02"), window.BaselineStart.Format("2006-01-02")),
			})
			continue
		}

		// Store must not have closed before or during the comparison window
		if s.ClosingDate != nil && !window.CurrentEnd.Before(*s.ClosingDate) {
			result.Excluded = append(result.Excluded, Exclusion{
				StoreID:   s.StoreID,
				StoreCode: s.StoreCode,
				StoreName: s.StoreName,
				Reason:    ExclusionClosed,
				Detail:    fmt.Sprintf("store closed on %s before period end %s", s.ClosingDate.Format("2006-01-02"), window.CurrentEnd.Format("2006-01-02")),
			})
			continue
		}

		if !s.IsActive {
			result.Excluded = append(result.Excluded, Exclusion{
				StoreID:   s.StoreID,
				StoreCode: s.StoreCode,
				StoreName: s.StoreName,
				Reason:    ExclusionClosed,
				Detail:    "store is marked inactive",
			})
			continue
		}

		result.Included = append(result.Included, s)
	}

	result.IncludedCount = len(result.Included)
	return result
}

type SSSGResult struct {
	Cohort            CohortResult         `json:"cohort"`
	SSSG              *float64             `json:"sssg"`
	CurrentRevenue    *float64             `json:"current_revenue"`
	BaselineRevenue   *float64             `json:"baseline_revenue"`
	CurrentAggregate  *retailkpi.Aggregate `json:"current_aggregate,omitempty"`
	BaselineAggregate *retailkpi.Aggregate `json:"baseline_aggregate,omitempty"`
	Reason            string               `json:"reason,omitempty"`
	DecisionReady     bool                 `json:"decision_ready"`
}

// CalculateSSSG computes Same-Store Sales Growth using only the comparable store cohort.
func CalculateSSSG(currentFacts, baselineFacts []retailkpi.DailyFact, cohort CohortResult, currentReq, baselineReq retailkpi.Request) SSSGResult {
	res := SSSGResult{Cohort: cohort}
	if len(cohort.Included) == 0 {
		res.Reason = "no_comparable_stores"
		return res
	}

	includedMap := make(map[string]bool, len(cohort.Included))
	for _, s := range cohort.Included {
		includedMap[s.StoreID] = true
	}

	filterFacts := func(facts []retailkpi.DailyFact) []retailkpi.DailyFact {
		filtered := make([]retailkpi.DailyFact, 0, len(facts))
		for _, f := range facts {
			if includedMap[f.StoreID] {
				filtered = append(filtered, f)
			}
		}
		return filtered
	}

	currFiltered := filterFacts(currentFacts)
	baseFiltered := filterFacts(baselineFacts)

	currentReq.GroupBy = "total"
	currentReq.ExpectedStoreCount = len(cohort.Included)
	currAggs, currCoverage, err1 := retailkpi.AggregateFacts(currFiltered, currentReq)

	baselineReq.GroupBy = "total"
	baselineReq.ExpectedStoreCount = len(cohort.Included)
	baseAggs, baseCoverage, err2 := retailkpi.AggregateFacts(baseFiltered, baselineReq)

	if err1 != nil || err2 != nil || len(currAggs) == 0 || len(baseAggs) == 0 {
		res.Reason = "aggregation_error"
		return res
	}

	currAgg := currAggs[0]
	baseAgg := baseAggs[0]
	res.CurrentAggregate = &currAgg
	res.BaselineAggregate = &baseAgg

	currRev := currAgg.KPIs["revenue"].Value
	baseRev := baseAgg.KPIs["revenue"].Value
	res.CurrentRevenue = currRev
	res.BaselineRevenue = baseRev

	if currRev != nil && baseRev != nil {
		change, reason := retailkpi.ChangeRate(currRev, baseRev, "percent")
		res.SSSG = change
		res.Reason = reason
	} else {
		res.Reason = "missing_revenue"
	}

	res.DecisionReady = currAgg.DecisionReady && baseAgg.DecisionReady && !retailkpi.CoverageIncomplete(currCoverage) && !retailkpi.CoverageIncomplete(baseCoverage) && res.SSSG != nil
	return res
}
