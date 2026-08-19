package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lease-management-system/core-service/internal/access"
)

type AccessPolicyRepository struct {
	db *pgxpool.Pool
}

func NewAccessPolicyRepository(db *pgxpool.Pool) *AccessPolicyRepository {
	return &AccessPolicyRepository{db: db}
}

func (r *AccessPolicyRepository) GetApprovalParticipants(ctx context.Context, recordType, recordID string) (access.ApprovalParticipants, bool, error) {
	var query string
	switch recordType {
	case "contract":
		query = `SELECT COALESCE(created_by::text, ''), COALESCE(reviewed_by::text, '') FROM lease_contracts WHERE id = $1`
	case "event":
		query = `SELECT COALESCE(created_by::text, ''), COALESCE(reviewed_by::text, '') FROM lease_events WHERE id = $1`
	case "journal_entry":
		query = `
			SELECT COALESCE(b.created_by::text, c.created_by::text, ''), ''
			FROM journal_entries e
			LEFT JOIN monthly_closing_batches b ON b.id = e.batch_id
			LEFT JOIN lease_contracts c ON c.id = e.contract_id
			WHERE e.id = $1
		`
	case "monthly_batch":
		query = `SELECT COALESCE(created_by::text, ''), '' FROM monthly_closing_batches WHERE id = $1`
	case "statement_template":
		query = `SELECT COALESCE(created_by::text, ''), COALESCE(reviewed_by::text, '') FROM fin_statement_templates WHERE id = $1`
	default:
		return access.ApprovalParticipants{}, false, fmt.Errorf("unsupported approval record type %q", recordType)
	}

	var participants access.ApprovalParticipants
	err := r.db.QueryRow(ctx, query, recordID).Scan(&participants.CreatorID, &participants.ReviewerID)
	if err == pgx.ErrNoRows {
		return access.ApprovalParticipants{}, false, nil
	}
	if err != nil {
		return access.ApprovalParticipants{}, false, fmt.Errorf("failed to load approval participants: %w", err)
	}
	return participants, true, nil
}
