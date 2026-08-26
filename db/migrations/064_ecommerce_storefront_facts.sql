-- 064_ecommerce_storefront_facts.sql (电商独立站模式 P0 · E1–E4 批次)
-- 新建站点主数据与三类事实族（storefront_day_facts / campaign_day_facts / order_line_evidence）、
-- 收款对账证据族（payout_lines / bank_lines / ad_invoices / media_rebates / rolling_reserve_events /
-- settlement_runs）与 GL 收入落地表、固定费分摊表。
-- 依据：docs/PRD_电商独立站经营分析模式.md（D2/D3/D6/D8）、docs/specs/ecommerce-dtc-mode-v1.md
-- R-E1-1..R-E1-4、R-E2-*、R-E4-*。事实族全部携带五信封字段（source_system / import_batch_id /
-- fact_version / as_of_at / data_classification）；重述以新 fact_version 追加、绝不覆盖（R-E2-2）。
-- 与 db/init/01_init.sql 的空库版本必须同时提供且等价（环境漂移守卫，教训 27ccdd2/32aac80）。

-- ====== 站点主数据 ======
CREATE TABLE IF NOT EXISTS storefronts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID NOT NULL REFERENCES legal_entities(id),
    code VARCHAR(64) NOT NULL,
    name VARCHAR(200) NOT NULL,
    market VARCHAR(100) NOT NULL DEFAULT '',
    currency VARCHAR(10) NOT NULL,
    platform VARCHAR(50) NOT NULL DEFAULT 'shopify',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (currency ~ '^[A-Z]{3}$'),
    CHECK (status IN ('active', 'inactive')),
    UNIQUE (legal_entity_id, code)
);

-- ====== 站点日事实（Storefront-Day Fact：站点 × 日 × 渠道 × SKU 聚合）======
-- 多来源并存：业务键含 source_system（shopify 出售售类度量、procurement 出落地成本、
-- 3pl 出履约成本），读取端按（业务键 × source_system）取最高 fact_version 后按度量求和；
-- 同一度量被两个来源同时给出时由读侧报 Source Conflict（R-E2-4）。
CREATE TABLE IF NOT EXISTS storefront_day_facts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID NOT NULL REFERENCES legal_entities(id),
    storefront_id UUID NOT NULL REFERENCES storefronts(id) ON DELETE CASCADE,
    business_date DATE NOT NULL,
    channel VARCHAR(120) NOT NULL DEFAULT 'direct',
    sku VARCHAR(120) NOT NULL DEFAULT '',
    currency VARCHAR(10) NOT NULL,
    gmv_amount DECIMAL(18,2),
    discount_amount DECIMAL(18,2),
    refund_amount DECIMAL(18,2),
    chargeback_loss_amount DECIMAL(18,2),
    order_count INTEGER,
    new_customer_orders INTEGER,
    landed_cost_amount DECIMAL(18,2),
    fulfillment_amount DECIMAL(18,2),
    payment_fee_amount DECIMAL(18,2),
    tax_collected_amount DECIMAL(18,2),
    source_system VARCHAR(100) NOT NULL,
    import_batch_id UUID NOT NULL REFERENCES operating_fact_batches(id),
    fact_version INTEGER NOT NULL DEFAULT 1,
    as_of_at TIMESTAMP WITH TIME ZONE NOT NULL,
    data_classification VARCHAR(20) NOT NULL,
    simulation_dataset_version VARCHAR(80),
    restated BOOLEAN NOT NULL DEFAULT FALSE,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (currency ~ '^[A-Z]{3}$'),
    CHECK (fact_version >= 1),
    CHECK (data_classification IN ('production', 'simulated', 'mixed')),
    CHECK (data_classification <> 'simulated' OR simulation_dataset_version IS NOT NULL),
    CHECK (data_classification <> 'production' OR simulation_dataset_version IS NULL),
    UNIQUE (storefront_id, business_date, channel, sku, source_system, fact_version)
);
CREATE INDEX IF NOT EXISTS idx_storefront_day_facts_lookup
    ON storefront_day_facts(storefront_id, business_date, fact_version DESC);
CREATE INDEX IF NOT EXISTS idx_storefront_day_facts_entity
    ON storefront_day_facts(legal_entity_id, business_date);

-- ====== 广告日事实（Campaign-Day Fact；booked / paid 双口径两行并存，R-E1-2 / R-T3）======
CREATE TABLE IF NOT EXISTS campaign_day_facts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID NOT NULL REFERENCES legal_entities(id),
    storefront_id UUID NOT NULL REFERENCES storefronts(id) ON DELETE CASCADE,
    campaign_id VARCHAR(150) NOT NULL DEFAULT 'all',
    campaign_name VARCHAR(200),
    business_date DATE NOT NULL,
    basis VARCHAR(10) NOT NULL,
    media_owner VARCHAR(50),
    spend_amount DECIMAL(18,2) NOT NULL DEFAULT 0,
    impressions BIGINT,
    clicks BIGINT,
    conversions DECIMAL(14,4),
    invoice_no VARCHAR(150),
    currency VARCHAR(10) NOT NULL,
    source_system VARCHAR(100) NOT NULL,
    import_batch_id UUID NOT NULL REFERENCES operating_fact_batches(id),
    fact_version INTEGER NOT NULL DEFAULT 1,
    as_of_at TIMESTAMP WITH TIME ZONE NOT NULL,
    data_classification VARCHAR(20) NOT NULL,
    simulation_dataset_version VARCHAR(80),
    restated BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (basis IN ('booked', 'paid')),
    CHECK (basis <> 'paid' OR invoice_no IS NOT NULL),
    CHECK (currency ~ '^[A-Z]{3}$'),
    CHECK (fact_version >= 1),
    CHECK (data_classification IN ('production', 'simulated', 'mixed')),
    CHECK (data_classification <> 'simulated' OR simulation_dataset_version IS NOT NULL),
    CHECK (data_classification <> 'production' OR simulation_dataset_version IS NULL),
    UNIQUE (storefront_id, campaign_id, business_date, basis, source_system, fact_version)
);
CREATE INDEX IF NOT EXISTS idx_campaign_day_facts_lookup
    ON campaign_day_facts(storefront_id, business_date, basis, fact_version DESC);

-- ====== 订单行证据（仅下钻与对账路径；不建分析索引——它是证据不是查询对象）======
CREATE TABLE IF NOT EXISTS order_line_evidence (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID NOT NULL REFERENCES legal_entities(id),
    storefront_id UUID NOT NULL REFERENCES storefronts(id) ON DELETE CASCADE,
    platform_order_no VARCHAR(100) NOT NULL,
    line_no INTEGER NOT NULL DEFAULT 1,
    business_date DATE NOT NULL,
    channel VARCHAR(120) NOT NULL DEFAULT 'direct',
    sku VARCHAR(120) NOT NULL DEFAULT '',
    quantity DECIMAL(14,4),
    gross_amount DECIMAL(18,2),
    discount_amount DECIMAL(18,2),
    refund_amount DECIMAL(18,2),
    tax_amount DECIMAL(18,2),
    currency VARCHAR(10) NOT NULL,
    payout_id VARCHAR(150),
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_system VARCHAR(100) NOT NULL,
    import_batch_id UUID NOT NULL REFERENCES operating_fact_batches(id),
    fact_version INTEGER NOT NULL DEFAULT 1,
    as_of_at TIMESTAMP WITH TIME ZONE NOT NULL,
    data_classification VARCHAR(20) NOT NULL,
    simulation_dataset_version VARCHAR(80),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (line_no >= 1),
    CHECK (fact_version >= 1),
    CHECK (data_classification IN ('production', 'simulated', 'mixed')),
    UNIQUE (storefront_id, platform_order_no, line_no, source_system, fact_version)
);

-- ====== payout 明细 ======
CREATE TABLE IF NOT EXISTS payout_lines (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID NOT NULL REFERENCES legal_entities(id),
    storefront_id UUID NOT NULL REFERENCES storefronts(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    payout_id VARCHAR(150) NOT NULL,
    payout_date DATE NOT NULL,
    currency VARCHAR(10) NOT NULL,
    gross_amount DECIMAL(18,2) NOT NULL DEFAULT 0,
    fee_amount DECIMAL(18,2) NOT NULL DEFAULT 0,
    refund_amount DECIMAL(18,2) NOT NULL DEFAULT 0,
    chargeback_amount DECIMAL(18,2) NOT NULL DEFAULT 0,
    fx_amount DECIMAL(18,2) NOT NULL DEFAULT 0,
    adjustment_amount DECIMAL(18,2) NOT NULL DEFAULT 0,
    reserve_hold_amount DECIMAL(18,2) NOT NULL DEFAULT 0,
    reserve_release_amount DECIMAL(18,2) NOT NULL DEFAULT 0,
    net_amount DECIMAL(18,2) NOT NULL DEFAULT 0,
    source_system VARCHAR(100) NOT NULL,
    import_batch_id UUID NOT NULL REFERENCES operating_fact_batches(id),
    fact_version INTEGER NOT NULL DEFAULT 1,
    as_of_at TIMESTAMP WITH TIME ZONE NOT NULL,
    data_classification VARCHAR(20) NOT NULL,
    simulation_dataset_version VARCHAR(80),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (currency ~ '^[A-Z]{3}$'),
    CHECK (fact_version >= 1),
    CHECK (data_classification IN ('production', 'simulated', 'mixed')),
    UNIQUE (storefront_id, provider, payout_id, source_system, fact_version)
);
CREATE INDEX IF NOT EXISTS idx_payout_lines_lookup ON payout_lines(storefront_id, payout_date);

-- ====== 银行到账流水 ======
CREATE TABLE IF NOT EXISTS bank_lines (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID NOT NULL REFERENCES legal_entities(id),
    storefront_id UUID NOT NULL REFERENCES storefronts(id) ON DELETE CASCADE,
    bank_ref VARCHAR(150) NOT NULL,
    value_date DATE NOT NULL,
    currency VARCHAR(10) NOT NULL,
    amount DECIMAL(18,2) NOT NULL,
    direction VARCHAR(6) NOT NULL DEFAULT 'in',
    counterparty VARCHAR(200),
    envelope JSONB NOT NULL DEFAULT '{}'::jsonb,
    source_system VARCHAR(100) NOT NULL,
    import_batch_id UUID NOT NULL REFERENCES operating_fact_batches(id),
    fact_version INTEGER NOT NULL DEFAULT 1,
    as_of_at TIMESTAMP WITH TIME ZONE NOT NULL,
    data_classification VARCHAR(20) NOT NULL,
    simulation_dataset_version VARCHAR(80),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (direction IN ('in', 'out')),
    CHECK (currency ~ '^[A-Z]{3}$'),
    CHECK (fact_version >= 1),
    CHECK (data_classification IN ('production', 'simulated', 'mixed')),
    UNIQUE (storefront_id, bank_ref, source_system, fact_version)
);
CREATE INDEX IF NOT EXISTS idx_bank_lines_lookup ON bank_lines(storefront_id, value_date);

-- ====== 代理发票（实付口径登记）======
CREATE TABLE IF NOT EXISTS ad_invoices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID NOT NULL REFERENCES legal_entities(id),
    storefront_id UUID NOT NULL REFERENCES storefronts(id) ON DELETE CASCADE,
    invoice_no VARCHAR(150) NOT NULL,
    agent_name VARCHAR(200),
    media_owner VARCHAR(50),
    period_start DATE,
    period_end DATE,
    invoice_date DATE,
    currency VARCHAR(10) NOT NULL,
    gross_amount DECIMAL(18,2) NOT NULL DEFAULT 0,
    rebate_amount DECIMAL(18,2) NOT NULL DEFAULT 0,
    payable_amount DECIMAL(18,2) NOT NULL DEFAULT 0,
    source_system VARCHAR(100) NOT NULL,
    import_batch_id UUID NOT NULL REFERENCES operating_fact_batches(id),
    fact_version INTEGER NOT NULL DEFAULT 1,
    as_of_at TIMESTAMP WITH TIME ZONE NOT NULL,
    data_classification VARCHAR(20) NOT NULL,
    simulation_dataset_version VARCHAR(80),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (fact_version >= 1),
    CHECK (data_classification IN ('production', 'simulated', 'mixed')),
    UNIQUE (invoice_no, source_system, fact_version)
);

-- ====== 媒体返点台账（冲减后续期间实付广告费）======
CREATE TABLE IF NOT EXISTS media_rebates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID NOT NULL REFERENCES legal_entities(id),
    storefront_id UUID NOT NULL REFERENCES storefronts(id) ON DELETE CASCADE,
    rebate_ref VARCHAR(150) NOT NULL,
    media_owner VARCHAR(50),
    apply_period VARCHAR(7) NOT NULL,
    currency VARCHAR(10) NOT NULL,
    amount DECIMAL(18,2) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'accrued',
    received_date DATE,
    source_system VARCHAR(100) NOT NULL,
    import_batch_id UUID NOT NULL REFERENCES operating_fact_batches(id),
    fact_version INTEGER NOT NULL DEFAULT 1,
    as_of_at TIMESTAMP WITH TIME ZONE NOT NULL,
    data_classification VARCHAR(20) NOT NULL,
    simulation_dataset_version VARCHAR(80),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (apply_period ~ '^[0-9]{4}-[0-9]{2}$'),
    CHECK (status IN ('accrued', 'received', 'applied')),
    CHECK (currency ~ '^[A-Z]{3}$'),
    CHECK (fact_version >= 1),
    CHECK (data_classification IN ('production', 'simulated', 'mixed')),
    UNIQUE (storefront_id, rebate_ref, source_system, fact_version)
);

-- ====== PayPal 滚动准备金占用/释放事件（状态机：hold open → released）======
CREATE TABLE IF NOT EXISTS rolling_reserve_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID NOT NULL REFERENCES legal_entities(id),
    storefront_id UUID NOT NULL REFERENCES storefronts(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    event_type VARCHAR(10) NOT NULL,
    event_date DATE NOT NULL,
    currency VARCHAR(10) NOT NULL,
    amount DECIMAL(18,2) NOT NULL,
    payout_id VARCHAR(150),
    hold_event_id UUID REFERENCES rolling_reserve_events(id),
    expected_release_date DATE,
    released_at DATE,
    status VARCHAR(20) NOT NULL DEFAULT 'open',
    source_system VARCHAR(100) NOT NULL,
    import_batch_id UUID NOT NULL REFERENCES operating_fact_batches(id),
    fact_version INTEGER NOT NULL DEFAULT 1,
    as_of_at TIMESTAMP WITH TIME ZONE NOT NULL,
    data_classification VARCHAR(20) NOT NULL,
    simulation_dataset_version VARCHAR(80),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (event_type IN ('hold', 'release')),
    CHECK (status IN ('open', 'released')),
    CHECK (event_type <> 'release' OR status = 'released'),
    CHECK (event_type <> 'release' OR hold_event_id IS NOT NULL),
    CHECK (event_type <> 'hold' OR hold_event_id IS NULL),
    CHECK (currency ~ '^[A-Z]{3}$'),
    CHECK (fact_version >= 1),
    CHECK (data_classification IN ('production', 'simulated', 'mixed')),
    UNIQUE (storefront_id, provider, event_type, event_date, amount, payout_id, source_system, fact_version)
);
CREATE INDEX IF NOT EXISTS idx_rolling_reserve_events_open
    ON rolling_reserve_events(storefront_id, status, currency);

-- ====== 对账 run 与签认状态机（Draft → Prepare → Pending → Approved）======
CREATE TABLE IF NOT EXISTS settlement_runs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID NOT NULL REFERENCES legal_entities(id),
    storefront_id UUID NOT NULL REFERENCES storefronts(id) ON DELETE CASCADE,
    period VARCHAR(7) NOT NULL,
    currency VARCHAR(10) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    policy_version VARCHAR(50) NOT NULL DEFAULT 'settlement-match-v1',
    gate_verdict VARCHAR(10),
    matched_count INTEGER NOT NULL DEFAULT 0,
    difference_count INTEGER NOT NULL DEFAULT 0,
    total_difference_amount DECIMAL(18,2) NOT NULL DEFAULT 0,
    results JSONB NOT NULL DEFAULT '[]'::jsonb,
    differences JSONB NOT NULL DEFAULT '[]'::jsonb,
    prepared_by UUID REFERENCES users(id),
    prepared_at TIMESTAMP WITH TIME ZONE,
    submitted_by UUID REFERENCES users(id),
    submitted_at TIMESTAMP WITH TIME ZONE,
    approved_by UUID REFERENCES users(id),
    approved_at TIMESTAMP WITH TIME ZONE,
    rejected_by UUID REFERENCES users(id),
    rejected_at TIMESTAMP WITH TIME ZONE,
    rejection_reason TEXT,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (period ~ '^[0-9]{4}-[0-9]{2}$'),
    CHECK (status IN ('draft', 'prepared', 'pending', 'approved', 'rejected')),
    CHECK (gate_verdict IS NULL OR gate_verdict IN ('allow', 'deny')),
    CHECK (currency ~ '^[A-Z]{3}$')
);
CREATE INDEX IF NOT EXISTS idx_settlement_runs_lookup ON settlement_runs(legal_entity_id, period);
-- 请求级幂等：同一法人下同一幂等键只允许一个 run
ALTER TABLE settlement_runs ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(255);
DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ux_settlement_runs_idempotency') THEN
        ALTER TABLE settlement_runs ADD CONSTRAINT ux_settlement_runs_idempotency
            UNIQUE (legal_entity_id, idempotency_key);
    END IF;
END $$;

-- ====== GL 收入落地点（会计口径收入唯一来源；只读消费，sitepnl 不自算收入 R-E3-5）======
CREATE TABLE IF NOT EXISTS storefront_gl_revenues (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID NOT NULL REFERENCES legal_entities(id),
    storefront_id UUID NOT NULL REFERENCES storefronts(id) ON DELETE CASCADE,
    period VARCHAR(7) NOT NULL,
    currency VARCHAR(10) NOT NULL,
    revenue_amount DECIMAL(18,2),
    gl_account VARCHAR(50),
    source_system VARCHAR(100) NOT NULL,
    import_batch_id UUID NOT NULL REFERENCES operating_fact_batches(id),
    fact_version INTEGER NOT NULL DEFAULT 1,
    as_of_at TIMESTAMP WITH TIME ZONE NOT NULL,
    data_classification VARCHAR(20) NOT NULL,
    simulation_dataset_version VARCHAR(80),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (period ~ '^[0-9]{4}-[0-9]{2}$'),
    CHECK (currency ~ '^[A-Z]{3}$'),
    CHECK (fact_version >= 1),
    CHECK (data_classification IN ('production', 'simulated', 'mixed')),
    UNIQUE (storefront_id, period, currency, source_system, fact_version)
);

-- ====== 分摊固定费（经营利润行的输入之一）======
CREATE TABLE IF NOT EXISTS storefront_fixed_costs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    legal_entity_id UUID NOT NULL REFERENCES legal_entities(id),
    storefront_id UUID NOT NULL REFERENCES storefronts(id) ON DELETE CASCADE,
    period VARCHAR(7) NOT NULL,
    currency VARCHAR(10) NOT NULL,
    fixed_cost_amount DECIMAL(18,2),
    memo TEXT,
    source_system VARCHAR(100) NOT NULL,
    import_batch_id UUID NOT NULL REFERENCES operating_fact_batches(id),
    fact_version INTEGER NOT NULL DEFAULT 1,
    as_of_at TIMESTAMP WITH TIME ZONE NOT NULL,
    data_classification VARCHAR(20) NOT NULL,
    simulation_dataset_version VARCHAR(80),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (period ~ '^[0-9]{4}-[0-9]{2}$'),
    CHECK (currency ~ '^[A-Z]{3}$'),
    CHECK (fact_version >= 1),
    CHECK (data_classification IN ('production', 'simulated', 'mixed')),
    UNIQUE (storefront_id, period, currency, source_system, fact_version)
);

-- ====== 请求级幂等登记（镜像 retail_store_day_fact_requests 形状）======
CREATE TABLE IF NOT EXISTS ecommerce_ingest_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    scope_key VARCHAR(100) NOT NULL,
    legal_entity_id UUID REFERENCES legal_entities(id),
    request_kind VARCHAR(50) NOT NULL,
    idempotency_key VARCHAR(255) NOT NULL,
    payload_sha256 VARCHAR(64) NOT NULL,
    record_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CHECK (BTRIM(scope_key) <> ''),
    CHECK (BTRIM(request_kind) <> ''),
    CHECK (BTRIM(idempotency_key) <> ''),
    CHECK (payload_sha256 ~ '^[0-9a-f]{64}$'),
    UNIQUE (scope_key, request_kind, idempotency_key)
);
CREATE INDEX IF NOT EXISTS idx_ecommerce_ingest_requests_entity
    ON ecommerce_ingest_requests(legal_entity_id, created_at DESC);
