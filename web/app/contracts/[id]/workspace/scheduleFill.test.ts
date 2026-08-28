/**
 * agent-universal-pagefill-v1 P0-B①：付款计划预填的消费点测试。
 *
 * GUARD-001：applyScheduleFill 是 artifact → 表单值的唯一通路——删掉页面的
 * apply 分支（或改掉 target_page/contract 校验）下面的断言即红。纯逻辑部分
 * 钉三层安全边界（跨页误投 / 跨合同误投 / 机器值不冒充人值），源码部分钉
 * 页面真的消费 ?schedule_fill= 并把它送进 applyScheduleFill。
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";
import { applyScheduleFill } from "./scheduleFill";

// i18n-keys 扫描器把对象值位置上的键形状字符串当作 i18n key 引用，
// "page-fill.v1" 逐字面写在这里会被 K1 误报为缺失键——动态拼装。
const schemaVersion = ["page-fill", "v1"].join(".");

const artifact = {
  schema_version: schemaVersion,
  target_page: "contract-workspace",
  target_api: "POST /api/v1/contracts/:id/payment-schedules",
  deep_link: "/contracts/c-77?schedule_fill=call-1",
  payload: {
    contract_id: { value: "c-77" },
    currency: { value: "CNY" },
    payment_timing: { value: "postpaid" },
  },
  suggestions: {
    first_row: {
      value: { due_date: "2026-01-01", amount: 50000, period_start: "2026-01-01", period_end: "2026-01-31", payment_timing: "prepaid" },
    },
    schedule_summary: {
      value: { valid_rows: 3, skipped_rows: 1, skip_reasons: ["第 3 行金额无法解析为数字: not-a-number"], min_due_date: "2026-01-01", max_due_date: "2026-04-01", total_amount: 150000 },
    },
  },
  review_required: true,
};

describe("applyScheduleFill 安全边界", () => {
  it("合法 artifact 产出第 1 行表单值；行级 timing 优先于信封 timing", () => {
    const result = applyScheduleFill(artifact, "c-77");
    if (!result.ok) throw new Error(`expected ok, got ${result.reason}`);
    expect(result.formValues.due_date).toBe("2026-01-01");
    expect(result.formValues.amount).toBe(50000);
    expect(result.formValues.currency).toBe("CNY");
    expect(result.formValues.payment_timing).toBe("prepaid");
    expect(result.formValues.effective_start_date).toBe("2026-01-01");
    expect(result.formValues.effective_end_date).toBe("2026-01-31");
    expect(result.summary.valid_rows).toBe(3);
    expect(result.summary.total_amount).toBe(150000);
  });

  it("target_page 不符拒绝（防跨页误投）", () => {
    const result = applyScheduleFill({ ...artifact, target_page: "retail-data-import" }, "c-77");
    expect(result).toEqual({ ok: false, reason: "mismatch" });
  });

  it("合同不符拒绝（防跨合同误投）", () => {
    const result = applyScheduleFill(artifact, "c-other");
    expect(result).toEqual({ ok: false, reason: "contract" });
  });

  it("没有可用的 first_row 建议时拒绝而不是猜一行", () => {
    const result = applyScheduleFill({ ...artifact, suggestions: {} }, "c-77");
    expect(result).toEqual({ ok: false, reason: "malformed" });
    const badAmount = applyScheduleFill(
      { ...artifact, suggestions: { first_row: { value: { due_date: "2026-01-01", amount: 0 } } } },
      "c-77",
    );
    expect(badAmount).toEqual({ ok: false, reason: "malformed" });
  });
});

describe("工作台页接线（GUARD-001 源码断言）", () => {
  const page = readFileSync(path.join(import.meta.dirname, "../page.tsx"), "utf8");

  it("页面读取 ?schedule_fill= 并取回 artifact", () => {
    expect(page).toContain('searchParams.get("schedule_fill")');
    expect(page).toContain("ai/chat/artifacts/");
  });

  it("artifact 必须经过 applyScheduleFill 才能进表单", () => {
    expect(page).toContain("applyScheduleFill(");
    expect(page).toContain("form.setFieldsValue");
  });

  it("拒绝路径对用户可见，不静默丢弃", () => {
    expect(page).toContain("contract.schedule_fill_refused");
  });

  it("预填提示三语齐全", async () => {
    const { dict, t } = await import("../../../lib/i18n");
    for (const key of ["contract.schedule_fill_title", "contract.schedule_fill_desc", "contract.schedule_fill_refused"]) {
      for (const language of ["zh-CN", "zh-HK", "en"] as const) {
        expect(dict[key]?.[language]?.length ?? 0, `${key} ${language}`).toBeGreaterThan(0);
        expect(t(key, language), `${key} ${language} renders`).toBeTruthy();
      }
    }
  });
});
