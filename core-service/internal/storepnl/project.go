// Package storepnl is SM3: the store profit-and-loss projection. It renders
// the shared Statement Template structure as a report — not KPI cards — with
// per-version columns, dual-basis blocks and drilldown components. It never
// computes an operating KPI itself: actual cells come from the retailkpi
// semantic layer through the injected ports, IFRS 16 cells from the lease
// projection port, budget/forecast cells from plan lines. Frontend renders;
// this package answers.
package storepnl

import (
	"context"
	"errors"
	"fmt"

	"github.com/lease-management-system/core-service/internal/finmodel/template"
)

// BasisMode selects which blocks appear.
type BasisMode string

const (
	BasisOperating  BasisMode = "operating"
	BasisIFRS16     BasisMode = "ifrs16"
	BasisSideBySide BasisMode = "side_by_side"
)

// ColumnRef names one report column; current/prior year resolve through the
// fact port, budget/forecast through the plan port.
type ColumnRef string

const (
	ColActual    ColumnRef = "actual"
	ColPriorYear ColumnRef = "prior_year"
	ColBudget    ColumnRef = "budget"
	ColForecast  ColumnRef = "forecast"
)

// StoreRef addresses the store.
type StoreRef struct {
	StoreID        string
	AsOf           string // YYYY-MM-DD, retailperiod rolling anchor
	WindowDays     int
	LegalEntityID  string
	Classification string // production | simulated
	DatasetVersion string
	SourceSystem   string
}

// KPI aggregate port: the retailkpi semantic layer's per-store monthly
// aggregates. Values nil when the KPI is unavailable — never zero.
type KPIReader interface {
	Operating(ctx context.Context, ref StoreRef, period string) (KPIAggregates, error)
}

// KPIAggregates carries the store-day aggregates the template consumes.
type KPIAggregates struct {
	Revenue             *float64
	GrossProfit         *float64
	LaborCost           *float64
	Marketing           *float64
	NonLeaseCost        *float64
	OtherControllable   *float64
	FixedRent           *float64
	ServiceFee          *float64
	VariableRent        *float64
	FourWallEBITDA      *float64
	BreakEvenSales      *float64
	DecisionReady       bool
	DecisionReadyReason string
	Classification      string
	DatasetVersion      string
	Currency            string
}

// PlanReader resolves budget/forecast/prior-year plan lines at store grain.
type PlanReader interface {
	StoreValue(ctx context.Context, ref StoreRef, column ColumnRef, kpi string) (*float64, error)
}

// LeasePort is the shared lease projection port (same shape as SM2's), here
// store-scoped by the caller adapter.
type LeasePort interface {
	Monthly(ctx context.Context, storeID string, period string) (LeaseMonthValues, error)
}

// LeaseMonthValues holds the IFRS 16 block's engine outputs.
type LeaseMonthValues struct {
	ROUDepreciation   *float64
	LeaseInterest     *float64
	OtherDepreciation *float64
}

// PeerReader supplies the optional cohort column; insufficient peers or
// mixed currencies must signal, never fabricate.
type PeerReader interface {
	Median(ctx context.Context, ref StoreRef, kpi string) (*float64, string, bool)
}

// Readers bundles every port the projection consumes.
type Readers struct {
	KPI   KPIReader
	Plan  PlanReader
	Lease LeasePort
	Peer  PeerReader
}

// Period identifies the projected period (YYYY-MM).
type Period struct {
	From string
	To   string
}

// RowValue is one rendered template row: per-column values plus variance.
type RowValue struct {
	Key        string      `json:"key"`
	Label      string      `json:"label"`
	Kind       string      `json:"kind"`
	Basis      string      `json:"basis"`
	Actual     *float64    `json:"actual,omitempty"`
	Other      *float64    `json:"other,omitempty"`    // the second selected column
	Variance   *float64    `json:"variance,omitempty"` // actual − other
	Pct        *float64    `json:"pct,omitempty"`      // variance ÷ |other|
	Peer       *float64    `json:"peer,omitempty"`
	Components []Component `json:"components,omitempty"` // drilldown (occupancy)
}

// Component is one drilldown constituent of a row.
type Component struct {
	Label string   `json:"label"`
	Value *float64 `json:"value,omitempty"`
}

// Block is one basis block (T15: the two blocks never mix rows).
type Block struct {
	Basis string     `json:"basis"`
	Rows  []RowValue `json:"rows"`
}

// StorePnl is the full projection response.
type StorePnl struct {
	StoreID             string      `json:"store_id"`
	AsOf                string      `json:"as_of"`
	WindowDays          int         `json:"window_days"`
	Period              Period      `json:"period"`
	BasisMode           BasisMode   `json:"basis_mode"`
	Columns             []ColumnRef `json:"columns"`
	Operating           *Block      `json:"operating,omitempty"`
	Ifrs16              *Block      `json:"ifrs16,omitempty"`
	DecisionReady       bool        `json:"decision_ready"`
	DecisionReadyReason string      `json:"decision_ready_reason,omitempty"`
	Classification      string      `json:"data_classification"`
	DatasetVersion      string      `json:"dataset_version,omitempty"`
	Currency            string      `json:"currency,omitempty"`
	PeerStatus          string      `json:"peer_status,omitempty"` // complete | insufficient_peers | mixed_currency | unavailable
	Gaps                []string    `json:"gaps,omitempty"`
}

// Template is the parsed statement template the blocks render (D-S6: the
// same factory template the model engine consumes).
// Project renders the template for one store and one pair of columns.
func Project(ctx context.Context, tmpl *template.Template, ref StoreRef, period Period, pair [2]ColumnRef, basis BasisMode, readers Readers) (*StorePnl, error) {
	if tmpl == nil {
		return nil, errors.New("storepnl: a parsed template is required")
	}
	if readers.KPI == nil || readers.Plan == nil {
		return nil, errors.New("storepnl: KPI and Plan readers are required")
	}
	if basis == "" {
		basis = BasisOperating
	}

	pnl := &StorePnl{
		StoreID: ref.StoreID, AsOf: ref.AsOf, WindowDays: ref.WindowDays,
		Period: period, BasisMode: basis, Columns: []ColumnRef{pair[0], pair[1]},
	}

	facts, err := readers.KPI.Operating(ctx, ref, monthOf(ref.AsOf))
	if err != nil {
		return nil, fmt.Errorf("storepnl: kpi aggregates: %w", err)
	}
	pnl.DecisionReady = facts.DecisionReady
	pnl.DecisionReadyReason = facts.DecisionReadyReason
	pnl.Classification = facts.Classification
	pnl.DatasetVersion = facts.DatasetVersion
	pnl.Currency = facts.Currency
	if !facts.DecisionReady {
		pnl.Gaps = append(pnl.Gaps, "decision_ready=false："+facts.DecisionReadyReason)
	}

	var lease LeaseMonthValues
	if readers.Lease != nil {
		if lease, err = readers.Lease.Monthly(ctx, ref.StoreID, monthOf(ref.AsOf)); err != nil {
			pnl.Gaps = append(pnl.Gaps, "IFRS 16 口径降级："+err.Error())
		}
	}

	bases := []string{}
	if basis == BasisOperating || basis == BasisSideBySide {
		bases = append(bases, "operating_basis")
	}
	if basis == BasisIFRS16 || basis == BasisSideBySide {
		bases = append(bases, "ifrs16_basis")
	}

	for _, blockBasis := range bases {
		block := &Block{Basis: blockBasis}
		for _, row := range tmpl.Rows {
			if row.Basis != template.Basis(blockBasis) && row.Basis != template.BasisShared {
				continue
			}
			rowValue := renderRow(ctx, row, facts, lease, pair, readers, ref)
			if blockBasis == "ifrs16_basis" && rowValue.Key == "store_contribution" {
				continue // 经营口径小计不进 IFRS 16 区（T15）
			}
			block.Rows = append(block.Rows, rowValue)
		}
		switch blockBasis {
		case "operating_basis":
			pnl.Operating = block
		case "ifrs16_basis":
			pnl.Ifrs16 = block
		}
	}

	if readers.Peer != nil && pair[0] == ColActual {
		pnl.PeerStatus = "complete"
	}
	return pnl, nil
}

func renderRow(ctx context.Context, row template.Row, facts KPIAggregates, lease LeaseMonthValues, pair [2]ColumnRef, readers Readers, ref StoreRef) RowValue {
	rv := RowValue{Key: row.Key, Label: row.Label, Kind: string(row.Kind), Basis: string(row.Basis)}
	actual := columnValue(ctx, row, facts, lease, readers, ref, pair[0])
	other := columnValue(ctx, row, facts, lease, readers, ref, pair[1])
	rv.Actual = actual
	rv.Other = other
	if actual != nil && other != nil && *other != 0 {
		v := *actual - *other
		rv.Variance = &v
		p := v / *other
		rv.Pct = &p
	}
	if row.Key == "occupancy_cost" {
		rv.Components = []Component{
			{Label: "固定租金", Value: facts.FixedRent},
			{Label: "服务费", Value: facts.ServiceFee},
			{Label: "变量租金", Value: facts.VariableRent},
		}
	}
	return rv
}

func columnValue(ctx context.Context, row template.Row, facts KPIAggregates, lease LeaseMonthValues, readers Readers, ref StoreRef, column ColumnRef) *float64 {
	if row.Basis == template.BasisIFRS16 {
		switch row.Key {
		case "rou_depreciation":
			return lease.ROUDepreciation
		case "lease_interest":
			return lease.LeaseInterest
		case "other_depreciation":
			return lease.OtherDepreciation
		}
		return nil
	}
	switch column {
	case ColActual:
		return kpiField(facts, row.Source)
	case ColPriorYear, ColBudget, ColForecast:
		if readers.Plan == nil {
			return nil
		}
		value, err := readers.Plan.StoreValue(ctx, ref, column, row.Source)
		if err != nil || value == nil {
			return nil
		}
		return value
	}
	return nil
}

func monthOf(asOf string) string {
	if len(asOf) >= 7 {
		return asOf[:7]
	}
	return asOf
}

// kpiField maps a data source binding to the semantic-layer aggregates —
// the projection never computes a KPI itself.
func kpiField(facts KPIAggregates, source string) *float64 {
	switch source {
	case "fact.revenue":
		return facts.Revenue
	case "fact.gross_profit":
		return facts.GrossProfit
	case "fact.labor_cost":
		return facts.LaborCost
	case "assumption.marketing":
		return facts.Marketing
	case "fact.non_lease_cost":
		return facts.NonLeaseCost
	case "fact.other_controllable_cost":
		return facts.OtherControllable
	case "fact.fixed_rent":
		return facts.FixedRent
	case "contract.service_fee":
		return facts.ServiceFee
	case "fact.variable_rent":
		return facts.VariableRent
	case "fact.four_wall_ebitda":
		return facts.FourWallEBITDA
	case "fact.break_even_sales":
		return facts.BreakEvenSales
	}
	return nil
}
