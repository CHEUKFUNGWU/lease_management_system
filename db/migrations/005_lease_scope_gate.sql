-- IFRS 16 scope gate: distinguish capitalized leases, exemptions, and non-lease contracts.

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

CREATE INDEX IF NOT EXISTS idx_lease_contracts_scope ON lease_contracts(lease_scope);
