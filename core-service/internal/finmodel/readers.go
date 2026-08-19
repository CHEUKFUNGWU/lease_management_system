// Package finmodel is SM2/SM4's engine half: the pure three-statement model
// Run plus the opening-balance gate. The engine imports neither ifrs16 nor
// any repository — lease numbers enter through the LeaseRollforwardReader
// projection port (D-S3, locked by importguard_test.go), and persistence has
// exactly one entry point, RunWriter.Persist (D-S2).
package finmodel

import (
	"context"
	"encoding/json"

	"github.com/lease-management-system/core-service/internal/finmodel/opening"
	"github.com/lease-management-system/core-service/internal/finmodel/template"
)

// OperatingFacts is one month of actual operating data aggregated through
// retail-kpi-v1 semantics. Missing values stay nil — never zero (D-S4).
type OperatingFacts struct {
	Revenue               *float64
	GrossProfit           *float64
	LaborCost             *float64
	FixedRent             *float64
	VariableRent          *float64
	NonLeaseCost          *float64
	OtherControllableCost *float64
	FourWallEBITDA        *float64
	BreakEvenSales        *float64
	// Honesty envelope.
	DecisionReady       bool
	DecisionReadyReason string
	DataClassification  string
	DatasetVersion      string
}

// FactReader is the actual-operating port (production adapter = retailkpi
// thin binding; memory adapter for golden tests).
type FactReader interface {
	Operating(ctx context.Context, legalEntityID, period string) (OperatingFacts, error)
}

// LeaseMonth is one month of the IFRS 16 roll-forward projection. Every
// field comes verbatim from the engine projection; the model never computes
// lease numbers itself.
type LeaseMonth struct {
	ROUAsset       *float64
	LeaseLiability *float64
	Interest       *float64
	Depreciation   *float64
	Payments       *float64
	Principal      *float64
	Additions      *float64
	Remeasurements *float64
	Terminations   *float64
}

// LeaseRollforwardReader is the lease projection port (D-S3).
type LeaseRollforwardReader interface {
	Monthly(ctx context.Context, legalEntityID, period string) (LeaseMonth, error)
}

// ScheduleFanout returns one month's schedule values.
type ScheduleFanout struct {
	Capex             *float64
	OtherDepreciation *float64
	ShareCapital      *float64
	ServiceFee        *float64
	Borrowings        *float64
}

// ScheduleReader supplies the long-term-asset and contract-driven inputs.
type ScheduleReader interface {
	Monthly(ctx context.Context, legalEntityID, period string) (ScheduleFanout, error)
}

// AssumptionReader resolves approved assumptions by key and period. The
// engine reads ONLY this port for assumptions — drafts can never leak into a
// formal run (the S4-6 acceptance test rides on this contract).
type AssumptionReader interface {
	Value(ctx context.Context, legalEntityID, key, period string) (json.RawMessage, error)
}

// OpeningBalanceReader is the opening BS port; the engine re-runs the SM4
// gates before consuming it.
type OpeningBalanceReader interface {
	Get(ctx context.Context, legalEntityID string) (*opening.OpeningBalance, []opening.ContractBalance, []opening.ContractBalance, opening.MergePolicy, error)
}

// ModelPolicy is the versioned accounting policy set (PRD S2-3 Step 3).
// An empty InterestCashFlowPresentation rejects the run (T9: 政策开关缺失时
// run 拒绝启动).
type ModelPolicy struct {
	Version                      string `json:"version"`
	InterestCashFlowPresentation string `json:"interest_cash_flow_presentation"` // "operating" | "financing"
	TaxLossCarryforward          bool   `json:"tax_loss_carryforward"`
	InterestMethod               string `json:"interest_method"` // "opening_balance" (S2-8)
}

// ModelDef is the runnable model definition.
type ModelDef struct {
	Name               string
	LegalEntityID      string
	Currency           string
	Template           *template.Template
	PeriodStart        string // "YYYY-MM" of the first model period
	HistoricalMonths   int
	ForecastMonths     int
	ActualCutoffPeriod string // inclusive last actual month; empty = no actuals
	Policy             ModelPolicy
}
