/**
 * 审计报告 §A/§E-2「包装了但页面没调用的 api.ts 函数」接线守卫
 * （docs/后端能力与前端触达差距审计_2026-08-27.md）。
 *
 * GUARD-001：本文件钉「包装函数真的被页面消费了」——审计时这四个函数
 * 都是只有 api.ts 包装、没有页面调用（closePackExport 连包装都没有）。
 * 把页面里的调用删掉，下面的断言即红；把宿主页换掉也要同步改这里。
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";
import { dict, t } from "./i18n";

const read = (...segments: string[]) => readFileSync(path.join(import.meta.dirname, ...segments), "utf8");

const api = read("api.ts");
const monthlyClosing = read("../monthly-closing/page.tsx");
const preDealPanel = read("../pre-deal/DealComparePanel.tsx");
const preDealPage = read("../pre-deal/page.tsx");
const roiPanel = read("../promotions/PromotionROIReportPanel.tsx");
const promotionsPage = read("../promotions/page.tsx");
const scenarioPage = read("../scenario-workbench/page.tsx");

describe("四个「最后一厘米」api 函数已接入宿主页", () => {
  it("closePackExport 有包装且被月结页锁账页签消费", () => {
    expect(api).toContain("close-pack/export");
    expect(monthlyClosing).toContain("reportApi.closePackExport(");
  });

  it("dealApi.compare 被报价对比面板消费，面板已挂 pre-deal 页", () => {
    expect(preDealPanel).toContain("dealApi.compare");
    expect(preDealPage).toContain("DealComparePanel");
  });

  it("performanceApi.storePromotionROI 被报告版面板消费，面板已挂 promotions 页", () => {
    expect(roiPanel).toContain("performanceApi.storePromotionROI");
    expect(promotionsPage).toContain("PromotionROIReportPanel");
  });

  it("storeDecisionEventDraft 被情景台消费，且只经显式审批勾选提交", () => {
    expect(scenarioPage).toContain("performanceApi.storeDecisionEventDraft");
    // approved 必须由人显式勾选，前端不得代批
    expect(scenarioPage).toContain("approved: eventDraftApproved");
    expect(scenarioPage).toContain("eventDraftApproved");
  });

  it("事件草稿写入只经 draft 层，结果必须回显 formal_event_created 语义", () => {
    expect(scenarioPage).toContain("formal_event_created");
    expect(scenarioPage).toContain("formalCreated");
  });
});

describe("新增 i18n 键三语齐全", () => {
  const keys = [
    "monthly.close_pack_title",
    "monthly.close_pack_export",
    "monthly.close_pack_desc",
    "pre_deal.compare.title",
    "pre_deal.compare.disagree",
    "promotion.roi_report.title",
    "promotion.roi_report.basis",
    "scenario.event.open",
    "scenario.event.formal_not_created",
    "common.optional",
  ];

  it("每个键三语都有非空文案", () => {
    for (const key of keys) {
      for (const language of ["zh-CN", "zh-HK", "en"] as const) {
        expect(dict[key]?.[language]?.length ?? 0, `${key} ${language}`).toBeGreaterThan(0);
        expect(t(key, language), `${key} ${language} renders`).toBeTruthy();
      }
    }
  });
});
