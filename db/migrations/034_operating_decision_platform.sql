-- +goose Up
-- +goose StatementBegin

-- Versioned operating facts sit beside (and never overwrite) the lease
-- sub-ledger.  They are deliberately a governed fact slice, not a second
-- POS/MES/ERP: the source system, batch and reconciliation state travel with
-- every row so a management view can say whether it is decision-ready.
CREATE TABLE IF NOT EXISTS operating_fact_batches (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID REFERENCES legal_entities(id),
    source_system VARCHAR(100) NOT NULL,
    source_file VARCHAR(255),
    as_of_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    status VARCHAR(30) NOT NULL DEFAULT 'received',
    total_rows INTEGER NOT NULL DEFAULT 0,
    accepted_rows INTEGER NOT NULL DEFAULT 0,
    rejected_rows INTEGER NOT NULL DEFAULT 0,
    reconciliation_status VARCHAR(30) NOT NULL DEFAULT 'unreconciled',
    error_summary JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (status IN ('received', 'processing', 'completed', 'failed')),
    CHECK (reconciliation_status IN ('unreconciled', 'matched', 'warning', 'failed'))
);

CREATE TABLE IF NOT EXISTS store_operating_facts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    store_id UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    period VARCHAR(7) NOT NULL,
    period_basis VARCHAR(20) NOT NULL,
    currency VARCHAR(10) NOT NULL,
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
    note TEXT,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (period ~ '^[0-9]{4}-[0-9]{2}$'),
    CHECK (period_basis IN ('calendar_month', 'fiscal_month', 'quarter', 'year')),
    CHECK (reconciliation_status IN ('unreconciled', 'matched', 'warning', 'failed')),
    CHECK (mapping_status IN ('mapped', 'unmapped', 'ambiguous')),
    CHECK (revenue >= 0),
    UNIQUE (store_id, period, version, source_system)
);
CREATE INDEX IF NOT EXISTS idx_store_operating_facts_period
    ON store_operating_facts(store_id, period, version DESC);

-- A small equipment master is enough to connect a lease to a plant/line while
-- leaving the authoritative fixed-asset/MES master in the source system.
CREATE TABLE IF NOT EXISTS equipment_assets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID REFERENCES legal_entities(id),
    plant_code VARCHAR(100) NOT NULL,
    production_line_code VARCHAR(100),
    equipment_code VARCHAR(150) NOT NULL,
    equipment_name VARCHAR(255) NOT NULL,
    cost_center VARCHAR(100),
    asset_identifier VARCHAR(150),
    contract_id UUID REFERENCES lease_contracts(id),
    asset_type VARCHAR(100),
    capacity DECIMAL(18,4),
    capacity_unit VARCHAR(40),
    currency VARCHAR(10),
    external_system VARCHAR(100),
    external_id VARCHAR(150),
    effective_from DATE,
    effective_to DATE,
    active BOOLEAN NOT NULL DEFAULT true,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (legal_entity_id, equipment_code)
);
CREATE INDEX IF NOT EXISTS idx_equipment_assets_plant_line
    ON equipment_assets(legal_entity_id, plant_code, production_line_code);

CREATE TABLE IF NOT EXISTS equipment_operating_facts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    equipment_id UUID NOT NULL REFERENCES equipment_assets(id) ON DELETE CASCADE,
    period VARCHAR(7) NOT NULL,
    currency VARCHAR(10) NOT NULL,
    output_qty DECIMAL(18,4),
    yield_pct DECIMAL(9,4),
    scrap_qty DECIMAL(18,4),
    downtime_hours DECIMAL(18,4),
    oee_pct DECIMAL(9,4),
    utilization_pct DECIMAL(9,4),
    labor_cost DECIMAL(18,2),
    energy_cost DECIMAL(18,2),
    maintenance_cost DECIMAL(18,2),
    standard_cost DECIMAL(18,2),
    actual_cost DECIMAL(18,2),
    material_usage_cost DECIMAL(18,2),
    overhead_absorption DECIMAL(18,2),
    source_system VARCHAR(100) NOT NULL,
    source_record_id VARCHAR(150),
    import_batch_id UUID REFERENCES operating_fact_batches(id),
    as_of_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    version INTEGER NOT NULL DEFAULT 1,
    reconciliation_status VARCHAR(30) NOT NULL DEFAULT 'unreconciled',
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (period ~ '^[0-9]{4}-[0-9]{2}$'),
    CHECK (reconciliation_status IN ('unreconciled', 'matched', 'warning', 'failed')),
    UNIQUE (equipment_id, period, version, source_system)
);
CREATE INDEX IF NOT EXISTS idx_equipment_operating_facts_period
    ON equipment_operating_facts(equipment_id, period, version DESC);

-- Unified action/exception lifecycle.  Detection evidence and human action
-- fields are separate: marking an item complete never asserts that a benefit
-- was realised until a later actual period is verified.
CREATE TABLE IF NOT EXISTS fpna_action_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID REFERENCES legal_entities(id),
    period VARCHAR(7),
    category VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL DEFAULT 'medium',
    status VARCHAR(30) NOT NULL DEFAULT 'open',
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    rule_code VARCHAR(100) NOT NULL,
    source_table VARCHAR(100) NOT NULL,
    source_record_id VARCHAR(150) NOT NULL,
    data_version VARCHAR(100) NOT NULL DEFAULT '',
    impact_amount DECIMAL(18,2),
    currency VARCHAR(10),
    owner_id UUID REFERENCES users(id),
    owner_name VARCHAR(255),
    due_date DATE,
    baseline_amount DECIMAL(18,2),
    target_amount DECIMAL(18,2),
    expected_benefit DECIMAL(18,2),
    verification_period VARCHAR(7),
    verified_amount DECIMAL(18,2),
    verification_status VARCHAR(30) NOT NULL DEFAULT 'not_due',
    human_root_cause TEXT,
    planned_action TEXT,
    ai_suggestion TEXT,
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by UUID REFERENCES users(id),
    updated_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    acknowledged_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    verified_at TIMESTAMP WITH TIME ZONE,
    CHECK (severity IN ('critical', 'high', 'medium', 'low', 'informational')),
    CHECK (status IN ('open', 'acknowledged', 'in_progress', 'completed', 'verified', 'accepted', 'dismissed')),
    CHECK (verification_status IN ('not_due', 'pending', 'verified', 'failed', 'not_applicable')),
    UNIQUE (legal_entity_id, rule_code, source_table, source_record_id, period)
);
CREATE INDEX IF NOT EXISTS idx_fpna_action_items_queue
    ON fpna_action_items(legal_entity_id, status, severity, due_date);
ALTER TABLE fpna_action_items
    DROP CONSTRAINT IF EXISTS fpna_action_items_rule_code_source_table_source_record_id_period_key;
CREATE UNIQUE INDEX IF NOT EXISTS ux_fpna_action_items_scope_dedupe
    ON fpna_action_items(legal_entity_id, rule_code, source_table, source_record_id, period);

CREATE TABLE IF NOT EXISTS fpna_assumption_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID REFERENCES legal_entities(id),
    assumption_key VARCHAR(150) NOT NULL,
    category VARCHAR(50) NOT NULL,
    value JSONB NOT NULL,
    unit VARCHAR(50),
    source VARCHAR(255) NOT NULL,
    owner_name VARCHAR(255),
    effective_from DATE NOT NULL,
    effective_to DATE,
    version INTEGER NOT NULL DEFAULT 1,
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (status IN ('draft', 'approved', 'retired')),
    UNIQUE (legal_entity_id, assumption_key, version)
);
CREATE INDEX IF NOT EXISTS idx_fpna_assumption_versions_lookup ON fpna_assumption_versions(legal_entity_id, assumption_key, effective_from DESC, version DESC);

-- Scenario calculations are immutable working drafts. They are never Budget,
-- Forecast, contract, event or journal state; approval is a separate human
-- workflow if a customer chooses to promote one later.
CREATE TABLE IF NOT EXISTS fpna_scenario_drafts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID REFERENCES legal_entities(id),
    scenario_type VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    assumptions JSONB NOT NULL DEFAULT '{}'::jsonb,
    result JSONB,
    data_version VARCHAR(100) NOT NULL DEFAULT '',
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    source_run_id VARCHAR(150),
    idempotency_key VARCHAR(255),
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (status IN ('draft', 'reviewed', 'approved', 'rejected'))
);
CREATE UNIQUE INDEX IF NOT EXISTS ux_fpna_scenario_drafts_idempotency
    ON fpna_scenario_drafts(legal_entity_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_fpna_scenario_drafts_lookup
    ON fpna_scenario_drafts(legal_entity_id, scenario_type, created_at DESC);

INSERT INTO permissions (role_id, resource, action) VALUES
    ('22222222-2222-2222-2222-222222222222', 'fpna_actions', 'write'),
    ('33333333-3333-3333-3333-333333333333', 'fpna_actions', 'write'),
    ('44444444-4444-4444-4444-444444444444', 'fpna_actions', 'write'),
    ('55555555-5555-5555-5555-555555555555', 'fpna_actions', 'read')
ON CONFLICT (role_id, resource, action) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS fpna_action_items;
DROP TABLE IF EXISTS fpna_assumption_versions;
DROP TABLE IF EXISTS fpna_scenario_drafts;
DROP TABLE IF EXISTS equipment_operating_facts;
DROP TABLE IF EXISTS equipment_assets;
DROP TABLE IF EXISTS store_operating_facts;
DROP TABLE IF EXISTS operating_fact_batches;
-- +goose StatementEnd
