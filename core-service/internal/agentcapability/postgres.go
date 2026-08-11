package agentcapability

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore persists only capability metadata and revocation timestamps.
// It is deliberately kept behind RevocationStore so the signer remains
// usable in unit tests and in deployments that use another durable store.
type PostgresStore struct {
	pool *pgxpool.Pool
}

type RevocationStats struct {
	Active  int64 `json:"active"`
	Revoked int64 `json:"revoked"`
	Expired int64 `json:"expired"`
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) Register(ctx context.Context, tokenID, runID, userID string, expiresAt time.Time) error {
	if s == nil || s.pool == nil {
		return errors.New("capability revocation store is unavailable")
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO agent_capability_grants (token_id, run_id, user_id, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (token_id) DO UPDATE SET
			run_id = EXCLUDED.run_id, user_id = EXCLUDED.user_id, expires_at = EXCLUDED.expires_at
	`, strings.TrimSpace(tokenID), strings.TrimSpace(runID), strings.TrimSpace(userID), expiresAt)
	if err != nil {
		return fmt.Errorf("register capability grant: %w", err)
	}
	return nil
}

func (s *PostgresStore) RevokeToken(ctx context.Context, tokenID, userID string, expiresAt time.Time) error {
	if s == nil || s.pool == nil {
		return errors.New("capability revocation store is unavailable")
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE agent_capability_grants
		SET revoked_at = COALESCE(revoked_at, NOW()), expires_at = GREATEST(expires_at, $3)
		WHERE token_id = $1 AND ($2 = '' OR user_id = $2)
	`, strings.TrimSpace(tokenID), strings.TrimSpace(userID), expiresAt)
	if err != nil {
		return fmt.Errorf("revoke capability token: %w", err)
	}
	return nil
}

func (s *PostgresStore) RevokeRun(ctx context.Context, runID, userID string, expiresAt time.Time) error {
	if s == nil || s.pool == nil {
		return errors.New("capability revocation store is unavailable")
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE agent_capability_grants
		SET revoked_at = COALESCE(revoked_at, NOW()), expires_at = GREATEST(expires_at, $3)
		WHERE run_id = $1 AND ($2 = '' OR user_id = $2)
	`, strings.TrimSpace(runID), strings.TrimSpace(userID), expiresAt)
	if err != nil {
		return fmt.Errorf("revoke capability run: %w", err)
	}
	return nil
}

func (s *PostgresStore) IsRevoked(ctx context.Context, tokenID, runID string, now time.Time) (bool, error) {
	if s == nil || s.pool == nil {
		return false, errors.New("capability revocation store is unavailable")
	}
	var revoked bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM agent_capability_grants
			WHERE (token_id = $1 OR run_id = $2)
			  AND (expires_at <= $3 OR revoked_at IS NOT NULL)
		)
	`, strings.TrimSpace(tokenID), strings.TrimSpace(runID), now).Scan(&revoked)
	if err != nil {
		return false, fmt.Errorf("check capability revocation: %w", err)
	}
	return revoked, nil
}

func (s *PostgresStore) TokenOwner(ctx context.Context, tokenID string) (string, error) {
	if s == nil || s.pool == nil {
		return "", errors.New("capability revocation store is unavailable")
	}
	var owner string
	err := s.pool.QueryRow(ctx, `SELECT user_id FROM agent_capability_grants WHERE token_id = $1`, strings.TrimSpace(tokenID)).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("load capability token owner: %w", err)
	}
	return owner, nil
}

func (s *PostgresStore) RunOwner(ctx context.Context, runID string) (string, error) {
	if s == nil || s.pool == nil {
		return "", errors.New("capability revocation store is unavailable")
	}
	var owner string
	err := s.pool.QueryRow(ctx, `
		SELECT user_id FROM agent_capability_grants WHERE run_id = $1 ORDER BY expires_at DESC LIMIT 1
	`, strings.TrimSpace(runID)).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("load capability run owner: %w", err)
	}
	return owner, nil
}

// CleanupExpired removes only metadata whose JWT lifetime has ended. A
// capability token is already unusable after expiry, so deleting this row is
// safe across multiple Core instances and keeps the revocation table bounded.
func (s *PostgresStore) CleanupExpired(ctx context.Context, before time.Time) (int64, error) {
	if s == nil || s.pool == nil {
		return 0, errors.New("capability revocation store is unavailable")
	}
	result, err := s.pool.Exec(ctx, `
		DELETE FROM agent_capability_grants
		WHERE expires_at <= $1
	`, before)
	if err != nil {
		return 0, fmt.Errorf("cleanup expired capability grants: %w", err)
	}
	return result.RowsAffected(), nil
}

func (s *PostgresStore) Stats(ctx context.Context, now time.Time) (RevocationStats, error) {
	if s == nil || s.pool == nil {
		return RevocationStats{}, errors.New("capability revocation store is unavailable")
	}
	var stats RevocationStats
	err := s.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE revoked_at IS NULL AND expires_at > $1),
			COUNT(*) FILTER (WHERE revoked_at IS NOT NULL),
			COUNT(*) FILTER (WHERE expires_at <= $1)
		FROM agent_capability_grants
	`, now).Scan(&stats.Active, &stats.Revoked, &stats.Expired)
	if err != nil {
		return RevocationStats{}, fmt.Errorf("read capability grant stats: %w", err)
	}
	return stats, nil
}

var _ RevocationStore = (*PostgresStore)(nil)
