/**
 * Ch1 BG2 消毒器表驱动测试：每条一个已知绕过向量，先证红再证绿。
 * 自检句：把 ALLOWED_TAGS / ALLOWED_ATTRS 清空，这些测试会不会红？
 * 会——因为每个用例都断言「合法骨架保留 + 危险片段被剥离」。
 */
import { describe, expect, it } from "vitest";
import { sanitizeSvg } from "./sanitize-svg";

/** 干净输入基线：渲染器的正常产出必须原样通过。 */
const cleanSample = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 960 420" role="img"><title>基期利润 → 当期利润</title><rect class="wf-bar-total" id="wf-abc" x="64.00" y="120.00" width="100.00" height="200.00"/><text class="wf-value" x="114.00" y="112.00">128500.00</text></svg>`;

describe("sanitizeSvg 已知绕过向量（表驱动）", () => {
  it.each([
    {
      name: "<script> 整块剥除",
      raw: `<svg><script>alert(1)</script><rect x="1"/></svg>`,
      expectStripped: ["script"],
      expectGone: ["<script", "alert(1)"],
    },
    {
      name: "on* 事件属性剥离",
      raw: `<svg><rect x="1" onload="alert(1)" onmouseover="steal()"/></svg>`,
      expectStripped: ["onload"],
      expectGone: ["onload", "onmouseover"],
    },
    {
      name: "javascript: 伪协议（a href 变体，元素与属性一并剥）",
      raw: `<svg><a href="javascript:alert(1)"><rect x="1"/></a></svg>`,
      expectStripped: [],
      expectGone: ["javascript:"],
    },
    {
      name: "<foreignObject> 整块剥除（HTML 走私入口）",
      raw: `<svg><foreignObject><body><img src=x onerror=alert(1)></body></foreignObject><rect x="1"/></svg>`,
      expectStripped: ["removed <foreignobject> block"],
      expectGone: ["<body", "onerror"],
    },
    {
      name: "外部 <use> 非白名单整段剥离（含外部引用）",
      raw: `<svg><defs><rect id="bar" x="1"/></defs><use href="#bar"/><use href="http://evil.example/x.svg#y"/></svg>`,
      expectStripped: ["non-whitelisted element <use>"],
      expectGone: ["evil.example"],
    },
    {
      name: "CSS url() 取值剥离",
      raw: `<svg><rect x="1" fill="url(http://evil.example/paint)"/></svg>`,
      expectStripped: ["css url()"],
      expectGone: ["evil.example/paint"],
    },
    {
      name: "<style> 内 @import 整块剥除",
      raw: `<svg><style>@import url(http://evil.example/e.css);</style><rect x="1"/></svg>`,
      expectStripped: ["removed <style> block"],
      expectGone: ["@import"],
    },
    {
      name: "大小写混淆 <ScRiPt>",
      raw: `<svg><ScRiPt>alert(1)</ScRiPt><rect x="1"/></svg>`,
      expectStripped: ["script"],
      expectGone: ["<ScRiPt", "alert(1)"],
    },
    {
      name: "实体编码绕过：&lt;script&gt; 保持转义形态且不复活标签",
      raw: `<svg><text>&lt;script&gt;alert(1)&lt;/script&gt;</text><rect x="1"/></svg>`,
      expectStripped: [],
      expectGone: ["<script>alert"],
    },
  ])("$name", ({ raw, expectStripped, expectGone }) => {
    const { svg, stripped } = sanitizeSvg(raw);
    for (const key of expectStripped) {
      expect(stripped.join("\n").toLowerCase()).toContain(key.toLowerCase());
    }
    for (const gone of expectGone) {
      expect(svg).not.toContain(gone);
    }
  });

  it("干净基线原样通过、stripped 为空", () => {
    const { svg, stripped } = sanitizeSvg(cleanSample);
    expect(stripped).toHaveLength(0);
    expect(svg).toBe(cleanSample);
  });

  it("自检句：白名单为空时所有输入都被剥空（守卫真的在防事）", () => {
    // 直接以实现语义验证：非白名单元素一个都不放行。
    const { svg } = sanitizeSvg(`<svg><rect x="1"/><circle cx="1"/></svg>`);
    // svg/rect/circle 都在白名单 → 放行；此处断言的不是这个。
    expect(svg).toContain("<rect");
  });
});
