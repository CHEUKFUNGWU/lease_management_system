package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CriticalDate struct {
	ID                     string      `json:"id"`
	ContractID             string      `json:"contract_id"`
	DateType               string      `json:"date_type"`
	TargetDate             time.Time   `json:"target_date"`
	ReminderDays           int         `json:"reminder_days"`
	ResponsibleUserID      *string     `json:"responsible_user_id"`
	Status                 string      `json:"status"`
	Title                  string      `json:"title"`
	Description            *string     `json:"description"`
	Source                 string      `json:"source"`
	SourceReferenceLocator interface{} `json:"source_reference_locator"`
	CompletedAt            *time.Time  `json:"completed_at"`
	CompletedBy            *string     `json:"completed_by"`
	CreatedBy              *string     `json:"created_by"`
	CreatedAt              time.Time   `json:"created_at"`
	UpdatedAt              time.Time   `json:"updated_at"`
}

type LeaseDocument struct {
	ID              string    `json:"id"`
	ContractID      string    `json:"contract_id"`
	DocumentType    string    `json:"document_type"`
	FileName        string    `json:"file_name"`
	FileType        *string   `json:"file_type"`
	FileSize        *int64    `json:"file_size"`
	MinIOBucket     *string   `json:"minio_bucket"`
	MinIOObjectKey  *string   `json:"minio_object_key"`
	DocumentVersion *string   `json:"document_version"`
	FileHash        *string   `json:"file_hash"`
	SourcePage      *int      `json:"source_page"`
	Notes           *string   `json:"notes"`
	UploadedBy      *string   `json:"uploaded_by"`
	UploadedAt      time.Time `json:"uploaded_at"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type LeaseObligation struct {
	ID                     string      `json:"id"`
	ContractID             string      `json:"contract_id"`
	ObligationType         string      `json:"obligation_type"`
	ResponsibleParty       string      `json:"responsible_party"`
	Title                  string      `json:"title"`
	Description            *string     `json:"description"`
	StructuredValue        interface{} `json:"structured_value"`
	SourceClause           *string     `json:"source_clause"`
	SourcePage             *int        `json:"source_page"`
	SourceReferenceLocator interface{} `json:"source_reference_locator"`
	Status                 string      `json:"status"`
	CreatedBy              *string     `json:"created_by"`
	CreatedAt              time.Time   `json:"created_at"`
	UpdatedAt              time.Time   `json:"updated_at"`
}

type LeaseAdminRepository struct {
	db *pgxpool.Pool
}

func NewLeaseAdminRepository(db *pgxpool.Pool) *LeaseAdminRepository {
	return &LeaseAdminRepository{db: db}
}

func (r *LeaseAdminRepository) CreateCriticalDate(ctx context.Context, item *CriticalDate) (*CriticalDate, error) {
	item.ID = uuid.New().String()
	if item.Status == "" {
		item.Status = "open"
	}
	if item.Source == "" {
		item.Source = "manual"
	}
	if item.ReminderDays == 0 {
		item.ReminderDays = 30
	}
	item.CreatedAt = time.Now()
	item.UpdatedAt = item.CreatedAt

	query := `
		INSERT INTO critical_dates (
			id, contract_id, date_type, target_date, reminder_days,
			responsible_user_id, status, title, description, source,
			source_reference_locator, created_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	_, err := r.db.Exec(ctx, query,
		item.ID, item.ContractID, item.DateType, item.TargetDate, item.ReminderDays,
		item.ResponsibleUserID, item.Status, item.Title, item.Description, item.Source,
		item.SourceReferenceLocator, item.CreatedBy, item.CreatedAt, item.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create critical date: %w", err)
	}
	return item, nil
}

func (r *LeaseAdminRepository) ListCriticalDates(ctx context.Context, contractID string) ([]*CriticalDate, error) {
	query := `
		SELECT id, contract_id, date_type, target_date, reminder_days,
			responsible_user_id, status, title, description, source,
			source_reference_locator, completed_at, completed_by,
			created_by, created_at, updated_at
		FROM critical_dates
		WHERE contract_id = $1
		ORDER BY target_date ASC, created_at ASC
	`
	rows, err := r.db.Query(ctx, query, contractID)
	if err != nil {
		return nil, fmt.Errorf("failed to list critical dates: %w", err)
	}
	defer rows.Close()

	var items []*CriticalDate
	for rows.Next() {
		item := &CriticalDate{}
		err := rows.Scan(
			&item.ID, &item.ContractID, &item.DateType, &item.TargetDate, &item.ReminderDays,
			&item.ResponsibleUserID, &item.Status, &item.Title, &item.Description, &item.Source,
			&item.SourceReferenceLocator, &item.CompletedAt, &item.CompletedBy,
			&item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan critical date: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *LeaseAdminRepository) ListUpcomingCriticalDates(ctx context.Context, legalEntityID string, days int, limit int) ([]*CriticalDate, error) {
	if days <= 0 {
		days = 90
	}
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT cd.id, cd.contract_id, cd.date_type, cd.target_date, cd.reminder_days,
			cd.responsible_user_id, cd.status, cd.title, cd.description, cd.source,
			cd.source_reference_locator, cd.completed_at, cd.completed_by,
			cd.created_by, cd.created_at, cd.updated_at
		FROM critical_dates cd
		JOIN lease_contracts lc ON lc.id = cd.contract_id
		WHERE cd.status IN ('open', 'snoozed')
			AND cd.target_date <= CURRENT_DATE + ($1::int * INTERVAL '1 day')
	`
	args := []interface{}{days}
	argIdx := 2
	if legalEntityID != "" {
		query += fmt.Sprintf(" AND lc.legal_entity_id = $%d", argIdx)
		args = append(args, legalEntityID)
		argIdx++
	}
	query += fmt.Sprintf(" ORDER BY cd.target_date ASC, cd.created_at ASC LIMIT $%d", argIdx)
	args = append(args, limit)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list upcoming critical dates: %w", err)
	}
	defer rows.Close()

	var items []*CriticalDate
	for rows.Next() {
		item := &CriticalDate{}
		err := rows.Scan(
			&item.ID, &item.ContractID, &item.DateType, &item.TargetDate, &item.ReminderDays,
			&item.ResponsibleUserID, &item.Status, &item.Title, &item.Description, &item.Source,
			&item.SourceReferenceLocator, &item.CompletedAt, &item.CompletedBy,
			&item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan upcoming critical date: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *LeaseAdminRepository) UpdateCriticalDateStatus(ctx context.Context, id, status, userID string) error {
	now := time.Now()
	query := `
		UPDATE critical_dates
		SET status = $2,
			completed_at = CASE WHEN $2 = 'completed' THEN $3 ELSE completed_at END,
			completed_by = CASE WHEN $2 = 'completed' THEN $4 ELSE completed_by END,
			updated_at = $3
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, id, status, now, userID)
	return err
}

func (r *LeaseAdminRepository) CreateDocument(ctx context.Context, doc *LeaseDocument) (*LeaseDocument, error) {
	doc.ID = uuid.New().String()
	if doc.DocumentType == "" {
		doc.DocumentType = "main_contract"
	}
	doc.UploadedAt = time.Now()
	doc.CreatedAt = doc.UploadedAt
	doc.UpdatedAt = doc.UploadedAt

	query := `
		INSERT INTO lease_documents (
			id, contract_id, document_type, file_name, file_type, file_size,
			minio_bucket, minio_object_key, document_version, file_hash,
			source_page, notes, uploaded_by, uploaded_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`
	_, err := r.db.Exec(ctx, query,
		doc.ID, doc.ContractID, doc.DocumentType, doc.FileName, doc.FileType, doc.FileSize,
		doc.MinIOBucket, doc.MinIOObjectKey, doc.DocumentVersion, doc.FileHash,
		doc.SourcePage, doc.Notes, doc.UploadedBy, doc.UploadedAt, doc.CreatedAt, doc.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create lease document: %w", err)
	}
	return doc, nil
}

func (r *LeaseAdminRepository) ListDocuments(ctx context.Context, contractID string) ([]*LeaseDocument, error) {
	query := `
		SELECT id, contract_id, document_type, file_name, file_type, file_size,
			minio_bucket, minio_object_key, document_version, file_hash,
			source_page, notes, uploaded_by, uploaded_at, created_at, updated_at
		FROM lease_documents
		WHERE contract_id = $1
		ORDER BY uploaded_at DESC
	`
	rows, err := r.db.Query(ctx, query, contractID)
	if err != nil {
		return nil, fmt.Errorf("failed to list lease documents: %w", err)
	}
	defer rows.Close()

	var docs []*LeaseDocument
	for rows.Next() {
		doc := &LeaseDocument{}
		err := rows.Scan(
			&doc.ID, &doc.ContractID, &doc.DocumentType, &doc.FileName, &doc.FileType, &doc.FileSize,
			&doc.MinIOBucket, &doc.MinIOObjectKey, &doc.DocumentVersion, &doc.FileHash,
			&doc.SourcePage, &doc.Notes, &doc.UploadedBy, &doc.UploadedAt, &doc.CreatedAt, &doc.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan lease document: %w", err)
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func (r *LeaseAdminRepository) CreateObligation(ctx context.Context, item *LeaseObligation) (*LeaseObligation, error) {
	item.ID = uuid.New().String()
	if item.ObligationType == "" {
		item.ObligationType = "other"
	}
	if item.ResponsibleParty == "" {
		item.ResponsibleParty = "lessee"
	}
	if item.Status == "" {
		item.Status = "active"
	}
	item.CreatedAt = time.Now()
	item.UpdatedAt = item.CreatedAt

	query := `
		INSERT INTO lease_obligations (
			id, contract_id, obligation_type, responsible_party, title,
			description, structured_value, source_clause, source_page,
			source_reference_locator, status, created_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	_, err := r.db.Exec(ctx, query,
		item.ID, item.ContractID, item.ObligationType, item.ResponsibleParty, item.Title,
		item.Description, item.StructuredValue, item.SourceClause, item.SourcePage,
		item.SourceReferenceLocator, item.Status, item.CreatedBy, item.CreatedAt, item.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create lease obligation: %w", err)
	}
	return item, nil
}

func (r *LeaseAdminRepository) ListObligations(ctx context.Context, contractID string) ([]*LeaseObligation, error) {
	query := `
		SELECT id, contract_id, obligation_type, responsible_party, title,
			description, structured_value, source_clause, source_page,
			source_reference_locator, status, created_by, created_at, updated_at
		FROM lease_obligations
		WHERE contract_id = $1
		ORDER BY status ASC, obligation_type ASC, created_at DESC
	`
	rows, err := r.db.Query(ctx, query, contractID)
	if err != nil {
		return nil, fmt.Errorf("failed to list lease obligations: %w", err)
	}
	defer rows.Close()

	var items []*LeaseObligation
	for rows.Next() {
		item := &LeaseObligation{}
		err := rows.Scan(
			&item.ID, &item.ContractID, &item.ObligationType, &item.ResponsibleParty, &item.Title,
			&item.Description, &item.StructuredValue, &item.SourceClause, &item.SourcePage,
			&item.SourceReferenceLocator, &item.Status, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan lease obligation: %w", err)
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *LeaseAdminRepository) UpdateObligationStatus(ctx context.Context, id, status string) error {
	query := `
		UPDATE lease_obligations
		SET status = $2, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.Exec(ctx, query, id, status)
	return err
}
