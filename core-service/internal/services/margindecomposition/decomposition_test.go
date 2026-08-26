package margindecomposition

import (
	"math"
	"testing"
)

func TestDecompose_StrictConservation(t *testing.T) {
	req := DecompositionRequest{
		Currency: "CNY",
		Base: []CategoryPeriodData{
			{CategoryCode: "BEV", CategoryName: "Beverages", Revenue: 100000, GrossProfit: 60000}, // 60% margin
			{CategoryCode: "SNK", CategoryName: "Snacks", Revenue: 100000, GrossProfit: 30000},    // 30% margin
		}, // Total base: Rev 200,000, GP 90,000, margin 45%
		Current: []CategoryPeriodData{
			{CategoryCode: "BEV", CategoryName: "Beverages", Revenue: 80000, GrossProfit: 44000}, // 55% margin (rate drop)
			{CategoryCode: "SNK", CategoryName: "Snacks", Revenue: 140000, GrossProfit: 42000},   // 30% margin (mix shift to snacks)
		}, // Total current: Rev 220,000, GP 86,000, margin ~39.09%
	}

	res := Decompose(req)

	if !res.IsConserved {
		t.Fatalf("expected strictly conserved decomposition, got residual=%.2f", res.RoundingResidual)
	}

	expectedGPVariance := -4000.0 // 86000 - 90000
	if res.TotalGPVariance != expectedGPVariance {
		t.Fatalf("expected GP variance %.2f, got %.2f", expectedGPVariance, res.TotalGPVariance)
	}

	// Volume effect: (220,000 - 200,000) * 0.45 = +9,000
	if math.Abs(res.VolumeEffect-9000.0) > 1.0 {
		t.Fatalf("expected volume effect ~9000, got %.2f", res.VolumeEffect)
	}

	// Mix effect: shift towards lower margin snacks should be negative
	if res.MixEffect >= 0 {
		t.Fatalf("expected negative mix effect due to snack mix growth, got %.2f", res.MixEffect)
	}

	// Rate effect: BEV margin dropped from 60% to 55%, so rate effect should be negative
	if res.RateEffect >= 0 {
		t.Fatalf("expected negative rate effect due to BEV margin cut, got %.2f", res.RateEffect)
	}

	// Sum of parts + residual = total variance
	sum := res.VolumeEffect + res.MixEffect + res.RateEffect + res.RoundingResidual
	if math.Abs(sum-res.TotalGPVariance) > 0.01 {
		t.Fatalf("sum of parts %.2f != total variance %.2f", sum, res.TotalGPVariance)
	}
}

func TestDecompose_ZeroBase(t *testing.T) {
	req := DecompositionRequest{
		Currency: "CNY",
		Base:     []CategoryPeriodData{},
		Current: []CategoryPeriodData{
			{CategoryCode: "BEV", CategoryName: "Beverages", Revenue: 50000, GrossProfit: 25000},
		},
	}

	res := Decompose(req)

	if !res.IsConserved {
		t.Fatalf("expected conserved with zero base, got residual=%.2f", res.RoundingResidual)
	}
	if res.TotalGPVariance != 25000.0 {
		t.Fatalf("expected variance 25000, got %.2f", res.TotalGPVariance)
	}
}
