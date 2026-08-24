-- 063_ai_chat_messages_tokens.sql (AR3 Context Assembler · 接线批次)
--
-- ai_chat_messages 增加 measured_tokens 列：assistant 消息回填当轮 provider
-- 实测 usage（llm.ParseUsage 的 InputTokens = prompt_tokens）。这是 AR3 双轨
-- 计数的主真值源（contextassembler.Message.MeasuredTokens 的持久层落点）：
-- 已发送过的消息用实测值计数，只有从未发送过的尾部消息才走 chars/4 兜底估算。
--
-- 0 是「尚未测量」的哨兵值，不是测得的零 —— 与 llm.UsageMetadata 的 refusal
-- to invent 一致：provider 未回传 usage 时该列保持默认 0，绝不编造。
--
-- 存量历史消息（迁移前发送的）一律为 0：它们没有实测真值，宁可让估算器
-- 多算一次，也不回填一个猜的数字。
--
-- 与 db/init/01_init.sql 的空库版本必须同时提供且等价（一致性测试照 062
-- 先例：internal/repository 迁移一致性测试；教训 27ccdd2/32aac80）。

ALTER TABLE ai_chat_messages
    ADD COLUMN IF NOT EXISTS measured_tokens INTEGER NOT NULL DEFAULT 0;
