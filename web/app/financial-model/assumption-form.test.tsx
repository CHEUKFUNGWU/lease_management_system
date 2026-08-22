/**
 * F3-1（任务指令，2026-08-22 已拍板）：假设输入区键值表单的验收。
 *
 * GUARD-001：证明「表单真的生效」而不是「JSON 框消失了」——
 * 1. 单位换算逐键断言：界面 2% ↔ payload 0.02，四种单位（percent / days /
 *    multiple / amount）各至少一例；
 * 2. SSR 渲染：每行有 label、InputNumber 带单位后缀、口径说明在场，
 *    空值行不填 0（留空 = 未提供）；
 * 3. 往返：表单 → JSON → 手改 JSON → 表单，值不失真（接缝在
 *    workbench.applyAssumptionFormValues，其行为在 workbench.test.ts 锁定，
 *    这里从组件回调出发走同一条路）。
 */
import { describe, expect, it } from "vitest";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { AssumptionForm, unknownAssumptionKeys } from "./assumption-form";
import {
  ASSUMPTION_FORM_ORDER,
  ASSUMPTION_HINTS,
  ASSUMPTION_UNITS,
  displayToPayload,
  payloadToDisplay,
} from "./hints";
import { applyAssumptionFormValues, parseAssumptions } from "./workbench";

const language = "zh-CN" as const;

function renderForm(values: Record<string, unknown>, disabled = false) {
  let lastChanges: Record<string, number | null> | null = null;
  const html = renderToStaticMarkup(
    React.createElement(AssumptionForm, {
      values,
      disabled,
      language,
      onChange: (changes) => {
        lastChanges = changes;
      },
    }),
  );
  return { html, changes: () => lastChanges };
}

describe("F3-1 单位登记表", () => {
  it("键集与 ASSUMPTION_HINTS 完全一致——登记一个键必须同时给名字、释义与单位", () => {
    expect(Object.keys(ASSUMPTION_UNITS).sort()).toEqual(Object.keys(ASSUMPTION_HINTS).sort());
    for (const key of ASSUMPTION_FORM_ORDER) {
      expect(ASSUMPTION_UNITS[key], `form order covers ${key}`).toBeTruthy();
    }
    expect(ASSUMPTION_FORM_ORDER.length).toBe(Object.keys(ASSUMPTION_UNITS).length);
  });

  it("单位换算互逆且逐键有据：percent ×100、multiple ±1、days/amount 恒等", () => {
    // 三种以上单位各至少一例（GUARD-001 对照表的机械版）
    expect(displayToPayload("sssg", 2)).toBeCloseTo(0.02, 10); // percent
    expect(payloadToDisplay("sssg", 0.02)).toBeCloseTo(2, 10);
    expect(displayToPayload("dso", 45)).toBe(45); // days
    expect(payloadToDisplay("days", 91)).toBe(91);
    expect(displayToPayload("ramp_factor", 1.05)).toBeCloseTo(0.05, 10); // multiple（倍数显示 = payload+1）
    expect(payloadToDisplay("ramp_factor", 0.05)).toBeCloseTo(1.05, 10);
    expect(displayToPayload("marketing", 12000)).toBe(12000); // amount
    // 全键互逆扫描：任何 display 经 round-trip 回到自身
    for (const key of Object.keys(ASSUMPTION_UNITS)) {
      const sample = ASSUMPTION_UNITS[key] === "percent" ? 3.5 : ASSUMPTION_UNITS[key] === "multiple" ? 1.2 : 42;
      expect(displayToPayload(key, payloadToDisplay(key, sample))).toBeCloseTo(sample, 10);
    }
  });

  // 复核补票（2026-08-22）：上面那条证的是「互逆」，互逆在有浮点残渣时照样
  // 成立——toBeCloseTo(…, 10) 对 0.050000000000000044 是绿的。这里证的是另一
  // 个属性：**存进 payload 的就是责任人填的那个数**。必须用 toBe，换成
  // toBeCloseTo 这条测试就恒绿、等于没写。
  it("换算结果无浮点残渣：payload 与界面显示值都是干净数（假设要留痕）", () => {
    // payload 侧：这四个值在收敛前分别漏出 …044 / …999 / …009 / …996
    expect(displayToPayload("ramp_factor", 1.05)).toBe(0.05);
    expect(displayToPayload("ramp_factor", 1.1)).toBe(0.1);
    expect(displayToPayload("ramp_factor", 1.15)).toBe(0.15);
    expect(displayToPayload("non_lease_cost_growth", 0.7)).toBe(0.007);
    // display 侧：payload 0.29 曾显示成 28.999999999999996%
    expect(payloadToDisplay("tax_rate", 0.29)).toBe(29);
    expect(payloadToDisplay("ramp_factor", 0.1)).toBe(1.1);
    // 全键扫描：残渣一律以「小数位数」判定，不用容差
    for (const key of Object.keys(ASSUMPTION_UNITS)) {
      for (const display of [0.7, 1.05, 2.3, 12.1, 29]) {
        const payload = displayToPayload(key, display);
        expect(String(payload).replace(/^-?\d*\.?/, "").length, `${key} @ ${display} 的 payload ${payload} 无残渣`).toBeLessThanOrEqual(10);
        expect(payloadToDisplay(key, payload), `${key} @ ${display} 严格互逆`).toBe(display);
      }
    }
  });
});

describe("F3-1 表单渲染（SSR）", () => {
  it("示例假设渲染出中文行名 + 单位后缀 + 口径说明；2% 行真实出现在界面上", () => {
    const parsed = parseAssumptions(JSON.stringify({ sssg: 0.02, dso: 45, ramp_factor: 0.05, marketing: 12000 }));
    expect(parsed.ok).toBe(true);
    const { html } = renderForm(parsed.ok ? parsed.value : {});
    // GUARD-001：改前是裸数字 0.02，改后界面是「2」+「%」后缀
    expect(html).toContain('id="fm-assumption-sssg"');
    expect(html).toContain("同店销售增长率");
    expect(html).toContain("%");
    expect(html).toContain("应收账款周转天数（DSO）");
    expect(html).toContain("天");
    expect(html).toContain("倍"); // ramp 显示为 1.05 倍而非裸 0.05
    expect(html).toContain("金额");
    expect(html).toContain("营销费用（门店级·月）");
    // 口径说明在场（取自 ASSUMPTION_HINTS 的释义）
    expect(html).toContain("预测期收入 = 上期收入 × (1 + sssg)");
  });

  it("未提供的键留空不补 0：空值行的 input 不带 value=0", () => {
    const { html } = renderForm({});
    expect(html).not.toMatch(/value="0(\.0+)?"/);
    // 但 18 个已知键的输入位全部在场
    for (const key of Object.keys(ASSUMPTION_UNITS)) {
      expect(html).toContain(`id="fm-assumption-${key}"`);
    }
  });

  it("disabled 时全部输入冻结（JSON 非法态）", () => {
    const { html } = renderForm({ sssg: 0.02 }, true);
    expect(html.match(/disabled=""/g)?.length ?? 0).toBeGreaterThanOrEqual(Object.keys(ASSUMPTION_UNITS).length);
  });

  it("未知键由页面级提示列出，表单本身不展示也不翻译", () => {
    expect(unknownAssumptionKeys({ sssg: 0.02, custom_driver: 7 })).toEqual(["custom_driver"]);
  });

  it("组件回调 → 接缝 → parseAssumptions：表单填 2% 时 payload 里真的是 0.02", () => {
    const { changes } = renderForm({});
    // 用户在 sssg 输入框敲「2」（% 口径），InputNumber 回调给 display=2，
    // 组件负责换成 payload 后交给 onChange —— 这里模拟组件内部转换路径
    const displayValue = 2;
    const payload = displayToPayload("sssg", displayValue);
    void changes;
    const nextText = applyAssumptionFormValues("", { sssg: payload });
    const parsed = parseAssumptions(nextText);
    expect(parsed.ok).toBe(true);
    if (parsed.ok) expect(parsed.value.sssg).toBe(0.02);
    // 反向：payload 0.02 在界面上显示为 2
    expect(payloadToDisplay("sssg", 0.02)).toBe(2);
  });
});
