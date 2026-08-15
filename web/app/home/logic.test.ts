import { describe, expect, it } from "vitest";
import type { RetailPulseResponse } from "../lib/api";
import type { HomeBriefResult } from "./types";
import { briefAttentionCards, buildBriefFilters, canViewHomeBrief, classifyHomeBrief, planToThinking } from "./logic";

// HOME-002 B3: the six roles branch on analysis permission exactly like
// the nav (AppLayout canViewAnalysis = admin || readonly || auditor).
describe("canViewHomeBrief (B3 role matrix)", () => {
  const role = (roleName: string) => ({ id: "user-1", username: "u", role: "", roles: [roleName] });
  it.each([
    ["admin", true],
    ["readonly", true],
    ["auditor", true],
    ["editor", false],
    ["reviewer", false],
    ["approver", false],
  ] as const)("role %s → %s", (roleName, expected) => {
    expect(canViewHomeBrief(role(roleName))).toBe(expected);
  });

  it("treats a missing user as no analysis access", () => {
    expect(canViewHomeBrief(null)).toBe(false);
    expect(canViewHomeBrief(undefined)).toBe(false);
  });
});

const coverage = (observed: number, expected: number, rate: number) => ({
  requested_date_from: "2026-05-25",
  requested_date_to: "2026-06-07",
  observed_store_days: observed,
  expected_store_days: expected,
  coverage_rate: rate,
});

const pulse = (overrides: Partial<RetailPulseResponse> = {}): RetailPulseResponse => ({
  basis: "Working",
  pulse_version: "retail-pulse-v1",
  formula_version: "retail-kpi-v1",
  data_classification: "simulated",
  requested_scope: { legal_entity_id: "entity-a", store_ids: [] },
  source_systems: ["retail_simulator"],
  fact_version_min: 1,
  fact_version_max: 1,
  multi_currency: false,
  current: { date_from: "2026-06-01", date_to: "2026-06-07" },
  comparison: { date_from: "2026-05-25", date_to: "2026-05-31" },
  current_coverage: coverage(420, 420, 100),
  comparison_coverage: coverage(420, 420, 100),
  decision_ready: true,
  summary: {},
  daily_trend: [],
  attention: [],
  attention_count: 0,
  generated_at: "2026-06-07T00:00:00Z",
  definitions_url: "",
  kpi_drilldown_url: "",
  store_drilldown_url: "",
  current_kpi_drilldown_url: "",
  comparison_kpi_drilldown_url: "",
  ...overrides,
});

const result = (operations: HomeBriefResult["retail_operations"], extra: Partial<HomeBriefResult> = {}): HomeBriefResult => ({
  answer: "晨检摘要",
  confidence: 0.9,
  retail_operations: operations,
  ...extra,
});

// HOME-002 B4: no data / not decision-ready / scope_denied must be
// distinct states; scope_denied must win over coverage signals.
describe("classifyHomeBrief (B4 three states)", () => {
  it("classifies a decision-ready pulse as ready", () => {
    const state = classifyHomeBrief(result({ pulse: pulse() }), null);
    expect(state).toBe("ready");
  });

  it("classifies production with zero observed store-days as no_data", () => {
    const empty = pulse({
      decision_ready: false,
      current_coverage: coverage(0, 420, 0),
      comparison_coverage: coverage(0, 420, 0),
    });
    expect(classifyHomeBrief(result({ pulse: empty }), null)).toBe("no_data");
  });

  it("classifies partial coverage as not_decision_ready", () => {
    const partial = pulse({
      decision_ready: false,
      current_coverage: coverage(200, 420, 47.6),
      comparison_coverage: coverage(200, 420, 47.6),
    });
    expect(classifyHomeBrief(result({ pulse: partial }), null)).toBe("not_decision_ready");
  });

  it("keeps scope_denied honest even when coverage looks empty", () => {
    const denied = result({ reason: "scope_denied", needs_input: true, pulse: null });
    expect(classifyHomeBrief(denied, null)).toBe("scope_denied");
  });

  it("classifies needs_input and error separately", () => {
    expect(classifyHomeBrief(result({ needs_input: true, reason: "missing_context" }), null)).toBe("needs_input");
    expect(classifyHomeBrief(result({ pulse: null }), null)).toBe("needs_input");
    expect(classifyHomeBrief(null, "boom")).toBe("error");
    expect(classifyHomeBrief(null, null)).toBe("loading");
  });
});

describe("briefAttentionCards", () => {
  it("maps attention rows into numbered cards", () => {
    const cards = briefAttentionCards(
      pulse({
        attention: [
          {
            rank: 1,
            store_id: "store-1",
            store_code: "SIM-006",
            store_name: "门店6",
            brand: "brand-a",
            region: "region-a",
            currency: "CNY",
            severity: "critical",
            score: 9.9,
            evidence: {
              current: { date_from: "2026-06-01", date_to: "2026-06-07" },
              comparison: { date_from: "2026-05-25", date_to: "2026-05-31" },
              current_fact_count: 7,
              comparison_fact_count: 7,
              source_systems: ["retail_simulator"],
              dataset_versions: ["ds-1"],
              formula_version: "retail-kpi-v1",
              pulse_version: "retail-pulse-v1",
            },
            observed_signals: [
              {
                signal_code: "occupancy_cost_rate_spike",
                observed_change: 10.08,
                threshold: 25,
                direction: "up",
                current: 30,
                comparison: 20,
                unit: "percentage_point",
                score_contribution: 0.6,
              },
            ],
            current_kpis: {},
            comparison_kpis: {},
            drilldown: {},
          },
        ],
      }),
    );
    expect(cards).toHaveLength(1);
    expect(cards[0].rank).toBe(1);
    expect(cards[0].store_code).toBe("SIM-006");
    expect(cards[0].currency).toBe("CNY");
    expect(cards[0].signals[0].signal_code).toBe("occupancy_cost_rate_spike");
  });

  it("returns an empty list when there is no pulse", () => {
    expect(briefAttentionCards(null)).toEqual([]);
    expect(briefAttentionCards(undefined)).toEqual([]);
  });
});

describe("planToThinking", () => {
  it("formats the agent plan into trace text", () => {
    const text = planToThinking([
      { id: "a", title: "读取经营事实", status: "completed" },
      { id: "b", title: "核对覆盖", status: "running" },
      { id: "c", title: "输出结论", status: "pending" },
    ]);
    expect(text).toContain("✓ 读取经营事实");
    expect(text).toContain("… 核对覆盖");
  });

  it("returns empty text for an absent plan", () => {
    expect(planToThinking(undefined)).toBe("");
    expect(planToThinking([])).toBe("");
  });
});

describe("buildBriefFilters", () => {
  it("adds dataset_version for simulated runs", () => {
    const filters = buildBriefFilters("simulated", "ds-2026-06", "2026-06-07", 7, "retail_simulator");
    expect(filters).toEqual({
      as_of: "2026-06-07",
      window_days: "7",
      data_classification: "simulated",
      dataset_version: "ds-2026-06",
      source_system: "retail_simulator",
    });
  });

  it("never carries dataset_version on production runs", () => {
    const filters = buildBriefFilters("production", undefined, "2026-06-07", 7);
    expect(filters.dataset_version).toBeUndefined();
    expect(filters).toEqual({ as_of: "2026-06-07", window_days: "7", data_classification: "production" });
  });
});
