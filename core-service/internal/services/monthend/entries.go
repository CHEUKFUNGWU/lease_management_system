package monthend

import (
	"fmt"
	"time"

	"github.com/lease-management-system/core-service/internal/money"
	"github.com/lease-management-system/core-service/internal/repository"
	ifrs16svc "github.com/lease-management-system/core-service/internal/services/ifrs16"
	"github.com/shopspring/decimal"
)

// measurementResult maps a monthly IFRS 16 entry to the persistable measurement
// record for a contract and period.
func measurementResult(contractID, period string, periodStart, periodEnd time.Time, monthly *ifrs16svc.MonthlyEntry, discountRate float64, batchID string, now time.Time) *repository.MeasurementResult {
	bid := batchID
	return &repository.MeasurementResult{
		ContractID:       contractID,
		AccountingPeriod: period,
		PeriodStartDate:  periodStart,
		PeriodEndDate:    periodEnd,
		OpeningLiability: monthly.OpeningLiability,
		InterestExpense:  monthly.InterestExpense,
		TotalPayment:     monthly.TotalPayments,
		// PrincipalRepayment stays zero exactly as the pre-migration code
		// persisted (an unset money.Amount would now fail to save instead of
		// silently writing 0). Computing TotalPayment − InterestExpense would
		// change stored data and belongs to a behavior ticket, not the
		// representation migration.
		PrincipalRepayment:  money.NewFromInt64(0),
		ClosingLiability:    monthly.ClosingLiability,
		OpeningROUAsset:     monthly.OpeningROUAsset,
		Depreciation:        monthly.Depreciation,
		ClosingROUAsset:     monthly.ClosingROUAsset,
		VariableRentExpense: monthly.VariableRentExpense,
		NonLeaseExpense:     monthly.NonLeaseExpense,
		DiscountRate:        discountRate,
		IsCalculated:        true,
		CalculationBatchID:  &bid,
		CalculatedAt:        &now,
	}
}

// buildJournalEntries produces the draft journal entries for one contract-period
// from its monthly IFRS 16 entry. Capitalized leases yield interest,
// depreciation, payment, variable-rent and non-lease entries; exempt
// (straight-line) leases yield expense, variable-rent and non-lease entries.
// Only amounts above the configured materiality threshold produce an entry.
//
// Entries are denominated in the contract's own currency. This subledger does
// not translate foreign-currency leases: translation and any resulting exchange
// difference belong to the general ledger.
func buildJournalEntries(contractID, currency, period string, entryDate time.Time, monthly *ifrs16svc.MonthlyEntry, batchID, measurementBasis string, materialityThreshold float64) []*repository.JournalEntry {
	var entries []*repository.JournalEntry

	add := func(entryType, debit, credit string, amount money.Amount, desc string) {
		// The materiality threshold is a policy float; amounts are exact
		// money. Only amounts above the threshold produce an entry.
		if amount.Decimal().Cmp(decimal.NewFromFloat(materialityThreshold)) <= 0 {
			return
		}
		entries = append(entries, &repository.JournalEntry{
			ContractID:       contractID,
			AccountingPeriod: period,
			EntryDate:        entryDate,
			EntryType:        entryType,
			DebitAccount:     debit,
			CreditAccount:    credit,
			Amount:           amount,
			Currency:         currency,
			Description:      strPtr(desc),
			PostingStatus:    "draft",
			BatchID:          &batchID,
		})
	}

	if measurementBasis == "straight_line_expense" {
		add("lease_expense", "6603-租赁费用-豁免租赁", "2202-应付账款", monthly.ExemptLeaseExpense, fmt.Sprintf("短期/低价值租赁直线法费用 %s", period))
		add("variable_rent", "6603-租赁费用-变量租金", "2202-应付账款", monthly.VariableRentExpense, fmt.Sprintf("变量租金 %s", period))
		add("non_lease", "6604-租赁费用-非租赁成分", "2202-应付账款", monthly.NonLeaseExpense, fmt.Sprintf("非租赁成分 %s", period))
		return entries
	}

	add("interest", "6601-租赁利息费用", "2801-租赁负债", monthly.InterestExpense, fmt.Sprintf("租赁利息费用 %s", period))
	add("depreciation", "6602-使用权资产折旧", "1701-使用权资产累计折旧", monthly.Depreciation, fmt.Sprintf("使用权资产折旧 %s", period))
	add("payment", "2801-租赁负债", "1002-银行存款", monthly.TotalPayments, fmt.Sprintf("租赁付款 %s", period))
	add("variable_rent", "6603-租赁费用-变量租金", "2202-应付账款", monthly.VariableRentExpense, fmt.Sprintf("变量租金 %s", period))
	add("non_lease", "6604-租赁费用-非租赁成分", "2202-应付账款", monthly.NonLeaseExpense, fmt.Sprintf("非租赁成分 %s", period))

	return entries
}

func strPtr(s string) *string { return &s }
