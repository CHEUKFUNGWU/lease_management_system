package operating

import (
	"testing"

	"github.com/lease-management-system/core-service/internal/repository"
)

func TestFourWallKeepsMissingEvidenceDistinct(t *testing.T) {
	fact := repository.StoreOperatingFact{StoreID: "s1", StoreCode: "S-001", Period: "2026-07", Currency: "CNY", Revenue: 100000, MappingStatus: "mapped", ReconciliationStatus: "warning"}
	result := CalculateFourWall(fact)
	if result.DataReady {
		t.Fatal("warning fact must not be decision-ready")
	}
	if result.FourWallEBITDA != nil || result.RentToSales != nil {
		t.Fatal("missing costs must not produce a fabricated metric")
	}
	if len(result.DataGaps) == 0 {
		t.Fatal("missing fields should be exposed")
	}
}

// KPI-001: the four-wall metrics now come from the retail-kpi-v1 semantic
// layer. Rent-to-sales includes variable rent, occupancy cost ratio includes
// non-lease cost, and a missing required field (other_controllable_cost)
// leaves the metric null instead of treating the missing value as zero.
func TestFourWallCalculatesGovernedMetrics(t *testing.T) {
	gp, labor, rent, variable, nonLease, other, area := 60000.0, 10000.0, 12000.0, 3000.0, 5000.0, 2000.0, 100.0
	fact := repository.StoreOperatingFact{StoreID: "s1", Period: "2026-07", Currency: "CNY", Revenue: 100000, GrossProfit: &gp, LaborCost: &labor, FixedRent: &rent, VariableRent: &variable, NonLeaseCost: &nonLease, OtherControllableCost: &other, AreaSqm: &area, MappingStatus: "mapped", ReconciliationStatus: "matched"}
	result := CalculateFourWall(fact)
	if !result.DataReady {
		t.Fatal("matched mapped fact should be ready")
	}
	if result.FourWallEBITDA == nil || *result.FourWallEBITDA != 28000 {
		t.Fatalf("four wall EBITDA=%v", result.FourWallEBITDA)
	}
	if result.RentToSales == nil || *result.RentToSales != 15 {
		t.Fatalf("rent to sales=%v", result.RentToSales)
	}
	if result.OccupancyCostRatio == nil || *result.OccupancyCostRatio != 20 {
		t.Fatalf("occupancy cost ratio=%v", result.OccupancyCostRatio)
	}
	if result.SalesPerSqm == nil || *result.SalesPerSqm != 1000 {
		t.Fatalf("sales per sqm=%v", result.SalesPerSqm)
	}
}

// K2: a zero-revenue store must not receive a fabricated ratio. The old
// engine's max(revenue, 1) invented a denominator; retail-kpi-v1 returns
// null for every zero-denominator metric.
func TestFourWallZeroRevenueYieldsNullRatios(t *testing.T) {
	gp, labor, rent, variable, nonLease, other, area := 0.0, 10000.0, 12000.0, 3000.0, 5000.0, 2000.0, 100.0
	fact := repository.StoreOperatingFact{StoreID: "s1", Period: "2026-07", Currency: "CNY", Revenue: 0, GrossProfit: &gp, LaborCost: &labor, FixedRent: &rent, VariableRent: &variable, NonLeaseCost: &nonLease, OtherControllableCost: &other, AreaSqm: &area, MappingStatus: "mapped", ReconciliationStatus: "matched"}
	result := CalculateFourWall(fact)
	if result.ContributionMargin != nil {
		t.Fatalf("zero-revenue contribution margin=%v, want null", *result.ContributionMargin)
	}
	if result.RentToSales != nil {
		t.Fatalf("zero-revenue rent to sales=%v, want null", *result.RentToSales)
	}
	if result.OccupancyCostRatio != nil {
		t.Fatalf("zero-revenue occupancy ratio=%v, want null", *result.OccupancyCostRatio)
	}
	// Sales per sqm divides by area, not revenue: 0 revenue on a real area
	// is a computable zero, not a fabricated denominator.
	if result.SalesPerSqm == nil || *result.SalesPerSqm != 0 {
		t.Fatalf("zero-revenue sales per sqm=%v, want 0", result.SalesPerSqm)
	}
	if result.BreakEvenSales != nil {
		t.Fatalf("zero-revenue break even=%v, want null", *result.BreakEvenSales)
	}
}

// KPI-001: a missing other_controllable_cost leaves contribution null — the
// unified engine never treats a missing required field as zero.
func TestFourWallMissingOtherCostLeavesContributionNull(t *testing.T) {
	gp, labor, rent, variable, nonLease, area := 60000.0, 10000.0, 12000.0, 3000.0, 5000.0, 100.0
	fact := repository.StoreOperatingFact{StoreID: "s1", Period: "2026-07", Currency: "CNY", Revenue: 100000, GrossProfit: &gp, LaborCost: &labor, FixedRent: &rent, VariableRent: &variable, NonLeaseCost: &nonLease, AreaSqm: &area, MappingStatus: "mapped", ReconciliationStatus: "matched"}
	result := CalculateFourWall(fact)
	if result.FourWallEBITDA != nil || result.ContributionMargin != nil {
		t.Fatalf("missing other_controllable_cost EBITDA=%v margin=%v, want null", result.FourWallEBITDA, result.ContributionMargin)
	}
}

func TestCostBridgeLeavesResidualExplicit(t *testing.T) {
	standard, actual, material, labor, energy, overhead := 100.0, 130.0, 10.0, 8.0, 4.0, 3.0
	bridge, err := CalculateCostBridge(repository.EquipmentOperatingFact{Period: "2026-07", Currency: "CNY", StandardCost: &standard, ActualCost: &actual, MaterialUsageCost: &material, LaborCost: &labor, EnergyCost: &energy, OverheadAbsorption: &overhead})
	if err != nil {
		t.Fatal(err)
	}
	if !bridge.TiesOut || bridge.Residual != 5 {
		t.Fatalf("bridge=%+v", bridge)
	}
}
