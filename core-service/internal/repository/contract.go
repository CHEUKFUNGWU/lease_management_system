package repository

import (
	"context"
	"fmt"
	"time"
	
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Contract struct {
	ID                    string    `json:"id"`
	ContractNumber        string    `json:"contract_number"`
	ContractName          string    `json:"contract_name"`
	LegalEntityID         string    `json:"legal_entity_id"`
	StoreID               string    `json:"store_id"`
	LandlordID            string    `json:"landlord_id"`
	AssetCategory         *string   `json:"asset_category"`
	PropertyCategory      *string   `json:"property_category"`
	Currency              string    `json:"currency"`
	SigningDate           *time.Time `json:"signing_date"`
	CommencementDate      time.Time `json:"commencement_date"`
	LeaseStartDate        time.Time `json:"lease_start_date"`
	LeaseEndDate          time.Time `json:"lease_end_date"`
	OriginalNonCancellablePeriod *string `json:"original_non_cancellable_period"`
	RenewalOptionDescription     *string `json:"renewal_option_description"`
	TerminationOptionDescription *string `json:"termination_option_description"`
	RenewalAssessment            *bool   `json:"renewal_assessment"`
	TerminationAssessment        *bool   `json:"termination_assessment"`
	DiscountRateType             *string `json:"discount_rate_type"`
	DiscountRateVersion          *string `json:"discount_rate_version"`
	DiscountRateMissing          bool    `json:"discount_rate_missing"`
	DiscountRateSource           *string `json:"discount_rate_source"`
	DiscountRatePolicyID         *string `json:"discount_rate_policy_id"`
	DiscountRateConfirmedBy      *string `json:"discount_rate_confirmed_by"`
	DiscountRateConfirmedAt      *time.Time `json:"discount_rate_confirmed_at"`
	AIExtractedDiscountRate      *string `json:"ai_extracted_discount_rate"`
	AISuggestedRatePolicies      interface{} `json:"ai_suggested_rate_policies"`
	Status               string     `json:"status"`
	ApprovalStatus       string     `json:"approval_status"`
	IsOfficialVersion    bool       `json:"is_official_version"`
	DraftVersionNo       int        `json:"draft_version_no"`
	IncludedInReporting  bool       `json:"included_in_reporting"`
	ReportMode           string     `json:"report_mode"`
	ReviewedBy           *string    `json:"reviewed_by"`
	ReviewedAt           *time.Time `json:"reviewed_at"`
	ApprovedBy           *string    `json:"approved_by"`
	ApprovedAt           *time.Time `json:"approved_at"`
	RejectedReason       *string    `json:"rejected_reason"`
	SubmittedAt          *time.Time `json:"submitted_at"`
	AIConfidenceScore    *float64   `json:"ai_confidence_score"`
	SourceReferenceLocator interface{} `json:"source_reference_locator"`
	CreatedBy            *string    `json:"created_by"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type ContractRepository struct {
	db *pgxpool.Pool
}

func NewContractRepository(db *pgxpool.Pool) *ContractRepository {
	return &ContractRepository{db: db}
}

func (r *ContractRepository) Create(ctx context.Context, contract *Contract) (*Contract, error) {
	contract.ID = uuid.New().String()
	contract.Status = "draft"
	contract.CreatedAt = time.Now()
	contract.UpdatedAt = time.Now()
	
	query := `
		INSERT INTO lease_contracts (
			id, contract_number, contract_name, legal_entity_id, store_id, landlord_id,
			asset_category, property_category, currency, signing_date,
			commencement_date, lease_start_date, lease_end_date,
			original_non_cancellable_period, renewal_option_description,
			termination_option_description, renewal_assessment,
			termination_assessment, discount_rate_type, discount_rate_version,
			status, created_by, approved_by, created_at, updated_at,
			approval_status, is_official_version, draft_version_no,
			included_in_reporting, report_mode, reviewed_by, reviewed_at,
			rejected_reason, submitted_at, discount_rate_missing,
			discount_rate_source, discount_rate_policy_id,
			discount_rate_confirmed_by, discount_rate_confirmed_at,
			ai_extracted_discount_rate, ai_suggested_rate_policies,
			ai_confidence_score, source_reference_locator, approved_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36, $37, $38, $39, $40, $41, $42, $43, $44)
		RETURNING approval_status, is_official_version, draft_version_no,
			included_in_reporting, report_mode
	`
	
	err := r.db.QueryRow(ctx, query,
		contract.ID, contract.ContractNumber, contract.ContractName,
		contract.LegalEntityID, contract.StoreID, contract.LandlordID,
		contract.AssetCategory, contract.PropertyCategory, contract.Currency,
		contract.SigningDate, contract.CommencementDate, contract.LeaseStartDate,
		contract.LeaseEndDate, contract.OriginalNonCancellablePeriod,
		contract.RenewalOptionDescription, contract.TerminationOptionDescription,
		contract.RenewalAssessment, contract.TerminationAssessment,
		contract.DiscountRateType, contract.DiscountRateVersion,
		contract.Status, contract.CreatedBy, contract.ApprovedBy,
		contract.CreatedAt, contract.UpdatedAt,
		contract.ApprovalStatus, contract.IsOfficialVersion, contract.DraftVersionNo,
		contract.IncludedInReporting, contract.ReportMode, contract.ReviewedBy,
		contract.ReviewedAt, contract.RejectedReason, contract.SubmittedAt,
		contract.DiscountRateMissing, contract.DiscountRateSource,
		contract.DiscountRatePolicyID, contract.DiscountRateConfirmedBy,
		contract.DiscountRateConfirmedAt, contract.AIExtractedDiscountRate,
		contract.AISuggestedRatePolicies, contract.AIConfidenceScore,
		contract.SourceReferenceLocator, contract.ApprovedAt,
	).Scan(
		&contract.ApprovalStatus, &contract.IsOfficialVersion, &contract.DraftVersionNo,
		&contract.IncludedInReporting, &contract.ReportMode,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create contract: %w", err)
	}
	
	return contract, nil
}

func (r *ContractRepository) GetAll(ctx context.Context, legalEntityID string) ([]*Contract, error) {
	var query string
	var args []interface{}
	
	if legalEntityID != "" {
		query = `
			SELECT id, contract_number, contract_name, legal_entity_id, store_id, landlord_id,
				asset_category, property_category, currency, signing_date,
				commencement_date, lease_start_date, lease_end_date, status,
				created_by, approved_by, created_at, updated_at,
				approval_status, is_official_version, draft_version_no,
				included_in_reporting, report_mode, reviewed_by, reviewed_at,
				rejected_reason, submitted_at, discount_rate_type, discount_rate_version,
				discount_rate_missing, discount_rate_source, discount_rate_policy_id,
				discount_rate_confirmed_by, discount_rate_confirmed_at,
				ai_extracted_discount_rate, ai_suggested_rate_policies,
				ai_confidence_score, source_reference_locator, approved_at
			FROM lease_contracts WHERE legal_entity_id = $1 ORDER BY created_at DESC
		`
		args = append(args, legalEntityID)
	} else {
		query = `
			SELECT id, contract_number, contract_name, legal_entity_id, store_id, landlord_id,
				asset_category, property_category, currency, signing_date,
				commencement_date, lease_start_date, lease_end_date, status,
				created_by, approved_by, created_at, updated_at,
				approval_status, is_official_version, draft_version_no,
				included_in_reporting, report_mode, reviewed_by, reviewed_at,
				rejected_reason, submitted_at, discount_rate_type, discount_rate_version,
				discount_rate_missing, discount_rate_source, discount_rate_policy_id,
				discount_rate_confirmed_by, discount_rate_confirmed_at,
				ai_extracted_discount_rate, ai_suggested_rate_policies,
				ai_confidence_score, source_reference_locator, approved_at
			FROM lease_contracts ORDER BY created_at DESC
		`
	}
	
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list contracts: %w", err)
	}
	defer rows.Close()
	
	var contracts []*Contract
	for rows.Next() {
		c := &Contract{}
		err := rows.Scan(
			&c.ID, &c.ContractNumber, &c.ContractName,
			&c.LegalEntityID, &c.StoreID, &c.LandlordID,
			&c.AssetCategory, &c.PropertyCategory, &c.Currency,
			&c.SigningDate, &c.CommencementDate, &c.LeaseStartDate,
			&c.LeaseEndDate, &c.Status,
			&c.CreatedBy, &c.ApprovedBy, &c.CreatedAt, &c.UpdatedAt,
			&c.ApprovalStatus, &c.IsOfficialVersion, &c.DraftVersionNo,
			&c.IncludedInReporting, &c.ReportMode, &c.ReviewedBy, &c.ReviewedAt,
			&c.RejectedReason, &c.SubmittedAt, &c.DiscountRateType, &c.DiscountRateVersion,
			&c.DiscountRateMissing, &c.DiscountRateSource, &c.DiscountRatePolicyID,
			&c.DiscountRateConfirmedBy, &c.DiscountRateConfirmedAt,
			&c.AIExtractedDiscountRate, &c.AISuggestedRatePolicies,
			&c.AIConfidenceScore, &c.SourceReferenceLocator, &c.ApprovedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan contract: %w", err)
		}
		contracts = append(contracts, c)
	}
	
	return contracts, nil
}

func (r *ContractRepository) GetByStatuses(ctx context.Context, statuses []string, legalEntityID string) ([]*Contract, error) {
	if len(statuses) == 0 {
		return r.GetAll(ctx, legalEntityID)
	}
	
	var query string
	var args []interface{}
	
	if legalEntityID != "" {
		query = `
			SELECT id, contract_number, contract_name, legal_entity_id, store_id, landlord_id,
				asset_category, property_category, currency, signing_date,
				commencement_date, lease_start_date, lease_end_date, status,
				created_by, approved_by, created_at, updated_at,
				approval_status, is_official_version, draft_version_no,
				included_in_reporting, report_mode, reviewed_by, reviewed_at,
				rejected_reason, submitted_at, discount_rate_type, discount_rate_version,
				discount_rate_missing, discount_rate_source, discount_rate_policy_id,
				discount_rate_confirmed_by, discount_rate_confirmed_at,
				ai_extracted_discount_rate, ai_suggested_rate_policies,
				ai_confidence_score, source_reference_locator, approved_at
			FROM lease_contracts
			WHERE approval_status = ANY($1) AND legal_entity_id = $2
			ORDER BY created_at DESC
		`
		args = append(args, statuses, legalEntityID)
	} else {
		query = `
			SELECT id, contract_number, contract_name, legal_entity_id, store_id, landlord_id,
				asset_category, property_category, currency, signing_date,
				commencement_date, lease_start_date, lease_end_date, status,
				created_by, approved_by, created_at, updated_at,
				approval_status, is_official_version, draft_version_no,
				included_in_reporting, report_mode, reviewed_by, reviewed_at,
				rejected_reason, submitted_at, discount_rate_type, discount_rate_version,
				discount_rate_missing, discount_rate_source, discount_rate_policy_id,
				discount_rate_confirmed_by, discount_rate_confirmed_at,
				ai_extracted_discount_rate, ai_suggested_rate_policies,
				ai_confidence_score, source_reference_locator, approved_at
			FROM lease_contracts
			WHERE approval_status = ANY($1)
			ORDER BY created_at DESC
		`
		args = append(args, statuses)
	}
	
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list contracts by status: %w", err)
	}
	defer rows.Close()
	
	var contracts []*Contract
	for rows.Next() {
		c := &Contract{}
		err := rows.Scan(
			&c.ID, &c.ContractNumber, &c.ContractName,
			&c.LegalEntityID, &c.StoreID, &c.LandlordID,
			&c.AssetCategory, &c.PropertyCategory, &c.Currency,
			&c.SigningDate, &c.CommencementDate, &c.LeaseStartDate,
			&c.LeaseEndDate, &c.Status,
			&c.CreatedBy, &c.ApprovedBy, &c.CreatedAt, &c.UpdatedAt,
			&c.ApprovalStatus, &c.IsOfficialVersion, &c.DraftVersionNo,
			&c.IncludedInReporting, &c.ReportMode, &c.ReviewedBy, &c.ReviewedAt,
			&c.RejectedReason, &c.SubmittedAt, &c.DiscountRateType, &c.DiscountRateVersion,
			&c.DiscountRateMissing, &c.DiscountRateSource, &c.DiscountRatePolicyID,
			&c.DiscountRateConfirmedBy, &c.DiscountRateConfirmedAt,
			&c.AIExtractedDiscountRate, &c.AISuggestedRatePolicies,
			&c.AIConfidenceScore, &c.SourceReferenceLocator, &c.ApprovedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan contract: %w", err)
		}
		contracts = append(contracts, c)
	}
	
	return contracts, nil
}

func (r *ContractRepository) GetByID(ctx context.Context, id string, legalEntityID string) (*Contract, error) {
	var query string
	var args []interface{}
	
	if legalEntityID != "" {
		query = `
			SELECT id, contract_number, contract_name, legal_entity_id, store_id, landlord_id,
				asset_category, property_category, currency, signing_date,
				commencement_date, lease_start_date, lease_end_date, status,
				created_by, approved_by, created_at, updated_at,
				approval_status, is_official_version, draft_version_no,
				included_in_reporting, report_mode, reviewed_by, reviewed_at,
				rejected_reason, submitted_at, discount_rate_type, discount_rate_version,
				discount_rate_missing, discount_rate_source, discount_rate_policy_id,
				discount_rate_confirmed_by, discount_rate_confirmed_at,
				ai_extracted_discount_rate, ai_suggested_rate_policies,
				ai_confidence_score, source_reference_locator, approved_at
			FROM lease_contracts WHERE id = $1 AND legal_entity_id = $2
		`
		args = append(args, id, legalEntityID)
	} else {
		query = `
			SELECT id, contract_number, contract_name, legal_entity_id, store_id, landlord_id,
				asset_category, property_category, currency, signing_date,
				commencement_date, lease_start_date, lease_end_date, status,
				created_by, approved_by, created_at, updated_at,
				approval_status, is_official_version, draft_version_no,
				included_in_reporting, report_mode, reviewed_by, reviewed_at,
				rejected_reason, submitted_at, discount_rate_type, discount_rate_version,
				discount_rate_missing, discount_rate_source, discount_rate_policy_id,
				discount_rate_confirmed_by, discount_rate_confirmed_at,
				ai_extracted_discount_rate, ai_suggested_rate_policies,
				ai_confidence_score, source_reference_locator, approved_at
			FROM lease_contracts WHERE id = $1
		`
		args = append(args, id)
	}
	
	c := &Contract{}
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&c.ID, &c.ContractNumber, &c.ContractName,
		&c.LegalEntityID, &c.StoreID, &c.LandlordID,
		&c.AssetCategory, &c.PropertyCategory, &c.Currency,
		&c.SigningDate, &c.CommencementDate, &c.LeaseStartDate,
		&c.LeaseEndDate, &c.Status,
		&c.CreatedBy, &c.ApprovedBy, &c.CreatedAt, &c.UpdatedAt,
		&c.ApprovalStatus, &c.IsOfficialVersion, &c.DraftVersionNo,
		&c.IncludedInReporting, &c.ReportMode, &c.ReviewedBy, &c.ReviewedAt,
		&c.RejectedReason, &c.SubmittedAt, &c.DiscountRateType, &c.DiscountRateVersion,
		&c.DiscountRateMissing, &c.DiscountRateSource, &c.DiscountRatePolicyID,
		&c.DiscountRateConfirmedBy, &c.DiscountRateConfirmedAt,
		&c.AIExtractedDiscountRate, &c.AISuggestedRatePolicies,
		&c.AIConfidenceScore, &c.SourceReferenceLocator, &c.ApprovedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get contract: %w", err)
	}
	
	return c, nil
}
