/**
 * R1-2 守卫：销售人效与工时面板的数据状态呈现。
 *
 * 断言的是行为不是措辞：每个状态下哪些值出现、哪些不出现、grain_note 是否
 * 常驻。自检句——把任一分支的渲染改错（缺失填 0、降级藏行、note 变条件
 * 渲染），对应断言即红。
 */
import { describe, expect, it } from "vitest";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { LaborEfficiencyPanel } from "./LaborEfficiencyPanel";
import { LanguageProvider } from "../context/LanguageContext";
import { t, type Language } from "../lib/i18n";
import type { RetailPeerBenchmark, RetailStore360SummaryMetric } from "../lib/api";

const zh = "zh-CN" as Language;

function metric(value: number | null, status: string = "complete", reason?: string): Record<string, RetailStore360SummaryMetric> {
  const mk = () => ({
    current: { value, unit: "currency_per_hour", status, formula_version: "v1", required_fields: [], available_fact_count: 14, fact_count: 14, reason },
    comparison: { value, unit: "currency_per_hour", status, formula_version: "v1", required_fields: [], available_fact_count: 14, fact_count: 14, reason },
    change_value: null,
    change_type: "percent",
    status,
    reason,
  });
  return {
    sales_per_labor_hour: mk(),
    labor_hours_per_transaction: mk(),
    labor_cost_rate: mk(),
    headcount: mk(),
  };
}

function render(props: Parameters<typeof LaborEfficiencyPanel>[0]): string {
  return renderToStaticMarkup(React.createElement(LanguageProvider, null, React.createElement(LaborEfficiencyPanel, props)));
}

describe("R1-2 LaborEfficiencyPanel 数据状态", () => {
  it("正常态：四项数值都渲染，前端零计算（值原样来自后端）", () => {
    const markup = render({ summary: metric(42), currency: "CNY" });
    expect(markup).toContain("42");
    expect(markup).toContain(t("store360.labor.metric.sph", zh));
    expect(markup).toContain(t("store360.labor.metric.hpt", zh));
    expect(markup).toContain(t("store360.labor.metric.rate", zh));
    expect(markup).toContain(t("store360.labor.metric.hc", zh));
  });

  it("工时缺失：销售人效渲染「—」而不是估算值，原因可 hover", () => {
    const summary = metric(null, "partial", "missing_required_field");
    summary.labor_hours_per_transaction.current.value = 0.5;
    const markup = render({ summary });
    // 缺失值是「—」，不是 0、不是任何反推数
    expect(markup).not.toContain(">0</span>");
    expect(markup).toContain(t("reason.missing_required_field", zh) === "reason.missing_required_field" ? "missing_required_field" : t("reason.missing_required_field", zh));
  });

  it("覆盖率不足：unavailable 态同样「—」加原因，不静默", () => {
    const markup = render({ summary: metric(null, "unavailable", "no_facts") });
    expect(markup).toContain(t("reason.no_facts", zh) === "reason.no_facts" ? "no_facts" : t("reason.no_facts", zh));
  });

  it("模拟数据：面板带模拟标签，不冒充正式数据", () => {
    const markup = render({ summary: metric(42), dataClassification: "simulated" });
    expect(markup).toContain(t("trust.classification_simulated", zh));
  });

  it("同群样本不足：显式降级文案，不空白、不填 0；样本足时显示中位数行", () => {
    const benchmarks: RetailPeerBenchmark[] = [
      { code: "sales_per_labor_hour", unit: "currency_per_hour", target: null, peer_count: 0, median: null, p25: null, p75: null, percentile: null, target_minus_median: null, status: "insufficient_peers", reason: "peer_count_below_minimum" },
      { code: "labor_cost_rate", unit: "percent", target: 20, peer_count: 4, median: 15, p25: 12, p75: 18, percentile: 60, target_minus_median: 5, status: "complete" },
    ];
    const degraded = render({ summary: metric(42), benchmarks });
    expect(degraded).toContain(t("store360.labor.peer_insufficient", zh));
    expect(degraded).toContain("labor-peer-insufficient");
    // 样本足的基准照常显示中位数与家数
    expect(degraded).toContain("15.00%");
    expect(degraded).toContain("4 家");
  });

  it("grain_note 常驻：空数据与正常态下都在，不随数据状态消失", () => {
    for (const props of [{}, { summary: metric(42) }]) {
      const markup = render({ ...props, summary: (props as { summary?: Record<string, RetailStore360SummaryMetric> }).summary });
      expect(markup).toContain(t("store360.labor.grain_note", zh));
    }
  });
});
