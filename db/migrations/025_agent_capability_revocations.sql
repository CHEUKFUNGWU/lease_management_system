-- Durable capability metadata and revocation state. Raw JWTs are never stored.

CREATE TABLE IF NOT EXISTS agent_capability_grants (
    token_id VARCHAR(120) PRIMARY KEY,
    run_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    revoked_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_capability_grants_run
    ON agent_capability_grants(run_id, expires_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_capability_grants_expiry
    ON agent_capability_grants(expires_at);
