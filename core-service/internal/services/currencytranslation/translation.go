package currencytranslation

import (
	"context"
	"fmt"
	"math"
	"time"
)

// ExchangeRate represents a currency conversion rate.
type ExchangeRate struct {
	FromCurrency string  `json:"from_currency"`
	ToCurrency   string  `json:"to_currency"`
	RateType     string  `json:"rate_type"` // closing, average, budget
	Rate         float64 `json:"rate"`
}

// RateVersion represents an approved exchange rate policy version.
type RateVersion struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	VersionType   string     `json:"version_type"` // closing, average, budget
	EffectiveFrom time.Time  `json:"effective_from"`
	EffectiveTo   *time.Time `json:"effective_to,omitempty"`
	Source        string     `json:"source"`
	Status        string     `json:"status"` // draft, approved, official
}

// RateVersionReader provides access to rate versions and rates.
type RateVersionReader interface {
	GetRateVersion(ctx context.Context, versionRef string) (*RateVersion, error)
	GetRates(ctx context.Context, versionID string, rateType string) ([]ExchangeRate, error)
}

// PlanLineItem is one un-translated plan line.
type PlanLineItem struct {
	Period         string  `json:"period"`
	Grain          string  `json:"grain"`
	StoreID        string  `json:"store_id,omitempty"`
	StoreName      string  `json:"store_name,omitempty"`
	Brand          string  `json:"brand,omitempty"`
	Region         string  `json:"region,omitempty"`
	OriginalCurr   string  `json:"original_currency"`
	Revenue        float64 `json:"revenue"`
	GrossProfit    float64 `json:"gross_profit"`
	LaborCost      float64 `json:"labor_cost"`
	FixedRent      float64 `json:"fixed_rent"`
	VariableRent   float64 `json:"variable_rent"`
	NonLeaseCost   float64 `json:"non_lease_cost"`
	FourWallEBITDA float64 `json:"four_wall_ebitda"`
	CashFlow       float64 `json:"cash_flow"`
	Capex          float64 `json:"capex"`
}

// MultiCurrencyPlanSet is the raw input of multi-currency plan lines.
type MultiCurrencyPlanSet struct {
	VersionID   string         `json:"version_id"`
	VersionName string         `json:"version_name"`
	Lines       []PlanLineItem `json:"lines"`
}

// TranslatedLine is a single translated plan line, preserving original values.
type TranslatedLine struct {
	PlanLineItem
	TargetCurrency   string  `json:"target_currency"`
	ExchangeRate     float64 `json:"exchange_rate"`
	TransRevenue     float64 `json:"translated_revenue"`
	TransGrossProfit float64 `json:"translated_gross_profit"`
	TransLaborCost   float64 `json:"translated_labor_cost"`
	TransFixedRent   float64 `json:"translated_fixed_rent"`
	TransVarRent     float64 `json:"translated_variable_rent"`
	TransNonLease    float64 `json:"translated_non_lease"`
	TransEBITDA      float64 `json:"translated_four_wall_ebitda"`
	TransCashFlow    float64 `json:"translated_cash_flow"`
	TransCapex       float64 `json:"translated_capex"`
}

// TranslatedPlanSet can ONLY be constructed through TranslationBasis.Translate.
// This enforces "making illegal states unrepresentable" (CodebaseDesign §8).
type TranslatedPlanSet struct {
	VersionID           string           `json:"version_id"`
	VersionName         string           `json:"version_name"`
	TargetCurrency      string           `json:"target_currency"`
	ExchangeRateVersion string           `json:"exchange_rate_version"`
	ExchangeRateType    string           `json:"exchange_rate_type"`
	Lines               []TranslatedLine `json:"lines"`
	TranslatedAt        time.Time        `json:"translated_at"`
}

// TranslatedSummary is the cross-currency aggregated total.
type TranslatedSummary struct {
	TargetCurrency      string  `json:"target_currency"`
	ExchangeRateVersion string  `json:"exchange_rate_version"`
	TotalRevenue        float64 `json:"total_revenue"`
	TotalGrossProfit    float64 `json:"total_gross_profit"`
	TotalLaborCost      float64 `json:"total_labor_cost"`
	TotalFixedRent      float64 `json:"total_fixed_rent"`
	TotalVariableRent   float64 `json:"total_variable_rent"`
	TotalNonLeaseCost   float64 `json:"total_non_lease_cost"`
	TotalFourWallEBITDA float64 `json:"total_four_wall_ebitda"`
	TotalCashFlow       float64 `json:"total_cash_flow"`
	TotalCapex          float64 `json:"total_capex"`
	LineCount           int     `json:"line_count"`
}

// TranslationBasis holds the loaded version and rate map.
type TranslationBasis struct {
	version  RateVersion
	ratesMap map[string]map[string]float64
}

// NewBasis is the sole constructor for TranslationBasis.
func NewBasis(ctx context.Context, versionRef string, reader RateVersionReader) (*TranslationBasis, error) {
	if versionRef == "" {
		return nil, fmt.Errorf("exchange rate versionRef is required")
	}
	if reader == nil {
		return nil, fmt.Errorf("rate version reader is required")
	}

	ver, err := reader.GetRateVersion(ctx, versionRef)
	if err != nil {
		return nil, fmt.Errorf("get rate version: %w", err)
	}
	if ver == nil {
		return nil, fmt.Errorf("rate version %q not found", versionRef)
	}

	rates, err := reader.GetRates(ctx, ver.ID, ver.VersionType)
	if err != nil {
		return nil, fmt.Errorf("get rates for version %q: %w", versionRef, err)
	}

	ratesMap := make(map[string]map[string]float64)
	for _, r := range rates {
		if ratesMap[r.FromCurrency] == nil {
			ratesMap[r.FromCurrency] = make(map[string]float64)
		}
		ratesMap[r.FromCurrency][r.ToCurrency] = r.Rate
		// Also support reciprocal rate if not explicitly set
		if ratesMap[r.ToCurrency] == nil {
			ratesMap[r.ToCurrency] = make(map[string]float64)
		}
		if _, exists := ratesMap[r.ToCurrency][r.FromCurrency]; !exists && r.Rate > 0 {
			ratesMap[r.ToCurrency][r.FromCurrency] = 1.0 / r.Rate
		}
	}

	return &TranslationBasis{
		version:  *ver,
		ratesMap: ratesMap,
	}, nil
}

// Translate converts multi-currency plan lines into a TranslatedPlanSet.
func (b *TranslationBasis) Translate(set MultiCurrencyPlanSet, targetCurrency string) (TranslatedPlanSet, error) {
	if targetCurrency == "" {
		return TranslatedPlanSet{}, fmt.Errorf("target_currency is required")
	}

	translatedLines := make([]TranslatedLine, 0, len(set.Lines))

	for _, line := range set.Lines {
		var rate float64 = 1.0
		if line.OriginalCurr != targetCurrency {
			fromMap, ok := b.ratesMap[line.OriginalCurr]
			if !ok || fromMap[targetCurrency] <= 0 {
				return TranslatedPlanSet{}, fmt.Errorf("missing exchange rate from %q to %q in rate version %q", line.OriginalCurr, targetCurrency, b.version.Name)
			}
			rate = fromMap[targetCurrency]
		}

		tLine := TranslatedLine{
			PlanLineItem:     line,
			TargetCurrency:   targetCurrency,
			ExchangeRate:     rate,
			TransRevenue:     round(line.Revenue * rate),
			TransGrossProfit: round(line.GrossProfit * rate),
			TransLaborCost:   round(line.LaborCost * rate),
			TransFixedRent:   round(line.FixedRent * rate),
			TransVarRent:     round(line.VariableRent * rate),
			TransNonLease:    round(line.NonLeaseCost * rate),
			TransEBITDA:      round(line.FourWallEBITDA * rate),
			TransCashFlow:    round(line.CashFlow * rate),
			TransCapex:       round(line.Capex * rate),
		}
		translatedLines = append(translatedLines, tLine)
	}

	return TranslatedPlanSet{
		VersionID:           set.VersionID,
		VersionName:         set.VersionName,
		TargetCurrency:      targetCurrency,
		ExchangeRateVersion: b.version.Name,
		ExchangeRateType:    b.version.VersionType,
		Lines:               translatedLines,
		TranslatedAt:        time.Now().UTC(),
	}, nil
}

// Total computes cross-currency totals exclusively on a valid TranslatedPlanSet.
func Total(set TranslatedPlanSet) TranslatedSummary {
	var summary TranslatedSummary
	summary.TargetCurrency = set.TargetCurrency
	summary.ExchangeRateVersion = set.ExchangeRateVersion
	summary.LineCount = len(set.Lines)

	for _, l := range set.Lines {
		summary.TotalRevenue += l.TransRevenue
		summary.TotalGrossProfit += l.TransGrossProfit
		summary.TotalLaborCost += l.TransLaborCost
		summary.TotalFixedRent += l.TransFixedRent
		summary.TotalVariableRent += l.TransVarRent
		summary.TotalNonLeaseCost += l.TransNonLease
		summary.TotalFourWallEBITDA += l.TransEBITDA
		summary.TotalCashFlow += l.TransCashFlow
		summary.TotalCapex += l.TransCapex
	}

	summary.TotalRevenue = round(summary.TotalRevenue)
	summary.TotalGrossProfit = round(summary.TotalGrossProfit)
	summary.TotalLaborCost = round(summary.TotalLaborCost)
	summary.TotalFixedRent = round(summary.TotalFixedRent)
	summary.TotalVariableRent = round(summary.TotalVariableRent)
	summary.TotalNonLeaseCost = round(summary.TotalNonLeaseCost)
	summary.TotalFourWallEBITDA = round(summary.TotalFourWallEBITDA)
	summary.TotalCashFlow = round(summary.TotalCashFlow)
	summary.TotalCapex = round(summary.TotalCapex)

	return summary
}

func round(v float64) float64 {
	return math.Round(v*100) / 100
}
