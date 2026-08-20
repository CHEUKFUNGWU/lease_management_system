-- 059_assumption_draft_idempotency_batch.sql — 修正 058 的唯一索引形状
-- 058 的 (legal_entity_id, idempotency_key) 每行唯一与「一批草稿共享同一
-- 幂等键」冲突：批量落库时第一批第二条即撞 23505。批次幂等应作用于行组合
-- (legal_entity_id, idempotency_key, assumption_key)——同一键的重放撞同
-- 一批的任一 assumption_key 即被识别为既有批次（重放返回既有，不再落第二条）。

DROP INDEX IF EXISTS ux_fpna_assumption_versions_idempotency;
CREATE UNIQUE INDEX IF NOT EXISTS ux_fpna_assumption_versions_idempotency
    ON fpna_assumption_versions(legal_entity_id, idempotency_key, assumption_key)
    WHERE idempotency_key IS NOT NULL;
