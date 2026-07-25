-- +goose Up
-- +goose StatementBegin

-- Exchange rates used to translate foreign-currency leases into a legal
-- entity's functional currency.
--
-- Rates are data, never derived: a close that needs a rate it does not have
-- fails loudly rather than guessing, mirroring how a missing discount rate
-- blocks measurement.
--
-- rate_type distinguishes the two rates IAS 21 needs: 'closing' for remeasuring
-- the monetary lease liability at period end, and 'average' for the period's
-- flows (interest and payments).
CREATE TABLE IF NOT EXISTS exchange_rates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    from_currency VARCHAR(10) NOT NULL,
    to_currency VARCHAR(10) NOT NULL,
    rate_date DATE NOT NULL,
    rate_type VARCHAR(20) NOT NULL DEFAULT 'closing',
    -- Units of to_currency per one unit of from_currency.
    rate DECIMAL(18, 8) NOT NULL,
    source VARCHAR(100),
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (rate > 0),
    CHECK (rate_type IN ('closing', 'average')),
    CHECK (from_currency <> to_currency),
    UNIQUE (from_currency, to_currency, rate_date, rate_type)
);

CREATE INDEX IF NOT EXISTS idx_exchange_rates_lookup
    ON exchange_rates(from_currency, to_currency, rate_type, rate_date DESC);

-- The approver owns the close, so they must be able to publish the month-end
-- rates it depends on. Admin already holds the wildcard permission.
INSERT INTO permissions (role_id, resource, action) VALUES
    ('44444444-4444-4444-4444-444444444444', 'settings', 'update')
ON CONFLICT (role_id, resource, action) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_exchange_rates_lookup;
DROP TABLE IF EXISTS exchange_rates;
DELETE FROM permissions WHERE role_id = '44444444-4444-4444-4444-444444444444' AND resource = 'settings' AND action = 'update';
-- +goose StatementEnd
