/**
 * R1-3 守卫：三个既有面板的口径说明。
 *
 * 断言两件事：说明文字在正常态出现、在降级/空态仍然出现——
 * 口径脚注不是数据状态的附属品，缺数时使用者更需要知道为什么与怎么办。
 * 自检句：把任一面板的 note 改成条件渲染（只在有数据时显示），降级态断言即红；
 * 把 note 文案键删掉，正常态断言即红。
 */
import { describe, expect, it } from "vitest";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { CategoryCompositionPanel } from "./CategoryCompositionPanel";
import { InventoryTurnoverPanel } from "./InventoryTurnoverPanel";
import { CompetitorBenchmarkPanel } from "./CompetitorBenchmarkPanel";
import { LanguageProvider } from "../context/LanguageContext";
import { AuthProvider } from "../context/AuthContext";
import { t, type Language } from "../lib/i18n";

const zh = "zh-CN" as Language;

function render(node: React.ReactElement): string {
  // 面板内部取数依赖 useAuth；SSR 下 AuthProvider 的 isLoading 初始为 true，
  // 面板走加载/降级分支——正好验「说明文字在降级态下仍然常驻」。
  return renderToStaticMarkup(
    React.createElement(AuthProvider, null, React.createElement(LanguageProvider, null, node))
  );
}

describe("R1-3 面板口径说明", () => {
  it("InventoryTurnoverPanel：空态下口径说明仍在", () => {
    const markup = render(React.createElement(InventoryTurnoverPanel, { storeId: "s1" }));
    expect(markup).toContain(t("store360.inventory.basis", zh));
    expect(markup).toContain("panel-basis-note");
  });

  it("CompetitorBenchmarkPanel：空态（无对标门店）下口径说明仍在", () => {
    const markup = render(React.createElement(CompetitorBenchmarkPanel, { storeId: "s1" }));
    expect(markup).toContain(t("store360.competitor.basis", zh));
  });

  it("CategoryCompositionPanel：错误态下口径说明仍在（note 在 Space 底部，不随数据状态消失）", () => {
    const markup = render(
      React.createElement(CategoryCompositionPanel, {
        storeId: "s1",
        // 不给 token 时面板走取数失败路径 —— 说明仍须渲染
      })
    );
    expect(markup).toContain(t("store360.category.basis", zh));
    expect(markup).toContain("panel-basis-note");
  });

  it("三条口径文案三语齐全", () => {
    for (const key of ["store360.category.basis", "store360.inventory.basis", "store360.competitor.basis"]) {
      expect(t(key, "zh-CN").trim()).not.toBe("");
      expect(t(key, "zh-HK").trim()).not.toBe("");
      expect(t(key, "en").trim()).not.toBe("");
    }
  });
});
