-- Versioned Agent Artifact/Evidence protocol.

ALTER TABLE ai_chat_artifacts
    ADD COLUMN IF NOT EXISTS schema_version VARCHAR(80) NOT NULL DEFAULT 'agent-artifact.v1',
    ADD COLUMN IF NOT EXISTS evidence_complete BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS review_reasons JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS model_version VARCHAR(150),
    ADD COLUMN IF NOT EXISTS rule_version VARCHAR(150);

UPDATE ai_chat_artifacts
SET schema_version = 'agent-artifact.v1'
WHERE schema_version IS NULL OR schema_version = '';

UPDATE ai_chat_artifacts
SET evidence_refs = '[]'::jsonb
WHERE evidence_refs IS NULL;

ALTER TABLE ai_chat_artifacts
    ALTER COLUMN evidence_refs SET DEFAULT '[]'::jsonb,
    ALTER COLUMN evidence_refs SET NOT NULL;

ALTER TABLE ai_chat_artifacts
    DROP CONSTRAINT IF EXISTS ai_chat_artifacts_artifact_type_check;

ALTER TABLE ai_chat_artifacts
    ADD CONSTRAINT ai_chat_artifacts_artifact_type_check CHECK (artifact_type IN (
        'contract_draft',
        'payment_schedule_draft',
        'event_draft',
        'audit_pack',
        'data_quality_issue_list',
        'report_explanation',
        'monthly_close_blockers',
        'generic'
    ));

CREATE INDEX IF NOT EXISTS idx_ai_chat_artifacts_schema_status
    ON ai_chat_artifacts(schema_version, status);
