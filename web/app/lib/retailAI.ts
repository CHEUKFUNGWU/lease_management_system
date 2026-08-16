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

/** Only server-created same-site paths may become clickable AI evidence. */
export function safeInternalAIURL(value?: string): string | undefined {
  if (!value || !value.startsWith("/") || value.startsWith("//") || /^\/\s*javascript:/i.test(value)) return undefined;
  const path = value.split("?", 1)[0];
  const allowedPage = path === "/operating-pulse" || path === "/store-360" || path === "/scenario-workbench";
  const allowedKPI = path.startsWith("/api/v1/retail/kpis/");
  if (!allowedPage && !allowedKPI) return undefined;
  return value;
}
