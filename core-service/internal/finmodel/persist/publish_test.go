package persist

import (
	"testing"

	"github.com/lease-management-system/core-service/internal/repository"
)

func pf(v float64) *float64 { return &v }

func TestApplyColumnMapsKnownKeysAndReportsUnknown(t *testing.T) {
	row := &repository.FPnAPlanLine{}
	if !applyColumn(&repository.FinModelRunLine{RowKey: "rev", Value: pf(100)}, row) || row.Revenue == nil || *row.Revenue != 100 {
		t.Fatalf("rev must land on revenue: %+v", row)
	}
	if !applyColumn(&repository.FinModelRunLine{RowKey: "operating_ebitda", Value: pf(42)}, row) || row.FourWallEBITDA == nil || *row.FourWallEBITDA != 42 {
		t.Fatalf("operating_ebitda must land on four_wall_ebitda: %+v", row)
	}
	if !applyColumn(&repository.FinModelRunLine{RowKey: "capex", Value: pf(7)}, row) || row.Capex == nil {
		t.Fatalf("capex must land on capex: %+v", row)
	}
	// 模板自定义行（如自定义公式行）没有 plan-lines 列：not silent。
	if applyColumn(&repository.FinModelRunLine{RowKey: "custom_ratio", Value: pf(0.1)}, row) {
		t.Fatal("custom row key must report unmapped, not silently stash a value")
	}
}

func TestPlanColumnsCoverTheStatementRows(t *testing.T) {
	for key, column := range map[string]string{
		"rev": "revenue", "gp": "gross_profit", "labor": "labor_cost",
		"fixed_rent": "fixed_rent", "variable_rent": "variable_rent",
		"non_lease": "non_lease_cost", "operating_ebitda": "four_wall_ebitda",
		"four_wall_ebitda": "four_wall_ebitda", "cash_flow": "cash_flow",
		"capex": "capex", "net_debt": "net_debt",
	} {
		if got := planColumns[key]; got != column {
			t.Fatalf("planColumns[%s] = %q, want %q", key, got, column)
		}
	}
}
