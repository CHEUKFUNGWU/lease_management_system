import { describe, expect, it } from "vitest";
import { ApiError } from "./api";
import { t } from "./i18n";

// FIX-002 A3: the discount-rate 422 must render actionable copy that names
// the contracts and says where to fix them — and must not tell the user to
// "try again later" for a request that will never succeed until the data is
// corrected.
describe("ApiError.userMessage discount-rate-missing data_unavailable", () => {
  const payload = {
    code: "data_unavailable",
    error: "discount rate requires policy matching or human confirmation",
    details: { discount_rate_missing: true, contracts: ["CT-LE001", "CT-LE002"] },
  };

  it("names the affected contract numbers", () => {
    const error = new ApiError("data_unavailable", 422, payload);
    expect(error.message).toContain("CT-LE001");
    expect(error.message).toContain("CT-LE002");
  });

  it("says where to fix it and never says retry later", () => {
    const error = new ApiError("data_unavailable", 422, payload);
    expect(error.message).toContain(t("api.discount_rate_missing", "zh-CN", { contracts: "CT-LE001、CT-LE002" }));
    expect(error.message).not.toContain("稍后重试");
    expect(error.message).not.toContain("请重试");
  });

  it("falls back to contract-free copy when the backend list is empty", () => {
    const error = new ApiError("data_unavailable", 422, {
      code: "data_unavailable",
      details: { discount_rate_missing: true, contracts: [] },
    });
    expect(error.message).toBe(t("api.discount_rate_missing_no_contracts", "zh-CN"));
  });

  it("keeps the generic copy for data_unavailable without the discount-rate flag", () => {
    const error = new ApiError("data_unavailable", 422, {
      code: "data_unavailable",
      details: { reason: "coverage_below_threshold" },
    });
    expect(error.message).toBe(t("api.request_failed", "zh-CN"));
  });

  it("works in all three languages", () => {
    for (const lang of ["zh-CN", "zh-HK", "en"] as const) {
      const error = new ApiError("data_unavailable", 422, payload);
      // apiLanguage defaults to zh-CN in tests; verify the dictionary entry
      // itself exists and interpolates in every language.
      expect(t("api.discount_rate_missing", lang, { contracts: "CT-LE001" })).toContain("CT-LE001");
      expect(t("api.discount_rate_missing", lang, { contracts: "CT-LE001" })).not.toBe("");
    }
  });
});

describe("DIAG-001 error classification", () => {
  it("names the fix for unconfigured rent-to-sales policy thresholds", () => {
    const error = new ApiError("data_unavailable", 422, {
      code: "data_unavailable",
      error: "rent-to-sales thresholds are not configured",
      details: { reason: "policy_thresholds_missing", resource: "rent-to-sales" },
    });
    expect(error.message).toBe(t("api.policy_thresholds_missing", "zh-CN"));
    expect(error.message).toContain("租售比");
    expect(error.message).not.toContain("请稍后重试");
  });

  it("names the failing endpoint in the unclassified fallback", () => {
    const error = new ApiError("http_422", 422, { error: "legacy body" }, "/api/v1/reports/rent-to-sales");
    expect(error.message).toContain("/api/v1/reports/rent-to-sales");
    expect(error.message).toContain("请求未成功");
  });

  it("keeps the bare fallback when no endpoint is known", () => {
    const error = new ApiError("http_422", 422, {});
    expect(error.message).toBe(t("api.request_failed", "zh-CN"));
  });
});
