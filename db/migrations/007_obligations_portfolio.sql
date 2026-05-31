-- Lease obligation register for operational clauses and non-accounting commitments.

CREATE TABLE IF NOT EXISTS lease_obligations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contract_id UUID NOT NULL REFERENCES lease_contracts(id) ON DELETE CASCADE,
    obligation_type VARCHAR(50) NOT NULL,
    responsible_party VARCHAR(50) NOT NULL DEFAULT 'lessee',
    title VARCHAR(255) NOT NULL,
    description TEXT,
    structured_value JSONB,
    source_clause TEXT,
    source_page INTEGER,
    source_reference_locator JSONB,
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (obligation_type IN ('maintenance', 'cam', 'insurance', 'index_adjustment', 'restoration', 'security_deposit', 'notice', 'other')),
    CHECK (responsible_party IN ('lessee', 'lessor', 'shared', 'third_party')),
    CHECK (status IN ('active', 'completed', 'waived', 'cancelled'))
);

CREATE INDEX IF NOT EXISTS idx_lease_obligations_contract ON lease_obligations(contract_id);
CREATE INDEX IF NOT EXISTS idx_lease_obligations_type ON lease_obligations(obligation_type);
CREATE INDEX IF NOT EXISTS idx_lease_obligations_status ON lease_obligations(status);
