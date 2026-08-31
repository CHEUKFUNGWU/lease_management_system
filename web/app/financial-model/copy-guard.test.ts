/**
 * /financial-model 文案层+交互层改造守卫（GUARD-001：证明 B 真的生效）。
 *
 * spec 追加决策①②的验收口径是「不懂实现的 FP&A 能只读文案说出每张卡片
 * 是什么、下一步做什么」。本文件锁住页面壳上那些让这件事成立的机械事实：
 * 删掉任何一项（步骤标题、诚实降级提示、示例填充、键释义消费点），对应
 * 断言即红。源码级断言，仿 ai-chat/styles.test.ts 先例。
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const page = readFileSync(join(import.meta.dirname, "page.tsx"), "utf8");

describe("① 文案层：步骤化与人话化", () => {
  it("PageHeader 只保留标题与动作区，不再堆叠副标题", () => {
    expect(page).not.toContain("meta={");
    expect(page).toContain('title={t("nav.financial_model", language)}');
    expect(page).toContain('primaryAction={');
  });

  it("五个步骤标题全部被引用：选模型/填假设/校期初/运行/发布导出", () => {
    for (const key of [
      "finmodel.step_select_def",
      "finmodel.step_assumptions",
      "finmodel.step_opening",
      "finmodel.step_run",
      "finmodel.step_publish_export",
    ]) {
      expect(page.includes(`"${key}"`), `page references ${key}`).toBe(true);
    }
  });

  it("假设键释义真的被消费：assumptionHint 渲染进解码列表，未知键有标注", () => {
    expect(page).toContain("assumptionHint(key, language)");
    expect(page).toContain("finmodel.hint_unknown");
  });

  it("两个「填充示例」按钮分别接在假设卡与期初卡上", () => {
    for (const handler of ["fillAssumptionExample", "fillOpeningExample"]) {
      expect(page.includes(handler), `${handler} wired`).toBe(true);
    }
    // 初始值就是示例：打开页面即见合法输入形状（F3-1 后仍是同一初始文本，
    // 表单与高级 JSON 共用这一份状态）
    expect(page).toContain("useState(EXAMPLE_ASSUMPTIONS)");
  });

  it("F3-1：假设区是键值表单，裸 JSON 降级为折叠的高级入口且两入口同源", () => {
    // 表单消费点：主输入区渲染 AssumptionForm，变更汇入 applyAssumptionFormValues
    expect(page).toContain("<AssumptionForm");
    expect(page).toContain("applyAssumptionFormValues(assumptionsText, changes)");
    // 裸 JSON 文本框降级：只能在 Collapse 的 advanced 子面板内出现
    const collapseAt = page.indexOf("finmodel.assumptions_advanced");
    expect(collapseAt).toBeGreaterThan(-1);
    const textareaAt = page.indexOf("Input.TextArea rows={6} value={assumptionsText}");
    expect(textareaAt, "assumptions JSON textarea lives inside the advanced collapse").toBeGreaterThan(collapseAt);
    // 未知键仍诚实标注（表单不展示、但不隐藏）
    expect(page).toContain("<AssumptionUnknownKeys");
  });
});

describe("② 交互层：期初闸目标流程与诚实降级", () => {
  it("第 1-4 步标签齐全，顺序为 选期间→自动取数→上传→校验", () => {
    const keys = ["finmodel.opening_step1", "finmodel.opening_step2", "finmodel.opening_step3", "finmodel.opening_step4"];
    let cursor = -1;
    for (const key of keys) {
      const at = page.indexOf(key, cursor + 1);
      expect(at, `${key} appears in flow order`).toBeGreaterThan(cursor);
      cursor = at;
    }
  });

  it("引擎取数与导入上传渲染诚实的不可用提示，而不是假按钮", () => {
    expect(page).toContain("finmodel.opening_auto_unavailable");
    expect(page).toContain("finmodel.opening_import_unavailable");
    // 不引入 Upload 组件冒充上传已可用
    expect(page).not.toMatch(/import\s*\{[^}]*\bUpload\b[^}]*\}\s*from\s*"antd"/);
  });

  it("手打合约行降级为高级模式：两张合约子表都在 Collapse 内", () => {
    const collapseAt = page.indexOf('key: "advanced"');
    expect(collapseAt).toBeGreaterThan(-1);
    const afterCollapse = page.slice(collapseAt);
    const leaseRefAt = afterCollapse.indexOf('"leaseRef"');
    const engineAt = afterCollapse.indexOf('"engine"');
    expect(leaseRefAt, "lease_ref table rendered inside advanced panel").toBeGreaterThan(-1);
    expect(engineAt, "engine table rendered inside advanced panel").toBeGreaterThan(leaseRefAt);
  });

  it("三道闸 tooltip 挂在图例 chip 与合约子表标题上", () => {
    expect(page).toContain('gateChip("finmodel.gate1_label", "finmodel.gate1_tooltip")');
    expect(page).toContain('gateChip("finmodel.gate2_label", "finmodel.gate2_tooltip")');
    expect(page).toContain('gateChip("finmodel.gate3_label", "finmodel.gate3_tooltip")');
    expect(page.match(/finmodel\.gate3_tooltip/g)?.length).toBeGreaterThanOrEqual(3); // 图例一次 + 两张子表各一次
  });

  it("校验错误码逐条映射到人话文案（无静默兜底）", () => {
    for (const code of ["no_periods", "bad_balances", "missing_balance_for_period"]) {
      expect(page.includes(code), `OPENING_ERROR_KEY covers ${code}`).toBe(true);
    }
  });
});
