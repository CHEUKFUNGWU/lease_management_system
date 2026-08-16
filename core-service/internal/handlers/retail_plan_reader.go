package handlers

import (
	"context"

	"github.com/lease-management-system/core-service/internal/access"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

// RetailPlanReader adapts the FP&A plan repository (fpna_plan_versions /
// fpna_plan_lines, store grain) to the retail PlanReader seam — the first
// real adapter; the fixed-seed simulated plan is the second.
type RetailPlanReader struct {
	repo *repository.FPnAGovernanceRepository
	// PreferredVersionTypes is tried in order for a period; budget first so
	// "actual vs budget" is the default comparison basis.
	PreferredVersionTypes []string
}

func NewRetailPlanReader(repo *repository.FPnAGovernanceRepository) *RetailPlanReader {
	return &RetailPlanReader{repo: repo, PreferredVersionTypes: []string{"budget", "forecast"}}
}

// ReadPlan resolves the authoritative plan version covering the period
// (official first, then the newest as_of snapshot — spec decision 1/2),
// then loads its store-grain lines. A nil set means no version covers the
// period; that absence is reported as "no plan block", never a zero.
func (r *RetailPlanReader) ReadPlan(ctx context.Context, legalEntityID, period string) (*retailkpi.PlanSet, error) {
	entity, err := access.EntityFilterFor(legalEntityID)
	if err != nil {
		return nil, err
	}
	var version *repository.FPnAPlanVersion
	for _, versionType := range r.PreferredVersionTypes {
		version, err = r.repo.ResolvePlanVersionForPeriod(ctx, entity, period, versionType)
		if err != nil {
			return nil, err
		}
		if version != nil {
			break
		}
	}
	if version == nil {
		return nil, nil
	}
	lines, err := r.repo.ListPlanLinesFiltered(ctx, version.ID, entity, period, "store", map[string]string{})
	if err != nil {
		return nil, err
	}
	facts := make([]retailkpi.PlanFact, 0, len(lines))
	for _, line := range lines {
		if line.StoreID == nil {
			continue
		}
		fact := retailkpi.PlanFact{StoreID: *line.StoreID, Period: line.Period, Currency: line.Currency}
		if line.Revenue != nil {
			value := float64(*line.Revenue)
			fact.Revenue = &value
		}
		if line.GrossProfit != nil {
			value := float64(*line.GrossProfit)
			fact.GrossProfit = &value
		}
		if line.LaborCost != nil {
			value := float64(*line.LaborCost)
			fact.LaborCost = &value
		}
		if line.FixedRent != nil {
			value := float64(*line.FixedRent)
			fact.FixedRent = &value
		}
		if line.VariableRent != nil {
			value := float64(*line.VariableRent)
			fact.VariableRent = &value
		}
		if line.NonLeaseCost != nil {
			value := float64(*line.NonLeaseCost)
			fact.NonLeaseCost = &value
		}
		facts = append(facts, fact)
	}
	if len(facts) == 0 {
		return nil, nil
	}
	return &retailkpi.PlanSet{
		VersionID: version.ID, VersionName: version.Name, VersionType: version.VersionType,
		AsOfPeriod: version.AsOfPeriod, Source: version.Source, IsOfficial: version.IsOfficial,
		Facts: facts,
	}, nil
}
