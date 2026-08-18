package cashplan

import (
	"context"
	"testing"

	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

type fakeOperatingReader struct {
	facts []OperatingFact
}

func (f *fakeOperatingReader) ReadOperating(ctx context.Context, legalEntityID, fromPeriod, toPeriod, classification, datasetVersion string, storeIDs []string) ([]OperatingFact, error) {
	return f.facts, nil
}

type fakeLeaseReader struct {
	facts []LeasePaymentFact
}

func (f *fakeLeaseReader) ReadLeasePayments(ctx context.Context, legalEntityID, fromPeriod, toPeriod string, storeIDs []string) ([]LeasePaymentFact, error) {
	return f.facts, nil
}

type fakeCapexReader struct {
	facts []CapexFact
}

func (f *fakeCapexReader) ReadCapex(ctx context.Context, legalEntityID, fromPeriod, toPeriod string, storeIDs []string) ([]CapexFact, error) {
	return f.facts, nil
}

func floatPtr(v float64) *float64 { return &v }

func TestCashPlanRentDeDuplicationAndConservation(t *testing.T) {
	// Scenario:
	// Store 1 in 2026-01:
	// Revenue = 100,000
	// Labor = 15,000, NonLease = 5,000
	// Operating Fixed Rent = 20,000, Variable Rent = 2,000
	// Operating Cash Flow = 100,000 - 15,000 - 5,000 - 22,000 = 58,000.
	//
	// Lease Payment Schedule for 2026-01:
	// Actual Lease Payment = Fixed 20,000 + Variable 2,000 + Tax 1,000 = 23,000.
	//
	// Capex for 2026-01:
	// Store Renovation = 10,000.
	//
	// Net Cash Plan calculation:
	// Without de-duplication: 58,000 - 23,000 - 10,000 = 25,000 (WRONG: Rent 22,000 deducted twice!)
	// With de-duplication: Operating Cash 58,000 + Rent Offset 22,000 - Lease Outflow 23,000 - Capex 10,000 = 47,000.
	// (True Cash Inflow 80,000 - Lease Outflow 23,000 - Capex 10,000 = 47,000).

	sources := Sources{
		Operating: &fakeOperatingReader{
			facts: []OperatingFact{
				{
					StoreID:       "store-1",
					StoreCode:     "S001",
					StoreName:     "Flagship Store",
					Period:        "2026-01",
					Currency:      "CNY",
					Revenue:       100000,
					LaborCost:     15000,
					NonLeaseCost:  5000,
					FixedRent:     20000,
					VariableRent:  2000,
					OperatingCash: 58000,
					Coverage:      retailkpi.Coverage{CoverageRate: floatPtr(100.0)},
				},
			},
		},
		Lease: &fakeLeaseReader{
			facts: []LeasePaymentFact{
				{
					ContractID:   "contract-1",
					StoreID:      "store-1",
					Period:       "2026-01",
					Currency:     "CNY",
					FixedRent:    20000,
					VariableRent: 2000,
					Tax:          1000,
					TotalOutflow: 23000,
				},
			},
		},
		Capex: &fakeCapexReader{
			facts: []CapexFact{
				{
					StoreID:  "store-1",
					Period:   "2026-01",
					Currency: "CNY",
					Category: "renovation",
					Amount:   10000,
				},
			},
		},
	}

	plan, err := Compose(context.Background(), Request{
		LegalEntityID: "entity-1",
		FromPeriod:    "2026-01",
		ToPeriod:      "2026-01",
	}, sources)
	if err != nil {
		t.Fatalf("Compose failed: %v", err)
	}

	if len(plan.Partitions) != 1 {
		t.Fatalf("expected 1 partition, got %d", len(plan.Partitions))
	}
	p := plan.Partitions[0]

	if p.TotalOperatingCash != 58000 {
		t.Errorf("expected OperatingCash 58000, got %f", p.TotalOperatingCash)
	}
	if p.TotalRentOffset != 22000 {
		t.Errorf("expected RentOffset 22000, got %f", p.TotalRentOffset)
	}
	if p.TotalLeaseOutflow != 23000 {
		t.Errorf("expected LeaseOutflow 23000, got %f", p.TotalLeaseOutflow)
	}
	if p.TotalCapexOutflow != 10000 {
		t.Errorf("expected CapexOutflow 10000, got %f", p.TotalCapexOutflow)
	}
	if p.TotalNetCashPlan != 47000 {
		t.Errorf("expected NetCashPlan 47000 (with rent de-duplication), got %f", p.TotalNetCashPlan)
	}

	// Conservation assertion
	if !p.Bridge.IsConserved {
		t.Errorf("expected Bridge to be conserved, got residual %f", p.Bridge.RoundingResidual)
	}
	if p.Bridge.RoundingResidual != 0 {
		t.Errorf("expected 0 residual, got %f", p.Bridge.RoundingResidual)
	}
}

func TestCashPlanCoverageDegradation(t *testing.T) {
	sources := Sources{
		Operating: &fakeOperatingReader{
			facts: []OperatingFact{
				{
					StoreID:       "store-1",
					Period:        "2026-01",
					Currency:      "CNY",
					Revenue:       50000,
					OperatingCash: 30000,
					Coverage:      retailkpi.Coverage{CoverageRate: floatPtr(85.0)}, // Incomplete coverage
				},
			},
		},
	}

	plan, err := Compose(context.Background(), Request{
		LegalEntityID: "entity-1",
		FromPeriod:    "2026-01",
		ToPeriod:      "2026-01",
	}, sources)
	if err != nil {
		t.Fatalf("Compose failed: %v", err)
	}

	p := plan.Partitions[0]
	if p.DecisionReady {
		t.Errorf("expected decision_ready=false due to 85%% coverage, got true")
	}
	if p.WeakestCoverageRatio == nil || *p.WeakestCoverageRatio != 85.0 {
		t.Errorf("expected weakest_coverage_ratio=85.0, got %+v", p.WeakestCoverageRatio)
	}
}

func TestCashPlanMultiCurrencyPartition(t *testing.T) {
	sources := Sources{
		Operating: &fakeOperatingReader{
			facts: []OperatingFact{
				{StoreID: "s1", Period: "2026-01", Currency: "CNY", Revenue: 10000, OperatingCash: 8000},
				{StoreID: "s2", Period: "2026-01", Currency: "HKD", Revenue: 20000, OperatingCash: 16000},
			},
		},
	}

	plan, err := Compose(context.Background(), Request{
		LegalEntityID: "entity-1",
		FromPeriod:    "2026-01",
		ToPeriod:      "2026-01",
	}, sources)
	if err != nil {
		t.Fatalf("Compose failed: %v", err)
	}

	if !plan.MultiCurrency {
		t.Errorf("expected MultiCurrency=true, got false")
	}
	if len(plan.Partitions) != 2 {
		t.Fatalf("expected 2 currency partitions (CNY, HKD), got %d", len(plan.Partitions))
	}
	if plan.Partitions[0].Currency != "CNY" || plan.Partitions[1].Currency != "HKD" {
		t.Errorf("currency partition mismatch: %+v", plan.Partitions)
	}
}
