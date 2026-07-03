-- +goose Up
-- +goose StatementBegin

ALTER TABLE roles ADD COLUMN IF NOT EXISTS code VARCHAR(50);

UPDATE roles SET code = CASE id
    WHEN '11111111-1111-1111-1111-111111111111' THEN 'admin'
    WHEN '22222222-2222-2222-2222-222222222222' THEN 'editor'
    WHEN '33333333-3333-3333-3333-333333333333' THEN 'reviewer'
    WHEN '44444444-4444-4444-4444-444444444444' THEN 'approver'
    WHEN '55555555-5555-5555-5555-555555555555' THEN 'auditor'
    WHEN '66666666-6666-6666-6666-666666666666' THEN 'readonly'
    ELSE 'custom-' || replace(id::text, '-', '')
END
WHERE code IS NULL;

ALTER TABLE roles ALTER COLUMN code SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_roles_code ON roles(code);

ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS legal_entity_id UUID REFERENCES legal_entities(id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_legal_entity ON audit_logs(legal_entity_id);

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
    ('33333333-3333-3333-3333-333333333333', 'monthly_closing', 'approve'),
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
    ('44444444-4444-4444-4444-444444444444', 'ai_chat', 'use'),
    ('44444444-4444-4444-4444-444444444444', 'master_data', 'read'),
    ('44444444-4444-4444-4444-444444444444', 'settings', 'read'),
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

INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id
FROM users u
JOIN roles r ON r.code = CASE WHEN u.role = 'user' THEN 'readonly' ELSE u.role END
ON CONFLICT (user_id, role_id) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_roles_code;
DROP INDEX IF EXISTS idx_audit_logs_legal_entity;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS legal_entity_id;
ALTER TABLE roles DROP COLUMN IF EXISTS code;
-- Permission and role-assignment rows are retained to avoid destructive rollback.
-- +goose StatementEnd
