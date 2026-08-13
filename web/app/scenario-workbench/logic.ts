import type { RetailScenarioAssumptions, RetailScenarioBridge, RetailScenarioInput, RetailScenarioResponse } from "../lib/api";

export const SCENARIO_CODES = [
  "revenue", "gross_profit", "gross_margin_rate", "labor_cost", "fixed_rent",
  "variable_rent_rate", "variable_rent", "non_lease_cost", "other_controllable_cost",
  "occupancy_cash_cost", "store_contribution", "store_contribution_margin",
] as const;

export const SCENARIO_LABELS: Record<string, string> = {
  revenue: "销售额", gross_profit: "毛利额", gross_margin_rate: "毛利率", labor_cost: "人工成本",
  fixed_rent: "固定现金租金", variable_rent_rate: "变动租金率", variable_rent: "变动租金",
  non_lease_cost: "非租赁占用成本", other_controllable_cost: "其他可控成本",
  occupancy_cash_cost: "经营占用现金成本", store_contribution: "门店贡献额", store_contribution_margin: "门店贡献率",
};

export const defaultAssumptions = (): RetailScenarioAssumptions => ({
  revenue_change_pct: 0, gross_margin_rate_change_pp: 0, labor_cost_change_pct: 0,
  fixed_rent_change_pct: 0, variable_rent_rate_change_pp: 0,
  non_lease_cost_change_pct: 0, other_controllable_cost_change_pct: 0,
});

export function scenarioQueryKey(params: { store_id: string; data_classification: string; dataset_version?: string; as_of: string; window_days: number; source_system?: string }): string {
  return [params.store_id, params.data_classification, params.dataset_version || "", params.as_of, params.window_days, params.source_system || ""].join("|");
}

export function scenarioRequest(params: { store_id: string; data_classification: "production" | "simulated"; dataset_version?: string; as_of: string; window_days: 7 | 14 | 28; source_system?: string }): URLSearchParams {
  const query = new URLSearchParams({ data_classification: params.data_classification, as_of: params.as_of, window_days: String(params.window_days) });
  if (params.dataset_version) query.set("dataset_version", params.dataset_version);
  if (params.source_system) query.set("source_system", params.source_system);
  return query;
}

export function scenarioInput(key: string, assumptions: RetailScenarioAssumptions): RetailScenarioInput {
  return { key, name: key === "baseline" ? "Baseline" : "Plan", assumptions };
}

export type ScenarioEvaluationScope = {
  store_id: string;
  data_classification: "production" | "simulated";
  dataset_version?: string;
  as_of: string;
  window_days: 7 | 14 | 28;
  source_system?: string;
};

export function evaluationSnapshotKey(scope: ScenarioEvaluationScope, horizonMonths: number, assumptions: RetailScenarioAssumptions): string {
  return JSON.stringify({ scope: scenarioQueryKey(scope), horizon_months: horizonMonths, assumptions: { ...defaultAssumptions(), ...assumptions } });
}

export function acceptsEvaluation(responseKey: string | null, currentKey: string, requestSequence: number, latestSequence: number): boolean {
  return responseKey === currentKey && requestSequence === latestSequence;
}

export function formatScenarioValue(value: number | null | undefined, unit: string, currency: string): string {
  if (value == null) return "—";
  const formatted = value.toLocaleString("zh-CN", { minimumFractionDigits: unit === "percent" ? 4 : 2, maximumFractionDigits: unit === "percent" ? 4 : 2 });
  if (unit === "percent") return `${formatted}%`;
  if (unit === "currency") return `${formatted} ${currency || "金额"}`;
  return `${formatted} ${unit}`;
}

export function responseHorizonLabel(response: Pick<RetailScenarioResponse, "horizon_months">): string {
  return `${response.horizon_months}个月`;
}

export function bridgeConservation(bridge: RetailScenarioBridge): number | null {
  if (bridge.total_change == null) return null;
  const sum = bridge.items.reduce((total, item) => total + (item.contribution || 0), 0);
  return sum + (bridge.rounding_residual || 0) - bridge.total_change;
}

export function actionKey(payload: unknown): string {
  const text = JSON.stringify(payload);
  let hash = 2166136261;
  for (let i = 0; i < text.length; i += 1) hash = Math.imul(hash ^ text.charCodeAt(i), 16777619);
  return `retail-scenario-${(hash >>> 0).toString(16)}`;
}

export function canSaveScenario(response: RetailScenarioResponse | null, selectedKey: string, responseKey?: string | null, currentKey?: string): boolean {
  return Boolean(response && response.scenarios.some((scenario) => scenario.key === selectedKey) && response.review_required && !response.official_impact && !response.ifrs16_impact && (responseKey === undefined || responseKey === currentKey));
}

export function selectedScenario(response: RetailScenarioResponse | null, key: string) {
  return response?.scenarios.find((scenario) => scenario.key === key) || null;
}

export function returnScenarioQuery(params: { get(name: string): string | null }): string {
  const query = new URLSearchParams();
  ["store_id", "data_classification", "dataset_version", "as_of", "window_days", "source_system"].forEach((key) => {
    const value = params.get(key);
    if (value) query.set(key, value);
  });
  return `/store-360${query.toString() ? `?${query.toString()}` : ""}`;
}
