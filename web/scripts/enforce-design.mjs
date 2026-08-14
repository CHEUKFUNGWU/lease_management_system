#!/usr/bin/env node
/**
 * DESIGN.md §13 止血条款的 diff 拦截器（ENF-001）
 *
 * 只检查「相对基线变更过的文件」——全树扫描会让存量违规（142 处
 * !important、906 处内联样式、13 处字重、99 行硬编码中文）立即全红，
 * 变成没人能合的噪音。CI 里基线是 origin/main，本地是 main。
 *
 * 变更集合 = `git diff <base>`（工作树 vs 基线）+ 未跟踪文件，因此对
 * 未提交的新改动同样生效。
 */
import { execSync } from "node:child_process";
import { readFileSync } from "node:fs";
import path from "node:path";

const root = execSync("git rev-parse --show-toplevel").toString().trim();
const base = process.env.CI ? "origin/main" : "main";

const changedFiles = [
  ...execSync(`git diff --name-only ${base}`, { cwd: root }).toString().split("\n"),
  // 未跟踪的新文件 git diff 看不到，但同样受止血条款约束。
  ...execSync(`git ls-files --others --exclude-standard`, { cwd: root }).toString().split("\n"),
]
  .map((p) => p.trim())
  .filter(Boolean);

const violations = [];
function fail(file, line, message) {
  violations.push(`${file}:${line}: ${message}`);
}

// ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
// ┃ ⚠️  豁免名单 — 看着难受就对了，这里每一条都是欠债                ┃
// ┃ 三个零售页的硬编码中文（约 99 行）归 UIUX 改善方案阶段四整体      ┃
// ┃ 整改。阶段四完成的那一天，删掉整个 Set，而不是往里面加一行。      ┃
// ┃ 往这个名单里加页面 = 承认新页面不配被翻译、不配被维护。          ┃
// ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
const CJK_EXEMPT_PAGES = new Set([
  "web/app/operating-pulse/page.tsx",
  "web/app/store-360/page.tsx",
  "web/app/scenario-workbench/page.tsx",
]);

// t() 词典文件、测试文件里的中文是内容本身；守卫脚本自身也不扫描。
const CJK_EXEMPT_SUFFIXES = [/\/lib\/i18n(\.\w+)*\.tsx?$/, /\.test\.(ts|tsx)$/, /\.spec\.(ts|tsx)$/];
const SELF_EXEMPT = ["web/scripts/enforce-design.mjs"];

const INLINE_STYLE_RE = /style=\{\{([^{}]*)\}\}/;
const FONT_WEIGHT_RE = /fontWeight\s*:\s*(['"]?)(700|800|900)\1/;
const IMPORTANT_RE = /!important/;
const CJK_RE = /[\u4e00-\u9fff]/;
const HARDCODED_TIMESTAMP_RE = /TIMESTAMPTZ\s*'\s*20\d\d-\d\d-\d\d|TIMESTAMP\s*'\s*20\d\d-\d\d-\d\d/;

// 静态内联样式判定：`key: value` 中任一 value 以标识符/表达式开头
// （引号、数字、#、%、( 开头的都是字面量）即视为动态值，DESIGN.md
// §13-2 允许，不拦截。无冒号的展开对象也视为动态。单行启发式。
function isStaticStyleObject(inner) {
  for (const pair of inner.split(",")) {
    const colon = pair.indexOf(":");
    if (colon === -1) {
      return false;
    }
    const value = pair.slice(colon + 1).trim();
    if (/^[A-Za-z_$]/.test(value)) {
      return false;
    }
  }
  return true;
}

for (const file of changedFiles) {
  if (SELF_EXEMPT.includes(file)) {
    continue;
  }
  const absolute = path.join(root, file);
  let content;
  try {
    content = readFileSync(absolute, "utf8");
  } catch {
    continue; // 文件被删除，无需检查
  }
  const lines = content.split("\n");

  if (file.startsWith("web/")) {
    lines.forEach((line, index) => {
      const number = index + 1;
      if (IMPORTANT_RE.test(line)) {
        fail(file, number, "新增 !important（DESIGN.md §13-1）：提高特异性或改 token");
      }
      const styleMatch = INLINE_STYLE_RE.exec(line);
      if (styleMatch && isStaticStyleObject(styleMatch[1])) {
        fail(file, number, "新增静态内联 style={{}}（DESIGN.md §13-2）：用类名 + CSS 变量");
      }
      if (FONT_WEIGHT_RE.test(line)) {
        fail(file, number, "新增 fontWeight > 600（DESIGN.md §13-6）：用尺寸和字距做层级");
      }
    });

    if (file.startsWith("web/app/")) {
      const exempt = CJK_EXEMPT_PAGES.has(file) || CJK_EXEMPT_SUFFIXES.some((re) => re.test(file));
      if (!exempt) {
        lines.forEach((line, index) => {
          const trimmed = line.trim();
          if (CJK_RE.test(line) && !/t\(\s*['"]/.test(line) && !/^(\/\/|\/\*|\*)/.test(trimmed)) {
            fail(file, index + 1, "新增硬编码中文（DESIGN.md §13-7）：文案走 t()，三种语言齐全");
          }
        });
      }
    }
  }

  if (file.endsWith("_test.go")) {
    lines.forEach((line, index) => {
      if (HARDCODED_TIMESTAMP_RE.test(line)) {
        fail(
          file,
          index + 1,
          "测试里硬编码绝对时间戳（挂钟越过即永久变红）：改用相对 NOW() + INTERVAL 的偏移",
        );
      }
    });
  }
}

if (violations.length > 0) {
  console.error("DESIGN.md §13 止血拦截失败：");
  for (const v of violations) {
    console.error("  " + v);
  }
  process.exit(1);
}
console.log(`enforce-design: ${changedFiles.length} 个变更文件，无新增违规`);
