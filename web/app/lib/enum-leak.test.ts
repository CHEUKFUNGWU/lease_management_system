/**
 * F0-5（任务指令：财务视角的 UI/UX 与术语整改）：「枚举不许裸渲染」CI 守卫。
 *
 * 实例不修机制，下次加页面还会犯。本守卫扫 app 目录下全部 .tsx，命中以下形态即红：
 *
 * 1. jsx_text_member      —— `>{x.status}</` / `>{x.kind}</` 直接进 JSX 文本位；
 * 2. paren_bare_enum      —— `({x.some_status})` 括号包裹的裸枚举文本；
 * 3. raw_status_or_fallback —— `(row.some_status || "—")` 渲染函数里的
 *                             「裸枚举或兜底符」，F0-3 修复前的原始形态；
 * 4. snake_label_literal  —— `label: "queued"` 这类未经 t() 的全小写下划线字面量。
 *
 * 已知合法例外**逐条登记在 ALLOWED 并写明理由**；白名单双向锁定——出现
 * 未登记的命中即红，登记过的残留被修掉也必须同步下调计数（不许挂虚账），
 * 同一桶里冒出第三处即红。仿 ai-chat/styles.test.ts 的残留清单先例。
 */
import { describe, expect, it } from "vitest";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";

const appDir = join(import.meta.dirname, "..");

function walk(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    if (entry === ".next" || entry === "node_modules") continue;
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) walk(full, out);
    else out.push(full);
  }
  return out;
}

type Rule = { id: string; re: RegExp; why: string };

const RULES: Rule[] = [
  {
    id: "jsx_text_member",
    re: />\s*\{[A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*\.(?:status|kind)\}\s*</g,
    why: "JSX 文本位直接渲染 x.status / x.kind",
  },
  {
    id: "paren_bare_enum",
    re: /\(\s*\{[A-Za-z_][\w]*(?:\.[A-Za-z_]\w*)*\.(?:[\w]*_)?(?:status|kind)\}\s*\)/g,
    why: "括号内裸枚举文本 ({x.some_status})",
  },
  {
    id: "raw_status_or_fallback",
    re: /\(\s*[a-z_]\w*\.[a-z_]*(?:status|kind)\s*\|\|/g,
    why: "渲染函数里「裸枚举 || 兜底」形态",
  },
  {
    id: "snake_label_literal",
    re: /label:\s*"[a-z][a-z0-9_]{2,}"/g,
    why: '未经 t() 的 snake_case 字面量 label',
  },
];

/** 白名单：file → rule → 允许数量与理由。新增命中必须改代码或显式记账。 */
type Allowance = { file: string; rule: string; count: number; reason: string };

const ALLOWED: Allowance[] = [
  {
    file: "app/monthly-closing/page.tsx",
    rule: "jsx_text_member",
    count: 1,
    reason:
      "存量泄漏（非本批次范围）：月结批次结果 StatusTag 渲染 {result.status} 原始枚举。" +
      "与 F0-2/F0-3 同类错误，但修复需要为 close 批次状态建立独立 i18n 键组与契约测试，" +
      "属下一个批次；已在交付报告中登记。",
  },
  {
    file: "app/store-360/page.tsx",
    rule: "paren_bare_enum",
    count: 1,
    reason:
      "存量泄漏（非本批次范围）：门店 360 头部渲染 ({response.currency_status}) 原始枚举。" +
      "后端 currency_status 取值集未封闭（conflict/unknown/…），修复需先在后端固化清单，" +
      "已在交付报告中登记为跨端前置项。",
  },
];

type Finding = { file: string; rule: string; line: number; snippet: string };

function scan(): Finding[] {
  const findings: Finding[] = [];
  for (const file of walk(appDir).filter((f) => /\.tsx$/.test(f))) {
    const rel = `app/${file.slice(appDir.length + 1)}`;
    const src = readFileSync(file, "utf8");
    for (const rule of RULES) {
      rule.re.lastIndex = 0;
      let m: RegExpExecArray | null;
      while ((m = rule.re.exec(src)) !== null) {
        const line = src.slice(0, m.index).split("\n").length;
        findings.push({ file: rel, rule: rule.id, line, snippet: src.split("\n")[line - 1].trim().slice(0, 140) });
      }
    }
  }
  return findings;
}

describe("F0-5 枚举不裸渲染守卫", () => {
  const findings = scan();

  function allowanceFor(f: Finding): Allowance | undefined {
    return ALLOWED.find((a) => a.file === f.file && a.rule === f.rule);
  }

  it("没有白名单之外的裸枚举渲染", () => {
    const unregistered = findings.filter((f) => !allowanceFor(f));
    expect(
      unregistered.map((f) => `${f.file}:${f.line} [${f.rule}] ${f.snippet}`),
      "新发现的裸枚举渲染必须改为 i18n 映射消费点，或在 ALLOWED 里带理由记账",
    ).toEqual([]);
  });

  it("白名单双向锁定：每条记账的命中数必须精确相等（不许悄悄增、也不许挂虚账）", () => {
    for (const entry of ALLOWED) {
      const actual = findings.filter((f) => f.file === entry.file && f.rule === entry.rule).length;
      expect(actual, `${entry.file} [${entry.rule}] 记账 ${entry.count} 处`).toBe(entry.count);
      expect(entry.reason.length).toBeGreaterThan(20);
    }
  });
});
