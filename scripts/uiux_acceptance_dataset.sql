-- UIUX acceptance dataset (24 contracts, two currencies, all scope/status variants).
-- DEVELOPMENT / DEMO ONLY. Run manually against a development database after db/init/01_init.sql.
-- It is deliberately not included in production initialization.

BEGIN;

WITH source_rows AS (
  SELECT
    i,
    'UIUX-ACCEPT-' || lpad(i::text, 2, '0') AS contract_number,
    'UIUX 验收合同 ' || lpad(i::text, 2, '0') AS contract_name,
    CASE WHEN i % 2 = 0 THEN 'b2c3d4e5-f6a7-8901-bcde-f12345678901'::uuid ELSE 'a1b2c3d4-e5f6-7890-abcd-ef1234567890'::uuid END AS legal_entity_id,
    CASE WHEN i % 2 = 0 THEN 'd4e5f6a7-b8c9-0123-def1-234567890123'::uuid ELSE 'c3d4e5f6-a7b8-9012-cdef-123456789012'::uuid END AS store_id,
    CASE WHEN i % 2 = 0 THEN 'f6a7b8c9-d0e1-2345-f123-456789012345'::uuid ELSE 'e5f6a7b8-c9d0-1234-ef12-345678901234'::uuid END AS landlord_id,
    CASE WHEN i % 3 = 0 THEN 'USD' ELSE 'CNY' END AS currency,
    (ARRAY['in_scope', 'short_term_exempt', 'low_value_exempt', 'not_a_lease'])[((i - 1) % 4) + 1] AS lease_scope,
    (ARRAY['real_estate', 'vehicle', 'it_equipment', 'machinery', 'other'])[((i - 1) % 5) + 1] AS asset_type,
    (ARRAY['draft', 'submitted', 'reviewed', 'pending_approval', 'approved', 'rejected'])[((i - 1) % 6) + 1] AS approval_status,
    DATE '2026-01-01' + (i * 2) AS lease_start_date,
    CASE WHEN i <= 4 THEN DATE '2026-09-01' + i ELSE DATE '2028-12-31' END AS lease_end_date,
    25000 + (i * 1250) AS monthly_rent
  FROM generate_series(1, 24) AS g(i)
)
INSERT INTO lease_contracts (
  contract_number, contract_name, legal_entity_id, store_id, landlord_id,
  lessee_name, lessor_name, store_name, asset_type, currency,
  commencement_date, lease_start_date, lease_end_date,
  discount_rate_value, discount_rate_missing, status, approval_status,
  is_official_version, included_in_reporting, report_mode, lease_scope,
  scope_source, created_at, updated_at
)
SELECT
  contract_number, contract_name, legal_entity_id, store_id, landlord_id,
  'UIUX 验收承租方', 'UIUX 验收出租方', 'UIUX 验收门店 ' || lpad(i::text, 2, '0'), asset_type, currency,
  lease_start_date, lease_start_date, lease_end_date,
  CASE WHEN i = 5 THEN NULL ELSE 0.045 + (i * 0.001) END,
  i = 5,
  approval_status, approval_status,
  approval_status = 'approved', true, CASE WHEN approval_status = 'approved' THEN 'official' ELSE 'working' END,
  lease_scope, 'manual', NOW(), NOW()
FROM source_rows
ON CONFLICT (contract_number) DO NOTHING;

INSERT INTO lease_payment_schedules (
  contract_id, effective_start_date, effective_end_date, coverage_start_date,
  coverage_end_date, due_date, payment_timing, amount, currency, amount_type,
  is_fixed, is_variable, is_lease_component, is_non_lease_component,
  included_in_liability_pv
)
SELECT
  c.id, c.lease_start_date, c.lease_end_date, c.lease_start_date,
  c.lease_end_date, c.lease_start_date, 'postpaid',
  25000 + (substring(c.contract_number from '[0-9]+')::integer * 1250), c.currency, 'fixed_rent',
  true, false, c.lease_scope = 'in_scope', c.lease_scope <> 'in_scope', c.lease_scope = 'in_scope'
FROM lease_contracts c
WHERE c.contract_number LIKE 'UIUX-ACCEPT-%'
  AND NOT EXISTS (
    SELECT 1 FROM lease_payment_schedules ps WHERE ps.contract_id = c.id
  );

-- Rebuild only the dedicated payment-shape fixtures on every run. The delete
-- is scoped to three UIUX contracts so repeated runs remain idempotent without
-- touching any non-UIUX contract or payment schedule.
DELETE FROM lease_payment_schedules ps
USING lease_contracts c
WHERE ps.contract_id = c.id
  AND c.contract_number IN ('UIUX-ACCEPT-21', 'UIUX-ACCEPT-22', 'UIUX-ACCEPT-23');

-- Three annual effective segments with a 5% escalation. The first segment
-- starts at the contract commencement date and is the only one in force today.
INSERT INTO lease_payment_schedules (
  contract_id, effective_start_date, effective_end_date, coverage_start_date,
  coverage_end_date, due_date, payment_timing, amount, currency, amount_type,
  is_fixed, is_variable, is_lease_component, is_non_lease_component,
  included_in_liability_pv
)
SELECT
  c.id,
  segment.effective_start,
  segment.effective_end,
  segment.effective_start,
  segment.effective_end,
  segment.effective_start,
  'prepaid',
  segment.amount,
  c.currency,
  'fixed_rent',
  true,
  false,
  true,
  false,
  true
FROM lease_contracts c
CROSS JOIN LATERAL (
  SELECT
    segment_no,
    (c.lease_start_date + make_interval(years => segment_no - 1))::date AS effective_start,
    (c.lease_start_date + make_interval(years => segment_no) - interval '1 day')::date AS effective_end,
    (51250.00 * power(1.05, segment_no - 1))::numeric(18, 2) AS amount
  FROM generate_series(1, 3) AS segment_no
) AS segment
WHERE c.contract_number = 'UIUX-ACCEPT-21';

-- A quarterly instalment: the amount is the real payment for the three-month
-- coverage period and must never be presented as a monthly amount.
INSERT INTO lease_payment_schedules (
  contract_id, effective_start_date, effective_end_date, coverage_start_date,
  coverage_end_date, due_date, payment_timing, amount, currency, amount_type,
  is_fixed, is_variable, is_lease_component, is_non_lease_component,
  included_in_liability_pv
)
SELECT
  c.id,
  q.start_date,
  q.end_date,
  q.start_date,
  q.end_date,
  q.start_date,
  'prepaid',
  24000.00,
  c.currency,
  'fixed_rent',
  true,
  false,
  true,
  false,
  true
FROM lease_contracts c
CROSS JOIN LATERAL (
  SELECT
    date_trunc('quarter', CURRENT_DATE)::date AS start_date,
    (date_trunc('quarter', CURRENT_DATE) + interval '3 months - 1 day')::date AS end_date
) AS q
WHERE c.contract_number = 'UIUX-ACCEPT-22';

-- Payment currency intentionally differs from the contract master currency.
INSERT INTO lease_payment_schedules (
  contract_id, effective_start_date, effective_end_date, coverage_start_date,
  coverage_end_date, due_date, payment_timing, amount, currency, amount_type,
  is_fixed, is_variable, is_lease_component, is_non_lease_component,
  included_in_liability_pv
)
SELECT
  c.id,
  CURRENT_DATE - 15,
  CURRENT_DATE + 15,
  CURRENT_DATE - 15,
  CURRENT_DATE + 15,
  CURRENT_DATE,
  'postpaid',
  4500.00,
  CASE WHEN c.currency = 'USD' THEN 'CNY' ELSE 'USD' END,
  'fixed_rent',
  true,
  false,
  true,
  false,
  true
FROM lease_contracts c
WHERE c.contract_number = 'UIUX-ACCEPT-23';

-- Long-name fixture for narrow-screen identity and ellipsis checks.
UPDATE lease_contracts
SET contract_name = 'UIUX 验收合同 — 华东旗舰综合体长期租赁及配套服务协议',
    updated_at = NOW()
WHERE contract_number = 'UIUX-ACCEPT-24';

INSERT INTO measurement_results (
  contract_id, accounting_period, period_start_date, period_end_date,
  closing_liability, closing_rou_asset, discount_rate, is_calculated, calculated_at
)
SELECT
  c.id, '2026-07', DATE '2026-07-01', DATE '2026-07-31',
  100000 + (substring(c.contract_number from '[0-9]+')::integer * 5000),
  95000 + (substring(c.contract_number from '[0-9]+')::integer * 4500),
  c.discount_rate_value, true, NOW()
FROM lease_contracts c
WHERE c.contract_number LIKE 'UIUX-ACCEPT-%'
  AND c.lease_scope = 'in_scope'
  AND c.discount_rate_missing = false
ON CONFLICT (contract_id, accounting_period) DO NOTHING;

-- A generated entry plus a dedicated future locked period exercises the close
-- control track without updating a real business period.
INSERT INTO measurement_results (
  contract_id, accounting_period, period_start_date, period_end_date,
  closing_liability, closing_rou_asset, discount_rate, is_calculated, calculated_at
)
SELECT
  c.id, '2099-07', DATE '2099-07-01', DATE '2099-07-31',
  205000.00, 198500.00, c.discount_rate_value, true, NOW()
FROM lease_contracts c
WHERE c.contract_number = 'UIUX-ACCEPT-21'
ON CONFLICT (contract_id, accounting_period) DO NOTHING;

INSERT INTO monthly_closing_batches (
  batch_number, accounting_period, legal_entity_id, status,
  total_contracts, processed_contracts, total_entries, posted_entries,
  started_at, completed_at
)
SELECT
  'UIUX-ACCEPT-LOCKED-2099-07', '2099-07', c.legal_entity_id, 'completed',
  1, 1, 1, 1, NOW(), NOW()
FROM lease_contracts c
WHERE c.contract_number = 'UIUX-ACCEPT-21'
ON CONFLICT (batch_number) DO UPDATE SET
  status = EXCLUDED.status,
  total_contracts = EXCLUDED.total_contracts,
  processed_contracts = EXCLUDED.processed_contracts,
  total_entries = EXCLUDED.total_entries,
  posted_entries = EXCLUDED.posted_entries,
  completed_at = EXCLUDED.completed_at,
  updated_at = NOW();

INSERT INTO journal_entries (
  contract_id, measurement_result_id, accounting_period, entry_date,
  entry_type, debit_account, credit_account, amount, currency,
  description, posting_status, posted_at, batch_id
)
SELECT
  c.id, mr.id, '2099-07', DATE '2099-07-31', 'payment',
  '租赁负债', '银行存款', 51250.00, c.currency,
  'UIUX acceptance locked-period entry', 'posted', NOW(), b.id
FROM lease_contracts c
JOIN measurement_results mr ON mr.contract_id = c.id AND mr.accounting_period = '2099-07'
JOIN monthly_closing_batches b ON b.batch_number = 'UIUX-ACCEPT-LOCKED-2099-07'
WHERE c.contract_number = 'UIUX-ACCEPT-21'
  AND NOT EXISTS (
    SELECT 1 FROM journal_entries je
    WHERE je.contract_id = c.id
      AND je.accounting_period = '2099-07'
      AND je.description = 'UIUX acceptance locked-period entry'
  );

INSERT INTO period_locks (
  accounting_period, legal_entity_id, is_locked, locked_at, created_at, updated_at
)
SELECT '2099-07', c.legal_entity_id, true, NOW(), NOW(), NOW()
FROM lease_contracts c
WHERE c.contract_number = 'UIUX-ACCEPT-21'
ON CONFLICT (accounting_period, legal_entity_id) DO UPDATE SET
  is_locked = true,
  locked_at = NOW(),
  unlocked_by = NULL,
  unlocked_at = NULL,
  updated_at = NOW();

INSERT INTO critical_dates (
  contract_id, date_type, target_date, reminder_days, status, title, description, source
)
SELECT c.id, v.date_type, v.target_date, 30, 'open', v.title, 'UIUX 验收关键日期', 'manual'
FROM (VALUES
  ('UIUX-ACCEPT-01', 'lease_expiry', DATE '2026-08-09', '已逾期租约到期'),
  ('UIUX-ACCEPT-02', 'renewal_deadline', DATE '2026-08-25', '近期续租截止'),
  ('UIUX-ACCEPT-03', 'rent_review', DATE '2026-09-15', '近期租金复核'),
  ('UIUX-ACCEPT-04', 'insurance_renewal', DATE '2026-10-01', '近期保险续保')
) AS v(contract_number, date_type, target_date, title)
JOIN lease_contracts c ON c.contract_number = v.contract_number
WHERE NOT EXISTS (
  SELECT 1 FROM critical_dates d WHERE d.contract_id = c.id AND d.title = v.title
);

COMMIT;
