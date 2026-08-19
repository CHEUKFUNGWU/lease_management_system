package adapter

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/finmodel/opening"
	"github.com/lease-management-system/core-service/internal/money"
	"github.com/lease-management-system/core-service/internal/repository"
)

func moneyPtr(v float64) money.Amount { return money.NewFromFloat(v) }

// —— Fake seams ——

type memMeasurements map[string][]*repository.MeasurementResult

func (m memMeasurements) ListMeasurementResultsByEntityPeriod(_ context.Context, _, period string) ([]*repository.MeasurementResult, error) {
	return m[period], nil
}

func measurement(contract string, openingLiab, interest, principal, payment, openingROU, depreciation float64, start, end time.Time) *repository.MeasurementResult {
	return &repository.MeasurementResult{
		ContractID: contract, OpeningLiability: moneyPtr(openingLiab), InterestExpense: moneyPtr(interest),
		PrincipalRepayment: moneyPtr(principal), TotalPayment: moneyPtr(payment),
		OpeningROUAsset: moneyPtr(openingROU), Depreciation: moneyPtr(depreciation),
		PeriodStartDate: start, PeriodEndDate: end,
	}
}

type memTrial struct {
	byPeriod map[string][]repository.TrialBalanceLine
	currency string
}

func (m memTrial) LatestTrialBalanceByPeriod(_ context.Context, _ string) (map[string][]repository.TrialBalanceLine, string, error) {
	return m.byPeriod, m.currency, nil
}

type memCapex map[string]float64

func (m memCapex) LatestForecastCapex(_ context.Context, _, period string) (*float64, error) {
	if value, ok := m[period]; ok {
		return &value, nil
	}
	return nil, nil
}

type memAssump map[string]string

func (m memAssump) Value(_ context.Context, _, key, _ string) (json.RawMessage, error) {
	if raw, ok := m[key]; ok {
		return json.RawMessage(raw), nil
	}
	return nil, nil
}

func TestLeaseReaderFoldsEntityMonth(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2029, 12, 31, 0, 0, 0, 0, time.UTC)
	rows := memMeasurements{
		"2026-07": {
			measurement("C1", 910, 10, 90, 100, 1150, 50, start, end),
			measurement("C2", 400, 5, 40, 45, 500, 20, start, end),
			// 当月新租约：Additions = 其期初 ROU 300
			measurement("C3", 0, 0, 0, 0, 300, 0, time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC), end),
			// 当月终止：Terminations = 其期末 ROU（此处 closing = opening − depreciation=80）
			{ContractID: "C4", OpeningLiability: moneyPtr(80), OpeningROUAsset: moneyPtr(100), Depreciation: moneyPtr(20),
				ClosingROUAsset: moneyPtr(80), PeriodStartDate: start, PeriodEndDate: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)},
		},
	}
	reader := NewLeaseReader(rows)
	out, err := reader.Monthly(context.Background(), "LE-1", "2026-07")
	if err != nil {
		t.Fatal(err)
	}
	if *out.ROUAsset != 1150+500+300+100 || *out.LeaseLiability != 910+400+0+80 {
		t.Fatalf("opening folds wrong: ROU=%v Liab=%v", *out.ROUAsset, *out.LeaseLiability)
	}
	if *out.Interest != 15 || *out.Depreciation != 50+20+0+20 || *out.Payments != 145 || *out.Principal != 130 {
		t.Fatalf("flow folds wrong: %+v", out)
	}
	if out.Additions == nil || *out.Additions != 300 {
		t.Fatalf("additions must come from in-month lease starts: %v", out.Additions)
	}
	if out.Terminations == nil || *out.Terminations != 80 {
		t.Fatalf("terminations must come from in-month lease ends: %v", out.Terminations)
	}
	if out.Remeasurements == nil || *out.Remeasurements != 0 {
		t.Fatalf("remeasurements must be explicit zero when rows exist: %v", out.Remeasurements)
	}
	// 无当月事件时，additions/terminations 是 0 而非缺失（事件可数）……
	noEvents := memMeasurements{"2026-09": {measurement("C1", 910, 10, 90, 100, 1150, 50, start, end)}}
	noEventOut, err := NewLeaseReader(noEvents).Monthly(context.Background(), "LE-1", "2026-09")
	if err != nil || noEventOut.Additions == nil || *noEventOut.Additions != 0 || noEventOut.Terminations == nil || *noEventOut.Terminations != 0 {
		t.Fatalf("countable zero events must be explicit zeros: %+v (%v)", noEventOut, err)
	}

	// 无租赁月份：全字段缺失，不编造 0。
	empty, err := reader.Monthly(context.Background(), "LE-1", "2026-08")
	if err != nil {
		t.Fatal(err)
	}
	if empty.ROUAsset != nil || empty.Interest != nil || empty.Remeasurements != nil {
		t.Fatalf("a month without leases must stay missing: %+v", empty)
	}
}

func TestScheduleReaderAdapterSources(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2029, 12, 31, 0, 0, 0, 0, time.UTC)
	rows := memMeasurements{"2026-07": {
		{ContractID: "C1", NonLeaseExpense: moneyPtr(15), PeriodStartDate: start, PeriodEndDate: end},
		{ContractID: "C2", NonLeaseExpense: moneyPtr(10), PeriodStartDate: start, PeriodEndDate: end},
	}}
	reader := NewScheduleReader(rows,
		memAssump{"share_capital": "500", "borrowings": "200", "other_depreciation": "20"},
		memCapex{"2026-07": 50})
	out, err := reader.Monthly(context.Background(), "LE-1", "2026-07")
	if err != nil {
		t.Fatal(err)
	}
	if *out.ServiceFee != 25 || *out.Capex != 50 || *out.ShareCapital != 500 || *out.Borrowings != 200 || *out.OtherDepreciation != 20 {
		t.Fatalf("schedule fanout wrong: %+v", out)
	}
	// 未登记的假设与无计划的 capex：保持缺失。
	sparse := NewScheduleReader(nil, memAssump{}, memCapex{})
	empty, err := sparse.Monthly(context.Background(), "LE-1", "2026-08")
	if err != nil || empty.ServiceFee != nil || empty.Capex != nil {
		t.Fatalf("unregistered sources must stay missing: %+v (%v)", empty, err)
	}
}

func TestOpeningReaderStandardBalanceAndMapping(t *testing.T) {
	lines := []repository.TrialBalanceLine{
		{AccountCode: "1001", Debit: 3400, Credit: 0},  // 现金
		{AccountCode: "1122", Debit: 500, Credit: 0},   // 应收
		{AccountCode: "1601", Debit: 1600, Credit: 0},  // PPE
		{AccountCode: "2202", Debit: 0, Credit: 1000},  // 应付
		{AccountCode: "2801", Debit: 0, Credit: 3500},  // 租赁负债
		{AccountCode: "4001", Debit: 0, Credit: 1000},  // 实收资本
		{AccountCode: "9999", Debit: 100, Credit: 100}, // 杂项 → 其他流动轧平
	}
	trial := memTrial{currency: "CNY", byPeriod: map[string][]repository.TrialBalanceLine{
		"2026-06": lines,
		"2026-07": lines, // 同一分录两期：归并稳定性可验
	}}
	reader := NewOpeningReader(trial, nil)
	balance, ref, engine, policy, err := reader.Get(context.Background(), "LE-1")
	if err != nil {
		t.Fatal(err)
	}
	if policy.Version != defaultMappingVersion {
		t.Fatalf("merge policy must declare its version: %+v", policy)
	}
	if balance.Currency != "CNY" || len(balance.Periods) != 2 {
		t.Fatalf("balance = %+v", balance)
	}
	first := balance.Periods[0]
	if first.Lines[opening.LineCash] != 3400 || first.Lines[opening.LineReceivables] != 500 ||
		first.Lines[opening.LinePPE] != 1600 || first.Lines[opening.LinePayables] != 1000 ||
		first.Lines[opening.LineLeaseLiability] != 3500 || first.Lines[opening.LineShareCapital] != 1000 {
		t.Fatalf("standardized lines wrong (credit-positive liability convention): %+v", first.Lines)
	}
	if first.Mapping["1001"] != opening.LineCash || first.Mapping["9999"] != opening.LineOtherCurrentAssets {
		t.Fatalf("mapping wrong: %+v", first.Mapping)
	}
	// 三闸自检：标准屏自身平衡（gate 1 恒过）。
	failures := opening.Validate(opening.ValidateInput{Balance: *balance, LeaseRef: ref, Engine: engine, Policy: policy})
	for _, failure := range failures {
		t.Fatalf("standard balance must pass the gates: %+v", failure)
	}
}
