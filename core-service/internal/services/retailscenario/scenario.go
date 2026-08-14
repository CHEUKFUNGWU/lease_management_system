package retailscenario

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailpulse"
	"github.com/lease-management-system/core-service/internal/services/retailstore360"
	"github.com/lease-management-system/core-service/internal/services/sourceenvelope"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

const (
	ScenarioVersion = "retail-store-scenario-v1"
	FormulaVersion  = retailkpi.FormulaVersion
)

var (
	ErrInvalidRequest  = errors.New("invalid retail store scenario request")
	ErrStoreNotFound   = errors.New("retail store is not visible")
	ErrDataUnavailable = errors.New("retail store scenario data is unavailable")
)

type DataUnavailableError struct{ Reason string }

type ScenarioEvidenceError struct {
	Reason   string
	Evidence Evidence
}

func (e *DataUnavailableError) Error() string {
	return "retail store scenario data is unavailable: " + e.Reason
}
func (e *DataUnavailableError) Unwrap() error { return ErrDataUnavailable }

func (e *ScenarioEvidenceError) Error() string {
	return "retail store scenario data is unavailable: " + e.Reason
}
func (e *ScenarioEvidenceError) Unwrap() error { return ErrDataUnavailable }

type FactReader interface {
	QueryFacts(context.Context, string, string, string, string, string, string, []string) (*repository.RetailKPIFactSet, error)
}

type Assumptions struct {
	RevenueChangePct               float64 `json:"revenue_change_pct"`
	GrossMarginRateChangePP        float64 `json:"gross_margin_rate_change_pp"`
	LaborCostChangePct             float64 `json:"labor_cost_change_pct"`
	FixedRentChangePct             float64 `json:"fixed_rent_change_pct"`
	VariableRentRateChangePP       float64 `json:"variable_rent_rate_change_pp"`
	NonLeaseCostChangePct          float64 `json:"non_lease_cost_change_pct"`
	OtherControllableCostChangePct float64 `json:"other_controllable_cost_change_pct"`
}

func (a Assumptions) IsZero() bool {
	return a == (Assumptions{})
}

type ScenarioInput struct {
	Key         string      `json:"key"`
	Name        string      `json:"name"`
	Assumptions Assumptions `json:"assumptions"`
}

type Query struct {
	LegalEntityID  string
	StoreID        string
	AsOf           time.Time
	WindowDays     int
	Classification string
	DatasetVersion string
	SourceSystem   string
}

type EvaluateRequest struct {
	HorizonMonths int             `json:"horizon_months"`
	Scenarios     []ScenarioInput `json:"scenarios"`
}

type Period struct {
	DateFrom string `json:"date_from"`
	DateTo   string `json:"date_to"`
}

type StoreIdentity struct {
	StoreID   string `json:"store_id"`
	StoreCode string `json:"store_code"`
	StoreName string `json:"store_name"`
	Brand     string `json:"brand"`
	Region    string `json:"region"`
}

type Metric struct {
	Baseline *float64 `json:"baseline"`
	Result   *float64 `json:"result"`
	Delta    *float64 `json:"delta"`
	Unit     string   `json:"unit"`
	Status   string   `json:"status"`
	Reason   string   `json:"reason,omitempty"`
}

type BridgeItem struct {
	Code         string   `json:"code"`
	Label        string   `json:"label"`
	Contribution *float64 `json:"contribution"`
	Unit         string   `json:"unit"`
}

type Bridge struct {
	Items            []BridgeItem `json:"items"`
	TotalChange      *float64     `json:"total_change"`
	RoundingResidual *float64     `json:"rounding_residual"`
	Status           string       `json:"status"`
	Reason           string       `json:"reason,omitempty"`
}

type ScenarioResult struct {
	Key                       string            `json:"key"`
	Name                      string            `json:"name"`
	Assumptions               Assumptions       `json:"assumptions"`
	Metrics                   map[string]Metric `json:"metrics"`
	MonthlyContributionChange *float64          `json:"monthly_contribution_change"`
	HorizonContributionChange *float64          `json:"horizon_contribution_change"`
	Bridge                    Bridge            `json:"bridge"`
}

type Evidence struct {
	Current            Period          `json:"current"`
	ObservedStoreDays  int             `json:"observed_store_days"`
	ExpectedStoreDays  int             `json:"expected_store_days"`
	CoverageRate       *float64        `json:"coverage_rate"`
	RequiredFields     []string        `json:"required_fields"`
	SourceSystems      []string        `json:"source_systems"`
	DatasetVersions    []string        `json:"dataset_versions"`
	FactVersionMin     int             `json:"fact_version_min"`
	FactVersionMax     int             `json:"fact_version_max"`
	HighestAsOf        *time.Time      `json:"highest_as_of,omitempty"`
	KPIDrilldownURL    string          `json:"kpi_drilldown_url"`
	RequestAssumptions json.RawMessage `json:"request_assumptions"`
}

type Response struct {
	Basis              string           `json:"basis"`
	ScenarioVersion    string           `json:"scenario_version"`
	FormulaVersion     string           `json:"formula_version"`
	DiagnosticsVersion string           `json:"diagnostics_version"`
	SideEffects        bool             `json:"side_effects"`
	ReviewRequired     bool             `json:"review_required"`
	OfficialImpact     bool             `json:"official_impact"`
	IFRS16Impact       bool             `json:"ifrs16_impact"`
	GeneratedAt        time.Time        `json:"generated_at"`
	Store              StoreIdentity    `json:"store"`
	DataClassification string           `json:"data_classification"`
	DatasetVersion     string           `json:"dataset_version,omitempty"`
	SourceSystem       string           `json:"source_system,omitempty"`
	Envelope           sourceenvelope.Envelope `json:"envelope"`
	Currency           string           `json:"currency"`
	Current            Period           `json:"current"`
	HorizonMonths      int              `json:"horizon_months"`
	Baseline           ScenarioResult   `json:"baseline"`
	Scenarios          []ScenarioResult `json:"scenarios"`
	Evidence           Evidence         `json:"evidence"`
}

type baseValues struct {
	Revenue, GrossProfit, LaborCost, FixedRent, VariableRent, NonLeaseCost, OtherCost float64
	GrossMarginRate, VariableRentRate                                                 float64
	Contribution, ContributionMargin, Occupancy                                       float64
}

func NewService(reader FactReader) *Service { return &Service{reader: reader, now: time.Now} }

type Service struct {
	reader FactReader
	now    func() time.Time
}

func (s *Service) Evaluate(ctx context.Context, q Query, req EvaluateRequest) (*Response, error) {
	if err := validateQuery(q); err != nil {
		return nil, err
	}
	if req.HorizonMonths == 0 {
		req.HorizonMonths = 12
	}
	if err := validateRequest(req); err != nil {
		return nil, err
	}
	currentEnd := dateOnly(q.AsOf)
	currentStart := currentEnd.AddDate(0, 0, -(q.WindowDays - 1))
	dateFrom := currentStart.Format("2006-01-02")
	dateTo := currentEnd.Format("2006-01-02")
	set, err := s.reader.QueryFacts(ctx, q.LegalEntityID, dateFrom, dateTo, q.Classification, q.DatasetVersion, q.SourceSystem, nil)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, fmt.Errorf("retail scenario fact reader returned nil")
	}
	population := findPopulation(set.ExpectedStores, q.StoreID)
	if population == nil {
		return nil, ErrStoreNotFound
	}
	storeFacts := filterStore(set.Facts, q.StoreID)
	base, observed, currency, reason := buildBase(storeFacts, currentStart, currentEnd, q.WindowDays)
	if reason != "" {
		return nil, &ScenarioEvidenceError{Reason: reason, Evidence: buildEvidence(q, storeFacts, currentStart, currentEnd, observed)}
	}
	results := make([]ScenarioResult, 0, len(req.Scenarios)-1)
	var baseline ScenarioResult
	for _, input := range req.Scenarios {
		if err := validateResultingRates(input.Assumptions, base); err != nil {
			return nil, &ScenarioEvidenceError{Reason: "resulting_rate_out_of_range", Evidence: buildEvidence(q, storeFacts, currentStart, currentEnd, observed)}
		}
		result := calculateScenario(input, base, req.HorizonMonths)
		if input.Key == "baseline" {
			baseline = result
		} else {
			results = append(results, result)
		}
	}
	coverage := float64(observed) / float64(q.WindowDays) * 100
	requestJSON, _ := json.Marshal(req)
	evidence := Evidence{Current: Period{DateFrom: dateFrom, DateTo: dateTo}, ObservedStoreDays: observed, ExpectedStoreDays: q.WindowDays, CoverageRate: &coverage, RequiredFields: scenarioRequiredFields(), RequestAssumptions: requestJSON, KPIDrilldownURL: kpiDrilldownURL(q, q.StoreID, currentStart, currentEnd)}
	response := &Response{Basis: "Scenario", ScenarioVersion: ScenarioVersion, FormulaVersion: FormulaVersion, DiagnosticsVersion: retailstore360.DiagnosticsVersion, SideEffects: false, ReviewRequired: true, OfficialImpact: false, IFRS16Impact: false, GeneratedAt: s.now(), Store: StoreIdentity{StoreID: population.StoreID, StoreCode: population.StoreCode, StoreName: population.StoreName, Brand: population.Brand, Region: population.Region}, DataClassification: q.Classification, DatasetVersion: q.DatasetVersion, SourceSystem: singleSource(storeFacts), Currency: currency, Current: Period{DateFrom: dateFrom, DateTo: dateTo}, HorizonMonths: req.HorizonMonths, Baseline: baseline, Scenarios: results, Evidence: evidence}
	scenarioFillEnvelope(response, q, storeFacts, currentStart, currentEnd, observed, q.WindowDays, true)
	return response, nil
}

// scenarioFillEnvelope builds the Source Envelope for the store's own facts
// and mirrors it into the response's provenance fields. ready=false means the
// evaluation did not run (buildBase failed or rates were out of range).
func scenarioFillEnvelope(response *Response, q Query, facts []retailkpi.DailyFact, from, to time.Time, observed, windowDays int, ready bool) {
	readyReason := ""
	if !ready {
		readyReason = "scenario_not_ready"
	} else if windowDays > 0 && observed < windowDays {
		readyReason = "incomplete_store_day_coverage"
	}
	env := sourceenvelope.Build(facts, sourceenvelope.Spec{
		Classification: q.Classification,
		FormulaVersion: FormulaVersion,
		PulseVersion:   retailpulse.PulseVersion,
		Current:        sourceenvelope.PeriodSpec{From: from, To: to, ExpectedStoreDays: windowDays},
		DecisionReady:  ready, DecisionReadyReason: readyReason,
		GeneratedAt: response.GeneratedAt,
	})
	response.Envelope = env
	response.Evidence.SourceSystems = env.SourceSystems
	response.Evidence.DatasetVersions = env.DatasetVersions
	response.Evidence.FactVersionMin = env.FactVersionMin
	response.Evidence.FactVersionMax = env.FactVersionMax
	response.Evidence.HighestAsOf = env.HighestAsOf
}

func validateQuery(q Query) error {
	if strings.TrimSpace(q.LegalEntityID) == "" || strings.TrimSpace(q.StoreID) == "" || q.AsOf.IsZero() || (q.WindowDays != 7 && q.WindowDays != 14 && q.WindowDays != 28) || (q.Classification != "production" && q.Classification != "simulated") {
		return ErrInvalidRequest
	}
	if q.Classification == "simulated" && strings.TrimSpace(q.DatasetVersion) == "" {
		return ErrInvalidRequest
	}
	if q.Classification == "production" && strings.TrimSpace(q.DatasetVersion) != "" {
		return ErrInvalidRequest
	}
	return nil
}

func validateRequest(req EvaluateRequest) error {
	if req.HorizonMonths != 3 && req.HorizonMonths != 6 && req.HorizonMonths != 12 {
		return ErrInvalidRequest
	}
	if len(req.Scenarios) < 2 || len(req.Scenarios) > 4 {
		return ErrInvalidRequest
	}
	seen := map[string]bool{}
	baseline := 0
	for _, item := range req.Scenarios {
		if strings.TrimSpace(item.Key) == "" || strings.TrimSpace(item.Name) == "" || seen[item.Key] {
			return ErrInvalidRequest
		}
		seen[item.Key] = true
		if item.Key == "baseline" {
			baseline++
			if !item.Assumptions.IsZero() {
				return ErrInvalidRequest
			}
		}
		if err := validateAssumptions(item.Assumptions); err != nil {
			return err
		}
	}
	if baseline != 1 {
		return ErrInvalidRequest
	}
	return nil
}

func validateAssumptions(a Assumptions) error {
	values := []float64{a.RevenueChangePct, a.GrossMarginRateChangePP, a.LaborCostChangePct, a.FixedRentChangePct, a.VariableRentRateChangePP, a.NonLeaseCostChangePct, a.OtherControllableCostChangePct}
	for i, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return ErrInvalidRequest
		}
		if i == 1 || i == 4 {
			if value < -100 || value > 100 {
				return ErrInvalidRequest
			}
		} else if value < -100 || value > 300 {
			return ErrInvalidRequest
		}
	}
	return nil
}

func validateResultingRates(a Assumptions, base baseValues) error {
	margin := base.GrossMarginRate + a.GrossMarginRateChangePP
	variable := base.VariableRentRate + a.VariableRentRateChangePP
	if margin < 0 || margin > 100 || variable < 0 || variable > 100 {
		return ErrInvalidRequest
	}
	return nil
}

func buildBase(facts []retailkpi.DailyFact, from, to time.Time, expected int) (baseValues, int, string, string) {
	currency, status := singleCurrency(facts)
	if status == "conflict" {
		return baseValues{}, 0, "", "currency_conflict"
	}
	if currency == "" {
		return baseValues{}, 0, "", "no_facts"
	}
	current := make([]retailkpi.DailyFact, 0, len(facts))
	for _, f := range facts {
		if !f.BusinessDate.Before(from) && !f.BusinessDate.After(to) && f.Currency == currency {
			current = append(current, f)
		}
	}
	if len(current) == 0 {
		return baseValues{}, 0, currency, "no_facts"
	}
	if len(current) != expected {
		return baseValues{}, len(current), currency, "incomplete_store_day_coverage"
	}
	for _, f := range current {
		if f.DataQualityStatus == "invalid" {
			return baseValues{}, len(current), currency, "data_quality_invalid"
		}
		if f.MappingStatus != "" && f.MappingStatus != "mapped" {
			return baseValues{}, len(current), currency, "mapping_" + f.MappingStatus
		}
	}
	fields := []*float64{}
	for _, f := range current {
		fields = append(fields, f.Revenue, f.GrossProfit, f.LaborCost, f.FixedRent, f.VariableRent, f.NonLeaseCost, f.OtherControllableCost)
	}
	for _, value := range fields {
		if value == nil {
			return baseValues{}, len(current), currency, "missing_required_field"
		}
	}
	b := baseValues{}
	for _, f := range current {
		b.Revenue += *f.Revenue
		b.GrossProfit += *f.GrossProfit
		b.LaborCost += *f.LaborCost
		b.FixedRent += *f.FixedRent
		b.VariableRent += *f.VariableRent
		b.NonLeaseCost += *f.NonLeaseCost
		b.OtherCost += *f.OtherControllableCost
	}
	factor := 30 / float64(expected)
	b.Revenue *= factor
	b.GrossProfit *= factor
	b.LaborCost *= factor
	b.FixedRent *= factor
	b.VariableRent *= factor
	b.NonLeaseCost *= factor
	b.OtherCost *= factor
	if b.Revenue == 0 {
		return baseValues{}, len(current), currency, "zero_denominator"
	}
	aggregates, _, aggregateErr := retailkpi.AggregateFacts(current, retailkpi.Request{DateFrom: from, DateTo: to, RequestedDateFrom: from.Format("2006-01-02"), RequestedDateTo: to.Format("2006-01-02"), GroupBy: "store", ExpectedStoreCount: 1})
	if aggregateErr != nil || len(aggregates) != 1 || !aggregates[0].DecisionReady {
		return baseValues{}, len(current), currency, "decision_not_ready"
	}
	b.GrossMarginRate = b.GrossProfit / b.Revenue * 100
	b.VariableRentRate = b.VariableRent / b.Revenue * 100
	b.Occupancy = b.FixedRent + b.VariableRent + b.NonLeaseCost
	b.Contribution = b.GrossProfit - b.LaborCost - b.Occupancy - b.OtherCost
	b.ContributionMargin = b.Contribution / b.Revenue * 100
	return b, len(current), currency, ""
}

func calculateScenario(input ScenarioInput, base baseValues, horizon int) ScenarioResult {
	a := input.Assumptions
	revenue := base.Revenue * (1 + a.RevenueChangePct/100)
	margin := base.GrossMarginRate + a.GrossMarginRateChangePP
	labor := base.LaborCost * (1 + a.LaborCostChangePct/100)
	fixed := base.FixedRent * (1 + a.FixedRentChangePct/100)
	variableRate := base.VariableRentRate + a.VariableRentRateChangePP
	variable := revenue * variableRate / 100
	nonLease := base.NonLeaseCost * (1 + a.NonLeaseCostChangePct/100)
	other := base.OtherCost * (1 + a.OtherControllableCostChangePct/100)
	grossProfit := revenue * margin / 100
	occupancy := fixed + variable + nonLease
	contribution := grossProfit - labor - occupancy - other
	contributionMargin := 0.0
	marginMetric := "complete"
	marginReason := ""
	if revenue == 0 {
		marginMetric = "unavailable"
		marginReason = "zero_denominator"
	} else {
		contributionMargin = contribution / revenue * 100
	}
	metrics := map[string]Metric{}
	put := func(code, unit string, baseline, result float64, status, reason string) {
		d := result - baseline
		metrics[code] = Metric{Baseline: roundMetricPtr(baseline, unit), Result: roundMetricPtr(result, unit), Delta: roundMetricPtr(d, unit), Unit: unit, Status: status, Reason: reason}
	}
	put("revenue", "currency", base.Revenue, revenue, "complete", "")
	put("gross_profit", "currency", base.GrossProfit, grossProfit, "complete", "")
	put("gross_margin_rate", "percent", base.GrossMarginRate, margin, "complete", "")
	put("labor_cost", "currency", base.LaborCost, labor, "complete", "")
	put("fixed_rent", "currency", base.FixedRent, fixed, "complete", "")
	put("variable_rent_rate", "percent", base.VariableRentRate, variableRate, "complete", "")
	put("variable_rent", "currency", base.VariableRent, variable, "complete", "")
	put("non_lease_cost", "currency", base.NonLeaseCost, nonLease, "complete", "")
	put("other_controllable_cost", "currency", base.OtherCost, other, "complete", "")
	put("occupancy_cash_cost", "currency", base.Occupancy, occupancy, "complete", "")
	put("store_contribution", "currency", base.Contribution, contribution, "complete", "")
	if marginMetric == "unavailable" {
		metrics["store_contribution_margin"] = Metric{Baseline: roundPtr(base.ContributionMargin), Result: nil, Delta: nil, Unit: "percent", Status: marginMetric, Reason: marginReason}
	} else {
		put("store_contribution_margin", "percent", base.ContributionMargin, contributionMargin, marginMetric, marginReason)
	}
	change := contribution - base.Contribution
	bridge := buildBridge(grossProfit-base.GrossProfit, -(labor - base.LaborCost), -(fixed - base.FixedRent), -(variable - base.VariableRent), -(nonLease - base.NonLeaseCost), -(other - base.OtherCost), change)
	horizonChange := change * float64(horizon)
	return ScenarioResult{Key: input.Key, Name: input.Name, Assumptions: a, Metrics: metrics, MonthlyContributionChange: roundPtr(change), HorizonContributionChange: roundPtr(horizonChange), Bridge: bridge}
}

func buildBridge(values ...float64) Bridge {
	if len(values) != 7 {
		return Bridge{Status: "unavailable", Reason: "bridge_unavailable"}
	}
	total := values[6]
	items := []BridgeItem{{Code: "gross_profit", Label: "毛利额", Contribution: roundPtr(values[0]), Unit: "currency"}, {Code: "labor_cost", Label: "人工成本", Contribution: roundPtr(values[1]), Unit: "currency"}, {Code: "fixed_rent", Label: "固定现金租金", Contribution: roundPtr(values[2]), Unit: "currency"}, {Code: "variable_rent", Label: "变动租金", Contribution: roundPtr(values[3]), Unit: "currency"}, {Code: "non_lease_cost", Label: "非租赁占用成本", Contribution: roundPtr(values[4]), Unit: "currency"}, {Code: "other_controllable_cost", Label: "其他可控成本", Contribution: roundPtr(values[5]), Unit: "currency"}}
	roundedTotal := round(total)
	sum := 0.0
	for _, item := range items {
		sum += valueOrZero(item.Contribution)
	}
	residual := round(roundedTotal - sum)
	return Bridge{Items: items, TotalChange: roundPtr(roundedTotal), RoundingResidual: roundPtr(residual), Status: "complete"}
}

func round(value float64) float64     { return math.Round(value*100) / 100 }
func roundPtr(value float64) *float64 { v := round(value); return &v }
func roundMetric(value float64, unit string) float64 {
	if unit == "percent" {
		return math.Round(value*10000) / 10000
	}
	return round(value)
}
func roundMetricPtr(value float64, unit string) *float64 {
	v := roundMetric(value, unit)
	return &v
}
func valueOrZero(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func findPopulation(stores []retailkpi.StorePopulation, id string) *retailkpi.StorePopulation {
	for i := range stores {
		if stores[i].StoreID == id {
			return &stores[i]
		}
	}
	return nil
}
func filterStore(facts []retailkpi.DailyFact, id string) []retailkpi.DailyFact {
	out := make([]retailkpi.DailyFact, 0)
	for _, f := range facts {
		if f.StoreID == id {
			out = append(out, f)
		}
	}
	return out
}
func singleCurrency(facts []retailkpi.DailyFact) (string, string) {
	values := map[string]bool{}
	for _, f := range facts {
		if f.Currency != "" {
			values[f.Currency] = true
		}
	}
	if len(values) == 0 {
		return "", "unknown"
	}
	if len(values) > 1 {
		return "", "conflict"
	}
	for value := range values {
		return value, "known"
	}
	return "", "unknown"
}
func storeSourceSystems(facts []retailkpi.DailyFact) []string {
	values := map[string]bool{}
	for _, f := range facts {
		if f.SourceSystem != "" {
			values[f.SourceSystem] = true
		}
	}
	return sortedKeys(values)
}
func singleSource(facts []retailkpi.DailyFact) string {
	values := sourceenvelope.SourceSystems(facts)
	if len(values) == 1 {
		return values[0]
	}
	return ""
}
func sortedKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func buildEvidence(q Query, facts []retailkpi.DailyFact, from, to time.Time, observed int) Evidence {
	coverage := (*float64)(nil)
	if q.WindowDays > 0 {
		value := float64(observed) / float64(q.WindowDays) * 100
		coverage = &value
	}
	// A failed evaluation still carries a full Source Envelope so the reason
	// is visible alongside the provenance — a rejected scenario is never
	// reported as if the data simply did not exist.
	env := sourceenvelope.Build(facts, sourceenvelope.Spec{
		Classification: q.Classification,
		FormulaVersion: FormulaVersion,
		PulseVersion:   retailpulse.PulseVersion,
		Current:        sourceenvelope.PeriodSpec{From: from, To: to, ExpectedStoreDays: q.WindowDays},
		DecisionReady:  false, DecisionReadyReason: "scenario_not_ready",
		GeneratedAt: time.Now().UTC(),
	})
	return Evidence{Current: Period{DateFrom: from.Format("2006-01-02"), DateTo: to.Format("2006-01-02")}, ObservedStoreDays: observed, ExpectedStoreDays: q.WindowDays, CoverageRate: coverage, RequiredFields: scenarioRequiredFields(), SourceSystems: env.SourceSystems, DatasetVersions: env.DatasetVersions, FactVersionMin: env.FactVersionMin, FactVersionMax: env.FactVersionMax, HighestAsOf: env.HighestAsOf, KPIDrilldownURL: kpiDrilldownURL(q, q.StoreID, from, to)}
}

func scenarioRequiredFields() []string {
	return []string{"revenue", "gross_profit", "labor_cost", "fixed_rent", "variable_rent", "non_lease_cost", "other_controllable_cost"}
}
func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
func kpiDrilldownURL(q Query, store string, from, to time.Time) string {
	parts := []string{"group_by=store", "store_id=" + url.QueryEscape(store), "date_from=" + from.Format("2006-01-02"), "date_to=" + to.Format("2006-01-02"), "data_classification=" + url.QueryEscape(q.Classification)}
	if q.DatasetVersion != "" {
		parts = append(parts, "dataset_version="+url.QueryEscape(q.DatasetVersion))
	}
	if q.SourceSystem != "" {
		parts = append(parts, "source_system="+url.QueryEscape(q.SourceSystem))
	}
	return "/api/v1/retail/kpis/store-days?" + strings.Join(parts, "&")
}

func RequestFingerprint(query Query, req EvaluateRequest, actionTitle, planned, owner, due, verification string) string {
	payload := struct {
		LegalEntityID  string          `json:"legal_entity_id"`
		StoreID        string          `json:"store_id"`
		AsOf           string          `json:"as_of"`
		WindowDays     int             `json:"window_days"`
		Classification string          `json:"data_classification"`
		DatasetVersion string          `json:"dataset_version"`
		SourceSystem   string          `json:"source_system"`
		Request        EvaluateRequest `json:"request"`
		Title          string          `json:"title"`
		Planned        string          `json:"planned_action"`
		Owner          string          `json:"owner_name"`
		Due            string          `json:"due_date"`
		Verification   string          `json:"verification_period"`
	}{LegalEntityID: query.LegalEntityID, StoreID: query.StoreID, AsOf: dateOnly(query.AsOf).Format("2006-01-02"), WindowDays: query.WindowDays, Classification: query.Classification, DatasetVersion: query.DatasetVersion, SourceSystem: query.SourceSystem, Request: req, Title: actionTitle, Planned: planned, Owner: owner, Due: due, Verification: verification}
	raw, _ := json.Marshal(payload)
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

// ActionRuleCode is a stable, bounded scope-dedupe value for the existing
// fpna_action_items unique key. The exact target store remains in
// source_record_id; this code only distinguishes immutable scenario drafts.
func ActionRuleCode(fingerprint, idempotencyKey string) string {
	digest := sha256.Sum256([]byte(fingerprint + "|" + idempotencyKey))
	return "retail_store_scenario_v1:" + hex.EncodeToString(digest[:])
}
