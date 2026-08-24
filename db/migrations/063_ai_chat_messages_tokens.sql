-- 063_ai_chat_messages_tokens.sql (AR3 Context Assembler · 接线批次)
--
-- ai_chat_messages 增加 measured_tokens 列：assistant 消息回填当轮 provider
-- 实测 usage（llm.ParseUsage 的 InputTokens = prompt_tokens）。列语义注意：
-- 存的是【该轮 prompt 总量】，不是本条消息自己的 token 数 —— 每轮重发全量
-- prompt，每条增量无法定义，摊法是编的（假精度，AF1-a）。读侧按基线语义
-- 消费：最近一轮的实测值是截至该轮全部历史的精确真值基线，只有基线之后的
-- 未发送尾部才走 chars/4 兜底估算（contextassembler.measuredBaselineIndex）。
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
