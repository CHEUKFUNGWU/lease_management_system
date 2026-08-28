/**
 * FP&A 分析师反馈 2026-08-27（P0-1）：/store-pnl 在模拟数据环境打不开。
 *
 * 根因有两处，测试各钉一处：
 * 1. 门店下拉走 operating-facts/stores（月粒度旧表 store_operating_facts，
 *    模拟数据不在其中）——本文件证明旧数据源从页面消失、新源
 *    retail/store-options 已接线；
 * 2. getPnl 不传 data_classification/dataset_version，后端默认 production
 *    拒绝模拟店——URL 层证据在 ../lib/retailAnalyticsApi.test.ts 的
 *    "store pnl carries the data environment" 用例（运行时断言）。
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";
import { dict, t, type Language } from "../lib/i18n";

const page = readFileSync(path.join(import.meta.dirname, "page.tsx"), "utf8");
const languages = ["zh-CN", "zh-HK", "en"] as Language[];

describe("store-pnl 数据环境链路（P0-1）", () => {
  it("旧的月粒度门店数据源不再出现在页面上", () => {
    expect(page).not.toContain("operatingFactsApi");
  });

  it("门店下拉走 retail/store-options 并携带 classification/dataset_version", () => {
    expect(page.replace(/\n\s*/g, "")).toContain("retailAnalyticsApi.storeOptions({");
    expect(page).toContain("data_classification: query.classification");
    expect(page).toContain("dataset_version: query.datasetVersion || undefined");
  });

  it("getPnl 显式透传数据环境参数", () => {
    expect(page).toContain("data_classification: query.classification as RetailDataClassification");
    expect(page).toContain("dataset_version: query.datasetVersion || undefined");
  });

  it("缺失环境时按最新模拟数据集补默认值，且绝不自动生成数据", () => {
    // 两个守卫式 effect：URL 无 classification / simulated 缺版本时回落 latest。
    const fillEffects = page.match(/query\.classification !== "" \|\| latest === undefined|query\.classification !== "simulated" \|\| query\.datasetVersion \|\| latest === undefined/g) || [];
    expect(fillEffects.length).toBe(2);
    expect(page).toContain("latestAnomalyDate(latest)");
    // 演示数据的生成入口只在经营脉搏，本页不写任何数据
    expect(page).not.toContain("generateDefaultSimulation");
  });

  it("无可用模拟数据集时渲染引导态（StateBlock actionable）", () => {
    expect(page).toContain("storepnl.no_dataset_title");
    expect(page).toContain('router.push("/operating-pulse")');
  });

  it("新增 i18n 键三语齐全（键名不在页面上裸拼）", () => {
    for (const key of ["storepnl.no_dataset_title", "storepnl.no_dataset_desc", "storepnl.loading_stores", "storepnl.no_selectable_stores", "storepnl.options_error"]) {
      for (const language of languages) {
        expect(dict[key]?.[language]?.length ?? 0, `${key} ${language}`).toBeGreaterThan(0);
        expect(t(key, language), `${key} ${language} renders`).toBeTruthy();
      }
    }
  });
});
