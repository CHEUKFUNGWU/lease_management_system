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

func TestFourWallCalculatesGovernedMetrics(t *testing.T) {
	gp, labor, rent, variable, nonLease, area := 60000.0, 10000.0, 12000.0, 3000.0, 5000.0, 100.0
	fact := repository.StoreOperatingFact{StoreID: "s1", Period: "2026-07", Currency: "CNY", Revenue: 100000, GrossProfit: &gp, LaborCost: &labor, FixedRent: &rent, VariableRent: &variable, NonLeaseCost: &nonLease, AreaSqm: &area, MappingStatus: "mapped", ReconciliationStatus: "matched"}
	result := CalculateFourWall(fact)
	if !result.DataReady {
		t.Fatal("matched mapped fact should be ready")
	}
	if result.FourWallEBITDA == nil || *result.FourWallEBITDA != 30000 {
		t.Fatalf("four wall EBITDA=%v", result.FourWallEBITDA)
	}
	if result.RentToSales == nil || *result.RentToSales != 12 {
		t.Fatalf("rent to sales=%v", result.RentToSales)
	}
	if result.SalesPerSqm == nil || *result.SalesPerSqm != 1000 {
		t.Fatalf("sales per sqm=%v", result.SalesPerSqm)
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
