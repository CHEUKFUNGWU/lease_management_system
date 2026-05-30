"use client";

import { useState, useRef, useEffect, useMemo, Suspense, useCallback } from "react";
import { useSearchParams } from "next/navigation";
import { motion, AnimatePresence } from "framer-motion";
import {
  Card,
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
} from "@ant-design/icons";
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import { vscDarkPlus } from "react-syntax-highlighter/dist/esm/styles/prism";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { aiChatApi, contractApi } from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t, type Language } from "../lib/i18n";

const { TextArea } = Input;
const { Text } = Typography;

// ─── Types ─────────────────────────────────────────────────────

interface UploadedFile {
  file_id: string;
  original_name: string;
  content_type: string;
  object_name?: string;
}

interface ContractDraftItem {
  contract_number: string;
  contract_name: string;
  lessee: string;
  lessor: string;
  store_name: string;
  store_address: string;
  commencement_date: string;
  lease_start_date: string;
  lease_end_date: string;
  currency: string;
  fixed_rent_amount: number;
  payment_frequency: string;
  payment_timing: string;
  renewal_option: boolean;
  termination_option: boolean;
  cam_amount: number;
  service_fee: number;
  discount_rate_type: string;
  discount_rate: number;
  confidence: number;
  missing_fields: string[];
  warnings: string[];
}

interface BatchParseSummary {
  total_count: number;
  overall_confidence: number;
  requires_human_confirmation: boolean;
  missing_fields: string[];
  warnings: string[];
}

interface Message {
  id: string;
  role: "user" | "assistant";
  content: string;
  timestamp: number;
  sources?: string[];
  attachments?: UploadedFile[];
  model?: string;
  thinking?: string;
  draftContracts?: ContractDraftItem[];
  batchSummary?: BatchParseSummary;
}

interface ChatSession {
  id: string;
  title: string;
  messages: Message[];
  createdAt: number;
  updatedAt: number;
  model: string;
}

// ─── Constants ─────────────────────────────────────────────────

const SESSIONS_KEY = "ifrs16_chat_sessions";
const ACTIVE_SESSION_KEY = "ifrs16_chat_active_session";

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

// ─── Helpers ───────────────────────────────────────────────────

function generateId() {
  return Date.now().toString(36) + Math.random().toString(36).substr(2, 9);
}

function getSessionTitle(messages: Message[], language: Language): string {
  if (messages.length === 0) return t("ai.new_session", language);
  const firstUserMsg = messages.find((m) => m.role === "user");
  if (!firstUserMsg) return t("ai.new_session", language);
  const text = firstUserMsg.content;
  return text.length > 20 ? text.slice(0, 20) + "..." : text;
}

function formatTime(timestamp: number): string {
  const date = new Date(timestamp);
  const now = new Date();
  const isToday = date.toDateString() === now.toDateString();
  if (isToday) {
    return date.toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
  }
  return date.toLocaleDateString("zh-CN", { month: "short", day: "numeric" });
}

function loadSessions(): ChatSession[] {
  try {
    const raw = localStorage.getItem(SESSIONS_KEY);
    if (raw) {
      const sessions = JSON.parse(raw);
      return Array.isArray(sessions) ? sessions : [];
    }
  } catch {
    // ignore
  }
  return [];
}

function saveSessions(sessions: ChatSession[]) {
  localStorage.setItem(SESSIONS_KEY, JSON.stringify(sessions));
}

function loadActiveSessionId(): string | null {
  try {
    return localStorage.getItem(ACTIVE_SESSION_KEY);
  } catch {
    return null;
  }
}

function saveActiveSessionId(id: string | null) {
  if (id) {
    localStorage.setItem(ACTIVE_SESSION_KEY, id);
  } else {
    localStorage.removeItem(ACTIVE_SESSION_KEY);
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

// ─── Message Content Renderer ──────────────────────────────────

function MessageContent({ content, sources, model, thinking, i18nLang }: { content: string; sources?: string[]; model?: string; thinking?: string; i18nLang: Language }) {
  const [showThinking, setShowThinking] = useState(false);

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
              color: "#262626",
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
            <Tag
              key={idx}
              icon={<FileTextOutlined />}
             
              style={{ fontSize: 11, borderRadius: 4 }}
            >
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

// ─── Typewriter Effect ─────────────────────────────────────────

function TypewriterMessage({ content, sources, model, thinking, i18nLang }: { content: string; sources?: string[]; model?: string; thinking?: string; i18nLang: Language }) {
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
    />
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
      style={{
        width: 260,
        borderRight: "1px solid #E5E5E5",
        background: "#FAFAFA",
        display: "flex",
        flexDirection: "column",
        height: "100%",
      }}
    >
      {/* Header */}
      <div
        style={{
          padding: "16px 12px",
          borderBottom: "1px solid #E5E5E5",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
        }}
      >
        <span style={{ fontSize: 14, fontWeight: 600, color: "#000" }}>
          {t("nav.ai_chat", language)}
        </span>
         <Tooltip title={t("ai.new_session_btn", language)}>
          <Button
            type="text"
            icon={<PlusOutlined />}
            onClick={onNew}
            style={{ color: "#000" }}
          />
        </Tooltip>
      </div>

      {/* Session List */}
      <div style={{ flex: 1, overflowY: "auto", padding: "8px" }}>
        {sessions.length === 0 ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={t("ai.no_sessions", language)}
            style={{ marginTop: 40 }}
          />
        ) : (
          <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            {sessions.map((session) => (
              <motion.div
                key={session.id}
                whileHover={{ scale: 1.01 }}
                whileTap={{ scale: 0.99 }}
              >
                <div
                  onClick={() => onSelect(session.id)}
                  style={{
                    padding: "10px 12px",
                    borderRadius: 8,
                    cursor: "pointer",
                    background: activeSessionId === session.id ? "#000" : "transparent",
                    color: activeSessionId === session.id ? "#fff" : "#262626",
                    transition: "all 0.15s",
                    display: "flex",
                    alignItems: "center",
                    gap: 10,
                    position: "relative",
                  }}
                  onMouseEnter={(e) => {
                    if (activeSessionId !== session.id) {
                      e.currentTarget.style.background = "#F0F0F0";
                    }
                  }}
                  onMouseLeave={(e) => {
                    if (activeSessionId !== session.id) {
                      e.currentTarget.style.background = "transparent";
                    }
                  }}
                >
                  <MessageOutlined
                    style={{
                      fontSize: 14,
                      flexShrink: 0,
                      color: activeSessionId === session.id ? "#fff" : "#8C8C8C",
                    }}
                  />
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div
                      style={{
                        fontSize: 13,
                        fontWeight: 500,
                        whiteSpace: "nowrap",
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                        lineHeight: 1.4,
                      }}
                    >
                      {session.title}
                    </div>
                    <div
                      style={{
                        fontSize: 11,
                        color: activeSessionId === session.id ? "rgba(255,255,255,0.6)" : "#8C8C8C",
                        marginTop: 2,
                        display: "flex",
                        alignItems: "center",
                        gap: 4,
                      }}
                    >
                      <ClockCircleOutlined style={{ fontSize: 10 }} />
                      {formatTime(session.updatedAt)}
                    </div>
                  </div>

                  {/* Delete button - only show on hover */}
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
                     
                      icon={<MoreOutlined />}
                      onClick={(e) => e.stopPropagation()}
                      style={{
                        opacity: 0,
                        transition: "opacity 0.15s",
                        color: activeSessionId === session.id ? "#fff" : "#8C8C8C",
                        padding: 0,
                        width: 24,
                        height: 24,
                      }}
                      className="session-more-btn"
                    />
                  </Dropdown>
                </div>
              </motion.div>
            ))}
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

  const hasLowConfidence = (c: ContractDraftItem) => c.confidence < 0.8 || c.missing_fields.length > 0;

  return (
    <div style={{ marginTop: 12, border: "1px solid #E5E5E5", borderRadius: 12, overflow: "hidden" }}>
      {/* Header */}
      <div style={{ padding: "12px 16px", background: "#FAFAFA", borderBottom: "1px solid #E5E5E5" }}>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
          <div>
            <Text strong style={{ fontSize: 14 }}>
              {t("ai.draft_panel_title", language)}
            </Text>
            <Text type="secondary" style={{ fontSize: 12, marginLeft: 8 }}>
              {t("ai.draft_panel_subtitle", language, { total: String(summary.total_count), confidence: String((summary.overall_confidence * 100).toFixed(0)) })}
            </Text>
          </div>
          <div style={{ display: "flex", gap: 8 }}>
            <Button size="small" onClick={toggleSelectAll}>
              {selectedIndices.size === editedContracts.length ? t("ai.deselect_all", language) : t("ai.select_all", language)}
            </Button>
            <Button size="small" danger onClick={onSkip}>
              {t("ai.skip", language)}
            </Button>
          </div>
        </div>
        {summary.requires_human_confirmation && (
          <div style={{ marginTop: 8, padding: "8px 12px", background: "#FFF7E6", borderRadius: 6, border: "1px solid #FFD591" }}>
            <Text style={{ fontSize: 12, color: "#D46B08" }}>
              ⚠️ {t("ai.draft_warning", language)}
            </Text>
          </div>
        )}
        {summary.warnings.length > 0 && (
          <div style={{ marginTop: 8 }}>
            {summary.warnings.slice(0, 3).map((w, i) => (
              <Text key={i} style={{ fontSize: 11, color: "#CF1322", display: "block" }}>
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
            style={{
              padding: "12px 16px",
              borderBottom: "1px solid #F0F0F0",
              background: selectedIndices.has(index) ? "#F6FFED" : "#fff",
              opacity: selectedIndices.has(index) ? 1 : 0.6,
            }}
          >
            <div style={{ display: "flex", alignItems: "flex-start", gap: 12 }}>
              <input
                type="checkbox"
                checked={selectedIndices.has(index)}
                onChange={() => toggleSelect(index)}
                style={{ marginTop: 4 }}
              />
              <div style={{ flex: 1 }}>
                {/* Row 1: Basic info */}
                <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginBottom: 8 }}>
                  <div style={{ flex: "1 1 200px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_contract_number", language)}</Text>
                    <Input
                      size="small"
                      value={contract.contract_number}
                      onChange={(e) => updateContract(index, "contract_number", e.target.value)}
                      style={{ fontSize: 13 }}
                    />
                  </div>
                  <div style={{ flex: "1 1 200px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_contract_name", language)}</Text>
                    <Input
                      size="small"
                      value={contract.contract_name}
                      onChange={(e) => updateContract(index, "contract_name", e.target.value)}
                      style={{ fontSize: 13 }}
                    />
                  </div>
                  <div style={{ flex: "1 1 150px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_currency", language)}</Text>
                    <Input
                      size="small"
                      value={contract.currency}
                      onChange={(e) => updateContract(index, "currency", e.target.value)}
                      style={{ fontSize: 13 }}
                      status={!contract.currency ? "error" : ""}
                    />
                  </div>
                </div>

                {/* Row 2: Parties */}
                <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginBottom: 8 }}>
                  <div style={{ flex: "1 1 200px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_lessee", language)}</Text>
                    <Input
                      size="small"
                      value={contract.lessee}
                      onChange={(e) => updateContract(index, "lessee", e.target.value)}
                      style={{ fontSize: 13 }}
                    />
                  </div>
                  <div style={{ flex: "1 1 200px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_lessor", language)}</Text>
                    <Input
                      size="small"
                      value={contract.lessor}
                      onChange={(e) => updateContract(index, "lessor", e.target.value)}
                      style={{ fontSize: 13 }}
                    />
                  </div>
                  <div style={{ flex: "1 1 150px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_store", language)}</Text>
                    <Input
                      size="small"
                      value={contract.store_name}
                      onChange={(e) => updateContract(index, "store_name", e.target.value)}
                      style={{ fontSize: 13 }}
                    />
                  </div>
                </div>

                {/* Row 3: Dates */}
                <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginBottom: 8 }}>
                  <div style={{ flex: "1 1 150px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_commencement_date", language)}</Text>
                    <Input
                      size="small"
                      value={contract.commencement_date}
                      onChange={(e) => updateContract(index, "commencement_date", e.target.value)}
                      style={{ fontSize: 13 }}
                    />
                  </div>
                  <div style={{ flex: "1 1 150px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_lease_start", language)}</Text>
                    <Input
                      size="small"
                      value={contract.lease_start_date}
                      onChange={(e) => updateContract(index, "lease_start_date", e.target.value)}
                      style={{ fontSize: 13 }}
                    />
                  </div>
                  <div style={{ flex: "1 1 150px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_lease_end", language)}</Text>
                    <Input
                      size="small"
                      value={contract.lease_end_date}
                      onChange={(e) => updateContract(index, "lease_end_date", e.target.value)}
                      style={{ fontSize: 13 }}
                    />
                  </div>
                </div>

                {/* Row 4: Financial */}
                <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginBottom: 8 }}>
                  <div style={{ flex: "1 1 120px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_fixed_rent", language)}</Text>
                    <Input
                      size="small"
                      value={contract.fixed_rent_amount}
                      onChange={(e) => updateContract(index, "fixed_rent_amount", parseFloat(e.target.value) || 0)}
                      style={{ fontSize: 13 }}
                    />
                  </div>
                  <div style={{ flex: "1 1 120px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_payment_timing", language)}</Text>
                    <Input
                      size="small"
                      value={contract.payment_timing}
                      onChange={(e) => updateContract(index, "payment_timing", e.target.value)}
                      style={{ fontSize: 13 }}
                    />
                  </div>
                  <div style={{ flex: "1 1 120px" }}>
                    <Text type="secondary" style={{ fontSize: 11 }}>{t("ai.draft_discount_rate", language)}</Text>
                    <Input
                      size="small"
                      value={contract.discount_rate}
                      onChange={(e) => updateContract(index, "discount_rate", parseFloat(e.target.value) || 0)}
                      style={{ fontSize: 13 }}
                      status={!contract.discount_rate ? "warning" : ""}
                    />
                  </div>
                </div>

                {/* Warnings & Confidence */}
                <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                  {hasLowConfidence(contract) && (
                    <Tag color="warning" style={{ fontSize: 11 }}>
                      {t("ai.draft_confidence", language, { value: String((contract.confidence * 100).toFixed(0)) })}
                    </Tag>
                  )}
                  {contract.missing_fields.length > 0 && (
                    <Tag color="error" style={{ fontSize: 11 }}>
                      {t("ai.draft_missing_fields", language, { fields: contract.missing_fields.join(", ") })}
                    </Tag>
                  )}
                  {contract.warnings.slice(0, 2).map((w, i) => (
                    <Text key={i} style={{ fontSize: 11, color: "#CF1322" }}>
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
      <div style={{ padding: "12px 16px", background: "#FAFAFA", borderTop: "1px solid #E5E5E5", display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <Text type="secondary" style={{ fontSize: 12 }}>
          {t("ai.draft_selected_count", language, { selected: String(selectedIndices.size), total: String(editedContracts.length) })}
        </Text>
        <Button
          type="primary"
          loading={creating}
          disabled={selectedIndices.size === 0}
          onClick={handleConfirm}
          style={{ background: "#000", borderColor: "#000" }}
        >
          {t("ai.draft_confirm_import", language)}
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

  // Page context from URL
  const pageContext = useMemo(() => {
    const page = searchParams.get("page");
    if (!page) return undefined;
    return {
      page,
      title: searchParams.get("title") || undefined,
      contract_id: searchParams.get("contract_id") || undefined,
      period: searchParams.get("period") || undefined,
      report_view: searchParams.get("report_view") || undefined,
      tags: searchParams.getAll("tags"),
      summary: searchParams.get("summary") || undefined,
    };
  }, [searchParams]);

  // Session state
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [lastUploadedFile, setLastUploadedFile] = useState<UploadedFile | null>(null);
  const [selectedModel, setSelectedModel] = useState("deepseek-v4-flash");
  const [typingMessageId, setTypingMessageId] = useState<string | null>(null);

  // Load sessions from localStorage on mount
  useEffect(() => {
    const loaded = loadSessions();
    if (loaded.length > 0) {
      setSessions(loaded);
      const activeId = loadActiveSessionId();
      if (activeId && loaded.find((s) => s.id === activeId)) {
        setActiveSessionId(activeId);
      } else {
        setActiveSessionId(loaded[0].id);
      }
    } else {
      // Create initial session
      createNewSession();
    }
  }, []);

  // Save sessions when they change
  useEffect(() => {
    if (sessions.length > 0) {
      saveSessions(sessions);
    }
  }, [sessions]);

  // Save active session id
  useEffect(() => {
    saveActiveSessionId(activeSessionId);
  }, [activeSessionId]);

  // Scroll to bottom when messages change
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [sessions, activeSessionId]);

  const activeSession = useMemo(
    () => sessions.find((s) => s.id === activeSessionId),
    [sessions, activeSessionId]
  );

  const chips = useMemo(() => {
    if (!pageContext) return defaultChips;
    return contextChipMap[pageContext.page!] || defaultChips;
  }, [pageContext]);

  const createNewSession = useCallback(() => {
    const newSession: ChatSession = {
      id: generateId(),
      title: t("ai.new_session", language),
      messages: [
        {
          id: "welcome",
          role: "assistant",
          content: t("ai.welcome", language),
          timestamp: Date.now(),
        },
      ],
      createdAt: Date.now(),
      updatedAt: Date.now(),
      model: selectedModel,
    };
    setSessions((prev) => [newSession, ...prev]);
    setActiveSessionId(newSession.id);
    setInput("");
    setLastUploadedFile(null);
  }, [language, selectedModel]);

  const deleteSession = useCallback((id: string) => {
    setSessions((prev) => {
      const filtered = prev.filter((s) => s.id !== id);
      if (filtered.length === 0) {
        // Create a new empty session
        const newSession: ChatSession = {
          id: generateId(),
      title: t("ai.new_session", language),
          messages: [
            {
              id: "welcome",
              role: "assistant",
              content: t("ai.welcome", language),
              timestamp: Date.now(),
            },
          ],
          createdAt: Date.now(),
          updatedAt: Date.now(),
          model: selectedModel,
        };
        setActiveSessionId(newSession.id);
        return [newSession];
      }
      // If deleting active session, switch to first available
      setActiveSessionId((current) => {
        if (current === id) {
          return filtered[0].id;
        }
        return current;
      });
      return filtered;
    });
  }, [language, selectedModel]);

  const updateSessionMessages = useCallback(
    (sessionId: string, updater: (messages: Message[]) => Message[]) => {
      setSessions((prev) =>
        prev.map((s) => {
          if (s.id !== sessionId) return s;
          const newMessages = updater(s.messages);
          return {
            ...s,
            messages: newMessages,
            title: getSessionTitle(newMessages, language),
            updatedAt: Date.now(),
          };
        })
      );
    },
    [language]
  );

  const getFileIcon = (type: string) => {
    if (type.includes("pdf")) return <FilePdfOutlined style={{ color: "#EF4444" }} />;
    if (type.includes("excel") || type.includes("sheet"))
      return <FileExcelOutlined style={{ color: "#10B981" }} />;
    return <FileImageOutlined style={{ color: "#666" }} />;
  };

  const handleFileUpload = async (options: any) => {
    const { file, onSuccess, onError } = options;
    const formData = new FormData();
    formData.append("file", file);
    formData.append("file_type", "contract");

    try {
      const response = await fetch(`${window.location.origin}/api/ai/files/upload`, {
        method: "POST",
        headers: {
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
        body: formData,
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => ({}));
        throw new Error(errorData.detail || `${t("ai.upload_failed", language, { name: "file" })}: ${response.status}`);
      }

      const data = await response.json();
      const uploadedFile: UploadedFile = {
        file_id: data.file_id,
        original_name: data.original_name,
        content_type: data.content_type,
        object_name: data.object_name,
      };

      setLastUploadedFile(uploadedFile);

      const userMessage: Message = {
        id: generateId(),
        role: "user",
        content: `${t("ai.file_uploaded", language)}${data.original_name}`,
        timestamp: Date.now(),
        attachments: [uploadedFile],
      };

      if (activeSessionId) {
        updateSessionMessages(activeSessionId, (msgs) => [...msgs, userMessage]);
      }

      message.success(`${data.original_name} ${t("ai.upload_success", language)}`);
      onSuccess(data, file);
    } catch (err: any) {
      onError(err);
      message.error(`${t("ai.upload_failed", language)}: ${err.message}`);
    }
  };

  const handleSend = async (messageOverride?: string) => {
    const msg = messageOverride ?? input;
    if (!msg.trim() || !token || !activeSessionId) return;

    const userMessage: Message = {
      id: generateId(),
      role: "user",
      content: msg,
      timestamp: Date.now(),
    };

    // Get history from active session
    const currentSession = sessions.find((s) => s.id === activeSessionId);
    const history = (currentSession?.messages || [])
      .filter((m) => m.id !== "welcome")
      .slice(-10)
      .map((m) => ({ role: m.role, content: m.content }));

    updateSessionMessages(activeSessionId, (msgs) => [...msgs, userMessage]);
    setInput("");
    setLoading(true);

    try {
      const chatData: any = { message: msg, history, language };
      if (lastUploadedFile) {
        chatData.file_id = lastUploadedFile.file_id;
        chatData.object_name = lastUploadedFile.object_name;
        chatData.content_type = lastUploadedFile.content_type;
        setLastUploadedFile(null);
      }
      if (pageContext) {
        chatData.page_context = {
          page: pageContext.page,
          title: pageContext.title,
          contract_id: pageContext.contract_id,
          period: pageContext.period,
          report_view: pageContext.report_view,
          summary: pageContext.summary,
        };
      }
      const contractIdFromUrl = searchParams.get("contract_id");
      if (contractIdFromUrl) {
        chatData.contract_id = contractIdFromUrl;
      }

      const data = await aiChatApi.chat(chatData, token);

      const aiMessage: Message = {
        id: generateId(),
        role: "assistant",
        content: data.answer || t("ai.could_not_understand", language),
        timestamp: Date.now(),
        sources: data.sources?.map((s: any) => s.title || s.type),
        model: data.model,
        thinking: data.thinking,
        draftContracts: data.draft_contracts,
        batchSummary: data.batch_summary,
      };

      updateSessionMessages(activeSessionId, (msgs) => [...msgs, aiMessage]);
      setTypingMessageId(aiMessage.id);

      // Clear typing indicator after animation
      setTimeout(() => {
        setTypingMessageId((current) => (current === aiMessage.id ? null : current));
      }, Math.min(aiMessage.content.length * 15 + 500, 3000));
    } catch (error: any) {
      const errorMessage: Message = {
        id: generateId(),
        role: "assistant",
        content: t("ai.request_failed", language, { error: error.message || t("ai.unknown_error", language) }),
        timestamp: Date.now(),
      };
      updateSessionMessages(activeSessionId, (msgs) => [...msgs, errorMessage]);
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
      handleSend();
    }
  };

  const currentMessages = activeSession?.messages || [];

  return (
    <ProtectedRoute>
      <AppLayout>
        <div
          style={{
            display: "flex",
            height: "calc(100vh - 64px)",
            margin: "-32px -48px",
            overflow: "hidden",
          }}
        >
          {/* Session Sidebar */}
          <SessionSidebar
            sessions={sessions}
            activeSessionId={activeSessionId}
            onSelect={setActiveSessionId}
            onNew={createNewSession}
            onDelete={(id) => {
              Modal.confirm({
                title: t("ai.delete_session_title", language),
                content: t("ai.delete_session_content", language),
                okText: t("ai.delete", language),
                okType: "danger",
                cancelText: t("ai.cancel", language),
                onOk: () => deleteSession(id),
              });
            }}
          />

          {/* Chat Area */}
          <div style={{ flex: 1, display: "flex", flexDirection: "column", minWidth: 0 }}>
            {/* Top Bar */}
            <div
              style={{
                height: 56,
                borderBottom: "1px solid #E5E5E5",
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                padding: "0 20px",
                background: "#fff",
                flexShrink: 0,
              }}
            >
              <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
                <RobotOutlined style={{ fontSize: 18, color: "#000" }} />
                <span style={{ fontSize: 15, fontWeight: 600, color: "#000" }}>
                  {activeSession?.title || t("ai.assistant_name", language)}
                </span>
              </div>

              <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
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
                   
                    style={{ fontSize: 13, color: "#595959" }}
                  >
                    {MODEL_OPTIONS.find((m) => m.value === selectedModel)?.label || selectedModel}
                    <DownOutlined style={{ fontSize: 10, marginLeft: 4 }} />
                  </Button>
                </Dropdown>

        <Tooltip title={t("ai.new_session_btn", language)}>
                  <Button
                    type="text"
                    icon={<PlusOutlined />}
                    onClick={createNewSession}
                    style={{ color: "#000" }}
                  />
                </Tooltip>
              </div>
            </div>

            {/* Messages Area */}
            <div
              style={{
                flex: 1,
                overflowY: "auto",
                padding: "20px 20%",
                background: "#fff",
              }}
            >
              {/* Context strip */}
              {pageContext && (
                <motion.div
                  initial={{ opacity: 0, y: -4 }}
                  animate={{ opacity: 1, y: 0 }}
                  style={{
                    padding: "8px 12px",
                    borderRadius: 8,
                    background: "#F7F7F7",
                    border: "1px solid #E5E5E5",
                    marginBottom: 16,
                    display: "flex",
                    alignItems: "center",
                    gap: 8,
                    flexWrap: "wrap",
                  }}
                >
                  <Text type="secondary" style={{ fontSize: 12, fontWeight: 500 }}>
                    {t("ai.context", language)}
                  </Text>
                  <Tag style={{ borderRadius: 4 }}>
                    {pageContext.title || pageContext.page}
                  </Tag>
                  {pageContext.contract_id && (
                    <Tag style={{ borderRadius: 4 }}>
                      {pageContext.contract_id}
                    </Tag>
                  )}
                  {pageContext.period && (
                    <Tag style={{ borderRadius: 4 }}>
                      {pageContext.period}
                    </Tag>
                  )}
                </motion.div>
              )}

              {/* Quick chips */}
              {currentMessages.length <= 1 && (
                <motion.div
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  style={{
                    display: "flex",
                    flexWrap: "wrap",
                    gap: 8,
                    marginBottom: 20,
                    alignItems: "center",
                  }}
                >
                  <Text type="secondary" style={{ fontSize: 12, marginRight: 4 }}>
                    {t("ai.quick_questions", language)}
                  </Text>
                  {chips.map((chipKey, idx) => (
                    <Button
                      key={idx}
                      
                      type="default"
                      onClick={() => handleChipClick(t(chipKey, language))}
                      disabled={loading}
                      style={{
                        fontSize: 12,
                        borderRadius: 9999,
                        borderColor: "#E5E5E5",
                        color: "#595959",
                      }}
                    >
                      {t(chipKey, language)}
                    </Button>
                  ))}
                </motion.div>
              )}

              {/* Messages */}
              <AnimatePresence>
                {currentMessages.map((msg, index) => (
                  <motion.div
                    key={msg.id}
                    initial={{ opacity: 0, y: 8 }}
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
                          backgroundColor: msg.role === "user" ? "#262626" : "#000",
                          flexShrink: 0,
                        }}
                        size={32}
                      />

                      <div
                        style={{
                          padding: "12px 16px",
                          borderRadius: msg.role === "user" ? "16px 16px 4px 16px" : "16px 16px 16px 4px",
                          backgroundColor: msg.role === "user" ? "#000" : "#F7F7F7",
                          border: msg.role === "user" ? "none" : "1px solid #E5E5E5",
                          color: msg.role === "user" ? "#fff" : "#262626",
                        }}
                      >
                        {msg.role === "assistant" && typingMessageId === msg.id ? (
                          <TypewriterMessage
                            content={msg.content}
                            sources={msg.sources}
                            model={msg.model}
                            thinking={msg.thinking}
                            i18nLang={language}
                          />
                        ) : (
                          <MessageContent
                            content={msg.content}
                            sources={msg.sources}
                            model={msg.model}
                            thinking={msg.thinking}
                            i18nLang={language}
                          />
                        )}

                        {msg.attachments && msg.attachments.length > 0 && (
                          <div style={{ marginTop: 8, display: "flex", gap: 6, flexWrap: "wrap" }}>
                            {msg.attachments.map((att, idx) => (
                              <Tag
                                key={idx}
                                icon={getFileIcon(att.content_type)}
                               
                                style={{
                                  borderRadius: 4,
                                  background: msg.role === "user" ? "rgba(255,255,255,0.1)" : "#fff",
                                  border: msg.role === "user" ? "1px solid rgba(255,255,255,0.2)" : "1px solid #E5E5E5",
                                  color: msg.role === "user" ? "#fff" : "#262626",
                                }}
                              >
                                {att.original_name}
                              </Tag>
                            ))}
                          </div>
                        )}

                        {/* Draft Confirmation Panel */}
                        {msg.draftContracts && msg.batchSummary && msg.role === "assistant" && (
                          <DraftConfirmationPanel
                            contracts={msg.draftContracts}
                            summary={msg.batchSummary}
                            language={language}
                            onConfirm={async (selectedContracts) => {
                              try {
                                const payload = selectedContracts.map((c) => ({
                                  contract_number: c.contract_number,
                                  contract_name: c.contract_name || c.contract_number,
                                  lessee_name: c.lessee,
                                  lessor_name: c.lessor,
                                  store_name: c.store_name,
                                  store_address: c.store_address,
                                  currency: c.currency || "CNY",
                                  commencement_date: c.commencement_date,
                                  lease_start_date: c.lease_start_date,
                                  lease_end_date: c.lease_end_date,
                                  discount_rate_type: c.discount_rate_type || null,
                                  discount_rate_value: c.discount_rate || null,
                                  tags: "",
                                }));

                                const result = await contractApi.batchCreate(payload, token!);

                                // Add result message
                                const resultMessage: Message = {
                                  id: generateId(),
                                  role: "assistant",
                                  content: t("ai.batch_create_result", language, { success: String(result.created_count), failed: String(result.failed_count), details: result.failed_count > 0 ? t("ai.batch_create_failed_details", language) + result.failed_contracts.map((f: any) => `- ${f.number}: ${f.error}`).join("\n") : "" }),
                                  timestamp: Date.now(),
                                };
                                updateSessionMessages(activeSessionId!, (msgs) => [...msgs, resultMessage]);
                                message.success(t("ai.batch_create_success", language, { count: String(result.created_count) }));
                              } catch (error: any) {
                                message.error(t("ai.batch_create_failed", language, { error: error.message }));
                              }
                            }}
                            onSkip={() => {
                              const skipMessage: Message = {
                                id: generateId(),
                                role: "assistant",
                                content: t("ai.skip_import", language),
                                timestamp: Date.now(),
                              };
                              updateSessionMessages(activeSessionId!, (msgs) => [...msgs, skipMessage]);
                            }}
                          />
                        )}
                      </div>
                    </Space>
                  </motion.div>
                ))}
              </AnimatePresence>

              {loading && !typingMessageId && (
                <motion.div
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  style={{ display: "flex", alignItems: "center", gap: 8, padding: "8px 0" }}
                >
                  <Avatar
                    icon={<RobotOutlined />}
                    style={{ background: "#000", flexShrink: 0 }}
                    size={32}
                  />
                  <div
                    style={{
                      padding: "12px 16px",
                      borderRadius: "16px 16px 16px 4px",
                      background: "#F7F7F7",
                      border: "1px solid #E5E5E5",
                    }}
                  >
                    <Spin />
                    <span style={{ marginLeft: 8, fontSize: 13, color: "#8C8C8C" }}>
                      {t("ai.thinking", language)}
                    </span>
                  </div>
                </motion.div>
              )}

              <div ref={messagesEndRef} />
            </div>

            {/* Input Area */}
            <div
              style={{
                padding: "16px 20%",
                borderTop: "1px solid #E5E5E5",
                background: "#fff",
                flexShrink: 0,
              }}
            >
              <div
                style={{
                  display: "flex",
                  alignItems: "flex-end",
                  gap: 8,
                  background: "#F7F7F7",
                  borderRadius: 24,
                  padding: "8px 16px",
                  border: "1px solid #E5E5E5",
                  transition: "border-color 0.15s",
                }}
                className="chat-input-wrapper"
              >
                <Upload
                  customRequest={handleFileUpload}
                  showUploadList={false}
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
                      message.error(t("ai.unsupported_file", language));
                      return Upload.LIST_IGNORE;
                    }
                    const isLt50M = file.size / 1024 / 1024 < 50;
                    if (!isLt50M) {
                      message.error(t("ai.file_too_large", language));
                      return Upload.LIST_IGNORE;
                    }
                    return true;
                  }}
                >
                  <Tooltip title={t("ai.upload_file_tooltip", language)}>
                    <Button
                      type="text"
                      icon={<PaperClipOutlined style={{ fontSize: 18, color: "#8C8C8C" }} />}
                      style={{ height: 36, width: 36, padding: 0 }}
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
                    padding: "6px 0",
                  }}
                  onFocus={(e) => {
                    e.currentTarget.parentElement!.parentElement!.style.borderColor = "#000";
                  }}
                  onBlur={(e) => {
                    e.currentTarget.parentElement!.parentElement!.style.borderColor = "#E5E5E5";
                  }}
                />

                <Button
                  type="primary"
                  shape="circle"
                  icon={<SendOutlined />}
                  onClick={() => handleSend()}
                  loading={loading}
                  disabled={!input.trim()}
                  style={{
                    width: 36,
                    height: 36,
                    flexShrink: 0,
                    background: input.trim() ? "#000" : "#D9D9D9",
                    borderColor: input.trim() ? "#000" : "#D9D9D9",
                  }}
                />
              </div>

              <Text
                type="secondary"
                style={{
                  fontSize: 11,
                  textAlign: "center",
                  display: "block",
                  marginTop: 8,
                  color: "#BFBFBF",
                }}
              >
                {t("ai.disclaimer", language)}
              </Text>
            </div>
          </div>
        </div>
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
