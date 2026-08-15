import { describe, expect, it } from "vitest";
import type { RetailBridge, RetailStore360Trend } from "../lib/api";
import { bridgeConservation, bridgeTone, bridgeWaterfall, bridgeWaterfallDomain, diagnosticQueryKey, formatPeerBenchmarkStatus, formatTrendTooltip, optionFields, returnPulseQuery, trendValue, validWindow } from "./logic";

describe("store 360 presentation contract", () => {
  it("supports only the fixed windows and stable query keys", () => {
    expect(validWindow(7)).toBe(true);
    expect(validWindow(14)).toBe(true);
    expect(validWindow(28)).toBe(true);
    expect(validWindow(8)).toBe(false);
    expect(diagnosticQueryKey({ storeID: "s", classification: "simulated", datasetVersion: "v", asOf: "2026-06-05", windowDays: 14, sourceSystem: "retail_simulator" })).toContain("s|simulated|v");
  });

  it("round-trips pulse filters and preserves store scope", () => {
    const params = new URLSearchParams("store_id=s%201&store_id=s%2F2&data_classification=simulated&dataset_version=plan%20A&as_of=2026-06-05&window_days=7&source_system=retail_simulator");
    const returned = returnPulseQuery(params);
    expect(returned).toContain("/operating-pulse?");
    const query = new URLSearchParams(returned.split("?")[1]);
    expect(query.getAll("store_id")).toEqual(["s 1", "s/2"]);
    expect(query.get("dataset_version")).toBe("plan A");
    expect(returnPulseQuery(new URLSearchParams("return_query=data_classification%3Dproduction%26as_of%3D2026-06-05%26window_days%3D14"))).toBe("/operating-pulse?data_classification=production&as_of=2026-06-05&window_days=14");
  });

  it("normalizes options and never draws a gap as zero", () => {
    expect(optionFields({ store_id: "s", store_code: "S1", store_name: "One", brand: "B", region: "R" }).storeCode).toBe("S1");
    expect(trendValue({ date: "2026-06-05", gap: true, target_kpis: { revenue: { value: 100, unit: "currency", status: "complete", formula_version: "test", required_fields: [], available_fact_count: 1, fact_count: 1 } }, peer_median: {}, peer_count: {} } as RetailStore360Trend, "revenue")).toBeNull();
    expect(bridgeTone(-1)).toBe("negative");
    expect(bridgeTone(0)).toBe("neutral");
  });

  it("checks bridge conservation including residual", () => {
    expect(bridgeConservation({ code: "test", method: "test", version: "test", status: "complete", current: 10, comparison: 0, total_change: 10, items: [{ code: "a", label: "a", contribution: 4.99, unit: "currency" }, { code: "b", label: "b", contribution: 5, unit: "currency" }], rounding_residual: 0.01 } as RetailBridge)).toBeCloseTo(0);
    expect(bridgeConservation({ code: "test", method: "test", version: "test", status: "unavailable", current: null, comparison: null, total_change: null, items: [], rounding_residual: null } as RetailBridge)).toBeNull();
  });

  it("keeps peer reason visible and scopes gap tooltip to target only", () => {
    expect(formatPeerBenchmarkStatus("insufficient_peers", "peer_count_below_minimum", "zh-CN")).toBe("同群样本不足 · peer_count_below_minimum");
    expect(formatTrendTooltip(1234, "target", true, "currency", "CNY", "zh-CN")).toEqual(["数据缺口", "目标门店"]);
    expect(formatTrendTooltip(1234, "peer", true, "currency", "CNY", "zh-CN")).toEqual(["1,234.00 CNY", "同群中位数"]);
  });
});

describe("FIX-018: bridge waterfall steps", () => {
  const labels = { start: "期初", end: "期末", residual: "残差" };
  const base = {
    code: "store_contribution", method: "m", version: "1", status: "complete",
    comparison: 1000, current: 900, total_change: -100, rounding_residual: null,
    items: [
      { code: "revenue", label: "收入", contribution: 200, unit: "currency" },
      { code: "labor", label: "人工", contribution: -300, unit: "currency" },
    ],
  };

  it("opens at the comparison value and closes at the current value", () => {
    const steps = bridgeWaterfall(base, labels);
    expect(steps[0]).toMatchObject({ name: "期初", range: [0, 1000], tone: "neutral" });
    expect(steps[steps.length - 1]).toMatchObject({ name: "期末", range: [0, 900], tone: "neutral" });
  });

  it("floats each item between its running start and end, signed by tone", () => {
    const steps = bridgeWaterfall(base, labels);
    expect(steps[1]).toMatchObject({ name: "收入", range: [1000, 1200], tone: "positive" });
    expect(steps[2]).toMatchObject({ name: "人工", range: [900, 1200], tone: "negative" });
  });

  it("shows a non-zero rounding residual as its own step so the bars reconcile", () => {
    const steps = bridgeWaterfall({ ...base, rounding_residual: -5, current: 895 }, labels);
    expect(steps.map((step) => step.name)).toContain("残差");
  });

  it("yields nothing for an incomplete bridge", () => {
    expect(bridgeWaterfall({ ...base, status: "unavailable" }, labels)).toEqual([]);
    expect(bridgeWaterfall({ ...base, comparison: null }, labels)).toEqual([]);
  });
});

describe("FIX-018a: waterfall axis domain", () => {
  const labels = { start: "期初", end: "期末", residual: "残差" };
  const bridge = {
    code: "revenue", method: "m", version: "1", status: "complete",
    comparison: 48000, current: 49500, total_change: 1500, rounding_residual: null,
    items: [{ code: "footfall", label: "客流", contribution: 1500, unit: "currency" }],
  };

  it("brackets the values the steps span instead of anchoring at zero", () => {
    const domain = bridgeWaterfallDomain(bridgeWaterfall(bridge, labels));
    expect(domain).not.toBeNull();
    const [low, high] = domain as [number, number];
    expect(low).toBeGreaterThan(40000);
    expect(high).toBeLessThan(60000);
    expect(low).toBeLessThan(48000);
    expect(high).toBeGreaterThan(49500);
  });

  it("returns null when there is nothing to bracket", () => {
    expect(bridgeWaterfallDomain([])).toBeNull();
  });
});
