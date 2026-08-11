package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNoQueuedAgentRun  = errors.New("no queued agent run available")
	ErrAgentRunLeaseLost = errors.New("agent run lease is not owned by the worker")
)

// AgentRunQueueRepository is intentionally separate from the owner-scoped
// Run repository. Queue operations are worker operations and therefore must
// not inherit a normal user's session scope.
type AgentRunQueueRepository struct {
	db *pgxpool.Pool
}

func NewAgentRunQueueRepository(db *pgxpool.Pool) *AgentRunQueueRepository {
	return &AgentRunQueueRepository{db: db}
}

// ClaimQueuedRun atomically claims the oldest queued Run, or a running Run
// whose lease expired. FOR UPDATE SKIP LOCKED prevents two workers from
// claiming the same Run while allowing healthy workers to make progress.
func (r *AgentRunQueueRepository) ClaimQueuedRun(ctx context.Context, workerID string, leaseDuration time.Duration) (*AIChatRun, string, error) {
	if r == nil || r.db == nil {
		return nil, "", errors.New("agent run queue repository unavailable")
	}
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return nil, "", errors.New("worker ID is required")
	}
	seconds := int(leaseDuration / time.Second)
	if seconds <= 0 {
		seconds = 60
	}
	var run AIChatRun
	var leaseToken string
	err := r.db.QueryRow(ctx, `
		UPDATE ai_chat_runs r
		SET worker_id = $1,
			lease_token = uuid_generate_v4()::text,
			leased_until = NOW() + ($2 * INTERVAL '1 second'),
			heartbeat_at = NOW(),
			attempt_count = COALESCE(r.attempt_count, 0) + 1,
			status = 'running',
			started_at = COALESCE(r.started_at, NOW()),
			completed_at = NULL
		WHERE r.id = (
			SELECT candidate.id
			FROM ai_chat_runs AS candidate
			WHERE candidate.status = 'queued'
			   OR (candidate.status = 'running' AND candidate.leased_until IS NOT NULL AND candidate.leased_until < NOW())
			ORDER BY candidate.created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING r.id, r.session_id, r.parent_run_id, r.status, r.agent_mode,
			r.skill_id, r.skill_version, COALESCE(r.page_context, 'null'::jsonb),
			r.review_required, r.summary_text, r.error_message, r.created_by,
			r.created_at, r.started_at, r.completed_at, r.worker_id,
			r.lease_token, r.leased_until, r.heartbeat_at, r.attempt_count
	`, workerID, seconds).Scan(
		&run.ID, &run.SessionID, &run.ParentRunID, &run.Status, &run.AgentMode,
		&run.SkillID, &run.SkillVersion, &run.PageContext, &run.ReviewRequired,
		&run.SummaryText, &run.ErrorMessage, &run.CreatedBy, &run.CreatedAt,
		&run.StartedAt, &run.CompletedAt, &run.WorkerID, &leaseToken,
		&run.LeasedUntil, &run.HeartbeatAt, &run.AttemptCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", ErrNoQueuedAgentRun
		}
		return nil, "", fmt.Errorf("claim agent run: %w", err)
	}
	run.LeaseToken = leaseToken
	return &run, leaseToken, nil
}

func (r *AgentRunQueueRepository) HeartbeatRunLease(ctx context.Context, runID, workerID, leaseToken string, leaseDuration time.Duration) error {
	if r == nil || r.db == nil {
		return errors.New("agent run queue repository unavailable")
	}
	seconds := int(leaseDuration / time.Second)
	if seconds <= 0 {
		seconds = 60
	}
	result, err := r.db.Exec(ctx, `
		UPDATE ai_chat_runs
		SET leased_until = NOW() + ($4 * INTERVAL '1 second'), heartbeat_at = NOW()
		WHERE id = $1 AND worker_id = $2 AND lease_token = $3
		  AND status IN ('queued', 'running', 'waiting_review', 'completed', 'failed', 'cancelled', 'aborted')
	`, strings.TrimSpace(runID), strings.TrimSpace(workerID), strings.TrimSpace(leaseToken), seconds)
	if err != nil {
		return fmt.Errorf("heartbeat agent run lease: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrAgentRunLeaseLost
	}
	return nil
}

func (r *AgentRunQueueRepository) ReleaseRunLease(ctx context.Context, runID, workerID, leaseToken string, requeue bool) error {
	if r == nil || r.db == nil {
		return errors.New("agent run queue repository unavailable")
	}
	result, err := r.db.Exec(ctx, `
		UPDATE ai_chat_runs
		SET status = CASE WHEN $4 THEN 'queued' ELSE status END,
			worker_id = NULL, lease_token = NULL,
			leased_until = NULL, heartbeat_at = NULL,
			error_message = CASE WHEN $4 THEN 'worker lease released; requeued' ELSE error_message END
		WHERE id = $1 AND worker_id = $2 AND lease_token = $3
		  AND (
			status IN ('queued', 'running')
			OR ($4 = FALSE AND status IN ('waiting_review', 'completed', 'failed', 'cancelled', 'aborted'))
		  )
	`, strings.TrimSpace(runID), strings.TrimSpace(workerID), strings.TrimSpace(leaseToken), requeue)
	if err != nil {
		return fmt.Errorf("release agent run lease: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrAgentRunLeaseLost
	}
	return nil
}

// RecoverExpiredRunLeases requeues expired workers. The queue_update event is
// persisted in the same transaction so the unified Trace explains why a Run
// was handed to another process.
func (r *AgentRunQueueRepository) RecoverExpiredRunLeases(ctx context.Context) (int, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("agent run queue repository unavailable")
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin recover expired agent run leases: %w", err)
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `
		WITH expired AS (
			SELECT id, session_id
			FROM ai_chat_runs
			WHERE status = 'running' AND leased_until IS NOT NULL AND leased_until < NOW()
			FOR UPDATE SKIP LOCKED
		), reset_runs AS (
			UPDATE ai_chat_runs r
			SET status = 'queued', worker_id = NULL, lease_token = NULL,
				leased_until = NULL, heartbeat_at = NULL,
				error_message = 'worker lease expired; requeued'
			FROM expired e
			WHERE r.id = e.id
			RETURNING r.id, r.session_id
		)
		SELECT id, session_id FROM reset_runs
	`)
	if err != nil {
		return 0, fmt.Errorf("recover expired agent run leases: %w", err)
	}
	var recovered []struct{ runID, sessionID string }
	for rows.Next() {
		var item struct{ runID, sessionID string }
		if err := rows.Scan(&item.runID, &item.sessionID); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan recovered agent run lease: %w", err)
		}
		recovered = append(recovered, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("read recovered agent run leases: %w", err)
	}
	rows.Close()
	for _, item := range recovered {
		payload, _ := json.Marshal(map[string]any{"reason": "lease_expired"})
		if err := appendAgentRunEvent(ctx, tx, &AIChatRunEvent{
			RunID: item.runID, SessionID: item.sessionID,
			EventType: "queue_update", Payload: payload,
		}); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit recover expired agent run leases: %w", err)
	}
	return len(recovered), nil
}

// GetClaimedRun is the worker-side read path. It intentionally does not join
// by session owner; the lease is the authorization boundary for a worker.
func (r *AgentRunQueueRepository) GetClaimedRun(ctx context.Context, runID, workerID, leaseToken string) (*AIChatRun, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("agent run queue repository unavailable")
	}
	run, err := scanClaimedAgentRun(r.db.QueryRow(ctx, claimedRunQuery, strings.TrimSpace(runID), strings.TrimSpace(workerID), strings.TrimSpace(leaseToken)))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAgentRunLeaseLost
		}
		return nil, fmt.Errorf("get claimed agent run: %w", err)
	}
	run.LeaseToken = strings.TrimSpace(leaseToken)
	return run, nil
}

func (r *AgentRunQueueRepository) ListClaimedRunEvents(ctx context.Context, runID, workerID, leaseToken string, afterSequence, limit int) ([]*AIChatRunEvent, error) {
	if _, err := r.GetClaimedRun(ctx, runID, workerID, leaseToken); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 200
	}
	rows, err := r.db.Query(ctx, `
		SELECT e.id, e.run_id, e.session_id, e.sequence_no, e.event_type, e.payload, e.is_terminal, e.created_at
		FROM ai_chat_run_events e
		WHERE e.run_id = $1 AND e.sequence_no > $2
		ORDER BY e.sequence_no ASC
		LIMIT $3
	`, strings.TrimSpace(runID), afterSequence, limit)
	if err != nil {
		return nil, fmt.Errorf("list claimed agent run events: %w", err)
	}
	defer rows.Close()
	var events []*AIChatRunEvent
	for rows.Next() {
		var event AIChatRunEvent
		if err := rows.Scan(&event.ID, &event.RunID, &event.SessionID, &event.SequenceNo, &event.EventType, &event.Payload, &event.IsTerminal, &event.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan claimed agent run event: %w", err)
		}
		events = append(events, &event)
	}
	return events, rows.Err()
}

// AppendClaimedRunEvent verifies the lease and allocates the event sequence
// under a transaction. The advisory lock closes the duplicate-sequence race
// between concurrent event writers for the same Run.
func (r *AgentRunQueueRepository) AppendClaimedRunEvent(ctx context.Context, runID, workerID, leaseToken string, event *AIChatRunEvent) error {
	if r == nil || r.db == nil {
		return errors.New("agent run queue repository unavailable")
	}
	runID, workerID, leaseToken = strings.TrimSpace(runID), strings.TrimSpace(workerID), strings.TrimSpace(leaseToken)
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin claimed event transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var sessionID string
	if err := tx.QueryRow(ctx, `
		SELECT session_id FROM ai_chat_runs
		WHERE id = $1 AND worker_id = $2 AND lease_token = $3
		  AND leased_until IS NOT NULL AND leased_until > NOW()
	`, runID, workerID, leaseToken).Scan(&sessionID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrAgentRunLeaseLost
		}
		return fmt.Errorf("verify claimed agent run event writer: %w", err)
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, runID); err != nil {
		return fmt.Errorf("lock claimed agent run event stream: %w", err)
	}
	var sequence int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(sequence_no), 0) + 1 FROM ai_chat_run_events WHERE run_id = $1`, runID).Scan(&sequence); err != nil {
		return fmt.Errorf("allocate claimed agent run event sequence: %w", err)
	}
	if event == nil {
		return errors.New("agent run event is required")
	}
	event.ID = uuid.NewString()
	event.RunID = runID
	event.SessionID = sessionID
	event.SequenceNo = sequence
	event.CreatedAt = time.Now().UTC()
	if _, err := tx.Exec(ctx, `
		INSERT INTO ai_chat_run_events (id, run_id, session_id, sequence_no, event_type, payload, is_terminal, created_at)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8)
	`, event.ID, event.RunID, event.SessionID, event.SequenceNo, event.EventType, normalizeJSON(event.Payload, "{}"), event.IsTerminal, event.CreatedAt); err != nil {
		return fmt.Errorf("append claimed agent run event: %w", err)
	}
	if err := upsertAgentRunAuditSummary(ctx, tx, event); err != nil {
		return fmt.Errorf("update claimed agent run audit summary: %w", err)
	}
	return tx.Commit(ctx)
}

func (r *AgentRunQueueRepository) UpdateClaimedRunStatus(ctx context.Context, runID, workerID, leaseToken, status string, reviewRequired bool, summaryText, errorMessage *string, startedAt, completedAt *time.Time) error {
	if r == nil || r.db == nil {
		return errors.New("agent run queue repository unavailable")
	}
	result, err := r.db.Exec(ctx, `
		UPDATE ai_chat_runs
		SET status = $4, review_required = $5, summary_text = $6, error_message = $7,
			started_at = COALESCE($8, started_at), completed_at = COALESCE($9, completed_at)
		WHERE id = $1 AND worker_id = $2 AND lease_token = $3
		  AND leased_until IS NOT NULL AND leased_until > NOW()
	`, strings.TrimSpace(runID), strings.TrimSpace(workerID), strings.TrimSpace(leaseToken), strings.TrimSpace(status), reviewRequired, summaryText, errorMessage, startedAt, completedAt)
	if err != nil {
		return fmt.Errorf("update claimed agent run status: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrAgentRunLeaseLost
	}
	return nil
}

func (r *AgentRunQueueRepository) GetClaimedRunCheckpoint(ctx context.Context, runID, workerID, leaseToken string) (json.RawMessage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("agent run queue repository unavailable")
	}
	var checkpoint json.RawMessage
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(checkpoint, 'null'::jsonb)
		FROM ai_chat_runs
		WHERE id = $1 AND worker_id = $2 AND lease_token = $3
		  AND leased_until IS NOT NULL AND leased_until > NOW()
	`, strings.TrimSpace(runID), strings.TrimSpace(workerID), strings.TrimSpace(leaseToken)).Scan(&checkpoint)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAgentRunLeaseLost
		}
		return nil, fmt.Errorf("get claimed agent run checkpoint: %w", err)
	}
	return checkpoint, nil
}

const claimedRunQuery = `
	SELECT r.id, r.session_id, r.trigger_message_id, r.parent_run_id, r.status, r.agent_mode,
	       r.skill_id, r.skill_version, COALESCE(r.page_context, 'null'::jsonb), r.review_required,
	       r.summary_text, r.error_message, r.created_by, r.created_at, r.started_at, r.completed_at,
	       r.worker_id, r.lease_token, r.leased_until, r.heartbeat_at, r.attempt_count
	FROM ai_chat_runs r
	WHERE r.id = $1 AND r.worker_id = $2 AND r.lease_token = $3
	  AND r.leased_until IS NOT NULL AND r.leased_until > NOW()
`

func scanClaimedAgentRun(row pgx.Row) (*AIChatRun, error) {
	var run AIChatRun
	err := row.Scan(
		&run.ID, &run.SessionID, &run.TriggerMessageID, &run.ParentRunID, &run.Status, &run.AgentMode,
		&run.SkillID, &run.SkillVersion, &run.PageContext, &run.ReviewRequired,
		&run.SummaryText, &run.ErrorMessage, &run.CreatedBy, &run.CreatedAt, &run.StartedAt, &run.CompletedAt,
		&run.WorkerID, &run.LeaseToken, &run.LeasedUntil, &run.HeartbeatAt, &run.AttemptCount,
	)
	return &run, err
}
