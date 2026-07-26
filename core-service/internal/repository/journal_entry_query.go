package repository

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// This file answers one question the close itself cannot: "what does period
// 2026-03 hold?". Until now entries could only be seen as the by-product of
// running a close, which meant reopening a finished period just to look at it.
// The queries here read the ledger directly, so a period can be inspected,
// reviewed and reconciled long after it was closed.

// JournalEntryQuery filters the ledger by period first, then by anything that
// narrows it further. A zero Page or PageSize is filled in with a default.
type JournalEntryQuery struct {
	LegalEntityID string
	Period        string
	ContractID    string
	Status        string
	EntryType     string
	Page          int
	PageSize      int
}

// JournalEntrySummary describes the whole filtered set rather than the page
// that was returned, so a reader on page 3 still sees what the period holds.
type JournalEntrySummary struct {
	Total         int     `json:"total"`
	DraftCount    int     `json:"draft_count"`
	ApprovedCount int     `json:"approved_count"`
	PostedCount   int     `json:"posted_count"`
	ReversedCount int     `json:"reversed_count"`
	TotalAmount   float64 `json:"total_amount"`
	ContractCount int     `json:"contract_count"`
}

// JournalEntryPeriod is one accounting period that actually carries entries.
// It exists so a period can be chosen from what the ledger contains instead of
// being typed from memory.
type JournalEntryPeriod struct {
	AccountingPeriod string    `json:"accounting_period"`
	EntryCount       int       `json:"entry_count"`
	ContractCount    int       `json:"contract_count"`
	DraftCount       int       `json:"draft_count"`
	PostedCount      int       `json:"posted_count"`
	TotalAmount      float64   `json:"total_amount"`
	IsLocked         bool      `json:"is_locked"`
	LastEntryAt      time.Time `json:"last_entry_at"`
}

const journalEntryColumns = `
	je.id, je.contract_id, je.measurement_result_id, je.accounting_period, je.entry_date,
	je.entry_type, je.debit_account, je.credit_account, je.amount, je.currency,
	je.description, je.voucher_number, je.posting_status, je.posted_at, je.posted_by,
	je.approved_by, je.approved_at, je.erp_reference, je.batch_id,
	je.reversal_of_entry_id, je.reversal_reason, je.reversed_at, je.reversed_by,
	je.created_at, je.updated_at
`

const (
	defaultEntryPageSize = 50
	maxEntryPageSize     = 500
)

// journalEntryFilter builds the predicate shared by the page query and the
// summary. Both must see exactly the same rows, so the clause is written once.
func journalEntryFilter(ctx context.Context, q JournalEntryQuery) (string, []interface{}, int) {
	where := "WHERE 1=1"
	args := make([]interface{}, 0, 5)
	argIdx := 1

	// journal_entries carries no legal entity of its own, so tenancy is read
	// through the contract the entry belongs to.
	if q.LegalEntityID != "" {
		where += fmt.Sprintf(" AND lc.legal_entity_id::text = $%d", argIdx)
		args = append(args, q.LegalEntityID)
		argIdx++
	}
	if q.Period != "" {
		where += fmt.Sprintf(" AND je.accounting_period = $%d", argIdx)
		args = append(args, q.Period)
		argIdx++
	}
	if q.ContractID != "" {
		where += fmt.Sprintf(" AND je.contract_id::text = $%d", argIdx)
		args = append(args, q.ContractID)
		argIdx++
	}
	if q.Status != "" {
		where += fmt.Sprintf(" AND je.posting_status = $%d", argIdx)
		args = append(args, q.Status)
		argIdx++
	}
	if q.EntryType != "" {
		where += fmt.Sprintf(" AND je.entry_type = $%d", argIdx)
		args = append(args, q.EntryType)
		argIdx++
	}
	return appendContractScopePredicate(ctx, where, args, argIdx, "lc")
}

// ListJournalEntries returns one page of entries together with a summary of the
// whole filtered set.
func (r *MonthlyClosingRepository) ListJournalEntries(ctx context.Context, q JournalEntryQuery) ([]*JournalEntry, JournalEntrySummary, error) {
	q.Page, q.PageSize = NormalizeEntryPaging(q.Page, q.PageSize)

	where, args, argIdx := journalEntryFilter(ctx, q)

	summary, err := r.summariseJournalEntries(ctx, where, args)
	if err != nil {
		return nil, JournalEntrySummary{}, err
	}

	// Newest period first, then the natural reading order within a period: an
	// accountant follows a date down the page, not an identifier.
	query := fmt.Sprintf(`
		SELECT %s
		FROM journal_entries je
		JOIN lease_contracts lc ON lc.id = je.contract_id
		%s
		ORDER BY je.accounting_period DESC, je.entry_date ASC, je.entry_type ASC, je.id ASC
		LIMIT $%d OFFSET $%d
	`, journalEntryColumns, where, argIdx, argIdx+1)
	args = append(args, q.PageSize, (q.Page-1)*q.PageSize)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, JournalEntrySummary{}, fmt.Errorf("failed to list journal entries: %w", err)
	}
	defer rows.Close()

	entries := make([]*JournalEntry, 0, q.PageSize)
	for rows.Next() {
		e := &JournalEntry{}
		if err := rows.Scan(
			&e.ID, &e.ContractID, &e.MeasurementResultID, &e.AccountingPeriod, &e.EntryDate,
			&e.EntryType, &e.DebitAccount, &e.CreditAccount, &e.Amount, &e.Currency,
			&e.Description, &e.VoucherNumber, &e.PostingStatus, &e.PostedAt, &e.PostedBy,
			&e.ApprovedBy, &e.ApprovedAt, &e.ERPReference, &e.BatchID,
			&e.ReversalOfEntryID, &e.ReversalReason, &e.ReversedAt, &e.ReversedBy,
			&e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, JournalEntrySummary{}, fmt.Errorf("failed to scan journal entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, summary, rows.Err()
}

func (r *MonthlyClosingRepository) summariseJournalEntries(ctx context.Context, where string, args []interface{}) (JournalEntrySummary, error) {
	// A reversed entry keeps its own amount in the ledger, so the total below is
	// the sum of every line the filter matches — including reversals and the
	// entries they cancel. The status counts are what tells the two apart.
	query := fmt.Sprintf(`
		SELECT COUNT(*),
			COUNT(*) FILTER (WHERE je.posting_status = 'draft'),
			COUNT(*) FILTER (WHERE je.posting_status = 'approved'),
			COUNT(*) FILTER (WHERE je.posting_status = 'posted'),
			COUNT(*) FILTER (WHERE je.posting_status = 'reversed'),
			COALESCE(SUM(je.amount), 0),
			COUNT(DISTINCT je.contract_id)
		FROM journal_entries je
		JOIN lease_contracts lc ON lc.id = je.contract_id
		%s
	`, where)

	var summary JournalEntrySummary
	if err := r.db.QueryRow(ctx, query, args...).Scan(
		&summary.Total, &summary.DraftCount, &summary.ApprovedCount, &summary.PostedCount,
		&summary.ReversedCount, &summary.TotalAmount, &summary.ContractCount,
	); err != nil {
		return JournalEntrySummary{}, fmt.Errorf("failed to summarise journal entries: %w", err)
	}
	return summary, nil
}

// ListEntryPeriods returns every accounting period that carries entries, newest
// first, with its lock state. This is what lets a period be picked rather than
// remembered.
func (r *MonthlyClosingRepository) ListEntryPeriods(ctx context.Context, legalEntityID string, limit int) ([]JournalEntryPeriod, error) {
	if limit <= 0 || limit > 120 {
		limit = 36
	}

	where, args, argIdx := journalEntryFilter(ctx, JournalEntryQuery{LegalEntityID: legalEntityID})

	// The lock is joined on the same legal entity the entries were filtered by;
	// a period locked for another entity says nothing about this one.
	query := fmt.Sprintf(`
		SELECT je.accounting_period,
			COUNT(*),
			COUNT(DISTINCT je.contract_id),
			COUNT(*) FILTER (WHERE je.posting_status = 'draft'),
			COUNT(*) FILTER (WHERE je.posting_status = 'posted'),
			COALESCE(SUM(je.amount), 0),
			COALESCE(BOOL_OR(pl.is_locked), false),
			MAX(je.created_at)
		FROM journal_entries je
		JOIN lease_contracts lc ON lc.id = je.contract_id
		LEFT JOIN period_locks pl
			ON pl.accounting_period = je.accounting_period
			AND pl.legal_entity_id IS NOT DISTINCT FROM lc.legal_entity_id
		%s
		GROUP BY je.accounting_period
		ORDER BY je.accounting_period DESC
		LIMIT $%d
	`, where, argIdx)
	args = append(args, limit)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list entry periods: %w", err)
	}
	defer rows.Close()

	periods := make([]JournalEntryPeriod, 0)
	for rows.Next() {
		var p JournalEntryPeriod
		if err := rows.Scan(&p.AccountingPeriod, &p.EntryCount, &p.ContractCount,
			&p.DraftCount, &p.PostedCount, &p.TotalAmount, &p.IsLocked, &p.LastEntryAt); err != nil {
			return nil, fmt.Errorf("failed to scan entry period: %w", err)
		}
		periods = append(periods, p)
	}
	return periods, rows.Err()
}

// NormalizeEntryPaging clamps a requested page into what the query will
// actually serve, so the caller and the response agree on the page size.
func NormalizeEntryPaging(page, pageSize int) (int, int) {
	if pageSize <= 0 {
		pageSize = defaultEntryPageSize
	}
	if pageSize > maxEntryPageSize {
		pageSize = maxEntryPageSize
	}
	if page <= 0 {
		page = 1
	}
	return page, pageSize
}

// NormalizeEntryStatus rejects a status the ledger never uses, so a typo in a
// query string returns an error instead of an empty page that looks like data.
func NormalizeEntryStatus(status string) (string, bool) {
	status = strings.TrimSpace(status)
	if status == "" {
		return "", true
	}
	switch status {
	case "draft", "approved", "posted", "reversed":
		return status, true
	default:
		return "", false
	}
}
