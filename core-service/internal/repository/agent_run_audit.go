package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AgentRunCheckpointAudit struct {
	ID               string    `json:"id"`
	RunID            string    `json:"run_id"`
	SessionID        string    `json:"session_id"`
	CheckpointHash   string    `json:"checkpoint_hash"`
	CheckpointSize   int       `json:"checkpoint_size_bytes"`
	SchemaVersion    *string   `json:"schema_version,omitempty"`
	CheckpointStatus string    `json:"checkpoint_status"`
	NextIndex        *int      `json:"next_index,omitempty"`
	ActorID          *string   `json:"actor_id,omitempty"`
	WorkerID         *string   `json:"worker_id,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type AgentRunTerminalAlert struct {
	ID             string          `json:"id"`
	RunID          string          `json:"run_id"`
	SessionID      string          `json:"session_id"`
	TerminalStatus string          `json:"terminal_status"`
	EventType      string          `json:"event_type"`
	ErrorMessage   *string         `json:"error_message,omitempty"`
	Payload        json.RawMessage `json:"payload"`
	Status         string          `json:"status"`
	CreatedAt      time.Time       `json:"created_at"`
	AcknowledgedAt *time.Time      `json:"acknowledged_at,omitempty"`
	AcknowledgedBy *string         `json:"acknowledged_by,omitempty"`
}

type AgentRunAuditLinkInput struct {
	BusinessTable    string
	BusinessRecordID string
	Relation         string
	ItemStatus       string
}

type AgentRunAuditLink struct {
	ID               string    `json:"id"`
	RunID            string    `json:"run_id"`
	ArtifactID       *string   `json:"artifact_id,omitempty"`
	BusinessTable    string    `json:"business_table"`
	BusinessRecordID string    `json:"business_record_id"`
	Relation         string    `json:"relation"`
	ItemStatus       *string   `json:"item_status,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// appendAgentRunEvent is the authoritative event writer. It allocates the
// sequence inside the same transaction as the insert, so a caller cannot
// reserve a sequence in one transaction and write it in another.
func appendAgentRunEvent(ctx context.Context, db DBTX, event *AIChatRunEvent) error {
	if db == nil || event == nil {
		return errors.New("agent run event and database are required")
	}
	event.RunID = strings.TrimSpace(event.RunID)
	event.SessionID = strings.TrimSpace(event.SessionID)
	event.EventType = strings.TrimSpace(event.EventType)
	if terminalStatus(event.EventType) != "" {
		event.IsTerminal = true
	}
	if event.RunID == "" || event.SessionID == "" || event.EventType == "" {
		return errors.New("agent run event requires run_id, session_id and event_type")
	}
	if _, err := db.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		"ai-run-events:"+event.RunID,
	); err != nil {
		return fmt.Errorf("lock agent run event stream: %w", err)
	}
	if err := db.QueryRow(ctx, `
		SELECT COALESCE(MAX(sequence_no), 0) + 1
		FROM ai_chat_run_events
		WHERE run_id = $1
	`, event.RunID).Scan(&event.SequenceNo); err != nil {
		return fmt.Errorf("allocate agent run event sequence: %w", err)
	}
	event.ID = uuid.NewString()
	event.CreatedAt = time.Now().UTC()
	if _, err := db.Exec(ctx, `
		INSERT INTO ai_chat_run_events (
			id, run_id, session_id, sequence_no, event_type, payload, is_terminal, created_at
		) VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8)
	`, event.ID, event.RunID, event.SessionID, event.SequenceNo, event.EventType,
		normalizeJSON(event.Payload, "{}"), event.IsTerminal, event.CreatedAt); err != nil {
		return fmt.Errorf("append agent run event: %w", err)
	}
	if err := upsertAgentRunAuditSummary(ctx, db, event); err != nil {
		return fmt.Errorf("update agent run audit summary: %w", err)
	}
	return nil
}

func checkpointMetadata(checkpoint json.RawMessage) (schemaVersion, status string, nextIndex *int) {
	var metadata map[string]any
	if json.Unmarshal(checkpoint, &metadata) != nil || metadata == nil {
		return "", "", nil
	}
	if value, ok := metadata["schema_version"].(string); ok {
		schemaVersion = strings.TrimSpace(value)
	}
	if value, ok := metadata["status"].(string); ok {
		status = strings.TrimSpace(value)
	}
	if value, ok := metadata["next_index"].(float64); ok && value >= 0 {
		parsed := int(value)
		nextIndex = &parsed
	}
	return schemaVersion, status, nextIndex
}

func checkpointHash(checkpoint json.RawMessage) string {
	digest := sha256.Sum256(checkpoint)
	return hex.EncodeToString(digest[:])
}

func terminalStatus(eventType string) string {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case "run_finished":
		return "completed"
	case "run_failed", "run_error":
		return "failed"
	case "run_cancelled":
		return "cancelled"
	default:
		return ""
	}
}

func terminalError(payload json.RawMessage) string {
	var value map[string]any
	if json.Unmarshal(payload, &value) == nil {
		if message, ok := value["error"].(string); ok {
			return strings.TrimSpace(message)
		}
		if message, ok := value["reason"].(string); ok {
			return strings.TrimSpace(message)
		}
	}
	if text := strings.TrimSpace(string(payload)); text != "" && text != "null" && text != "{}" {
		return text
	}
	return ""
}

func upsertAgentRunTerminalAlert(ctx context.Context, db DBTX, event *AIChatRunEvent) error {
	status := terminalStatus(event.EventType)
	if status == "" {
		status = "terminal"
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO agent_run_terminal_alerts (
			id, run_id, session_id, terminal_status, event_type, error_message, payload, status, created_at
		) VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7::jsonb, 'pending', NOW())
		ON CONFLICT (run_id) DO NOTHING
	`, uuid.NewString(), event.RunID, event.SessionID, status, event.EventType,
		terminalError(event.Payload), normalizeJSON(event.Payload, "{}")); err != nil {
		return fmt.Errorf("enqueue agent terminal alert: %w", err)
	}
	return nil
}

func saveRunCheckpointInDB(
	ctx context.Context,
	db DBTX,
	runID, ownerID, workerID string,
	checkpoint json.RawMessage,
) error {
	if len(checkpoint) == 0 || !json.Valid(checkpoint) {
		return errors.New("checkpoint must be valid JSON")
	}
	var sessionID string
	var err error
	if strings.TrimSpace(workerID) != "" {
		err = db.QueryRow(ctx, `
			UPDATE ai_chat_runs
			SET checkpoint = $4::jsonb
			WHERE id = $1 AND worker_id = $2 AND lease_token = $3
			  AND leased_until IS NOT NULL AND leased_until > NOW()
			RETURNING session_id
		`, strings.TrimSpace(runID), strings.TrimSpace(workerID), strings.TrimSpace(ownerID), checkpoint).Scan(&sessionID)
	} else {
		err = db.QueryRow(ctx, `
			UPDATE ai_chat_runs
			SET checkpoint = $3::jsonb
			WHERE id = $1 AND created_by = $2
			RETURNING session_id
		`, strings.TrimSpace(runID), strings.TrimSpace(ownerID), checkpoint).Scan(&sessionID)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		if strings.TrimSpace(workerID) != "" {
			return ErrAgentRunLeaseLost
		}
		return errors.New("agent run not found")
	}
	if err != nil {
		return fmt.Errorf("save agent run checkpoint: %w", err)
	}

	schemaVersion, status, nextIndex := checkpointMetadata(checkpoint)
	if status == "" {
		status = "saved"
	}
	hash := checkpointHash(checkpoint)
	actorID := strings.TrimSpace(ownerID)
	if strings.TrimSpace(workerID) != "" {
		actorID = ""
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO agent_run_checkpoint_audits (
			id, run_id, session_id, checkpoint_hash, checkpoint_size_bytes,
			schema_version, checkpoint_status, next_index, actor_id, worker_id, created_at
		) VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7, $8, NULLIF($9, '')::uuid, NULLIF($10, ''), NOW())
	`, uuid.NewString(), runID, sessionID, hash, len(checkpoint), schemaVersion,
		status, nextIndex, actorID, strings.TrimSpace(workerID)); err != nil {
		return fmt.Errorf("record agent checkpoint audit: %w", err)
	}
	payload, _ := json.Marshal(map[string]any{
		"checkpoint_hash": hash, "checkpoint_size_bytes": len(checkpoint),
		"schema_version": schemaVersion, "checkpoint_status": status, "next_index": nextIndex,
	})
	return appendAgentRunEvent(ctx, db, &AIChatRunEvent{
		RunID: runID, SessionID: sessionID, EventType: "checkpoint_saved", Payload: payload,
	})
}

func (r *AIChatRuntimeRepository) SaveRunCheckpoint(ctx context.Context, runID, userID string, checkpoint json.RawMessage) error {
	if r == nil || r.db == nil {
		return errors.New("agent checkpoint store unavailable")
	}
	if pool, ok := r.db.(*pgxpool.Pool); ok {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin checkpoint transaction: %w", err)
		}
		defer tx.Rollback(ctx)
		if err := saveRunCheckpointInDB(ctx, tx, runID, userID, "", checkpoint); err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit checkpoint transaction: %w", err)
		}
		return nil
	}
	return saveRunCheckpointInDB(ctx, r.db, runID, userID, "", checkpoint)
}

func (r *AgentRunQueueRepository) SaveClaimedRunCheckpoint(ctx context.Context, runID, workerID, leaseToken string, checkpoint json.RawMessage) error {
	if r == nil || r.db == nil {
		return errors.New("agent run queue repository unavailable")
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin claimed checkpoint transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := saveRunCheckpointInDB(ctx, tx, runID, leaseToken, workerID, checkpoint); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit claimed checkpoint transaction: %w", err)
	}
	return nil
}

func (r *AIChatRuntimeRepository) ListTerminalAlerts(ctx context.Context, userID, status string, limit int) ([]*AgentRunTerminalAlert, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	conditions := "s.user_id = $1"
	args := []any{strings.TrimSpace(userID)}
	if strings.TrimSpace(status) != "" {
		conditions += " AND a.status = $2"
		args = append(args, strings.TrimSpace(status))
	}
	args = append(args, limit)
	limitArg := len(args)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
		SELECT a.id, a.run_id, a.session_id, a.terminal_status, a.event_type,
		       a.error_message, a.payload, a.status, a.created_at,
		       a.acknowledged_at, a.acknowledged_by
		FROM agent_run_terminal_alerts a
		JOIN ai_chat_sessions s ON s.id = a.session_id
		WHERE %s
		ORDER BY a.created_at DESC
		LIMIT $%d
	`, conditions, limitArg), args...)
	if err != nil {
		return nil, fmt.Errorf("list agent terminal alerts: %w", err)
	}
	defer rows.Close()
	var alerts []*AgentRunTerminalAlert
	for rows.Next() {
		var alert AgentRunTerminalAlert
		if err := rows.Scan(&alert.ID, &alert.RunID, &alert.SessionID, &alert.TerminalStatus,
			&alert.EventType, &alert.ErrorMessage, &alert.Payload, &alert.Status,
			&alert.CreatedAt, &alert.AcknowledgedAt, &alert.AcknowledgedBy); err != nil {
			return nil, fmt.Errorf("scan agent terminal alert: %w", err)
		}
		alerts = append(alerts, &alert)
	}
	return alerts, rows.Err()
}

func (r *AIChatRuntimeRepository) AcknowledgeTerminalAlert(ctx context.Context, alertID, userID string) error {
	result, err := r.db.Exec(ctx, `
		UPDATE agent_run_terminal_alerts a
		SET status = 'acknowledged', acknowledged_at = NOW(), acknowledged_by = $2::uuid
		FROM ai_chat_sessions s
		WHERE a.id = $1 AND a.session_id = s.id AND s.user_id = $2
	`, strings.TrimSpace(alertID), strings.TrimSpace(userID))
	if err != nil {
		return fmt.Errorf("acknowledge agent terminal alert: %w", err)
	}
	if result.RowsAffected() != 1 {
		return errors.New("agent terminal alert not found")
	}
	return nil
}

// CreateRunAuditLinks is called inside the review transaction. A link is a
// small immutable index; it deliberately does not duplicate the business row
// or the full Artifact JSON.
func (r *AIChatRuntimeRepository) CreateRunAuditLinks(
	ctx context.Context,
	runID, artifactID, relation string,
	items []AgentRunAuditLinkInput,
) error {
	if r == nil || r.db == nil {
		return errors.New("agent audit link store unavailable")
	}
	for _, item := range items {
		if strings.TrimSpace(item.BusinessTable) == "" || strings.TrimSpace(item.BusinessRecordID) == "" {
			continue
		}
		linkRelation := strings.TrimSpace(item.Relation)
		if linkRelation == "" {
			linkRelation = strings.TrimSpace(relation)
		}
		if _, err := r.db.Exec(ctx, `
			INSERT INTO agent_run_audit_links (
				id, run_id, artifact_id, business_table, business_record_id,
				relation, item_status, created_at
			) VALUES ($1, $2, NULLIF($3, '')::uuid, $4, $5, $6, NULLIF($7, ''), NOW())
			ON CONFLICT (run_id, artifact_id, business_table, business_record_id, relation)
			DO UPDATE SET item_status = EXCLUDED.item_status
		`, uuid.NewString(), strings.TrimSpace(runID), strings.TrimSpace(artifactID),
			strings.TrimSpace(item.BusinessTable), strings.TrimSpace(item.BusinessRecordID),
			linkRelation, strings.TrimSpace(item.ItemStatus)); err != nil {
			return fmt.Errorf("create agent run audit link: %w", err)
		}
	}
	return nil
}

func (r *AIChatRuntimeRepository) ListRunAuditLinks(ctx context.Context, runID, userID string) ([]*AgentRunAuditLink, error) {
	rows, err := r.db.Query(ctx, `
		SELECT l.id, l.run_id, l.artifact_id, l.business_table, l.business_record_id,
		       l.relation, l.item_status, l.created_at
		FROM agent_run_audit_links l
		JOIN ai_chat_sessions s ON s.id = (
			SELECT session_id FROM ai_chat_runs WHERE id = l.run_id
		)
		WHERE l.run_id = $1 AND s.user_id = $2
		ORDER BY l.created_at ASC
	`, strings.TrimSpace(runID), strings.TrimSpace(userID))
	if err != nil {
		return nil, fmt.Errorf("list agent run audit links: %w", err)
	}
	defer rows.Close()
	var links []*AgentRunAuditLink
	for rows.Next() {
		var link AgentRunAuditLink
		if err := rows.Scan(&link.ID, &link.RunID, &link.ArtifactID, &link.BusinessTable,
			&link.BusinessRecordID, &link.Relation, &link.ItemStatus, &link.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan agent run audit link: %w", err)
		}
		links = append(links, &link)
	}
	return links, rows.Err()
}

func (r *AIChatRuntimeRepository) ListRunCheckpointAudits(ctx context.Context, runID, userID string) ([]*AgentRunCheckpointAudit, error) {
	rows, err := r.db.Query(ctx, `
		SELECT a.id, a.run_id, a.session_id, a.checkpoint_hash, a.checkpoint_size_bytes,
		       a.schema_version, a.checkpoint_status, a.next_index, a.actor_id,
		       a.worker_id, a.created_at
		FROM agent_run_checkpoint_audits a
		JOIN ai_chat_sessions s ON s.id = a.session_id
		WHERE a.run_id = $1 AND s.user_id = $2
		ORDER BY a.created_at ASC
	`, strings.TrimSpace(runID), strings.TrimSpace(userID))
	if err != nil {
		return nil, fmt.Errorf("list agent checkpoint audits: %w", err)
	}
	defer rows.Close()
	var audits []*AgentRunCheckpointAudit
	for rows.Next() {
		var audit AgentRunCheckpointAudit
		if err := rows.Scan(&audit.ID, &audit.RunID, &audit.SessionID, &audit.CheckpointHash,
			&audit.CheckpointSize, &audit.SchemaVersion, &audit.CheckpointStatus,
			&audit.NextIndex, &audit.ActorID, &audit.WorkerID, &audit.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan agent checkpoint audit: %w", err)
		}
		audits = append(audits, &audit)
	}
	return audits, rows.Err()
}
