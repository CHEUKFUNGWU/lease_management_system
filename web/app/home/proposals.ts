import type { ApprovalProposalLike } from "../components/ApprovalCard";
import { retailAnalyticsApi, type RetailDataClassification, type RetailScenarioResponse } from "../lib/api";
import type { HomeBriefResult } from "./types";

/**
 * HOME-003: agent action proposals settle into the right column. The adopt
 * path mirrors the /ai-chat page's adoptRetailProposal contract exactly —
 * same scenario-action-drafts endpoint, same scope/body payload, same
 * `retail-proposal-<id>` idempotency key — so there is no new backend
 * surface (P3). Until a person confirms, nothing is written (P2).
 */

export interface HomeProposalItem {
  /** Stable per run: dedupe + idempotency key. */
  key: string;
  response: HomeBriefResult;
}

export function proposalRunKey(response: HomeBriefResult): string | undefined {
  return response.run_id || response.session_id || undefined;
}

/** Only responses that actually carry a proposal become items. */
export function toHomeProposalItem(response: HomeBriefResult): HomeProposalItem | null {
  if (!response.retail_action_proposal) return null;
  const key = proposalRunKey(response);
  if (!key) return null;
  return { key, response };
}

export function proposalIdempotencyKey(item: HomeProposalItem): string {
  return `retail-proposal-${item.key}`;
}

export function adoptHomeProposal(item: HomeProposalItem, token: string) {
  const proposal = item.response.retail_action_proposal as ApprovalProposalLike | undefined;
  const scenario = proposal?.scenario as RetailScenarioResponse | undefined;
  if (!token || !proposal || !scenario) return Promise.reject(new Error("missing scenario"));
  const selected = scenario.scenarios.find((option) => option.key !== "baseline") || scenario.scenarios[0];
  if (!selected) return Promise.reject(new Error("missing scenario option"));
  const days = Math.round((new Date(scenario.current.date_to).getTime() - new Date(scenario.current.date_from).getTime()) / 86400000) + 1;
  const windowDays = days === 7 || days === 14 || days === 28 ? days : 7;
  const scope = {
    store_id: scenario.store.store_id,
    data_classification: scenario.data_classification as RetailDataClassification,
    dataset_version: scenario.dataset_version || undefined,
    as_of: scenario.current.date_to,
    window_days: windowDays as 7 | 14 | 28,
    source_system: scenario.source_system || undefined,
  };
  const body = {
    horizon_months: scenario.horizon_months,
    selected_scenario: { key: selected.key, name: selected.name, assumptions: selected.assumptions },
    title: proposal.title || scenario.scenarios[0]?.name || "retail action proposal",
    planned_action: proposal.planned_action || "",
    owner_name: proposal.owner_name || undefined,
    due_date: proposal.due_date || undefined,
    verification_period: proposal.verification_period || undefined,
  };
  return retailAnalyticsApi.saveStoreScenarioAction(scope, body, proposalIdempotencyKey(item), token);
}
