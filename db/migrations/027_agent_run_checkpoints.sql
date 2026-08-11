-- Migration 027: durable Runner checkpoint metadata on the owned Core Run.

ALTER TABLE ai_chat_runs
    ADD COLUMN IF NOT EXISTS checkpoint JSONB;

CREATE INDEX IF NOT EXISTS idx_ai_chat_runs_checkpoint_updated
	ON ai_chat_runs(created_at DESC)
    WHERE checkpoint IS NOT NULL;
