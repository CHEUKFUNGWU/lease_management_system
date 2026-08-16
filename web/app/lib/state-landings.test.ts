/**
 * STATE-001 验收：四处落点各有一条断言（源码层）。
 * 判定本身由 dataState.test.ts 的矩阵覆盖；这里钉住落点接线。
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";

const store360 = readFileSync(path.join(import.meta.dirname, "../store-360/page.tsx"), "utf8");
const workspace = readFileSync(path.join(import.meta.dirname, "../contracts/[id]/workspace/workspace.ts"), "utf8");
const settings = readFileSync(path.join(import.meta.dirname, "../settings/page.tsx"), "utf8");
const plFlowPanel = readFileSync(path.join(import.meta.dirname, "../store-360/ProfitFlowPanel.tsx"), "utf8");
const dataState = readFileSync(path.join(import.meta.dirname, "dataState.ts"), "utf8");

describe("STATE-001 landings", () => {
  it("store-360: 正式数据 404 → actionable（切模拟数据）", () => {
    expect(store360).toContain("classifyDataState");
    expect(store360).toContain("store360.actionable_production_empty");
    expect(store360).toContain("status === 404 && query.classification === \"production\"");
  });

  it("合同详情: 无付款计划 calculate → actionable + 打开付款计划页签", () => {
    expect(workspace).toContain("isActionableCalculateError");
    expect(workspace).toContain("contract_detail.calculate_no_schedules");
    expect(workspace).toContain("activeTab: \"payments\"");
  });

  it("设置页: 标签统计 422 → inline actionable（不再只 toast）", () => {
    expect(settings).toContain("classifyDataState");
    expect(settings).toContain("settings.tags_actionable");
    expect(settings).toContain("settings-tags-actionable");
  });

  it("pl-flow 参考实现（FIX-024）: 失败呈现为错误而非空白", () => {
    expect(plFlowPanel).toContain("error");
    expect(plFlowPanel).toContain("store360.pl_flow.load_failed");
  });

  it("判定函数区分 scope_denied 与三分法（独立第四态）", () => {
    expect(dataState).toContain('"scope_denied"');
    expect(dataState).toContain("scope_denied");
  });
});
