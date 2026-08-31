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
	"strings"
	"time"

	"github.com/lease-management-system/core-service/internal/finmodel/template"
	"github.com/lease-management-system/core-service/internal/services/sourceenvelope"
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

// StoreRef addresses the store and one resolved period. DateFrom/DateTo
// carry the resolved retailperiod window (day/week/month/quarter/year);
// when empty the adapter falls back to the rolling AsOf+WindowDays anchor.
type StoreRef struct {
	StoreID        string
	AsOf           string // YYYY-MM-DD, retailperiod rolling anchor
	WindowDays     int
	DateFrom       string // YYYY-MM-DD, resolved by retailperiod (calendar + day grains)
	DateTo         string
	PeriodLabel    string // display label of the resolved window ("2026-Q3")
	PeriodKind     string // rolling | calendar
	LegalEntityID  string
	Classification string // production | simulated
	DatasetVersion string
	SourceSystem   string
	Currency       string // currency of the actual KPI facts for this projection
}

// KPI aggregate port: the retailkpi semantic layer's per-store aggregates
// over the resolved window in StoreRef. Values nil when the KPI is
// unavailable — never zero.
type KPIReader interface {
	Operating(ctx context.Context, ref StoreRef) (KPIAggregates, error)
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
	// Provenance maps KPI code → source envelope (S1-5 level 3): the
	// sources/batches/versions/as-of of the facts that produced the KPI.
	// Rows surface the envelope matching their source binding; derived or
	// assumption-bound rows have no envelope (nil).
	Provenance map[string]FactEnvelope
	// Envelope is the store-level semantic-layer envelope (formula/pulse
	// versions, coverage, decision status).
	Envelope *sourceenvelope.Envelope
}

// FactEnvelope is one KPI's source trace: source systems, import
// batches, fact-version range, highest as-of and the number of
// contributing store-days.
type FactEnvelope struct {
	SourceSystems      []string `json:"source_systems"`
	ImportBatchIDs     []string `json:"import_batch_ids,omitempty"`
	FactVersionMin     int      `json:"fact_version_min"`
	FactVersionMax     int      `json:"fact_version_max"`
	HighestAsOf        string   `json:"highest_as_of,omitempty"`
	DataClassification string   `json:"data_classification"`
	SourceDays         int      `json:"source_days"`
}

// PlanReader resolves budget/forecast/prior-year plan lines at store grain.
type PlanReader interface {
	StoreValue(ctx context.Context, ref StoreRef, column ColumnRef, kpi string) (*float64, error)
}

// PlanCurrencyReader is an optional production seam for checking that the
// selected plan uses the same currency as the actual facts. A mismatch must
// stay unavailable rather than rendering a number under the wrong currency.
type PlanCurrencyReader interface {
	PlanCurrency(ctx context.Context, ref StoreRef, column ColumnRef) (string, error)
}

// LeasePort is the shared lease projection port (same shape as SM2's), here
// store-scoped by the caller adapter. legalEntityID rides alongside the store
// id so the adapter can enforce the caller's entity on the engine rows it
// reads (bottom line 1: the port cannot be an entity-blind tunnel).
type LeasePort interface {
	Monthly(ctx context.Context, storeID, legalEntityID string, period string) (LeaseMonthValues, error)
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

// Readers bundles every port the projection consumes. Governed lists the
// row keys registered in fpna_metric_definitions (approved) — a
// formula/check row not on the list is a template-custom row and must be
// marked 未经指标治理 (S3-6); an empty map marks every formula row.
type Readers struct {
	KPI       KPIReader
	Plan      PlanReader
	Lease     LeasePort
	Peer      PeerReader
	Occupancy OccupancyReader
	Governed  map[string]bool
}

// Period identifies the projected period (YYYY-MM).
type Period struct {
	From string
	To   string
}

// RowValue is one rendered template row: per-column values plus variance.
type RowValue struct {
	Key        string          `json:"key"`
	Label      string          `json:"label"`
	Kind       string          `json:"kind"`
	Basis      string          `json:"basis"`
	Actual     *float64        `json:"actual,omitempty"`
	Other      *float64        `json:"other,omitempty"`    // the second selected column
	Variance   *float64        `json:"variance,omitempty"` // actual − other
	Pct        *float64        `json:"pct,omitempty"`      // variance ÷ |other|
	Peer       *float64        `json:"peer,omitempty"`
	PeerStatus string          `json:"peer_status,omitempty"` // empty = no peer column for this row
	Format     template.Format `json:"format"`                // S3-7 display contract from the template
	Ungoverned bool            `json:"ungoverned,omitempty"`  // S3-6: template-custom formula row
	// Provenance is the row's source envelope (S1-5 level 3); nil for
	// derived/assumption/lease rows.
	Provenance *FactEnvelope `json:"provenance,omitempty"` // S1-5 level 3 source trace
	// ContractSplit is the occupancy contract-level drill (S1-5 level 2): per
	// contract 基本租金/服务费/变量租金.
	ContractSplit []ContractSplit `json:"contract_split,omitempty"`
	// Source and FormulaText round-trip the template declaration (S1-9's
	// page editor rebuilds defs from the rendered rows).
	Source      string      `json:"source,omitempty"`
	FormulaText string      `json:"formula_text,omitempty"`
	Components  []Component `json:"components,omitempty"` // drilldown (occupancy)
	// Subtotal wiring for live-formula exports: children ± subtracted rows.
	Children   []string `json:"children,omitempty"`
	Subtracted []string `json:"subtracted,omitempty"`
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
	StoreID             string                   `json:"store_id"`
	AsOf                string                   `json:"as_of"`
	WindowDays          int                      `json:"window_days"`
	PeriodLabel         string                   `json:"period_label,omitempty"`
	PeriodKind          string                   `json:"period_kind,omitempty"` // rolling | calendar
	Period              Period                   `json:"period"`
	BasisMode           BasisMode                `json:"basis_mode"`
	Columns             []ColumnRef              `json:"columns"`
	Operating           *Block                   `json:"operating,omitempty"`
	Ifrs16              *Block                   `json:"ifrs16,omitempty"`
	DecisionReady       bool                     `json:"decision_ready"`
	DecisionReadyReason string                   `json:"decision_ready_reason,omitempty"`
	Classification      string                   `json:"data_classification"`
	DatasetVersion      string                   `json:"dataset_version,omitempty"`
	Currency            string                   `json:"currency,omitempty"`
	PeerStatus          string                   `json:"peer_status,omitempty"` // complete | insufficient_peers | mixed_currency | unavailable
	Envelope            *sourceenvelope.Envelope `json:"envelope,omitempty"`    // 门店级语义层信封
	Gaps                []string                 `json:"gaps,omitempty"`
}

// Template is the parsed statement template the blocks render (D-S6: the
// same factory template the model engine consumes).
// Project renders the template for one store and one pair of columns.
func Project(ctx context.Context, tmpl *template.Template, ref StoreRef, period Period, pair [2]ColumnRef, basis BasisMode, readers Readers) (*StorePnl, error) {
	if tmpl == nil {
		return nil, errors.New("storepnl: a parsed template is required")
	}
	if readers.KPI == nil {
		return nil, errors.New("storepnl: KPI reader is required")
	}
	if basis == "" {
		basis = BasisOperating
	}

	pnl := &StorePnl{
		StoreID: ref.StoreID, AsOf: ref.AsOf, WindowDays: ref.WindowDays,
		PeriodLabel: ref.PeriodLabel, PeriodKind: ref.PeriodKind,
		Period: period, BasisMode: basis, Columns: []ColumnRef{pair[0], pair[1]},
	}

	facts, err := readers.KPI.Operating(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("storepnl: kpi aggregates: %w", err)
	}
	pnl.DecisionReady = facts.DecisionReady
	pnl.DecisionReadyReason = facts.DecisionReadyReason
	pnl.Classification = facts.Classification
	pnl.DatasetVersion = facts.DatasetVersion
	pnl.Currency = facts.Currency
	pnl.Envelope = facts.Envelope
	ref.Currency = facts.Currency
	if !facts.DecisionReady {
		pnl.Gaps = append(pnl.Gaps, "decision_ready=false："+facts.DecisionReadyReason)
	}
	if readers.Plan != nil && (pair[1] == ColBudget || pair[1] == ColForecast) {
		if strings.TrimSpace(facts.Currency) == "" {
			pnl.DecisionReady = false
			pnl.Gaps = append(pnl.Gaps, "actual_currency_unavailable")
		}
		if currencyReader, ok := readers.Plan.(PlanCurrencyReader); ok {
			planCurrency, currencyErr := currencyReader.PlanCurrency(ctx, ref, pair[1])
			switch {
			case currencyErr != nil:
				pnl.DecisionReady = false
				pnl.Gaps = append(pnl.Gaps, "budget_currency_unavailable:"+currencyErr.Error())
			case planCurrency != "" && facts.Currency != "" && !strings.EqualFold(planCurrency, facts.Currency):
				pnl.DecisionReady = false
				pnl.Gaps = append(pnl.Gaps, fmt.Sprintf("budget_currency_mismatch:%s_vs_%s", planCurrency, facts.Currency))
			}
		}
	}

	var lease LeaseMonthValues
	if readers.Lease != nil {
		// 计量引擎按月出数：用窗口起始月（calendar 期间即其月份；滚窗跨月
		// 取起始月，与引擎粒度一致）。
		leasePeriod := period.From
		if len(leasePeriod) >= 7 {
			leasePeriod = leasePeriod[:7]
		}
		if leasePeriod == "" {
			leasePeriod = monthOf(ref.AsOf)
		}
		if lease, err = readers.Lease.Monthly(ctx, ref.StoreID, ref.LegalEntityID, leasePeriod); err != nil {
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

	// Peer column (S1-6): probe once per peerable row, ahead of rendering;
	// degraded rows stay empty and the status is aggregated to the header.
	peers := map[string]peerProbe{}
	if readers.Peer != nil && pair[0] == ColActual {
		for _, row := range tmpl.Rows {
			code := kpiCode(row.Source)
			if code == "" {
				continue
			}
			value, status, ok := readers.Peer.Median(ctx, ref, code)
			peers[row.Key] = peerProbe{value: value, status: status, ok: ok}
		}
	}

	for _, blockBasis := range bases {
		block := &Block{Basis: blockBasis}
		for _, row := range tmpl.Rows {
			if row.Basis != template.Basis(blockBasis) && row.Basis != template.BasisShared {
				continue
			}
			rowValue := renderRow(ctx, row, facts, lease, pair, readers, ref, peers[row.Key])
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

	pnl.PeerStatus = aggregatePeerStatus(peers)

	// 后端求值（PRD §3）：公式行与小计行用与引擎同一份 AST 在后端算出——
	// 前端严禁重算任何模型行。单窗口投影无跨期值，lag(x,n>0) 一律缺失。
	evaluateTemplateRows(tmpl, pnl, pair)
	return pnl, nil
}

// evaluateTemplateRows fills formula/subtotal row values per column and
// recomputes variance/pct uniformly. Evaluation is template-wide in
// dependency order (formula Deps + subtotal Children, exactly the engine's
// graph); missing children degrade a subtotal to nil like the engine's
// evalSubtotal. Both basis blocks share the same row copies, so each
// evaluated row is written back to every block it appears in.
func evaluateTemplateRows(tmpl *template.Template, pnl *StorePnl, pair [2]ColumnRef) {
	byKey := map[string][]*RowValue{}
	collect := func(block *Block) {
		if block == nil {
			return
		}
		for i := range block.Rows {
			byKey[block.Rows[i].Key] = append(byKey[block.Rows[i].Key], &block.Rows[i])
		}
	}
	collect(pnl.Operating)
	collect(pnl.Ifrs16)

	ordered := topoRows(tmpl.Rows)
	for _, column := range []ColumnRef{pair[0], pair[1]} {
		values := map[string]*float64{}
		for key, copies := range byKey {
			var value *float64
			for _, copy := range copies {
				if column == pair[0] && copy.Actual != nil {
					value = copy.Actual
					break
				}
				if column == pair[1] && copy.Other != nil {
					value = copy.Other
					break
				}
			}
			values[key] = value
		}
		for _, row := range ordered {
			copies := byKey[row.Key]
			if len(copies) == 0 {
				continue
			}
			var value *float64
			switch row.Kind {
			case template.RowFormula, template.RowCheck:
				if row.Formula == nil {
					break
				}
				value = row.Formula.Eval(
					func(ref string) *float64 { return values[ref] },
					func(ref string, n int) *float64 {
						if n == 0 {
							return values[ref]
						}
						return nil // 单窗口投影：无跨期值
					})
			case template.RowSubtotal:
				var sum float64
				complete := true
				for _, child := range row.Children {
					v := values[child]
					if v == nil {
						complete = false
						break
					}
					sum += *v * row.ChildSign(child)
				}
				if complete {
					value = &sum
				}
			default:
				value = values[row.Key] // link/input 行保留既有值
			}
			values[row.Key] = value
			if column == pair[0] {
				for _, copy := range copies {
					copy.Actual = value
				}
			} else {
				for _, copy := range copies {
					copy.Other = value
				}
			}
		}
	}
	// 统一重算差异额/差异率（含新求值的公式与小计行）。
	for _, copies := range byKey {
		for _, row := range copies {
			if row.Actual != nil && row.Other != nil && *row.Other != 0 {
				v := *row.Actual - *row.Other
				row.Variance = &v
				p := v / *row.Other
				row.Pct = &p
			} else {
				row.Variance, row.Pct = nil, nil
			}
		}
	}
}

// topoRows orders the template rows so every formula/subtotal row follows
// its dependencies (formula Deps + subtotal Children — the same graph the
// engine's topoOrder walks).
func topoRows(rows []template.Row) []template.Row {
	index := map[string]int{}
	for i, row := range rows {
		index[row.Key] = i
	}
	deps := map[string][]string{}
	for _, row := range rows {
		switch row.Kind {
		case template.RowFormula, template.RowCheck:
			deps[row.Key] = append(deps[row.Key], row.Formula.Deps()...)
		case template.RowSubtotal:
			deps[row.Key] = append(deps[row.Key], row.Children...)
		}
	}
	order := make([]template.Row, 0, len(rows))
	placed := map[string]bool{}
	visiting := map[string]bool{}
	var visit func(key string)
	visit = func(key string) {
		if placed[key] || visiting[key] {
			return
		}
		visiting[key] = true
		for _, dep := range deps[key] {
			if idx, ok := index[dep]; ok {
				_ = idx
				visit(dep)
			}
		}
		visiting[key] = false
		order = append(order, rows[index[key]])
		placed[key] = true
	}
	for _, row := range rows {
		visit(row.Key)
	}
	return order
}

// peerProbe is one resolved peer datum for a row.
type peerProbe struct {
	value  *float64
	status string
	ok     bool
}

// aggregatePeerStatus collapses the per-row probes into the header status:
// complete only when every probed row delivered a peer number; any
// degradation names itself (样本不足/混币种/不可用显式降级，不出数字).
func aggregatePeerStatus(peers map[string]peerProbe) string {
	if len(peers) == 0 {
		return ""
	}
	probed := false
	for _, probe := range peers {
		probed = true
		if !probe.ok || probe.value == nil || probe.status != "complete" {
			return probe.status
		}
	}
	if !probed {
		return ""
	}
	return "complete"
}

func renderRow(ctx context.Context, row template.Row, facts KPIAggregates, lease LeaseMonthValues, pair [2]ColumnRef, readers Readers, ref StoreRef, peer peerProbe) RowValue {
	rv := RowValue{Key: row.Key, Label: row.Label, Kind: string(row.Kind), Basis: string(row.Basis), Children: row.Children, Subtracted: row.Subtract, Format: row.Format, Source: row.Source, FormulaText: row.FormulaText}
	if (row.Kind == template.RowFormula || row.Kind == template.RowCheck) && !readers.Governed[row.Key] {
		// S3-6: 模板内自定义公式行，未经指标治理 —— fail-closed：登记集为空
		// 时所有公式行都带标识。
		rv.Ungoverned = true
	}
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
	if peer.ok && peer.value != nil {
		rv.Peer = peer.value
		rv.PeerStatus = "complete"
	} else if peer.status != "" {
		rv.PeerStatus = peer.status
	}
	if envelope, ok := facts.Provenance[kpiCode(row.Source)]; ok {
		value := envelope
		rv.Provenance = &value
	}
	if row.Key == "occupancy_cost" {
		rv.Components = []Component{
			{Label: "固定租金", Value: facts.FixedRent},
			{Label: "服务费", Value: facts.ServiceFee},
			{Label: "变量租金", Value: facts.VariableRent},
		}
		// S1-5 层级 2：合同级拆分。有拆分时，聚合构成改由拆分求和导出，
		// 两级永远一致；端口未接或无合同则保留事实层聚合。
		if readers.Occupancy != nil {
			from, to := windowSpan(ref)
			if splits, err := readers.Occupancy.Contracts(ctx, ref.StoreID, ref.LegalEntityID, from, to); err == nil && len(splits) > 0 {
				basic, service, variable := ComponentSum(splits)
				if basic != nil && service != nil && variable != nil {
					rv.Components = []Component{
						{Label: "基本租金", Value: basic},
						{Label: "服务费", Value: service},
						{Label: "变量租金", Value: variable},
					}
				}
				rv.ContractSplit = splits
			}
		}
	}
	return rv
}

// windowSpan resolves the window the store projection aggregates over: the
// explicit calendar/rolling resolution when present, otherwise the legacy
// AsOf+WindowDays anchor — same resolution the KPI adapter applies.
func windowSpan(ref StoreRef) (string, string) {
	if ref.DateFrom != "" && ref.DateTo != "" {
		return ref.DateFrom, ref.DateTo
	}
	asOf, err := time.Parse("2006-01-02", ref.AsOf)
	if err != nil {
		return ref.AsOf, ref.AsOf
	}
	days := ref.WindowDays
	if days < 1 {
		days = 1
	}
	return asOf.AddDate(0, 0, -(days - 1)).Format("2006-01-02"), asOf.Format("2006-01-02")
}

// kpiCode strips the source-binding prefix (fact. / contract. /
// assumption.) to the KPI code the peer benchmarks are keyed by.
func kpiCode(source string) string {
	dot := strings.Index(source, ".")
	if dot < 0 || dot == len(source)-1 {
		return ""
	}
	return source[dot+1:]
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
