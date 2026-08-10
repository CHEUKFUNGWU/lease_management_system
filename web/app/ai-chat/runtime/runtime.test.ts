import { describe, expect, it } from "vitest";

import { applyRuntimeEvent, enrichMessages, mergeServerSessions } from "./state";
import { createBrowserRuntimeStorage } from "./storage";
import { createSSEParser } from "./stream";
import type { ChatSession, Message } from "./types";

describe("AI chat runtime", () => {
  it("projects a contract artifact onto the assistant message", () => {
    const message: Message = {
      id: "message-1",
      role: "assistant",
      content: "",
      timestamp: 1,
      runId: "run-1",
    };

    const result = applyRuntimeEvent(message, {
      event_type: "artifact_ready",
      payload: {
        artifact_id: "artifact-1",
        artifact_type: "contract_draft",
        data: {
          contracts: [{ contract_number: "LEASE-001" }],
          summary: { total_count: 1 },
        },
      },
    });

    expect(result.message.contractDraftArtifactId).toBe("artifact-1");
    expect(result.message.artifacts?.[0].schema_version).toBeUndefined();
    expect(result.message.artifacts?.[0].artifact_type).toBe("contract_draft");
    expect(result.message.draftContracts?.[0].contract_number).toBe("LEASE-001");
    expect(result.terminal).toBe(false);
  });

  it("hydrates server sessions without duplicating runtime-backed local sessions", () => {
    const local: ChatSession[] = [
      {
        id: "local-1",
        title: "Local draft",
        messages: [{ id: "welcome", role: "assistant", content: "Hi", timestamp: 1 }],
        createdAt: 1,
        updatedAt: 1,
        model: "model-a",
      },
      {
        id: "local-server-1",
        title: "Existing title",
        messages: [],
        createdAt: 2,
        updatedAt: 2,
        model: "model-a",
        serverSessionId: "server-1",
      },
    ];

    const merged = mergeServerSessions(local, [
      { id: "server-1", title: "Server title", created_at: "2026-01-01", updated_at: "2026-01-02" },
      { id: "server-2", title: "Second", created_at: "2026-01-03", updated_at: "2026-01-04" },
    ], {
      defaultTitle: "New chat",
      selectedModel: "model-b",
      now: () => 10,
    });

    expect(merged.filter((session) => session.serverSessionId === "server-1")).toHaveLength(1);
    expect(merged.find((session) => session.serverSessionId === "server-1")?.id).toBe("local-server-1");
    expect(merged.find((session) => session.serverSessionId === "server-2")?.id).toBe("server:server-2");
    expect(merged.find((session) => session.id === "local-1")?.title).toBe("Local draft");
  });

  it("parses fragmented SSE frames without losing runtime events", () => {
    const received: Array<{ event: string; data: any }> = [];
    const parser = createSSEParser((event, data) => received.push({ event, data }));

    parser.push('event: run_event\ndata: {"event":{"event_type":"message');
    parser.push('_end","payload":{"content":"done"}}}\n\n');
    parser.push('event: complete\ndata: {"status":"completed"}\n\n');
    parser.finish();

    expect(received).toEqual([
      {
        event: "run_event",
        data: { event: { event_type: "message_end", payload: { content: "done" } } },
      },
      { event: "complete", data: { status: "completed" } },
    ]);
  });

  it("parses CRLF SSE delimiters split across transport chunks", () => {
    const received: Array<{ event: string; data: any }> = [];
    const parser = createSSEParser((event, data) => received.push({ event, data }));

    parser.push('event: complete\r\ndata: {"status":"completed"}\r');
    parser.push('\n\r\n');
    parser.finish();

    expect(received).toEqual([
      { event: "complete", data: { status: "completed" } },
    ]);
  });

  it("hydrates artifacts and ordered review actions onto their run message", () => {
    const hydrated = enrichMessages(
      [{ id: "message-1", role: "assistant", content: "", timestamp: 1, runId: "run-1" }],
      [
        {
          id: "artifact-1",
          run_id: "run-1",
          artifact_type: "payment_schedule_draft",
          data: JSON.stringify({ schedules: [{ amount: 1000 }], summary: { total_count: 1 } }),
        },
      ],
      [
        { id: "action-2", artifact_id: "artifact-1", action_type: "import", acted_at: "2026-01-02" },
        { id: "action-1", artifact_id: "artifact-1", action_type: "confirm", acted_at: "2026-01-01" },
      ],
    );

    expect(hydrated[0].paymentScheduleArtifactId).toBe("artifact-1");
    expect(hydrated[0].artifacts?.[0].artifact_type).toBe("payment_schedule_draft");
    expect(hydrated[0].draftPaymentSchedules?.[0].amount).toBe(1000);
    expect(hydrated[0].reviewActions?.map((action) => action.id)).toEqual([
      "action-1",
      "action-2",
    ]);
  });

  it("persists and restores one runtime snapshot through the storage adapter", () => {
    const values = new Map<string, string>();
    const storage = createBrowserRuntimeStorage({
      getItem: (key) => values.get(key) ?? null,
      setItem: (key, value) => values.set(key, value),
      removeItem: (key) => values.delete(key),
    });
    const session: ChatSession = {
      id: "session-1",
      title: "Saved",
      messages: [],
      createdAt: 1,
      updatedAt: 2,
      model: "model",
    };

    storage.save({ sessions: [session], activeSessionId: session.id });

    expect(storage.load()).toEqual({ sessions: [session], activeSessionId: "session-1" });
  });

  it("marks run errors terminal while preserving the runtime error", () => {
    const message: Message = {
      id: "message-error",
      role: "assistant",
      content: "",
      timestamp: 1,
    };

    const result = applyRuntimeEvent(message, {
      event_type: "run_error",
      payload: { error: "stream failed", model: "model-a" },
    });

    expect(result.terminal).toBe(true);
    expect(result.message.content).toBe("stream failed");
    expect(result.message.model).toBe("model-a");
  });
});
