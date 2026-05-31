-- Lease administration platform primitives:
-- asset type dimension, critical date reminders, and centralized document metadata.

ALTER TABLE lease_contracts
    ADD COLUMN IF NOT EXISTS asset_type VARCHAR(50) NOT NULL DEFAULT 'real_estate'
        CHECK (asset_type IN ('real_estate', 'vehicle', 'it_equipment', 'machinery', 'other'));

CREATE TABLE IF NOT EXISTS lease_documents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contract_id UUID NOT NULL REFERENCES lease_contracts(id) ON DELETE CASCADE,
    document_type VARCHAR(50) NOT NULL DEFAULT 'main_contract',
    file_name VARCHAR(255) NOT NULL,
    file_type VARCHAR(100),
    file_size BIGINT,
    minio_bucket VARCHAR(100),
    minio_object_key VARCHAR(500),
    document_version VARCHAR(50),
    file_hash VARCHAR(128),
    source_page INTEGER,
    notes TEXT,
    uploaded_by UUID REFERENCES users(id),
    uploaded_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS critical_dates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contract_id UUID NOT NULL REFERENCES lease_contracts(id) ON DELETE CASCADE,
    date_type VARCHAR(50) NOT NULL,
    target_date DATE NOT NULL,
    reminder_days INTEGER NOT NULL DEFAULT 30,
    responsible_user_id UUID REFERENCES users(id),
    status VARCHAR(50) NOT NULL DEFAULT 'open',
    title VARCHAR(255) NOT NULL,
    description TEXT,
    source VARCHAR(50) NOT NULL DEFAULT 'manual',
    source_reference_locator JSONB,
    completed_at TIMESTAMP WITH TIME ZONE,
    completed_by UUID REFERENCES users(id),
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (date_type IN ('renewal_deadline', 'break_notice', 'rent_review', 'lease_expiry', 'insurance_renewal', 'other')),
    CHECK (status IN ('open', 'snoozed', 'completed', 'cancelled')),
    CHECK (source IN ('manual', 'ai_suggested', 'policy_rule', 'system_generated'))
);

CREATE INDEX IF NOT EXISTS idx_lease_contracts_asset_type ON lease_contracts(asset_type);
CREATE INDEX IF NOT EXISTS idx_critical_dates_contract ON critical_dates(contract_id);
CREATE INDEX IF NOT EXISTS idx_critical_dates_due ON critical_dates(status, target_date);
CREATE INDEX IF NOT EXISTS idx_lease_documents_contract ON lease_documents(contract_id);
