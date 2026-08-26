package retailpulse

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailcohort"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/services/retailperiod"
	"github.com/lease-management-system/core-service/internal/services/sourceenvelope"
)

const (
	PulseVersion          = "retail-pulse-v1"
	DefaultWindowDays     = retailperiod.DefaultRollingDays
	DefaultAttentionLimit = 10
	UnknownCurrencyStatus = "unknown"
)

type FactReader interface {
	QueryFacts(context.Context, string, string, string, string, string, string, []string) (*repository.RetailKPIFactSet, error)
}

type StoreLifecycleReader interface {
	ListStoreLifecycles(context.Context, string, string, string, []string) ([]retailcohort.StoreLifecycle, error)
}

type Query struct {
	LegalEntityID  string
	AsOf           time.Time
	WindowDays     int
	Classification string
	DatasetVersion string
	SourceSystem   string
	StoreIDs       []string
	AttentionLimit int
	// M4: attach the actual-vs-plan comparison for the calendar month of
	// the current window end; the threshold comes from system settings via
	// the handler (0 = service default of 5%).
	PlanComparison              bool
	PlanMaterialityThresholdPct float64
	// M5: group the attention ranking — "" and "total" keep the per-store
	// ranking (zero regression); "store" labels it; "region"/"brand" rank
	// groups built from the same facts and signal rules.
	GroupBy string
	// M2 calendar periods: when the handler resolves a period like 2026-07
	// through retailperiod it passes the four boundaries here; they override
	// the rolling derivation below. WindowDays then carries the current
	// window's day count for sizing, never for derivation.
	DateFrom           time.Time
	DateTo             time.Time
	ComparisonDateFrom time.Time
	ComparisonDateTo   time.Time
	PeriodLabel        string
}

type Period struct {
	DateFrom string `json:"date_from"`
	DateTo   string `json:"date_to"`
}

type SummaryMetric struct {
	Current        retailkpi.KPIValue `json:"current"`
	Comparison     retailkpi.KPIValue `json:"comparison"`
	ChangeValue    *float64           `json:"change_value"`
	ChangeType     string             `json:"change_type"`
	ChangeMarginPP *float64           `json:"change_margin_pp,omitempty"`
	Status         string             `json:"status"`
	Reason         string             `json:"reason,omitempty"`
}

type DailyTrend struct {
	Date           string                        `json:"date"`
	Currency       string                        `json:"currency"`
	CurrencyStatus string                        `json:"currency_status,omitempty"`
	Gap            bool                          `json:"gap"`
	Coverage       retailkpi.Coverage            `json:"coverage"`
	KPIs           map[string]retailkpi.KPIValue `json:"kpis,omitempty"`
}

type Signal struct {
	SignalCode        string   `json:"signal_code"`
	ObservedChange    *float64 `json:"observed_change"`
	Threshold         float64  `json:"threshold"`
	Direction         string   `json:"direction"`
	Current           *float64 `json:"current"`
	Comparison        *float64 `json:"comparison"`
	Unit              string   `json:"unit"`
	ScoreContribution float64  `json:"score_contribution"`
}

type Evidence struct {
	Current             Period   `json:"current"`
	Comparison          Period   `json:"comparison"`
	CurrentFactCount    int      `json:"current_fact_count"`
	ComparisonFactCount int      `json:"comparison_fact_count"`
	SourceSystems       []string `json:"source_systems"`
	DatasetVersions     []string `json:"dataset_versions"`
	FormulaVersion      string   `json:"formula_version"`
	PulseVersion        string   `json:"pulse_version"`
}

type Attention struct {
	Rank            int                           `json:"rank"`
	GroupBy         string                        `json:"group_by,omitempty"`
	GroupKey        string                        `json:"group_key,omitempty"`
	GroupLabel      string                        `json:"group_label,omitempty"`
	StoreID         string                        `json:"store_id"`
	StoreCode       string                        `json:"store_code"`
	StoreName       string                        `json:"store_name"`
	Brand           string                        `json:"brand"`
	Region          string                        `json:"region"`
	StoreFormat     string                        `json:"store_format,omitempty"`
	LifecycleStatus string                        `json:"lifecycle_status,omitempty"`
	Currency        string                        `json:"currency"`
	CurrencyStatus  string                        `json:"currency_status,omitempty"`
	Score           float64                       `json:"score"`
	Severity        string                        `json:"severity"`
	ObservedSignals []Signal                      `json:"observed_signals"`
	CurrentKPIs     map[string]retailkpi.KPIValue `json:"current_kpis"`
	ComparisonKPIs  map[string]retailkpi.KPIValue `json:"comparison_kpis"`
	Evidence        Evidence                      `json:"evidence"`
	Drilldown       map[string]string             `json:"drilldown"`
}

type SuppressedAttention struct {
	GroupBy            string             `json:"group_by,omitempty"`
	GroupKey           string             `json:"group_key,omitempty"`
	GroupLabel         string             `json:"group_label,omitempty"`
	StoreID            string             `json:"store_id"`
	StoreCode          string             `json:"store_code"`
	StoreName          string             `json:"store_name"`
	Brand              string             `json:"brand"`
	Region             string             `json:"region"`
	Currency           string             `json:"currency"`
	CurrencyStatus     string             `json:"currency_status,omitempty"`
	Reason             string             `json:"reason"`
	Reasons            []string           `json:"reasons,omitempty"`
	CurrentCoverage    retailkpi.Coverage `json:"current_coverage"`
	ComparisonCoverage retailkpi.Coverage `json:"comparison_coverage"`
}

type Partition struct {
	Currency            string                   `json:"currency"`
	CurrencyStatus      string                   `json:"currency_status,omitempty"`
	Current             Period                   `json:"current"`
	Comparison          Period                   `json:"comparison"`
	CurrentCoverage     retailkpi.Coverage       `json:"current_coverage"`
	ComparisonCoverage  retailkpi.Coverage       `json:"comparison_coverage"`
	DecisionReady       bool                     `json:"decision_ready"`
	Summary             map[string]SummaryMetric `json:"summary"`
	SSSG                *retailcohort.SSSGResult `json:"sssg,omitempty"`
	DailyTrend          []DailyTrend             `json:"daily_trend"`
	Attention           []Attention              `json:"attention"`
	SuppressedAttention []SuppressedAttention    `json:"suppressed_attention,omitempty"`
	AttentionCount      int                      `json:"attention_count"`
}

type Response struct {
	Basis                     string                      `json:"basis"`
	PulseVersion              string                      `json:"pulse_version"`
	FormulaVersion            string                      `json:"formula_version"`
	DataClassification        string                      `json:"data_classification"`
	DatasetVersion            string                      `json:"dataset_version,omitempty"`
	SimulationDatasetVersions []string                    `json:"simulation_dataset_versions,omitempty"`
	RequestedScope            map[string]any              `json:"requested_scope"`
	RequestedStores           []retailkpi.StorePopulation `json:"requested_stores,omitempty"`
	SourceSystems             []string                    `json:"source_systems"`
	FactVersionMin            int                         `json:"fact_version_min"`
	FactVersionMax            int                         `json:"fact_version_max"`
	HighestAsOf               *time.Time                  `json:"highest_as_of,omitempty"`
	MultiCurrency             bool                        `json:"multi_currency"`
	MixedCurrencyStores       int                         `json:"mixed_currency_stores,omitempty"`
	PeriodLabel               string                      `json:"period_label,omitempty"`
	Plan                      *retailkpi.PlanComparison   `json:"plan,omitempty"`
	GroupBy                   string                      `json:"group_by,omitempty"`
	Currency                  string                      `json:"currency,omitempty"`
	CurrencyStatus            string                      `json:"currency_status,omitempty"`
	Current                   Period                      `json:"current"`
	Comparison                Period                      `json:"comparison"`
	CurrentCoverage           retailkpi.Coverage          `json:"current_coverage"`
	ComparisonCoverage        retailkpi.Coverage          `json:"comparison_coverage"`
	DecisionReady             bool                        `json:"decision_ready"`
	DecisionReadyReason       string                      `json:"decision_ready_reason,omitempty"`
	Envelope                  sourceenvelope.Envelope     `json:"envelope"`
	Summary                   map[string]SummaryMetric    `json:"summary,omitempty"`
	SSSG                      *retailcohort.SSSGResult    `json:"sssg,omitempty"`
	DailyTrend                []DailyTrend                `json:"daily_trend,omitempty"`
	Attention                 []Attention                 `json:"attention,omitempty"`
	SuppressedAttention       []SuppressedAttention       `json:"suppressed_attention,omitempty"`
	AttentionCount            int                         `json:"attention_count"`
	Partitions                []Partition                 `json:"partitions,omitempty"`
	GeneratedAt               time.Time                   `json:"generated_at"`
	DefinitionsURL            string                      `json:"definitions_url"`
	KPIDrilldownURL           string                      `json:"kpi_drilldown_url"`
	StoreDrilldownURL         string                      `json:"store_drilldown_url"`
	CurrentKPIDrilldownURL    string                      `json:"current_kpi_drilldown_url"`
	ComparisonKPIDrilldownURL string                      `json:"comparison_kpi_drilldown_url"`
}

// WithPlanReader enables the M4 plan comparison block on Build responses;
// a nil reader keeps the response unchanged (zero regression).
func (s *Service) WithPlanReader(reader retailkpi.PlanReader) *Service {
	s.planReader = reader
	return s
}

type Service struct {
	reader     FactReader
	now        func() time.Time
	planReader retailkpi.PlanReader
}

func NewService(reader FactReader) *Service { return &Service{reader: reader, now: time.Now} }

func (s *Service) Build(ctx context.Context, query Query) (*Response, error) {
	if s.reader == nil {
		return nil, fmt.Errorf("retail pulse fact reader is required")
	}
	if query.LegalEntityID == "" {
		return nil, fmt.Errorf("legal entity scope is required")
	}
	if query.DateTo.IsZero() != query.DateFrom.IsZero() || query.ComparisonDateFrom.IsZero() != query.ComparisonDateTo.IsZero() || query.DateTo.IsZero() != query.ComparisonDateTo.IsZero() {
		return nil, fmt.Errorf("explicit period boundaries must be complete (from/to for current and comparison)")
	}
	if query.WindowDays == 0 && query.DateTo.IsZero() {
		query.WindowDays = DefaultWindowDays
	}
	if query.DateTo.IsZero() {
		if _, err := retailperiod.ParseRollingDays(query.WindowDays); err != nil {
			return nil, fmt.Errorf("window_days must be between 7 and 28")
		}
	}
	if query.AsOf.IsZero() {
		return nil, fmt.Errorf("as_of is required")
	}
	if query.Classification != "production" && query.Classification != "simulated" {
		return nil, fmt.Errorf("data_classification must be production or simulated")
	}
	if query.Classification == "simulated" && query.DatasetVersion == "" {
		return nil, fmt.Errorf("dataset version is required for simulated data")
	}
	if query.Classification == "production" && query.DatasetVersion != "" {
		return nil, fmt.Errorf("dataset version is not allowed for production data")
	}
	switch query.GroupBy {
	case "", "total", "region", "brand", "store":
	default:
		return nil, fmt.Errorf("group_by must be one of total, region, brand, store")
	}
	if query.AttentionLimit == 0 {
		query.AttentionLimit = DefaultAttentionLimit
	}
	if query.AttentionLimit < 1 || query.AttentionLimit > 50 {
		return nil, fmt.Errorf("attention_limit must be 1-50")
	}
	currentEnd := dateOnly(query.AsOf)
	currentStart := currentEnd.AddDate(0, 0, -(query.WindowDays - 1))
	comparisonEnd := currentStart.AddDate(0, 0, -1)
	comparisonStart := comparisonEnd.AddDate(0, 0, -(query.WindowDays - 1))
	periodLabel := fmt.Sprintf("近 %d 天", query.WindowDays)
	if !query.DateTo.IsZero() {
		// Calendar period: the handler resolved the boundaries through
		// retailperiod (previous calendar month/quarter as comparison).
		currentStart, currentEnd = dateOnly(query.DateFrom), dateOnly(query.DateTo)
		comparisonStart, comparisonEnd = dateOnly(query.ComparisonDateFrom), dateOnly(query.ComparisonDateTo)
		query.WindowDays = inclusiveDays(currentStart, currentEnd)
		periodLabel = query.PeriodLabel
	}
	dateFrom, dateTo := comparisonStart.Format("2006-01-02"), currentEnd.Format("2006-01-02")
	set, err := s.reader.QueryFacts(ctx, query.LegalEntityID, dateFrom, dateTo, query.Classification, query.DatasetVersion, query.SourceSystem, query.StoreIDs)
	if err != nil {
		return nil, err
	}
	if set == nil {
		return nil, fmt.Errorf("retail pulse fact reader returned no fact set")
	}
	linkQuery := query
	if linkQuery.SourceSystem == "" && len(set.SourceSystems) == 1 {
		linkQuery.SourceSystem = set.SourceSystems[0]
	}
	var lifecycles []retailcohort.StoreLifecycle
	if lr, ok := s.reader.(StoreLifecycleReader); ok {
		lifecycles, _ = lr.ListStoreLifecycles(ctx, query.LegalEntityID, query.Classification, query.DatasetVersion, query.StoreIDs)
	}
	partitions := s.buildPartitions(set, linkQuery, currentStart, currentEnd, comparisonStart, comparisonEnd, lifecycles)
	response := &Response{
		Basis: "Working", PulseVersion: PulseVersion, FormulaVersion: retailkpi.FormulaVersion,
		DataClassification: query.Classification, DatasetVersion: query.DatasetVersion,
		SimulationDatasetVersions: set.DatasetVersions,
		RequestedScope:            map[string]any{"legal_entity_id": query.LegalEntityID, "store_ids": query.StoreIDs},
		RequestedStores:           set.ExpectedStores,
		MultiCurrency:             distinctCurrencies(set.Facts) > 1,
		MixedCurrencyStores:       mixedCurrencyStores(set.Facts),
		Current:                   Period{DateFrom: currentStart.Format("2006-01-02"), DateTo: currentEnd.Format("2006-01-02")},
		Comparison:                Period{DateFrom: comparisonStart.Format("2006-01-02"), DateTo: comparisonEnd.Format("2006-01-02")},
		PeriodLabel:               periodLabel,
		GroupBy:                   query.GroupBy,
		GeneratedAt:               s.now(), DefinitionsURL: "/api/v1/retail/kpis/definitions",
		KPIDrilldownURL:           drilldownTemplate(linkQuery, "{group_by}", "{store_id}", "{date_from}", "{date_to}"),
		StoreDrilldownURL:         drilldownTemplate(linkQuery, "store", "{store_id}", "{date_from}", "{date_to}"),
		CurrentKPIDrilldownURL:    drilldownURL(linkQuery, "total", "", currentStart, currentEnd),
		ComparisonKPIDrilldownURL: drilldownURL(linkQuery, "total", "", comparisonStart, comparisonEnd),
	}
	if len(partitions) == 1 {
		p := partitions[0]
		response.Currency, response.CurrencyStatus = p.Currency, p.CurrencyStatus
		response.CurrentCoverage, response.ComparisonCoverage, response.DecisionReady, response.Summary, response.SSSG, response.DailyTrend, response.Attention, response.SuppressedAttention, response.AttentionCount = p.CurrentCoverage, p.ComparisonCoverage, p.DecisionReady, p.Summary, p.SSSG, p.DailyTrend, p.Attention, p.SuppressedAttention, p.AttentionCount
		response.Current, response.Comparison = p.Current, p.Comparison
	} else {
		response.Partitions = partitions
		response.DecisionReady = len(partitions) > 0
		for _, p := range partitions {
			response.DecisionReady = response.DecisionReady && p.DecisionReady
			response.AttentionCount += p.AttentionCount
		}
	}
	response.DecisionReadyReason = pulseDecisionReadyReason(response)
	expectedStores := set.ExpectedStoreCount
	if expectedStores == 0 && len(set.ExpectedStores) > 0 {
		expectedStores = len(set.ExpectedStores)
	}
	env := sourceenvelope.Build(set.Facts, sourceenvelope.Spec{
		Classification: query.Classification,
		FormulaVersion: retailkpi.FormulaVersion,
		PulseVersion:   PulseVersion,
		Current: sourceenvelope.PeriodSpec{From: currentStart, To: currentEnd,
			ExpectedStoreDays: expectedStores * inclusiveDays(currentStart, currentEnd)},
		Comparison: sourceenvelope.PeriodSpec{From: comparisonStart, To: comparisonEnd,
			ExpectedStoreDays: expectedStores * inclusiveDays(comparisonStart, comparisonEnd)},
		DecisionReady:       response.DecisionReady,
		DecisionReadyReason: response.DecisionReadyReason,
		GeneratedAt:         response.GeneratedAt,
	})
	response.Envelope = env
	response.SourceSystems = env.SourceSystems
	response.FactVersionMin = env.FactVersionMin
	response.FactVersionMax = env.FactVersionMax
	response.HighestAsOf = env.HighestAsOf
	if s.planReader != nil && query.PlanComparison {
		if err := s.attachPlanComparison(ctx, query, set, expectedStores, response); err != nil {
			return nil, err
		}
	}
	return response, nil
}

// attachPlanComparison pairs the current window's facts with the plan basis
// of its calendar month (M4); no version covering the month means no block —
// the absence is honest, not a zero.
func (s *Service) attachPlanComparison(ctx context.Context, query Query, set *repository.RetailKPIFactSet, expectedStores int, response *Response) error {
	planPeriod := currentEndOf(response).Format("2006-01")
	planSet, err := s.planReader.ReadPlan(ctx, query.LegalEntityID, planPeriod)
	if err != nil {
		return err
	}
	if planSet == nil {
		return nil
	}
	monthWindow, periodErr := retailperiod.Parse(planPeriod, time.Time{})
	if periodErr != nil {
		return periodErr
	}
	comparison, err := retailkpi.ComparePlan(set.Facts, planSet.Facts, retailkpi.ComparePlanRequest{
		Period: planPeriod, ExpectedStoreCount: expectedStores,
		ExpectedDaysInMonth:     inclusiveDays(monthWindow.From, monthWindow.To),
		MaterialityThresholdPct: query.PlanMaterialityThresholdPct,
	})
	if err != nil {
		return fmt.Errorf("plan comparison: %w", err)
	}
	comparison.PlanVersionID = planSet.VersionID
	comparison.PlanVersionName = planSet.VersionName
	comparison.PlanVersionType = planSet.VersionType
	comparison.PlanAsOfPeriod = planSet.AsOfPeriod
	comparison.PlanSource = planSet.Source
	comparison.PlanIsOfficial = planSet.IsOfficial
	response.Plan = comparison
	return nil
}

func currentEndOf(response *Response) time.Time {
	parsed, err := time.Parse("2006-01-02", response.Current.DateTo)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

// pulseDecisionReadyReason names the first reason a pulse read is not
// decision-ready; an empty reason means it is ready.
func pulseDecisionReadyReason(response *Response) string {
	if response.DecisionReady {
		return ""
	}
	for _, coverage := range []retailkpi.Coverage{response.CurrentCoverage, response.ComparisonCoverage} {
		if coverage.ExpectedStoreDays > 0 && coverage.ObservedStoreDays < coverage.ExpectedStoreDays {
			return "incomplete_store_day_coverage"
		}
	}
	return "not_decision_ready"
}

func drilldownURL(query Query, groupBy, storeID string, from, to time.Time) string {
	dateFrom, dateTo := from.Format("2006-01-02"), to.Format("2006-01-02")
	if groupBy == "total" && storeID == "" && len(query.StoreIDs) > 0 {
		return drilldownURLWithStores(query, groupBy, query.StoreIDs, dateFrom, dateTo)
	}
	return drilldownTemplate(query, groupBy, storeID, dateFrom, dateTo)
}

func drilldownTemplate(query Query, groupBy, storeID, dateFrom, dateTo string) string {
	return drilldownURLWithStores(query, groupBy, []string{storeID}, dateFrom, dateTo)
}

func drilldownURLWithStores(query Query, groupBy string, storeIDs []string, dateFrom, dateTo string) string {
	params := []string{
		"data_classification=" + url.QueryEscape(query.Classification),
		"source_system=" + url.QueryEscape(query.SourceSystem),
		"date_from=" + dateFrom,
		"date_to=" + dateTo,
		"group_by=" + groupBy,
	}
	for _, storeID := range storeIDs {
		params = append(params, "store_id="+templateValue(storeID))
	}
	if query.Classification == "simulated" {
		params = append(params, "simulation_dataset_version="+url.QueryEscape(query.DatasetVersion))
	}
	return "/api/v1/retail/kpis/store-days?" + strings.Join(params, "&")
}

func templateValue(value string) string {
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		return value
	}
	return url.QueryEscape(value)
}

func (s *Service) buildPartitions(set *repository.RetailKPIFactSet, query Query, currentStart, currentEnd, comparisonStart, comparisonEnd time.Time, lifecycles []retailcohort.StoreLifecycle) []Partition {
	// P0-7: population attribution derives from each store's full currency
	// set, never from fact iteration order. A store whose facts span
	// currencies stays in every currency partition where it has facts and
	// counts toward each partition's expected population — instead of
	// landing in whichever currency happened to be written last.
	storeCurrencies := map[string]map[string]bool{}
	for _, fact := range set.Facts {
		if storeCurrencies[fact.StoreID] == nil {
			storeCurrencies[fact.StoreID] = map[string]bool{}
		}
		storeCurrencies[fact.StoreID][fact.Currency] = true
	}
	byCurrency := map[string][]retailkpi.DailyFact{}
	for _, fact := range set.Facts {
		byCurrency[fact.Currency] = append(byCurrency[fact.Currency], fact)
	}
	currencies := make([]string, 0, len(byCurrency))
	for currency := range byCurrency {
		currencies = append(currencies, currency)
	}
	sort.Strings(currencies)
	populationByCurrency := map[string]int{}
	observedStoreCurrency := map[string]bool{}
	for storeID, currencySet := range storeCurrencies {
		observedStoreCurrency[storeID] = true
		for currency := range currencySet {
			populationByCurrency[currency]++
		}
	}
	unobserved := make([]retailkpi.StorePopulation, 0)
	for _, store := range set.ExpectedStores {
		if !observedStoreCurrency[store.StoreID] {
			unobserved = append(unobserved, store)
		}
	}
	result := make([]Partition, 0, len(currencies))
	for _, currency := range currencies {
		facts := byCurrency[currency]
		expected := populationByCurrency[currency]
		if len(currencies) == 1 {
			expected = set.ExpectedStoreCount
			if expected == 0 && len(set.ExpectedStores) > 0 {
				expected = len(set.ExpectedStores)
			}
		}
		if expected == 0 {
			expected = distinctStores(facts)
		}
		currentFacts := between(facts, currentStart, currentEnd)
		comparisonFacts := between(facts, comparisonStart, comparisonEnd)
		currentAgg, currentCoverage := totalAggregate(currentFacts, currentStart, currentEnd, expected)
		comparisonAgg, comparisonCoverage := totalAggregate(comparisonFacts, comparisonStart, comparisonEnd, expected)
		partition := Partition{Currency: currency, Current: Period{DateFrom: currentStart.Format("2006-01-02"), DateTo: currentEnd.Format("2006-01-02")}, Comparison: Period{DateFrom: comparisonStart.Format("2006-01-02"), DateTo: comparisonEnd.Format("2006-01-02")}, CurrentCoverage: currentCoverage, ComparisonCoverage: comparisonCoverage, DecisionReady: currentAgg != nil && comparisonAgg != nil && currentAgg.DecisionReady && comparisonAgg.DecisionReady}
		if currentAgg != nil || comparisonAgg != nil {
			partition.Summary = buildSummary(currentAgg, comparisonAgg)
		}
		if len(lifecycles) > 0 {
			cohort := retailcohort.EvaluateComparableCohort(lifecycles, retailcohort.PeriodPair{
				CurrentStart: currentStart, CurrentEnd: currentEnd,
				BaselineStart: comparisonStart, BaselineEnd: comparisonEnd,
			}, retailcohort.DefaultPolicy())
			sssgRes := retailcohort.CalculateSSSG(currentFacts, comparisonFacts, cohort, retailkpi.Request{DateFrom: currentStart, DateTo: currentEnd, RequestedDateFrom: currentStart.Format("2006-01-02"), RequestedDateTo: currentEnd.Format("2006-01-02")}, retailkpi.Request{DateFrom: comparisonStart, DateTo: comparisonEnd, RequestedDateFrom: comparisonStart.Format("2006-01-02"), RequestedDateTo: comparisonEnd.Format("2006-01-02")})
			partition.SSSG = &sssgRes
		}
		partition.Attention, partition.SuppressedAttention = buildAttention(facts, currency, query, query.AttentionLimit, currentStart, currentEnd, comparisonStart, comparisonEnd, set, lifecycles)
		partition.DailyTrend = buildTrend(facts, currency, currentStart, currentEnd, expected)
		partition.AttentionCount = len(partition.Attention)
		result = append(result, partition)
	}
	if len(unobserved) > 0 {
		sort.SliceStable(unobserved, func(i, j int) bool {
			if unobserved[i].StoreCode != unobserved[j].StoreCode {
				return unobserved[i].StoreCode < unobserved[j].StoreCode
			}
			return unobserved[i].StoreID < unobserved[j].StoreID
		})
		expected := len(unobserved)
		currentCoverage := emptyCoverage(currentStart, currentEnd, expected)
		comparisonCoverage := emptyCoverage(comparisonStart, comparisonEnd, expected)
		partition := Partition{Currency: "", CurrencyStatus: UnknownCurrencyStatus, Current: Period{DateFrom: currentStart.Format("2006-01-02"), DateTo: currentEnd.Format("2006-01-02")}, Comparison: Period{DateFrom: comparisonStart.Format("2006-01-02"), DateTo: comparisonEnd.Format("2006-01-02")}, CurrentCoverage: currentCoverage, ComparisonCoverage: comparisonCoverage, DecisionReady: false, DailyTrend: buildTrend(nil, "", currentStart, currentEnd, expected)}
		for _, store := range unobserved {
			partition.SuppressedAttention = append(partition.SuppressedAttention, SuppressedAttention{StoreID: store.StoreID, StoreCode: store.StoreCode, StoreName: store.StoreName, Brand: store.Brand, Region: store.Region, Currency: "", CurrencyStatus: UnknownCurrencyStatus, Reason: "no_facts_in_requested_range", Reasons: []string{"no_facts_in_requested_range"}, CurrentCoverage: currentCoverage, ComparisonCoverage: comparisonCoverage})
		}
		partition.AttentionCount = 0
		if len(result) == 1 {
			result[0].SuppressedAttention = append(result[0].SuppressedAttention, partition.SuppressedAttention...)
			sort.SliceStable(result[0].SuppressedAttention, func(i, j int) bool {
				if result[0].SuppressedAttention[i].StoreCode != result[0].SuppressedAttention[j].StoreCode {
					return result[0].SuppressedAttention[i].StoreCode < result[0].SuppressedAttention[j].StoreCode
				}
				return result[0].SuppressedAttention[i].StoreID < result[0].SuppressedAttention[j].StoreID
			})
			result[0].DecisionReady = false
		} else {
			result = append(result, partition)
		}
	}
	// MAX-003 now supplies ExpectedStores, but retain an explicit partition-level
	// contract for older readers that only return an expected count. This keeps
	// a fully empty authorized population visible instead of dropping it with
	// the facts map; store-level identity is intentionally unavailable there.
	if len(result) == 0 {
		expected := set.ExpectedStoreCount
		if expected == 0 && len(set.ExpectedStores) > 0 {
			expected = len(set.ExpectedStores)
		}
		currentCoverage := emptyCoverage(currentStart, currentEnd, expected)
		comparisonCoverage := emptyCoverage(comparisonStart, comparisonEnd, expected)
		var suppressed []SuppressedAttention
		if expected > 0 {
			suppressed = []SuppressedAttention{{Currency: "", CurrencyStatus: UnknownCurrencyStatus, Reason: "no_facts_in_requested_range", Reasons: []string{"no_facts_in_requested_range"}, CurrentCoverage: currentCoverage, ComparisonCoverage: comparisonCoverage}}
		}
		result = append(result, Partition{
			Currency: "", CurrencyStatus: UnknownCurrencyStatus, Current: Period{DateFrom: currentStart.Format("2006-01-02"), DateTo: currentEnd.Format("2006-01-02")},
			Comparison:      Period{DateFrom: comparisonStart.Format("2006-01-02"), DateTo: comparisonEnd.Format("2006-01-02")},
			CurrentCoverage: currentCoverage, ComparisonCoverage: comparisonCoverage, DecisionReady: false,
			DailyTrend:          buildTrend(nil, "", currentStart, currentEnd, expected),
			SuppressedAttention: suppressed,
		})
	}
	return result
}

func emptyCoverage(from, to time.Time, expected int) retailkpi.Coverage {
	// CoverageRate stays nil when expected store-days are unknown — a missing
	// signal is never zero-filled (AGENTS.md: 不用 0 填补缺失).
	return retailkpi.Coverage{RequestedDateFrom: from.Format("2006-01-02"), RequestedDateTo: to.Format("2006-01-02"), ObservedStoreDays: 0, ExpectedStoreDays: expected * inclusiveDays(from, to)}
}

func totalAggregate(facts []retailkpi.DailyFact, from, to time.Time, expected int) (*retailkpi.Aggregate, retailkpi.Coverage) {
	rows, coverage, _ := retailkpi.AggregateFacts(facts, retailkpi.Request{DateFrom: from, DateTo: to, RequestedDateFrom: from.Format("2006-01-02"), RequestedDateTo: to.Format("2006-01-02"), GroupBy: "total", ExpectedStoreCount: expected})
	if len(rows) == 0 {
		return nil, coverage
	}
	return &rows[0], coverage
}

func buildSummary(current, comparison *retailkpi.Aggregate) map[string]SummaryMetric {
	result := map[string]SummaryMetric{}
	for _, code := range []string{"revenue", "gross_profit", "gross_margin_rate", "footfall", "transactions", "conversion_rate", "average_transaction_value", "labor_cost_rate", "occupancy_cash_cost_rate", "store_contribution", "store_contribution_margin", "sales_per_sqm", "sales_per_labor_hour", "labor_hours_per_transaction"} {
		var currentKPI, comparisonKPI retailkpi.KPIValue
		if current != nil {
			currentKPI = current.KPIs[code]
		} else {
			currentKPI = retailkpi.KPIValue{Status: retailkpi.StatusUnavailable, Reason: "missing_current_facts"}
		}
		if comparison != nil {
			comparisonKPI = comparison.KPIs[code]
		} else {
			comparisonKPI = retailkpi.KPIValue{Status: retailkpi.StatusUnavailable, Reason: "missing_comparison_facts"}
		}
		metric := SummaryMetric{Current: currentKPI, Comparison: comparisonKPI, ChangeType: retailkpi.ChangeRateType(code), Status: summaryStatus(currentKPI, comparisonKPI)}
		metric.ChangeValue, metric.Reason = retailkpi.ChangeRate(currentKPI.Value, comparisonKPI.Value, metric.ChangeType)
		if code == "store_contribution" && current != nil && comparison != nil {
			metric.ChangeMarginPP = changeRate(current.KPIs["store_contribution_margin"].Value, comparison.KPIs["store_contribution_margin"].Value)
		}
		result[code] = metric
	}
	return result
}

func summaryStatus(current, comparison retailkpi.KPIValue) string {
	if current.Status == retailkpi.StatusUnavailable && comparison.Status == retailkpi.StatusUnavailable {
		return string(retailkpi.StatusUnavailable)
	}
	if current.Status == retailkpi.StatusUnavailable || comparison.Status == retailkpi.StatusUnavailable {
		return string(retailkpi.StatusPartial)
	}
	if current.Status != retailkpi.StatusComplete || comparison.Status != retailkpi.StatusComplete {
		return string(retailkpi.StatusPartial)
	}
	return string(retailkpi.StatusComplete)
}
func changeRate(current, comparison *float64) *float64 {
	if current == nil || comparison == nil {
		return nil
	}
	v := *current - *comparison
	return roundPtr(&v)
}

type signalRule struct {
	code, metric, direction, unit string
	threshold                     float64
	percentagePoint               bool
}

var signalRules = []signalRule{{"revenue_decline", "revenue", "down", "percent", -15, false}, {"footfall_decline", "footfall", "down", "percent", -15, false}, {"conversion_drop", "conversion_rate", "down", "percentage_point", -3, true}, {"average_ticket_drop", "average_transaction_value", "down", "percent", -15, false}, {"gross_margin_compression", "gross_margin_rate", "down", "percentage_point", -5, true}, {"labor_cost_rate_spike", "labor_cost_rate", "up", "percentage_point", 5, true}, {"occupancy_cost_rate_spike", "occupancy_cash_cost_rate", "up", "percentage_point", 5, true}}

// attentionTarget is one attention unit: a store, or a region/brand group
// when the request groups the view (M5). The evaluation core below is
// identical for both — suppression, signal rules, scoring and evidence.
type attentionTarget struct {
	groupBy, groupKey, groupLabel, sortKey       string
	storeID, storeCode, storeName, brand, region string
	expectedStores                               int
	facts                                        []retailkpi.DailyFact
}

func buildAttention(facts []retailkpi.DailyFact, currency string, query Query, limit int, currentStart, currentEnd, comparisonStart, comparisonEnd time.Time, set *repository.RetailKPIFactSet, lifecycles []retailcohort.StoreLifecycle) ([]Attention, []SuppressedAttention) {
	lifecycleMap := make(map[string]retailcohort.StoreLifecycle, len(lifecycles))
	for _, lc := range lifecycles {
		lifecycleMap[lc.StoreID] = lc
	}
	targets := attentionTargets(facts, query, set)
	type candidate struct{ attention Attention }
	candidates := []candidate{}
	suppressed := []SuppressedAttention{}
	for _, target := range targets {
		currentFacts, comparisonFacts := between(target.facts, currentStart, currentEnd), between(target.facts, comparisonStart, comparisonEnd)
		currentAgg, currentCoverage := totalAggregate(currentFacts, currentStart, currentEnd, target.expectedStores)
		comparisonAgg, comparisonCoverage := totalAggregate(comparisonFacts, comparisonStart, comparisonEnd, target.expectedStores)
		reasons := suppressionReasons(currentAgg, comparisonAgg, currentCoverage, comparisonCoverage)
		if len(reasons) > 0 || currentAgg == nil || comparisonAgg == nil || !currentAgg.DecisionReady || !comparisonAgg.DecisionReady {
			if len(reasons) == 0 {
				reasons = []string{"partial_or_unavailable_kpi"}
			}
			suppressed = append(suppressed, SuppressedAttention{GroupBy: target.groupBy, GroupKey: target.groupKey, GroupLabel: target.groupLabel, StoreID: target.storeID, StoreCode: target.storeCode, StoreName: target.storeName, Brand: target.brand, Region: target.region, Currency: currency, CurrencyStatus: currencyStatus(currency), Reason: primarySuppressionReason(reasons), Reasons: reasons, CurrentCoverage: currentCoverage, ComparisonCoverage: comparisonCoverage})
			continue
		}
		signals := make([]Signal, 0)
		for _, rule := range signalRules {
			if signal, ok := evaluateSignal(rule, *currentAgg, *comparisonAgg); ok {
				signals = append(signals, signal)
			}
		}
		if turnsNegative(currentAgg.KPIs["store_contribution"], comparisonAgg.KPIs["store_contribution"]) {
			signals = append(signals, Signal{SignalCode: "contribution_turns_negative", ObservedChange: changeRate(currentAgg.KPIs["store_contribution"].Value, comparisonAgg.KPIs["store_contribution"].Value), Threshold: 0, Direction: "down", Current: currentAgg.KPIs["store_contribution"].Value, Comparison: comparisonAgg.KPIs["store_contribution"].Value, Unit: "currency", ScoreContribution: 1})
		}
		if len(signals) == 0 {
			continue
		}
		score := 0.0
		for _, signal := range signals {
			score += signal.ScoreContribution
		}
		sort.Slice(signals, func(i, j int) bool { return signals[i].SignalCode < signals[j].SignalCode })
		unitFacts := append(append([]retailkpi.DailyFact{}, currentFacts...), comparisonFacts...)
		unitSources, unitDatasets := factEnvelope(unitFacts)
		drilldownGroupBy := "store"
		drilldownStoreID := target.storeID
		if target.groupBy == "region" || target.groupBy == "brand" {
			drilldownGroupBy = target.groupBy
			drilldownStoreID = ""
		}
		var lcStatus, storeFmt string
		if lc, ok := lifecycleMap[target.storeID]; ok {
			storeFmt = lc.StoreFormat
			lcStatus = string(retailcohort.CalculateLifecycleStatus(lc.OpeningDate, lc.ClosingDate, currentEnd, 12))
		}
		candidates = append(candidates, candidate{attention: Attention{GroupBy: target.groupBy, GroupKey: target.groupKey, GroupLabel: target.groupLabel, StoreID: target.storeID, StoreCode: target.storeCode, StoreName: target.storeName, Brand: target.brand, Region: target.region, StoreFormat: storeFmt, LifecycleStatus: lcStatus, Currency: currency, CurrencyStatus: currencyStatus(currency), Score: round(score), Severity: severity(score), ObservedSignals: signals, CurrentKPIs: currentAgg.KPIs, ComparisonKPIs: comparisonAgg.KPIs, Evidence: Evidence{Current: Period{DateFrom: currentStart.Format("2006-01-02"), DateTo: currentEnd.Format("2006-01-02")}, Comparison: Period{DateFrom: comparisonStart.Format("2006-01-02"), DateTo: comparisonEnd.Format("2006-01-02")}, CurrentFactCount: len(currentFacts), ComparisonFactCount: len(comparisonFacts), SourceSystems: unitSources, DatasetVersions: unitDatasets, FormulaVersion: retailkpi.FormulaVersion, PulseVersion: PulseVersion}, Drilldown: map[string]string{"group_by": drilldownGroupBy, "store_id": drilldownStoreID, "store_code": target.storeCode, "currency": currency, "data_classification": query.Classification, "simulation_dataset_version": query.DatasetVersion, "source_system": query.SourceSystem, "current_date_from": currentStart.Format("2006-01-02"), "current_date_to": currentEnd.Format("2006-01-02"), "comparison_date_from": comparisonStart.Format("2006-01-02"), "comparison_date_to": comparisonEnd.Format("2006-01-02"), "current_url": drilldownURL(query, drilldownGroupBy, drilldownStoreID, currentStart, currentEnd), "comparison_url": drilldownURL(query, drilldownGroupBy, drilldownStoreID, comparisonStart, comparisonEnd)}}})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].attention.Score != candidates[j].attention.Score {
			return candidates[i].attention.Score > candidates[j].attention.Score
		}
		return attentionSortKey(candidates[i].attention) < attentionSortKey(candidates[j].attention)
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	sort.SliceStable(suppressed, func(i, j int) bool {
		return suppressed[i].StoreCode+suppressed[i].GroupLabel < suppressed[j].StoreCode+suppressed[j].GroupLabel
	})
	result := make([]Attention, len(candidates))
	for i := range candidates {
		result[i] = candidates[i].attention
		result[i].Rank = i + 1
	}
	return result, suppressed
}

// attentionTargets splits the partition facts into attention units. The
// default (and group_by=store) keeps the per-store ranking — zero regression;
// region/brand group the same facts per group value with the expected store
// count taken from the authorized population so missing stores suppress
// rather than vanish.
func attentionTargets(facts []retailkpi.DailyFact, query Query, set *repository.RetailKPIFactSet) []attentionTarget {
	if query.GroupBy != "region" && query.GroupBy != "brand" {
		byStore := map[string][]retailkpi.DailyFact{}
		metadata := map[string]retailkpi.DailyFact{}
		for _, fact := range facts {
			byStore[fact.StoreID] = append(byStore[fact.StoreID], fact)
			metadata[fact.StoreID] = fact
		}
		targets := make([]attentionTarget, 0, len(byStore))
		for storeID, storeFacts := range byStore {
			store := metadata[storeID]
			targets = append(targets, attentionTarget{groupBy: query.GroupBy, groupKey: storeID, groupLabel: store.StoreName, sortKey: store.StoreCode, storeID: storeID, storeCode: store.StoreCode, storeName: store.StoreName, brand: store.Brand, region: store.Region, expectedStores: 1, facts: storeFacts})
		}
		return targets
	}
	byGroup := map[string][]retailkpi.DailyFact{}
	storesInGroup := map[string]map[string]bool{}
	for _, fact := range facts {
		key := fact.Region
		if query.GroupBy == "brand" {
			key = fact.Brand
		}
		byGroup[key] = append(byGroup[key], fact)
		if storesInGroup[key] == nil {
			storesInGroup[key] = map[string]bool{}
		}
		storesInGroup[key][fact.StoreID] = true
	}
	expectedByGroup := map[string]int{}
	for _, store := range set.ExpectedStores {
		key := store.Region
		if query.GroupBy == "brand" {
			key = store.Brand
		}
		expectedByGroup[key]++
	}
	keys := make([]string, 0, len(byGroup))
	for key := range byGroup {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	targets := make([]attentionTarget, 0, len(keys))
	for _, key := range keys {
		expected := expectedByGroup[key]
		if expected == 0 {
			expected = len(storesInGroup[key])
		}
		targets = append(targets, attentionTarget{groupBy: query.GroupBy, groupKey: key, groupLabel: key, sortKey: key, expectedStores: expected, facts: byGroup[key]})
	}
	return targets
}

func attentionSortKey(attention Attention) string {
	if attention.StoreCode != "" {
		return attention.StoreCode
	}
	return attention.GroupLabel
}

func evaluateSignal(rule signalRule, current, comparison retailkpi.Aggregate) (Signal, bool) {
	currentKPI, comparisonKPI := current.KPIs[rule.metric], comparison.KPIs[rule.metric]
	observed, _ := retailkpi.ChangeRate(currentKPI.Value, comparisonKPI.Value, map[bool]string{true: "percentage_point", false: "percent"}[rule.percentagePoint])
	if observed == nil {
		return Signal{}, false
	}
	trigger := (rule.threshold < 0 && *observed <= rule.threshold) || (rule.threshold > 0 && *observed >= rule.threshold)
	if !trigger {
		return Signal{}, false
	}
	contribution := math.Min(math.Abs(*observed/rule.threshold), 3)
	return Signal{SignalCode: rule.code, ObservedChange: observed, Threshold: rule.threshold, Direction: rule.direction, Current: currentKPI.Value, Comparison: comparisonKPI.Value, Unit: rule.unit, ScoreContribution: round(contribution)}, true
}
func turnsNegative(current, comparison retailkpi.KPIValue) bool {
	return current.Value != nil && comparison.Value != nil && *comparison.Value >= 0 && *current.Value < 0
}
func severity(score float64) string {
	if score >= 6 {
		return "critical"
	}
	if score >= 3 {
		return "high"
	}
	if score >= 1 {
		return "medium"
	}
	return "low"
}
func buildTrend(facts []retailkpi.DailyFact, currency string, from, to time.Time, expected int) []DailyTrend {
	result := []DailyTrend{}
	for date := from; !date.After(to); date = date.AddDate(0, 0, 1) {
		dayFacts := []retailkpi.DailyFact{}
		for _, fact := range facts {
			if sameDate(fact.BusinessDate, date) {
				dayFacts = append(dayFacts, fact)
			}
		}
		_, coverage := totalAggregate(dayFacts, date, date, expected)
		// rate stays nil when the day has no expected store-days; the
		// frontend renders null as "—" instead of a fabricated 0.
		row := DailyTrend{Date: date.Format("2006-01-02"), Currency: currency, Gap: len(dayFacts) == 0, Coverage: coverage}
		if currency == "" {
			row.CurrencyStatus = UnknownCurrencyStatus
		}
		if len(dayFacts) > 0 {
			aggregates, _, _ := retailkpi.AggregateFacts(dayFacts, retailkpi.Request{DateFrom: date, DateTo: date, RequestedDateFrom: date.Format("2006-01-02"), RequestedDateTo: date.Format("2006-01-02"), GroupBy: "total", ExpectedStoreCount: expected})
			if len(aggregates) > 0 {
				row.KPIs = selectTrendKPIs(aggregates[0].KPIs)
			}
		}
		result = append(result, row)
	}
	return result
}

// FIX-031: gross_profit was missing here while the trend switcher offered
// 毛利额 — picking it drew an empty chart directly under a KPI card showing
// that very number. This whitelist must cover every code the switcher exposes.
func selectTrendKPIs(all map[string]retailkpi.KPIValue) map[string]retailkpi.KPIValue {
	result := map[string]retailkpi.KPIValue{}
	for _, code := range []string{"revenue", "gross_profit", "gross_margin_rate", "footfall", "conversion_rate", "average_transaction_value", "labor_cost_rate", "occupancy_cash_cost_rate", "store_contribution"} {
		result[code] = all[code]
	}
	return result
}
func suppressionReasons(currentAgg, comparisonAgg *retailkpi.Aggregate, currentCoverage, comparisonCoverage retailkpi.Coverage) []string {
	seen := map[string]bool{}
	add := func(reason string) {
		if reason != "" && !seen[reason] {
			seen[reason] = true
		}
	}
	if currentAgg == nil || comparisonAgg == nil {
		add("missing_period_facts")
	}
	if retailkpi.CoverageIncomplete(currentCoverage) || retailkpi.CoverageIncomplete(comparisonCoverage) {
		add("incomplete_store_day_coverage")
	}
	for _, aggregate := range []*retailkpi.Aggregate{currentAgg, comparisonAgg} {
		if aggregate == nil {
			continue
		}
		for _, issue := range aggregate.DataQualityIssues {
			switch {
			case issue == "data_quality_invalid":
				add("data_quality_invalid")
			case strings.HasPrefix(issue, "mapping_"):
				add(issue)
			}
		}
		for _, d := range retailkpi.Definitions() {
			if !d.IsCore {
				continue
			}
			value := aggregate.KPIs[d.Code]
			if value.Status == retailkpi.StatusPartial || value.Status == retailkpi.StatusUnavailable {
				add("partial_or_unavailable_kpi")
				break
			}
		}
	}
	result := make([]string, 0, len(seen))
	for reason := range seen {
		result = append(result, reason)
	}
	sort.Strings(result)
	return result
}

func primarySuppressionReason(reasons []string) string {
	priority := []string{"missing_period_facts", "incomplete_store_day_coverage", "data_quality_invalid", "partial_or_unavailable_kpi"}
	for _, preferred := range priority {
		for _, reason := range reasons {
			if reason == preferred || strings.HasPrefix(reason, "mapping_") && preferred == "partial_or_unavailable_kpi" {
				return reason
			}
		}
	}
	if len(reasons) == 0 {
		return "partial_or_unavailable_kpi"
	}
	return reasons[0]
}

func factEnvelope(facts []retailkpi.DailyFact) ([]string, []string) {
	sources, datasets := map[string]bool{}, map[string]bool{}
	for _, fact := range facts {
		if fact.SourceSystem != "" {
			sources[fact.SourceSystem] = true
		}
		if fact.SimulationDatasetVersion != nil && *fact.SimulationDatasetVersion != "" {
			datasets[*fact.SimulationDatasetVersion] = true
		}
	}
	resultSources, resultDatasets := make([]string, 0, len(sources)), make([]string, 0, len(datasets))
	for source := range sources {
		resultSources = append(resultSources, source)
	}
	for dataset := range datasets {
		resultDatasets = append(resultDatasets, dataset)
	}
	sort.Strings(resultSources)
	sort.Strings(resultDatasets)
	return resultSources, resultDatasets
}
func between(facts []retailkpi.DailyFact, from, to time.Time) []retailkpi.DailyFact {
	result := []retailkpi.DailyFact{}
	for _, f := range facts {
		if !f.BusinessDate.Before(from) && !f.BusinessDate.After(to) {
			result = append(result, f)
		}
	}
	return result
}
func distinctStores(facts []retailkpi.DailyFact) int {
	stores := map[string]bool{}
	for _, f := range facts {
		stores[f.StoreID] = true
	}
	return len(stores)
}

func distinctCurrencies(facts []retailkpi.DailyFact) int {
	currencies := map[string]bool{}
	for _, fact := range facts {
		currencies[fact.Currency] = true
	}
	return len(currencies)
}

// mixedCurrencyStores counts stores whose facts span more than one currency —
// the P0-7 signal surfaced on the response so partition readers can see the
// attribution basis, not just the partition split.
func mixedCurrencyStores(facts []retailkpi.DailyFact) int {
	byStore := map[string]map[string]bool{}
	for _, fact := range facts {
		if byStore[fact.StoreID] == nil {
			byStore[fact.StoreID] = map[string]bool{}
		}
		byStore[fact.StoreID][fact.Currency] = true
	}
	count := 0
	for _, currencies := range byStore {
		if len(currencies) > 1 {
			count++
		}
	}
	return count
}
func currencyStatus(currency string) string {
	if currency == "" {
		return UnknownCurrencyStatus
	}
	return ""
}
func sameDate(a, b time.Time) bool { return dateOnly(a).Equal(dateOnly(b)) }
func dateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func inclusiveDays(from, to time.Time) int {
	if to.Before(from) {
		return 0
	}
	return int(to.Sub(from).Hours()/24) + 1
}

func ptr(value float64) *float64  { return &value }
func round(value float64) float64 { return math.Round(value*100) / 100 }
func roundPtr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	v := round(*value)
	return &v
}
