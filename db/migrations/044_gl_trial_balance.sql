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
