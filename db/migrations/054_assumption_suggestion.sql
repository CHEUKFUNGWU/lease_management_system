-- 054_assumption_suggestion.sql — SM5 AI 假设建议（PRD S4-2/S4-3）
-- AI suggestions live as draft rows with source=ai_suggestion, an evidence
-- array and a derived confidence. They never enter a formal run until a
-- human confirms them (status flips to approved by the human path only).

ALTER TABLE fpna_assumption_versions
    ADD COLUMN IF NOT EXISTS evidence JSONB NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE fpna_assumption_versions
    ADD COLUMN IF NOT EXISTS confidence DOUBLE PRECISION;
