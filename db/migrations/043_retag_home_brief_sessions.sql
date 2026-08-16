-- DATA-001: retag legacy home-brief sessions as system-initiated.
--
-- CHAT-001 (042) made NEW auto-runs carry initiator='system', but the
-- sessions created before that migration still say 'user' and crowd the
-- user-facing AI Chat sidebar. This migration retags exactly those.
--
-- 判据 (matches the backend's summarizeTitle): the session title is the
-- first-20-runes prefix of home.brief_prompt in one of the three languages,
-- plus "..." — and the session context is the home page. The prefix values
-- below are generated from web/app/lib/i18n.ts home.brief_prompt, NOT
-- hand-copied prose: if the copy changes, regenerate them the same way.
--   zh-CN: 请读取当前经营脉搏并生成今日经营简报：总...
--   zh-HK: 請讀取當前經營脈搏並生成今日經營簡報：總...
--   en:    Read the current ope...
--
-- A real user question like「哪些门店需要关注？」(9 runes) matches none of
-- these prefixes, so it stays user-visible. The dry-run (SELECT before the
-- UPDATE) counted 48 sessions (47 zh-CN + 1 en); the user question was NOT
-- among them.
--
-- 回滚: the affected session ids were copied to data_001_retag_backup
-- before the UPDATE; to restore, run:
--   UPDATE ai_chat_sessions s SET initiator='user'
--   FROM data_001_retag_backup b WHERE s.id = b.session_id;

-- 1) snapshot the affected ids for rollback (idempotent: only inserts rows
--    that are not already backed up)
CREATE TEMP TABLE _brief_sessions AS
SELECT id
FROM ai_chat_sessions
WHERE initiator = 'user'
  AND context_snapshot->>'page' = 'home'
  AND (
    title LIKE '请读取当前经营脉搏并生成今日经营简报：总%'
    OR title LIKE '請讀取當前經營脈搏並生成今日經營簡報：總%'
    OR title LIKE 'Read the current ope%'
  );

CREATE TABLE IF NOT EXISTS data_001_retag_backup (session_id UUID PRIMARY KEY);
INSERT INTO data_001_retag_backup (session_id)
SELECT id FROM _brief_sessions
ON CONFLICT (session_id) DO NOTHING;

-- 2) retag
UPDATE ai_chat_sessions
SET initiator = 'system'
WHERE id IN (SELECT id FROM _brief_sessions);
