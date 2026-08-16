package retailstore360

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/services/retailpulse"
	"github.com/lease-management-system/core-service/internal/services/sourceenvelope"
)

const (
	DiagnosticsVersion = "retail-store-diagnostics-v1"
	// MinimumPeerCount is the shared retail-kpi-v1 cohort rule.
	MinimumPeerCount = retailkpi.MinimumPeerCount
)

var (
	ErrInvalidQuery     = errors.New("invalid retail store diagnostics query")
	ErrStoreNotFound    = errors.New("retail store is not visible")
	ErrInsufficientData = errors.New("retail store diagnostics data is unavailable")
)

type FactReader interface {
	QueryFacts(context.Context, string, string, string, string, string, string, []string) (*repository.RetailKPIFactSet, error)
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

type SummaryMetric struct {
	Current     retailkpi.KPIValue `json:"current"`
	Comparison  retailkpi.KPIValue `json:"comparison"`
	ChangeValue *float64           `json:"change_value"`
	ChangeType  string             `json:"change_type"`
	Status      string             `json:"status"`
	Reason      string             `json:"reason,omitempty"`
}

type DailyTrend struct {
	Date       string                        `json:"date"`
	Gap        bool                          `json:"gap"`
	TargetKPIs map[string]retailkpi.KPIValue `json:"target_kpis"`
	PeerMedian map[string]*float64           `json:"peer_median"`
	PeerCount  map[string]int                `json:"peer_count"`
}

type PeerBenchmark struct {
	Code              string   `json:"code"`
	Unit              string   `json:"unit"`
	Target            *float64 `json:"target"`
	PeerCount         int      `json:"peer_count"`
	Median            *float64 `json:"median"`
	P25               *float64 `json:"p25"`
	P75               *float64 `json:"p75"`
	Percentile        *float64 `json:"percentile"`
	TargetMinusMedian *float64 `json:"target_minus_median"`
	Status            string   `json:"status"`
	Reason            string   `json:"reason,omitempty"`
}

type BridgeItem struct {
	Code         string   `json:"code"`
	Label        string   `json:"label"`
	Contribution *float64 `json:"contribution"`
	Unit         string   `json:"unit"`
}
type Bridge struct {
	Code             string       `json:"code"`
	Method           string       `json:"method"`
	Version          string       `json:"version"`
	Status           string       `json:"status"`
	Current          *float64     `json:"current"`
	Comparison       *float64     `json:"comparison"`
	TotalChange      *float64     `json:"total_change"`
	Items            []BridgeItem `json:"items"`
	RoundingResidual *float64     `json:"rounding_residual"`
	Reason           string       `json:"reason,omitempty"`
}

type Observation struct {
	Code        string   `json:"code"`
	Label       string   `json:"label"`
	Statement   string   `json:"statement"`
	Reference   string   `json:"reference"`
	Status      string   `json:"status"`
	EvidenceIDs []string `json:"evidence_ids"`
}

type Evidence struct {
	Current           Period     `json:"current"`
	Comparison        Period     `json:"comparison"`
	ObservedStoreDays int        `json:"observed_store_days"`
	ExpectedStoreDays int        `json:"expected_store_days"`
	RequiredFields    []string   `json:"required_fields"`
	FormulaVersion    string     `json:"formula_version"`
	SourceSystems     []string   `json:"source_systems"`
	DatasetVersions   []string   `json:"dataset_versions"`
	FactVersionMin    int        `json:"fact_version_min"`
	FactVersionMax    int        `json:"fact_version_max"`
	HighestAsOf       *time.Time `json:"highest_as_of,omitempty"`
	DataQualityIssues []string   `json:"data_quality_issues,omitempty"`
	KPIDrilldownURL   string     `json:"kpi_drilldown_url"`
}

type Response struct {
	Basis               string                   `json:"basis"`
	DiagnosticsVersion  string                   `json:"diagnostics_version"`
	FormulaVersion      string                   `json:"formula_version"`
	PulseVersion        string                   `json:"pulse_version"`
	DataClassification  string                   `json:"data_classification"`
	DatasetVersion      string                   `json:"dataset_version,omitempty"`
	GeneratedAt         time.Time                `json:"generated_at"`
	Store               StoreIdentity            `json:"store"`
	Current             Period                   `json:"current"`
	Comparison          Period                   `json:"comparison"`
	TargetCoverage      retailkpi.Coverage       `json:"target_coverage"`
	ComparisonCoverage  retailkpi.Coverage       `json:"comparison_coverage"`
	DecisionReady       bool                     `json:"decision_ready"`
	DecisionReadyReason string                   `json:"decision_ready_reason,omitempty"`
	Envelope            sourceenvelope.Envelope  `json:"envelope"`
	Currency            string                   `json:"currency"`
	CurrencyStatus      string                   `json:"currency_status"`
	Summary             map[string]SummaryMetric `json:"summary"`
	DailyTrend          []DailyTrend             `json:"daily_trend"`
	PeerDefinition      string                   `json:"peer_definition"`
	MinimumPeerCount    int                      `json:"minimum_peer_count"`
	PeerBenchmark       []PeerBenchmark          `json:"peer_benchmark"`
	Bridges             []Bridge                 `json:"bridges"`
	Observations        []Observation            `json:"observations"`
	Evidence            Evidence                 `json:"evidence"`
	SourceSystems       []string                 `json:"source_systems"`
	DatasetVersions     []string                 `json:"dataset_versions"`
	FactVersionMin      int                      `json:"fact_version_min"`
	FactVersionMax      int                      `json:"fact_version_max"`
	HighestAsOf         *time.Time               `json:"highest_as_of,omitempty"`
	DataQualityIssues   []string                 `json:"data_quality_issues,omitempty"`
	KPIDrilldownURL     string                   `json:"kpi_drilldown_url"`
}

type Service struct {
	reader FactReader
	now    func() time.Time
}

func NewService(reader FactReader) *Service { return &Service{reader: reader, now: time.Now} }

var benchmarkCodes = []string{"revenue", "gross_profit", "gross_margin_rate", "footfall", "conversion_rate", "average_transaction_value", "labor_cost_rate", "occupancy_cash_cost_rate", "store_contribution", "store_contribution_margin", "sales_per_sqm"}

var summaryCodes = []string{"revenue", "gross_profit", "gross_margin_rate", "footfall", "conversion_rate", "average_transaction_value", "labor_cost", "occupancy_cash_cost", "other_controllable_cost", "labor_cost_rate", "occupancy_cash_cost_rate", "store_contribution", "store_contribution_margin", "sales_per_sqm"}

func (s *Service) Build(ctx context.Context, q Query) (*Response, error) {
	if s.reader == nil || strings.TrimSpace(q.LegalEntityID) == "" || strings.TrimSpace(q.StoreID) == "" || q.AsOf.IsZero() || (q.WindowDays != 7 && q.WindowDays != 14 && q.WindowDays != 28) || (q.Classification != "production" && q.Classification != "simulated") {
		return nil, ErrInvalidQuery
	}
	if q.Classification == "simulated" && strings.TrimSpace(q.DatasetVersion) == "" {
		return nil, ErrInvalidQuery
	}
	if q.Classification == "production" && strings.TrimSpace(q.DatasetVersion) != "" {
		return nil, ErrInvalidQuery
	}
	currentEnd := dateOnly(q.AsOf)
	currentStart := currentEnd.AddDate(0, 0, -(q.WindowDays - 1))
	comparisonEnd := currentStart.AddDate(0, 0, -1)
	comparisonStart := comparisonEnd.AddDate(0, 0, -(q.WindowDays - 1))
	dateFrom, dateTo := comparisonStart.Format("2006-01-02"), currentEnd.Format("2006-01-02")
	set, err := s.reader.QueryFacts(ctx, q.LegalEntityID, dateFrom, dateTo, q.Classification, q.DatasetVersion, q.SourceSystem, nil)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, fmt.Errorf("retail store diagnostics fact reader returned nil")
	}
	var population *retailkpi.StorePopulation
	for i := range set.ExpectedStores {
		if set.ExpectedStores[i].StoreID == q.StoreID {
			population = &set.ExpectedStores[i]
			break
		}
	}
	if population == nil {
		return nil, ErrStoreNotFound
	}
	storeFacts := filterStore(set.Facts, q.StoreID)
	targetCurrency, currencyStatus := singleCurrency(storeFacts)
	currentFacts := filterPeriod(storeFacts, currentStart, currentEnd, targetCurrency)
	comparisonFacts := filterPeriod(storeFacts, comparisonStart, comparisonEnd, targetCurrency)
	currentAgg, currentCoverage := aggregateOne(currentFacts, currentStart, currentEnd, q.WindowDays, population)
	comparisonAgg, comparisonCoverage := aggregateOne(comparisonFacts, comparisonStart, comparisonEnd, q.WindowDays, population)
	if currencyStatus == "conflict" {
		currentAgg, comparisonAgg = nil, nil
	}
	summary := makeSummary(currentAgg, comparisonAgg)
	decisionReady := currentAgg != nil && comparisonAgg != nil && currentAgg.DecisionReady && comparisonAgg.DecisionReady && currencyStatus != "conflict"
	peerStores := eligiblePeers(set.ExpectedStores, *population, set.Facts, currentStart, currentEnd, targetCurrency)
	peers := make([]retailkpi.Aggregate, 0, len(peerStores))
	for _, peer := range peerStores {
		facts := filterPeriod(filterStore(set.Facts, peer.StoreID), currentStart, currentEnd, targetCurrency)
		if agg, _ := aggregateOne(facts, currentStart, currentEnd, q.WindowDays, &peer); agg != nil && agg.DecisionReady {
			peers = append(peers, *agg)
		}
	}
	benchmarks := buildBenchmarks(summary, peers)
	trend := buildDailyTrend(set.Facts, q.StoreID, peerStores, targetCurrency, currentStart, currentEnd)
	bridges := buildBridges(summary)
	quality := append([]string(nil), currentCoverage.MissingFields...)
	quality = append(quality, comparisonCoverage.MissingFields...)
	quality = uniqueSorted(quality)
	for _, benchmark := range benchmarks {
		if benchmark.Status == "insufficient_peers" {
			quality = appendUnique(quality, "insufficient_peer_count")
			break
		}
	}
	if currencyStatus == "conflict" {
		quality = append(quality, "currency_conflict")
	}
	if !decisionReady {
		quality = appendUnique(quality, "diagnostics_not_decision_ready")
	}
	if retailkpi.CoverageIncomplete(currentCoverage) || retailkpi.CoverageIncomplete(comparisonCoverage) {
		quality = appendUnique(quality, "incomplete_store_day_coverage")
		for i := range benchmarks {
			benchmarks[i].Status = "unavailable"
			benchmarks[i].Median, benchmarks[i].P25, benchmarks[i].P75, benchmarks[i].Percentile, benchmarks[i].TargetMinusMedian = nil, nil, nil, nil, nil
			benchmarks[i].Reason = "incomplete_store_day_coverage"
		}
	}
	if !decisionReady {
		bridgeReason := "diagnostics_not_decision_ready"
		if retailkpi.CoverageIncomplete(currentCoverage) || retailkpi.CoverageIncomplete(comparisonCoverage) {
			bridgeReason = "incomplete_store_day_coverage"
		}
		if currencyStatus == "conflict" {
			bridgeReason = "currency_conflict"
		}
		for i := range bridges {
			bridges[i] = unavailableBridge(bridges[i], bridgeReason)
		}
	}
	observations := buildObservations(summary, benchmarks, bridges, quality, decisionReady)
	linkQuery := q
	if linkQuery.SourceSystem == "" && len(set.SourceSystems) == 1 {
		linkQuery.SourceSystem = set.SourceSystems[0]
	}
	response := &Response{Basis: "Working", DiagnosticsVersion: DiagnosticsVersion, FormulaVersion: retailkpi.FormulaVersion, PulseVersion: retailpulse.PulseVersion, DataClassification: q.Classification, DatasetVersion: q.DatasetVersion, GeneratedAt: s.now(), Store: StoreIdentity{StoreID: population.StoreID, StoreCode: population.StoreCode, StoreName: population.StoreName, Brand: population.Brand, Region: population.Region}, Current: Period{currentStart.Format("2006-01-02"), currentEnd.Format("2006-01-02")}, Comparison: Period{comparisonStart.Format("2006-01-02"), comparisonEnd.Format("2006-01-02")}, TargetCoverage: currentCoverage, ComparisonCoverage: comparisonCoverage, DecisionReady: decisionReady, Currency: targetCurrency, CurrencyStatus: currencyStatus, Summary: summary, DailyTrend: trend, PeerDefinition: "same brand + region + currency, current decision-ready, excluding target", MinimumPeerCount: MinimumPeerCount, PeerBenchmark: benchmarks, Bridges: bridges, Observations: observations, KPIDrilldownURL: diagnosticKPIDrilldown(linkQuery, q.StoreID, currentStart, currentEnd)}
	response.DataQualityIssues = quality
	response.DecisionReadyReason = storeDecisionReadyReason(decisionReady, quality)
	// The envelope is the single provenance shape: sources, dataset
	// versions, fact version range, as-of and coverage for the store's own
	// facts. The response keeps its historical top-level fields, filled
	// from the envelope instead of a hand-rolled rollup.
	env := sourceenvelope.Build(storeFacts, sourceenvelope.Spec{
		Classification: q.Classification,
		FormulaVersion: retailkpi.FormulaVersion,
		PulseVersion:   retailpulse.PulseVersion,
		Current:        sourceenvelope.PeriodSpec{From: currentStart, To: currentEnd, ExpectedStoreDays: currentCoverage.ExpectedStoreDays},
		Comparison:     sourceenvelope.PeriodSpec{From: comparisonStart, To: comparisonEnd, ExpectedStoreDays: comparisonCoverage.ExpectedStoreDays},
		DecisionReady:  decisionReady, DecisionReadyReason: response.DecisionReadyReason,
		GeneratedAt: response.GeneratedAt,
	})
	response.Envelope = env
	response.SourceSystems = env.SourceSystems
	response.DatasetVersions = env.DatasetVersions
	response.FactVersionMin = env.FactVersionMin
	response.FactVersionMax = env.FactVersionMax
	response.HighestAsOf = env.HighestAsOf
	response.Evidence = Evidence{Current: response.Current, Comparison: response.Comparison, ObservedStoreDays: currentCoverage.ObservedStoreDays + comparisonCoverage.ObservedStoreDays, ExpectedStoreDays: currentCoverage.ExpectedStoreDays + comparisonCoverage.ExpectedStoreDays, RequiredFields: requiredFields(), FormulaVersion: retailkpi.FormulaVersion, SourceSystems: response.SourceSystems, DatasetVersions: response.DatasetVersions, FactVersionMin: env.FactVersionMin, FactVersionMax: env.FactVersionMax, HighestAsOf: response.HighestAsOf, DataQualityIssues: quality, KPIDrilldownURL: response.KPIDrilldownURL}
	return response, nil
}

func storeDecisionReadyReason(decisionReady bool, quality []string) string {
	if decisionReady {
		return ""
	}
	for _, issue := range quality {
		switch issue {
		case "incomplete_store_day_coverage", "currency_conflict", "insufficient_peer_count", "diagnostics_not_decision_ready", "data_quality_invalid":
			return issue
		}
	}
	return "not_decision_ready"
}

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
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
func filterPeriod(facts []retailkpi.DailyFact, from, to time.Time, currency string) []retailkpi.DailyFact {
	out := make([]retailkpi.DailyFact, 0)
	for _, f := range facts {
		if !f.BusinessDate.Before(from) && !f.BusinessDate.After(to) && (currency == "" || f.Currency == currency) {
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
	for v := range values {
		return v, "known"
	}
	return "", "unknown"
}
func aggregateOne(facts []retailkpi.DailyFact, from, to time.Time, days int, store *retailkpi.StorePopulation) (*retailkpi.Aggregate, retailkpi.Coverage) {
	rows, cov, _ := retailkpi.AggregateFacts(facts, retailkpi.Request{DateFrom: from, DateTo: to, RequestedDateFrom: from.Format("2006-01-02"), RequestedDateTo: to.Format("2006-01-02"), GroupBy: "store", ExpectedStoreCount: 1})
	if len(rows) == 0 {
		return nil, cov
	}
	return &rows[0], cov
}
func emptyKPI(code string) retailkpi.KPIValue {
	for _, d := range retailkpi.Definitions() {
		if d.Code == code {
			return retailkpi.KPIValue{Unit: d.Unit, Status: retailkpi.StatusUnavailable, FormulaVersion: retailkpi.FormulaVersion, RequiredFields: d.RequiredFields, Reason: "no_facts"}
		}
	}
	return retailkpi.KPIValue{Status: retailkpi.StatusUnavailable, Reason: "unavailable"}
}
func kpis(agg *retailkpi.Aggregate) map[string]retailkpi.KPIValue {
	out := map[string]retailkpi.KPIValue{}
	for _, code := range allCodes() {
		if agg == nil {
			out[code] = emptyKPI(code)
		} else {
			out[code] = agg.KPIs[code]
		}
	}
	return out
}
func allCodes() []string {
	out := make([]string, 0)
	for _, d := range retailkpi.Definitions() {
		out = append(out, d.Code)
	}
	return out
}
func makeSummary(current, comparison *retailkpi.Aggregate) map[string]SummaryMetric {
	out := map[string]SummaryMetric{}
	for _, code := range summaryCodes {
		c, p := kpis(current)[code], kpis(comparison)[code]
		var change *float64
		typ := "percent"
		if c.Unit == "percent" {
			typ = "percentage_point"
		}
		status := "complete"
		reason := ""
		if c.Status != retailkpi.StatusComplete || p.Status != retailkpi.StatusComplete {
			status = "partial"
			if c.Status == retailkpi.StatusUnavailable || p.Status == retailkpi.StatusUnavailable {
				status = "unavailable"
			}
			reason = firstReason(c.Reason, p.Reason)
		}
		if c.Value != nil && p.Value != nil {
			var changeReason string
			change, changeReason = retailkpi.ChangeRate(c.Value, p.Value, typ)
			if changeReason != "" {
				reason = changeReason
			}
		}
		out[code] = SummaryMetric{Current: c, Comparison: p, ChangeValue: change, ChangeType: typ, Status: status, Reason: reason}
	}
	return out
}
func firstReason(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
func eligiblePeers(pop []retailkpi.StorePopulation, target retailkpi.StorePopulation, facts []retailkpi.DailyFact, from, to time.Time, currency string) []retailkpi.StorePopulation {
	out := make([]retailkpi.StorePopulation, 0)
	for _, p := range pop {
		if p.StoreID == target.StoreID || p.Brand != target.Brand || p.Region != target.Region {
			continue
		}
		peerPeriodFacts := filterPeriod(filterStore(facts, p.StoreID), from, to, "")
		cur, _ := singleCurrency(peerPeriodFacts)
		if cur != currency {
			continue
		}
		agg, _ := aggregateOne(filterPeriod(filterStore(facts, p.StoreID), from, to, currency), from, to, int(to.Sub(from).Hours()/24)+1, &p)
		if agg != nil && agg.DecisionReady {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StoreCode != out[j].StoreCode {
			return out[i].StoreCode < out[j].StoreCode
		}
		return out[i].StoreID < out[j].StoreID
	})
	return out
}
func buildBenchmarks(summary map[string]SummaryMetric, peers []retailkpi.Aggregate) []PeerBenchmark {
	out := make([]PeerBenchmark, 0, len(benchmarkCodes))
	for _, code := range benchmarkCodes {
		values := make([]float64, 0, len(peers))
		for _, p := range peers {
			if v := p.KPIs[code].Value; v != nil && p.KPIs[code].Status == retailkpi.StatusComplete {
				values = append(values, *v)
			}
		}
		sort.Float64s(values)
		target := summary[code].Current.Value
		b := PeerBenchmark{Code: code, Unit: summary[code].Current.Unit, Target: target, PeerCount: len(values), Status: "complete"}
		if len(values) < MinimumPeerCount {
			b.Status = "insufficient_peers"
			b.Reason = "peer_count_below_minimum"
		} else if target == nil || summary[code].Current.Status != retailkpi.StatusComplete {
			b.Status = "unavailable"
			b.Reason = firstReason(summary[code].Current.Reason, "target_metric_unavailable")
		} else {
			m, p25, p75 := quantile(values, .5), quantile(values, .25), quantile(values, .75)
			b.Median = &m
			b.P25 = &p25
			b.P75 = &p75
			b.Percentile = retailkpi.PercentileRank(values, *target)
			d := *target - m
			b.TargetMinusMedian = &d
		}
		out = append(out, b)
	}
	return out
}
func quantile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	if len(values) == 1 {
		return values[0]
	}
	pos := (float64(len(values)) - 1) * p
	lo := int(math.Floor(pos))
	hi := int(math.Ceil(pos))
	if lo == hi {
		return values[lo]
	}
	return values[lo] + (values[hi]-values[lo])*(pos-float64(lo))
}
func buildDailyTrend(allFacts []retailkpi.DailyFact, targetID string, peers []retailkpi.StorePopulation, currency string, from, to time.Time) []DailyTrend {
	out := make([]DailyTrend, 0, int(to.Sub(from).Hours()/24)+1)
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		tf := filterPeriod(filterStore(allFacts, targetID), d, d, currency)
		ta, _ := aggregateOne(tf, d, d, 1, nil)
		row := DailyTrend{Date: d.Format("2006-01-02"), TargetKPIs: kpis(ta), PeerMedian: map[string]*float64{}, PeerCount: map[string]int{}}
		row.Gap = len(tf) == 0
		for _, code := range benchmarkCodes {
			vals := []float64{}
			for _, p := range peers {
				pf := filterPeriod(filterStore(allFacts, p.StoreID), d, d, currency)
				pa, _ := aggregateOne(pf, d, d, 1, &p)
				if pa != nil && pa.DecisionReady && pa.KPIs[code].Status == retailkpi.StatusComplete && pa.KPIs[code].Value != nil {
					vals = append(vals, *pa.KPIs[code].Value)
				}
			}
			sort.Float64s(vals)
			row.PeerCount[code] = len(vals)
			if len(vals) >= MinimumPeerCount {
				v := quantile(vals, .5)
				row.PeerMedian[code] = &v
			} else {
				row.PeerMedian[code] = nil
			}
		}
		out = append(out, row)
	}
	return out
}
func buildBridges(summary map[string]SummaryMetric) []Bridge {
	return []Bridge{revenueBridge(summary), grossProfitBridge(summary), contributionBridge(summary)}
}
func revenueBridge(s map[string]SummaryMetric) Bridge {
	b := Bridge{Code: "revenue", Method: "revenue_shapley_v1", Version: "retail-store-diagnostics-v1", Items: []BridgeItem{{Code: "footfall", Label: "客流", Unit: "currency"}, {Code: "conversion_rate", Label: "转化率", Unit: "currency"}, {Code: "average_transaction_value", Label: "客单价", Unit: "currency"}}}
	f := metricNumber(s, "footfall")
	c := metricNumber(s, "conversion_rate")
	a := metricNumber(s, "average_transaction_value")
	if !completeMetric(s, "revenue", "footfall", "conversion_rate", "average_transaction_value") {
		return unavailableBridge(b, "required_metric_unavailable")
	}
	return shapleyBridge(b, []float64{*f.comparison, *c.comparison / 100, *a.comparison}, []float64{*f.current, *c.current / 100, *a.current}, func(v []float64) float64 { return v[0] * v[1] * v[2] }, s["revenue"])
}

type metricPair struct{ current, comparison *float64 }

func metricNumber(s map[string]SummaryMetric, code string) metricPair {
	return metricPair{s[code].Current.Value, s[code].Comparison.Value}
}
func completeMetric(s map[string]SummaryMetric, codes ...string) bool {
	for _, c := range codes {
		if s[c].Current.Status != retailkpi.StatusComplete || s[c].Comparison.Status != retailkpi.StatusComplete || s[c].Current.Value == nil || s[c].Comparison.Value == nil {
			return false
		}
	}
	return true
}
func shapleyBridge(b Bridge, old, new []float64, formula func([]float64) float64, total SummaryMetric) Bridge {
	perms := [][]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}}
	con := make([]float64, 3)
	for _, p := range perms {
		state := append([]float64(nil), old...)
		base := formula(state)
		for _, i := range p {
			state[i] = new[i]
			next := formula(state)
			con[i] += next - base
			base = next
		}
	}
	for i := range con {
		con[i] /= float64(len(perms))
		b.Items[i].Contribution = &con[i]
	}
	return finalizeBridge(b, total)
}
func grossProfitBridge(s map[string]SummaryMetric) Bridge {
	b := Bridge{Code: "gross_profit", Method: "gross_profit_shapley_v1", Version: DiagnosticsVersion, Items: []BridgeItem{{Code: "revenue", Label: "销售额", Unit: "currency"}, {Code: "gross_margin_rate", Label: "毛利率", Unit: "currency"}}}
	if !completeMetric(s, "gross_profit", "revenue", "gross_margin_rate") {
		return unavailableBridge(b, "required_metric_unavailable")
	}
	return shapleyBridge2(b, []float64{*s["revenue"].Comparison.Value, *s["gross_margin_rate"].Comparison.Value / 100}, []float64{*s["revenue"].Current.Value, *s["gross_margin_rate"].Current.Value / 100}, func(v []float64) float64 { return v[0] * v[1] }, s["gross_profit"])
}
func shapleyBridge2(b Bridge, old, new []float64, formula func([]float64) float64, total SummaryMetric) Bridge {
	contributions := []float64{0, 0}
	for _, permutation := range [][]int{{0, 1}, {1, 0}} {
		state := append([]float64(nil), old...)
		base := formula(state)
		for _, index := range permutation {
			state[index] = new[index]
			next := formula(state)
			contributions[index] += next - base
			base = next
		}
	}
	for index := range contributions {
		contributions[index] /= 2
		b.Items[index].Contribution = &contributions[index]
	}
	return finalizeBridge(b, total)
}
func contributionBridge(s map[string]SummaryMetric) Bridge {
	b := Bridge{Code: "store_contribution", Method: "store_contribution_additive_v1", Version: DiagnosticsVersion, Items: []BridgeItem{{Code: "gross_profit", Label: "毛利额", Unit: "currency"}, {Code: "labor_cost", Label: "人工成本", Unit: "currency"}, {Code: "occupancy_cash_cost", Label: "经营占用现金成本", Unit: "currency"}, {Code: "other_controllable_cost", Label: "其他可控成本", Unit: "currency"}}}
	if !completeMetric(s, "store_contribution", "gross_profit", "labor_cost", "occupancy_cash_cost", "other_controllable_cost") {
		return unavailableBridge(b, "required_metric_unavailable")
	}
	vals := []float64{*s["gross_profit"].Current.Value - *s["gross_profit"].Comparison.Value, -(*s["labor_cost"].Current.Value - *s["labor_cost"].Comparison.Value), -(*s["occupancy_cash_cost"].Current.Value - *s["occupancy_cash_cost"].Comparison.Value), -(*s["other_controllable_cost"].Current.Value - *s["other_controllable_cost"].Comparison.Value)}
	for i := range vals {
		b.Items[i].Contribution = &vals[i]
	}
	return finalizeBridge(b, s["store_contribution"])
}
func finalizeBridge(b Bridge, total SummaryMetric) Bridge {
	b.Status = "complete"
	b.Current = total.Current.Value
	b.Comparison = total.Comparison.Value
	if b.Current != nil && b.Comparison != nil {
		d := *b.Current - *b.Comparison
		b.TotalChange = &d
		sum := 0.0
		for _, i := range b.Items {
			if i.Contribution != nil {
				sum += *i.Contribution
			}
		}
		r := d - sum
		b.RoundingResidual = &r
	}
	return b
}
func unavailableBridge(b Bridge, reason string) Bridge {
	b.Status = "unavailable"
	b.Reason = reason
	for i := range b.Items {
		b.Items[i].Contribution = nil
	}
	return b
}
func buildObservations(s map[string]SummaryMetric, b []PeerBenchmark, bridges []Bridge, quality []string, decisionReady bool) []Observation {
	type candidate struct {
		observation Observation
		magnitude   float64
	}
	candidates := make([]candidate, 0, len(quality)+len(s)+len(b))
	for _, q := range quality {
		candidates = append(candidates, candidate{observation: Observation{
			Code: q, Label: "数据状态", Statement: "当前数据不足，建议先核实事实覆盖与字段质量。",
			Reference: "evidence", Status: "unavailable", EvidenceIDs: []string{"evidence"},
		}})
	}
	if !decisionReady {
		out := make([]Observation, 0, len(candidates))
		for _, item := range candidates {
			out = append(out, item.observation)
		}
		return out
	}
	labels := map[string]string{
		"revenue": "销售额", "gross_profit": "毛利额", "footfall": "客流", "conversion_rate": "转化率",
		"average_transaction_value": "客单价", "labor_cost": "人工成本", "occupancy_cash_cost": "经营占用现金成本",
		"other_controllable_cost": "其他可控成本", "labor_cost_rate": "人工成本率", "occupancy_cash_cost_rate": "经营占用成本率",
		"store_contribution": "门店经营利润", "store_contribution_margin": "门店经营利润率", "sales_per_sqm": "期间坪效",
	}
	for code, metric := range s {
		if metric.ChangeValue == nil {
			continue
		}
		unit := "%"
		if metric.ChangeType == "percentage_point" {
			unit = "pp"
		}
		candidates = append(candidates, candidate{observation: Observation{
			Code: code, Label: labels[code], Statement: fmt.Sprintf("%s本期较对比期变化 %.2f%s；这是期间差异的事实描述。", labels[code], *metric.ChangeValue, unit),
			Reference: "summary:" + code, Status: metric.Status, EvidenceIDs: []string{"evidence", "summary:" + code},
		}, magnitude: math.Abs(*metric.ChangeValue)})
	}
	for _, benchmark := range b {
		if benchmark.Status != "complete" || benchmark.TargetMinusMedian == nil {
			continue
		}
		label := labels[benchmark.Code]
		candidates = append(candidates, candidate{observation: Observation{
			Code: benchmark.Code, Label: label, Statement: fmt.Sprintf("%s目标值与同群中位数差异 %.2f；这是同群位置的观察信号。", label, *benchmark.TargetMinusMedian),
			Reference: "benchmark:" + benchmark.Code, Status: "complete", EvidenceIDs: []string{"evidence", "benchmark:" + benchmark.Code},
		}, magnitude: math.Abs(*benchmark.TargetMinusMedian)})
	}
	for _, br := range bridges {
		for _, item := range br.Items {
			if br.Status != "complete" || item.Contribution == nil || math.Abs(*item.Contribution) <= 0.01 {
				continue
			}
			candidates = append(candidates, candidate{observation: Observation{
				Code: item.Code, Label: item.Label,
				Statement: fmt.Sprintf("%s变化贡献为 %.2f；该结果仅是观察信号，不作解释性判断。", item.Label, *item.Contribution),
				Reference: br.Code, Status: "complete",
				EvidenceIDs: []string{"evidence", "bridge:" + br.Code},
			}, magnitude: math.Abs(*item.Contribution)})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		missingI := candidates[i].observation.Status == "unavailable"
		missingJ := candidates[j].observation.Status == "unavailable"
		if missingI != missingJ {
			return missingI
		}
		if candidates[i].magnitude != candidates[j].magnitude {
			return candidates[i].magnitude > candidates[j].magnitude
		}
		if candidates[i].observation.Code != candidates[j].observation.Code {
			return candidates[i].observation.Code < candidates[j].observation.Code
		}
		return candidates[i].observation.Reference < candidates[j].observation.Reference
	})
	out := make([]Observation, 0, len(candidates))
	for _, item := range candidates {
		out = append(out, item.observation)
	}
	return out
}
func appendUnique(xs []string, v string) []string {
	for _, x := range xs {
		if x == v {
			return xs
		}
	}
	return append(xs, v)
}
func uniqueSorted(xs []string) []string {
	m := map[string]bool{}
	for _, x := range xs {
		if x != "" {
			m[x] = true
		}
	}
	out := make([]string, 0, len(m))
	for x := range m {
		out = append(out, x)
	}
	sort.Strings(out)
	return out
}
func requiredFields() []string {
	return []string{"revenue", "gross_profit", "transactions", "footfall", "area_sqm", "labor_cost", "fixed_rent", "variable_rent", "non_lease_cost", "other_controllable_cost"}
}
func diagnosticKPIDrilldown(q Query, store string, from, to time.Time) string {
	parts := []string{"group_by=store", "store_id=" + store, "date_from=" + from.Format("2006-01-02"), "date_to=" + to.Format("2006-01-02"), "data_classification=" + q.Classification}
	if q.DatasetVersion != "" {
		parts = append(parts, "dataset_version="+q.DatasetVersion)
	}
	if q.SourceSystem != "" {
		parts = append(parts, "source_system="+q.SourceSystem)
	}
	return "/api/v1/retail/kpis/store-days?" + strings.Join(parts, "&")
}
