// M4 plan comparison (design §6): the single semantic authority for
// "actual vs plan" on the retail side. Same discipline as retail-kpi-v1 —
// coverage thresholds downgrade instead of guessing, missing plan rows are
// null (never zero), mixed currencies degrade explicitly, and every
// variance carries its basis (actual, plan, variance, attainment, materiality).
package retailkpi

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/services/retailperiod"
)

// PlanFact is one plan line at store grain for a calendar month.
type PlanFact struct {
	StoreID      string
	Period       string
	Currency     string
	Revenue      *float64
	GrossProfit  *float64
	LaborCost    *float64
	FixedRent    *float64
	VariableRent *float64
	NonLeaseCost *float64
}

// PlanVariance is one KPI's actual-vs-plan verdict.
type PlanVariance struct {
	KPI                 string   `json:"kpi"`
	Actual              *float64 `json:"actual"`
	Plan                *float64 `json:"plan"`
	Variance            *float64 `json:"variance"`
	VariancePct         *float64 `json:"variance_pct"`
	AttainmentPct       *float64 `json:"attainment_pct"`
	MaterialityExceeded bool     `json:"materiality_exceeded"`
	DecisionReady       bool     `json:"decision_ready"`
	DowngradeReason     string   `json:"downgrade_reason,omitempty"`
}

// PlanComparison is the full verdict plus its provenance basis.
type PlanComparison struct {
	Period             string         `json:"period"`
	PlanVersionID      string         `json:"plan_version_id,omitempty"`
	PlanVersionName    string         `json:"plan_version_name,omitempty"`
	PlanVersionType    string         `json:"plan_version_type,omitempty"`
	PlanAsOfPeriod     string         `json:"plan_as_of_period,omitempty"`
	PlanSource         string         `json:"plan_source,omitempty"`
	PlanIsOfficial     bool           `json:"plan_is_official"`
	Currency           string         `json:"currency,omitempty"`
	ExpectedStoreCount int            `json:"expected_store_count"`
	ActualStoreCount   int            `json:"actual_store_count"`
	PlanStoreCount     int            `json:"plan_store_count"`
	Variances          []PlanVariance `json:"variances"`
	DecisionReady      bool           `json:"decision_ready"`
	DowngradeReason    string         `json:"downgrade_reason,omitempty"`
}

// PlanSet is the resolved plan basis for one calendar month.
type PlanSet struct {
	VersionID   string
	VersionName string
	VersionType string
	AsOfPeriod  string
	Source      string
	IsOfficial  bool
	Facts       []PlanFact
}

// PlanReader resolves the authoritative plan basis for a period. The two
// real adapters are the repository over fpna_plan_lines (store grain) and
// the fixed-seed simulated plan; a nil set means no plan covers the period.
type PlanReader interface {
	ReadPlan(ctx context.Context, legalEntityID, period string) (*PlanSet, error)
}

// ComparePlanRequest scopes one calendar-month comparison.
type ComparePlanRequest struct {
	Period             string // "YYYY-MM"
	ExpectedStoreCount int
	// ExpectedDaysInMonth is the calendar month's day count; when set, every
	// store compared on both sides must have at least this many distinct
	// observed days, otherwise the comparison downgrades — actual for half a
	// month against a full-month plan is a mismatch, not a variance (c1).
	ExpectedDaysInMonth     int
	MaterialityThresholdPct float64
}

type planKPI struct {
	code        string
	actualValue func(DailyFact) *float64
	planValue   func(PlanFact) *float64
}

var planKPIs = []planKPI{
	{code: "revenue", actualValue: func(f DailyFact) *float64 { return f.Revenue }, planValue: func(p PlanFact) *float64 { return p.Revenue }},
	{code: "gross_profit", actualValue: func(f DailyFact) *float64 { return f.GrossProfit }, planValue: func(p PlanFact) *float64 { return p.GrossProfit }},
	{code: "labor_cost", actualValue: func(f DailyFact) *float64 { return f.LaborCost }, planValue: func(p PlanFact) *float64 { return p.LaborCost }},
	{code: "fixed_rent", actualValue: func(f DailyFact) *float64 { return f.FixedRent }, planValue: func(p PlanFact) *float64 { return p.FixedRent }},
	{code: "variable_rent", actualValue: func(f DailyFact) *float64 { return f.VariableRent }, planValue: func(p PlanFact) *float64 { return p.VariableRent }},
	{code: "non_lease_cost", actualValue: func(f DailyFact) *float64 { return f.NonLeaseCost }, planValue: func(p PlanFact) *float64 { return p.NonLeaseCost }},
}

// ComparePlan aggregates store-day facts to the calendar month and compares
// them with the store-grain plan lines of the same month. Stores missing on
// either side downgrade per-KPI readiness without inventing zeros; sums use
// only stores present on both sides (compare existing coverage, never pad).
func ComparePlan(actual []DailyFact, plan []PlanFact, request ComparePlanRequest) (*PlanComparison, error) {
	if request.MaterialityThresholdPct <= 0 {
		request.MaterialityThresholdPct = 5
	}
	window, err := retailperiod.Parse(request.Period, time.Time{})
	if err != nil {
		return nil, fmt.Errorf("plan period: %w", err)
	}
	comparison := &PlanComparison{
		Period:             request.Period,
		ExpectedStoreCount: request.ExpectedStoreCount,
	}

	actualByStore := map[string][]DailyFact{}
	for _, fact := range actual {
		if fact.BusinessDate.Before(window.From) || fact.BusinessDate.After(window.To) {
			continue
		}
		actualByStore[fact.StoreID] = append(actualByStore[fact.StoreID], fact)
	}
	planByStore := map[string]PlanFact{}
	currencies := map[string]bool{}
	for _, fact := range plan {
		if fact.Period != request.Period || fact.StoreID == "" {
			continue
		}
		planByStore[fact.StoreID] = fact
		currencies[fact.Currency] = true
	}
	for _, facts := range actualByStore {
		for _, fact := range facts {
			currencies[fact.Currency] = true
		}
	}
	comparison.ActualStoreCount = len(actualByStore)
	comparison.PlanStoreCount = len(planByStore)
	comparison.Currency = singleKey(currencies)
	if len(currencies) > 1 {
		comparison.DecisionReady = false
		comparison.DowngradeReason = "mixed_currency"
		comparison.Variances = emptyVariances("mixed_currency")
		return comparison, nil
	}

	reasons := []string{}
	if comparison.ActualStoreCount < request.ExpectedStoreCount {
		reasons = append(reasons, fmt.Sprintf("actual_coverage_insufficient_%d_of_%d", comparison.ActualStoreCount, request.ExpectedStoreCount))
	}
	if comparison.PlanStoreCount < request.ExpectedStoreCount {
		reasons = append(reasons, fmt.Sprintf("plan_coverage_insufficient_%d_of_%d", comparison.PlanStoreCount, request.ExpectedStoreCount))
	}
	if request.ExpectedDaysInMonth > 0 {
		shortDays, minObserved := monthDayCoverage(actualByStore, planByStore, request.ExpectedDaysInMonth)
		if shortDays > 0 {
			reasons = append(reasons, fmt.Sprintf("actual_day_coverage_insufficient_%d_stores_min_%d_of_%d_days", shortDays, minObserved, request.ExpectedDaysInMonth))
		}
	}

	comparison.Variances = make([]PlanVariance, 0, len(planKPIs)+1)
	for _, kpi := range planKPIs {
		comparison.Variances = append(comparison.Variances, compareKPI(kpi, actualByStore, planByStore, request))
	}
	comparison.Variances = append(comparison.Variances, compareContribution(actualByStore, planByStore, request))
	comparison.DecisionReady = true
	for _, variance := range comparison.Variances {
		if !variance.DecisionReady {
			comparison.DecisionReady = false
		}
	}
	if len(reasons) > 0 {
		comparison.DecisionReady = false
		comparison.DowngradeReason = strings.Join(reasons, "; ")
	}
	return comparison, nil
}

func compareKPI(kpi planKPI, actualByStore map[string][]DailyFact, planByStore map[string]PlanFact, request ComparePlanRequest) PlanVariance {
	variance := PlanVariance{KPI: kpi.code, DecisionReady: true}
	_, planMissing := sumActual(kpi, actualByStore, planByStore)
	_, actualMissing := sumPlan(kpi, planByStore, actualByStore)
	reasons := []string{}
	// actualMissing counts plan stores with no actual facts; planMissing
	// counts actual stores with no plan line (missing plan rows stay null).
	if actualMissing > 0 {
		reasons = append(reasons, fmt.Sprintf("actual_missing_for_%d_stores", actualMissing))
	}
	if planMissing > 0 {
		reasons = append(reasons, fmt.Sprintf("plan_missing_for_%d_stores", planMissing))
	}
	// Sums compare only stores present on both sides.
	actualTotal, planTotal := sumOverIntersection(kpi, actualByStore, planByStore)
	if actualTotal == nil || planTotal == nil {
		variance.DecisionReady = false
		variance.DowngradeReason = strings.Join(reasons, "; ")
		return variance
	}
	variance.Actual, variance.Plan = actualTotal, planTotal
	diff := *actualTotal - *planTotal
	variance.Variance = &diff
	if *planTotal != 0 {
		pct := diff / math.Abs(*planTotal) * 100
		variance.VariancePct = &pct
		variance.MaterialityExceeded = math.Abs(pct) >= request.MaterialityThresholdPct
		attainment := *actualTotal / *planTotal * 100
		variance.AttainmentPct = &attainment
	} else {
		reasons = append(reasons, "zero_plan")
		variance.DecisionReady = false
	}
	if len(reasons) > 0 {
		variance.DecisionReady = false
		variance.DowngradeReason = strings.Join(reasons, "; ")
	}
	return variance
}

func compareContribution(actualByStore map[string][]DailyFact, planByStore map[string]PlanFact, request ComparePlanRequest) PlanVariance {
	variance := PlanVariance{KPI: "store_contribution", DecisionReady: true}
	reasons := []string{}
	actualTotal := 0.0
	planTotal := 0.0
	compared := 0
	for storeID := range actualByStore {
		plan, planOK := planByStore[storeID]
		if !planOK {
			continue
		}
		actualValue := contributionOfActual(actualByStore[storeID])
		planValue := contributionOfPlan(plan)
		if actualValue == nil || planValue == nil {
			reasons = append(reasons, fmt.Sprintf("component_missing_for_%s", storeID))
			continue
		}
		actualTotal += *actualValue
		planTotal += *planValue
		compared++
	}
	for storeID := range planByStore {
		if _, ok := actualByStore[storeID]; !ok {
			reasons = append(reasons, "actual_missing_for_plan_store")
			break
		}
	}
	if compared == 0 {
		variance.DecisionReady = false
		variance.DowngradeReason = strings.Join(append(reasons, "no_comparable_stores"), "; ")
		return variance
	}
	variance.Actual = &actualTotal
	variance.Plan = &planTotal
	diff := actualTotal - planTotal
	variance.Variance = &diff
	if planTotal != 0 {
		pct := diff / math.Abs(planTotal) * 100
		variance.VariancePct = &pct
		variance.MaterialityExceeded = math.Abs(pct) >= request.MaterialityThresholdPct
		attainment := actualTotal / planTotal * 100
		variance.AttainmentPct = &attainment
	}
	if len(reasons) > 0 {
		variance.DecisionReady = false
		variance.DowngradeReason = strings.Join(reasons, "; ")
	}
	return variance
}

// monthDayCoverage counts how many stores compared on both sides fell short
// of the expected distinct days in the month, and the smallest observed day
// count among them.
func monthDayCoverage(actualByStore map[string][]DailyFact, planByStore map[string]PlanFact, expectedDays int) (shortStores, minObserved int) {
	compared := 0
	for storeID, facts := range actualByStore {
		if _, ok := planByStore[storeID]; !ok {
			continue
		}
		days := map[string]bool{}
		for _, fact := range facts {
			days[fact.BusinessDate.Format("2006-01-02")] = true
		}
		compared++
		if compared == 1 || len(days) < minObserved {
			minObserved = len(days)
		}
		if len(days) < expectedDays {
			shortStores++
		}
	}
	return shortStores, minObserved
}

func sumActual(kpi planKPI, actualByStore map[string][]DailyFact, planByStore map[string]PlanFact) (total *float64, missing int) {
	for storeID := range actualByStore {
		if _, ok := planByStore[storeID]; !ok {
			missing++
		}
	}
	return nil, missing
}

func sumPlan(kpi planKPI, planByStore map[string]PlanFact, actualByStore map[string][]DailyFact) (total *float64, missing int) {
	for storeID := range planByStore {
		if _, ok := actualByStore[storeID]; !ok {
			missing++
		}
	}
	return nil, missing
}

// sumOverIntersection is the actual comparison basis: stores with both
// sides present.
func sumOverIntersection(kpi planKPI, actualByStore map[string][]DailyFact, planByStore map[string]PlanFact) (*float64, *float64) {
	var actualTotal, planTotal *float64
	for storeID, facts := range actualByStore {
		plan, ok := planByStore[storeID]
		if !ok {
			continue
		}
		actualStore := 0.0
		actualSeen := false
		for _, fact := range facts {
			if value := kpi.actualValue(fact); value != nil {
				actualStore += *value
				actualSeen = true
			}
		}
		planValue := kpi.planValue(plan)
		if !actualSeen || planValue == nil {
			continue
		}
		if actualTotal == nil {
			zero := 0.0
			actualTotal = &zero
			planTotal = new(float64)
		}
		*actualTotal += actualStore
		*planTotal += *planValue
	}
	return actualTotal, planTotal
}

func contributionOfActual(facts []DailyFact) *float64 {
	var gross, labor, occupancy, other *float64
	for _, fact := range facts {
		gross = addPtr(gross, fact.GrossProfit)
		labor = addPtr(labor, fact.LaborCost)
		occupancy = addPtr(occupancy, addPtr(addPtr(fact.FixedRent, fact.VariableRent), fact.NonLeaseCost))
		other = addPtr(other, fact.OtherControllableCost)
	}
	return subtractContribution(gross, labor, occupancy, other)
}

func contributionOfPlan(plan PlanFact) *float64 {
	return subtractContribution(plan.GrossProfit, plan.LaborCost, addPtr(addPtr(plan.FixedRent, plan.VariableRent), plan.NonLeaseCost), nil)
}

func subtractContribution(gross, labor, occupancy, other *float64) *float64 {
	if gross == nil {
		return nil
	}
	total := *gross
	for _, component := range []*float64{labor, occupancy, other} {
		if component != nil {
			total -= *component
		}
	}
	return &total
}

func addPtr(base *float64, value *float64) *float64 {
	if value == nil {
		return base
	}
	if base == nil {
		copy := *value
		return &copy
	}
	*base += *value
	return base
}

func emptyVariances(reason string) []PlanVariance {
	codes := []string{"revenue", "gross_profit", "labor_cost", "fixed_rent", "variable_rent", "non_lease_cost", "store_contribution"}
	results := make([]PlanVariance, 0, len(codes))
	for _, code := range codes {
		results = append(results, PlanVariance{KPI: code, DecisionReady: false, DowngradeReason: reason})
	}
	return results
}

func singleKey(values map[string]bool) string {
	if len(values) != 1 {
		return ""
	}
	for key := range values {
		return key
	}
	return ""
}
