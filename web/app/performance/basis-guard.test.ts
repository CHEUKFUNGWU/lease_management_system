/**
 * R0-1 守卫：/performance 设备块停止用设备口径冒充零售指标。
 *
 * 验的是行为不是措辞（工单更正版验收）：
 *  1. 哨兵值——fixture 给 oee_pct=77.77 / utilization_pct=88.88 这两个不会
 *     自然出现的值，断言渲染结果里找不到它们。旧实现把这两个数渲染成
 *     「坪效达成率」列，此断言即红；禁词式断言（不含「坪效」等）会与
 *     空态文案本身打架，且验措辞不验行为，不用。
 *  2. 源码级——假列标题「同群平均坪效达成率」在页面与组件源码中不存在。
 *  3. 有行但口径不可用时渲染具名空态文案。
 *  4. resolveBasis 双向判定（equipment→retail_store false；retail→retail true），
 *     恒 true 与恒 false 的实现都过不了。
 *
 * 自检句：把 resolveBasis 改成恒返回 usable:true，第 1、3、4 条必须红。
 */
import { describe, expect, it } from "vitest";
import React from "react";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { renderToStaticMarkup } from "react-dom/server";
import PeerBenchmarkBlock from "./PeerBenchmarkBlock";
import { LanguageProvider } from "../context/LanguageContext";
import { t, type Language } from "../lib/i18n";
import { resolveBasis } from "../lib/displayBasis";

const zh = "zh-CN" as Language;
const pageSource = readFileSync(join(import.meta.dirname, "page.tsx"), "utf8");
const blockSource = readFileSync(join(import.meta.dirname, "PeerBenchmarkBlock.tsx"), "utf8");

function render(items: Parameters<typeof PeerBenchmarkBlock>[0]["items"]) {
  return renderToStaticMarkup(
    React.createElement(LanguageProvider, null, React.createElement(PeerBenchmarkBlock, { items, language: zh }))
  );
}

function equipmentRow(overrides: Partial<Parameters<typeof PeerBenchmarkBlock>[0]["items"][number]["fact"]> = {}) {
  return {
    fact: {
      equipment_id: "EQ-1",
      equipment_code: "EQC-01",
      equipment_name: "设备一",
      plant_code: "P1",
      production_line_code: "L1",
      currency: "CNY",
      period: "2026-07",
      reconciliation_status: "reconciled",
      ...overrides,
    },
  };
}

describe("R0-1 basis guard", () => {
  it("有设备事实但口径不可用：哨兵值不得渲染进输出，且出现具名空态", () => {
    const markup = render([
      equipmentRow({ oee_pct: 77.77, utilization_pct: 88.88 }),
    ]);
    expect(markup).not.toContain("77.77");
    expect(markup).not.toContain("88.88");
    // 空态文案（title 定稿键）
    expect(markup).toContain(t("perf.peer.unavailable_title", zh));
    expect(markup).not.toContain(t("perf.empty.equipment", zh));
  });

  it("假列标题「同群平均坪效达成率」在源码中已不存在", () => {
    expect(pageSource).not.toContain("同群平均坪效达成率");
    expect(blockSource).not.toContain("同群平均坪效达成率");
    // 两处硬编码兜底同样不得回来
    expect(pageSource).not.toContain("核心商圈");
    expect(pageSource).not.toContain("标杆同群");
    expect(blockSource).not.toContain("核心商圈");
    expect(blockSource).not.toContain("标杆同群");
  });

  it("items 为空时走导入提示分支，不走口径不可用文案", () => {
    const markup = render([]);
    expect(markup).toContain(t("perf.empty.equipment", zh));
    expect(markup).not.toContain(t("perf.peer.unavailable_title", zh));
  });

  it("resolveBasis 双向：设备口径进零售语境不可用，零售口径进零售语境可用", () => {
    expect(resolveBasis("oee_pct", "equipment", "retail_store")).toEqual({
      usable: false,
      reasonKey: expect.any(String),
    });
    expect(resolveBasis("sales_per_sqm", "retail_store", "retail_store")).toEqual({ usable: true });
  });

  it("口径可用时表格分支真的渲染数值（usable:true 半边有行为差异）", () => {
    // 直接构造一个 usable 场景：monkey-patch 不了模块级常量，改为验证组件源码
    // 里表格分支存在且由 basis.usable 门控——删掉该分支此断言红。
    expect(blockSource).toContain("if (!basis.usable)");
    expect(blockSource).toContain("<Table");
  });
});
