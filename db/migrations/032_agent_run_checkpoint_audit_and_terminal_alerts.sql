-- Migration 032: immutable checkpoint audit index and terminal alert outbox.
-- The checkpoint payload remains on ai_chat_runs; this table stores only
-- metadata needed for replay and audit without duplicating model/tool output.

CREATE TABLE IF NOT EXISTS agent_run_checkpoint_audits (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id UUID NOT NULL REFERENCES ai_chat_runs(id) ON DELETE CASCADE,
    session_id UUID NOT NULL REFERENCES ai_chat_sessions(id) ON DELETE CASCADE,
    checkpoint_hash CHAR(64) NOT NULL,
    checkpoint_size_bytes INTEGER NOT NULL CHECK (checkpoint_size_bytes >= 0),
    schema_version VARCHAR(100),
    checkpoint_status VARCHAR(30) NOT NULL DEFAULT 'saved',
    next_index INTEGER CHECK (next_index IS NULL OR next_index >= 0),
    actor_id UUID,
    worker_id VARCHAR(150),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_run_checkpoint_audits_run_created
    ON agent_run_checkpoint_audits(run_id, created_at DESC);

CREATE TABLE IF NOT EXISTS agent_run_terminal_alerts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id UUID NOT NULL UNIQUE REFERENCES ai_chat_runs(id) ON DELETE CASCADE,
    session_id UUID NOT NULL REFERENCES ai_chat_sessions(id) ON DELETE CASCADE,
    terminal_status VARCHAR(30) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    error_message TEXT,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'acknowledged')),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    acknowledged_at TIMESTAMP WITH TIME ZONE,
    acknowledged_by UUID
);

CREATE INDEX IF NOT EXISTS idx_agent_run_terminal_alerts_status_created
    ON agent_run_terminal_alerts(status, created_at DESC);

ALTER TABLE agent_run_audit_summaries
    ADD COLUMN IF NOT EXISTS checkpoint_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_checkpoint_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS terminal_status VARCHAR(30),
    ADD COLUMN IF NOT EXISTS terminal_error TEXT;

ALTER TABLE ai_chat_run_events
    DROP CONSTRAINT IF EXISTS ai_chat_run_events_event_type_check;

ALTER TABLE ai_chat_run_events
    ADD CONSTRAINT ai_chat_run_events_event_type_check CHECK (event_type IN (
        'message_start', 'message_delta', 'message_end',
        'tool_start', 'tool_update', 'tool_end', 'tool_execution',
        'tool_started', 'tool_completed',
        'review_prompt', 'artifact_ready', 'queue_update',
        'run_status', 'run_started', 'run_resumed', 'run_steered',
        'run_steer', 'run_follow_up', 'run_branch_created',
        'planner_usage',
        'checkpoint_saved', 'checkpoint_restored',
        'run_end', 'run_finished', 'run_error', 'run_failed', 'run_cancelled'
    ));
