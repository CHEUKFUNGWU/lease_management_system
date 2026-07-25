// Package budget compares a frozen measurement snapshot with what the close
// actually produced, and explains the difference.
//
// The explanation is the point. A variance number on its own invites the
// question "why", and the event-driven ledger already knows: a lease signed
// after the budget was frozen, a renewal, a rent change, an exchange movement.
// Everything the events cannot explain is reported as a residual rather than
// spread across the named causes, so the bridge always ties back to the total.
package budget

import (
	"math"
	"sort"
)

// Variance causes, in the order they are attributed. A contract touched by
// several kinds of event is attributed to the first cause that matches, so one
// contract is never counted twice.
const (
	CauseNewLease     = "new_lease"
	CauseEnded        = "ended"
	CauseRenewal      = "renewal_or_termination"
	CauseRentChange   = "rent_change"
	CauseExchangeRate = "exchange_rate"
	CauseOther        = "other"
)

// eventCause maps a lease event type to the cause it explains.
var eventCause = map[string]string{
	"renewal":              CauseRenewal,
	"early_termination":    CauseRenewal,
	"rent_change":          CauseRentChange,
	"index_update":         CauseRentChange,
	"area_adjustment":      CauseRentChange,
	"discount_rate_change": CauseOther,
	"impairment":           CauseOther,
}

// causeOrder fixes the attribution priority when a contract carries events of
// several kinds.
var causeOrder = []string{CauseRenewal, CauseRentChange, CauseExchangeRate, CauseOther}

// ContractPeriod is one contract's lease cost for a period, from either the
// budget or the actual close.
type ContractPeriod struct {
	ContractID     string
	ContractNumber string
	ContractName   string
	Currency       string
	LeaseCost      float64 // interest + depreciation
}

// Input is everything needed to explain one period's variance.
type Input struct {
	Period string
	Budget []ContractPeriod
	Actual []ContractPeriod

	// EventsByContract lists the lease event types effective in the period,
	// which is what turns a number into an explanation.
	EventsByContract map[string][]string

	// FXByContract is the exchange difference recognised in the period, by
	// contract. It is attributed to its own cause because it is a rate effect,
	// not a change in the lease.
	FXByContract map[string]float64
}

// CauseAmount is one line of the variance bridge.
type CauseAmount struct {
	Cause         string  `json:"cause"`
	Amount        float64 `json:"amount"`
	ContractCount int     `json:"contract_count"`
}

// ContractVariance is one contract's contribution, kept so any bridge line can
// be drilled into.
type ContractVariance struct {
	ContractID     string  `json:"contract_id"`
	ContractNumber string  `json:"contract_number"`
	ContractName   string  `json:"contract_name"`
	Currency       string  `json:"currency"`
	Budget         float64 `json:"budget"`
	Actual         float64 `json:"actual"`
	Variance       float64 `json:"variance"`
	Cause          string  `json:"cause"`
}

// Result is the variance and its explanation.
type Result struct {
	Period        string             `json:"period"`
	BudgetTotal   float64            `json:"budget_total"`
	ActualTotal   float64            `json:"actual_total"`
	Variance      float64            `json:"variance"`
	Bridge        []CauseAmount      `json:"bridge"`
	ByContract    []ContractVariance `json:"by_contract"`
	BridgeTiesOut bool               `json:"bridge_ties_out"`
}

// Explain compares budget with actual and attributes the difference.
//
// Variance is actual minus budget: positive means the period cost more than
// planned, which is how a reader expects to see an overspend.
func Explain(input Input) Result {
	budgetByContract := indexByContract(input.Budget)
	actualByContract := indexByContract(input.Actual)

	result := Result{Period: input.Period}
	bridgeByCause := map[string]*CauseAmount{}
	addToBridge := func(cause string, amount float64) {
		entry := bridgeByCause[cause]
		if entry == nil {
			entry = &CauseAmount{Cause: cause}
			bridgeByCause[cause] = entry
		}
		entry.Amount += amount
		entry.ContractCount++
	}

	// Every contract on either side is walked once, so nothing is dropped.
	for _, contractID := range unionOfKeys(budgetByContract, actualByContract) {
		budgeted, inBudget := budgetByContract[contractID]
		actual, inActual := actualByContract[contractID]

		identity := actual
		if !inActual {
			identity = budgeted
		}

		variance := actual.LeaseCost - budgeted.LeaseCost
		cause := attribute(contractID, inBudget, inActual, input)

		result.BudgetTotal += budgeted.LeaseCost
		result.ActualTotal += actual.LeaseCost
		addToBridge(cause, variance)
		result.ByContract = append(result.ByContract, ContractVariance{
			ContractID:     contractID,
			ContractNumber: identity.ContractNumber,
			ContractName:   identity.ContractName,
			Currency:       identity.Currency,
			Budget:         round2(budgeted.LeaseCost),
			Actual:         round2(actual.LeaseCost),
			Variance:       round2(variance),
			Cause:          cause,
		})
	}

	result.BudgetTotal = round2(result.BudgetTotal)
	result.ActualTotal = round2(result.ActualTotal)
	result.Variance = round2(result.ActualTotal - result.BudgetTotal)

	for _, cause := range []string{CauseNewLease, CauseEnded, CauseRenewal, CauseRentChange, CauseExchangeRate, CauseOther} {
		if entry, present := bridgeByCause[cause]; present {
			entry.Amount = round2(entry.Amount)
			result.Bridge = append(result.Bridge, *entry)
		}
	}

	// A bridge that does not add up to the variance explains nothing, so the
	// check travels with the result instead of being assumed.
	var bridgeSum float64
	for _, entry := range result.Bridge {
		bridgeSum += entry.Amount
	}
	result.BridgeTiesOut = math.Abs(round2(bridgeSum)-result.Variance) <= 0.05

	sort.Slice(result.ByContract, func(i, j int) bool {
		return math.Abs(result.ByContract[i].Variance) > math.Abs(result.ByContract[j].Variance)
	})
	return result
}

// attribute decides which cause explains a contract's variance.
func attribute(contractID string, inBudget, inActual bool, input Input) string {
	// A lease the budget never saw, or one the budget expected but that no
	// longer runs, is explained by its own presence rather than by an event.
	if !inBudget && inActual {
		return CauseNewLease
	}
	if inBudget && !inActual {
		return CauseEnded
	}

	causes := map[string]bool{}
	for _, eventType := range input.EventsByContract[contractID] {
		if cause, known := eventCause[eventType]; known {
			causes[cause] = true
		} else {
			causes[CauseOther] = true
		}
	}
	if math.Abs(input.FXByContract[contractID]) > 0.01 {
		causes[CauseExchangeRate] = true
	}

	for _, cause := range causeOrder {
		if causes[cause] {
			return cause
		}
	}
	return CauseOther
}

func indexByContract(rows []ContractPeriod) map[string]ContractPeriod {
	indexed := make(map[string]ContractPeriod, len(rows))
	for _, row := range rows {
		existing := indexed[row.ContractID]
		existing.ContractID = row.ContractID
		existing.ContractNumber = row.ContractNumber
		existing.ContractName = row.ContractName
		existing.Currency = row.Currency
		existing.LeaseCost += row.LeaseCost
		indexed[row.ContractID] = existing
	}
	return indexed
}

func unionOfKeys(left, right map[string]ContractPeriod) []string {
	seen := map[string]bool{}
	keys := make([]string, 0, len(left)+len(right))
	for key := range left {
		seen[key] = true
		keys = append(keys, key)
	}
	for key := range right {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	return keys
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}
