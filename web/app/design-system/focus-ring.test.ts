/**
 * D13 / DESIGN.md §11 双环焦点环守卫（UIUX 审查报告 2026-08-21 P0-A）。
 *
 * 历史缺陷：全站焦点环是单环 `outline: 2px solid var(--fg-primary)`（纯黑），
 * 在 --admin-surface、代码块深色面以及整个暗色主题上不可见——键盘
 * 用户在暗色模式下找不到焦点在哪。暗色模式（DARK-003）上线后，不可见范围
 * 从两个局部面扩大到整个主题，所以这条从 P2 升级成 P0。
 *
 * 处方（DESIGN.md §11 原文）：双环 box-shadow —— 内环匹配页面画布、外环是
 * 交互强调色。四条焦点规则 + 一条输入框例外全部锁定精确规则体。
 *
 * GUARD-001 自检：把 :root 里 --focus-ring 的定义删掉或改错、把任一规则的
 * box-shadow 换回 outline 单环，下面的断言都会红。断言取的是目标选择器的
 * 规则体本身，不是全文正则（FIX-021 教训）。
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";

const css = readFileSync(path.join(process.cwd(), "app/globals.css"), "utf8");

/** 取「选择器文本紧随其后的 {} 块」的规则体；选择器用正则源书写，
 *  分组选择器的逗号换行由 \\s* 吸收。找不到即抛错（规则被删 = 测试红）。 */
function ruleBody(selectorSource: string): string {
  const match = new RegExp(`${selectorSource}\\s*\\{([^}]*)\\}`).exec(css);
  if (!match) throw new Error(`globals.css 缺少规则：${selectorSource}`);
  return match[1];
}

/** [data-theme="dark"] 块（无嵌套花括号，与 tokens-alignment.test.ts 同法）。 */
function darkBlock(): string {
  const match = /\[data-theme="dark"\][^{]*\{([^}]*)\}/.exec(css);
  if (!match) throw new Error('globals.css 缺少 [data-theme="dark"] 块');
  return match[1];
}

/** 四条焦点规则的完整选择器（正则源）。逐条锁定，防止某一条被漏改回去。 */
const FOCUS_RULES: Array<[string, string]> = [
  ["全局 :focus-visible", "(?:^|[^.\\w]):focus-visible"],
  [
    "头部图标按钮组",
    "\\.layout-icon-button:focus-visible,\\s*\\.app-logo:focus-visible,\\s*\\.ant-layout-header button:focus-visible,\\s*\\.notification-view-all:focus-visible",
  ],
  ["AntD 按钮/选择器", "body \\.ant-btn:focus-visible,\\s*body \\.ant-select:focus-visible"],
  ["AI 会话项", "\\.ai-chat-session-item:focus-visible"],
];

describe("双环焦点环（D13 / DESIGN.md §11）", () => {
  it(":root 定义了交互强调色与双环公式", () => {
    const root = ruleBody(":root");
    expect(root).toContain("--accent-interactive: #5A6F87;");
    expect(root).toContain(
      "--focus-ring: 0 0 0 2px var(--bg-page), 0 0 0 4px var(--accent-interactive);"
    );
  });

  it("暗色主题换更亮的强调色，双环公式经 var 引用自动适配", () => {
    // 公式本体只在 :root 定义一次；暗色块只覆盖强调色。
    expect(darkBlock()).toContain("--accent-interactive: #64B5F6;");
    expect(darkBlock()).not.toContain("--focus-ring:");
  });

  it.each(FOCUS_RULES)("焦点规则带双环且不再引用黑色单环：%s", (_name, selector) => {
    const body = ruleBody(selector);
    expect(body).toContain("outline: none;");
    expect(body).toContain("box-shadow: var(--focus-ring);");
    expect(body).not.toContain("var(--fg-primary)");
  });

  it("输入框例外仍然只靠边框+光晕表达焦点（不叠双环）", () => {
    const body = ruleBody(
      "\\.ant-input:focus-visible,\\s*\\.ant-input-affix-wrapper \\.ant-input:focus-visible"
    );
    expect(body).toContain("outline: none;");
    expect(body).toContain("box-shadow: none;");
  });

  it("全文件不再有黑色单环焦点规则（负向清扫）", () => {
    expect(css).not.toContain("outline: 2px solid var(--fg-primary)");
  });
});
