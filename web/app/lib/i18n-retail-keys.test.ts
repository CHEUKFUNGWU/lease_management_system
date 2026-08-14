/**
 * I18N-001 双语补完验收测试。
 *
 * G4：三页新增 key 三语齐全、无空串。
 * G5：切到 en / zh-HK 无残留中文 —— 源层面三页与共享 logic 零硬编码 CJK；
 *     翻译层面 en 文案不含 CJK 字符、zh-HK 与 en 均非空。
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";
import { t, type Language } from "./i18n";

const RETAIL_KEY_PREFIXES = ["retail.", "pulse.", "store360.", "scenario.", "common.", "trust."];
const RETAIL_FILES = [
  "app/operating-pulse/page.tsx",
  "app/operating-pulse/logic.ts",
  "app/store-360/page.tsx",
  "app/store-360/logic.ts",
  "app/scenario-workbench/page.tsx",
  "app/scenario-workbench/logic.ts",
];
const CJK = /[\u4e00-\u9fff]/;

function keysInFile(relative: string): string[] {
  const source = readFileSync(path.join(process.cwd(), relative), "utf8");
  const keys = new Set<string>();
  const re = /t\(\s*["']([^"']+)["']/g;
  let match;
  while ((match = re.exec(source)) !== null) keys.add(match[1]);
  return Array.from(keys);
}

describe("G4: retail page translation keys are complete in three languages", () => {
  it("every key used by the three pages (retail prefix) resolves in zh-CN / zh-HK / en without empty strings", () => {
    const used = new Set<string>();
    RETAIL_FILES.forEach((file) => keysInFile(file).forEach((key) => {
      if (RETAIL_KEY_PREFIXES.some((prefix) => key.startsWith(prefix))) used.add(key);
    }));
    expect(used.size).toBeGreaterThan(40);
    for (const key of Array.from(used)) {
      for (const lang of ["zh-CN", "zh-HK", "en"] as Language[]) {
        const value = t(key, lang);
        expect(value.trim(), `${key} in ${lang}`).not.toBe("");
      }
    }
  });
});

describe("G5: no Chinese residue when switching to en / zh-HK", () => {
  it("the three retail pages and their shared logic contain zero hardcoded CJK", () => {
    for (const file of RETAIL_FILES) {
      const source = readFileSync(path.join(process.cwd(), file), "utf8");
      const lines = source.split("\n");
      const hits = lines.map((line, index) => (CJK.test(line) ? index + 1 : 0)).filter(Boolean);
      expect(hits, `${file} has hardcoded CJK at lines ${hits.join(",")}`).toEqual([]);
    }
  });

  it("en translations for the retail keys contain no CJK characters", () => {
    const used = new Set<string>();
    RETAIL_FILES.forEach((file) => keysInFile(file).forEach((key) => {
      if (RETAIL_KEY_PREFIXES.some((prefix) => key.startsWith(prefix))) used.add(key);
    }));
    for (const key of Array.from(used)) {
      expect(CJK.test(t(key, "en")), `${key} en copy contains CJK`).toBe(false);
    }
  });

  it("zh-HK translations exist for the retail keys (no empty fallback)", () => {
    const used = new Set<string>();
    RETAIL_FILES.forEach((file) => keysInFile(file).forEach((key) => {
      if (RETAIL_KEY_PREFIXES.some((prefix) => key.startsWith(prefix))) used.add(key);
    }));
    for (const key of Array.from(used)) {
      expect(t(key, "zh-HK").trim(), `${key} in zh-HK`).not.toBe("");
    }
  });
});
