-- +goose Up
-- +goose StatementBegin

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

INSERT INTO permissions (role_id, resource, action) VALUES
    ('22222222-2222-2222-2222-222222222222', 'variance_actions', 'write'),
    ('22222222-2222-2222-2222-222222222222', 'renewal_decisions', 'write'),
    ('33333333-3333-3333-3333-333333333333', 'variance_actions', 'write'),
    ('33333333-3333-3333-3333-333333333333', 'renewal_decisions', 'write'),
    ('44444444-4444-4444-4444-444444444444', 'variance_actions', 'write'),
    ('44444444-4444-4444-4444-444444444444', 'renewal_decisions', 'write')
ON CONFLICT (role_id, resource, action) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_renewal_decisions_contract;
DROP TABLE IF EXISTS renewal_decision_snapshots;
DROP INDEX IF EXISTS idx_variance_actions_period;
DROP TABLE IF EXISTS variance_actions;
ALTER TABLE budget_versions DROP CONSTRAINT IF EXISTS budget_versions_version_type_check;
ALTER TABLE budget_versions
    DROP COLUMN IF EXISTS is_official,
    DROP COLUMN IF EXISTS coverage_scope,
    DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS version_type;
-- +goose StatementEnd
