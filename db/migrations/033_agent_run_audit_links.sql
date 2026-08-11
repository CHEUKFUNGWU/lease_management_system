-- Migration 033: explicit cross-system Run/Artifact/business record links.

CREATE TABLE IF NOT EXISTS agent_run_audit_links (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id UUID NOT NULL REFERENCES ai_chat_runs(id) ON DELETE CASCADE,
    artifact_id UUID REFERENCES ai_chat_artifacts(id) ON DELETE SET NULL,
    business_table VARCHAR(100) NOT NULL,
    business_record_id VARCHAR(150) NOT NULL,
    relation VARCHAR(50) NOT NULL,
    item_status VARCHAR(30),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (run_id, artifact_id, business_table, business_record_id, relation)
);

CREATE INDEX IF NOT EXISTS idx_agent_run_audit_links_run
    ON agent_run_audit_links(run_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_run_audit_links_business
    ON agent_run_audit_links(business_table, business_record_id);
