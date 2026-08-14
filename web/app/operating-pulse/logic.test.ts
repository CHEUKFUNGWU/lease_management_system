import { describe, expect, it } from "vitest";
import type { RetailKPIValue, RetailPulseResponse } from "../lib/api";
import { changeTone, formatChange, formatKPIValue, formatSignalValue, kpiLabel, latestAnomalyDate, metricStatusLabel, responsePartitions, signalLabel, switchClassification, trendValue } from "./logic";

const value = (unit: string, current: number | null): RetailKPIValue => ({
  value: current,
  unit,
  status: current === null ? "unavailable" : "complete",
  formula_version: "retail-kpi-v1",
  required_fields: [],
  available_fact_count: current === null ? 0 : 1,
  fact_count: 1,
  reason: current === null ? "zero_denominator" : undefined,
});

const response = (overrides: Partial<RetailPulseResponse> = {}): RetailPulseResponse => ({
  basis: "Working", pulse_version: "retail-pulse-v1", formula_version: "retail-kpi-v1", data_classification: "simulated",
  requested_scope: { legal_entity_id: "entity-a", store_ids: [] }, source_systems: ["retail_simulator"], fact_version_min: 1, fact_version_max: 1,
  multi_currency: false, current: { date_from: "2026-06-01", date_to: "2026-06-07" }, comparison: { date_from: "2026-05-25", date_to: "2026-05-31" },
  current_coverage: { requested_date_from: "2026-06-01", requested_date_to: "2026-06-07", observed_store_days: 7, expected_store_days: 7, coverage_rate: 100 },
  comparison_coverage: { requested_date_from: "2026-05-25", requested_date_to: "2026-05-31", observed_store_days: 7, expected_store_days: 7, coverage_rate: 100 },
  decision_ready: true, summary: {}, daily_trend: [], attention: [], attention_count: 0, generated_at: "2026-06-07T00:00:00Z", definitions_url: "", kpi_drilldown_url: "", store_drilldown_url: "", current_kpi_drilldown_url: "", comparison_kpi_drilldown_url: "",
  ...overrides,
});

describe("operating pulse presentation adapter", () => {
  it("formats money, percent, count and null without inventing zero", () => {
    expect(formatKPIValue(value("currency", 1234.5), "CNY", "zh-CN")).toBe("1,234.50 CNY");
    expect(formatKPIValue(value("percent", 12.5), undefined, "zh-CN")).toBe("12.50%");
    expect(formatKPIValue(value("count", 42), undefined, "zh-CN")).toBe("42 笔/人次");
    expect(formatKPIValue(value("currency", null), "CNY", "zh-CN")).toBe("—");
  });

  it("keeps pp and percent changes distinct and maps unfavorable direction", () => {
    expect(formatChange({ current: value("percent", 10), comparison: value("percent", 12), change_value: -2, change_type: "percentage_point", status: "complete" })).toBe("-2.00pp");
    expect(formatChange({ current: value("currency", 90), comparison: value("currency", 100), change_value: -10, change_type: "percent", status: "complete" })).toBe("-10.00%");
    expect(changeTone("revenue", { current: value("currency", 90), comparison: value("currency", 100), change_value: -10, change_type: "percent", status: "complete" })).toBe("bad");
    expect(changeTone("labor_cost_rate", { current: value("percent", 12), comparison: value("percent", 10), change_value: 2, change_type: "percentage_point", status: "complete" })).toBe("bad");
    expect(formatSignalValue(-10, "percent", undefined, "zh-CN")).toBe("-10.00%");
    expect(formatSignalValue(-2, "percentage_point", undefined, "zh-CN")).toBe("-2.00pp");
    expect(formatSignalValue(20, "count", undefined, "zh-CN")).toBe("20 笔/人次");
  });

  it("preserves partitions, gaps and null trend values", () => {
    const partition = { currency: "CNY", current: response().current, comparison: response().comparison, current_coverage: response().current_coverage, comparison_coverage: response().comparison_coverage, decision_ready: false, daily_trend: [{ date: "2026-06-01", currency: "CNY", gap: true, coverage: response().current_coverage, kpis: {} }], attention: [], attention_count: 0 };
    const result = response({ multi_currency: true, partitions: [partition] });
    expect(responsePartitions(result)).toHaveLength(1);
    expect(trendValue(partition.daily_trend[0], "revenue")).toBeNull();
    expect(trendValue({ ...partition.daily_trend[0], gap: true, kpis: { revenue: value("currency", 99) } }, "revenue")).toBeNull();
    expect(responsePartitions(response({ multi_currency: true, partitions: [partition, { ...partition, currency: "USD" }] }))).toHaveLength(2);
  });

  it("labels all fixed anomaly manifest types", () => {
    expect([
      "footfall_continuous_decline", "conversion_rate_drop", "average_ticket_drop",
      "gross_margin_compression", "labor_cost_spike", "occupancy_cost_burden",
    ].map((code) => signalLabel(code, "zh-CN"))).toEqual(["连续客流下降", "转化率下降", "客单价下降", "毛利率收窄", "人工成本率上升", "经营占用成本率上升"]);
  });

  it("normalizes the three classification switch paths", () => {
    const latest = { id: "d1", dataset_version: "sim-v1", generator_version: "gen-v1", seed: 1, date_from: "2026-01-01", date_to: "2026-06-30", store_count: 60, fact_count: 10860, status: "completed", anomaly_manifest: [{ id: "a", type: "revenue_decline", store_code: "S1", date_from: "2026-06-01", date_to: "2026-06-05", expected_direction: "down", description: "" }], created_at: "2026-07-01T00:00:00Z" };
    expect(switchClassification("simulated", latest, "2026-06-30", "2026-08-12")).toEqual({ classification: "simulated", datasetVersion: "sim-v1", asOf: "2026-06-05", sourceSystem: "retail_simulator", clearToEmpty: false });
    expect(switchClassification("production", latest, "2026-06-05", "2026-08-12")).toEqual({ classification: "production", asOf: "2026-08-12", clearToEmpty: false });
    expect(switchClassification("simulated", null, "2026-06-05", "2026-08-12")).toEqual({ classification: "simulated", asOf: "2026-06-05", sourceSystem: "retail_simulator", clearToEmpty: true });
    expect(latestAnomalyDate({ ...latest, anomaly_manifest: [] })).toBe("2026-06-30");
  });

  it("exposes complete, partial and unavailable metric status reasons", () => {
    const complete = { current: value("percent", 10), comparison: value("percent", 9), change_value: 1, change_type: "percentage_point", status: "complete" } as const;
    const partial = { current: { ...value("percent", 10), status: "partial", reason: "coverage_below_threshold" }, comparison: value("percent", 9), change_value: null, change_type: "percentage_point", status: "partial" } as const;
    const unavailable = { current: { ...value("percent", null), status: "unavailable", reason: "zero_denominator" }, comparison: value("percent", 9), change_value: null, change_type: "percentage_point", status: "unavailable" } as const;
    expect(metricStatusLabel(complete, "zh-CN")).toEqual({ status: "complete", label: "完整", reason: undefined });
    expect(metricStatusLabel(partial, "zh-CN")).toEqual({ status: "partial", label: "部分", reason: "coverage_below_threshold" });
    expect(metricStatusLabel(unavailable, "zh-CN")).toEqual({ status: "missing", label: "缺失", reason: "zero_denominator" });
    expect(metricStatusLabel(null, "zh-CN")).toEqual({ status: "missing", label: "缺失", reason: "指标不可用" });
  });
});
