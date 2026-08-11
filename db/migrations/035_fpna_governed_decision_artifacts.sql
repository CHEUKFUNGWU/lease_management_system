-- +goose Up
-- +goose StatementBegin

-- The first operating-facts slice deliberately keeps source metadata on each
-- row.  These additions make retries and restatements explicit rather than
-- relying on an application caller to infer them.
ALTER TABLE operating_fact_batches
    ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(255),
    ADD COLUMN IF NOT EXISTS fact_version VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS retry_of_batch_id UUID REFERENCES operating_fact_batches(id);
CREATE UNIQUE INDEX IF NOT EXISTS ux_operating_fact_batches_idempotency
    ON operating_fact_batches(legal_entity_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

ALTER TABLE fpna_action_items
    ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(255);
CREATE UNIQUE INDEX IF NOT EXISTS ux_fpna_action_items_idempotency
    ON fpna_action_items(legal_entity_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

ALTER TABLE store_operating_facts
    ADD COLUMN IF NOT EXISTS business_segment VARCHAR(100),
    ADD COLUMN IF NOT EXISTS fiscal_year VARCHAR(20),
    ADD COLUMN IF NOT EXISTS store_age_months INTEGER,
    ADD COLUMN IF NOT EXISTS cohort_code VARCHAR(100),
    ADD COLUMN IF NOT EXISTS data_quality_status VARCHAR(30) NOT NULL DEFAULT 'unassessed';

ALTER TABLE equipment_operating_facts
    ADD COLUMN IF NOT EXISTS purchase_price DECIMAL(18,2),
    ADD COLUMN IF NOT EXISTS purchase_price_variance DECIMAL(18,2),
    ADD COLUMN IF NOT EXISTS capacity_available DECIMAL(18,4),
    ADD COLUMN IF NOT EXISTS lease_cost DECIMAL(18,2),
    ADD COLUMN IF NOT EXISTS contractual_rent DECIMAL(18,2),
    ADD COLUMN IF NOT EXISTS data_quality_status VARCHAR(30) NOT NULL DEFAULT 'unassessed';

-- A common plan/forecast/scenario contract.  Existing budget_versions remains
-- backwards compatible for lease-only plans; these tables add the operating
-- grain and lifecycle needed by the FP&A platform.
CREATE TABLE IF NOT EXISTS fpna_plan_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID REFERENCES legal_entities(id),
    name VARCHAR(255) NOT NULL,
    version_type VARCHAR(20) NOT NULL,
    scenario_type VARCHAR(30) NOT NULL DEFAULT 'baseline',
    source VARCHAR(255) NOT NULL,
    coverage_scope JSONB NOT NULL DEFAULT '{}'::jsonb,
    currency VARCHAR(10),
    as_of_period VARCHAR(7) NOT NULL,
    from_period VARCHAR(7) NOT NULL,
    to_period VARCHAR(7) NOT NULL,
    actual_cutoff_period VARCHAR(7),
    prior_version_id UUID REFERENCES fpna_plan_versions(id),
    assumption_version VARCHAR(100),
    exchange_rate_version VARCHAR(100),
    metric_definition_version VARCHAR(100),
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    is_official BOOLEAN NOT NULL DEFAULT false,
    frozen_at TIMESTAMP WITH TIME ZONE,
    approved_at TIMESTAMP WITH TIME ZONE,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (version_type IN ('actual','prior_year','budget','forecast','scenario')),
    CHECK (scenario_type IN ('baseline','upside','downside','custom')),
    CHECK (status IN ('draft','review','approved','official','retired')),
    CHECK (from_period <= to_period),
    UNIQUE (legal_entity_id, name, as_of_period)
);
CREATE INDEX IF NOT EXISTS idx_fpna_plan_versions_lookup
    ON fpna_plan_versions(legal_entity_id, version_type, as_of_period DESC, created_at DESC);

CREATE TABLE IF NOT EXISTS fpna_plan_lines (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    plan_version_id UUID NOT NULL REFERENCES fpna_plan_versions(id) ON DELETE CASCADE,
    period VARCHAR(7) NOT NULL,
    grain VARCHAR(30) NOT NULL DEFAULT 'group',
    legal_entity_id UUID REFERENCES legal_entities(id),
    business_segment VARCHAR(100),
    brand VARCHAR(100),
    region VARCHAR(100),
    store_id UUID REFERENCES stores(id),
    plant_code VARCHAR(100),
    production_line_code VARCHAR(100),
    equipment_id UUID REFERENCES equipment_assets(id),
    asset_type VARCHAR(100),
    currency VARCHAR(10) NOT NULL,
    revenue DECIMAL(18,2),
    gross_profit DECIMAL(18,2),
    labor_cost DECIMAL(18,2),
    fixed_rent DECIMAL(18,2),
    variable_rent DECIMAL(18,2),
    non_lease_cost DECIMAL(18,2),
    four_wall_ebitda DECIMAL(18,2),
    cash_flow DECIMAL(18,2),
    net_debt DECIMAL(18,2),
    operational_kpis JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_system VARCHAR(100) NOT NULL,
    source_record_id VARCHAR(150),
    as_of_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    actual_flag BOOLEAN NOT NULL DEFAULT false,
    forecast_flag BOOLEAN NOT NULL DEFAULT false,
    scenario_inputs JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (period ~ '^[0-9]{4}-[0-9]{2}$'),
    UNIQUE (plan_version_id, period, grain, legal_entity_id, business_segment, brand, region, store_id, plant_code, production_line_code, equipment_id, asset_type, currency)
);
CREATE INDEX IF NOT EXISTS idx_fpna_plan_lines_period
    ON fpna_plan_lines(plan_version_id, period, grain);

-- Formula ownership is centralized here.  UI, exports and Agent tools may
-- consume the same definition/version and must not silently reimplement it.
CREATE TABLE IF NOT EXISTS fpna_metric_definitions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    metric_key VARCHAR(100) NOT NULL,
    version VARCHAR(100) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    formula TEXT NOT NULL,
    grain VARCHAR(100) NOT NULL,
    currency_policy VARCHAR(100) NOT NULL,
    fiscal_period_rule VARCHAR(255) NOT NULL,
    exclusions JSONB NOT NULL DEFAULT '{}'::jsonb,
    owner_name VARCHAR(255) NOT NULL,
    effective_from DATE NOT NULL,
    effective_to DATE,
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (status IN ('draft','approved','retired')),
    UNIQUE (metric_key, version)
);

-- External aliases/mappings are effective-dated and never inferred into an
-- official result when they are ambiguous.
CREATE TABLE IF NOT EXISTS fpna_master_data_mappings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID REFERENCES legal_entities(id),
    mapping_type VARCHAR(30) NOT NULL,
    external_system VARCHAR(100) NOT NULL,
    external_id VARCHAR(255) NOT NULL,
    external_name VARCHAR(255),
    alias VARCHAR(255),
    target_id UUID,
    target_code VARCHAR(150),
    effective_from DATE NOT NULL,
    effective_to DATE,
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (mapping_type IN ('legal_entity','cost_center','store','plant','line','equipment','contract')),
    CHECK (status IN ('draft','approved','retired')),
    UNIQUE (legal_entity_id, mapping_type, external_system, external_id, effective_from)
);
CREATE INDEX IF NOT EXISTS idx_fpna_master_data_mappings_lookup
    ON fpna_master_data_mappings(legal_entity_id, mapping_type, external_system, effective_from DESC);

CREATE TABLE IF NOT EXISTS fpna_data_quality_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID REFERENCES legal_entities(id),
    batch_id UUID REFERENCES operating_fact_batches(id),
    period VARCHAR(7),
    dimension VARCHAR(50) NOT NULL,
    category VARCHAR(50) NOT NULL,
    severity VARCHAR(20) NOT NULL DEFAULT 'medium',
    source_table VARCHAR(100) NOT NULL,
    source_record_id VARCHAR(150) NOT NULL,
    data_version VARCHAR(100) NOT NULL DEFAULT '',
    description TEXT NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'open',
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMP WITH TIME ZONE,
    CHECK (category IN ('unmapped','ambiguous_mapping','missing','low_confidence','reconciliation','duplicate','invalid')),
    CHECK (status IN ('open','acknowledged','resolved','accepted'))
);
CREATE INDEX IF NOT EXISTS idx_fpna_data_quality_queue
    ON fpna_data_quality_items(legal_entity_id, status, severity, period);

CREATE TABLE IF NOT EXISTS fpna_action_realizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    action_id UUID NOT NULL REFERENCES fpna_action_items(id) ON DELETE CASCADE,
    period VARCHAR(7) NOT NULL,
    baseline_amount DECIMAL(18,2),
    target_amount DECIMAL(18,2),
    actual_amount DECIMAL(18,2),
    realized_benefit DECIMAL(18,2),
    currency VARCHAR(10),
    source_table VARCHAR(100) NOT NULL,
    source_record_id VARCHAR(150) NOT NULL,
    data_version VARCHAR(100) NOT NULL DEFAULT '',
    status VARCHAR(30) NOT NULL DEFAULT 'pending',
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    verified_by UUID REFERENCES users(id),
    verified_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (status IN ('pending','verified','failed')),
    UNIQUE (action_id, period)
);

-- A memo is a structured, reviewable artifact.  Facts/calculations/human
-- inputs/AI narrative are separate JSON documents so the latter cannot be
-- mistaken for a system conclusion.
CREATE TABLE IF NOT EXISTS fpna_decision_memos (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID REFERENCES legal_entities(id),
    memo_type VARCHAR(50) NOT NULL,
    title VARCHAR(255) NOT NULL,
    basis VARCHAR(20) NOT NULL DEFAULT 'Scenario',
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    scenario_draft_id UUID REFERENCES fpna_scenario_drafts(id),
    system_facts JSONB NOT NULL DEFAULT '{}'::jsonb,
    deterministic_calculations JSONB NOT NULL DEFAULT '{}'::jsonb,
    human_inputs JSONB NOT NULL DEFAULT '{}'::jsonb,
    ai_narrative JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_references JSONB NOT NULL DEFAULT '[]'::jsonb,
    data_version VARCHAR(100) NOT NULL DEFAULT '',
    assumption_version VARCHAR(100) NOT NULL DEFAULT '',
    metric_definition_version VARCHAR(100) NOT NULL DEFAULT '',
    idempotency_key VARCHAR(255),
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    reviewed_by UUID REFERENCES users(id),
    reviewed_at TIMESTAMP WITH TIME ZONE,
    CHECK (basis IN ('Working','Official','Scenario')),
    CHECK (status IN ('draft','review','approved','rejected'))
);
CREATE INDEX IF NOT EXISTS idx_fpna_decision_memos_lookup
    ON fpna_decision_memos(legal_entity_id, memo_type, status, created_at DESC);
ALTER TABLE fpna_decision_memos
    ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(255);
CREATE UNIQUE INDEX IF NOT EXISTS ux_fpna_decision_memos_idempotency
    ON fpna_decision_memos(legal_entity_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE TABLE IF NOT EXISTS fpna_report_artifacts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID REFERENCES legal_entities(id),
    report_type VARCHAR(20) NOT NULL,
    view_type VARCHAR(30) NOT NULL DEFAULT 'group',
    period VARCHAR(7) NOT NULL,
    basis VARCHAR(20) NOT NULL DEFAULT 'Working',
    format VARCHAR(20) NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    manifest_sha256 VARCHAR(128),
    data_version VARCHAR(100) NOT NULL DEFAULT '',
    assumption_version VARCHAR(100) NOT NULL DEFAULT '',
    metric_definition_version VARCHAR(100) NOT NULL DEFAULT '',
    generated_by UUID REFERENCES users(id),
    generated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (report_type IN ('WBR','MBR','QBR')),
    CHECK (basis IN ('Working','Official','Scenario')),
    CHECK (format IN ('json','html','csv','xlsx','pdf','pptx')),
    CHECK (status IN ('draft','review','published','failed'))
);
CREATE INDEX IF NOT EXISTS idx_fpna_report_artifacts_lookup
    ON fpna_report_artifacts(legal_entity_id, report_type, period, basis, generated_at DESC);

-- Agent Signals are not Detection Events or formal Control Conclusions.  The
-- separate table is an intentional control boundary.
CREATE TABLE IF NOT EXISTS fpna_agent_signals (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID REFERENCES legal_entities(id),
    period VARCHAR(7),
    rule_code VARCHAR(100) NOT NULL,
    severity VARCHAR(20) NOT NULL DEFAULT 'medium',
    source_table VARCHAR(100) NOT NULL,
    source_record_id VARCHAR(150) NOT NULL,
    data_version VARCHAR(100) NOT NULL DEFAULT '',
    signal JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(30) NOT NULL DEFAULT 'open',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (status IN ('open','acknowledged','dismissed'))
);

INSERT INTO permissions (role_id, resource, action) VALUES
    ('22222222-2222-2222-2222-222222222222','fpna_memos','write'),
    ('33333333-3333-3333-3333-333333333333','fpna_memos','write'),
    ('44444444-4444-4444-4444-444444444444','fpna_memos','write'),
    ('55555555-5555-5555-5555-555555555555','fpna_memos','read'),
    ('22222222-2222-2222-2222-222222222222','fpna_reports','write'),
    ('33333333-3333-3333-3333-333333333333','fpna_reports','write'),
    ('44444444-4444-4444-4444-444444444444','fpna_reports','write'),
    ('55555555-5555-5555-5555-555555555555','fpna_reports','read'),
    ('22222222-2222-2222-2222-222222222222','fpna_mappings','write'),
    ('33333333-3333-3333-3333-333333333333','fpna_mappings','write'),
    ('44444444-4444-4444-4444-444444444444','fpna_mappings','write'),
    ('55555555-5555-5555-5555-555555555555','fpna_data_quality','read'),
    ('22222222-2222-2222-2222-222222222222','fpna_data_quality','write'),
    ('33333333-3333-3333-3333-333333333333','fpna_data_quality','write'),
    ('44444444-4444-4444-4444-444444444444','fpna_data_quality','write')
ON CONFLICT (role_id, resource, action) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS fpna_agent_signals;
DROP TABLE IF EXISTS fpna_report_artifacts;
DROP INDEX IF EXISTS ux_fpna_decision_memos_idempotency;
DROP TABLE IF EXISTS fpna_decision_memos;
DROP TABLE IF EXISTS fpna_action_realizations;
DROP TABLE IF EXISTS fpna_data_quality_items;
DROP TABLE IF EXISTS fpna_master_data_mappings;
DROP TABLE IF EXISTS fpna_metric_definitions;
DROP TABLE IF EXISTS fpna_plan_lines;
DROP TABLE IF EXISTS fpna_plan_versions;
DROP INDEX IF EXISTS ux_operating_fact_batches_idempotency;
DROP INDEX IF EXISTS ux_fpna_action_items_idempotency;
ALTER TABLE fpna_action_items DROP COLUMN IF EXISTS idempotency_key;
ALTER TABLE equipment_operating_facts
    DROP COLUMN IF EXISTS purchase_price,
    DROP COLUMN IF EXISTS purchase_price_variance,
    DROP COLUMN IF EXISTS capacity_available,
    DROP COLUMN IF EXISTS lease_cost,
    DROP COLUMN IF EXISTS contractual_rent,
    DROP COLUMN IF EXISTS data_quality_status;
ALTER TABLE store_operating_facts
    DROP COLUMN IF EXISTS business_segment,
    DROP COLUMN IF EXISTS fiscal_year,
    DROP COLUMN IF EXISTS store_age_months,
    DROP COLUMN IF EXISTS cohort_code,
    DROP COLUMN IF EXISTS data_quality_status;
ALTER TABLE operating_fact_batches
    DROP COLUMN IF EXISTS idempotency_key,
    DROP COLUMN IF EXISTS fact_version,
    DROP COLUMN IF EXISTS retry_of_batch_id;
-- +goose StatementEnd
