/**
 * Ch1 BG2：SVG 消毒器（纯函数，白名单制）。
 *
 * 三条硬约束（模块设计 D-B18/D-B19）：
 *   - 白名单是模块不变量：不做参数、不导出常量。把安全决策外推给调用方，
 *     第二个调用方一定会抄第一个的参数再加一项。
 *   - 不引入 DOMPurify / react-markdown：前端新增依赖为零。我们消毒的是
 *     「受控 SVG 子集」，不是任意 HTML——自写白名单攻击面小一个量级。
 *   - 出参 { svg, stripped }：stripped 是被剥离项清单（Story 9），空是常态，
 *     非空可被观察（剥离量异常 = 有人尝试注入）。
 */

export interface SanitizeResult {
  svg: string;
  stripped: string[];
}

/** 允许的 SVG 元素白名单（瀑布图渲染器只会产出这些）。 */
const ALLOWED_TAGS = new Set([
  "svg", "title", "desc", "defs", "g",
  "rect", "line", "circle", "ellipse", "path", "polygon", "polyline",
  "text", "tspan",
]);

/** 允许的属性白名单。事件属性、外部引用一律不在列。 */
// 判定用小写集合（SVG 属性大小写敏感，输出保留调用方的原始写法）。
const ALLOWED_ATTRS_LOWER = new Set([
  "x", "y", "x1", "y1", "x2", "y2", "cx", "cy", "r", "rx", "ry",
  "width", "height", "dx", "dy", "d", "points",
  "fill", "stroke", "stroke-width", "opacity", "fill-opacity",
  "text-anchor", "font-size", "font-family", "font-weight",
  "class", "id", "role", "xmlns", "viewbox", "transform",
]);
const ALLOWED_ATTRS = { has(name: string): boolean { return ALLOWED_ATTRS_LOWER.has(name.toLowerCase()); } };

/** 危险元素：整体连同内容一起剥除。 */
const STRIP_WITH_CONTENT = new Set(["script", "style", "foreignobject"]);

/** URL 属性只允许安全协议与内部锚点。 */
const SAFE_URL = /^(?:[#/]|https?:$)/i;

interface Token {
  kind: "text" | "open" | "close" | "comment" | "doctype";
  raw: string;
  name: string;
}

/** 极简分词器：只区分文本 / 开标签 / 闭标签 / 注释 / doctype。 */
function tokenize(raw: string): Token[] {
  const tokens: Token[] = [];
  let i = 0;
  while (i < raw.length) {
    const lt = raw.indexOf("<", i);
    if (lt < 0) {
      tokens.push({ kind: "text", raw: raw.slice(i), name: "" });
      break;
    }
    if (lt > i) tokens.push({ kind: "text", raw: raw.slice(i, lt), name: "" });
    if (raw.startsWith("<!--", lt)) {
      const end = raw.indexOf("-->", lt);
      const stop = end < 0 ? raw.length : end + 3;
      tokens.push({ kind: "comment", raw: raw.slice(lt, stop), name: "" });
      i = stop;
      continue;
    }
    if (/^<[!?]/.test(raw.slice(lt))) {
      const gt = raw.indexOf(">", lt);
      const stop = gt < 0 ? raw.length : gt + 1;
      tokens.push({ kind: "doctype", raw: raw.slice(lt, stop), name: "" });
      i = stop;
      continue;
    }
    const gt = raw.indexOf(">", lt);
    if (gt < 0) {
      tokens.push({ kind: "text", raw: raw.slice(lt), name: "" });
      break;
    }
    const inner = raw.slice(lt + 1, gt);
    const closing = inner.startsWith("/");
    const name = (closing ? inner.slice(1) : inner).trim().split(/\s+/)[0] ?? "";
    tokens.push({
      kind: closing ? "close" : "open",
      raw: raw.slice(lt, gt + 1),
      name: name.toLowerCase(),
    });
    i = gt + 1;
  }
  return tokens;
}

/** 属性级清洗：拆 key="value"，白名单外或含危险协议的一律丢弃并计数。 */
function sanitizeAttrs(
  tagOpen: string,
  tagName: string,
  stripped: string[],
): string | null {
  const attrPattern = /([^\s=/>]+)(?:\s*=\s*("[^"]*"|'[^']*'|[^\s"'>]+))?/g;
  let match: RegExpExecArray | null;
  const kept: string[] = [];
  while ((match = attrPattern.exec(tagOpen)) !== null) {
    const rawAttr = match[0];
    const rawName = match[1] ?? "";
    // SVG 属性大小写敏感（viewBox），白名单按小写判定、输出保留原样。
    const name = rawName.toLowerCase();
    const value = (match[2] ?? "").replace(/^["']|["']$/g, "");
    if (!rawName) continue;
    if (!ALLOWED_ATTRS.has(name)) {
      stripped.push(`<${tagName}> dropped attribute ${rawName}`);
      continue;
    }
    // URL 承载属性：javascript:/data: 等伪协议一律剥除。
    if (/^(href|xlink:href|src)$/.test(name)) {
      const candidate = value.trim();
      if (candidate.startsWith("#")) {
        kept.push(`${rawName}="${value.replace(/"/g, "&quot;")}"`);
      } else if (/^https?:\/\//i.test(candidate) || /^data:/i.test(candidate)) {
        stripped.push(`<${tagName}> dropped external reference ${name}="${value.slice(0, 40)}"`);
      } else if (SAFE_URL.test(candidate.split(":")[0] + ":")) {
        kept.push(`${rawName}="${value.replace(/"/g, "&quot;")}"`);
      } else {
        stripped.push(`<${tagName}> dropped attribute ${name} (unknown scheme)`);
      }
      continue;
    }
    // 白名单属性本身携带 url() 或 javascript: 的同样剥除。
    if (/url\s*\(/i.test(value)) {
      stripped.push(`<${tagName}> dropped ${rawName} css url() value: ${value.slice(0, 40)}`);
      continue;
    }
    if (/javascript:/i.test(value)) {
      stripped.push(`<${tagName}> dropped ${rawName} javascript: value`);
      continue;
    }
    kept.push(`${rawName}="${value.replace(/"/g, "&quot;")}"`);
  }
  if (kept.length === 0 && !/\S/.test(tagOpen)) return "";
  return kept.join(" ");
}

function reopenTag(name: string, sanitizedAttrs: string | null): string {
  return `<${name}${sanitizedAttrs ? " " + sanitizedAttrs : ""}>`;
}

/**
 * sanitizeSvg 消毒一段受信来源但不可信任内容的 SVG（D-B2 白名单制）。
 * 已知绕过向量由 sanitize-svg.test.ts 表驱动锁定；把 ALLOWED_TAGS 清空，
 * 那些测试必须全红。
 */
export function sanitizeSvg(raw: string): SanitizeResult {
  const stripped: string[] = [];
  const tokens = tokenize(raw);
  const out: string[] = [];
  // 剥除栈：进入危险元素后，其嵌套内容全部静默丢弃。
  const suppressStack: string[] = [];

  for (const token of tokens) {
    if (suppressStack.length > 0) {
      if (token.kind === "close" && token.name === suppressStack[suppressStack.length - 1]) {
        stripped.push(`removed <${token.name}> block with content`);
        suppressStack.pop();
      }
      continue;
    }
    switch (token.kind) {
      case "text":
        out.push(token.raw);
        break;
      case "comment":
      case "doctype":
        stripped.push(`removed ${token.kind === "comment" ? "comment" : token.raw.slice(0, 15)}`);
        break;
      case "open": {
        if (STRIP_WITH_CONTENT.has(token.name)) {
          if (!token.raw.endsWith("/>")) suppressStack.push(token.name);
          else stripped.push(`removed self-closing <${token.name}/>`);
          break;
        }
        if (!ALLOWED_TAGS.has(token.name)) {
          stripped.push(`dropped non-whitelisted element <${token.name}>`);
          break;
        }
        const inner = token.raw.slice(1, -1).replace(new RegExp("^" + token.name), "");
        const attrs = sanitizeAttrs(inner, token.name, stripped);
        if (attrs === null) {
          stripped.push(`dropped <${token.name}> all attributes unsafe`);
          break;
        }
        // 自闭合形态必须保留（<rect …/>），否则子树结构被破坏。
        const selfClosed = /\/>\s*$/.test(token.raw);
        out.push(selfClosed ? `<${token.name}${attrs ? " " + attrs : ""}/>` : reopenTag(token.name, attrs));
        break;
      }
      case "close":
        if (!ALLOWED_TAGS.has(token.name)) {
          stripped.push(`dropped non-whitelisted closing tag </${token.name}>`);
          break;
        }
        out.push(`</${token.name}>`);
        break;
    }
  }
  return { svg: out.join(""), stripped };
}
