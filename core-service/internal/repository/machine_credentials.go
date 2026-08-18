package repository

import (
	"context"
	"fmt"
	"time"
)

type MachineCredential struct {
	ID            string     `json:"id"`
	LegalEntityID string     `json:"legal_entity_id"`
	Name          string     `json:"name"`
	ClientID      string     `json:"client_id"`
	SecretHash    string     `json:"-"`
	Scopes        []string   `json:"scopes"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt    *time.Time `json:"last_used_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type SourceFeedConfig struct {
	ID            string    `json:"id"`
	LegalEntityID string    `json:"legal_entity_id"`
	Name          string    `json:"name"`
	FeedType      string    `json:"feed_type"`
	ConfigJSON    string    `json:"config_json"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
}

type MachineCredentialRepository struct {
	db DBTX
}

func NewMachineCredentialRepository(db DBTX) *MachineCredentialRepository {
	return &MachineCredentialRepository{db: db}
}

func (r *MachineCredentialRepository) ListCredentials(ctx context.Context, legalEntityID string) ([]MachineCredential, error) {
	query := `
		SELECT id, legal_entity_id, name, client_id, secret_hash, scopes,
		       expires_at, revoked_at, last_used_at, created_at
		FROM machine_credentials
		WHERE legal_entity_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, legalEntityID)
	if err != nil {
		return nil, fmt.Errorf("list machine credentials: %w", err)
	}
	defer rows.Close()

	var list []MachineCredential
	for rows.Next() {
		var c MachineCredential
		if err := rows.Scan(
			&c.ID, &c.LegalEntityID, &c.Name, &c.ClientID, &c.SecretHash, &c.Scopes,
			&c.ExpiresAt, &c.RevokedAt, &c.LastUsedAt, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan credential: %w", err)
		}
		list = append(list, c)
	}
	return list, nil
}

func (r *MachineCredentialRepository) GetCredentialByClientID(ctx context.Context, clientID string) (*MachineCredential, error) {
	query := `
		SELECT id, legal_entity_id, name, client_id, secret_hash, scopes,
		       expires_at, revoked_at, last_used_at, created_at
		FROM machine_credentials
		WHERE client_id = $1
	`
	var c MachineCredential
	err := r.db.QueryRow(ctx, query, clientID).Scan(
		&c.ID, &c.LegalEntityID, &c.Name, &c.ClientID, &c.SecretHash, &c.Scopes,
		&c.ExpiresAt, &c.RevokedAt, &c.LastUsedAt, &c.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get machine credential: %w", err)
	}
	return &c, nil
}

func (r *MachineCredentialRepository) CreateCredential(ctx context.Context, c *MachineCredential) error {
	query := `
		INSERT INTO machine_credentials (
			legal_entity_id, name, client_id, secret_hash, scopes, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`
	return r.db.QueryRow(
		ctx, query,
		c.LegalEntityID, c.Name, c.ClientID, c.SecretHash, c.Scopes, c.ExpiresAt,
	).Scan(&c.ID, &c.CreatedAt)
}

func (r *MachineCredentialRepository) RevokeCredential(ctx context.Context, legalEntityID, clientID string) error {
	query := `
		UPDATE machine_credentials
		SET revoked_at = CURRENT_TIMESTAMP
		WHERE legal_entity_id = $1 AND client_id = $2 AND revoked_at IS NULL
	`
	_, err := r.db.Exec(ctx, query, legalEntityID, clientID)
	return err
}

func (r *MachineCredentialRepository) TouchCredentialUsage(ctx context.Context, clientID string) error {
	query := `
		UPDATE machine_credentials
		SET last_used_at = CURRENT_TIMESTAMP
		WHERE client_id = $1
	`
	_, err := r.db.Exec(ctx, query, clientID)
	return err
}

func (r *MachineCredentialRepository) ListFeedConfigs(ctx context.Context, legalEntityID string) ([]SourceFeedConfig, error) {
	query := `
		SELECT id, legal_entity_id, name, feed_type, config_json, status, created_at
		FROM source_feed_configs
		WHERE legal_entity_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, legalEntityID)
	if err != nil {
		return nil, fmt.Errorf("list feed configs: %w", err)
	}
	defer rows.Close()

	var list []SourceFeedConfig
	for rows.Next() {
		var f SourceFeedConfig
		if err := rows.Scan(&f.ID, &f.LegalEntityID, &f.Name, &f.FeedType, &f.ConfigJSON, &f.Status, &f.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan feed config: %w", err)
		}
		list = append(list, f)
	}
	return list, nil
}

func (r *MachineCredentialRepository) CreateFeedConfig(ctx context.Context, f *SourceFeedConfig) error {
	query := `
		INSERT INTO source_feed_configs (
			legal_entity_id, name, feed_type, config_json, status
		) VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`
	return r.db.QueryRow(
		ctx, query,
		f.LegalEntityID, f.Name, f.FeedType, f.ConfigJSON, f.Status,
	).Scan(&f.ID, &f.CreatedAt)
}
