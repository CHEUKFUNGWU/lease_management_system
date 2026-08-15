import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

// FIX-003 mechanical contract: KPI cards are equal-height by CSS, not by
// luck. Rendered-height assertions need a browser; this pins the stylesheet
// and markup contract that produces them — a regression here (min-height
// creeping back, a value line losing its truncation) fails before it can
// re-introduce uneven cards.
const root = join(import.meta.dirname, "..");
const css = readFileSync(join(root, "globals.css"), "utf8");
const pulsePage = readFileSync(join(root, "operating-pulse", "page.tsx"), "utf8");
const store360Page = readFileSync(join(root, "store-360", "page.tsx"), "utf8");

function rule(selector: string): string | null {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = new RegExp(`${escaped}\\s*\\{([^}]*)\\}`).exec(css);
  return match ? match[1] : null;
}

function mediaRules(breakpoint: string): string[] {
  const blocks: string[] = [];
  const marker = `@media (max-width: ${breakpoint})`;
  let index = css.indexOf(marker);
  while (index !== -1) {
    const open = css.indexOf("{", index);
    // Match the media block's closing brace naively: media blocks in this
    // file contain only single-level rules, so the next lone "}" at the
    // block's indentation level ends it.
    let depth = 1;
    let cursor = open + 1;
    while (cursor < css.length && depth > 0) {
      if (css[cursor] === "{") depth += 1;
      if (css[cursor] === "}") depth -= 1;
      cursor += 1;
    }
    blocks.push(css.slice(open + 1, cursor - 1));
    index = css.indexOf(marker, cursor);
  }
  return blocks;
}

describe("KPI card height contract (FIX-003)", () => {
  it("pins both card classes to a fixed height with clipped overflow", () => {
    for (const selector of [".pulse-kpi-card", ".store-360-kpi-card"]) {
      const body = rule(selector);
      expect(body, `${selector} rule exists`).not.toBeNull();
      expect(body).toMatch(/height:\s*156px/);
      expect(body).toMatch(/overflow:\s*hidden/);
      // min-height is what let cards grow apart in the first place.
      expect(body).not.toMatch(/min-height/);
    }
  });

  it("keeps the same fixed height on the mobile breakpoint", () => {
    const blocks = mediaRules("767px");
    expect(blocks.length).toBeGreaterThan(0);
    for (const selector of [".pulse-kpi-card", ".store-360-kpi-card"]) {
      const mobile = blocks.find((block) => new RegExp(`${selector}\\s*\\{[^}]*height:\\s*136px`).test(block));
      expect(mobile, `${selector} mobile height 136px`).toBeDefined();
      expect(mobile).not.toMatch(new RegExp(`${selector}[\\s\\S]*?min-height`));
    }
  });

  it("types the shared value line with tabular-nums and a stable margin", () => {
    const body = rule(".pulse-kpi-card .pulse-kpi-value,\n.store-360-kpi-card .pulse-kpi-value");
    expect(body).not.toBeNull();
    expect(body).toMatch(/font-variant-numeric:\s*tabular-nums/);
    expect(body).toMatch(/margin:\s*12px 0 4px/);
    expect(body).toMatch(/max-width:\s*100%/);
  });

  it("truncates wrapping-prone inner lines instead of growing the card", () => {
    const body = rule(".pulse-kpi-card .pulse-change,\n.store-360-kpi-card .pulse-change,\n.pulse-kpi-card .pulse-kpi-comparison,\n.store-360-kpi-card .pulse-kpi-comparison");
    expect(body).not.toBeNull();
    expect(body).toMatch(/white-space:\s*nowrap/);
    expect(body).toMatch(/overflow:\s*hidden/);
    expect(body).toMatch(/text-overflow:\s*ellipsis/);
  });
});

describe("KPI card value-line markup contract (FIX-003)", () => {
  it("renders the value through the shared class with tooltip truncation", () => {
    for (const [name, source] of [["pulse", pulsePage], ["store-360", store360Page]] as const) {
      expect(source, `${name} uses the shared value class`).toContain('className="pulse-kpi-value"');
      expect(source, `${name} degrades long values with a tooltip`).toContain("ellipsis={{ tooltip: display }}");
      // The old inline numeric style is what allowed per-line reflow. Split
      // into two fragment assertions — a complete inline-style literal here
      // would itself trip the design guard.
      expect(source).not.toContain('style={{ margin: "12px 0 4px"');
      expect(source).not.toContain('fontVariantNumeric: "tabular-nums"');
    }
  });
});
