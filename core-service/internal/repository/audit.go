package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditLog struct {
	ID             string     `json:"id"`
	TableName      string     `json:"table_name"`
	RecordID       string     `json:"record_id"`
	Action         string     `json:"action"`
	OldValues      *string    `json:"old_values"`
	NewValues      *string    `json:"new_values"`
	ChangedBy      *string    `json:"changed_by"`
	ChangedByName  *string    `json:"changed_by_name"`
	ChangedAt      time.Time  `json:"changed_at"`
	IPAddress      *string    `json:"ip_address"`
	UserAgent      *string    `json:"user_agent"`
}

type AuditRepository struct {
	db *pgxpool.Pool
}

func NewAuditRepository(db *pgxpool.Pool) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) Create(ctx context.Context, log *AuditLog) error {
	log.ID = uuid.New().String()

	// Convert changed_by to UUID if provided
	var changedBy interface{}
	if log.ChangedBy != nil && *log.ChangedBy != "" {
		changedBy = *log.ChangedBy
	}

	query := `
		INSERT INTO audit_logs (id, table_name, record_id, action, old_values, new_values, changed_by, changed_at, ip_address, user_agent)
		VALUES ($1, $2, $3::uuid, $4, $5::jsonb, $6::jsonb, $7::uuid, $8, $9::inet, $10)
	`
	_, err := r.db.Exec(ctx, query,
		log.ID,
		log.TableName,
		log.RecordID,
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

func (r *AuditRepository) List(ctx context.Context, filter AuditLogFilter) ([]*AuditLog, int, error) {
	conditions := []string{}
	args := []interface{}{}
	argIdx := 1

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
		SELECT a.id, a.table_name, a.record_id, a.action, 
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
			&log.ID, &log.TableName, &log.RecordID, &log.Action,
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
