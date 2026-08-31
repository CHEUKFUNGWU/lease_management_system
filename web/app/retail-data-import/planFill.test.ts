/**
 * agent-universal-pagefill-v1 P0-B①：预算计划行预填的消费点测试。
 *
 * GUARD-001：applyPlanFill 是 artifact → 预算导入表单值的唯一通路——删掉
 * 页面的 apply 分支（或改掉 target_page 校验）下面的断言即红。纯逻辑部分钉
 * 安全边界（跨页误投 / 半套信封拒绝），源码部分钉页面真的消费 ?plan_fill=
 * 并把它送进 applyPlanFill。
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";
import { applyPlanFill } from "./planFill";

// i18n-keys 扫描器把对象值位置上的键形状字符串当作 i18n key 引用——
// 动态拼装，见 scheduleFill.test.ts 的同类处理。
const schemaVersion = ["page-fill", "v1"].join(".");

const artifact = {
  schema_version: schemaVersion,
  target_page: "retail-data-import",
  target_api: "POST /api/v1/fpna/plan-versions/import",
  deep_link: "/retail-data-import?plan_fill=call-1&section=plan",
  payload: {
    name: { value: "BUDGET 2026H2" },
    version_type: { value: "budget" },
    as_of_period: { value: "2026-08" },
    from_period: { value: "2026-08" },
    to_period: { value: "2026-12" },
    source: { value: "excel-import" },
  },
  suggestions: {
    plan_summary: {
      value: { valid_rows: 2, skipped_rows: 2, skip_reasons: ["第 3 行 period 必须是 YYYY-MM: 2026-13"], min_period: "2026-08", max_period: "2026-09", store_count: 2, total_revenue: 240000 },
    },
    first_row: {
      value: { store_code: "s-001", period: "2026-08", revenue: 120000 },
    },
  },
  review_required: true,
};

describe("applyPlanFill 安全边界", () => {
  it("合法 artifact 产出成套的信封表单值与行级摘要", () => {
    const result = applyPlanFill(artifact);
    if (!result.ok) throw new Error(`expected ok, got ${result.reason}`);
    expect(result.formValues.name).toBe("BUDGET 2026H2");
    expect(result.formValues.version_type).toBe("budget");
    expect(result.formValues.as_of_period).toBe("2026-08");
    expect(result.formValues.from_period).toBe("2026-08");
    expect(result.formValues.to_period).toBe("2026-12");
    expect(result.formValues.is_official).toBeUndefined();
    expect(result.summary.valid_rows).toBe(2);
    expect(result.summary.total_revenue).toBe(240000);
  });

  it("显式 is_official 才进表单值", () => {
    const official = applyPlanFill({
      ...artifact,
      payload: { ...artifact.payload, is_official: { value: "true" } },
    });
    if (!official.ok) throw new Error("expected ok");
    expect(official.formValues.is_official).toBe(true);
  });

  it("target_page 不符拒绝（防跨页误投）", () => {
    const result = applyPlanFill({ ...artifact, target_page: "contract-workspace" });
    expect(result).toEqual({ ok: false, reason: "mismatch" });
  });

  it("半套信封拒绝：类型/覆盖期间缺一角都不进表单", () => {
    const withoutType = applyPlanFill({ ...artifact, payload: { ...artifact.payload, version_type: { value: "wishful" } } });
    expect(withoutType).toEqual({ ok: false, reason: "malformed" });
    const withoutTo = applyPlanFill({ ...artifact, payload: { ...artifact.payload, to_period: { value: "2026-13" } } });
    expect(withoutTo).toEqual({ ok: false, reason: "malformed" });
  });
});

describe("导入页接线（GUARD-001 源码断言）", () => {
  const page = readFileSync(path.join(import.meta.dirname, "page.tsx"), "utf8");

  it("页面读取 ?plan_fill= 并经 usePageFill 消费", () => {
    expect(page).toContain('searchParams.get("plan_fill")');
    // P0-C 起 artifact 取数在共享 hook 内；页面不再出现手写 artifact fetch。
    expect(page).toContain("const planFill = usePageFill({");
    const fetchSites = page.match(/ai\/chat\/artifacts\//g) ?? [];
    expect(fetchSites.length).toBe(0);
  });

  it("artifact 必须经过 applyPlanFill 才能进表单", () => {
    expect(page).toContain("applyPlanFill(");
    expect(page).toContain("setPlanName(");
  });

  it("拒绝路径对用户可见，不静默丢弃", () => {
    expect(page).toContain("plan_fill.refused");
  });

  it("预填提示三语齐全", async () => {
    const { dict, t } = await import("../lib/i18n");
    for (const key of ["plan_fill.title", "plan_fill.desc", "plan_fill.refused"]) {
      for (const language of ["zh-CN", "zh-HK", "en"] as const) {
        expect(dict[key]?.[language]?.length ?? 0, `${key} ${language}`).toBeGreaterThan(0);
        expect(t(key, language), `${key} ${language} renders`).toBeTruthy();
      }
    }
  });
});
