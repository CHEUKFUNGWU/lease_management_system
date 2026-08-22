/**
 * R2-3 守卫：利润差异归因面板三分支 + 顺序说明常驻。
 *
 * unachievable / material 是本票的两条行为死线，各配反向断言：
 *  - unavailable 分支不得出现任何因子标签或图表（不做部分归因）；
 *  - residual_material=false 时警示不得出现（不许常驻吓人）。
 *
 * 自检句均已实测：
 *  - 把 AttributionView 的 unavailable 分支改成照常渲染瀑布 → 「不渲染图表」红；
 *  - 把 order note 移进 complete 分支 → 「unavailable 下顺序说明仍在」红。
 */
import { describe, expect, it } from "vitest";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { AttributionView, factorLabel } from "./VarianceAttributionPanel";
import { LanguageProvider } from "../context/LanguageContext";
import { t, type Language } from "../lib/i18n";
import type { VarianceAttributionResult } from "../lib/api";

const zh = "zh-CN" as Language;

function render(result: VarianceAttributionResult | null): string {
  return renderToStaticMarkup(
    React.createElement(LanguageProvider, null, React.createElement(AttributionView, { result, currency: "CNY" }))
  );
}

const completeResult: VarianceAttributionResult = {
  currency: "CNY",
  base_profit: 4000,
  current_profit: 2900,
  total_variance: -1100,
  factors: [
    { factor: "footfall", base: 1000, current: 1100, effect: 600, intermediate_profit: 4600 },
    { factor: "conversion_rate", base: 0.1, current: 0.09, effect: -660, intermediate_profit: 3940 },
    { factor: "average_transaction_value", base: 200, current: 250, effect: 1485, intermediate_profit: 5425 },
    { factor: "gross_margin_rate", base: 0.3, current: 0.2, effect: -2475, intermediate_profit: 2950 },
    { factor: "labor_cost", base: 1000, current: 1200, effect: -200, intermediate_profit: 2750 },
    { factor: "occupancy_cost", base: 800, current: 700, effect: 100, intermediate_profit: 2850 },
    { factor: "other_controllable_cost", base: 200, current: 150, effect: 50, intermediate_profit: 2900 },
  ],
  residual: 0,
  residual_material: false,
  decomposition_order: ["footfall", "conversion_rate", "average_transaction_value", "gross_margin_rate", "labor_cost", "occupancy_cost", "other_controllable_cost"],
  status: "complete",
};

const unavailableResult: VarianceAttributionResult = {
  base_profit: 0,
  current_profit: 0,
  total_variance: 0,
  factors: [],
  residual: 0,
  residual_material: false,
  decomposition_order: ["footfall", "conversion_rate", "average_transaction_value", "gross_margin_rate", "labor_cost", "occupancy_cost", "other_controllable_cost"],
  status: "unavailable",
  missing_facts: ["base.labor_cost", "current.revenue"],
};

describe("R2-3 AttributionView 三分支", () => {
  it("complete：残差行渲染，三色制在源码级锁定（SSR 下图表柱体不经测量不落 DOM）", () => {
    const markup = render(completeResult);
    expect(markup).toContain(t("store360.attribution.order", zh));
    expect(markup).toContain(t("store360.attribution.residual", zh, { amount: "\u00a50.00", threshold: "5%" }));
    // 残差不材料时不显示警示
    expect(markup).not.toContain(t("store360.attribution.material_warning", zh));
  });

  it("unavailable：列出缺失字段，且不渲染图表框架；顺序说明仍然常驻", () => {
    const markup = render(unavailableResult);
    expect(markup).toContain(t("store360.attribution.unavailable", zh, { fields: "base.labor_cost, current.revenue" }));
    // 不做部分归因：降级态下连图表框架都不出现
    expect(markup).not.toContain("variance-chart-frame");
    // 顺序说明在数据分支之外：降级态下仍可读
    expect(markup).toContain(t("store360.attribution.order", zh));
  });

  it("residual_material=true：追加解释完整度警示", () => {
    const markup = render({ ...completeResult, residual_material: true });
    expect(markup).toContain(t("store360.attribution.material_warning", zh));
  });

  it("factorLabel：已知因子走 i18n，未知因子原样回显（前端零计算纪律的取名侧）", () => {
    expect(factorLabel("footfall", zh)).toBe(t("store360.attribution.f.footfall", zh));
    expect(factorLabel("brand_new_factor", zh)).toBe("brand_new_factor");
  });
});

describe("R2-3 源码级结构断言", () => {
  const panelSource = readPanel();

  it("瀑布图包在 ResponsiveContainer 里，三色制按语义取色（§8.1 + §8.2）", () => {
    expect(panelSource).toContain("<ResponsiveContainer");
    expect(panelSource).toContain('dataKey="base"');
    expect(panelSource).toContain('dataKey="delta"');
    expect(panelSource).toContain('"var(--chart-primary)"');
    expect(panelSource).toContain('"var(--chart-accent)"');
    expect(panelSource).toContain('"var(--chart-negative)"');
  });

  it("前端零计算：组件不对因子值做算术加工（几何定位的 min/abs 除外）", () => {
    // 禁止出现重算语义的表达：比率、百分比换算、乘除因子值
    expect(panelSource).not.toMatch(/effect\s*[*\/]\s*\d/);
    expect(panelSource).not.toMatch(/\.effect\s*-\s/);
  });
});

function readPanel(): string {
  // eslint-disable-next-line @typescript-eslint/no-var-requires
  const { readFileSync } = require("node:fs") as typeof import("node:fs");
  const { join } = require("node:path") as typeof import("node:path");
  return readFileSync(join(import.meta.dirname, "VarianceAttributionPanel.tsx"), "utf8");
}
