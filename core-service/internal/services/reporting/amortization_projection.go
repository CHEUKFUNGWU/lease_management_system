package reporting

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/money"
	"github.com/lease-management-system/core-service/internal/repository"
)

type AmortizationRow struct {
	GroupKey            string       `json:"group_key"`
	GroupLabel          string       `json:"group_label"`
	ContractID          string       `json:"contract_id,omitempty"`
	ContractNumber      string       `json:"contract_number,omitempty"`
	ContractName        string       `json:"contract_name,omitempty"`
	StoreName           string       `json:"store_name,omitempty"`
	PeriodKey           string       `json:"period_key"`
	PeriodStart         string       `json:"period_start"`
	PeriodEnd           string       `json:"period_end"`
	OpeningLiability    money.Amount `json:"opening_liability"`
	InterestExpense     money.Amount `json:"interest_expense"`
	Payment             money.Amount `json:"payment"`
	PrepaidPayment      money.Amount `json:"prepaid_payment"`
	LiabilityAdjustment money.Amount `json:"liability_adjustment"`
	ClosingLiability    money.Amount `json:"closing_liability"`
	OpeningROUAsset     money.Amount `json:"opening_rou_asset"`
	Depreciation        money.Amount `json:"depreciation"`
	Impairment          money.Amount `json:"impairment"`
	ROUAdjustment       money.Amount `json:"rou_adjustment"`
	ClosingROUAsset     money.Amount `json:"closing_rou_asset"`
	VariableRentExpense money.Amount `json:"variable_rent_expense"`
	NonLeaseExpense     money.Amount `json:"non_lease_expense"`
	PnLAdjustment       money.Amount `json:"pnl_adjustment"`
	Currency            string       `json:"currency,omitempty"`
}

type amortizationBucket struct {
	contractID, contractNumber, contractName, storeName, currency string
	periodKey                                                     string
	periodStart, periodEnd                                        time.Time
	openingLiability, openingROU                                  money.Amount
	interest, payment, prepaidPayment                             money.Amount
	liabilityAdjustment, closingLiability                         money.Amount
	depreciation, impairment, rouAdjustment, closingROU           money.Amount
	variableRent, nonLease, pnlAdjustment                         money.Amount
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
			bucket.interest = bucket.interest.Add(entry.InterestExpense)
			bucket.payment = bucket.payment.Add(entry.Payment)
			bucket.prepaidPayment = bucket.prepaidPayment.Add(entry.PrepaidPayment)
			bucket.liabilityAdjustment = bucket.liabilityAdjustment.Add(entry.LiabilityAdjustment)
			bucket.closingLiability = entry.ClosingLiability
			bucket.depreciation = bucket.depreciation.Add(entry.Depreciation)
			bucket.rouAdjustment = bucket.rouAdjustment.Add(entry.ROUAdjustment)
			bucket.closingROU = entry.ClosingROUAsset
			bucket.variableRent = bucket.variableRent.Add(entry.VariableRentExpense)
			bucket.nonLease = bucket.nonLease.Add(entry.NonLeaseExpense)
		}
		for _, adjustment := range fact.EventAdjustments {
			if adjustment.EffectiveDate.Before(request.StartDate) || adjustment.EffectiveDate.After(request.EndDate) {
				continue
			}
			key := projectionBucketKey(adjustment.EffectiveDate, request.Granularity)
			bucket := ensureAmortizationBucket(buckets, key, adjustment.EffectiveDate, request.Granularity, contract)
			if adjustment.AdjustmentType == "impairment" {
				// Event adjustment amounts are stored decimals read as float64;
				// the conversion happens once, at this seam.
				impairment := money.NewFromFloat(math.Abs(adjustment.ROUAdjustment))
				if impairment.IsZero() {
					impairment = money.NewFromFloat(adjustment.PnLLoss)
				}
				bucket.impairment = bucket.impairment.Add(impairment)
			} else {
				bucket.liabilityAdjustment = bucket.liabilityAdjustment.Add(money.NewFromFloat(adjustment.LiabilityAdjustment))
				bucket.rouAdjustment = bucket.rouAdjustment.Add(money.NewFromFloat(adjustment.ROUAdjustment))
			}
			bucket.pnlAdjustment = bucket.pnlAdjustment.Add(money.NewFromFloat(adjustment.PnLGain).Sub(money.NewFromFloat(adjustment.PnLLoss)))
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
	row.OpeningLiability = row.OpeningLiability.Add(bucket.openingLiability)
	row.InterestExpense = row.InterestExpense.Add(bucket.interest)
	row.Payment = row.Payment.Add(bucket.payment)
	row.PrepaidPayment = row.PrepaidPayment.Add(bucket.prepaidPayment)
	row.LiabilityAdjustment = row.LiabilityAdjustment.Add(bucket.liabilityAdjustment)
	row.ClosingLiability = row.ClosingLiability.Add(bucket.closingLiability)
	row.OpeningROUAsset = row.OpeningROUAsset.Add(bucket.openingROU)
	row.Depreciation = row.Depreciation.Add(bucket.depreciation)
	row.Impairment = row.Impairment.Add(bucket.impairment)
	row.ROUAdjustment = row.ROUAdjustment.Add(bucket.rouAdjustment)
	row.ClosingROUAsset = row.ClosingROUAsset.Add(bucket.closingROU)
	row.VariableRentExpense = row.VariableRentExpense.Add(bucket.variableRent)
	row.NonLeaseExpense = row.NonLeaseExpense.Add(bucket.nonLease)
	row.PnLAdjustment = row.PnLAdjustment.Add(bucket.pnlAdjustment)
}

// applyProjectionExchangeRate translates a row into the report currency. The
// rate is a policy float; each amount is multiplied at full precision.
func applyProjectionExchangeRate(row *AmortizationRow, rate float64) {
	factor := money.NewFromFloat(rate)
	row.OpeningLiability = row.OpeningLiability.Mul(factor)
	row.InterestExpense = row.InterestExpense.Mul(factor)
	row.Payment = row.Payment.Mul(factor)
	row.PrepaidPayment = row.PrepaidPayment.Mul(factor)
	row.LiabilityAdjustment = row.LiabilityAdjustment.Mul(factor)
	row.ClosingLiability = row.ClosingLiability.Mul(factor)
	row.OpeningROUAsset = row.OpeningROUAsset.Mul(factor)
	row.Depreciation = row.Depreciation.Mul(factor)
	row.Impairment = row.Impairment.Mul(factor)
	row.ROUAdjustment = row.ROUAdjustment.Mul(factor)
	row.ClosingROUAsset = row.ClosingROUAsset.Mul(factor)
	row.VariableRentExpense = row.VariableRentExpense.Mul(factor)
	row.NonLeaseExpense = row.NonLeaseExpense.Mul(factor)
	row.PnLAdjustment = row.PnLAdjustment.Mul(factor)
}
