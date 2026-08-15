"use client";

import { Alert, Button, Empty, Skeleton, Typography } from "antd";
import { ReloadOutlined } from "@ant-design/icons";
import ConfidenceBadge from "../components/ConfidenceBadge";
import DataTrustBar from "../components/DataTrustBar";
import { SeverityDot, toSeverity } from "../components/SeverityDot";
import SourceCitation from "../components/SourceCitation";
import ThinkingTrace from "../components/ThinkingTrace";
import ToolChip from "../components/ToolChip";
import { t, type Language } from "../lib/i18n";
import { formatSignalValue, signalLabel } from "../operating-pulse/logic";
import { briefAttentionCards, planToThinking, type HomeBriefState } from "./logic";
import type { HomeBriefResult } from "./types";

export interface BriefViewProps {
  state: HomeBriefState;
  result: HomeBriefResult | null;
  error: string | null;
  language: Language;
  onRetry: () => void;
}

/**
 * HOME-002: the auto-generated morning brief. Renders the agent run
 * through the shared explainability components (DESIGN.md §9) — the brief
 * never re-implements trust, confidence, citations or severity rendering.
 */
export default function BriefView({ state, result, error, language, onRetry }: BriefViewProps) {
  if (state === "loading") {
    return (
      <div className="home-brief-state">
        <Skeleton active paragraph={{ rows: 6 }} />
      </div>
    );
  }
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

  const pulse = result?.retail_operations?.pulse;
  const attention = briefAttentionCards(pulse);
  const thinking = planToThinking(result?.agent_plan);
  const sources = result?.sources || [];
  return (
    <div className="home-brief" data-testid="home-brief">
      <DataTrustBar envelope={pulse?.envelope} basis={pulse?.basis} />
      {typeof result?.confidence === "number" && <ConfidenceBadge confidence={result.confidence} />}
      {result?.tool_calls && result.tool_calls.length > 0 && (
        <div className="ai-tool-row">
          {result.tool_calls.map((call, index) => <ToolChip key={index} call={call} />)}
        </div>
      )}
      <Typography.Paragraph className="home-brief-answer">{result?.answer}</Typography.Paragraph>
      {attention.length > 0 && (
        <div className="home-brief-attention">
          <div className="home-brief-attention-title">{t("home.brief_attention", language)}</div>
          {attention.map((card) => (
            <div key={card.store_id} className="home-brief-attention-card">
              <span className="home-brief-attention-rank">#{card.rank}</span>
              <SeverityDot severity={toSeverity(card.severity)} />
              <span className="home-brief-attention-store">{card.store_code} · {card.store_name}</span>
              <span className="home-brief-attention-citation">[{card.rank}]</span>
              <div className="home-brief-attention-signals">
                {card.signals.map((signal) => (
                  <span key={signal.signal_code} className="home-brief-attention-signal">
                    {signalLabel(signal.signal_code, language)} {formatSignalValue(signal.observed_change, signal.unit, card.currency, language)}
                  </span>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
      {thinking && <ThinkingTrace thinking={thinking} />}
      {sources.length > 0 && (
        <div className="home-brief-sources">
          {sources.map((source, index) => (
            <span key={index} className="home-brief-source">
              <span className="home-brief-source-index">[{index + 1}]</span>
              <SourceCitation source={source} />
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
