/**
 * DESIGN.md §14 全树级债务预算（UIUX 审查报告 2026-08-21 P0-B 结构性缺口）。
 *
 * 缺口：enforce-design 是 diff 级拦截器（只看相对基线的新增行），
 * design-debt-baseline.json 只记分支级新增——**已合入 main 的漂移没有任何
 * 计数器盯着**。2026-08-18→08-21 内联样式 946→1032 的回潮正是从这条缝
 * 进来的：PR 阶段没跑 lint，合并后 diff 归零，守卫永远绿。
 *
 * 本测试是最后一条防线：对全树重算五个止血条款指标，超过 design-tree-budget.json
 * 的预算即失败，无论违规来自哪个分支、哪次合并。清理债务后把预算数字
 * 下调（见 JSON 内 purpose 说明），让预算只紧不松。
 */
import { describe, expect, it } from "vitest";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";

const appDir = join(import.meta.dirname, "..");
const budget = JSON.parse(readFileSync(join(import.meta.dirname, "design-tree-budget.json"), "utf8"));

function walk(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    if (entry === "node_modules" || entry === ".next") continue;
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) walk(full, out);
    else out.push(full);
  }
  return out;
}

const files = walk(appDir);
const tsxFiles = files.filter((f) => /\.tsx$/.test(f));
const countMatches = (list: string[], re: RegExp): number =>
  list.reduce((sum, f) => sum + (readFileSync(f, "utf8").match(re) || []).length, 0);

describe("全树级债务预算（design-tree-budget.json）", () => {
  it("内联 style={{ 开头行数不超预算", () => {
    const count = countMatches(tsxFiles, /style=\{\{/g);
    expect(count).toBeLessThanOrEqual(budget.budgets.inline_style_openers);
  });

  it("tsx 字面量边框不超预算", () => {
    const count = countMatches(tsxFiles, /border(?:-[a-z]+)?\s*:\s*["']?1px\s+solid/g);
    expect(count).toBeLessThanOrEqual(budget.budgets.tsx_border_1px_solid);
  });

  it("JS hover handler 不超预算", () => {
    const count = countMatches(tsxFiles, /onMouse(?:Enter|Leave)=/g);
    expect(count).toBeLessThanOrEqual(budget.budgets.js_hover_handlers);
  });

  it("字重 >600 不超预算", () => {
    const count = countMatches(tsxFiles, /fontWeight\s*:\s*(['"]?)([789]00)\1/g);
    expect(count).toBeLessThanOrEqual(budget.budgets.font_weight_over_600);
  });

  it("globals.css !important 不超预算", () => {
    const css = readFileSync(join(appDir, "app", "globals.css"), "utf8");
    const count = (css.match(/!important/g) || []).length;
    expect(count).toBeLessThanOrEqual(budget.budgets.globals_css_important);
  });

  it("预算数字与实盘一致（防止只升不降的橡皮图章：清了债就要下调预算）", () => {
    // 允许实盘 ≤ 预算（清理在途），但若实盘已经低于预算超过 10%，说明
    // 有人清了债没记账——提醒下调，保持这个文件是活的真相。
    const actual = {
      inline_style_openers: countMatches(tsxFiles, /style=\{\{/g),
      tsx_border_1px_solid: countMatches(tsxFiles, /border(?:-[a-z]+)?\s*:\s*["']?1px\s+solid/g),
      js_hover_handlers: countMatches(tsxFiles, /onMouse(?:Enter|Leave)=/g),
      font_weight_over_600: countMatches(tsxFiles, /fontWeight\s*:\s*(['"]?)([789]00)\1/g),
      globals_css_important: (readFileSync(join(appDir, "app", "globals.css"), "utf8").match(/!important/g) || []).length,
    };
    for (const [key, value] of Object.entries(actual)) {
      const allowed = budget.budgets[key as keyof typeof budget.budgets];
      // 清理后请同步下调 design-tree-budget.json 的对应数字。
      expect(value, `${key} 实盘 ${value} 已低于预算 ${allowed}，请下调预算记账`).toBeGreaterThanOrEqual(
        Math.floor(allowed * 0.9)
      );
    }
  });
});
