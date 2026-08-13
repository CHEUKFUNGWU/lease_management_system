-- Migration 038: store-day operating facts are kept separate from the existing
-- monthly table.  The table and index are idempotent so this repair can be
-- applied to an existing volume as well as included in a fresh init.
-- This lets the retail workstation add a daily grain without changing any
-- lease accounting or monthly FP&A consumers.
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
