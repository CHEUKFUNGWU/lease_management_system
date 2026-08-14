package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lease-management-system/core-service/internal/access"
)

type RenewalDecisionSnapshot struct {
	ID              string          `json:"id"`
	ContractID      string          `json:"contract_id"`
	LegalEntityID   *string         `json:"legal_entity_id"`
	DecisionDate    time.Time       `json:"decision_date"`
	OwnerName       string          `json:"owner_name"`
	BusinessOpinion string          `json:"business_opinion"`
	Evidence        string          `json:"evidence"`
	Snapshot        json.RawMessage `json:"snapshot"`
	CreatedBy       *string         `json:"created_by"`
	CreatedAt       time.Time       `json:"created_at"`
}

type RenewalDecisionRepository struct {
	db DBTX
}

func NewRenewalDecisionRepository(db DBTX) *RenewalDecisionRepository {
	return &RenewalDecisionRepository{db: db}
}

func (r *RenewalDecisionRepository) Create(ctx context.Context, snapshot *RenewalDecisionSnapshot) (*RenewalDecisionSnapshot, error) {
	snapshot.ID = uuid.New().String()
	if len(snapshot.Snapshot) == 0 {
		snapshot.Snapshot = json.RawMessage(`{}`)
	}
	err := r.db.QueryRow(ctx, `
		INSERT INTO renewal_decision_snapshots (
			id, contract_id, legal_entity_id, decision_date, owner_name,
			business_opinion, evidence, snapshot, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at
	`, snapshot.ID, snapshot.ContractID, snapshot.LegalEntityID, snapshot.DecisionDate,
		snapshot.OwnerName, snapshot.BusinessOpinion, snapshot.Evidence, snapshot.Snapshot,
		snapshot.CreatedBy).Scan(&snapshot.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create renewal decision snapshot: %w", err)
	}
	return snapshot, nil
}

func (r *RenewalDecisionRepository) List(ctx context.Context, contractID string, entity access.EntityFilter) ([]*RenewalDecisionSnapshot, error) {
	args := []any{contractID}
	query := `
		SELECT id, contract_id, legal_entity_id, decision_date, owner_name,
			business_opinion, evidence, snapshot, created_by, created_at
		FROM renewal_decision_snapshots
		WHERE contract_id = $1`
	if clause, arg, err := entity.SQLClause("legal_entity_id", len(args)+1); err != nil {
		return nil, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query += `
		ORDER BY decision_date DESC, created_at DESC
		LIMIT 100
	`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list renewal decisions: %w", err)
	}
	defer rows.Close()
	result := make([]*RenewalDecisionSnapshot, 0)
	for rows.Next() {
		item := &RenewalDecisionSnapshot{}
		if err := rows.Scan(&item.ID, &item.ContractID, &item.LegalEntityID, &item.DecisionDate,
			&item.OwnerName, &item.BusinessOpinion, &item.Evidence, &item.Snapshot,
			&item.CreatedBy, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan renewal decision: %w", err)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate renewal decisions: %w", err)
	}
	return result, nil
}

func (r *RenewalDecisionRepository) Exists(ctx context.Context, contractID string, entity access.EntityFilter) (bool, error) {
	var exists bool
	args := []any{contractID}
	query := `
		SELECT EXISTS (
			SELECT 1 FROM renewal_decision_snapshots
			WHERE contract_id = $1`
	if clause, arg, err := entity.SQLClause("legal_entity_id", len(args)+1); err != nil {
		return false, err
	} else if clause != "" {
		query += " AND " + clause
		args = append(args, arg)
	}
	query += `
		)
	`
	err := r.db.QueryRow(ctx, query, args...).Scan(&exists)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check renewal decisions: %w", err)
	}
	return exists, nil
}
