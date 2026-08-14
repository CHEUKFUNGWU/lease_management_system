import { describe, expect, it } from "vitest";
import type { RetailScenarioResponse } from "../lib/api";
import { acceptsEvaluation, actionKey, bridgeConservation, canSaveScenario, defaultAssumptions, evaluationSnapshotKey, formatScenarioValue, responseHorizonLabel, returnScenarioQuery, scenarioQueryKey, scenarioRequest } from "./logic";

const response = (): RetailScenarioResponse => ({
  basis: "Scenario", scenario_version: "retail-store-scenario-v1", formula_version: "retail-kpi-v1", diagnostics_version: "retail-store-diagnostics-v1",
  side_effects: false, review_required: true, official_impact: false, ifrs16_impact: false, generated_at: "2026-06-05T00:00:00Z",
  store: { store_id: "s1", store_code: "S001", store_name: "店一", brand: "品牌", region: "华东" }, data_classification: "simulated", dataset_version: "planA-v1", source_system: "retail_simulator", currency: "CNY",
  current: { date_from: "2026-05-23", date_to: "2026-06-05" }, horizon_months: 12,
  baseline: { key: "baseline", name: "Baseline", assumptions: defaultAssumptions(), metrics: {}, monthly_contribution_change: 0, horizon_contribution_change: 0, bridge: { items: [], total_change: 0, rounding_residual: 0, status: "complete" } },
  scenarios: [{ key: "plan", name: "Plan", assumptions: defaultAssumptions(), metrics: {}, monthly_contribution_change: 100, horizon_contribution_change: 1200, bridge: { items: [{ code: "gross_profit", label: "毛利", contribution: 100, unit: "currency" }], total_change: 100, rounding_residual: 0, status: "complete" } }],
  evidence: { current: { date_from: "2026-05-23", date_to: "2026-06-05" }, observed_store_days: 14, expected_store_days: 14, coverage_rate: 100, required_fields: [], source_systems: ["retail_simulator"], dataset_versions: ["planA-v1"], fact_version_min: 1, fact_version_max: 1, kpi_drilldown_url: "", request_assumptions: {} },
});

describe("scenario workbench pure logic", () => {
  it("round-trips scope and keeps dataset/source explicit", () => {
    const query = { store_id: "s/1", data_classification: "simulated" as const, dataset_version: "plan A/v1", as_of: "2026-06-05", window_days: 28 as const, source_system: "retail_simulator" };
    expect(scenarioQueryKey(query)).toBe("s/1|simulated|plan A/v1|2026-06-05|28|retail_simulator");
    const encoded = scenarioRequest(query);
    expect(encoded.get("dataset_version")).toBe("plan A/v1");
    expect(encoded.get("window_days")).toBe("28");
    expect(returnScenarioQuery(new URLSearchParams("store_id=s1&data_classification=simulated&dataset_version=v1&as_of=2026-06-05&window_days=14"))).toContain("/store-360?");
  });

  it("formats null and scenario units without inventing zero", () => {
    expect(formatScenarioValue(null, "currency", "CNY", "zh-CN")).toBe("—");
    expect(formatScenarioValue(12.3456, "percent", "CNY", "zh-CN")).toBe("12.3456%");
    expect(formatScenarioValue(1234.5, "currency", "CNY", "zh-CN")).toBe("1,234.50 CNY");
  });

  it("checks bridge conservation including rounding residual", () => {
    expect(bridgeConservation(response().scenarios[0].bridge)).toBe(0);
  });

  it("creates stable but payload-sensitive action keys", () => {
    expect(actionKey({ store: "s1", plan: 1 })).toBe(actionKey({ store: "s1", plan: 1 }));
    expect(actionKey({ store: "s1", plan: 1 })).not.toBe(actionKey({ store: "s1", plan: 2 }));
  });

  it("reuses the retry key only for the exact confirmed action payload", () => {
    const confirmed = { evaluation: "snapshot-1", title: "T", planned_action: "A", owner_name: "O", due_date: null };
    expect(actionKey(confirmed)).toBe(actionKey({ ...confirmed }));
    expect(actionKey(confirmed)).not.toBe(actionKey({ ...confirmed, planned_action: "B" }));
  });

  it("invalidates evaluation snapshots when scope, horizon, or assumptions change", () => {
    const scope = { store_id: "s1", data_classification: "simulated" as const, dataset_version: "v1", as_of: "2026-06-05", window_days: 14 as const };
    const original = evaluationSnapshotKey(scope, 12, defaultAssumptions());
    expect(evaluationSnapshotKey(scope, 12, defaultAssumptions())).toBe(original);
    expect(evaluationSnapshotKey(scope, 3, defaultAssumptions())).not.toBe(original);
    expect(evaluationSnapshotKey(scope, 12, { ...defaultAssumptions(), labor_cost_change_pct: -10 })).not.toBe(original);
    expect(acceptsEvaluation(original, original, 2, 2)).toBe(true);
    expect(acceptsEvaluation(original, original, 1, 2)).toBe(false);
    expect(acceptsEvaluation(original, "new", 2, 2)).toBe(false);
  });

  it("requires a fresh response before save", () => {
    const value = response();
    expect(canSaveScenario(value, "plan", "same", "same")).toBe(true);
    expect(canSaveScenario(value, "plan", "old", "new")).toBe(false);
  });

  it("labels an expired result with the response horizon, not current controls", () => {
    const value = response();
    expect(responseHorizonLabel(value, "zh-CN")).toBe("12个月");
  });
});
