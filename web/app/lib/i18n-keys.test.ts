/**
 * I18N-002 保护网：任何被引用的 i18n key 都必须存在于字典中。
 *
 * K1：t() 字面量、回退字面量、常量表值、模板前缀与纯文本引用的 key，若
 *     字典缺失即失败。t() 对未知 key 静默返回空串，删错一个 key 就是
 *     生产空白文案 —— 这张网在删除死 key 之后保证没有误删。
 * K2：常量表（*_KEYS / Record map / 字面量数组，含函数内联 map）的值逐一
 *     断言存在 —— 这些 key 经 t(KEY[x]) 或 `const key = KEY[x]` 间接引用，
 *     朴素 grep 找不到。
 *
 * 引用规则与 web/scripts/audit-i18n.mjs 一致，宁严勿松。
 *
 * 已知既有缺陷（K1 显式豁免，不补文案，另行开票）：
 *   - retail.kpi.labor_cost：scenario-workbench 的 KPI 表引用但字典缺失，
 *     情景工作台的「人工成本」标签当前渲染为空串（main 上即存在）。
 */
import { describe, expect, it } from "vitest";
import { readFileSync, readdirSync, statSync } from "node:fs";
import path from "node:path";

// Documented pre-existing gaps the net must not trip on (see header).
const KNOWN_MISSING: ReadonlySet<string> = new Set(["retail.kpi.labor_cost"]);

function walk(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    if (entry === ".next" || entry === "node_modules") continue;
    const full = path.join(dir, entry);
    if (statSync(full).isDirectory()) walk(full, out);
    else if (/\.(ts|tsx)$/.test(entry)) out.push(full);
  }
  return out;
}

function dictKeysOf(source: string): Set<string> {
  const keys = new Set<string>();
  for (const match of Array.from(source.matchAll(/^\s*"([^"]+)":\s*\{/gm))) keys.add(match[1]);
  return keys;
}

const KEY_SHAPE = /^[a-z][a-z0-9_-]*(?:\.[a-z0-9_-]+)+$/;

// referencedKeys returns every i18n key the file references, under the four
// rules the dead-key audit uses.
function referencedKeys(source: string, dictKeys: Set<string>): Set<string> {
  const referenced = new Set<string>();
  const dictKeyList = Array.from(dictKeys);

  // 1. t() literal calls and fallback literals inside them
  const callRe = /\bt\(\s*([`'"])/g;
  let m: RegExpExecArray | null;
  while ((m = callRe.exec(source)) !== null) {
    const quote = m[1];
    const start = m.index + m[0].length - 1;
    if (quote === "`") {
      const end = source.indexOf("`", start + 1);
      if (end === -1) continue;
      const body = source.slice(start + 1, end);
      if (!body.includes("${")) referenced.add(body);
      continue;
    }
    const end = source.indexOf(quote, start + 1);
    if (end === -1) continue;
    referenced.add(source.slice(start + 1, end));
    let depth = 1;
    let i = end + 1;
    while (i < source.length && depth > 0) {
      const ch = source[i];
      if (ch === "(") depth++;
      else if (ch === ")") depth--;
      else if ((ch === '"' || ch === "'") && depth === 1) {
        const qEnd = source.indexOf(ch, i + 1);
        if (qEnd !== -1) {
          const fallback = source.slice(i + 1, qEnd);
          // Fallback literals are keys only when they look like one;
          // replacement values (e.g. { total: "__TOTAL__" }) are not.
          if (KEY_SHAPE.test(fallback)) referenced.add(fallback);
          i = qEnd;
        }
      }
      i++;
    }
  }

  // 2. constant-table values: value-position strings shaped like i18n keys
  const mapRe = /const\s+[A-Za-z_$][\w$]*\s*[:=]\s*(?:Record<[^>]+>\s*=\s*|\{)/g;
  while ((m = mapRe.exec(source)) !== null) {
    let depth = 0;
    let i = m.index + m[0].length - 1;
    while (i < source.length) {
      const ch = source[i];
      if (ch === "{") depth++;
      else if (ch === "}") {
        depth--;
        if (depth === 0) break;
      } else if (ch === '"' && depth > 0) {
        let j = i - 1;
        while (j >= 0 && /\s/.test(source[j])) j--;
        if (source[j] !== ":") {
          i++;
          continue;
        }
        const qEnd = source.indexOf('"', i + 1);
        if (qEnd === -1) break;
        const value = source.slice(i + 1, qEnd);
        if (KEY_SHAPE.test(value)) referenced.add(value);
        i = qEnd;
      }
      i++;
    }
  }

  // 3. template prefixes (`reports.asset_type_${x}` protects all its keys)
  const templateRe = /`([^`]*)`/g;
  while ((m = templateRe.exec(source)) !== null) {
    const prefix = m[1].split("${")[0];
    if (KEY_SHAPE.test(prefix)) {
      for (const key of dictKeyList) {
        if (key.startsWith(prefix)) referenced.add(key);
      }
    }
  }

  // 4. plain-text occurrence (ternaries and any other pattern)
  for (const key of dictKeyList) {
    if (source.includes(key)) referenced.add(key);
  }
  return referenced;
}

function sourceFiles(): string[] {
  return walk(path.join(process.cwd(), "app")).filter(
    (file) => !file.endsWith(path.join("lib", "i18n.ts")),
  );
}

describe("K1: every referenced i18n key exists in the dictionary", () => {
  it("no referenced key is missing (literal, fallback, table, template or text)", () => {
    const dictSource = readFileSync(path.join(process.cwd(), "app", "lib", "i18n.ts"), "utf8");
    const dictKeys = dictKeysOf(dictSource);
    const missing = new Set<string>();
    for (const file of Array.from(sourceFiles())) {
      const referenced = referencedKeys(readFileSync(file, "utf8"), dictKeys);
      for (const key of Array.from(referenced)) {
        if (!dictKeys.has(key) && !KNOWN_MISSING.has(key)) missing.add(key);
      }
    }
    expect(Array.from(missing).sort()).toEqual([]);
  });
});

describe("K2: constant-table keys all exist in the dictionary", () => {
  it("every table value referenced through t() resolves", () => {
    const dictSource = readFileSync(path.join(process.cwd(), "app", "lib", "i18n.ts"), "utf8");
    const dictKeys = dictKeysOf(dictSource);
    const missing = new Set<string>();
    for (const file of Array.from(sourceFiles())) {
      const source = readFileSync(file, "utf8");
      const referenced = referencedKeys(source, dictKeys);
      for (const key of Array.from(referenced)) {
        if (!dictKeys.has(key) && !KNOWN_MISSING.has(key)) missing.add(key);
      }
    }
    // K2 focuses the net on the tables; K1 above is the same scan, so this
    // assertion guards the documented gap list stays small and explicit.
    expect(Array.from(missing).sort()).toEqual([]);
    expect(KNOWN_MISSING.size).toBeLessThanOrEqual(5);
  });
});
