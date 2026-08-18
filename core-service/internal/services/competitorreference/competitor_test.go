package competitorreference

import (
	"testing"
)

func TestCompetitorReference_Summarize(t *testing.T) {
	p1 := 1.10
	p2 := 0.90
	obs := []Observation{
		{
			CompetitorName: "竞品超市A",
			PriceIndex:     &p1,
			PromoIntensity: "medium",
		},
		{
			CompetitorName: "竞品专卖店B",
			PriceIndex:     &p2,
			PromoIntensity: "aggressive",
		},
	}

	summary := SummarizeStoreCompetitors("S001", obs)

	if summary.CompetitorCount != 2 {
		t.Fatalf("expected 2 competitors, got %d", summary.CompetitorCount)
	}

	// Avg price index = (1.10 + 0.90) / 2 = 1.00
	if summary.AvgPriceIndex == nil || *summary.AvgPriceIndex != 1.00 {
		t.Fatalf("expected avg price 1.00, got %v", summary.AvgPriceIndex)
	}

	if summary.HighestPromoThreat != "aggressive" {
		t.Fatalf("expected threat aggressive, got %s", summary.HighestPromoThreat)
	}
}
