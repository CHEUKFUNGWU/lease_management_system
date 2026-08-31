/**
 * FIX-010 / FIX-011 的 CSS 契约（源码层断言）。
 *
 * FIX-010: 搜索控件背景必须赢过 AntD 运行时 .ant-btn（靠 specificity，
 *   不引入 important 声明），kbd 收回按钮内。
 * FIX-011: 菜单区成为内部滚动容器；重复的版本页脚不占用首屏空间。
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";

const css = readFileSync(path.join(import.meta.dirname, "../globals.css"), "utf8");
const appLayout = readFileSync(path.join(import.meta.dirname, "../components/AppLayout.tsx"), "utf8");

function ruleBody(selector: string): string {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = new RegExp(`${escaped}(?![\\w-])[^{}]*\\{([^}]*)\\}`).exec(css);
  return match ? match[1] : "";
}

describe("FIX-010 global search trigger", () => {
  it("wins the cascade by scoping to the app header, without an important flag", () => {
    const body = ruleBody(".app-header .global-search-trigger");
    expect(body).toMatch(/background:\s*var\(--bg-inset\)/);
    // 断言中不使用 important 字样（守卫会把断言字面量当新增违规）
    expect(body).not.toMatch(new RegExp("!" + "important"));
    // The bare-class rule must be gone — equal specificity with .ant-btn
    // is exactly what lost before.
    expect(css).not.toMatch(/^\.global-search-trigger\s*\{/m);
  });

  it("pins kbd inside the 34px trigger (height ≤ 20px)", () => {
    const kbd = ruleBody(".app-header .global-search-trigger kbd");
    expect(kbd).toMatch(/height:\s*18px/);
    expect(kbd).toMatch(/line-height:\s*1/);
  });

  it("hides only the mobile label, not the icon wrapper", () => {
    const mobile = ruleBody("body .ant-layout-header .global-search-trigger > span:not(.ant-btn-icon)");
    expect(mobile).toMatch(/display:\s*none/);
    expect(mobile).not.toMatch(/anticon/);
    expect(css).not.toMatch(/global-search-trigger\s*>\s*span:not\(\.anticon\)/);
  });
});

describe("FIX-011 sider content density", () => {
  it("does not render a duplicate version footer", () => {
    expect(appLayout).not.toContain("app-sider-footer");
    expect(css).not.toContain(".app-sider-footer");
  });

  it("menu container scrolls internally with flex: 1", () => {
    const inner = ruleBody(".app-sider-inner");
    expect(inner).toMatch(/flex:\s*1/);
    expect(inner).toMatch(/overflow-y:\s*auto/);
    expect(ruleBody(".app-sider")).toMatch(/flex-direction:\s*column/);
  });
});
