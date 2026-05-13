package repository

import (
	"context"
	"fmt"
	"time"
	
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LeaseEvent struct {
	ID                    string     `json:"id"`
	ContractID            string     `json:"contract_id"`
	EventType             string     `json:"event_type"`
	EffectiveDate         time.Time  `json:"effective_date"`
	ApplicationDate       *time.Time `json:"application_date"`
	ApprovalDate          *time.Time `json:"approval_date"`
	OriginalValue         *string    `json:"original_value"`
	NewValue              *string    `json:"new_value"`
	ChangeReason          *string    `json:"change_reason"`
	JudgmentBasis         *string    `json:"judgment_basis"`
	Status                string     `json:"status"`
	RecalculationBatchID  *string    `json:"recalculation_batch_id"`
	CreatedBy             *string    `json:"created_by"`
	ApprovedBy            *string    `json:"approved_by"`
	ApprovalStatus        string     `json:"approval_status"`
	IsOfficialVersion     bool       `json:"is_official_version"`
	ReviewedBy            *string    `json:"reviewed_by"`
	ReviewedAt            *time.Time `json:"reviewed_at"`
	RejectedReason        *string    `json:"rejected_reason"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type EventRepository struct {
	db *pgxpool.Pool
}

func NewEventRepository(db *pgxpool.Pool) *EventRepository {
	return &EventRepository{db: db}
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
			reviewed_at, rejected_reason
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
	`
	
	_, err := r.db.Exec(ctx, query,
		event.ID, event.ContractID, event.EventType, event.EffectiveDate,
		event.ApplicationDate, event.ApprovalDate, event.OriginalValue,
		event.NewValue, event.ChangeReason, event.JudgmentBasis,
		event.Status, event.RecalculationBatchID,
		event.CreatedBy, event.ApprovedBy, event.CreatedAt, event.UpdatedAt,
		event.ApprovalStatus, event.IsOfficialVersion, event.ReviewedBy,
		event.ReviewedAt, event.RejectedReason,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create event: %w", err)
	}
	
	return event, nil
}

func (r *EventRepository) GetByContractID(ctx context.Context, contractID string) ([]*LeaseEvent, error) {
	query := `
		SELECT id, contract_id, event_type, effective_date, application_date,
			approval_date, original_value, new_value, change_reason,
			judgment_basis, status, recalculation_batch_id,
			created_by, approved_by, created_at, updated_at,
			approval_status, is_official_version, reviewed_by,
			reviewed_at, rejected_reason
		FROM lease_events WHERE contract_id = $1 ORDER BY created_at DESC
	`
	
	rows, err := r.db.Query(ctx, query, contractID)
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
			&e.ReviewedAt, &e.RejectedReason,
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
			reviewed_at, rejected_reason
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
		&e.ReviewedAt, &e.RejectedReason,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get event: %w", err)
	}
	
	return e, nil
}
