package reporting

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/repository"
)

type AmortizationRow struct {
	GroupKey            string  `json:"group_key"`
	GroupLabel          string  `json:"group_label"`
	ContractID          string  `json:"contract_id,omitempty"`
	ContractNumber      string  `json:"contract_number,omitempty"`
	ContractName        string  `json:"contract_name,omitempty"`
	StoreName           string  `json:"store_name,omitempty"`
	PeriodKey           string  `json:"period_key"`
	PeriodStart         string  `json:"period_start"`
	PeriodEnd           string  `json:"period_end"`
	OpeningLiability    float64 `json:"opening_liability"`
	InterestExpense     float64 `json:"interest_expense"`
	Payment             float64 `json:"payment"`
	PrepaidPayment      float64 `json:"prepaid_payment"`
	LiabilityAdjustment float64 `json:"liability_adjustment"`
	ClosingLiability    float64 `json:"closing_liability"`
	OpeningROUAsset     float64 `json:"opening_rou_asset"`
	Depreciation        float64 `json:"depreciation"`
	Impairment          float64 `json:"impairment"`
	ROUAdjustment       float64 `json:"rou_adjustment"`
	ClosingROUAsset     float64 `json:"closing_rou_asset"`
	VariableRentExpense float64 `json:"variable_rent_expense"`
	NonLeaseExpense     float64 `json:"non_lease_expense"`
	PnLAdjustment       float64 `json:"pnl_adjustment"`
	Currency            string  `json:"currency,omitempty"`
}

type amortizationBucket struct {
	contractID, contractNumber, contractName, storeName, currency string
	periodKey                                                     string
	periodStart, periodEnd                                        time.Time
	openingLiability, openingROU                                  float64
	interest, payment, prepaidPayment                             float64
	liabilityAdjustment, closingLiability                         float64
	depreciation, impairment, rouAdjustment, closingROU           float64
	variableRent, nonLease, pnlAdjustment                         float64
	seen                                                          bool
}

func projectAmortization(snapshot *Snapshot, request ProjectionRequest) (ProjectionResult, error) {
	if request.View == "" {
		request.View = ViewSummary
	}
	if request.Granularity == "" {
		request.Granularity = GranularityMonth
	}
	if request.EndDate.Before(request.StartDate) {
		return ProjectionResult{}, fmt.Errorf("end date must not be before start date")
	}
	if request.View != ViewContract && request.View != ViewStore && request.View != ViewSummary && request.View != ViewTag {
		return ProjectionResult{}, fmt.Errorf("invalid amortization view %q", request.View)
	}
	if request.Granularity != GranularityDay && request.Granularity != GranularityMonth && request.Granularity != GranularityQuarter && request.Granularity != GranularityHalfYear && request.Granularity != GranularityYear {
		return ProjectionResult{}, fmt.Errorf("invalid amortization granularity %q", request.Granularity)
	}
	if request.ExchangeRate < 0 {
		return ProjectionResult{}, fmt.Errorf("exchange rate must be greater than zero")
	}

	tagFilters := normalizeProjectionTokens(request.Tags)
	contractTags := make(map[string][]string)
	contractRows := make([]amortizationBucket, 0)
	for index := range snapshot.Contracts {
		fact := &snapshot.Contracts[index]
		contract := fact.Contract
		if !matchesProjectionFilters(contract, request, tagFilters) {
			continue
		}
		contractTags[contract.ID] = splitProjectionTags(contract.Tags)
		rate := fact.DiscountRate
		if request.Rate != nil {
			rate = *request.Rate
		}
		payments := repository.ToIFRS16Payments(fact.PaymentSchedules)
		calculation, err := calculateContract(fact, payments, rate)
		if err != nil {
			continue
		}
		buckets := make(map[string]*amortizationBucket)
		for _, entry := range calculation.DailyAmortization {
			if entry.Date.Before(request.StartDate) || entry.Date.After(request.EndDate) {
				continue
			}
			key := projectionBucketKey(entry.Date, request.Granularity)
			bucket := ensureAmortizationBucket(buckets, key, entry.Date, request.Granularity, contract)
			if !bucket.seen {
				bucket.openingLiability = entry.OpeningLiability
				bucket.openingROU = entry.OpeningROUAsset
				bucket.seen = true
			}
			bucket.interest += entry.InterestExpense
			bucket.payment += entry.Payment
			bucket.prepaidPayment += entry.PrepaidPayment
			bucket.liabilityAdjustment += entry.LiabilityAdjustment
			bucket.closingLiability = entry.ClosingLiability
			bucket.depreciation += entry.Depreciation
			bucket.rouAdjustment += entry.ROUAdjustment
			bucket.closingROU = entry.ClosingROUAsset
			bucket.variableRent += entry.VariableRentExpense
			bucket.nonLease += entry.NonLeaseExpense
		}
		for _, adjustment := range fact.EventAdjustments {
			if adjustment.EffectiveDate.Before(request.StartDate) || adjustment.EffectiveDate.After(request.EndDate) {
				continue
			}
			key := projectionBucketKey(adjustment.EffectiveDate, request.Granularity)
			bucket := ensureAmortizationBucket(buckets, key, adjustment.EffectiveDate, request.Granularity, contract)
			if adjustment.AdjustmentType == "impairment" {
				impairment := math.Abs(adjustment.ROUAdjustment)
				if impairment == 0 {
					impairment = adjustment.PnLLoss
				}
				bucket.impairment += impairment
			} else {
				bucket.liabilityAdjustment += adjustment.LiabilityAdjustment
				bucket.rouAdjustment += adjustment.ROUAdjustment
			}
			bucket.pnlAdjustment += adjustment.PnLGain - adjustment.PnLLoss
		}
		for _, bucket := range buckets {
			contractRows = append(contractRows, *bucket)
		}
	}

	rows := aggregateAmortization(contractRows, contractTags, request)
	if request.ExchangeRate > 0 {
		for index := range rows {
			applyProjectionExchangeRate(&rows[index], request.ExchangeRate)
		}
	}
	var rate any
	if request.Rate != nil {
		rate = *request.Rate
	}
	return ProjectionResult{Payload: projectionPayload(snapshot, rows, map[string]any{
		"view": request.View, "granularity": request.Granularity,
		"start_date": request.StartDate.Format("2006-01-02"), "end_date": request.EndDate.Format("2006-01-02"),
		"report_currency": request.ReportCurrency, "exchange_rate": request.ExchangeRate,
		"discount_rate_override": rate,
	})}, nil
}

func ensureAmortizationBucket(buckets map[string]*amortizationBucket, key string, date time.Time, granularity string, contract *repository.Contract) *amortizationBucket {
	if bucket := buckets[key]; bucket != nil {
		return bucket
	}
	start, end := projectionBucketRange(date, granularity)
	bucket := &amortizationBucket{
		contractID: contract.ID, contractNumber: contract.ContractNumber,
		contractName: contract.ContractName, storeName: contract.StoreName,
		currency: contract.Currency, periodKey: key, periodStart: start, periodEnd: end,
	}
	buckets[key] = bucket
	return bucket
}

func matchesProjectionFilters(contract *repository.Contract, request ProjectionRequest, tagFilters []string) bool {
	if request.ContractID != "" && contract.ID != request.ContractID {
		return false
	}
	if request.Store != "" {
		storeID := ""
		if contract.StoreID != nil {
			storeID = *contract.StoreID
		}
		if !strings.Contains(contract.StoreName, request.Store) && !strings.Contains(storeID, request.Store) {
			return false
		}
	}
	return len(tagFilters) == 0 || matchesAnyTag(splitProjectionTags(contract.Tags), tagFilters)
}

func aggregateAmortization(contractRows []amortizationBucket, contractTags map[string][]string, request ProjectionRequest) []AmortizationRow {
	rows := make(map[string]*AmortizationRow)
	currencySet := make(map[string]struct{})
	for _, bucket := range contractRows {
		if bucket.currency != "" {
			currencySet[bucket.currency] = struct{}{}
		}
	}
	separateCurrencies := request.View != ViewContract && len(currencySet) > 1
	for _, bucket := range contractRows {
		groups := projectionGroups(bucket, contractTags, request.View)
		for _, group := range groups {
			key := group.key + "|" + bucket.periodKey
			if separateCurrencies {
				key = group.key + "|" + bucket.currency + "|" + bucket.periodKey
			}
			row := rows[key]
			if row == nil {
				currency := request.ReportCurrency
				if currency == "" && request.View == ViewContract {
					currency = bucket.currency
				}
				if currency == "" {
					currency = bucket.currency
				}
				groupKey := group.key
				if separateCurrencies {
					groupKey += "|" + bucket.currency
				}
				row = &AmortizationRow{
					GroupKey: groupKey, GroupLabel: group.label, StoreName: group.store,
					PeriodKey: bucket.periodKey, PeriodStart: bucket.periodStart.Format("2006-01-02"),
					PeriodEnd: bucket.periodEnd.Format("2006-01-02"), Currency: currency,
				}
				if request.View == ViewContract {
					row.ContractID, row.ContractNumber, row.ContractName = bucket.contractID, bucket.contractNumber, bucket.contractName
				}
				rows[key] = row
			}
			addAmortization(row, bucket)
		}
	}
	result := make([]AmortizationRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, *row)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].GroupKey != result[j].GroupKey {
			return result[i].GroupKey < result[j].GroupKey
		}
		return result[i].PeriodStart < result[j].PeriodStart
	})
	return result
}

type projectionGroup struct{ key, label, store string }

func projectionGroups(bucket amortizationBucket, contractTags map[string][]string, view string) []projectionGroup {
	switch view {
	case ViewContract:
		return []projectionGroup{{bucket.contractID, bucket.contractNumber + " - " + bucket.contractName, bucket.storeName}}
	case ViewStore:
		store := fallbackProjectionStore(bucket.storeName)
		return []projectionGroup{{store, store, store}}
	case ViewTag:
		groups := make([]projectionGroup, 0, len(contractTags[bucket.contractID]))
		for _, tag := range contractTags[bucket.contractID] {
			groups = append(groups, projectionGroup{tag, tag, ""})
		}
		return groups
	default:
		return []projectionGroup{{"summary", "汇总", ""}}
	}
}

func addAmortization(row *AmortizationRow, bucket amortizationBucket) {
	row.OpeningLiability += bucket.openingLiability
	row.InterestExpense += bucket.interest
	row.Payment += bucket.payment
	row.PrepaidPayment += bucket.prepaidPayment
	row.LiabilityAdjustment += bucket.liabilityAdjustment
	row.ClosingLiability += bucket.closingLiability
	row.OpeningROUAsset += bucket.openingROU
	row.Depreciation += bucket.depreciation
	row.Impairment += bucket.impairment
	row.ROUAdjustment += bucket.rouAdjustment
	row.ClosingROUAsset += bucket.closingROU
	row.VariableRentExpense += bucket.variableRent
	row.NonLeaseExpense += bucket.nonLease
	row.PnLAdjustment += bucket.pnlAdjustment
}

func applyProjectionExchangeRate(row *AmortizationRow, rate float64) {
	row.OpeningLiability *= rate
	row.InterestExpense *= rate
	row.Payment *= rate
	row.PrepaidPayment *= rate
	row.LiabilityAdjustment *= rate
	row.ClosingLiability *= rate
	row.OpeningROUAsset *= rate
	row.Depreciation *= rate
	row.Impairment *= rate
	row.ROUAdjustment *= rate
	row.ClosingROUAsset *= rate
	row.VariableRentExpense *= rate
	row.NonLeaseExpense *= rate
	row.PnLAdjustment *= rate
}
