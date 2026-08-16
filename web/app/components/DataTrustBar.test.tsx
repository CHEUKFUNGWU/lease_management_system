/**
 * ENV-002 DataTrustBar tests.
 *
 * D2: decision_ready=false degrades the bar (is-degraded class) and the
 *     KPI badge renders.
 * D3: a null coverage rate renders "—", never 0.
 * D4: copy comes from t() — the zh-CN copies are asserted here.
 */
import { describe, expect, it } from "vitest";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import DataTrustBar, { KPIReadyBadge } from "./DataTrustBar";
import type { SourceEnvelope } from "../lib/api";
import { t } from "../lib/i18n";
import { LanguageProvider } from "../context/LanguageContext";

function renderBar(envelopeValue: SourceEnvelope) {
  return renderToStaticMarkup(
    React.createElement(LanguageProvider, null, React.createElement(DataTrustBar, { envelope: envelopeValue }))
  );
}

function envelope(overrides: Partial<SourceEnvelope> = {}): SourceEnvelope {
  return {
    data_classification: "production",
    source_systems: ["pos"],
    dataset_versions: [],
    fact_version_min: 1,
    fact_version_max: 3,
    current_coverage: { requested_date_from: "2026-01-01", requested_date_to: "2026-01-07", observed_store_days: 7, expected_store_days: 7, coverage_rate: 100 },
    comparison_coverage: { requested_date_from: "2025-12-25", requested_date_to: "2025-12-31", observed_store_days: 7, expected_store_days: 7, coverage_rate: 100 },
    decision_ready: true,
    formula_version: "retail-kpi-v1",
    pulse_version: "retail-pulse-v1",
    semantic_version: "retail-envelope-v1",
    generated_at: "2026-01-08T00:00:00Z",
    ...overrides,
  };
}

describe("DataTrustBar", () => {
  it("D2: decision_ready=false degrades the bar and shows the reason", () => {
    const html = renderBar(envelope({ decision_ready: false, decision_ready_reason: "incomplete_store_day_coverage" }));
    expect(html).toContain("data-trust-bar is-degraded");
    expect(html).toContain(t("trust.not_ready", "zh-CN"));
    expect(html).toContain(t("trust.reason.incomplete_store_day_coverage", "zh-CN"));
  });

  it("D2: decision_ready=true renders the ready state without degradation", () => {
    const html = renderBar(envelope());
    expect(html).not.toContain("is-degraded");
    expect(html).toContain(t("trust.ready", "zh-CN"));
  });

  it("D3: a null coverage rate renders as —, never 0", () => {
    const html = renderBar(
      envelope({
        current_coverage: { requested_date_from: "2026-01-01", requested_date_to: "2026-01-07", observed_store_days: 3, expected_store_days: 7, coverage_rate: null },
      })
    );
    expect(html).toContain("3/7");
    expect(html).toContain("· —");
    expect(html).not.toContain("· 0%");
  });

  it("D1: simulated classification carries the status dot and simulated label", () => {
    const html = renderBar(envelope({ data_classification: "simulated" }));
    expect(html).toContain("trust-classification-dot is-simulated");
    expect(html).toContain(t("trust.classification_simulated", "zh-CN"));
  });

  it("shows comparison coverage when the envelope carries one", () => {
    const html = renderBar(envelope());
    expect(html).toContain(`${t("trust.comparison", "zh-CN")} 7/7`);
  });

  it("FIX-006: uncontrolled mode keeps the built-in toggle", () => {
    const html = renderBar(envelope());
    expect(html).toContain("data-trust-bar-toggle");
    expect(html).toContain(t("trust.expand", "zh-CN"));
  });

  it("FIX-006: controlled mode hides the bar's own toggle (one expander wins)", () => {
    const html = renderToStaticMarkup(
      React.createElement(
        LanguageProvider,
        null,
        React.createElement(DataTrustBar, { envelope: envelope(), expanded: false, onToggle: () => {} })
      )
    );
    expect(html).not.toContain("data-trust-bar-toggle");
  });
});

describe("KPIReadyBadge", () => {
  it("D2: renders the view-only badge with the t() copy", () => {
    const html = renderToStaticMarkup(
      React.createElement(LanguageProvider, null, React.createElement(KPIReadyBadge))
    );
    expect(html).toContain("kpi-not-ready-badge");
    expect(html).toContain(t("trust.kpi_not_ready", "zh-CN"));
  });
});
