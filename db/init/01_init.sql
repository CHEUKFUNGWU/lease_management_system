-- IFRS 16 Auto-Init Script
-- This runs automatically when PostgreSQL container starts with an empty data directory.
-- Combines all goose migrations (Up sections only) from 001, 002, 003.

-- ============================================================================
-- Migration 001: Initial Schema
-- ============================================================================

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(255) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'user',
    legal_entity_id UUID,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS roles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS permissions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    resource VARCHAR(255) NOT NULL,
    action VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(role_id, resource, action)
);

CREATE TABLE IF NOT EXISTS legal_entities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    country VARCHAR(50),
    currency VARCHAR(10) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS stores (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    legal_entity_id UUID NOT NULL REFERENCES legal_entities(id),
    brand VARCHAR(100),
    region VARCHAR(100),
    address TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    data_classification VARCHAR(20) NOT NULL DEFAULT 'production',
    simulation_dataset_version VARCHAR(100),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (data_classification IN ('production', 'simulated')),
    CHECK (
        (data_classification = 'simulated' AND NULLIF(BTRIM(simulation_dataset_version), '') IS NOT NULL)
        OR (data_classification = 'production' AND simulation_dataset_version IS NULL)
    )
);

CREATE TABLE IF NOT EXISTS landlords (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    contact_person VARCHAR(255),
    contact_email VARCHAR(255),
    contact_phone VARCHAR(50),
    address TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS lease_contracts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contract_number VARCHAR(100) NOT NULL UNIQUE,
    contract_name VARCHAR(255) NOT NULL,
    legal_entity_id UUID NOT NULL REFERENCES legal_entities(id),
    store_id UUID NOT NULL REFERENCES stores(id),
    landlord_id UUID NOT NULL REFERENCES landlords(id),
    lessee_name VARCHAR(255),
    lessor_name VARCHAR(255),
    store_name VARCHAR(255),
    store_address TEXT,
    tags VARCHAR(500),
    asset_type VARCHAR(50) NOT NULL,
    asset_category VARCHAR(100),
    property_category VARCHAR(100),
    -- 租赁面积(㎡):按本合同承租的面积,用于每平米单价对比;设备/车辆租赁留空
    area_sqm DECIMAL(12, 2) CHECK (area_sqm IS NULL OR area_sqm > 0),
    currency VARCHAR(10) NOT NULL,
    signing_date DATE,
    commencement_date DATE NOT NULL,
    lease_start_date DATE NOT NULL,
    lease_end_date DATE NOT NULL,
    original_non_cancellable_period INTERVAL,
    renewal_option_description TEXT,
    termination_option_description TEXT,
    renewal_assessment BOOLEAN,
    termination_assessment BOOLEAN,
    discount_rate_type VARCHAR(50),
    discount_rate_version VARCHAR(50),
    discount_rate_value DECIMAL(10,6),
    status VARCHAR(50) NOT NULL DEFAULT 'draft',
    created_by UUID REFERENCES users(id),
    approved_by UUID REFERENCES users(id),
    approved_at TIMESTAMP WITH TIME ZONE,
    -- 最后修改人:合同更新与折现率确认都会写入
    updated_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS lease_contract_attachments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contract_id UUID NOT NULL REFERENCES lease_contracts(id) ON DELETE CASCADE,
    file_name VARCHAR(255) NOT NULL,
    file_type VARCHAR(100),
    file_size BIGINT,
    minio_bucket VARCHAR(100),
    minio_object_key VARCHAR(500),
    uploaded_by UUID REFERENCES users(id),
    uploaded_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS lease_documents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contract_id UUID NOT NULL REFERENCES lease_contracts(id) ON DELETE CASCADE,
    document_type VARCHAR(50) NOT NULL DEFAULT 'main_contract',
    file_name VARCHAR(255) NOT NULL,
    file_type VARCHAR(100),
    file_size BIGINT,
    minio_bucket VARCHAR(100),
    minio_object_key VARCHAR(500),
    document_version VARCHAR(50),
    file_hash VARCHAR(128),
    source_page INTEGER,
    notes TEXT,
    uploaded_by UUID REFERENCES users(id),
    uploaded_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS critical_dates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contract_id UUID NOT NULL REFERENCES lease_contracts(id) ON DELETE CASCADE,
    date_type VARCHAR(50) NOT NULL,
    target_date DATE NOT NULL,
    reminder_days INTEGER NOT NULL DEFAULT 30,
    responsible_user_id UUID REFERENCES users(id),
    status VARCHAR(50) NOT NULL DEFAULT 'open',
    title VARCHAR(255) NOT NULL,
    description TEXT,
    source VARCHAR(50) NOT NULL DEFAULT 'manual',
    source_reference_locator JSONB,
    completed_at TIMESTAMP WITH TIME ZONE,
    completed_by UUID REFERENCES users(id),
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (date_type IN ('renewal_deadline', 'break_notice', 'rent_review', 'lease_expiry', 'insurance_renewal', 'other')),
    CHECK (status IN ('open', 'snoozed', 'completed', 'cancelled')),
    CHECK (source IN ('manual', 'ai_suggested', 'policy_rule', 'system_generated'))
);

CREATE TABLE IF NOT EXISTS lease_obligations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contract_id UUID NOT NULL REFERENCES lease_contracts(id) ON DELETE CASCADE,
    obligation_type VARCHAR(50) NOT NULL,
    responsible_party VARCHAR(50) NOT NULL DEFAULT 'lessee',
    title VARCHAR(255) NOT NULL,
    description TEXT,
    structured_value JSONB,
    source_clause TEXT,
    source_page INTEGER,
    source_reference_locator JSONB,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (obligation_type IN ('maintenance', 'cam', 'insurance', 'index_adjustment', 'restoration', 'security_deposit', 'notice', 'other')),
    CHECK (responsible_party IN ('lessee', 'lessor', 'shared', 'third_party')),
    CHECK (status IN ('active', 'completed', 'waived', 'cancelled'))
);

CREATE TABLE IF NOT EXISTS lease_payment_schedules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contract_id UUID NOT NULL REFERENCES lease_contracts(id) ON DELETE CASCADE,
    effective_start_date DATE NOT NULL,
    effective_end_date DATE NOT NULL,
    coverage_start_date DATE NOT NULL,
    coverage_end_date DATE NOT NULL,
    due_date DATE NOT NULL,
    actual_payment_date DATE,
    payment_timing VARCHAR(10) NOT NULL CHECK (payment_timing IN ('prepaid', 'postpaid')),
    amount DECIMAL(18, 2) NOT NULL,
    currency VARCHAR(10) NOT NULL,
    tax_amount DECIMAL(18, 2),
    amount_type VARCHAR(50) NOT NULL,
    is_fixed BOOLEAN NOT NULL DEFAULT true,
    is_variable BOOLEAN NOT NULL DEFAULT false,
    is_index_adjusted BOOLEAN NOT NULL DEFAULT false,
    is_lease_component BOOLEAN NOT NULL DEFAULT true,
    is_non_lease_component BOOLEAN NOT NULL DEFAULT false,
    included_in_liability_pv BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS lease_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contract_id UUID NOT NULL REFERENCES lease_contracts(id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL,
    effective_date DATE NOT NULL,
    application_date DATE,
    approval_date DATE,
    original_value TEXT,
    new_value TEXT,
    change_reason TEXT,
    judgment_basis TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    recalculation_batch_id UUID,
    created_by UUID REFERENCES users(id),
    approved_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ai_tasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_type VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    file_id UUID,
    contract_id UUID REFERENCES lease_contracts(id),
    result JSONB,
    confidence_scores JSONB,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE IF NOT EXISTS ai_uploaded_files (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    file_name VARCHAR(255) NOT NULL,
    file_type VARCHAR(100),
    file_hash VARCHAR(255),
    file_size BIGINT,
    minio_bucket VARCHAR(100),
    minio_object_key VARCHAR(500),
    uploaded_by UUID REFERENCES users(id),
    uploaded_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ai_contract_drafts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES ai_tasks(id) ON DELETE CASCADE,
    contract_data JSONB NOT NULL,
    confidence_scores JSONB,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    reviewed_by UUID REFERENCES users(id),
    reviewed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ai_payment_schedule_drafts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES ai_tasks(id) ON DELETE CASCADE,
    contract_id UUID REFERENCES lease_contracts(id),
    payment_schedules JSONB NOT NULL,
    confidence_scores JSONB,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    reviewed_by UUID REFERENCES users(id),
    reviewed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    table_name VARCHAR(100) NOT NULL,
    record_id UUID NOT NULL,
    legal_entity_id UUID REFERENCES legal_entities(id),
    action VARCHAR(50) NOT NULL,
    old_values JSONB,
    new_values JSONB,
    changed_by UUID REFERENCES users(id),
    changed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    ip_address INET,
    user_agent TEXT
);

CREATE INDEX idx_lease_contracts_legal_entity ON lease_contracts(legal_entity_id);
CREATE INDEX idx_lease_contracts_store ON lease_contracts(store_id);
CREATE INDEX idx_lease_contracts_status ON lease_contracts(status);
CREATE INDEX idx_payment_schedules_contract ON lease_payment_schedules(contract_id);
CREATE INDEX idx_events_contract ON lease_events(contract_id);
CREATE INDEX idx_events_type ON lease_events(event_type);
CREATE INDEX idx_audit_logs_table_record ON audit_logs(table_name, record_id);
CREATE INDEX idx_audit_logs_changed_at ON audit_logs(changed_at);
CREATE INDEX idx_audit_logs_legal_entity ON audit_logs(legal_entity_id);

-- ============================================================================
-- Migration 002: Seed Data
-- ============================================================================

INSERT INTO legal_entities (id, code, name, country, currency, is_active)
VALUES
    ('a1b2c3d4-e5f6-7890-abcd-ef1234567890', 'LE001', '零售集团总公司', 'CN', 'CNY', true),
    ('b2c3d4e5-f6a7-8901-bcde-f12345678901', 'LE002', '零售集团上海公司', 'CN', 'CNY', true)
ON CONFLICT (code) DO NOTHING;

INSERT INTO stores (id, code, name, legal_entity_id, brand, region, is_active)
VALUES
    ('c3d4e5f6-a7b8-9012-cdef-123456789012', 'ST001', '南京东路旗舰店', 'a1b2c3d4-e5f6-7890-abcd-ef1234567890', '主品牌', '华东', true),
    ('d4e5f6a7-b8c9-0123-def1-234567890123', 'ST002', '淮海路店', 'b2c3d4e5-f6a7-8901-bcde-f12345678901', '主品牌', '华东', true)
ON CONFLICT (code) DO NOTHING;

INSERT INTO landlords (id, code, name, contact_person, contact_email, is_active)
VALUES
    ('e5f6a7b8-c9d0-1234-ef12-345678901234', 'LL001', '上海商业地产集团', '张先生', 'zhang@property.com', true),
    ('f6a7b8c9-d0e1-2345-f123-456789012345', 'LL002', '北京购物中心管理', '李女士', 'li@shopping.com', true)
ON CONFLICT (code) DO NOTHING;

-- ============================================================================
-- Migration 003: RBAC, Approval, Discount Rate
-- ============================================================================

-- 1. User-Role mapping table
CREATE TABLE IF NOT EXISTS user_roles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    assigned_by UUID REFERENCES users(id),
    assigned_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, role_id)
);

-- 2. User data scope table
CREATE TABLE IF NOT EXISTS user_data_scopes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    dimension VARCHAR(50) NOT NULL CHECK (dimension IN ('legal_entity', 'store', 'region', 'brand')),
    target_id VARCHAR(100) NOT NULL,
    target_name VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, dimension, target_id)
);

-- 3. Approval status enum type
DO $$ BEGIN
    CREATE TYPE approval_status AS ENUM (
        'draft',
        'submitted',
        'reviewed',
        'pending_approval',
        'approved',
        'rejected',
        'returned_to_editor'
    );
EXCEPTION WHEN duplicate_object THEN null;
END $$;

-- 4. Contract table: approval & version control columns
ALTER TABLE lease_contracts
    ADD COLUMN IF NOT EXISTS approval_status VARCHAR(50) NOT NULL DEFAULT 'draft',
    ADD COLUMN IF NOT EXISTS is_official_version BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS draft_version_no INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS included_in_reporting BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS report_mode VARCHAR(50) NOT NULL DEFAULT 'working',
    ADD COLUMN IF NOT EXISTS reviewed_by UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS rejected_reason TEXT,
    ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS ai_confidence_score DECIMAL(3, 2),
    ADD COLUMN IF NOT EXISTS source_reference_locator JSONB;

-- 5. Contract table: discount rate human-in-the-loop columns
ALTER TABLE lease_contracts
    ADD COLUMN IF NOT EXISTS discount_rate_value DECIMAL(10,6),
    ADD COLUMN IF NOT EXISTS discount_rate_missing BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS discount_rate_source VARCHAR(100),
    ADD COLUMN IF NOT EXISTS discount_rate_policy_id UUID,
    ADD COLUMN IF NOT EXISTS discount_rate_confirmed_by UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS discount_rate_confirmed_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS ai_extracted_discount_rate TEXT,
    ADD COLUMN IF NOT EXISTS ai_suggested_rate_policies JSONB;

-- 5b. Contract table: IFRS 16 scope gate columns
ALTER TABLE lease_contracts
    ADD COLUMN IF NOT EXISTS lease_scope VARCHAR(50) NOT NULL
        CHECK (lease_scope IN ('in_scope', 'short_term_exempt', 'low_value_exempt', 'not_a_lease')),
    ADD COLUMN IF NOT EXISTS exemption_reason TEXT,
    ADD COLUMN IF NOT EXISTS scope_classified_by UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS scope_classified_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS scope_source VARCHAR(50)
        CHECK (scope_source IS NULL OR scope_source IN ('ai_suggested', 'manual', 'policy_rule')),
    ADD COLUMN IF NOT EXISTS scope_confidence DECIMAL(3, 2)
        CHECK (scope_confidence IS NULL OR (scope_confidence >= 0 AND scope_confidence <= 1));

-- 5c. Contract table: lease administration dimensions
ALTER TABLE lease_contracts
    ADD COLUMN IF NOT EXISTS asset_type VARCHAR(50) NOT NULL
        CHECK (asset_type IN ('real_estate', 'vehicle', 'it_equipment', 'machinery', 'other'));

-- 6. Payment schedules: approval status
ALTER TABLE lease_payment_schedules
    ADD COLUMN IF NOT EXISTS approval_status VARCHAR(50) NOT NULL DEFAULT 'approved',
    ADD COLUMN IF NOT EXISTS is_official_version BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS reviewed_by UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS approved_by UUID REFERENCES users(id);

-- 7. Events: approval status
ALTER TABLE lease_events
    ADD COLUMN IF NOT EXISTS approval_status VARCHAR(50) NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS is_official_version BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS reviewed_by UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS rejected_reason TEXT;

-- 8. AI drafts: enhanced fields
ALTER TABLE ai_contract_drafts
    ADD COLUMN IF NOT EXISTS approval_status VARCHAR(50) NOT NULL DEFAULT 'draft',
    ADD COLUMN IF NOT EXISTS submitted_by UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS reviewed_by UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS approved_by UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS approved_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS rejected_reason TEXT,
    ADD COLUMN IF NOT EXISTS returned_to_editor_at TIMESTAMP WITH TIME ZONE;

-- 9. Indexes
CREATE INDEX IF NOT EXISTS idx_user_roles_user ON user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_role ON user_roles(role_id);
CREATE INDEX IF NOT EXISTS idx_user_data_scopes_user ON user_data_scopes(user_id);
CREATE INDEX IF NOT EXISTS idx_lease_contracts_approval ON lease_contracts(approval_status);
CREATE INDEX IF NOT EXISTS idx_lease_contracts_official ON lease_contracts(is_official_version);
CREATE INDEX IF NOT EXISTS idx_lease_contracts_dr_missing ON lease_contracts(discount_rate_missing);
CREATE INDEX IF NOT EXISTS idx_lease_contracts_scope ON lease_contracts(lease_scope);
CREATE INDEX IF NOT EXISTS idx_lease_contracts_asset_type ON lease_contracts(asset_type);
CREATE INDEX IF NOT EXISTS idx_critical_dates_contract ON critical_dates(contract_id);
CREATE INDEX IF NOT EXISTS idx_critical_dates_due ON critical_dates(status, target_date);
CREATE INDEX IF NOT EXISTS idx_lease_documents_contract ON lease_documents(contract_id);
CREATE INDEX IF NOT EXISTS idx_lease_obligations_contract ON lease_obligations(contract_id);
CREATE INDEX IF NOT EXISTS idx_lease_obligations_type ON lease_obligations(obligation_type);
CREATE INDEX IF NOT EXISTS idx_lease_obligations_status ON lease_obligations(status);

-- ============================================================================
-- Migration 008: AI Chat Runtime Persistence
-- ============================================================================

CREATE TABLE IF NOT EXISTS ai_chat_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    legal_entity_id UUID REFERENCES legal_entities(id),
    title VARCHAR(255) NOT NULL DEFAULT '新会话',
    status VARCHAR(30) NOT NULL DEFAULT 'active',
    bound_contract_id UUID REFERENCES lease_contracts(id),
    context_snapshot JSONB,
    initiator VARCHAR(20) NOT NULL DEFAULT 'user',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_message_at TIMESTAMP WITH TIME ZONE,
    archived_at TIMESTAMP WITH TIME ZONE,
    CHECK (status IN ('active', 'archived', 'closed')),
    CHECK (initiator IN ('user', 'system'))
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
    worker_id VARCHAR(150),
    lease_token VARCHAR(120),
    leased_until TIMESTAMP WITH TIME ZONE,
    heartbeat_at TIMESTAMP WITH TIME ZONE,
    attempt_count INTEGER NOT NULL DEFAULT 0,
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
    confidence DOUBLE PRECISION,
    confidence_reason TEXT,
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
        'event_draft',
        'audit_pack',
        'data_quality_issue_list',
        'report_explanation',
        'monthly_close_blockers',
        'retail_action_proposal',
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
CREATE INDEX IF NOT EXISTS idx_lease_payment_schedules_contract ON lease_payment_schedules(contract_id);
CREATE INDEX IF NOT EXISTS idx_lease_events_status ON lease_events(approval_status);
CREATE INDEX IF NOT EXISTS idx_ai_contract_drafts_status ON ai_contract_drafts(approval_status);

-- 10. Preset system roles
INSERT INTO roles (id, code, name, description) VALUES
    ('11111111-1111-1111-1111-111111111111', 'admin', 'System Admin', '系统管理员：管理用户、角色、主数据和系统参数'),
    ('22222222-2222-2222-2222-222222222222', 'editor', 'Finance Editor', '财务录入员：上传合同、维护草稿、录入台账'),
    ('33333333-3333-3333-3333-333333333333', 'reviewer', 'Finance Reviewer', '财务复核员：复核合同、付款计划和事件草稿'),
    ('44444444-4444-4444-4444-444444444444', 'approver', 'Finance Approver', '财务审批员：审批正式入库和关键会计处理'),
    ('55555555-5555-5555-5555-555555555555', 'auditor', 'Auditor Readonly', '审计只读：只读查看合同、台账、摊销和审计轨迹'),
    ('66666666-6666-6666-6666-666666666666', 'readonly', 'Business Readonly', '业务只读：查看授权范围内的合同和报表')
ON CONFLICT (name) DO NOTHING;

-- 11. Preset permissions
INSERT INTO permissions (role_id, resource, action) VALUES
    ('11111111-1111-1111-1111-111111111111', '*', '*'),
    ('22222222-2222-2222-2222-222222222222', 'contracts', 'create'),
    ('22222222-2222-2222-2222-222222222222', 'contracts', 'read'),
    ('22222222-2222-2222-2222-222222222222', 'contracts', 'update'),
    ('22222222-2222-2222-2222-222222222222', 'contracts', 'draft'),
    ('22222222-2222-2222-2222-222222222222', 'payment_schedules', 'create'),
    ('22222222-2222-2222-2222-222222222222', 'payment_schedules', 'read'),
    ('22222222-2222-2222-2222-222222222222', 'payment_schedules', 'update'),
    ('22222222-2222-2222-2222-222222222222', 'ai_tasks', 'create'),
    ('22222222-2222-2222-2222-222222222222', 'ai_tasks', 'read'),
    ('22222222-2222-2222-2222-222222222222', 'uploads', 'create'),
    ('33333333-3333-3333-3333-333333333333', 'contracts', 'read'),
    ('33333333-3333-3333-3333-333333333333', 'contracts', 'review'),
    ('33333333-3333-3333-3333-333333333333', 'payment_schedules', 'read'),
    ('33333333-3333-3333-3333-333333333333', 'payment_schedules', 'review'),
    ('33333333-3333-3333-3333-333333333333', 'events', 'read'),
    ('33333333-3333-3333-3333-333333333333', 'events', 'review'),
    ('33333333-3333-3333-3333-333333333333', 'ai_drafts', 'read'),
    ('33333333-3333-3333-3333-333333333333', 'ai_drafts', 'review'),
    ('44444444-4444-4444-4444-444444444444', 'contracts', 'read'),
    ('44444444-4444-4444-4444-444444444444', 'contracts', 'approve'),
    ('44444444-4444-4444-4444-444444444444', 'payment_schedules', 'read'),
    ('44444444-4444-4444-4444-444444444444', 'payment_schedules', 'approve'),
    ('44444444-4444-4444-4444-444444444444', 'events', 'read'),
    ('44444444-4444-4444-4444-444444444444', 'events', 'approve'),
    ('44444444-4444-4444-4444-444444444444', 'calculations', 'trigger'),
    ('55555555-5555-5555-5555-555555555555', 'contracts', 'read'),
    ('55555555-5555-5555-5555-555555555555', 'payment_schedules', 'read'),
    ('55555555-5555-5555-5555-555555555555', 'events', 'read'),
    ('55555555-5555-5555-5555-555555555555', 'calculations', 'read'),
    ('55555555-5555-5555-5555-555555555555', 'audit_logs', 'read'),
    ('55555555-5555-5555-5555-555555555555', 'reports', 'read'),
    ('55555555-5555-5555-5555-555555555555', 'reports', 'export'),
    ('66666666-6666-6666-6666-666666666666', 'contracts', 'read'),
    ('66666666-6666-6666-6666-666666666666', 'reports', 'read')
ON CONFLICT (role_id, resource, action) DO NOTHING;

INSERT INTO permissions (role_id, resource, action) VALUES
    ('22222222-2222-2222-2222-222222222222', 'variance_actions', 'write'),
    ('22222222-2222-2222-2222-222222222222', 'renewal_decisions', 'write'),
    ('33333333-3333-3333-3333-333333333333', 'variance_actions', 'write'),
    ('33333333-3333-3333-3333-333333333333', 'renewal_decisions', 'write'),
    ('44444444-4444-4444-4444-444444444444', 'variance_actions', 'write'),
    ('44444444-4444-4444-4444-444444444444', 'renewal_decisions', 'write')
ON CONFLICT (role_id, resource, action) DO NOTHING;

-- 12. Expanded access-policy permissions for all protected modules
INSERT INTO permissions (role_id, resource, action) VALUES
    ('22222222-2222-2222-2222-222222222222', 'identity', 'read'),
    ('22222222-2222-2222-2222-222222222222', 'calculations', 'read'),
    ('22222222-2222-2222-2222-222222222222', 'reports', 'read'),
    ('22222222-2222-2222-2222-222222222222', 'contracts', 'submit'),
    ('22222222-2222-2222-2222-222222222222', 'events', 'create'),
    ('22222222-2222-2222-2222-222222222222', 'events', 'read'),
    ('22222222-2222-2222-2222-222222222222', 'events', 'submit'),
    ('22222222-2222-2222-2222-222222222222', 'lease_admin', 'create'),
    ('22222222-2222-2222-2222-222222222222', 'lease_admin', 'read'),
    ('22222222-2222-2222-2222-222222222222', 'lease_admin', 'update'),
    ('22222222-2222-2222-2222-222222222222', 'ai_chat', 'use'),
    ('22222222-2222-2222-2222-222222222222', 'ai_drafts', 'confirm'),
    ('22222222-2222-2222-2222-222222222222', 'master_data', 'read'),
    ('22222222-2222-2222-2222-222222222222', 'settings', 'read'),
    ('33333333-3333-3333-3333-333333333333', 'calculations', 'read'),
    ('33333333-3333-3333-3333-333333333333', 'calculations', 'trigger'),
    ('33333333-3333-3333-3333-333333333333', 'reports', 'read'),
    ('33333333-3333-3333-3333-333333333333', 'monthly_closing', 'generate'),
    ('33333333-3333-3333-3333-333333333333', 'monthly_closing', 'read'),
    ('33333333-3333-3333-3333-333333333333', 'ai_chat', 'use'),
    ('33333333-3333-3333-3333-333333333333', 'master_data', 'read'),
    ('33333333-3333-3333-3333-333333333333', 'settings', 'read'),
    ('33333333-3333-3333-3333-333333333333', 'identity', 'read'),
    ('33333333-3333-3333-3333-333333333333', 'lease_admin', 'read'),
    ('44444444-4444-4444-4444-444444444444', 'reports', 'read'),
    ('44444444-4444-4444-4444-444444444444', 'reports', 'export'),
    ('44444444-4444-4444-4444-444444444444', 'monthly_closing', 'generate'),
    ('44444444-4444-4444-4444-444444444444', 'monthly_closing', 'read'),
    ('44444444-4444-4444-4444-444444444444', 'monthly_closing', 'approve'),
    ('44444444-4444-4444-4444-444444444444', 'monthly_closing', 'post'),
    ('44444444-4444-4444-4444-444444444444', 'monthly_closing', 'export'),
    ('44444444-4444-4444-4444-444444444444', 'monthly_closing', 'writeback'),
    ('44444444-4444-4444-4444-444444444444', 'monthly_closing', 'lock'),
    ('44444444-4444-4444-4444-444444444444', 'monthly_closing', 'unlock'),
    ('44444444-4444-4444-4444-444444444444', 'monthly_closing', 'reverse'),
    ('44444444-4444-4444-4444-444444444444', 'ai_chat', 'use'),
    ('44444444-4444-4444-4444-444444444444', 'master_data', 'read'),
    ('44444444-4444-4444-4444-444444444444', 'settings', 'read'),
    ('44444444-4444-4444-4444-444444444444', 'settings', 'update'),
    ('44444444-4444-4444-4444-444444444444', 'identity', 'read'),
    ('44444444-4444-4444-4444-444444444444', 'lease_admin', 'read'),
    ('55555555-5555-5555-5555-555555555555', 'monthly_closing', 'read'),
    ('55555555-5555-5555-5555-555555555555', 'monthly_closing', 'export'),
    ('55555555-5555-5555-5555-555555555555', 'master_data', 'read'),
    ('55555555-5555-5555-5555-555555555555', 'settings', 'read'),
    ('55555555-5555-5555-5555-555555555555', 'identity', 'read'),
    ('55555555-5555-5555-5555-555555555555', 'lease_admin', 'read'),
    ('55555555-5555-5555-5555-555555555555', 'ai_chat', 'use'),
    ('66666666-6666-6666-6666-666666666666', 'identity', 'read'),
    ('66666666-6666-6666-6666-666666666666', 'payment_schedules', 'read'),
    ('66666666-6666-6666-6666-666666666666', 'events', 'read'),
    ('66666666-6666-6666-6666-666666666666', 'calculations', 'read'),
    ('66666666-6666-6666-6666-666666666666', 'lease_admin', 'read'),
    ('66666666-6666-6666-6666-666666666666', 'ai_chat', 'use'),
    ('66666666-6666-6666-6666-666666666666', 'master_data', 'read'),
    ('66666666-6666-6666-6666-666666666666', 'settings', 'read')
ON CONFLICT (role_id, resource, action) DO NOTHING;

-- Backfill authoritative role assignments from the legacy single-role field.
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON r.code = CASE WHEN u.role = 'user' THEN 'readonly' ELSE u.role END
ON CONFLICT (user_id, role_id) DO NOTHING;
-- +goose Up
-- +goose StatementBegin

-- 1. 月度计量结果表（IFRS 16 计算结果）
CREATE TABLE IF NOT EXISTS measurement_results (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contract_id UUID NOT NULL REFERENCES lease_contracts(id) ON DELETE CASCADE,
    accounting_period VARCHAR(7) NOT NULL, -- YYYY-MM
    period_start_date DATE NOT NULL,
    period_end_date DATE NOT NULL,
    opening_liability DECIMAL(18, 2) NOT NULL DEFAULT 0,
    interest_expense DECIMAL(18, 2) NOT NULL DEFAULT 0,
    principal_repayment DECIMAL(18, 2) NOT NULL DEFAULT 0,
    total_payment DECIMAL(18, 2) NOT NULL DEFAULT 0,
    closing_liability DECIMAL(18, 2) NOT NULL DEFAULT 0,
    opening_rou_asset DECIMAL(18, 2) NOT NULL DEFAULT 0,
    depreciation DECIMAL(18, 2) NOT NULL DEFAULT 0,
    closing_rou_asset DECIMAL(18, 2) NOT NULL DEFAULT 0,
    variable_rent_expense DECIMAL(18, 2) NOT NULL DEFAULT 0,
    non_lease_expense DECIMAL(18, 2) NOT NULL DEFAULT 0,
    discount_rate DECIMAL(10, 6) NOT NULL,
    is_calculated BOOLEAN NOT NULL DEFAULT false,
    calculation_batch_id UUID,
    calculated_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(contract_id, accounting_period)
);

-- 2. 会计分录表
CREATE TABLE IF NOT EXISTS journal_entries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contract_id UUID NOT NULL REFERENCES lease_contracts(id) ON DELETE CASCADE,
    measurement_result_id UUID REFERENCES measurement_results(id),
    accounting_period VARCHAR(7) NOT NULL, -- YYYY-MM
    entry_date DATE NOT NULL,
    entry_type VARCHAR(50) NOT NULL, -- interest, depreciation, payment, variable_rent, non_lease, modification, reassessment
    debit_account VARCHAR(100) NOT NULL,
    credit_account VARCHAR(100) NOT NULL,
    amount DECIMAL(18, 2) NOT NULL,
    currency VARCHAR(10) NOT NULL,
    description TEXT,
    voucher_number VARCHAR(100),
    posting_status VARCHAR(20) NOT NULL DEFAULT 'draft', -- draft, preview, approved, posted, reversed
    posted_at TIMESTAMP WITH TIME ZONE,
    posted_by UUID REFERENCES users(id),
    approved_by UUID REFERENCES users(id),
    approved_at TIMESTAMP WITH TIME ZONE,
    erp_reference VARCHAR(255),
    batch_id UUID,
    -- 红冲:关联字段挂在红冲分录上,原分录只记录被红冲的时间与操作人
    reversal_of_entry_id UUID REFERENCES journal_entries(id),
    reversal_reason TEXT,
    reversed_at TIMESTAMP WITH TIME ZONE,
    reversed_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 3. 月结批次表
CREATE TABLE IF NOT EXISTS monthly_closing_batches (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    batch_number VARCHAR(50) NOT NULL UNIQUE,
    accounting_period VARCHAR(7) NOT NULL,
    legal_entity_id UUID REFERENCES legal_entities(id),
    scope_contract_id UUID REFERENCES lease_contracts(id),
    region VARCHAR(100),
    brand VARCHAR(100),
    status VARCHAR(32) NOT NULL DEFAULT 'draft', -- draft, running, completed, completed_with_errors, failed, cancelled
    total_contracts INTEGER NOT NULL DEFAULT 0,
    processed_contracts INTEGER NOT NULL DEFAULT 0,
    failed_contracts INTEGER NOT NULL DEFAULT 0,
    total_entries INTEGER NOT NULL DEFAULT 0,
    posted_entries INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 4. 索引
CREATE INDEX IF NOT EXISTS idx_measurement_results_contract ON measurement_results(contract_id);
CREATE INDEX IF NOT EXISTS idx_measurement_results_period ON measurement_results(accounting_period);
CREATE INDEX IF NOT EXISTS idx_measurement_results_batch ON measurement_results(calculation_batch_id);
CREATE INDEX IF NOT EXISTS idx_journal_entries_contract ON journal_entries(contract_id);
CREATE INDEX IF NOT EXISTS idx_journal_entries_period ON journal_entries(accounting_period);
CREATE INDEX IF NOT EXISTS idx_journal_entries_batch ON journal_entries(batch_id);
CREATE INDEX IF NOT EXISTS idx_journal_entries_posting ON journal_entries(posting_status);
-- 一笔分录只能被红冲一次,由唯一索引在库层保证
CREATE UNIQUE INDEX IF NOT EXISTS idx_journal_entries_reversal_of
    ON journal_entries(reversal_of_entry_id)
    WHERE reversal_of_entry_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_monthly_closing_period ON monthly_closing_batches(accounting_period);
CREATE INDEX IF NOT EXISTS idx_monthly_closing_scope ON monthly_closing_batches(accounting_period, legal_entity_id, scope_contract_id);

-- +goose StatementEnd

-- ============================================================================
-- Migration 004: Period Locks
-- ============================================================================

-- +goose Up
CREATE TABLE IF NOT EXISTS period_locks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    accounting_period VARCHAR(7) NOT NULL,
    legal_entity_id UUID REFERENCES legal_entities(id),
    is_locked BOOLEAN NOT NULL DEFAULT false,
    locked_by UUID REFERENCES users(id),
    locked_at TIMESTAMP WITH TIME ZONE,
    unlocked_by UUID REFERENCES users(id),
    unlocked_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(accounting_period, legal_entity_id)
);
CREATE INDEX IF NOT EXISTS idx_period_locks_period ON period_locks(accounting_period);

-- ============================================================================
-- Migration 005: IFRS 16 Modification/Reassessment — Event Adjustments
-- ============================================================================

-- +goose Up
-- Add IFRS 16 treatment classification to events
ALTER TABLE lease_events ADD COLUMN IF NOT EXISTS ifrs16_treatment VARCHAR(20);

-- Event adjustments: stores one record per approved event with before/after state
CREATE TABLE IF NOT EXISTS event_adjustments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_id UUID NOT NULL UNIQUE REFERENCES lease_events(id) ON DELETE CASCADE,
    contract_id UUID NOT NULL REFERENCES lease_contracts(id) ON DELETE CASCADE,
    adjustment_type VARCHAR(20) NOT NULL CHECK (adjustment_type IN ('modification', 'reassessment', 'impairment')),
    effective_date DATE NOT NULL,
    liability_before DECIMAL(18,2) NOT NULL,
    liability_after DECIMAL(18,2) NOT NULL,
    liability_adjustment DECIMAL(18,2) NOT NULL,
    rou_before DECIMAL(18,2) NOT NULL,
    rou_after DECIMAL(18,2) NOT NULL,
    rou_adjustment DECIMAL(18,2) NOT NULL,
    pnl_gain DECIMAL(18,2) NOT NULL DEFAULT 0,
    pnl_loss DECIMAL(18,2) NOT NULL DEFAULT 0,
    revised_discount_rate DECIMAL(10,6) NOT NULL,
    discount_rate_source VARCHAR(100),
    calculation_batch_id UUID,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_event_adjustments_event ON event_adjustments(event_id);
CREATE INDEX IF NOT EXISTS idx_event_adjustments_contract ON event_adjustments(contract_id);
CREATE INDEX IF NOT EXISTS idx_event_adjustments_effective ON event_adjustments(effective_date);

-- ============================================================================
-- Migration 006: System Settings
-- ============================================================================

-- +goose Up
CREATE TABLE IF NOT EXISTS system_settings (
    setting_key VARCHAR(100) PRIMARY KEY,
    setting_value TEXT NOT NULL,
    description TEXT,
    updated_by UUID REFERENCES users(id),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 预算版本:把某时点的计量前瞻表固化,供后续月份做预算 vs 实际对比
CREATE TABLE IF NOT EXISTS budget_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    legal_entity_id UUID REFERENCES legal_entities(id),
    version_type VARCHAR(20) NOT NULL DEFAULT 'budget',
    source VARCHAR(255) NOT NULL,
    coverage_scope TEXT NOT NULL DEFAULT '',
    is_official BOOLEAN NOT NULL DEFAULT false,
    as_of_period VARCHAR(7) NOT NULL,
    from_period VARCHAR(7) NOT NULL,
    to_period VARCHAR(7) NOT NULL,
    contract_count INTEGER NOT NULL DEFAULT 0,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

ALTER TABLE budget_versions
    ADD COLUMN IF NOT EXISTS version_type VARCHAR(20) NOT NULL DEFAULT 'budget',
    ADD COLUMN IF NOT EXISTS source VARCHAR(255) NOT NULL,
    ADD COLUMN IF NOT EXISTS coverage_scope TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS is_official BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE budget_versions
    DROP CONSTRAINT IF EXISTS budget_versions_version_type_check;

ALTER TABLE budget_versions
    ADD CONSTRAINT budget_versions_version_type_check
    CHECK (version_type IN ('budget', 'forecast', 'scenario'));

CREATE TABLE IF NOT EXISTS budget_lines (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    budget_version_id UUID NOT NULL REFERENCES budget_versions(id) ON DELETE CASCADE,
    contract_id UUID NOT NULL REFERENCES lease_contracts(id) ON DELETE CASCADE,
    accounting_period VARCHAR(7) NOT NULL,
    currency VARCHAR(10) NOT NULL,
    interest_expense DECIMAL(18, 2) NOT NULL DEFAULT 0,
    depreciation DECIMAL(18, 2) NOT NULL DEFAULT 0,
    total_payment DECIMAL(18, 2) NOT NULL DEFAULT 0,
    closing_liability DECIMAL(18, 2) NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (budget_version_id, contract_id, accounting_period)
);

CREATE INDEX IF NOT EXISTS idx_budget_lines_period
    ON budget_lines(budget_version_id, accounting_period);

-- 汇率表:外币租赁折算为法人主体职能货币
-- closing 用于期末重估货币性项目(租赁负债);average 用于当期流量(利息与付款)
CREATE TABLE IF NOT EXISTS exchange_rates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    from_currency VARCHAR(10) NOT NULL,
    to_currency VARCHAR(10) NOT NULL,
    rate_date DATE NOT NULL,
    rate_type VARCHAR(20) NOT NULL DEFAULT 'closing',
    rate DECIMAL(18, 8) NOT NULL,
    source VARCHAR(100),
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (rate > 0),
    CHECK (rate_type IN ('closing', 'average')),
    CHECK (from_currency <> to_currency),
    UNIQUE (from_currency, to_currency, rate_date, rate_type)
);

CREATE INDEX IF NOT EXISTS idx_exchange_rates_lookup
    ON exchange_rates(from_currency, to_currency, rate_type, rate_date DESC);

-- Do not seed a discount rate. A missing rate must stay missing until an
-- approved policy or human confirmation supplies one.

-- ----------------------------------------------------------------------------
-- 迁移 017:事件结构化条款参数
-- 事件此前只能用 new_value 一个自由文本记录变更,系统读不懂 CPI、上下限、
-- 阶梯这类条款,只能先手工改付款计划再录事件。revision_parameters 按出租方
-- 通知书的原话保存条款,使修订付款流可以被推导出来。
-- ----------------------------------------------------------------------------

ALTER TABLE lease_events
    ADD COLUMN IF NOT EXISTS revision_parameters JSONB;

CREATE INDEX IF NOT EXISTS idx_lease_events_revision_parameters
    ON lease_events USING GIN (revision_parameters)
    WHERE revision_parameters IS NOT NULL;

-- ----------------------------------------------------------------------------
-- 迁移 018:门店营收薄片与别名映射
-- ----------------------------------------------------------------------------

-- Rent-to-sales is the first number a retail lease decision turns on, and the
-- system has had no way to compute it: it knows the rent and nothing about the
-- sales.
--
-- store_metrics is deliberately a thin slice — store × period × two or three
-- figures — not a P&L. The authoritative source stays the customer's POS, ERP
-- or BI; this table is only ever a consumer of that data, and every report
-- built on it says so. Trying to hold more would put the system in competition
-- with the system of record, which it would lose.

CREATE TABLE IF NOT EXISTS store_metrics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    store_id UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,

    -- The period the figures cover, as YYYY-MM. Which calendar that month
    -- follows is the customer's business, so period_basis records what they
    -- told us rather than assuming a natural month.
    period VARCHAR(7) NOT NULL,
	period_basis VARCHAR(20) NOT NULL,

    revenue DECIMAL(18, 2) NOT NULL,
    gross_profit DECIMAL(18, 2),
    -- Sales in one currency against rent in another is not a ratio, so the
    -- currency travels with the figure and the report checks it.
	currency VARCHAR(10) NOT NULL,

    -- Revenue gets restated. version keeps the earlier submission rather than
    -- overwriting it, so a report can say which vintage it was based on.
    version INTEGER NOT NULL DEFAULT 1,
    -- Where the figure came from: 'manual', 'api', 'ai_upload'. Reports name it
    -- so a reader knows how much weight it carries.
	source VARCHAR(50) NOT NULL,
    note TEXT,

    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CHECK (revenue >= 0),
    CHECK (period ~ '^[0-9]{4}-[0-9]{2}$'),
    CHECK (period_basis IN ('calendar_month', 'fiscal_month')),
    UNIQUE (store_id, period, version)
);

COMMENT ON TABLE store_metrics IS
    '门店营收薄片:仅供租售比等分析消费,权威源为客户的 POS/ERP/BI';

CREATE INDEX IF NOT EXISTS idx_store_metrics_lookup
    ON store_metrics(store_id, period, version DESC);
CREATE INDEX IF NOT EXISTS idx_store_metrics_period
    ON store_metrics(period);

-- The Excel a business partner uploads says "万象城店"; the system calls it
-- "SZ-MOC-001 深圳万象城". Confirming that match by hand every month is what
-- stops the upload becoming a habit, so a confirmed match is remembered.
CREATE TABLE IF NOT EXISTS store_aliases (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    store_id UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    alias VARCHAR(255) NOT NULL,
    source VARCHAR(50) NOT NULL DEFAULT 'manual',
    confirmed_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (alias)
);

COMMENT ON TABLE store_aliases IS
    '门店别名映射:营收表里的叫法确认一次后记住,避免每月重复匹配';

-- ============================================================================
-- Migration 019: Close Exception Governance
-- ============================================================================

CREATE TABLE IF NOT EXISTS close_control_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rule_code VARCHAR(100) NOT NULL,
    rule_version VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    severity VARCHAR(20) NOT NULL DEFAULT 'blocking',
    gate_effect VARCHAR(100) NOT NULL,
    title VARCHAR(255) NOT NULL,
    reason_template TEXT NOT NULL DEFAULT '',
    remediation TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    effective_from DATE NOT NULL DEFAULT CURRENT_DATE,
    effective_to DATE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (rule_code, rule_version),
    CHECK (severity IN ('blocking', 'warning', 'informational'))
);

CREATE TABLE IF NOT EXISTS close_detection_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    control_rule_id UUID NOT NULL REFERENCES close_control_rules(id),
    rule_code VARCHAR(100) NOT NULL,
    rule_version VARCHAR(50) NOT NULL,
    legal_entity_id UUID REFERENCES legal_entities(id),
    accounting_period VARCHAR(7) NOT NULL,
    projection_version VARCHAR(100) NOT NULL,
    subject_type VARCHAR(50) NOT NULL,
    subject_id UUID NOT NULL,
    subject_contract_id UUID REFERENCES lease_contracts(id) ON DELETE CASCADE,
    fingerprint VARCHAR(64) NOT NULL,
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    detected_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

ALTER TABLE close_detection_events
    DROP CONSTRAINT IF EXISTS close_detection_events_fingerprint_projection_version_key;

CREATE TABLE IF NOT EXISTS close_exceptions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    detection_event_id UUID NOT NULL REFERENCES close_detection_events(id),
    fingerprint VARCHAR(64) NOT NULL UNIQUE,
    rule_code VARCHAR(100) NOT NULL,
    rule_version VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    gate_effect VARCHAR(100) NOT NULL,
    legal_entity_id UUID REFERENCES legal_entities(id),
    accounting_period VARCHAR(7) NOT NULL,
    subject_type VARCHAR(50) NOT NULL,
    subject_id UUID NOT NULL,
    subject_contract_id UUID REFERENCES lease_contracts(id) ON DELETE CASCADE,
    projection_version VARCHAR(100) NOT NULL,
    exception_state VARCHAR(30) NOT NULL DEFAULT 'open',
    closing_disposition VARCHAR(40) NOT NULL DEFAULT 'unresolved',
    owner_id UUID REFERENCES users(id),
    reviewer_id UUID REFERENCES users(id),
    approver_id UUID REFERENCES users(id),
    opened_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_detected_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    investigating_at TIMESTAMP WITH TIME ZONE,
    resolved_at TIMESTAMP WITH TIME ZONE,
    waived_at TIMESTAMP WITH TIME ZONE,
    closed_at TIMESTAMP WITH TIME ZONE,
    resolution_note TEXT,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (exception_state IN ('open', 'investigating', 'resolved', 'waived', 'closed')),
    CHECK (closing_disposition IN ('unresolved', 'verified_resolution', 'accounting_conclusion', 'period_waiver', 'standing_waiver')),
    CHECK (severity IN ('blocking', 'warning', 'informational'))
);

CREATE INDEX IF NOT EXISTS idx_close_detection_period
    ON close_detection_events(accounting_period, legal_entity_id, detected_at DESC);
CREATE INDEX IF NOT EXISTS idx_close_detection_fingerprint
    ON close_detection_events(fingerprint);
CREATE INDEX IF NOT EXISTS idx_close_exceptions_period
    ON close_exceptions(accounting_period, legal_entity_id, exception_state);
CREATE INDEX IF NOT EXISTS idx_close_exceptions_subject
    ON close_exceptions(subject_contract_id, subject_id);

ALTER TABLE close_control_rules
    ADD COLUMN IF NOT EXISTS reason_template TEXT NOT NULL DEFAULT '';

INSERT INTO close_control_rules (rule_code, rule_version, name, severity, gate_effect, title, reason_template, remediation)
VALUES
    ('missing_payment_schedule', 'v1', 'Approved payment schedule', 'blocking', 'formal_calculation', '缺少已批准付款计划', '已批准合同没有可用于本期间计量的正式付款计划', '补充或导入付款计划，并完成付款计划复核/审批流程'),
    ('missing_discount_rate', 'v1', 'Confirmed discount rate', 'blocking', 'formal_calculation', '缺少已确认折现率', '合同没有已确认折现率，且当前政策库没有可用折现率', '通过折现率政策匹配或人工确认补充折现率，不得由系统猜测'),
    ('pending_event_before_period_end', 'v1', 'Pending event before period end', 'blocking', 'formal_calculation', '存在期间内待审批事件', '合同存在生效日在本期间结束日前、但尚未完成审批的事件', '通过事件工作流完成复核、审批或退回处理，再重新运行预检'),
    ('failed_close_batch', 'v1', 'Failed close batch', 'blocking', 'close_preparation', '本期间存在失败月结批次', '月结批次 %s 状态为 %s，失败合同数为 %d', '打开结账中心检查失败合同，完成修正后再重新生成本期间月结')
ON CONFLICT (rule_code, rule_version) DO UPDATE SET
    name = EXCLUDED.name,
    severity = EXCLUDED.severity,
    gate_effect = EXCLUDED.gate_effect,
    title = EXCLUDED.title,
    reason_template = EXCLUDED.reason_template,
    remediation = EXCLUDED.remediation,
    enabled = true;

INSERT INTO permissions (role_id, resource, action) VALUES
    ('33333333-3333-3333-3333-333333333333', 'monthly_closing', 'exception_read'),
    ('33333333-3333-3333-3333-333333333333', 'monthly_closing', 'exception_detect'),
    ('33333333-3333-3333-3333-333333333333', 'monthly_closing', 'exception_manage'),
    ('44444444-4444-4444-4444-444444444444', 'monthly_closing', 'exception_read'),
    ('44444444-4444-4444-4444-444444444444', 'monthly_closing', 'exception_detect'),
    ('44444444-4444-4444-4444-444444444444', 'monthly_closing', 'exception_manage'),
    ('55555555-5555-5555-5555-555555555555', 'monthly_closing', 'exception_read')
ON CONFLICT (role_id, resource, action) DO NOTHING;

-- ============================================================================
-- Migration 020: FP&A version metadata, variance actions and renewal decisions
-- ============================================================================

CREATE TABLE IF NOT EXISTS variance_actions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    budget_version_id UUID NOT NULL REFERENCES budget_versions(id) ON DELETE CASCADE,
    contract_id UUID NOT NULL REFERENCES lease_contracts(id) ON DELETE CASCADE,
    accounting_period VARCHAR(7) NOT NULL,
    explanation TEXT NOT NULL DEFAULT '',
    owner_name VARCHAR(255) NOT NULL DEFAULT '',
    due_date DATE,
    status VARCHAR(30) NOT NULL DEFAULT 'open',
    updated_by UUID REFERENCES users(id),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (budget_version_id, contract_id, accounting_period),
    CHECK (status IN ('open', 'in_progress', 'resolved', 'accepted'))
);

CREATE INDEX IF NOT EXISTS idx_variance_actions_period
    ON variance_actions(budget_version_id, accounting_period, status);

CREATE TABLE IF NOT EXISTS renewal_decision_snapshots (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contract_id UUID NOT NULL REFERENCES lease_contracts(id) ON DELETE CASCADE,
    legal_entity_id UUID REFERENCES legal_entities(id),
    decision_date DATE NOT NULL,
    owner_name VARCHAR(255) NOT NULL DEFAULT '',
    business_opinion TEXT NOT NULL DEFAULT '',
    evidence TEXT NOT NULL DEFAULT '',
    snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_renewal_decisions_contract
    ON renewal_decision_snapshots(contract_id, decision_date DESC, created_at DESC);

-- ============================================================================
-- Migration 022: transactional idempotency for draft application commands
-- ============================================================================

CREATE TABLE IF NOT EXISTS agent_draft_idempotency (
    operation VARCHAR(120) NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL,
    result JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (operation, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_agent_draft_idempotency_created
    ON agent_draft_idempotency(created_at DESC);

-- ============================================================================
-- Migration 023: versioned Agent Artifact/Evidence protocol
-- ============================================================================

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
        'retail_action_proposal',
        'generic'
    ));

CREATE INDEX IF NOT EXISTS idx_ai_chat_artifacts_schema_status
    ON ai_chat_artifacts(schema_version, status);

-- ============================================================================
-- Migration 024: persisted batch envelope for partial draft recovery
-- ============================================================================

CREATE TABLE IF NOT EXISTS agent_draft_batches (
    batch_id UUID PRIMARY KEY,
    operation VARCHAR(120) NOT NULL,
    status VARCHAR(30) NOT NULL,
    items JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_by VARCHAR(255),
    run_id VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (status IN ('in_progress', 'completed', 'partial_failed', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_agent_draft_batches_status_updated
    ON agent_draft_batches(status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_draft_batches_run
    ON agent_draft_batches(run_id, updated_at DESC);

-- Durable capability metadata and revocation state. Raw JWTs are never stored.

CREATE TABLE IF NOT EXISTS agent_capability_grants (
    token_id VARCHAR(120) PRIMARY KEY,
    run_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    revoked_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_capability_grants_run
    ON agent_capability_grants(run_id, expires_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_capability_grants_expiry
    ON agent_capability_grants(expires_at);

-- ============================================================================
-- Migration 026: persist evidence linkage on event drafts
-- ============================================================================

ALTER TABLE lease_events
    ADD COLUMN IF NOT EXISTS source_reference_locator JSONB;

CREATE INDEX IF NOT EXISTS idx_lease_events_source_reference
    ON lease_events USING GIN (source_reference_locator)
    WHERE source_reference_locator IS NOT NULL;

-- ============================================================================
-- Migration 027: durable Runner checkpoint metadata on the owned Core Run
-- ============================================================================

ALTER TABLE ai_chat_runs
    ADD COLUMN IF NOT EXISTS checkpoint JSONB;

-- ============================================================================
-- Migration 028: Agent Run event types
-- ============================================================================

ALTER TABLE ai_chat_run_events
    DROP CONSTRAINT IF EXISTS ai_chat_run_events_event_type_check;

ALTER TABLE ai_chat_run_events
    ADD CONSTRAINT ai_chat_run_events_event_type_check CHECK (event_type IN (
        'message_start', 'message_delta', 'message_end',
        'tool_start', 'tool_update', 'tool_end', 'tool_execution',
        'tool_started', 'tool_completed',
        'review_prompt', 'artifact_ready', 'queue_update',
        'run_status', 'run_started', 'run_resumed', 'run_steered',
        'run_steer', 'run_follow_up', 'run_branch_created',
        'checkpoint_restored', 'run_end', 'run_finished',
        'run_error', 'run_failed', 'run_cancelled'
    ));

CREATE INDEX IF NOT EXISTS idx_ai_chat_runs_checkpoint_updated
    ON ai_chat_runs(created_at DESC)
    WHERE checkpoint IS NOT NULL;

-- ==========================================================================
-- Migration 029: durable Agent Run worker leases
-- ==========================================================================

ALTER TABLE ai_chat_runs
    ADD COLUMN IF NOT EXISTS worker_id VARCHAR(150),
    ADD COLUMN IF NOT EXISTS lease_token VARCHAR(120),
    ADD COLUMN IF NOT EXISTS leased_until TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS heartbeat_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS attempt_count INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_ai_chat_runs_worker_lease
    ON ai_chat_runs(status, leased_until, created_at)
    WHERE status IN ('queued', 'running');

-- Migration 030: persistent, one-time refresh-token sessions
CREATE TABLE IF NOT EXISTS auth_refresh_sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_id UUID NOT NULL UNIQUE,
    token_hash VARCHAR(128) NOT NULL UNIQUE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMP WITH TIME ZONE,
    replaced_by UUID,
    ip_address INET,
    user_agent TEXT
);

CREATE INDEX IF NOT EXISTS idx_auth_refresh_sessions_user_active
    ON auth_refresh_sessions(user_id, expires_at)
    WHERE revoked_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_auth_refresh_sessions_expiry
    ON auth_refresh_sessions(expires_at);

-- Migration 031: durable Run audit summary read model
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

-- Migration 032: immutable checkpoint audit index and terminal alert outbox.
CREATE TABLE IF NOT EXISTS agent_run_checkpoint_audits (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id UUID NOT NULL REFERENCES ai_chat_runs(id) ON DELETE CASCADE,
    session_id UUID NOT NULL REFERENCES ai_chat_sessions(id) ON DELETE CASCADE,
    checkpoint_hash CHAR(64) NOT NULL,
    checkpoint_size_bytes INTEGER NOT NULL CHECK (checkpoint_size_bytes >= 0),
    schema_version VARCHAR(100),
    checkpoint_status VARCHAR(30) NOT NULL DEFAULT 'saved',
    next_index INTEGER CHECK (next_index IS NULL OR next_index >= 0),
    actor_id UUID,
    worker_id VARCHAR(150),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_agent_run_checkpoint_audits_run_created
    ON agent_run_checkpoint_audits(run_id, created_at DESC);

CREATE TABLE IF NOT EXISTS agent_run_terminal_alerts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id UUID NOT NULL UNIQUE REFERENCES ai_chat_runs(id) ON DELETE CASCADE,
    session_id UUID NOT NULL REFERENCES ai_chat_sessions(id) ON DELETE CASCADE,
    terminal_status VARCHAR(30) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    error_message TEXT,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(20) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'acknowledged')),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    acknowledged_at TIMESTAMP WITH TIME ZONE,
    acknowledged_by UUID
);
CREATE INDEX IF NOT EXISTS idx_agent_run_terminal_alerts_status_created
    ON agent_run_terminal_alerts(status, created_at DESC);

ALTER TABLE agent_run_audit_summaries
    ADD COLUMN IF NOT EXISTS checkpoint_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS last_checkpoint_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS terminal_status VARCHAR(30),
    ADD COLUMN IF NOT EXISTS terminal_error TEXT;

ALTER TABLE ai_chat_run_events
    DROP CONSTRAINT IF EXISTS ai_chat_run_events_event_type_check;
ALTER TABLE ai_chat_run_events
    ADD CONSTRAINT ai_chat_run_events_event_type_check CHECK (event_type IN (
        'message_start', 'message_delta', 'message_end',
        'tool_start', 'tool_update', 'tool_end', 'tool_execution',
        'tool_started', 'tool_completed',
        'review_prompt', 'artifact_ready', 'queue_update',
        'run_status', 'run_started', 'run_resumed', 'run_steered',
        'run_steer', 'run_follow_up', 'run_branch_created',
        'planner_usage',
        'checkpoint_saved', 'checkpoint_restored',
        'run_end', 'run_finished', 'run_error', 'run_failed', 'run_cancelled'
    ));

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

-- Migration 034: operating facts, equipment context and unified FP&A actions.
CREATE TABLE IF NOT EXISTS operating_fact_batches (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(), legal_entity_id UUID REFERENCES legal_entities(id),
    source_system VARCHAR(100) NOT NULL, source_file VARCHAR(255), as_of_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    status VARCHAR(30) NOT NULL DEFAULT 'received', total_rows INTEGER NOT NULL DEFAULT 0,
    accepted_rows INTEGER NOT NULL DEFAULT 0, rejected_rows INTEGER NOT NULL DEFAULT 0,
    reconciliation_status VARCHAR(30) NOT NULL DEFAULT 'unreconciled', error_summary JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_by UUID REFERENCES users(id), created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (status IN ('received','processing','completed','failed')),
    CHECK (reconciliation_status IN ('unreconciled','matched','warning','failed'))
);
CREATE TABLE IF NOT EXISTS store_operating_facts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(), store_id UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    period VARCHAR(7) NOT NULL, period_basis VARCHAR(20) NOT NULL, currency VARCHAR(10) NOT NULL,
    revenue DECIMAL(18,2) NOT NULL DEFAULT 0, gross_profit DECIMAL(18,2), transactions DECIMAL(18,2), footfall DECIMAL(18,2), area_sqm DECIMAL(18,2),
    labor_cost DECIMAL(18,2), fixed_rent DECIMAL(18,2), variable_rent DECIMAL(18,2), non_lease_cost DECIMAL(18,2), other_controllable_cost DECIMAL(18,2),
    source_system VARCHAR(100) NOT NULL, source_record_id VARCHAR(150), import_batch_id UUID REFERENCES operating_fact_batches(id),
    as_of_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(), version INTEGER NOT NULL DEFAULT 1,
    reconciliation_status VARCHAR(30) NOT NULL DEFAULT 'unreconciled', mapping_status VARCHAR(30) NOT NULL DEFAULT 'mapped', note TEXT,
    created_by UUID REFERENCES users(id), created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(), updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (period ~ '^[0-9]{4}-[0-9]{2}$'), CHECK (period_basis IN ('calendar_month','fiscal_month','quarter','year')),
    CHECK (reconciliation_status IN ('unreconciled','matched','warning','failed')), CHECK (mapping_status IN ('mapped','unmapped','ambiguous')),
    CHECK (revenue >= 0), UNIQUE (store_id, period, version, source_system)
);
CREATE INDEX IF NOT EXISTS idx_store_operating_facts_period ON store_operating_facts(store_id, period, version DESC);
CREATE TABLE IF NOT EXISTS equipment_assets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(), legal_entity_id UUID REFERENCES legal_entities(id), plant_code VARCHAR(100) NOT NULL,
    production_line_code VARCHAR(100), equipment_code VARCHAR(150) NOT NULL, equipment_name VARCHAR(255) NOT NULL, cost_center VARCHAR(100),
    asset_identifier VARCHAR(150), contract_id UUID REFERENCES lease_contracts(id), asset_type VARCHAR(100), capacity DECIMAL(18,4), capacity_unit VARCHAR(40),
    currency VARCHAR(10), external_system VARCHAR(100), external_id VARCHAR(150), effective_from DATE, effective_to DATE, active BOOLEAN NOT NULL DEFAULT true,
    created_by UUID REFERENCES users(id), created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(), updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (legal_entity_id, equipment_code)
);
CREATE INDEX IF NOT EXISTS idx_equipment_assets_plant_line ON equipment_assets(legal_entity_id, plant_code, production_line_code);
CREATE TABLE IF NOT EXISTS equipment_operating_facts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(), equipment_id UUID NOT NULL REFERENCES equipment_assets(id) ON DELETE CASCADE, period VARCHAR(7) NOT NULL,
    currency VARCHAR(10) NOT NULL, output_qty DECIMAL(18,4), yield_pct DECIMAL(9,4), scrap_qty DECIMAL(18,4), downtime_hours DECIMAL(18,4),
    oee_pct DECIMAL(9,4), utilization_pct DECIMAL(9,4), labor_cost DECIMAL(18,2), energy_cost DECIMAL(18,2), maintenance_cost DECIMAL(18,2),
    standard_cost DECIMAL(18,2), actual_cost DECIMAL(18,2), material_usage_cost DECIMAL(18,2), overhead_absorption DECIMAL(18,2),
    source_system VARCHAR(100) NOT NULL, source_record_id VARCHAR(150), import_batch_id UUID REFERENCES operating_fact_batches(id), as_of_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    version INTEGER NOT NULL DEFAULT 1, reconciliation_status VARCHAR(30) NOT NULL DEFAULT 'unreconciled', created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(), updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (period ~ '^[0-9]{4}-[0-9]{2}$'), CHECK (reconciliation_status IN ('unreconciled','matched','warning','failed')),
    UNIQUE (equipment_id, period, version, source_system)
);
CREATE INDEX IF NOT EXISTS idx_equipment_operating_facts_period ON equipment_operating_facts(equipment_id, period, version DESC);
CREATE TABLE IF NOT EXISTS fpna_action_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(), legal_entity_id UUID REFERENCES legal_entities(id), period VARCHAR(7), category VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL DEFAULT 'medium', status VARCHAR(30) NOT NULL DEFAULT 'open', title VARCHAR(255) NOT NULL, description TEXT NOT NULL DEFAULT '',
    rule_code VARCHAR(100) NOT NULL, source_table VARCHAR(100) NOT NULL, source_record_id VARCHAR(150) NOT NULL, data_version VARCHAR(100) NOT NULL DEFAULT '', idempotency_key VARCHAR(255),
    impact_amount DECIMAL(18,2), currency VARCHAR(10), owner_id UUID REFERENCES users(id), owner_name VARCHAR(255), due_date DATE,
    baseline_amount DECIMAL(18,2), target_amount DECIMAL(18,2), expected_benefit DECIMAL(18,2), verification_period VARCHAR(7), verified_amount DECIMAL(18,2),
    verification_status VARCHAR(30) NOT NULL DEFAULT 'not_due', human_root_cause TEXT, planned_action TEXT, ai_suggestion TEXT, evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by UUID REFERENCES users(id), updated_by UUID REFERENCES users(id), created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(), updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    acknowledged_at TIMESTAMP WITH TIME ZONE, completed_at TIMESTAMP WITH TIME ZONE, verified_at TIMESTAMP WITH TIME ZONE,
    CHECK (severity IN ('critical','high','medium','low','informational')), CHECK (status IN ('open','acknowledged','in_progress','completed','verified','accepted','dismissed')),
    CHECK (verification_status IN ('not_due','pending','verified','failed','not_applicable')),
    UNIQUE (legal_entity_id, rule_code, source_table, source_record_id, period)
);
CREATE INDEX IF NOT EXISTS idx_fpna_action_items_queue ON fpna_action_items(legal_entity_id, status, severity, due_date);
ALTER TABLE fpna_action_items DROP CONSTRAINT IF EXISTS fpna_action_items_rule_code_source_table_source_record_id_period_key;
CREATE UNIQUE INDEX IF NOT EXISTS ux_fpna_action_items_scope_dedupe ON fpna_action_items(legal_entity_id, rule_code, source_table, source_record_id, period);
CREATE UNIQUE INDEX IF NOT EXISTS ux_fpna_action_items_idempotency ON fpna_action_items(legal_entity_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE TABLE IF NOT EXISTS fpna_assumption_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(), legal_entity_id UUID REFERENCES legal_entities(id), assumption_key VARCHAR(150) NOT NULL,
    category VARCHAR(50) NOT NULL, value JSONB NOT NULL, unit VARCHAR(50), source VARCHAR(255) NOT NULL, owner_name VARCHAR(255),
    effective_from DATE NOT NULL, effective_to DATE, version INTEGER NOT NULL DEFAULT 1, status VARCHAR(30) NOT NULL DEFAULT 'draft',
    created_by UUID REFERENCES users(id), created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (status IN ('draft','approved','retired')), UNIQUE (legal_entity_id, assumption_key, version)
);
CREATE INDEX IF NOT EXISTS idx_fpna_assumption_versions_lookup ON fpna_assumption_versions(legal_entity_id, assumption_key, effective_from DESC, version DESC);
CREATE TABLE IF NOT EXISTS fpna_scenario_drafts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(), legal_entity_id UUID REFERENCES legal_entities(id), scenario_type VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL, assumptions JSONB NOT NULL DEFAULT '{}'::jsonb, result JSONB, data_version VARCHAR(100) NOT NULL DEFAULT '',
    status VARCHAR(30) NOT NULL DEFAULT 'draft', source_run_id VARCHAR(150), idempotency_key VARCHAR(255), created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(), updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (status IN ('draft','reviewed','approved','rejected'))
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_fpna_scenario_drafts_idempotency ON fpna_scenario_drafts(legal_entity_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_fpna_scenario_drafts_lookup ON fpna_scenario_drafts(legal_entity_id, scenario_type, created_at DESC);
INSERT INTO permissions (role_id, resource, action) VALUES
    ('22222222-2222-2222-2222-222222222222','fpna_actions','write'),
    ('33333333-3333-3333-3333-333333333333','fpna_actions','write'),
    ('44444444-4444-4444-4444-444444444444','fpna_actions','write'),
    ('55555555-5555-5555-5555-555555555555','fpna_actions','read')
ON CONFLICT (role_id, resource, action) DO NOTHING;

-- ============================================================================
-- Migration 035: governed FP&A plans, mappings, data quality and artifacts
ALTER TABLE operating_fact_batches ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(255), ADD COLUMN IF NOT EXISTS fact_version VARCHAR(100) NOT NULL DEFAULT '', ADD COLUMN IF NOT EXISTS retry_of_batch_id UUID REFERENCES operating_fact_batches(id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_operating_fact_batches_idempotency ON operating_fact_batches(legal_entity_id, idempotency_key) WHERE idempotency_key IS NOT NULL;
ALTER TABLE store_operating_facts ADD COLUMN IF NOT EXISTS business_segment VARCHAR(100), ADD COLUMN IF NOT EXISTS fiscal_year VARCHAR(20), ADD COLUMN IF NOT EXISTS store_age_months INTEGER, ADD COLUMN IF NOT EXISTS cohort_code VARCHAR(100), ADD COLUMN IF NOT EXISTS data_quality_status VARCHAR(30) NOT NULL DEFAULT 'unassessed';
ALTER TABLE equipment_operating_facts ADD COLUMN IF NOT EXISTS purchase_price DECIMAL(18,2), ADD COLUMN IF NOT EXISTS purchase_price_variance DECIMAL(18,2), ADD COLUMN IF NOT EXISTS capacity_available DECIMAL(18,4), ADD COLUMN IF NOT EXISTS lease_cost DECIMAL(18,2), ADD COLUMN IF NOT EXISTS contractual_rent DECIMAL(18,2), ADD COLUMN IF NOT EXISTS data_quality_status VARCHAR(30) NOT NULL DEFAULT 'unassessed';
CREATE TABLE IF NOT EXISTS fpna_plan_versions (
 id UUID PRIMARY KEY DEFAULT uuid_generate_v4(), legal_entity_id UUID REFERENCES legal_entities(id), name VARCHAR(255) NOT NULL, version_type VARCHAR(20) NOT NULL, scenario_type VARCHAR(30) NOT NULL DEFAULT 'baseline', source VARCHAR(255) NOT NULL, coverage_scope JSONB NOT NULL DEFAULT '{}'::jsonb, currency VARCHAR(10), as_of_period VARCHAR(7) NOT NULL, from_period VARCHAR(7) NOT NULL, to_period VARCHAR(7) NOT NULL, actual_cutoff_period VARCHAR(7), prior_version_id UUID REFERENCES fpna_plan_versions(id), assumption_version VARCHAR(100), exchange_rate_version VARCHAR(100), metric_definition_version VARCHAR(100), status VARCHAR(30) NOT NULL DEFAULT 'draft', is_official BOOLEAN NOT NULL DEFAULT false, frozen_at TIMESTAMP WITH TIME ZONE, approved_at TIMESTAMP WITH TIME ZONE, created_by UUID REFERENCES users(id), created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(), CHECK (version_type IN ('actual','prior_year','budget','forecast','scenario')), CHECK (scenario_type IN ('baseline','upside','downside','custom')), CHECK (status IN ('draft','review','approved','official','retired')), CHECK (from_period <= to_period), UNIQUE (legal_entity_id,name,as_of_period)
);
CREATE INDEX IF NOT EXISTS idx_fpna_plan_versions_lookup ON fpna_plan_versions(legal_entity_id,version_type,as_of_period DESC,created_at DESC);
CREATE TABLE IF NOT EXISTS fpna_plan_lines (
 id UUID PRIMARY KEY DEFAULT uuid_generate_v4(), plan_version_id UUID NOT NULL REFERENCES fpna_plan_versions(id) ON DELETE CASCADE, period VARCHAR(7) NOT NULL, grain VARCHAR(30) NOT NULL DEFAULT 'group', legal_entity_id UUID REFERENCES legal_entities(id), business_segment VARCHAR(100), brand VARCHAR(100), region VARCHAR(100), store_id UUID REFERENCES stores(id), plant_code VARCHAR(100), production_line_code VARCHAR(100), equipment_id UUID REFERENCES equipment_assets(id), asset_type VARCHAR(100), currency VARCHAR(10) NOT NULL, revenue DECIMAL(18,2), gross_profit DECIMAL(18,2), labor_cost DECIMAL(18,2), fixed_rent DECIMAL(18,2), variable_rent DECIMAL(18,2), non_lease_cost DECIMAL(18,2), four_wall_ebitda DECIMAL(18,2), cash_flow DECIMAL(18,2), net_debt DECIMAL(18,2), operational_kpis JSONB NOT NULL DEFAULT '{}'::jsonb, source_system VARCHAR(100) NOT NULL, source_record_id VARCHAR(150), as_of_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(), actual_flag BOOLEAN NOT NULL DEFAULT false, forecast_flag BOOLEAN NOT NULL DEFAULT false, scenario_inputs JSONB NOT NULL DEFAULT '{}'::jsonb, CHECK (period ~ '^[0-9]{4}-[0-9]{2}$'), UNIQUE (plan_version_id,period,grain,legal_entity_id,business_segment,brand,region,store_id,plant_code,production_line_code,equipment_id,asset_type,currency)
);
CREATE INDEX IF NOT EXISTS idx_fpna_plan_lines_period ON fpna_plan_lines(plan_version_id,period,grain);
CREATE TABLE IF NOT EXISTS fpna_metric_definitions (
 id UUID PRIMARY KEY DEFAULT uuid_generate_v4(), metric_key VARCHAR(100) NOT NULL, version VARCHAR(100) NOT NULL, display_name VARCHAR(255) NOT NULL, formula TEXT NOT NULL, grain VARCHAR(100) NOT NULL, currency_policy VARCHAR(100) NOT NULL, fiscal_period_rule VARCHAR(255) NOT NULL, exclusions JSONB NOT NULL DEFAULT '{}'::jsonb, owner_name VARCHAR(255) NOT NULL, effective_from DATE NOT NULL, effective_to DATE, status VARCHAR(30) NOT NULL DEFAULT 'draft', created_by UUID REFERENCES users(id), created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(), CHECK (status IN ('draft','approved','retired')), UNIQUE (metric_key,version)
);
CREATE TABLE IF NOT EXISTS fpna_master_data_mappings (
 id UUID PRIMARY KEY DEFAULT uuid_generate_v4(), legal_entity_id UUID REFERENCES legal_entities(id), mapping_type VARCHAR(30) NOT NULL, external_system VARCHAR(100) NOT NULL, external_id VARCHAR(255) NOT NULL, external_name VARCHAR(255), alias VARCHAR(255), target_id UUID, target_code VARCHAR(150), effective_from DATE NOT NULL, effective_to DATE, status VARCHAR(30) NOT NULL DEFAULT 'draft', evidence JSONB NOT NULL DEFAULT '{}'::jsonb, created_by UUID REFERENCES users(id), created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(), CHECK (mapping_type IN ('legal_entity','cost_center','store','plant','line','equipment','contract')), CHECK (status IN ('draft','approved','retired')), UNIQUE (legal_entity_id,mapping_type,external_system,external_id,effective_from)
);
CREATE INDEX IF NOT EXISTS idx_fpna_master_data_mappings_lookup ON fpna_master_data_mappings(legal_entity_id,mapping_type,external_system,effective_from DESC);
CREATE TABLE IF NOT EXISTS fpna_data_quality_items (
 id UUID PRIMARY KEY DEFAULT uuid_generate_v4(), legal_entity_id UUID REFERENCES legal_entities(id), batch_id UUID REFERENCES operating_fact_batches(id), period VARCHAR(7), dimension VARCHAR(50) NOT NULL, category VARCHAR(50) NOT NULL, severity VARCHAR(20) NOT NULL DEFAULT 'medium', source_table VARCHAR(100) NOT NULL, source_record_id VARCHAR(150) NOT NULL, data_version VARCHAR(100) NOT NULL DEFAULT '', description TEXT NOT NULL, status VARCHAR(30) NOT NULL DEFAULT 'open', evidence JSONB NOT NULL DEFAULT '{}'::jsonb, created_by UUID REFERENCES users(id), created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(), resolved_at TIMESTAMP WITH TIME ZONE, CHECK (category IN ('unmapped','ambiguous_mapping','missing','low_confidence','reconciliation','duplicate','invalid')), CHECK (status IN ('open','acknowledged','resolved','accepted'))
);
CREATE INDEX IF NOT EXISTS idx_fpna_data_quality_queue ON fpna_data_quality_items(legal_entity_id,status,severity,period);
CREATE TABLE IF NOT EXISTS fpna_action_realizations (
 id UUID PRIMARY KEY DEFAULT uuid_generate_v4(), action_id UUID NOT NULL REFERENCES fpna_action_items(id) ON DELETE CASCADE, period VARCHAR(7) NOT NULL, baseline_amount DECIMAL(18,2), target_amount DECIMAL(18,2), actual_amount DECIMAL(18,2), realized_benefit DECIMAL(18,2), currency VARCHAR(10), source_table VARCHAR(100) NOT NULL, source_record_id VARCHAR(150) NOT NULL, data_version VARCHAR(100) NOT NULL DEFAULT '', status VARCHAR(30) NOT NULL DEFAULT 'pending', evidence JSONB NOT NULL DEFAULT '{}'::jsonb, verified_by UUID REFERENCES users(id), verified_at TIMESTAMP WITH TIME ZONE, created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(), CHECK (status IN ('pending','verified','failed')), UNIQUE (action_id,period)
);
CREATE TABLE IF NOT EXISTS fpna_decision_memos (
 id UUID PRIMARY KEY DEFAULT uuid_generate_v4(), legal_entity_id UUID REFERENCES legal_entities(id), memo_type VARCHAR(50) NOT NULL, title VARCHAR(255) NOT NULL, basis VARCHAR(20) NOT NULL DEFAULT 'Scenario', status VARCHAR(30) NOT NULL DEFAULT 'draft', scenario_draft_id UUID REFERENCES fpna_scenario_drafts(id), system_facts JSONB NOT NULL DEFAULT '{}'::jsonb, deterministic_calculations JSONB NOT NULL DEFAULT '{}'::jsonb, human_inputs JSONB NOT NULL DEFAULT '{}'::jsonb, ai_narrative JSONB NOT NULL DEFAULT '{}'::jsonb, source_references JSONB NOT NULL DEFAULT '[]'::jsonb, data_version VARCHAR(100) NOT NULL DEFAULT '', assumption_version VARCHAR(100) NOT NULL DEFAULT '', metric_definition_version VARCHAR(100) NOT NULL DEFAULT '', idempotency_key VARCHAR(255), created_by UUID REFERENCES users(id), created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(), updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(), reviewed_by UUID REFERENCES users(id), reviewed_at TIMESTAMP WITH TIME ZONE, CHECK (basis IN ('Working','Official','Scenario')), CHECK (status IN ('draft','review','approved','rejected'))
);
CREATE INDEX IF NOT EXISTS idx_fpna_decision_memos_lookup ON fpna_decision_memos(legal_entity_id,memo_type,status,created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS ux_fpna_decision_memos_idempotency ON fpna_decision_memos(legal_entity_id,idempotency_key) WHERE idempotency_key IS NOT NULL;
CREATE TABLE IF NOT EXISTS fpna_report_artifacts (
 id UUID PRIMARY KEY DEFAULT uuid_generate_v4(), legal_entity_id UUID REFERENCES legal_entities(id), report_type VARCHAR(20) NOT NULL, view_type VARCHAR(30) NOT NULL DEFAULT 'group', period VARCHAR(7) NOT NULL, basis VARCHAR(20) NOT NULL DEFAULT 'Working', format VARCHAR(20) NOT NULL, status VARCHAR(30) NOT NULL DEFAULT 'draft', payload JSONB NOT NULL DEFAULT '{}'::jsonb, source_metadata JSONB NOT NULL DEFAULT '{}'::jsonb, manifest_sha256 VARCHAR(128), data_version VARCHAR(100) NOT NULL DEFAULT '', assumption_version VARCHAR(100) NOT NULL DEFAULT '', metric_definition_version VARCHAR(100) NOT NULL DEFAULT '', generated_by UUID REFERENCES users(id), generated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(), CHECK (report_type IN ('WBR','MBR','QBR')), CHECK (basis IN ('Working','Official','Scenario')), CHECK (format IN ('json','html','csv','xlsx','pdf','pptx')), CHECK (status IN ('draft','review','published','failed'))
);
CREATE INDEX IF NOT EXISTS idx_fpna_report_artifacts_lookup ON fpna_report_artifacts(legal_entity_id,report_type,period,basis,generated_at DESC);
CREATE TABLE IF NOT EXISTS fpna_agent_signals (
 id UUID PRIMARY KEY DEFAULT uuid_generate_v4(), legal_entity_id UUID REFERENCES legal_entities(id), period VARCHAR(7), rule_code VARCHAR(100) NOT NULL, severity VARCHAR(20) NOT NULL DEFAULT 'medium', source_table VARCHAR(100) NOT NULL, source_record_id VARCHAR(150) NOT NULL, data_version VARCHAR(100) NOT NULL DEFAULT '', signal JSONB NOT NULL DEFAULT '{}'::jsonb, status VARCHAR(30) NOT NULL DEFAULT 'open', created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(), CHECK (status IN ('open','acknowledged','dismissed'))
);
INSERT INTO permissions (role_id,resource,action) VALUES
 ('22222222-2222-2222-2222-222222222222','fpna_memos','write'),('33333333-3333-3333-3333-333333333333','fpna_memos','write'),('44444444-4444-4444-4444-444444444444','fpna_memos','write'),('55555555-5555-5555-5555-555555555555','fpna_memos','read'),
 ('22222222-2222-2222-2222-222222222222','fpna_reports','write'),('33333333-3333-3333-3333-333333333333','fpna_reports','write'),('44444444-4444-4444-4444-444444444444','fpna_reports','write'),('55555555-5555-5555-5555-555555555555','fpna_reports','read'),
 ('22222222-2222-2222-2222-222222222222','fpna_mappings','write'),('33333333-3333-3333-3333-333333333333','fpna_mappings','write'),('44444444-4444-4444-4444-444444444444','fpna_mappings','write'),('55555555-5555-5555-5555-555555555555','fpna_data_quality','read'),('22222222-2222-2222-2222-222222222222','fpna_data_quality','write'),('33333333-3333-3333-3333-333333333333','fpna_data_quality','write'),('44444444-4444-4444-4444-444444444444','fpna_data_quality','write')
ON CONFLICT (role_id,resource,action) DO NOTHING;

-- ============================================================================
-- Migration 038: daily retail operating facts with explicit simulated source.
CREATE TABLE IF NOT EXISTS retail_store_day_facts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    store_id UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    business_date DATE NOT NULL,
    currency VARCHAR(3) NOT NULL,
    revenue DECIMAL(18,2) NOT NULL DEFAULT 0,
    gross_profit DECIMAL(18,2),
    transactions DECIMAL(18,2),
    footfall DECIMAL(18,2),
    area_sqm DECIMAL(18,2),
    labor_cost DECIMAL(18,2),
    fixed_rent DECIMAL(18,2),
    variable_rent DECIMAL(18,2),
    non_lease_cost DECIMAL(18,2),
    other_controllable_cost DECIMAL(18,2),
    source_system VARCHAR(100) NOT NULL,
    source_record_id VARCHAR(150),
    import_batch_id UUID REFERENCES operating_fact_batches(id),
    as_of_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    version INTEGER NOT NULL DEFAULT 1,
    reconciliation_status VARCHAR(30) NOT NULL DEFAULT 'unreconciled',
    mapping_status VARCHAR(30) NOT NULL DEFAULT 'mapped',
    data_quality_status VARCHAR(30) NOT NULL DEFAULT 'unassessed',
    data_classification VARCHAR(20) NOT NULL,
    simulation_dataset_version VARCHAR(100),
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (currency ~ '^[A-Z]{3}$'),
    CHECK (version > 0),
    CHECK (BTRIM(source_system) <> ''),
    CHECK (reconciliation_status IN ('unreconciled', 'matched', 'warning', 'failed')),
    CHECK (mapping_status IN ('mapped', 'unmapped', 'ambiguous')),
    CHECK (data_quality_status IN ('unassessed', 'valid', 'warning', 'invalid')),
    CHECK (data_classification IN ('production', 'simulated')),
    CHECK (
        (data_classification = 'simulated' AND NULLIF(BTRIM(simulation_dataset_version), '') IS NOT NULL)
        OR (data_classification = 'production' AND simulation_dataset_version IS NULL)
    ),
    CHECK (revenue >= 0),
    CHECK (gross_profit IS NULL OR gross_profit >= 0),
    CHECK (transactions IS NULL OR transactions >= 0),
    CHECK (footfall IS NULL OR footfall >= 0),
    CHECK (area_sqm IS NULL OR area_sqm >= 0),
    CHECK (labor_cost IS NULL OR labor_cost >= 0),
    CHECK (fixed_rent IS NULL OR fixed_rent >= 0),
    CHECK (variable_rent IS NULL OR variable_rent >= 0),
    CHECK (non_lease_cost IS NULL OR non_lease_cost >= 0),
    CHECK (other_controllable_cost IS NULL OR other_controllable_cost >= 0),
    UNIQUE (store_id, business_date, version, source_system)
);
CREATE INDEX IF NOT EXISTS idx_retail_store_day_facts_lookup
    ON retail_store_day_facts(store_id, business_date, version DESC);

-- Request-level idempotency is separate from the fact business key.  A
-- replay must not re-run an upsert or append another audit event, while a
-- different payload under the same key is a deterministic conflict.
CREATE TABLE IF NOT EXISTS retail_store_day_fact_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    scope_key VARCHAR(100) NOT NULL,
    legal_entity_id UUID REFERENCES legal_entities(id),
    idempotency_key VARCHAR(255) NOT NULL,
    payload_sha256 VARCHAR(64) NOT NULL,
    fact_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (BTRIM(scope_key) <> ''),
    CHECK (BTRIM(idempotency_key) <> ''),
    CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
    UNIQUE (scope_key, idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_retail_store_day_fact_requests_entity
    ON retail_store_day_fact_requests(legal_entity_id, created_at DESC);

-- ============================================================================
-- Migration 039: deterministic retail simulation datasets and store source flags.
ALTER TABLE stores
    ADD COLUMN IF NOT EXISTS data_classification VARCHAR(20) NOT NULL DEFAULT 'production',
    ADD COLUMN IF NOT EXISTS simulation_dataset_version VARCHAR(100);
UPDATE stores SET data_classification = 'production'
WHERE data_classification IS NULL OR BTRIM(data_classification) = '';
ALTER TABLE stores
    ALTER COLUMN data_classification SET DEFAULT 'production',
    ALTER COLUMN data_classification SET NOT NULL;
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'stores'::regclass AND conname = 'stores_data_classification_check') THEN
        ALTER TABLE stores ADD CONSTRAINT stores_data_classification_check CHECK (data_classification IN ('production', 'simulated'));
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = 'stores'::regclass AND conname = 'stores_simulation_version_check') THEN
        ALTER TABLE stores ADD CONSTRAINT stores_simulation_version_check CHECK (
            (data_classification = 'simulated' AND NULLIF(BTRIM(simulation_dataset_version), '') IS NOT NULL)
            OR (data_classification = 'production' AND simulation_dataset_version IS NULL)
        );
    END IF;
END $$;
CREATE TABLE IF NOT EXISTS retail_simulation_datasets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID NOT NULL REFERENCES legal_entities(id),
    dataset_version VARCHAR(100) NOT NULL,
    generator_version VARCHAR(100) NOT NULL,
    seed BIGINT NOT NULL,
    date_from DATE NOT NULL,
    date_to DATE NOT NULL,
    store_count INTEGER NOT NULL,
    fact_count INTEGER NOT NULL DEFAULT 0,
    parameters JSONB NOT NULL DEFAULT '{}'::jsonb,
    anomaly_manifest JSONB NOT NULL DEFAULT '[]'::jsonb,
    payload_sha256 VARCHAR(64) NOT NULL,
    business_sha256 VARCHAR(64) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'generating',
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    idempotency_key VARCHAR(255),
    import_batch_id UUID REFERENCES operating_fact_batches(id),
    CHECK (date_to >= date_from),
    CHECK (store_count BETWEEN 10 AND 100),
    CHECK (fact_count >= 0),
    CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
    CHECK (business_sha256 ~ '^[0-9a-f]{64}$'),
    CHECK (status IN ('generating', 'completed', 'failed')),
    UNIQUE (legal_entity_id, dataset_version)
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_retail_simulation_datasets_idempotency
    ON retail_simulation_datasets(legal_entity_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_retail_simulation_datasets_entity_created
    ON retail_simulation_datasets(legal_entity_id, created_at DESC);

-- Migration 044: versioned, content-identified GL Trial Balance
-- (ADR-0009). Functional currency is the reconciliation basis; versions are
-- deduplicated by content hash so re-importing the same extract replays the
-- same version instead of creating a duplicate.
CREATE TABLE IF NOT EXISTS gl_trial_balance_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID REFERENCES legal_entities(id),
    name VARCHAR(255) NOT NULL,
    source_system VARCHAR(100) NOT NULL,
    period VARCHAR(7) NOT NULL,
    functional_currency VARCHAR(3) NOT NULL,
    content_sha256 VARCHAR(64) NOT NULL,
    total_debit DECIMAL(18,2) NOT NULL DEFAULT 0,
    total_credit DECIMAL(18,2) NOT NULL DEFAULT 0,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (period ~ '^[0-9]{4}-(0[1-9]|1[0-2])$'),
    CHECK (functional_currency ~ '^[A-Z]{3}$'),
    UNIQUE (legal_entity_id, source_system, period, content_sha256)
);

CREATE TABLE IF NOT EXISTS gl_trial_balance_lines (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    trial_balance_version_id UUID NOT NULL REFERENCES gl_trial_balance_versions(id) ON DELETE CASCADE,
    account_code VARCHAR(100) NOT NULL,
    account_name VARCHAR(255),
    debit DECIMAL(18,2) NOT NULL DEFAULT 0,
    credit DECIMAL(18,2) NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (debit >= 0),
    CHECK (credit >= 0),
    UNIQUE (trial_balance_version_id, account_code)
);

CREATE INDEX IF NOT EXISTS idx_gl_trial_balance_versions_lookup
    ON gl_trial_balance_versions(legal_entity_id, period DESC, created_at DESC);

-- End of init script
