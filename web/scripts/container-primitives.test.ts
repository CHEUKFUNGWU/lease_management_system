/**
 * PRIM-001: 容器原语的守卫契约（全仓机械扫描）。
 *
 * 规则（DESIGN.md §8.1）：
 *   1. 表格横向滚动必须走 tableScrollX；裸 `scroll={{ x: 数字` 只允许在
 *      「有数据才渲染」的条件分支里。
 *   2. recharts 图表必须包在 ResponsiveContainer 里（Sankey/LineChart/
 *      BarChart/AreaChart/ComposedChart/PieChart）。
 *   3. enforce-design.mjs 对新文件整文件扫描（新增文件不得引入内联样式 /
 *      important 声明——STY-005 止血条款）。
 */
import { describe, expect, it } from "vitest";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";

const appDir = join(import.meta.dirname, "../app");
const guard = readFileSync(join(import.meta.dirname, "enforce-design.mjs"), "utf8");

function walk(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    if (entry === ".next" || entry === "node_modules") continue;
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) walk(full, out);
    else if (entry.endsWith(".tsx") || entry.endsWith(".ts")) out.push(full);
  }
  return out;
}

const sources = walk(appDir).map((p) => ({ path: p, text: readFileSync(p, "utf8") }));

describe("PRIM-001 container primitives guard", () => {
  it("every bare scroll={{ x: appears inside a conditional render branch", () => {
    const offenders: string[] = [];
    for (const { path, text } of sources) {
      if (path.endsWith(".test.ts") || path.endsWith(".test.tsx") || path.endsWith(".spec.ts")) continue;
      text.split("\n").forEach((line, index) => {
        if (!line.includes("scroll={{ x:")) return;
        // 条件分支里的空表不存在（rows.length ? <Table/> : <Empty/>），放行；
        // tableScrollX 是唯一其它合法来源。
        const hasCondition = line.includes("?") || line.includes("tableScrollX");
        if (!hasCondition) offenders.push(`${path}:${index + 1}`);
      });
    }
    expect(offenders).toEqual([]);
  });

  it("every recharts chart component sits inside a ResponsiveContainer", () => {
    const CHART_TAGS = ["LineChart", "BarChart", "Sankey", "AreaChart", "ComposedChart", "PieChart"];
    const offenders: string[] = [];
    for (const { path, text } of sources) {
      if (path.endsWith(".test.ts") || path.endsWith(".test.tsx") || path.endsWith(".spec.ts")) continue;
      for (const tag of CHART_TAGS) {
        // 精确组件名（不含 BarChartOutlined 这类图标）：标签后必须跟空白/>/换行
        const used = new RegExp(`<${tag}(\\s|>|/)`).test(text);
        if (!used) continue;
        if (!text.includes("ResponsiveContainer")) {
          offenders.push(`${path}: uses <${tag}> without ResponsiveContainer`);
        }
      }
    }
    expect(offenders).toEqual([]);
  });

  it("STY-005: the guard scans whole untracked files (new files cannot smuggle inline styles)", () => {
    expect(guard).toContain("git ls-files --others --exclude-standard");
    expect(guard).toContain("untracked.has(file)");
    expect(guard).toContain("readFileSync(path.join(root, file)");
  });
});
