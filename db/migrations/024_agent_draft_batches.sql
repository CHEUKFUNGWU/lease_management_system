-- Persisted batch envelope for partial draft recovery.

CREATE TABLE IF NOT EXISTS agent_draft_batches (
    batch_id UUID PRIMARY KEY,
    operation VARCHAR(120) NOT NULL,
    status VARCHAR(30) NOT NULL,
    items JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_by VARCHAR(255),
    run_id VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (status IN ('in_progress', 'completed', 'partial_failed', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_agent_draft_batches_status_updated
    ON agent_draft_batches(status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_draft_batches_run
    ON agent_draft_batches(run_id, updated_at DESC);
