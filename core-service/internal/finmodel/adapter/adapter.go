// Package adapter is the production wiring for the S2-3/S4 seams: the fact
// reader that aggregates store-day facts into the entity-month values the
// engine's FactReader port expects, the statement-model readers behind the
// S4-1 read/evaluate tools, and the draft writer behind the S4-2/S4-3
// suggestion tools. Every read stays scoped by the caller's principal; the
// writers keep the draft-only contract (status=draft, source=ai_suggestion).
package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lease-management-system/core-service/internal/agenttools"
	"github.com/lease-management-system/core-service/internal/finmodel"
	"github.com/lease-management-system/core-service/internal/finmodel/suggestion"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

// FactsSource is the seam the fact adapter reads store-day facts from; the
// RetailKPIRepository satisfies it directly.
type FactsSource interface {
	QueryFacts(ctx context.Context, legalEntityID, dateFrom, dateTo, classification, datasetVersion, sourceSystem string, storeIDs []string) (*repository.RetailKPIFactSet, error)
}

// FactReader is the S2-3 production fact port: per entity-month operating
// aggregates summed from store-day facts. Coverage rule: every store in the
// population must have contributed at least one fact row in the month, and
// every fact's currency must agree — anything less degrades DecisionReady
// with the reason, never a fabricated number.
type FactReader struct {
	facts FactsSource
}

// NewFactReader builds the production fact port.
func NewFactReader(facts FactsSource) *FactReader {
	return &FactReader{facts: facts}
}

// Operating implements finmodel.FactReader.
func (r *FactReader) Operating(ctx context.Context, legalEntityID, period string) (finmodel.OperatingFacts, error) {
	if r.facts == nil {
		return finmodel.OperatingFacts{}, errors.New("fact reader unavailable")
	}
	if len(period) < 7 {
		return finmodel.OperatingFacts{}, fmt.Errorf("operating facts need a YYYY-MM period, got %q", period)
	}
	from := period + "-01"
	to := lastDayOf(period)
	set, err := r.facts.QueryFacts(ctx, legalEntityID, from, to, "production", "", "", nil)
	if err != nil {
		return finmodel.OperatingFacts{}, err
	}
	return AggregateMonthFacts(period, set), nil
}

func lastDayOf(period string) string {
	year, month := 0, 0
	for _, r := range period[:4] {
		year = year*10 + int(r-'0')
	}
	for _, r := range period[5:7] {
		month = month*10 + int(r-'0')
	}
	days := []int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}[month-1]
	if month == 2 {
		days = 28
		if year%4 == 0 && (year%100 != 0 || year%400 == 0) {
			days = 29
		}
	}
	return fmt.Sprintf("%s-%02d", period, days)
}

// AggregateMonthFacts is the pure month fold: flows sum per field, a field
// with no contributing rows stays nil, coverage and currency are decided
// on the raw population.
func AggregateMonthFacts(_ string, set *repository.RetailKPIFactSet) finmodel.OperatingFacts {
	out := finmodel.OperatingFacts{DataClassification: "production"}
	if set == nil {
		out.DecisionReady = false
		out.DecisionReadyReason = "事实集为空"
		return out
	}
	currency := ""
	contributingStores := map[string]bool{}
	sum := func(get func(*retailkpi.DailyFact) *float64) *float64 {
		var total float64
		found := false
		for i := range set.Facts {
			fact := &set.Facts[i]
			if value := get(fact); value != nil {
				total += *value
				found = true
			}
		}
		if !found {
			return nil
		}
		return &total
	}
	out.Revenue = sum(func(f *retailkpi.DailyFact) *float64 { return f.Revenue })
	out.GrossProfit = sum(func(f *retailkpi.DailyFact) *float64 { return f.GrossProfit })
	out.LaborCost = sum(func(f *retailkpi.DailyFact) *float64 { return f.LaborCost })
	out.FixedRent = sum(func(f *retailkpi.DailyFact) *float64 { return f.FixedRent })
	out.VariableRent = sum(func(f *retailkpi.DailyFact) *float64 { return f.VariableRent })
	out.NonLeaseCost = sum(func(f *retailkpi.DailyFact) *float64 { return f.NonLeaseCost })
	out.OtherControllableCost = sum(func(f *retailkpi.DailyFact) *float64 { return f.OtherControllableCost })

	for i := range set.Facts {
		fact := &set.Facts[i]
		contributingStores[fact.StoreID] = true
		if currency == "" {
			currency = fact.Currency
			continue
		}
		if currency != fact.Currency {
			currency = ""
		}
	}
	switch {
	case set.ExpectedStoreCount > len(contributingStores):
		out.DecisionReady = false
		out.DecisionReadyReason = fmt.Sprintf("覆盖不足：%d/%d 家门店在本月有事实", len(contributingStores), set.ExpectedStoreCount)
	case currency == "":
		out.DecisionReady = false
		out.DecisionReadyReason = "混币种或币种缺失：不同意口径"
	default:
		out.DecisionReady = len(contributingStores) > 0
		if !out.DecisionReady {
			out.DecisionReadyReason = "当月无事实行"
		}
	}
	return out
}

// StatementReader serves fpna.statement_model.read over the repository.
// Reads are scoped by the tool's middleware chain (TenantScope); this
// adapter reports what the repository holds.
type StatementReader struct {
	repo *repository.FinModelRepository
}

// NewStatementReader builds the reader.
func NewStatementReader(repo *repository.FinModelRepository) *StatementReader {
	return &StatementReader{repo: repo}
}

// ReadRun marshals one run with its lines and tie-outs.
func (r *StatementReader) ReadRun(ctx context.Context, runID string) (json.RawMessage, error) {
	if r.repo == nil {
		return nil, errors.New("statement model repository unavailable")
	}
	run, err := r.repo.GetModelRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	lines, err := r.repo.ListRunLines(ctx, runID)
	if err != nil {
		return nil, err
	}
	outs, err := r.repo.ListTieOuts(ctx, runID)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"run": run, "lines": lines, "tie_outs": outs})
}

// ReadDefinition marshals one definition with its bound template.
func (r *StatementReader) ReadDefinition(ctx context.Context, definitionID string) (json.RawMessage, error) {
	if r.repo == nil {
		return nil, errors.New("statement model repository unavailable")
	}
	def, err := r.repo.GetModelDefinition(ctx, definitionID)
	if err != nil {
		return nil, err
	}
	tmpl, err := r.repo.LoadStatementTemplate(ctx, def.TemplateID)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"definition": def, "template_name": tmpl.Name, "template_version": tmpl.Major, "rows": tmpl.Rows})
}

// DraftWriter adapts the repository's draft-only assumption write path to
// the tools' AssumptionDraftWriter seam. Owner flows from the assumption
// row's eventual human confirmation — drafts carry evidence + confidence
// and no approved route exists anywhere in this adapter.
type DraftWriter struct {
	repo *repository.FinModelRepository
}

// NewDraftWriter builds the suggestion writer.
func NewDraftWriter(repo *repository.FinModelRepository) *DraftWriter {
	return &DraftWriter{repo: repo}
}

// SaveDrafts implements tools.AssumptionDraftWriter / suggestion.Store.
func (w *DraftWriter) SaveDrafts(ctx context.Context, legalEntityID string, drafts []suggestion.SuggestionDraft, idempotencyKey string) ([]string, error) {
	if w.repo == nil {
		return nil, errors.New("assumption draft repository unavailable")
	}
	rows := make([]repository.AssumptionDraftRow, 0, len(drafts))
	for _, draft := range drafts {
		if err := draft.Validate(); err != nil {
			return nil, err
		}
		evidence, err := json.Marshal(draft.Basis)
		if err != nil {
			return nil, err
		}
		confidence := draft.Confidence
		rows = append(rows, repository.AssumptionDraftRow{
			Key: draft.AssumptionKey, Category: draft.Category, Value: draft.Value,
			Unit: draft.Unit, Source: "ai_suggestion", Owner: "",
			EffectiveFrom: time.Now(), Evidence: evidence, Confidence: &confidence,
		})
	}
	return w.repo.SaveAssumptionDrafts(ctx, legalEntityID, rows, idempotencyKey)
}

// NewSuggestionStore exposes the same writer through the suggestion.Store
// seam (used by the batch engine's SaveBatch).
func (w *DraftWriter) AsSuggestionStore() suggestion.Store { return w }

// Compile-time seams.
var (
	_ finmodel.FactReader = (*FactReader)(nil)
	_ suggestion.Store    = (*DraftWriter)(nil)
)

// ApprovedAssumptions reads the approved assumption rows only — the same
// path the formal run uses, so the evaluate tool's dry-run numbers cannot
// differ from a real run given the same overrides.
type ApprovedAssumptions struct {
	repo *repository.FinModelRepository
}

// NewApprovedAssumptions builds the reader.
func NewApprovedAssumptions(repo *repository.FinModelRepository) *ApprovedAssumptions {
	return &ApprovedAssumptions{repo: repo}
}

// Value implements finmodel.AssumptionReader.
func (a *ApprovedAssumptions) Value(ctx context.Context, legalEntityID, key, period string) (json.RawMessage, error) {
	if a.repo == nil {
		return nil, nil
	}
	values, err := a.repo.LatestApprovedAssumptions(ctx, legalEntityID, []string{key}, period)
	if err != nil {
		return nil, err
	}
	return values[key], nil
}

// overlay prefers the request's explicit values and falls back to the
// approved rows — drafts never appear because the reader queries only
// status='approved'.
type assumptionOverlay struct {
	base     map[string]json.RawMessage
	approved *ApprovedAssumptions
}

func (o assumptionOverlay) Value(ctx context.Context, entity, key, period string) (json.RawMessage, error) {
	if raw, ok := o.base[key]; ok {
		return raw, nil
	}
	return o.approved.Value(ctx, entity, key, period)
}

// PortsBuilder is the production FinModelPorts behind the S4-1 evaluate
// tool and the S4-5 paper tool: it loads the definition and template,
// binds the fact reader and the approved-assumption reader, and leaves
// the lease/schedule/opening ports to degrade honestly until their
// production adapters land (D-S2 discipline: the engine refuses nothing,
// it reports gaps).
type PortsBuilder struct {
	repo  *repository.FinModelRepository
	facts FactsSource
}

// NewPortsBuilder builds the port factory.
func NewPortsBuilder(repo *repository.FinModelRepository, facts FactsSource) *PortsBuilder {
	return &PortsBuilder{repo: repo, facts: facts}
}

type portsRequest struct {
	DefinitionID       string                     `json:"definition_id"`
	Assumptions        map[string]json.RawMessage `json:"assumptions"`
	DataClassification string                     `json:"data_classification"`
	Versions           finmodel.VersionSet        `json:"versions"`
}

// Build implements the FinModelPorts seam (agenttools). Entity/isolation
// filtering rides the tool middleware chain; here the definition's own
// legal entity is authoritative, matching the formal run path.
func (b *PortsBuilder) Build(ctx context.Context, principal agenttools.Principal, request json.RawMessage) (finmodel.ModelDef, finmodel.ModelInputs, error) {
	_ = principal
	if b.repo == nil {
		return finmodel.ModelDef{}, finmodel.ModelInputs{}, errors.New("statement model repository unavailable")
	}
	var req portsRequest
	if err := json.Unmarshal(request, &req); err != nil {
		return finmodel.ModelDef{}, finmodel.ModelInputs{}, fmt.Errorf("ports: %w", err)
	}
	defRow, err := b.repo.GetModelDefinition(ctx, req.DefinitionID)
	if err != nil {
		return finmodel.ModelDef{}, finmodel.ModelInputs{}, err
	}
	tmpl, err := b.repo.LoadStatementTemplate(ctx, defRow.TemplateID)
	if err != nil {
		return finmodel.ModelDef{}, finmodel.ModelInputs{}, err
	}
	var policy finmodel.ModelPolicy
	if len(defRow.Policy) > 0 {
		_ = json.Unmarshal(defRow.Policy, &policy)
	}
	if policy.Version == "" {
		policy = finmodel.ModelPolicy{Version: "v1", InterestCashFlowPresentation: "financing"}
	}
	periodStart := nowMinusMonths(11)
	if defRow.ActualCutoffPeriod != nil && *defRow.ActualCutoffPeriod != "" {
		if t, err := time.Parse("2006-01", *defRow.ActualCutoffPeriod); err == nil {
			periodStart = t.AddDate(0, -11, 0).Format("2006-01")
		}
	}
	classification := req.DataClassification
	if classification == "" {
		classification = "production"
	}
	def := finmodel.ModelDef{
		Name: defRow.Name, LegalEntityID: defRow.LegalEntityID, Currency: req.Versions.Data,
		Template: tmpl, PeriodStart: periodStart,
		HistoricalMonths: 12, ForecastMonths: 24,
		ActualCutoffPeriod: derefString(defRow.ActualCutoffPeriod),
		Policy:             policy,
	}
	if def.Currency == "" {
		def.Currency = "CNY"
	}
	var facts finmodel.FactReader
	if b.facts != nil {
		facts = NewFactReader(b.facts)
	}
	inputs := finmodel.ModelInputs{
		Assumptions:        assumptionOverlay{base: req.Assumptions, approved: NewApprovedAssumptions(b.repo)},
		Versions:           req.Versions,
		DataClassification: classification,
		Facts:              facts,
		// 租赁/付款计划/期初三个生产适配器接线前：诚实降级为缺口。
		Lease: nil, Schedules: nil, Opening: nil,
	}
	return def, inputs, nil
}

func nowMinusMonths(months int) string {
	return time.Now().AddDate(0, -months, 0).Format("2006-01")
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

var _ = nowMinusMonths
