-- 055_saved_views_and_template_governance.sql — PRD S3-4 / S3-5
-- Template governance: the three-state flow (draft → review → approved)
-- gains a reviewer stamp so the second state is a real person's action, not
-- an unrecorded hop. Saved views hold presentation config only (period
-- range, version lines, basis mode, row show/hide, grain and region/brand/
-- store filters + personal-default / entity-shared flags). A view NEVER
-- carries data — the whole point of S3-5 — so the render path keeps its
-- rows and its permissions from the caller's scope (bottom lines 1 + 3).

ALTER TABLE fin_statement_templates ADD COLUMN IF NOT EXISTS reviewed_by UUID REFERENCES users(id);
ALTER TABLE fin_statement_templates ADD COLUMN IF NOT EXISTS reviewed_at TIMESTAMP WITH TIME ZONE;

CREATE TABLE IF NOT EXISTS fin_saved_views (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID NOT NULL REFERENCES legal_entities(id),
    kind VARCHAR(30) NOT NULL,
    name VARCHAR(200) NOT NULL,
    config JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_shared BOOLEAN NOT NULL DEFAULT false,
    is_default BOOLEAN NOT NULL DEFAULT false,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (kind IN ('store_pnl','financial_model','group_view')),
    UNIQUE (legal_entity_id, kind, name, created_by)
);

-- At most one personal default per (user, surface) — enforced by the
-- database, not by handler politeness.
CREATE UNIQUE INDEX IF NOT EXISTS ux_fin_saved_views_default
    ON fin_saved_views(created_by, kind) WHERE is_default;

CREATE INDEX IF NOT EXISTS idx_fin_saved_views_lookup
    ON fin_saved_views(legal_entity_id, kind, created_at DESC);

-- Resource grants per PRD §6.2: Editor builds/modifies, Reviewer reviews,
-- Approver approves and publishes, Auditor/Readonly read. fin_views is the
-- saved-view resource; personal view maintenance is write, entity sharing
-- is its own action so a bug elsewhere cannot widen visibility.
INSERT INTO permissions (role_id, resource, action) VALUES
    ('22222222-2222-2222-2222-222222222222', 'statement_templates', 'read'),
    ('22222222-2222-2222-2222-222222222222', 'statement_templates', 'write'),
    ('33333333-3333-3333-3333-333333333333', 'statement_templates', 'read'),
    ('33333333-3333-3333-3333-333333333333', 'statement_templates', 'review'),
    ('44444444-4444-4444-4444-444444444444', 'statement_templates', 'read'),
    ('44444444-4444-4444-4444-444444444444', 'statement_templates', 'approve'),
    ('55555555-5555-5555-5555-555555555555', 'statement_templates', 'read'),
    ('66666666-6666-6666-6666-666666666666', 'statement_templates', 'read'),
    ('22222222-2222-2222-2222-222222222222', 'fin_models', 'read'),
    ('22222222-2222-2222-2222-222222222222', 'fin_models', 'write'),
    ('33333333-3333-3333-3333-333333333333', 'fin_models', 'read'),
    ('33333333-3333-3333-3333-333333333333', 'fin_models', 'review'),
    ('44444444-4444-4444-4444-444444444444', 'fin_models', 'read'),
    ('44444444-4444-4444-4444-444444444444', 'fin_models', 'approve'),
    ('55555555-5555-5555-5555-555555555555', 'fin_models', 'read'),
    ('66666666-6666-6666-6666-666666666666', 'fin_models', 'read'),
    ('22222222-2222-2222-2222-222222222222', 'fin_views', 'read'),
    ('22222222-2222-2222-2222-222222222222', 'fin_views', 'write'),
    ('22222222-2222-2222-2222-222222222222', 'fin_views', 'share'),
    ('33333333-3333-3333-3333-333333333333', 'fin_views', 'read'),
    ('33333333-3333-3333-3333-333333333333', 'fin_views', 'write'),
    ('33333333-3333-3333-3333-333333333333', 'fin_views', 'share'),
    ('44444444-4444-4444-4444-444444444444', 'fin_views', 'read'),
    ('44444444-4444-4444-4444-444444444444', 'fin_views', 'write'),
    ('44444444-4444-4444-4444-444444444444', 'fin_views', 'share'),
    ('55555555-5555-5555-5555-555555555555', 'fin_views', 'read'),
    ('66666666-6666-6666-6666-666666666666', 'fin_views', 'read')
ON CONFLICT (role_id, resource, action) DO NOTHING;
