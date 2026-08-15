import { hasRole, type User } from "../context/AuthContext";
import type { RetailAttention, RetailPulseResponse } from "../lib/api";
import type { HomeBriefPlanStep, HomeBriefResult } from "./types";

/**
 * HOME-002 pure contracts: role branching (B3), brief states (B4) and the
 * structured pieces rendered from the agent response. All business wording
 * lives in i18n keys; this module only decides which state is shown.
 */

// §1.2: editor / reviewer / approver do not have analysis permission and
// must not get a brief they cannot act on. The branch mirrors the nav rule
// in AppLayout.tsx (canViewAnalysis).
export function canViewHomeBrief(user: User | null | undefined): boolean {
  if (!user) return false;
  return hasRole(user, "admin") || hasRole(user, "readonly") || hasRole(user, "auditor");
}

export type HomeBriefState =
  | "loading"
  | "ready"
  | "no_data"
  | "not_decision_ready"
  | "needs_input"
  | "scope_denied"
  | "error";

/**
 * B4: the three degraded states must be distinguishable and scope_denied
 * must never be softened into "no data" — the reason field wins over any
 * coverage signal (AGENTS.md: 权限拒绝必须保持原因).
 */
export function classifyHomeBrief(result: HomeBriefResult | null, error: string | null): HomeBriefState {
  if (error) return "error";
  if (!result) return "loading";
  const operations = result.retail_operations;
  if (operations?.reason === "scope_denied") return "scope_denied";
  if (operations?.needs_input) return "needs_input";
  const pulse = operations?.pulse;
  if (!pulse) return "needs_input";
  if (!pulse.decision_ready) {
    const observed = (pulse.current_coverage?.observed_store_days ?? 0) + (pulse.comparison_coverage?.observed_store_days ?? 0);
    return observed === 0 ? "no_data" : "not_decision_ready";
  }
  return "ready";
}

export interface BriefAttentionCard {
  rank: number;
  store_id: string;
  store_code: string;
  store_name: string;
  severity: string;
  currency?: string;
  signals: RetailAttention["observed_signals"];
}

export function briefAttentionCards(pulse: RetailPulseResponse | null | undefined): BriefAttentionCard[] {
  if (!pulse) return [];
  return (pulse.attention || []).map((item) => ({
    rank: item.rank,
    store_id: item.store_id,
    store_code: item.store_code,
    store_name: item.store_name,
    severity: item.severity,
    currency: item.currency,
    signals: item.observed_signals || [],
  }));
}

/** The agent plan becomes the collapsed trace text (<ThinkingTrace> input). */
export function planToThinking(plan: HomeBriefPlanStep[] | undefined): string {
  if (!plan || plan.length === 0) return "";
  return plan
    .map((step) => {
      const mark = step.status === "completed" ? "✓" : step.status === "failed" ? "✗" : "…";
      return `${mark} ${step.title}`;
    })
    .join("\n");
}

/**
 * The filters the home auto-run sends as page context. Simulated runs need
 * a dataset version; production runs must not carry one (backend contract).
 */
export function buildBriefFilters(
  classification: "production" | "simulated",
  datasetVersion: string | undefined,
  asOf: string,
  windowDays: number,
  sourceSystem?: string,
): Record<string, string> {
  const filters: Record<string, string> = {
    as_of: asOf,
    window_days: String(windowDays),
    data_classification: classification,
  };
  if (classification === "simulated" && datasetVersion) filters.dataset_version = datasetVersion;
  if (sourceSystem) filters.source_system = sourceSystem;
  return filters;
}
