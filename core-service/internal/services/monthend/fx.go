package monthend

import (
	"context"
	"fmt"
	"time"

	"github.com/lease-management-system/core-service/internal/money"
	"github.com/lease-management-system/core-service/internal/repository"
	ifrs16svc "github.com/lease-management-system/core-service/internal/services/ifrs16"
	"github.com/shopspring/decimal"
)

// FXInput aliases the engine's remeasurement input so the close reads in its own
// vocabulary while the IAS 21 arithmetic stays with the measurement engine.
type FXInput = ifrs16svc.FXRemeasurementInput

// remeasureLiability delegates to the measurement engine, where the rule is
// covered by the IFRS 16 regression suite.
func remeasureLiability(input FXInput) (money.Amount, error) {
	return ifrs16svc.RemeasureForeignCurrencyLiability(input)
}

// fxEntryAccounts returns the debit and credit accounts for an exchange
// difference. A positive difference means the liability grew in functional
// terms: an exchange loss.
func fxEntryAccounts(difference money.Amount) (debit, credit string) {
	if difference.Decimal().IsPositive() {
		return "6603-财务费用-汇兑损益", "2801-租赁负债"
	}
	return "2801-租赁负债", "6603-财务费用-汇兑损益"
}

// fxEntryDescription states the rates used, so the entry can be checked without
// re-deriving them.
func fxEntryDescription(period string, input FXInput, approximated bool) string {
	base := fmt.Sprintf("外币租赁负债期末重估 %s（%s→%s，期初 %.6f / 期末 %.6f / 平均 %.6f）",
		period, input.ContractCurrency, input.FunctionalCurrency,
		input.OpeningRate, input.ClosingRate, input.AverageRate)
	if approximated {
		return base + "；当期无平均汇率，暂用期末收盘价折算当期流量"
	}
	return base
}

// periodBounds returns the first and last day of a "2006-01" period.
func periodBounds(period string) (time.Time, time.Time, error) {
	start, err := time.Parse("2006-01", period)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid accounting period %q: %w", period, err)
	}
	return start, start.AddDate(0, 1, -1), nil
}

// buildFXEntry produces the exchange difference entry for a foreign-currency
// lease, or nil when the lease is already in the functional currency, is not
// capitalized, or the difference rounds to nothing.
//
// The entry is denominated in the functional currency — unlike the lease's own
// entries, which stay in the contract currency — because an exchange difference
// only exists in the functional currency.
func buildFXEntry(
	ctx context.Context,
	store closeStore,
	contract *repository.Contract,
	cmd Command,
	monthly *ifrs16svc.MonthlyEntry,
	measurementBasis string,
	periodEnd time.Time,
	batchID string,
) (*repository.JournalEntry, error) {
	// Only a capitalized lease carries a liability to remeasure.
	if measurementBasis != "capitalized" {
		return nil, nil
	}

	legalEntityID := cmd.LegalEntityID
	if legalEntityID == "" && contract.LegalEntityID != nil {
		legalEntityID = *contract.LegalEntityID
	}
	functional, err := store.FunctionalCurrency(ctx, legalEntityID)
	if err != nil {
		return nil, err
	}
	// Without a known functional currency there is nothing to translate into;
	// the lease's own entries already carry its currency.
	if functional == "" || contract.Currency == "" || functional == contract.Currency {
		return nil, nil
	}

	periodStart, _, err := periodBounds(cmd.AccountingPeriod)
	if err != nil {
		return nil, err
	}

	closingRate, err := store.GetRate(ctx, contract.Currency, functional, repository.RateTypeClosing, periodEnd)
	if err != nil {
		return nil, err
	}
	// The opening balance already sits in the books at the prior period's
	// closing rate.
	openingRate, err := store.GetRate(ctx, contract.Currency, functional,
		repository.RateTypeClosing, periodStart.AddDate(0, 0, -1))
	if err != nil {
		return nil, err
	}
	approximated := false
	averageRate, err := store.GetRate(ctx, contract.Currency, functional, repository.RateTypeAverage, periodEnd)
	if err != nil {
		// An average rate is a refinement, not a prerequisite: fall back to the
		// closing rate and say so on the entry rather than failing the close.
		averageRate = closingRate
		approximated = true
	}

	input := FXInput{
		ContractCurrency:   contract.Currency,
		FunctionalCurrency: functional,
		OpeningLiability:   monthly.OpeningLiability,
		Interest:           monthly.InterestExpense,
		Payments:           monthly.TotalPayments,
		ClosingLiability:   monthly.ClosingLiability,
		OpeningRate:        openingRate,
		ClosingRate:        closingRate,
		AverageRate:        averageRate,
	}
	difference, err := remeasureLiability(input)
	if err != nil {
		return nil, err
	}
	// The threshold is a policy float; the difference is exact money. An
	// exchange difference at or below it is left unbooked, as before.
	threshold := decimal.NewFromFloat(store.GetFloat64(ctx, "journal_entry_materiality_threshold", 0))
	if difference.Abs().Decimal().Cmp(threshold) <= 0 {
		return nil, nil
	}

	debit, credit := fxEntryAccounts(difference)
	description := fxEntryDescription(cmd.AccountingPeriod, input, approximated)
	return &repository.JournalEntry{
		ContractID:       contract.ID,
		AccountingPeriod: cmd.AccountingPeriod,
		EntryDate:        periodEnd,
		EntryType:        "fx_remeasurement",
		DebitAccount:     debit,
		CreditAccount:    credit,
		Amount:           difference.Abs(),
		Currency:         functional,
		Description:      &description,
		PostingStatus:    "draft",
		BatchID:          &batchID,
	}, nil
}
