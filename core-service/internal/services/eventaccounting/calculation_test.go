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
}

func date(value string) time.Time {
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		panic(err)
	}
	return parsed
}
