package budget

import (
	"math"
	"testing"
)

func contract(id string, cost float64) ContractPeriod {
	return ContractPeriod{
		ContractID: id, ContractNumber: id, ContractName: id,
		Currency: "CNY", LeaseCost: cost,
	}
}

// The bridge is the deliverable: if its lines do not add up to the variance it
// explains nothing, so this is asserted on every shape of input.
func assertBridgeTies(t *testing.T, result Result) {
	t.Helper()
	var sum float64
	for _, line := range result.Bridge {
		sum += line.Amount
	}
	if math.Abs(sum-result.Variance) > 0.05 {
		t.Errorf("bridge sums to %.2f but variance is %.2f", sum, result.Variance)
	}
	if !result.BridgeTiesOut {
		t.Error("result must report that the bridge ties out")
	}
}

func TestExplainReportsOverspendAsPositiveVariance(t *testing.T) {
	result := Explain(Input{
		Period: "2026-03",
		Budget: []ContractPeriod{contract("c1", 1000)},
		Actual: []ContractPeriod{contract("c1", 1200)},
	})

	if result.Variance != 200 {
		t.Errorf("variance = %.2f, want 200 (actual over budget)", result.Variance)
	}
	assertBridgeTies(t, result)
}

func TestExplainAttributesNewAndEndedLeases(t *testing.T) {
	result := Explain(Input{
		Period: "2026-03",
		Budget: []ContractPeriod{contract("kept", 1000), contract("ended", 400)},
		Actual: []ContractPeriod{contract("kept", 1000), contract("new", 700)},
	})

	byCause := map[string]float64{}
	for _, line := range result.Bridge {
		byCause[line.Cause] = line.Amount
	}
	if byCause[CauseNewLease] != 700 {
		t.Errorf("new lease = %.2f, want 700", byCause[CauseNewLease])
	}
	if byCause[CauseEnded] != -400 {
		t.Errorf("ended lease = %.2f, want -400", byCause[CauseEnded])
	}
	if result.Variance != 300 {
		t.Errorf("variance = %.2f, want 300", result.Variance)
	}
	assertBridgeTies(t, result)
}

func TestExplainAttributesEventsToTheirCause(t *testing.T) {
	result := Explain(Input{
		Period: "2026-03",
		Budget: []ContractPeriod{contract("renewed", 1000), contract("repriced", 1000)},
		Actual: []ContractPeriod{contract("renewed", 1300), contract("repriced", 1100)},
		EventsByContract: map[string][]string{
			"renewed":  {"renewal"},
			"repriced": {"rent_change"},
		},
	})

	byCause := map[string]float64{}
	for _, line := range result.Bridge {
		byCause[line.Cause] = line.Amount
	}
	if byCause[CauseRenewal] != 300 {
		t.Errorf("renewal = %.2f, want 300", byCause[CauseRenewal])
	}
	if byCause[CauseRentChange] != 100 {
		t.Errorf("rent change = %.2f, want 100", byCause[CauseRentChange])
	}
	assertBridgeTies(t, result)
}

// An exchange movement is a rate effect, not a change in the lease, so it gets
// its own bridge line.
func TestExplainSeparatesExchangeRateEffect(t *testing.T) {
	result := Explain(Input{
		Period:       "2026-03",
		Budget:       []ContractPeriod{contract("usd", 1000)},
		Actual:       []ContractPeriod{contract("usd", 1150)},
		FXByContract: map[string]float64{"usd": 150},
	})

	if len(result.Bridge) != 1 || result.Bridge[0].Cause != CauseExchangeRate {
		t.Fatalf("bridge = %#v, want a single exchange-rate line", result.Bridge)
	}
	assertBridgeTies(t, result)
}

// A lease that both renewed and repriced must land in exactly one bridge line,
// otherwise the bridge would double-count it.
func TestExplainCountsAContractOnlyOnce(t *testing.T) {
	result := Explain(Input{
		Period: "2026-03",
		Budget: []ContractPeriod{contract("c1", 1000)},
		Actual: []ContractPeriod{contract("c1", 1500)},
		EventsByContract: map[string][]string{
			"c1": {"renewal", "rent_change"},
		},
		FXByContract: map[string]float64{"c1": 20},
	})

	total := 0
	for _, line := range result.Bridge {
		total += line.ContractCount
	}
	if total != 1 {
		t.Errorf("contract counted %d times across the bridge, want 1", total)
	}
	if result.Bridge[0].Cause != CauseRenewal {
		t.Errorf("cause = %s, want the highest-priority renewal", result.Bridge[0].Cause)
	}
	assertBridgeTies(t, result)
}

// Anything the events cannot explain must stay visible as a residual rather than
// being spread across the named causes.
func TestExplainKeepsUnexplainedVarianceAsResidual(t *testing.T) {
	result := Explain(Input{
		Period: "2026-03",
		Budget: []ContractPeriod{contract("c1", 1000)},
		Actual: []ContractPeriod{contract("c1", 1080)},
	})

	if len(result.Bridge) != 1 || result.Bridge[0].Cause != CauseOther {
		t.Fatalf("bridge = %#v, want the difference reported as a residual", result.Bridge)
	}
	if result.Bridge[0].Amount != 80 {
		t.Errorf("residual = %.2f, want 80", result.Bridge[0].Amount)
	}
	assertBridgeTies(t, result)
}

func TestExplainRanksLargestVariancesFirst(t *testing.T) {
	result := Explain(Input{
		Period: "2026-03",
		Budget: []ContractPeriod{contract("small", 100), contract("big", 100)},
		Actual: []ContractPeriod{contract("small", 110), contract("big", 900)},
	})

	if result.ByContract[0].ContractID != "big" {
		t.Errorf("first row = %s, want the largest variance first", result.ByContract[0].ContractID)
	}
}

func TestExplainHandlesAnEmptyPeriod(t *testing.T) {
	result := Explain(Input{Period: "2026-03"})
	if result.Variance != 0 || len(result.Bridge) != 0 {
		t.Errorf("empty period must produce no variance and no bridge: %#v", result)
	}
	if !result.BridgeTiesOut {
		t.Error("an empty bridge trivially ties out")
	}
}
