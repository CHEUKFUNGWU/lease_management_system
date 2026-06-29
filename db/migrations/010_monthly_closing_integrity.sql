-- Keep mixed-success close batches persistable on existing databases.
ALTER TABLE monthly_closing_batches
    ALTER COLUMN status TYPE VARCHAR(32);

ALTER TABLE monthly_closing_batches
    ADD COLUMN IF NOT EXISTS scope_contract_id UUID REFERENCES lease_contracts(id);

CREATE INDEX IF NOT EXISTS idx_monthly_closing_scope
    ON monthly_closing_batches(accounting_period, legal_entity_id, scope_contract_id);
