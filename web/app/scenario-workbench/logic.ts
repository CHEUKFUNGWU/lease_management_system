import { t, type Language } from "../lib/i18n";
import type { RetailScenarioAssumptions, RetailScenarioBridge, RetailScenarioInput, RetailScenarioResponse } from "../lib/api";

export const SCENARIO_CODES = [
  "revenue", "gross_profit", "gross_margin_rate", "labor_cost", "fixed_rent",
  "variable_rent_rate", "variable_rent", "non_lease_cost", "other_controllable_cost",
  "occupancy_cash_cost", "store_contribution", "store_contribution_margin",
] as const;

const SCENARIO_LABEL_KEYS: Record<string, string> = {
  revenue: "retail.kpi.revenue", gross_profit: "retail.kpi.gross_profit", gross_margin_rate: "retail.kpi.gross_margin_rate", labor_cost: "retail.kpi.labor_cost",
  fixed_rent: "retail.kpi.fixed_rent", variable_rent_rate: "retail.kpi.variable_rent_rate", variable_rent: "retail.kpi.variable_rent",
  non_lease_cost: "retail.kpi.non_lease_cost", other_controllable_cost: "retail.kpi.other_controllable_cost",
  occupancy_cash_cost: "retail.kpi.occupancy_cash_cost", store_contribution: "retail.kpi.store_contribution", store_contribution_margin: "retail.kpi.store_contribution_margin",
};

export function scenarioLabel(code: string, language: Language): string {
  const key = SCENARIO_LABEL_KEYS[code];
  return key ? t(key, language) : code;
}

export const defaultAssumptions = (): RetailScenarioAssumptions => ({
  revenue_change_pct: 0, gross_margin_rate_change_pp: 0, labor_cost_change_pct: 0,
  fixed_rent_change_pct: 0, variable_rent_rate_change_pp: 0,
  non_lease_cost_change_pct: 0, other_controllable_cost_change_pct: 0,
});

export function scenarioQueryKey(params: { store_id: string; data_classification: string; dataset_version?: string; as_of: string; window_days: number; source_system?: string }): string {
  return [params.store_id, params.data_classification, params.dataset_version || "", params.as_of, params.window_days, params.source_system || ""].join("|");
}

export function scenarioRequest(params: { store_id: string; data_classification: "production" | "simulated"; dataset_version?: string; as_of: string; window_days: number; // M2: custom rolling windows, 7-28
  source_system?: string }): URLSearchParams {
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
  window_days: number; // M2: custom rolling windows, 7-28
  source_system?: string;
};

export function evaluationSnapshotKey(scope: ScenarioEvaluationScope, horizonMonths: number, assumptions: RetailScenarioAssumptions): string {
  return JSON.stringify({ scope: scenarioQueryKey(scope), horizon_months: horizonMonths, assumptions: { ...defaultAssumptions(), ...assumptions } });
}

export function acceptsEvaluation(responseKey: string | null, currentKey: string, requestSequence: number, latestSequence: number): boolean {
  return responseKey === currentKey && requestSequence === latestSequence;
}

export function formatScenarioValue(value: number | null | undefined, unit: string, currency: string, language: Language): string {
  if (value == null) return "—";
  const formatted = value.toLocaleString("zh-CN", { minimumFractionDigits: unit === "percent" ? 4 : 2, maximumFractionDigits: unit === "percent" ? 4 : 2 });
  if (unit === "percent") return `${formatted}%`;
  if (unit === "currency") return `${formatted} ${currency || t("retail.unit.currency", language)}`;
  return `${formatted} ${unit}`;
}

export function responseHorizonLabel(response: Pick<RetailScenarioResponse, "horizon_months">, language: Language): string {
  return `${response.horizon_months}${t("retail.months", language)}`;
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
