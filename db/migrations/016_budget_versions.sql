-- +goose Up
-- +goose StatementBegin

-- A budget version freezes the measured forward schedule at a point in time, so
-- later months can be compared against what was expected rather than against a
-- number that moves every time the portfolio changes.
CREATE TABLE IF NOT EXISTS budget_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    legal_entity_id UUID REFERENCES legal_entities(id),
    -- as_of_period is the period the snapshot was taken in; every line covers a
    -- period at or after it.
    as_of_period VARCHAR(7) NOT NULL,
    from_period VARCHAR(7) NOT NULL,
    to_period VARCHAR(7) NOT NULL,
    contract_count INTEGER NOT NULL DEFAULT 0,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- One frozen row per contract and period. Amounts are the lease cost the budget
-- expected: interest plus depreciation, with the cash payment kept alongside for
-- cash-flow comparisons.
CREATE TABLE IF NOT EXISTS budget_lines (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    budget_version_id UUID NOT NULL REFERENCES budget_versions(id) ON DELETE CASCADE,
    contract_id UUID NOT NULL REFERENCES lease_contracts(id) ON DELETE CASCADE,
    accounting_period VARCHAR(7) NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'CNY',
    interest_expense DECIMAL(18, 2) NOT NULL DEFAULT 0,
    depreciation DECIMAL(18, 2) NOT NULL DEFAULT 0,
    total_payment DECIMAL(18, 2) NOT NULL DEFAULT 0,
    closing_liability DECIMAL(18, 2) NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (budget_version_id, contract_id, accounting_period)
);

CREATE INDEX IF NOT EXISTS idx_budget_lines_period
    ON budget_lines(budget_version_id, accounting_period);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_budget_lines_period;
DROP TABLE IF EXISTS budget_lines;
DROP TABLE IF EXISTS budget_versions;
-- +goose StatementEnd
