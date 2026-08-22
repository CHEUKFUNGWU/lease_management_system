/**
 * R0-2 守卫：/performance 行动清单与 /store-360 币种状态的枚举不裸渲染。
 *
 * 三层锁定：
 *  1. 后端一致性——severity/status 键集 ⊆ db/init 的 CHECK 约束；已知
 *     category 每个键都能在后端源码里找到写入字面量（hints.test.ts 惯例：
 *     前端不得翻译一个后端不会写的值）。从约束里删掉一个值而不同步前端即红。
 *  2. 渲染行为——每个枚举值经 ActionCells 渲染，输出不含裸枚举字面量；
 *     表外值走「原样 + 未识别」兜底而不是猜中文。
 *  3. 页面消费——page.tsx 真的在用映射组件，不是把枚举塞回 JSX 文本位。
 */
import { describe, expect, it } from "vitest";
import React from "react";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { renderToStaticMarkup } from "react-dom/server";
import { ActionCategoryText, ActionStatusTag, SeverityTag } from "./ActionCells";
import {
  ACTION_CATEGORIES,
  ACTION_SEVERITIES,
  ACTION_STATUSES,
  ACTION_STATUS_LABEL,
  CATEGORY_LABEL,
  SEVERITY_LABEL,
} from "./enums";
import { currencyStatusLabel, CURRENCY_STATUSES } from "../store-360/enums";
import { dict, type Language } from "../lib/i18n";
import { LanguageProvider } from "../context/LanguageContext";

const zh = "zh-CN" as Language;
const repoRoot = join(import.meta.dirname, "../../../");
const sqlInit = readFileSync(join(repoRoot, "db/init/01_init.sql"), "utf8");
const agentDecisionGo = readFileSync(join(repoRoot, "core-service/internal/agenttools/tools/decision.go"), "utf8");
const retailScenariosGo = readFileSync(join(repoRoot, "core-service/internal/handlers/retail_scenarios.go"), "utf8");
const store360Go = readFileSync(join(repoRoot, "core-service/internal/services/retailstore360/store360.go"), "utf8");
const perfPage = readFileSync(join(import.meta.dirname, "page.tsx"), "utf8");
const s360Page = readFileSync(join(import.meta.dirname, "../store-360/page.tsx"), "utf8");

function render(node: React.ReactElement): string {
  return renderToStaticMarkup(React.createElement(LanguageProvider, null, node));
}

/** 从 fpna_action_items 建表块提取指定列的 CHECK 值清单。 */
function checkValues(column: "severity" | "status"): string[] {
  const tableStart = sqlInit.indexOf("CREATE TABLE IF NOT EXISTS fpna_action_items");
  const tableEnd = sqlInit.indexOf(");", tableStart);
  const block = sqlInit.slice(tableStart, tableEnd);
  const match = new RegExp(`CHECK \\(${column} IN \\(([^)]*)\\)\\)`).exec(block);
  expect(match, `fpna_action_items.${column} CHECK found`).not.toBeNull();
  return match![1].split(",").map((v) => v.trim().replaceAll("'", ""));
}

function assertThreeLanguages(key: string) {
  expect(dict[key], `${key} exists`).toBeTruthy();
  for (const lang of ["zh-CN", "zh-HK", "en"] as const) {
    expect(dict[key][lang], `${key} has ${lang}`).toBeTruthy();
  }
}

describe("R0-2 枚举键集与后端一致", () => {
  it("ACTION_SEVERITIES ⊆ fpna_action_items.severity CHECK，且每张表恰好覆盖全部值", () => {
    const constraint = checkValues("severity");
    for (const value of ACTION_SEVERITIES) {
      expect(constraint, `constraint covers ${value}`).toContain(value);
    }
    expect(constraint).toHaveLength(ACTION_SEVERITIES.length);
  });

  it("ACTION_STATUSES ⊆ fpna_action_items.status CHECK，且每张表恰好覆盖全部值", () => {
    const constraint = checkValues("status");
    for (const value of ACTION_STATUSES) {
      expect(constraint, `constraint covers ${value}`).toContain(value);
    }
    expect(constraint).toHaveLength(ACTION_STATUSES.length);
  });

  it("已知 category 每个键都在后端源码里有写入字面量（前端不译后端不会写的值）", () => {
    expect(agentDecisionGo).toContain('"variance_explanation"');
    expect(retailScenariosGo).toContain('"retail_store_scenario"');
    for (const category of ACTION_CATEGORIES) {
      expect(CATEGORY_LABEL[category]).toBeTruthy();
    }
  });

  it("currency_status 取值集合在 Go 侧封闭且被完整覆盖", () => {
    // singleCurrency 只返回这三个字面量；Go 源码里新增第四个而前端没跟上，
    // 这条 not.toHaveLength 会先红提醒同步。
    for (const status of CURRENCY_STATUSES) {
      expect(store360Go).toContain(`"${status}"`);
    }
    expect(CURRENCY_STATUSES).toHaveLength(3);
  });

  it("所有标签键三语齐全", () => {
    for (const value of ACTION_SEVERITIES) assertThreeLanguages(SEVERITY_LABEL[value]);
    for (const value of ACTION_STATUSES) assertThreeLanguages(ACTION_STATUS_LABEL[value]);
    for (const category of ACTION_CATEGORIES) assertThreeLanguages(CATEGORY_LABEL[category]);
    assertThreeLanguages("perf.category.unrecognized");
    assertThreeLanguages("perf.enum.unrecognized");
  });
});

describe("R0-2 渲染不出现裸枚举", () => {
  it("每个 severity 值渲染为中文，输出不含英文枚举字面量", () => {
    for (const value of ACTION_SEVERITIES) {
      const markup = render(React.createElement(SeverityTag, { value, language: zh }));
      expect(markup).toContain(t_of(SEVERITY_LABEL[value]));
      expect(markup).not.toContain(`>${value}<`);
    }
  });

  it("每个 status 值渲染为中文，输出不含英文枚举字面量", () => {
    for (const value of ACTION_STATUSES) {
      const markup = render(React.createElement(ActionStatusTag, { value, language: zh }));
      expect(markup).toContain(t_of(ACTION_STATUS_LABEL[value]));
      expect(markup).not.toContain(`>${value}<`);
    }
  });

  it("已知 category 给中文；未知 category 原样显示并标注未识别", () => {
    const known = render(React.createElement(ActionCategoryText, { value: "variance_explanation", language: zh }));
    expect(known).toContain(t_of("perf.category.variance_explanation"));
    const unknown = render(React.createElement(ActionCategoryText, { value: "some_new_kind", language: zh }));
    expect(unknown).toContain("some_new_kind"); // 开放集合：原样保留机器值供追溯
    expect(unknown).toContain(t_of("perf.category.unrecognized"));
  });

  it("currencyStatusLabel：三个已知值给中文，表外值原样加未识别", () => {
    expect(currencyStatusLabel("known", zh)).toBe(t_of("store360.currency_status.known"));
    expect(currencyStatusLabel("conflict", zh)).toBe(t_of("store360.currency_status.conflict"));
    expect(currencyStatusLabel("unknown", zh)).toBe(t_of("store360.currency_status.unknown"));
    expect(currencyStatusLabel("brand_new_value", zh)).toContain("brand_new_value");
    expect(currencyStatusLabel("brand_new_value", zh)).toContain(t_of("store360.currency_status.unrecognized"));
  });
});

describe("R0-2 页面真的消费映射（GUARD-001：B 生效）", () => {
  it("performance/page.tsx 经 ActionCells 取文案，不再拼接原始枚举", () => {
    expect(perfPage).toContain("<SeverityTag value={value}");
    expect(perfPage).toContain("<ActionStatusTag value={value}");
    expect(perfPage).toContain("<ActionCategoryText value={row.category}");
    // 旧的裸渲染形态不得回来
    expect(perfPage).not.toMatch(/>\{row\.category\}</);
    expect(perfPage).not.toMatch(/<StatusTag[^>]*>\{value\}<\/StatusTag>/);
  });

  it("store-360/page.tsx 经 currencyStatusLabel 取文案", () => {
    expect(s360Page).toContain("currencyStatusLabel(response.currency_status");
    expect(s360Page).not.toContain("({response.currency_status})");
  });
});

/** t() 的测试侧等价：直接查 dict，避免依赖组件运行时。 */
function t_of(key: string): string {
  const entry = dict[key];
  expect(entry, `dict has ${key}`).toBeTruthy();
  return entry![zh];
}
