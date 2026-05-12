package repository

import (
	"context"
	"fmt"
	"time"
	
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ContractApproval struct {
	ContractID        string     `json:"contract_id"`
	ApprovalStatus    string     `json:"approval_status"`
	IsOfficialVersion bool       `json:"is_official_version"`
	DraftVersionNo    int        `json:"draft_version_no"`
	ReportMode        string     `json:"report_mode"`
	ReviewedBy        *string    `json:"reviewed_by"`
	ReviewedAt        *time.Time `json:"reviewed_at"`
	ApprovedBy        *string    `json:"approved_by"`
	ApprovedAt        *time.Time `json:"approved_at"`
	RejectedReason    *string    `json:"rejected_reason"`
}

type ApprovalRepository struct {
	db *pgxpool.Pool
}

func NewApprovalRepository(db *pgxpool.Pool) *ApprovalRepository {
	return &ApprovalRepository{db: db}
}

func (r *ApprovalRepository) SubmitForReview(ctx context.Context, contractID, submittedBy string) error {
	query := `
		UPDATE lease_contracts
		SET approval_status = 'submitted',
		    submitted_at = $2,
		    updated_at = $2
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, contractID, time.Now())
	return err
}

func (r *ApprovalRepository) Review(ctx context.Context, contractID, reviewedBy string, approved bool, reason string) error {
	status := "reviewed"
	if !approved {
		status = "returned_to_editor"
	}
	
	query := `
		UPDATE lease_contracts
		SET approval_status = $3,
		    reviewed_by = $2,
		    reviewed_at = $4,
		    rejected_reason = $5,
		    updated_at = $4
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, contractID, reviewedBy, status, time.Now(), reason)
	return err
}

func (r *ApprovalRepository) Approve(ctx context.Context, contractID, approvedBy string) error {
	query := `
		UPDATE lease_contracts
		SET approval_status = 'approved',
		    is_official_version = true,
		    approved_by = $2,
		    approved_at = $3,
		    updated_at = $3
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, contractID, approvedBy, time.Now())
	return err
}

func (r *ApprovalRepository) Reject(ctx context.Context, contractID, rejectedBy, reason string) error {
	query := `
		UPDATE lease_contracts
		SET approval_status = 'rejected',
		    approved_by = $2,
		    approved_at = $3,
		    rejected_reason = $4,
		    updated_at = $3
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, contractID, rejectedBy, time.Now(), reason)
	return err
}

func (r *ApprovalRepository) GetApprovalStatus(ctx context.Context, contractID string) (*ContractApproval, error) {
	query := `
		SELECT id, approval_status, is_official_version, draft_version_no,
		       report_mode, reviewed_by, reviewed_at, approved_by, approved_at, rejected_reason
		FROM lease_contracts WHERE id = $1
	`
	
	ca := &ContractApproval{}
	var id string
	err := r.db.QueryRow(ctx, query, contractID).Scan(
		&id, &ca.ApprovalStatus, &ca.IsOfficialVersion, &ca.DraftVersionNo,
		&ca.ReportMode, &ca.ReviewedBy, &ca.ReviewedAt,
		&ca.ApprovedBy, &ca.ApprovedAt, &ca.RejectedReason,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get approval status: %w", err)
	}
	
	ca.ContractID = id
	return ca, nil
}

func (r *ApprovalRepository) ListByStatus(ctx context.Context, status string, officialOnly bool) ([]*Contract, error) {
	query := `
		SELECT id, contract_number, contract_name, legal_entity_id, store_id, landlord_id,
			asset_category, property_category, currency, signing_date,
			commencement_date, lease_start_date, lease_end_date, status,
			approval_status, is_official_version, created_by, approved_by,
			created_at, updated_at
		FROM lease_contracts
		WHERE ($1 = '' OR approval_status = $1)
		  AND ($2 = false OR is_official_version = true)
		ORDER BY created_at DESC
	`
	
	rows, err := r.db.Query(ctx, query, status, officialOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to list contracts by status: %w", err)
	}
	defer rows.Close()
	
	var contracts []*Contract
	for rows.Next() {
		c := &Contract{}
		var approvalStatus string
		var isOfficial bool
		err := rows.Scan(
			&c.ID, &c.ContractNumber, &c.ContractName,
			&c.LegalEntityID, &c.StoreID, &c.LandlordID,
			&c.AssetCategory, &c.PropertyCategory, &c.Currency,
			&c.SigningDate, &c.CommencementDate, &c.LeaseStartDate,
			&c.LeaseEndDate, &c.Status,
			&approvalStatus, &isOfficial, &c.CreatedBy, &c.ApprovedBy,
			&c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		contracts = append(contracts, c)
	}
	
	return contracts, nil
}
