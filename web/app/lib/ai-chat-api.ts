import { apiRequest } from "./api-client";
import type {
  AIContinuationRequest,
  AICreateRunRequest,
  AICreateSessionRequest,
  AICreateSessionResponse,
  AIGetSessionResponse,
  AIListSessionsResponse,
  AIReviewActionRequest,
  AIReviewActionResponse,
  AIRunStartResponse,
} from "./types/ai-chat";

export const aiChatApi = {
  createSession: (data: AICreateSessionRequest, token: string) =>
    apiRequest<AICreateSessionResponse>("/api/v1/ai/chat/sessions", {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  listSessions: (token: string, params?: { limit?: number; offset?: number; status?: string }) => {
    const qs = new URLSearchParams();
    if (params?.limit !== undefined) qs.append("limit", String(params.limit));
    if (params?.offset !== undefined) qs.append("offset", String(params.offset));
    if (params?.status) qs.append("status", params.status);
    const queryString = qs.toString();
    return apiRequest<AIListSessionsResponse>(
      `/api/v1/ai/chat/sessions${queryString ? `?${queryString}` : ""}`,
      { token }
    );
  },

  getSession: (sessionId: string, token: string) =>
    apiRequest<AIGetSessionResponse>(`/api/v1/ai/chat/sessions/${sessionId}`, {
      token,
    }),

  createReviewAction: (artifactId: string, data: AIReviewActionRequest, token: string) =>
    apiRequest<AIReviewActionResponse>(`/api/v1/ai/chat/artifacts/${artifactId}/actions`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  createContinuation: (data: AIContinuationRequest, token: string) =>
    apiRequest<AIRunStartResponse>("/api/v1/ai/chat/continuations", {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),

  createRun: (sessionId: string, data: AICreateRunRequest, token: string) =>
    apiRequest<AIRunStartResponse>(`/api/v1/ai/chat/sessions/${sessionId}/runs`, {
      method: "POST",
      body: JSON.stringify(data),
      token,
    }),
};
