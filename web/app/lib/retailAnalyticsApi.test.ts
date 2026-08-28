import { describe, expect, it, vi, beforeEach } from "vitest";
import { retailAnalyticsApi, storePnlApi } from "./api";

describe("retailAnalyticsApi", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => ({
      ok: true,
      status: 200,
      json: async () => ({ url: String(input), init }),
    })));
  });

  it("uses repeated encoded store_id keys and dataset version for simulated pulse", async () => {
    await retailAnalyticsApi.operatingPulse({ data_classification: "simulated", dataset_version: "sim v/1", as_of: "2026-06-05", window_days: 7, source_system: "retail_simulator", store_ids: ["store A", "store/B"] }, "token");
    const call = vi.mocked(fetch).mock.calls[0];
    const url = new URL(String(call[0]));
    expect(url.searchParams.getAll("store_id")).toEqual(["store A", "store/B"]);
    expect(url.searchParams.get("dataset_version")).toBe("sim v/1");
    expect(url.searchParams.get("source_system")).toBe("retail_simulator");
  });

  it("round-trips every supported window without collapsing repeated scope", async () => {
    for (const windowDays of [7, 14, 28] as const) {
      await retailAnalyticsApi.operatingPulse({ data_classification: "production", as_of: "2026-06-05", window_days: windowDays, store_ids: ["A", "B"] }, "token");
    }
    expect(vi.mocked(fetch)).toHaveBeenCalledTimes(3);
    const lastURL = new URL(String(vi.mocked(fetch).mock.calls[2][0]));
    expect(lastURL.searchParams.get("window_days")).toBe("28");
    expect(lastURL.searchParams.getAll("store_id")).toEqual(["A", "B"]);
  });

  it("rejects production with a dataset version and sends the stable generation key", async () => {
    expect(() => retailAnalyticsApi.operatingPulse({ data_classification: "production", dataset_version: "should-not-pass", as_of: "2026-06-05", window_days: 14 }, "token")).toThrow("production pulse cannot include dataset_version");
    await retailAnalyticsApi.generateDefaultSimulation("token");
    const init = vi.mocked(fetch).mock.calls[0][1] as RequestInit;
    expect((init.headers as Record<string, string>)["Idempotency-Key"]).toBe("max-005-retail-sim-v1-default");
  });

  it("encodes store diagnostics and keeps production/dataset mutually exclusive", async () => {
    await retailAnalyticsApi.storeDiagnostics({ store_id: "store A/1", data_classification: "simulated", dataset_version: "plan A/v1", as_of: "2026-06-05", window_days: 14, source_system: "retail simulator" }, "token");
    const url = new URL(String(vi.mocked(fetch).mock.calls[0][0]));
    expect(url.pathname).toContain("store%20A%2F1");
    expect(url.searchParams.get("dataset_version")).toBe("plan A/v1");
    expect(url.searchParams.get("source_system")).toBe("retail simulator");
  });

  it("lists store options with classification and dataset", async () => {
    await retailAnalyticsApi.storeOptions({ data_classification: "simulated", dataset_version: "planA-v1" }, "token");
    const url = new URL(String(vi.mocked(fetch).mock.calls[0][0]));
    expect(url.pathname).toContain("/retail/store-options");
    expect(url.searchParams.get("data_classification")).toBe("simulated");
  });

  it("enforces dataset/classification exclusivity for store 360 APIs", () => {
    expect(() => retailAnalyticsApi.storeOptions({ data_classification: "simulated" }, "token")).toThrow("requires dataset_version");
    expect(() => retailAnalyticsApi.storeOptions({ data_classification: "production", dataset_version: "v1" }, "token")).toThrow("cannot include dataset_version");
    expect(() => retailAnalyticsApi.storeDiagnostics({ store_id: "s", data_classification: "simulated", as_of: "2026-06-05", window_days: 7 }, "token")).toThrow("requires dataset_version");
    expect(() => retailAnalyticsApi.storeDiagnostics({ store_id: "s", data_classification: "production", dataset_version: "v1", as_of: "2026-06-05", window_days: 7 }, "token")).toThrow("cannot include dataset_version");
  });

  it("keeps scenario evaluate/action queries mutually exclusive and sends idempotency", async () => {
    const body = { horizon_months: 12, scenarios: [{ key: "baseline", name: "Baseline", assumptions: { revenue_change_pct: 0, gross_margin_rate_change_pp: 0, labor_cost_change_pct: 0, fixed_rent_change_pct: 0, variable_rent_rate_change_pp: 0, non_lease_cost_change_pct: 0, other_controllable_cost_change_pct: 0 } }, { key: "plan", name: "Plan", assumptions: { revenue_change_pct: 10, gross_margin_rate_change_pp: 0, labor_cost_change_pct: 0, fixed_rent_change_pct: 0, variable_rent_rate_change_pp: 0, non_lease_cost_change_pct: 0, other_controllable_cost_change_pct: 0 } }] };
    await retailAnalyticsApi.evaluateStoreScenario({ store_id: "s/1", data_classification: "simulated", dataset_version: "planA", as_of: "2026-06-05", window_days: 14 }, body, "token");
    await retailAnalyticsApi.saveStoreScenarioAction({ store_id: "s/1", data_classification: "simulated", dataset_version: "planA", as_of: "2026-06-05", window_days: 14 }, { horizon_months: 12, selected_scenario: body.scenarios[1], title: "T", planned_action: "A" }, "scenario-key", "token");
    const second = vi.mocked(fetch).mock.calls[1];
    const url = new URL(String(second[0]));
    expect(url.searchParams.get("dataset_version")).toBe("planA");
    expect((second[1]?.headers as Record<string, string>)["Idempotency-Key"]).toBe("scenario-key");
    expect(() => retailAnalyticsApi.evaluateStoreScenario({ store_id: "s", data_classification: "production", dataset_version: "bad", as_of: "2026-06-05", window_days: 7 }, body, "token")).toThrow("cannot include dataset_version");
    expect(() => retailAnalyticsApi.saveStoreScenarioAction({ store_id: "s", data_classification: "simulated", as_of: "2026-06-05", window_days: 7 }, { horizon_months: 12, selected_scenario: body.scenarios[1], title: "T", planned_action: "A" }, "scenario-key", "token")).toThrow("requires dataset_version");
  });

  // FP&A 反馈 2026-08-27（P0-1）：store-pnl 过去不带数据环境参数，后端默认
  // production，模拟店一律 not visible。透传后与脉搏/门店 360 同一套互斥纪律。
  it("store pnl carries the data environment and keeps classification/dataset exclusive", async () => {
    await storePnlApi.getPnl({ store_id: "st/7", as_of: "2026-06-05", window_days: 7, basis: "side_by_side", secondary: "budget", data_classification: "simulated", dataset_version: "plan A/v1" }, "token");
    const url = new URL(String(vi.mocked(fetch).mock.calls[0][0]));
    expect(url.pathname).toContain("/stores/st%2F7/pnl");
    expect(url.searchParams.get("data_classification")).toBe("simulated");
    expect(url.searchParams.get("dataset_version")).toBe("plan A/v1");
    expect(url.searchParams.get("window_days")).toBe("7");

    await storePnlApi.getPnl({ store_id: "s", as_of: "2026-06-05", window_days: 7, basis: "side_by_side", data_classification: "production" }, "token");
    const prodURL = new URL(String(vi.mocked(fetch).mock.calls[1][0]));
    expect(prodURL.searchParams.has("dataset_version")).toBe(false);

    expect(() => storePnlApi.getPnl({ store_id: "s", as_of: "2026-06-05", window_days: 7, basis: "side_by_side", data_classification: "simulated" }, "token")).toThrow("requires dataset_version");
    expect(() => storePnlApi.getPnl({ store_id: "s", as_of: "2026-06-05", window_days: 7, basis: "side_by_side", data_classification: "production", dataset_version: "v1" }, "token")).toThrow("cannot include dataset_version");
  });
});
