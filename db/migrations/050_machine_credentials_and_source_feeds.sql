-- 050_machine_credentials_and_source_feeds.sql
-- Batch F6: Machine Credentials & Source Feed Layer (PRD §3.F6 / CodebaseDesign §11)

CREATE TABLE IF NOT EXISTS machine_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    legal_entity_id VARCHAR(64) NOT NULL,
    name VARCHAR(128) NOT NULL,
    client_id VARCHAR(64) NOT NULL UNIQUE,
    secret_hash VARCHAR(128) NOT NULL,
    scopes TEXT[] NOT NULL DEFAULT '{"operating_facts:write"}',
    expires_at TIMESTAMP WITH TIME ZONE,
    revoked_at TIMESTAMP WITH TIME ZONE,
    last_used_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_machine_credentials_lookup 
ON machine_credentials(client_id, legal_entity_id);

CREATE TABLE IF NOT EXISTS source_feed_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    legal_entity_id VARCHAR(64) NOT NULL,
    name VARCHAR(128) NOT NULL,
    feed_type VARCHAR(32) NOT NULL, -- api_push, sftp_s3, self_service
    config_json JSONB NOT NULL DEFAULT '{}',
    status VARCHAR(32) NOT NULL DEFAULT 'active', -- active, paused, archived
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_source_feed_configs_lookup 
ON source_feed_configs(legal_entity_id, status);
