/**
 * HOME-002 验收测试。
 *
 * B2: 简报只复用既有可解释性组件（DataTrustBar / ToolChip /
 *     ConfidenceBadge / SourceCitation / ThinkingTrace / SeverityDot），
 *     不出现第二套可信度 / 引用 / 置信度渲染。
 * B4: 无数据 / 非 decision-ready / scope_denied 三态各渲染其专属文案，
 *     scope_denied 不软化（不含「暂无数据」类文案）。
 */
import { describe, expect, it } from "vitest";
import React from "react";
import { readFileSync } from "node:fs";
import path from "node:path";
import { renderToStaticMarkup } from "react-dom/server";
import BriefView from "./BriefView";
import { LanguageProvider } from "../context/LanguageContext";
import { AuthProvider } from "../context/AuthContext";
import { t, type Language } from "../lib/i18n";
import type { SourceEnvelope } from "../lib/api";
import type { HomeBriefResult } from "./types";

const zh = "zh-CN" as Language;

function render(children: React.ReactNode) {
  return renderToStaticMarkup(
    React.createElement(LanguageProvider, null, React.createElement(AuthProvider, null, children))
  );
}

const envelope: SourceEnvelope = {
  data_classification: "simulated",
  source_systems: ["retail_simulator"],
  dataset_versions: ["ds-1"],
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
};

const coverage = (observed: number, expected: number, rate: number) => ({
  requested_date_from: "2026-05-25",
  requested_date_to: "2026-06-07",
  observed_store_days: observed,
  expected_store_days: expected,
  coverage_rate: rate,
});

const readyResult: HomeBriefResult = {
  answer: "昨日销售 -8.2%，主要来自 3 家门店。",
  confidence: 0.9,
  tool_calls: [{ tool: "retail.operating_pulse.read", status: "completed", duration_ms: 42 }],
  agent_plan: [
    { id: "load_retail_context", title: "读取经营事实与来源", status: "completed" },
    { id: "check_retail_quality", title: "核对覆盖、币种和来源冲突", status: "completed" },
    { id: "prepare_review", title: "输出可复核结论或情景提议", status: "completed" },
  ],
  sources: [{ title: "retail-pulse-v1 / 2026-06-07", url: "/operating-pulse?as_of=2026-06-07" }],
  retail_operations: {
    intent: "pulse_summary",
    data_classification: "simulated",
    dataset_version: "ds-1",
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
      current: { date_from: "2026-06-01", date_to: "2026-06-07" },
      comparison: { date_from: "2026-05-25", date_to: "2026-05-31" },
      current_coverage: coverage(420, 420, 100),
      comparison_coverage: coverage(420, 420, 100),
      decision_ready: true,
      summary: {},
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
      attention_count: 1,
      generated_at: "2026-06-07T00:00:00Z",
      envelope,
      definitions_url: "",
      kpi_drilldown_url: "",
      store_drilldown_url: "",
      current_kpi_drilldown_url: "",
      comparison_kpi_drilldown_url: "",
    },
  },
};

const base = {
  language: zh,
  onRetry: () => {},
};

describe("BriefView (B4 states)", () => {
  it("renders the ready brief with trust bar, tool chip, cards and citations", () => {
    const markup = render(React.createElement(BriefView, { state: "ready", result: readyResult, error: null, ...base }));
    expect(markup).toContain(t("trust.classification_simulated", zh));
    expect(markup).toContain("retail.operating_pulse.read");
    expect(markup).toContain("SIM-006");
    expect(markup).toContain("retail-pulse-v1 / 2026-06-07");
    expect(markup).toContain(t("home.brief_attention", zh));
    expect(markup).toContain(t("ai.thinking_process", zh));
    expect(markup).toContain("90%");
  });

  it("renders the no-data state with its own copy", () => {
    const markup = render(React.createElement(BriefView, { state: "no_data", result: null, error: null, ...base }));
    expect(markup).toContain(t("home.brief_no_data", zh));
  });

  it("renders the not-decision-ready state with its own copy", () => {
    const markup = render(React.createElement(BriefView, { state: "not_decision_ready", result: null, error: null, ...base }));
    expect(markup).toContain(t("home.brief_not_ready_title", zh));
    expect(markup).toContain(t("home.brief_not_ready", zh));
  });

  it("renders scope_denied honestly and never as no data", () => {
    const markup = render(React.createElement(BriefView, { state: "scope_denied", result: readyResult, error: null, ...base }));
    expect(markup).toContain(t("api.scope_denied", zh));
    expect(markup).not.toContain(t("home.brief_no_data", zh));
    expect(markup).not.toContain("暂无数据");
    expect(markup).not.toContain(t("home.brief_not_ready_title", zh));
  });

  it("renders needs_input with the backend reason and error with retry", () => {
    const needsInput = render(React.createElement(BriefView, { state: "needs_input", result: { ...readyResult, answer: "请提供 as_of、window_days" }, error: null, ...base }));
    expect(needsInput).toContain(t("home.brief_needs_input_title", zh));
    expect(needsInput).toContain("请提供 as_of、window_days");

    const errored = render(React.createElement(BriefView, { state: "error", result: null, error: "network down", ...base }));
    expect(errored).toContain(t("home.brief_error_title", zh));
    expect(errored).toContain("network down");
  });
});

describe("BriefView (B2: reuse, no second implementation)", () => {
  const source = readFileSync(path.join(__dirname, "BriefView.tsx"), "utf8");
  it("imports every shared explainability component", () => {
    expect(source).toContain('DataTrustBar from "../components/DataTrustBar"');
    expect(source).toContain('ToolChip from "../components/ToolChip"');
    expect(source).toContain('ConfidenceBadge from "../components/ConfidenceBadge"');
    expect(source).toContain('SourceCitation from "../components/SourceCitation"');
    expect(source).toContain('ThinkingTrace from "../components/ThinkingTrace"');
    expect(source).toContain('{ SeverityDot, toSeverity } from "../components/SeverityDot"');
  });

  it("does not re-implement trust, confidence or citation rendering", () => {
    expect(source).not.toContain("ai-confidence-badge");
    expect(source).not.toContain("ai-tool-chip");
    expect(source).not.toContain("ai-source-citation");
    expect(source).not.toContain("data-trust-bar");
  });
});
