#!/usr/bin/env node
/**
 * I18N-002 审计：统计 i18n key 的引用，输出「确认无引用」的死 key 清单。
 *
 * 引用判定（宁可少删）：
 *   1. t("字面量") 直接调用与 t(...) 内的回退字面量
 *   2. 常量表 / 局部 map / 数组字面量的值（值含 "." 且最终流向 t()）
 *   3. 模板拼接前缀（`reports.asset_type_${x}` 保护所有以该前缀开头的 key）
 *   4. 全文件纯文本子串出现（含三元表达式、不同注解的 map —— 前三类
 *      都漏掉但文本里确实写着这个 key 的情形）
 *
 * 只有 1–4 全部不命中的 key 才算死 key。
 * 用法：node audit-i18n.mjs [--json]
 */
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";

const root = join(import.meta.dirname, "..", "..", "web");
const appDir = join(root, "app");
const dictFile = join(appDir, "lib", "i18n.ts");

const dictSource = readFileSync(dictFile, "utf8");
const dictKeys = new Set();
for (const match of dictSource.matchAll(/^\s*"([^"]+)":\s*\{/gm)) {
  dictKeys.add(match[1]);
}

function walk(dir, out = []) {
  for (const entry of readdirSync(dir)) {
    if (entry === ".next" || entry === "node_modules") continue;
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) walk(full, out);
    else if (/\.(ts|tsx)$/.test(entry)) out.push(full);
  }
  return out;
}
const files = walk(appDir);

const referenced = new Set();
const templatePrefixes = [];
const dynamicSites = [];

// i18n keys are lowercase dotted identifiers; CSS token values and
// replacement placeholders are not keys.
const keyShape = /^[a-z][a-z0-9_-]*(?:\.[a-z0-9_-]+)+$/;

for (const file of files) {
  const source = readFileSync(file, "utf8");

  // 1. t() literals and fallback literals inside the call
  const callRe = /\bt\(\s*([`'"])/g;
  let m;
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
          // fallback literals are keys only when they look like one;
          // replacement values (e.g. { total: "__TOTAL__" }) are not
          if (keyShape.test(fallback)) referenced.add(fallback);
          i = qEnd;
        }
      }
      i++;
    }
  }

  // 2. const map / array values: only strings in value position (after a
  // colon) that look like i18n keys (lowercase dotted identifiers). Map keys
  // (e.g. command names) and CSS token values never reach t().
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
        if (source[j] !== ":") { i++; continue; }
        const qEnd = source.indexOf('"', i + 1);
        if (qEnd === -1) break;
        const value = source.slice(i + 1, qEnd);
        if (keyShape.test(value)) referenced.add(value);
        i = qEnd;
      }
      i++;
    }
  }

  // 3. template prefixes: the static part before the first interpolation of
  // every template literal (linear scan; no nested quantifiers)
  const templateRe = /`([^`]*)`/g;
  while ((m = templateRe.exec(source)) !== null) {
    const prefix = m[1].split("${")[0];
    if (prefix.includes(".")) templatePrefixes.push(prefix);
  }

  // dynamic t() sites for the report
  const dynamicRe = /\bt\(\s*([^"'\`][^,)]{0,40})/g;
  while ((m = dynamicRe.exec(source)) !== null) {
    const arg = m[0].slice(2, m[0].length - 1).trim();
    if (arg.length > 2 && !arg.startsWith("`")) dynamicSites.push(arg);
  }

  // 4. plain-text occurrence (the dictionary's own file is excluded — every
  // key trivially appears in its definition)
  if (file === dictFile) continue;
  for (const key of dictKeys) {
    if (source.includes(key)) referenced.add(key);
  }
}

for (const prefix of templatePrefixes) {
  for (const key of dictKeys) {
    if (key.startsWith(prefix)) referenced.add(key);
  }
}

const dead = [...dictKeys].filter((key) => !referenced.has(key)).sort();

if (process.argv.includes("--json")) {
  console.log(JSON.stringify({ total: dictKeys.size, dead, dynamicSites: [...new Set(dynamicSites)].sort() }, null, 2));
  process.exit(0);
}

console.log(`dict keys: ${dictKeys.size}`);
console.log(`referenced: ${referenced.size}`);
console.log(`dead candidates: ${dead.length}`);
for (const key of dead) console.log(`  ${key}`);
