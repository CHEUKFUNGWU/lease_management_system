"use client";

import { Alert, Button, Empty, Typography } from "antd";
import { ReloadOutlined } from "@ant-design/icons";
import DataTrustBar from "../components/DataTrustBar";
import { t, type Language } from "../lib/i18n";
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
 */
export default function BriefView({ state, result, error, language, onRetry }: BriefViewProps) {
  if (state === "error") {
    return (
      <Alert
        type="error"
        showIcon
        message={t("home.brief_error_title", language)}
        description={error || ""}
        action={<Button size="small" icon={<ReloadOutlined />} onClick={onRetry}>{t("common.retry", language)}</Button>}
      />
    );
  }
  if (state === "scope_denied") {
    // B4: scope_denied must stay honest — never softened into "no data".
    return (
      <Alert
        type="error"
        showIcon
        className="home-brief-state"
        message={t("api.scope_denied", language)}
        description={result?.answer || undefined}
      />
    );
  }
  if (state === "no_data") {
    return (
      <div className="home-brief-state">
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("home.brief_no_data", language)} />
      </div>
    );
  }
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
  if (state === "needs_input") {
    return (
      <div className="home-brief-state">
        <Alert
          type="warning"
          showIcon
          message={t("home.brief_needs_input_title", language)}
          description={result?.answer || undefined}
        />
      </div>
    );
  }
  // loading / ready are rendered by BriefBand; nothing left to do here.
  return null;
}
