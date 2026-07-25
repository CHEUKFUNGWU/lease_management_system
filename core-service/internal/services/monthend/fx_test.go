package monthend

import (
	"math"
	"strings"
	"testing"
)

func usdInput() FXInput {
	// A USD lease held by a CNY entity. The liability fell from 100,000 to
	// 92,000 over the month after 500 of interest and 8,500 of payments.
	return FXInput{
		ContractCurrency:   "USD",
		FunctionalCurrency: "CNY",
		OpeningLiability:   100000,
		Interest:           500,
		Payments:           8500,
		ClosingLiability:   92000,
		OpeningRate:        7.10,
		ClosingRate:        7.20,
		AverageRate:        7.15,
	}
}

func TestRemeasureLiabilityIsolatesTheRateEffect(t *testing.T) {
	input := usdInput()
	difference, err := remeasureLiability(input)
	if err != nil {
		t.Fatalf("remeasure: %v", err)
	}

	// opening 100,000 x 7.10 = 710,000
	// flows (500 - 8,500) x 7.15 = -57,200
	// expected closing = 652,800; actual 92,000 x 7.20 = 662,400
	want := 662400.0 - 652800.0
	if math.Abs(difference-want) > 0.01 {
		t.Errorf("difference = %.2f, want %.2f", difference, want)
	}
	// The dollar weakened the yuan holder's position: the liability costs more.
	if difference <= 0 {
		t.Error("a rising rate on a liability must produce an exchange loss")
	}
}

func TestRemeasureLiabilityIsZeroWhenRatesDoNotMove(t *testing.T) {
	input := usdInput()
	input.OpeningRate, input.ClosingRate, input.AverageRate = 7.10, 7.10, 7.10

	difference, err := remeasureLiability(input)
	if err != nil {
		t.Fatalf("remeasure: %v", err)
	}
	if math.Abs(difference) > 0.01 {
		t.Errorf("difference = %.2f, want 0 when the rate is unchanged", difference)
	}
}

// A lease already in the functional currency has nothing to translate.
func TestRemeasureLiabilitySkipsFunctionalCurrencyLeases(t *testing.T) {
	input := usdInput()
	input.ContractCurrency = "CNY"

	difference, err := remeasureLiability(input)
	if err != nil {
		t.Fatalf("remeasure: %v", err)
	}
	if difference != 0 {
		t.Errorf("difference = %.2f, want 0", difference)
	}
}

// Rates are data. Without them the close must refuse rather than assume parity.
func TestRemeasureLiabilityRequiresRates(t *testing.T) {
	for name, mutate := range map[string]func(*FXInput){
		"missing opening": func(i *FXInput) { i.OpeningRate = 0 },
		"missing closing": func(i *FXInput) { i.ClosingRate = 0 },
		"missing average": func(i *FXInput) { i.AverageRate = 0 },
	} {
		input := usdInput()
		mutate(&input)
		if _, err := remeasureLiability(input); err == nil {
			t.Errorf("%s: expected an error rather than an assumed rate", name)
		}
	}
}

// A falling rate reduces the liability in functional terms: an exchange gain,
// which reverses the entry's direction.
func TestFXEntryDirectionFollowsTheSignOfTheDifference(t *testing.T) {
	lossDebit, lossCredit := fxEntryAccounts(1000)
	gainDebit, gainCredit := fxEntryAccounts(-1000)

	if lossDebit != gainCredit || lossCredit != gainDebit {
		t.Errorf("a gain must mirror a loss: loss %s/%s vs gain %s/%s",
			lossDebit, lossCredit, gainDebit, gainCredit)
	}
	if lossDebit != "6603-财务费用-汇兑损益" {
		t.Errorf("an exchange loss must debit the FX expense account, got %s", lossDebit)
	}
}

func TestFXEntryDescriptionRecordsTheRatesUsed(t *testing.T) {
	description := fxEntryDescription("2026-03", usdInput(), false)
	for _, want := range []string{"2026-03", "USD", "CNY", "7.10", "7.20", "7.15"} {
		if !strings.Contains(description, want) {
			t.Errorf("description %q must state %q", description, want)
		}
	}
	approximated := fxEntryDescription("2026-03", usdInput(), true)
	if !strings.Contains(approximated, "无平均汇率") {
		t.Errorf("an approximated translation must say so: %q", approximated)
	}
}
