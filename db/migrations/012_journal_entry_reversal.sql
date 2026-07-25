-- +goose Up
-- +goose StatementBegin

-- Journal entry reversal.
--
-- Correcting a posted entry must never require unlocking a period: the ledger
-- stays immutable and the correction is expressed as a second, opposite entry
-- that points back at the original. These columns carry that link and the
-- reversal's provenance.
--
-- The link lives on the reversing entry (reversal_of_entry_id, reversal_reason);
-- the original records only who reversed it and when, alongside its
-- posting_status = 'reversed'.

ALTER TABLE journal_entries
    ADD COLUMN IF NOT EXISTS reversal_of_entry_id UUID REFERENCES journal_entries(id),
    ADD COLUMN IF NOT EXISTS reversal_reason TEXT,
    ADD COLUMN IF NOT EXISTS reversed_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS reversed_by UUID REFERENCES users(id);

-- An entry may be reversed at most once. Enforcing this in the schema means a
-- concurrent double reversal fails at the database rather than double-crediting
-- the ledger.
CREATE UNIQUE INDEX IF NOT EXISTS idx_journal_entries_reversal_of
    ON journal_entries(reversal_of_entry_id)
    WHERE reversal_of_entry_id IS NOT NULL;

-- Reversal is a distinct authority from posting: an organization may allow a
-- role to post the close yet require a different role to undo a posted entry.
INSERT INTO permissions (role_id, resource, action) VALUES
    ('44444444-4444-4444-4444-444444444444', 'monthly_closing', 'reverse')
ON CONFLICT (role_id, resource, action) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM permissions
 WHERE role_id = '44444444-4444-4444-4444-444444444444'
   AND resource = 'monthly_closing' AND action = 'reverse';
DROP INDEX IF EXISTS idx_journal_entries_reversal_of;
ALTER TABLE journal_entries
    DROP COLUMN IF EXISTS reversed_by,
    DROP COLUMN IF EXISTS reversed_at,
    DROP COLUMN IF EXISTS reversal_reason,
    DROP COLUMN IF EXISTS reversal_of_entry_id;
-- +goose StatementEnd
