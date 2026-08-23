/**
 * R2-4 守卫：公式编辑器前端零本地校验（D-R16）+ 三分支渲染。
 *
 * D-R16 的反面教材是「界面说没问题、保存时报错」：一旦前端长出第二套
 * 解析器（哪怕只是括号配对），它必然与后端 AST 分叉。所以这里有两层锁：
 *  1. 源码级——组件不得出现解析逻辑（括号计数、token 切分、正则校验
 *     公式内容）；校验必须经 finModelTemplatesApi.validate；
 *  2. 渲染级——三类后端错误各自渲染成对应文案，循环引用展示完整链路，
 *     unknown 展示 ref_key，都不 split 后端文本。
 *
 * 自检句均已实测：
 *  - 给组件加一个 `formula.split("(")` 括号计数 → 「零本地校验」红；
 *  - 把 err_cycle 分支的 path 换成截断文本 → 循环渲染断言红。
 */
import { describe, expect, it } from "vitest";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { FormulaEditor } from "./FormulaEditor";
import { LanguageProvider } from "../context/LanguageContext";
import { AuthProvider } from "../context/AuthContext";
import { t, type Language } from "../lib/i18n";
import type { TemplateValidationResult } from "../lib/api";

const zh = "zh-CN" as Language;
const panelSource = readFileSync(join(import.meta.dirname, "FormulaEditor.tsx"), "utf8");

function renderWithAuth(result: TemplateValidationResult | null): string {
  return renderToStaticMarkup(
    React.createElement(
      AuthProvider,
      null,
      React.createElement(LanguageProvider, null, React.createElement(FormulaEditor, { language: zh, __testResult: result }))
    )
  );
}

const completeResult: TemplateValidationResult = { valid: true };
const cycleResult: TemplateValidationResult = {
  valid: false,
  errors: [{ kind: "circular_reference", message: "template: circular reference: a -> b -> a", cycle_path: ["a", "b", "a"] }],
};
const unknownResult: TemplateValidationResult = {
  valid: false,
  errors: [{ kind: "unknown_reference", row_key: "__editor_formula__", ref_key: "nope", message: 'reference to unknown row "nope"' }],
};
const syntaxResult: TemplateValidationResult = {
  valid: false,
  errors: [{ kind: "syntax", row_key: "__editor_formula__", message: "unexpected token" }],
};

describe("R2-4 前端零本地校验（源码级）", () => {
  it("组件不含任何解析逻辑：括号计数 / token 切分 / 公式内容正则校验", () => {
    expect(panelSource).not.toMatch(/split\(["']\(/);
    expect(panelSource).not.toMatch(/count.*[({[]|[)}\]]\s*count/i);
    expect(panelSource).not.toMatch(/tokeniz|lex[ie]|parseFormula|validateLocally/);
    expect(panelSource).not.toMatch(/\.match\((?!.*i18n)/);
    // 校验只准走后端接缝
    expect(panelSource).toContain("finModelTemplatesApi.validate");
  });

  it("错误渲染不 split 后端文本：cycle 用结构化 cycle_path", () => {
    expect(panelSource).toContain("cycle.cycle_path.join(\" → \")");
    expect(panelSource).toContain("unknown.ref_key");
    expect(panelSource).not.toMatch(/errors\[0\]\.message\.split|message\.split\("->"\)/);
  });
});

describe("R2-4 编辑器三分支渲染", () => {
  it("valid：显示「公式没问题」，无错误 Alert", () => {
    const markup = renderWithAuth(completeResult);
    expect(markup).toContain(t("finmodel.formula.valid", zh));
    expect(markup).not.toContain("ant-alert-error");
  });

  it("circular：展示完整链路 a → b → a，不是截断文本", () => {
    const markup = renderWithAuth(cycleResult);
    expect(markup).toContain(t("finmodel.formula.err_cycle", zh).replace("{path}", "a → b → a"));
    expect(markup).toContain("a → b → a");
  });

  it("unknown：展示缺失科目键；syntax：展示定位与详情", () => {
    const unknownMarkup = renderWithAuth(unknownResult);
    expect(unknownMarkup).toContain(t("finmodel.formula.err_unknown", zh).replace("{key}", "nope"));
    const syntaxMarkup = renderWithAuth(syntaxResult);
    expect(syntaxMarkup).toContain("unexpected token");
    expect(syntaxMarkup).not.toContain("¥");
  });
});
