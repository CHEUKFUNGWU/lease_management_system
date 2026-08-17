import { describe, expect, it } from "vitest";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import MarkdownText, { parseBlocks } from "./MarkdownText";

describe("parseBlocks", () => {
  it("reads a heading, a paragraph and a bullet list", () => {
    const blocks = parseBlocks("## 经营脉搏\n\n销售额 100。\n\n- 门店 A\n- 门店 B");
    expect(blocks).toEqual([
      { kind: "heading", level: 2, text: "经营脉搏" },
      { kind: "paragraph", text: "销售额 100。" },
      { kind: "list", ordered: false, items: ["门店 A", "门店 B"] },
    ]);
  });

  it("distinguishes ordered from unordered lists", () => {
    expect(parseBlocks("1. 第一\n2. 第二")).toEqual([
      { kind: "list", ordered: true, items: ["第一", "第二"] },
    ]);
  });

  it("reads a pipe table only when a divider row follows the header", () => {
    const table = parseBlocks("| 指标 | 值 |\n| --- | --- |\n| 销售额 | 100 |");
    expect(table).toEqual([
      { kind: "table", header: ["指标", "值"], rows: [["销售额", "100"]] },
    ]);
    // Without the divider the pipes are ordinary characters, not a table.
    const prose = parseBlocks("| 这不是表格 |");
    expect(prose[0].kind).toBe("paragraph");
  });

  it("keeps unrecognised syntax as literal text instead of dropping it", () => {
    const blocks = parseBlocks("> 引用块暂不支持");
    expect(blocks).toEqual([{ kind: "paragraph", text: "> 引用块暂不支持" }]);
  });
});

describe("MarkdownText", () => {
  it("renders bold and inline code as elements", () => {
    const html = renderToStaticMarkup(<MarkdownText content="毛利率 **31.02%**，来自 `retail-kpi-v1`。" />);
    expect(html).toContain("<strong>31.02%</strong>");
    expect(html).toContain('class="ai-md-code">retail-kpi-v1</code>');
  });

  it("renders a table with its header and body cells", () => {
    const html = renderToStaticMarkup(
      <MarkdownText content={"| 指标 | 值 |\n| --- | --- |\n| 销售额 | 100 |"} />,
    );
    expect(html.match(/<th>/g)).toHaveLength(2);
    expect(html.match(/<td>/g)).toHaveLength(2);
    expect(html).toContain("销售额");
  });

  it("never injects raw HTML from the message", () => {
    const html = renderToStaticMarkup(<MarkdownText content={'<img src=x onerror="alert(1)">'} />);
    // React escapes it, so the angle brackets survive as text and no <img>
    // element is ever created. This is the property that lets us skip a
    // sanitiser: the renderer has no path from message text to markup.
    expect(html).not.toContain("<img");
    expect(html).toContain("&lt;img src=x onerror=");
  });
});
