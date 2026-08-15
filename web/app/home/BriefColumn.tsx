"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Button, Input, Space, Typography } from "antd";
import { SendOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import ConfidenceBadge from "../components/ConfidenceBadge";
import DataTrustBar from "../components/DataTrustBar";
import SourceCitation from "../components/SourceCitation";
import ToolChip from "../components/ToolChip";
import { aiChatApi, apiErrorMessage, retailAnalyticsApi } from "../lib/api";
import { t, type Language } from "../lib/i18n";
import { latestAnomalyDate } from "../operating-pulse/logic";
import { createLatestRequestGate } from "../operating-pulse/requestGate";
import BriefBand from "./BriefBand";
import { resetHomeBriefCache, runHomeBrief } from "./briefGate";
import { buildBriefFilters, classifyHomeBrief, type HomeBriefState } from "./logic";
import type { HomeBriefResult, HomeChatMessage } from "./types";

export interface BriefColumnProps {
  token: string | null;
  language: Language;
  /** HOME-003: every assistant run may carry an action proposal. */
  onProposal?: (response: HomeBriefResult) => void;
}

function FollowUpMessage({ message, language }: { message: HomeChatMessage; language: Language }) {
  if (message.role === "user") {
    return <div className="home-followup is-user">{message.content}</div>;
  }
  if (message.error) {
    return <div className="home-followup is-assistant is-error">{message.error}</div>;
  }
  const response = message.response;
  if (!response) return null;
  return (
    <div className="home-followup is-assistant">
      {response.tool_calls && response.tool_calls.length > 0 && (
        <div className="ai-tool-row">
          {response.tool_calls.map((call, index) => <ToolChip key={index} call={call} />)}
        </div>
      )}
      {typeof response.confidence === "number" && <ConfidenceBadge confidence={response.confidence} />}
      {response.retail_operations?.pulse && <DataTrustBar envelope={response.retail_operations.pulse.envelope} basis={response.retail_operations.pulse.basis} />}
      <Typography.Paragraph>{response.answer}</Typography.Paragraph>
      {response.sources && response.sources.length > 0 && (
        <div className="ai-tool-row">
          {response.sources.map((source, index) => <SourceCitation key={index} source={source} />)}
        </div>
      )}
    </div>
  );
}

/**
 * HOME-002: the middle column for analysis roles. On first mount it runs
 * the morning brief through the existing retail Agent path (page load auto
 * run, §1.1), then keeps a follow-up composer below it. Follow-ups use the
 * exact RetailAIDrawer call path (aiChatApi.chat + page context) and the
 * shared explainability components.
 */
export default function BriefColumn({ token, language, onProposal }: BriefColumnProps) {
  const [brief, setBrief] = useState<HomeBriefResult | null>(null);
  const [state, setState] = useState<HomeBriefState>("loading");
  const [error, setError] = useState<string | null>(null);
  const [filters, setFilters] = useState<Record<string, string>>({});
  const [messages, setMessages] = useState<HomeChatMessage[]>([]);
  const [input, setInput] = useState("");
  const [sending, setSending] = useState(false);
  const [runNonce, setRunNonce] = useState(0);
  const gate = useRef(createLatestRequestGate());
  // The parent may pass an inline callback; keep the run effect stable.
  const onProposalRef = useRef(onProposal);
  useEffect(() => {
    onProposalRef.current = onProposal;
  }, [onProposal]);

  const runBrief = useCallback(async () => {
    if (!token) return;
    const id = gate.current.begin();
    setState("loading");
    setError(null);
    try {
      const latest = await retailAnalyticsApi.latestSimulationDataset(token);
      const classification: "production" | "simulated" = latest?.data ? "simulated" : "production";
      const nextFilters = buildBriefFilters(
        classification,
        latest?.data?.dataset_version,
        latest?.data ? latestAnomalyDate(latest.data) : dayjs().format("YYYY-MM-DD"),
        7,
        classification === "simulated" ? "retail_simulator" : undefined,
      );
      const result = await runHomeBrief({
        token,
        language,
        message: t("home.brief_prompt", language),
        title: t("home.brief_title", language),
        filters: nextFilters,
      });
      gate.current.commit(id, () => {
        setFilters(nextFilters);
        setBrief(result);
        setState(classifyHomeBrief(result, null));
        onProposalRef.current?.(result);
      });
    } catch (runError) {
      gate.current.commit(id, () => {
        setError(apiErrorMessage(runError));
        setState(classifyHomeBrief(null, "error"));
      });
    }
  }, [token, language]);

  useEffect(() => {
    if (!token) return;
    runBrief();
  }, [token, runBrief, runNonce]);

  const retry = () => {
    resetHomeBriefCache();
    setRunNonce((value) => value + 1);
  };

  const send = async () => {
    const message = input.trim();
    if (!message || sending || !token) return;
    setInput("");
    setSending(true);
    setMessages((current) => [...current, { role: "user", content: message }]);
    try {
      const response = (await aiChatApi.chat(
        {
          message,
          language,
          skill_id: "retail_operations",
          skill_version: "v1",
          page_context: { page: "home", title: t("home.brief_title", language), filters },
        },
        token,
      )) as HomeBriefResult;
      setMessages((current) => [...current, { role: "assistant", content: response.answer, response }]);
      onProposal?.(response);
    } catch (sendError) {
      setMessages((current) => [...current, { role: "assistant", content: "", error: apiErrorMessage(sendError) }]);
    } finally {
      setSending(false);
    }
  };

  return (
    <div className="home-brief-column">
      {/* HOME-004: the brief is one compact strip above the conversation,
          no longer the page body. */}
      <BriefBand state={state} result={brief} error={error} language={language} onRetry={retry} />
      {messages.length > 0 && (
        <div className="home-followups">
          {messages.map((message, index) => <FollowUpMessage key={index} message={message} language={language} />)}
        </div>
      )}
      <Space.Compact className="home-brief-composer">
        <Input
          value={input}
          onChange={(event) => setInput(event.target.value)}
          onPressEnter={send}
          placeholder={t("ai.drawer.placeholder", language)}
          disabled={sending}
        />
        <Button type="primary" icon={<SendOutlined />} onClick={send} loading={sending}>
          {t("ai.drawer.send", language)}
        </Button>
      </Space.Compact>
    </div>
  );
}
