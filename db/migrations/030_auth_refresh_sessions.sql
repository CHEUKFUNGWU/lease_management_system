-- Migration 030: persistent, one-time refresh-token sessions.
-- Only a SHA-256 token hash is stored; the signed refresh JWT is never stored.

CREATE TABLE IF NOT EXISTS auth_refresh_sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_id UUID NOT NULL UNIQUE,
    token_hash VARCHAR(128) NOT NULL UNIQUE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMP WITH TIME ZONE,
    replaced_by UUID,
    ip_address INET,
    user_agent TEXT
);

CREATE INDEX IF NOT EXISTS idx_auth_refresh_sessions_user_active
    ON auth_refresh_sessions(user_id, expires_at)
    WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_auth_refresh_sessions_expiry
    ON auth_refresh_sessions(expires_at);
