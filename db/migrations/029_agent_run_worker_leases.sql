-- Migration 029: durable Agent Run worker leases
-- A lease is owned by a worker identity and opaque token. Expired running
-- Runs are eligible for a later atomic claim, which gives process restarts a
-- safe recovery seam without letting two workers execute the same Run.

ALTER TABLE ai_chat_runs
    ADD COLUMN IF NOT EXISTS worker_id VARCHAR(150),
    ADD COLUMN IF NOT EXISTS lease_token VARCHAR(120),
    ADD COLUMN IF NOT EXISTS leased_until TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS heartbeat_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS attempt_count INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_ai_chat_runs_worker_lease
    ON ai_chat_runs(status, leased_until, created_at)
    WHERE status IN ('queued', 'running');

