-- 053_statement_models.sql — 三表财务模型与单店利润表（PRD §6.1 / SM 模块）
-- five core objects: statement templates (versioned, frozen), model
-- definitions, runs (input snapshot + five version lines), run lines
-- (row × period × value + provenance) and tie-out results. All objects
-- carry legal_entity_id (bottom line 1), idempotency keys (bottom line 4)
-- and provenance (bottom line 3).

CREATE TABLE IF NOT EXISTS fin_statement_templates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID REFERENCES legal_entities(id),
    name VARCHAR(200) NOT NULL,
    version INTEGER NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    rows JSONB NOT NULL,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    approved_by UUID REFERENCES users(id),
    approved_at TIMESTAMP WITH TIME ZONE,
    CHECK (status IN ('draft','review','approved','retired')),
    CHECK (version > 0),
    UNIQUE (legal_entity_id, name, version)
);
CREATE INDEX IF NOT EXISTS idx_fin_statement_templates_lookup
    ON fin_statement_templates(legal_entity_id, name, status);

CREATE TABLE IF NOT EXISTS fin_model_definitions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID NOT NULL REFERENCES legal_entities(id),
    name VARCHAR(200) NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    template_id UUID NOT NULL REFERENCES fin_statement_templates(id),
    actual_cutoff_period VARCHAR(7),
    policy JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_bindings JSONB NOT NULL DEFAULT '{}'::jsonb,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (status IN ('draft','review','approved','retired')),
    CHECK (actual_cutoff_period IS NULL OR actual_cutoff_period ~ '^[0-9]{4}-[0-9]{2}$'),
    CHECK (version > 0),
    UNIQUE (legal_entity_id, name, version)
);
CREATE INDEX IF NOT EXISTS idx_fin_model_definitions_lookup
    ON fin_model_definitions(legal_entity_id, status);

CREATE TABLE IF NOT EXISTS fin_model_runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID NOT NULL REFERENCES legal_entities(id),
    model_definition_id UUID NOT NULL REFERENCES fin_model_definitions(id),
    model_definition_version INTEGER NOT NULL,
    data_version VARCHAR(100),
    assumption_version VARCHAR(100),
    exchange_rate_version VARCHAR(100),
    metric_definition_version VARCHAR(100),
    data_classification VARCHAR(20) NOT NULL DEFAULT 'production',
    status VARCHAR(20) NOT NULL DEFAULT 'queued',
    tie_out_status VARCHAR(20) NOT NULL DEFAULT 'pending',
    input_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb,
    idempotency_key VARCHAR(200) NOT NULL,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    CHECK (data_classification IN ('production','simulated','mixed')),
    CHECK (status IN ('queued','running','completed','failed','cancelled')),
    CHECK (tie_out_status IN ('pending','passed','failed','degraded')),
    UNIQUE (model_definition_id, idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_fin_model_runs_lookup
    ON fin_model_runs(legal_entity_id, model_definition_id, created_at DESC);

CREATE TABLE IF NOT EXISTS fin_model_run_lines (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id UUID NOT NULL REFERENCES fin_model_runs(id) ON DELETE CASCADE,
    row_key VARCHAR(100) NOT NULL,
    period VARCHAR(7) NOT NULL,
    value DOUBLE PRECISION,
    currency VARCHAR(10),
    provenance JSONB NOT NULL,
    CHECK (period ~ '^[0-9]{4}-[0-9]{2}$'),
    UNIQUE (run_id, row_key, period)
);
CREATE INDEX IF NOT EXISTS idx_fin_model_run_lines_period
    ON fin_model_run_lines(run_id, period);

CREATE TABLE IF NOT EXISTS fin_model_tie_outs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    run_id UUID NOT NULL REFERENCES fin_model_runs(id) ON DELETE CASCADE,
    check_code VARCHAR(20) NOT NULL,
    period VARCHAR(7) NOT NULL,
    expected DOUBLE PRECISION,
    actual DOUBLE PRECISION,
    diff DOUBLE PRECISION,
    status VARCHAR(20) NOT NULL DEFAULT 'failed',
    CHECK (status IN ('passed','failed','not_applicable')),
    UNIQUE (run_id, check_code, period)
);
CREATE INDEX IF NOT EXISTS idx_fin_model_tie_outs_lookup
    ON fin_model_tie_outs(run_id, status);
