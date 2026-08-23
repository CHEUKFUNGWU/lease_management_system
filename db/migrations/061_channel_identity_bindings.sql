-- 061_channel_identity_bindings.sql (Ch3a · IM 网关的租户层，ADR-0026 §3)
--
-- 渠道身份（飞书 open_id / 企微 userid）→ 内部用户的绑定表。
-- internal/gateway 的 Resolve 是渠道进入系统的唯一入口：绑定命中后委托给与
-- JWT 完全相同的 Scope 解析器（middleware.LoadUserAccess + BuildAccessScope），
-- 渠道层拿不到任何拼装权限的材料。未绑定即拒绝，无默认/兜底租户（D-B14）。

CREATE TABLE IF NOT EXISTS channel_identity_bindings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    channel VARCHAR(32) NOT NULL CHECK (channel IN ('feishu', 'wecom')),
    external_user_id VARCHAR(255) NOT NULL,
    internal_user_id UUID NOT NULL REFERENCES users(id),
    bound_by UUID REFERENCES users(id),
    bound_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    -- 同一渠道身份只能映射到一个内部用户；换绑走显式 DELETE + INSERT，
    -- 不做静默覆盖（审计可追溯谁在什么时候绑的）。
    CONSTRAINT uq_channel_identity UNIQUE (channel, external_user_id)
);

CREATE INDEX IF NOT EXISTS idx_channel_identity_bindings_user
    ON channel_identity_bindings(internal_user_id);
