/**
 * AI-001 可解释组件层测试。
 *
 * A1: 四个组件各有单元测试。
 * A2: ConfidenceBadge 高低置信度渲染可区分，降级原因可见。
 * A3: ThinkingTrace 默认收起。
 * A4: 文案走 t()（断言 zh-CN 文案）。
 */
import { describe, expect, it } from "vitest";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import ToolChip from "./ToolChip";
import ThinkingTrace from "./ThinkingTrace";
import SourceCitation from "./SourceCitation";
import ConfidenceBadge, { confidenceBand } from "./ConfidenceBadge";
import { LanguageProvider } from "../context/LanguageContext";
import { t } from "../lib/i18n";

function render(children: React.ReactNode) {
  return renderToStaticMarkup(React.createElement(LanguageProvider, null, children));
}

describe("ToolChip", () => {
  it("renders tool name, status and data volume from the output summary", () => {
    const html = render(React.createElement(ToolChip, { call: { tool: "retail.operating_pulse.read", status: "completed", output_summary: "42 rows" } }));
    expect(html).toContain("retail.operating_pulse.read");
    expect(html).toContain(t("ai.tool.completed", "zh-CN"));
    expect(html).toContain(t("ai.tool.output_chars", "zh-CN").replace("{n}", "7"));
  });

  it("renders a failed status distinctly", () => {
    const html = render(React.createElement(ToolChip, { call: { tool: "contract.get", status: "failed" } }));
    expect(html).toContain(t("ai.tool.failed", "zh-CN"));
    expect(html).toContain("data-status=\"failed\"");
  });

  it("renders duration when the backend supplies it, never fabricates one", () => {
    const withDuration = render(React.createElement(ToolChip, { call: { tool: "t", status: "completed", duration_ms: 320 } }));
    expect(withDuration).toContain("320ms");
    const without = render(React.createElement(ToolChip, { call: { tool: "t", status: "completed" } }));
    expect(without).not.toMatch(/\d+ms/);
  });
});

describe("ThinkingTrace", () => {
  it("A3: is collapsed by default and expands on demand", () => {
    const html = render(React.createElement(ThinkingTrace, { thinking: "step 1\nstep 2" }));
    expect(html).toContain("aria-expanded=\"false\"");
    expect(html).not.toContain("step 1");
    expect(html).toContain(t("ai.thinking_process", "zh-CN"));
  });

  it("renders nothing when there is no thinking text", () => {
    expect(render(React.createElement(ThinkingTrace, { thinking: "" }))).toBe("");
  });
});

describe("SourceCitation", () => {
  it("renders a titled citation and links back to the evidence URL", () => {
    const html = render(React.createElement(SourceCitation, { source: { title: "retail_store_day_facts 2026-06", url: "/api/v1/retail/kpis/store-days" } }));
    expect(html).toContain("retail_store_day_facts 2026-06");
    expect(html).toContain("href=\"/api/v1/retail/kpis/store-days\"");
  });

  it("falls back to type/id and never renders anonymous", () => {
    const html = render(React.createElement(SourceCitation, { source: { type: "contract", id: "c-1" } }));
    expect(html).toContain("c-1");
  });

  it("accepts plain strings", () => {
    const html = render(React.createElement(SourceCitation, { source: "lease.contract.read" }));
    expect(html).toContain("lease.contract.read");
  });
});

describe("ConfidenceBadge", () => {
  it("A2: 0.40 and 0.90 render in different bands with different labels", () => {
    const low = render(React.createElement(ConfidenceBadge, { confidence: 0.4 }));
    const high = render(React.createElement(ConfidenceBadge, { confidence: 0.9 }));
    expect(low).toContain("is-low");
    expect(high).toContain("is-high");
    expect(low).toContain(t("ai.confidence.low", "zh-CN"));
    expect(high).toContain(t("ai.confidence.high", "zh-CN"));
    expect(low).not.toBe(high);
  });

  it("A2: shows the degradation reason when present", () => {
    const html = render(React.createElement(ConfidenceBadge, { confidence: 0.4, reason: "missing_reconciliation" }));
    expect(html).toContain(`${t("ai.confidence.reason", "zh-CN")}: missing_reconciliation`);
  });

  it("clamps out-of-range values and bands the boundary", () => {
    expect(confidenceBand(0.5)).toBe("low");
    expect(confidenceBand(0.6)).toBe("medium");
    expect(confidenceBand(0.8)).toBe("high");
    const html = render(React.createElement(ConfidenceBadge, { confidence: 1.4 }));
    expect(html).toContain("100%");
  });
});
