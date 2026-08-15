import { afterEach, describe, expect, it, vi } from "vitest";
import { aiChatApi } from "../lib/api";
import { resetHomeBriefCache, runHomeBrief } from "./briefGate";

vi.mock("../lib/api", () => ({
  aiChatApi: { chat: vi.fn() },
}));

const chatMock = vi.mocked(aiChatApi.chat);

const params = {
  token: "token-a",
  language: "zh-CN" as const,
  message: "今日晨检",
  title: "今日经营简报",
  filters: { as_of: "2026-06-07", window_days: "7", data_classification: "simulated", dataset_version: "ds-1" },
};

const briefResponse = { answer: "晨检摘要", confidence: 0.9 };

// HOME-002 B5: the same SPA session must not repeat the auto-run.
describe("runHomeBrief (B5 session memoization)", () => {
  afterEach(() => {
    resetHomeBriefCache();
    chatMock.mockReset();
  });

  it("runs the brief once for the same key, even across re-entries", async () => {
    chatMock.mockResolvedValue(briefResponse);
    const first = await runHomeBrief(params);
    const second = await runHomeBrief(params);
    expect(first).toEqual(briefResponse);
    expect(second).toEqual(briefResponse);
    expect(chatMock).toHaveBeenCalledTimes(1);
  });

  it("deduplicates concurrent in-flight runs", async () => {
    let resolveFirst: (value: unknown) => void;
    chatMock.mockReturnValue(new Promise((resolve) => { resolveFirst = resolve; }));
    const first = runHomeBrief(params);
    const second = runHomeBrief(params);
    expect(chatMock).toHaveBeenCalledTimes(1);
    resolveFirst!(briefResponse);
    await expect(first).resolves.toEqual(briefResponse);
    await expect(second).resolves.toEqual(briefResponse);
  });

  it("calls the retail Agent path with skill and page context", async () => {
    chatMock.mockResolvedValue(briefResponse);
    await runHomeBrief(params);
    expect(chatMock).toHaveBeenCalledWith(
      expect.objectContaining({
        message: params.message,
        language: "zh-CN",
        skill_id: "retail_operations",
        skill_version: "v1",
        page_context: { page: "home", title: params.title, filters: params.filters },
      }),
      "token-a",
    );
  });

  it("CHAT-001: every brief run marks the session as system-initiated", async () => {
    // Three independent SPA visits (refresh / new tab / re-login each reset
    // the module cache) must all carry initiator=system, so the backend can
    // keep the auto-run out of the user-visible session list while still
    // recording the run and its audit trail.
    chatMock.mockResolvedValue(briefResponse);
    for (let i = 0; i < 3; i++) {
      resetHomeBriefCache();
      await runHomeBrief(params);
    }
    expect(chatMock).toHaveBeenCalledTimes(3);
    for (const call of chatMock.mock.calls) {
      expect(call[0]).toMatchObject({ initiator: "system" });
    }
  });

  it("does not cache a failure, so the retry button can re-run", async () => {
    chatMock.mockRejectedValueOnce(new Error("network"));
    await expect(runHomeBrief(params)).rejects.toThrow("network");
    chatMock.mockResolvedValue(briefResponse);
    await expect(runHomeBrief(params)).resolves.toEqual(briefResponse);
    expect(chatMock).toHaveBeenCalledTimes(2);
  });

  it("uses a different key when the filters change", async () => {
    chatMock.mockResolvedValue(briefResponse);
    await runHomeBrief(params);
    await runHomeBrief({ ...params, filters: { ...params.filters, as_of: "2026-06-08" } });
    expect(chatMock).toHaveBeenCalledTimes(2);
  });
});
