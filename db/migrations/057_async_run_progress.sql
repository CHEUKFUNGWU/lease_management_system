-- 057_async_run_progress.sql — PRD S2-5 异步 Run（可查进度、可取消）
-- fin_model_runs already carries the queued/running/completed/failed/
-- cancelled state machine; an async failure needs a place to say WHY it
-- failed (honest progress, not a bare status flip).

ALTER TABLE fin_model_runs ADD COLUMN IF NOT EXISTS failure_reason TEXT;
