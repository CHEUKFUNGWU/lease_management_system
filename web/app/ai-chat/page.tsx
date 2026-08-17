"use client";

import { StatusTag, statusKindFromAntColor } from "../components/StatusTag";

import { useState, useRef, useEffect, useMemo, Suspense } from "react";
import { useSearchParams } from "next/navigation";
import { motion, AnimatePresence } from "framer-motion";
import {
  Input,
  Button,
  Avatar,
  Typography,
  Space,
  Tag,
  Spin,
  Upload,
  message,
  Tooltip,
  Dropdown,
  Empty,
  Modal,
  Drawer,
} from "antd";
import {
  SendOutlined,
  UserOutlined,
  RobotOutlined,
  FileTextOutlined,
  PaperClipOutlined,
  FilePdfOutlined,
  FileExcelOutlined,
  FileImageOutlined,
  PlusOutlined,
  DeleteOutlined,
  MoreOutlined,
  CopyOutlined,
  CheckOutlined,
  DownOutlined,
  MessageOutlined,
  ClockCircleOutlined,
  ToolOutlined,
  CheckCircleOutlined,
  ExclamationCircleOutlined,
  CloseCircleOutlined,
} from "@ant-design/icons";
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import { vscDarkPlus } from "react-syntax-highlighter/dist/esm/styles/prism";
import AppLayout from "../components/AppLayout";
import DataTrustBar from "../components/DataTrustBar";
import ToolChip from "../components/ToolChip";
import ThinkingTrace from "../components/ThinkingTrace";
import SourceCitation from "../components/SourceCitation";
import ConfidenceBadge from "../components/ConfidenceBadge";
import ApprovalCard, { type ApprovalProposalLike } from "../components/ApprovalCard";
import ProtectedRoute from "../components/ProtectedRoute";
import { useAuth } from "../context/AuthContext";
import { useRouter } from "next/navigation";
import { useLanguage } from "../context/LanguageContext";
import { t, type Language } from "../lib/i18n";
import {
  generateRuntimeId,
  getSessionTitle,
  useAIChatRuntime,
  type AgentPlanStep,
  type AgentReviewPrompt,
  type AgentToolCall,
  type BatchParseSummary,
  type ChatSession,
  type ContractDraftItem,
  type Message,
  type PaymentScheduleDraftItem,
  type PaymentScheduleParseSummary,
  type RuntimeReviewAction,
  type RuntimeArtifact,
  type RuntimeSource,
  type UploadedFile,
} from "./runtime";
import { notifyError } from "../lib/notify";
import { safeInternalAIURL } from "../lib/retailAI";
import { apiErrorMessage, retailAnalyticsApi, type RetailDataClassification, type RetailScenarioResponse } from "../lib/api";
import MarkdownText from "./MarkdownText";
import { AI_CHAT_SESSION_ITEM_CLASS, AI_CHAT_SESSION_MORE_CLASS, getAIChatResponsiveState, getAIChatSessionButtonProps, getAIChatSessionRowProps, transitionAIChatDrawer, type AIChatDrawerEvent } from "./responsive";

const { TextArea } = Input;
const { Text } = Typography;

// ─── Constants ─────────────────────────────────────────────────

const MODEL_OPTIONS = [
  { value: "deepseek-v4-flash", label: "DeepSeek V4 Flash" },
  { value: "deepseek-v4-pro", label: "DeepSeek V4 Pro" },
  { value: "gpt-4o", label: "GPT-4o" },
];

const contextChipMap: Record<string, string[]> = {
  "contract-detail": [
    "ai.chip_risk",
    "ai.chip_why_no_calc",
    "ai.chip_accounting_impact",
  ],
  reports: [
    "ai.chip_explain_report",
    "ai.chip_query_scope",
    "ai.chip_anomalies",
  ],
  "monthly-closing": [
    "ai.chip_blockers",
    "ai.chip_entries_source",
    "ai.chip_next_steps",
  ],
};

const defaultChips = [
  "ai.chip_missing_dr",
  "ai.chip_pending",
  "ai.chip_expiring",
];

const agentSkillStarters = [
  {
    key: "retail-operations",
    labelKey: "ai.skill_retail_operations",
    promptKey: "ai.skill_retail_operations_prompt",
    icon: "retail",
    skillId: "retail_operations",
    skillVersion: "v1",
  },
  {
    key: "excel-ledger",
    labelKey: "ai.skill_excel_ledger",
    promptKey: "ai.skill_excel_ledger_prompt",
    icon: "excel",
    skillId: undefined,
    skillVersion: undefined,
  },
  {
    key: "contract-review",
    labelKey: "ai.skill_contract_review",
    promptKey: "ai.skill_contract_review_prompt",
    icon: "pdf",
    skillId: undefined,
    skillVersion: undefined,
  },
  {
    key: "payment-schedule",
    labelKey: "ai.skill_payment_schedule",
    promptKey: "ai.skill_payment_schedule_prompt",
    icon: "file",
    skillId: undefined,
    skillVersion: undefined,
  },
  {
    key: "audit-pack",
    labelKey: "ai.skill_audit_pack",
    promptKey: "ai.skill_audit_pack_prompt",
    icon: "tool",
    skillId: undefined,
    skillVersion: undefined,
  },
];

// ─── Helpers ───────────────────────────────────────────────────

function formatTime(timestamp: number, language?: Language): string {
  const diff = Date.now() - timestamp;
  if (diff < 60 * 1000) return language ? t("ai.just_now", language) : "刚刚";
  if (diff < 60 * 60 * 1000) return `${Math.floor(diff / (60 * 1000))}m`;
  if (diff < 24 * 60 * 60 * 1000) return `${Math.floor(diff / (60 * 60 * 1000))}h`;
  if (diff < 30 * 24 * 60 * 60 * 1000) return `${Math.floor(diff / (24 * 60 * 60 * 1000))}d`;
  return `${Math.floor(diff / (30 * 24 * 60 * 60 * 1000))}mo`;
}

function getContractDraftView(message: Message) {
  const artifact = message.artifacts?.find((item) => item.artifact_type === "contract_draft");
  const data = artifact?.data || {};
  const contracts = data.contracts || message.draftContracts;
  const summary = data.summary || message.batchSummary;
  if (!contracts || !summary) return null;
  return {
    artifactId: artifact?.id || message.contractDraftArtifactId,
    contracts: contracts as ContractDraftItem[],
    summary: summary as BatchParseSummary,
  };
}

function getPaymentScheduleDraftView(message: Message) {
  const artifact = message.artifacts?.find((item) => item.artifact_type === "payment_schedule_draft");
  const data = artifact?.data || {};
  const schedules = data.schedules || message.draftPaymentSchedules;
  const summary = data.summary || message.paymentScheduleSummary;
  if (!schedules || !summary) return null;
  return {
    artifactId: artifact?.id || message.paymentScheduleArtifactId,
    schedules: schedules as PaymentScheduleDraftItem[],
    summary: summary as PaymentScheduleParseSummary,
  };
}

function ArtifactSummaryPanel({ artifacts }: { artifacts?: RuntimeArtifact[] }) {
  const { token } = useAuth();
  const { language } = useLanguage();
  const router = useRouter();
  const [adoptingArtifact, setAdoptingArtifact] = useState<string | null>(null);
  const genericArtifacts = (artifacts || []).filter(
    (artifact) => artifact.artifact_type !== "contract_draft" && artifact.artifact_type !== "payment_schedule_draft",
  );
  if (genericArtifacts.length === 0) return null;

  // 采纳走既有 action API（零售情景行动草稿，幂等）：确认前零写入，
  // 与 AGENTS.md 底线一致——写操作只发生在这一个调用点。
  const adoptRetailProposal = async (artifactId: string, raw: unknown) => {
    const proposal = raw as ApprovalProposalLike;
    const scenario = proposal.scenario as RetailScenarioResponse | undefined;
    if (!token || !scenario) return;
    const selected = scenario.scenarios.find((item) => item.key !== "baseline") || scenario.scenarios[0];
    if (!selected) return;
    const days = Math.round((new Date(scenario.current.date_to).getTime() - new Date(scenario.current.date_from).getTime()) / 86400000) + 1;
    // P0-6: round the verification window UP to the nearest allowed size so
    // it never collapses below the evaluated span (10 evaluated days used to
    // shrink to 7).
    const windowDays = days <= 7 ? 7 : days <= 14 ? 14 : 28;
    const scope = {
      store_id: scenario.store.store_id,
      data_classification: scenario.data_classification as RetailDataClassification,
      dataset_version: scenario.dataset_version || undefined,
      as_of: scenario.current.date_to,
      window_days: windowDays as 7 | 14 | 28,
      source_system: scenario.source_system || undefined,
    };
    const body = {
      horizon_months: scenario.horizon_months,
      selected_scenario: { key: selected.key, name: selected.name, assumptions: selected.assumptions },
      title: proposal.title || scenario.scenarios[0]?.name || "retail action proposal",
      planned_action: proposal.planned_action || "",
      owner_name: proposal.owner_name || undefined,
      due_date: proposal.due_date || undefined,
      verification_period: proposal.verification_period || undefined,
    };
    setAdoptingArtifact(artifactId);
    try {
      const result = await retailAnalyticsApi.saveStoreScenarioAction(scope, body, `retail-proposal-${artifactId}`, token);
      message.success(result.idempotent_replay ? t("scenario.saved_replay", language) : t("scenario.saved", language));
    } catch (error) {
      message.error(apiErrorMessage(error));
    } finally {
      setAdoptingArtifact(null);
    }
  };

  return (
    <div className="sty-4a94ec25">
      {genericArtifacts.map((artifact) => (
        <div
          key={artifact.id}
          className="sty-1e27758a"
        >
          <Space wrap>
            <FileTextOutlined className="sty-edb10ce4" />
            <Text strong>{artifact.title || artifact.artifact_type}</Text>
            <StatusTag kind={statusKindFromAntColor(artifact.status === "confirmed" ? "green" : "blue")}>{artifact.status || "ready"}</StatusTag>
            {artifact.evidence_complete ? <StatusTag kind="success">证据完整</StatusTag> : <StatusTag kind="warning">需补证据</StatusTag>}
          </Space>
          {artifact.review_reasons && artifact.review_reasons.length > 0 && (
            <div className="sty-2c2c74e0">
              复核项：{artifact.review_reasons.join("、")}
            </div>
          )}
          {artifact.artifact_type === "retail_action_proposal" && artifact.data && (
            <div className="sty-f82c4a7a">
              <Text type="secondary" className="sty-2c2c74e0">{t("ai.approval.notice", language)}</Text>
              <ApprovalCard
                proposal={{ ...artifact.data, envelope: artifact.data.envelope, next_url: artifact.data.next_url } as ApprovalProposalLike}
                adopting={adoptingArtifact === artifact.id}
                onAdopt={(proposal) => adoptRetailProposal(artifact.id, proposal)}
                onModify={(proposal) => {
                  const url = proposal.next_url;
                  if (url && String(url).startsWith("/")) router.push(String(url));
                }}
                onReject={() => message.info(t("ai.approval.rejected", language))}
              />
            </div>
          )}
          <details className="sty-4b5b9834">
            <summary className="sty-b673d124">查看结构化 Artifact</summary>
            <pre className="sty-77dd66d1">
              {JSON.stringify(artifact.data || {}, null, 2)}
            </pre>
          </details>
        </div>
      ))}
    </div>
  );
}

function reviewActionLabel(actionType: string, i18nLang: Language): string {
  switch (actionType) {
    case "confirm":
      return t("ai.review_action_confirm", i18nLang);
    case "skip":
      return t("ai.review_action_skip", i18nLang);
    case "import":
      return t("ai.review_action_import", i18nLang);
    case "create_draft":
      return t("ai.review_action_create_draft", i18nLang);
    case "reject":
      return t("ai.review_action_reject", i18nLang);
    default:
      return actionType;
  }
}

// ─── Code Block Component ──────────────────────────────────────

function CodeBlock({ code, language, i18nLang }: { code: string; language: string; i18nLang: Language }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(code);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div className="sty-721b1879">
      <div
        className="sty-ea8440d7"
      >
        <span className="sty-8d1e93e5">
          {language || "code"}
        </span>
        <Button
          type="text"
          
          icon={copied ? <CheckOutlined className="sty-f46f799b" /> : <CopyOutlined />}
          onClick={handleCopy}
          className="sty-5822ad35"
        >
          {copied ? t("ai.copied", i18nLang) : t("ai.copy", i18nLang)}
        </Button>
      </div>
      <SyntaxHighlighter
        language={language || "text"}
        style={vscDarkPlus}
        customStyle={{
          margin: 0,
          borderRadius: "0 0 8px 8px",
          fontSize: 13,
          lineHeight: 1.5,
          padding: "12px 16px",
        }}
      >
        {code}
      </SyntaxHighlighter>
    </div>
  );
}

// ─── Message Content Renderer ──────────────────────────────────

function MessageContent({
  content,
  sources,
  model,
  thinking,
  toolCalls,
  confidence,
  confidenceReason,
  i18nLang,
  role = "assistant",
}: {
  content: string;
  sources?: Array<string | RuntimeSource>;
  model?: string;
  thinking?: string;
  toolCalls?: AgentToolCall[];
  confidence?: number;
  confidenceReason?: string;
  i18nLang: Language;
  role?: "user" | "assistant";
}) {
  const textColor = role === "user" ? "var(--fg-inverse)" : "var(--fg-secondary)";

  // Parse markdown-like code blocks
  const parts = useMemo(() => {
    const result: { type: "text" | "code"; content: string; language?: string }[] = [];
    const regex = /```(\w+)?\n([\s\S]*?)```/g;
    let lastIndex = 0;
    let match;

    while ((match = regex.exec(content)) !== null) {
      if (match.index > lastIndex) {
        result.push({ type: "text", content: content.slice(lastIndex, match.index) });
      }
      result.push({ type: "code", content: match[2], language: match[1] });
      lastIndex = match.index + match[0].length;
    }

    if (lastIndex < content.length) {
      result.push({ type: "text", content: content.slice(lastIndex) });
    }

    if (result.length === 0) {
      result.push({ type: "text", content });
    }

    return result;
  }, [content]);

  return (
    <div>
      {thinking && (
        <div className="sty-5822ad35">
          <ThinkingTrace thinking={thinking} />
        </div>
      )}

      {parts.map((part, idx) =>
        part.type === "code" ? (
          <CodeBlock key={idx} code={part.content} language={part.language || "text"} i18nLang={i18nLang} />
        ) : role === "user" ? (
          // The user's own text is shown verbatim — they did not write markdown.
          <Text key={idx} style={{ color: "#FFFFFF", fontSize: 14, lineHeight: 1.6 }}>
            {part.content}
          </Text>
        ) : (
          <div key={idx} style={{ width: "100%", fontSize: 14, lineHeight: 1.65 }}>
            <MarkdownText content={part.content} />
          </div>
        )
      )}

      {toolCalls && toolCalls.length > 0 && (
        <div className="ai-tool-row">
          {toolCalls.map((call, idx) => <ToolChip key={idx} call={call} />)}
        </div>
      )}

      {typeof confidence === "number" && (
        <div className="ai-confidence-row">
          <ConfidenceBadge confidence={confidence} reason={confidenceReason} />
        </div>
      )}

      {sources && sources.length > 0 && (
        <div className="sty-c9f7b4b7">
          <Text type="secondary" className="sty-090832a7">
            {t("ai.sources", i18nLang)}
          </Text>
          {sources.map((source, idx) => {
            const value = typeof source === "string" ? source : { ...source, url: safeInternalAIURL(source.url) };
            return <SourceCitation key={idx} source={value} />;
          })}
        </div>
      )}
      {/* M6.2: an answer without citations is labeled honestly, never padded
          with "all known sources" the model never claimed. */}
      {!thinking && content && sources && sources.length === 0 && (
        <div className="sty-c9f7b4b7">
          <Text type="secondary" className="sty-090832a7">{t("ai.no_sources", i18nLang)}</Text>
        </div>
      )}

      {model && (
        <div style={{ marginTop: 8, fontSize: 11, color: "var(--fg-muted, #94A3B8)", display: "flex", alignItems: "center", gap: 6 }}>
          <span>{t("ai.model_label", i18nLang)}:</span>
          <code style={{ background: "var(--bg-inset, #F1F5F9)", padding: "1px 6px", borderRadius: 4, fontSize: 10, color: "var(--fg-secondary, #64748B)" }}>{model}</code>
        </div>
      )}
    </div>
  );
}

// ─── Typewriter Effect ─────────────────────────────────────────

function TypewriterMessage({ content, sources, model, thinking, toolCalls, confidence, confidenceReason, i18nLang }: { content: string; sources?: Array<string | RuntimeSource>; model?: string; thinking?: string; toolCalls?: AgentToolCall[]; confidence?: number; confidenceReason?: string; i18nLang: Language }) {
  const [displayedContent, setDisplayedContent] = useState("");
  const contentRef = useRef(content);
  const indexRef = useRef(0);

  useEffect(() => {
    contentRef.current = content;
    indexRef.current = 0;
    setDisplayedContent("");

    const interval = setInterval(() => {
      if (indexRef.current < contentRef.current.length) {
        indexRef.current += 1;
        setDisplayedContent(contentRef.current.slice(0, indexRef.current));
      } else {
        clearInterval(interval);
      }
    }, 15);

    return () => clearInterval(interval);
  }, [content]);

  return (
    <MessageContent
      content={displayedContent}
      sources={displayedContent.length === content.length ? sources : undefined}
      model={displayedContent.length === content.length ? model : undefined}
      thinking={thinking}
      toolCalls={toolCalls}
      confidence={confidence}
      i18nLang={i18nLang}
      role="assistant"
    />
  );
}

// ─── Agent Tool Trace ──────────────────────────────────────────

function statusMeta(status: string, language: Language) {
  if (status === "completed") {
    return { color: "success", label: t("ai.agent_status_completed", language), icon: <CheckCircleOutlined /> };
  }
  if (status === "needs_review") {
    return { color: "warning", label: t("ai.agent_status_needs_review", language), icon: <ExclamationCircleOutlined /> };
  }
  if (status === "failed") {
    return { color: "error", label: t("ai.agent_status_failed", language), icon: <ExclamationCircleOutlined /> };
  }
  if (status === "running") {
    return { color: "processing", label: t("ai.agent_status_running", language), icon: <ToolOutlined /> };
  }
  return { color: "default", label: t("ai.agent_status_pending", language), icon: <ToolOutlined /> };
}

function AgentTracePanel({
  plan = [],
  toolCalls = [],
  language,
}: {
  plan?: AgentPlanStep[];
  toolCalls?: AgentToolCall[];
  language: Language;
}) {
  plan = Array.isArray(plan) ? plan : [];
  toolCalls = Array.isArray(toolCalls) ? toolCalls : [];
  if (plan.length === 0 && toolCalls.length === 0) return null;

  return (
    <div
      className="sty-6eac3fc5"
    >
      <div
        className="sty-3974a389"
      >
        <ToolOutlined className="sty-e3e86ee5" />
        <Text strong style={{ fontSize: 13 }}>
          {t("ai.agent_trace_title", language)}
        </Text>
      </div>

      {plan.length > 0 && (
        <div className="sty-e24ca831">
          <div className="sty-57670c22">
            {plan.map((step) => {
              const meta = statusMeta(step.status, language);
              return (
                <StatusTag key={step.id} kind={statusKindFromAntColor(meta.color as any)} icon={meta.icon} className="sty-51302e56">
                  {step.title}
                </StatusTag>
              );
            })}
          </div>
        </div>
      )}

      {toolCalls.length > 0 && (
        <div style={{ display: "flex", flexDirection: "column" }}>
          {toolCalls.map((call, index) => {
            const meta = statusMeta(call.status, language);
            return (
              <div
                key={`${call.tool}-${index}`}
                className="sty-1696a1bb"
              >
                <div className="sty-32c4a785">
                  <Text strong className="sty-f1e765ee">
                    {call.skill}
                  </Text>
                  <StatusTag kind={statusKindFromAntColor(meta.color as any)} className="sty-6319d0fa">
                    {meta.label}
                  </StatusTag>
                  <Text type="secondary" className="sty-1f609006">
                    {call.tool}
                  </Text>
                </div>
                <Text className="sty-14aa9694">
                  {call.output_summary || call.input_summary}
                </Text>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

function reviewSeverityMeta(severity: string) {
  if (severity === "critical") {
    return { color: "var(--state-error-text)", background: "var(--state-error-bg)", border: "var(--state-error-border)" };
  }
  if (severity === "warning") {
    return { color: "var(--state-warning-text)", background: "var(--state-warning-bg)", border: "var(--state-warning-border)" };
  }
  return { color: "var(--state-info-text)", background: "var(--state-info-bg)", border: "var(--state-info-border)" };
}

function AgentReviewPanel({
  prompts = [],
  language,
}: {
  prompts?: AgentReviewPrompt[];
  language: Language;
}) {
  prompts = Array.isArray(prompts) ? prompts : [];
  if (prompts.length === 0) return null;

  return (
    <div
      className="sty-6eac3fc5"
    >
      <div
        className="sty-e5092e30"
      >
        <ExclamationCircleOutlined className="sty-e3e86ee5" />
        <Text strong className="sty-51302e56">
          {t("ai.agent_review_title", language)}
        </Text>
      </div>

      <div style={{ display: "flex", flexDirection: "column" }}>
        {prompts.map((prompt, index) => {
          const meta = reviewSeverityMeta(prompt.severity);
          return (
            <div
              key={prompt.id || index}
              className="sty-1696a1bb"
            >
              <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                <Text strong style={{ fontSize: 12, color: meta.color }}>
                  {prompt.title}
                </Text>
                <StatusTag className="sty-1f609006">
                  {prompt.severity === "critical" ? t("ai.agent_severity_critical", language) : prompt.severity === "warning" ? t("ai.agent_severity_warning", language) : t("ai.agent_severity_info", language)}
                </StatusTag>
              </div>
              <Text className="sty-d9bf0a72">
                {prompt.description}
              </Text>
              <Text className="sty-8d0db302">
                {prompt.action}
              </Text>
              {prompt.contract_numbers && prompt.contract_numbers.length > 0 && (
                <div className="sty-c60b95f4">
                  {prompt.contract_numbers.map((contractNumber) => (
                    <StatusTag key={contractNumber} className="sty-14aa9694">
                      {contractNumber}
                    </StatusTag>
                  ))}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

function ReviewActionHistoryPanel({
  actions = [],
  language,
  onContinue,
}: {
  actions?: RuntimeReviewAction[];
  language: Language;
  onContinue: (action: RuntimeReviewAction) => void;
}) {
  actions = Array.isArray(actions) ? actions : [];
  if (actions.length === 0) return null;

  return (
    <div
      className="sty-6eac3fc5"
    >
      <div
        className="sty-6af63e1e"
      >
        <ClockCircleOutlined className="sty-e3e86ee5" />
        <Text strong className="sty-51302e56">
          {t("ai.review_action_history", language)}
        </Text>
      </div>

      <div style={{ display: "flex", flexDirection: "column" }}>
        {actions.map((action, index) => (
          <div
            key={action.id}
            className="sty-1696a1bb"
          >
            <div className="sty-57670c22">
              <StatusTag className="sty-32c4a785">
                {reviewActionLabel(action.actionType, language)}
              </StatusTag>
              <Text type="secondary" className="sty-53077cb2">
                {formatTime(action.actedAt)}
              </Text>
              {action.comment && (
                <Text className="sty-ea458b5c">
                  {action.comment}
                </Text>
              )}
            </div>

            <Button
              type="text"
              size="small"
              icon={<MessageOutlined />}
              onClick={() => onContinue(action)}
              className="sty-8b7f5990"
            >
              {t("ai.continue_from_action", language)}
            </Button>
          </div>
        ))}
      </div>
    </div>
  );
}

// ─── Session Sidebar ───────────────────────────────────────────

function SessionSidebar({
  sessions,
  activeSessionId,
  onSelect,
  onNew,
  onDelete,
}: {
  sessions: ChatSession[];
  activeSessionId: string | null;
  onSelect: (id: string) => void;
  onNew: () => void;
  onDelete: (id: string) => void;
}) {
  const { language } = useLanguage();

  return (
    <div
      className="ai-chat-session-sidebar"
      style={{
        width: 256,
        height: "100%",
        borderRight: "1px solid var(--border-default, #EAECF0)",
        background: "var(--bg-surface, #FAFAFA)",
        display: "flex",
        flexDirection: "column",
        flexShrink: 0,
      }}
    >
      {/* 1. Header (Flush 52px height) */}
      <div
        style={{
          height: 52,
          padding: "0 14px",
          borderBottom: "1px solid var(--border-default, #EAECF0)",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          boxSizing: "border-box",
        }}
      >
        <span style={{ fontWeight: 600, fontSize: 13, color: "var(--fg-primary)", letterSpacing: "-0.01em" }}>
          {t("nav.ai_chat", language)}
        </span>
        <Tooltip title={t("ai.new_session_btn", language)}>
          <Button
            type="text"
            size="small"
            icon={<PlusOutlined style={{ fontSize: 12 }} />}
            onClick={onNew}
            style={{ borderRadius: 6, width: 26, height: 26, padding: 0 }}
          />
        </Tooltip>
      </div>

      {/* 2. New Conversation Action Button */}
      <div style={{ padding: "10px 10px 4px 10px" }}>
        <Button
          type="default"
          icon={<PlusOutlined style={{ fontSize: 12 }} />}
          onClick={onNew}
          style={{
            width: "100%",
            height: 34,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            gap: 6,
            borderRadius: 7,
            fontWeight: 500,
            fontSize: 12,
            background: "var(--bg-card, #FFFFFF)",
            borderColor: "var(--border-default, #D0D5DD)",
            color: "var(--fg-primary)",
            boxShadow: "0 1px 2px rgba(16, 24, 40, 0.04)",
          }}
        >
          {t("ai.new_session_btn", language)}
        </Button>
      </div>

      {/* 3. Section Header */}
      <div style={{ padding: "10px 14px 4px 14px", display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <span style={{ fontSize: 11, fontWeight: 600, color: "var(--fg-muted, #98A2B3)", textTransform: "uppercase", letterSpacing: "0.04em" }}>
          {t("ai.history_sessions", language)}
        </span>
        <span style={{ fontSize: 11, color: "var(--fg-muted, #98A2B3)", fontVariantNumeric: "tabular-nums" }}>
          {sessions.length}
        </span>
      </div>

      {/* 4. Session List */}
      <div style={{ flex: 1, overflowY: "auto", padding: "4px 8px" }}>
        {sessions.length === 0 ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={t("ai.no_sessions", language)}
            style={{ padding: "32px 0" }}
          />
        ) : (
          <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
            {sessions.map((session) => {
              const isActive = activeSessionId === session.id;
              return (
                <div
                  key={session.id}
                  {...getAIChatSessionRowProps(isActive)}
                  style={{
                    position: "relative",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "space-between",
                    padding: "7px 10px",
                    borderRadius: 6,
                    cursor: "pointer",
                    background: isActive ? "var(--morandi-alabaster, #F2EDE9)" : "transparent",
                    color: isActive ? "var(--morandi-charcoal, #2B2A29)" : "var(--fg-secondary, #475467)",
                    fontWeight: isActive ? 600 : 400,
                    transition: "all 0.15s ease",
                    border: isActive ? "1px solid var(--border-subtle, #E4DFDA)" : "1px solid transparent",
                  }}
                  onClick={() => onSelect(session.id)}
                  onMouseEnter={(e) => {
                    if (!isActive) e.currentTarget.style.background = "var(--bg-inset, #F8F9FA)";
                  }}
                  onMouseLeave={(e) => {
                    if (!isActive) e.currentTarget.style.background = "transparent";
                  }}
                >
                  <button
                    {...getAIChatSessionButtonProps(isActive)}
                    className={AI_CHAT_SESSION_ITEM_CLASS}
                    aria-label={`选择会话 ${session.title}`}
                    style={{
                      border: 0,
                      font: "inherit",
                      textAlign: "left",
                      padding: 0,
                      cursor: "pointer",
                      background: "transparent",
                      color: "inherit",
                      fontWeight: "inherit",
                      display: "flex",
                      alignItems: "center",
                      gap: 8,
                      flex: 1,
                      minWidth: 0,
                    }}
                  >
                    <span
                      style={{
                        fontSize: 13,
                        lineHeight: "20px",
                        whiteSpace: "nowrap",
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                        flex: 1,
                      }}
                    >
                      {session.title || t("ai.new_session", language)}
                    </span>
                  </button>

                  <div style={{ display: "flex", alignItems: "center", gap: 4, flexShrink: 0, marginLeft: 6 }}>
                    <span style={{ fontSize: 11, color: "var(--fg-muted, #98A2B3)", fontVariantNumeric: "tabular-nums" }}>
                      {formatTime(session.updatedAt, language)}
                    </span>
                    <Dropdown
                      menu={{
                        items: [
                          {
                            key: "delete",
                            label: t("ai.delete_session", language),
                            icon: <DeleteOutlined />,
                            danger: true,
                            onClick: (e) => {
                              e.domEvent.stopPropagation();
                              onDelete(session.id);
                            },
                          },
                        ],
                      }}
                      trigger={["click"]}
                      placement="bottomRight"
                    >
                      <Button
                        type="text"
                        size="small"
                        aria-label={`删除会话 ${session.title}`}
                        icon={<MoreOutlined style={{ fontSize: 12 }} />}
                        onClick={(e) => e.stopPropagation()}
                        className={AI_CHAT_SESSION_MORE_CLASS}
                        style={{ width: 18, height: 18, padding: 0, color: "var(--fg-muted)" }}
                      />
                    </Dropdown>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

// ─── Draft Confirmation Panel ────────────────────────────────

interface DraftPanelProps {
  contracts: ContractDraftItem[];
  summary: BatchParseSummary;
  onConfirm: (selectedContracts: ContractDraftItem[]) => void;
  onSkip: () => void;
  language: Language;
}

function DraftConfirmationPanel({ contracts, summary, onConfirm, onSkip, language }: DraftPanelProps) {
  const [editedContracts, setEditedContracts] = useState<ContractDraftItem[]>(
    contracts.map((c) => ({ ...c }))
  );
  const [selectedIndices, setSelectedIndices] = useState<Set<number>>(
    new Set(contracts.map((_, i) => i))
  );
  const [creating, setCreating] = useState(false);

  const toggleSelect = (index: number) => {
    setSelectedIndices((prev) => {
      const next = new Set(prev);
      if (next.has(index)) {
        next.delete(index);
      } else {
        next.add(index);
      }
      return next;
    });
  };

  const toggleSelectAll = () => {
    if (selectedIndices.size === editedContracts.length) {
      setSelectedIndices(new Set());
    } else {
      setSelectedIndices(new Set(editedContracts.map((_, i) => i)));
    }
  };

  const updateContract = (index: number, field: keyof ContractDraftItem, value: any) => {
    setEditedContracts((prev) => {
      const next = [...prev];
      next[index] = { ...next[index], [field]: value };
      return next;
    });
  };

  const handleConfirm = async () => {
    const selected = Array.from(selectedIndices).map((i) => editedContracts[i]);
    if (selected.length === 0) {
      message.warning(t("ai.draft_select_at_least_one", language));
      return;
    }
    setCreating(true);
    try {
      await onConfirm(selected);
    } finally {
      setCreating(false);
    }
  };

  const hasLowConfidence = (c: ContractDraftItem) =>
    c.confidence < 0.8 || (c.scope_confidence ?? 1) < 0.8 || c.missing_fields.length > 0;

  return (
    <div className="sty-586c0725">
      {/* Header */}
      <div className="sty-c3336829">
        <div className="sty-b032ab3e">
          <div>
            <Text strong className="sty-df813d4e">
              {t("ai.draft_panel_title", language)}
            </Text>
            <Text type="secondary" className="sty-94051f6b">
              {t("ai.draft_panel_subtitle", language, { total: String(summary.total_count), confidence: String((summary.overall_confidence * 100).toFixed(0)) })}
            </Text>
          </div>
          <div className="sty-af8ee755">
            <Button size="small" onClick={toggleSelectAll}>
              {selectedIndices.size === editedContracts.length ? t("ai.deselect_all", language) : t("ai.select_all", language)}
            </Button>
            <Button size="small" danger onClick={onSkip}>
              {t("ai.skip", language)}
            </Button>
          </div>
        </div>
        {summary.requires_human_confirmation && (
          <div className="sty-77f17887">
            <Text className="sty-2c2c74e0">
              ⚠️ {t("ai.draft_warning", language)}
            </Text>
          </div>
        )}
        {summary.warnings.length > 0 && (
          <div className="sty-5ebea2d1">
            {summary.warnings.slice(0, 3).map((w, i) => (
              <Text key={i} className="sty-f6e0794d">
                • {w}
              </Text>
            ))}
          </div>
        )}
      </div>

      {/* Contract List */}
      <div style={{ maxHeight: 400, overflowY: "auto" }}>
        {editedContracts.map((contract, index) => (
          <div
            key={index}
            className="sty-b76fb200"
          >
            <div className="sty-7fb4a862">
              <input
                type="checkbox"
                checked={selectedIndices.has(index)}
                onChange={() => toggleSelect(index)}
                className="sty-83725d2c"
              />
              <div className="sty-90cfbbc8">
                {/* Row 1: Basic info */}
                <div className="sty-11a714e0">
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-e3e86ee5">{t("ai.draft_contract_number", language)}</Text>
                    <Input
                      size="small"
                      value={contract.contract_number}
                      onChange={(e) => updateContract(index, "contract_number", e.target.value)}
                      className="sty-11a714e0"
                    />
                  </div>
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-e3e86ee5">{t("ai.draft_contract_name", language)}</Text>
                    <Input
                      size="small"
                      value={contract.contract_name}
                      onChange={(e) => updateContract(index, "contract_name", e.target.value)}
                      className="sty-0b1fa162"
                    />
                  </div>
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-e3e86ee5">{t("ai.draft_currency", language)}</Text>
                    <Input
                      size="small"
                      value={contract.currency}
                      onChange={(e) => updateContract(index, "currency", e.target.value)}
                      className="sty-90cfbbc8"
                      status={!contract.currency ? "error" : ""}
                    />
                  </div>
                </div>

                {/* Row 2: Parties */}
                <div className="sty-11a714e0">
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-e3e86ee5">{t("ai.draft_lessee", language)}</Text>
                    <Input
                      size="small"
                      value={contract.lessee}
                      onChange={(e) => updateContract(index, "lessee", e.target.value)}
                      className="sty-11a714e0"
                    />
                  </div>
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-e3e86ee5">{t("ai.draft_lessor", language)}</Text>
                    <Input
                      size="small"
                      value={contract.lessor}
                      onChange={(e) => updateContract(index, "lessor", e.target.value)}
                      className="sty-0b1fa162"
                    />
                  </div>
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-e3e86ee5">{t("ai.draft_store", language)}</Text>
                    <Input
                      size="small"
                      value={contract.store_name}
                      onChange={(e) => updateContract(index, "store_name", e.target.value)}
                      className="sty-90cfbbc8"
                    />
                  </div>
                </div>

                {/* Row 3: Dates */}
                <div className="sty-0b1fa162">
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-e3e86ee5">{t("ai.draft_commencement_date", language)}</Text>
                    <Input
                      size="small"
                      value={contract.commencement_date}
                      onChange={(e) => updateContract(index, "commencement_date", e.target.value)}
                      className="sty-0b1fa162"
                    />
                  </div>
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-e3e86ee5">{t("ai.draft_lease_start", language)}</Text>
                    <Input
                      size="small"
                      value={contract.lease_start_date}
                      onChange={(e) => updateContract(index, "lease_start_date", e.target.value)}
                      className="sty-0b1fa162"
                    />
                  </div>
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-e3e86ee5">{t("ai.draft_lease_end", language)}</Text>
                    <Input
                      size="small"
                      value={contract.lease_end_date}
                      onChange={(e) => updateContract(index, "lease_end_date", e.target.value)}
                      className="sty-90cfbbc8"
                    />
                  </div>
                </div>

                {/* Row 4: Financial */}
                <div className="sty-ed8b9551">
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-e3e86ee5">{t("ai.draft_fixed_rent", language)}</Text>
                    <Input
                      size="small"
                      value={contract.fixed_rent_amount}
                      onChange={(e) => updateContract(index, "fixed_rent_amount", parseFloat(e.target.value) || 0)}
                      className="sty-ed8b9551"
                    />
                  </div>
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-e3e86ee5">{t("ai.draft_payment_timing", language)}</Text>
                    <Input
                      size="small"
                      value={contract.payment_timing}
                      onChange={(e) => updateContract(index, "payment_timing", e.target.value)}
                      className="sty-ed8b9551"
                    />
                  </div>
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-e3e86ee5">{t("ai.draft_discount_rate", language)}</Text>
                    <Input
                      size="small"
                      value={contract.discount_rate}
                      onChange={(e) => updateContract(index, "discount_rate", parseFloat(e.target.value) || 0)}
                      className="sty-90cfbbc8"
                      status={!contract.discount_rate ? "warning" : ""}
                    />
                  </div>
                </div>

                {/* Row 5: Scope gate */}
                <div className="sty-82cf7cd4">
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-e3e86ee5">资产类型</Text>
                    <Input
                      size="small"
                      value={contract.asset_type || ""}
                      onChange={(e) => updateContract(index, "asset_type", e.target.value)}
                      className="sty-bbe582ff"
                    />
                  </div>
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-e3e86ee5">租赁范围</Text>
                    <Input
                      size="small"
                      value={contract.lease_scope || contract.suggested_scope || ""}
                      onChange={(e) => updateContract(index, "lease_scope", e.target.value)}
                      className="sty-82cf7cd4"
                      status={!contract.lease_scope && !contract.suggested_scope ? "warning" : ""}
                    />
                  </div>
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-e3e86ee5">范围置信度</Text>
                    <Input
                      size="small"
                      value={contract.scope_confidence ?? ""}
                      onChange={(e) => updateContract(index, "scope_confidence", parseFloat(e.target.value) || 0)}
                      className="sty-4cb899b8"
                      status={(contract.scope_confidence ?? 1) < 0.8 ? "warning" : ""}
                    />
                  </div>
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-e3e86ee5">豁免/排除原因</Text>
                    <Input
                      size="small"
                      value={contract.exemption_reason || ""}
                      onChange={(e) => updateContract(index, "exemption_reason", e.target.value)}
                      className="sty-1696a1bb"
                    />
                  </div>
                </div>

                {/* Warnings & Confidence */}
                <div className="sty-6319d0fa">
                  {hasLowConfidence(contract) && (
                    <StatusTag kind="warning" className="sty-6319d0fa">
                      {t("ai.draft_confidence", language, { value: String((contract.confidence * 100).toFixed(0)) })}
                    </StatusTag>
                  )}
                  {contract.lease_scope && (
                    <StatusTag kind={statusKindFromAntColor(contract.lease_scope === "in_scope" ? "blue" : "orange")} className="sty-6319d0fa">
                      Scope: {contract.lease_scope}
                    </StatusTag>
                  )}
                  {contract.missing_fields.length > 0 && (
                    <StatusTag kind="error" className="sty-afdb217d">
                      {t("ai.draft_missing_fields", language, { fields: contract.missing_fields.join(", ") })}
                    </StatusTag>
                  )}
                  {contract.warnings.slice(0, 2).map((w, i) => (
                    <Text key={i} className="sty-0ec5707c">
                      {w}
                    </Text>
                  ))}
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Footer */}
      <div className="sty-32c4a785">
        <Text type="secondary" className="sty-9b921b3e">
          {t("ai.draft_selected_count", language, { selected: String(selectedIndices.size), total: String(editedContracts.length) })}
        </Text>
        <Button
          type="primary"
          loading={creating}
          disabled={selectedIndices.size === 0}
          onClick={handleConfirm}
          className="sty-f1151548"
        >
          {t("ai.draft_confirm_import", language)}
        </Button>
      </div>
    </div>
  );
}

// ─── Payment Schedule Draft Panel ─────────────────────────────

interface PaymentScheduleDraftPanelProps {
  schedules: PaymentScheduleDraftItem[];
  summary: PaymentScheduleParseSummary;
  onConfirm: (selectedSchedules: PaymentScheduleDraftItem[]) => void;
  onSkip: () => void;
  language: Language;
}

function PaymentScheduleDraftPanel({ schedules, summary, onConfirm, onSkip, language }: PaymentScheduleDraftPanelProps) {
  const [editedSchedules, setEditedSchedules] = useState<PaymentScheduleDraftItem[]>(
    schedules.map((s) => ({ ...s }))
  );
  const [selectedIndices, setSelectedIndices] = useState<Set<number>>(
    new Set(schedules.map((_, i) => i))
  );
  const [creating, setCreating] = useState(false);

  const toggleSelect = (index: number) => {
    setSelectedIndices((prev) => {
      const next = new Set(prev);
      if (next.has(index)) {
        next.delete(index);
      } else {
        next.add(index);
      }
      return next;
    });
  };

  const toggleSelectAll = () => {
    if (selectedIndices.size === editedSchedules.length) {
      setSelectedIndices(new Set());
    } else {
      setSelectedIndices(new Set(editedSchedules.map((_, i) => i)));
    }
  };

  const updateSchedule = (index: number, field: keyof PaymentScheduleDraftItem, value: any) => {
    setEditedSchedules((prev) => {
      const next = [...prev];
      next[index] = { ...next[index], [field]: value };
      return next;
    });
  };

  const handleConfirm = async () => {
    const selected = Array.from(selectedIndices).map((i) => editedSchedules[i]);
    if (selected.length === 0) {
      message.warning(t("ai.draft_select_at_least_one", language));
      return;
    }
    if (!summary.can_import || !summary.contract_id) {
      message.warning(t("ai.schedule_bind_contract_first", language));
      return;
    }
    const hasInvalidAccountingFields = selected.some(
      (schedule) =>
        !["prepaid", "postpaid"].includes(schedule.payment_timing) ||
        !Number.isFinite(schedule.amount) ||
        schedule.amount <= 0,
    );
    if (hasInvalidAccountingFields) {
      message.warning(t("ai.schedule_review_warning", language));
      return;
    }
    setCreating(true);
    try {
      await onConfirm(selected);
    } finally {
      setCreating(false);
    }
  };

  return (
    <div className="sty-586c0725">
      <div className="sty-b586e929">
        <div className="sty-b032ab3e">
          <div>
            <Text strong className="sty-df813d4e">
              {t("ai.schedule_panel_title", language)}
            </Text>
            <Text type="secondary" className="sty-94051f6b">
              {t("ai.draft_panel_subtitle", language, { total: String(summary.total_count), confidence: String((summary.overall_confidence * 100).toFixed(0)) })}
            </Text>
          </div>
          <div className="sty-52c0468d">
            <Button size="small" onClick={toggleSelectAll}>
              {selectedIndices.size === editedSchedules.length ? t("ai.deselect_all", language) : t("ai.select_all", language)}
            </Button>
            <Button size="small" danger onClick={onSkip}>
              {t("ai.skip", language)}
            </Button>
          </div>
        </div>
        {!summary.can_import && (
          <div className="sty-2a40f57c">
            <Text className="sty-af8ee755">
              {t("ai.schedule_bind_contract_first", language)}
            </Text>
          </div>
        )}
        {(summary.requires_human_confirmation || summary.warnings.length > 0) && (
          <div className="sty-77f17887">
            <Text className="sty-5a47b5f2">
              {t("ai.schedule_review_warning", language)}
            </Text>
          </div>
        )}
      </div>

      <div style={{ maxHeight: 360, overflowY: "auto" }}>
        {editedSchedules.map((schedule, index) => (
          <div
            key={index}
            className="sty-b76fb200"
          >
            <div className="sty-7fb4a862">
              <input
                type="checkbox"
                checked={selectedIndices.has(index)}
                onChange={() => toggleSelect(index)}
                className="sty-83725d2c"
              />
              <div className="sty-90cfbbc8">
                <div className="sty-ed8b9551">
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-ed8b9551">{t("ai.schedule_period_start", language)}</Text>
                    <Input size="small" value={schedule.period_start} onChange={(e) => updateSchedule(index, "period_start", e.target.value)} />
                  </div>
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-ed8b9551">{t("ai.schedule_period_end", language)}</Text>
                    <Input size="small" value={schedule.period_end} onChange={(e) => updateSchedule(index, "period_end", e.target.value)} />
                  </div>
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-d9e0af7d">{t("ai.schedule_due_date", language)}</Text>
                    <Input size="small" value={schedule.due_date} onChange={(e) => updateSchedule(index, "due_date", e.target.value)} />
                  </div>
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-90cfbbc8">{t("ai.schedule_amount", language)}</Text>
                    <Input size="small" value={schedule.amount} onChange={(e) => updateSchedule(index, "amount", parseFloat(e.target.value) || 0)} status={schedule.amount <= 0 ? "error" : ""} />
                  </div>
                </div>
                <div className="sty-d9e0af7d">
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-ed8b9551">{t("ai.draft_currency", language)}</Text>
                    <Input size="small" value={schedule.currency || ""} onChange={(e) => updateSchedule(index, "currency", e.target.value)} status={!schedule.currency ? "warning" : ""} />
                  </div>
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-82cf7cd4">{t("ai.draft_payment_timing", language)}</Text>
                    <Input size="small" value={schedule.payment_timing} onChange={(e) => updateSchedule(index, "payment_timing", e.target.value)} />
                  </div>
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-5904eb2a">{t("ai.schedule_amount_type", language)}</Text>
                    <Input size="small" value={schedule.amount_type} onChange={(e) => updateSchedule(index, "amount_type", e.target.value)} />
                  </div>
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-0b1fa162">{t("ai.schedule_is_fixed", language)}</Text>
                    <Input size="small" value={schedule.is_fixed ? "true" : "false"} onChange={(e) => updateSchedule(index, "is_fixed", e.target.value === "true")} />
                  </div>
                  <div className="sty-6319d0fa">
                    <Text type="secondary" className="sty-1696a1bb">{t("ai.schedule_is_lease_component", language)}</Text>
                    <Input size="small" value={schedule.is_lease_component ? "true" : "false"} onChange={(e) => updateSchedule(index, "is_lease_component", e.target.value === "true")} />
                  </div>
                </div>
                <div className="sty-6319d0fa">
                  <StatusTag kind={statusKindFromAntColor(schedule.confidence < 0.8 ? "warning" : "green")} className="sty-6319d0fa">
                    {t("ai.draft_confidence", language, { value: String((schedule.confidence * 100).toFixed(0)) })}
                  </StatusTag>
                  {!schedule.is_fixed && <StatusTag kind="warning" className="sty-6319d0fa">{t("ai.schedule_variable_rent", language)}</StatusTag>}
                  {!schedule.is_lease_component && <StatusTag kind="warning" className="sty-0ec5707c">{t("ai.schedule_non_lease_component", language)}</StatusTag>}
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>

      <div className="sty-32c4a785">
        <Text type="secondary" className="sty-9b921b3e">
          {t("ai.draft_selected_count", language, { selected: String(selectedIndices.size), total: String(editedSchedules.length) })}
        </Text>
        <Button
          type="primary"
          loading={creating}
          disabled={selectedIndices.size === 0 || !summary.can_import}
          onClick={handleConfirm}
          className="sty-e2ea03b3"
        >
          {t("ai.schedule_confirm_import", language)}
        </Button>
      </div>
    </div>
  );
}

// ─── Main Chat Page ────────────────────────────────────────────

function AIChatPageContent() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const searchParams = useSearchParams();
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const sessionDrawerTriggerRef = useRef<HTMLButtonElement>(null);
  // Keep server and first client render identical; update after mount.
  const [viewportWidth, setViewportWidth] = useState(768);
  const [sessionDrawerOpen, setSessionDrawerOpen] = useState(false);
  const responsiveState = getAIChatResponsiveState(viewportWidth);
  const isMobileChat = responsiveState.isMobile;

  useEffect(() => {
    const updateViewportWidth = () => setViewportWidth(window.innerWidth);
    updateViewportWidth();
    window.addEventListener("resize", updateViewportWidth);
    return () => window.removeEventListener("resize", updateViewportWidth);
  }, []);

  useEffect(() => {
    if (!isMobileChat) setSessionDrawerOpen(false);
  }, [isMobileChat]);

  // Page context from URL
  const pageContext = useMemo(() => {
    const page = searchParams.get("page");
    if (!page) return undefined;
    const filterKeys = [
      "as_of", "window_days", "classification", "data_classification", "dataset_version", "source_system",
      "store_id", "store_ids", "horizon_months", "revenue_change_pct", "gross_margin_rate_change_pp",
      "labor_cost_change_pct", "fixed_rent_change_pct", "variable_rent_rate_change_pp",
      "non_lease_cost_change_pct", "other_controllable_cost_change_pct",
    ];
    const filters: Record<string, string> = {};
    for (const key of filterKeys) {
      const values = searchParams.getAll(key);
      if (values.length > 0) filters[key] = key === "store_ids" ? values.join(",") : values[0];
    }
    return {
      page,
      title: searchParams.get("title") || undefined,
      tags: searchParams.getAll("tags"),
      contract_id: searchParams.get("contract_id") || undefined,
      period: searchParams.get("period") || undefined,
      report_view: searchParams.get("report_view") || undefined,
      filters,
      summary: searchParams.get("summary") || undefined,
    };
  }, [searchParams]);

  // P0-4: `?message=` prefills the composer — same URL-parameter contract as
  // the page/title/tags keys handled in pageContext above.
  const [input, setInput] = useState(() => searchParams.get("message") || "");
  const [selectedSkill, setSelectedSkill] = useState<{ id: string; version: string } | undefined>(undefined);

  useEffect(() => {
    if (pageContext?.page === "operating-pulse" || pageContext?.page === "store-360" || pageContext?.page === "scenario-workbench") {
      setSelectedSkill({ id: "retail_operations", version: "v1" });
    }
  }, [pageContext?.page]);
  const [selectedModel, setSelectedModel] = useState("deepseek-v4-flash");
  const [traceRunId, setTraceRunId] = useState<string | null>(null);
  const [traceData, setTraceData] = useState<any>(null);
  const [traceLoading, setTraceLoading] = useState(false);
  const runtime = useAIChatRuntime({ token, language, selectedModel });
  const {
    sessions,
    activeSessionId,
    activeSession,
    loading,
    typingMessageId,
    setActiveSessionId,
    setLoading,
    createNewSession: createRuntimeSession,
    deleteSession,
    updateSessionMessages,
    updateSession,
    setPendingUpload,
    buildHistory,
    ensureServerSession,
    createAndStartRun,
    continueFromTarget,
    continueActive,
    recordReviewAction,
    getRunTrace,
    cancelRun,
    steerRun,
    followUpRun,
    branchRun,
  } = runtime;

  // Scroll to bottom when messages change
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [sessions, activeSessionId]);

  const activePendingUpload = activeSession?.pendingUpload ?? null;

  const chips = useMemo(() => {
    if (!pageContext) return defaultChips;
    return contextChipMap[pageContext.page!] || defaultChips;
  }, [pageContext]);

  const createNewSession = () => {
    createRuntimeSession();
    setInput("");
  };

  const transitionSessionDrawer = (event: AIChatDrawerEvent) => {
    const next = transitionAIChatDrawer(sessionDrawerOpen, event);
    setSessionDrawerOpen(next.open);
    if (next.restoreFocus) {
      if (typeof window !== "undefined") {
        window.requestAnimationFrame(() => sessionDrawerTriggerRef.current?.focus());
      } else {
        sessionDrawerTriggerRef.current?.focus();
      }
    }
  };

  const closeSessionDrawer = () => transitionSessionDrawer("close");

  const confirmDeleteSession = (id: string) => {
    Modal.confirm({
      title: t("ai.delete_session_title", language),
      content: t("ai.delete_session_content", language),
      okText: t("ai.delete", language),
      okType: "danger",
      cancelText: t("ai.cancel", language),
      onOk: () => deleteSession(id),
    });
  };

  const getFileIcon = (type: string) => {
    if (type.includes("pdf")) return <FilePdfOutlined className="sty-8d1e93e5" />;
    if (type.includes("excel") || type.includes("sheet"))
      return <FileExcelOutlined className="sty-6af63e1e" />;
    return <FileImageOutlined className="sty-f0618391" />;
  };

  const getSkillIcon = (icon: string) => {
    if (icon === "excel") return <FileExcelOutlined />;
    if (icon === "pdf") return <FilePdfOutlined />;
    if (icon === "tool") return <ToolOutlined />;
    if (icon === "retail") return <RobotOutlined />;
    return <FileTextOutlined />;
  };

  const inferUploadTaskType = (prompt: string) => {
    const normalized = prompt.toLowerCase();
    if (
      normalized.includes("租金表") ||
      normalized.includes("付款计划") ||
      normalized.includes("付款表") ||
      normalized.includes("rent schedule") ||
      normalized.includes("payment schedule")
    ) {
      return "payment_schedule";
    }
    if (
      normalized.includes("事件") ||
      normalized.includes("变更") ||
      normalized.includes("modification") ||
      normalized.includes("reassessment")
    ) {
      return "event";
    }
    return "contract";
  };

  const handleFileUpload = async (options: any) => {
    const { file, onSuccess, onError } = options;
    const formData = new FormData();
    const taskType = inferUploadTaskType(input);
    formData.append("file", file);
    formData.append("task_type", taskType);

    try {
      const response = await fetch(`${window.location.origin}/api/ai/files/upload`, {
        method: "POST",
        headers: {
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: formData,
      });

      if (!response.ok) {
        throw new Error(t("ai.upload_failed", language, { name: "file" }));
      }

      const data = await response.json();
      const uploadedFile: UploadedFile = {
        file_id: data.file_id,
        original_name: data.original_name,
        content_type: data.content_type,
        object_name: data.object_name,
      };

      message.success(`${data.original_name} ${t("ai.upload_success", language)}`);
      onSuccess(data, file);
      if (activeSessionId) {
        setPendingUpload(activeSessionId, uploadedFile);
      }
    } catch (err: any) {
      onError(err);
      notifyError(`${t("ai.upload_failed", language)}: ${err.message}`);
    }
  };

  const triggerRuntimeContinuation = async (
    target: { type: "run" | "message" | "artifact" | "action"; id: string },
    options?: { instruction?: string; contractId?: string },
  ) => {
    try {
      await continueActive(target, options);
    } catch (error: any) {
      notifyError(
        t("ai.request_failed", language, {
          error: error.message || t("ai.unknown_error", language),
        }),
      );
    }
  };

  const handleOpenTrace = async (runId: string) => {
    setTraceRunId(runId);
    setTraceLoading(true);
    setTraceData(null);
    try {
      setTraceData(await getRunTrace(runId));
    } catch (error: any) {
      notifyError(
        t("ai.request_failed", language, {
          error: error.message || t("ai.unknown_error", language),
        }),
      );
    } finally {
      setTraceLoading(false);
    }
  };

  const handleRunControl = (runId: string, action: "cancel" | "steer" | "follow-up" | "branch") => {
    if (action === "cancel") {
      Modal.confirm({
        title: t("ai.run_cancel_title", language),
        content: t("ai.run_cancel_content", language),
        okText: t("ai.run_cancel", language),
        okType: "danger",
        cancelText: t("ai.cancel", language),
        onOk: async () => {
          await cancelRun(runId);
          setLoading(false);
          if (activeSessionId) updateSession(activeSessionId, { currentRunId: undefined });
          message.success(t("ai.run_control_sent", language));
        },
      });
      return;
    }

    let instruction = "";
    const titleKey = action === "steer"
      ? "ai.run_steer_title"
      : action === "follow-up"
        ? "ai.run_follow_up_title"
        : "ai.run_branch_title";
    Modal.confirm({
      title: t(titleKey, language),
      content: (
        <Input
          autoFocus
          placeholder={t("ai.run_control_placeholder", language)}
          onChange={(event) => {
            instruction = event.target.value;
          }}
        />
      ),
      okText: t("ai.confirm", language),
      cancelText: t("ai.cancel", language),
      onOk: async () => {
        const value = instruction.trim();
        if (!value) {
          message.warning(t("ai.run_control_required", language));
          throw new Error("instruction is required");
        }
        if (action === "steer") await steerRun(runId, value);
        if (action === "follow-up") await followUpRun(runId, value);
        if (action === "branch") await branchRun(runId, value);
        message.success(t("ai.run_control_sent", language));
      },
    });
  };

  const handleSend = async (messageOverride?: string, fileOverride?: UploadedFile) => {
    const fileForRequest = fileOverride ?? activePendingUpload;
    const msg = (messageOverride ?? input).trim();
    if ((!msg && !fileForRequest) || !token || !activeSessionId) return;
    const messageText = msg || "请解析这个文件并导入台账：先生成合同草稿卡片，等待我确认后再入库。";

    const userMessage: Message = {
      id: generateRuntimeId(),
      role: "user",
      content: messageText,
      timestamp: Date.now(),
      attachments: fileForRequest ? [fileForRequest] : undefined,
    };

    // Get history from active session
    const currentSession = sessions.find((s) => s.id === activeSessionId);
    const history = buildHistory(currentSession?.messages || []);

    updateSessionMessages(activeSessionId, (msgs) => [...msgs, userMessage]);
    setInput("");
    setLoading(true);

    try {
      const serverSessionId = await ensureServerSession(activeSessionId, {
          title: currentSession?.title || getSessionTitle([userMessage], language),
          bound_contract_id: searchParams.get("contract_id") || undefined,
        context_snapshot: pageContext
            ? {
                page: pageContext.page,
                title: pageContext.title,
                tags: pageContext.tags,
                contract_id: pageContext.contract_id,
                period: pageContext.period,
                report_view: pageContext.report_view,
                filters: pageContext.filters,
                summary: pageContext.summary,
              }
            : undefined,
        });

      const chatData: any = { message: messageText, history, language };
      if (selectedSkill) {
        chatData.skill_id = selectedSkill.id;
        chatData.skill_version = selectedSkill.version;
      }
      if (fileForRequest) {
        chatData.file_id = fileForRequest.file_id;
        chatData.object_name = fileForRequest.object_name;
        chatData.content_type = fileForRequest.content_type;
        setPendingUpload(activeSessionId, null);
      }
      if (pageContext) {
        chatData.page_context = {
          page: pageContext.page,
          title: pageContext.title,
          tags: pageContext.tags,
          contract_id: pageContext.contract_id,
          period: pageContext.period,
          report_view: pageContext.report_view,
          filters: pageContext.filters,
          summary: pageContext.summary,
        };
      }
      const contractIdFromUrl = searchParams.get("contract_id");
      if (contractIdFromUrl) {
        chatData.contract_id = contractIdFromUrl;
      }

      await createAndStartRun(activeSessionId, serverSessionId, chatData, {});
    } catch (error: any) {
      const errorMessage: Message = {
        id: generateRuntimeId(),
        role: "assistant",
        content: t("ai.request_failed", language, { error: error.message || t("ai.unknown_error", language) }),
        timestamp: Date.now(),
      };
      updateSessionMessages(activeSessionId, (msgs) => [...msgs, errorMessage]);
      updateSession(activeSessionId, { currentRunId: undefined });
    } finally {
      setLoading(false);
    }
  };

  const handleChipClick = (question: string) => {
    handleSend(question);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      if (input.trim() || activePendingUpload) {
        handleSend();
      }
    }
  };

  const currentMessages = activeSession?.messages || [];

  return (
    <ProtectedRoute>
      <AppLayout>
        <div
          className="ai-chat-shell"
          style={{
            display: "flex",
            height: "calc(100vh - 64px)",
            width: "100%",
            background: "var(--bg-page)",
            overflow: "hidden",
          }}
        >
          {/* Session Sidebar */}
          {responsiveState.showDesktopSidebar && (
            <SessionSidebar
              sessions={sessions}
              activeSessionId={activeSessionId}
              onSelect={setActiveSessionId}
              onNew={createNewSession}
              onDelete={confirmDeleteSession}
            />
          )}

          {/* Chat Area */}
          <div
            style={{
              flex: 1,
              display: "flex",
              flexDirection: "column",
              height: "100%",
              minWidth: 0,
              background: "var(--bg-page)",
              overflow: "hidden",
            }}
          >
            {/* Top Bar (matching height 52px flush with sidebar) */}
            <div
              style={{
                height: 52,
                borderBottom: "1px solid var(--border-default, #EAECF0)",
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                padding: "0 20px",
                background: "var(--bg-elevated, #FFFFFF)",
                flexShrink: 0,
                boxSizing: "border-box",
              }}
            >
              <div style={{ display: "flex", alignItems: "center", gap: 10, minWidth: 0, flex: 1 }}>
                {responsiveState.showMobileSessionTrigger && (
                  <Button
                    ref={sessionDrawerTriggerRef}
                    type="text"
                    aria-label="打开会话"
                    icon={<MessageOutlined />}
                    onClick={() => transitionSessionDrawer("open")}
                  />
                )}
                <RobotOutlined style={{ fontSize: 15, color: "var(--morandi-charcoal, #5A5958)" }} />
                <span
                  style={{
                    fontSize: 13,
                    fontWeight: 600,
                    color: "var(--fg-primary)",
                    whiteSpace: "nowrap",
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                  }}
                >
                  {activeSession?.title || t("ai.assistant_name", language)}
                </span>
              </div>

              <div style={{ display: "flex", alignItems: "center", gap: 8, flexShrink: 0 }}>
                {/* Model Selector */}
                <Dropdown
                  menu={{
                    items: MODEL_OPTIONS.map((m) => ({
                      key: m.value,
                      label: m.label,
                      onClick: () => setSelectedModel(m.value),
                    })),
                  }}
                >
                  <Button
                    type="text"
                    size="small"
                    style={{ fontSize: 12, borderRadius: 6, color: "var(--fg-secondary)" }}
                    aria-label={`选择模型，当前 ${MODEL_OPTIONS.find((m) => m.value === selectedModel)?.label || selectedModel}`}
                  >
                    <span>{MODEL_OPTIONS.find((m) => m.value === selectedModel)?.label || selectedModel}</span>
                    <DownOutlined style={{ fontSize: 9, marginLeft: 4 }} />
                  </Button>
                </Dropdown>
              </div>
            </div>

            {/* Messages Area */}
            <div
              className="ai-chat-messages"
              style={{
                flex: 1,
                overflowY: "auto",
                padding: "24px 32px",
                maxWidth: 960,
                width: "100%",
                margin: "0 auto",
                display: "flex",
                flexDirection: "column",
                gap: 16,
              }}
            >
              {/* Context strip */}
              {pageContext && (
                <motion.div
                  initial={false}
                  animate={{ opacity: 1, y: 0 }}
                  className="sty-3c01e428"
                >
                  <Text type="secondary" className="sty-31eb049e">
                    {t("ai.context", language)}
                  </Text>
                  <StatusTag className="sty-31eb049e">
                    {pageContext.title || pageContext.page}
                  </StatusTag>
                  {pageContext.contract_id && (
                    <StatusTag className="sty-31eb049e">
                      {pageContext.contract_id}
                    </StatusTag>
                  )}
                  {pageContext.period && (
                    <StatusTag className="sty-31eb049e">
                      {pageContext.period}
                    </StatusTag>
                  )}
                  {pageContext.filters?.classification && <StatusTag kind={pageContext.filters.classification === "simulated" ? "warning" : "processing"}>{pageContext.filters.classification === "simulated" ? "模拟 · Working" : "正式 · Working"}</StatusTag>}
                  {pageContext.filters?.dataset_version && <StatusTag className="sty-31eb049e">dataset: {pageContext.filters.dataset_version}</StatusTag>}
                  {pageContext.filters?.source_system && <StatusTag className="sty-31eb049e">source: {pageContext.filters.source_system}</StatusTag>}
                  {pageContext.filters?.as_of && <StatusTag className="sty-31eb049e">as of: {pageContext.filters.as_of}</StatusTag>}
                  {pageContext.filters?.window_days && <StatusTag className="sty-c53970c1">window: {pageContext.filters.window_days}天</StatusTag>}
                </motion.div>
              )}

              {/* Hero Welcome & Skill Grid */}
              {currentMessages.length <= 1 && (
                <motion.div
                  initial={{ opacity: 0, y: 10 }}
                  animate={{ opacity: 1, y: 0 }}
                  style={{
                    marginBottom: 24,
                    background: "var(--bg-elevated)",
                    border: "1px solid var(--border-default)",
                    borderRadius: 16,
                    padding: "24px 28px",
                    boxShadow: "0 2px 12px rgba(0, 0, 0, 0.03)",
                  }}
                >
                  <div style={{ marginBottom: 18 }}>
                    <div style={{ fontSize: 18, fontWeight: 600, color: "var(--fg-primary)", marginBottom: 6 }}>
                      {t("ai.assistant_name", language)}
                    </div>
                    <div style={{ fontSize: 13, color: "var(--fg-secondary)", lineHeight: 1.6 }}>
                      {t("ai.welcome_subtitle", language) || "连接门店销售、毛利、客流、占用成本与租赁合同，驱动「发现问题 — 解释原因 — 模拟方案 — 形成行动」闭环。"}
                    </div>
                  </div>

                  {/* 2x2 Skill Grid */}
                  <div
                    style={{
                      display: "grid",
                      gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))",
                      gap: 12,
                      marginBottom: 16,
                    }}
                  >
                    {agentSkillStarters.map((skill) => (
                      <div
                        key={skill.key}
                        onClick={() => {
                          setInput(t(skill.promptKey, language));
                          setSelectedSkill(skill.skillId ? { id: skill.skillId, version: skill.skillVersion || "v1" } : undefined);
                        }}
                        style={{
                          display: "flex",
                          alignItems: "flex-start",
                          gap: 12,
                          padding: "12px 14px",
                          borderRadius: 10,
                          border: "1px solid var(--border-subtle, #EAECF0)",
                          background: "var(--bg-surface)",
                          cursor: "pointer",
                          transition: "all 0.2s ease",
                        }}
                        onMouseEnter={(e) => {
                          e.currentTarget.style.borderColor = "var(--morandi-sand, #D8BB8F)";
                          e.currentTarget.style.boxShadow = "0 2px 8px rgba(0,0,0,0.04)";
                        }}
                        onMouseLeave={(e) => {
                          e.currentTarget.style.borderColor = "var(--border-subtle, #EAECF0)";
                          e.currentTarget.style.boxShadow = "none";
                        }}
                      >
                        <div
                          style={{
                            width: 32,
                            height: 32,
                            borderRadius: 8,
                            background: "var(--morandi-cream, #F2EDE9)",
                            color: "var(--morandi-slate, #5A5958)",
                            display: "flex",
                            alignItems: "center",
                            justifyContent: "center",
                            fontSize: 15,
                            flexShrink: 0,
                          }}
                        >
                          {getSkillIcon(skill.icon)}
                        </div>
                        <div>
                          <div style={{ fontSize: 13, fontWeight: 600, color: "var(--fg-primary)", marginBottom: 2 }}>
                            {t(skill.labelKey, language)}
                          </div>
                          <div style={{ fontSize: 11, color: "var(--fg-muted)", lineHeight: 1.4 }}>
                            {t(skill.promptKey, language).slice(0, 24)}...
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>

                  {/* Quick Question Chips */}
                  <div style={{ display: "flex", alignItems: "center", flexWrap: "wrap", gap: 8, paddingTop: 12, borderTop: "1px solid var(--border-subtle, #F1F5F9)" }}>
                    <span style={{ fontSize: 12, color: "var(--fg-muted)" }}>{t("ai.quick_questions", language)}:</span>
                    {chips.map((chipKey, idx) => (
                      <Button
                        key={idx}
                        size="small"
                        type="default"
                        onClick={() => handleChipClick(t(chipKey, language))}
                        disabled={loading}
                        style={{
                          fontSize: 12,
                          borderRadius: 9999,
                          borderColor: "var(--border-default)",
                          color: "var(--fg-secondary)",
                          background: "var(--bg-elevated)",
                        }}
                      >
                        {t(chipKey, language)}
                      </Button>
                    ))}
                  </div>
                </motion.div>
              )}

              {/* Messages */}
              <AnimatePresence>
                {currentMessages.map((msg, index) => (
                  <motion.div
                    key={msg.id}
                    initial={false}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ duration: 0.25, delay: index === currentMessages.length - 1 ? 0 : 0 }}
                    style={{
                      display: "flex",
                      justifyContent: msg.role === "user" ? "flex-end" : "flex-start",
                      marginBottom: 20,
                    }}
                  >
                    <Space
                      align="start"
                      style={{
                        flexDirection: msg.role === "user" ? "row-reverse" : "row",
                        maxWidth: "85%",
                      }}
                    >
                      <Avatar
                        icon={msg.role === "user" ? <UserOutlined /> : <RobotOutlined />}
                        style={{
                          backgroundColor: msg.role === "user" ? "var(--fg-secondary)" : "var(--fg-primary)",
                          flexShrink: 0,
                        }}
                        size={32}
                      />

                      <div
                        style={{
                          borderRadius: 12,
                          padding: "12px 18px",
                          background: msg.role === "user" ? "var(--mono-20, #1E293B)" : "var(--bg-elevated, #FFFFFF)",
                          color: msg.role === "user" ? "#FFFFFF" : "var(--fg-primary, #0F172A)",
                          border: msg.role === "user" ? "none" : "1px solid var(--border-default, #E2E8F0)",
                          boxShadow: msg.role === "user" ? "0 1px 2px rgba(0,0,0,0.12)" : "0 1px 3px rgba(0,0,0,0.04)",
                          maxWidth: "100%",
                          wordBreak: "break-word",
                        }}
                      >
                        {msg.role === "assistant" && typingMessageId === msg.id ? (
                          <TypewriterMessage
                            content={msg.id === "welcome" ? t("ai.welcome", language) : msg.content}
                            sources={msg.sources}
                            model={msg.model}
                            thinking={msg.thinking}
                            toolCalls={msg.toolCalls}
                            confidence={msg.confidence}
                            confidenceReason={msg.confidenceReason}
                            i18nLang={language}
                          />
                        ) : (
                          <MessageContent
                            content={msg.id === "welcome" ? t("ai.welcome", language) : msg.content}
                            sources={msg.sources}
                            model={msg.model}
                            thinking={msg.thinking}
                            toolCalls={msg.toolCalls}
                            confidence={msg.confidence}
                            confidenceReason={msg.confidenceReason}
                            i18nLang={language}
                            role={msg.role}
                          />
                        )}

                        {msg.role === "assistant" && msg.agentMode && (
                          <AgentTracePanel
                            plan={msg.agentPlan}
                            toolCalls={msg.toolCalls}
                            language={language}
                          />
                        )}

                        {msg.role === "assistant" && msg.agentMode && (
                          <AgentReviewPanel
                            prompts={msg.reviewPrompts}
                            language={language}
                          />
                        )}

                        {msg.role === "assistant" && <ArtifactSummaryPanel artifacts={msg.artifacts} />}

                        {msg.role === "assistant" && (
                          <ReviewActionHistoryPanel
                            actions={msg.reviewActions}
                            language={language}
                            onContinue={(action) =>
                              triggerRuntimeContinuation(
                                { type: "action", id: action.id },
                                {
                                  contractId:
                                    getPaymentScheduleDraftView(msg)?.summary.contract_id ||
                                    searchParams.get("contract_id") ||
                                    undefined,
                                }
                              )
                            }
                          />
                        )}

                        {msg.attachments && msg.attachments.length > 0 && (
                          <div style={{ marginTop: 8, display: "flex", gap: 6, flexWrap: "wrap" }}>
                            {msg.attachments.map((att, idx) => (
                              <StatusTag
                                key={idx}
                                icon={getFileIcon(att.content_type)}
                               
                                style={{
                                  borderRadius: 4,
                                  background: msg.role === "user" ? "rgba(255,255,255,0.1)" : "var(--fg-inverse)",
                                  border: msg.role === "user" ? "1px solid rgba(255,255,255,0.2)" : "1px solid var(--border-default)",
                                  color: msg.role === "user" ? "var(--fg-inverse)" : "var(--fg-secondary)",
                                }}
                              >
                                {att.original_name}
                              </StatusTag>
                            ))}
                          </div>
                        )}

                        {/* Draft Confirmation Panel */}
                        {getContractDraftView(msg) && msg.role === "assistant" && (
                          <DraftConfirmationPanel
                            contracts={getContractDraftView(msg)!.contracts}
                            summary={getContractDraftView(msg)!.summary}
                            language={language}
                            onConfirm={async (selectedContracts) => {
                              try {
                                const draftView = getContractDraftView(msg);
                                if (!draftView?.artifactId) {
                                  throw new Error("合同草稿 Artifact 不存在，无法提交确认");
                                }
                                const selectedNumbers = selectedContracts.map((contract) => contract.contract_number);
                                const selectedNumberSet = new Set(selectedNumbers);
                                const selectedIndexes = (draftView.contracts || [])
                                  .map((contract, index) => selectedNumberSet.has(contract.contract_number) ? index : -1)
                                  .filter((index) => index >= 0);
                                const createDraftAction = await recordReviewAction(
                                  activeSessionId!,
                                  draftView.artifactId,
                                  "create_draft",
                                  {
                                    selected_count: selectedContracts.length,
                                    selected_indexes: selectedIndexes,
                                    contract_numbers: selectedNumbers,
                                  },
                                );
                                const result = createDraftAction.draftResult;
                                const createdCount = Number(result?.created_count || 0);
                                const replayedCount = Number(result?.replayed_count || 0);
                                const failedCount = Number(result?.failed_count || 0);
                                const successfulCount = createdCount + replayedCount;
                                if (failedCount === 0) {
                                  message.success(t("ai.batch_create_success", language, { count: String(successfulCount) }));
                                } else {
                                  message.warning(t("ai.batch_create_result", language, { success: String(successfulCount), failed: String(failedCount), details: "" }));
                                }
                                if (activeSession?.serverSessionId && createDraftAction.id) {
                                  setLoading(true);
                                  try {
                                    await continueFromTarget(
                                      activeSessionId!,
                                      activeSession.serverSessionId,
                                      { type: "action", id: createDraftAction.id },
                                      { contractId: searchParams.get("contract_id") || undefined }
                                    );
                                  } catch (error: any) {
                                    setLoading(false);
                                    notifyError(t("ai.request_failed", language, { error: error.message || t("ai.unknown_error", language) }));
                                  }
                                }
                              } catch (error: any) {
                                notifyError(t("ai.batch_create_failed", language, { error: error.message }));
                              }
                            }}
                            onSkip={() => {
                              (async () => {
                                try {
                                  let skipAction: RuntimeReviewAction | undefined;
                                  const draftView = getContractDraftView(msg);
                                  if (draftView?.artifactId) {
                                    skipAction = await recordReviewAction(
                                      activeSessionId!,
                                      draftView.artifactId,
                                      "skip",
                                      {
                                        reason: "user_skipped_contract_draft_import",
                                      },
                                    );
                                  }
                                  if (activeSession?.serverSessionId && skipAction?.id) {
                                    setLoading(true);
                                    try {
                                      await continueFromTarget(
                                        activeSessionId!,
                                        activeSession.serverSessionId,
                                        { type: "action", id: skipAction.id },
                                        { contractId: searchParams.get("contract_id") || undefined }
                                      );
                                    } catch (error: any) {
                                      setLoading(false);
                                      notifyError(t("ai.request_failed", language, { error: error.message || t("ai.unknown_error", language) }));
                                    }
                                  }
                                } catch (error: any) {
                                  notifyError(t("ai.request_failed", language, { error: error.message || t("ai.unknown_error", language) }));
                                }
                              })();
                            }}
                          />
                        )}

                        {getPaymentScheduleDraftView(msg) && msg.role === "assistant" && (
                          <PaymentScheduleDraftPanel
                            schedules={getPaymentScheduleDraftView(msg)!.schedules}
                            summary={getPaymentScheduleDraftView(msg)!.summary}
                            language={language}
                            onConfirm={async (selectedSchedules) => {
                              const scheduleView = getPaymentScheduleDraftView(msg);
                              const contractId = scheduleView?.summary.contract_id;
                              if (!contractId) {
                                message.warning(t("ai.schedule_bind_contract_first", language));
                                return;
                              }
                              try {
                                if (!scheduleView?.artifactId) {
                                  throw new Error("付款计划草稿 Artifact 不存在，无法提交确认");
                                }
                                const selectedIndexes = (scheduleView.schedules || [])
                                  .map((schedule, index) => selectedSchedules.includes(schedule) ? index : -1)
                                  .filter((index) => index >= 0);
                                const importActionResponse = await recordReviewAction(
                                  activeSessionId!,
                                  scheduleView.artifactId,
                                  "import",
                                  {
                                    selected_count: selectedSchedules.length,
                                    selected_indexes: selectedIndexes,
                                    contract_id: contractId,
                                  },
                                );
                                const result = importActionResponse.draftResult;
                                const createdCount = Number(result?.created_count || 0);
                                const replayedCount = Number(result?.replayed_count || 0);
                                const failedCount = Number(result?.failed_count || 0);
                                const successCount = createdCount + replayedCount;
                                if (failedCount === 0) {
                                  message.success(t("ai.schedule_import_success", language, { count: String(successCount) }));
                                } else {
                                  message.warning(t("ai.batch_create_result", language, { success: String(successCount), failed: String(failedCount), details: "" }));
                                }
                                if (activeSession?.serverSessionId && importActionResponse.id) {
                                  setLoading(true);
                                  try {
                                    await continueFromTarget(
                                      activeSessionId!,
                                      activeSession.serverSessionId,
                                      { type: "action", id: importActionResponse.id },
                                      { contractId }
                                    );
                                  } catch (error: any) {
                                    setLoading(false);
                                    notifyError(t("ai.request_failed", language, { error: error.message || t("ai.unknown_error", language) }));
                                  }
                                }
                              } catch (error: any) {
                                notifyError(t("ai.schedule_import_failed", language, { error: error.message }));
                              }
                            }}
                            onSkip={() => {
                              (async () => {
                                try {
                                  let skipAction: RuntimeReviewAction | undefined;
                                  const scheduleView = getPaymentScheduleDraftView(msg);
                                  if (scheduleView?.artifactId) {
                                    skipAction = await recordReviewAction(
                                      activeSessionId!,
                                      scheduleView.artifactId,
                                      "skip",
                                      {
                                        reason: "user_skipped_payment_schedule_import",
                                        contract_id: scheduleView.summary.contract_id,
                                      },
                                    );
                                  }
                                  if (activeSession?.serverSessionId && skipAction?.id) {
                                    setLoading(true);
                                    try {
                                      await continueFromTarget(
                                        activeSessionId!,
                                        activeSession.serverSessionId,
                                        { type: "action", id: skipAction.id },
                                        { contractId: scheduleView?.summary.contract_id }
                                      );
                                    } catch (error: any) {
                                      setLoading(false);
                                      notifyError(t("ai.request_failed", language, { error: error.message || t("ai.unknown_error", language) }));
                                    }
                                  }
                                } catch (error: any) {
                                  notifyError(t("ai.request_failed", language, { error: error.message || t("ai.unknown_error", language) }));
                                }
                              })();
                            }}
                          />
                        )}

                        {msg.id !== "welcome" && activeSession?.serverSessionId && (
                          <div
                            style={{
                              marginTop: 10,
                              paddingTop: 8,
                              borderTop: msg.role === "user" ? "1px solid rgba(255,255,255,0.12)" : "1px solid var(--border-subtle, #F1F5F9)",
                              display: "flex",
                              gap: 6,
                              flexWrap: "wrap",
                              alignItems: "center",
                            }}
                          >
                            <Button
                              type="text"
                              size="small"
                              icon={<MessageOutlined style={{ fontSize: 11 }} />}
                              disabled={loading}
                              onClick={() => triggerRuntimeContinuation({ type: "message", id: msg.id })}
                              style={{
                                fontSize: 11,
                                paddingInline: 6,
                                height: 22,
                                color: msg.role === "user" ? "rgba(255,255,255,0.8)" : "var(--fg-muted, #94A3B8)",
                              }}
                            >
                              {t("ai.continue_from_message", language)}
                            </Button>

                            {msg.runId && (
                              <>
                                <Button
                                  type="text"
                                  size="small"
                                  icon={<ClockCircleOutlined style={{ fontSize: 11 }} />}
                                  onClick={() => handleOpenTrace(msg.runId!)}
                                  style={{
                                    fontSize: 11,
                                    paddingInline: 6,
                                    height: 22,
                                    color: msg.role === "user" ? "rgba(255,255,255,0.8)" : "var(--fg-muted, #94A3B8)",
                                  }}
                                >
                                  {t("ai.view_trace", language)}
                                </Button>
                                {msg.role === "assistant" && (
                                  <>
                                    <Button
                                      type="text"
                                      size="small"
                                      icon={<MessageOutlined style={{ fontSize: 11 }} />}
                                      disabled={loading}
                                      onClick={() => handleRunControl(msg.runId!, "follow-up")}
                                      style={{
                                        fontSize: 11,
                                        paddingInline: 6,
                                        height: 22,
                                        color: "var(--fg-muted, #94A3B8)",
                                      }}
                                    >
                                      {t("ai.run_follow_up", language)}
                                    </Button>
                                  </>
                                )}
                              </>
                            )}

                            {(msg.artifacts?.length || msg.contractDraftArtifactId || msg.paymentScheduleArtifactId) && (
                              <Button
                                type="text"
                                size="small"
                                icon={<FileTextOutlined />}
                                disabled={loading}
                                onClick={() =>
                                  triggerRuntimeContinuation({
                                    type: "artifact",
                                    id: msg.artifacts?.[0]?.id || msg.contractDraftArtifactId || msg.paymentScheduleArtifactId!,
                                  })
                                }
                                className="sty-e6452550"
                              >
                                {t("ai.continue_from_artifact", language)}
                              </Button>
                            )}
                          </div>
                        )}
                      </div>
                    </Space>
                  </motion.div>
                ))}
              </AnimatePresence>

              {loading && !typingMessageId && (
                <motion.div
                  initial={false}
                  animate={{ opacity: 1 }}
                  className="sty-ab5dc9e7"
                >
                  <Avatar
                    icon={<RobotOutlined />}
                    className="sty-e495fd05"
                    size={32}
                  />
                  <div
                    className="sty-b561115b"
                  >
                    <Spin />
                    <span className="sty-879ad78b">
                      {t("ai.thinking", language)}
                    </span>
                    <Typography.Text type="secondary" className="ai-thinking-progress">
                      {t("ai.thinking_progress", language)}
                    </Typography.Text>
                  </div>
                </motion.div>
              )}

              <div ref={messagesEndRef} />
            </div>

            {/* Input Area */}
            <div
              className="ai-chat-input"
              style={{
                padding: "16px 24px 20px",
                background: "var(--bg-page)",
                borderTop: "1px solid var(--border-default)",
                width: "100%",
                flexShrink: 0,
              }}
            >
              <div style={{ maxWidth: 960, width: "100%", margin: "0 auto" }}>
                {activePendingUpload && (
                  <div
                    style={{
                      display: "inline-flex",
                      alignItems: "center",
                      gap: 8,
                      padding: "4px 10px",
                      background: "var(--bg-inset)",
                      borderRadius: 6,
                      fontSize: 12,
                      marginBottom: 8,
                      border: "1px solid var(--border-default)",
                    }}
                  >
                    <PaperClipOutlined style={{ color: "var(--fg-muted)" }} />
                    <span style={{ maxWidth: 200, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                      {activePendingUpload.original_name}
                    </span>
                    <Button
                      type="text"
                      size="small"
                      icon={<CloseCircleOutlined style={{ fontSize: 12, color: "var(--fg-muted)" }} />}
                      onClick={() => activeSessionId && setPendingUpload(activeSessionId, null)}
                      style={{ padding: 0, height: 16, width: 16 }}
                    />
                  </div>
                )}
                <div
                  className="chat-input-wrapper"
                  style={{
                    display: "flex",
                    alignItems: "center",
                    gap: 10,
                    padding: "8px 14px",
                    background: "var(--bg-elevated)",
                    borderRadius: 24,
                    border: "1px solid var(--border-default)",
                    boxShadow: "0 2px 8px rgba(0, 0, 0, 0.04)",
                    transition: "border-color 0.2s, box-shadow 0.2s",
                  }}
                >
                  <Upload
                    customRequest={handleFileUpload}
                    showUploadList={false}
                    disabled={loading}
                    beforeUpload={(file) => {
                      const allowedTypes = [
                        "application/pdf",
                        "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
                        "application/vnd.ms-excel",
                        "image/jpeg",
                        "image/png",
                        "image/tiff",
                      ];
                      if (!allowedTypes.includes(file.type)) {
                        notifyError(t("ai.unsupported_file", language));
                        return Upload.LIST_IGNORE;
                      }
                      const isLt50M = file.size / 1024 / 1024 < 50;
                      if (!isLt50M) {
                        notifyError(t("ai.file_too_large", language));
                        return Upload.LIST_IGNORE;
                      }
                      return true;
                    }}
                  >
                    <Tooltip title={t("ai.upload_file_tooltip", language)}>
                      <Button
                        type="text"
                        shape="circle"
                        icon={<PaperClipOutlined style={{ color: "var(--fg-muted)", fontSize: 16 }} />}
                        disabled={loading}
                        style={{ width: 32, height: 32 }}
                      />
                    </Tooltip>
                  </Upload>

                  <TextArea
                    value={input}
                    onChange={(e) => setInput(e.target.value)}
                    onKeyDown={handleKeyDown}
                    placeholder={t("ai.placeholder", language)}
                    autoSize={{ minRows: 1, maxRows: 6 }}
                    style={{
                      flex: 1,
                      background: "transparent",
                      border: "none",
                      boxShadow: "none",
                      resize: "none",
                      fontSize: 14,
                      lineHeight: 1.6,
                      padding: "4px 0",
                    }}
                    onFocus={(e) => {
                      const parent = e.currentTarget.closest(".chat-input-wrapper") as HTMLElement | null;
                      if (parent) {
                        parent.style.borderColor = "var(--fg-primary)";
                        parent.style.boxShadow = "0 2px 12px rgba(0, 0, 0, 0.08)";
                      }
                    }}
                    onBlur={(e) => {
                      const parent = e.currentTarget.closest(".chat-input-wrapper") as HTMLElement | null;
                      if (parent) {
                        parent.style.borderColor = "var(--border-default)";
                        parent.style.boxShadow = "0 2px 8px rgba(0, 0, 0, 0.04)";
                      }
                    }}
                  />

                  <Button
                    type="primary"
                    shape="circle"
                    icon={<SendOutlined style={{ fontSize: 13 }} />}
                    onClick={() => handleSend()}
                    loading={loading}
                    disabled={loading || (!input.trim() && !activePendingUpload)}
                    style={{ width: 32, height: 32 }}
                  />
                </div>

                <div style={{ textAlign: "center", marginTop: 8 }}>
                  <Text
                    type="secondary"
                    style={{ fontSize: 11, color: "var(--fg-muted)" }}
                  >
                    {t("ai.disclaimer", language)}
                  </Text>
                </div>
              </div>
            </div>
          </div>
        </div>
        {responsiveState.showMobileSessionTrigger && (
          <Drawer
            title={t("nav.ai_chat", language)}
            placement="left"
            width={280}
            open={sessionDrawerOpen}
            onClose={closeSessionDrawer}
            bodyStyle={{ padding: 0 }}
            destroyOnClose={false}
          >
            <SessionSidebar
              sessions={sessions}
              activeSessionId={activeSessionId}
              onSelect={(id) => {
                setActiveSessionId(id);
                transitionSessionDrawer("selection");
              }}
              onNew={() => {
                createNewSession();
                transitionSessionDrawer("new");
              }}
              onDelete={confirmDeleteSession}
            />
          </Drawer>
        )}
        <Modal
          open={Boolean(traceRunId)}
          title={t("ai.agent_trace_title", language)}
          footer={null}
          width={820}
          onCancel={() => {
            setTraceRunId(null);
            setTraceData(null);
          }}
        >
          {traceLoading ? (
            <div className="sty-37c08d18">
              <Spin />
            </div>
          ) : traceData ? (
            <div>
              <Space wrap className="sty-2122cedf">
                <StatusTag kind="processing">{String(traceData.run?.status || "unknown")}</StatusTag>
                <StatusTag>{`events: ${Array.isArray(traceData.events) ? traceData.events.length : 0}`}</StatusTag>
                <StatusTag>{`artifacts: ${Array.isArray(traceData.artifacts) ? traceData.artifacts.length : 0}`}</StatusTag>
                <StatusTag>{`reviews: ${Array.isArray(traceData.review_actions) ? traceData.review_actions.length : 0}`}</StatusTag>
                <StatusTag>{`audits: ${traceData.audit_total ?? 0}`}</StatusTag>
                {traceData.summary && (
                  <>
                    <StatusTag kind={statusKindFromAntColor(traceData.summary.terminal ? "green" : "gold")}>
                      {`summary events: ${traceData.summary.event_count ?? 0}`}
                    </StatusTag>
                    <StatusTag>{`tools: ${traceData.summary.tool_event_count ?? 0}`}</StatusTag>
                    <StatusTag>{`failed: ${traceData.summary.failed_event_count ?? 0}`}</StatusTag>
                  </>
                )}
              </Space>
              <pre
                className="sty-50db4c92"
              >
                {JSON.stringify(traceData, null, 2)}
              </pre>
            </div>
          ) : (
            <Empty description={t("ai.agent_trace_empty", language)} />
          )}
        </Modal>
      </AppLayout>
    </ProtectedRoute>
  );
}

export default function AIChatPage() {
  return (
    <Suspense
      fallback={
        <div style={{ display: "flex", height: "100vh", alignItems: "center", justifyContent: "center" }}>
          <Spin size="large" />
        </div>
      }
    >
      <AIChatPageContent />
    </Suspense>
  );
}
