/**
 * F3-2（任务指令，2026-08-22 已拍板）：导航按 CONTEXT.md 的领域边界重新
 * 分组 + 导航标签与页面标题对齐。
 *
 * 守卫两件事：
 * 1. 「分析与决策」拆为「经营分析」（零售经营域）与「租赁决策」（租赁/交易域），
 *    成员清单逐项锁定——把两个域重新混进一组即红；
 * 2. nav 标签 = 页面标题（pf.title / cashflow.title / promotion.title / fpna.workbench_title），
 *    漂移即红。CONTEXT.md 开篇明确两个领域不得混为一谈，导航是最先违反它的地方。
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { dict, t } from "../lib/i18n";

const layout = readFileSync(join(import.meta.dirname, "../components/AppLayout.tsx"), "utf8");

const OPERATING_GROUP = [
  "/operating-pulse",
  "/store-360",
  "/store-pnl",
  "/financial-model",
  "/scenario-workbench",
  "/fpna-workbench",
  "/performance",
  "/promotions",
];

const LEASE_GROUP = ["/portfolio", "/pre-deal", "/deal-compare", "/sensitivity", "/cashflow-forecast"];

function groupRegion(groupKey: string): string {
  const start = layout.indexOf(`key: "${groupKey}"`);
  expect(start, `group ${groupKey} exists`).toBeGreaterThan(-1);
  const nextGroup = layout.indexOf('groups.push({', start + 1);
  return layout.slice(start, nextGroup === -1 ? layout.length : nextGroup);
}

describe("F3-2 导航分组", () => {
  it("两组并存：经营分析（零售经营域）与租赁决策（租赁/交易域）", () => {
    const operating = groupRegion("operating-analysis");
    const lease = groupRegion("lease-decision");
    for (const route of OPERATING_GROUP) {
      expect(operating, `${route} in 经营分析组`).toContain(`item("${route}"`);
      expect(lease, `${route} not in 租赁决策组`).not.toContain(`item("${route}"`);
    }
    for (const route of LEASE_GROUP) {
      expect(lease, `${route} in 租赁决策组`).toContain(`item("${route}"`);
      expect(operating, `${route} not in 经营分析组`).not.toContain(`item("${route}"`);
    }
    // 旧混组标签不再被引用
    expect(layout).not.toContain("nav.group_analysis");
    expect(dict["nav.group_operating_analysis"]).toBeTruthy();
    expect(dict["nav.group_lease_decision"]).toBeTruthy();
  });

  it("nav.portfolio 与页面标题 pf.title 一致（补回「租赁」消歧词）", () => {
    for (const lang of ["zh-CN", "zh-HK", "en"] as const) {
      expect(t("nav.portfolio", lang)).toBe(t("pf.title", lang));
    }
  });

  it("其余导航标签与页面标题一致（以页面标题为准）", () => {
    expect(t("nav.cashflow", "zh-CN")).toBe("未来现金流与财务预测");
    expect(t("nav.promotions", "zh-CN")).toBe(t("promotion.title", "zh-CN"));
    expect(t("nav.fpna_workbench", "zh-CN")).toBe("FP&A 经营工作台");
  });
});
