"use client";

import { useState, useRef, useEffect, useMemo, Suspense, useCallback } from "react";
import { useSearchParams } from "next/navigation";
import { Spin, Upload, message, Modal } from "antd";
import {
  FileTextOutlined,
  FilePdfOutlined,
  FileExcelOutlined,
  FileImageOutlined,
  ToolOutlined,
} from "@ant-design/icons";
import { DraftConfirmationPanel } from "./components/DraftConfirmationPanel";
import {
  ChatComposer,
  ConversationLoadingState,
  ConversationMessageList,
} from "./components/ConversationPanels";
import { PaymentScheduleDraftPanel } from "./components/PaymentScheduleDraftPanel";
import { SessionSidebar } from "./components/SessionSidebar";
import { ChatContextStrip, ChatHeader, ConversationStarters } from "./components/ChatChrome";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { API_BASE_URL, aiChatApi, contractApi, paymentScheduleApi } from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t, type Language } from "../lib/i18n";
import type {
  ChatMessage as Message,
  ChatSession,
  ContractDraftItem,
  PaymentScheduleDraftItem,
  RuntimeReviewAction,
  UploadedFile,
} from "../lib/types/ai-chat";

const SESSIONS_KEY = "lease_chat_sessions";
const ACTIVE_SESSION_KEY = "lease_chat_active_session";

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
    key: "excel-ledger",
    labelKey: "ai.skill_excel_ledger",
    promptKey: "ai.skill_excel_ledger_prompt",
    icon: "excel",
  },
  {
    key: "contract-review",
    labelKey: "ai.skill_contract_review",
    promptKey: "ai.skill_contract_review_prompt",
    icon: "pdf",
  },
  {
    key: "payment-schedule",
    labelKey: "ai.skill_payment_schedule",
    promptKey: "ai.skill_payment_schedule_prompt",
    icon: "file",
  },
  {
    key: "audit-pack",
    labelKey: "ai.skill_audit_pack",
    promptKey: "ai.skill_audit_pack_prompt",
    icon: "tool",
  },
];

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

function createWelcomeMessage(language: Language): Message {
  return {
    id: "welcome",
    role: "assistant",
    content: t("ai.welcome", language),
    timestamp: Date.now(),
  };
}

function parseRuntimeField<T>(value: any): T | undefined {
  if (value == null) return undefined;
  if (typeof value === "string") {
    try {
      return JSON.parse(value) as T;
    } catch {
      return undefined;
    }
  }
  return value as T;
}

function toTimestamp(value: any): number {
  if (!value) return Date.now();
  const timestamp = new Date(value).getTime();
  return Number.isFinite(timestamp) ? timestamp : Date.now();
}

function mapServerMessageToUI(message: any): Message {
  const runtimeSources = parseRuntimeField<any[]>(message.sources) || [];
  const runtimeAttachments = parseRuntimeField<UploadedFile[]>(message.attachments);
  return {
    id: message.id,
    role: message.role === "assistant" ? "assistant" : "user",
    content: message.content || "",
    timestamp: toTimestamp(message.created_at),
    runId: message.run_id || undefined,
    sources: runtimeSources.map((source) => source?.title || source?.id || source?.type).filter(Boolean),
    attachments: runtimeAttachments,
    model: message.model || undefined,
  };
}

function mapReviewAction(action: any): RuntimeReviewAction {
  return {
    id: action.id,
    actionType: action.action_type || "",
    actedAt: toTimestamp(action.acted_at),
    artifactId: action.artifact_id || undefined,
    runId: action.run_id || undefined,
    comment: action.comment || undefined,
    payload: parseRuntimeField<Record<string, any>>(action.action_payload),
  };
}

function enrichMessagesWithRuntimeData(messages: Message[], artifacts: any[], reviewActions: any[]): Message[] {
  if ((!Array.isArray(artifacts) || artifacts.length === 0) && (!Array.isArray(reviewActions) || reviewActions.length === 0)) {
    return messages.length > 0 ? messages : [];
  }

  const artifactByRunId = new Map<string, any[]>();
  (artifacts || []).forEach((artifact) => {
    if (!artifact?.run_id) return;
    const current = artifactByRunId.get(artifact.run_id) || [];
    current.push(artifact);
    artifactByRunId.set(artifact.run_id, current);
  });

  const actionsByArtifactId = new Map<string, RuntimeReviewAction[]>();
  (reviewActions || []).forEach((action) => {
    const mapped = mapReviewAction(action);
    if (!mapped.artifactId) return;
    const current = actionsByArtifactId.get(mapped.artifactId) || [];
    current.push(mapped);
    actionsByArtifactId.set(mapped.artifactId, current);
  });

  return messages.map((message) => {
    if (!message.runId) return message;
    const runArtifacts = artifactByRunId.get(message.runId) || [];
    if (runArtifacts.length === 0) return message;

    const nextMessage = { ...message };
    runArtifacts.forEach((artifact) => {
      const artifactData = parseRuntimeField<any>(artifact.data) || {};
      if (artifact.artifact_type === "contract_draft") {
        nextMessage.draftContracts = artifactData.contracts;
        nextMessage.batchSummary = artifactData.summary;
        nextMessage.contractDraftArtifactId = artifact.id;
        nextMessage.reviewActions = [
          ...(nextMessage.reviewActions || []),
          ...(actionsByArtifactId.get(artifact.id) || []),
        ];
      }
      if (artifact.artifact_type === "payment_schedule_draft") {
        nextMessage.draftPaymentSchedules = artifactData.schedules;
        nextMessage.paymentScheduleSummary = artifactData.summary;
        nextMessage.paymentScheduleArtifactId = artifact.id;
        nextMessage.reviewActions = [
          ...(nextMessage.reviewActions || []),
          ...(actionsByArtifactId.get(artifact.id) || []),
        ];
      }
    });
    if (nextMessage.reviewActions) {
      nextMessage.reviewActions = nextMessage.reviewActions
        .slice()
        .sort((a, b) => a.actedAt - b.actedAt);
    }
    return nextMessage;
  });
}

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
  const [selectedModel, setSelectedModel] = useState("deepseek-v4-flash");
  const [typingMessageId, setTypingMessageId] = useState<string | null>(null);
  const [hydratedServerSessions, setHydratedServerSessions] = useState(false);

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

  useEffect(() => {
    if (!token || hydratedServerSessions) return;

    let cancelled = false;
    aiChatApi
      .listSessions(token, { limit: 50 })
      .then((response) => {
        if (cancelled) return;
        const serverSessions = Array.isArray(response.sessions) ? response.sessions : [];
        if (serverSessions.length === 0) {
          setHydratedServerSessions(true);
          return;
        }

        setSessions((prev) => {
          const runtimeBacked = new Map(
            prev
              .filter((session) => session.serverSessionId)
              .map((session) => [session.serverSessionId!, session])
          );
          const localOnly = prev.filter((session) => !session.serverSessionId);
          const shouldDropPlaceholder =
            localOnly.length === 1 &&
            localOnly[0].messages.length === 1 &&
            localOnly[0].messages[0].id === "welcome";
          const preservedLocal = shouldDropPlaceholder ? [] : localOnly;

          const merged = serverSessions.map((serverSession: any) => {
            const existing = runtimeBacked.get(serverSession.id);
            return {
              id: existing?.id || `server:${serverSession.id}`,
              title: serverSession.title || existing?.title || t("ai.new_session", language),
              messages: existing?.messages || [],
              createdAt: existing?.createdAt || toTimestamp(serverSession.created_at),
              updatedAt: toTimestamp(serverSession.updated_at),
              model: existing?.model || selectedModel,
              pendingUpload: existing?.pendingUpload,
              serverSessionId: serverSession.id,
              currentRunId: existing?.currentRunId,
            } satisfies ChatSession;
          });

          return [...preservedLocal, ...merged].sort((a, b) => b.updatedAt - a.updatedAt);
        });

        setActiveSessionId((current) => {
          const mergedIds = new Set([
            ...serverSessions.map((serverSession: any) => `server:${serverSession.id}`),
            ...sessions.filter((session) => session.serverSessionId).map((session) => session.id),
          ]);
          if (current && mergedIds.has(current)) {
            return current;
          }
          return `server:${serverSessions[0].id}`;
        });
        setHydratedServerSessions(true);
      })
      .catch(() => {
        if (!cancelled) {
          setHydratedServerSessions(true);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [hydratedServerSessions, language, selectedModel, sessions, token]);

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
  const activePendingUpload = activeSession?.pendingUpload ?? null;

  const chips = useMemo(() => {
    if (!pageContext) return defaultChips;
    return contextChipMap[pageContext.page!] || defaultChips;
  }, [pageContext]);

  const createNewSession = useCallback(() => {
    const newSession: ChatSession = {
      id: generateId(),
      title: t("ai.new_session", language),
      messages: [createWelcomeMessage(language)],
      createdAt: Date.now(),
      updatedAt: Date.now(),
      model: selectedModel,
      pendingUpload: undefined,
    };
    setSessions((prev) => [newSession, ...prev]);
    setActiveSessionId(newSession.id);
    setInput("");
  }, [language, selectedModel]);

  const deleteSession = useCallback((id: string) => {
    setSessions((prev) => {
      const filtered = prev.filter((s) => s.id !== id);
      if (filtered.length === 0) {
        // Create a new empty session
        const newSession: ChatSession = {
          id: generateId(),
          title: t("ai.new_session", language),
          messages: [createWelcomeMessage(language)],
          createdAt: Date.now(),
          updatedAt: Date.now(),
          model: selectedModel,
          pendingUpload: undefined,
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

  const updateSessionRuntime = useCallback((sessionId: string, updates: Partial<ChatSession>) => {
    setSessions((prev) =>
      prev.map((session) =>
        session.id === sessionId
          ? {
              ...session,
              ...updates,
              updatedAt: updates.updatedAt ?? Date.now(),
            }
          : session
      )
    );
  }, []);

  const setSessionPendingUpload = useCallback((sessionId: string, uploadedFile: UploadedFile | null) => {
    setSessions((prev) =>
      prev.map((s) =>
        s.id === sessionId
          ? {
              ...s,
              pendingUpload: uploadedFile ?? undefined,
              updatedAt: Date.now(),
            }
          : s
      )
    );
  }, []);

  const patchAssistantMessage = useCallback(
    (sessionId: string, messageId: string, patch: Partial<Message>) => {
      updateSessionMessages(sessionId, (messages) =>
        messages.map((message) =>
          message.id === messageId
            ? {
                ...message,
                ...patch,
              }
            : message
        )
      );
    },
    [updateSessionMessages]
  );

  const appendReviewActionToMessage = useCallback(
    (sessionId: string, artifactId: string | undefined, action: any) => {
      if (!artifactId || !action) return;
      const mapped = mapReviewAction(action);
      updateSessionMessages(sessionId, (messages) =>
        messages.map((message) => {
          const matchesArtifact =
            message.contractDraftArtifactId === artifactId ||
            message.paymentScheduleArtifactId === artifactId;
          if (!matchesArtifact) return message;
          const existing = message.reviewActions || [];
          if (existing.some((item) => item.id === mapped.id)) return message;
          return {
            ...message,
            reviewActions: [...existing, mapped].sort((a, b) => a.actedAt - b.actedAt),
          };
        })
      );
    },
    [updateSessionMessages]
  );

  const buildHistoryFromMessages = useCallback((messages: Message[]) => {
    return messages
      .filter((message) => message.id !== "welcome")
      .slice(-10)
      .map((message) => ({ role: message.role, content: message.content }));
  }, []);

  const hydrateServerSession = useCallback(async (sessionId: string, serverSessionId: string) => {
    if (!token) return;
    const response = await aiChatApi.getSession(serverSessionId, token);
    const runtimeMessages = Array.isArray(response.messages) ? response.messages.map(mapServerMessageToUI) : [];
    const hydratedMessages = enrichMessagesWithRuntimeData(
      runtimeMessages,
      response.artifacts || [],
      response.review_actions || []
    );
    updateSessionRuntime(sessionId, {
      title: response.session?.title || t("ai.new_session", language),
      messages: hydratedMessages.length > 0 ? hydratedMessages : [createWelcomeMessage(language)],
      updatedAt: toTimestamp(response.session?.updated_at),
      createdAt: toTimestamp(response.session?.created_at),
    });
  }, [language, token, updateSessionRuntime]);

  useEffect(() => {
    if (!activeSessionId || !activeSession?.serverSessionId || !token) return;
    if (activeSession.messages.length > 0) return;

    hydrateServerSession(activeSessionId, activeSession.serverSessionId).catch(() => {
      // Best-effort hydration. Keep local session shell if backend fetch fails.
    });
  }, [activeSession, activeSessionId, hydrateServerSession, token]);

  const getFileIcon = (type: string) => {
    if (type.includes("pdf")) return <FilePdfOutlined style={{ color: "#EF4444" }} />;
    if (type.includes("excel") || type.includes("sheet"))
      return <FileExcelOutlined style={{ color: "#10B981" }} />;
    return <FileImageOutlined style={{ color: "#666" }} />;
  };

  const getSkillIcon = (icon: string) => {
    if (icon === "excel") return <FileExcelOutlined />;
    if (icon === "pdf") return <FilePdfOutlined />;
    if (icon === "tool") return <ToolOutlined />;
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

      message.success(`${data.original_name} ${t("ai.upload_success", language)}`);
      onSuccess(data, file);
      if (activeSessionId) {
        setSessionPendingUpload(activeSessionId, uploadedFile);
      }
    } catch (err: any) {
      onError(err);
      message.error(`${t("ai.upload_failed", language)}: ${err.message}`);
    }
  };

  const consumeRunStream = useCallback(
    async (sessionId: string, runId: string, assistantMessageId: string) => {
      const response = await fetch(`${API_BASE_URL}/api/v1/ai/chat/runs/${runId}/stream`, {
        headers: {
          Accept: "text/event-stream",
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
        },
      });

      if (!response.ok || !response.body) {
        throw new Error(`HTTP ${response.status}`);
      }

      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let buffer = "";
      let terminalSeen = false;

      const applyRuntimeEvent = (runtimeEvent: any) => {
        const payload = runtimeEvent?.payload || {};
        switch (runtimeEvent?.event_type) {
          case "message_end":
            patchAssistantMessage(sessionId, assistantMessageId, {
              content: payload.content || "",
              model: payload.model,
              sources: Array.isArray(payload.sources)
                ? payload.sources.map((source: any) => source.title || source.id || source.type).filter(Boolean)
                : undefined,
            });
            setTypingMessageId(assistantMessageId);
            setTimeout(() => {
              setTypingMessageId((current) => (current === assistantMessageId ? null : current));
            }, Math.min(String(payload.content || "").length * 15 + 500, 3000));
            break;
          case "tool_end":
            patchAssistantMessage(sessionId, assistantMessageId, {
              toolCalls: Array.isArray(payload) ? payload : undefined,
            });
            break;
          case "review_prompt":
            patchAssistantMessage(sessionId, assistantMessageId, {
              reviewPrompts: Array.isArray(payload) ? payload : undefined,
            });
            break;
          case "artifact_ready":
            if (payload?.artifact_type === "contract_draft") {
              patchAssistantMessage(sessionId, assistantMessageId, {
                draftContracts: payload?.data?.contracts,
                batchSummary: payload?.data?.summary,
                contractDraftArtifactId: payload?.artifact_id,
              });
            }
            if (payload?.artifact_type === "payment_schedule_draft") {
              patchAssistantMessage(sessionId, assistantMessageId, {
                draftPaymentSchedules: payload?.data?.schedules,
                paymentScheduleSummary: payload?.data?.summary,
                paymentScheduleArtifactId: payload?.artifact_id,
              });
            }
            break;
          case "run_error":
            patchAssistantMessage(sessionId, assistantMessageId, {
              content: payload?.error || t("ai.request_failed", language, { error: t("ai.unknown_error", language) }),
              model: payload?.model,
            });
            break;
          case "run_end":
            terminalSeen = true;
            setLoading(false);
            updateSessionRuntime(sessionId, { currentRunId: undefined });
            break;
          default:
            break;
        }
      };

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const chunks = buffer.split("\n\n");
        buffer = chunks.pop() || "";

        for (const chunk of chunks) {
          const lines = chunk.split("\n");
          let eventName = "message";
          const dataLines: string[] = [];

          for (const line of lines) {
            if (line.startsWith("event:")) {
              eventName = line.slice(6).trim();
            } else if (line.startsWith("data:")) {
              dataLines.push(line.slice(5).trim());
            }
          }

          if (dataLines.length === 0) continue;

          try {
            const payload = JSON.parse(dataLines.join("\n"));
            if (eventName === "run_event" && payload?.event) {
              applyRuntimeEvent(payload.event);
            } else if (eventName === "complete") {
              terminalSeen = true;
              setLoading(false);
              updateSessionRuntime(sessionId, { currentRunId: undefined });
            }
          } catch {
            // Ignore malformed SSE chunks.
          }
        }
      }

      if (!terminalSeen) {
        setLoading(false);
        updateSessionRuntime(sessionId, { currentRunId: undefined });
      }
    },
    [language, patchAssistantMessage, token, updateSessionRuntime]
  );

  const startStreamedAssistantRun = useCallback(
    async (
      localSessionId: string,
      serverSessionId: string,
      runResponse: any,
      assistantSeed: Partial<Message>
    ) => {
      const run = runResponse.run;
      const assistantMessageId = generateId();
      const aiMessage: Message = {
        id: assistantMessageId,
        role: "assistant",
        content: "",
        timestamp: Date.now(),
        runId: run?.id,
        agentMode: true,
        agentPlan: runResponse.agent_plan,
        toolCalls: runResponse.tool_calls,
        reviewPrompts: runResponse.review_prompts,
        ...assistantSeed,
      };
      updateSessionMessages(localSessionId, (messages) => [...messages, aiMessage]);
      updateSessionRuntime(localSessionId, {
        serverSessionId,
        currentRunId: run?.id,
      });

      if (!run?.id) {
        throw new Error("missing run id");
      }

      await consumeRunStream(localSessionId, run.id, assistantMessageId);
      return runResponse;
    },
    [consumeRunStream, updateSessionMessages, updateSessionRuntime]
  );

  const createAndStartAssistantRun = useCallback(
    async (
      localSessionId: string,
      serverSessionId: string,
      runRequest: {
        message: string;
        parent_run_id?: string;
        contract_id?: string;
        history?: any[];
        file_id?: string;
        object_name?: string;
        content_type?: string;
        language?: string;
        page_context?: {
          page?: string;
          title?: string;
          contract_id?: string;
          period?: string;
          report_view?: string;
          filters?: Record<string, string>;
          summary?: string;
        };
      },
      assistantSeed: Partial<Message>
    ) => {
      const runResp = await aiChatApi.createRun(serverSessionId, runRequest, token!);
      return startStreamedAssistantRun(localSessionId, serverSessionId, runResp, assistantSeed);
    },
    [startStreamedAssistantRun, token]
  );

  const continueFromRuntimeTarget = useCallback(
    async (
      localSessionId: string,
      serverSessionId: string,
      target: { type: "run" | "message" | "artifact" | "action"; id: string },
      options?: {
        instruction?: string;
        contractId?: string;
        pageContext?: {
          page?: string;
          title?: string;
          contract_id?: string;
          period?: string;
          report_view?: string;
          filters?: Record<string, string>;
          summary?: string;
        };
      },
      assistantSeed: Partial<Message> = {}
    ) => {
      const continuationResp = await aiChatApi.createContinuation(
        {
          target,
          instruction: options?.instruction,
          contract_id: options?.contractId,
          language,
          page_context: options?.pageContext,
        },
        token!
      );
      return startStreamedAssistantRun(localSessionId, serverSessionId, continuationResp, assistantSeed);
    },
    [language, startStreamedAssistantRun, token]
  );

  const triggerRuntimeContinuation = useCallback(
    async (
      target: { type: "run" | "message" | "artifact" | "action"; id: string },
      options?: {
        instruction?: string;
        contractId?: string;
        pageContext?: {
          page?: string;
          title?: string;
          contract_id?: string;
          period?: string;
          report_view?: string;
          filters?: Record<string, string>;
          summary?: string;
        };
      }
    ) => {
      if (!activeSessionId || !activeSession?.serverSessionId) {
        message.warning(t("ai.request_failed", language, { error: t("ai.unknown_error", language) }));
        return;
      }

      setLoading(true);
      try {
        await continueFromRuntimeTarget(
          activeSessionId,
          activeSession.serverSessionId,
          target,
          options
        );
      } catch (error: any) {
        setLoading(false);
        message.error(t("ai.request_failed", language, { error: error.message || t("ai.unknown_error", language) }));
      }
    },
    [activeSession, activeSessionId, continueFromRuntimeTarget, language]
  );

  const handleSend = async (messageOverride?: string, fileOverride?: UploadedFile) => {
    const fileForRequest = fileOverride ?? activePendingUpload;
    const msg = (messageOverride ?? input).trim();
    if ((!msg && !fileForRequest) || !token || !activeSessionId) return;
    const messageText = msg || "请解析这个文件并导入台账：先生成合同草稿卡片，等待我确认后再入库。";

    const userMessage: Message = {
      id: generateId(),
      role: "user",
      content: messageText,
      timestamp: Date.now(),
      attachments: fileForRequest ? [fileForRequest] : undefined,
    };

    // Get history from active session
    const currentSession = sessions.find((s) => s.id === activeSessionId);
    const history = buildHistoryFromMessages(currentSession?.messages || []);

    updateSessionMessages(activeSessionId, (msgs) => [...msgs, userMessage]);
    setInput("");
    setLoading(true);

    try {
      let serverSessionId = currentSession?.serverSessionId;
      if (!serverSessionId) {
        const sessionResp = await aiChatApi.createSession({
          title: currentSession?.title || getSessionTitle([userMessage], language),
          bound_contract_id: searchParams.get("contract_id") || undefined,
          context_snapshot: pageContext
            ? {
                page: pageContext.page,
                title: pageContext.title,
                contract_id: pageContext.contract_id,
                period: pageContext.period,
                report_view: pageContext.report_view,
                summary: pageContext.summary,
              }
            : undefined,
        }, token);
        serverSessionId = sessionResp.session?.id;
        if (serverSessionId) {
          updateSessionRuntime(activeSessionId, { serverSessionId });
        }
      }

      const chatData: any = { message: messageText, history, language };
      if (fileForRequest) {
        chatData.file_id = fileForRequest.file_id;
        chatData.object_name = fileForRequest.object_name;
        chatData.content_type = fileForRequest.content_type;
        setSessionPendingUpload(activeSessionId, null);
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

      if (!serverSessionId) {
        throw new Error("missing server session id");
      }

      await createAndStartAssistantRun(activeSessionId, serverSessionId, chatData, {});
    } catch (error: any) {
      const errorMessage: Message = {
        id: generateId(),
        role: "assistant",
        content: t("ai.request_failed", language, { error: error.message || t("ai.unknown_error", language) }),
        timestamp: Date.now(),
      };
      updateSessionMessages(activeSessionId, (msgs) => [...msgs, errorMessage]);
      updateSessionRuntime(activeSessionId, { currentRunId: undefined });
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
            language={language}
            onSelect={setActiveSessionId}
            onNew={createNewSession}
            onDelete={(id: string) => {
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

          <div style={{ flex: 1, display: "flex", flexDirection: "column", minWidth: 0 }}>
            <ChatHeader
              title={activeSession?.title}
              selectedModel={selectedModel}
              modelOptions={MODEL_OPTIONS}
              language={language}
              onSelectModel={setSelectedModel}
              onNewSession={createNewSession}
            />

            <div
              style={{
                flex: 1,
                overflowY: "auto",
                padding: "20px 20%",
                background: "#fff",
              }}
            >
              <ChatContextStrip pageContext={pageContext} language={language} />

              <ConversationStarters
                visible={currentMessages.length <= 1}
                loading={loading}
                chips={chips}
                skillStarters={agentSkillStarters}
                language={language}
                renderSkillIcon={getSkillIcon}
                onSelectSkillPrompt={setInput}
                onSelectChip={handleChipClick}
              />

              <ConversationMessageList
                messages={currentMessages}
                typingMessageId={typingMessageId}
                loading={loading}
                language={language}
                serverSessionId={activeSession?.serverSessionId}
                renderAttachmentIcon={getFileIcon}
                onContinueFromAction={(msg, action) =>
                  triggerRuntimeContinuation(
                    { type: "action", id: action.id },
                    {
                      contractId:
                        msg.paymentScheduleSummary?.contract_id ||
                        searchParams.get("contract_id") ||
                        undefined,
                    }
                  )
                }
                onContinueFromMessage={(messageId) => triggerRuntimeContinuation({ type: "message", id: messageId })}
                onContinueFromRun={(runId) => triggerRuntimeContinuation({ type: "run", id: runId })}
                onContinueFromArtifact={(artifactId) =>
                  triggerRuntimeContinuation({
                    type: "artifact",
                    id: artifactId,
                  })
                }
                renderDraftPanels={(msg) => (
                  <>
                    {msg.draftContracts && msg.batchSummary && msg.role === "assistant" && (
                      <DraftConfirmationPanel
                        contracts={msg.draftContracts}
                        summary={msg.batchSummary}
                        language={language}
                        onConfirm={async (selectedContracts: ContractDraftItem[]) => {
                              try {
                                if (msg.contractDraftArtifactId) {
                                  const confirmActionResponse = await aiChatApi.createReviewAction(
                                    msg.contractDraftArtifactId,
                                    {
                                      action_type: "confirm",
                                      action_payload: {
                                        selected_count: selectedContracts.length,
                                        contract_numbers: selectedContracts.map((contract: ContractDraftItem) => contract.contract_number),
                                      },
                                    },
                                    token!
                                  );
                                  appendReviewActionToMessage(activeSessionId!, msg.contractDraftArtifactId, confirmActionResponse.action);
                                }
                                const payload = selectedContracts.map((c: ContractDraftItem) => ({
                                  contract_number: c.contract_number,
                                  contract_name: c.contract_name || c.contract_number,
                                  lessee_name: c.lessee,
                                  lessor_name: c.lessor,
                                  store_name: c.store_name,
                                  store_address: c.store_address,
                                  currency: c.currency || "CNY",
                                  asset_type: c.asset_type || "real_estate",
                                  commencement_date: c.commencement_date,
                                  lease_start_date: c.lease_start_date,
                                  lease_end_date: c.lease_end_date,
                                  discount_rate_type: c.discount_rate_type || null,
                                  discount_rate_value: c.discount_rate || null,
                                  lease_scope: c.lease_scope || c.suggested_scope || "in_scope",
                                  exemption_reason: c.exemption_reason || null,
                                  scope_source: c.scope_source || "ai_suggested",
                                  scope_confidence: c.scope_confidence ?? null,
                                  tags: "",
                                }));

                                const result = await contractApi.batchCreate(payload, token!);
                                const failedContracts = result.failed_contracts || [];
                                const createSucceeded = Number(result.failed_count || 0) === 0;
                                let createDraftActionResponse: any = null;
                                if (msg.contractDraftArtifactId) {
                                  createDraftActionResponse = await aiChatApi.createReviewAction(
                                    msg.contractDraftArtifactId,
                                    {
                                      action_type: "create_draft",
                                      action_payload: {
                                        selected_count: selectedContracts.length,
                                        created_count: result.created_count,
                                        failed_count: result.failed_count,
                                        failed_contracts: failedContracts,
                                      },
                                    },
                                    token!
                                  );
                                  appendReviewActionToMessage(activeSessionId!, msg.contractDraftArtifactId, createDraftActionResponse.action);
                                }
                                if (createSucceeded) {
                                  message.success(t("ai.batch_create_success", language, { count: String(result.created_count) }));
                                } else {
                                  message.warning(t("ai.batch_create_result", language, { success: String(result.created_count), failed: String(result.failed_count), details: "" }));
                                }
                                if (activeSession?.serverSessionId && createDraftActionResponse?.action?.id) {
                                  setLoading(true);
                                  try {
                                    await continueFromRuntimeTarget(
                                      activeSessionId!,
                                      activeSession.serverSessionId,
                                      { type: "action", id: createDraftActionResponse.action.id },
                                      { contractId: searchParams.get("contract_id") || undefined }
                                    );
                                  } catch (error: any) {
                                    setLoading(false);
                                    message.error(t("ai.request_failed", language, { error: error.message || t("ai.unknown_error", language) }));
                                  }
                                }
                              } catch (error: any) {
                                message.error(t("ai.batch_create_failed", language, { error: error.message }));
                              }
                        }}
                        onSkip={() => {
                          (async () => {
                            try {
                              let skipActionResponse: any = null;
                              if (msg.contractDraftArtifactId) {
                                skipActionResponse = await aiChatApi.createReviewAction(
                                  msg.contractDraftArtifactId,
                                  {
                                    action_type: "skip",
                                    action_payload: {
                                      reason: "user_skipped_contract_draft_import",
                                    },
                                  },
                                  token!
                                );
                                appendReviewActionToMessage(activeSessionId!, msg.contractDraftArtifactId, skipActionResponse.action);
                              }
                              if (activeSession?.serverSessionId && skipActionResponse?.action?.id) {
                                setLoading(true);
                                try {
                                  await continueFromRuntimeTarget(
                                    activeSessionId!,
                                    activeSession.serverSessionId,
                                    { type: "action", id: skipActionResponse.action.id },
                                    { contractId: searchParams.get("contract_id") || undefined }
                                  );
                                } catch (error: any) {
                                  setLoading(false);
                                  message.error(t("ai.request_failed", language, { error: error.message || t("ai.unknown_error", language) }));
                                }
                              }
                            } catch (error: any) {
                              message.error(t("ai.request_failed", language, { error: error.message || t("ai.unknown_error", language) }));
                            }
                          })();
                        }}
                      />
                    )}

                    {msg.draftPaymentSchedules && msg.paymentScheduleSummary && msg.role === "assistant" && (
                      <PaymentScheduleDraftPanel
                        schedules={msg.draftPaymentSchedules}
                        summary={msg.paymentScheduleSummary}
                        language={language}
                        onConfirm={async (selectedSchedules: PaymentScheduleDraftItem[]) => {
                              const contractId = msg.paymentScheduleSummary?.contract_id;
                              if (!contractId) {
                                message.warning(t("ai.schedule_bind_contract_first", language));
                                return;
                              }
                              try {
                                let importActionResponse: any = null;
                                if (msg.paymentScheduleArtifactId) {
                                  const confirmActionResponse = await aiChatApi.createReviewAction(
                                    msg.paymentScheduleArtifactId,
                                    {
                                      action_type: "confirm",
                                      action_payload: {
                                        selected_count: selectedSchedules.length,
                                        contract_id: contractId,
                                      },
                                    },
                                    token!
                                  );
                                  appendReviewActionToMessage(activeSessionId!, msg.paymentScheduleArtifactId, confirmActionResponse.action);
                                }
                                let successCount = 0;
                                for (const schedule of selectedSchedules) {
                                  await paymentScheduleApi.create(contractId, {
                                    contract_id: contractId,
                                    effective_start_date: schedule.period_start || schedule.due_date,
                                    effective_end_date: schedule.period_end || schedule.due_date,
                                    coverage_start_date: schedule.period_start || schedule.due_date,
                                    coverage_end_date: schedule.period_end || schedule.due_date,
                                    due_date: schedule.due_date,
                                    payment_timing: schedule.payment_timing || "postpaid",
                                    amount: schedule.amount,
                                    currency: schedule.currency || "CNY",
                                    amount_type: schedule.amount_type || "fixed_rent",
                                    is_fixed: schedule.is_fixed,
                                    is_variable: !schedule.is_fixed,
                                    is_lease_component: schedule.is_lease_component,
                                    is_non_lease_component: !schedule.is_lease_component,
                                    included_in_liability_pv: schedule.is_lease_component && schedule.is_fixed,
                                  }, token!);
                                  successCount++;
                                }
                                if (msg.paymentScheduleArtifactId) {
                                  importActionResponse = await aiChatApi.createReviewAction(
                                    msg.paymentScheduleArtifactId,
                                    {
                                      action_type: "import",
                                      action_payload: {
                                        imported_count: successCount,
                                        selected_count: selectedSchedules.length,
                                        contract_id: contractId,
                                      },
                                    },
                                    token!
                                  );
                                  appendReviewActionToMessage(activeSessionId!, msg.paymentScheduleArtifactId, importActionResponse.action);
                                }
                                message.success(t("ai.schedule_import_success", language, { count: String(successCount) }));
                                if (activeSession?.serverSessionId && importActionResponse?.action?.id) {
                                  setLoading(true);
                                  try {
                                    await continueFromRuntimeTarget(
                                      activeSessionId!,
                                      activeSession.serverSessionId,
                                      { type: "action", id: importActionResponse.action.id },
                                      { contractId }
                                    );
                                  } catch (error: any) {
                                    setLoading(false);
                                    message.error(t("ai.request_failed", language, { error: error.message || t("ai.unknown_error", language) }));
                                  }
                                }
                              } catch (error: any) {
                                message.error(t("ai.schedule_import_failed", language, { error: error.message }));
                              }
                        }}
                        onSkip={() => {
                          (async () => {
                            try {
                              let skipActionResponse: any = null;
                              if (msg.paymentScheduleArtifactId) {
                                skipActionResponse = await aiChatApi.createReviewAction(
                                  msg.paymentScheduleArtifactId,
                                  {
                                    action_type: "skip",
                                    action_payload: {
                                      reason: "user_skipped_payment_schedule_import",
                                      contract_id: msg.paymentScheduleSummary?.contract_id,
                                    },
                                  },
                                  token!
                                );
                                appendReviewActionToMessage(activeSessionId!, msg.paymentScheduleArtifactId, skipActionResponse.action);
                              }
                              if (activeSession?.serverSessionId && skipActionResponse?.action?.id) {
                                setLoading(true);
                                try {
                                  await continueFromRuntimeTarget(
                                    activeSessionId!,
                                    activeSession.serverSessionId,
                                    { type: "action", id: skipActionResponse.action.id },
                                    { contractId: msg.paymentScheduleSummary?.contract_id }
                                  );
                                } catch (error: any) {
                                  setLoading(false);
                                  message.error(t("ai.request_failed", language, { error: error.message || t("ai.unknown_error", language) }));
                                }
                              }
                            } catch (error: any) {
                              message.error(t("ai.request_failed", language, { error: error.message || t("ai.unknown_error", language) }));
                            }
                          })();
                        }}
                      />
                    )}
                  </>
                )}
              />

              <ConversationLoadingState visible={loading && !typingMessageId} language={language} />

              <div ref={messagesEndRef} />
            </div>

            <ChatComposer
              input={input}
              loading={loading}
              pendingUpload={activePendingUpload}
              language={language}
              onInputChange={setInput}
              onKeyDown={handleKeyDown}
              onFileUpload={handleFileUpload}
              onBeforeUpload={(file) => {
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
              onSend={() => handleSend()}
              onClearUpload={() => activeSessionId && setSessionPendingUpload(activeSessionId, null)}
            />
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
