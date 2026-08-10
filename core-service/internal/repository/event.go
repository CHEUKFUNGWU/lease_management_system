package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type LeaseEvent struct {
	ID              string     `json:"id"`
	ContractID      string     `json:"contract_id"`
	EventType       string     `json:"event_type"`
	EffectiveDate   time.Time  `json:"effective_date"`
	ApplicationDate *time.Time `json:"application_date"`
	ApprovalDate    *time.Time `json:"approval_date"`
	OriginalValue   *string    `json:"original_value"`
	NewValue        *string    `json:"new_value"`
	// RevisionParameters carries the clause in structured form so the revised
	// payment schedule can be derived. Events recorded before it existed leave
	// it nil and keep calculating from NewValue exactly as they did.
	RevisionParameters     []byte      `json:"revision_parameters,omitempty"`
	ChangeReason           *string     `json:"change_reason"`
	JudgmentBasis          *string     `json:"judgment_basis"`
	Status                 string      `json:"status"`
	RecalculationBatchID   *string     `json:"recalculation_batch_id"`
	CreatedBy              *string     `json:"created_by"`
	ApprovedBy             *string     `json:"approved_by"`
	ApprovalStatus         string      `json:"approval_status"`
	IsOfficialVersion      bool        `json:"is_official_version"`
	ReviewedBy             *string     `json:"reviewed_by"`
	ReviewedAt             *time.Time  `json:"reviewed_at"`
	RejectedReason         *string     `json:"rejected_reason"`
	SourceReferenceLocator interface{} `json:"source_reference_locator,omitempty"`
	CreatedAt              time.Time   `json:"created_at"`
	UpdatedAt              time.Time   `json:"updated_at"`
}

type EventRepository struct {
	db DBTX
}

func NewEventRepository(db DBTX) *EventRepository {
	return &EventRepository{db: db}
}

func (r *EventRepository) WithTx(tx DBTX) *EventRepository {
	return &EventRepository{db: tx}
}

func (r *EventRepository) Create(ctx context.Context, event *LeaseEvent) (*LeaseEvent, error) {
	event.ID = uuid.New().String()
	event.Status = "pending"
	event.ApprovalStatus = "draft"
	event.CreatedAt = time.Now()
	event.UpdatedAt = time.Now()

	query := `
		INSERT INTO lease_events (
			id, contract_id, event_type, effective_date, application_date,
			approval_date, original_value, new_value, change_reason,
			judgment_basis, status, recalculation_batch_id,
			created_by, approved_by, created_at, updated_at,
			approval_status, is_official_version, reviewed_by,
			reviewed_at, rejected_reason, revision_parameters, source_reference_locator
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23)
	`

	_, err := r.db.Exec(ctx, query,
		event.ID, event.ContractID, event.EventType, event.EffectiveDate,
		event.ApplicationDate, event.ApprovalDate, event.OriginalValue,
		event.NewValue, event.ChangeReason, event.JudgmentBasis,
		event.Status, event.RecalculationBatchID,
		event.CreatedBy, event.ApprovedBy, event.CreatedAt, event.UpdatedAt,
		event.ApprovalStatus, event.IsOfficialVersion, event.ReviewedBy,
		event.ReviewedAt, event.RejectedReason, nullableJSON(event.RevisionParameters), nullableJSONValue(event.SourceReferenceLocator),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create event: %w", err)
	}

	return event, nil
}

func (r *EventRepository) GetByContractID(ctx context.Context, contractID string) ([]*LeaseEvent, error) {
	query := `
		SELECT le.id, le.contract_id, le.event_type, le.effective_date, le.application_date,
			le.approval_date, le.original_value, le.new_value, le.change_reason,
			le.judgment_basis, le.status, le.recalculation_batch_id,
			le.created_by, le.approved_by, le.created_at, le.updated_at,
			le.approval_status, le.is_official_version, le.reviewed_by,
			le.reviewed_at, le.rejected_reason, le.revision_parameters, le.source_reference_locator
		FROM lease_events le
		JOIN lease_contracts lc ON lc.id = le.contract_id
		WHERE le.contract_id = $1
	`
	args := []interface{}{contractID}
	query, args, _ = appendContractScopePredicate(ctx, query, args, 2, "lc")
	query += " ORDER BY le.created_at DESC"

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}
	defer rows.Close()

	var events []*LeaseEvent
	for rows.Next() {
		e := &LeaseEvent{}
		err := rows.Scan(
			&e.ID, &e.ContractID, &e.EventType, &e.EffectiveDate,
			&e.ApplicationDate, &e.ApprovalDate, &e.OriginalValue,
			&e.NewValue, &e.ChangeReason, &e.JudgmentBasis,
			&e.Status, &e.RecalculationBatchID,
			&e.CreatedBy, &e.ApprovedBy, &e.CreatedAt, &e.UpdatedAt,
			&e.ApprovalStatus, &e.IsOfficialVersion, &e.ReviewedBy,
			&e.ReviewedAt, &e.RejectedReason, &e.RevisionParameters, &e.SourceReferenceLocator,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, e)
	}

	return events, nil
}

func (r *EventRepository) GetByID(ctx context.Context, id string) (*LeaseEvent, error) {
	query := `
		SELECT id, contract_id, event_type, effective_date, application_date,
			approval_date, original_value, new_value, change_reason,
			judgment_basis, status, recalculation_batch_id,
			created_by, approved_by, created_at, updated_at,
			approval_status, is_official_version, reviewed_by,
			reviewed_at, rejected_reason, revision_parameters, source_reference_locator
		FROM lease_events WHERE id = $1
	`

	e := &LeaseEvent{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&e.ID, &e.ContractID, &e.EventType, &e.EffectiveDate,
		&e.ApplicationDate, &e.ApprovalDate, &e.OriginalValue,
		&e.NewValue, &e.ChangeReason, &e.JudgmentBasis,
		&e.Status, &e.RecalculationBatchID,
		&e.CreatedBy, &e.ApprovedBy, &e.CreatedAt, &e.UpdatedAt,
		&e.ApprovalStatus, &e.IsOfficialVersion, &e.ReviewedBy,
		&e.ReviewedAt, &e.RejectedReason, &e.RevisionParameters, &e.SourceReferenceLocator,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get event: %w", err)
	}

	return e, nil
}

func (r *EventRepository) SubmitForReview(ctx context.Context, eventID string) error {
	query := `
		UPDATE lease_events
		SET approval_status = 'submitted',
		    is_official_version = false,
		    reviewed_by = NULL,
		    reviewed_at = NULL,
		    approved_by = NULL,
		    approval_date = NULL,
		    rejected_reason = NULL,
		    updated_at = $2
		WHERE id = $1
		  AND approval_status IN ('draft', 'returned_to_editor', 'rejected')
	`
	result, err := r.db.Exec(ctx, query, eventID, time.Now())
	return requireWorkflowTransition(result, err, "event", eventID)
}

func (r *EventRepository) Review(ctx context.Context, eventID, userID string, approved bool, reason string) error {
	status := "reviewed"
	if !approved {
		status = "returned_to_editor"
	}

	now := time.Now()
	query := `
		UPDATE lease_events
		SET approval_status = $3,
		    reviewed_by = $2,
		    reviewed_at = $4,
		    rejected_reason = CASE WHEN $3 = 'returned_to_editor' THEN NULLIF($5, '') ELSE NULL END,
		    updated_at = $4
		WHERE id = $1
		  AND approval_status = 'submitted'
	`
	result, err := r.db.Exec(ctx, query, eventID, userID, status, now, reason)
	return requireWorkflowTransition(result, err, "event", eventID)
}

func (r *EventRepository) Approve(ctx context.Context, eventID, userID, ifrs16Treatment string) error {
	now := time.Now()
	query := `
		UPDATE lease_events
		SET approval_status = 'approved',
		    is_official_version = true,
		    approved_by = $2,
		    approval_date = $3,
		    ifrs16_treatment = $4,
		    status = 'active',
		    rejected_reason = NULL,
		    updated_at = $3
		WHERE id = $1
		  AND approval_status = 'reviewed'
	`
	result, err := r.db.Exec(ctx, query, eventID, userID, now, ifrs16Treatment)
	return requireWorkflowTransition(result, err, "event", eventID)
}

// GetApprovedEventsForContract returns all approved events for a contract ordered by effective date.
func (r *EventRepository) GetApprovedEventsForContract(ctx context.Context, contractID string) ([]*LeaseEvent, error) {
	query := `
		SELECT id, contract_id, event_type, effective_date, application_date,
			approval_date, original_value, new_value, change_reason,
			judgment_basis, status, recalculation_batch_id,
			created_by, approved_by, created_at, updated_at,
			approval_status, is_official_version, reviewed_by,
			reviewed_at, rejected_reason, revision_parameters, source_reference_locator
		FROM lease_events
		WHERE contract_id = $1 AND approval_status = 'approved'
		ORDER BY effective_date ASC
	`

	rows, err := r.db.Query(ctx, query, contractID)
	if err != nil {
		return nil, fmt.Errorf("failed to list approved events: %w", err)
	}
	defer rows.Close()

	var events []*LeaseEvent
	for rows.Next() {
		e := &LeaseEvent{}
		err := rows.Scan(
			&e.ID, &e.ContractID, &e.EventType, &e.EffectiveDate,
			&e.ApplicationDate, &e.ApprovalDate, &e.OriginalValue,
			&e.NewValue, &e.ChangeReason, &e.JudgmentBasis,
			&e.Status, &e.RecalculationBatchID,
			&e.CreatedBy, &e.ApprovedBy, &e.CreatedAt, &e.UpdatedAt,
			&e.ApprovalStatus, &e.IsOfficialVersion, &e.ReviewedBy,
			&e.ReviewedAt, &e.RejectedReason, &e.RevisionParameters, &e.SourceReferenceLocator,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, e)
	}

	return events, nil
}

// LinkRecalculationBatch updates an event's recalculation_batch_id after recalculation.
func (r *EventRepository) LinkRecalculationBatch(ctx context.Context, eventID, batchID string) error {
	query := `
		UPDATE lease_events
		SET recalculation_batch_id = $1,
		    updated_at = NOW()
		WHERE id = $2
	`
	_, err := r.db.Exec(ctx, query, batchID, eventID)
	return err
}

func (r *EventRepository) Reject(ctx context.Context, eventID, userID, reason string) error {
	now := time.Now()
	query := `
		UPDATE lease_events
		SET approval_status = 'rejected',
		    is_official_version = false,
		    rejected_reason = $3,
		    approved_by = $2,
		    approval_date = $4,
		    updated_at = $4
		WHERE id = $1
		  AND approval_status = 'reviewed'
	`
	result, err := r.db.Exec(ctx, query, eventID, userID, reason, now)
	return requireWorkflowTransition(result, err, "event", eventID)
}

// nullableJSON keeps an absent clause out of the column entirely. Writing an
// empty byte slice would store an invalid JSON document and fail the insert.
func nullableJSON(raw []byte) interface{} {
	if len(raw) == 0 {
		return nil
	}
	return raw
}

func nullableJSONValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return raw
}
