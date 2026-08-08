-- Remove the deployment-time discount-rate seed. A rate is a controlled
-- accounting policy input, not a safe universal default.
DELETE FROM system_settings
WHERE setting_key = 'global_discount_rate'
  AND setting_value = '0.05'
  AND updated_by IS NULL;

ALTER TABLE store_metrics
    ALTER COLUMN period_basis DROP DEFAULT,
    ALTER COLUMN currency DROP DEFAULT,
    ALTER COLUMN source DROP DEFAULT;

ALTER TABLE legal_entities
    ALTER COLUMN currency DROP DEFAULT;

ALTER TABLE lease_contracts
    ALTER COLUMN currency DROP DEFAULT;

ALTER TABLE lease_payment_schedules
    ALTER COLUMN currency DROP DEFAULT;

ALTER TABLE journal_entries
    ALTER COLUMN currency DROP DEFAULT;

ALTER TABLE budget_lines
    ALTER COLUMN currency DROP DEFAULT;

ALTER TABLE budget_versions
    ALTER COLUMN source DROP DEFAULT;

ALTER TABLE lease_contracts
    ALTER COLUMN asset_type DROP DEFAULT,
    ALTER COLUMN lease_scope DROP DEFAULT;
