package inventorymetrics

import (
	"testing"
)

func TestInventoryMetrics_Calculations(t *testing.T) {
	// Ending stock = 200,000, 30-day period, COGS = 300,000
	doi := CalculateDOI(200000, 300000, 30)
	if doi == nil || *doi != 20.0 {
		t.Fatalf("expected DOI 20.0, got %v", doi)
	}

	turnover := CalculateTurnover(300000, 200000)
	if turnover == nil || *turnover != 1.5 {
		t.Fatalf("expected turnover 1.5, got %v", turnover)
	}

	sc, tc, total := CalculateCarryingCost(200000, 50000, 0.10, 365)
	if sc != 20000.0 || tc != 5000.0 || total != 25000.0 {
		t.Fatalf("expected 20k/5k/25k carrying costs, got sc=%.2f tc=%.2f total=%.2f", sc, tc, total)
	}
}

func TestInventoryMetrics_ZeroSafe(t *testing.T) {
	doi := CalculateDOI(200000, 0, 30)
	if doi != nil {
		t.Fatalf("expected nil DOI on zero COGS, got %v", doi)
	}

	turnover := CalculateTurnover(300000, 0)
	if turnover != nil {
		t.Fatalf("expected nil turnover on zero stock, got %v", turnover)
	}
}
