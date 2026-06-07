package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AIChatSession struct {
	ID              string          `json:"id"`
	UserID          string          `json:"user_id"`
	LegalEntityID   *string         `json:"legal_entity_id"`
	Title           string          `json:"title"`
	Status          string          `json:"status"`
	BoundContractID *string         `json:"bound_contract_id"`
	ContextSnapshot json.RawMessage `json:"context_snapshot"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	LastMessageAt   *time.Time      `json:"last_message_at"`
	ArchivedAt      *time.Time      `json:"archived_at"`
}

type AIChatRun struct {
	ID               string          `json:"id"`
	SessionID        string          `json:"session_id"`
	TriggerMessageID *string         `json:"trigger_message_id"`
	ParentRunID      *string         `json:"parent_run_id"`
	Status           string          `json:"status"`
	AgentMode        bool            `json:"agent_mode"`
	SkillID          *string         `json:"skill_id"`
	SkillVersion     *string         `json:"skill_version"`
	PageContext      json.RawMessage `json:"page_context"`
	ReviewRequired   bool            `json:"review_required"`
	SummaryText      *string         `json:"summary_text"`
	ErrorMessage     *string         `json:"error_message"`
	CreatedBy        *string         `json:"created_by"`
	CreatedAt        time.Time       `json:"created_at"`
	StartedAt        *time.Time      `json:"started_at"`
	CompletedAt      *time.Time      `json:"completed_at"`
}

type AIChatMessage struct {
	ID          string          `json:"id"`
	SessionID   string          `json:"session_id"`
	RunID       *string         `json:"run_id"`
	Role        string          `json:"role"`
	MessageType string          `json:"message_type"`
	SequenceNo  int             `json:"sequence_no"`
	Content     string          `json:"content"`
	ContentJSON json.RawMessage `json:"content_json"`
	Sources     json.RawMessage `json:"sources"`
	Attachments json.RawMessage `json:"attachments"`
	Model       *string         `json:"model"`
	CreatedBy   *string         `json:"created_by"`
	CreatedAt   time.Time       `json:"created_at"`
}

type AIChatRunEvent struct {
	ID         string          `json:"id"`
	RunID      string          `json:"run_id"`
	SessionID  string          `json:"session_id"`
	SequenceNo int             `json:"sequence_no"`
	EventType  string          `json:"event_type"`
	Payload    json.RawMessage `json:"payload"`
	IsTerminal bool            `json:"is_terminal"`
	CreatedAt  time.Time       `json:"created_at"`
}

type AIChatArtifact struct {
	ID             string          `json:"id"`
	SessionID      string          `json:"session_id"`
	RunID          string          `json:"run_id"`
	ArtifactType   string          `json:"artifact_type"`
	Title          string          `json:"title"`
	Status         string          `json:"status"`
	Data           json.RawMessage `json:"data"`
	Actions        json.RawMessage `json:"actions"`
	EvidenceRefs   json.RawMessage `json:"evidence_refs"`
	ReviewRequired bool            `json:"review_required"`
	CreatedBy      *string         `json:"created_by"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type AIChatReviewAction struct {
	ID            string          `json:"id"`
	SessionID     string          `json:"session_id"`
	RunID         *string         `json:"run_id"`
	ArtifactID    *string         `json:"artifact_id"`
	ActionType    string          `json:"action_type"`
	ActionPayload json.RawMessage `json:"action_payload"`
	Comment       *string         `json:"comment"`
	ActedBy       string          `json:"acted_by"`
	ActedAt       time.Time       `json:"acted_at"`
}

type AIChatAttachment struct {
	ID             string    `json:"id"`
	SessionID      string    `json:"session_id"`
	MessageID      *string   `json:"message_id"`
	RunID          *string   `json:"run_id"`
	FileID         string    `json:"file_id"`
	OriginalName   string    `json:"original_name"`
	ContentType    string    `json:"content_type"`
	MinioBucket    *string   `json:"minio_bucket"`
	MinioObjectKey *string   `json:"minio_object_key"`
	FileSize       *int64    `json:"file_size"`
	OCRStatus      string    `json:"ocr_status"`
	ParseStatus    string    `json:"parse_status"`
	CreatedBy      *string   `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
}

type AIChatSessionFilter struct {
	UserID        string
	LegalEntityID string
	Status        string
	Limit         int
	Offset        int
}

type AIChatRuntimeRepository struct {
	db *pgxpool.Pool
}

func NewAIChatRuntimeRepository(db *pgxpool.Pool) *AIChatRuntimeRepository {
	return &AIChatRuntimeRepository{db: db}
}

func normalizeJSON(raw json.RawMessage, fallback string) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(fallback)
	}
	return raw
}

func (r *AIChatRuntimeRepository) CreateSession(ctx context.Context, session *AIChatSession) error {
	session.ID = uuid.New().String()
	if session.Title == "" {
		session.Title = "新会话"
	}
	if session.Status == "" {
		session.Status = "active"
	}
	now := time.Now()
	session.CreatedAt = now
	session.UpdatedAt = now

	_, err := r.db.Exec(ctx, `
		INSERT INTO ai_chat_sessions (
			id, user_id, legal_entity_id, title, status, bound_contract_id,
			context_snapshot, created_at, updated_at, last_message_at, archived_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, $11)
	`,
		session.ID, session.UserID, session.LegalEntityID, session.Title, session.Status, session.BoundContractID,
		normalizeJSON(session.ContextSnapshot, "null"), session.CreatedAt, session.UpdatedAt, session.LastMessageAt, session.ArchivedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create ai chat session: %w", err)
	}
	return nil
}

func (r *AIChatRuntimeRepository) GetSessionByID(ctx context.Context, sessionID, userID string) (*AIChatSession, error) {
	var session AIChatSession
	err := r.db.QueryRow(ctx, `
		SELECT id, user_id, legal_entity_id, title, status, bound_contract_id,
		       COALESCE(context_snapshot, 'null'::jsonb), created_at, updated_at, last_message_at, archived_at
		FROM ai_chat_sessions
		WHERE id = $1 AND user_id = $2
	`, sessionID, userID).Scan(
		&session.ID, &session.UserID, &session.LegalEntityID, &session.Title, &session.Status, &session.BoundContractID,
		&session.ContextSnapshot, &session.CreatedAt, &session.UpdatedAt, &session.LastMessageAt, &session.ArchivedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get ai chat session: %w", err)
	}
	return &session, nil
}

func (r *AIChatRuntimeRepository) ListSessions(ctx context.Context, filter AIChatSessionFilter) ([]*AIChatSession, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}

	query := `
		SELECT id, user_id, legal_entity_id, title, status, bound_contract_id,
		       COALESCE(context_snapshot, 'null'::jsonb), created_at, updated_at, last_message_at, archived_at
		FROM ai_chat_sessions
		WHERE user_id = $1
		  AND ($2 = '' OR COALESCE(legal_entity_id::text, '') = $2)
		  AND ($3 = '' OR status = $3)
		ORDER BY COALESCE(last_message_at, updated_at) DESC
		LIMIT $4 OFFSET $5
	`
	rows, err := r.db.Query(ctx, query, filter.UserID, filter.LegalEntityID, filter.Status, filter.Limit, filter.Offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list ai chat sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*AIChatSession
	for rows.Next() {
		var session AIChatSession
		if err := rows.Scan(
			&session.ID, &session.UserID, &session.LegalEntityID, &session.Title, &session.Status, &session.BoundContractID,
			&session.ContextSnapshot, &session.CreatedAt, &session.UpdatedAt, &session.LastMessageAt, &session.ArchivedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ai chat session: %w", err)
		}
		sessions = append(sessions, &session)
	}
	return sessions, rows.Err()
}

func (r *AIChatRuntimeRepository) CreateRun(ctx context.Context, run *AIChatRun) error {
	run.ID = uuid.New().String()
	if run.Status == "" {
		run.Status = "queued"
	}
	run.CreatedAt = time.Now()

	_, err := r.db.Exec(ctx, `
		INSERT INTO ai_chat_runs (
			id, session_id, trigger_message_id, parent_run_id, status, agent_mode,
			skill_id, skill_version, page_context, review_required, summary_text,
			error_message, created_by, created_at, started_at, completed_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::jsonb, $10, $11, $12, $13, $14, $15, $16)
	`,
		run.ID, run.SessionID, run.TriggerMessageID, run.ParentRunID, run.Status, run.AgentMode,
		run.SkillID, run.SkillVersion, normalizeJSON(run.PageContext, "null"), run.ReviewRequired, run.SummaryText,
		run.ErrorMessage, run.CreatedBy, run.CreatedAt, run.StartedAt, run.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create ai chat run: %w", err)
	}
	return nil
}

func (r *AIChatRuntimeRepository) LinkRunTriggerMessage(ctx context.Context, runID, triggerMessageID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ai_chat_runs
		SET trigger_message_id = $2
		WHERE id = $1
	`, runID, triggerMessageID)
	if err != nil {
		return fmt.Errorf("failed to link ai chat run trigger message: %w", err)
	}
	return nil
}

func (r *AIChatRuntimeRepository) UpdateRunStatus(ctx context.Context, runID, status string, reviewRequired bool, summaryText, errorMessage *string, startedAt, completedAt *time.Time) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ai_chat_runs
		SET status = $2,
		    review_required = $3,
		    summary_text = COALESCE($4, summary_text),
		    error_message = $5,
		    started_at = COALESCE($6, started_at),
		    completed_at = COALESCE($7, completed_at)
		WHERE id = $1
	`, runID, status, reviewRequired, summaryText, errorMessage, startedAt, completedAt)
	if err != nil {
		return fmt.Errorf("failed to update ai chat run status: %w", err)
	}
	return nil
}

func (r *AIChatRuntimeRepository) UpdateRunParent(ctx context.Context, runID, parentRunID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ai_chat_runs
		SET parent_run_id = $2
		WHERE id = $1
	`, runID, parentRunID)
	if err != nil {
		return fmt.Errorf("failed to update ai chat run parent: %w", err)
	}
	return nil
}

func (r *AIChatRuntimeRepository) GetRunByID(ctx context.Context, runID, userID string) (*AIChatRun, error) {
	var run AIChatRun
	err := r.db.QueryRow(ctx, `
		SELECT r.id, r.session_id, r.trigger_message_id, r.parent_run_id, r.status, r.agent_mode,
		       r.skill_id, r.skill_version, COALESCE(r.page_context, 'null'::jsonb), r.review_required,
		       r.summary_text, r.error_message, r.created_by, r.created_at, r.started_at, r.completed_at
		FROM ai_chat_runs r
		INNER JOIN ai_chat_sessions s ON s.id = r.session_id
		WHERE r.id = $1 AND s.user_id = $2
	`, runID, userID).Scan(
		&run.ID, &run.SessionID, &run.TriggerMessageID, &run.ParentRunID, &run.Status, &run.AgentMode,
		&run.SkillID, &run.SkillVersion, &run.PageContext, &run.ReviewRequired,
		&run.SummaryText, &run.ErrorMessage, &run.CreatedBy, &run.CreatedAt, &run.StartedAt, &run.CompletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get ai chat run: %w", err)
	}
	return &run, nil
}

func (r *AIChatRuntimeRepository) ListRunsBySession(ctx context.Context, sessionID string, limit, offset int) ([]*AIChatRun, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, session_id, trigger_message_id, parent_run_id, status, agent_mode,
		       skill_id, skill_version, COALESCE(page_context, 'null'::jsonb), review_required,
		       summary_text, error_message, created_by, created_at, started_at, completed_at
		FROM ai_chat_runs
		WHERE session_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, sessionID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list ai chat runs: %w", err)
	}
	defer rows.Close()

	var runs []*AIChatRun
	for rows.Next() {
		var run AIChatRun
		if err := rows.Scan(
			&run.ID, &run.SessionID, &run.TriggerMessageID, &run.ParentRunID, &run.Status, &run.AgentMode,
			&run.SkillID, &run.SkillVersion, &run.PageContext, &run.ReviewRequired,
			&run.SummaryText, &run.ErrorMessage, &run.CreatedBy, &run.CreatedAt, &run.StartedAt, &run.CompletedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ai chat run: %w", err)
		}
		runs = append(runs, &run)
	}
	return runs, rows.Err()
}

func (r *AIChatRuntimeRepository) CreateMessage(ctx context.Context, message *AIChatMessage) error {
	message.ID = uuid.New().String()
	message.CreatedAt = time.Now()
	if message.Role == "" {
		message.Role = "assistant"
	}
	if message.MessageType == "" {
		message.MessageType = "text"
	}

	_, err := r.db.Exec(ctx, `
		INSERT INTO ai_chat_messages (
			id, session_id, run_id, role, message_type, sequence_no, content,
			content_json, sources, attachments, model, created_by, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9::jsonb, $10::jsonb, $11, $12, $13)
	`,
		message.ID, message.SessionID, message.RunID, message.Role, message.MessageType, message.SequenceNo, message.Content,
		normalizeJSON(message.ContentJSON, "null"), normalizeJSON(message.Sources, "null"),
		normalizeJSON(message.Attachments, "null"), message.Model, message.CreatedBy, message.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create ai chat message: %w", err)
	}

	_, err = r.db.Exec(ctx, `
		UPDATE ai_chat_sessions
		SET updated_at = $2, last_message_at = $2
		WHERE id = $1
	`, message.SessionID, message.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to bump ai chat session timestamp: %w", err)
	}
	return nil
}

func (r *AIChatRuntimeRepository) GetNextMessageSequence(ctx context.Context, sessionID string) (int, error) {
	var nextSeq int
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(MAX(sequence_no), 0) + 1
		FROM ai_chat_messages
		WHERE session_id = $1
	`, sessionID).Scan(&nextSeq)
	if err != nil {
		return 0, fmt.Errorf("failed to get next ai chat message sequence: %w", err)
	}
	return nextSeq, nil
}

func (r *AIChatRuntimeRepository) ListMessagesBySession(ctx context.Context, sessionID string, limit int) ([]*AIChatMessage, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, session_id, run_id, role, message_type, sequence_no, content,
		       COALESCE(content_json, 'null'::jsonb), COALESCE(sources, 'null'::jsonb),
		       COALESCE(attachments, 'null'::jsonb), model, created_by, created_at
		FROM ai_chat_messages
		WHERE session_id = $1
		ORDER BY sequence_no DESC
		LIMIT $2
	`, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list ai chat messages: %w", err)
	}
	defer rows.Close()

	var messages []*AIChatMessage
	for rows.Next() {
		var message AIChatMessage
		if err := rows.Scan(
			&message.ID, &message.SessionID, &message.RunID, &message.Role, &message.MessageType,
			&message.SequenceNo, &message.Content, &message.ContentJSON, &message.Sources,
			&message.Attachments, &message.Model, &message.CreatedBy, &message.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ai chat message: %w", err)
		}
		messages = append(messages, &message)
	}
	return messages, rows.Err()
}

func (r *AIChatRuntimeRepository) GetMessageByID(ctx context.Context, messageID, userID string) (*AIChatMessage, error) {
	var message AIChatMessage
	err := r.db.QueryRow(ctx, `
		SELECT m.id, m.session_id, m.run_id, m.role, m.message_type, m.sequence_no, m.content,
		       COALESCE(m.content_json, 'null'::jsonb), COALESCE(m.sources, 'null'::jsonb),
		       COALESCE(m.attachments, 'null'::jsonb), m.model, m.created_by, m.created_at
		FROM ai_chat_messages m
		INNER JOIN ai_chat_sessions s ON s.id = m.session_id
		WHERE m.id = $1 AND s.user_id = $2
	`, messageID, userID).Scan(
		&message.ID, &message.SessionID, &message.RunID, &message.Role, &message.MessageType,
		&message.SequenceNo, &message.Content, &message.ContentJSON, &message.Sources,
		&message.Attachments, &message.Model, &message.CreatedBy, &message.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get ai chat message: %w", err)
	}
	return &message, nil
}

func (r *AIChatRuntimeRepository) AppendRunEvent(ctx context.Context, event *AIChatRunEvent) error {
	event.ID = uuid.New().String()
	event.CreatedAt = time.Now()
	_, err := r.db.Exec(ctx, `
		INSERT INTO ai_chat_run_events (
			id, run_id, session_id, sequence_no, event_type, payload, is_terminal, created_at
		) VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8)
	`,
		event.ID, event.RunID, event.SessionID, event.SequenceNo, event.EventType,
		normalizeJSON(event.Payload, "{}"), event.IsTerminal, event.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to append ai chat run event: %w", err)
	}
	return nil
}

func (r *AIChatRuntimeRepository) ListRunEvents(ctx context.Context, runID string, afterSequence, limit int) ([]*AIChatRunEvent, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, run_id, session_id, sequence_no, event_type, payload, is_terminal, created_at
		FROM ai_chat_run_events
		WHERE run_id = $1
		  AND sequence_no > $2
		ORDER BY sequence_no ASC
		LIMIT $3
	`, runID, afterSequence, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list ai chat run events: %w", err)
	}
	defer rows.Close()

	var events []*AIChatRunEvent
	for rows.Next() {
		var event AIChatRunEvent
		if err := rows.Scan(
			&event.ID, &event.RunID, &event.SessionID, &event.SequenceNo, &event.EventType,
			&event.Payload, &event.IsTerminal, &event.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ai chat run event: %w", err)
		}
		events = append(events, &event)
	}
	return events, rows.Err()
}

func (r *AIChatRuntimeRepository) GetNextRunEventSequence(ctx context.Context, runID string) (int, error) {
	var nextSeq int
	err := r.db.QueryRow(ctx, `
		SELECT COALESCE(MAX(sequence_no), 0) + 1
		FROM ai_chat_run_events
		WHERE run_id = $1
	`, runID).Scan(&nextSeq)
	if err != nil {
		return 0, fmt.Errorf("failed to get next ai chat run event sequence: %w", err)
	}
	return nextSeq, nil
}

func (r *AIChatRuntimeRepository) CreateArtifact(ctx context.Context, artifact *AIChatArtifact) error {
	artifact.ID = uuid.New().String()
	if artifact.Status == "" {
		artifact.Status = "ready"
	}
	now := time.Now()
	artifact.CreatedAt = now
	artifact.UpdatedAt = now

	_, err := r.db.Exec(ctx, `
		INSERT INTO ai_chat_artifacts (
			id, session_id, run_id, artifact_type, title, status, data,
			actions, evidence_refs, review_required, created_by, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb, $9::jsonb, $10, $11, $12, $13)
	`,
		artifact.ID, artifact.SessionID, artifact.RunID, artifact.ArtifactType, artifact.Title, artifact.Status,
		normalizeJSON(artifact.Data, "{}"), normalizeJSON(artifact.Actions, "null"),
		normalizeJSON(artifact.EvidenceRefs, "null"), artifact.ReviewRequired, artifact.CreatedBy,
		artifact.CreatedAt, artifact.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create ai chat artifact: %w", err)
	}
	return nil
}

func (r *AIChatRuntimeRepository) GetArtifactByID(ctx context.Context, artifactID, userID string) (*AIChatArtifact, error) {
	var artifact AIChatArtifact
	err := r.db.QueryRow(ctx, `
		SELECT a.id, a.session_id, a.run_id, a.artifact_type, a.title, a.status,
		       COALESCE(a.data, '{}'::jsonb), COALESCE(a.actions, 'null'::jsonb),
		       COALESCE(a.evidence_refs, 'null'::jsonb), a.review_required, a.created_by,
		       a.created_at, a.updated_at
		FROM ai_chat_artifacts a
		INNER JOIN ai_chat_sessions s ON s.id = a.session_id
		WHERE a.id = $1 AND s.user_id = $2
	`, artifactID, userID).Scan(
		&artifact.ID, &artifact.SessionID, &artifact.RunID, &artifact.ArtifactType, &artifact.Title, &artifact.Status,
		&artifact.Data, &artifact.Actions, &artifact.EvidenceRefs, &artifact.ReviewRequired, &artifact.CreatedBy,
		&artifact.CreatedAt, &artifact.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get ai chat artifact: %w", err)
	}
	return &artifact, nil
}

func (r *AIChatRuntimeRepository) ListArtifactsBySession(ctx context.Context, sessionID string, limit int) ([]*AIChatArtifact, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, session_id, run_id, artifact_type, title, status,
		       COALESCE(data, '{}'::jsonb), COALESCE(actions, 'null'::jsonb),
		       COALESCE(evidence_refs, 'null'::jsonb), review_required, created_by,
		       created_at, updated_at
		FROM ai_chat_artifacts
		WHERE session_id = $1
		ORDER BY created_at ASC
		LIMIT $2
	`, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list ai chat artifacts: %w", err)
	}
	defer rows.Close()

	var artifacts []*AIChatArtifact
	for rows.Next() {
		var artifact AIChatArtifact
		if err := rows.Scan(
			&artifact.ID, &artifact.SessionID, &artifact.RunID, &artifact.ArtifactType, &artifact.Title, &artifact.Status,
			&artifact.Data, &artifact.Actions, &artifact.EvidenceRefs, &artifact.ReviewRequired, &artifact.CreatedBy,
			&artifact.CreatedAt, &artifact.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ai chat artifact: %w", err)
		}
		artifacts = append(artifacts, &artifact)
	}
	return artifacts, rows.Err()
}

func (r *AIChatRuntimeRepository) UpdateArtifactStatus(ctx context.Context, artifactID, status string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ai_chat_artifacts
		SET status = $2,
		    updated_at = NOW()
		WHERE id = $1
	`, artifactID, status)
	if err != nil {
		return fmt.Errorf("failed to update ai chat artifact status: %w", err)
	}
	return nil
}

func (r *AIChatRuntimeRepository) RecordReviewAction(ctx context.Context, action *AIChatReviewAction) error {
	action.ID = uuid.New().String()
	action.ActedAt = time.Now()
	_, err := r.db.Exec(ctx, `
		INSERT INTO ai_chat_review_actions (
			id, session_id, run_id, artifact_id, action_type, action_payload, comment, acted_by, acted_at
		) VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8, $9)
	`,
		action.ID, action.SessionID, action.RunID, action.ArtifactID, action.ActionType,
		normalizeJSON(action.ActionPayload, "null"), action.Comment, action.ActedBy, action.ActedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to record ai chat review action: %w", err)
	}
	return nil
}

func (r *AIChatRuntimeRepository) GetReviewActionByID(ctx context.Context, actionID, userID string) (*AIChatReviewAction, error) {
	var action AIChatReviewAction
	err := r.db.QueryRow(ctx, `
		SELECT ra.id, ra.session_id, ra.run_id, ra.artifact_id, ra.action_type,
		       COALESCE(ra.action_payload, 'null'::jsonb), ra.comment, ra.acted_by, ra.acted_at
		FROM ai_chat_review_actions ra
		INNER JOIN ai_chat_sessions s ON s.id = ra.session_id
		WHERE ra.id = $1 AND s.user_id = $2
	`, actionID, userID).Scan(
		&action.ID, &action.SessionID, &action.RunID, &action.ArtifactID, &action.ActionType,
		&action.ActionPayload, &action.Comment, &action.ActedBy, &action.ActedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get ai chat review action: %w", err)
	}
	return &action, nil
}

func (r *AIChatRuntimeRepository) ListReviewActionsBySession(ctx context.Context, sessionID string, limit int) ([]*AIChatReviewAction, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, session_id, run_id, artifact_id, action_type,
		       COALESCE(action_payload, 'null'::jsonb), comment, acted_by, acted_at
		FROM ai_chat_review_actions
		WHERE session_id = $1
		ORDER BY acted_at ASC
		LIMIT $2
	`, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list ai chat review actions: %w", err)
	}
	defer rows.Close()

	var actions []*AIChatReviewAction
	for rows.Next() {
		var action AIChatReviewAction
		if err := rows.Scan(
			&action.ID, &action.SessionID, &action.RunID, &action.ArtifactID, &action.ActionType,
			&action.ActionPayload, &action.Comment, &action.ActedBy, &action.ActedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ai chat review action: %w", err)
		}
		actions = append(actions, &action)
	}
	return actions, rows.Err()
}

func (r *AIChatRuntimeRepository) CreateAttachment(ctx context.Context, attachment *AIChatAttachment) error {
	attachment.ID = uuid.New().String()
	if attachment.OCRStatus == "" {
		attachment.OCRStatus = "pending"
	}
	if attachment.ParseStatus == "" {
		attachment.ParseStatus = "pending"
	}
	attachment.CreatedAt = time.Now()
	_, err := r.db.Exec(ctx, `
		INSERT INTO ai_chat_attachments (
			id, session_id, message_id, run_id, file_id, original_name, content_type,
			minio_bucket, minio_object_key, file_size, ocr_status, parse_status, created_by, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`,
		attachment.ID, attachment.SessionID, attachment.MessageID, attachment.RunID, attachment.FileID,
		attachment.OriginalName, attachment.ContentType, attachment.MinioBucket, attachment.MinioObjectKey,
		attachment.FileSize, attachment.OCRStatus, attachment.ParseStatus, attachment.CreatedBy, attachment.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create ai chat attachment: %w", err)
	}
	return nil
}
