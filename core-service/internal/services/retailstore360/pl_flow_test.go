package retailstore360

import (
	"context"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

type plFlowFactReader struct {
	facts  []retailkpi.DailyFact
	stores []retailkpi.StorePopulation
}

func (r plFlowFactReader) QueryFacts(context.Context, string, string, string, string, string, string, []string) (*repository.RetailKPIFactSet, error) {
	return &repository.RetailKPIFactSet{Facts: r.facts, ExpectedStores: r.stores}, nil
}

func f64(v float64) *float64 { return &v }

func plFlowFixture() []retailkpi.DailyFact {
	// 7-day window, one store, all fields present, CNY.
	day := time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC)
	facts := make([]retailkpi.DailyFact, 0, 7)
	for i := 0; i < 7; i++ {
		facts = append(facts, retailkpi.DailyFact{
			StoreID: "store-1", BusinessDate: day.AddDate(0, 0, -i), Currency: "CNY",
			Revenue: f64(1000), LaborCost: f64(300), FixedRent: f64(200),
			VariableRent: f64(50), NonLeaseCost: f64(100), OtherControllableCost: f64(150),
		})
	}
	return facts
}

func plFlowStore() []retailkpi.StorePopulation {
	return []retailkpi.StorePopulation{{StoreID: "store-1", StoreCode: "S-1", StoreName: "门店1"}}
}

func TestPlFlowCompleteAllFieldsPresent(t *testing.T) {
	svc := NewService(plFlowFactReader{facts: plFlowFixture(), stores: plFlowStore()})
	result, err := svc.PlFlow(context.Background(), Query{
		LegalEntityID: "le-1", StoreID: "store-1",
		AsOf: time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC), WindowDays: 7,
		Classification: "production",
	})
	if err != nil {
		t.Fatalf("PlFlow error = %v", err)
	}
	if result.Status != "complete" {
		t.Fatalf("status = %q, want complete", result.Status)
	}
	if result.Currency != "CNY" {
		t.Fatalf("currency = %q", result.Currency)
	}
	// 7 days: revenue 7000, labor 2100, rent 1750, non_lease 700, other 1050
	// contribution = 7000 - (2100+1750+700+1050) = 1400
	byTo := map[string]float64{}
	for _, link := range result.Links {
		byTo[link.To] = link.Value
	}
	if byTo["labor"] != 2100 || byTo["rent"] != 1750 || byTo["non_lease"] != 700 || byTo["other"] != 1050 {
		t.Fatalf("cost links = %#v", byTo)
	}
	if byTo["contribution"] != 1400 {
		t.Fatalf("contribution = %v, want 1400", byTo["contribution"])
	}
	if result.Residual != 0 {
		t.Fatalf("residual = %v, want 0 when complete", result.Residual)
	}
	if len(result.Links) != 5 {
		t.Fatalf("links = %d, want 5", len(result.Links))
	}
}

func TestPlFlowPartialExposesMissingFields(t *testing.T) {
	facts := plFlowFixture()
	// 去掉 labor_cost（3 天为 nil）→ labor 只覆盖 4/7 天
	for i := 0; i < 3; i++ {
		facts[i].LaborCost = nil
	}
	svc := NewService(plFlowFactReader{facts: facts, stores: plFlowStore()})
	result, err := svc.PlFlow(context.Background(), Query{
		LegalEntityID: "le-1", StoreID: "store-1",
		AsOf: time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC), WindowDays: 7,
		Classification: "production",
	})
	if err != nil {
		t.Fatalf("PlFlow error = %v", err)
	}
	if result.Status != "partial" {
		t.Fatalf("status = %q, want partial", result.Status)
	}
	if len(result.Missing) != 1 || result.Missing[0] != "labor_cost" {
		t.Fatalf("missing = %#v, want [labor_cost]", result.Missing)
	}
	// 观测口径：labor = 4 天 × 300 = 1200；贡献按观测费用 = 7000-(1200+1750+700+1050) = 2300
	byTo := map[string]float64{}
	for _, link := range result.Links {
		byTo[link.To] = link.Value
	}
	if byTo["labor"] != 1200 {
		t.Fatalf("labor = %v, want 1200 (observed days only)", byTo["labor"])
	}
	if byTo["contribution"] != 2300 {
		t.Fatalf("contribution = %v, want 2300", byTo["contribution"])
	}
	// 观测流自洽：残差 0，缺失由 status/missing 表达
	if result.Residual != 0 {
		t.Fatalf("residual = %v, want 0 (missing expressed via status)", result.Residual)
	}
}

func TestPlFlowAllCostsMissingExposesResidual(t *testing.T) {
	facts := plFlowFixture()
	for i := range facts {
		facts[i].LaborCost = nil
		facts[i].FixedRent = nil
		facts[i].VariableRent = nil
		facts[i].NonLeaseCost = nil
		facts[i].OtherControllableCost = nil
	}
	svc := NewService(plFlowFactReader{facts: facts, stores: plFlowStore()})
	result, err := svc.PlFlow(context.Background(), Query{
		LegalEntityID: "le-1", StoreID: "store-1",
		AsOf: time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC), WindowDays: 7,
		Classification: "production",
	})
	if err != nil {
		t.Fatalf("PlFlow error = %v", err)
	}
	if result.Status != "partial" {
		t.Fatalf("status = %q, want partial", result.Status)
	}
	// 无任何费用可观测 → 无贡献流，residual = revenue = 7000
	for _, link := range result.Links {
		if link.To == "contribution" {
			t.Fatalf("contribution link present while no cost is observable")
		}
	}
	if result.Residual != 7000 {
		t.Fatalf("residual = %v, want 7000 (whole revenue unattributed)", result.Residual)
	}
	if len(result.Links) != 4 {
		t.Fatalf("links = %d, want 4 zero-value cost links", len(result.Links))
	}
}

func TestPlFlowUnavailableWithoutFacts(t *testing.T) {
	svc := NewService(plFlowFactReader{facts: nil, stores: plFlowStore()})
	result, err := svc.PlFlow(context.Background(), Query{
		LegalEntityID: "le-1", StoreID: "store-1",
		AsOf: time.Date(2026, 6, 7, 0, 0, 0, 0, time.UTC), WindowDays: 7,
		Classification: "production",
	})
	if err != nil {
		t.Fatalf("PlFlow error = %v", err)
	}
	if result.Status != "unavailable" {
		t.Fatalf("status = %q, want unavailable", result.Status)
	}
	if len(result.Links) != 0 {
		t.Fatalf("links = %d, want 0", len(result.Links))
	}
}
