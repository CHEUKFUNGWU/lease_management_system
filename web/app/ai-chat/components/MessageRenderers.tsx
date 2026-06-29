"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { Button, Tag, Typography } from "antd";
import {
  CheckCircleOutlined,
  CheckOutlined,
  ClockCircleOutlined,
  CopyOutlined,
  ExclamationCircleOutlined,
  FileTextOutlined,
  MessageOutlined,
  ToolOutlined,
} from "@ant-design/icons";
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import { vscDarkPlus } from "react-syntax-highlighter/dist/esm/styles/prism";
import { t, type Language } from "../../lib/i18n";
import type {
  AgentPlanStep,
  AgentReviewPrompt,
  AgentToolCall,
  RuntimeReviewAction,
} from "../../lib/types/ai-chat";

const { Text } = Typography;

function formatTime(timestamp: number): string {
  const date = new Date(timestamp);
  const now = new Date();
  const isToday = date.toDateString() === now.toDateString();
  if (isToday) {
    return date.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
  }
  return date.toLocaleDateString("zh-CN", { month: "short", day: "numeric" });
}

function reviewActionLabel(actionType: string, language: Language): string {
  switch (actionType) {
    case "confirm":
      return t("ai.review_action_confirm", language);
    case "skip":
      return t("ai.review_action_skip", language);
    case "import":
      return t("ai.review_action_import", language);
    case "create_draft":
      return t("ai.review_action_create_draft", language);
    case "reject":
      return t("ai.review_action_reject", language);
    default:
      return actionType;
  }
}

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

function reviewSeverityMeta(severity: string) {
  if (severity === "critical") {
    return { color: "#CF1322", background: "#FFF1F0", border: "#FFA39E" };
  }
  if (severity === "warning") {
    return { color: "#D46B08", background: "#FFF7E6", border: "#FFD591" };
  }
  return { color: "#0958D9", background: "#E6F4FF", border: "#91CAFF" };
}

function CodeBlock({ code, language, i18nLang }: { code: string; language: string; i18nLang: Language }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(code);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <div style={{ position: "relative", marginTop: 8, marginBottom: 8 }}>
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          padding: "4px 12px",
          background: "#1E1E1E",
          borderRadius: "8px 8px 0 0",
          borderBottom: "1px solid #333",
        }}
      >
        <span style={{ fontSize: 11, color: "#888", fontFamily: "monospace" }}>
          {language || "code"}
        </span>
        <Button
          type="text"
          icon={copied ? <CheckOutlined style={{ color: "#10B981" }} /> : <CopyOutlined />}
          onClick={handleCopy}
          style={{ color: "#888", fontSize: 12, height: 24, padding: "0 8px" }}
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

export function MessageContent({
  content,
  sources,
  model,
  thinking,
  i18nLang,
  role = "assistant",
}: {
  content: string;
  sources?: string[];
  model?: string;
  thinking?: string;
  i18nLang: Language;
  role?: "user" | "assistant";
}) {
  const [showThinking, setShowThinking] = useState(false);
  const textColor = role === "user" ? "#fff" : "#262626";

  const parts = useMemo(() => {
    const result: { type: "text" | "code"; content: string; language?: string }[] = [];
    const regex = /```(\w+)?\n([\s\S]*?)```/g;
    let lastIndex = 0;
    let match: RegExpExecArray | null;

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
        <div style={{ marginBottom: 8 }}>
          <Button
            type="text"
            onClick={() => setShowThinking(!showThinking)}
            style={{ fontSize: 12, color: "#8C8C8C", padding: 0, height: 24 }}
          >
            <span style={{ marginRight: 4 }}>{showThinking ? "▼" : "▶"}</span>
            {t("ai.thinking_process", i18nLang)}
          </Button>
          <AnimatePresence>
            {showThinking && (
              <motion.div
                initial={{ height: 0, opacity: 0 }}
                animate={{ height: "auto", opacity: 1 }}
                exit={{ height: 0, opacity: 0 }}
                transition={{ duration: 0.2 }}
                style={{ overflow: "hidden" }}
              >
                <div
                  style={{
                    background: "#F7F7F7",
                    border: "1px solid #E5E5E5",
                    borderRadius: 6,
                    padding: "8px 12px",
                    marginTop: 4,
                    fontSize: 12,
                    color: "#595959",
                    lineHeight: 1.6,
                    whiteSpace: "pre-wrap",
                  }}
                >
                  {thinking}
                </div>
              </motion.div>
            )}
          </AnimatePresence>
        </div>
      )}

      {parts.map((part, idx) =>
        part.type === "code" ? (
          <CodeBlock key={idx} code={part.content} language={part.language || "text"} i18nLang={i18nLang} />
        ) : (
          <Text
            key={idx}
            style={{
              whiteSpace: "pre-wrap",
              lineHeight: 1.7,
              fontSize: 14,
              color: textColor,
            }}
          >
            {part.content}
          </Text>
        )
      )}

      {sources && sources.length > 0 && (
        <div style={{ marginTop: 12, display: "flex", flexWrap: "wrap", gap: 6 }}>
          <Text type="secondary" style={{ fontSize: 12, marginRight: 4 }}>
            {t("ai.sources", i18nLang)}
          </Text>
          {sources.map((source, idx) => (
            <Tag key={idx} icon={<FileTextOutlined />} style={{ fontSize: 11, borderRadius: 4 }}>
              {source}
            </Tag>
          ))}
        </div>
      )}

      {model && (
        <Text type="secondary" style={{ fontSize: 11, marginTop: 8, display: "block" }}>
          {t("ai.model_label", i18nLang)} {model}
        </Text>
      )}
    </div>
  );
}

export function TypewriterMessage({
  content,
  sources,
  model,
  thinking,
  i18nLang,
}: {
  content: string;
  sources?: string[];
  model?: string;
  thinking?: string;
  i18nLang: Language;
}) {
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
      i18nLang={i18nLang}
      role="assistant"
    />
  );
}

export function AgentTracePanel({
  plan = [],
  toolCalls = [],
  language,
}: {
  plan?: AgentPlanStep[];
  toolCalls?: AgentToolCall[];
  language: Language;
}) {
  if (plan.length === 0 && toolCalls.length === 0) return null;

  return (
    <div
      style={{
        marginTop: 12,
        border: "1px solid #E5E5E5",
        borderRadius: 8,
        overflow: "hidden",
        background: "#fff",
      }}
    >
      <div
        style={{
          padding: "10px 12px",
          borderBottom: "1px solid #F0F0F0",
          display: "flex",
          alignItems: "center",
          gap: 8,
        }}
      >
        <ToolOutlined style={{ color: "#262626" }} />
        <Text strong style={{ fontSize: 13 }}>
          {t("ai.agent_trace_title", language)}
        </Text>
      </div>

      {plan.length > 0 && (
        <div style={{ padding: "10px 12px", borderBottom: toolCalls.length > 0 ? "1px solid #F0F0F0" : "none" }}>
          <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
            {plan.map((step) => {
              const meta = statusMeta(step.status, language);
              return (
                <Tag key={step.id} color={meta.color} icon={meta.icon} style={{ borderRadius: 4, marginInlineEnd: 0 }}>
                  {step.title}
                </Tag>
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
                style={{
                  padding: "10px 12px",
                  borderBottom: index === toolCalls.length - 1 ? "none" : "1px solid #F5F5F5",
                }}
              >
                <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                  <Text strong style={{ fontSize: 12 }}>
                    {call.skill}
                  </Text>
                  <Tag color={meta.color} style={{ borderRadius: 4, fontSize: 11 }}>
                    {meta.label}
                  </Tag>
                  <Text type="secondary" style={{ fontSize: 11 }}>
                    {call.tool}
                  </Text>
                </div>
                <Text style={{ display: "block", fontSize: 12, color: "#595959", marginTop: 4 }}>
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

export function AgentReviewPanel({
  prompts = [],
  language,
}: {
  prompts?: AgentReviewPrompt[];
  language: Language;
}) {
  if (prompts.length === 0) return null;

  return (
    <div
      style={{
        marginTop: 12,
        border: "1px solid #E5E5E5",
        borderRadius: 8,
        overflow: "hidden",
        background: "#fff",
      }}
    >
      <div
        style={{
          padding: "10px 12px",
          borderBottom: "1px solid #F0F0F0",
          display: "flex",
          alignItems: "center",
          gap: 8,
        }}
      >
        <ExclamationCircleOutlined style={{ color: "#D46B08" }} />
        <Text strong style={{ fontSize: 13 }}>
          {t("ai.agent_review_title", language)}
        </Text>
      </div>

      <div style={{ display: "flex", flexDirection: "column" }}>
        {prompts.map((prompt, index) => {
          const meta = reviewSeverityMeta(prompt.severity);
          return (
            <div
              key={prompt.id || index}
              style={{
                padding: "10px 12px",
                borderBottom: index === prompts.length - 1 ? "none" : "1px solid #F5F5F5",
                background: meta.background,
              }}
            >
              <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                <Text strong style={{ fontSize: 12, color: meta.color }}>
                  {prompt.title}
                </Text>
                <Tag style={{ borderRadius: 4, color: meta.color, borderColor: meta.border, background: "#fff", fontSize: 11 }}>
                  {prompt.severity === "critical"
                    ? t("ai.agent_severity_critical", language)
                    : prompt.severity === "warning"
                      ? t("ai.agent_severity_warning", language)
                      : t("ai.agent_severity_info", language)}
                </Tag>
              </div>
              <Text style={{ display: "block", fontSize: 12, color: "#595959", marginTop: 4 }}>
                {prompt.description}
              </Text>
              <Text style={{ display: "block", fontSize: 12, color: "#262626", marginTop: 4 }}>
                {prompt.action}
              </Text>
              {prompt.contract_numbers && prompt.contract_numbers.length > 0 && (
                <div style={{ display: "flex", gap: 4, flexWrap: "wrap", marginTop: 6 }}>
                  {prompt.contract_numbers.map((contractNumber) => (
                    <Tag key={contractNumber} style={{ borderRadius: 4, marginInlineEnd: 0, fontSize: 11 }}>
                      {contractNumber}
                    </Tag>
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

export function ReviewActionHistoryPanel({
  actions = [],
  language,
  onContinue,
}: {
  actions?: RuntimeReviewAction[];
  language: Language;
  onContinue: (action: RuntimeReviewAction) => void;
}) {
  if (actions.length === 0) return null;

  return (
    <div
      style={{
        marginTop: 12,
        border: "1px solid #E5E5E5",
        borderRadius: 8,
        overflow: "hidden",
        background: "#fff",
      }}
    >
      <div
        style={{
          padding: "10px 12px",
          borderBottom: "1px solid #F0F0F0",
          display: "flex",
          alignItems: "center",
          gap: 8,
        }}
      >
        <ClockCircleOutlined style={{ color: "#595959" }} />
        <Text strong style={{ fontSize: 13 }}>
          {t("ai.review_action_history", language)}
        </Text>
      </div>

      <div style={{ display: "flex", flexDirection: "column" }}>
        {actions.map((action, index) => (
          <div
            key={action.id}
            style={{
              padding: "10px 12px",
              borderBottom: index === actions.length - 1 ? "none" : "1px solid #F5F5F5",
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: 12,
              flexWrap: "wrap",
            }}
          >
            <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
              <Tag style={{ borderRadius: 4, marginInlineEnd: 0 }}>
                {reviewActionLabel(action.actionType, language)}
              </Tag>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {formatTime(action.actedAt)}
              </Text>
              {action.comment && (
                <Text style={{ fontSize: 12, color: "#595959" }}>
                  {action.comment}
                </Text>
              )}
            </div>

            <Button
              type="text"
              size="small"
              icon={<MessageOutlined />}
              onClick={() => onContinue(action)}
              style={{ paddingInline: 8, color: "#595959" }}
            >
              {t("ai.continue_from_action", language)}
            </Button>
          </div>
        ))}
      </div>
    </div>
  );
}
