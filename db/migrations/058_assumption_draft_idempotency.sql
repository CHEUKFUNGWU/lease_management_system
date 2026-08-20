-- 058_assumption_draft_idempotency.sql — SM5 假设草稿幂等（PRD S4-6 / 底线 4）
-- AI 建议草稿（fpna.assumptions.suggest）此前靠错误文案里的 key 冒充幂等；
-- 重放会撞 23505、批量可部分落库。这里把幂等变成数据库约束：
--   1) fpna_assumption_versions 增加 idempotency_key 列；
--   2) (legal_entity_id, idempotency_key) 唯一（partial），重放命中同批。
-- 批量写入包事务在仓库层（SaveAssumptionDrafts）。

ALTER TABLE fpna_assumption_versions
    ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(255);

CREATE UNIQUE INDEX IF NOT EXISTS ux_fpna_assumption_versions_idempotency
    ON fpna_assumption_versions(legal_entity_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
