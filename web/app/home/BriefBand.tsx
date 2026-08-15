"use client";

import { useState } from "react";
import { Button, Skeleton } from "antd";
import ConfidenceBadge from "../components/ConfidenceBadge";
import DataTrustBar from "../components/DataTrustBar";
import { SeverityDot, toSeverity } from "../components/SeverityDot";
import SourceCitation from "../components/SourceCitation";
import ThinkingTrace from "../components/ThinkingTrace";
import ToolChip from "../components/ToolChip";
import { t, type Language } from "../lib/i18n";
import { formatChange, formatKPIValue, formatSignalValue, kpiLabel, signalLabel, changeTone } from "../operating-pulse/logic";
import BriefView from "./BriefView";
import { briefAttentionCards, planToThinking, type HomeBriefState } from "./logic";
import type { HomeBriefResult } from "./types";

export interface BriefBandProps {
  state: HomeBriefState;
  result: HomeBriefResult | null;
  error: string | null;
  language: Language;
  onRetry: () => void;
}

const BAND_KPI_CODES = ["revenue", "gross_margin_rate", "store_contribution"] as const;

/**
 * HOME-004 §2: the brief demoted from the page body to one compact,
 * expandable context strip above the chat. The collapsed strip answers
 * "how trustworthy is today and how many stores need attention" in one line
 * (title · attention count · the DataTrustBar summary); the expanded body
 * adds KPI cards, structured attention cards and the agent trace.
 *
 * The machine-readable `answer` (backend fmt.Sprintf context dump) is no
 * longer rendered as page content — it moves into <ThinkingTrace> together
 * with the plan, exactly like §2.2 demands. Degraded states keep their
 * dedicated presentations via BriefView (scope_denied is never softened).
 */
export default function BriefBand({ state, result, error, language, onRetry }: BriefBandProps) {
  const [expanded, setExpanded] = useState(false);
  const pulse = result?.retail_operations?.pulse;
  const attention = briefAttentionCards(pulse);

  if (state === "loading") {
    return (
      <div className="home-brief-band" data-testid="home-brief-band">
        <div className="home-brief-band-header">
          <span className="home-brief-band-title">{t("home.brief_title", language)}</span>
          <Skeleton active paragraph={{ rows: 1 }} className="home-brief-band-skeleton" />
        </div>
      </div>
    );
  }

  // Degraded states stay honest and compact: BriefView keeps the dedicated
  // no-data / not-ready / needs-input / scope_denied / error presentations.
  if (state !== "ready") {
    return (
      <div className="home-brief-band" data-testid="home-brief-band">
        <div className="home-brief-band-header">
          <span className="home-brief-band-title">{t("home.brief_title", language)}</span>
        </div>
        <BriefView state={state} result={result} error={error} language={language} onRetry={onRetry} />
      </div>
    );
  }

  const summary = pulse?.summary || {};
  const currency = pulse?.currency || "";
  const machineTrace = [planToThinking(result?.agent_plan), result?.answer].filter(Boolean).join("\n\n");
  const sources = result?.sources || [];

  return (
    <div className="home-brief-band" data-testid="home-brief-band">
      <div className="home-brief-band-header">
        <span className="home-brief-band-title">{t("home.brief_title", language)}</span>
        <span className="home-brief-band-count">{t("home.band_attention_count", language, { count: String(attention.length) })}</span>
        <Button
          type="text"
          size="small"
          className="home-brief-band-toggle"
          onClick={() => setExpanded(!expanded)}
          aria-expanded={expanded}
        >
          <span className="home-brief-band-arrow" aria-hidden="true">{expanded ? "▼" : "▶"}</span>
          {expanded ? t("trust.collapse", language) : t("trust.expand", language)}
        </Button>
      </div>
      {/* The trust bar IS the collapsed line (classification · coverage ·
          decision-ready); one toggle drives the bar detail and the band body
          together so the two never disagree. */}
      <DataTrustBar
        envelope={pulse?.envelope}
        basis={pulse?.basis}
        expanded={expanded}
        onToggle={() => setExpanded(!expanded)}
      />
      {expanded && (
        <div className="home-brief-band-body">
          <div className="home-brief-band-kpis">
            {BAND_KPI_CODES.map((code) => {
              const metric = summary[code];
              const tone = changeTone(code, metric);
              return (
                <div key={code} className="home-band-kpi">
                  <span className="home-band-kpi-label">{kpiLabel(code, language)}</span>
                  <span className="home-band-kpi-value">{formatKPIValue(metric?.current, currency, language)}</span>
                  <span className={`home-band-kpi-change pulse-change-${tone}`}>{formatChange(metric)}</span>
                </div>
              );
            })}
          </div>
          {attention.length > 0 && (
            <div className="home-band-attention">
              <div className="home-band-attention-title">{t("home.brief_attention", language)}</div>
              {attention.map((card) => (
                <div key={card.store_id} className="home-band-attention-card">
                  <div className="home-band-attention-store">
                    <span className="home-band-attention-rank">#{card.rank}</span>
                    <span className="home-band-attention-storecode">{card.store_code}</span>
                    <span className="home-band-attention-storename">{card.store_name}</span>
                    <span className="home-band-attention-citation">[{card.rank}]</span>
                  </div>
                  <span className="home-band-attention-severity">
                    <SeverityDot severity={toSeverity(card.severity)} />
                    {t(`home.severity_${toSeverity(card.severity)}`, language)}
                  </span>
                  <div className="home-band-attention-signals">
                    {card.signals.map((signal) => (
                      <span key={signal.signal_code} className="home-band-attention-signal">
                        {signalLabel(signal.signal_code, language)} {formatSignalValue(signal.observed_change, signal.unit, card.currency, language)}
                      </span>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )}
          <div className="home-band-meta">
            {/* L4: the confidence badge shares the tool-chip row instead of
                claiming its own line. */}
            {(result?.tool_calls?.length || typeof result?.confidence === "number") && (
              <div className="ai-tool-row">
                {result?.tool_calls?.map((call, index) => <ToolChip key={index} call={call} />)}
                {typeof result?.confidence === "number" && <ConfidenceBadge confidence={result.confidence} />}
              </div>
            )}
            {machineTrace && <ThinkingTrace thinking={machineTrace} />}
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
        </div>
      )}
    </div>
  );
}
