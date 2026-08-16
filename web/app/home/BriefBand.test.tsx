/**
 * HOME-004 §2 验收（源码层 + SSR 渲染层）。
 *
 * - 收起态一行：标题 + 关注门店数 + DataTrustBar 摘要行（复用，无第二套）
 * - `answer` 机器文本不再作为正文渲染，只进 ThinkingTrace（§2.2）
 * - 关注门店卡三槽位：门店标识 / 信号（tabular-nums）/ 严重度点 + 文字
 * - 降级态沿用 BriefView 专属呈现，scope_denied 不软化
 */
import { describe, expect, it } from "vitest";
import React from "react";
import { readFileSync } from "node:fs";
import path from "node:path";
import { renderToStaticMarkup } from "react-dom/server";
import BriefBand from "./BriefBand";
import { LanguageProvider } from "../context/LanguageContext";
import { AuthProvider } from "../context/AuthContext";
import { t, type Language } from "../lib/i18n";
import type { RetailSummaryMetric } from "../lib/api";
import type { HomeBriefResult } from "./types";

const zh = "zh-CN" as Language;

function render(children: React.ReactNode) {
  return renderToStaticMarkup(
    React.createElement(LanguageProvider, null, React.createElement(AuthProvider, null, children))
  );
}

const base = { language: zh, onRetry: () => {} };

// Deterministic ready-brief fixture shaped like the real retail_operations
// response: machine answer text, plan, tool calls, confidence, one critical
// attention store and the three summary metrics the band cards consume.
const coverage = (observed: number, expected: number, rate: number) => ({
  requested_date_from: "2026-06-01",
  requested_date_to: "2026-06-07",
  observed_store_days: observed,
  expected_store_days: expected,
  coverage_rate: rate,
});

const metric = (value: number, unit: string, change: number | null, changeType: string): RetailSummaryMetric => ({
  current: { value, unit, status: "complete", formula_version: "retail-kpi-v1", required_fields: [], available_fact_count: 420, fact_count: 420 },
  comparison: { value: value - 1000, unit, status: "complete", formula_version: "retail-kpi-v1", required_fields: [], available_fact_count: 420, fact_count: 420 },
  change_value: change,
  change_type: changeType,
  status: "complete",
});

const readyBriefFixture: HomeBriefResult = {
  answer:
    "数据上下文：simulated · dataset=retail-sim-v1 · source=retail_simulator · as_of=2026-06-07 · window=7天 · formula=retail-kpi-v1 · evidence=complete · confidence=0.90。",
  confidence: 0.9,
  tool_calls: [{ tool: "retail.operating_pulse.read", status: "completed", duration_ms: 42 }],
  agent_plan: [
    { id: "load_retail_context", title: "读取经营事实与来源", status: "completed" },
    { id: "prepare_review", title: "输出可复核结论", status: "completed" },
  ],
  sources: [{ title: "retail-pulse-v1 / 2026-06-07", url: "/operating-pulse?as_of=2026-06-07" }],
  retail_operations: {
    intent: "pulse_summary",
    data_classification: "simulated",
    dataset_version: "retail-sim-v1",
    as_of: "2026-06-07",
    window_days: 7,
    formula_version: "retail-kpi-v1",
    evidence_status: "complete",
    pulse: {
      basis: "Working",
      pulse_version: "retail-pulse-v1",
      formula_version: "retail-kpi-v1",
      data_classification: "simulated",
      requested_scope: { legal_entity_id: "entity-a", store_ids: [] },
      source_systems: ["retail_simulator"],
      fact_version_min: 1,
      fact_version_max: 1,
      multi_currency: false,
      currency: "CNY",
      current: { date_from: "2026-06-01", date_to: "2026-06-07" },
      comparison: { date_from: "2026-05-25", date_to: "2026-05-31" },
      current_coverage: coverage(420, 420, 100),
      comparison_coverage: coverage(420, 420, 100),
      decision_ready: true,
      summary: {
        revenue: metric(1801064.11, "currency", 0.36, "percent"),
        gross_margin_rate: metric(31.51, "percent", -0.8, "percentage_point"),
        store_contribution: metric(280000, "currency", -2.1, "percent"),
      },
      daily_trend: [],
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
            dataset_versions: ["retail-sim-v1"],
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
      attention_count: 1,
      generated_at: "2026-06-07T00:00:00Z",
      envelope: {
        data_classification: "simulated",
        source_systems: ["retail_simulator"],
        dataset_versions: ["retail-sim-v1"],
        fact_version_min: 1,
        fact_version_max: 1,
        highest_as_of: "2026-06-07",
        current_coverage: { observed_store_days: 420, expected_store_days: 420, coverage_rate: 100 },
        comparison_coverage: { observed_store_days: 420, expected_store_days: 420, coverage_rate: 100 },
        decision_ready: true,
        formula_version: "retail-kpi-v1",
        pulse_version: "retail-pulse-v1",
        semantic_version: "retail-kpi-v1",
        generated_at: "2026-06-07T00:00:00Z",
      },
      definitions_url: "",
      kpi_drilldown_url: "",
      store_drilldown_url: "",
      current_kpi_drilldown_url: "",
      comparison_kpi_drilldown_url: "",
    },
  },
};

describe("BriefBand (collapsed strip)", () => {
  it("renders title, attention count and the trust bar summary in one header block", () => {
    const markup = render(React.createElement(BriefBand, { state: "ready", result: readyBriefFixture, error: null, ...base }));
    expect(markup).toContain("home-brief-band");
    expect(markup).toContain(t("home.brief_title", zh));
    expect(markup).toContain(t("home.band_attention_count", zh, { count: "1" }));
    expect(markup).toContain("data-trust-bar");
    expect(markup).toContain(t("trust.classification_simulated", zh));
  });

  it("does not render the machine answer as page content", () => {
    const markup = render(React.createElement(BriefBand, { state: "ready", result: readyBriefFixture, error: null, ...base }));
    // The raw answer stays available inside the collapsed trace only — the
    // band markup must not contain a visible answer paragraph.
    expect(markup).not.toContain("home-brief-answer");
    const bandSource = readFileSync(path.join(__dirname, "BriefBand.tsx"), "utf8");
    expect(bandSource).not.toContain('className="home-brief-answer"');
  });

  it("FIX-006: collapsed band exposes exactly one expand control", () => {
    const markup = render(React.createElement(BriefBand, { state: "ready", result: readyBriefFixture, error: null, ...base }));
    const buttons = (markup.match(/<button/g) || []).length;
    expect(buttons).toBe(1);
    expect(markup).toContain("home-brief-band-toggle");
    expect(markup).not.toContain("data-trust-bar-toggle");
  });

  it("keeps degraded states on BriefView, scope_denied unsoftened", () => {
    const denied = render(React.createElement(BriefBand, { state: "scope_denied", result: readyBriefFixture, error: null, ...base }));
    expect(denied).toContain(t("api.scope_denied", zh));
    expect(denied).not.toContain("暂无数据");
    const noData = render(React.createElement(BriefBand, { state: "no_data", result: null, error: null, ...base }));
    expect(noData).toContain(t("home.brief_no_data", zh));
  });
});

describe("BriefBand source contract (§2.2 / §2.3 / L4)", () => {
  const source = readFileSync(path.join(__dirname, "BriefBand.tsx"), "utf8");

  it("reuses the shared explainability components, no second implementation", () => {
    expect(source).toContain('DataTrustBar from "../components/DataTrustBar"');
    expect(source).toContain('ThinkingTrace from "../components/ThinkingTrace"');
    expect(source).toContain('ToolChip from "../components/ToolChip"');
    expect(source).toContain('ConfidenceBadge from "../components/ConfidenceBadge"');
    expect(source).toContain('{ SeverityDot, toSeverity } from "../components/SeverityDot"');
  });

  it("drives the trust bar and the band body with one toggle", () => {
    expect(source).toContain("expanded={expanded}");
    expect(source).toContain("onToggle={() => setExpanded(!expanded)}");
  });

  it("puts the confidence badge on the tool-chip row (L4)", () => {
    const metaBlock = source.slice(source.indexOf("home-band-meta"));
    const chipIndex = metaBlock.indexOf("ai-tool-row");
    const badgeIndex = metaBlock.indexOf("<ConfidenceBadge");
    expect(chipIndex).toBeGreaterThan(-1);
    expect(badgeIndex).toBeGreaterThan(chipIndex);
  });

  it("gives attention cards the three structured slots (§2.3)", () => {
    expect(source).toContain("home-band-attention-store");
    expect(source).toContain("home-band-attention-signals");
    expect(source).toContain("home-band-attention-severity");
    expect(source).toContain("home.severity_");
  });
});
