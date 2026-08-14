#!/usr/bin/env node
/**
 * MONEY-002 守卫：新增的金额字段不得是 float64（ADR-0020 §Decision 8）。
 *
 * 复用 web/scripts/enforce-design.mjs 已验证的计数法 + 行级新增思路：
 * 只检查相对基线的变更行，违规次数比旧行多即失败。存量 float64 金额
 * 字段（MONEY-003 迁移前）在未变更文件里不触发——本守卫只防新增。
 *
 * 词表（写在这里，可扩展）：字段名包含以下任一单词且类型是 float64
 * 即视为金额字段。若词表在既有代码上大量误报，收敛词表，不要加豁免。
 */
import { execSync } from "node:child_process";
import { readFileSync } from "node:fs";
import path from "node:path";

const root = execSync("git rev-parse --show-toplevel").toString().trim();
const base = process.env.CI ? "origin/main" : "main";

const changedFiles = [
  ...execSync(`git diff --name-only ${base}`, { cwd: root }).toString().split("\n"),
  ...execSync(`git ls-files --others --exclude-standard`, { cwd: root }).toString().split("\n"),
]
  .map((p) => p.trim())
  .filter(Boolean)
  .filter((p) => p.endsWith(".go"));

const untracked = new Set(
  execSync(`git ls-files --others --exclude-standard`, { cwd: root }).toString().split("\n").map((p) => p.trim()).filter(Boolean),
);

function changedLines(file) {
  if (untracked.has(file)) {
    try {
      return readFileSync(path.join(root, file), "utf8").split("\n").map((text, index) => ({ number: index + 1, text, oldText: "" }));
    } catch {
      return [];
    }
  }
  const diff = execSync(`git diff -U0 ${base} -- ${file}`, { cwd: root }).toString();
  const lines = [];
  let newLine = 0;
  let removedInHunk = [];
  let addedInHunk = [];
  const flushHunk = () => {
    for (let i = 0; i < addedInHunk.length; i += 1) {
      lines.push({ number: addedInHunk[i].number, text: addedInHunk[i].text, oldText: removedInHunk[i] || "" });
    }
    removedInHunk = [];
    addedInHunk = [];
  };
  for (const raw of diff.split("\n")) {
    const hunk = /^@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/.exec(raw);
    if (hunk) {
      flushHunk();
      newLine = Number(hunk[2]);
      continue;
    }
    if (raw.startsWith("+++") || raw.startsWith("---") || raw.startsWith("@@")) continue;
    if (raw.startsWith("+")) {
      addedInHunk.push({ number: newLine, text: raw.slice(1) });
      newLine += 1;
    } else if (raw.startsWith("-")) {
      removedInHunk.push(raw.slice(1));
    } else {
      newLine += 1;
    }
  }
  flushHunk();
  return lines;
}

// 金额字段名词表——收敛在这里，不要散落。字段名里出现这些词且类型为
// float64（含指针/切片/映射）即命中。
const MONEY_WORDS = "Amount|Balance|Liability|Revenue|Rent|Cost|Interest|Depreciation|Payment|Price|Fee";
// struct 字段声明：`Name float64` / `Name *float64` / `Name []float64` /
// `Name map[string]float64`，Name 含金额词。`_test.go` 里测试夹具的字段
// 同样受约束（夹具是契约的一部分）。
const MONEY_FIELD_RE = new RegExp(
  `\\b\\w*(?:${MONEY_WORDS})\\w*\\s+(?:\\[\\]|\\[\\d+\\]|map\\[[^\\]]+\\])?\\*?float64\\b`,
);

function violationCount(pattern, text) {
  return (text.match(new RegExp(pattern.source, pattern.flags + "g")) || []).length;
}

const violations = [];
for (const file of changedFiles) {
  for (const { number, text: line, oldText } of changedLines(file)) {
    if (violationCount(MONEY_FIELD_RE, line) > violationCount(MONEY_FIELD_RE, oldText)) {
      violations.push(`${file}:${number}: 新增 float64 金额字段（ADR-0020）：用 money.Amount`);
    }
  }
}

if (violations.length > 0) {
  console.error("MONEY-002 守卫拦截失败：");
  for (const violation of violations) {
    console.error("  " + violation);
  }
  process.exit(1);
}
console.log(`enforce-money: ${changedFiles.length} 个变更 Go 文件，无新增 float64 金额字段`);
