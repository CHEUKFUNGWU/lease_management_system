-- AI Chat runtime persistence: server-side sessions, runs, events, artifacts, and review actions.

CREATE TABLE IF NOT EXISTS ai_chat_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    legal_entity_id UUID REFERENCES legal_entities(id),
    title VARCHAR(255) NOT NULL DEFAULT '新会话',
    status VARCHAR(30) NOT NULL DEFAULT 'active',
    bound_contract_id UUID REFERENCES lease_contracts(id),
    context_snapshot JSONB,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_message_at TIMESTAMP WITH TIME ZONE,
    archived_at TIMESTAMP WITH TIME ZONE,
    CHECK (status IN ('active', 'archived', 'closed'))
);

CREATE TABLE IF NOT EXISTS ai_chat_runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES ai_chat_sessions(id) ON DELETE CASCADE,
    trigger_message_id UUID,
    parent_run_id UUID REFERENCES ai_chat_runs(id),
    status VARCHAR(30) NOT NULL DEFAULT 'queued',
    agent_mode BOOLEAN NOT NULL DEFAULT false,
    skill_id VARCHAR(100),
    skill_version VARCHAR(50),
    page_context JSONB,
    review_required BOOLEAN NOT NULL DEFAULT false,
    summary_text TEXT,
    error_message TEXT,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    CHECK (status IN ('queued', 'running', 'waiting_review', 'completed', 'failed', 'cancelled', 'aborted'))
);

CREATE TABLE IF NOT EXISTS ai_chat_messages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES ai_chat_sessions(id) ON DELETE CASCADE,
    run_id UUID REFERENCES ai_chat_runs(id) ON DELETE SET NULL,
    role VARCHAR(20) NOT NULL,
    message_type VARCHAR(30) NOT NULL DEFAULT 'text',
    sequence_no INTEGER NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    content_json JSONB,
    sources JSONB,
    attachments JSONB,
    model VARCHAR(100),
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (role IN ('system', 'user', 'assistant', 'tool')),
    CHECK (message_type IN ('text', 'thinking', 'tool_result', 'review_prompt', 'artifact_notice')),
    UNIQUE(session_id, sequence_no)
);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'fk_ai_chat_runs_trigger_message'
    ) THEN
        ALTER TABLE ai_chat_runs
            ADD CONSTRAINT fk_ai_chat_runs_trigger_message
            FOREIGN KEY (trigger_message_id) REFERENCES ai_chat_messages(id) ON DELETE SET NULL;
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS ai_chat_run_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id UUID NOT NULL REFERENCES ai_chat_runs(id) ON DELETE CASCADE,
    session_id UUID NOT NULL REFERENCES ai_chat_sessions(id) ON DELETE CASCADE,
    sequence_no INTEGER NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_terminal BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (event_type IN (
        'message_start', 'message_delta', 'message_end',
        'tool_start', 'tool_update', 'tool_end',
        'review_prompt', 'artifact_ready', 'queue_update',
        'run_status', 'run_end', 'run_error'
    )),
    UNIQUE(run_id, sequence_no)
);

CREATE TABLE IF NOT EXISTS ai_chat_artifacts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES ai_chat_sessions(id) ON DELETE CASCADE,
    run_id UUID NOT NULL REFERENCES ai_chat_runs(id) ON DELETE CASCADE,
    artifact_type VARCHAR(50) NOT NULL,
    title VARCHAR(255) NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'ready',
    data JSONB NOT NULL DEFAULT '{}'::jsonb,
    actions JSONB,
    evidence_refs JSONB,
    review_required BOOLEAN NOT NULL DEFAULT false,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (artifact_type IN (
        'contract_draft',
        'payment_schedule_draft',
        'audit_pack',
        'data_quality_issue_list',
        'report_explanation',
        'monthly_close_blockers',
        'generic'
    )),
    CHECK (status IN ('draft', 'ready', 'confirmed', 'rejected', 'archived'))
);

CREATE TABLE IF NOT EXISTS ai_chat_review_actions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES ai_chat_sessions(id) ON DELETE CASCADE,
    run_id UUID REFERENCES ai_chat_runs(id) ON DELETE CASCADE,
    artifact_id UUID REFERENCES ai_chat_artifacts(id) ON DELETE CASCADE,
    action_type VARCHAR(50) NOT NULL,
    action_payload JSONB,
    comment TEXT,
    acted_by UUID NOT NULL REFERENCES users(id),
    acted_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (action_type IN (
        'confirm',
        'reject',
        'edit',
        'skip',
        'retry',
        'import',
        'create_draft',
        'custom'
    ))
);

CREATE TABLE IF NOT EXISTS ai_chat_attachments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES ai_chat_sessions(id) ON DELETE CASCADE,
    message_id UUID REFERENCES ai_chat_messages(id) ON DELETE SET NULL,
    run_id UUID REFERENCES ai_chat_runs(id) ON DELETE SET NULL,
    file_id VARCHAR(255) NOT NULL,
    original_name VARCHAR(255) NOT NULL,
    content_type VARCHAR(150) NOT NULL,
    minio_bucket VARCHAR(100),
    minio_object_key VARCHAR(500),
    file_size BIGINT,
    ocr_status VARCHAR(30) NOT NULL DEFAULT 'pending',
    parse_status VARCHAR(30) NOT NULL DEFAULT 'pending',
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (ocr_status IN ('pending', 'processing', 'completed', 'failed', 'skipped')),
    CHECK (parse_status IN ('pending', 'processing', 'completed', 'failed', 'skipped'))
);

CREATE INDEX IF NOT EXISTS idx_ai_chat_sessions_user_updated ON ai_chat_sessions(user_id, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_chat_sessions_legal_entity ON ai_chat_sessions(legal_entity_id);
CREATE INDEX IF NOT EXISTS idx_ai_chat_sessions_contract ON ai_chat_sessions(bound_contract_id);

CREATE INDEX IF NOT EXISTS idx_ai_chat_runs_session_created ON ai_chat_runs(session_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_chat_runs_status ON ai_chat_runs(status);
CREATE INDEX IF NOT EXISTS idx_ai_chat_runs_skill ON ai_chat_runs(skill_id);

CREATE INDEX IF NOT EXISTS idx_ai_chat_messages_session_sequence ON ai_chat_messages(session_id, sequence_no);
CREATE INDEX IF NOT EXISTS idx_ai_chat_messages_run ON ai_chat_messages(run_id);
CREATE INDEX IF NOT EXISTS idx_ai_chat_messages_created_at ON ai_chat_messages(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_ai_chat_run_events_run_sequence ON ai_chat_run_events(run_id, sequence_no);
CREATE INDEX IF NOT EXISTS idx_ai_chat_run_events_session_created ON ai_chat_run_events(session_id, created_at);
CREATE INDEX IF NOT EXISTS idx_ai_chat_run_events_type ON ai_chat_run_events(event_type);

CREATE INDEX IF NOT EXISTS idx_ai_chat_artifacts_run ON ai_chat_artifacts(run_id);
CREATE INDEX IF NOT EXISTS idx_ai_chat_artifacts_session ON ai_chat_artifacts(session_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_chat_artifacts_type_status ON ai_chat_artifacts(artifact_type, status);

CREATE INDEX IF NOT EXISTS idx_ai_chat_review_actions_run ON ai_chat_review_actions(run_id, acted_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_chat_review_actions_artifact ON ai_chat_review_actions(artifact_id, acted_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_chat_review_actions_actor ON ai_chat_review_actions(acted_by, acted_at DESC);

CREATE INDEX IF NOT EXISTS idx_ai_chat_attachments_session ON ai_chat_attachments(session_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ai_chat_attachments_message ON ai_chat_attachments(message_id);
CREATE INDEX IF NOT EXISTS idx_ai_chat_attachments_run ON ai_chat_attachments(run_id);
CREATE INDEX IF NOT EXISTS idx_ai_chat_attachments_file_id ON ai_chat_attachments(file_id);
