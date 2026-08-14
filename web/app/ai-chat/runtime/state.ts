import type {
  AgentReviewPrompt,
  AgentToolCall,
  ChatSession,
  ContractDraftItem,
  Message,
  PaymentScheduleDraftItem,
  RuntimeEvent,
  RuntimeArtifact,
  RuntimeReviewAction,
  RuntimeSource,
  UploadedFile,
} from "./types";

export function parseRuntimeField<T>(value: unknown): T | undefined {
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

export function toTimestamp(value: unknown, now = Date.now): number {
  if (!value) return now();
  const timestamp = new Date(String(value)).getTime();
  return Number.isFinite(timestamp) ? timestamp : now();
}

export function mapServerMessage(message: any): Message {
  const runtimeSources = parseRuntimeField<any[]>(message.sources) || [];
  return {
    id: message.id,
    role: message.role === "assistant" ? "assistant" : "user",
    content: message.content || "",
    timestamp: toTimestamp(message.created_at),
    runId: message.run_id || undefined,
    sources: runtimeSources
      .map((source) => typeof source === "string" ? source : ({
        type: source?.type, id: source?.id, title: source?.title, snippet: source?.snippet,
        url: source?.url, classification: source?.classification, dataset_version: source?.dataset_version,
        as_of: source?.as_of, formula_version: source?.formula_version,
      } satisfies RuntimeSource))
      .filter(Boolean),
    attachments: parseRuntimeField<UploadedFile[]>(message.attachments),
    model: message.model || undefined,
    confidence: typeof message.confidence === "number" ? message.confidence : undefined,
    confidenceReason: typeof message.confidence_reason === "string" ? message.confidence_reason : undefined,
  };
}

export function mapReviewAction(action: any): RuntimeReviewAction {
  return {
    id: action.id,
    actionType: action.action_type || "",
    actedAt: toTimestamp(action.acted_at),
    artifactId: action.artifact_id || undefined,
    runId: action.run_id || undefined,
    comment: action.comment || undefined,
    payload: parseRuntimeField<Record<string, unknown>>(action.action_payload),
  };
}

export function enrichMessages(
  messages: Message[],
  artifacts: any[],
  reviewActions: any[],
): Message[] {
  const artifactByRunId = new Map<string, any[]>();
  for (const artifact of artifacts || []) {
    if (!artifact?.run_id) continue;
    artifactByRunId.set(artifact.run_id, [
      ...(artifactByRunId.get(artifact.run_id) || []),
      artifact,
    ]);
  }

  const actionsByArtifactId = new Map<string, RuntimeReviewAction[]>();
  for (const action of reviewActions || []) {
    const mapped = mapReviewAction(action);
    if (!mapped.artifactId) continue;
    actionsByArtifactId.set(mapped.artifactId, [
      ...(actionsByArtifactId.get(mapped.artifactId) || []),
      mapped,
    ]);
  }

  return messages.map((message) => {
    if (!message.runId) return message;
    let next = message;
    for (const artifact of artifactByRunId.get(message.runId) || []) {
      const data = parseRuntimeField<any>(artifact.data) || {};
      const event: RuntimeEvent = {
        event_type: "artifact_ready",
        payload: {
          artifact_id: artifact.id,
          artifact_type: artifact.artifact_type,
          title: artifact.title,
          status: artifact.status,
          schema_version: artifact.schema_version,
          data,
          evidence_refs: parseRuntimeField<any[]>(artifact.evidence_refs) || [],
          evidence_complete: artifact.evidence_complete,
          review_required: artifact.review_required,
          review_reasons: parseRuntimeField<string[]>(artifact.review_reasons) || [],
          model_version: artifact.model_version,
          rule_version: artifact.rule_version,
        },
      };
      next = applyRuntimeEvent(next, event).message;
      const actions = actionsByArtifactId.get(artifact.id) || [];
      for (const action of actions) next = appendReviewAction(next, artifact.id, action);
    }
    return next;
  });
}

export interface RuntimeEventResult {
  message: Message;
  terminal: boolean;
  startsTyping: boolean;
}

export function applyRuntimeEvent(message: Message, event: RuntimeEvent): RuntimeEventResult {
  const payload: any = event.payload || {};
  let patch: Partial<Message> = {};
  let terminal = false;
  let startsTyping = false;

  switch (event.event_type) {
    case "message_end":
      patch = {
        content: payload.content || "",
        model: payload.model,
        confidence: typeof payload.confidence === "number" ? payload.confidence : undefined,
        confidenceReason: typeof payload.confidence_reason === "string" ? payload.confidence_reason : undefined,
        sources: Array.isArray(payload.sources)
          ? payload.sources
              .map((source: any) => typeof source === "string" ? source : ({
                type: source?.type, id: source?.id, title: source?.title, snippet: source?.snippet,
                url: source?.url, classification: source?.classification, dataset_version: source?.dataset_version,
                as_of: source?.as_of, formula_version: source?.formula_version,
              } satisfies RuntimeSource))
              .filter((source: any) => typeof source === "string" || source.title || source.id || source.type)
          : undefined,
      };
      startsTyping = true;
      break;
    case "tool_end":
      patch = { toolCalls: Array.isArray(payload) ? (payload as AgentToolCall[]) : undefined };
      break;
    case "review_prompt":
      patch = {
        reviewPrompts: Array.isArray(payload) ? (payload as AgentReviewPrompt[]) : undefined,
      };
      break;
    case "artifact_ready":
      {
        const artifact: RuntimeArtifact = {
          id: payload.artifact_id,
          artifact_type: payload.artifact_type || "generic",
          title: payload.title,
          status: payload.status,
          schema_version: payload.schema_version,
          data: payload.data || {},
          evidence_refs: payload.evidence_refs || [],
          evidence_complete: payload.evidence_complete,
          review_required: payload.review_required,
          review_reasons: payload.review_reasons || [],
          model_version: payload.model_version,
          rule_version: payload.rule_version,
        };
        const artifacts = (message.artifacts || []).filter((item) => item.id !== artifact.id);
        artifacts.push(artifact);
        patch = { artifacts };
      }
      if (payload.artifact_type === "contract_draft") {
        patch = {
          ...patch,
          draftContracts: payload.data?.contracts as ContractDraftItem[] | undefined,
          batchSummary: payload.data?.summary,
          contractDraftArtifactId: payload.artifact_id,
        };
      } else if (payload.artifact_type === "payment_schedule_draft") {
        patch = {
          ...patch,
          draftPaymentSchedules: payload.data?.schedules as PaymentScheduleDraftItem[] | undefined,
          paymentScheduleSummary: payload.data?.summary,
          paymentScheduleArtifactId: payload.artifact_id,
        };
      }
      break;
    case "run_error":
      patch = { content: payload.error || "Unknown runtime error", model: payload.model };
      terminal = true;
      break;
    case "run_end":
      terminal = true;
      break;
    default:
      break;
  }

  return { message: { ...message, ...patch }, terminal, startsTyping };
}

export function appendReviewAction(
  message: Message,
  artifactId: string,
  action: RuntimeReviewAction,
): Message {
  const matches = (message.artifacts || []).some((artifact) => artifact.id === artifactId) ||
    message.contractDraftArtifactId === artifactId ||
    message.paymentScheduleArtifactId === artifactId;
  if (!matches) return message;
  const existing = message.reviewActions || [];
  if (existing.some((item) => item.id === action.id)) return message;
  return {
    ...message,
    reviewActions: [...existing, action].sort((a, b) => a.actedAt - b.actedAt),
  };
}

export function patchSessionMessage(
  sessions: ChatSession[],
  sessionId: string,
  messageId: string,
  update: (message: Message) => Message,
  titleFor: (messages: Message[]) => string,
  now = Date.now,
): ChatSession[] {
  return sessions.map((session) => {
    if (session.id !== sessionId) return session;
    const messages = session.messages.map((message) =>
      message.id === messageId ? update(message) : message,
    );
    return { ...session, messages, title: titleFor(messages), updatedAt: now() };
  });
}

export function buildHistory(messages: Message[]) {
  return messages
    .filter((message) => message.id !== "welcome")
    .slice(-10)
    .map((message) => ({ role: message.role, content: message.content }));
}

interface ServerSessionRecord {
  id: string;
  title?: string;
  created_at?: string;
  updated_at?: string;
}

export function mergeServerSessions(
  localSessions: ChatSession[],
  serverSessions: ServerSessionRecord[],
  options: { defaultTitle: string; selectedModel: string; now?: () => number },
): ChatSession[] {
  const runtimeBacked = new Map(
    localSessions
      .filter((session) => session.serverSessionId)
      .map((session) => [session.serverSessionId!, session]),
  );
  const localOnly = localSessions.filter((session) => !session.serverSessionId);
  const shouldDropPlaceholder =
    localOnly.length === 1 &&
    localOnly[0].title === options.defaultTitle &&
    localOnly[0].messages.length === 1 &&
    localOnly[0].messages[0].id === "welcome";
  const preservedLocal = shouldDropPlaceholder ? [] : localOnly;

  const merged = serverSessions.map((serverSession) => {
    const existing = runtimeBacked.get(serverSession.id);
    return {
      id: existing?.id || `server:${serverSession.id}`,
      title: serverSession.title || existing?.title || options.defaultTitle,
      messages: existing?.messages || [],
      createdAt:
        existing?.createdAt || toTimestamp(serverSession.created_at, options.now),
      updatedAt: toTimestamp(serverSession.updated_at, options.now),
      model: existing?.model || options.selectedModel,
      pendingUpload: existing?.pendingUpload,
      serverSessionId: serverSession.id,
      currentRunId: existing?.currentRunId,
    } satisfies ChatSession;
  });

  return [...preservedLocal, ...merged].sort((a, b) => b.updatedAt - a.updatedAt);
}

export function chooseActiveSessionId(
  current: string | null,
  sessions: ChatSession[],
): string | null {
  if (current && sessions.some((session) => session.id === current)) return current;
  return sessions[0]?.id || null;
}
