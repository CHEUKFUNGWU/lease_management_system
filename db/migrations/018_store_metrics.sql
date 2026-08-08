-- Rent-to-sales is the first number a retail lease decision turns on, and the
-- system has had no way to compute it: it knows the rent and nothing about the
-- sales.
--
-- store_metrics is deliberately a thin slice — store × period × two or three
-- figures — not a P&L. The authoritative source stays the customer's POS, ERP
-- or BI; this table is only ever a consumer of that data, and every report
-- built on it says so. Trying to hold more would put the system in competition
-- with the system of record, which it would lose.

CREATE TABLE IF NOT EXISTS store_metrics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    store_id UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,

    -- The period the figures cover, as YYYY-MM. Which calendar that month
    -- follows is the customer's business, so period_basis records what they
    -- told us rather than assuming a natural month.
    period VARCHAR(7) NOT NULL,
    period_basis VARCHAR(20) NOT NULL,

    revenue DECIMAL(18, 2) NOT NULL,
    gross_profit DECIMAL(18, 2),
    -- Sales in one currency against rent in another is not a ratio, so the
    -- currency travels with the figure and the report checks it.
    currency VARCHAR(10) NOT NULL,

    -- Revenue gets restated. version keeps the earlier submission rather than
    -- overwriting it, so a report can say which vintage it was based on.
    version INTEGER NOT NULL DEFAULT 1,
    -- Where the figure came from: 'manual', 'api', 'ai_upload'. Reports name it
    -- so a reader knows how much weight it carries.
    source VARCHAR(50) NOT NULL,
    note TEXT,

    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CHECK (revenue >= 0),
    CHECK (period ~ '^[0-9]{4}-[0-9]{2}$'),
    CHECK (period_basis IN ('calendar_month', 'fiscal_month')),
    UNIQUE (store_id, period, version)
);

COMMENT ON TABLE store_metrics IS
    '门店营收薄片:仅供租售比等分析消费,权威源为客户的 POS/ERP/BI';

CREATE INDEX IF NOT EXISTS idx_store_metrics_lookup
    ON store_metrics(store_id, period, version DESC);
CREATE INDEX IF NOT EXISTS idx_store_metrics_period
    ON store_metrics(period);

-- The Excel a business partner uploads says "万象城店"; the system calls it
-- "SZ-MOC-001 深圳万象城". Confirming that match by hand every month is what
-- stops the upload becoming a habit, so a confirmed match is remembered.
CREATE TABLE IF NOT EXISTS store_aliases (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    store_id UUID NOT NULL REFERENCES stores(id) ON DELETE CASCADE,
    alias VARCHAR(255) NOT NULL,
    source VARCHAR(50) NOT NULL DEFAULT 'manual',
    confirmed_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE (alias)
);

COMMENT ON TABLE store_aliases IS
    '门店别名映射:营收表里的叫法确认一次后记住,避免每月重复匹配';
