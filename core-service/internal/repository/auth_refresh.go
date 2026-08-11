package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrRefreshTokenInvalid = errors.New("refresh token session is invalid or already revoked")

type AuthRefreshSession struct {
	ID         string
	UserID     string
	TokenID    string
	TokenHash  string
	ExpiresAt  time.Time
	CreatedAt  time.Time
	RevokedAt  *time.Time
	ReplacedBy *string
	IPAddress  string
	UserAgent  string
}

// AuthRefreshRepository persists only a hash of the refresh credential. The
// signed JWT remains the transport credential, while this row provides
// one-time rotation and explicit revocation/device-session control.
type AuthRefreshRepository struct {
	db interface {
		DBTX
		Begin(context.Context) (pgx.Tx, error)
	}
}

func NewAuthRefreshRepository(db interface {
	DBTX
	Begin(context.Context) (pgx.Tx, error)
}) *AuthRefreshRepository {
	return &AuthRefreshRepository{db: db}
}

func (r *AuthRefreshRepository) Create(ctx context.Context, session *AuthRefreshSession) error {
	if r == nil || r.db == nil {
		return errors.New("auth refresh repository unavailable")
	}
	if session == nil || strings.TrimSpace(session.UserID) == "" || strings.TrimSpace(session.TokenID) == "" || strings.TrimSpace(session.TokenHash) == "" {
		return errors.New("auth refresh session identity is required")
	}
	if session.ID == "" {
		session.ID = uuid.NewString()
	}
	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now().UTC()
	}
	return r.create(ctx, r.db, session)
}

func (r *AuthRefreshRepository) create(ctx context.Context, db DBTX, session *AuthRefreshSession) error {
	_, err := db.Exec(ctx, `
		INSERT INTO auth_refresh_sessions
			(id, user_id, token_id, token_hash, expires_at, created_at, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, '')::inet, NULLIF($8, ''))
	`, session.ID, session.UserID, session.TokenID, session.TokenHash, session.ExpiresAt, session.CreatedAt, session.IPAddress, session.UserAgent)
	if err != nil {
		return fmt.Errorf("create auth refresh session: %w", err)
	}
	return nil
}

// Rotate consumes the old credential and creates its replacement in one
// transaction, so a concurrent refresh cannot either replay the old token or
// leave a revoked token without a valid replacement.
func (r *AuthRefreshRepository) Rotate(ctx context.Context, tokenID, tokenHash string, replacement *AuthRefreshSession) error {
	if r == nil || r.db == nil {
		return errors.New("auth refresh repository unavailable")
	}
	if replacement == nil {
		return errors.New("replacement refresh session is required")
	}
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin auth refresh rotation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `
		UPDATE auth_refresh_sessions
		SET revoked_at = NOW(), replaced_by = $3
		WHERE token_id = $1 AND token_hash = $2
		  AND revoked_at IS NULL AND expires_at > NOW()
	`, strings.TrimSpace(tokenID), strings.TrimSpace(tokenHash), replacement.TokenID)
	if err != nil {
		return fmt.Errorf("consume auth refresh session for rotation: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrRefreshTokenInvalid
	}
	if err := r.create(ctx, tx, replacement); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Consume atomically marks a refresh credential as used. Rotation must never
// accept the same token twice, even when two refresh requests race.
func (r *AuthRefreshRepository) Consume(ctx context.Context, tokenID, tokenHash string, replacedBy string) error {
	if r == nil || r.db == nil {
		return errors.New("auth refresh repository unavailable")
	}
	result, err := r.db.Exec(ctx, `
		UPDATE auth_refresh_sessions
		SET revoked_at = NOW(), replaced_by = NULLIF($3, '')
		WHERE token_id = $1 AND token_hash = $2
		  AND revoked_at IS NULL AND expires_at > NOW()
	`, strings.TrimSpace(tokenID), strings.TrimSpace(tokenHash), strings.TrimSpace(replacedBy))
	if err != nil {
		return fmt.Errorf("consume auth refresh session: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrRefreshTokenInvalid
	}
	return nil
}

func (r *AuthRefreshRepository) Revoke(ctx context.Context, tokenID, tokenHash string) error {
	if r == nil || r.db == nil {
		return errors.New("auth refresh repository unavailable")
	}
	result, err := r.db.Exec(ctx, `
		UPDATE auth_refresh_sessions
		SET revoked_at = COALESCE(revoked_at, NOW())
		WHERE token_id = $1 AND token_hash = $2
	`, strings.TrimSpace(tokenID), strings.TrimSpace(tokenHash))
	if err != nil {
		return fmt.Errorf("revoke auth refresh session: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrRefreshTokenInvalid
	}
	return nil
}

func (r *AuthRefreshRepository) RevokeAll(ctx context.Context, userID string) error {
	if r == nil || r.db == nil {
		return errors.New("auth refresh repository unavailable")
	}
	_, err := r.db.Exec(ctx, `
		UPDATE auth_refresh_sessions
		SET revoked_at = COALESCE(revoked_at, NOW())
		WHERE user_id = $1 AND revoked_at IS NULL
	`, strings.TrimSpace(userID))
	if err != nil {
		return fmt.Errorf("revoke all auth refresh sessions: %w", err)
	}
	return nil
}

func (r *AuthRefreshRepository) List(ctx context.Context, userID string) ([]*AuthRefreshSession, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("auth refresh repository unavailable")
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, user_id, token_id, expires_at, created_at, revoked_at, replaced_by,
		       COALESCE(ip_address::text, ''), COALESCE(user_agent, '')
		FROM auth_refresh_sessions
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, strings.TrimSpace(userID))
	if err != nil {
		return nil, fmt.Errorf("list auth refresh sessions: %w", err)
	}
	defer rows.Close()
	var sessions []*AuthRefreshSession
	for rows.Next() {
		var session AuthRefreshSession
		if err := rows.Scan(&session.ID, &session.UserID, &session.TokenID, &session.ExpiresAt, &session.CreatedAt, &session.RevokedAt, &session.ReplacedBy, &session.IPAddress, &session.UserAgent); err != nil {
			return nil, fmt.Errorf("scan auth refresh session: %w", err)
		}
		sessions = append(sessions, &session)
	}
	return sessions, rows.Err()
}

func (r *AuthRefreshRepository) RevokeByID(ctx context.Context, userID, sessionID string) error {
	if r == nil || r.db == nil {
		return errors.New("auth refresh repository unavailable")
	}
	result, err := r.db.Exec(ctx, `
		UPDATE auth_refresh_sessions
		SET revoked_at = COALESCE(revoked_at, NOW())
		WHERE id = $1 AND user_id = $2
	`, strings.TrimSpace(sessionID), strings.TrimSpace(userID))
	if err != nil {
		return fmt.Errorf("revoke auth refresh session by id: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ErrRefreshTokenInvalid
	}
	return nil
}

// CleanupExpired removes credentials that can no longer be exchanged. The
// signed JWT is never stored, so deleting this server-side session row cannot
// invalidate a still-valid credential; expiry has already made it unusable.
// This keeps long-running installations from accumulating device sessions.
func (r *AuthRefreshRepository) CleanupExpired(ctx context.Context, before time.Time) (int64, error) {
	if r == nil || r.db == nil {
		return 0, errors.New("auth refresh repository unavailable")
	}
	if before.IsZero() {
		before = time.Now().UTC()
	}
	result, err := r.db.Exec(ctx, `
		DELETE FROM auth_refresh_sessions
		WHERE expires_at <= $1
	`, before)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired auth refresh sessions: %w", err)
	}
	return result.RowsAffected(), nil
}
