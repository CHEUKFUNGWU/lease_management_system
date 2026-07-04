"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { t, type Language } from "../../lib/i18n";
import {
  createLocalSession,
  createWelcomeMessage,
  generateRuntimeId,
  getSessionTitle,
} from "./factory";
import {
  appendReviewAction,
  applyRuntimeEvent,
  buildHistory,
  chooseActiveSessionId,
  enrichMessages,
  mapReviewAction,
  mapServerMessage,
  mergeServerSessions,
  toTimestamp,
} from "./state";
import { createBrowserRuntimeStorage, type RuntimeStorageAdapter } from "./storage";
import { createHTTPRuntimeTransport, type RuntimeTransport } from "./transport";
import type {
  ChatSession,
  Message,
  PageContext,
  RunRequest,
  RuntimeReviewAction,
  RuntimeTarget,
  UploadedFile,
} from "./types";

interface UseAIChatRuntimeOptions {
  token?: string | null;
  language: Language;
  selectedModel: string;
  transport?: RuntimeTransport;
  storage?: RuntimeStorageAdapter;
}

interface ContinuationOptions {
  instruction?: string;
  contractId?: string;
  pageContext?: PageContext;
}

export function useAIChatRuntime({
  token,
  language,
  selectedModel,
  transport: suppliedTransport,
  storage: suppliedStorage,
}: UseAIChatRuntimeOptions) {
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [typingMessageId, setTypingMessageId] = useState<string | null>(null);
  const [localHydrated, setLocalHydrated] = useState(false);
  const [serverHydrated, setServerHydrated] = useState(false);
  const typingTimers = useRef<Set<ReturnType<typeof setTimeout>>>(new Set());
  const initialPreferences = useRef({ language, selectedModel });
  const transport = useMemo(
    () => suppliedTransport || (token ? createHTTPRuntimeTransport(token) : null),
    [suppliedTransport, token],
  );

  const activeSession = useMemo(
    () => sessions.find((session) => session.id === activeSessionId),
    [activeSessionId, sessions],
  );

  const createNewSession = useCallback(() => {
    const session = createLocalSession(language, selectedModel);
    setSessions((current) => [session, ...current]);
    setActiveSessionId(session.id);
    return session;
  }, [language, selectedModel]);

  const deleteSession = useCallback(
    (sessionId: string) => {
      setSessions((current) => {
        const remaining = current.filter((session) => session.id !== sessionId);
        const next = remaining.length > 0 ? remaining : [createLocalSession(language, selectedModel)];
        setActiveSessionId((active) =>
          active === sessionId ? next[0].id : chooseActiveSessionId(active, next),
        );
        return next;
      });
    },
    [language, selectedModel],
  );

  const updateSessionMessages = useCallback(
    (sessionId: string, update: (messages: Message[]) => Message[]) => {
      setSessions((current) =>
        current.map((session) => {
          if (session.id !== sessionId) return session;
          const messages = update(session.messages);
          return {
            ...session,
            messages,
            title: getSessionTitle(messages, language),
            updatedAt: Date.now(),
          };
        }),
      );
    },
    [language],
  );

  const updateSession = useCallback((sessionId: string, patch: Partial<ChatSession>) => {
    setSessions((current) =>
      current.map((session) =>
        session.id === sessionId
          ? { ...session, ...patch, updatedAt: patch.updatedAt ?? Date.now() }
          : session,
      ),
    );
  }, []);

  const setPendingUpload = useCallback(
    (sessionId: string, uploadedFile: UploadedFile | null) => {
      updateSession(sessionId, { pendingUpload: uploadedFile || undefined });
    },
    [updateSession],
  );

  const patchMessage = useCallback(
    (sessionId: string, messageId: string, patch: Partial<Message>) => {
      updateSessionMessages(sessionId, (messages) =>
        messages.map((message) =>
          message.id === messageId ? { ...message, ...patch } : message,
        ),
      );
    },
    [updateSessionMessages],
  );

  const appendReviewActionToArtifact = useCallback(
    (sessionId: string, artifactId: string, action: RuntimeReviewAction) => {
      updateSessionMessages(sessionId, (messages) =>
        messages.map((message) => appendReviewAction(message, artifactId, action)),
      );
    },
    [updateSessionMessages],
  );

  useEffect(() => {
    const storage = suppliedStorage || createBrowserRuntimeStorage(window.localStorage);
    const snapshot = storage.load();
    if (snapshot.sessions.length > 0) {
      setSessions(snapshot.sessions);
      setActiveSessionId(chooseActiveSessionId(snapshot.activeSessionId, snapshot.sessions));
    } else {
      const initial = createLocalSession(
        initialPreferences.current.language,
        initialPreferences.current.selectedModel,
      );
      setSessions([initial]);
      setActiveSessionId(initial.id);
    }
    setLocalHydrated(true);
  }, [suppliedStorage]);

  useEffect(() => {
    if (!localHydrated) return;
    const storage = suppliedStorage || createBrowserRuntimeStorage(window.localStorage);
    storage.save({ sessions, activeSessionId });
  }, [activeSessionId, localHydrated, sessions, suppliedStorage]);

  useEffect(() => {
    setServerHydrated(false);
  }, [transport]);

  useEffect(() => {
    if (!transport || !localHydrated || serverHydrated) return;
    let cancelled = false;
    transport
      .listSessions()
      .then((response) => {
        if (cancelled) return;
        const records = Array.isArray(response.sessions) ? response.sessions : [];
        setSessions((current) => {
          const merged = mergeServerSessions(current, records, {
            defaultTitle: t("ai.new_session", language),
            selectedModel,
          });
          setActiveSessionId((active) => chooseActiveSessionId(active, merged));
          return merged;
        });
      })
      .catch(() => undefined)
      .finally(() => {
        if (!cancelled) setServerHydrated(true);
      });
    return () => {
      cancelled = true;
    };
  }, [language, localHydrated, selectedModel, serverHydrated, transport]);

  const hydrateServerSession = useCallback(
    async (sessionId: string, serverSessionId: string) => {
      if (!transport) return;
      const response = await transport.getSession(serverSessionId);
      const messages = Array.isArray(response.messages)
        ? response.messages.map(mapServerMessage)
        : [];
      const hydrated = enrichMessages(
        messages,
        response.artifacts || [],
        response.review_actions || [],
      );
      updateSession(sessionId, {
        title: response.session?.title || t("ai.new_session", language),
        messages: hydrated.length > 0 ? hydrated : [createWelcomeMessage(language)],
        updatedAt: toTimestamp(response.session?.updated_at),
        createdAt: toTimestamp(response.session?.created_at),
      });
    },
    [language, transport, updateSession],
  );

  useEffect(() => {
    if (!activeSessionId || !activeSession?.serverSessionId || !transport) return;
    if (activeSession.messages.length > 0) return;
    hydrateServerSession(activeSessionId, activeSession.serverSessionId).catch(() => undefined);
  }, [activeSession, activeSessionId, hydrateServerSession, transport]);

  useEffect(
    () => () => {
      typingTimers.current.forEach((timer) => clearTimeout(timer));
      typingTimers.current.clear();
    },
    [],
  );

  const consumeRun = useCallback(
    async (sessionId: string, runId: string, assistantMessageId: string) => {
      if (!transport) throw new Error("missing runtime transport");

      let terminal = false;
      await transport.consumeRun(runId, (eventName, data) => {
        if (eventName === "complete") {
          terminal = true;
          setLoading(false);
          updateSession(sessionId, { currentRunId: undefined });
          return;
        }
        if (eventName !== "run_event" || !data?.event) return;
        updateSessionMessages(sessionId, (messages) =>
          messages.map((message) => {
            if (message.id !== assistantMessageId) return message;
            const result = applyRuntimeEvent(message, data.event);
            if (result.terminal) terminal = true;
            if (result.startsTyping) {
              setTypingMessageId(assistantMessageId);
              const timer = setTimeout(() => {
                setTypingMessageId((current) =>
                  current === assistantMessageId ? null : current,
                );
                typingTimers.current.delete(timer);
              }, Math.min(result.message.content.length * 15 + 500, 3000));
              typingTimers.current.add(timer);
            }
            return result.message;
          }),
        );
        if (terminal) {
          setLoading(false);
          updateSession(sessionId, { currentRunId: undefined });
        }
      });

      if (!terminal) {
        setLoading(false);
        updateSession(sessionId, { currentRunId: undefined });
      }
    },
    [transport, updateSession, updateSessionMessages],
  );

  const startRun = useCallback(
    async (
      localSessionId: string,
      serverSessionId: string,
      runResponse: any,
      assistantSeed: Partial<Message> = {},
    ) => {
      const run = runResponse.run;
      if (!run?.id) throw new Error("missing run id");
      const assistantMessageId = generateRuntimeId();
      const assistantMessage: Message = {
        id: assistantMessageId,
        role: "assistant",
        content: "",
        timestamp: Date.now(),
        runId: run.id,
        agentMode: true,
        agentPlan: runResponse.agent_plan,
        toolCalls: runResponse.tool_calls,
        reviewPrompts: runResponse.review_prompts,
        ...assistantSeed,
      };
      updateSessionMessages(localSessionId, (messages) => [...messages, assistantMessage]);
      updateSession(localSessionId, { serverSessionId, currentRunId: run.id });
      setLoading(true);
      try {
        await consumeRun(localSessionId, run.id, assistantMessageId);
      } finally {
        setLoading(false);
      }
      return runResponse;
    },
    [consumeRun, updateSession, updateSessionMessages],
  );

  const createAndStartRun = useCallback(
    async (
      localSessionId: string,
      serverSessionId: string,
      request: RunRequest,
      assistantSeed: Partial<Message> = {},
    ) => {
      if (!transport) throw new Error("missing runtime transport");
      const response = await transport.createRun(serverSessionId, request);
      return startRun(localSessionId, serverSessionId, response, assistantSeed);
    },
    [startRun, transport],
  );

  const continueFromTarget = useCallback(
    async (
      localSessionId: string,
      serverSessionId: string,
      target: RuntimeTarget,
      options?: ContinuationOptions,
      assistantSeed: Partial<Message> = {},
    ) => {
      if (!transport) throw new Error("missing runtime transport");
      const response = await transport.createContinuation(
        {
          target,
          instruction: options?.instruction,
          contract_id: options?.contractId,
          language,
          page_context: options?.pageContext,
        },
      );
      return startRun(localSessionId, serverSessionId, response, assistantSeed);
    },
    [language, startRun, transport],
  );

  const continueActive = useCallback(
    async (target: RuntimeTarget, options?: ContinuationOptions) => {
      if (!activeSessionId || !activeSession?.serverSessionId) {
        throw new Error("missing server session id");
      }
      return continueFromTarget(
        activeSessionId,
        activeSession.serverSessionId,
        target,
        options,
      );
    },
    [activeSession, activeSessionId, continueFromTarget],
  );

  const recordReviewAction = useCallback(
    async (
      sessionId: string,
      artifactId: string,
      actionType: string,
      actionPayload?: Record<string, unknown>,
      comment?: string,
    ) => {
      if (!transport) throw new Error("missing runtime transport");
      const response = await transport.createReviewAction(
        artifactId,
        { action_type: actionType, action_payload: actionPayload, comment },
      );
      const action = mapReviewAction(response.action);
      appendReviewActionToArtifact(sessionId, artifactId, action);
      return action;
    },
    [appendReviewActionToArtifact, transport],
  );

  const ensureServerSession = useCallback(
    async (
      localSessionId: string,
      data: {
        title?: string;
        bound_contract_id?: string;
        context_snapshot?: Record<string, any>;
      },
    ) => {
      const existing = sessions.find((session) => session.id === localSessionId)?.serverSessionId;
      if (existing) return existing;
      if (!transport) throw new Error("missing runtime transport");
      const response = await transport.createSession(data);
      const serverSessionId = response.session?.id;
      if (!serverSessionId) throw new Error("missing server session id");
      updateSession(localSessionId, { serverSessionId });
      return serverSessionId as string;
    },
    [sessions, transport, updateSession],
  );

  return {
    sessions,
    activeSessionId,
    activeSession,
    loading,
    typingMessageId,
    setActiveSessionId,
    setLoading,
    createNewSession,
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
  };
}
