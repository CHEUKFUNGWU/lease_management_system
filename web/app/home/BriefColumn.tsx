"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Button, Input, Space, Typography } from "antd";
import { RobotOutlined, SendOutlined, UserOutlined } from "@ant-design/icons";
import { Avatar } from "antd";
import dayjs from "dayjs";
import ConfidenceBadge from "../components/ConfidenceBadge";
import DataTrustBar from "../components/DataTrustBar";
import SourceCitation from "../components/SourceCitation";
import ThinkingTrace from "../components/ThinkingTrace";
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

/**
 * HOME-004 §3: the home conversation is a real message stream, not an
 * isolated input. Bubbles reuse the /ai-chat visual language (user right /
 * assistant left, avatars, radius 16 with one flattened corner) but render
 * through the shared explainability components — no copy of MessageContent,
 * /ai-chat itself is untouched. The full rendering lives in this module as
 * home-local CSS classes; the machine answer stays a trace, never the body.
 */
function ChatMessage({ message, language }: { message: HomeChatMessage; language: Language }) {
  if (message.role === "user") {
    return (
      <div className="home-msg is-user">
        <Avatar icon={<UserOutlined />} className="home-msg-avatar" />
        <div className="home-msg-bubble is-user">{message.content}</div>
      </div>
    );
  }
  if (message.error) {
    return (
      <div className="home-msg is-assistant">
        <Avatar icon={<RobotOutlined />} className="home-msg-avatar" />
        <div className="home-msg-bubble is-assistant home-msg-error">{message.error}</div>
      </div>
    );
  }
  const response = message.response;
  if (!response) return null;
  const trace = [response.answer].filter(Boolean).join("\n");
  return (
    <div className="home-msg is-assistant">
      <Avatar icon={<RobotOutlined />} className="home-msg-avatar" />
      <div className="home-msg-bubble is-assistant">
        {(response.tool_calls?.length || typeof response.confidence === "number") && (
          <div className="ai-tool-row">
            {response.tool_calls?.map((call, index) => <ToolChip key={index} call={call} />)}
            {typeof response.confidence === "number" && <ConfidenceBadge confidence={response.confidence} />}
          </div>
        )}
        {response.retail_operations?.pulse && (
          <DataTrustBar envelope={response.retail_operations.pulse.envelope} basis={response.retail_operations.pulse.basis} />
        )}
        {/* FIX-013: the plan steps unfold in order once the answer arrives —
            the request is one-shot, so the steps cannot stream, but they
            render progressively instead of flashing whole. */}
        {response.agent_plan && response.agent_plan.length > 0 && (
          <ol className="home-chat-steps">
            {response.agent_plan.map((step) => (
              <li key={step.id} className={`home-chat-step is-${step.status === "completed" ? "done" : step.status === "failed" ? "failed" : "pending"}`}>
                <span className="home-chat-step-mark" aria-hidden="true">
                  {step.status === "completed" ? "✓" : step.status === "failed" ? "✗" : "…"}
                </span>
                <span>{step.title}</span>
              </li>
            ))}
          </ol>
        )}
        {trace && <ThinkingTrace thinking={trace} />}
        {response.sources && response.sources.length > 0 && (
          <div className="ai-tool-row">
            {response.sources.map((source, index) => <SourceCitation key={index} source={source} />)}
          </div>
        )}
      </div>
    </div>
  );
}

/** The empty conversation offers the same starter questions as /ai-chat. */
const HOME_STARTER_KEYS = ["ai.chip_missing_dr", "ai.chip_pending", "ai.chip_expiring"];

/**
 * HOME-002: the middle column for analysis roles. On first mount it runs
 * the morning brief through the existing retail Agent path (page load auto
 * run, §1.1). HOME-004 flips the hierarchy: the brief is one compact band
 * on top and the conversation below is the page body, with the composer
 * pinned to the column bottom.
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
  const messagesEndRef = useRef<HTMLDivElement>(null);
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

  // The newest message scrolls into view like /ai-chat — but only when a
  // conversation exists; on first load the end-of-stream ref is empty and
  // scrolling to it would push the whole page down (FIX-007).
  useEffect(() => {
    if (messages.length === 0) return;
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, sending]);

  const retry = () => {
    resetHomeBriefCache();
    setRunNonce((value) => value + 1);
  };

  const sendText = async (text: string) => {
    const message = text.trim();
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

  const send = () => sendText(input);

  // FIX-015: an empty conversation must not stretch the column to full height,
  // or the composer sits ~800px below the starter chips with nothing between.
  const isEmptyConversation = messages.length === 0 && !sending;

  return (
    <div className="home-chat-column">
      <BriefBand state={state} result={brief} error={error} language={language} onRetry={retry} />
      <div className={isEmptyConversation ? "home-chat-body is-empty" : "home-chat-body"}>
        {isEmptyConversation && (
          <div className="home-chat-starters">
            <Typography.Text type="secondary" className="home-chat-starters-label">
              {t("ai.quick_questions", language)}
            </Typography.Text>
            <div className="home-chat-starter-chips">
              {/* FIX-008: starters send immediately, matching /ai-chat's
                  handleChipClick → handleSend, instead of only filling the
                  composer. */}
              {HOME_STARTER_KEYS.map((key) => (
                <Button key={key} size="small" className="home-chat-starter-chip" onClick={() => sendText(t(key, language))}>
                  {t(key, language)}
                </Button>
              ))}
            </div>
          </div>
        )}
        <div className="home-chat-messages">
          {messages.map((message, index) => <ChatMessage key={index} message={message} language={language} />)}
          {sending && (
            <div className="home-msg is-assistant">
              <Avatar icon={<RobotOutlined />} className="home-msg-avatar" />
              <div className="home-msg-bubble is-assistant home-msg-pending">
                {/* FIX-013: the pending bubble shows the step scaffold so the
                    wait is readable, not a lone spinner text. */}
                <div className="home-chat-steps" aria-hidden="true">
                  <div className="home-chat-step is-pending" />
                  <div className="home-chat-step is-pending" />
                  <div className="home-chat-step is-pending" />
                </div>
                <span className="home-chat-pending-text">{t("home.chat_thinking", language)}</span>
              </div>
            </div>
          )}
          <div ref={messagesEndRef} />
        </div>
      </div>
      <Space.Compact className="home-chat-composer">
        <Input
          value={input}
          onChange={(event) => setInput(event.target.value)}
          onPressEnter={send}
          placeholder={t("ai.drawer.placeholder", language)}
          disabled={sending}
          aria-label={t("ai.drawer.placeholder", language)}
        />
        <Button type="primary" icon={<SendOutlined />} onClick={send} loading={sending}>
          {t("ai.drawer.send", language)}
        </Button>
      </Space.Compact>
    </div>
  );
}
