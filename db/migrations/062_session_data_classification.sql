-- 062_session_data_classification.sql (AR2 Session Manager · C1 批次)
--
-- ai_chat_sessions 增加 data_classification 列（底线 2 在会话层的落点）。
-- agentcontext.ContextKey 携带 classification 维度（production/simulated/mixed，
-- D-C20），会话行必须能存它，否则模拟数据语境的会话与正式会话在存储层不可区分。
--
-- 与 db/init/01_init.sql 的空库版本必须同时提供且等价（环境漂移守卫见
-- internal/sessionmanager 的迁移一致性测试；教训 27ccdd2/32aac80）。

ALTER TABLE ai_chat_sessions
    ADD COLUMN IF NOT EXISTS data_classification VARCHAR(20) NOT NULL DEFAULT 'production';

ALTER TABLE ai_chat_sessions
    DROP CONSTRAINT IF EXISTS ai_chat_sessions_classification_check;

ALTER TABLE ai_chat_sessions
    ADD CONSTRAINT ai_chat_sessions_classification_check
    CHECK (data_classification IN ('production', 'simulated', 'mixed'));
