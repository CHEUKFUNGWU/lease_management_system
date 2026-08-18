package categoryreconciliation

import (
	"testing"
)

func TestReconcile_AllTie(t *testing.T) {
	summaries := []DailySummaryFact{
		{StoreID: "S01", BusinessDate: "2026-01-01", Currency: "CNY", Revenue: 1000.0, GrossProfit: 400.0},
		{StoreID: "S01", BusinessDate: "2026-01-02", Currency: "CNY", Revenue: 2000.0, GrossProfit: 800.0},
	}

	details := []CategoryFact{
		{StoreID: "S01", BusinessDate: "2026-01-01", Currency: "CNY", CategoryCode: "CAT_A", Revenue: 600.0, GrossProfit: 240.0},
		{StoreID: "S01", BusinessDate: "2026-01-01", Currency: "CNY", CategoryCode: "CAT_B", Revenue: 400.0, GrossProfit: 160.0},
		{StoreID: "S01", BusinessDate: "2026-01-02", Currency: "CNY", CategoryCode: "CAT_A", Revenue: 1500.0, GrossProfit: 600.0},
		{StoreID: "S01", BusinessDate: "2026-01-02", Currency: "CNY", CategoryCode: "CAT_B", Revenue: 500.0, GrossProfit: 200.0},
	}

	res := Reconcile(details, summaries, DefaultTolerance())

	if res.OverallStatus != StatusTie {
		t.Fatalf("expected overall tie, got %s", res.OverallStatus)
	}
	if res.TieCount != 2 || res.MismatchCount != 0 {
		t.Fatalf("expected 2 ties 0 mismatches, got ties=%d mismatches=%d", res.TieCount, res.MismatchCount)
	}
	if len(res.Mismatches) != 0 {
		t.Fatalf("expected 0 mismatches in list, got %d", len(res.Mismatches))
	}
}

func TestReconcile_WithinTolerance(t *testing.T) {
	summaries := []DailySummaryFact{
		{StoreID: "S01", BusinessDate: "2026-01-01", Currency: "CNY", Revenue: 1000.0, GrossProfit: 400.0},
	}

	// 0.5 difference is within 1.0 absolute tolerance
	details := []CategoryFact{
		{StoreID: "S01", BusinessDate: "2026-01-01", Currency: "CNY", CategoryCode: "CAT_A", Revenue: 600.5, GrossProfit: 400.3},
		{StoreID: "S01", BusinessDate: "2026-01-01", Currency: "CNY", CategoryCode: "CAT_B", Revenue: 400.0, GrossProfit: 0.0},
	}

	res := Reconcile(details, summaries, DefaultTolerance())

	if res.OverallStatus != StatusWithinTolerance {
		t.Fatalf("expected within_tolerance, got %s", res.OverallStatus)
	}
	if res.WithinToleranceCount != 1 {
		t.Fatalf("expected 1 within_tolerance, got %d", res.WithinToleranceCount)
	}
}

func TestReconcile_MismatchNeverAutoBalanced(t *testing.T) {
	summaries := []DailySummaryFact{
		{StoreID: "S01", BusinessDate: "2026-01-01", Currency: "CNY", Revenue: 1000.0, GrossProfit: 400.0},
	}

	// 200.0 mismatch
	details := []CategoryFact{
		{StoreID: "S01", BusinessDate: "2026-01-01", Currency: "CNY", CategoryCode: "CAT_A", Revenue: 500.0, GrossProfit: 200.0},
		{StoreID: "S01", BusinessDate: "2026-01-01", Currency: "CNY", CategoryCode: "CAT_B", Revenue: 300.0, GrossProfit: 100.0},
	}

	res := Reconcile(details, summaries, DefaultTolerance())

	if res.OverallStatus != StatusMismatch {
		t.Fatalf("expected mismatch, got %s", res.OverallStatus)
	}
	if res.MismatchCount != 1 {
		t.Fatalf("expected 1 mismatch, got %d", res.MismatchCount)
	}
	if len(res.Mismatches) != 1 {
		t.Fatalf("expected 1 mismatch in list, got %d", len(res.Mismatches))
	}
	if res.Mismatches[0].RevenueDiff != -200.0 {
		t.Fatalf("expected -200.0 diff, got %.2f", res.Mismatches[0].RevenueDiff)
	}
	// Assert detail and summary revenue are faithfully preserved and not force-balanced
	if res.StoreDayResults[0].SummaryRevenue != 1000.0 || res.StoreDayResults[0].DetailRevenue != 800.0 {
		t.Fatalf("expected preserved summary and detail values, got summary=%.2f detail=%.2f",
			res.StoreDayResults[0].SummaryRevenue, res.StoreDayResults[0].DetailRevenue)
	}
}

func TestReconcile_NoDetail(t *testing.T) {
	summaries := []DailySummaryFact{
		{StoreID: "S01", BusinessDate: "2026-01-01", Currency: "CNY", Revenue: 1000.0, GrossProfit: 400.0},
	}
	details := []CategoryFact{}

	res := Reconcile(details, summaries, DefaultTolerance())

	if res.OverallStatus != StatusNoDetail {
		t.Fatalf("expected no_detail, got %s", res.OverallStatus)
	}
	if res.NoDetailCount != 1 {
		t.Fatalf("expected 1 no_detail count, got %d", res.NoDetailCount)
	}
}
