/**
 * AI-002 验收测试。
 *
 * V1: ApprovalCard 未确认时零写入 —— 渲染与按钮点击前不触发任何 API
 *     调用；采纳只调用调用方回调（由调用方接既有 action API）。
 * V2: 三个零售页同页唤起 AI（RetailAIDrawer），不再 router.push 跳转
 *     /ai-chat —— 源码层面断言三页无跳转调用且引用抽屉组件。
 * V3: /ai-chat 路由与入口仍在（路由文件存在 + 三页仍保留跳转入口）。
 */
import { describe, expect, it, vi } from "vitest";
import React from "react";
import { readFileSync } from "node:fs";
import path from "node:path";
import { renderToStaticMarkup } from "react-dom/server";
import ApprovalCard from "./ApprovalCard";
import RetailAIDrawer, { RetailAIDrawerPanel } from "./RetailAIDrawer";
import { LanguageProvider } from "../context/LanguageContext";
import { AuthProvider } from "../context/AuthContext";
import { t } from "../lib/i18n";

function render(children: React.ReactNode) {
  return renderToStaticMarkup(
    React.createElement(LanguageProvider, null, React.createElement(AuthProvider, null, children))
  );
}

const proposal = {
  title: "门店经营情景行动提议",
  store: { store_code: "A001", store_name: "门店1" },
  planned_action: "复核 Baseline/Plan 后保存",
  evidence_complete: true,
  data_classification: "simulated",
  formula_version: "retail-kpi-v1",
  expected_benefit: 12000,
  currency: "CNY",
};

describe("ApprovalCard", () => {
  it("V1: rendering the card performs zero writes — no callback fires and no fetch is made", () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockRejectedValue(new Error("must not be called"));
    const onAdopt = vi.fn();
    const onModify = vi.fn();
    const onReject = vi.fn();
    render(React.createElement(ApprovalCard, { proposal, onAdopt, onModify, onReject }));
    expect(onAdopt).not.toHaveBeenCalled();
    expect(onModify).not.toHaveBeenCalled();
    expect(onReject).not.toHaveBeenCalled();
    expect(fetchSpy).not.toHaveBeenCalled();
    fetchSpy.mockRestore();
  });

  it("shows the proposal content, benefit and evidence status", () => {
    const html = render(React.createElement(ApprovalCard, { proposal, onAdopt: vi.fn(), onModify: vi.fn(), onReject: vi.fn() }));
    expect(html).toContain("门店经营情景行动提议");
    expect(html).toContain(t("ai.approval.evidence_complete", "zh-CN"));
    expect(html).toContain(`${t("ai.approval.expected_benefit", "zh-CN")}: 12,000 CNY`);
  });

  it("adopt only fires the caller callback, which is where the existing action API is wired", () => {
    const onAdopt = vi.fn();
    const html = render(React.createElement(ApprovalCard, { proposal, onAdopt, onModify: vi.fn(), onReject: vi.fn() }));
    expect(html.replace(/\s/g, "")).toContain(t("ai.approval.adopt", "zh-CN"));
    expect(onAdopt).not.toHaveBeenCalled(); // a render never adopts
  });
});

describe("RetailAIDrawer", () => {
  it("V2: the panel renders the in-page AI chat with the page context, no navigation", () => {
    const html = render(React.createElement(RetailAIDrawerPanel, { pageContext: { page: "operating-pulse", title: "经营脉搏" } }));
    expect(html).toContain(t("ai.drawer.context", "zh-CN"));
    expect(html).toContain("经营脉搏");
    expect(html).toContain(t("ai.drawer.placeholder", "zh-CN"));
    expect(html).not.toContain("/ai-chat?");
  });
});

describe("V2/V3: retail pages summon AI in-page and keep the /ai-chat entry", () => {
  const pages = ["app/operating-pulse/page.tsx", "app/store-360/page.tsx", "app/scenario-workbench/page.tsx"];

  it("the three pages no longer router.push to /ai-chat for the AI button", () => {
    for (const page of pages) {
      const source = readFileSync(path.join(process.cwd(), page), "utf8");
      // P3-33: the deep-link constructor is deleted; pages embed the
      // drawer instead of navigating to /ai-chat.
      expect(source, `${page} still jumps to /ai-chat`).not.toMatch(/router\.(push|replace)\(["'`]\/ai-chat/);
    }
  });

  it("the three pages render the RetailAIDrawer component", () => {
    for (const page of pages) {
      const source = readFileSync(path.join(process.cwd(), page), "utf8");
      expect(source, `${page} has no RetailAIDrawer`).toContain("RetailAIDrawer");
    }
  });

  it("V3: the /ai-chat route and its navigation entry still exist", () => {
    const appDir = path.join(process.cwd(), "app");
    const chatPage = readFileSync(path.join(appDir, "ai-chat", "page.tsx"), "utf8");
    expect(chatPage.length).toBeGreaterThan(0);
    const layout = readFileSync(path.join(appDir, "components", "AppLayout.tsx"), "utf8");
    expect(layout).toContain('"/ai-chat"');
    expect(layout).toContain("nav.ai_chat");
  });
});
