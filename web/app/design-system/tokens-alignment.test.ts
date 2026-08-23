/**
 * tokens.ts ↔ globals.css :root 对齐测试（DESIGN.md §1「单一真相源」）。
 *
 * tokens.ts 喂 Ant Design（ConfigProvider 只能吃 JS 值），globals.css 的
 * :root 喂所有自定义 CSS。两边的值必须逐项一致；改一边忘另一边时本测试
 * 必须失败——这正是历史上两边漂移的方式。
 *
 * 规则（DESIGN.md §1）：改令牌时 改 tokens.ts → 同步 :root → 跑本测试。
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";
import { colors, darkColors, depth, typography } from "./tokens";

const css = readFileSync(path.join(process.cwd(), "app/globals.css"), "utf8");

function cssVar(name: string): string {
  const match = new RegExp(`--${name}\\s*:\\s*([^;]+);`).exec(css);
  if (!match) throw new Error(`globals.css 缺少 --${name}`);
  return match[1].trim();
}

// 解析 :root 中 var(--x) 引用的最终值
function cssVarResolved(name: string): string {
  const raw = cssVar(name);
  const ref = /^var\(--([^)]+)\)$/.exec(raw);
  if (ref) return cssVarResolved(ref[1]);
  return raw;
}

// 取出 [data-theme="dark"] 块（无嵌套花括号，匹配到第一个 } 即收口）。
function darkBlock(): string {
  const match = /\[data-theme="dark"\][^{]*\{([^}]*)\}/.exec(css);
  if (!match) throw new Error("globals.css 缺少 [data-theme=\"dark\"] 块");
  return match[1];
}

function darkCssVar(name: string): string {
  const match = new RegExp(`--${name}\\s*:\\s*([^;]+);`).exec(darkBlock());
  if (!match) throw new Error(`[data-theme="dark"] 缺少 --${name}`);
  return match[1].trim();
}

function pageHeaderTitle(): { size: string; weight: string } {
  const block = /\.page-header-title\s*\{([^}]*)\}/.exec(css);
  if (!block) throw new Error("globals.css 缺少 .page-header-title");
  const size = /font-size:\s*([^;]+);/.exec(block[1]);
  const weight = /font-weight:\s*([^;]+);/.exec(block[1]);
  if (!size || !weight) throw new Error(".page-header-title 缺 font-size/weight");
  return { size: size[1].trim(), weight: weight[1].trim() };
}

describe("tokens.ts 与 globals.css :root 对齐（DESIGN.md §1）", () => {
  it("边框三档颜色一致", () => {
    expect(cssVarResolved("border-default")).toBe(colors.border.default);
    expect(cssVarResolved("border-strong")).toBe(colors.border.strong);
    expect(cssVarResolved("border-subtle")).toBe(colors.border.subtle);
  });

  it("背景与前景灰阶一致", () => {
    expect(cssVarResolved("bg-page")).toBe(colors.background.page);
    expect(cssVarResolved("bg-surface")).toBe(colors.background.surface);
    expect(cssVarResolved("bg-inset")).toBe(colors.background.inset);
    expect(cssVarResolved("fg-primary")).toBe(colors.foreground.primary);
    expect(cssVarResolved("fg-secondary")).toBe(colors.foreground.secondary);
    expect(cssVarResolved("fg-tertiary")).toBe(colors.foreground.tertiary);
    expect(cssVarResolved("fg-muted")).toBe(colors.foreground.muted);
    expect(cssVarResolved("fg-inverse")).toBe(colors.foreground.inverse);
  });

  it("状态色 text 三件套与 tokens 一致", () => {
    expect(cssVarResolved("state-success-text")).toBe(colors.state.success);
    expect(cssVarResolved("state-warning-text")).toBe(colors.state.warning);
    expect(cssVarResolved("state-error-text")).toBe(colors.state.error);
    expect(cssVarResolved("state-info-text")).toBe(colors.state.info);
  });

  it("焦点环强调色与 state.info 同源（D13，focus-ring.test.ts 锁规则体）", () => {
    // --accent-interactive 不另立新值：亮色 = colors.state.info，
    // 暗色 = darkColors.state.info。改 tokens 不同步 CSS 时这里红。
    expect(cssVarResolved("accent-interactive")).toBe(colors.state.info);
    expect(darkCssVar("accent-interactive")).toBe(darkColors.state.info);
  });

  it("页面主标题字号与字重一致（.page-header-title ↔ display）", () => {
    const title = pageHeaderTitle();
    expect(title.size).toBe(`${typography.sizes.display.size}px`);
    expect(title.weight).toBe(String(typography.sizes.display.weight));
  });

  it("静态深度一致（--shadow-static ↔ depth.static.shadow）", () => {
    expect(cssVarResolved("shadow-static")).toBe(depth.static.shadow);
  });

  it("字重只有三档：tokens 不再定义 700/800（DESIGN.md §4.2，STY-002）", () => {
    const weights = Object.values(typography.weights);
    expect(weights).toEqual([400, 500, 600]);
    for (const size of Object.values(typography.sizes)) {
      expect(size.weight).toBeLessThanOrEqual(600);
    }
  });

  it("图表色板七个槽位与 tokens 一致（DESIGN.md §3.4）", () => {
    expect(cssVarResolved("chart-blue")).toBe(colors.chart.blue);
    expect(cssVarResolved("chart-purple")).toBe(colors.chart.purple);
    expect(cssVarResolved("chart-primary")).toBe(colors.chart.primary);
    expect(cssVarResolved("chart-accent")).toBe(colors.chart.accent);
    expect(cssVarResolved("chart-secondary")).toBe(colors.chart.secondary);
    expect(cssVarResolved("chart-negative")).toBe(colors.chart.negative);
    expect(cssVarResolved("chart-fill")).toBe(colors.chart.fill);
  });

  it("暗色块七个图表槽位齐全，且逐项镜像 darkColors.chart（DARK-004）", () => {
    // 只定义一两个槽位是历史漂移的成因——缺槽位会静默回落到 :root 的浅色值。
    const slots = ["chart-blue", "chart-purple", "chart-primary", "chart-accent", "chart-secondary", "chart-negative", "chart-fill"];
    for (const slot of slots) {
      expect(darkBlock()).toContain(`--${slot}:`);
    }
    expect(darkCssVar("chart-blue")).toBe(darkColors.chart.blue);
    expect(darkCssVar("chart-purple")).toBe(darkColors.chart.purple);
    expect(darkCssVar("chart-primary")).toBe(darkColors.chart.primary);
    expect(darkCssVar("chart-accent")).toBe(darkColors.chart.accent);
    expect(darkCssVar("chart-secondary")).toBe(darkColors.chart.secondary);
    expect(darkCssVar("chart-negative")).toBe(darkColors.chart.negative);
    expect(darkCssVar("chart-fill")).toBe(darkColors.chart.fill);
  });
});
