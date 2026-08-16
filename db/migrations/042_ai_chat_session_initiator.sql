-- CHAT-001: mark sessions created by system-initiated runs (e.g. the home
-- brief) so the user-facing session list can filter them out while the run,
-- message and audit records stay fully intact. Existing rows default to
-- 'user' — nothing before this migration was system-initiated, so no backfill
-- invents data.
ALTER TABLE ai_chat_sessions
    ADD COLUMN IF NOT EXISTS initiator VARCHAR(20) NOT NULL DEFAULT 'user',
    ADD CONSTRAINT ai_chat_sessions_initiator_check CHECK (initiator IN ('user', 'system'));
