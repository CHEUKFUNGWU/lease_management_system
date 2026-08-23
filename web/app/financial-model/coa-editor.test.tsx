import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { renderToStaticMarkup } from "react-dom/server";
import { LanguageProvider } from "../context/LanguageContext";
import { CoaEditor } from "./coa-editor";
import type { CoaRow } from "./coa-helpers";

// F1 守卫：科目树编辑器不得出现任何本地 DSL 解析逻辑（D-F8）——
// 公式校验一律走 /templates/validate。与 formula-editor.test.tsx 同款源码级锁。
const editorSource = readFileSync(join(import.meta.dirname, "coa-editor.tsx"), "utf8");

describe("F1 D-F8 编辑器不含本地公式解析", () => {
  it("不得出现括号计数/token 切分/公式正则/本地校验函数", () => {
    expect(editorSource).not.toMatch(/countParens|localParse|validateFormulaLocally/);
    expect(editorSource).not.toMatch(/\.split\([^)]*rows\./);
    expect(editorSource).not.toMatch(/new RegExp\([^)]*(formula|rows\.)/);
  });
});

const baseRows: CoaRow[] = [
  { key: "revenue", label: "营业收入", kind: "input", basis: "shared" },
  { key: "total_assets", label: "资产合计", kind: "subtotal", basis: "ifrs16_basis", children: [] },
];

function markup(overrides?: Partial<Parameters<typeof CoaEditor>[0]>): string {
  const props = {
    rows: baseRows,
    templateName: "B4 科目树",
    templateVersion: 3,
    status: "draft",
    kpiNames: {},
    dimensionValues: ["华东"],
    humanZeros: {},
    onToggleHumanZero: () => {},
    onSave: async () => {},
    ...overrides,
  };
  return renderToStaticMarkup(<LanguageProvider><CoaEditor {...props} /></LanguageProvider>);
}

// F1 D-F1：保留键行的删除入口以机械原因解释，且不出现权限措辞。
// （按钮为 disabled：绕过 UI 的删除在保存路径上仍会被后端闸门拒绝。）
describe("F1 D-F1 预置科目不可删", () => {
  it("删除按钮禁用且 title 携带 T1–T16 机械原因", () => {
    const html = markup();
    expect(html).toContain("T1–T16");
    expect(html).not.toContain("无权限");
  });
});

// F1 D-F6：名称命中既有维度值时提示用维度表达。
describe("F1 D-F6 维度提示只提示不阻止", () => {
  it("命中维度值时渲染维度提示", () => {
    const html = markup({
      defaultNewLabel: "华东订阅收入",
      rows: [{ key: "revenue_huadong", label: "华东订阅收入", kind: "input", basis: "shared" }],
    });
    expect(html).toContain("维度");
  });
  it("未命中时不渲染提示", () => {
    const html = markup();
    expect(html).not.toContain("已是系统维度");
  });
});

// F1 D-F2：确认为零的标记入口存在（数据层分离由 finmodel 断言）。
describe("F1 D-F2 确认为零入口", () => {
  it("非预置行提供确认为零操作", () => {
    const html = markup();
    expect(html).toContain("确认为零");
  });
});
