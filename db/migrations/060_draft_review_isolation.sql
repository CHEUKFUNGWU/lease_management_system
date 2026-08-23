-- 060_draft_review_isolation.sql (Ch2 草稿复核工作台 · 底线 1/底线 2)
--
-- ai_contract_drafts 原先没有法人列：agent_draft_batches 按 created_by 过滤，
-- 是操作者级隔离；Reviewer 复核 Editor 的草稿时一条也看不到。
--
-- legal_entity_id 可空而不是 NOT NULL：存量行没有可靠的法人来源，
-- contract_data 是 AI 抽取结果，不能拿它当隔离键（AI 抽错即越权）。
-- 因此隔离是 fail-closed：列表端点要求 legal_entity_id 与调用者 scope 相等，
-- NULL 不匹配任何人——历史行对所有账号不可见，而不是对所有人可见。

ALTER TABLE ai_contract_drafts
    ADD COLUMN IF NOT EXISTS legal_entity_id UUID REFERENCES legal_entities(id);

ALTER TABLE ai_contract_drafts
    ADD COLUMN IF NOT EXISTS data_classification VARCHAR(20);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'ai_contract_drafts_classification_check'
    ) THEN
        ALTER TABLE ai_contract_drafts
            ADD CONSTRAINT ai_contract_drafts_classification_check
            CHECK (data_classification IN ('production', 'simulated', 'mixed'));
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_ai_contract_drafts_legal_entity
    ON ai_contract_drafts(legal_entity_id);
