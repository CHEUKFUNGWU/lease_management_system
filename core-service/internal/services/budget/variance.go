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
	CauseNewLease       = "new_lease"
	CauseEnded          = "ended"
	CauseRenewal        = "renewal_or_termination"
	CauseRentChange     = "rent_change"
	CauseIndexChange    = "index_adjustment"
	CauseDiscountRate   = "discount_rate"
	CausePaymentTiming  = "payment_timing"
	CauseDataCorrection = "data_correction"
	CauseExchangeRate   = "exchange_rate"
	CauseOther          = "other"
)

// eventCause maps a lease event type to the cause it explains.
var eventCause = map[string]string{
	"renewal":               CauseRenewal,
	"early_termination":     CauseRenewal,
	"termination":           CauseRenewal,
	"store_closure":         CauseEnded,
	"closure":               CauseEnded,
	"rent_change":           CauseRentChange,
	"index_update":          CauseIndexChange,
	"cpi_adjustment":        CauseIndexChange,
	"area_adjustment":       CauseRentChange,
	"discount_rate_change":  CauseDiscountRate,
	"payment_timing_change": CausePaymentTiming,
	"payment_date_change":   CausePaymentTiming,
	"data_correction":       CauseDataCorrection,
	"impairment":            CauseOther,
}

// causeOrder fixes the attribution priority when a contract carries events of
// several kinds.
var causeOrder = []string{
	CauseRenewal, CauseRentChange, CauseIndexChange, CauseDiscountRate,
	CausePaymentTiming, CauseExchangeRate, CauseDataCorrection, CauseOther,
}

// ContractPeriod is one contract's lease cost for a period, from either the
// budget or the actual close.
type ContractPeriod struct {
	ContractID     string
	ContractNumber string
	ContractName   string
	Currency       string
	LeaseCost      float64 // interest + depreciation
	TotalPayment   float64
}

// Input is everything needed to explain one period's variance.
type Input struct {
	Period string
	Budget []ContractPeriod
	Actual []ContractPeriod
	// MaterialityThreshold is supplied by accounting policy. Zero means no
	// variance is suppressed when the policy has not been configured.
	MaterialityThreshold float64
	// TieOutTolerance is the approved rounding tolerance for bridge checks.
	TieOutTolerance float64

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
	Explanation    string  `json:"explanation,omitempty"`
	OwnerName      string  `json:"owner_name,omitempty"`
	DueDate        string  `json:"due_date,omitempty"`
	ActionStatus   string  `json:"action_status,omitempty"`
	IsOverdue      bool    `json:"is_overdue"`
}

// Result is the variance and its explanation.
type Result struct {
	Period              string             `json:"period"`
	BudgetTotal         float64            `json:"budget_total"`
	ActualTotal         float64            `json:"actual_total"`
	Variance            float64            `json:"variance"`
	Bridge              []CauseAmount      `json:"bridge"`
	ByContract          []ContractVariance `json:"by_contract"`
	BridgeTiesOut       bool               `json:"bridge_ties_out"`
	ExplainedCount      int                `json:"explained_count"`
	VarianceCount       int                `json:"variance_count"`
	ExplanationCoverage float64            `json:"explanation_coverage"`
	OpenActionAmount    float64            `json:"open_action_amount"`
	OpenActionCount     int                `json:"open_action_count"`
}

// VersionComparison compares two frozen plan-like views. It is intentionally
// smaller than Result: there is no event attribution when both sides are
// plans, but every contract-level amount still remains drillable.
type VersionComparison struct {
	Period     string             `json:"period"`
	Left       string             `json:"left"`
	Right      string             `json:"right"`
	LeftTotal  float64            `json:"left_total"`
	RightTotal float64            `json:"right_total"`
	Variance   float64            `json:"variance"`
	ByContract []ContractVariance `json:"by_contract"`
	TiesOut    bool               `json:"ties_out"`
}

// CompareVersions compares two plan-like snapshots without inventing a
// business cause. The caller supplies the labels because the right side may
// be the read-only Actual fact layer, together with the approved tie-out
// tolerance.
func CompareVersions(period, leftLabel, rightLabel string, left, right []ContractPeriod, tieOutTolerance float64) VersionComparison {
	leftByContract := indexByContract(left)
	rightByContract := indexByContract(right)
	result := VersionComparison{Period: period, Left: leftLabel, Right: rightLabel}
	for _, contractID := range unionOfKeys(leftByContract, rightByContract) {
		leftRow := leftByContract[contractID]
		rightRow := rightByContract[contractID]
		identity := leftRow
		if identity.ContractID == "" {
			identity = rightRow
		}
		result.LeftTotal += leftRow.LeaseCost
		result.RightTotal += rightRow.LeaseCost
		result.ByContract = append(result.ByContract, ContractVariance{
			ContractID: contractID, ContractNumber: identity.ContractNumber,
			ContractName: identity.ContractName, Currency: identity.Currency,
			Budget: round2(leftRow.LeaseCost), Actual: round2(rightRow.LeaseCost),
			Variance: round2(rightRow.LeaseCost - leftRow.LeaseCost), Cause: "comparison",
		})
	}
	result.LeftTotal = round2(result.LeftTotal)
	result.RightTotal = round2(result.RightTotal)
	result.Variance = round2(result.RightTotal - result.LeftTotal)
	var detailTotal float64
	for i := range result.ByContract {
		detailTotal += result.ByContract[i].Variance
	}
	result.TiesOut = math.Abs(round2(detailTotal)-result.Variance) <= tieOutTolerance
	sort.Slice(result.ByContract, func(i, j int) bool {
		return math.Abs(result.ByContract[i].Variance) > math.Abs(result.ByContract[j].Variance)
	})
	return result
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

	for _, cause := range []string{
		CauseNewLease, CauseEnded, CauseRenewal, CauseRentChange, CauseIndexChange,
		CauseDiscountRate, CausePaymentTiming, CauseDataCorrection, CauseExchangeRate, CauseOther,
	} {
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
	result.BridgeTiesOut = math.Abs(round2(bridgeSum)-result.Variance) <= input.TieOutTolerance
	result.VarianceCount = len(result.ByContract)

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
	if math.Abs(input.FXByContract[contractID]) > input.MaterialityThreshold {
		causes[CauseExchangeRate] = true
	}
	if math.Abs(actualPayment(input, contractID)-budgetPayment(input, contractID)) > input.MaterialityThreshold {
		causes[CausePaymentTiming] = true
	}

	for _, cause := range causeOrder {
		if causes[cause] {
			return cause
		}
	}
	return CauseOther
}

func budgetPayment(input Input, contractID string) float64 {
	for _, row := range input.Budget {
		if row.ContractID == contractID {
			return row.TotalPayment
		}
	}
	return 0
}

func actualPayment(input Input, contractID string) float64 {
	for _, row := range input.Actual {
		if row.ContractID == contractID {
			return row.TotalPayment
		}
	}
	return 0
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
		existing.TotalPayment += row.TotalPayment
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
