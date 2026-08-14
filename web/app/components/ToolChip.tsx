"use client";

import React from "react";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { StatusTag } from "./StatusTag";

// DESIGN.md §9: one tool call from the run events — tool name, data volume
// and duration. Duration is rendered only when the backend supplies it; a
// missing duration is never fabricated.
export interface ToolCallLike {
  tool: string;
  status: string;
  input_summary?: string;
  output_summary?: string;
  requires_review?: boolean;
  duration_ms?: number;
}

function dataVolume(summary: string | undefined): number {
  return summary ? summary.length : 0;
}

export default function ToolChip({ call }: { call: ToolCallLike }) {
  const { language } = useLanguage();
  const failed = call.status === "failed";
  const review = call.requires_review || call.status === "needs_review";
  const kind = failed ? "error" : review ? "warning" : "neutral";
  return (
    <span className="ai-tool-chip" data-status={call.status}>
      <StatusTag kind={kind} className="ai-tool-chip-status">
        {failed ? t("ai.tool.failed", language) : review ? t("ai.tool.needs_review", language) : t("ai.tool.completed", language)}
      </StatusTag>
      <span className="ai-tool-chip-name">{call.tool}</span>
      {dataVolume(call.output_summary) > 0 && (
        <span className="ai-tool-chip-meta">
          {t("ai.tool.output_chars", language).replace("{n}", String(dataVolume(call.output_summary)))}
        </span>
      )}
      {typeof call.duration_ms === "number" && call.duration_ms >= 0 && (
        <span className="ai-tool-chip-meta">
          {t("ai.tool.duration", language).replace("{ms}", String(call.duration_ms))}
        </span>
      )}
    </span>
  );
}
