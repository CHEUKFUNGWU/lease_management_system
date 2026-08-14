"use client";

import React from "react";
import { Tooltip } from "antd";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";

// DESIGN.md §9: confidence with a visible reason. 0.40 and 0.90 must be
// distinguishable — the badge uses three bands with different dot colors,
// label text and border treatment, so color is never the only signal.
export type ConfidenceBand = "low" | "medium" | "high";

export function confidenceBand(confidence: number): ConfidenceBand {
  if (confidence < 0.6) return "low";
  if (confidence < 0.8) return "medium";
  return "high";
}

export default function ConfidenceBadge({ confidence, reason }: { confidence: number; reason?: string }) {
  const { language } = useLanguage();
  const clamped = Math.min(1, Math.max(0, confidence));
  const band = confidenceBand(clamped);
  const labelKey = band === "low" ? "ai.confidence.low" : band === "medium" ? "ai.confidence.medium" : "ai.confidence.high";
  // The reason rides on a native title attribute: it is visible to
  // assistive tech and to static rendering, and cannot be lost to a portal.
  const reasonTitle = reason ? `${t("ai.confidence.reason", language)}: ${reason}` : undefined;
  return (
    <span className={`ai-confidence-badge is-${band}`} data-band={band} title={reasonTitle}>
      <span className="ai-confidence-dot" aria-hidden="true" />
      <span className="ai-confidence-text">
        {t(labelKey, language)} · {Math.round(clamped * 100)}%
      </span>
    </span>
  );
}
