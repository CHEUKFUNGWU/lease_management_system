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
 *
 * 存量债务显式记账（T6b）：DESIGN.md §14 登记的违规由
 * `design-debt-baseline.json` 按「文件 × 规则」记录允许数量（带日期），
 * 守卫对超出基线数量的部分仍然失败——把基线挪走或只记总量都会彻底放行，
 * 记到每个文件的精确数量才能继续拦住新增（某文件从 40 涨到 41 就红）。
 */
import { execSync } from "node:child_process";
import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = execSync("git rev-parse --show-toplevel").toString().trim();
const defaultBase = process.env.CI ? "origin/main" : "main";
const BASELINE_PATH = path.join(root, "web/scripts/design-debt-baseline.json");

// ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
// ┃ 规则模式与豁免                                                    ┃
// ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
const INLINE_STYLE_RE = /style=\{\{([^{}]*)\}\}/;
const FONT_WEIGHT_RE = /fontWeight\s*:\s*(['"]?)(700|800|900)\1/;
const IMPORTANT_RE = /!important/;
const CJK_RE = /[\u4e00-\u9fff]/;
const HARDCODED_COLOR_RE = /#[0-9a-fA-F]{3,8}\b|rgba?\(/;
const BORDER_1PX_SOLID_RE = /border(?:-[a-z]+)?\s*:\s*["']?1px\s+solid/;
const HARDCODED_TIMESTAMP_RE = /TIMESTAMPTZ\s*'\s*20\d\d-\d\d-\d\d|TIMESTAMP\s*'\s*20\d\d-\d\d-\d\d/;

// t() 词典文件、测试文件里的中文是内容本身；守卫脚本自身也不扫描。
// I18N-001：三个零售页的硬编码中文已全部走 t()，CJK_EXEMPT_PAGES 与
// INLINE_STYLE_EXEMPT_PAGES 两个豁免名单已删除——计数守卫现在对这三页
// 的硬编码中文与静态内联样式同样生效（收紧，见 I18N-001 守卫 commit）。
// §13-4/§13-8 同样豁免测试文件：断言里写出的期望颜色是测试内容，不是
// UI 硬编码，与 CJK 豁免同一条理由。
const CJK_EXEMPT_SUFFIXES = [/\/lib\/i18n(\.\w+)*\.tsx?$/, /\.test\.(ts|tsx)$/, /\.spec\.(ts|tsx)$/];
// DESIGN.md §13-4 豁免：令牌定义本身与品牌图形常量（tokens.ts 是
// 颜色的单一真相源；BrandIcon 的图形值属于 brand.* 图形语义）。
const COLOR_EXEMPT_FILES = new Set(["web/app/design-system/tokens.ts", "web/app/components/BrandIcon.tsx"]);
const SELF_EXEMPT = ["web/scripts/enforce-design.mjs", "web/scripts/design-debt-baseline.json"];

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

// 行级变更提取：只检查「新增行」，已存在于基线里的违规行不触发。
// 改过的文件里若有存量违规（906 处内联样式那一类），本守卫放行——
// 存量是阶段一的事，本守卫只防新增。
function changedFiles(base) {
  return [
    ...execSync(`git diff --name-only ${base}`, { cwd: root }).toString().split("\n"),
    // 未跟踪的新文件 git diff 看不到，但同样受止血条款约束。
    ...execSync(`git ls-files --others --exclude-standard`, { cwd: root }).toString().split("\n"),
  ]
    .map((p) => p.trim())
    .filter(Boolean);
}

function changedLines(base, file) {
  const untracked = new Set(
    execSync(`git ls-files --others --exclude-standard`, { cwd: root }).toString().split("\n").map((p) => p.trim()).filter(Boolean),
  );
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

// 「新增」的精确读法：违规必须在**数量**上比旧行多才成立（ENF-003）。
// 上一版用有/无判定，语义漂成了「不得新增违规行」——已经违规的行可以
// 随意加码（往已有 !important 的行再加一个、把内联样式从 2 个属性扩写
// 到 5 个都被放行）。计数法两个都拦：改到含存量违规的行不误报（数量
// 不变），但任何加码都会让数量上升。
function violationCount(pattern, text) {
  return (text.match(new RegExp(pattern.source, pattern.flags + "g")) || []).length;
}

function isNewViolation(pattern, line, oldText) {
  return violationCount(pattern, line) > violationCount(pattern, oldText);
}

// 静态内联样式按**属性个数**比，不是按 style={{ 出现次数（ENF-003）：
// 每个 `key: value` 里 value 以标识符/表达式开头的（引号、数字、#、%、
// ( 开头的都是字面量）视为动态值，DESIGN.md §13-2 允许，不计数；无
// 冒号的展开对象也视为动态。既有 2 个静态属性扩写到 5 个，新行计数
// 5 > 旧行 2，拦截。
export function staticStylePropCount(text) {
  let total = 0;
  const re = new RegExp(INLINE_STYLE_RE.source, "g");
  let match;
  while ((match = re.exec(text)) !== null) {
    for (const pair of match[1].split(",")) {
      const colon = pair.indexOf(":");
      if (colon === -1) {
        continue;
      }
      const value = pair.slice(colon + 1).trim();
      if (/^[A-Za-z_$]/.test(value)) {
        continue;
      }
      total += 1;
    }
  }
  return total;
}

function isNewStaticStyle(line, oldText) {
  return staticStylePropCount(line) > staticStylePropCount(oldText);
}

// 多行内联样式盲区修补（UIUX 审查报告 2026-08-21 P0-B）：INLINE_STYLE_RE
// 要求 `style={{…}}` 同行闭合，而本仓主流写法是 opener 行只写 `style={{`、
// 属性在后续行——逐行扫描对这类新增全盲（实测 1032 处内联样式中 103 处
// 属此形态，且 ai-chat 等重灾区恰以这种写法扩张）。对新增的 opener 行，
// 向前收集到花括号配平再数静态属性；旧行已含 style={{ 视为改写存量
// （ENF-003：数量未增不算新增），动态属性块为 0 放行。
const MULTILINE_STYLE_OPENER_RE = /style=\{\{\s*$/;
const BLOCK_SCAN_LIMIT = 120;

export function styleBlockStaticProps(fileLines, openerIdx) {
  let depth = 0;
  const collected = [];
  for (let i = openerIdx; i < Math.min(fileLines.length, openerIdx + BLOCK_SCAN_LIMIT); i += 1) {
    const text = fileLines[i];
    collected.push(text);
    for (const ch of text) {
      if (ch === "{") depth += 1;
      if (ch === "}") depth -= 1;
    }
    if (depth <= 0 && i > openerIdx) {
      break;
    }
  }
  const text = collected.join("\n");
  const start = text.indexOf("{{") + 2;
  const end = text.lastIndexOf("}}");
  if (start < 2 || end <= start) {
    return 0;
  }
  const inner = text.slice(start, end);
  // 顶层逗号切分属性（嵌套对象/三元里的逗号不计），再逐属性找「深度 0
  // 的冒号」判定静态性。条件展开 `...(x ? { a: 1 } : {})` 的内部属性不追
  // ——与旧行为一致（单行正则同样看不见嵌套花括号内层），宁可少计不可误伤。
  let level = 0;
  const props = [];
  let current = "";
  for (const ch of inner) {
    if ("{([".includes(ch)) level += 1;
    if ("})]".includes(ch)) level -= 1;
    if (ch === "," && level === 0) {
      props.push(current);
      current = "";
      continue;
    }
    current += ch;
  }
  props.push(current);

  let total = 0;
  for (const prop of props) {
    let propLevel = 0;
    let colonAt = -1;
    for (let i = 0; i < prop.length; i += 1) {
      const ch = prop[i];
      if ("{([".includes(ch)) propLevel += 1;
      else if ("})]".includes(ch)) propLevel -= 1;
      else if (ch === ":" && propLevel === 0) {
        colonAt = i;
        break;
      }
    }
    if (colonAt === -1) continue;
    const value = prop.slice(colonAt + 1).trim();
    if (!value || /^[A-Za-z_$]/.test(value)) continue;
    total += 1;
  }
  return total;
}

// §13-3 的窄实现（此前整条规则因「纯正则误报率高」暂缓）：只拦「事件处
// 理器里直接改 .style」这一种形态，即键盘 focus 拿不到同款反馈的那类。
// onMouseEnter 用于埋点/聚焦等非样式用途不匹配，保持放行。
export const JS_HOVER_STYLE_RE = /onMouse(?:Enter|Leave)=\{?\(?(?:e|event)\)?\s*=>.*\.style\.[A-Za-z]+\s*=/;

// ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
// ┃ 扫描：收集每条违规为 { file, line, rule, message }                ┃
// ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
export function collectViolations(base = defaultBase) {
  const violations = [];
  const fail = (file, line, rule, message) => {
    violations.push({ file, line, rule, message: `${file}:${line}: ${message}` });
  };

  for (const file of changedFiles(base)) {
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
    const lines = changedLines(base, file);

    if (file.startsWith("web/")) {
      const fileLines = content.split("\n");
      // 测试文件里的违规样例是夹具内容，不是 UI（与 §13-4/13-8 豁免
      // 测试文件同一条理由）；13-2/13-3 同样放行。
      const isTestFixture = CJK_EXEMPT_SUFFIXES.some((re) => re.test(file));
      for (const { number, text: line, oldText } of lines) {
        // 测试文件断言里写出的 !important 是被测内容（如 design-tree-budget
        // 对 globals.css 的计数），不是 UI 样式，与 §13-4/§13-8 同一豁免理由。
        if (!isTestFixture && isNewViolation(IMPORTANT_RE, line, oldText)) {
          fail(file, number, "13-1", "新增 !important（DESIGN.md §13-1）：提高特异性或改 token");
        }
        // 「营」徽标只豁免内联样式一条（品牌 mark 的存量写法）；
        // !important 与 fontWeight 检查照常生效（上一批 Review §4）。
        const styleExempt = BRAND_BADGE_LINE.test(line) || isTestFixture;
        if (!styleExempt && isNewStaticStyle(line, oldText)) {
          fail(file, number, "13-2", "新增静态内联 style={{}}（DESIGN.md §13-2）：用类名 + CSS 变量");
        }
        // 多行 opener：本行只写 `style={{`、属性在后续行，单行正则看不见。
        if (!styleExempt && MULTILINE_STYLE_OPENER_RE.test(line) && !/style=\{\{/.test(oldText)) {
          if (styleBlockStaticProps(fileLines, number - 1) > 0) {
            fail(file, number, "13-2", "新增多行静态内联 style 块（DESIGN.md §13-2）：用类名 + CSS 变量");
          }
        }
        if (!isTestFixture && isNewViolation(JS_HOVER_STYLE_RE, line, oldText)) {
          fail(file, number, "13-3", "JS hover 改样式（DESIGN.md §13-3）：用 CSS :hover, :focus-visible");
        }
        if (isNewViolation(FONT_WEIGHT_RE, line, oldText)) {
          fail(file, number, "13-6", "新增 fontWeight > 600（DESIGN.md §13-6）：用尺寸和字距做层级");
        }
      }
    }

    if (file.startsWith("web/app/")) {
      const exempt = METADATA_EXEMPT_FILES.has(file) || CJK_EXEMPT_SUFFIXES.some((re) => re.test(file));
      for (const { number, text: line, oldText } of lines) {
        const trimmed = line.trim();
        if (BRAND_BADGE_LINE.test(line)) {
          continue; // 「营」徽标：品牌 mark，见豁免名单
        }
        if (!exempt && isNewViolation(CJK_RE, line, oldText) && !/t\(\s*['"]/.test(line) && !/^(\/\/|\/\*|\*)/.test(trimmed)) {
          fail(file, number, "13-7", "新增硬编码中文（DESIGN.md §13-7）：文案走 t()，三种语言齐全");
        }
        // §13-4 硬编码颜色（T5）：只在 .ts/.tsx 生效——tokens.ts 的
        // 十六进制是令牌本身，BrandIcon 是图形常量，两者豁免；
        // globals.css 的 :root 是 CSS 文件，本规则不扫。
        if (!COLOR_EXEMPT_FILES.has(file) && /\.(ts|tsx)$/.test(file) && !exempt && isNewViolation(HARDCODED_COLOR_RE, line, oldText)) {
          fail(file, number, "13-4", "新增硬编码颜色值（DESIGN.md §13-4）：用 token，不要写 #hex / rgb() / rgba()");
        }
        // §13-8 字面量边框（T5）：DESIGN.md §6 要求走 --shadow-* 环形阴影。
        if (/\.(ts|tsx)$/.test(file) && !exempt && isNewViolation(BORDER_1PX_SOLID_RE, line, oldText)) {
          fail(file, number, "13-8", "新增 border: 1px solid（DESIGN.md §13-8）：用 --shadow-* 环形阴影");
        }
      }
    }

    if (file.endsWith("_test.go")) {
      for (const { number, text: line, oldText } of lines) {
        if (isNewViolation(HARDCODED_TIMESTAMP_RE, line, oldText)) {
          fail(
            file,
            number,
            "go-timestamp",
            "测试里硬编码绝对时间戳（挂钟越过即永久变红）：改用相对 NOW() + INTERVAL 的偏移",
          );
        }
      }
    }
  }
  return violations;
}

// ┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
// ┃ 存量债务基线（T6b）：按「文件 × 规则」记录允许数量，超出即失败    ┃
// ┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
export function loadBaseline(filePath = BASELINE_PATH) {
  try {
    return JSON.parse(readFileSync(filePath, "utf8"));
  } catch {
    // 基线文件缺失 = 不允许任何存量债务，等价于全量收紧。守卫自己的
    // 报错信息会说明这一点，而不是静默跳过。
    return { as_of: "missing", files: {} };
  }
}

// violations: collectViolations 的输出。返回 { excess, allowed, summary }：
// 每个「文件 × 规则」组先吸收基线允许的数量，超出部分进入 excess
// （每条都失败）；allowed 是被吸收的条数（只出现在汇总信息里）。
export function applyBaseline(violations, baseline) {
  const files = baseline?.files || {};
  const grouped = new Map(); // "file\0rule" -> violations[]
  for (const v of violations) {
    const key = `${v.file}\u0000${v.rule}`;
    if (!grouped.has(key)) {
      grouped.set(key, []);
    }
    grouped.get(key).push(v);
  }
  const excess = [];
  let allowed = 0;
  const summary = {};
  for (const [key, group] of grouped) {
    const [file, rule] = key.split("\u0000");
    const allowance = files[file]?.[rule] ?? 0;
    const absorbed = Math.min(allowance, group.length);
    allowed += absorbed;
    excess.push(...group.slice(absorbed));
    if (group.length > allowance) {
      summary[`${file} ${rule}`] = { count: group.length, allowance };
    }
  }
  return { excess, allowed, summary };
}

// 直接执行时才跑主流程；被测试 import 时只导出函数。
if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const violations = collectViolations();
  const baseline = loadBaseline();
  const { excess, allowed, summary } = applyBaseline(violations, baseline);

  const over = Object.entries(summary);
  if (over.length > 0) {
    console.error("DESIGN.md §13 止血拦截失败：");
    for (const [scope, { count, allowance }] of over) {
      console.error(`  ${scope}: 新增 ${count} 处，基线允许 ${allowance} 处`);
    }
    console.error("  超出基线的具体行：");
    for (const v of excess) {
      console.error(`  ${v.message}`);
    }
    process.exit(1);
  }
  console.log(`enforce-design: ${changedFiles(defaultBase).length} 个变更文件，无违规${allowed > 0 ? `（${allowed} 处存量债务按基线放行，见 design-debt-baseline.json）` : ""}`);
}
