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
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/services/sourceenvelope"
)

const (
	PulseVersion          = "retail-pulse-v1"
	DefaultWindowDays     = 7
	DefaultAttentionLimit = 10
	UnknownCurrencyStatus = "unknown"
)

type FactReader interface {
	QueryFacts(context.Context, string, string, string, string, string, string, []string) (*repository.RetailKPIFactSet, error)
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
	StoreID         string                        `json:"store_id"`
	StoreCode       string                        `json:"store_code"`
	StoreName       string                        `json:"store_name"`
	Brand           string                        `json:"brand"`
	Region          string                        `json:"region"`
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

type Service struct {
	reader FactReader
	now    func() time.Time
}

func NewService(reader FactReader) *Service { return &Service{reader: reader, now: time.Now} }

func (s *Service) Build(ctx context.Context, query Query) (*Response, error) {
	if s.reader == nil {
		return nil, fmt.Errorf("retail pulse fact reader is required")
	}
	if query.LegalEntityID == "" {
		return nil, fmt.Errorf("legal entity scope is required")
	}
	if query.WindowDays == 0 {
		query.WindowDays = DefaultWindowDays
	}
	if query.WindowDays < 7 || query.WindowDays > 28 {
		return nil, fmt.Errorf("window_days must be between 7 and 28")
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
	partitions := s.buildPartitions(set, linkQuery, currentStart, currentEnd, comparisonStart, comparisonEnd)
	response := &Response{
		Basis: "Working", PulseVersion: PulseVersion, FormulaVersion: retailkpi.FormulaVersion,
		DataClassification: query.Classification, DatasetVersion: query.DatasetVersion,
		SimulationDatasetVersions: set.DatasetVersions,
		RequestedScope:            map[string]any{"legal_entity_id": query.LegalEntityID, "store_ids": query.StoreIDs},
		RequestedStores:           set.ExpectedStores,
		MultiCurrency:             distinctCurrencies(set.Facts) > 1,
		Current:                   Period{DateFrom: currentStart.Format("2006-01-02"), DateTo: currentEnd.Format("2006-01-02")},
		Comparison:                Period{DateFrom: comparisonStart.Format("2006-01-02"), DateTo: comparisonEnd.Format("2006-01-02")},
		GeneratedAt:               s.now(), DefinitionsURL: "/api/v1/retail/kpis/definitions",
		KPIDrilldownURL:           drilldownTemplate(linkQuery, "{group_by}", "{store_id}", "{date_from}", "{date_to}"),
		StoreDrilldownURL:         drilldownTemplate(linkQuery, "store", "{store_id}", "{date_from}", "{date_to}"),
		CurrentKPIDrilldownURL:    drilldownURL(linkQuery, "total", "", currentStart, currentEnd),
		ComparisonKPIDrilldownURL: drilldownURL(linkQuery, "total", "", comparisonStart, comparisonEnd),
	}
	if len(partitions) == 1 {
		p := partitions[0]
		response.Currency, response.CurrencyStatus = p.Currency, p.CurrencyStatus
		response.CurrentCoverage, response.ComparisonCoverage, response.DecisionReady, response.Summary, response.DailyTrend, response.Attention, response.SuppressedAttention, response.AttentionCount = p.CurrentCoverage, p.ComparisonCoverage, p.DecisionReady, p.Summary, p.DailyTrend, p.Attention, p.SuppressedAttention, p.AttentionCount
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
	return response, nil
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

func (s *Service) buildPartitions(set *repository.RetailKPIFactSet, query Query, currentStart, currentEnd, comparisonStart, comparisonEnd time.Time) []Partition {
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
	observedStoreCurrency := map[string]string{}
	for _, fact := range set.Facts {
		observedStoreCurrency[fact.StoreID] = fact.Currency
	}
	unobserved := make([]retailkpi.StorePopulation, 0)
	for _, store := range set.ExpectedStores {
		if currency, ok := observedStoreCurrency[store.StoreID]; ok {
			populationByCurrency[currency]++
		} else {
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
		if currentAgg != nil && comparisonAgg != nil {
			partition.Summary = buildSummary(*currentAgg, *comparisonAgg)
		}
		partition.Attention, partition.SuppressedAttention = buildAttention(facts, currency, query, query.AttentionLimit, currentStart, currentEnd, comparisonStart, comparisonEnd, set)
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

func buildSummary(current, comparison retailkpi.Aggregate) map[string]SummaryMetric {
	result := map[string]SummaryMetric{}
	for _, code := range []string{"revenue", "gross_profit", "gross_margin_rate", "footfall", "transactions", "conversion_rate", "average_transaction_value", "labor_cost_rate", "occupancy_cash_cost_rate", "store_contribution", "store_contribution_margin", "sales_per_sqm"} {
		currentKPI, comparisonKPI := current.KPIs[code], comparison.KPIs[code]
		metric := SummaryMetric{Current: currentKPI, Comparison: comparisonKPI, ChangeType: retailkpi.ChangeRateType(code), Status: summaryStatus(currentKPI, comparisonKPI)}
		metric.ChangeValue, metric.Reason = retailkpi.ChangeRate(currentKPI.Value, comparisonKPI.Value, metric.ChangeType)
		if code == "store_contribution" {
			metric.ChangeMarginPP = changeRate(current.KPIs["store_contribution_margin"].Value, comparison.KPIs["store_contribution_margin"].Value)
		}
		result[code] = metric
	}
	return result
}

func summaryStatus(current, comparison retailkpi.KPIValue) string {
	if current.Status == retailkpi.StatusUnavailable || comparison.Status == retailkpi.StatusUnavailable {
		return string(retailkpi.StatusUnavailable)
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

func buildAttention(facts []retailkpi.DailyFact, currency string, query Query, limit int, currentStart, currentEnd, comparisonStart, comparisonEnd time.Time, set *repository.RetailKPIFactSet) ([]Attention, []SuppressedAttention) {
	byStore := map[string][]retailkpi.DailyFact{}
	metadata := map[string]retailkpi.DailyFact{}
	for _, fact := range facts {
		key := fact.StoreID
		byStore[key] = append(byStore[key], fact)
		metadata[key] = fact
	}
	type candidate struct{ attention Attention }
	candidates := []candidate{}
	suppressed := []SuppressedAttention{}
	for storeID, storeFacts := range byStore {
		store := metadata[storeID]
		currentFacts, comparisonFacts := between(storeFacts, currentStart, currentEnd), between(storeFacts, comparisonStart, comparisonEnd)
		currentAgg, currentCoverage := totalAggregate(currentFacts, currentStart, currentEnd, 1)
		comparisonAgg, comparisonCoverage := totalAggregate(comparisonFacts, comparisonStart, comparisonEnd, 1)
		reasons := suppressionReasons(currentAgg, comparisonAgg, currentCoverage, comparisonCoverage)
		if len(reasons) > 0 || currentAgg == nil || comparisonAgg == nil || !currentAgg.DecisionReady || !comparisonAgg.DecisionReady {
			if len(reasons) == 0 {
				reasons = []string{"partial_or_unavailable_kpi"}
			}
			suppressed = append(suppressed, SuppressedAttention{StoreID: storeID, StoreCode: store.StoreCode, StoreName: store.StoreName, Brand: store.Brand, Region: store.Region, Currency: currency, CurrencyStatus: currencyStatus(currency), Reason: primarySuppressionReason(reasons), Reasons: reasons, CurrentCoverage: currentCoverage, ComparisonCoverage: comparisonCoverage})
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
		storeSourceSystems, storeDatasetVersions := factEnvelope(append(append([]retailkpi.DailyFact{}, currentFacts...), comparisonFacts...))
		candidates = append(candidates, candidate{attention: Attention{StoreID: storeID, StoreCode: store.StoreCode, StoreName: store.StoreName, Brand: store.Brand, Region: store.Region, Currency: currency, CurrencyStatus: currencyStatus(currency), Score: round(score), Severity: severity(score), ObservedSignals: signals, CurrentKPIs: currentAgg.KPIs, ComparisonKPIs: comparisonAgg.KPIs, Evidence: Evidence{Current: Period{DateFrom: currentStart.Format("2006-01-02"), DateTo: currentEnd.Format("2006-01-02")}, Comparison: Period{DateFrom: comparisonStart.Format("2006-01-02"), DateTo: comparisonEnd.Format("2006-01-02")}, CurrentFactCount: len(currentFacts), ComparisonFactCount: len(comparisonFacts), SourceSystems: storeSourceSystems, DatasetVersions: storeDatasetVersions, FormulaVersion: retailkpi.FormulaVersion, PulseVersion: PulseVersion}, Drilldown: map[string]string{"store_id": storeID, "store_code": store.StoreCode, "currency": currency, "data_classification": query.Classification, "simulation_dataset_version": query.DatasetVersion, "source_system": query.SourceSystem, "current_date_from": currentStart.Format("2006-01-02"), "current_date_to": currentEnd.Format("2006-01-02"), "comparison_date_from": comparisonStart.Format("2006-01-02"), "comparison_date_to": comparisonEnd.Format("2006-01-02"), "current_url": drilldownURL(query, "store", storeID, currentStart, currentEnd), "comparison_url": drilldownURL(query, "store", storeID, comparisonStart, comparisonEnd)}}})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].attention.Score != candidates[j].attention.Score {
			return candidates[i].attention.Score > candidates[j].attention.Score
		}
		return candidates[i].attention.StoreCode < candidates[j].attention.StoreCode
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}
	sort.SliceStable(suppressed, func(i, j int) bool {
		if suppressed[i].StoreCode != suppressed[j].StoreCode {
			return suppressed[i].StoreCode < suppressed[j].StoreCode
		}
		return suppressed[i].StoreID < suppressed[j].StoreID
	})
	result := make([]Attention, len(candidates))
	for i := range candidates {
		result[i] = candidates[i].attention
		result[i].Rank = i + 1
	}
	return result, suppressed
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
func selectTrendKPIs(all map[string]retailkpi.KPIValue) map[string]retailkpi.KPIValue {
	result := map[string]retailkpi.KPIValue{}
	for _, code := range []string{"revenue", "gross_margin_rate", "footfall", "conversion_rate", "average_transaction_value", "labor_cost_rate", "occupancy_cash_cost_rate", "store_contribution"} {
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
		for _, value := range aggregate.KPIs {
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
