package handlers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailstore360"
	"github.com/lease-management-system/core-service/internal/storepnl"
)

// StoreFactsSource is the narrow facts seam the KPI adapter consumes; the
// RetailKPIRepository satisfies it directly.
type StoreFactsSource interface {
	QueryFacts(ctx context.Context, legalEntityID, dateFrom, dateTo, classification, datasetVersion, sourceSystem string, storeIDs []string) (*repository.RetailKPIFactSet, error)
}

// storePnlKPIAdapter is the production KPI + peer port: the retailkpi
// semantic layer runs (store-360 Build) and the projection only reads its
// summary and peer benchmarks — no KPI is ever recomputed here. One Build
// serves both ports per (store, window) through a single-slot cache, so
// the KPI pass and the per-row peer pass see the same semantic layer.
type storePnlKPIAdapter struct {
	facts StoreFactsSource

	mu           sync.Mutex
	lastKey      string
	lastResponse *retailstore360.Response
}

// NewStorePnlKPIAdapter builds the production adapter; the same instance
// satisfies storepnl.KPIReader and storepnl.PeerReader.
func NewStorePnlKPIAdapter(facts StoreFactsSource) *storePnlKPIAdapter {
	return &storePnlKPIAdapter{facts: facts}
}

func refKey(ref storepnl.StoreRef) string {
	return fmt.Sprintf("%s|%s|%s|%s|%d|%s|%s|%s", ref.LegalEntityID, ref.StoreID, ref.DateFrom, ref.DateTo, ref.WindowDays, ref.Classification, ref.DatasetVersion, ref.SourceSystem)
}

// build runs the semantic layer once per distinct ref; the response value
// is immutable after Build, so a cached pointer is safe to share.
func (a *storePnlKPIAdapter) build(ctx context.Context, ref storepnl.StoreRef) (*retailstore360.Response, error) {
	key := refKey(ref)
	a.mu.Lock()
	if a.lastKey == key && a.lastResponse != nil {
		response := a.lastResponse
		a.mu.Unlock()
		return response, nil
	}
	a.mu.Unlock()

	query := retailstore360.Query{
		LegalEntityID: ref.LegalEntityID, StoreID: ref.StoreID,
		Classification: ref.Classification, DatasetVersion: ref.DatasetVersion,
		SourceSystem: ref.SourceSystem, PeriodLabel: ref.PeriodLabel,
	}
	// S1-2: resolved calendar/rolling window wins; the legacy AsOf+WindowDays
	// anchor stays as the fallback for callers that pass neither.
	if ref.DateFrom != "" && ref.DateTo != "" {
		from, err := time.Parse("2006-01-02", ref.DateFrom)
		if err != nil {
			return nil, err
		}
		to, err := time.Parse("2006-01-02", ref.DateTo)
		if err != nil {
			return nil, err
		}
		query.DateFrom, query.DateTo = from, to
		query.AsOf = to
	} else {
		asOf, err := time.Parse("2006-01-02", ref.AsOf)
		if err != nil {
			return nil, err
		}
		query.AsOf = asOf
		query.WindowDays = ref.WindowDays
	}
	response, err := retailstore360.NewService(a.facts).Build(ctx, query)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.lastKey, a.lastResponse = key, response
	a.mu.Unlock()
	return response, nil
}

// Median serves the S1-6 peer column from the same semantic layer's
// peer benchmarks: insufficient peers or mixed currencies arrive as the
// benchmark's own status, which the projection surfaces instead of a
// number (从不编造同群数字).
func (a *storePnlKPIAdapter) Median(ctx context.Context, ref storepnl.StoreRef, kpi string) (*float64, string, bool) {
	response, err := a.build(ctx, ref)
	if err != nil {
		return nil, "unavailable", false
	}
	for _, b := range response.PeerBenchmark {
		if b.Code != kpi {
			continue
		}
		status := b.Status
		if status == "" {
			status = "complete"
		}
		return b.Median, status, b.Median != nil
	}
	return nil, "unavailable", false
}

func (a *storePnlKPIAdapter) Operating(ctx context.Context, ref storepnl.StoreRef) (storepnl.KPIAggregates, error) {
	response, err := a.build(ctx, ref)
	if err != nil {
		return storepnl.KPIAggregates{}, err
	}
	out := storepnl.KPIAggregates{
		DecisionReady:       response.DecisionReady,
		DecisionReadyReason: response.DecisionReadyReason,
		Classification:      response.DataClassification,
		DatasetVersion:      response.DatasetVersion,
		Currency:            response.Currency,
	}
	code := func(key string) *float64 {
		m, ok := response.Summary[key]
		if !ok {
			return nil
		}
		return m.Current.Value
	}
	out.Revenue = code("revenue")
	out.GrossProfit = code("gross_profit")
	out.LaborCost = code("labor_cost")
	out.OtherControllable = code("other_controllable_cost")
	// 门店 360 语义层不暴露 non_lease/fixed/variable/service 的分项与四墙
	// EBITDA 聚合——这些行保持缺失（诚实降级），待合同级占用成本投影接入。
	return out, nil
}

// storePnlPlanAdapter resolves a second column from one store-grain plan
// version's lines. Version selection belongs to the caller (the UI passes
// plan_version_id); the adapter is a thin binding over
// ListPlanLinesFiltered.
type storePnlPlanAdapter struct {
	repo          *repository.FPnAGovernanceRepository
	planVersionID string
}

// SetStorePnlPlanReader builds a per-request plan adapter for the given
// version.
func SetStorePnlPlanReader(repo *repository.FPnAGovernanceRepository, planVersionID string) storepnl.PlanReader {
	return storePnlPlanAdapter{repo: repo, planVersionID: planVersionID}
}

func (a storePnlPlanAdapter) StoreValue(ctx context.Context, ref storepnl.StoreRef, column storepnl.ColumnRef, kpi string) (*float64, error) {
	if a.repo == nil || a.planVersionID == "" {
		return nil, nil
	}
	filter, err := access.EntityFilterFor(ref.LegalEntityID)
	if err != nil {
		return nil, err
	}
	lines, err := a.repo.ListPlanLines(ctx, a.planVersionID, filter, monthOf(ref.AsOf), "store")
	if err != nil {
		return nil, err
	}
	for _, line := range lines {
		if line.StoreID == nil || *line.StoreID != ref.StoreID {
			continue
		}
		if v := planLineField(line, kpi); v != nil {
			return v, nil
		}
	}
	return nil, nil
}

func planLineField(line *repository.FPnAPlanLine, source string) *float64 {
	switch source {
	case "fact.revenue":
		return line.Revenue
	case "fact.gross_profit":
		return line.GrossProfit
	case "fact.labor_cost":
		return line.LaborCost
	case "fact.fixed_rent":
		return line.FixedRent
	case "fact.variable_rent":
		return line.VariableRent
	case "fact.non_lease_cost":
		return line.NonLeaseCost
	case "fact.four_wall_ebitda":
		return line.FourWallEBITDA
	}
	return nil
}
