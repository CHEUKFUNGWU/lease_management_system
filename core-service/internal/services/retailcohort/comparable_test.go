package retailcohort

import (
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/services/retailkpi"
)

func ptr(v float64) *float64 { return &v }

func TestCalculateLifecycleStatus(t *testing.T) {
	asOf := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	openFuture := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	openRamp := time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC)  // 8 months before asOf
	openMature := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) // 29 months before asOf
	closePast := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	if got := CalculateLifecycleStatus(nil, nil, asOf, 12); got != LifecycleUndecided {
		t.Fatalf("missing opening date must be undecided, got %v", got)
	}
	if got := CalculateLifecycleStatus(&openMature, &closePast, asOf, 12); got != LifecycleClosed {
		t.Fatalf("closed store must be closed, got %v", got)
	}
	if got := CalculateLifecycleStatus(&openFuture, nil, asOf, 12); got != LifecyclePreOpening {
		t.Fatalf("future store must be pre_opening, got %v", got)
	}
	if got := CalculateLifecycleStatus(&openRamp, nil, asOf, 12); got != LifecycleRampUp {
		t.Fatalf("8-month old store must be ramp_up, got %v", got)
	}
	if got := CalculateLifecycleStatus(&openMature, nil, asOf, 12); got != LifecycleMature {
		t.Fatalf("29-month old store must be mature, got %v", got)
	}
}

func TestEvaluateComparableCohort(t *testing.T) {
	window := PeriodPair{
		CurrentStart:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrentEnd:    time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
		BaselineStart: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		BaselineEnd:   time.Date(2025, 6, 30, 0, 0, 0, 0, time.UTC),
	}
	policy := DefaultPolicy() // 12 months ramp-up

	oldOpen := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)     // Mature before 2025-01-01 -> Included
	recentOpen := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)  // Ramp finishes 2025-06-01 (after baseline start 2025-01-01) -> Excluded (too_new)
	closedStore := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC) // Closed during current period -> Excluded (closed)

	stores := []StoreLifecycle{
		{StoreID: "s1", StoreCode: "S1", StoreName: "Store 1", OpeningDate: &oldOpen, IsActive: true},
		{StoreID: "s2", StoreCode: "S2", StoreName: "Store 2", OpeningDate: &recentOpen, IsActive: true},
		{StoreID: "s3", StoreCode: "S3", StoreName: "Store 3", OpeningDate: nil, IsActive: true},
		{StoreID: "s4", StoreCode: "S4", StoreName: "Store 4", OpeningDate: &oldOpen, ClosingDate: &closedStore, IsActive: true},
	}

	cohort := EvaluateComparableCohort(stores, window, policy)

	if cohort.TotalStores != 4 {
		t.Fatalf("expected 4 total stores, got %d", cohort.TotalStores)
	}
	if cohort.IncludedCount != 1 || len(cohort.Included) != 1 || cohort.Included[0].StoreID != "s1" {
		t.Fatalf("expected only s1 in Included, got %+v", cohort.Included)
	}

	if len(cohort.Undecidable) != 1 || cohort.Undecidable[0].StoreID != "s3" || cohort.Undecidable[0].Reason != ExclusionUndecidable {
		t.Fatalf("missing opening date must be in Undecidable, got %+v", cohort.Undecidable)
	}

	if len(cohort.Excluded) != 2 {
		t.Fatalf("expected 2 excluded stores, got %+v", cohort.Excluded)
	}

	reasons := map[string]ExclusionReason{}
	for _, e := range cohort.Excluded {
		reasons[e.StoreID] = e.Reason
	}
	if reasons["s2"] != ExclusionTooNew {
		t.Fatalf("s2 should be excluded with too_new, got %v", reasons["s2"])
	}
	if reasons["s4"] != ExclusionClosed {
		t.Fatalf("s4 should be excluded with closed, got %v", reasons["s4"])
	}
}

func TestCalculateSSSG(t *testing.T) {
	window := PeriodPair{
		CurrentStart:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		CurrentEnd:    time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		BaselineStart: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		BaselineEnd:   time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC),
	}
	matureDate := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)

	stores := []StoreLifecycle{
		{StoreID: "s1", StoreCode: "S1", StoreName: "Store 1", OpeningDate: &matureDate, IsActive: true},
		{StoreID: "s2", StoreCode: "S2", StoreName: "Store 2", OpeningDate: nil, IsActive: true}, // Undecidable -> filtered out
	}
	cohort := EvaluateComparableCohort(stores, window, DefaultPolicy())

	// S1: Baseline rev = 100 + 100 = 200; Current rev = 120 + 130 = 250; SSSG = (250-200)/200 * 100 = +25%
	// S2: Baseline rev = 500; Current rev = 500; but S2 is NOT comparable so ignored
	baseFacts := []retailkpi.DailyFact{
		{StoreID: "s1", StoreCode: "S1", Currency: "CNY", BusinessDate: window.BaselineStart, Revenue: ptr(100), GrossProfit: ptr(40), Transactions: ptr(10), Footfall: ptr(20), AreaSqm: ptr(50), LaborCost: ptr(10), FixedRent: ptr(5), VariableRent: ptr(2), NonLeaseCost: ptr(3), OtherControllableCost: ptr(4), MappingStatus: "mapped"},
		{StoreID: "s1", StoreCode: "S1", Currency: "CNY", BusinessDate: window.BaselineEnd, Revenue: ptr(100), GrossProfit: ptr(40), Transactions: ptr(10), Footfall: ptr(20), AreaSqm: ptr(50), LaborCost: ptr(10), FixedRent: ptr(5), VariableRent: ptr(2), NonLeaseCost: ptr(3), OtherControllableCost: ptr(4), MappingStatus: "mapped"},
		{StoreID: "s2", StoreCode: "S2", Currency: "CNY", BusinessDate: window.BaselineStart, Revenue: ptr(500), GrossProfit: ptr(200), Transactions: ptr(50), Footfall: ptr(100), AreaSqm: ptr(100), LaborCost: ptr(50), FixedRent: ptr(20), VariableRent: ptr(10), NonLeaseCost: ptr(5), OtherControllableCost: ptr(10), MappingStatus: "mapped"},
	}

	currFacts := []retailkpi.DailyFact{
		{StoreID: "s1", StoreCode: "S1", Currency: "CNY", BusinessDate: window.CurrentStart, Revenue: ptr(120), GrossProfit: ptr(50), Transactions: ptr(12), Footfall: ptr(24), AreaSqm: ptr(50), LaborCost: ptr(10), FixedRent: ptr(5), VariableRent: ptr(2), NonLeaseCost: ptr(3), OtherControllableCost: ptr(4), MappingStatus: "mapped"},
		{StoreID: "s1", StoreCode: "S1", Currency: "CNY", BusinessDate: window.CurrentEnd, Revenue: ptr(130), GrossProfit: ptr(55), Transactions: ptr(13), Footfall: ptr(26), AreaSqm: ptr(50), LaborCost: ptr(10), FixedRent: ptr(5), VariableRent: ptr(2), NonLeaseCost: ptr(3), OtherControllableCost: ptr(4), MappingStatus: "mapped"},
		{StoreID: "s2", StoreCode: "S2", Currency: "CNY", BusinessDate: window.CurrentStart, Revenue: ptr(500), GrossProfit: ptr(200), Transactions: ptr(50), Footfall: ptr(100), AreaSqm: ptr(100), LaborCost: ptr(50), FixedRent: ptr(20), VariableRent: ptr(10), NonLeaseCost: ptr(5), OtherControllableCost: ptr(10), MappingStatus: "mapped"},
	}

	currReq := retailkpi.Request{
		DateFrom:          window.CurrentStart,
		DateTo:            window.CurrentEnd,
		RequestedDateFrom: "2026-01-01",
		RequestedDateTo:   "2026-01-02",
	}
	baseReq := retailkpi.Request{
		DateFrom:          window.BaselineStart,
		DateTo:            window.BaselineEnd,
		RequestedDateFrom: "2025-01-01",
		RequestedDateTo:   "2025-01-02",
	}

	res := CalculateSSSG(currFacts, baseFacts, cohort, currReq, baseReq)

	if res.SSSG == nil || *res.SSSG != 25.0 {
		t.Fatalf("expected SSSG 25.0%%, got %+v", res.SSSG)
	}
	if res.CurrentRevenue == nil || *res.CurrentRevenue != 250.0 {
		t.Fatalf("expected CurrentRevenue 250.0, got %v", res.CurrentRevenue)
	}
	if res.BaselineRevenue == nil || *res.BaselineRevenue != 200.0 {
		t.Fatalf("expected BaselineRevenue 200.0, got %v", res.BaselineRevenue)
	}
	if !res.DecisionReady {
		t.Fatalf("expected DecisionReady true, got false (reason: %s)", res.Reason)
	}
}
