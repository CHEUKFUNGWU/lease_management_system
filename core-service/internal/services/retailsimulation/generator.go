package retailsimulation

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	GeneratorVersion        = "retail-sim-v1"
	DefaultSeed       int64 = 20260812
	DefaultDateFrom         = "2026-01-01"
	DefaultDateTo           = "2026-06-30"
	DefaultStoreCount       = 60
	DefaultCurrency         = "CNY"
)

type Input struct {
	Seed       int64
	DateFrom   string
	DateTo     string
	StoreCount int
}

type StorePlan struct {
	Index    int     `json:"index"`
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	Brand    string  `json:"brand"`
	Region   string  `json:"region"`
	AreaSqm  float64 `json:"area_sqm"`
	Currency string  `json:"currency"`
}

type FactPlan struct {
	StoreIndex            int     `json:"store_index"`
	StoreCode             string  `json:"store_code"`
	BusinessDate          string  `json:"business_date"`
	Currency              string  `json:"currency"`
	Revenue               float64 `json:"revenue"`
	GrossProfit           float64 `json:"gross_profit"`
	Transactions          float64 `json:"transactions"`
	Footfall              float64 `json:"footfall"`
	AreaSqm               float64 `json:"area_sqm"`
	LaborCost             float64 `json:"labor_cost"`
	FixedRent             float64 `json:"fixed_rent"`
	VariableRent          float64 `json:"variable_rent"`
	NonLeaseCost          float64 `json:"non_lease_cost"`
	OtherControllableCost float64 `json:"other_controllable_cost"`
	SourceRecordID        string  `json:"source_record_id"`
}

type Anomaly struct {
	ID                string `json:"id"`
	Type              string `json:"type"`
	StoreCode         string `json:"store_code"`
	DateFrom          string `json:"date_from"`
	DateTo            string `json:"date_to"`
	ExpectedDirection string `json:"expected_direction"`
	Description       string `json:"description"`
}

type Plan struct {
	DatasetVersion   string         `json:"dataset_version"`
	GeneratorVersion string         `json:"generator_version"`
	Seed             int64          `json:"seed"`
	DateFrom         string         `json:"date_from"`
	DateTo           string         `json:"date_to"`
	StoreCount       int            `json:"store_count"`
	FactCount        int            `json:"fact_count"`
	Parameters       map[string]any `json:"parameters"`
	Stores           []StorePlan    `json:"stores"`
	Facts            []FactPlan     `json:"facts"`
	Anomalies        []Anomaly      `json:"anomaly_manifest"`
	BusinessSHA256   string         `json:"business_sha256"`
}

func Normalize(input Input) (Input, time.Time, time.Time, error) {
	if input.Seed == 0 {
		input.Seed = DefaultSeed
	}
	if strings.TrimSpace(input.DateFrom) == "" {
		input.DateFrom = DefaultDateFrom
	}
	if strings.TrimSpace(input.DateTo) == "" {
		input.DateTo = DefaultDateTo
	}
	if input.StoreCount == 0 {
		input.StoreCount = DefaultStoreCount
	}
	from, err := time.Parse("2006-01-02", strings.TrimSpace(input.DateFrom))
	if err != nil {
		return Input{}, time.Time{}, time.Time{}, fmt.Errorf("date_from must be YYYY-MM-DD")
	}
	to, err := time.Parse("2006-01-02", strings.TrimSpace(input.DateTo))
	if err != nil {
		return Input{}, time.Time{}, time.Time{}, fmt.Errorf("date_to must be YYYY-MM-DD")
	}
	days := int(to.Sub(from).Hours()/24) + 1
	if to.Before(from) || days < 28 || days > 366 {
		return Input{}, time.Time{}, time.Time{}, fmt.Errorf("date range must be between 28 and 366 days")
	}
	if input.StoreCount < 10 || input.StoreCount > 100 {
		return Input{}, time.Time{}, time.Time{}, fmt.Errorf("store_count must be between 10 and 100")
	}
	input.DateFrom = from.Format("2006-01-02")
	input.DateTo = to.Format("2006-01-02")
	return input, from, to, nil
}

func Build(legalEntityID string, input Input) (*Plan, error) {
	if strings.TrimSpace(legalEntityID) == "" {
		return nil, fmt.Errorf("legal entity is required")
	}
	normalized, from, to, err := Normalize(input)
	if err != nil {
		return nil, err
	}
	parameters := map[string]any{
		"seed": normalized.Seed, "date_from": normalized.DateFrom, "date_to": normalized.DateTo,
		"store_count": normalized.StoreCount, "generator_version": GeneratorVersion,
		"legal_entity_id": legalEntityID,
	}
	parameterJSON, _ := json.Marshal(parameters)
	datasetDigest := sha256.Sum256(parameterJSON)
	datasetVersion := GeneratorVersion + "-" + hex.EncodeToString(datasetDigest[:])[:16]
	datasetShort := strings.ToUpper(hex.EncodeToString(datasetDigest[:])[:8])

	brands := []string{"Northwind", "Harbor", "Mosaic", "Summit"}
	regions := []string{"North", "East", "South", "West"}
	stores := make([]StorePlan, 0, normalized.StoreCount)
	for index := 0; index < normalized.StoreCount; index++ {
		stores = append(stores, StorePlan{
			Index: index, Code: fmt.Sprintf("SIM-%s-%03d", datasetShort, index+1),
			Name: fmt.Sprintf("Simulation Store %03d", index+1), Brand: brands[index%len(brands)],
			Region: regions[index%len(regions)], AreaSqm: round2(90 + float64((index*37)%180)), Currency: DefaultCurrency,
		})
	}

	days := int(to.Sub(from).Hours()/24) + 1
	anomalies := buildAnomalies(stores, from, days)
	facts := make([]FactPlan, 0, normalized.StoreCount*days)
	for storeIndex, store := range stores {
		for dayIndex := 0; dayIndex < days; dayIndex++ {
			date := from.AddDate(0, 0, dayIndex)
			fact := baselineFact(normalized.Seed, storeIndex, store, date, dayIndex)
			applyAnomalies(&fact, anomalies, normalized.Seed, storeIndex, store, date, from)
			fact.SourceRecordID = fmt.Sprintf("%s|%s|%s", datasetVersion, store.Code, fact.BusinessDate)
			facts = append(facts, fact)
		}
	}
	plan := &Plan{
		DatasetVersion: datasetVersion, GeneratorVersion: GeneratorVersion, Seed: normalized.Seed,
		DateFrom: normalized.DateFrom, DateTo: normalized.DateTo, StoreCount: normalized.StoreCount,
		FactCount: len(facts), Parameters: parameters, Stores: stores, Facts: facts, Anomalies: anomalies,
	}
	plan.BusinessSHA256 = businessHash(plan)
	return plan, nil
}

func PayloadSHA256(legalEntityID string, input Input) (string, Input, error) {
	normalized, _, _, err := Normalize(input)
	if err != nil {
		return "", Input{}, err
	}
	payload := struct {
		LegalEntityID    string `json:"legal_entity_id"`
		GeneratorVersion string `json:"generator_version"`
		Input            Input  `json:"input"`
	}{legalEntityID, GeneratorVersion, normalized}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", Input{}, err
	}
	digest := sha256.Sum256(b)
	return hex.EncodeToString(digest[:]), normalized, nil
}

func businessHash(plan *Plan) string {
	payload := struct {
		DatasetVersion   string      `json:"dataset_version"`
		GeneratorVersion string      `json:"generator_version"`
		Seed             int64       `json:"seed"`
		DateFrom         string      `json:"date_from"`
		DateTo           string      `json:"date_to"`
		Stores           []StorePlan `json:"stores"`
		Facts            []FactPlan  `json:"facts"`
		Anomalies        []Anomaly   `json:"anomalies"`
	}{plan.DatasetVersion, plan.GeneratorVersion, plan.Seed, plan.DateFrom, plan.DateTo, plan.Stores, plan.Facts, plan.Anomalies}
	b, _ := json.Marshal(payload)
	digest := sha256.Sum256(b)
	return hex.EncodeToString(digest[:])
}

func deterministicUnit(seed int64, storeIndex, dayIndex int, salt string) float64 {
	key := fmt.Sprintf("%d|%d|%d|%s", seed, storeIndex, dayIndex, salt)
	digest := sha256.Sum256([]byte(key))
	value := binary.BigEndian.Uint64(digest[:8])
	return float64(value) / float64(math.MaxUint64)
}

func baselineFact(seed int64, storeIndex int, store StorePlan, date time.Time, dayIndex int) FactPlan {
	weekdayFactor := []float64{0.79, 0.98, 1.01, 1.03, 1.08, 0.96, 0.82}[int(date.Weekday())]
	seasonFactor := 1 + 0.08*math.Sin(2*math.Pi*float64(date.YearDay())/365)
	trendFactor := 1 + 0.0008*float64(dayIndex)
	sizeFactor := 0.82 + float64((storeIndex*17)%31)/100
	noise := 0.96 + 0.08*deterministicUnit(seed, storeIndex, dayIndex, "footfall")
	footfall := math.Round(340 * sizeFactor * weekdayFactor * seasonFactor * trendFactor * noise)
	conversion := 0.135 + 0.025*deterministicUnit(seed, storeIndex, dayIndex, "conversion")
	transactions := math.Min(footfall, math.Round(footfall*conversion))
	aov := 68 + 14*deterministicUnit(seed, storeIndex, dayIndex, "aov") + float64(storeIndex%5)*2
	revenue := math.Round(transactions*aov*100) / 100
	margin := 0.27 + 0.08*deterministicUnit(seed, storeIndex, dayIndex, "margin")
	grossProfit := round2(revenue * margin)
	labor := round2(revenue * (0.105 + 0.025*deterministicUnit(seed, storeIndex, dayIndex, "labor")))
	fixedRent := round2(store.AreaSqm * (1.55 + 0.35*deterministicUnit(seed, storeIndex, dayIndex, "rent")))
	return FactPlan{
		StoreIndex: storeIndex, StoreCode: store.Code, BusinessDate: date.Format("2006-01-02"), Currency: store.Currency,
		Revenue: revenue, GrossProfit: grossProfit, Transactions: transactions, Footfall: footfall, AreaSqm: store.AreaSqm,
		LaborCost: labor, FixedRent: fixedRent, VariableRent: round2(revenue * 0.012), NonLeaseCost: round2(revenue * 0.009),
		OtherControllableCost: round2(revenue * 0.055),
	}
}

func buildAnomalies(stores []StorePlan, from time.Time, days int) []Anomaly {
	types := []struct {
		id, kind, direction, description string
	}{
		{"traffic-decline", "footfall_continuous_decline", "down", "footfall and transactions decline on each successive day"},
		{"conversion-drop", "conversion_rate_drop", "down", "transactions fall while footfall remains on baseline"},
		{"aov-drop", "average_ticket_drop", "down", "revenue falls relative to transactions"},
		{"margin-compression", "gross_margin_compression", "down", "gross profit rate compresses while revenue remains stable"},
		{"labor-spike", "labor_cost_spike", "up", "labor cost is elevated versus baseline"},
		{"occupancy-burden", "occupancy_cost_burden", "up", "fixed rent burden is elevated versus baseline"},
	}
	result := make([]Anomaly, 0, len(types))
	window := maxInt(3, days/24)
	for index, item := range types {
		start := int(float64(days-window) * float64(index+1) / float64(len(types)+1))
		if start < 1 {
			start = 1
		}
		if start+window > days {
			start = days - window
		}
		result = append(result, Anomaly{ID: item.id, Type: item.kind, StoreCode: stores[index+1].Code,
			DateFrom: from.AddDate(0, 0, start).Format("2006-01-02"), DateTo: from.AddDate(0, 0, start+window-1).Format("2006-01-02"),
			ExpectedDirection: item.direction, Description: item.description})
	}
	return result
}

func applyAnomalies(fact *FactPlan, anomalies []Anomaly, seed int64, storeIndex int, store StorePlan, date time.Time, from time.Time) {
	for index, anomaly := range anomalies {
		if storeIndex != index+1 || fact.BusinessDate < anomaly.DateFrom || fact.BusinessDate > anomaly.DateTo {
			continue
		}
		day := int(date.Sub(from).Hours() / 24)
		start, _ := time.Parse("2006-01-02", anomaly.DateFrom)
		startDay := int(start.Sub(from).Hours() / 24)
		progress := day - startDay + 1
		grossMarginRate := safeRatio(fact.GrossProfit, fact.Revenue)
		laborCostRate := safeRatio(fact.LaborCost, fact.Revenue)
		variableRentRate := safeRatio(fact.VariableRent, fact.Revenue)
		nonLeaseCostRate := safeRatio(fact.NonLeaseCost, fact.Revenue)
		otherCostRate := safeRatio(fact.OtherControllableCost, fact.Revenue)
		switch anomaly.Type {
		case "footfall_continuous_decline":
			anchor := baselineFact(seed, storeIndex, store, start, startDay).Footfall
			step := math.Max(1, math.Round(anchor*0.06))
			target := math.Inf(1)
			for offset := 0; offset < progress; offset++ {
				baselineDate := start.AddDate(0, 0, offset)
				baselineValue := baselineFact(seed, storeIndex, store, baselineDate, startDay+offset).Footfall
				candidate := baselineValue - step*float64(progress-1-offset)
				if candidate < target {
					target = candidate
				}
			}
			fact.Footfall = math.Max(1, target)
			fact.Transactions = math.Min(fact.Footfall, math.Max(1, math.Round(fact.Footfall*0.15)))
			fact.Revenue = proportionalRevenue(fact.Revenue, fact.Transactions, baselineFact(seed, storeIndex, store, date, day).Transactions)
			recomputeSalesRatios(fact, grossMarginRate, laborCostRate, variableRentRate, nonLeaseCostRate, otherCostRate)
		case "conversion_rate_drop":
			oldTransactions := fact.Transactions
			fact.Transactions = math.Min(fact.Footfall, math.Round(fact.Transactions*0.42))
			fact.Revenue = proportionalRevenue(fact.Revenue, fact.Transactions, oldTransactions)
			recomputeSalesRatios(fact, grossMarginRate, laborCostRate, variableRentRate, nonLeaseCostRate, otherCostRate)
		case "average_ticket_drop":
			fact.Revenue = round2(fact.Revenue * 0.58)
			recomputeSalesRatios(fact, grossMarginRate, laborCostRate, variableRentRate, nonLeaseCostRate, otherCostRate)
		case "gross_margin_compression":
			fact.GrossProfit = round2(fact.GrossProfit * 0.48)
		case "labor_cost_spike":
			fact.LaborCost = round2(fact.LaborCost * 2.6)
		case "occupancy_cost_burden":
			fact.FixedRent = round2(fact.FixedRent * 2.8)
		}
	}
}

func safeRatio(numerator, denominator float64) float64 {
	if denominator == 0 {
		return 0
	}
	return numerator / denominator
}

func proportionalRevenue(revenue, transactions, baselineTransactions float64) float64 {
	if baselineTransactions <= 0 {
		return 0
	}
	return round2(revenue * transactions / baselineTransactions)
}

func recomputeSalesRatios(fact *FactPlan, grossMarginRate, laborCostRate, variableRentRate, nonLeaseCostRate, otherCostRate float64) {
	fact.GrossProfit = round2(fact.Revenue * grossMarginRate)
	fact.LaborCost = round2(fact.Revenue * laborCostRate)
	fact.VariableRent = round2(fact.Revenue * variableRentRate)
	fact.NonLeaseCost = round2(fact.Revenue * nonLeaseCostRate)
	fact.OtherControllableCost = round2(fact.Revenue * otherCostRate)
}

func round2(value float64) float64 { return math.Round(value*100) / 100 }

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
