"use client";

import React, { useState } from "react";
import { Button, Drawer, Empty, Input, Space, Spin, Typography } from "antd";
import { SendOutlined } from "@ant-design/icons";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { aiChatApi, apiErrorMessage } from "../lib/api";
import ToolChip from "./ToolChip";
import SourceCitation from "./SourceCitation";
import ConfidenceBadge from "./ConfidenceBadge";
import DataTrustBar from "./DataTrustBar";

import { SparkleGlyph } from "./MonochromeGlyphs";
import MarkdownText from "../ai-chat/MarkdownText";

// AI-002: the same-page AI drawer for the retail pages. The /ai-chat page
// stays; this drawer gives the current page an AI panel without dropping
// the context. Chat goes through the existing aiChatApi with the page
// context attached — no new backend surface.
export interface RetailAIDrawerContext {
  page: string;
  title?: string;
  filters?: Record<string, string>;
  summary?: string;
}

interface DrawerMessage {
  role: "user" | "assistant";
  content: string;
  sources?: Array<{ title?: string; url?: string; type?: string } | string>;
  confidence?: number;
  toolCalls?: Array<{ tool: string; status: string; output_summary?: string; duration_ms?: number }>;
  envelope?: unknown;
}

// The chat panel is exported separately from the Drawer shell so tests can
// render it without AntD's client-side portal (rc-drawer returns null in
// static markup).
const drawerSessionKey = (page: string) => `retail-ai-drawer:${page}`;

function loadDrawerSession(page: string): { messages: DrawerMessage[]; sessionId: string } {
  try {
    const raw = window.sessionStorage.getItem(drawerSessionKey(page));
    if (raw) {
      const parsed = JSON.parse(raw) as { messages?: DrawerMessage[]; session_id?: string };
      return { messages: parsed.messages || [], sessionId: parsed.session_id || "" };
    }
  } catch {
    // Corrupted session storage falls back to a fresh conversation.
  }
  return { messages: [], sessionId: "" };
}

export function RetailAIDrawerPanel({ pageContext }: { pageContext: RetailAIDrawerContext }) {
  const { token } = useAuth();
  const { language } = useLanguage();
  // P3-32: the conversation survives page navigation — transcript and
  // session id persist per page in sessionStorage and the server resumes
  // the same session (042/043 session entities) when the id is sent.
  const [persisted] = useState(() => loadDrawerSession(pageContext.page));
  const [messages, setMessages] = useState<DrawerMessage[]>(persisted.messages);
  const [sessionId, setSessionId] = useState(persisted.sessionId);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);

  const persist = (nextMessages: DrawerMessage[], nextSessionId: string) => {
    try {
      window.sessionStorage.setItem(drawerSessionKey(pageContext.page), JSON.stringify({ messages: nextMessages, session_id: nextSessionId }));
    } catch {
      // Storage unavailable — the conversation still lives in component state.
    }
  };

  const send = async () => {
    const message = input.trim();
    if (!message || !token || loading) return;
    setInput("");
    setLoading(true);
    setMessages((current) => [...current, { role: "user", content: message }]);
    try {
      const response = await aiChatApi.chat(
        {
          message, language, session_id: sessionId || undefined,
          history: messages.filter((item) => item.role === "user" || item.role === "assistant").map((item) => ({ role: item.role as "user" | "assistant", content: item.content })),
          page_context: { page: pageContext.page, title: pageContext.title, filters: pageContext.filters, summary: pageContext.summary },
        },
        token
      );
      setMessages((current) => {
        const next = [...current, {
          role: "assistant" as const,
          content: response.answer || t("ai.drawer.no_answer", language),
          sources: response.sources,
          confidence: typeof response.confidence === "number" ? response.confidence : undefined,
          toolCalls: response.tool_calls,
          envelope: response.retail_action_proposal?.envelope || response.retail_operations?.envelope,
        }];
        persist(next, response.session_id || sessionId);
        return next;
      });
      if (response.session_id) setSessionId(response.session_id);
    } catch (error) {
      setMessages((current) => [...current, { role: "assistant", content: apiErrorMessage(error) }]);
    } finally {
      setLoading(false);
    }
  };

  return (
      <div className="retail-ai-drawer">
        <div className="retail-ai-drawer-context">
          <Typography.Text type="secondary">{t("ai.drawer.context", language)}</Typography.Text>
          <Typography.Text strong>{pageContext.title || pageContext.page}</Typography.Text>
        </div>
        <div className="retail-ai-drawer-messages">
          {messages.length === 0 && !loading ? (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("ai.drawer.empty", language)} />
          ) : (
            messages.map((message, index) => (
              <div key={index} className={`retail-ai-drawer-message is-${message.role}`}>
                {message.toolCalls && message.toolCalls.length > 0 && (
                  <div className="ai-tool-row">
                    {message.toolCalls.map((call, idx) => <ToolChip key={idx} call={call} />)}
                  </div>
                )}
                {typeof message.confidence === "number" && (
                  <div className="ai-confidence-row"><ConfidenceBadge confidence={message.confidence} /></div>
                )}
                {message.role === "assistant" ? (
                  <MarkdownText content={message.content} />
                ) : (
                  <Typography.Paragraph>{message.content}</Typography.Paragraph>
                )}
                {message.envelope ? <DataTrustBar envelope={message.envelope as never} /> : null}
                {message.sources && message.sources.length > 0 && (
                  <div className="ai-tool-row">
                    {message.sources.map((source, idx) => <SourceCitation key={idx} source={source} />)}
                  </div>
                )}
              </div>
            ))
          )}
          {loading && <Spin size="small" />}
        </div>
        <Space.Compact className="retail-ai-drawer-input">
          <Input
            value={input}
            onChange={(event) => setInput(event.target.value)}
            onPressEnter={send}
            placeholder={t("ai.drawer.placeholder", language)}
            disabled={loading}
          />
          <Button type="primary" icon={<SendOutlined />} onClick={send} loading={loading}>
            {t("ai.drawer.send", language)}
          </Button>
        </Space.Compact>
      </div>
  );
}

export default function RetailAIDrawer({
  open,
  onClose,
  pageContext,
}: {
  open: boolean;
  onClose: () => void;
  pageContext: RetailAIDrawerContext;
}) {
  const { language } = useLanguage();
  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={
        <Space>
          <SparkleGlyph size={14} />
          <span>{t("ai.drawer.title", language)}</span>
        </Space>
      }
      placement="right"
      width={460}
      classNames={{ body: "app-drawer-body" }}
    >
      <RetailAIDrawerPanel pageContext={pageContext} />
    </Drawer>
  );
}
