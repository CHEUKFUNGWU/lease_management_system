-- Migration 047: CAPEX dimensions on fpna_plan_lines and exchange_rate_versions
-- Aligns with PRD F3 (Cash Plan Composition & Currency Translation)

-- 1. Add CAPEX fields to fpna_plan_lines
ALTER TABLE fpna_plan_lines
    ADD COLUMN IF NOT EXISTS capex DECIMAL(18,2),
    ADD COLUMN IF NOT EXISTS capex_category VARCHAR(50);

-- 2. Create exchange_rate_versions table for controlled currency translation
CREATE TABLE IF NOT EXISTS exchange_rate_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    version_type VARCHAR(30) NOT NULL, -- closing, average, budget
    effective_from DATE NOT NULL,
    effective_to DATE,
    source VARCHAR(100) NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (version_type IN ('closing', 'average', 'budget')),
    CHECK (status IN ('draft', 'review', 'approved', 'official', 'retired')),
    UNIQUE (name)
);

CREATE INDEX IF NOT EXISTS idx_exchange_rate_versions_effective
    ON exchange_rate_versions(version_type, effective_from DESC, status);

-- 3. Link exchange_rates to versions (optional for backwards compatibility)
ALTER TABLE exchange_rates
    ADD COLUMN IF NOT EXISTS version_id UUID REFERENCES exchange_rate_versions(id);

CREATE INDEX IF NOT EXISTS idx_exchange_rates_version
    ON exchange_rates(version_id);
