"use client";

import React, { useEffect, useRef, useState } from "react";
import { Button, Drawer, Empty, Input, Space, Spin, Typography } from "antd";
import { SendOutlined } from "@ant-design/icons";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { apiErrorMessage } from "../lib/api";
import { useAIChatRuntime, generateRuntimeId } from "../ai-chat/runtime";
import ToolChip from "./ToolChip";
import SourceCitation from "./SourceCitation";
import ConfidenceBadge from "./ConfidenceBadge";

import { SparkleGlyph } from "./MonochromeGlyphs";
import MarkdownText from "../ai-chat/MarkdownText";

// AI-002: the same-page AI drawer for the retail pages. Since stage 5 it
// runs on the UNIFIED chat runtime (useAIChatRuntime): SSE streaming,
// server sessions, review prompts and artifacts behave exactly as /ai-chat.
export interface RetailAIDrawerContext {
  page: string;
  title?: string;
  filters?: Record<string, string>;
  summary?: string;
}

// Each retail page owns one runtime session; the page→session map lives in
// sessionStorage so navigation resumes the same conversation (P3-32).
const drawerSessionMapKey = "retail-ai-drawer-sessions";

function loadPageSession(page: string): string {
  try {
    const raw = window.sessionStorage.getItem(drawerSessionMapKey);
    if (raw) {
      const map = JSON.parse(raw) as Record<string, string>;
      return map[page] || "";
    }
  } catch {
    // Corrupted storage falls back to a fresh session.
  }
  return "";
}

function savePageSession(page: string, sessionId: string) {
  try {
    const raw = window.sessionStorage.getItem(drawerSessionMapKey);
    const map = raw ? (JSON.parse(raw) as Record<string, string>) : {};
    map[page] = sessionId;
    window.sessionStorage.setItem(drawerSessionMapKey, JSON.stringify(map));
  } catch {
    // Storage unavailable — the mapping lives in component state only.
  }
}

export function RetailAIDrawerPanel({ pageContext }: { pageContext: RetailAIDrawerContext }) {
  const { token } = useAuth();
  const { language } = useLanguage();
  const runtime = useAIChatRuntime({ token, language, selectedModel: "deepseek-v4-flash" });
  const [pageSessionId, setPageSessionId] = useState<string>("");
  const [input, setInput] = useState("");
  const adopted = useRef(false);

  // Adopt the page's existing session, or create one, once local hydration
  // settles. Runs once per page context.
  useEffect(() => {
    if (!runtime.localHydrated || adopted.current) return;
    adopted.current = true;
    const mapped = loadPageSession(pageContext.page);
    const known = runtime.sessions.find((session) => session.id === mapped);
    if (known) {
      runtime.setActiveSessionId(known.id);
      setPageSessionId(known.id);
      return;
    }
    const session = runtime.createNewSession();
    savePageSession(pageContext.page, session.id);
    setPageSessionId(session.id);
    // The runtime object is recreated per render; the guard makes this run
    // exactly once per mounted panel.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [runtime.localHydrated]);

  const session = runtime.sessions.find((item) => item.id === pageSessionId) || null;

  const send = async () => {
    const message = input.trim();
    if (!message || !session) return;
    setInput("");
    runtime.updateSessionMessages(session.id, (messages) => [
      ...messages,
      { id: generateRuntimeId(), role: "user" as const, content: message, timestamp: Date.now() },
    ]);
    try {
      const serverSessionId = await runtime.ensureServerSession(session.id, {});
      await runtime.createAndStartRun(session.id, serverSessionId, {
        message,
        language,
        page_context: {
          page: pageContext.page,
          title: pageContext.title,
          filters: pageContext.filters,
          summary: pageContext.summary,
        },
      });
    } catch (error) {
      runtime.updateSessionMessages(session.id, (messages) => [
        ...messages,
        { id: generateRuntimeId(), role: "assistant" as const, content: apiErrorMessage(error), timestamp: Date.now() },
      ]);
    } finally {
      runtime.setLoading(false);
    }
  };

  return (
    <div className="retail-ai-drawer">
      <div className="retail-ai-drawer-context">
        <Typography.Text type="secondary">{t("ai.drawer.context", language)}</Typography.Text>
        <Typography.Text strong>{pageContext.title || pageContext.page}</Typography.Text>
      </div>
      <div className="retail-ai-drawer-messages">
        {(!session || session.messages.length === 0) && !runtime.loading ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("ai.drawer.empty", language)} />
        ) : (
          (session?.messages || []).map((message) => {
            if (!message.content && message.role === "assistant" && !message.artifacts?.length) return null;
            return (
              <div key={message.id || message.timestamp} className={`retail-ai-drawer-message is-${message.role}`}>
                {message.toolCalls && message.toolCalls.length > 0 && (
                  <div className="ai-tool-row">
                    {message.toolCalls.map((call, idx) => <ToolChip key={idx} call={call} />)}
                  </div>
                )}
                {typeof message.confidence === "number" && (
                  <div className="ai-confidence-row"><ConfidenceBadge confidence={message.confidence} /></div>
                )}
                {message.role === "assistant" ? (
                  message.content ? <MarkdownText content={message.content} /> : <Spin size="small" />
                ) : (
                  <Typography.Paragraph>{message.content}</Typography.Paragraph>
                )}
                {message.sources && message.sources.length > 0 && (
                  <div className="ai-tool-row">
                    {message.sources.map((source, idx) => <SourceCitation key={idx} source={source} />)}
                  </div>
                )}
              </div>
            );
          })
        )}
        {runtime.loading && !session && <Spin size="small" />}
      </div>
      <Space.Compact className="retail-ai-drawer-input">
        <Input
          value={input}
          onChange={(event) => setInput(event.target.value)}
          onPressEnter={send}
          placeholder={t("ai.drawer.placeholder", language)}
          disabled={runtime.loading || !session}
        />
        <Button type="primary" icon={<SendOutlined />} onClick={send} loading={runtime.loading && !session}>
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
