-- Migration 039: deterministic retail simulation registry and store source flags.
-- All statements are idempotent so this can be applied to an existing 038 database.

ALTER TABLE stores
    ADD COLUMN IF NOT EXISTS data_classification VARCHAR(20) NOT NULL DEFAULT 'production',
    ADD COLUMN IF NOT EXISTS simulation_dataset_version VARCHAR(100);

UPDATE stores
SET data_classification = 'production'
WHERE data_classification IS NULL OR BTRIM(data_classification) = '';

ALTER TABLE stores
    ALTER COLUMN data_classification SET DEFAULT 'production',
    ALTER COLUMN data_classification SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'stores'::regclass AND conname = 'stores_data_classification_check'
    ) THEN
        ALTER TABLE stores ADD CONSTRAINT stores_data_classification_check
            CHECK (data_classification IN ('production', 'simulated'));
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'stores'::regclass AND conname = 'stores_simulation_version_check'
    ) THEN
        ALTER TABLE stores ADD CONSTRAINT stores_simulation_version_check
            CHECK (
                (data_classification = 'simulated' AND NULLIF(BTRIM(simulation_dataset_version), '') IS NOT NULL)
                OR (data_classification = 'production' AND simulation_dataset_version IS NULL)
            );
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS retail_simulation_datasets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID NOT NULL REFERENCES legal_entities(id),
    dataset_version VARCHAR(100) NOT NULL,
    generator_version VARCHAR(100) NOT NULL,
    seed BIGINT NOT NULL,
    date_from DATE NOT NULL,
    date_to DATE NOT NULL,
    store_count INTEGER NOT NULL,
    fact_count INTEGER NOT NULL DEFAULT 0,
    parameters JSONB NOT NULL DEFAULT '{}'::jsonb,
    anomaly_manifest JSONB NOT NULL DEFAULT '[]'::jsonb,
    payload_sha256 VARCHAR(64) NOT NULL,
    business_sha256 VARCHAR(64) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'generating',
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    idempotency_key VARCHAR(255),
    import_batch_id UUID REFERENCES operating_fact_batches(id),
    CHECK (date_to >= date_from),
    CHECK (store_count BETWEEN 10 AND 100),
    CHECK (fact_count >= 0),
    CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
    CHECK (business_sha256 ~ '^[0-9a-f]{64}$'),
    CHECK (status IN ('generating', 'completed', 'failed')),
    UNIQUE (legal_entity_id, dataset_version)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_retail_simulation_datasets_idempotency
    ON retail_simulation_datasets(legal_entity_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_retail_simulation_datasets_entity_created
    ON retail_simulation_datasets(legal_entity_id, created_at DESC);
