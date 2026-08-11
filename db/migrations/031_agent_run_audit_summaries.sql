-- Migration 031: durable, bounded Run audit summary read model.

CREATE TABLE IF NOT EXISTS agent_run_audit_summaries (
    run_id UUID PRIMARY KEY REFERENCES ai_chat_runs(id) ON DELETE CASCADE,
    session_id UUID NOT NULL REFERENCES ai_chat_sessions(id) ON DELETE CASCADE,
    event_count INTEGER NOT NULL DEFAULT 0,
    tool_event_count INTEGER NOT NULL DEFAULT 0,
    failed_event_count INTEGER NOT NULL DEFAULT 0,
    review_event_count INTEGER NOT NULL DEFAULT 0,
    artifact_count INTEGER NOT NULL DEFAULT 0,
    last_sequence INTEGER NOT NULL DEFAULT 0,
    last_event_type VARCHAR(100),
    terminal BOOLEAN NOT NULL DEFAULT FALSE,
    last_event_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

INSERT INTO agent_run_audit_summaries (
    run_id, session_id, event_count, tool_event_count, failed_event_count,
    review_event_count, artifact_count, last_sequence, last_event_type,
    terminal, last_event_at, updated_at
)
SELECT e.run_id, e.session_id,
       COUNT(*),
       COUNT(*) FILTER (WHERE e.event_type LIKE 'tool_%' OR e.event_type = 'tool_execution'),
       COUNT(*) FILTER (WHERE e.event_type IN ('run_failed', 'run_error', 'tool_error')),
       COUNT(*) FILTER (WHERE e.event_type IN ('review_prompt', 'artifact_ready')),
       (SELECT COUNT(*) FROM ai_chat_artifacts a WHERE a.run_id = e.run_id),
       MAX(e.sequence_no),
       (array_agg(e.event_type ORDER BY e.sequence_no DESC))[1],
       BOOL_OR(e.is_terminal), MAX(e.created_at), NOW()
FROM ai_chat_run_events e
GROUP BY e.run_id, e.session_id
ON CONFLICT (run_id) DO UPDATE SET
    session_id = EXCLUDED.session_id,
    event_count = EXCLUDED.event_count,
    tool_event_count = EXCLUDED.tool_event_count,
    failed_event_count = EXCLUDED.failed_event_count,
    review_event_count = EXCLUDED.review_event_count,
    artifact_count = EXCLUDED.artifact_count,
    last_sequence = EXCLUDED.last_sequence,
    last_event_type = EXCLUDED.last_event_type,
    terminal = EXCLUDED.terminal,
    last_event_at = EXCLUDED.last_event_at,
    updated_at = EXCLUDED.updated_at;

CREATE INDEX IF NOT EXISTS idx_agent_run_audit_summaries_session
    ON agent_run_audit_summaries(session_id, updated_at DESC);
