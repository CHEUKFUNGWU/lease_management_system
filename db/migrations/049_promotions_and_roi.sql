-- 049_promotions_and_roi.sql
-- Batch F5: Promotion Master Data & ROI Attribution (PRD §3.F5 / CodebaseDesign §10)

CREATE TABLE IF NOT EXISTS promotions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    legal_entity_id VARCHAR(64) NOT NULL,
    promo_code VARCHAR(64) NOT NULL,
    name VARCHAR(128) NOT NULL,
    promo_type VARCHAR(32) NOT NULL, -- discount, coupon, gift, member_day, other
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    target_scope VARCHAR(32) NOT NULL DEFAULT 'all', -- all, region, brand, store_list
    scope_values TEXT[] DEFAULT '{}',
    currency VARCHAR(16) NOT NULL DEFAULT 'CNY',
    budget_amount DECIMAL(18,2) NOT NULL DEFAULT 0,
    owner VARCHAR(64),
    approval_status VARCHAR(32) NOT NULL DEFAULT 'draft', -- draft, approved, completed, cancelled
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_promotions_code UNIQUE (legal_entity_id, promo_code)
);

CREATE INDEX IF NOT EXISTS idx_promotions_lookup 
ON promotions(legal_entity_id, start_date, end_date);

CREATE TABLE IF NOT EXISTS promotion_costs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    promotion_id UUID NOT NULL REFERENCES promotions(id) ON DELETE CASCADE,
    period VARCHAR(32) NOT NULL,
    cost_category VARCHAR(50) NOT NULL, -- subsidy, materials, labor, marketing, other
    amount DECIMAL(18,2) NOT NULL DEFAULT 0,
    currency VARCHAR(16) NOT NULL DEFAULT 'CNY',
    notes TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_promotion_costs_promo 
ON promotion_costs(promotion_id, period);
