/**
 * F0-2（任务指令：财务视角的 UI/UX 与术语整改）：机器枚举不许裸渲染。
 *
 * 源码级守卫（仿 copy-guard.test.ts 先例）：删掉任一映射消费点、或把枚举
 * 字面量直接塞回 JSX 文本位，对应断言即红。映射表内容本身（键集 ↔ 后端、
 * i18n 三语齐全）由 gap-kinds.test.ts 锁定；这里锁「页面真的在消费它」。
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { dict } from "../lib/i18n";
import {
  FIN_MODEL_RUN_STATUSES,
  FIN_MODEL_RUN_TIE_OUT_STATUSES,
  GAP_KIND_LABEL,
  PERIOD_GRAIN_LABEL,
  RUN_STATUS_LABEL,
  TIE_OUT_LABEL,
} from "./enums";

const page = readFileSync(join(import.meta.dirname, "page.tsx"), "utf8");

describe("F0-2 页面消费枚举译名，不渲染裸枚举", () => {
  it("四个修复点都改为经映射表取文案", () => {
    // :311 运行状态 chip
    expect(page).toContain("RUN_STATUS_LABEL[wb.status]");
    // :360 勾稽总状态 Alert（模板串里不再拼接原始 tie_out_status）
    expect(page).not.toContain("${run.tie_out_status}");
    expect(page).toContain("TIE_OUT_LABEL[tieOutStatus]");
    // :372 缺口类型 tag（已知键给中文，未知键标注未识别）
    expect(page).toContain("GAP_KIND_LABEL[gap.kind]");
    expect(page).toContain("finmodel.gap_kind_unknown");
    // :391-393 导出粒度 Select
    expect(page).not.toMatch(/label:\s*"(month|quarter|year)"/);
    expect(page).toContain("PERIOD_GRAIN_LABEL[grain]");
    // 勾稽明细表逐行状态列同样不裸渲染（§0.2 纪律的同一形态）
    expect(page).not.toMatch(/<StatusTag[^>]*>\{v\}<\/StatusTag>/);
  });

  it("每张映射表指向的三语文案都存在且不含英文枚举值", () => {
    for (const status of FIN_MODEL_RUN_STATUSES) {
      const key = RUN_STATUS_LABEL[status];
      expect(dict[key]).toBeTruthy();
      for (const lang of ["zh-CN", "zh-HK"] as const) {
        expect(dict[key][lang].toLowerCase()).not.toContain(status);
        expect(dict[key][lang].length).toBeGreaterThan(0);
      }
    }
    for (const status of FIN_MODEL_RUN_TIE_OUT_STATUSES) {
      const key = TIE_OUT_LABEL[status];
      expect(dict[key]).toBeTruthy();
      for (const lang of ["zh-CN", "zh-HK"] as const) {
        expect(dict[key][lang]).not.toBe(status);
        expect(dict[key][lang].length).toBeGreaterThan(0);
      }
    }
    for (const [grain, key] of Object.entries(PERIOD_GRAIN_LABEL)) {
      for (const lang of ["zh-CN", "zh-HK"] as const) {
        expect(dict[key][lang]).toBe({ month: "月", quarter: "季", year: "年" }[grain as keyof typeof PERIOD_GRAIN_LABEL]);
      }
    }
    for (const key of Object.values(GAP_KIND_LABEL)) {
      expect(dict[key]).toBeTruthy();
    }
  });
});
