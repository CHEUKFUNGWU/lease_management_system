/**
 * ai-chat 专项守卫（UIUX 审查报告 2026-08-21 §9 遗留项）。
 *
 * 背景：本页曾是全仓最大内联样式单点（118 处，占全仓 11%）。2026-08-22
 * 整体迁移到 globals.css 的「AI Chat Page」区段。本文件锁三件事：
 *
 * 1. 页面样式全部走 globals.css；SyntaxHighlighter 的主题对象是组件 API，
 *    不属于页面 CSS 内联样式。新增静态内联样式应让 count 断言变红。
 * 2. 页面内不再有 JS 改样式的 hover/focus 处理器（§13-3）。
 * 3. 关键类的规则体存在且值正确（class-coverage.test.ts 只验证「类有规
 *    则」，这里验证「规则是对的」——FIX-021 教训：不能只靠覆盖率）。
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const root = join(import.meta.dirname, "..", "..");
const page = readFileSync(join(root, "app/ai-chat/page.tsx"), "utf8");
const css = readFileSync(join(root, "app/globals.css"), "utf8");

function ruleBody(selectorSource: string): string {
  const escaped = selectorSource.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = new RegExp(`${escaped}\\s*\\{([^}]*)\\}`).exec(css);
  if (!match) throw new Error(`globals.css 缺少规则：${selectorSource}`);
  return match[1];
}

/** 页面不保留 CSS 内联样式；组件主题对象不计入 style={{ ... }}。 */
describe("ai-chat/page.tsx 内联样式残留清单", () => {
  const openerMatches = page.match(/style=\{\{/g) || [];

  it("页面不保留 CSS 内联样式", () => {
    expect(openerMatches.length).toBe(0);
  });

  it("样式对象不会回到页面 JSX", () => {
    const blocks: string[] = [];
    const re = /style=\{\{([^}]*)\}\}/g;
    let m: RegExpExecArray | null;
    while ((m = re.exec(page)) !== null) blocks.push(m[1]);
    for (const block of blocks) {
      expect(block).toContain("backgroundColor");
      expect(block).toContain("var(--");
      expect(block).not.toMatch(/#[0-9a-fA-F]{3,8}/);
      expect(block).not.toMatch(/rgba?\(/);
    }
  });

  it("页面内没有 JS 改样式的 hover/focus 处理器（§13-3）", () => {
    expect(page).not.toMatch(/onMouse(?:Enter|Leave)=/);
    expect(page).not.toMatch(/\.(style)\.\w+\s*=/);
    expect(page).not.toMatch(/onFocus=\{\(e\)[^}]*closest/);
    expect(page).not.toMatch(/onBlur=\{\(e\)[^}]*closest/);
  });
});

describe("关键类规则体正确性（globals.css AI Chat 区段）", () => {
  it("会话行基础态与选中态拆分为 is-active 修饰符", () => {
    const row = ruleBody(".ai-chat-session-row");
    expect(row).toContain("padding: 7px 10px;");
    expect(row).toContain("border: 1px solid transparent;");
    const active = ruleBody(".ai-chat-session-row.is-active");
    expect(active).toContain("background: var(--bg-inset);");
    expect(active).toContain("border-color: var(--border-default);");
    // 原 JS hover 只作用于未选中行的守卫条件，由 :not(.is-active) 承接。
    const hover = new RegExp(
      "\\.ai-chat-session-row:not\\(\\.is-active\\):hover\\s*\\{([^}]*)\\}"
    ).exec(css);
    expect(hover).not.toBeNull();
    expect(hover![1]).toContain("background: var(--bg-inset);");
  });

  it("composer 容器的 focus 态走 :focus-within，取值与被删除的 JS 一致", () => {
    const base = ruleBody(".chat-input-wrapper");
    expect(base).toContain("border: 1px solid var(--border-default);");
    expect(base).toContain("box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);");
    const focus = /focus-within\s*\{([^}]*)\}/.exec(css);
    expect(focus).not.toBeNull();
    expect(focus![1]).toContain("border-color: var(--fg-primary);");
    expect(focus![1]).toContain("box-shadow: 0 2px 12px rgba(0, 0, 0, 0.08);");
  });

  it("面板容器用环形阴影替代字面量边框（§13-8）", () => {
    for (const cls of [".ai-trace-panel", ".ai-review-panel"]) {
      const body = ruleBody(cls);
      expect(body).toContain("box-shadow: var(--shadow-static), 0 0 0 1px var(--border-default);");
      expect(body).not.toContain("border:");
    }
    const bubble = ruleBody(".ai-bubble");
    expect(bubble).toContain("border-radius: 12px;");
    expect(bubble).toContain("box-shadow: var(--shadow-static), 0 0 0 1px var(--border-default), 0 1px 3px rgba(0, 0, 0, 0.04);");
  });

  it("技能卡 hover 取值与原 JS 逐字一致（GUARD-001）", () => {
    const card = ruleBody(".ai-skill-card");
    expect(card).toContain("box-shadow: 0 0 0 1px var(--border-subtle);");
    const hover = /\.ai-skill-card:hover,\s*\.ai-skill-card:focus-visible\s*\{([^}]*)\}/.exec(css);
    expect(hover).not.toBeNull();
    expect(hover![1]).toContain(
      "box-shadow: 0 0 0 1px var(--border-strong), 0 2px 8px rgba(0, 0, 0, 0.04);"
    );
  });
});
