-- Persisted idempotency records for draft application commands.
-- The business write and this record are committed in the same transaction by
-- core-service/internal/services/draftapp.PostgresUnitOfWork.

CREATE TABLE IF NOT EXISTS agent_draft_idempotency (
    operation VARCHAR(120) NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL,
    result JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (operation, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_agent_draft_idempotency_created
    ON agent_draft_idempotency(created_at DESC);
