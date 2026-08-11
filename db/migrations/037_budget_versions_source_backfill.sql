-- Repair migration for existing PostgreSQL volumes.
--
-- Migration 020 (and the copy of it in db/init/01_init.sql) adds four columns
-- to budget_versions in a single ALTER, but gives a DEFAULT to only three of
-- them:
--
--     ADD COLUMN IF NOT EXISTS source VARCHAR(255) NOT NULL,
--
-- PostgreSQL cannot add a NOT NULL column without a default to a table that
-- already holds rows, so the whole statement aborts and every object declared
-- after it in migration 020 — variance_actions and renewal_decision_snapshots
-- — is never created. A fresh volume never hits this, because 01_init.sql
-- creates budget_versions with the column already present and no rows in it.
-- Any volume created before 020 that has since recorded a budget version does.
--
-- Add the column with a default first so migration 020 can be re-applied; its
-- ADD COLUMN IF NOT EXISTS then skips this column and the rest of the file
-- proceeds. Existing rows are backfilled with 'legacy' to record that their
-- provenance predates the field rather than asserting a source they never had.

ALTER TABLE budget_versions
    ADD COLUMN IF NOT EXISTS source VARCHAR(255) NOT NULL DEFAULT 'legacy';

-- New rows must state their own provenance; only the backfilled ones may keep
-- the placeholder.
ALTER TABLE budget_versions
    ALTER COLUMN source DROP DEFAULT;
