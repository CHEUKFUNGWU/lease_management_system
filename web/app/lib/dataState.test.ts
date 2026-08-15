import { describe, expect, it } from "vitest";
import { ApiError } from "./api";
import { classifyDataState, isRequestFailure } from "./dataState";

const ok = (data: unknown, error: unknown = null) => classifyDataState({ data, error });

describe("STATE-001 classifyDataState matrix", () => {
  it("成功且数据为空 → empty", () => {
    expect(ok([]).kind).toBe("empty");
    expect(ok(null).kind).toBe("empty");
    expect(ok(undefined).kind).toBe("empty");
  });

  it("成功且有数据 → ready", () => {
    const state = ok([{ a: 1 }]);
    expect(state.kind).toBe("ready");
    expect(state.data).toEqual([{ a: 1 }]);
  });

  it("404 → empty（后端说没有），不是 failed", () => {
    const state = ok(null, new ApiError("not_found", 404, { code: "not_found", error: "x" }));
    expect(state.kind).toBe("empty");
    expect(state.reason).toBeTruthy();
  });

  it("422 data_unavailable → empty（数据条件不满足），经 actionFor 升级为 actionable", () => {
    const error = new ApiError("data_unavailable", 422, {
      code: "data_unavailable",
      details: { discount_rate_missing: true, contracts: ["CT-LE001"] },
    });
    const bare = ok(null, error);
    expect(bare.kind).toBe("empty");
    const upgraded = classifyDataState({
      data: null,
      error,
      actionFor: () => ({ message: "去补折现率", actionLabel: "补录" }),
    });
    expect(upgraded.kind).toBe("actionable");
    expect(upgraded.actionLabel).toBe("补录");
  });

  it("500 → failed（请求没打通）", () => {
    expect(ok(null, new ApiError("system_failure", 500, {})).kind).toBe("failed");
  });

  it("网络失败 → failed", () => {
    expect(ok(null, new ApiError("network_error", 0, {})).kind).toBe("failed");
    expect(isRequestFailure(new ApiError("network_error", 0, {}))).toBe(true);
  });

  it("其余 4xx 无 actionFor → failed", () => {
    expect(ok(null, new ApiError("invalid_arguments", 400, {})).kind).toBe("failed");
  });

  it("scope_denied 独立第四态，永不并入 empty/actionable/failed", () => {
    const error = new ApiError("scope_denied", 403, { code: "scope_denied" });
    const state = ok(null, error);
    expect(state.kind).toBe("scope_denied");
    expect(state.message).toContain("数据范围");
    const upgraded = classifyDataState({ data: null, error, actionFor: () => ({ message: "x", actionLabel: "y" }) });
    expect(upgraded.kind).toBe("scope_denied");
  });
});
