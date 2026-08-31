import { describe, expect, it } from "vitest";
import { planComparisonTestables, PlanComparisonUnavailable } from "./PlanComparisonPanel";
import type { StorePnlAggregateResult, StorePnlProjection } from "../lib/api";

const rows = [
  { key: "revenue", label: "Revenue", kind: "link", basis: "operating_basis", actual: 90, other: 100, variance: -10, pct: -0.1 },
  { key: "gross_profit", label: "Gross profit", kind: "link", basis: "operating_basis", actual: 30, other: null, variance: null, pct: null },
];

describe("Actual vs Budget presentation", () => {
  it("exposes a dash and reason when the Budget version is absent", () => {
    expect(PlanComparisonUnavailable).toBeTypeOf("function");
    expect(planComparisonTestables.summaryRows({
      period: "2026-06", plan_is_official: false, expected_store_count: 1, actual_store_count: 1, plan_store_count: 0,
      variances: [{ kpi: "revenue", actual: null, plan: null, variance: null, variance_pct: null, materiality_exceeded: false, decision_ready: false, downgrade_reason: "budget_missing" }], decision_ready: false,
    }, "en")).toMatchObject([{ actual: null, budget: null, variance: null, variancePct: null, reason: "budget_missing" }]);
  });

  it("keeps a missing Budget as dash-ready null with a reason", () => {
    const projection = {
      store_id: "s1", as_of: "2026-06-30", window_days: 1, period: { from: "2026-06-01", to: "2026-06-30" },
      columns: ["actual", "budget"], operating: { basis: "operating_basis", rows }, decision_ready: false,
      data_classification: "simulated", currency: "CNY", gaps: ["budget_missing"],
    } as StorePnlProjection;
    const result = planComparisonTestables.projectionRows(projection);
    expect(result[0]).toMatchObject({ actual: 90, budget: 100, variance: -10, variancePct: -10 });
    expect(result[1]).toMatchObject({ budget: null, variance: null, variancePct: null, reason: "budget_missing" });
  });

  it("preserves currency partitions instead of creating a mixed-currency total", () => {
    const aggregate = {
      group_by: "region", period: { from: "2026-06-01", to: "2026-06-30" }, columns: ["actual", "budget"],
      groups: [{ key: "North", store_count: 2, mixed_currency: true, partitions: [
        { currency: "CNY", decision_ready: true, operating: { basis: "operating_basis", rows } },
        { currency: "USD", decision_ready: true, operating: { basis: "operating_basis", rows } },
      ] }],
    } as StorePnlAggregateResult;
    const result = planComparisonTestables.aggregateRows(aggregate);
    expect(new Set(result.map((row) => row.currency))).toEqual(new Set(["CNY", "USD"]));
    expect(result.every((row) => row.group === "North")).toBe(true);
  });
});
