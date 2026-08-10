-- Migration 026: persist evidence linkage on event drafts.
-- AI-created events remain draft/pending; the locator is retained for review
-- and audit replay and is never used to bypass the event approval workflow.

ALTER TABLE lease_events
    ADD COLUMN IF NOT EXISTS source_reference_locator JSONB;

CREATE INDEX IF NOT EXISTS idx_lease_events_source_reference
    ON lease_events USING GIN (source_reference_locator)
    WHERE source_reference_locator IS NOT NULL;
