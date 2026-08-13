-- MAX-009: allow the existing retail action proposal Artifact contract.
-- This migration only normalizes the existing check constraint; it adds no
-- tables, columns, business writes, or audit scope.

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
        'retail_action_proposal',
        'generic'
    ));
