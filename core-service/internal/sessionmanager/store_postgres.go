package sessionmanager

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lease-management-system/core-service/internal/agentcontext"
)

// PostgresStore adapts ai_chat_sessions to the Store port. Ownership
// enforcement happens here, at the data boundary: Load locates the row by the
// key's session id and refuses with ErrScopeDenied when its legal entity or
// user disagrees with the key — the cross-tenant rejection keeps its reason
// instead of softening into "not found".
type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

// Load locates a session row by the key's session id and checks ownership.
//
// Rows with a NULL legal_entity_id (legacy sessions created outside any
// tenant) can never match a ContextKey — the key always carries an entity —
// so they refuse with ErrScopeDenied like any other foreign row.
func (s *PostgresStore) Load(ctx context.Context, key agentcontext.ContextKey) (*Session, error) {
	var (
		row    Session
		entity *string
	)
	err := s.pool.QueryRow(ctx, `
		SELECT id::text, user_id::text,
		       COALESCE(legal_entity_id::text, ''),
		       data_classification, title, status, created_at, updated_at
		FROM ai_chat_sessions
		WHERE id = $1::uuid`,
		key.SessionID(),
	).Scan(&row.SessionID, &row.UserID, &entity, &row.Classification, &row.Title, &row.Status, &row.CreatedAt, &row.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, key.SessionID())
	}
	if err != nil {
		return nil, fmt.Errorf("load ai chat session %s: %w", key.SessionID(), err)
	}
	if entity != nil {
		row.LegalEntityID = *entity
	}
	return ownershipChecked(&row, key)
}

// Save inserts or updates the module-owned columns. The session id comes from
// the key, never from the caller; legal_entity_id/user_id/classification are
// written from the KEY on insert and never updated afterwards — ownership is
// immutable once established.
//
// The UPDATE branch carries an ownership predicate: a key that locates a row
// owned by another entity or user matches zero rows and the save refuses with
// ErrScopeDenied instead of silently editing someone else's conversation.
func (s *PostgresStore) Save(ctx context.Context, key agentcontext.ContextKey, sess *Session) error {
	// 单一 SQL 形状覆盖两种键。参数类型纪律：每个 $N 全句只推断一种类型，
	// 避免 pgx 42P18（同一参数双 cast 无法确定类型）。
	//  $1/$2（id、user_id）恒为 uuid：WHERE 侧 cast 列而非参数。
	//  $3 恒为 text（法人 id 或 global 的空串）：VALUES 写 NULLIF($3,'')::uuid
	//  —— global 键的空串在此变为 SQL NULL（而非非法 uuid），scoped 键原样
	//  落库；WHERE 用 COALESCE(col,'') = $3 —— global 键（''）只认 NULL 法人行
	//  （SEC-004 全局 filter 含 NULL 行的语义），scoped 键认精确法人。
	tag, err := s.pool.Exec(ctx, `
		INSERT INTO ai_chat_sessions
			(id, user_id, legal_entity_id, title, status, data_classification, updated_at)
		VALUES ($1::uuid, $2::uuid, NULLIF($3, '')::uuid, $4, $5, $6, NOW())
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			status = EXCLUDED.status,
			data_classification = EXCLUDED.data_classification,
			updated_at = NOW()
		WHERE ai_chat_sessions.user_id = $2::uuid
		  AND COALESCE(ai_chat_sessions.legal_entity_id::text, '') = $3`,
		key.SessionID(), key.UserID(), key.LegalEntityID(), sess.Title, sess.Status, key.Classification(),
	)
	if err != nil {
		return fmt.Errorf("save ai chat session %s: %w", key.SessionID(), err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: session %s is owned by another legal entity or user", ErrScopeDenied, key.SessionID())
	}
	sess.LegalEntityID = key.LegalEntityID()
	sess.UserID = key.UserID()
	sess.SessionID = key.SessionID()
	if key.Classification() != "" {
		sess.Classification = key.Classification()
	}
	return nil
}

// ownershipChecked refuses rows whose stored owner disagrees with the key.
// Global keys own NULL-entity rows (the only rows they can own); scoped keys
// refuse NULL and foreign rows alike.
func ownershipChecked(row *Session, key agentcontext.ContextKey) (*Session, error) {
	if row.LegalEntityID != key.LegalEntityID() || row.UserID != key.UserID() {
		keyState := "global"
		if !key.IsGlobal() {
			keyState = "scoped"
		}
		return nil, fmt.Errorf("%w (session owner entity=%q user=%q; key %s entity=%q)",
			ErrScopeDenied, row.LegalEntityID, maskUser(row.UserID), keyState, key.LegalEntityID())
	}
	return row, nil
}

// maskUser keeps the refusal log useful without echoing a full foreign user id.
func maskUser(userID string) string {
	if len(userID) <= 8 {
		return userID
	}
	return userID[:8] + "…"
}

var _ Store = (*PostgresStore)(nil)
