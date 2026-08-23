/**
 * F1 批次（任务指令：财务视角的 UI/UX 与术语整改）文案守卫。
 *
 * unslop 口径：界面是财务用户的语言，不是研发的语言——机器词（run/Gap/
 * run-rate/DSL）翻成业务词；语域收敛（不绿→未通过、人话→含义与口径）；
 * 颜色不再承担语义；数据标识与版本态两个坐标轴不许焊死在一个字符串里。
 * 三语同步改，这里逐键断言 zh-CN / zh-HK（en 术语本身是英文，另行核对）。
 */
import { describe, expect, it } from "vitest";
import { dict } from "../lib/i18n";
import { readFileSync } from "node:fs";
import { join } from "node:path";

function val(key: string, lang: "zh-CN" | "zh-HK" | "en"): string {
  const entry = dict[key];
  expect(entry, `dict has ${key}`).toBeTruthy();
  return entry![lang];
}

describe("F1-1 中文文案不含英文机器词", () => {
  it("run → 测算；Gap → 数据缺口说明", () => {
    expect(val("finmodel.basis_note", "zh-CN")).toBe("管理口径三表模型；勾稽未通过的测算不可发布");
    expect(val("finmodel.basis_note", "zh-HK")).toBe("管理口徑三表模型；勾稽未通過的測算不可發佈");
    expect(val("finmodel.gaps", "zh-CN")).not.toMatch(/\bGap\b/);
    expect(val("finmodel.gaps", "zh-HK")).not.toMatch(/\bGap\b/);
    expect(val("finmodel.tie_out_gate_note", "zh-CN")).toBe("勾稽未全部通过：本次测算不能发布为计划版本");
    // F0-4 已把 opening_auto_unavailable 里的 run 一并清掉
    expect(val("finmodel.opening_auto_unavailable", "zh-CN")).not.toMatch(/\brun\b/);
  });

  it("译名全仓统一：界面文案里的「run」一律作「测算」", () => {
    for (const [key, entry] of Object.entries(dict)) {
      if (!key.startsWith("finmodel.")) continue;
      for (const lang of ["zh-CN", "zh-HK"] as const) {
        // 中文字符串里夹裸英文 run / Gap 即违规（en 值除外）
        expect(entry[lang], `${key}[${lang}] contains raw "run"`).not.toMatch(/(?<![A-Za-z])run(?![A-Za-z])/);
        expect(entry[lang], `${key}[${lang}] contains raw "Gap"`).not.toMatch(/(?<![A-Za-z])Gap(?![A-Za-z])/);
      }
    }
  });
});

describe("F1-2 语域与颜色依赖", () => {
  it("不绿/未全绿 → 未通过/未全部通过；人话 → 含义与计算口径", () => {
    for (const key of ["finmodel.basis_note", "finmodel.tie_out_gate_note"]) {
      for (const lang of ["zh-CN", "zh-HK"] as const) {
        expect(val(key, lang)).not.toContain(lang === "zh-CN" ? "不绿" : "不綠");
        expect(val(key, lang)).not.toContain(lang === "zh-CN" ? "未全绿" : "未全綠");
      }
    }
    for (const lang of ["zh-CN", "zh-HK"] as const) {
      expect(val("finmodel.assumptions_hint", lang)).not.toContain(lang === "zh-CN" ? "人话" : "人話");
      expect(val("finmodel.assumptions_hint", lang)).toContain(lang === "zh-CN" ? "含义与计算口径" : "含義與計算口徑");
    }
  });
});

describe("F1-3 分析页英文技术词", () => {
  it("30-day run-rate → 30 天日均折算值；observed store-days → 实际观测店天数", () => {
    for (const key of ["scenario.scope_note", "scenario.loading", "scenario.evidence.formula"]) {
      for (const lang of ["zh-CN", "zh-HK"] as const) {
        expect(val(key, lang), `${key}[${lang}]`).not.toContain("run-rate");
        expect(val(key, lang)).toContain("30 天日均折算");
      }
    }
    expect(val("scenario.evidence.formula", "zh-CN")).not.toContain("observed store-days");
    expect(val("scenario.evidence.formula", "zh-CN")).toContain("实际观测店天数");
    // 口径澄清句原样保留——这正是财务用户需要的
    expect(val("scenario.evidence.formula", "zh-CN")).toContain("经营占用现金成本不等同 IFRS 16 会计费用");
    expect(val("scenario.evidence.formula", "zh-HK")).toContain("經營佔用現金成本不等同 IFRS 16 會計費用");
  });

  it("白名单 DSL → 受限语法", () => {
    expect(val("storepnl.formula_hint", "zh-CN")).toBe(
      "公式（仅支持受限语法，如 rows.revenue - rows.labor_cost；保存时校验）",
    );
    expect(val("storepnl.formula_hint", "zh-CN")).not.toContain("DSL");
    expect(val("storepnl.formula_hint", "en")).not.toContain("whitelist");
  });
});

describe("F1-4 数据标识与版本态拆轴", () => {
  it("trust.classification_* 不再焊接版本态后缀", () => {
    for (const key of ["trust.classification_production", "trust.classification_simulated", "trust.classification_mixed"]) {
      for (const lang of ["zh-CN", "zh-HK", "en"] as const) {
        expect(val(key, lang), `${key}[${lang}]`).not.toMatch(/·\s*(Working|Official)/);
      }
    }
    // 版本态由独立键表达
    expect(dict["trust.basis_working"]).toBeTruthy();
    expect(dict["trust.basis_official"]).toBeTruthy();
  });

  it("financial-model 页：分类下拉走 trust 键，版本态是独立标签 + tooltip", () => {
    const page = readFileSync(join(import.meta.dirname, "../financial-model/page.tsx"), "utf8");
    expect(page).not.toContain('"production · Working"');
    expect(page).not.toContain('"simulated · Working"');
    expect(page).toContain('label: t("trust.classification_production", language)');
    expect(page).toContain('t("trust.basis_working", language)');
    expect(page).toContain("finmodel.version_state_tooltip");
  });

  it("ai-chat 页：上下文标签走 t()，不再硬编码中文复合字面量", () => {
    const page = readFileSync(join(import.meta.dirname, "../ai-chat/page.tsx"), "utf8");
    expect(page).not.toContain("模拟 · Working");
    expect(page).not.toContain("正式 · Working");
    expect(page).toMatch(/classification === "simulated" \? "trust\.classification_simulated" : "trust\.classification_production"/);
  });
});
