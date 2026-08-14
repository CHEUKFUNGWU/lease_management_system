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

// 行级变更提取：只检查「新增行」，已存在于基线里的违规行不触发。
// 改过的文件里若有存量违规（906 处内联样式那一类），本守卫放行——
// 存量是阶段一的事，本守卫只防新增。
const untracked = new Set(
  execSync(`git ls-files --others --exclude-standard`, { cwd: root }).toString().split("\n").map((p) => p.trim()).filter(Boolean),
);

function changedLines(file) {
  if (untracked.has(file)) {
    // 未跟踪文件整文件都是新增，无旧行可比
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
    // hunk 内按序配对：第 i 条新增行对第 i 条删除行。跨 hunk 的行号
    // 偏移不影响配对，只有真正改写过的行才拿得到旧行内容。
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

// 「新增」的精确读法：违规模式必须不在旧行里才成立。改到一行含有
// 存量违规的行（906 处内联样式 / 142 处 !important 之一）不是新增，
// 行级 diff 必须结合旧行内容判断。
function isNewViolation(pattern, line, oldText) {
  return pattern.test(line) && !pattern.test(oldText);
}

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

// 内联样式对三个零售页豁免——它们整页都是存量内联样式（阶段四整体
// 整改），任何触碰都会被行级 diff 误判为「新增静态样式」，页面将无法
// 维护。与 CJK 豁免同一批文件、同一个理由：阶段四完成的那一天，
// 两个 Set 一起删。!important 与 fontWeight 检查对这三页照常生效。
const INLINE_STYLE_EXEMPT_PAGES = CJK_EXEMPT_PAGES;

// t() 词典文件、测试文件里的中文是内容本身；守卫脚本自身也不扫描。
const CJK_EXEMPT_SUFFIXES = [/\/lib\/i18n(\.\w+)*\.tsx?$/, /\.test\.(ts|tsx)$/, /\.spec\.(ts|tsx)$/];
const SELF_EXEMPT = ["web/scripts/enforce-design.mjs"];

// ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
// ┃ 徽标与元数据豁免 — 品牌图形不是文案，浏览器标题不是 UI 文案      ┃
// ┃ 「营」徽标是品牌 mark（继承旧 L16 的位置），没有可翻译的文案；    ┃
// ┃   它的内联样式是 UI-002 之前的存量，本票只允许改字重与字形。      ┃
// ┃ layout.tsx 的 metadata 是 Next 静态元数据；走 generateMetadata    ┃
// ┃   做 i18n 是独立改进，归 UIUX 阶段四。                            ┃
// ┃ 这里只允许「行模式/整文件元数据」级别的窄豁免；                   ┃
// ┃ 任何人拿它当整页文件的挡箭牌 = 绕过止血。                         ┃
// ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
const BRAND_BADGE_LINE = /aria-hidden="true"[^>]*>\u8425<\/span>/;
const METADATA_EXEMPT_FILES = new Set(["web/app/layout.tsx"]);

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
  const lines = changedLines(file);

  if (file.startsWith("web/")) {
    for (const { number, text: line, oldText } of lines) {
      if (isNewViolation(IMPORTANT_RE, line, oldText)) {
        fail(file, number, "新增 !important（DESIGN.md §13-1）：提高特异性或改 token");
      }
      // 「营」徽标只豁免内联样式一条（品牌 mark 的存量写法）；
      // !important 与 fontWeight 检查照常生效（上一批 Review §4）。
      const styleMatch = INLINE_STYLE_RE.exec(line);
      const styleExempt =
        BRAND_BADGE_LINE.test(line) || INLINE_STYLE_EXEMPT_PAGES.has(file);
      if (styleMatch && !styleExempt && isNewViolation(INLINE_STYLE_RE, line, oldText) && isStaticStyleObject(styleMatch[1])) {
        fail(file, number, "新增静态内联 style={{}}（DESIGN.md §13-2）：用类名 + CSS 变量");
      }
      if (isNewViolation(FONT_WEIGHT_RE, line, oldText)) {
        fail(file, number, "新增 fontWeight > 600（DESIGN.md §13-6）：用尺寸和字距做层级");
      }
    }

    if (file.startsWith("web/app/")) {
      const exempt = CJK_EXEMPT_PAGES.has(file) || METADATA_EXEMPT_FILES.has(file) || CJK_EXEMPT_SUFFIXES.some((re) => re.test(file));
      if (!exempt) {
        for (const { number, text: line, oldText } of lines) {
          const trimmed = line.trim();
          if (BRAND_BADGE_LINE.test(line)) {
            continue; // 「营」徽标：品牌 mark，见豁免名单
          }
          if (isNewViolation(CJK_RE, line, oldText) && !/t\(\s*['"]/.test(line) && !/^(\/\/|\/\*|\*)/.test(trimmed)) {
            fail(file, number, "新增硬编码中文（DESIGN.md §13-7）：文案走 t()，三种语言齐全");
          }
        }
      }
    }
  }

  if (file.endsWith("_test.go")) {
    for (const { number, text: line, oldText } of lines) {
      if (isNewViolation(HARDCODED_TIMESTAMP_RE, line, oldText)) {
        fail(
          file,
          number,
          "测试里硬编码绝对时间戳（挂钟越过即永久变红）：改用相对 NOW() + INTERVAL 的偏移",
        );
      }
    }
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
