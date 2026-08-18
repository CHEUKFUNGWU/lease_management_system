package cashplan

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

const (
	Version = "cash-plan-v1"
)

// OperatingFact represents monthly operating cash metrics for a store.
type OperatingFact struct {
	StoreID       string             `json:"store_id"`
	StoreCode     string             `json:"store_code"`
	StoreName     string             `json:"store_name"`
	Period        string             `json:"period"` // YYYY-MM
	Currency      string             `json:"currency"`
	Revenue       float64            `json:"revenue"`
	GrossProfit   float64            `json:"gross_profit"`
	LaborCost     float64            `json:"labor_cost"`
	FixedRent     float64            `json:"fixed_rent"`
	VariableRent  float64            `json:"variable_rent"`
	NonLeaseCost  float64            `json:"non_lease_cost"`
	OtherCost     float64            `json:"other_cost"`
	OperatingCash float64            `json:"operating_cash"` // revenue - labor - nonlease - other - fixedrent - variablerent
	Coverage      retailkpi.Coverage `json:"coverage"`
}

// LeasePaymentFact represents monthly lease payment schedule commitments.
type LeasePaymentFact struct {
	ContractID   string  `json:"contract_id"`
	StoreID      string  `json:"store_id"`
	Period       string  `json:"period"` // YYYY-MM
	Currency     string  `json:"currency"`
	FixedRent    float64 `json:"fixed_rent"`
	VariableRent float64 `json:"variable_rent"`
	NonLease     float64 `json:"non_lease"`
	Tax          float64 `json:"tax"`
	TotalOutflow float64 `json:"total_outflow"`
}

// CapexFact represents planned capital expenditure.
type CapexFact struct {
	StoreID     string  `json:"store_id"`
	Period      string  `json:"period"` // YYYY-MM
	Currency    string  `json:"currency"`
	Category    string  `json:"category"` // new_store, renovation, equipment, it
	Amount      float64 `json:"amount"`
	Description string  `json:"description,omitempty"`
}

// Sources provides readers for the three cash flow dimensions.
type Sources struct {
	Operating OperatingReader
	Lease     LeaseReader
	Capex     CapexReader
}

type OperatingReader interface {
	ReadOperating(ctx context.Context, legalEntityID, fromPeriod, toPeriod, classification, datasetVersion string, storeIDs []string) ([]OperatingFact, error)
}

type LeaseReader interface {
	ReadLeasePayments(ctx context.Context, legalEntityID, fromPeriod, toPeriod string, storeIDs []string) ([]LeasePaymentFact, error)
}

type CapexReader interface {
	ReadCapex(ctx context.Context, legalEntityID, fromPeriod, toPeriod string, storeIDs []string) ([]CapexFact, error)
}

type Request struct {
	LegalEntityID      string   `json:"legal_entity_id"`
	FromPeriod         string   `json:"from_period"` // YYYY-MM
	ToPeriod           string   `json:"to_period"`   // YYYY-MM
	DataClassification string   `json:"data_classification"`
	DatasetVersion     string   `json:"dataset_version,omitempty"`
	StoreIDs           []string `json:"store_ids,omitempty"`
}

// BridgeStep is one step in the cash flow reconciliation waterfall.
type BridgeStep struct {
	Label        string  `json:"label"`
	Code         string  `json:"code"`
	Amount       float64 `json:"amount"`
	IsDeduction  bool    `json:"is_deduction"`
	RunningTotal float64 `json:"running_total"`
}

// ConservationBridge holds the waterfall from operating cash to net cash plan.
type ConservationBridge struct {
	Steps            []BridgeStep `json:"steps"`
	OperatingCash    float64      `json:"operating_cash"`
	RentOffset       float64      `json:"rent_offset"`
	LeaseOutflow     float64      `json:"lease_outflow"`
	CapexOutflow     float64      `json:"capex_outflow"`
	NetCashPlan      float64      `json:"net_cash_plan"`
	RoundingResidual float64      `json:"rounding_residual"`
	IsConserved      bool         `json:"is_conserved"`
}

// MonthlyPlan is the monthly breakdown of the cash plan.
type MonthlyPlan struct {
	Period               string             `json:"period"`
	Currency             string             `json:"currency"`
	Revenue              float64            `json:"revenue"`
	OperatingCash        float64            `json:"operating_cash"`
	OperatingRentExpense float64            `json:"operating_rent_expense"`
	RentOffset           float64            `json:"rent_offset"`
	LeaseOutflow         float64            `json:"lease_outflow"`
	CapexOutflow         float64            `json:"capex_outflow"`
	NetCashPlan          float64            `json:"net_cash_plan"`
	Bridge               ConservationBridge `json:"bridge"`
}

// Partition is the cash plan grouped by currency.
type Partition struct {
	Currency             string             `json:"currency"`
	FromPeriod           string             `json:"from_period"`
	ToPeriod             string             `json:"to_period"`
	DecisionReady        bool               `json:"decision_ready"`
	TotalRevenue         float64            `json:"total_revenue"`
	TotalOperatingCash   float64            `json:"total_operating_cash"`
	TotalRentOffset      float64            `json:"total_rent_offset"`
	TotalLeaseOutflow    float64            `json:"total_lease_outflow"`
	TotalCapexOutflow    float64            `json:"total_capex_outflow"`
	TotalNetCashPlan     float64            `json:"total_net_cash_plan"`
	Monthly              []MonthlyPlan      `json:"monthly"`
	Bridge               ConservationBridge `json:"bridge"`
	WeakestCoverageRatio *float64           `json:"weakest_coverage_ratio,omitempty"`
}

// Plan is the overall synthesized cash plan response.
type Plan struct {
	Version            string      `json:"version"`
	LegalEntityID      string      `json:"legal_entity_id"`
	FromPeriod         string      `json:"from_period"`
	ToPeriod           string      `json:"to_period"`
	DataClassification string      `json:"data_classification"`
	DatasetVersion     string      `json:"dataset_version,omitempty"`
	MultiCurrency      bool        `json:"multi_currency"`
	Partitions         []Partition `json:"partitions"`
	GeneratedAt        time.Time   `json:"generated_at"`
}

// Compose executes the pure composition and conservation bridge calculation.
func Compose(ctx context.Context, req Request, sources Sources) (*Plan, error) {
	if req.LegalEntityID == "" {
		return nil, fmt.Errorf("legal_entity_id is required")
	}
	if req.FromPeriod == "" || req.ToPeriod == "" {
		return nil, fmt.Errorf("from_period and to_period are required")
	}
	if req.FromPeriod > req.ToPeriod {
		return nil, fmt.Errorf("from_period %q is after to_period %q", req.FromPeriod, req.ToPeriod)
	}

	var opFacts []OperatingFact
	var leaseFacts []LeasePaymentFact
	var capexFacts []CapexFact
	var err error

	if sources.Operating != nil {
		opFacts, err = sources.Operating.ReadOperating(ctx, req.LegalEntityID, req.FromPeriod, req.ToPeriod, req.DataClassification, req.DatasetVersion, req.StoreIDs)
		if err != nil {
			return nil, fmt.Errorf("read operating: %w", err)
		}
	}
	if sources.Lease != nil {
		leaseFacts, err = sources.Lease.ReadLeasePayments(ctx, req.LegalEntityID, req.FromPeriod, req.ToPeriod, req.StoreIDs)
		if err != nil {
			return nil, fmt.Errorf("read lease payments: %w", err)
		}
	}
	if sources.Capex != nil {
		capexFacts, err = sources.Capex.ReadCapex(ctx, req.LegalEntityID, req.FromPeriod, req.ToPeriod, req.StoreIDs)
		if err != nil {
			return nil, fmt.Errorf("read capex: %w", err)
		}
	}

	// Group facts by Currency
	currenciesMap := make(map[string]bool)
	for _, f := range opFacts {
		if f.Currency != "" {
			currenciesMap[f.Currency] = true
		}
	}
	for _, f := range leaseFacts {
		if f.Currency != "" {
			currenciesMap[f.Currency] = true
		}
	}
	for _, f := range capexFacts {
		if f.Currency != "" {
			currenciesMap[f.Currency] = true
		}
	}

	currencies := make([]string, 0, len(currenciesMap))
	for c := range currenciesMap {
		currencies = append(currencies, c)
	}
	if len(currencies) == 0 {
		currencies = append(currencies, "CNY")
	}
	sort.Strings(currencies)

	periods := generatePeriods(req.FromPeriod, req.ToPeriod)

	partitions := make([]Partition, 0, len(currencies))
	for _, curr := range currencies {
		partition := buildCurrencyPartition(curr, req.FromPeriod, req.ToPeriod, periods, opFacts, leaseFacts, capexFacts)
		partitions = append(partitions, partition)
	}

	return &Plan{
		Version:            Version,
		LegalEntityID:      req.LegalEntityID,
		FromPeriod:         req.FromPeriod,
		ToPeriod:           req.ToPeriod,
		DataClassification: req.DataClassification,
		DatasetVersion:     req.DatasetVersion,
		MultiCurrency:      len(currencies) > 1,
		Partitions:         partitions,
		GeneratedAt:        time.Now().UTC(),
	}, nil
}

func buildCurrencyPartition(curr, fromPeriod, toPeriod string, periods []string, opFacts []OperatingFact, leaseFacts []LeasePaymentFact, capexFacts []CapexFact) Partition {
	monthlyPlans := make([]MonthlyPlan, 0, len(periods))

	var totalRev, totalOpCash, totalRentOffset, totalLeaseOutflow, totalCapexOutflow, totalNetCash float64
	var minCoverageRate *float64
	decisionReady := true

	for _, p := range periods {
		var rev, opCash, opRent, leaseOut, capexOut float64

		for _, f := range opFacts {
			if f.Currency == curr && f.Period == p {
				rev += f.Revenue
				opCash += f.OperatingCash
				opRent += (f.FixedRent + f.VariableRent)
				if f.Coverage.CoverageRate != nil {
					if minCoverageRate == nil || *f.Coverage.CoverageRate < *minCoverageRate {
						r := *f.Coverage.CoverageRate
						minCoverageRate = &r
					}
					if *f.Coverage.CoverageRate < 100.0 {
						decisionReady = false
					}
				}
			}
		}

		for _, f := range leaseFacts {
			if f.Currency == curr && f.Period == p {
				leaseOut += f.TotalOutflow
			}
		}

		for _, f := range capexFacts {
			if f.Currency == curr && f.Period == p {
				capexOut += f.Amount
			}
		}

		// RENT DE-DUPLICATION RULE (PRD F3 / CodebaseDesign §7):
		// OperatingCash already deducted FixedRent + VariableRent.
		// Rent offset cancels the duplicate rent from operating cash flow.
		// Net Cash Plan = OperatingCash + RentOffset - LeaseOutflow - CapexOutflow
		rentOffset := opRent
		netCash := opCash + rentOffset - leaseOut - capexOut

		bridge := buildBridge(opCash, rentOffset, leaseOut, capexOut, netCash)

		monthlyPlans = append(monthlyPlans, MonthlyPlan{
			Period:               p,
			Currency:             curr,
			Revenue:              round(rev),
			OperatingCash:        round(opCash),
			OperatingRentExpense: round(opRent),
			RentOffset:           round(rentOffset),
			LeaseOutflow:         round(leaseOut),
			CapexOutflow:         round(capexOut),
			NetCashPlan:          round(netCash),
			Bridge:               bridge,
		})

		totalRev += rev
		totalOpCash += opCash
		totalRentOffset += rentOffset
		totalLeaseOutflow += leaseOut
		totalCapexOutflow += capexOut
		totalNetCash += netCash
	}

	totalBridge := buildBridge(totalOpCash, totalRentOffset, totalLeaseOutflow, totalCapexOutflow, totalNetCash)

	return Partition{
		Currency:             curr,
		FromPeriod:           fromPeriod,
		ToPeriod:             toPeriod,
		DecisionReady:        decisionReady,
		TotalRevenue:         round(totalRev),
		TotalOperatingCash:   round(totalOpCash),
		TotalRentOffset:      round(totalRentOffset),
		TotalLeaseOutflow:    round(totalLeaseOutflow),
		TotalCapexOutflow:    round(totalCapexOutflow),
		TotalNetCashPlan:     round(totalNetCash),
		Monthly:              monthlyPlans,
		Bridge:               totalBridge,
		WeakestCoverageRatio: minCoverageRate,
	}
}

func buildBridge(opCash, rentOffset, leaseOut, capexOut, netCash float64) ConservationBridge {
	// Reconciled math: opCash + rentOffset - leaseOut - capexOut = calculatedNet
	calculatedNet := opCash + rentOffset - leaseOut - capexOut
	residual := round(netCash - calculatedNet)

	running := opCash
	steps := []BridgeStep{
		{Label: "Operating Cash Flow", Code: "operating_cash", Amount: round(opCash), IsDeduction: false, RunningTotal: round(running)},
	}
	if rentOffset != 0 {
		running += rentOffset
		steps = append(steps, BridgeStep{Label: "Rent De-duplication Offset", Code: "rent_offset", Amount: round(rentOffset), IsDeduction: false, RunningTotal: round(running)})
	}
	if leaseOut != 0 {
		running -= leaseOut
		steps = append(steps, BridgeStep{Label: "Lease Outflows", Code: "lease_outflow", Amount: round(leaseOut), IsDeduction: true, RunningTotal: round(running)})
	}
	if capexOut != 0 {
		running -= capexOut
		steps = append(steps, BridgeStep{Label: "CAPEX Outflows", Code: "capex_outflow", Amount: round(capexOut), IsDeduction: true, RunningTotal: round(running)})
	}

	return ConservationBridge{
		Steps:            steps,
		OperatingCash:    round(opCash),
		RentOffset:       round(rentOffset),
		LeaseOutflow:     round(leaseOut),
		CapexOutflow:     round(capexOut),
		NetCashPlan:      round(netCash),
		RoundingResidual: residual,
		IsConserved:      math.Abs(residual) < 0.01,
	}
}

func generatePeriods(from, to string) []string {
	var periods []string
	var y1, m1, y2, m2 int
	fmt.Sscanf(from, "%d-%d", &y1, &m1)
	fmt.Sscanf(to, "%d-%d", &y2, &m2)

	currY, currM := y1, m1
	for {
		if currY > y2 || (currY == y2 && currM > m2) {
			break
		}
		periods = append(periods, fmt.Sprintf("%04d-%02d", currY, currM))
		currM++
		if currM > 12 {
			currY++
			currM = 1
		}
	}
	return periods
}

func round(v float64) float64 {
	return math.Round(v*100) / 100
}
