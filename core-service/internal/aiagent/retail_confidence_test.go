package aiagent

import (
	"testing"

	"github.com/lease-management-system/core-service/internal/services/retailkpi"
	"github.com/lease-management-system/core-service/internal/services/retailpulse"
)

func ptrF(value float64) *float64 { return &value }

// P3-35: confidence moves with the signals — partial coverage and small
// samples push it down, a clean high-coverage read lands near the top, and
// an unready read stays at the refusal floor.
func TestRetailPulseConfidenceDerivesFromSignals(t *testing.T) {
	fullCoverage := ptrF(100.0)
	partialCoverage := ptrF(85.0)
	noRate := (*float64)(nil)
	base := func() *retailpulse.Response {
		return &retailpulse.Response{
			DecisionReady: true,
			Summary: map[string]retailpulse.SummaryMetric{
				"revenue": {Status: "complete", Current: retailkpi.KPIValue{Status: "complete"}, Comparison: retailkpi.KPIValue{Status: "complete"}},
			},
			CurrentCoverage: retailkpi.Coverage{CoverageRate: fullCoverage, ObservedStoreDays: 30},
		}
	}
	strong := base()
	strong.Attention = []retailpulse.Attention{{Score: 4}}
	if got := retailPulseConfidence(strong); got != 0.95 {
		t.Fatalf("strong read confidence=%v, want 0.95", got)
	}
	clean := base()
	if got := retailPulseConfidence(clean); got != 0.85 {
		t.Fatalf("clean read confidence=%v, want 0.85", got)
	}
	weak := base()
	weak.CurrentCoverage = retailkpi.Coverage{CoverageRate: partialCoverage, ObservedStoreDays: 10}
	// 85% coverage lands in the >=70 band (-0.15), small sample -0.05, no
	// rule hits -0.05 → 0.90 - 0.25 = 0.65.
	if got := retailPulseConfidence(weak); got != 0.65 {
		t.Fatalf("partial coverage + small sample confidence=%v, want 0.65", got)
	}
	unready := base()
	unready.DecisionReady = false
	if got := retailPulseConfidence(unready); got != 0.40 {
		t.Fatalf("unready confidence=%v, want 0.40", got)
	}
	unknownCoverage := base()
	unknownCoverage.CurrentCoverage = retailkpi.Coverage{CoverageRate: noRate, ObservedStoreDays: 30}
	if got := retailPulseConfidence(unknownCoverage); got != 0.70 {
		t.Fatalf("unknown coverage confidence=%v, want 0.70", got)
	}
}

func TestClampRetailConfidenceBounds(t *testing.T) {
	if clampRetailConfidence(5) != 0.95 || clampRetailConfidence(-5) != 0.35 {
		t.Fatal("clamp bounds broken")
	}
	if clampRetailConfidence(0.866) != 0.87 {
		t.Fatalf("rounding=%v", clampRetailConfidence(0.866))
	}
}
