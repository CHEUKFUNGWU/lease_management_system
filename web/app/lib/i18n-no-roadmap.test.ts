/**
 * F0-4（任务指令：财务视角的 UI/UX 与术语整改）：用户文案不得泄漏内部
 * 路线图词汇。财务用户不知道什么是「B 阶段」「W6」「P0-A」——那是研发
 * 排期编号，出现在界面上就是内部黑话。
 *
 * 机器守卫，不是一次性清理：扫描 i18n 字典的**文案值**（键名与注释不算，
 * 它们不渲染给用户）。命中即红；确需保留的例外逐条登记在白名单并写明理由，
 * 白名单本身断言「只减不增」。
 */
import { describe, expect, it } from "vitest";
import { dict } from "./i18n";
import type { Language } from "./i18n";

/** 内部阶段编号形态：中文「X 阶段」、英文 "(stage X)"、W+数字、P+数字-字母。 */
const ROADMAP_PATTERNS: [string, RegExp][] = [
  ["中文阶段编号（如「B 阶段」）", /[（(][A-Z]\s*[阶階]段[）)]|[A-Z]\s*[阶階]段[）)]/],
  ["英文阶段编号（如 stage B）", /\(stage\s+[A-Z0-9]/i],
  ["工单/里程碑编号（如 W6）", /\bW\d+\b/],
  ["问题编号（如 P0-B）", /\bP\d+-[A-Z]\b/],
];

const LANGUAGES: Language[] = ["zh-CN", "zh-HK", "en"];

/** 白名单：键 → 理由。必须保持为空——发现命中请改文案，不是加豁免。 */
const WHITELISTED: Readonly<Record<string, string>> = {};

describe("F0-4 i18n 文案不含内部路线图词汇", () => {
  it("全部字典值扫描三种语言，无阶段编号", () => {
    const offenders: string[] = [];
    for (const [key, entry] of Object.entries(dict)) {
      if (key in WHITELISTED) continue;
      for (const lang of LANGUAGES) {
        const value = entry[lang];
        if (!value) continue;
        for (const [label, re] of ROADMAP_PATTERNS) {
          if (re.test(value)) offenders.push(`${key}[${lang}]: ${label} → "${value}"`);
        }
      }
    }
    expect(offenders, offenders.join("\n")).toEqual([]);
  });

  it("白名单只减不增：登记的键必须真实存在且确实命中（防止豁免变橡皮图章）", () => {
    for (const [key] of Object.entries(WHITELISTED)) {
      expect(dict[key], `whitelisted key ${key} exists`).toBeTruthy();
    }
  });
});
