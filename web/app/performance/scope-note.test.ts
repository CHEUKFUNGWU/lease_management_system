/**
 * R0-3 守卫：三个页面的定位说明真的渲染，且挂在 PageHeader 之下。
 *
 * 渲染断言直接打 ScopeNote 组件（页面内联 Alert 够不着 SSR——
 * ProtectedRoute/AuthContext 会挡）；挂载位置与消费点用源码级断言锁定，
 * 仿 copy-guard.test.ts 先例。导航不变性（/roi 不在导航、其余路由不删）
 * 在 web/app/lib/nav-grouping.test.ts。
 */
import { describe, expect, it } from "vitest";
import React from "react";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { renderToStaticMarkup } from "react-dom/server";
import ScopeNote from "../components/ScopeNote";
import { LanguageProvider } from "../context/LanguageContext";
import { dict, t, type Language } from "../lib/i18n";

const perfPage = readFileSync(join(import.meta.dirname, "page.tsx"), "utf8");
const pulsePage = readFileSync(join(import.meta.dirname, "../operating-pulse/page.tsx"), "utf8");
const roiPage = readFileSync(join(import.meta.dirname, "../roi/page.tsx"), "utf8");

const CASES = [
  { key: "perf.scope_note", className: "perf-scope-note", page: perfPage, pageName: "/performance" },
  { key: "pulse.scope_note", className: "pulse-scope-note", page: pulsePage, pageName: "/operating-pulse" },
  { key: "roi.scope_note", className: "roi-scope-note", page: roiPage, pageName: "/roi" },
] as const;

function render(noteKey: string, language: Language): string {
  return renderToStaticMarkup(
    React.createElement(
      LanguageProvider,
      null,
      React.createElement(ScopeNote, { noteKey, className: "x", language })
    )
  );
}

describe("R0-3 定位说明", () => {
  it("三条文案三语齐全", () => {
    for (const c of CASES) {
      expect(dict[c.key], `${c.key} exists`).toBeTruthy();
      for (const lang of ["zh-CN", "zh-HK", "en"] as const) {
        expect(dict[c.key]![lang], `${c.key} has ${lang}`).toBeTruthy();
      }
    }
  });

  it.each(CASES)("ScopeNote 渲染 $pageName 的定位说明（三语言逐个验）", ({ key }) => {
    for (const lang of ["zh-CN", "zh-HK", "en"] as const) {
      const markup = render(key, lang);
      expect(markup).toContain(t(key, lang));
      expect(markup).toContain("ant-alert-info"); // 信息色，不是警告色
    }
  });

  it.each(CASES)("$pageName 页面真的挂载 ScopeNote，且在 PageHeader 之下", ({ className, key, page }) => {
    expect(page).toContain(`<ScopeNote noteKey="${key}"`);
    expect(page).toContain(`className="${className}"`);
    // 位置：ScopeNote 出现在 PageHeader 之后（R0-3 要求放标题区与内容之间）
    expect(page.indexOf("<ScopeNote")).toBeGreaterThan(page.indexOf("<PageHeader"));
    expect(page).toContain("../components/ScopeNote");
  });
});
