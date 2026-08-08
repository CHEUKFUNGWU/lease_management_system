package eventaccounting

import (
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/services/ifrs16"
)

func TestCalculateEarlyTerminationReturnsOneReusableAccountingPlan(t *testing.T) {
	newEndDate := "2025-07-01"
	result, err := Calculate(Input{
		EventID:          "12345678-event",
		ContractID:       "contract-1",
		EventType:        "early_termination",
		EffectiveDate:    date("2025-01-01"),
		CommencementDate: date("2024-01-01"),
		LeaseEndDate:     date("2026-01-01"),
		NewValue:         &newEndDate,
		Currency:         "CNY",
		LeaseScope:       ifrs16.LeaseScopeInScope,
		DiscountRate:     0.05,
		Payments: []ifrs16.LeasePayment{
			{Date: date("2025-06-30"), Amount: 100000, Timing: "postpaid", Type: "fixed"},
			{Date: date("2025-12-31"), Amount: 100000, Timing: "postpaid", Type: "fixed"},
		},
	})
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}

	if got, want := result.Treatment, "reassessment"; got != want {
		t.Fatalf("Treatment = %q, want %q", got, want)
	}
	if got, want := result.LeaseEndDate, date("2025-07-01"); !got.Equal(want) {
		t.Fatalf("LeaseEndDate = %v, want %v", got, want)
	}
	if result.Adjustment.ContractID != "contract-1" || result.Adjustment.EventID != "12345678-event" {
		t.Fatalf("adjustment identity = %#v", result.Adjustment)
	}
	if len(result.ForwardSchedule) == 0 {
		t.Fatal("ForwardSchedule is empty")
	}
	if len(result.JournalEntries) == 0 {
		t.Fatalf("JournalEntries is empty; adjustment = %#v", result.Adjustment)
	}
	entry := result.JournalEntries[0]
	if entry.EntryType != result.Treatment || entry.Currency != "CNY" {
		t.Fatalf("journal entry = %#v", entry)
	}
	liabilityDebit := 0.0
	rouCredit := 0.0
	gainCredit := 0.0
	for _, journal := range result.JournalEntries {
		if journal.DebitAccount == "2801-租赁负债" {
			liabilityDebit += journal.Amount
		}
		if journal.CreditAccount == "1701-使用权资产" {
			rouCredit += journal.Amount
		}
		if journal.CreditAccount == "6301-资产处置收益" {
			gainCredit += journal.Amount
		}
	}
	if !closeAmount(liabilityDebit, -result.Adjustment.LiabilityAdjustment) ||
		!closeAmount(rouCredit, -result.Adjustment.ROUAdjustment) ||
		!closeAmount(gainCredit, result.Adjustment.PnLGain) {
		t.Fatalf("unbalanced termination journals: %#v; adjustment: %#v", result.JournalEntries, result.Adjustment)
	}
}

func TestCalculateRentModificationUsesRevisedFuturePayments(t *testing.T) {
	newRent := "120000"
	result, err := Calculate(Input{
		EventID:          "rent-event",
		ContractID:       "contract-1",
		EventType:        "rent_change",
		EffectiveDate:    date("2025-01-01"),
		CommencementDate: date("2024-01-01"),
		LeaseEndDate:     date("2026-01-01"),
		NewValue:         &newRent,
		Currency:         "CNY",
		LeaseScope:       ifrs16.LeaseScopeInScope,
		DiscountRate:     0.05,
		Payments: []ifrs16.LeasePayment{
			{Date: date("2025-06-30"), Amount: 100000, Timing: "postpaid", Type: "fixed"},
			{Date: date("2025-12-31"), Amount: 100000, Timing: "postpaid", Type: "fixed"},
		},
	})
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}

	if got, want := result.Treatment, "modification"; got != want {
		t.Fatalf("Treatment = %q, want %q", got, want)
	}
	if got, want := result.Adjustment.RevisedDiscountRate, 0.05; got != want {
		t.Fatalf("RevisedDiscountRate = %v, want %v", got, want)
	}
	if result.Adjustment.LiabilityAdjustment <= 0 {
		t.Fatalf("LiabilityAdjustment = %v, want increase when future rent rises", result.Adjustment.LiabilityAdjustment)
	}
	if len(result.JournalEntries) == 0 || result.JournalEntries[0].DebitAccount != "1701-使用权资产" {
		t.Fatalf("JournalEntries = %#v, want ROU increase entry", result.JournalEntries)
	}
}

func TestCalculateDiscountRateChangeIsReassessmentAtRevisedRate(t *testing.T) {
	newRate := "8"
	result, err := Calculate(Input{
		EventID:          "rate-event",
		ContractID:       "contract-1",
		EventType:        "discount_rate_change",
		EffectiveDate:    date("2025-01-01"),
		CommencementDate: date("2024-01-01"),
		LeaseEndDate:     date("2026-01-01"),
		NewValue:         &newRate,
		Currency:         "CNY",
		LeaseScope:       ifrs16.LeaseScopeInScope,
		DiscountRate:     0.05,
		Payments: []ifrs16.LeasePayment{
			{Date: date("2025-06-30"), Amount: 100000, Timing: "postpaid", Type: "fixed"},
			{Date: date("2025-12-31"), Amount: 100000, Timing: "postpaid", Type: "fixed"},
		},
	})
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	if got, want := result.Treatment, "reassessment"; got != want {
		t.Fatalf("Treatment = %q, want %q", got, want)
	}
	if got, want := result.Adjustment.RevisedDiscountRate, 0.08; got != want {
		t.Fatalf("RevisedDiscountRate = %v, want %v", got, want)
	}
	if result.Adjustment.LiabilityAdjustment >= 0 {
		t.Fatalf("LiabilityAdjustment = %v, want decrease when rate rises", result.Adjustment.LiabilityAdjustment)
	}
}

func TestCalculateImpairmentReducesROUWithoutRemeasuringLiability(t *testing.T) {
	postImpairmentROU := "50000"
	result, err := Calculate(Input{
		EventID:          "impairment-event",
		ContractID:       "contract-1",
		EventType:        "impairment",
		EffectiveDate:    date("2025-01-01"),
		CommencementDate: date("2024-01-01"),
		LeaseEndDate:     date("2026-01-01"),
		NewValue:         &postImpairmentROU,
		Currency:         "CNY",
		LeaseScope:       ifrs16.LeaseScopeInScope,
		DiscountRate:     0.05,
		Payments: []ifrs16.LeasePayment{
			{Date: date("2025-06-30"), Amount: 100000, Timing: "postpaid", Type: "fixed"},
			{Date: date("2025-12-31"), Amount: 100000, Timing: "postpaid", Type: "fixed"},
		},
	})
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}

	if got, want := result.Treatment, "impairment"; got != want {
		t.Fatalf("Treatment = %q, want %q", got, want)
	}
	if got, want := result.Adjustment.ROUAfter, 50000.0; got != want {
		t.Fatalf("ROUAfter = %v, want %v", got, want)
	}
	if got := result.Adjustment.LiabilityAdjustment; got != 0 {
		t.Fatalf("LiabilityAdjustment = %v, want 0", got)
	}
	if len(result.JournalEntries) != 1 {
		t.Fatalf("JournalEntries = %#v, want one impairment entry", result.JournalEntries)
	}
	entry := result.JournalEntries[0]
	wantLoss := result.Adjustment.ROUBefore - result.Adjustment.ROUAfter
	if entry.Amount != wantLoss || entry.DebitAccount != "6701-资产减值损失" || entry.CreditAccount != "1702-使用权资产减值准备" {
		t.Fatalf("impairment journal = %#v, want amount %v", entry, wantLoss)
	}
	if len(result.ForwardSchedule) == 0 || result.ForwardSchedule[0].OpeningROUAsset != 50000 {
		t.Fatalf("forward schedule does not start from impaired ROU: %#v", result.ForwardSchedule)
	}
}

func date(value string) time.Time {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func closeAmount(got, want float64) bool {
	difference := got - want
	if difference < 0 {
		difference = -difference
	}
	return difference < 0.01
}
