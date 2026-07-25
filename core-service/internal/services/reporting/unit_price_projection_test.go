package reporting

import (
	"math"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/repository"
)

func areaContract(id, store, currency string, area *float64, monthlyRent float64, months int) ContractFact {
	commencement := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	schedules := make([]*repository.PaymentSchedule, 0, months)
	for month := 0; month < months; month++ {
		schedules = append(schedules, &repository.PaymentSchedule{
			DueDate: commencement.AddDate(0, month, 0), Amount: monthlyRent,
			IsFixed: true, IsLeaseComponent: true,
		})
	}
	return ContractFact{
		Contract: &repository.Contract{
			ID: id, ContractNumber: id, ContractName: id, StoreName: store,
			Currency: currency, AreaSqm: area,
			CommencementDate: commencement,
			LeaseEndDate:     commencement.AddDate(0, months, 0),
		},
		PaymentSchedules: schedules,
	}
}

func area(value float64) *float64 { return &value }

func unitPriceRows(t *testing.T, snapshot *Snapshot, groupBy string) ([]UnitPriceRow, map[string]any) {
	t.Helper()
	result, err := Project(snapshot, ProjectionRequest{Kind: KindUnitPrice, View: groupBy})
	if err != nil {
		t.Fatalf("project unit price: %v", err)
	}
	rows, ok := result.Payload["data"].([]UnitPriceRow)
	if !ok {
		t.Fatalf("unexpected payload: %#v", result.Payload["data"])
	}
	return rows, result.Payload
}

func TestUnitPriceDividesStraightLineRentByArea(t *testing.T) {
	// 100 sqm at 20,000/month over 12 months -> 200/sqm.
	snapshot := &Snapshot{
		ID: "s1", PolicyVersion: policyVersion, Mode: Working, GeneratedAt: time.Now(),
		Contracts: []ContractFact{areaContract("c1", "南京东路店", "CNY", area(100), 20000, 12)},
	}

	rows, _ := unitPriceRows(t, snapshot, GroupByStore)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if math.Abs(row.MonthlyRentPerSqm-200) > 1 {
		t.Errorf("rent per sqm = %.2f, want ~200", row.MonthlyRentPerSqm)
	}
	if math.Abs(row.TotalAreaSqm-100) > 0.01 {
		t.Errorf("area = %.2f, want 100", row.TotalAreaSqm)
	}
	if math.Abs(row.AnnualFixedRent-row.MonthlyFixedRent*12) > 0.5 {
		t.Errorf("annual %.2f is not 12x monthly %.2f", row.AnnualFixedRent, row.MonthlyFixedRent)
	}
}

// A lease with a rent-free period must compare on its straight-lined average,
// otherwise a shop looks cheap purely because of how its schedule is shaped.
func TestUnitPriceStraightLinesUnevenSchedules(t *testing.T) {
	flat := areaContract("flat", "A店", "CNY", area(100), 10000, 12)

	stepped := areaContract("stepped", "B店", "CNY", area(100), 0, 0)
	commencement := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	stepped.Contract.LeaseEndDate = commencement.AddDate(0, 12, 0)
	// Same 120,000 total, but two rent-free months then higher instalments.
	for month := 0; month < 12; month++ {
		amount := 0.0
		if month >= 2 {
			amount = 120000.0 / 10
		}
		stepped.PaymentSchedules = append(stepped.PaymentSchedules, &repository.PaymentSchedule{
			DueDate: commencement.AddDate(0, month, 0), Amount: amount,
			IsFixed: true, IsLeaseComponent: true,
		})
	}

	snapshot := &Snapshot{
		ID: "s2", PolicyVersion: policyVersion, Mode: Working, GeneratedAt: time.Now(),
		Contracts: []ContractFact{flat, stepped},
	}
	rows, _ := unitPriceRows(t, snapshot, GroupByStore)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if math.Abs(rows[0].MonthlyRentPerSqm-rows[1].MonthlyRentPerSqm) > 1 {
		t.Errorf("same total rent and area must give the same unit price: %.2f vs %.2f",
			rows[0].MonthlyRentPerSqm, rows[1].MonthlyRentPerSqm)
	}
}

// A contract with no recorded area must stay out of both sides of the ratio.
// Counting its rent but not its area would understate the group's unit price.
func TestUnitPriceExcludesContractsWithoutAreaFromBothSides(t *testing.T) {
	withArea := areaContract("c1", "华东仓", "CNY", area(100), 20000, 12)
	withoutArea := areaContract("c2", "华东仓", "CNY", nil, 999999, 12)

	snapshot := &Snapshot{
		ID: "s3", PolicyVersion: policyVersion, Mode: Working, GeneratedAt: time.Now(),
		Contracts: []ContractFact{withArea, withoutArea},
	}
	rows, payload := unitPriceRows(t, snapshot, GroupByStore)
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1 group", len(rows))
	}
	row := rows[0]
	if math.Abs(row.MonthlyRentPerSqm-200) > 1 {
		t.Errorf("rent per sqm = %.2f, want ~200 (the area-less lease must not count)", row.MonthlyRentPerSqm)
	}
	if row.ContractCount != 2 || row.AreaCoverageCount != 1 {
		t.Errorf("coverage = %d/%d, want 1 of 2", row.AreaCoverageCount, row.ContractCount)
	}
	if payload["contracts_without_area"] != 1 || payload["area_basis_caveat"] != true {
		t.Errorf("payload must flag the incomplete area basis: %#v", payload)
	}
}

// Mixing currencies in one average would produce a meaningless number.
func TestUnitPriceKeepsCurrenciesApart(t *testing.T) {
	snapshot := &Snapshot{
		ID: "s4", PolicyVersion: policyVersion, Mode: Working, GeneratedAt: time.Now(),
		Contracts: []ContractFact{
			areaContract("c1", "铜锣湾店", "HKD", area(100), 30000, 12),
			areaContract("c2", "铜锣湾店", "CNY", area(100), 20000, 12),
		},
	}
	rows, _ := unitPriceRows(t, snapshot, GroupByStore)
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want one per currency", len(rows))
	}
	for _, row := range rows {
		if row.Currency == "" {
			t.Error("every row must state its currency")
		}
	}
}

func TestUnitPriceGroupsByBrandAndRegion(t *testing.T) {
	east := areaContract("c1", "A店", "CNY", area(100), 20000, 12)
	east.Brand, east.Region = "主品牌", "华东"
	north := areaContract("c2", "B店", "CNY", area(100), 10000, 12)
	north.Brand, north.Region = "主品牌", "华北"

	snapshot := &Snapshot{
		ID: "s5", PolicyVersion: policyVersion, Mode: Working, GeneratedAt: time.Now(),
		Contracts: []ContractFact{east, north},
	}

	brandRows, _ := unitPriceRows(t, snapshot, GroupByBrand)
	if len(brandRows) != 1 || brandRows[0].GroupLabel != "主品牌" {
		t.Fatalf("brand rows = %#v, want one 主品牌 group", brandRows)
	}
	// 30,000 total monthly rent over 200 sqm.
	if math.Abs(brandRows[0].MonthlyRentPerSqm-150) > 1 {
		t.Errorf("brand rent per sqm = %.2f, want ~150", brandRows[0].MonthlyRentPerSqm)
	}

	regionRows, _ := unitPriceRows(t, snapshot, GroupByRegion)
	if len(regionRows) != 2 {
		t.Fatalf("region rows = %d, want 2", len(regionRows))
	}
	// Most expensive first, so a BP reads the outliers at the top.
	if regionRows[0].GroupLabel != "华东" {
		t.Errorf("rows must be sorted by unit price desc, got %s first", regionRows[0].GroupLabel)
	}
}

func TestUnitPriceLabelsUnassignedGroups(t *testing.T) {
	fact := areaContract("c1", "", "CNY", area(50), 5000, 12)
	snapshot := &Snapshot{
		ID: "s6", PolicyVersion: policyVersion, Mode: Working, GeneratedAt: time.Now(),
		Contracts: []ContractFact{fact},
	}
	rows, _ := unitPriceRows(t, snapshot, GroupByBrand)
	if len(rows) != 1 || rows[0].GroupLabel != unassignedGroupLabel {
		t.Fatalf("rows = %#v, want an explicit unassigned group", rows)
	}
}

func TestUnitPriceRejectsUnknownGrouping(t *testing.T) {
	snapshot := &Snapshot{ID: "s7", PolicyVersion: policyVersion, Mode: Working, GeneratedAt: time.Now()}
	if _, err := Project(snapshot, ProjectionRequest{Kind: KindUnitPrice, View: "asset_type"}); err == nil {
		t.Fatal("expected an error for an unsupported grouping")
	}
}
