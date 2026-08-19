package handlers

import (
	"context"
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

// storePnlKPIAdapter is the production KPI port: the retailkpi semantic
// layer runs (store-360 Build) and the projection only reads its summary —
// no KPI is ever recomputed here.
type storePnlKPIAdapter struct{ facts StoreFactsSource }

// NewStorePnlKPIAdapter builds the production KPI adapter.
func NewStorePnlKPIAdapter(facts StoreFactsSource) storepnl.KPIReader {
	return storePnlKPIAdapter{facts: facts}
}

func (a storePnlKPIAdapter) Operating(ctx context.Context, ref storepnl.StoreRef, period string) (storepnl.KPIAggregates, error) {
	asOf, err := time.Parse("2006-01-02", ref.AsOf)
	if err != nil {
		return storepnl.KPIAggregates{}, err
	}
	response, err := retailstore360.NewService(a.facts).Build(ctx, retailstore360.Query{
		LegalEntityID: ref.LegalEntityID, StoreID: ref.StoreID, AsOf: asOf,
		WindowDays: ref.WindowDays, Classification: ref.Classification,
		DatasetVersion: ref.DatasetVersion,
	})
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
