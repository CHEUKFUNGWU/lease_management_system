package repository

import (
	"context"
	"fmt"
	"time"
)

// WorkQueueItem is one thing waiting on somebody, normalised across contracts,
// events, journal entries and critical dates so a single list can be presented.
type WorkQueueItem struct {
	Kind        string     `json:"kind"`
	Stage       string     `json:"stage"`
	RecordID    string     `json:"record_id"`
	ContractID  string     `json:"contract_id"`
	Title       string     `json:"title"`
	Subtitle    string     `json:"subtitle"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	Amount      *float64   `json:"amount,omitempty"`
	Currency    string     `json:"currency,omitempty"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`
}

// WorkQueue is everything currently waiting, grouped by what has to happen next.
type WorkQueue struct {
	ContractsPendingReview   []WorkQueueItem `json:"contracts_pending_review"`
	ContractsPendingApproval []WorkQueueItem `json:"contracts_pending_approval"`
	EventsPending            []WorkQueueItem `json:"events_pending"`
	EntriesPendingApproval   []WorkQueueItem `json:"entries_pending_approval"`
	EntriesPendingPosting    []WorkQueueItem `json:"entries_pending_posting"`
	CriticalDatesDue         []WorkQueueItem `json:"critical_dates_due"`
	Total                    int             `json:"total"`
}

type WorkQueueRepository struct {
	db DBTX
}

func NewWorkQueueRepository(db DBTX) *WorkQueueRepository {
	return &WorkQueueRepository{db: db}
}

// Load gathers every outstanding item the caller is allowed to see.
//
// Each query carries the caller's data scope, so a regional or brand-scoped user
// sees only their own backlog. Items are capped per group: a work queue is a
// prompt to act, not a report.
func (r *WorkQueueRepository) Load(ctx context.Context, legalEntityID string, criticalDateWindowDays int) (*WorkQueue, error) {
	queue := &WorkQueue{
		ContractsPendingReview:   []WorkQueueItem{},
		ContractsPendingApproval: []WorkQueueItem{},
		EventsPending:            []WorkQueueItem{},
		EntriesPendingApproval:   []WorkQueueItem{},
		EntriesPendingPosting:    []WorkQueueItem{},
		CriticalDatesDue:         []WorkQueueItem{},
	}

	contracts, err := r.contractsAwaiting(ctx, legalEntityID)
	if err != nil {
		return nil, err
	}
	for _, item := range contracts {
		if item.Stage == "pending_approval" {
			queue.ContractsPendingApproval = append(queue.ContractsPendingApproval, item)
			continue
		}
		queue.ContractsPendingReview = append(queue.ContractsPendingReview, item)
	}

	if queue.EventsPending, err = r.eventsAwaiting(ctx, legalEntityID); err != nil {
		return nil, err
	}

	entries, err := r.entriesAwaiting(ctx, legalEntityID)
	if err != nil {
		return nil, err
	}
	for _, item := range entries {
		if item.Stage == "approved" {
			queue.EntriesPendingPosting = append(queue.EntriesPendingPosting, item)
			continue
		}
		queue.EntriesPendingApproval = append(queue.EntriesPendingApproval, item)
	}

	if queue.CriticalDatesDue, err = r.criticalDatesDue(ctx, legalEntityID, criticalDateWindowDays); err != nil {
		return nil, err
	}

	queue.Total = len(queue.ContractsPendingReview) + len(queue.ContractsPendingApproval) +
		len(queue.EventsPending) + len(queue.EntriesPendingApproval) +
		len(queue.EntriesPendingPosting) + len(queue.CriticalDatesDue)
	return queue, nil
}

func (r *WorkQueueRepository) contractsAwaiting(ctx context.Context, legalEntityID string) ([]WorkQueueItem, error) {
	query := `
		SELECT c.id, COALESCE(c.contract_number, ''), COALESCE(c.contract_name, ''),
			COALESCE(c.store_name, ''), c.approval_status, c.submitted_at
		FROM lease_contracts c
		WHERE c.approval_status IN ('submitted', 'reviewed', 'pending_approval')
	`
	args := []interface{}{}
	argIdx := 1
	if legalEntityID != "" {
		query += fmt.Sprintf(" AND c.legal_entity_id = $%d", argIdx)
		args = append(args, legalEntityID)
		argIdx++
	}
	query, args, _ = appendContractScopePredicate(ctx, query, args, argIdx, "c")
	query += " ORDER BY c.submitted_at ASC NULLS LAST LIMIT 50"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to load contracts awaiting action: %w", err)
	}
	defer rows.Close()

	items := []WorkQueueItem{}
	for rows.Next() {
		var item WorkQueueItem
		var number, name, store, status string
		if err := rows.Scan(&item.RecordID, &number, &name, &store, &status, &item.SubmittedAt); err != nil {
			return nil, fmt.Errorf("failed to scan contract work item: %w", err)
		}
		item.Kind = "contract"
		item.Stage = status
		item.ContractID = item.RecordID
		item.Title = number + " " + name
		item.Subtitle = store
		items = append(items, item)
	}
	return items, nil
}

func (r *WorkQueueRepository) eventsAwaiting(ctx context.Context, legalEntityID string) ([]WorkQueueItem, error) {
	query := `
		SELECT e.id, e.contract_id, COALESCE(e.event_type, ''), COALESCE(e.change_reason, ''),
			e.approval_status, e.effective_date, COALESCE(c.contract_number, '')
		FROM lease_events e
		JOIN lease_contracts c ON c.id = e.contract_id
		WHERE e.approval_status IN ('submitted', 'reviewed', 'pending_approval')
	`
	args := []interface{}{}
	argIdx := 1
	if legalEntityID != "" {
		query += fmt.Sprintf(" AND c.legal_entity_id = $%d", argIdx)
		args = append(args, legalEntityID)
		argIdx++
	}
	query, args, _ = appendContractScopePredicate(ctx, query, args, argIdx, "c")
	query += " ORDER BY e.created_at ASC LIMIT 50"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to load events awaiting action: %w", err)
	}
	defer rows.Close()

	items := []WorkQueueItem{}
	for rows.Next() {
		var item WorkQueueItem
		var eventType, changeReason, status, contractNumber string
		var effectiveDate time.Time
		if err := rows.Scan(&item.RecordID, &item.ContractID, &eventType, &changeReason,
			&status, &effectiveDate, &contractNumber); err != nil {
			return nil, fmt.Errorf("failed to scan event work item: %w", err)
		}
		item.Kind = "event"
		item.Stage = status
		item.Title = eventType
		item.Subtitle = contractNumber + " " + changeReason
		item.DueDate = &effectiveDate
		items = append(items, item)
	}
	return items, nil
}

func (r *WorkQueueRepository) entriesAwaiting(ctx context.Context, legalEntityID string) ([]WorkQueueItem, error) {
	query := `
		SELECT je.id, je.contract_id, je.entry_type, je.accounting_period,
			je.amount, je.currency, je.posting_status, COALESCE(c.contract_number, '')
		FROM journal_entries je
		JOIN lease_contracts c ON c.id = je.contract_id
		WHERE je.posting_status IN ('draft', 'approved')
	`
	args := []interface{}{}
	argIdx := 1
	if legalEntityID != "" {
		query += fmt.Sprintf(" AND c.legal_entity_id = $%d", argIdx)
		args = append(args, legalEntityID)
		argIdx++
	}
	query, args, _ = appendContractScopePredicate(ctx, query, args, argIdx, "c")
	query += " ORDER BY je.accounting_period DESC, je.entry_type LIMIT 100"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to load journal entries awaiting action: %w", err)
	}
	defer rows.Close()

	items := []WorkQueueItem{}
	for rows.Next() {
		var item WorkQueueItem
		var entryType, period, status, contractNumber string
		var amount float64
		if err := rows.Scan(&item.RecordID, &item.ContractID, &entryType, &period,
			&amount, &item.Currency, &status, &contractNumber); err != nil {
			return nil, fmt.Errorf("failed to scan journal entry work item: %w", err)
		}
		item.Kind = "journal_entry"
		item.Stage = status
		item.Title = period + " " + entryType
		item.Subtitle = contractNumber
		item.Amount = &amount
		items = append(items, item)
	}
	return items, nil
}

func (r *WorkQueueRepository) criticalDatesDue(ctx context.Context, legalEntityID string, windowDays int) ([]WorkQueueItem, error) {
	if windowDays <= 0 {
		windowDays = 30
	}
	query := `
		SELECT cd.id, cd.contract_id, cd.title, cd.date_type, cd.target_date,
			COALESCE(c.contract_number, '')
		FROM critical_dates cd
		JOIN lease_contracts c ON c.id = cd.contract_id
		WHERE cd.status = 'open' AND cd.target_date <= CURRENT_DATE + make_interval(days => $1)
	`
	args := []interface{}{windowDays}
	argIdx := 2
	if legalEntityID != "" {
		query += fmt.Sprintf(" AND c.legal_entity_id = $%d", argIdx)
		args = append(args, legalEntityID)
		argIdx++
	}
	query, args, _ = appendContractScopePredicate(ctx, query, args, argIdx, "c")
	query += " ORDER BY cd.target_date ASC LIMIT 50"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to load critical dates due: %w", err)
	}
	defer rows.Close()

	items := []WorkQueueItem{}
	for rows.Next() {
		var item WorkQueueItem
		var title, dateType, contractNumber string
		var target time.Time
		if err := rows.Scan(&item.RecordID, &item.ContractID, &title, &dateType,
			&target, &contractNumber); err != nil {
			return nil, fmt.Errorf("failed to scan critical date work item: %w", err)
		}
		item.Kind = "critical_date"
		item.Stage = dateType
		item.Title = title
		item.Subtitle = contractNumber
		item.DueDate = &target
		items = append(items, item)
	}
	return items, nil
}
