/**
 * enforce-design.mjs 守卫自身的规则体测试（P0-B 修补，UIUX 审查报告
 * 2026-08-21）。
 *
 * 背景：§13-2 的逐行正则要求 style={{…}} 同行闭合，多行展开的内联样式
 * （本仓主流写法）对守卫全盲——946→1032 的回潮正是从这条缝进来的。
 * 本文件锁定两个修补点：
 *   1. 多行 opener 块的静态属性计数（styleBlockStaticProps）
 *   2. §13-3 窄规则的匹配域（JS_HOVER_STYLE_RE：只拦改 .style 的 hover，
 *      不拦埋点等非样式用途）
 *
 * GUARD-001 自检：把任一断言对应的实现删掉或放宽（比如块计数改回只看
 * 单行、正则放行 .style 赋值），对应用例立即变红。
 */
import { describe, expect, it } from "vitest";
import { JS_HOVER_STYLE_RE, TAG_PRESET_COLOR_RE, staticStylePropCount, styleBlockStaticProps } from "./enforce-design.mjs";

describe("多行内联样式块计数（§13-2 盲区修补）", () => {
  const multilineBlock = [
    "<div",
    "  style={{",
    '    display: "flex",',
    "    gap: 12,",
    "  }}",
    ">",
  ];

  it("opener 行自身在单行正则下不可见（回归锚点：这正是漏网形态）", () => {
    expect(staticStylePropCount("style={{")).toBe(0);
    expect(staticStylePropCount(multilineBlock[2])).toBe(0);
  });

  it("整块收集后静态属性可数（2 个字面量属性）", () => {
    expect(styleBlockStaticProps(multilineBlock, 1)).toBe(2);
  });

  it("动态属性块为 0 —— §13-2 允许的运行时值不拦", () => {
    const dynamicBlock = [
      "  style={{",
      "    width: pct,",
      "    height: rowHeight,",
      "  }}",
    ];
    expect(styleBlockStaticProps(dynamicBlock, 0)).toBe(0);
  });

  it("混合块只计静态部分", () => {
    const mixed = [
      "  style={{",
      "    width: pct,",
      '    padding: "8px 16px",',
      "  }}",
    ];
    expect(styleBlockStaticProps(mixed, 0)).toBe(1);
  });

  it("跨行嵌套（媒体查询对象/嵌套花括号）不提前截断", () => {
    const nested = [
      "  style={{",
      '    boxShadow: "var(--shadow-static)",',
      "    ...(cond ? { borderRadius: 8 } : {}),",
      "  }}",
    ];
    expect(styleBlockStaticProps(nested, 0)).toBeGreaterThanOrEqual(1);
  });

  it("单行完整样式仍按原路径计数（不因修补回归）", () => {
    expect(staticStylePropCount(`style={{ display: "flex", gap: 12 }}`)).toBe(2);
  });
});

describe("§13-3 JS hover 改样式的窄匹配域", () => {
  it("命中：箭头函数直接改 currentTarget.style（本次修复前的真实形态）", () => {
    expect(JS_HOVER_STYLE_RE.test('onMouseEnter={(e) => (e.currentTarget.style.background = "var(--bg-inset)")}')).toBe(true);
    expect(JS_HOVER_STYLE_RE.test("onMouseLeave={(event) => event.currentTarget.style.opacity = \"1\"}")).toBe(true);
    expect(JS_HOVER_STYLE_RE.test("onMouseEnter={e => { e.currentTarget.style.background = x }}")).toBe(true);
  });

  it("放行：非样式用途的 mouse 处理器（埋点、聚焦逻辑）", () => {
    expect(JS_HOVER_STYLE_RE.test('onMouseEnter={() => track("hover_card")}')).toBe(false);
    expect(JS_HOVER_STYLE_RE.test("onMouseLeave={closeTooltip}")).toBe(false);
    expect(JS_HOVER_STYLE_RE.test("onMouseEnter={() => setHovered(id)}")).toBe(false);
  });

  it("放行：CSS 类名切换走 className，不属于本条", () => {
    expect(JS_HOVER_STYLE_RE.test('onMouseEnter={() => setHoverClass("is-hover")}')).toBe(false);
  });
});

describe("§13-5 AntD Tag 预设色的匹配域（UIUX 任务书 2026-08-26）", () => {
  it("命中：字面量预设色、表达式色、带其他属性的 Tag", () => {
    expect(TAG_PRESET_COLOR_RE.test('<Tag color="blue">{x}</Tag>')).toBe(true);
    expect(TAG_PRESET_COLOR_RE.test("<Tag color={conf.color}>{label}</Tag>")).toBe(true);
    expect(TAG_PRESET_COLOR_RE.test('<Tag color={colors[type] || "default"}>{type}</Tag>')).toBe(true);
    expect(TAG_PRESET_COLOR_RE.test('<Tag color="success" icon={<CheckCircleOutlined />}>ok</Tag>')).toBe(true);
    expect(TAG_PRESET_COLOR_RE.test('<Tag key={s} color="blue">{s}</Tag>')).toBe(true);
  });

  it("放行：无 color 的 Tag 与非 AntD 组件", () => {
    expect(TAG_PRESET_COLOR_RE.test("<Tag closable onClose={close}>x</Tag>")).toBe(false);
    expect(TAG_PRESET_COLOR_RE.test('<StatusTag kind="processing">x</StatusTag>')).toBe(false);
    expect(TAG_PRESET_COLOR_RE.test("<SeverityDot severity=\"high\" />")).toBe(false);
    // \b 防前缀误伤：Tagged / TagsList 不是 <Tag 元素
    expect(TAG_PRESET_COLOR_RE.test('<Tagged color="blue" />')).toBe(false);
    expect(TAG_PRESET_COLOR_RE.test('<TagsList color="blue" />')).toBe(false);
  });
});
