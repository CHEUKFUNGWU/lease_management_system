/**
 * HOME-003 验收测试。
 *
 * P2: 确认前零写入 —— 渲染与采纳之前不触发任何业务表写入；只有显式
 *     调用采纳路径才调用既有 action API。
 * P3: 采纳走既有 action API —— 同一
 *     /retail/stores/:id/scenario-action-drafts 端点、同一
 *     `retail-proposal-<id>` 幂等键，无新增端点。
 */
import { afterEach, describe, expect, it, vi } from "vitest";
import { retailAnalyticsApi } from "../lib/api";
import { adoptHomeProposal, proposalIdempotencyKey, proposalRunKey, toHomeProposalItem } from "./proposals";
import type { HomeBriefResult } from "./types";

vi.mock("../lib/api", () => ({
  retailAnalyticsApi: { saveStoreScenarioAction: vi.fn() },
}));

const saveMock = vi.mocked(retailAnalyticsApi.saveStoreScenarioAction);

afterEach(() => {
  saveMock.mockReset();
});

const proposalPayload = {
  type: "retail_action_proposal",
  status: "proposal",
  title: "门店经营情景行动提议",
  planned_action: "复核 Baseline/Plan 后保存",
  evidence_complete: true,
  data_classification: "simulated",
  dataset_version: "ds-1",
  formula_version: "retail-kpi-v1",
  next_url: "/scenario-workbench?store_id=store-1",
  scenario: {
    basis: "Scenario",
    scenario_version: "retail-scenario-v1",
    formula_version: "retail-kpi-v1",
    diagnostics_version: "retail-store360-v1",
    side_effects: false,
    review_required: true,
    official_impact: false,
    ifrs16_impact: false,
    generated_at: "2026-06-07T00:00:00Z",
    store: { store_id: "store-1", store_code: "SIM-006", store_name: "门店6", brand: "brand-a", region: "region-a" },
    data_classification: "simulated",
    dataset_version: "ds-1",
    source_system: "retail_simulator",
    currency: "CNY",
    current: { date_from: "2026-06-01", date_to: "2026-06-07" },
    horizon_months: 6,
    baseline: { key: "baseline", name: "Baseline" },
    scenarios: [
      { key: "baseline", name: "Baseline", assumptions: {} },
      { key: "labor-10", name: "人工-10%", assumptions: { labor_cost_change_pct: -10 } },
    ],
    evidence: {
      current: { date_from: "2026-06-01", date_to: "2026-06-07" },
      observed_store_days: 7,
      expected_store_days: 7,
      coverage_rate: 100,
      required_fields: [],
      source_systems: ["retail_simulator"],
      dataset_versions: ["ds-1"],
      fact_version_min: 1,
      fact_version_max: 1,
      kpi_drilldown_url: "",
      request_assumptions: {},
    },
  },
};

const briefWithProposal: HomeBriefResult = {
  answer: "情景评估完成",
  run_id: "run-123",
  session_id: "session-1",
  retail_action_proposal: proposalPayload,
};

describe("proposal extraction (P1/P3 helpers)", () => {
  it("extracts an item only when the response carries a proposal and a run id", () => {
    expect(toHomeProposalItem(briefWithProposal)).toEqual({ key: "run-123", response: briefWithProposal });
    expect(toHomeProposalItem({ answer: "no proposal", run_id: "run-2" })).toBeNull();
    expect(toHomeProposalItem({ answer: "no run", retail_action_proposal: proposalPayload })).toBeNull();
  });

  it("uses the run id as the stable key", () => {
    expect(proposalRunKey(briefWithProposal)).toBe("run-123");
    expect(proposalIdempotencyKey({ key: "run-123", response: briefWithProposal })).toBe("retail-proposal-run-123");
  });
});

describe("adoptHomeProposal (P2 zero-write, P3 existing API)", () => {
  it("never touches the action API before an explicit adopt call", () => {
    expect(saveMock).not.toHaveBeenCalled();
    toHomeProposalItem(briefWithProposal);
    proposalIdempotencyKey({ key: "run-123", response: briefWithProposal });
    expect(saveMock).not.toHaveBeenCalled();
  });

  it("adopts through the existing scenario-action-drafts API with the proposal idempotency key", async () => {
    saveMock.mockResolvedValue({ basis: "Working", formal_execution: false, review_required: true, data: {}, idempotent_replay: false });
    const item = toHomeProposalItem(briefWithProposal)!;
    await adoptHomeProposal(item, "token-a");
    expect(saveMock).toHaveBeenCalledTimes(1);
    const [scope, body, key, token] = saveMock.mock.calls[0];
    expect(token).toBe("token-a");
    expect(key).toBe("retail-proposal-run-123");
    expect(scope).toEqual({
      store_id: "store-1",
      data_classification: "simulated",
      dataset_version: "ds-1",
      as_of: "2026-06-07",
      window_days: 7,
      source_system: "retail_simulator",
    });
    expect(body).toMatchObject({
      horizon_months: 6,
      title: "门店经营情景行动提议",
      planned_action: "复核 Baseline/Plan 后保存",
      selected_scenario: { key: "labor-10", name: "人工-10%", assumptions: { labor_cost_change_pct: -10 } },
    });
  });

  it("rejects without calling the API when the scenario is missing", async () => {
    const item = { key: "run-9", response: { answer: "x", run_id: "run-9", retail_action_proposal: { title: "no scenario" } } };
    await expect(adoptHomeProposal(item, "token-a")).rejects.toThrow("missing scenario");
    expect(saveMock).not.toHaveBeenCalled();
  });
});
