-- Backfill a demonstrable Actual-vs-Budget basis for every completed retail
-- simulation dataset. The plan is explicitly simulated, stays Draft and is
-- never Official. Fresh simulations create the same shape transactionally in
-- RetailSimulationRepository.Generate.

INSERT INTO fpna_plan_versions (
    legal_entity_id, name, version_type, scenario_type, source, coverage_scope,
    currency, as_of_period, from_period, to_period, status, is_official, created_by
)
SELECT
    d.legal_entity_id,
    '模拟预算 · ' || d.dataset_version,
    'budget',
    'baseline',
    'retail_simulator_budget',
    jsonb_build_object(
        'data_classification', 'simulated',
        'simulation_dataset_version', d.dataset_version,
        'grain', 'store_month'
    ),
    'CNY',
    to_char(d.date_from, 'YYYY-MM'),
    to_char(d.date_from, 'YYYY-MM'),
    to_char(d.date_to, 'YYYY-MM'),
    'draft',
    false,
    d.created_by
FROM retail_simulation_datasets d
WHERE d.status = 'completed'
ON CONFLICT (legal_entity_id, name, as_of_period) DO NOTHING;

WITH actual AS (
    SELECT
        d.dataset_version,
        d.legal_entity_id,
        s.id AS store_id,
        s.code AS store_code,
        s.brand,
        s.region,
        f.currency,
        to_char(f.business_date, 'YYYY-MM') AS period,
        SUM(f.revenue) AS revenue,
        SUM(f.gross_profit) AS gross_profit,
        SUM(f.labor_cost) AS labor_cost,
        SUM(f.fixed_rent) AS fixed_rent,
        SUM(f.variable_rent) AS variable_rent,
        SUM(f.non_lease_cost) AS non_lease_cost,
        SUM(f.other_controllable_cost) AS other_cost,
        0.97 + (abs(hashtextextended(s.code, 0)) % 7)::numeric / 100 AS revenue_factor,
        0.98 + (abs(hashtextextended(s.code, 1)) % 5)::numeric / 100 AS cost_factor,
        0.96 + (abs(hashtextextended(s.code, 2)) % 9)::numeric / 100 AS gross_factor
    FROM retail_simulation_datasets d
    JOIN stores s
      ON s.legal_entity_id = d.legal_entity_id
     AND s.data_classification = 'simulated'
     AND s.simulation_dataset_version = d.dataset_version
    JOIN retail_store_day_facts f
      ON f.store_id = s.id
     AND f.data_classification = 'simulated'
     AND f.simulation_dataset_version = d.dataset_version
    WHERE d.status = 'completed'
    GROUP BY d.dataset_version, d.legal_entity_id, s.id, s.code, s.brand, s.region,
             f.currency, to_char(f.business_date, 'YYYY-MM')
), versioned AS (
    SELECT a.*, v.id AS version_id
    FROM actual a
    JOIN fpna_plan_versions v
      ON v.legal_entity_id = a.legal_entity_id
     AND v.name = '模拟预算 · ' || a.dataset_version
     AND v.as_of_period = (
         SELECT to_char(d.date_from, 'YYYY-MM')
         FROM retail_simulation_datasets d
         WHERE d.legal_entity_id = a.legal_entity_id
           AND d.dataset_version = a.dataset_version
     )
)
INSERT INTO fpna_plan_lines (
    plan_version_id, period, grain, legal_entity_id, brand, region, store_id,
    currency, revenue, gross_profit, labor_cost, fixed_rent, variable_rent,
    non_lease_cost, four_wall_ebitda, operational_kpis, source_system,
    source_record_id, as_of_at, actual_flag, forecast_flag, scenario_inputs
)
SELECT
    version_id,
    period,
    'store',
    legal_entity_id,
    brand,
    region,
    store_id,
    currency,
    revenue * revenue_factor,
    gross_profit * gross_factor,
    labor_cost * cost_factor,
    fixed_rent,
    variable_rent * revenue_factor,
    non_lease_cost * cost_factor,
    gross_profit * gross_factor - labor_cost * cost_factor - fixed_rent
        - variable_rent * revenue_factor - non_lease_cost * cost_factor
        - other_cost * cost_factor,
    jsonb_build_object(
        'other_controllable_cost', other_cost * cost_factor,
        'data_classification', 'simulated',
        'simulation_dataset_version', dataset_version
    ),
    'retail_simulator_budget',
    dataset_version || ':' || store_code || ':' || period,
    now(),
    false,
    false,
    jsonb_build_object(
        'data_classification', 'simulated',
        'simulation_dataset_version', dataset_version
    )
FROM versioned v
WHERE NOT EXISTS (
    SELECT 1
    FROM fpna_plan_lines existing
    WHERE existing.plan_version_id = v.version_id
      AND existing.period = v.period
      AND existing.grain = 'store'
      AND existing.legal_entity_id = v.legal_entity_id
      AND existing.brand = v.brand
      AND existing.region = v.region
      AND existing.store_id = v.store_id
      AND existing.currency = v.currency
);
