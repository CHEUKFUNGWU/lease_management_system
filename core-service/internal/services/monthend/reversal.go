package monthend

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lease-management-system/core-service/internal/repository"
	"github.com/lease-management-system/core-service/internal/services/audit"
)

// Reversal errors. Each maps to a distinct caller-visible outcome, so the HTTP
// layer can answer "why not" without re-deriving the rules.
var (
	// ErrEntryNotFound covers both a missing entry and one outside the caller's
	// data scope: the caller must not be able to tell those apart.
	ErrEntryNotFound = errors.New("journal entry not found")

	// ErrEntryNotPosted rejects reversing anything that has not been posted.
	// Draft and approved entries are corrected by regenerating or rejecting
	// them, which leaves no accounting effect to cancel.
	ErrEntryNotPosted = errors.New("only posted journal entries can be reversed")

	// ErrEntryAlreadyReversed enforces one reversal per entry.
	ErrEntryAlreadyReversed = errors.New("journal entry has already been reversed")

	// ErrReversalPeriodLocked is returned when the period the reversal would
	// land in is locked. Correcting a posted entry must never require unlocking
	// a period; the caller posts the reversal into an open period instead.
	ErrReversalPeriodLocked = errors.New("target accounting period is locked")

	// ErrReversalReasonRequired keeps the audit trail meaningful.
	ErrReversalReasonRequired = errors.New("reversal reason is required")
)

// ReversalCommand is the complete input to reversing one posted journal entry.
type ReversalCommand struct {
	EntryID string
	// AccountingPeriod is where the reversing entry lands. Empty means the
	// original entry's period, which is the common case when the error is caught
	// before that period is locked.
	AccountingPeriod string
	Reason           string
	LegalEntityID    string
	Actor            audit.Metadata
}

// ReversalResult describes the reversing entry that was created.
type ReversalResult struct {
	OriginalEntryID  string  `json:"original_entry_id"`
	ReversalEntryID  string  `json:"reversal_entry_id"`
	AccountingPeriod string  `json:"accounting_period"`
	EntryType        string  `json:"entry_type"`
	DebitAccount     string  `json:"debit_account"`
	CreditAccount    string  `json:"credit_account"`
	Amount           float64 `json:"amount"`
	Currency         string  `json:"currency"`
}

// Reverse cancels a posted journal entry by posting an opposite one and marking
// the original reversed, as a single transaction.
//
// The original entry is never edited or deleted, so a locked period stays
// byte-for-byte what was reported. When the original's period is already locked
// the caller supplies an open period for the correction instead of unlocking.
func (s *Service) Reverse(ctx context.Context, cmd ReversalCommand) (*ReversalResult, error) {
	reason := strings.TrimSpace(cmd.Reason)
	if reason == "" {
		return nil, ErrReversalReasonRequired
	}

	var result *ReversalResult
	err := s.uow.DoReversal(ctx, reversalLockKey(cmd.EntryID), func(store reversalStore, a auditSink) error {
		original, err := store.GetJournalEntryByID(ctx, cmd.EntryID)
		if err != nil {
			return err
		}
		if original == nil {
			return ErrEntryNotFound
		}
		if original.PostingStatus != "posted" {
			if original.PostingStatus == "reversed" {
				return ErrEntryAlreadyReversed
			}
			return fmt.Errorf("%w (current status: %s)", ErrEntryNotPosted, original.PostingStatus)
		}

		period := cmd.AccountingPeriod
		if period == "" {
			period = original.AccountingPeriod
		}
		entryDate, err := periodEndDate(period)
		if err != nil {
			return err
		}

		locked, err := store.IsPeriodLocked(ctx, period, cmd.LegalEntityID)
		if err != nil {
			return fmt.Errorf("failed to check period lock: %w", err)
		}
		if locked {
			return fmt.Errorf("%w: %s", ErrReversalPeriodLocked, period)
		}

		reversal := reversingEntry(original, period, entryDate, reason, cmd.Actor.ChangedBy)
		if err := store.CreateJournalEntry(ctx, reversal); err != nil {
			// The unique index on reversal_of_entry_id is the last line of
			// defense against two concurrent reversals of the same entry.
			if isUniqueViolation(err) {
				return ErrEntryAlreadyReversed
			}
			return err
		}
		if err := store.MarkJournalEntryReversed(ctx, original.ID, cmd.Actor.ChangedBy); err != nil {
			return err
		}

		if a != nil {
			if err := a.LogEvent(ctx, "journal_entries", original.ID, "reverse",
				map[string]interface{}{"posting_status": original.PostingStatus},
				map[string]interface{}{
					"posting_status":    "reversed",
					"reversal_entry_id": reversal.ID,
					"accounting_period": period,
					"reason":            reason,
					"amount":            original.Amount,
					"currency":          original.Currency,
				}, cmd.Actor); err != nil {
				return fmt.Errorf("failed to write audit log: %w", err)
			}
		}

		result = &ReversalResult{
			OriginalEntryID:  original.ID,
			ReversalEntryID:  reversal.ID,
			AccountingPeriod: period,
			EntryType:        reversal.EntryType,
			DebitAccount:     reversal.DebitAccount,
			CreditAccount:    reversal.CreditAccount,
			Amount:           reversal.Amount,
			Currency:         reversal.Currency,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// reversingEntry mirrors the original with its debit and credit accounts
// swapped. Amount, currency and entry type are preserved so the pair nets to
// zero and still classifies the same way in exports and reports.
//
// The reversal is created already posted: it corrects a posted entry, and
// leaving it as a draft would keep the ledger wrong until someone approved it.
func reversingEntry(original *repository.JournalEntry, period string, entryDate time.Time, reason, actorID string) *repository.JournalEntry {
	now := time.Now()
	description := fmt.Sprintf("红冲 %s(原分录 %s):%s", original.EntryType, original.ID, reason)

	var postedBy *string
	if actorID != "" {
		postedBy = &actorID
	}
	originalID := original.ID

	return &repository.JournalEntry{
		ContractID:          original.ContractID,
		MeasurementResultID: original.MeasurementResultID,
		AccountingPeriod:    period,
		EntryDate:           entryDate,
		EntryType:           original.EntryType,
		DebitAccount:        original.CreditAccount,
		CreditAccount:       original.DebitAccount,
		Amount:              original.Amount,
		Currency:            original.Currency,
		Description:         &description,
		PostingStatus:       "posted",
		PostedAt:            &now,
		PostedBy:            postedBy,
		BatchID:             original.BatchID,
		ReversalOfEntryID:   &originalID,
		ReversalReason:      &reason,
	}
}

// periodEndDate returns the last day of a "2006-01" accounting period, matching
// the entry date the close itself uses.
func periodEndDate(period string) (time.Time, error) {
	start, err := time.Parse("2006-01", period)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid accounting period %q: %w", period, err)
	}
	return start.AddDate(0, 1, -1), nil
}

func reversalLockKey(entryID string) string {
	return fmt.Sprintf("monthend:reversal:%s", entryID)
}

// isUniqueViolation reports whether err is a Postgres unique-constraint
// violation (SQLSTATE 23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
