package renttosales

import (
	"strings"
	"testing"
)

func revenue(value float64) *float64 { return &value }

func store(code string, rent float64, sales *float64) StoreInput {
	return StoreInput{
		StoreID: code, StoreCode: code, StoreName: code + " 店",
		CashRent: rent, RentCurrency: "CNY",
		Revenue: sales, RevenueCurrency: "CNY", RevenueSource: "manual",
	}
}

func policyInput(input Input) Input {
	if input.HealthyCeiling <= 0 {
		input.HealthyCeiling = 15
	}
	if input.WarningCeiling <= 0 {
		input.WarningCeiling = 20
	}
	return input
}

func find(t *testing.T, result Result, code string) StoreRatio {
	t.Helper()
	for _, row := range result.Stores {
		if row.StoreCode == code {
			return row
		}
	}
	t.Fatalf("store %s missing from the result", code)
	return StoreRatio{}
}

func TestCalculate_BandsTheRatioAgainstTheThresholds(t *testing.T) {
	result, err := Calculate(policyInput(Input{Period: "2026-06", Stores: []StoreInput{
		store("HEALTHY", 100000, revenue(1000000)), // 10%
		store("WATCH", 180000, revenue(1000000)),   // 18%
		store("OVER", 250000, revenue(1000000)),    // 25%
	}}))
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}

	if got := find(t, result, "HEALTHY"); got.Status != StatusHealthy || *got.RentToSales != 10 {
		t.Errorf("10%% should be healthy: %+v", got)
	}
	if got := find(t, result, "WATCH"); got.Status != StatusWatch {
		t.Errorf("18%% sits between the lines and should be watched: %+v", got)
	}
	if got := find(t, result, "OVER"); got.Status != StatusOverThreshold {
		t.Errorf("25%% is over the warning line: %+v", got)
	}
	if result.StoresOverLine != 1 {
		t.Errorf("expected one store over the line, got %d", result.StoresOverLine)
	}
}

// The three ways a ratio can be absent mean different things, and a report that
// shows them all as 0% or as blank would be making a claim it cannot support.
func TestCalculate_KeepsUnknownApartFromZeroAndFromMismatch(t *testing.T) {
	mismatch := store("FX", 100000, revenue(1000000))
	mismatch.RevenueCurrency = "USD"

	result, err := Calculate(policyInput(Input{Period: "2026-06", Stores: []StoreInput{
		store("UNREPORTED", 100000, nil),
		store("ZERO", 100000, revenue(0)),
		mismatch,
	}}))
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}

	unreported := find(t, result, "UNREPORTED")
	if unreported.Status != StatusNoRevenue || unreported.RentToSales != nil {
		t.Errorf("an unreported store has no ratio, not a zero one: %+v", unreported)
	}
	zero := find(t, result, "ZERO")
	if zero.Status != StatusZeroRevenue || zero.RentToSales != nil {
		t.Errorf("zero sales makes the ratio undefined, not infinite: %+v", zero)
	}
	clash := find(t, result, "FX")
	if clash.Status != StatusCurrencyClash || clash.RentToSales != nil {
		t.Errorf("rent and sales in different currencies do not divide: %+v", clash)
	}
	if result.StoresNoRevenue != 1 {
		t.Errorf("only the unreported store counts as missing revenue, got %d", result.StoresNoRevenue)
	}
}

// A portfolio ratio computed over the stores that happen to have data reads as
// though it covered them all.
func TestCalculate_WithholdsThePortfolioRatioWhenCoverageIsPartial(t *testing.T) {
	partial, err := Calculate(policyInput(Input{Period: "2026-06", Stores: []StoreInput{
		store("A", 100000, revenue(1000000)),
		store("B", 100000, nil),
	}}))
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if partial.PortfolioRatio != nil {
		t.Errorf("coverage is partial, so no portfolio ratio should be stated: %v", *partial.PortfolioRatio)
	}
	if partial.PortfolioCaveat == "" {
		t.Error("withholding the figure needs to be explained, not silent")
	}

	full, err := Calculate(policyInput(Input{Period: "2026-06", Stores: []StoreInput{
		store("A", 100000, revenue(1000000)),
		store("B", 300000, revenue(1000000)),
	}}))
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if full.PortfolioRatio == nil {
		t.Fatal("with every store reported the portfolio ratio can be stated")
	}
	// 400,000 of rent on 2,000,000 of sales.
	if *full.PortfolioRatio != 20 {
		t.Errorf("portfolio ratio = %.2f, want 20", *full.PortfolioRatio)
	}
}

func TestCalculate_SortsWorstFirstAndUnknownLast(t *testing.T) {
	result, err := Calculate(policyInput(Input{Period: "2026-06", Stores: []StoreInput{
		store("LOW", 50000, revenue(1000000)),
		store("UNKNOWN", 90000, nil),
		store("HIGH", 300000, revenue(1000000)),
	}}))
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	order := []string{result.Stores[0].StoreCode, result.Stores[1].StoreCode, result.Stores[2].StoreCode}
	want := []string{"HIGH", "LOW", "UNKNOWN"}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

func TestCalculate_SalesPerSqmOnlyWhenAreaIsKnown(t *testing.T) {
	area := 500.0
	withArea := store("WITH", 100000, revenue(1000000))
	withArea.AreaSqm = &area

	result, err := Calculate(policyInput(Input{Period: "2026-06", Stores: []StoreInput{
		withArea,
		store("WITHOUT", 100000, revenue(1000000)),
	}}))
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if got := find(t, result, "WITH"); got.SalesPerSqm == nil || *got.SalesPerSqm != 2000 {
		t.Errorf("1,000,000 over 500 sqm is 2,000: %+v", got.SalesPerSqm)
	}
	if got := find(t, result, "WITHOUT"); got.SalesPerSqm != nil {
		t.Errorf("no area means no sales density, got %v", *got.SalesPerSqm)
	}
}

func TestCalculate_ThresholdsAreOverridable(t *testing.T) {
	// A jewellery counter carries a much higher rent-to-sales than a
	// supermarket, so one fixed line cannot serve both.
	result, err := Calculate(Input{
		Period: "2026-06", HealthyCeiling: 25, WarningCeiling: 35,
		Stores: []StoreInput{store("JEWELLERY", 300000, revenue(1000000))}, // 30%
	})
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if got := find(t, result, "JEWELLERY"); got.Status != StatusWatch {
		t.Errorf("30%% is inside a 25/35 band and should only be watched: %+v", got)
	}
}

func TestCalculate_RejectsInputItCannotJudge(t *testing.T) {
	if _, err := Calculate(Input{Stores: []StoreInput{store("A", 1, revenue(1))}}); err == nil {
		t.Error("a ratio without a period is not attributable to anything")
	}
	if _, err := Calculate(Input{
		Period: "2026-06", HealthyCeiling: 30, WarningCeiling: 20,
		Stores: []StoreInput{store("A", 1, revenue(1))},
	}); err == nil {
		t.Error("a warning line below the healthy line cannot be applied")
	}
}

func TestCalculate_CoverageStatementNamesTheSourceOfTheNumbers(t *testing.T) {
	result, err := Calculate(policyInput(Input{Period: "2026-06", Stores: []StoreInput{
		store("A", 100000, revenue(1000000)),
		store("B", 100000, nil),
	}}))
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	// The system is a consumer of this data, never its owner, and the report
	// has to say so wherever the number is read.
	if result.CoverageStatement == "" {
		t.Fatal("the coverage statement is missing")
	}
	for _, phrase := range []string{"2 家门店", "1 家已报营收", "客户提供的营收口径"} {
		if !strings.Contains(result.CoverageStatement, phrase) {
			t.Errorf("coverage statement %q is missing %q", result.CoverageStatement, phrase)
		}
	}
}
