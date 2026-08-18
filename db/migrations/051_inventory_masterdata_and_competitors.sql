-- 051_inventory_masterdata_and_competitors.sql
-- Consolidated Batches F7, F8, F9 (PRD §3.F7~F9 / CodebaseDesign §12~13)

-- 1. Batch F7: Store-Day Inventory & In-Transit Facts
CREATE TABLE IF NOT EXISTS retail_store_day_inventory_facts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    legal_entity_id VARCHAR(64) NOT NULL,
    store_id VARCHAR(64) NOT NULL,
    business_date DATE NOT NULL,
    currency VARCHAR(16) NOT NULL DEFAULT 'CNY',
    category_code VARCHAR(64),
    sku_code VARCHAR(64),
    stock_qty DECIMAL(18,2) NOT NULL DEFAULT 0,
    stock_cost DECIMAL(18,2) NOT NULL DEFAULT 0,
    in_transit_qty DECIMAL(18,2) NOT NULL DEFAULT 0,
    in_transit_cost DECIMAL(18,2) NOT NULL DEFAULT 0,
    days_of_inventory DECIMAL(10,2),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_retail_store_day_inventory UNIQUE (legal_entity_id, store_id, business_date, currency, category_code, sku_code)
);

CREATE INDEX IF NOT EXISTS idx_retail_inventory_facts_lookup 
ON retail_store_day_inventory_facts(legal_entity_id, store_id, business_date);

-- 2. Batch F8: Master Data Entity Mappings Expansion
-- Ensure mapping_type supports store, sku, category
CREATE TABLE IF NOT EXISTS master_data_entity_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    legal_entity_id VARCHAR(64) NOT NULL,
    entity_kind VARCHAR(32) NOT NULL, -- store, sku, category
    external_system VARCHAR(64) NOT NULL,
    raw_identifier VARCHAR(256) NOT NULL,
    canonical_id VARCHAR(64) NOT NULL,
    canonical_name VARCHAR(256),
    confidence_score DECIMAL(5,4) NOT NULL DEFAULT 1.0000,
    resolved_by VARCHAR(32) NOT NULL DEFAULT 'manual', -- manual, rule, ai
    is_confirmed BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_entity_mapping UNIQUE (legal_entity_id, entity_kind, external_system, raw_identifier)
);

CREATE INDEX IF NOT EXISTS idx_entity_mappings_lookup 
ON master_data_entity_mappings(legal_entity_id, entity_kind, external_system);

-- 3. Batch F9: Competitor Observations (Physically Isolated from Retail KPI Facts)
CREATE TABLE IF NOT EXISTS retail_competitor_observations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    legal_entity_id VARCHAR(64) NOT NULL,
    store_id VARCHAR(64) NOT NULL,
    competitor_name VARCHAR(128) NOT NULL,
    competitor_brand VARCHAR(128),
    distance_meters INT,
    observation_date DATE NOT NULL,
    price_index DECIMAL(5,2), -- e.g. 1.05 = 5% higher than our store
    promo_intensity VARCHAR(32) NOT NULL DEFAULT 'medium', -- low, medium, high, aggressive
    footfall_estimate INT,
    observer VARCHAR(64),
    notes TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_competitor_obs_lookup 
ON retail_competitor_observations(legal_entity_id, store_id, observation_date);
