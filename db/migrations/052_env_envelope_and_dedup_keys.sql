-- 052_env_envelope_and_dedup_keys.sql
-- Dashboards: source-envelope + dedup keys for the retail fact tables, and the
-- drift repair for tables that existed only in 01_init.sql.
--
-- AGENTS.md: store-day facts must carry a provenance envelope and repeated
-- imports must be idempotent (bottom lines 2/3/4). Three tables collided with
-- those rules:
--   * retail_store_day_inventory_facts   — no envelope; unique key contained
--     nullable category_code / sku_code, so NULL never conflicted and replays
--     inserted duplicates; the Go upsert referenced a COALESCE'd conflict
--     target no index matched, so it failed at runtime.
--   * retail_competitor_observations     — no envelope, no unique key.
--   * promotion_costs                    — no dedup key; a re-import doubled
--     costs and silently halved ROI.
-- 048's retail_store_day_category_facts unique key contains the nullable
-- simulation_dataset_version — NULL for every production row — so production
-- category facts had no duplicate-import protection either. That constraint is
-- replaced with the same COALESCE-based expression index.

-- ── 1. Drift repair: tables that only ever existed in 01_init.sql ──────────
CREATE TABLE IF NOT EXISTS period_locks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    accounting_period VARCHAR(7) NOT NULL,
    legal_entity_id UUID REFERENCES legal_entities(id),
    is_locked BOOLEAN NOT NULL DEFAULT false,
    locked_by UUID REFERENCES users(id),
    locked_at TIMESTAMP WITH TIME ZONE,
    unlocked_by UUID REFERENCES users(id),
    unlocked_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(accounting_period, legal_entity_id)
);

CREATE TABLE IF NOT EXISTS event_adjustments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_id UUID NOT NULL UNIQUE REFERENCES lease_events(id) ON DELETE CASCADE,
    contract_id UUID NOT NULL REFERENCES lease_contracts(id) ON DELETE CASCADE,
    adjustment_type VARCHAR(20) NOT NULL CHECK (adjustment_type IN ('modification', 'reassessment', 'impairment')),
    effective_date DATE NOT NULL,
    liability_before DECIMAL(18,2) NOT NULL,
    liability_after DECIMAL(18,2) NOT NULL,
    liability_adjustment DECIMAL(18,2) NOT NULL,
    rou_before DECIMAL(18,2) NOT NULL,
    rou_after DECIMAL(18,2) NOT NULL,
    rou_adjustment DECIMAL(18,2) NOT NULL,
    pnl_gain DECIMAL(18,2) NOT NULL DEFAULT 0,
    pnl_loss DECIMAL(18,2) NOT NULL DEFAULT 0,
    revised_discount_rate DECIMAL(10,6) NOT NULL,
    discount_rate_source VARCHAR(100),
    calculation_batch_id UUID,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS system_settings (
    setting_key VARCHAR(100) PRIMARY KEY,
    setting_value TEXT NOT NULL,
    description TEXT,
    updated_by UUID REFERENCES users(id),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- ── 2. retail_store_day_category_facts (048) — NULL-proof dedup key ────────
DELETE FROM retail_store_day_category_facts
WHERE id NOT IN (
    SELECT DISTINCT ON (
        legal_entity_id, store_id, business_date, currency, category_code,
        data_classification, COALESCE(simulation_dataset_version, ''), version
    ) id
    FROM retail_store_day_category_facts
    ORDER BY legal_entity_id, store_id, business_date, currency, category_code,
             data_classification, COALESCE(simulation_dataset_version, ''), version,
             created_at, id
);

ALTER TABLE retail_store_day_category_facts DROP CONSTRAINT IF EXISTS uq_retail_store_day_category_fact;
CREATE UNIQUE INDEX IF NOT EXISTS uq_retail_store_day_category_fact_dedup
ON retail_store_day_category_facts (
    legal_entity_id, store_id, business_date, currency, category_code,
    data_classification, COALESCE(simulation_dataset_version, ''), version
);

-- ── 3. retail_store_day_inventory_facts (051) — envelope + dedup key ────────
ALTER TABLE retail_store_day_inventory_facts
    ADD COLUMN IF NOT EXISTS data_classification VARCHAR(32) NOT NULL DEFAULT 'production';
ALTER TABLE retail_store_day_inventory_facts
    ADD COLUMN IF NOT EXISTS source_system VARCHAR(100) NOT NULL DEFAULT 'unknown';
ALTER TABLE retail_store_day_inventory_facts
    ADD COLUMN IF NOT EXISTS import_batch_id VARCHAR(64);
ALTER TABLE retail_store_day_inventory_facts
    ADD COLUMN IF NOT EXISTS as_of_at TIMESTAMP WITH TIME ZONE;
ALTER TABLE retail_store_day_inventory_facts
    ADD COLUMN IF NOT EXISTS version INT NOT NULL DEFAULT 1;

DELETE FROM retail_store_day_inventory_facts
WHERE id NOT IN (
    SELECT DISTINCT ON (
        legal_entity_id, store_id, business_date, currency,
        COALESCE(category_code, ''), COALESCE(sku_code, '')
    ) id
    FROM retail_store_day_inventory_facts
    ORDER BY legal_entity_id, store_id, business_date, currency,
             COALESCE(category_code, ''), COALESCE(sku_code, ''),
             created_at, id
);

ALTER TABLE retail_store_day_inventory_facts DROP CONSTRAINT IF EXISTS uq_retail_store_day_inventory;
CREATE UNIQUE INDEX IF NOT EXISTS uq_retail_store_day_inventory_dedup
ON retail_store_day_inventory_facts (
    legal_entity_id, store_id, business_date, currency,
    COALESCE(category_code, ''), COALESCE(sku_code, '')
);

-- ── 4. retail_competitor_observations (051) — envelope + dedup key ──────────
ALTER TABLE retail_competitor_observations
    ADD COLUMN IF NOT EXISTS data_classification VARCHAR(32) NOT NULL DEFAULT 'production';
ALTER TABLE retail_competitor_observations
    ADD COLUMN IF NOT EXISTS source_system VARCHAR(100) NOT NULL DEFAULT 'manual_import';
ALTER TABLE retail_competitor_observations
    ADD COLUMN IF NOT EXISTS import_batch_id VARCHAR(64);

DELETE FROM retail_competitor_observations
WHERE id NOT IN (
    SELECT DISTINCT ON (legal_entity_id, store_id, observation_date, competitor_name) id
    FROM retail_competitor_observations
    ORDER BY legal_entity_id, store_id, observation_date, competitor_name, created_at, id
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_retail_competitor_observation
ON retail_competitor_observations (legal_entity_id, store_id, observation_date, competitor_name);

-- ── 5. promotion_costs (049) — dedup key + provenance ───────────────────────
ALTER TABLE promotion_costs
    ADD COLUMN IF NOT EXISTS source_system VARCHAR(100) NOT NULL DEFAULT 'manual_import';
ALTER TABLE promotion_costs
    ADD COLUMN IF NOT EXISTS import_batch_id VARCHAR(64);

DELETE FROM promotion_costs
WHERE id NOT IN (
    SELECT DISTINCT ON (promotion_id, period, cost_category, amount, currency) id
    FROM promotion_costs
    ORDER BY promotion_id, period, cost_category, amount, currency, created_at, id
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_promotion_costs_dedup
ON promotion_costs (promotion_id, period, cost_category, amount, currency);