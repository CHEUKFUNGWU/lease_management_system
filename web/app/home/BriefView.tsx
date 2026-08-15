"use client";

import { Empty, Typography } from "antd";
import DataTrustBar from "../components/DataTrustBar";
import { StateBlock } from "../components/StateBlock";
import { t, type Language } from "../lib/i18n";
import type { DataState } from "../lib/dataState";
import type { HomeBriefState } from "./logic";
import type { HomeBriefResult } from "./types";

export interface BriefViewProps {
  state: HomeBriefState;
  result: HomeBriefResult | null;
  error: string | null;
  language: Language;
  onRetry: () => void;
}

/**
 * HOME-002 introduced this view for the auto-run brief; HOME-004 demoted the
 * brief to a band (BriefBand) and the band now renders the ready state.
 * What remains here are the degraded presentations — loading is handled by
 * the band, and scope_denied is never softened into "no data" (AGENTS.md:
 * 权限拒绝必须保持原因).
 *
 * STATE-003: the degraded states map onto the shared DataState kinds and
 * render through StateBlock; not_decision_ready keeps its DataTrustBar
 * preamble (the trust evidence must stay visible).
 */
export default function BriefView({ state, result, error, language, onRetry }: BriefViewProps) {
  const blockState: DataState<unknown> =
    state === "error"
      ? { kind: "failed", message: t("home.brief_error_title", language), reason: error || undefined }
      : state === "scope_denied"
        ? { kind: "scope_denied", message: t("api.scope_denied", language), reason: result?.answer || undefined }
        : state === "no_data"
          ? { kind: "empty", reason: t("home.brief_no_data", language) }
          : state === "needs_input"
            ? { kind: "actionable", message: t("home.brief_needs_input_title", language), reason: result?.answer || undefined }
            : { kind: "ready" };

  if (state === "not_decision_ready") {
    return (
      <div className="home-brief-state">
        {result?.retail_operations?.pulse && <DataTrustBar envelope={result.retail_operations.pulse.envelope} basis={result.retail_operations.pulse.basis} />}
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={
            <Typography.Text type="secondary">
              {t("home.brief_not_ready_title", language)}
              <br />
              {t("home.brief_not_ready", language)}
            </Typography.Text>
          }
        />
      </div>
    );
  }

  if (blockState.kind === "ready") return null;

  return (
    <div className="home-brief-state">
      <StateBlock state={blockState} language={language} onRetry={state === "error" ? onRetry : undefined} />
    </div>
  );
}
