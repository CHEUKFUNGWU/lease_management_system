"use client";

import React, { useState } from "react";
import { Button, Space } from "antd";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import type { SourceEnvelope, SourceEnvelopeCoverage } from "../lib/api";

// ENV-002: the single trust-rendering component for DESIGN.md §10. It
// consumes the ENV-001 Source Envelope; no page renders provenance any
// other way. decision_ready=false degrades the whole bar and every KPI card
// shows <KPIReadyBadge>; a missing coverage rate renders "—", never 0.

const REASON_KEYS: Record<string, string> = {
  incomplete_store_day_coverage: "trust.reason.incomplete_store_day_coverage",
  not_decision_ready: "trust.reason.not_decision_ready",
  diagnostics_not_decision_ready: "trust.reason.diagnostics_not_decision_ready",
  scenario_not_ready: "trust.reason.scenario_not_ready",
  currency_conflict: "trust.reason.currency_conflict",
  insufficient_peer_count: "trust.reason.insufficient_peer_count",
  data_quality_invalid: "trust.reason.data_quality_invalid",
  no_facts: "trust.reason.no_facts",
  raw_facts_read: "trust.reason.raw_facts_read",
};

function classificationLabel(classification: string, language: ReturnType<typeof useLanguage>["language"]): string {
  if (classification === "simulated") return t("trust.classification_simulated", language);
  if (classification === "mixed") return t("trust.classification_mixed", language);
  return t("trust.classification_production", language);
}

function reasonLabel(reason: string | undefined, language: ReturnType<typeof useLanguage>["language"]): string {
  if (!reason) return "";
  const key = REASON_KEYS[reason];
  return key ? t(key, language) : reason;
}

// D3: coverage rate null renders "—" — a missing signal is never zero-filled.
function coverageText(coverage: SourceEnvelopeCoverage | undefined, language: ReturnType<typeof useLanguage>["language"]): string {
  if (!coverage) return "—";
  const rate = coverage.coverage_rate == null ? "—" : `${coverage.coverage_rate.toFixed(1)}%`;
  return `${coverage.observed_store_days ?? 0}/${coverage.expected_store_days ?? 0} ${t("trust.store_days", language)} · ${rate}`;
}

// The uniform KPI-card badge shown whenever decision_ready is false.
export function KPIReadyBadge() {
  const { language } = useLanguage();
  return <span className="kpi-not-ready-badge">{t("trust.kpi_not_ready", language)}</span>;
}

export default function DataTrustBar({
  envelope,
  basis,
  detailExtra,
  expanded,
  onToggle,
}: {
  envelope?: SourceEnvelope | null;
  basis?: string;
  detailExtra?: React.ReactNode;
  /** HOME-004: when provided (with onToggle) the bar becomes controlled — the
   *  caller drives expansion so one toggle can open the bar and its container
   *  together. Existing callers pass neither and keep the internal state. */
  expanded?: boolean;
  onToggle?: () => void;
}) {
  const { language } = useLanguage();
  const [internalExpanded, setInternalExpanded] = useState(false);
  const controlled = expanded !== undefined || onToggle !== undefined;
  const isOpen = controlled ? Boolean(expanded) : internalExpanded;
  const toggle = () => {
    if (onToggle) onToggle();
    else if (!controlled) setInternalExpanded(!internalExpanded);
  };
  if (!envelope) return null;
  const degraded = !envelope.decision_ready;
  const reason = reasonLabel(envelope.decision_ready_reason, language);
  const comparison = envelope.comparison_coverage && (envelope.comparison_coverage.expected_store_days ?? 0) > 0
    ? coverageText(envelope.comparison_coverage, language)
    : null;
  return (
    <div className={`data-trust-bar${degraded ? " is-degraded" : ""}`}>
      <div className="data-trust-bar-summary">
        <Space size={10} wrap>
          <span className="data-trust-bar-classification">
            <span className={`trust-classification-dot is-${envelope.data_classification}`} aria-hidden="true" />
            {classificationLabel(envelope.data_classification, language)}
          </span>
          {basis && <span>{basis}</span>}
          <span>{coverageText(envelope.current_coverage, language)}</span>
          {comparison && <span>{t("trust.comparison", language)} {comparison}</span>}
          <span className={degraded ? "is-not-ready" : "is-ready"}>
            {degraded ? t("trust.not_ready", language) : t("trust.ready", language)}
          </span>
          {degraded && reason && <span className="data-trust-bar-reason">{reason}</span>}
          <Button type="text" size="small" className="data-trust-bar-toggle" onClick={toggle} aria-expanded={isOpen}>
            {isOpen ? t("trust.collapse", language) : t("trust.expand", language)}
          </Button>
        </Space>
      </div>
      {isOpen && (
        <div className="data-trust-bar-detail">
          <Space size={10} wrap>
            <span>{t("trust.source", language)}: {envelope.source_systems.join(", ") || "—"}</span>
            <span>{t("trust.dataset", language)}: {envelope.dataset_versions.join(", ") || "—"}</span>
            <span>{t("trust.fact_version", language)}: {envelope.fact_version_min}–{envelope.fact_version_max}</span>
            {envelope.highest_as_of && <span>{t("trust.as_of", language)}: {envelope.highest_as_of}</span>}
            <span>{t("trust.formula", language)}: {envelope.formula_version}</span>
            <span>{t("trust.pulse", language)}: {envelope.pulse_version}</span>
            <span>{t("trust.semantic", language)}: {envelope.semantic_version}</span>
            {detailExtra}
          </Space>
        </div>
      )}
    </div>
  );
}
