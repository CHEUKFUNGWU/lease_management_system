-- +goose Up
-- +goose StatementBegin

-- 1. 月度计量结果表（IFRS 16 计算结果）
CREATE TABLE IF NOT EXISTS measurement_results (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contract_id UUID NOT NULL REFERENCES lease_contracts(id) ON DELETE CASCADE,
    accounting_period VARCHAR(7) NOT NULL, -- YYYY-MM
    period_start_date DATE NOT NULL,
    period_end_date DATE NOT NULL,
    opening_liability DECIMAL(18, 2) NOT NULL DEFAULT 0,
    interest_expense DECIMAL(18, 2) NOT NULL DEFAULT 0,
    principal_repayment DECIMAL(18, 2) NOT NULL DEFAULT 0,
    total_payment DECIMAL(18, 2) NOT NULL DEFAULT 0,
    closing_liability DECIMAL(18, 2) NOT NULL DEFAULT 0,
    opening_rou_asset DECIMAL(18, 2) NOT NULL DEFAULT 0,
    depreciation DECIMAL(18, 2) NOT NULL DEFAULT 0,
    closing_rou_asset DECIMAL(18, 2) NOT NULL DEFAULT 0,
    variable_rent_expense DECIMAL(18, 2) NOT NULL DEFAULT 0,
    non_lease_expense DECIMAL(18, 2) NOT NULL DEFAULT 0,
    discount_rate DECIMAL(10, 6) NOT NULL,
    is_calculated BOOLEAN NOT NULL DEFAULT false,
    calculation_batch_id UUID,
    calculated_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(contract_id, accounting_period)
);

-- 2. 会计分录表
CREATE TABLE IF NOT EXISTS journal_entries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    contract_id UUID NOT NULL REFERENCES lease_contracts(id) ON DELETE CASCADE,
    measurement_result_id UUID REFERENCES measurement_results(id),
    accounting_period VARCHAR(7) NOT NULL, -- YYYY-MM
    entry_date DATE NOT NULL,
    entry_type VARCHAR(50) NOT NULL, -- interest, depreciation, payment, variable_rent, non_lease, modification, reassessment
    debit_account VARCHAR(100) NOT NULL,
    credit_account VARCHAR(100) NOT NULL,
    amount DECIMAL(18, 2) NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'CNY',
    description TEXT,
    voucher_number VARCHAR(100),
    posting_status VARCHAR(20) NOT NULL DEFAULT 'draft', -- draft, preview, approved, posted, reversed
    posted_at TIMESTAMP WITH TIME ZONE,
    posted_by UUID REFERENCES users(id),
    approved_by UUID REFERENCES users(id),
    approved_at TIMESTAMP WITH TIME ZONE,
    erp_reference VARCHAR(255),
    batch_id UUID,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 3. 月结批次表
CREATE TABLE IF NOT EXISTS monthly_closing_batches (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    batch_number VARCHAR(50) NOT NULL UNIQUE,
    accounting_period VARCHAR(7) NOT NULL,
    legal_entity_id UUID REFERENCES legal_entities(id),
    region VARCHAR(100),
    brand VARCHAR(100),
    status VARCHAR(20) NOT NULL DEFAULT 'draft', -- draft, running, completed, failed, cancelled
    total_contracts INTEGER NOT NULL DEFAULT 0,
    processed_contracts INTEGER NOT NULL DEFAULT 0,
    failed_contracts INTEGER NOT NULL DEFAULT 0,
    total_entries INTEGER NOT NULL DEFAULT 0,
    posted_entries INTEGER NOT NULL DEFAULT 0,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 4. 索引
CREATE INDEX IF NOT EXISTS idx_measurement_results_contract ON measurement_results(contract_id);
CREATE INDEX IF NOT EXISTS idx_measurement_results_period ON measurement_results(accounting_period);
CREATE INDEX IF NOT EXISTS idx_measurement_results_batch ON measurement_results(calculation_batch_id);
CREATE INDEX IF NOT EXISTS idx_journal_entries_contract ON journal_entries(contract_id);
CREATE INDEX IF NOT EXISTS idx_journal_entries_period ON journal_entries(accounting_period);
CREATE INDEX IF NOT EXISTS idx_journal_entries_batch ON journal_entries(batch_id);
CREATE INDEX IF NOT EXISTS idx_journal_entries_posting ON journal_entries(posting_status);
CREATE INDEX IF NOT EXISTS idx_monthly_closing_period ON monthly_closing_batches(accounting_period);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_monthly_closing_period;
DROP INDEX IF EXISTS idx_journal_entries_posting;
DROP INDEX IF EXISTS idx_journal_entries_batch;
DROP INDEX IF EXISTS idx_journal_entries_period;
DROP INDEX IF EXISTS idx_journal_entries_contract;
DROP INDEX IF EXISTS idx_measurement_results_batch;
DROP INDEX IF EXISTS idx_measurement_results_period;
DROP INDEX IF EXISTS idx_measurement_results_contract;
DROP TABLE IF EXISTS monthly_closing_batches;
DROP TABLE IF EXISTS journal_entries;
DROP TABLE IF EXISTS measurement_results;
-- +goose StatementEnd
