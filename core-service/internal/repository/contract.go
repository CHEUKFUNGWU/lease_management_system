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
	ID                           string      `json:"id"`
	ContractNumber               string      `json:"contract_number"`
	ContractName                 string      `json:"contract_name"`
	LegalEntityID                *string     `json:"legal_entity_id"`
	StoreID                      *string     `json:"store_id"`
	LandlordID                   *string     `json:"landlord_id"`
	LesseeName                   string      `json:"lessee_name"`
	LessorName                   string      `json:"lessor_name"`
	StoreName                    string      `json:"store_name"`
	StoreAddress                 string      `json:"store_address"`
	Tags                         string      `json:"tags"`
	AssetType                    string      `json:"asset_type"`
	AssetCategory                *string     `json:"asset_category"`
	PropertyCategory             *string     `json:"property_category"`
	Currency                     string      `json:"currency"`
	SigningDate                  *time.Time  `json:"signing_date"`
	CommencementDate             time.Time   `json:"commencement_date"`
	LeaseStartDate               time.Time   `json:"lease_start_date"`
	LeaseEndDate                 time.Time   `json:"lease_end_date"`
	OriginalNonCancellablePeriod *string     `json:"original_non_cancellable_period"`
	RenewalOptionDescription     *string     `json:"renewal_option_description"`
	TerminationOptionDescription *string     `json:"termination_option_description"`
	RenewalAssessment            *bool       `json:"renewal_assessment"`
	TerminationAssessment        *bool       `json:"termination_assessment"`
	DiscountRateType             *string     `json:"discount_rate_type"`
	DiscountRateVersion          *string     `json:"discount_rate_version"`
	DiscountRateValue            *float64    `json:"discount_rate_value"`
	DiscountRateMissing          bool        `json:"discount_rate_missing"`
	DiscountRateSource           *string     `json:"discount_rate_source"`
	DiscountRatePolicyID         *string     `json:"discount_rate_policy_id"`
	DiscountRateConfirmedBy      *string     `json:"discount_rate_confirmed_by"`
	DiscountRateConfirmedAt      *time.Time  `json:"discount_rate_confirmed_at"`
	AIExtractedDiscountRate      *string     `json:"ai_extracted_discount_rate"`
	AISuggestedRatePolicies      interface{} `json:"ai_suggested_rate_policies"`
	LeaseScope                   string      `json:"lease_scope"`
	ExemptionReason              *string     `json:"exemption_reason"`
	ScopeClassifiedBy            *string     `json:"scope_classified_by"`
	ScopeClassifiedAt            *time.Time  `json:"scope_classified_at"`
	ScopeSource                  *string     `json:"scope_source"`
	ScopeConfidence              *float64    `json:"scope_confidence"`
	Status                       string      `json:"status"`
	ApprovalStatus               string      `json:"approval_status"`
	IsOfficialVersion            bool        `json:"is_official_version"`
	DraftVersionNo               int         `json:"draft_version_no"`
	IncludedInReporting          bool        `json:"included_in_reporting"`
	ReportMode                   string      `json:"report_mode"`
	ReviewedBy                   *string     `json:"reviewed_by"`
	ReviewedAt                   *time.Time  `json:"reviewed_at"`
	ApprovedBy                   *string     `json:"approved_by"`
	ApprovedAt                   *time.Time  `json:"approved_at"`
	RejectedReason               *string     `json:"rejected_reason"`
	SubmittedAt                  *time.Time  `json:"submitted_at"`
	AIConfidenceScore            *float64    `json:"ai_confidence_score"`
	SourceReferenceLocator       interface{} `json:"source_reference_locator"`
	CreatedBy                    *string     `json:"created_by"`
	CreatedAt                    time.Time   `json:"created_at"`
	UpdatedAt                    time.Time   `json:"updated_at"`
}

type ContractRepository struct {
	db *pgxpool.Pool
}

func NewContractRepository(db *pgxpool.Pool) *ContractRepository {
	return &ContractRepository{db: db}
}

func autoCode(prefix string) string {
	return fmt.Sprintf("%s-%s", prefix, uuid.New().String()[:8])
}

func (r *ContractRepository) ResolveLegalEntityID(ctx context.Context, nameOrCode, currency string) (*string, error) {
	var id string
	if nameOrCode != "" {
		err := r.db.QueryRow(ctx, `
			SELECT id
			FROM legal_entities
			WHERE is_active = true AND (name = $1 OR code = $1)
			LIMIT 1
		`, nameOrCode).Scan(&id)
		if err == nil {
			return &id, nil
		}
		if err != pgx.ErrNoRows {
			return nil, fmt.Errorf("failed to resolve legal entity: %w", err)
		}

		if currency == "" {
			currency = "CNY"
		}
		err = r.db.QueryRow(ctx, `
			INSERT INTO legal_entities (code, name, country, currency, is_active)
			VALUES ($1, $2, $3, $4, true)
			RETURNING id
		`, autoCode("LE"), nameOrCode, "CN", currency).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("failed to create legal entity: %w", err)
		}
		return &id, nil
	}

	err := r.db.QueryRow(ctx, `
		SELECT id
		FROM legal_entities
		WHERE is_active = true
		ORDER BY code
		LIMIT 1
	`).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve default legal entity: %w", err)
	}
	return &id, nil
}

func (r *ContractRepository) ResolveOrCreateStoreID(ctx context.Context, name, address string, legalEntityID *string) (*string, error) {
	if legalEntityID == nil || *legalEntityID == "" {
		return nil, fmt.Errorf("legal_entity_id is required before resolving store")
	}

	if name == "" {
		name = "未命名门店-" + uuid.New().String()[:8]
	}

	var id string
	err := r.db.QueryRow(ctx, `
		SELECT id
		FROM stores
		WHERE is_active = true AND name = $1 AND legal_entity_id = $2
		LIMIT 1
	`, name, *legalEntityID).Scan(&id)
	if err == nil {
		return &id, nil
	}
	if err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to resolve store: %w", err)
	}

	err = r.db.QueryRow(ctx, `
		INSERT INTO stores (code, name, legal_entity_id, address, is_active)
		VALUES ($1, $2, $3, $4, true)
		RETURNING id
	`, autoCode("ST"), name, *legalEntityID, address).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to create store: %w", err)
	}
	return &id, nil
}

func (r *ContractRepository) ResolveOrCreateLandlordID(ctx context.Context, name string) (*string, error) {
	if name == "" {
		name = "未命名出租方-" + uuid.New().String()[:8]
	}

	var id string
	err := r.db.QueryRow(ctx, `
		SELECT id
		FROM landlords
		WHERE is_active = true AND name = $1
		LIMIT 1
	`, name).Scan(&id)
	if err == nil {
		return &id, nil
	}
	if err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to resolve landlord: %w", err)
	}

	err = r.db.QueryRow(ctx, `
		INSERT INTO landlords (code, name, is_active)
		VALUES ($1, $2, true)
		RETURNING id
	`, autoCode("LL"), name).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("failed to create landlord: %w", err)
	}
	return &id, nil
}

func (r *ContractRepository) Create(ctx context.Context, contract *Contract) (*Contract, error) {
	contract.ID = uuid.New().String()
	contract.Status = "draft"
	if contract.LeaseScope == "" {
		contract.LeaseScope = "in_scope"
	}
	if contract.AssetType == "" {
		contract.AssetType = "real_estate"
	}
	contract.CreatedAt = time.Now()
	contract.UpdatedAt = time.Now()

	query := `
		INSERT INTO lease_contracts (
			id, contract_number, contract_name, legal_entity_id, store_id, landlord_id,
			lessee_name, lessor_name, store_name, store_address, tags,
			asset_type, asset_category, property_category, currency, signing_date,
			commencement_date, lease_start_date, lease_end_date,
			original_non_cancellable_period, renewal_option_description,
			termination_option_description, renewal_assessment,
			termination_assessment, discount_rate_type, discount_rate_version, discount_rate_value,
			status, created_by, approved_by, created_at, updated_at,
			approval_status, is_official_version, draft_version_no,
			included_in_reporting, report_mode, reviewed_by, reviewed_at,
			rejected_reason, submitted_at, discount_rate_missing,
			discount_rate_source, discount_rate_policy_id,
			discount_rate_confirmed_by, discount_rate_confirmed_at,
			ai_extracted_discount_rate, ai_suggested_rate_policies,
			ai_confidence_score, source_reference_locator, approved_at,
			lease_scope, exemption_reason, scope_classified_by, scope_classified_at,
			scope_source, scope_confidence
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36, $37, $38, $39, $40, $41, $42, $43, $44, $45, $46, $47, $48, $49, $50, $51, $52, $53, $54, $55, $56, $57)
		RETURNING approval_status, is_official_version, draft_version_no,
			included_in_reporting, report_mode
	`

	err := r.db.QueryRow(ctx, query,
		contract.ID, contract.ContractNumber, contract.ContractName,
		contract.LegalEntityID, contract.StoreID, contract.LandlordID,
		contract.LesseeName, contract.LessorName, contract.StoreName,
		contract.StoreAddress, contract.Tags,
		contract.AssetType, contract.AssetCategory, contract.PropertyCategory, contract.Currency,
		contract.SigningDate, contract.CommencementDate, contract.LeaseStartDate,
		contract.LeaseEndDate, contract.OriginalNonCancellablePeriod,
		contract.RenewalOptionDescription, contract.TerminationOptionDescription,
		contract.RenewalAssessment, contract.TerminationAssessment,
		contract.DiscountRateType, contract.DiscountRateVersion, contract.DiscountRateValue,
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
		contract.LeaseScope, contract.ExemptionReason, contract.ScopeClassifiedBy,
		contract.ScopeClassifiedAt, contract.ScopeSource, contract.ScopeConfidence,
	).Scan(
		&contract.ApprovalStatus, &contract.IsOfficialVersion, &contract.DraftVersionNo,
		&contract.IncludedInReporting, &contract.ReportMode,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create contract: %w", err)
	}

	return contract, nil
}

// ListContractsFilter holds optional filters for listing contracts.
type ListContractsFilter struct {
	Search    string // search in contract_number, contract_name, lessee_name, lessor_name, store_name
	Status    string // filter by exact approval_status
	SortBy    string // sort column: contract_number, commencement_date, lease_end_date, approval_status, created_at
	SortOrder string // asc or desc
}

// allowedSortColumns defines the whitelist of columns that can be used for sorting.
var allowedSortColumns = map[string]string{
	"contract_number":   "contract_number",
	"commencement_date": "commencement_date",
	"lease_end_date":    "lease_end_date",
	"approval_status":   "approval_status",
	"created_at":        "created_at",
}

// List returns contracts with optional filters, search, and sorting.
// Empty filter = same behavior as GetAll.
func (r *ContractRepository) List(ctx context.Context, legalEntityID string, filter ListContractsFilter) ([]*Contract, error) {
	// Build SELECT column list (reusable)
	sel := `
		SELECT id, contract_number, contract_name, legal_entity_id, store_id, landlord_id,
			COALESCE(lessee_name, '') as lessee_name, COALESCE(lessor_name, '') as lessor_name, COALESCE(store_name, '') as store_name, COALESCE(store_address, '') as store_address, COALESCE(tags, '') as tags,
			COALESCE(asset_type, 'real_estate') AS asset_type, asset_category, property_category, currency, signing_date,
			commencement_date, lease_start_date, lease_end_date, status,
			created_by, approved_by, created_at, updated_at,
			approval_status, is_official_version, draft_version_no,
			included_in_reporting, report_mode, reviewed_by, reviewed_at,
			rejected_reason, submitted_at, discount_rate_type, discount_rate_version,
			discount_rate_value, discount_rate_missing, discount_rate_source, discount_rate_policy_id,
			discount_rate_confirmed_by, discount_rate_confirmed_at,
			ai_extracted_discount_rate, ai_suggested_rate_policies,
			ai_confidence_score, source_reference_locator, approved_at,
			COALESCE(lease_scope, 'in_scope') AS lease_scope, exemption_reason,
			scope_classified_by, scope_classified_at, scope_source, scope_confidence
		FROM lease_contracts
	`

	var conditions []string
	var args []interface{}
	argIdx := 1

	// Tenant filter
	if legalEntityID != "" {
		conditions = append(conditions, fmt.Sprintf("legal_entity_id = $%d", argIdx))
		args = append(args, legalEntityID)
		argIdx++
	}

	// Status filter
	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("approval_status = $%d", argIdx))
		args = append(args, filter.Status)
		argIdx++
	}

	// Search filter — ILIKE across multiple text fields
	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		searchCond := fmt.Sprintf(
			"(contract_number ILIKE $%d OR contract_name ILIKE $%d OR lessee_name ILIKE $%d OR lessor_name ILIKE $%d OR store_name ILIKE $%d)",
			argIdx, argIdx+1, argIdx+2, argIdx+3, argIdx+4,
		)
		conditions = append(conditions, searchCond)
		args = append(args, searchPattern, searchPattern, searchPattern, searchPattern, searchPattern)
		argIdx += 5
	}

	// Build WHERE clause
	query := sel
	if len(conditions) > 0 {
		query += " WHERE " + fmt.Sprintf("%s", conditions[0])
		for i := 1; i < len(conditions); i++ {
			query += " AND " + conditions[i]
		}
	}

	// Build ORDER BY clause (with whitelist validation to prevent injection)
	sortCol, ok := allowedSortColumns[filter.SortBy]
	if !ok {
		sortCol = "created_at" // default sort
	}
	sortOrder := "DESC"
	if filter.SortOrder == "asc" || filter.SortOrder == "ASC" {
		sortOrder = "ASC"
	}
	query += fmt.Sprintf(" ORDER BY %s %s", sortCol, sortOrder)

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
			&c.LesseeName, &c.LessorName, &c.StoreName, &c.StoreAddress, &c.Tags,
			&c.AssetType, &c.AssetCategory, &c.PropertyCategory, &c.Currency,
			&c.SigningDate, &c.CommencementDate, &c.LeaseStartDate,
			&c.LeaseEndDate, &c.Status,
			&c.CreatedBy, &c.ApprovedBy, &c.CreatedAt, &c.UpdatedAt,
			&c.ApprovalStatus, &c.IsOfficialVersion, &c.DraftVersionNo,
			&c.IncludedInReporting, &c.ReportMode, &c.ReviewedBy, &c.ReviewedAt,
			&c.RejectedReason, &c.SubmittedAt, &c.DiscountRateType, &c.DiscountRateVersion,
			&c.DiscountRateValue, &c.DiscountRateMissing, &c.DiscountRateSource, &c.DiscountRatePolicyID,
			&c.DiscountRateConfirmedBy, &c.DiscountRateConfirmedAt,
			&c.AIExtractedDiscountRate, &c.AISuggestedRatePolicies,
			&c.AIConfidenceScore, &c.SourceReferenceLocator, &c.ApprovedAt,
			&c.LeaseScope, &c.ExemptionReason, &c.ScopeClassifiedBy,
			&c.ScopeClassifiedAt, &c.ScopeSource, &c.ScopeConfidence,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan contract: %w", err)
		}
		contracts = append(contracts, c)
	}

	return contracts, nil
}

func (r *ContractRepository) GetAll(ctx context.Context, legalEntityID string) ([]*Contract, error) {
	return r.List(ctx, legalEntityID, ListContractsFilter{})
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
				COALESCE(lessee_name, '') as lessee_name, COALESCE(lessor_name, '') as lessor_name, COALESCE(store_name, '') as store_name, COALESCE(store_address, '') as store_address, COALESCE(tags, '') as tags,
				COALESCE(asset_type, 'real_estate') AS asset_type, asset_category, property_category, currency, signing_date,
				commencement_date, lease_start_date, lease_end_date, status,
				created_by, approved_by, created_at, updated_at,
				approval_status, is_official_version, draft_version_no,
				included_in_reporting, report_mode, reviewed_by, reviewed_at,
				rejected_reason, submitted_at, discount_rate_type, discount_rate_version,
				discount_rate_value, discount_rate_missing, discount_rate_source, discount_rate_policy_id,
				discount_rate_confirmed_by, discount_rate_confirmed_at,
				ai_extracted_discount_rate, ai_suggested_rate_policies,
				ai_confidence_score, source_reference_locator, approved_at,
				COALESCE(lease_scope, 'in_scope') AS lease_scope, exemption_reason,
				scope_classified_by, scope_classified_at, scope_source, scope_confidence
			FROM lease_contracts
			WHERE approval_status = ANY($1) AND legal_entity_id = $2
			ORDER BY created_at DESC
		`
		args = append(args, statuses, legalEntityID)
	} else {
		query = `
			SELECT id, contract_number, contract_name, legal_entity_id, store_id, landlord_id,
				COALESCE(lessee_name, '') as lessee_name, COALESCE(lessor_name, '') as lessor_name, COALESCE(store_name, '') as store_name, COALESCE(store_address, '') as store_address, COALESCE(tags, '') as tags,
				COALESCE(asset_type, 'real_estate') AS asset_type, asset_category, property_category, currency, signing_date,
				commencement_date, lease_start_date, lease_end_date, status,
				created_by, approved_by, created_at, updated_at,
				approval_status, is_official_version, draft_version_no,
				included_in_reporting, report_mode, reviewed_by, reviewed_at,
				rejected_reason, submitted_at, discount_rate_type, discount_rate_version,
				discount_rate_value, discount_rate_missing, discount_rate_source, discount_rate_policy_id,
				discount_rate_confirmed_by, discount_rate_confirmed_at,
				ai_extracted_discount_rate, ai_suggested_rate_policies,
				ai_confidence_score, source_reference_locator, approved_at,
				COALESCE(lease_scope, 'in_scope') AS lease_scope, exemption_reason,
				scope_classified_by, scope_classified_at, scope_source, scope_confidence
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
			&c.LesseeName, &c.LessorName, &c.StoreName, &c.StoreAddress, &c.Tags,
			&c.AssetType, &c.AssetCategory, &c.PropertyCategory, &c.Currency,
			&c.SigningDate, &c.CommencementDate, &c.LeaseStartDate,
			&c.LeaseEndDate, &c.Status,
			&c.CreatedBy, &c.ApprovedBy, &c.CreatedAt, &c.UpdatedAt,
			&c.ApprovalStatus, &c.IsOfficialVersion, &c.DraftVersionNo,
			&c.IncludedInReporting, &c.ReportMode, &c.ReviewedBy, &c.ReviewedAt,
			&c.RejectedReason, &c.SubmittedAt, &c.DiscountRateType, &c.DiscountRateVersion,
			&c.DiscountRateValue, &c.DiscountRateMissing, &c.DiscountRateSource, &c.DiscountRatePolicyID,
			&c.DiscountRateConfirmedBy, &c.DiscountRateConfirmedAt,
			&c.AIExtractedDiscountRate, &c.AISuggestedRatePolicies,
			&c.AIConfidenceScore, &c.SourceReferenceLocator, &c.ApprovedAt,
			&c.LeaseScope, &c.ExemptionReason, &c.ScopeClassifiedBy,
			&c.ScopeClassifiedAt, &c.ScopeSource, &c.ScopeConfidence,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan contract: %w", err)
		}
		contracts = append(contracts, c)
	}

	return contracts, nil
}

// Update updates a contract's basic editable fields.
// Only allowed if approval_status is 'draft' or 'rejected'.
// updatedBy is the user ID performing the update.
func (r *ContractRepository) Update(ctx context.Context, contract *Contract, legalEntityID string, updatedBy string) error {
	if contract.LeaseScope == "" {
		contract.LeaseScope = "in_scope"
	}
	if contract.AssetType == "" {
		contract.AssetType = "real_estate"
	}

	// Verify contract exists and is editable
	var currentStatus string
	checkQuery := `SELECT approval_status FROM lease_contracts WHERE id = $1`
	checkArgs := []interface{}{contract.ID}
	if legalEntityID != "" {
		checkQuery += ` AND legal_entity_id = $2`
		checkArgs = append(checkArgs, legalEntityID)
	}
	err := r.db.QueryRow(ctx, checkQuery, checkArgs...).Scan(&currentStatus)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("contract not found")
		}
		return fmt.Errorf("failed to check contract: %w", err)
	}
	if currentStatus != "draft" && currentStatus != "rejected" {
		return fmt.Errorf("contract cannot be edited in '%s' status, only draft or rejected", currentStatus)
	}

	query := `
		UPDATE lease_contracts SET
			contract_number = $1,
			contract_name = $2,
			legal_entity_id = $3,
			store_id = $4,
			landlord_id = $5,
			lessee_name = $6,
			lessor_name = $7,
			store_name = $8,
			store_address = $9,
			tags = $10,
			asset_type = $11,
			currency = $12,
			signing_date = $13,
			commencement_date = $14,
			lease_start_date = $15,
			lease_end_date = $16,
			discount_rate_type = $17,
			discount_rate_version = $18,
			discount_rate_value = $19,
			discount_rate_missing = $20,
			discount_rate_source = $21,
			discount_rate_policy_id = $22,
			lease_scope = $23,
			exemption_reason = $24,
			scope_classified_by = $25,
			scope_classified_at = $26,
			scope_source = $27,
			scope_confidence = $28,
			status = $29,
			updated_by = $30,
			updated_at = NOW()
		WHERE id = $31`

	args := []interface{}{
		contract.ContractNumber,
		contract.ContractName,
		contract.LegalEntityID,
		contract.StoreID,
		contract.LandlordID,
		contract.LesseeName,
		contract.LessorName,
		contract.StoreName,
		contract.StoreAddress,
		contract.Tags,
		contract.AssetType,
		contract.Currency,
		contract.SigningDate,
		contract.CommencementDate,
		contract.LeaseStartDate,
		contract.LeaseEndDate,
		contract.DiscountRateType,
		contract.DiscountRateVersion,
		contract.DiscountRateValue,
		contract.DiscountRateMissing,
		contract.DiscountRateSource,
		contract.DiscountRatePolicyID,
		contract.LeaseScope,
		contract.ExemptionReason,
		contract.ScopeClassifiedBy,
		contract.ScopeClassifiedAt,
		contract.ScopeSource,
		contract.ScopeConfidence,
		contract.Status,
		updatedBy,
		contract.ID,
	}

	if legalEntityID != "" {
		query += ` AND legal_entity_id = $32`
		args = append(args, legalEntityID)
	}

	ct, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update contract: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("contract not found or not updatable")
	}
	return nil
}

func (r *ContractRepository) GetByID(ctx context.Context, id string, legalEntityID string) (*Contract, error) {
	var query string
	var args []interface{}

	if legalEntityID != "" {
		query = `
			SELECT id, contract_number, contract_name, legal_entity_id, store_id, landlord_id,
				COALESCE(lessee_name, '') as lessee_name, COALESCE(lessor_name, '') as lessor_name, COALESCE(store_name, '') as store_name, COALESCE(store_address, '') as store_address, COALESCE(tags, '') as tags,
				COALESCE(asset_type, 'real_estate') AS asset_type, asset_category, property_category, currency, signing_date,
				commencement_date, lease_start_date, lease_end_date, status,
				created_by, approved_by, created_at, updated_at,
				approval_status, is_official_version, draft_version_no,
				included_in_reporting, report_mode, reviewed_by, reviewed_at,
				rejected_reason, submitted_at, discount_rate_type, discount_rate_version,
				discount_rate_value, discount_rate_missing, discount_rate_source, discount_rate_policy_id,
				discount_rate_confirmed_by, discount_rate_confirmed_at,
				ai_extracted_discount_rate, ai_suggested_rate_policies,
				ai_confidence_score, source_reference_locator, approved_at,
				COALESCE(lease_scope, 'in_scope') AS lease_scope, exemption_reason,
				scope_classified_by, scope_classified_at, scope_source, scope_confidence
			FROM lease_contracts WHERE id = $1 AND legal_entity_id = $2
		`
		args = append(args, id, legalEntityID)
	} else {
		query = `
			SELECT id, contract_number, contract_name, legal_entity_id, store_id, landlord_id,
				COALESCE(lessee_name, '') as lessee_name, COALESCE(lessor_name, '') as lessor_name, COALESCE(store_name, '') as store_name, COALESCE(store_address, '') as store_address, COALESCE(tags, '') as tags,
				COALESCE(asset_type, 'real_estate') AS asset_type, asset_category, property_category, currency, signing_date,
				commencement_date, lease_start_date, lease_end_date, status,
				created_by, approved_by, created_at, updated_at,
				approval_status, is_official_version, draft_version_no,
				included_in_reporting, report_mode, reviewed_by, reviewed_at,
				rejected_reason, submitted_at, discount_rate_type, discount_rate_version,
				discount_rate_value, discount_rate_missing, discount_rate_source, discount_rate_policy_id,
				discount_rate_confirmed_by, discount_rate_confirmed_at,
				ai_extracted_discount_rate, ai_suggested_rate_policies,
				ai_confidence_score, source_reference_locator, approved_at,
				COALESCE(lease_scope, 'in_scope') AS lease_scope, exemption_reason,
				scope_classified_by, scope_classified_at, scope_source, scope_confidence
			FROM lease_contracts WHERE id = $1
		`
		args = append(args, id)
	}

	c := &Contract{}
	err := r.db.QueryRow(ctx, query, args...).Scan(
		&c.ID, &c.ContractNumber, &c.ContractName,
		&c.LegalEntityID, &c.StoreID, &c.LandlordID,
		&c.LesseeName, &c.LessorName, &c.StoreName, &c.StoreAddress, &c.Tags,
		&c.AssetType, &c.AssetCategory, &c.PropertyCategory, &c.Currency,
		&c.SigningDate, &c.CommencementDate, &c.LeaseStartDate,
		&c.LeaseEndDate, &c.Status,
		&c.CreatedBy, &c.ApprovedBy, &c.CreatedAt, &c.UpdatedAt,
		&c.ApprovalStatus, &c.IsOfficialVersion, &c.DraftVersionNo,
		&c.IncludedInReporting, &c.ReportMode, &c.ReviewedBy, &c.ReviewedAt,
		&c.RejectedReason, &c.SubmittedAt, &c.DiscountRateType, &c.DiscountRateVersion,
		&c.DiscountRateValue, &c.DiscountRateMissing, &c.DiscountRateSource, &c.DiscountRatePolicyID,
		&c.DiscountRateConfirmedBy, &c.DiscountRateConfirmedAt,
		&c.AIExtractedDiscountRate, &c.AISuggestedRatePolicies,
		&c.AIConfidenceScore, &c.SourceReferenceLocator, &c.ApprovedAt,
		&c.LeaseScope, &c.ExemptionReason, &c.ScopeClassifiedBy,
		&c.ScopeClassifiedAt, &c.ScopeSource, &c.ScopeConfidence,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get contract: %w", err)
	}

	return c, nil
}
