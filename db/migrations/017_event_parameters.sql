-- Lease events recorded a change as a single free-text value (new_value), which
-- the system could not read. Anything beyond "the new rent is X" — a CPI clause,
-- a capped escalation, a stepped ladder, a rent-free window — had to be applied
-- to the payment schedule by hand before the event was recorded, with no check
-- that the two agreed.
--
-- revision_parameters carries the clause as the landlord's notice states it, so
-- the revised payment schedule can be derived rather than retyped. new_value is
-- kept as-is: existing events were recorded that way and must keep calculating
-- exactly as they did.

ALTER TABLE lease_events
    ADD COLUMN IF NOT EXISTS revision_parameters JSONB;

COMMENT ON COLUMN lease_events.revision_parameters IS
    '结构化条款参数(定额/固定比例/CPI 指数含上下限/阶梯),用于推导修订付款流;为空时回落到 new_value 的旧口径';

-- Only events that carry parameters are ever looked up by them, so the index
-- skips the rows that do not.
CREATE INDEX IF NOT EXISTS idx_lease_events_revision_parameters
    ON lease_events USING GIN (revision_parameters)
    WHERE revision_parameters IS NOT NULL;
