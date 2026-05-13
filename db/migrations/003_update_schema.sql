-- +goose Up
-- +goose StatementBegin

-- 1. 角色权限映射表（用户-角色多对多）
CREATE TABLE IF NOT EXISTS user_roles (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    assigned_by UUID REFERENCES users(id),
    assigned_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, role_id)
);

-- 2. 用户数据权限范围表
CREATE TABLE IF NOT EXISTS user_data_scopes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    dimension VARCHAR(50) NOT NULL CHECK (dimension IN ('legal_entity', 'store', 'region', 'brand')),
    target_id VARCHAR(100) NOT NULL,
    target_name VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, dimension, target_id)
);

-- 3. AI 草稿审批状态枚举类型
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

-- 4. 合同表增加审批和版本控制字段
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

-- 5. 合同表增加 discount rate 人机协同字段
ALTER TABLE lease_contracts
    ADD COLUMN IF NOT EXISTS discount_rate_missing BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS discount_rate_source VARCHAR(100),
    ADD COLUMN IF NOT EXISTS discount_rate_policy_id UUID,
    ADD COLUMN IF NOT EXISTS discount_rate_confirmed_by UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS discount_rate_confirmed_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS ai_extracted_discount_rate TEXT,
    ADD COLUMN IF NOT EXISTS ai_suggested_rate_policies JSONB;

-- 6. 付款计划表增加审批状态
ALTER TABLE lease_payment_schedules
    ADD COLUMN IF NOT EXISTS approval_status VARCHAR(50) NOT NULL DEFAULT 'approved',
    ADD COLUMN IF NOT EXISTS is_official_version BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS reviewed_by UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS approved_by UUID REFERENCES users(id);

-- 7. 事件表增加审批状态
ALTER TABLE lease_events
    ADD COLUMN IF NOT EXISTS approval_status VARCHAR(50) NOT NULL DEFAULT 'pending',
    ADD COLUMN IF NOT EXISTS is_official_version BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS reviewed_by UUID REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS rejected_reason TEXT;

-- 8. AI 草稿表增强字段
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

-- 9. 添加索引
CREATE INDEX IF NOT EXISTS idx_user_roles_user ON user_roles(user_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_role ON user_roles(role_id);
CREATE INDEX IF NOT EXISTS idx_user_data_scopes_user ON user_data_scopes(user_id);
CREATE INDEX IF NOT EXISTS idx_lease_contracts_approval ON lease_contracts(approval_status);
CREATE INDEX IF NOT EXISTS idx_lease_contracts_official ON lease_contracts(is_official_version);
CREATE INDEX IF NOT EXISTS idx_lease_contracts_dr_missing ON lease_contracts(discount_rate_missing);
CREATE INDEX IF NOT EXISTS idx_lease_payment_schedules_contract ON lease_payment_schedules(contract_id);
CREATE INDEX IF NOT EXISTS idx_lease_events_status ON lease_events(approval_status);
CREATE INDEX IF NOT EXISTS idx_ai_contract_drafts_status ON ai_contract_drafts(approval_status);

-- 10. 预置系统角色
INSERT INTO roles (id, name, description) VALUES
    ('11111111-1111-1111-1111-111111111111', 'System Admin', '系统管理员：管理用户、角色、主数据和系统参数'),
    ('22222222-2222-2222-2222-222222222222', 'Finance Editor', '财务录入员：上传合同、维护草稿、录入台账'),
    ('33333333-3333-3333-3333-333333333333', 'Finance Reviewer', '财务复核员：复核合同、付款计划和事件草稿'),
    ('44444444-4444-4444-4444-444444444444', 'Finance Approver', '财务审批员：审批正式入库和关键会计处理'),
    ('55555555-5555-5555-5555-555555555555', 'Auditor Readonly', '审计只读：只读查看合同、台账、摊销和审计轨迹'),
    ('66666666-6666-6666-6666-666666666666', 'Business Readonly', '业务只读：查看授权范围内的合同和报表')
ON CONFLICT (name) DO NOTHING;

-- 11. 预置权限
INSERT INTO permissions (role_id, resource, action) VALUES
    -- System Admin 权限
    ('11111111-1111-1111-1111-111111111111', '*', '*'),
    -- Finance Editor 权限
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
    -- Finance Reviewer 权限
    ('33333333-3333-3333-3333-333333333333', 'contracts', 'read'),
    ('33333333-3333-3333-3333-333333333333', 'contracts', 'review'),
    ('33333333-3333-3333-3333-333333333333', 'payment_schedules', 'read'),
    ('33333333-3333-3333-3333-333333333333', 'payment_schedules', 'review'),
    ('33333333-3333-3333-3333-333333333333', 'events', 'read'),
    ('33333333-3333-3333-3333-333333333333', 'events', 'review'),
    ('33333333-3333-3333-3333-333333333333', 'ai_drafts', 'read'),
    ('33333333-3333-3333-3333-333333333333', 'ai_drafts', 'review'),
    -- Finance Approver 权限
    ('44444444-4444-4444-4444-444444444444', 'contracts', 'read'),
    ('44444444-4444-4444-4444-444444444444', 'contracts', 'approve'),
    ('44444444-4444-4444-4444-444444444444', 'payment_schedules', 'read'),
    ('44444444-4444-4444-4444-444444444444', 'payment_schedules', 'approve'),
    ('44444444-4444-4444-4444-444444444444', 'events', 'read'),
    ('44444444-4444-4444-4444-444444444444', 'events', 'approve'),
    ('44444444-4444-4444-4444-444444444444', 'calculations', 'trigger'),
    -- Auditor Readonly 权限
    ('55555555-5555-5555-5555-555555555555', 'contracts', 'read'),
    ('55555555-5555-5555-5555-555555555555', 'payment_schedules', 'read'),
    ('55555555-5555-5555-5555-555555555555', 'events', 'read'),
    ('55555555-5555-5555-5555-555555555555', 'calculations', 'read'),
    ('55555555-5555-5555-5555-555555555555', 'audit_logs', 'read'),
    ('55555555-5555-5555-5555-555555555555', 'reports', 'read'),
    ('55555555-5555-5555-5555-555555555555', 'reports', 'export'),
    -- Business Readonly 权限
    ('66666666-6666-6666-6666-666666666666', 'contracts', 'read'),
    ('66666666-6666-6666-6666-666666666666', 'reports', 'read')
ON CONFLICT (role_id, resource, action) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_ai_contract_drafts_status;
DROP INDEX IF EXISTS idx_lease_events_status;
DROP INDEX IF EXISTS idx_lease_payment_schedules_contract;
DROP INDEX IF EXISTS idx_lease_contracts_dr_missing;
DROP INDEX IF EXISTS idx_lease_contracts_official;
DROP INDEX IF EXISTS idx_lease_contracts_approval;
DROP INDEX IF EXISTS idx_user_data_scopes_user;
DROP INDEX IF EXISTS idx_user_roles_role;
DROP INDEX IF EXISTS idx_user_roles_user;

ALTER TABLE ai_contract_drafts DROP COLUMN IF EXISTS returned_to_editor_at;
ALTER TABLE ai_contract_drafts DROP COLUMN IF EXISTS rejected_reason;
ALTER TABLE ai_contract_drafts DROP COLUMN IF EXISTS approved_at;
ALTER TABLE ai_contract_drafts DROP COLUMN IF EXISTS approved_by;
ALTER TABLE ai_contract_drafts DROP COLUMN IF EXISTS reviewed_at;
ALTER TABLE ai_contract_drafts DROP COLUMN IF EXISTS reviewed_by;
ALTER TABLE ai_contract_drafts DROP COLUMN IF EXISTS submitted_at;
ALTER TABLE ai_contract_drafts DROP COLUMN IF EXISTS submitted_by;
ALTER TABLE ai_contract_drafts DROP COLUMN IF EXISTS approval_status;

ALTER TABLE lease_events DROP COLUMN IF EXISTS rejected_reason;
ALTER TABLE lease_events DROP COLUMN IF EXISTS reviewed_at;
ALTER TABLE lease_events DROP COLUMN IF EXISTS reviewed_by;
ALTER TABLE lease_events DROP COLUMN IF EXISTS is_official_version;
ALTER TABLE lease_events DROP COLUMN IF EXISTS approval_status;

ALTER TABLE lease_payment_schedules DROP COLUMN IF EXISTS approved_by;
ALTER TABLE lease_payment_schedules DROP COLUMN IF EXISTS reviewed_by;
ALTER TABLE lease_payment_schedules DROP COLUMN IF EXISTS is_official_version;
ALTER TABLE lease_payment_schedules DROP COLUMN IF EXISTS approval_status;

ALTER TABLE lease_contracts DROP COLUMN IF EXISTS ai_suggested_rate_policies;
ALTER TABLE lease_contracts DROP COLUMN IF EXISTS ai_extracted_discount_rate;
ALTER TABLE lease_contracts DROP COLUMN IF EXISTS discount_rate_confirmed_at;
ALTER TABLE lease_contracts DROP COLUMN IF EXISTS discount_rate_confirmed_by;
ALTER TABLE lease_contracts DROP COLUMN IF EXISTS discount_rate_policy_id;
ALTER TABLE lease_contracts DROP COLUMN IF EXISTS discount_rate_source;
ALTER TABLE lease_contracts DROP COLUMN IF EXISTS discount_rate_missing;
ALTER TABLE lease_contracts DROP COLUMN IF EXISTS source_reference_locator;
ALTER TABLE lease_contracts DROP COLUMN IF EXISTS ai_confidence_score;
ALTER TABLE lease_contracts DROP COLUMN IF EXISTS submitted_at;
ALTER TABLE lease_contracts DROP COLUMN IF EXISTS rejected_reason;
ALTER TABLE lease_contracts DROP COLUMN IF EXISTS reviewed_at;
ALTER TABLE lease_contracts DROP COLUMN IF EXISTS reviewed_by;
ALTER TABLE lease_contracts DROP COLUMN IF EXISTS report_mode;
ALTER TABLE lease_contracts DROP COLUMN IF EXISTS included_in_reporting;
ALTER TABLE lease_contracts DROP COLUMN IF EXISTS draft_version_no;
ALTER TABLE lease_contracts DROP COLUMN IF EXISTS is_official_version;
ALTER TABLE lease_contracts DROP COLUMN IF EXISTS approval_status;

DROP TABLE IF EXISTS user_data_scopes;
DROP TABLE IF EXISTS user_roles;
DROP TYPE IF EXISTS approval_status;
-- +goose StatementEnd
