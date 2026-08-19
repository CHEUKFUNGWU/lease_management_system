package storepnl

import (
	"testing"
)

func TestFoldContractOccupancyBucketsAndPrates(t *testing.T) {
	rows := []OccupancySchedule{
		// 合同 1：全月基本租金 3000，服务费 300，变量租金 600。
		{ContractID: "C1", ContractNumber: "CT-001", CoverageStart: "2026-07-01", CoverageEnd: "2026-07-31", Amount: 3000},
		{ContractID: "C1", ContractNumber: "CT-001", CoverageStart: "2026-07-01", CoverageEnd: "2026-07-31", Amount: 300, IsNonLease: true},
		{ContractID: "C1", ContractNumber: "CT-001", CoverageStart: "2026-07-01", CoverageEnd: "2026-07-31", Amount: 600, IsVariable: true},
		// 合同 2：指数调整固定租金（按基本租金归类）。
		{ContractID: "C2", ContractNumber: "CT-002", CoverageStart: "2026-07-01", CoverageEnd: "2026-07-31", Amount: 1200},
		// 窗口外行：不进入。
		{ContractID: "C2", ContractNumber: "CT-002", CoverageStart: "2026-08-01", CoverageEnd: "2026-08-31", Amount: 9999},
		// 无覆盖交叉的行：跳过。
		{ContractID: "C3", ContractNumber: "CT-003", CoverageStart: "2026-01-01", CoverageEnd: "2026-06-30", Amount: 555},
	}
	splits := FoldContractOccupancy(rows, "2026-07-01", "2026-07-31")
	if len(splits) != 2 {
		t.Fatalf("splits = %+v", splits)
	}
	if splits[0].ContractID != "C1" || *splits[0].BasicRent != 3000 || *splits[0].ServiceFee != 300 || *splits[0].VariableRent != 600 {
		t.Fatalf("C1 split wrong: %+v", splits[0])
	}
	if splits[1].ContractID != "C2" || *splits[1].BasicRent != 1200 || splits[1].ServiceFee != nil {
		t.Fatalf("C2 split wrong: %+v", splits[1])
	}

	basic, service, variable := ComponentSum(splits)
	if basic == nil || *basic != 4200 || service == nil || *service != 300 || variable == nil || *variable != 600 {
		t.Fatalf("aggregate components must equal the split sums: %v/%v/%v", basic, service, variable)
	}
}

func TestFoldContractOccupancyPartialWindowPrates(t *testing.T) {
	rows := []OccupancySchedule{{
		ContractID: "C1", ContractNumber: "CT-001",
		CoverageStart: "2026-07-01", CoverageEnd: "2026-07-31",
		Amount: 310,
	}}
	// 10 天窗口（7-01..7-10）→ 310 × 10/31。
	splits := FoldContractOccupancy(rows, "2026-07-01", "2026-07-10")
	if len(splits) != 1 || splits[0].BasicRent == nil {
		t.Fatalf("splits = %+v", splits)
	}
	expected := 310.0 * 10 / 31
	if diff := *splits[0].BasicRent - expected; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("prorated basic rent = %v, want %v", *splits[0].BasicRent, expected)
	}

	// 无重叠 → 空。
	if got := FoldContractOccupancy(rows, "2026-08-01", "2026-08-31"); len(got) != 0 {
		t.Fatalf("no-overlap window must yield no splits: %+v", got)
	}
	// 非法窗口 → nil，绝不 panic。
	if got := FoldContractOccupancy(rows, "2026-08-01", "2026-07-01"); got != nil {
		t.Fatalf("inverted window must refuse: %+v", got)
	}
}

func TestFoldContractOccupancySkipsEmptyContracts(t *testing.T) {
	rows := []OccupancySchedule{
		// 该合同在窗口内只有完全无金额行外的行：覆盖为零即整合同无任何计数，
		// 不出现空合同行。
		{ContractID: "C1", ContractNumber: "CT-001", CoverageStart: "2026-08-01", CoverageEnd: "2026-08-31", Amount: 100},
	}
	if got := FoldContractOccupancy(rows, "2026-07-01", "2026-07-31"); len(got) != 0 {
		t.Fatalf("contracts with no in-window amounts must not appear: %+v", got)
	}
}
