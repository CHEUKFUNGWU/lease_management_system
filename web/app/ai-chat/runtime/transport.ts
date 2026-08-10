import { API_BASE_URL, aiChatApi } from "../../lib/api";
import { consumeSSEStream } from "./stream";
import type { PageContext, RunRequest, RuntimeTarget } from "./types";

export interface RuntimeTransport {
  listSessions(): Promise<any>;
  getSession(sessionId: string): Promise<any>;
  createSession(data: {
    title?: string;
    bound_contract_id?: string;
    context_snapshot?: Record<string, any>;
  }): Promise<any>;
  createRun(sessionId: string, request: RunRequest): Promise<any>;
  createContinuation(data: {
    target: RuntimeTarget;
    instruction?: string;
    contract_id?: string;
    language?: string;
    page_context?: PageContext;
  }): Promise<any>;
  createReviewAction(
    artifactId: string,
    data: {
      action_type: string;
      action_payload?: Record<string, unknown>;
      comment?: string;
    },
  ): Promise<any>;
  getDraftBatch(batchId: string): Promise<any>;
  retryDraftBatch(batchId: string, data: {
    artifact_id: string;
    action_payload?: Record<string, unknown>;
    comment?: string;
  }): Promise<any>;
  getRunTrace(runId: string): Promise<any>;
  cancelRun(runId: string): Promise<any>;
  steerRun(runId: string, instruction: string): Promise<any>;
  followUpRun(runId: string, instruction: string): Promise<any>;
  branchRun(runId: string, message: string): Promise<any>;
  consumeRun(runId: string, onFrame: (event: string, data: any) => void): Promise<void>;
}

export function createHTTPRuntimeTransport(token: string): RuntimeTransport {
  return {
    listSessions: () => aiChatApi.listSessions(token, { limit: 50 }),
    getSession: (sessionId) => aiChatApi.getSession(sessionId, token),
    createSession: (data) => aiChatApi.createSession(data, token),
    createRun: (sessionId, request) => aiChatApi.createRun(sessionId, request, token),
    createContinuation: (data) => aiChatApi.createContinuation(data, token),
    createReviewAction: (artifactId, data) =>
      aiChatApi.createReviewAction(artifactId, data, token),
    getDraftBatch: (batchId) => aiChatApi.getDraftBatch(batchId, token),
    retryDraftBatch: (batchId, data) => aiChatApi.retryDraftBatch(batchId, data, token),
    getRunTrace: (runId) => aiChatApi.getRunTrace(runId, token),
    cancelRun: (runId) => aiChatApi.cancelRun(runId, token),
    steerRun: (runId, instruction) => aiChatApi.steerRun(runId, instruction, token),
    followUpRun: (runId, instruction) => aiChatApi.followUpRun(runId, instruction, token),
    branchRun: (runId, message) => aiChatApi.branchRun(runId, message, token),
    async consumeRun(runId, onFrame) {
      const response = await fetch(`${API_BASE_URL}/api/v1/ai/chat/runs/${runId}/stream`, {
        headers: { Accept: "text/event-stream", Authorization: `Bearer ${token}` },
      });
      if (!response.ok || !response.body) throw new Error(`HTTP ${response.status}`);
      await consumeSSEStream(response.body, onFrame);
    },
  };
}
