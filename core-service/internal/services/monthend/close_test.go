package monthend

import (
	"testing"
	"time"

	contractsvc "github.com/lease-management-system/core-service/internal/services/contracts"
	ifrs16svc "github.com/lease-management-system/core-service/internal/services/ifrs16"
)

func TestResolveDiscountRate(t *testing.T) {
	contractRate := 0.07
	tests := []struct {
		name         string
		override     float64
		globalRate   float64
		contractRate *float64
		want         float64
		wantErr      bool
	}{
		{"override wins", 0.03, 0.04, &contractRate, 0.03, false},
		{"contract before global", 0, 0.04, &contractRate, 0.07, false},
		{"contract when no override or global", 0, 0, &contractRate, 0.07, false},
		{"missing input fails", 0, 0, nil, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := contractsvc.ResolveDiscountRateValues(tc.override, tc.globalRate, tc.contractRate, "in_scope")
			if got != tc.want || (err != nil) != tc.wantErr {
				t.Fatalf("ResolveDiscountRateValues = (%v, %v), want (%v, error=%t)", got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestCloseStatus(t *testing.T) {
	tests := []struct {
		processed, failed int
		want              string
	}{
		{3, 0, "completed"},
		{2, 1, "completed_with_errors"},
		{0, 2, "failed"},
		{0, 0, "completed"},
	}
	for _, tc := range tests {
		if got := closeStatus(tc.processed, tc.failed); got != tc.want {
			t.Errorf("closeStatus(%d,%d) = %q, want %q", tc.processed, tc.failed, got, tc.want)
		}
	}
}

func TestBuildJournalEntries_Capitalized(t *testing.T) {
	monthly := &ifrs16svc.MonthlyEntry{
		InterestExpense:     100,
		Depreciation:        200,
		TotalPayments:       300,
		VariableRentExpense: 50,
		NonLeaseExpense:     40,
		ExemptLeaseExpense:  999, // must be ignored for capitalized basis
	}
	entryDate := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	entries := buildJournalEntries("c1", "HKD", "2026-01", entryDate, monthly, "batch1", "capitalized")

	byType := map[string]float64{}
	for _, e := range entries {
		byType[e.EntryType] = e.Amount
		if e.PostingStatus != "draft" {
			t.Errorf("entry %s posting_status = %q, want draft", e.EntryType, e.PostingStatus)
		}
		if e.BatchID == nil || *e.BatchID != "batch1" {
			t.Errorf("entry %s batch id not set to batch1", e.EntryType)
		}
		// Entries must carry the contract's own currency, never a hardcoded one.
		if e.Currency != "HKD" {
			t.Errorf("entry %s currency = %q, want HKD", e.EntryType, e.Currency)
		}
	}

	want := map[string]float64{"interest": 100, "depreciation": 200, "payment": 300, "variable_rent": 50, "non_lease": 40}
	if len(byType) != len(want) {
		t.Fatalf("got %d entries (%v), want %d", len(byType), byType, len(want))
	}
	for k, v := range want {
		if byType[k] != v {
			t.Errorf("entry %s amount = %v, want %v", k, byType[k], v)
		}
	}
	if _, ok := byType["lease_expense"]; ok {
		t.Error("capitalized basis should not produce a lease_expense entry")
	}
}

func TestBuildJournalEntries_StraightLine(t *testing.T) {
	monthly := &ifrs16svc.MonthlyEntry{
		InterestExpense:     500, // must be ignored for straight-line basis
		ExemptLeaseExpense:  120,
		VariableRentExpense: 30,
		NonLeaseExpense:     20,
	}
	entries := buildJournalEntries("c1", "HKD", "2026-01", time.Now(), monthly, "batch1", "straight_line_expense")

	types := map[string]bool{}
	for _, e := range entries {
		types[e.EntryType] = true
	}
	if types["interest"] || types["depreciation"] || types["payment"] {
		t.Error("straight-line basis must not produce capitalized entries")
	}
	for _, want := range []string{"lease_expense", "variable_rent", "non_lease"} {
		if !types[want] {
			t.Errorf("missing expected straight-line entry %q", want)
		}
	}
}

func TestBuildJournalEntries_OmitsTinyAmounts(t *testing.T) {
	monthly := &ifrs16svc.MonthlyEntry{
		InterestExpense: 0.005, // below the 0.01 threshold
		Depreciation:    200,
	}
	entries := buildJournalEntries("c1", "HKD", "2026-01", time.Now(), monthly, "batch1", "capitalized")
	for _, e := range entries {
		if e.EntryType == "interest" {
			t.Error("sub-threshold interest amount should not produce an entry")
		}
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1 (depreciation only)", len(entries))
	}
}

func TestMeasurementResult_Mapping(t *testing.T) {
	monthly := &ifrs16svc.MonthlyEntry{
		OpeningLiability: 1000,
		InterestExpense:  100,
		TotalPayments:    300,
		ClosingLiability: 800,
		OpeningROUAsset:  900,
		Depreciation:     150,
		ClosingROUAsset:  750,
	}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)
	now := time.Now()

	mr := measurementResult("c1", "2026-01", start, end, monthly, 0.05, "batch1", now)

	if mr.ContractID != "c1" || mr.AccountingPeriod != "2026-01" {
		t.Errorf("identity mismatch: %+v", mr)
	}
	if mr.OpeningLiability != 1000 || mr.ClosingLiability != 800 || mr.Depreciation != 150 {
		t.Errorf("amount mapping mismatch: %+v", mr)
	}
	if mr.DiscountRate != 0.05 {
		t.Errorf("discount rate = %v, want 0.05", mr.DiscountRate)
	}
	if !mr.IsCalculated {
		t.Error("IsCalculated should be true")
	}
	if mr.CalculationBatchID == nil || *mr.CalculationBatchID != "batch1" {
		t.Error("CalculationBatchID not set to batch1")
	}
	if mr.CalculatedAt == nil || !mr.CalculatedAt.Equal(now) {
		t.Error("CalculatedAt not set")
	}
}
