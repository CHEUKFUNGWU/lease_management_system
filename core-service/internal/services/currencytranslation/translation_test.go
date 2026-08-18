package currencytranslation

import (
	"context"
	"testing"
	"time"
)

type fakeRateReader struct {
	versions map[string]*RateVersion
	rates    map[string][]ExchangeRate
}

func (f *fakeRateReader) GetRateVersion(ctx context.Context, versionRef string) (*RateVersion, error) {
	return f.versions[versionRef], nil
}

func (f *fakeRateReader) GetRates(ctx context.Context, versionID string, rateType string) ([]ExchangeRate, error) {
	return f.rates[versionID], nil
}

func TestCurrencyTranslationBasisAndTotal(t *testing.T) {
	reader := &fakeRateReader{
		versions: map[string]*RateVersion{
			"FY2026-Budget-Rates": {
				ID:            "v-1",
				Name:          "FY2026-Budget-Rates",
				VersionType:   "budget",
				EffectiveFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				Status:        "approved",
			},
		},
		rates: map[string][]ExchangeRate{
			"v-1": {
				{FromCurrency: "HKD", ToCurrency: "CNY", RateType: "budget", Rate: 0.92},
				{FromCurrency: "USD", ToCurrency: "CNY", RateType: "budget", Rate: 7.20},
			},
		},
	}

	// 1. Missing version ref errors
	if _, err := NewBasis(context.Background(), "", reader); err == nil {
		t.Error("expected error for empty versionRef, got nil")
	}

	// 2. Valid basis creation
	basis, err := NewBasis(context.Background(), "FY2026-Budget-Rates", reader)
	if err != nil {
		t.Fatalf("NewBasis failed: %v", err)
	}

	// 3. Multi-currency plan set
	rawSet := MultiCurrencyPlanSet{
		VersionID:   "plan-1",
		VersionName: "Q1 Forecast",
		Lines: []PlanLineItem{
			{Period: "2026-01", StoreID: "s1", StoreName: "Shanghai Store", OriginalCurr: "CNY", Revenue: 100000, GrossProfit: 40000, LaborCost: 15000, CashFlow: 20000, Capex: 5000},
			{Period: "2026-01", StoreID: "s2", StoreName: "Hong Kong Store", OriginalCurr: "HKD", Revenue: 100000, GrossProfit: 40000, LaborCost: 15000, CashFlow: 20000, Capex: 5000},
			{Period: "2026-01", StoreID: "s3", StoreName: "US Store", OriginalCurr: "USD", Revenue: 10000, GrossProfit: 4000, LaborCost: 1500, CashFlow: 2000, Capex: 500},
		},
	}

	// 4. Translate to CNY
	translated, err := basis.Translate(rawSet, "CNY")
	if err != nil {
		t.Fatalf("Translate failed: %v", err)
	}

	if translated.TargetCurrency != "CNY" || translated.ExchangeRateVersion != "FY2026-Budget-Rates" {
		t.Errorf("translated metadata mismatch: %+v", translated)
	}
	if len(translated.Lines) != 3 {
		t.Fatalf("expected 3 translated lines, got %d", len(translated.Lines))
	}

	// S1 (CNY -> CNY: rate 1.0): Rev = 100,000
	if translated.Lines[0].TransRevenue != 100000 {
		t.Errorf("s1 translated rev expected 100000, got %f", translated.Lines[0].TransRevenue)
	}
	// S2 (HKD -> CNY: rate 0.92): Rev = 92,000
	if translated.Lines[1].TransRevenue != 92000 {
		t.Errorf("s2 translated rev expected 92000, got %f", translated.Lines[1].TransRevenue)
	}
	// S3 (USD -> CNY: rate 7.20): Rev = 72,000
	if translated.Lines[2].TransRevenue != 72000 {
		t.Errorf("s3 translated rev expected 72000, got %f", translated.Lines[2].TransRevenue)
	}

	// 5. Total aggregation
	total := Total(translated)
	// Expected Total Revenue = 100,000 + 92,000 + 72,000 = 264,000
	if total.TotalRevenue != 264000 {
		t.Errorf("expected total revenue 264000, got %f", total.TotalRevenue)
	}
	// Expected Total Gross Profit = 40,000 + (40,000 * 0.92 = 36,800) + (4,000 * 7.2 = 28,800) = 105,600
	if total.TotalGrossProfit != 105600 {
		t.Errorf("expected total gross profit 105600, got %f", total.TotalGrossProfit)
	}
	// Expected Total Capex = 5,000 + (5,000 * 0.92 = 4,600) + (500 * 7.2 = 3,600) = 13,200
	if total.TotalCapex != 13200 {
		t.Errorf("expected total capex 13200, got %f", total.TotalCapex)
	}
	if total.LineCount != 3 {
		t.Errorf("expected line count 3, got %d", total.LineCount)
	}
}

func TestCurrencyTranslationMissingRateRejection(t *testing.T) {
	reader := &fakeRateReader{
		versions: map[string]*RateVersion{
			"v-test": {ID: "v-test", Name: "v-test", VersionType: "closing"},
		},
		rates: map[string][]ExchangeRate{
			"v-test": {
				{FromCurrency: "EUR", ToCurrency: "CNY", Rate: 7.8},
			},
		},
	}

	basis, err := NewBasis(context.Background(), "v-test", reader)
	if err != nil {
		t.Fatal(err)
	}

	// JPY to CNY is not in the rate map
	rawSet := MultiCurrencyPlanSet{
		Lines: []PlanLineItem{
			{OriginalCurr: "JPY", Revenue: 1000000},
		},
	}

	_, err = basis.Translate(rawSet, "CNY")
	if err == nil {
		t.Error("expected error for unmapped JPY currency, got nil")
	}
}
