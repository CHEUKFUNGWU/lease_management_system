/**
 * R2-1 守卫：投前保本卡片三分支 + payload 尾数。
 *
 * unachievable 是整张票最要紧的分支——它是唯一挡住「返回一个巨大数字
 * 假装有解」的地方。后端守得住（PromoMarginRate<=0 → 金额 nil），前端
 * 拿到 nil 之后也不许自己填一个，所以这条用反向断言锁。
 *
 * 自检句（均已实测）：
 *  - 把 unachievable 分支改成照常渲染金额 → 「unachievable 不渲染任何金额」红；
 *  - 把 cleanFloat 改成恒等函数 (x)=>x → payload 尾数断言红（0.33299999999999996 ≠ 0.333，toBe 严格比较）。
 */
import { describe, expect, it } from "vitest";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { BreakevenPanel, buildBreakevenRequest, upliftPercent } from "./BreakevenPanel";
import { LanguageProvider } from "../context/LanguageContext";
import { t, type Language } from "../lib/i18n";
import type { PromotionBreakevenResult } from "../lib/api";

const zh = "zh-CN" as Language;

function render(result: PromotionBreakevenResult | null): string {
  return renderToStaticMarkup(
    React.createElement(
      LanguageProvider,
      null,
      React.createElement(BreakevenPanel, {
        loading: false,
        rate: 22,
        cost: 5000,
        onRateChange: () => {},
        onCostChange: () => {},
        onRun: () => {},
        result,
      })
    )
  );
}

const achievableResult: PromotionBreakevenResult = {
  currency: "CNY",
  event_days: 7,
  baseline_revenue: 70000,
  required_incremental_revenue: 48181.82,
  required_uplift_rate: 0.6883,
  margin_sacrifice: 5600,
  status: "achievable",
};

const unachievableResult: PromotionBreakevenResult = {
  currency: "CNY",
  event_days: 7,
  baseline_revenue: 70000,
  required_incremental_revenue: null,
  required_uplift_rate: null,
  margin_sacrifice: 21000,
  status: "unachievable",
  unachievable_reason: "折后每多卖一元也不赚钱（-5.00% ≤ 0），不存在保本点。",
};

describe("R2-1 BreakevenPanel 三分支", () => {
  it("achievable：渲染保本增量额与增幅，且不出现警示样式", () => {
    const markup = render(achievableResult);
    expect(markup).toContain(t("promotion.breakeven.result", zh, { amount: "¥48,181.82", pct: "68.8%" }));
    expect(markup).toContain(t("promotion.breakeven.sacrifice", zh, { amount: "¥5,600.00" }));
    expect(markup).not.toContain("ant-alert-warning");
  });

  it("unachievable：渲染警示文案，且不渲染任何金额数字（反向断言）", () => {
    const markup = render(unachievableResult);
    expect(markup).toContain(t("promotion.breakeven.unachievable", zh));
    expect(markup).toContain("breakeven-unachievable-alert");
    // 后端的两个金额字段是 nil；前端不许自己填一个。
    // fixture 里 margin_sacrifice=21000 是后端确实返回的字段——它也不许出现在这个分支里。
    expect(markup).not.toContain("¥");
    expect(markup).not.toContain("21,000");
    expect(markup).not.toContain("48,181");
  });

  it("invalid_input：渲染错误提示而不是数字", () => {
    const markup = render({
      currency: "CNY", event_days: 7, baseline_revenue: 0, margin_sacrifice: 0,
      status: "invalid_input" as const, unachievable_reason: "输入无效：天数、金额不能为负，毛利率必须在 0 与 1 之间",
    });
    expect(markup).toContain("输入无效");
    expect(markup).not.toContain("¥");
  });
});

describe("R2-1 payload 浮点残渣收敛（6009f86 同坑）", () => {
  it("填 33.3% 时 promo_margin_rate 严格等于 0.333（toBe，不是 toBeCloseTo）", () => {
    const req = buildBreakevenRequest("p1", 33.3, 5000);
    expect(req.promo_margin_rate).toBe(0.333);
    expect(req.fixed_marketing_cost).toBe(5000);
    expect(req.valid).toBe(true);
  });

  it("增幅显示同样过收敛：0.6883 × 100 不带尾数", () => {
    // 0.6883 * 100 === 68.83000000000001
    expect(upliftPercent(0.6883)).toBe(68.83);
  });

  it("未填字段标记 invalid，不发请求语义由调用方执行", () => {
    expect(buildBreakevenRequest("p1", null, 5000).valid).toBe(false);
    expect(buildBreakevenRequest("p1", 22, null).valid).toBe(false);
  });
});
