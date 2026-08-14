package ifrs16

import (
	"fmt"

	"github.com/lease-management-system/core-service/internal/money"
	"github.com/shopspring/decimal"
)

// FXRemeasurementInput is one period's foreign-currency lease liability position,
// expressed in the contract's own currency, plus the rates needed to translate it.
type FXRemeasurementInput struct {
	ContractCurrency   string
	FunctionalCurrency string

	// Balances and flows in the contract currency.
	OpeningLiability money.Amount
	Interest         money.Amount
	Payments         money.Amount
	ClosingLiability money.Amount

	// OpeningRate is the closing rate of the prior period, at which the opening
	// liability already sits in the books. ClosingRate remeasures the period-end
	// balance; AverageRate translates the period's flows.
	OpeningRate float64
	ClosingRate float64
	AverageRate float64
}

// RemeasureForeignCurrencyLiability computes the period's exchange difference on
// a foreign-currency lease liability.
//
// IAS 21 treats the lease liability as a monetary item: it is retranslated at
// the closing rate every period end, and the resulting difference goes to profit
// or loss. The right-of-use asset is a non-monetary item carried at historical
// cost, so it is deliberately absent here — retranslating it would misstate both
// the asset and the P&L.
//
// The difference is what is left after the opening balance and the period's
// flows are translated at the rates that actually applied:
//
//	closing × closingRate − (opening × openingRate + (interest − payments) × averageRate)
//
// A positive result means the liability grew in functional-currency terms, which
// is an exchange loss for the lessee.
func RemeasureForeignCurrencyLiability(input FXRemeasurementInput) (money.Amount, error) {
	if input.ContractCurrency == input.FunctionalCurrency {
		return money.NewFromInt64(0), nil
	}
	if input.OpeningRate <= 0 || input.ClosingRate <= 0 || input.AverageRate <= 0 {
		return money.Amount{}, fmt.Errorf("exchange rates are required to remeasure a %s liability into %s",
			input.ContractCurrency, input.FunctionalCurrency)
	}

	openingFunctional := input.OpeningLiability.Decimal().Mul(decimal.NewFromFloat(input.OpeningRate))
	flowsFunctional := input.Interest.Sub(input.Payments).Decimal().Mul(decimal.NewFromFloat(input.AverageRate))
	expectedClosing := openingFunctional.Add(flowsFunctional)
	actualClosing := input.ClosingLiability.Decimal().Mul(decimal.NewFromFloat(input.ClosingRate))

	return money.New(actualClosing.Sub(expectedClosing)).Round("CNY"), nil
}
