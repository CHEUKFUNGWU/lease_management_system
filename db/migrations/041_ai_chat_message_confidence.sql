-- FIX-001: persist the AI chat assistant message confidence and its
-- degradation reason, so a reloaded session can still render the
-- ConfidenceBadge. Existing rows keep NULL, which the API and the frontend
-- already treat as "confidence not available" — no backfill invents data.
ALTER TABLE ai_chat_messages
    ADD COLUMN IF NOT EXISTS confidence DOUBLE PRECISION,
    ADD COLUMN IF NOT EXISTS confidence_reason TEXT;
