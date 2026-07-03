package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lease-management-system/core-service/internal/access"
)

type AuditLog struct {
	ID            string    `json:"id"`
	TableName     string    `json:"table_name"`
	RecordID      string    `json:"record_id"`
	LegalEntityID *string   `json:"legal_entity_id"`
	Action        string    `json:"action"`
	OldValues     *string   `json:"old_values"`
	NewValues     *string   `json:"new_values"`
	ChangedBy     *string   `json:"changed_by"`
	ChangedByName *string   `json:"changed_by_name"`
	ChangedAt     time.Time `json:"changed_at"`
	IPAddress     *string   `json:"ip_address"`
	UserAgent     *string   `json:"user_agent"`
}

type AuditRepository struct {
	db DBTX
}

func NewAuditRepository(db DBTX) *AuditRepository {
	return &AuditRepository{db: db}
}

// WithTx returns a copy of the repository whose writes run on the given
// transaction, so an audit record can be committed atomically with the change
// it describes.
func (r *AuditRepository) WithTx(tx DBTX) *AuditRepository {
	return &AuditRepository{db: tx}
}

func (r *AuditRepository) Create(ctx context.Context, log *AuditLog) error {
	log.ID = uuid.New().String()
	if log.LegalEntityID == nil {
		legalEntityID, err := r.resolveRecordLegalEntity(ctx, log.TableName, log.RecordID)
		if err != nil {
			return fmt.Errorf("failed to resolve audit legal entity: %w", err)
		}
		log.LegalEntityID = legalEntityID
	}

	// Convert changed_by to UUID if provided
	var changedBy interface{}
	if log.ChangedBy != nil && *log.ChangedBy != "" {
		changedBy = *log.ChangedBy
	}

	query := `
		INSERT INTO audit_logs (id, table_name, record_id, legal_entity_id, action, old_values, new_values, changed_by, changed_at, ip_address, user_agent)
		VALUES ($1, $2, $3::uuid, $4::uuid, $5, $6::jsonb, $7::jsonb, $8::uuid, $9, $10::inet, $11)
	`
	_, err := r.db.Exec(ctx, query,
		log.ID,
		log.TableName,
		log.RecordID,
		log.LegalEntityID,
		log.Action,
		log.OldValues,
		log.NewValues,
		changedBy,
		log.ChangedAt,
		log.IPAddress,
		log.UserAgent,
	)
	return err
}

// resolveRecordLegalEntity attributes cross-tenant administrator actions to
// the affected legal entity instead of leaving the audit row globally scoped.
func (r *AuditRepository) resolveRecordLegalEntity(ctx context.Context, tableName, recordID string) (*string, error) {
	var query string
	switch tableName {
	case "lease_contracts":
		query = `SELECT legal_entity_id::text FROM lease_contracts WHERE id = $1 AND legal_entity_id IS NOT NULL`
	case "lease_events":
		query = `SELECT c.legal_entity_id::text FROM lease_events e JOIN lease_contracts c ON c.id = e.contract_id WHERE e.id = $1 AND c.legal_entity_id IS NOT NULL`
	case "journal_entries":
		query = `SELECT c.legal_entity_id::text FROM journal_entries e JOIN lease_contracts c ON c.id = e.contract_id WHERE e.id = $1 AND c.legal_entity_id IS NOT NULL`
	case "monthly_closing_batches":
		query = `SELECT legal_entity_id::text FROM monthly_closing_batches WHERE id = $1 AND legal_entity_id IS NOT NULL`
	case "critical_dates":
		query = `SELECT c.legal_entity_id::text FROM critical_dates d JOIN lease_contracts c ON c.id = d.contract_id WHERE d.id = $1 AND c.legal_entity_id IS NOT NULL`
	case "lease_documents":
		query = `SELECT c.legal_entity_id::text FROM lease_documents d JOIN lease_contracts c ON c.id = d.contract_id WHERE d.id = $1 AND c.legal_entity_id IS NOT NULL`
	case "lease_obligations":
		query = `SELECT c.legal_entity_id::text FROM lease_obligations o JOIN lease_contracts c ON c.id = o.contract_id WHERE o.id = $1 AND c.legal_entity_id IS NOT NULL`
	default:
		return nil, nil
	}

	var legalEntityID string
	if err := r.db.QueryRow(ctx, query, recordID).Scan(&legalEntityID); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &legalEntityID, nil
}

type AuditLogFilter struct {
	TableName string
	RecordID  string
	Action    string
	ChangedBy string
	StartDate string
	EndDate   string
	Limit     int
	Offset    int
}

func appendAuditDimensionScope(scope access.Scope, conditions []string, args []interface{}, argIdx int) ([]string, []interface{}, int) {
	if len(scope.StoreIDs) == 0 && len(scope.Regions) == 0 && len(scope.Brands) == 0 {
		return conditions, args, argIdx
	}

	predicates := make([]string, 0, 3)
	if len(scope.StoreIDs) > 0 {
		predicates = append(predicates, fmt.Sprintf("c.store_id::text = ANY($%d)", argIdx))
		args = append(args, scope.StoreIDs)
		argIdx++
	}
	if len(scope.Regions) > 0 {
		predicates = append(predicates, fmt.Sprintf("s.region = ANY($%d)", argIdx))
		args = append(args, scope.Regions)
		argIdx++
	}
	if len(scope.Brands) > 0 {
		predicates = append(predicates, fmt.Sprintf("s.brand = ANY($%d)", argIdx))
		args = append(args, scope.Brands)
		argIdx++
	}
	dimensionPredicate := strings.Join(predicates, " AND ")
	contractTables := "'lease_contracts', 'lease_events', 'journal_entries', 'critical_dates', 'lease_documents', 'lease_obligations', 'monthly_closing_batches'"
	conditions = append(conditions, fmt.Sprintf(`(
		(a.table_name = 'lease_contracts' AND EXISTS (
			SELECT 1 FROM lease_contracts c LEFT JOIN stores s ON s.id = c.store_id
			WHERE c.id = a.record_id AND %s
		)) OR
		(a.table_name = 'lease_events' AND EXISTS (
			SELECT 1 FROM lease_events e JOIN lease_contracts c ON c.id = e.contract_id LEFT JOIN stores s ON s.id = c.store_id
			WHERE e.id = a.record_id AND %s
		)) OR
		(a.table_name = 'journal_entries' AND EXISTS (
			SELECT 1 FROM journal_entries e JOIN lease_contracts c ON c.id = e.contract_id LEFT JOIN stores s ON s.id = c.store_id
			WHERE e.id = a.record_id AND %s
		)) OR
		(a.table_name = 'critical_dates' AND EXISTS (
			SELECT 1 FROM critical_dates d JOIN lease_contracts c ON c.id = d.contract_id LEFT JOIN stores s ON s.id = c.store_id
			WHERE d.id = a.record_id AND %s
		)) OR
		(a.table_name = 'lease_documents' AND EXISTS (
			SELECT 1 FROM lease_documents d JOIN lease_contracts c ON c.id = d.contract_id LEFT JOIN stores s ON s.id = c.store_id
			WHERE d.id = a.record_id AND %s
		)) OR
		(a.table_name = 'lease_obligations' AND EXISTS (
			SELECT 1 FROM lease_obligations o JOIN lease_contracts c ON c.id = o.contract_id LEFT JOIN stores s ON s.id = c.store_id
			WHERE o.id = a.record_id AND %s
		)) OR
		(a.table_name = 'monthly_closing_batches' AND NOT EXISTS (
			SELECT 1 FROM journal_entries e JOIN lease_contracts c ON c.id = e.contract_id LEFT JOIN stores s ON s.id = c.store_id
			WHERE e.batch_id = a.record_id AND NOT (%s)
		)) OR
		a.table_name NOT IN (%s)
	)`, dimensionPredicate, dimensionPredicate, dimensionPredicate, dimensionPredicate,
		dimensionPredicate, dimensionPredicate, dimensionPredicate, contractTables))
	return conditions, args, argIdx
}

func (r *AuditRepository) List(ctx context.Context, filter AuditLogFilter) ([]*AuditLog, int, error) {
	conditions := []string{}
	args := []interface{}{}
	argIdx := 1
	if scope, scoped := access.ScopeFromContext(ctx); scoped && !scope.Global {
		if scope.LegalEntityID == "" {
			conditions = append(conditions, "FALSE")
		} else {
			conditions = append(conditions, fmt.Sprintf("a.legal_entity_id::text = $%d", argIdx))
			args = append(args, scope.LegalEntityID)
			argIdx++
		}
		conditions, args, argIdx = appendAuditDimensionScope(scope, conditions, args, argIdx)
	}

	if filter.TableName != "" {
		conditions = append(conditions, fmt.Sprintf("a.table_name = $%d", argIdx))
		args = append(args, filter.TableName)
		argIdx++
	}
	if filter.RecordID != "" {
		conditions = append(conditions, fmt.Sprintf("a.record_id::text = $%d", argIdx))
		args = append(args, filter.RecordID)
		argIdx++
	}
	if filter.Action != "" {
		conditions = append(conditions, fmt.Sprintf("a.action = $%d", argIdx))
		args = append(args, filter.Action)
		argIdx++
	}
	if filter.ChangedBy != "" {
		conditions = append(conditions, fmt.Sprintf("a.changed_by::text = $%d", argIdx))
		args = append(args, filter.ChangedBy)
		argIdx++
	}
	if filter.StartDate != "" {
		conditions = append(conditions, fmt.Sprintf("a.changed_at >= $%d::timestamp", argIdx))
		args = append(args, filter.StartDate)
		argIdx++
	}
	if filter.EndDate != "" {
		conditions = append(conditions, fmt.Sprintf("a.changed_at < $%d::timestamp + interval '1 day'", argIdx))
		args = append(args, filter.EndDate)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count query
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM audit_logs a %s", whereClause)
	var total int
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count audit logs: %w", err)
	}

	// Data query with user join
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	dataQuery := fmt.Sprintf(`
		SELECT a.id, a.table_name, a.record_id, a.legal_entity_id, a.action,
		       COALESCE(a.old_values::text, 'null') as old_values, 
		       COALESCE(a.new_values::text, 'null') as new_values,
		       a.changed_by, u.username as changed_by_name,
		       a.changed_at, a.ip_address::text, a.user_agent
		FROM audit_logs a
		LEFT JOIN users u ON a.changed_by = u.id
		%s
		ORDER BY a.changed_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	args = append(args, filter.Limit, filter.Offset)
	rows, err := r.db.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	var logs []*AuditLog
	for rows.Next() {
		var log AuditLog
		var changedByName *string
		var ipText, uaText *string
		if err := rows.Scan(
			&log.ID, &log.TableName, &log.RecordID, &log.LegalEntityID, &log.Action,
			&log.OldValues, &log.NewValues,
			&log.ChangedBy, &changedByName,
			&log.ChangedAt, &ipText, &uaText,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan audit log: %w", err)
		}
		log.ChangedByName = changedByName
		// Convert empty strings to nil for IP and UserAgent
		if ipText != nil && *ipText != "" {
			log.IPAddress = ipText
		}
		if uaText != nil && *uaText != "" {
			log.UserAgent = uaText
		}
		logs = append(logs, &log)
	}

	return logs, total, rows.Err()
}
