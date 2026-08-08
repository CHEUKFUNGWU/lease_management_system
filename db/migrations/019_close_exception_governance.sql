-- Close Exception Governance

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
