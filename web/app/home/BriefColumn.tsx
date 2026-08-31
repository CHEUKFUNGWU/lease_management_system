"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Button, Input, Space, Typography } from "antd";
import { RobotOutlined, SendOutlined, UserOutlined } from "@ant-design/icons";
import { Avatar } from "antd";
import dayjs from "dayjs";
import ConfidenceBadge from "../components/ConfidenceBadge";
import SourceCitation from "../components/SourceCitation";
import MarkdownText from "../ai-chat/MarkdownText";
import ToolChip from "../components/ToolChip";
import { apiErrorMessage, retailAnalyticsApi } from "../lib/api";
import { t, type Language } from "../lib/i18n";
import { latestAnomalyDate } from "../operating-pulse/logic";
import { createLatestRequestGate } from "../operating-pulse/requestGate";
import { useAIChatRuntime, generateRuntimeId } from "../ai-chat/runtime";
import BriefBand from "./BriefBand";
import { resetHomeBriefCache, runHomeBrief } from "./briefGate";
import { buildBriefFilters, classifyHomeBrief, type HomeBriefState } from "./logic";
import type { HomeBriefResult } from "./types";

export interface BriefColumnProps {
  token: string | null;
  language: Language;
  /** HOME-003: every assistant run may carry an action proposal. */
  onProposal?: (response: HomeBriefResult) => void;
}

// Stage 5 (D-E3): the home conversation now runs on the unified chat
// runtime. Bubbles keep the home-local classes (contract: chatLayout.test
// asserts the literal strings) but the stream, sessions and artifacts are
// the /ai-chat runtime's. The morning brief band is unchanged: it stays the
// system-initiated one-shot summary above the thread.
const homeSessionKey = "retail-ai-home-session";

function loadHomeSession(): string {
  try {
    return window.sessionStorage.getItem(homeSessionKey) || "";
  } catch {
    return "";
  }
}

function saveHomeSession(sessionId: string) {
  try {
    window.sessionStorage.setItem(homeSessionKey, sessionId);
  } catch {
    // The mapping lives in component state only.
  }
}

function ChatMessage({
  id,
  role,
  content,
  error,
  toolCalls,
  confidence,
  agentPlan,
  sources,
  pending,
  language,
}: {
  id: string;
  role: "user" | "assistant";
  content: string;
  error?: string;
  toolCalls?: Array<{ tool: string; status: string; requires_review?: boolean }>;
  confidence?: number;
  agentPlan?: Array<{ id: string; title: string; status: string }>;
  sources?: Array<{ title?: string; url?: string; type?: string }>;
  pending?: boolean;
  language: Language;
}) {
  if (role === "user") {
    return (
      <div className="home-msg is-user">
        <Avatar icon={<UserOutlined />} className="home-msg-avatar" />
        <div className="home-msg-bubble is-user">{content}</div>
      </div>
    );
  }
  if (error) {
    return (
      <div className="home-msg is-assistant">
        <Avatar icon={<RobotOutlined />} className="home-msg-avatar" />
        <div className="home-msg-bubble is-assistant home-msg-error">{error}</div>
      </div>
    );
  }
  return (
    <div className="home-msg is-assistant" key={id}>
      <Avatar icon={<RobotOutlined />} className="home-msg-avatar" />
      <div className="home-msg-bubble is-assistant">
        {pending && !content && (
          <>
            <div className="home-chat-steps" aria-hidden="true">
              <div className="home-chat-step is-pending" />
              <div className="home-chat-step is-pending" />
              <div className="home-chat-step is-pending" />
            </div>
            <span className="home-chat-pending-text">{t("home.chat_thinking", language)}</span>
          </>
        )}
        {(toolCalls?.length || typeof confidence === "number") && (
          <div className="ai-tool-row">
            {toolCalls?.map((call, index) => <ToolChip key={index} call={call} />)}
            {typeof confidence === "number" && <ConfidenceBadge confidence={confidence} />}
          </div>
        )}
        {/* FIX-013: the plan steps unfold in order once the answer arrives. */}
        {agentPlan && agentPlan.length > 0 && (
          <ol className="home-chat-steps">
            {agentPlan.map((step) => (
              <li key={step.id} className={`home-chat-step is-${step.status === "completed" ? "done" : step.status === "failed" ? "failed" : "pending"}`}>
                <span className="home-chat-step-mark" aria-hidden="true">
                  {step.status === "completed" ? "✓" : step.status === "failed" ? "✗" : "…"}
                </span>
                <span>{step.title}</span>
              </li>
            ))}
          </ol>
        )}
        {content && <div className="home-chat-answer"><MarkdownText content={content} /></div>}
        {sources && sources.length > 0 && (
          <div className="ai-tool-row">
            {sources.map((source, index) => <SourceCitation key={index} source={source} />)}
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
 * the morning brief through the existing retail Agent path. HOME-004 flips
 * the hierarchy: the brief is one compact band on top and the conversation
 * below is the page body, with the composer pinned to the column bottom.
 */
export default function BriefColumn({ token, language, onProposal }: BriefColumnProps) {
  const [brief, setBrief] = useState<HomeBriefResult | null>(null);
  const [state, setState] = useState<HomeBriefState>("loading");
  const [error, setError] = useState<string | null>(null);
  const [filters, setFilters] = useState<Record<string, string>>({});
  const [input, setInput] = useState("");
  const [runNonce, setRunNonce] = useState(0);
  const gate = useRef(createLatestRequestGate());
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const onProposalRef = useRef(onProposal);
  useEffect(() => {
    onProposalRef.current = onProposal;
  }, [onProposal]);

  const runtime = useAIChatRuntime({ token, language, selectedModel: "deepseek-v4-flash" });
  const [homeSessionId, setHomeSessionId] = useState<string>("");
  const adoptedHome = useRef(false);
  useEffect(() => {
    if (!runtime.localHydrated || adoptedHome.current) return;
    adoptedHome.current = true;
    const mapped = loadHomeSession();
    const known = runtime.sessions.find((session) => session.id === mapped);
    if (known) {
      runtime.setActiveSessionId(known.id);
      setHomeSessionId(known.id);
      return;
    }
    const session = runtime.createNewSession();
    saveHomeSession(session.id);
    setHomeSessionId(session.id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [runtime.localHydrated]);

  const homeSession = runtime.sessions.find((item) => item.id === homeSessionId) || null;

  const runBrief = useCallback(async () => {
    if (!token) return;
    const id = gate.current.begin();
    setState("loading");
    setError(null);
    try {
      let latestData = null;
      try {
        const latest = await retailAnalyticsApi.latestSimulationDataset(token);
        latestData = latest?.data || null;
      } catch {
        latestData = null;
      }
      if (!latestData) {
        try {
          const gen = await retailAnalyticsApi.generateDefaultSimulation(token);
          if (gen?.dataset_version) {
            latestData = {
              dataset_id: gen.dataset_id,
              dataset_version: gen.dataset_version,
              as_of: gen.date_to || "2026-06-05",
              anomalies: gen.anomaly_manifest || [],
            } as any;
          }
        } catch {
          // ignore
        }
      }
      const classification: "production" | "simulated" = "simulated";
      const datasetVersion = latestData?.dataset_version || "retail-sim-v1-2853d653";
      const asOf = latestData ? latestAnomalyDate(latestData) : "2026-06-05";
      const nextFilters = buildBriefFilters(
        classification,
        datasetVersion,
        asOf,
        7,
        "retail_simulator",
      );
      let result: HomeBriefResult;
      try {
        result = await runHomeBrief({
          token,
          language,
          message: t("home.brief_prompt", language),
          title: t("home.brief_title", language),
          filters: nextFilters,
        });
      } catch (aiErr) {
        try {
          const pulse = await retailAnalyticsApi.operatingPulse({
            data_classification: classification,
            dataset_version: datasetVersion,
            as_of: asOf,
            window_days: 7,
            source_system: "retail_simulator",
          }, token);
          result = {
            answer: language === "en" ? "Today's retail operating pulse is ready." : "今日零售经营脉搏数据已就绪。",
            retail_operations: { pulse },
            sources: [{ type: "retail_kpi_dataset", title: datasetVersion }],
          };
        } catch {
          throw aiErr;
        }
      }
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

  const messages = homeSession?.messages || [];
  const sending = runtime.loading && Boolean(homeSession?.currentRunId);

  useEffect(() => {
    if (messages.length === 0) return;
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages.length, sending]);

  const retry = () => {
    resetHomeBriefCache();
    setRunNonce((value) => value + 1);
  };

  const emitHomeProposals = (sessionId: string) => {
    const session = runtime.sessions.find((item) => item.id === sessionId);
    if (!session || !onProposalRef.current) return;
    for (const message of session.messages) {
      for (const artifact of message.artifacts || []) {
        if (artifact.artifact_type === "retail_action_proposal" && artifact.data) {
          onProposalRef.current?.({ answer: message.content, retail_action_proposal: artifact.data } as HomeBriefResult);
        }
      }
    }
  };

  const sendText = async (text: string) => {
    const message = text.trim();
    if (!message || !homeSession) return;
    setInput("");
    runtime.updateSessionMessages(homeSession.id, (current) => [
      ...current,
      { id: generateRuntimeId(), role: "user" as const, content: message, timestamp: Date.now() },
    ]);
    try {
      const serverSessionId = await runtime.ensureServerSession(homeSession.id, {});
      await runtime.createAndStartRun(homeSession.id, serverSessionId, {
        message,
        language,
        skill_id: "retail_operations",
        skill_version: "v1",
        page_context: { page: "home", title: t("home.brief_title", language), filters },
      });
      emitHomeProposals(homeSession.id);
    } catch (sendError) {
      runtime.updateSessionMessages(homeSession.id, (current) => [
        ...current,
        { id: generateRuntimeId(), role: "assistant" as const, content: "", timestamp: Date.now() } as never,
      ].concat([{ id: generateRuntimeId(), role: "assistant" as const, content: apiErrorMessage(sendError), timestamp: Date.now() }]));
    } finally {
      runtime.setLoading(false);
    }
  };

  const send = () => sendText(input);

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
              {HOME_STARTER_KEYS.map((key) => (
                <Button key={key} size="small" className="home-chat-starter-chip" onClick={() => sendText(t(key, language))}>
                  {t(key, language)}
                </Button>
              ))}
            </div>
          </div>
        )}
        <div className="home-chat-messages">
          {messages.map((message) => (
            <ChatMessage
              key={message.id || message.timestamp}
              id={message.id || String(message.timestamp)}
              role={message.role as "user" | "assistant"}
              content={message.content || ""}
              toolCalls={message.toolCalls}
              confidence={typeof message.confidence === "number" ? message.confidence : undefined}
              agentPlan={message.agentPlan}
              sources={message.sources?.filter((source) => typeof source !== "string").map((source) => ({
                title: (source as { title?: string }).title,
                url: (source as { url?: string }).url,
                type: (source as { type?: string }).type,
              })) || undefined}
              pending={runtime.typingMessageId === message.id}
              language={language}
            />
          ))}
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
