package retailkpi

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// FormulaVersion is the immutable semantic contract consumed by the daily
// retail KPI API. Changes to formulas must introduce a new version.
const FormulaVersion = "retail-kpi-v1"

type KPIStatus string

const (
	StatusComplete    KPIStatus = "complete"
	StatusPartial     KPIStatus = "partial"
	StatusUnavailable KPIStatus = "unavailable"
)

// DailyFact is the nullable source-grain input to the semantic layer. The
// repository performs tenant, source and highest-version selection before
// handing facts to this package.
type DailyFact struct {
	StoreID, StoreCode, StoreName, Brand, Region string
	BusinessDate                                 time.Time
	AsOfAt                                       time.Time
	Currency, SourceSystem, DataClassification   string
	SimulationDatasetVersion                     *string
	Version                                      int
	Revenue, GrossProfit, Transactions, Footfall *float64
	AreaSqm, LaborCost, FixedRent, VariableRent  *float64
	NonLeaseCost, OtherControllableCost          *float64
	DataQualityStatus, MappingStatus             string
}

// StorePopulation is the authorized active store set used to make coverage
// explicit even when a store has no facts in the requested range.
type StorePopulation struct {
	StoreID   string
	StoreCode string
	StoreName string
	Brand     string
	Region    string
}

type KPIValue struct {
	Value              *float64  `json:"value"`
	Unit               string    `json:"unit"`
	Status             KPIStatus `json:"status"`
	FormulaVersion     string    `json:"formula_version"`
	RequiredFields     []string  `json:"required_fields"`
	AvailableFactCount int       `json:"available_fact_count"`
	FactCount          int       `json:"fact_count"`
	Reason             string    `json:"reason,omitempty"`
}

type Definition struct {
	Code            string   `json:"code"`
	Name            string   `json:"name"`
	NameZH          string   `json:"name_zh"`
	Unit            string   `json:"unit"`
	Formula         string   `json:"formula"`
	RequiredFields  []string `json:"required_fields"`
	NullRule        string   `json:"null_rule"`
	DenominatorRule string   `json:"denominator_rule"`
	Description     string   `json:"description"`
}

var definitions = []Definition{
	{Code: "revenue", Name: "Revenue", Unit: "currency", Formula: "SUM(revenue)", RequiredFields: []string{"revenue"}, NullRule: "missing required facts yield partial/null", DenominatorRule: "not applicable", Description: "Daily net sales in the fact currency."},
	{Code: "gross_profit", Name: "Gross profit", Unit: "currency", Formula: "SUM(gross_profit)", RequiredFields: []string{"gross_profit"}, NullRule: "missing required facts yield partial/null", DenominatorRule: "not applicable", Description: "Daily gross profit."},
	{Code: "footfall", Name: "Footfall", Unit: "count", Formula: "SUM(footfall)", RequiredFields: []string{"footfall"}, NullRule: "missing required facts yield partial/null", DenominatorRule: "not applicable", Description: "Daily observed visitors."},
	{Code: "transactions", Name: "Transactions", Unit: "count", Formula: "SUM(transactions)", RequiredFields: []string{"transactions"}, NullRule: "missing required facts yield partial/null", DenominatorRule: "not applicable", Description: "Daily transactions."},
	{Code: "labor_cost", Name: "Labor cost", Unit: "currency", Formula: "SUM(labor_cost)", RequiredFields: []string{"labor_cost"}, NullRule: "missing required facts yield partial/null", DenominatorRule: "not applicable", Description: "Daily labor cost."},
	{Code: "fixed_rent", Name: "Fixed rent", Unit: "currency", Formula: "SUM(fixed_rent)", RequiredFields: []string{"fixed_rent"}, NullRule: "missing required facts yield partial/null", DenominatorRule: "not applicable", Description: "Daily fixed rent."},
	{Code: "variable_rent", Name: "Variable rent", Unit: "currency", Formula: "SUM(variable_rent)", RequiredFields: []string{"variable_rent"}, NullRule: "missing required facts yield partial/null", DenominatorRule: "not applicable", Description: "Daily sales-linked rent."},
	{Code: "non_lease_cost", Name: "Non-lease cost", Unit: "currency", Formula: "SUM(non_lease_cost)", RequiredFields: []string{"non_lease_cost"}, NullRule: "missing required facts yield partial/null", DenominatorRule: "not applicable", Description: "Daily operating non-lease occupancy cost."},
	{Code: "other_controllable_cost", Name: "Other controllable cost", Unit: "currency", Formula: "SUM(other_controllable_cost)", RequiredFields: []string{"other_controllable_cost"}, NullRule: "missing required facts yield partial/null", DenominatorRule: "not applicable", Description: "Daily controllable operating cost outside labor and occupancy."},
	{Code: "occupancy_cash_cost", Name: "Occupancy cash cost", Unit: "currency", Formula: "fixed_rent + variable_rent + non_lease_cost", RequiredFields: []string{"fixed_rent", "variable_rent", "non_lease_cost"}, NullRule: "missing required facts yield partial/null", DenominatorRule: "not applicable", Description: "Operating cash occupancy cost; excludes IFRS16 depreciation, interest, ROU and lease liability movements."},
	{Code: "store_contribution", Name: "Store contribution", Unit: "currency", Formula: "gross_profit - labor_cost - occupancy_cash_cost - other_controllable_cost", RequiredFields: []string{"gross_profit", "labor_cost", "fixed_rent", "variable_rent", "non_lease_cost", "other_controllable_cost"}, NullRule: "missing required facts yield partial/null", DenominatorRule: "not applicable", Description: "Operating contribution before IFRS16 accounting measures."},
	{Code: "gross_margin_rate", Name: "Gross margin rate", Unit: "percent", Formula: "gross_profit / revenue * 100", RequiredFields: []string{"gross_profit", "revenue"}, NullRule: "missing required facts yield partial/null", DenominatorRule: "zero revenue => null, reason zero_denominator", Description: "Gross profit as a percentage of revenue."},
	{Code: "conversion_rate", Name: "Conversion rate", Unit: "percent", Formula: "transactions / footfall * 100", RequiredFields: []string{"transactions", "footfall"}, NullRule: "missing required facts yield partial/null", DenominatorRule: "zero footfall => null, reason zero_denominator", Description: "Transactions as a percentage of visitors."},
	{Code: "average_transaction_value", Name: "Average transaction value", Unit: "currency", Formula: "revenue / transactions", RequiredFields: []string{"revenue", "transactions"}, NullRule: "missing required facts yield partial/null", DenominatorRule: "zero transactions => null, reason zero_denominator", Description: "Revenue per transaction."},
	{Code: "labor_cost_rate", Name: "Labor cost rate", Unit: "percent", Formula: "labor_cost / revenue * 100", RequiredFields: []string{"labor_cost", "revenue"}, NullRule: "missing required facts yield partial/null", DenominatorRule: "zero revenue => null, reason zero_denominator", Description: "Labor cost as a percentage of revenue."},
	{Code: "rent_to_sales_rate", Name: "Rent to sales rate", Unit: "percent", Formula: "(fixed_rent + variable_rent) / revenue * 100", RequiredFields: []string{"fixed_rent", "variable_rent", "revenue"}, NullRule: "missing required facts yield partial/null", DenominatorRule: "zero revenue => null, reason zero_denominator", Description: "Cash rent as a percentage of revenue."},
	{Code: "occupancy_cash_cost_rate", Name: "Occupancy cash cost rate", Unit: "percent", Formula: "occupancy_cash_cost / revenue * 100", RequiredFields: []string{"fixed_rent", "variable_rent", "non_lease_cost", "revenue"}, NullRule: "missing required facts yield partial/null", DenominatorRule: "zero revenue => null, reason zero_denominator", Description: "Operating cash occupancy cost as a percentage of revenue."},
	{Code: "store_contribution_margin", Name: "Store contribution margin", Unit: "percent", Formula: "store_contribution / revenue * 100", RequiredFields: []string{"gross_profit", "labor_cost", "fixed_rent", "variable_rent", "non_lease_cost", "other_controllable_cost", "revenue"}, NullRule: "missing required facts yield partial/null", DenominatorRule: "zero revenue => null, reason zero_denominator", Description: "Operating contribution as a percentage of revenue."},
	{Code: "average_daily_area_sqm", Name: "Average daily area", Unit: "sqm", Formula: "SUM(area_sqm by store-day) / distinct_business_days", RequiredFields: []string{"area_sqm"}, NullRule: "missing area facts yield partial/null", DenominatorRule: "zero distinct business days => null, reason zero_denominator", Description: "Daily store area averaged over distinct business days; area is never summed across days."},
	{Code: "sales_per_sqm", Name: "Sales per square metre", Unit: "currency_per_sqm", Formula: "revenue / average_daily_area_sqm", RequiredFields: []string{"revenue", "area_sqm"}, NullRule: "missing area facts yield partial/null", DenominatorRule: "zero average_daily_area_sqm => null, reason zero_denominator", Description: "Revenue divided by average daily area; area is averaged by distinct business day, never summed across days."},
	{Code: "revenue_per_store_day", Name: "Revenue per store-day", Unit: "currency", Formula: "revenue / observed_store_days", RequiredFields: []string{"revenue"}, NullRule: "no observed store-days => unavailable/null", DenominatorRule: "zero observed store-days => null, reason zero_denominator", Description: "Revenue divided by observed store-day rows."},
}

func Definitions() []Definition {
	result := make([]Definition, len(definitions))
	copy(result, definitions)
	for i := range result {
		result[i].RequiredFields = append([]string(nil), result[i].RequiredFields...)
		result[i].NameZH = chineseNames[result[i].Code]
	}
	return result
}

var chineseNames = map[string]string{
	"revenue": "销售额", "gross_profit": "毛利额", "footfall": "客流", "transactions": "交易数",
	"labor_cost": "人工成本", "fixed_rent": "固定现金租金", "variable_rent": "变动租金", "non_lease_cost": "非租赁占用成本",
	"other_controllable_cost": "其他可控成本", "occupancy_cash_cost": "经营占用现金成本", "store_contribution": "门店经营利润",
	"gross_margin_rate": "毛利率", "conversion_rate": "转化率", "average_transaction_value": "客单价", "labor_cost_rate": "人工成本率",
	"rent_to_sales_rate": "租金销售比", "occupancy_cash_cost_rate": "经营占用成本率", "store_contribution_margin": "门店经营利润率",
	"average_daily_area_sqm": "平均日经营面积", "sales_per_sqm": "期间坪效", "revenue_per_store_day": "单店日均销售",
}

type Coverage struct {
	RequestedDateFrom string   `json:"requested_date_from"`
	RequestedDateTo   string   `json:"requested_date_to"`
	ObservedDateFrom  string   `json:"observed_date_from,omitempty"`
	ObservedDateTo    string   `json:"observed_date_to,omitempty"`
	ObservedStoreDays int      `json:"observed_store_days"`
	ExpectedStoreDays int      `json:"expected_store_days"`
	CoverageRate      *float64 `json:"coverage_rate"`
	MissingFields     []string `json:"missing_fields,omitempty"`
}

// MinimumPeerCount is the minimum size a Peer Cohort must have before a
// benchmark is produced (CONTEXT.md: Peer Cohort — a cohort below the
// minimum yields no benchmark rather than a weak one).
const MinimumPeerCount = 3

// CoverageIncomplete reports missing store-days: the expected population is
// known and the observed store-days fall below it. Over-coverage
// (observed > expected) is not incomplete. This is the single Fact Coverage
// verdict predicate — every retail read evaluates coverage through it.
func CoverageIncomplete(c Coverage) bool {
	return c.ExpectedStoreDays > 0 && c.ObservedStoreDays < c.ExpectedStoreDays
}

// CoverageComplete reports the coverage is sufficient to judge on: the
// expected population is known and fully observed.
func CoverageComplete(c Coverage) bool {
	return c.ExpectedStoreDays > 0 && c.ObservedStoreDays >= c.ExpectedStoreDays
}

// PercentileRank returns the percentile of target within values
// ((less + 0.5*equal) / n * 100), or nil when values is empty.
func PercentileRank(values []float64, target float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	less, equal := 0, 0
	for _, value := range values {
		if value < target {
			less++
		} else if value == target {
			equal++
		}
	}
	rank := (float64(less) + 0.5*float64(equal)) / float64(len(values)) * 100
	return &rank
}

// ChangeRateType classifies a metric's period-over-period change: rates and
// margins move in percentage points, volume and amount metrics in percent.
func ChangeRateType(code string) string {
	switch code {
	case "gross_margin_rate", "conversion_rate", "labor_cost_rate", "occupancy_cash_cost_rate", "store_contribution_margin", "rent_to_sales_rate":
		return "percentage_point"
	default:
		return "percent"
	}
}

// ChangeRate computes the period-over-period change of two KPI values under
// retail-kpi-v1 null semantics. A missing side yields nil with a reason; a
// percentage_point change is a plain difference; a percent change against a
// zero comparison is refused (no fabricated change).
func ChangeRate(current, comparison *float64, changeType string) (*float64, string) {
	if current == nil || comparison == nil {
		return nil, "missing_value"
	}
	if changeType == "percentage_point" {
		value := *current - *comparison
		return roundPtr(&value), ""
	}
	if *comparison == 0 {
		return nil, "zero_comparison"
	}
	value := (*current / *comparison - 1) * 100
	return roundPtr(&value), ""
}

type Aggregate struct {
	GroupBy              string              `json:"group_by"`
	GroupKey             string              `json:"group_key"`
	GroupLabel           string              `json:"group_label,omitempty"`
	StoreID              string              `json:"store_id,omitempty"`
	StoreCode            string              `json:"store_code,omitempty"`
	StoreName            string              `json:"store_name,omitempty"`
	Brand                string              `json:"brand,omitempty"`
	Region               string              `json:"region,omitempty"`
	Currency             string              `json:"currency"`
	ObservedStoreDays    int                 `json:"observed_store_days"`
	DistinctBusinessDays int                 `json:"distinct_business_days"`
	AverageDailyAreaSqm  *float64            `json:"average_daily_area_sqm"`
	KPIs                 map[string]KPIValue `json:"kpis"`
	DecisionReady        bool                `json:"decision_ready"`
	DataQualityIssues    []string            `json:"data_quality_issues,omitempty"`
}

type Request struct {
	DateFrom           time.Time
	DateTo             time.Time
	RequestedDateFrom  string
	RequestedDateTo    string
	GroupBy            string
	ExpectedStoreCount int
}

func AggregateFacts(facts []DailyFact, req Request) ([]Aggregate, Coverage, error) {
	if req.GroupBy == "" {
		req.GroupBy = "total"
	}
	if req.GroupBy != "total" && req.GroupBy != "region" && req.GroupBy != "brand" && req.GroupBy != "store" {
		return nil, Coverage{}, fmt.Errorf("group_by must be one of total, region, brand, store")
	}
	coverage := buildCoverage(facts, req)
	groups := map[string][]DailyFact{}
	labels := map[string]string{}
	for _, f := range facts {
		key, label := groupKey(f, req.GroupBy)
		groups[key+"\x00"+f.Currency] = append(groups[key+"\x00"+f.Currency], f)
		labels[key+"\x00"+f.Currency] = label
	}
	keys := make([]string, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	results := make([]Aggregate, 0, len(keys))
	for _, key := range keys {
		parts := strings.SplitN(key, "\x00", 2)
		result := aggregateGroup(groups[key], req.GroupBy, parts[0], labels[key], parts[1])
		results = append(results, result)
	}
	if CoverageIncomplete(coverage) {
		for i := range results {
			results[i].DataQualityIssues = appendUniqueIssue(results[i].DataQualityIssues, "incomplete_store_day_coverage")
			results[i].DecisionReady = false
		}
	}
	return results, coverage, nil
}

func appendUniqueIssue(issues []string, issue string) []string {
	for _, existing := range issues {
		if existing == issue {
			return issues
		}
	}
	issues = append(issues, issue)
	sort.Strings(issues)
	return issues
}

func buildCoverage(facts []DailyFact, req Request) Coverage {
	c := Coverage{RequestedDateFrom: req.RequestedDateFrom, RequestedDateTo: req.RequestedDateTo, ExpectedStoreDays: req.ExpectedStoreCount * inclusiveDays(req.DateFrom, req.DateTo)}
	dates := map[string]bool{}
	stores := map[string]bool{}
	missing := map[string]bool{}
	for _, f := range facts {
		dates[f.BusinessDate.Format("2006-01-02")] = true
		stores[f.StoreID] = true
		for _, d := range definitions {
			for _, field := range d.RequiredFields {
				if fieldValue(f, field) == nil {
					missing[field] = true
				}
			}
		}
	}
	c.ObservedStoreDays = len(facts)
	if len(dates) > 0 {
		ordered := make([]string, 0, len(dates))
		for d := range dates {
			ordered = append(ordered, d)
		}
		sort.Strings(ordered)
		c.ObservedDateFrom, c.ObservedDateTo = ordered[0], ordered[len(ordered)-1]
	}
	if c.ExpectedStoreDays > 0 {
		value := float64(c.ObservedStoreDays) / float64(c.ExpectedStoreDays) * 100
		c.CoverageRate = &value
	}
	for field := range missing {
		c.MissingFields = append(c.MissingFields, field)
	}
	sort.Strings(c.MissingFields)
	return c
}

func aggregateGroup(facts []DailyFact, groupBy, key, label, currency string) Aggregate {
	r := Aggregate{GroupBy: groupBy, GroupKey: key, GroupLabel: label, Currency: currency, ObservedStoreDays: len(facts), KPIs: map[string]KPIValue{}}
	if len(facts) > 0 {
		if groupBy == "store" {
			r.StoreID, r.StoreCode, r.StoreName, r.Brand, r.Region = facts[0].StoreID, facts[0].StoreCode, facts[0].StoreName, facts[0].Brand, facts[0].Region
		}
		if groupBy == "brand" {
			r.Brand = facts[0].Brand
		}
		if groupBy == "region" {
			r.Region = facts[0].Region
		}
	}
	days := map[string]bool{}
	for _, f := range facts {
		days[f.BusinessDate.Format("2006-01-02")] = true
	}
	r.DistinctBusinessDays = len(days)
	areaSum := 0.0
	areaAvailable := 0
	for _, f := range facts {
		if f.AreaSqm != nil {
			areaSum += *f.AreaSqm
			areaAvailable++
		}
	}
	if len(facts) > 0 && areaAvailable == len(facts) {
		v := areaSum / float64(maxInt(r.DistinctBusinessDays, 1))
		r.AverageDailyAreaSqm = &v
	}
	values := map[string]*float64{}
	for _, code := range []string{"revenue", "gross_profit", "footfall", "transactions", "labor_cost", "fixed_rent", "variable_rent", "non_lease_cost", "other_controllable_cost"} {
		values[code] = sumField(facts, code)
	}
	values["occupancy_cash_cost"] = combine(values["fixed_rent"], values["variable_rent"], values["non_lease_cost"], false)
	values["store_contribution"] = combine(values["gross_profit"], values["labor_cost"], values["occupancy_cash_cost"], true)
	if values["store_contribution"] != nil && values["other_controllable_cost"] != nil {
		v := *values["store_contribution"] - *values["other_controllable_cost"]
		values["store_contribution"] = &v
	} else {
		values["store_contribution"] = nil
	}
	values["gross_margin_rate"] = ratio(values["gross_profit"], values["revenue"], 100)
	values["conversion_rate"] = ratio(values["transactions"], values["footfall"], 100)
	values["average_transaction_value"] = ratio(values["revenue"], values["transactions"], 1)
	values["labor_cost_rate"] = ratio(values["labor_cost"], values["revenue"], 100)
	values["rent_to_sales_rate"] = ratio(combine(values["fixed_rent"], values["variable_rent"], nil, false), values["revenue"], 100)
	values["occupancy_cash_cost_rate"] = ratio(values["occupancy_cash_cost"], values["revenue"], 100)
	values["store_contribution_margin"] = ratio(values["store_contribution"], values["revenue"], 100)
	if r.AverageDailyAreaSqm != nil {
		values["average_daily_area_sqm"] = r.AverageDailyAreaSqm
		values["sales_per_sqm"] = ratio(values["revenue"], r.AverageDailyAreaSqm, 1)
	} else {
		values["average_daily_area_sqm"] = nil
		values["sales_per_sqm"] = nil
	}
	if r.ObservedStoreDays > 0 {
		v := valueOrNil(values["revenue"])
		if v != nil {
			x := *v / float64(r.ObservedStoreDays)
			values["revenue_per_store_day"] = &x
		}
	}
	for _, d := range definitions {
		v := values[d.Code]
		available := availableFor(d.Code, facts)
		status := StatusComplete
		reason := ""
		if len(facts) == 0 {
			status = StatusUnavailable
			reason = "no_facts"
			v = nil
		} else if available < len(facts) {
			status = StatusPartial
			v = nil
			reason = "missing_required_field"
		}
		if isZeroDenominatorMetric(d.Code) {
			if v == nil && reason == "" {
				status = StatusUnavailable
				reason = "zero_denominator"
			}
		}
		if d.Code == "revenue_per_store_day" && len(facts) == 0 {
			status, reason, v = StatusUnavailable, "zero_denominator", nil
		}
		r.KPIs[d.Code] = KPIValue{Value: roundPtr(v), Unit: d.Unit, Status: status, FormulaVersion: FormulaVersion, RequiredFields: append([]string(nil), d.RequiredFields...), AvailableFactCount: available, FactCount: len(facts), Reason: reason}
	}
	issues := map[string]bool{}
	for _, f := range facts {
		if f.DataQualityStatus == "invalid" {
			issues["data_quality_invalid"] = true
		}
		if f.MappingStatus != "" && f.MappingStatus != "mapped" {
			issues["mapping_"+f.MappingStatus] = true
		}
	}
	for issue := range issues {
		r.DataQualityIssues = append(r.DataQualityIssues, issue)
	}
	sort.Strings(r.DataQualityIssues)
	r.DecisionReady = len(r.DataQualityIssues) == 0 && len(facts) > 0 && allCoreComplete(r.KPIs)
	return r
}

func isZeroDenominatorMetric(code string) bool {
	switch code {
	case "gross_margin_rate", "conversion_rate", "average_transaction_value", "labor_cost_rate", "rent_to_sales_rate", "occupancy_cash_cost_rate", "store_contribution_margin", "sales_per_sqm":
		return true
	default:
		return false
	}
}

func allCoreComplete(values map[string]KPIValue) bool {
	for _, definition := range definitions {
		if values[definition.Code].Status != StatusComplete {
			return false
		}
	}
	return true
}

// EvaluateStorePeriod computes the retail-kpi-v1 KPI set for one store's
// period facts at store grain. The monthly four-wall view feeds this with
// its single period row so both engines speak the same semantic layer;
// zero denominators stay null, never fabricated.
func EvaluateStorePeriod(facts []DailyFact) map[string]KPIValue {
	if len(facts) == 0 {
		return map[string]KPIValue{}
	}
	first := facts[0]
	return aggregateGroup(facts, "store", first.StoreID, first.StoreCode, first.Currency).KPIs
}

func groupKey(f DailyFact, groupBy string) (string, string) {
	switch groupBy {
	case "region":
		return f.Region, f.Region
	case "brand":
		return f.Brand, f.Brand
	case "store":
		return f.StoreID, f.StoreCode + " - " + f.StoreName
	default:
		return "total", "Total"
	}
}

func sumField(facts []DailyFact, field string) *float64 {
	total := 0.0
	for _, f := range facts {
		v := fieldValue(f, field)
		if v == nil {
			return nil
		}
		total += *v
	}
	return &total
}
func availableFor(code string, facts []DailyFact) int {
	count := 0
	for _, f := range facts {
		if code == "sales_per_sqm" || code == "average_daily_area_sqm" {
			if f.AreaSqm != nil && f.Revenue != nil {
				count++
			}
			continue
		}
		ok := true
		for _, field := range required(code) {
			if fieldValue(f, field) == nil {
				ok = false
				break
			}
		}
		if ok {
			count++
		}
	}
	return count
}
func required(code string) []string {
	for _, d := range definitions {
		if d.Code == code {
			return d.RequiredFields
		}
	}
	return nil
}
func fieldValue(f DailyFact, field string) *float64 {
	switch field {
	case "revenue":
		return f.Revenue
	case "gross_profit":
		return f.GrossProfit
	case "transactions":
		return f.Transactions
	case "footfall":
		return f.Footfall
	case "area_sqm":
		return f.AreaSqm
	case "labor_cost":
		return f.LaborCost
	case "fixed_rent":
		return f.FixedRent
	case "variable_rent":
		return f.VariableRent
	case "non_lease_cost":
		return f.NonLeaseCost
	case "other_controllable_cost":
		return f.OtherControllableCost
	}
	return nil
}
func combine(a, b, c *float64, subtract bool) *float64 {
	if a == nil || b == nil || (subtract && c == nil) {
		return nil
	}
	v := *a
	if subtract {
		v -= *b
		v -= *c
	} else {
		v += *b
		if c != nil {
			v += *c
		}
	}
	return &v
}
func ratio(n, d *float64, multiplier float64) *float64 {
	if n == nil || d == nil || *d == 0 {
		return nil
	}
	v := *n / *d * multiplier
	return &v
}
func valueOrNil(v *float64) *float64 { return v }
func roundPtr(v *float64) *float64 {
	if v == nil {
		return nil
	}
	x := math.Round(*v*100) / 100
	return &x
}
func inclusiveDays(from, to time.Time) int {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return 0
	}
	return int(to.Sub(from).Hours()/24) + 1
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
