-- 048_category_facts_and_reconciliation.sql
-- Batch F4: Category Dimension & Margin Decomposition (PRD §3.F4 / CodebaseDesign §9)

CREATE TABLE IF NOT EXISTS retail_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    legal_entity_id VARCHAR(64) NOT NULL,
    category_code VARCHAR(64) NOT NULL,
    name VARCHAR(128) NOT NULL,
    parent_code VARCHAR(64),
    effective_from DATE NOT NULL,
    effective_to DATE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_retail_category_code UNIQUE (legal_entity_id, category_code, effective_from)
);

CREATE INDEX IF NOT EXISTS idx_retail_categories_parent ON retail_categories(legal_entity_id, parent_code);

CREATE TABLE IF NOT EXISTS retail_store_day_category_facts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    legal_entity_id VARCHAR(64) NOT NULL,
    store_id VARCHAR(64) NOT NULL,
    business_date DATE NOT NULL,
    currency VARCHAR(16) NOT NULL,
    category_code VARCHAR(64) NOT NULL,
    revenue DECIMAL(18,2),
    gross_profit DECIMAL(18,2),
    transactions INT,
    units DECIMAL(18,2),
    
    -- Source Envelope
    source_system VARCHAR(64) NOT NULL,
    import_batch_id VARCHAR(64) NOT NULL,
    as_of_at TIMESTAMP WITH TIME ZONE NOT NULL,
    version INT NOT NULL DEFAULT 1,
    data_classification VARCHAR(32) NOT NULL DEFAULT 'production',
    simulation_dataset_version VARCHAR(64),
    data_quality_status VARCHAR(32) NOT NULL DEFAULT 'valid',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    CONSTRAINT uq_retail_store_day_category_fact UNIQUE (
        legal_entity_id, store_id, business_date, currency, category_code,
        data_classification, simulation_dataset_version, version
    )
);

CREATE INDEX IF NOT EXISTS idx_retail_store_day_category_facts_lookup 
ON retail_store_day_category_facts(legal_entity_id, store_id, business_date);
