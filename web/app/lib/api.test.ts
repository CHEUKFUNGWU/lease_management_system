/**
 * ERR-002 错误契约映射测试。
 *
 * C4：每个 code 到文案的映射有断言。分支词汇与 ERR-001 一致（13 个
 * errcontract code），scope_denied 必须与 permission_denied 文案不同，
 * 且与「暂无数据」完全不同。断言的是 zh-CN 文案（apiLanguage 默认值），
 * 三种语言的齐全性由 i18n 词典本身保证（dict 类型强制三语）。
 */
import { describe, expect, it } from "vitest";
import { ApiError, apiErrorMessage } from "./api";
import { t } from "./i18n";

describe("ApiError userMessage code mapping (ERR-001 vocabulary)", () => {
  it("maps unauthenticated to the session-expired copy", () => {
    expect(ApiError.userMessage("unauthenticated", 401)).toBe(t("api.session_expired", "zh-CN"));
  });

  it("maps permission_denied to the forbidden copy", () => {
    expect(ApiError.userMessage("permission_denied", 403)).toBe(t("api.forbidden", "zh-CN"));
  });

  it("gives scope_denied its own copy, distinct from permission_denied and from no-data", () => {
    const scopeCopy = ApiError.userMessage("scope_denied", 422);
    expect(scopeCopy).toBe(t("api.scope_denied", "zh-CN"));
    expect(scopeCopy).not.toBe(ApiError.userMessage("permission_denied", 403));
    // The copy must name the data scope, not claim the data is missing.
    expect(scopeCopy).toContain("数据范围");
    expect(scopeCopy).not.toContain("暂无数据");
  });

  it("maps not_found, network_error and system_failure to their copies", () => {
    expect(ApiError.userMessage("not_found", 404)).toBe(t("api.not_found", "zh-CN"));
    expect(ApiError.userMessage("network_error", 0)).toBe(t("api.network_error", "zh-CN"));
    expect(ApiError.userMessage("system_failure", 500)).toBe(t("api.server_unavailable", "zh-CN"));
  });

  it("maps invalid_arguments, business_failure, data_unavailable, review_required, capability_denied, timeout and cancelled to the generic copy", () => {
    const generic = t("api.request_failed", "zh-CN");
    for (const code of [
      "invalid_arguments",
      "business_failure",
      "data_unavailable",
      "review_required",
      "capability_denied",
      "timeout",
      "cancelled",
    ]) {
      expect(ApiError.userMessage(code, 422), code).toBe(generic);
    }
  });

  it("maps a source_conflict detail reason to the specific copy, other conflicts stay generic", () => {
    const detail = { code: "conflict", error: "retail KPI source conflict", details: { reason: "source_conflict" } };
    expect(ApiError.userMessage("conflict", 409, detail)).toBe(t("api.source_conflict", "zh-CN"));
    expect(ApiError.userMessage("conflict", 409)).toBe(t("api.request_failed", "zh-CN"));
  });

  it("keeps the status-code fallback for legacy endpoints that send no code", () => {
    expect(ApiError.userMessage("http_401", 401)).toBe(t("api.session_expired", "zh-CN"));
    expect(ApiError.userMessage("http_403", 403)).toBe(t("api.forbidden", "zh-CN"));
    expect(ApiError.userMessage("http_404", 404)).toBe(t("api.not_found", "zh-CN"));
    expect(ApiError.userMessage("http_500", 500)).toBe(t("api.server_unavailable", "zh-CN"));
    expect(ApiError.userMessage("http_422", 422)).toBe(t("api.request_failed", "zh-CN"));
  });
});

describe("apiErrorMessage shared page helper", () => {
  it("maps an ApiError through the same code vocabulary", () => {
    expect(apiErrorMessage(new ApiError("scope_denied", 422))).toBe(t("api.scope_denied", "zh-CN"));
  });

  it("falls back to the error message for plain Error instances", () => {
    expect(apiErrorMessage(new Error("plain failure"))).toBe("plain failure");
  });

  it("falls back to the generic copy for unknown values", () => {
    expect(apiErrorMessage(undefined)).toBe(t("api.request_failed", "zh-CN"));
  });
});
