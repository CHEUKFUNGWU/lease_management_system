package monthend

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lease-management-system/core-service/internal/money"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
)

func postedEntry() *repository.JournalEntry {
	description := "租赁利息费用 2026-01"
	batchID := "batch-1"
	return &repository.JournalEntry{
		ID:               "entry-original",
		ContractID:       "c1",
		AccountingPeriod: "2026-01",
		EntryDate:        time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
		EntryType:        "interest",
		DebitAccount:     "6601-租赁利息费用",
		CreditAccount:    "2801-租赁负债",
		Amount:           money.NewFromFloat(1234.56),
		Currency:         "HKD",
		Description:      &description,
		PostingStatus:    "posted",
		BatchID:          &batchID,
	}
}

func reversalService(entry *repository.JournalEntry, locked bool) (*Service, *fakeUOW) {
	uow := &fakeUOW{
		writer: &fakeWriter{existingEntry: entry, locked: locked},
		audit:  &fakeAudit{},
	}
	return &Service{uow: uow}, uow
}

func reverseCommand() ReversalCommand {
	return ReversalCommand{
		EntryID:       "entry-original",
		Reason:        "科目录错",
		LegalEntityID: "le1",
		Actor:         audit.Metadata{ChangedBy: "user-9"},
	}
}

func TestReverse_CreatesOppositeEntryAndFlagsOriginal(t *testing.T) {
	original := postedEntry()
	svc, uow := reversalService(original, false)

	result, err := svc.Reverse(context.Background(), reverseCommand())
	if err != nil {
		t.Fatalf("Reverse returned error: %v", err)
	}
	if !uow.committed {
		t.Fatal("expected the reversal transaction to commit")
	}
	if len(uow.writer.entries) != 1 {
		t.Fatalf("expected exactly one reversing entry, got %d", len(uow.writer.entries))
	}

	reversal := uow.writer.entries[0]
	// Debit and credit swap; everything else that determines the amount posted
	// stays identical, so the pair nets to zero.
	if reversal.DebitAccount != original.CreditAccount || reversal.CreditAccount != original.DebitAccount {
		t.Errorf("accounts not swapped: debit=%q credit=%q", reversal.DebitAccount, reversal.CreditAccount)
	}
	if !reversal.Amount.Equal(original.Amount) {
		t.Errorf("amount = %v, want %v", reversal.Amount, original.Amount)
	}
	if reversal.Currency != original.Currency {
		t.Errorf("currency = %q, want %q", reversal.Currency, original.Currency)
	}
	if reversal.EntryType != original.EntryType {
		t.Errorf("entry type = %q, want %q", reversal.EntryType, original.EntryType)
	}
	if reversal.AccountingPeriod != "2026-01" {
		t.Errorf("period = %q, want the original's 2026-01", reversal.AccountingPeriod)
	}
	if !reversal.EntryDate.Equal(time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("entry date = %v, want period end 2026-01-31", reversal.EntryDate)
	}

	// A reversal corrects posted accounting, so it is itself posted rather than
	// waiting in draft while the ledger stays wrong.
	if reversal.PostingStatus != "posted" {
		t.Errorf("reversal posting_status = %q, want posted", reversal.PostingStatus)
	}
	if reversal.ReversalOfEntryID == nil || *reversal.ReversalOfEntryID != original.ID {
		t.Error("reversing entry must link back to the original")
	}
	if reversal.ReversalReason == nil || *reversal.ReversalReason != "科目录错" {
		t.Error("reversal reason must be persisted on the reversing entry")
	}

	if uow.writer.reversedID != original.ID || uow.writer.reversedBy != "user-9" {
		t.Errorf("original not flagged reversed by actor: id=%q by=%q", uow.writer.reversedID, uow.writer.reversedBy)
	}
	if !uow.audit.logged {
		t.Error("expected the reversal to be audited inside the transaction")
	}
	if result.ReversalEntryID != reversal.ID || result.OriginalEntryID != original.ID {
		t.Errorf("result does not describe the pair: %+v", result)
	}
}

func TestReverse_RejectsEntriesThatWereNeverPosted(t *testing.T) {
	for _, status := range []string{"draft", "approved"} {
		entry := postedEntry()
		entry.PostingStatus = status
		svc, uow := reversalService(entry, false)

		_, err := svc.Reverse(context.Background(), reverseCommand())
		if !errors.Is(err, ErrEntryNotPosted) {
			t.Errorf("status %q: error = %v, want ErrEntryNotPosted", status, err)
		}
		if uow.committed {
			t.Errorf("status %q: nothing may be committed", status)
		}
		if len(uow.writer.entries) != 0 {
			t.Errorf("status %q: no reversing entry may be written", status)
		}
	}
}

func TestReverse_RejectsSecondReversal(t *testing.T) {
	entry := postedEntry()
	entry.PostingStatus = "reversed"
	svc, uow := reversalService(entry, false)

	_, err := svc.Reverse(context.Background(), reverseCommand())
	if !errors.Is(err, ErrEntryAlreadyReversed) {
		t.Fatalf("error = %v, want ErrEntryAlreadyReversed", err)
	}
	if uow.committed {
		t.Error("a repeat reversal must not commit")
	}
}

// A locked period is the whole point of the control: correcting an entry must
// never be able to write into it.
func TestReverse_RefusesLockedTargetPeriod(t *testing.T) {
	svc, uow := reversalService(postedEntry(), true)

	_, err := svc.Reverse(context.Background(), reverseCommand())
	if !errors.Is(err, ErrReversalPeriodLocked) {
		t.Fatalf("error = %v, want ErrReversalPeriodLocked", err)
	}
	if uow.committed {
		t.Error("no reversal may commit into a locked period")
	}
	if len(uow.writer.entries) != 0 {
		t.Error("no entry may be written into a locked period")
	}
}

// When the original period is closed, the correction goes to an open period
// instead of unlocking the closed one.
func TestReverse_PostsIntoRequestedOpenPeriod(t *testing.T) {
	svc, uow := reversalService(postedEntry(), false)

	cmd := reverseCommand()
	cmd.AccountingPeriod = "2026-03"
	if _, err := svc.Reverse(context.Background(), cmd); err != nil {
		t.Fatalf("Reverse returned error: %v", err)
	}

	reversal := uow.writer.entries[0]
	if reversal.AccountingPeriod != "2026-03" {
		t.Errorf("period = %q, want the requested 2026-03", reversal.AccountingPeriod)
	}
	if !reversal.EntryDate.Equal(time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("entry date = %v, want 2026-03-31", reversal.EntryDate)
	}
}

func TestReverse_RequiresReason(t *testing.T) {
	svc, uow := reversalService(postedEntry(), false)

	cmd := reverseCommand()
	cmd.Reason = "   "
	_, err := svc.Reverse(context.Background(), cmd)
	if !errors.Is(err, ErrReversalReasonRequired) {
		t.Fatalf("error = %v, want ErrReversalReasonRequired", err)
	}
	if uow.ran {
		t.Error("a missing reason must be rejected before opening a transaction")
	}
}

func TestReverse_ReportsMissingEntry(t *testing.T) {
	svc, uow := reversalService(nil, false)

	_, err := svc.Reverse(context.Background(), reverseCommand())
	if !errors.Is(err, ErrEntryNotFound) {
		t.Fatalf("error = %v, want ErrEntryNotFound", err)
	}
	if uow.committed {
		t.Error("nothing may commit when the entry does not exist")
	}
}

// A failure after the reversing entry is written must roll everything back,
// otherwise the ledger would carry a reversal whose original still reads posted.
func TestReverse_RollsBackWhenFlaggingOriginalFails(t *testing.T) {
	uow := &fakeUOW{
		writer: &fakeWriter{existingEntry: postedEntry(), failOn: "MarkJournalEntryReversed"},
		audit:  &fakeAudit{},
	}
	svc := &Service{uow: uow}

	if _, err := svc.Reverse(context.Background(), reverseCommand()); err == nil {
		t.Fatal("expected an error when flagging the original fails")
	}
	if uow.committed {
		t.Error("transaction must not commit when the original cannot be flagged")
	}
	if uow.audit.logged {
		t.Error("audit must not be written for a rolled-back reversal")
	}
}
