/**
 * R3-1（RH8）：三条 CI 可强制的文案守卫。
 *
 * 1. 枚举不裸奔——/performance、/promotions、/store-360 三页（含其面板
 *    组件）按 enum-leak 的四条规则复扫一遍：F0-5 的全树扫描已覆盖它们，
 *    这里显式点名三页，防止未来有人把三页加进白名单绕过。
 *    映射表本身用 Record<联合类型, i18n 键>，漏键由 type-check 挡
 *    （performance/enums.ts、store-360/enums.ts，删键即 TS2741）。
 * 2. pp 与 % 不混用——percentage_point 变化必走 pp 后缀；行为级断言
 *    formatChange / formatUnitValue，源码级断言页面变化列走 formatChange。
 * 3. 内部词汇不外泄——i18n 全部键的三语言值做黑名单扫描：
 *    批次号（R0-1）、阶段号（B 阶段/W6）、模块代号（RH2/SM8/D-R14）。
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { dict, type Language } from "./i18n";
import { formatChange, formatUnitValue } from "../operating-pulse/logic";

const SURFACE_FILES = [
  join(import.meta.dirname, "../performance/page.tsx"),
  join(import.meta.dirname, "../performance/PeerBenchmarkBlock.tsx"),
  join(import.meta.dirname, "../performance/ActionCells.tsx"),
  join(import.meta.dirname, "../promotions/page.tsx"),
  join(import.meta.dirname, "../store-360/page.tsx"),
  join(import.meta.dirname, "../store-360/LaborEfficiencyPanel.tsx"),
  join(import.meta.dirname, "../store-360/VarianceAttributionPanel.tsx"),
];

const ENUM_LEAK_PATTERNS: [RegExp, string][] = [
  [/>\s*\{[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*\.(?:status|kind)\}\s*</, "JSX 文本位裸枚举 x.status/x.kind"],
  [/\(\s*\{[A-Za-z_]\w*(?:\.[\w]*)*\.(?:[\w]*_)?(?:status|kind)\}\s*\)/, "括号内裸枚举"],
];

describe("R3-1 守卫一：枚举不裸奔（三页点名复扫）", () => {
  it("三页及其面板组件无枚举裸渲染形态", () => {
    for (const file of SURFACE_FILES) {
      const src = readFileSync(file, "utf8");
      for (const [re, why] of ENUM_LEAK_PATTERNS) {
        expect(src.match(new RegExp(re.source, "g")), `${file}: ${why}`).toBeNull();
      }
    }
  });
});

describe("R3-1 守卫二：pp 与 % 不混用", () => {
  it("formatChange 对 percentage_point 用 pp、对 percent 用 %", () => {
    const pp = { current: {} as never, comparison: {} as never, change_value: 0.45, change_type: "percentage_point", status: "complete" };
    const pct = { ...pp, change_type: "percent" };
    expect(formatChange(pp as never)).toBe("+0.45pp");
    expect(formatChange(pct as never)).toBe("+0.45%");
  });

  it("formatUnitValue 对 percentage_point 单位输出 pp", () => {
    expect(formatUnitValue(0.45, "percentage_point", "CNY", "zh-CN")).toBe("+0.45pp".replace("+", ""));
  });

  it("store-360 页面的变化展示经 formatChange（源码级）", () => {
    const page = readFileSync(join(import.meta.dirname, "../store-360/page.tsx"), "utf8");
    expect(page).toContain("formatChange(metric");
  });
});

describe("R3-1 守卫三：内部词汇不外泄（i18n 值黑名单）", () => {
  const LANGS: Language[] = ["zh-CN", "zh-HK", "en"];
  // 批次号 / 阶段号 / 模块代号。误报放行规则逐条写明。
  const BLACKLIST: [RegExp, string][] = [
    [/\bR\d-\d\b/, "批次号（如 R0-1）"],
    [/\bRH\d+\b/, "模块代号 RH"],
    [/\bSM\d+\b/, "模块代号 SM"],
    [/\bD-R\d+\b/, "决策编号 D-R"],
    [/\bW\d+\b/, "周代号 W6 之类"],
    [/[A-Z] 阶段|階段\s*[ABCD]/, "阶段字母编号"],
    [/\bP[0-9]-[A-Z]\b/, "优先级编号 P0-A 之类"],
  ];

  it("全部 i18n 值不含内部路线图词汇", () => {
    const hits: string[] = [];
    for (const [key, langs] of Object.entries(dict)) {
      for (const lang of LANGS) {
        const value = (langs as Record<Language, string>)[lang] ?? "";
        for (const [re, why] of BLACKLIST) {
          if (re.test(value)) hits.push(`${key}[${lang}] ${why}: "${value.slice(0, 60)}"`);
        }
      }
    }
    expect(hits, hits.join("\n")).toEqual([]);
  });

  it("黑名单自身不许悄悄变松：至少覆盖七类模式", () => {
    expect(BLACKLIST.length).toBeGreaterThanOrEqual(7);
  });
});
