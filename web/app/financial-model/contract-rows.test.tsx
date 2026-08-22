/**
 * F0-1（任务指令：财务视角的 UI/UX 与术语整改）：期初合约行必须有可视
 * label。GUARD-001：替换类改动要证明 B 真的生效——这里用 SSR 渲染断言
 * `<label for>` 与 `<input id>` 真实配对、placeholder 不再兼任字段语义。
 *
 * 自检句：把 ContractRowInputs 里的 <label> 删掉，本文件第一条测试就红；
 * 把 placeholder 改回与 label 同文，第二条就红。
 */
import { describe, expect, it } from "vitest";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { ContractRowInputs } from "./contract-rows";

const language = "zh-CN" as const;

function renderRow(index: number) {
  return renderToStaticMarkup(
    React.createElement(ContractRowInputs, {
      row: { contract_id: "", lease_liability: "", rou_asset: "" },
      index,
      idPrefix: "fm-opening-leaseRef",
      language,
      onChange: () => undefined,
      onRemove: () => undefined,
    }),
  );
}

describe("F0-1 合约行可视 label", () => {
  it("第一行三个输入各自有 <label for> 与 input id 配对，label 文案来自字段语义键", () => {
    const html = renderRow(0);
    for (const [field, labelText] of [
      ["contract_id", "合约编号 / ID"],
      ["lease_liability", "租赁负债"],
      ["rou_asset", "使用权资产"],
    ] as const) {
      const inputId = `fm-opening-leaseRef-0-${field}`;
      expect(html).toContain(`for="${inputId}"`);
      expect(html).toContain(`id="${inputId}"`);
      // label 元素内出现字段语义文案（GUARD-001：不是「旧值消失」，是「新值渲染」）
      const labelRe = new RegExp(`<label[^>]*for="${inputId}"[^>]*>([^<]*)</label>`);
      const match = labelRe.exec(html);
      expect(match, `visible label for ${field}`).not.toBeNull();
      expect(match![1]).toContain(labelText);
    }
  });

  it("placeholder 是填写示例，不再等于字段语义文案", () => {
    const html = renderRow(0);
    expect(html).toContain('placeholder="如 CT-2026-001"');
    expect(html).toContain('placeholder="如 3200000.00"');
    expect(html).toContain('placeholder="如 3255676.79"');
    for (const semantic of ["合约编号 / ID", "租赁负债", "使用权资产"]) {
      expect(html).not.toContain(`placeholder="${semantic}"`);
    }
  });

  it("第二行不再重复可视 label，改用 aria-label 承接字段语义", () => {
    const html = renderRow(1);
    expect(html).not.toContain("<label");
    for (const [field, labelText] of [
      ["contract_id", "合约编号 / ID"],
      ["lease_liability", "租赁负债"],
      ["rou_asset", "使用权资产"],
    ] as const) {
      const inputId = `fm-opening-leaseRef-1-${field}`;
      const inputRe = new RegExp(`<input[^>]*id="${inputId}"[^>]*>`);
      const match = inputRe.exec(html);
      expect(match, `input ${field} rendered`).not.toBeNull();
      expect(match![0]).toContain(`aria-label="${labelText}"`);
    }
  });

  it("两张子表的 DOM id 互斥（idPrefix 不同）", () => {
    const leaseRef = renderToStaticMarkup(
      React.createElement(ContractRowInputs, {
        row: { contract_id: "", lease_liability: "", rou_asset: "" },
        index: 0,
        idPrefix: "fm-opening-leaseRef",
        language,
        onChange: () => undefined,
        onRemove: () => undefined,
      }),
    );
    const engine = renderToStaticMarkup(
      React.createElement(ContractRowInputs, {
        row: { contract_id: "", lease_liability: "", rou_asset: "" },
        index: 0,
        idPrefix: "fm-opening-engine",
        language,
        onChange: () => undefined,
        onRemove: () => undefined,
      }),
    );
    expect(leaseRef).toContain("fm-opening-leaseRef-0-contract_id");
    expect(engine).not.toContain("fm-opening-leaseRef");
    expect(engine).toContain("fm-opening-engine-0-contract_id");
  });
});
