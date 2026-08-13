import type { RetailScenarioAssumptions } from "./api";

export interface RetailAIContext {
  page: "operating-pulse" | "store-360" | "scenario-workbench";
  title: string;
  asOf?: string;
  windowDays?: number;
  classification?: "production" | "simulated";
  datasetVersion?: string;
  sourceSystem?: string;
  storeID?: string;
  storeIDs?: string[];
  horizonMonths?: number;
  assumptions?: Partial<RetailScenarioAssumptions>;
}

const assumptionKeys = [
  "revenue_change_pct", "gross_margin_rate_change_pp", "labor_cost_change_pct", "fixed_rent_change_pct",
  "variable_rent_rate_change_pp", "non_lease_cost_change_pct", "other_controllable_cost_change_pct",
] as const;

/** Build a same-site AI Chat link from server-owned retail query context. */
export function retailAIHref(context: RetailAIContext): string {
  const params = new URLSearchParams();
  params.set("page", context.page);
  params.set("title", context.title);
  if (context.asOf) params.set("as_of", context.asOf);
  if (context.windowDays) params.set("window_days", String(context.windowDays));
  if (context.classification) params.set("classification", context.classification);
  if (context.datasetVersion) params.set("dataset_version", context.datasetVersion);
  if (context.sourceSystem) params.set("source_system", context.sourceSystem);
  if (context.storeID) params.set("store_id", context.storeID);
  for (const storeID of context.storeIDs || []) params.append("store_ids", storeID);
  if (context.horizonMonths) params.set("horizon_months", String(context.horizonMonths));
  for (const key of assumptionKeys) {
    const value = context.assumptions?.[key];
    if (typeof value === "number" && Number.isFinite(value)) params.set(key, String(value));
  }
  return `/ai-chat?${params.toString()}`;
}

/** Only server-created same-site paths may become clickable AI evidence. */
export function safeInternalAIURL(value?: string): string | undefined {
  if (!value || !value.startsWith("/") || value.startsWith("//") || /^\/\s*javascript:/i.test(value)) return undefined;
  const path = value.split("?", 1)[0];
  const allowedPage = path === "/operating-pulse" || path === "/store-360" || path === "/scenario-workbench";
  const allowedKPI = path.startsWith("/api/v1/retail/kpis/");
  if (!allowedPage && !allowedKPI) return undefined;
  return value;
}
