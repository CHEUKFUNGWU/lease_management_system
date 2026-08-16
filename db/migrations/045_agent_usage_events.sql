-- Migration 045: per-user AI usage events power the rate and cost budgets
-- (agentguard, M6.3). Counters live in this append-only table so the guard
-- spans instances; memory counting stays the single-instance/test adapter.
CREATE TABLE IF NOT EXISTS agent_usage_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    kind VARCHAR(30) NOT NULL,
    tokens INTEGER NOT NULL DEFAULT 0,
    cost_usd DECIMAL(12,6) NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_usage_events_user_recent
    ON agent_usage_events(user_id, kind, created_at DESC);
