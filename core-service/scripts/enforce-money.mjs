#!/usr/bin/env node
/**
 * MONEY-002 守卫：新增的金额字段不得是 float64（ADR-0020 §Decision 8）。
 *
 * 复用 web/scripts/enforce-design.mjs 已验证的计数法 + 行级新增思路：
 * 只检查相对基线的变更行，违规次数比旧行多即失败。存量 float64 金额
 * 字段（MONEY-003 迁移前）在未变更文件里不触发——本守卫只防新增。
 *
 * 分支级存量显式记账（T6b 延伸）：零售 MVP 在 ADR-0020 落地前写下的
 * float64 金额字段已按「文件 → 允许数量」记入 money-debt-baseline.json
 * （带日期），超出允许数量仍然失败。MONEY-003 迁移逐笔清偿时下调对应
 * 数字；把基线挪走或只记总量都会放行新增，只有按文件记数能拦住
 * 「某文件从 40 涨到 41」。
 *
 * 词表（写在这里，可扩展）：字段名包含以下任一单词且类型是 float64
 * 即视为金额字段。若词表在既有代码上大量误报，收敛词表，不要加豁免。
 */
import { execSync } from "node:child_process";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = execSync("git rev-parse --show-toplevel").toString().trim();
const defaultBase = process.env.CI ? "origin/main" : "main";
const BASELINE_PATH = path.join(root, "core-service/scripts/money-debt-baseline.json");

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

export function changedFiles(base = defaultBase) {
  return [
    ...execSync(`git diff --name-only ${base}`, { cwd: root }).toString().split("\n"),
    ...execSync(`git ls-files --others --exclude-standard`, { cwd: root }).toString().split("\n"),
  ]
    .map((p) => p.trim())
    .filter(Boolean)
    .filter((p) => p.endsWith(".go"));
}

export function collectViolations(base = defaultBase) {
  const untracked = new Set(
    execSync(`git ls-files --others --exclude-standard`, { cwd: root }).toString().split("\n").map((p) => p.trim()).filter(Boolean),
  );
  const changedLines = (file) => {
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
  };

  const violations = [];
  for (const file of changedFiles(base)) {
    for (const { number, text: line, oldText } of changedLines(file)) {
      if (violationCount(MONEY_FIELD_RE, line) > violationCount(MONEY_FIELD_RE, oldText)) {
        violations.push({ file, line: number, message: `${file}:${number}: 新增 float64 金额字段（ADR-0020）：用 money.Amount` });
      }
    }
  }
  return violations;
}

// 与 enforce-design.mjs 的 applyBaseline 同一契约：按「文件 × 规则」吸收
// 允许数量，超出部分即失败。金额守卫只有一个规则，基线按文件记数即可。
export function applyBaseline(violations, baseline) {
  const files = baseline?.files || {};
  const grouped = new Map();
  for (const v of violations) {
    if (!grouped.has(v.file)) {
      grouped.set(v.file, []);
    }
    grouped.get(v.file).push(v);
  }
  const excess = [];
  let allowed = 0;
  const summary = {};
  for (const [file, group] of grouped) {
    const allowance = files[file]?.money ?? 0;
    const absorbed = Math.min(allowance, group.length);
    allowed += absorbed;
    excess.push(...group.slice(absorbed));
    if (group.length > allowance) {
      summary[file] = { count: group.length, allowance };
    }
  }
  return { excess, allowed, summary };
}

export function loadBaseline(filePath = BASELINE_PATH) {
  try {
    return JSON.parse(readFileSync(filePath, "utf8"));
  } catch {
    return { as_of: "missing", files: {} };
  }
}

// 基线机制自检：每次运行前用合成数据验证吸收/超额算术，防止守卫在
// 基线路径上静默变绿（MONEY-002 的牙齿不能只靠人记得）。
function runSelfTest() {
  const fake = (file, n) => Array.from({ length: n }, (_, i) => ({ file, line: i + 1, message: "fake" }));
  const baseline = { as_of: "self-test", files: { "a.go": { money: 3 } } };
  let ok = true;
  const exact = applyBaseline(fake("a.go", 3), baseline);
  ok &&= exact.excess.length === 0 && exact.allowed === 3;
  const over = applyBaseline(fake("a.go", 4), baseline);
  ok &&= over.excess.length === 1 && over.allowed === 3;
  const none = applyBaseline(fake("b.go", 1), baseline);
  ok &&= none.excess.length === 1 && none.allowed === 0;
  if (!ok) {
    console.error("enforce-money: 基线自检失败，守卫退出");
    process.exit(1);
  }
}

// 直接执行时才跑主流程；被 import 时只导出函数。
if (process.argv[1] === fileURLToPath(import.meta.url)) {
  runSelfTest();
  const violations = collectViolations();
  const baseline = loadBaseline();
  const { excess, allowed, summary } = applyBaseline(violations, baseline);

  const over = Object.entries(summary);
  if (over.length > 0) {
    console.error("MONEY-002 守卫拦截失败：");
    for (const [file, { count, allowance }] of over) {
      console.error(`  ${file}: 新增 ${count} 处，基线允许 ${allowance} 处`);
    }
    for (const v of excess) {
      console.error(`  ${v.message}`);
    }
    process.exit(1);
  }
  console.log(`enforce-money: ${changedFiles().length} 个变更 Go 文件，无新增 float64 金额字段${allowed > 0 ? `（${allowed} 处存量债务按基线放行，见 money-debt-baseline.json）` : ""}`);
}
