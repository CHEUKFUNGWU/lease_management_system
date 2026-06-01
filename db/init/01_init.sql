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
    currency VARCHAR(10) NOT NULL DEFAULT 'CNY',
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
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
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
    asset_type VARCHAR(50) NOT NULL DEFAULT 'real_estate',
    asset_category VARCHAR(100),
    property_category VARCHAR(100),
    currency VARCHAR(10) NOT NULL DEFAULT 'CNY',
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
    currency VARCHAR(10) NOT NULL DEFAULT 'CNY',
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
    ADD COLUMN IF NOT EXISTS lease_scope VARCHAR(50) NOT NULL DEFAULT 'in_scope'
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
    ADD COLUMN IF NOT EXISTS asset_type VARCHAR(50) NOT NULL DEFAULT 'real_estate'
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
CREATE INDEX IF NOT EXISTS idx_lease_payment_schedules_contract ON lease_payment_schedules(contract_id);
CREATE INDEX IF NOT EXISTS idx_lease_events_status ON lease_events(approval_status);
CREATE INDEX IF NOT EXISTS idx_ai_contract_drafts_status ON ai_contract_drafts(approval_status);

-- 10. Preset system roles
INSERT INTO roles (id, name, description) VALUES
    ('11111111-1111-1111-1111-111111111111', 'System Admin', '系统管理员：管理用户、角色、主数据和系统参数'),
    ('22222222-2222-2222-2222-222222222222', 'Finance Editor', '财务录入员：上传合同、维护草稿、录入台账'),
    ('33333333-3333-3333-3333-333333333333', 'Finance Reviewer', '财务复核员：复核合同、付款计划和事件草稿'),
    ('44444444-4444-4444-4444-444444444444', 'Finance Approver', '财务审批员：审批正式入库和关键会计处理'),
    ('55555555-5555-5555-5555-555555555555', 'Auditor Readonly', '审计只读：只读查看合同、台账、摊销和审计轨迹'),
    ('66666666-6666-6666-6666-666666666666', 'Business Readonly', '业务只读：查看授权范围内的合同和报表')
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
    currency VARCHAR(10) NOT NULL DEFAULT 'CNY',
    description TEXT,
    voucher_number VARCHAR(100),
    posting_status VARCHAR(20) NOT NULL DEFAULT 'draft', -- draft, preview, approved, posted, reversed
    posted_at TIMESTAMP WITH TIME ZONE,
    posted_by UUID REFERENCES users(id),
    approved_by UUID REFERENCES users(id),
    approved_at TIMESTAMP WITH TIME ZONE,
    erp_reference VARCHAR(255),
    batch_id UUID,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 3. 月结批次表
CREATE TABLE IF NOT EXISTS monthly_closing_batches (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    batch_number VARCHAR(50) NOT NULL UNIQUE,
    accounting_period VARCHAR(7) NOT NULL,
    legal_entity_id UUID REFERENCES legal_entities(id),
    region VARCHAR(100),
    brand VARCHAR(100),
    status VARCHAR(20) NOT NULL DEFAULT 'draft', -- draft, running, completed, failed, cancelled
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
CREATE INDEX IF NOT EXISTS idx_monthly_closing_period ON monthly_closing_batches(accounting_period);

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

INSERT INTO system_settings (setting_key, setting_value, description)
VALUES ('global_discount_rate', '0.05', '集团默认折现率（年化，小数）')
ON CONFLICT (setting_key) DO NOTHING;

-- ============================================================================
-- End of init script
