/**
 * FIX-019: AntD line-height tokens must be ratios, not pixels.
 *
 * AntD generates runtime CSS from these tokens (line-height: 32 with no
 * unit is a multiplier, 32 × 24px = 768px — the whole-modal bug). The
 * defect is invisible to source scanning, so this pins the token values
 * themselves: every *LineHeight / lineHeight token handed to AntD must be
 * a small ratio, never a pixel size.
 */
import { describe, expect, it } from "vitest";
import { antdTheme } from "./theme";

describe("theme.ts AntD line-height tokens (FIX-019)", () => {
  it("Modal.titleLineHeight is a ratio (< 3), not the 32px pixel value", () => {
    const value = antdTheme.components.Modal.titleLineHeight;
    expect(typeof value).toBe("number");
    expect(value).toBeGreaterThan(1);
    expect(value).toBeLessThan(3);
    // 32px / 24px = 1.333… — the exact division result.
    expect(value).toBeCloseTo(32 / 24, 5);
  });

  it("every *LineHeight token the theme hands to AntD is a ratio", () => {
    const lineHeightTokens: Array<[string, unknown]> = [
      ["token.lineHeight", antdTheme.token.lineHeight],
      ["Modal.titleLineHeight", antdTheme.components.Modal.titleLineHeight],
    ];
    for (const [name, value] of lineHeightTokens) {
      expect(typeof value, `${name} is a number`).toBe("number");
      expect(value as number, `${name} is a ratio (< 3)`).toBeLessThan(3);
      expect(value as number, `${name} is > 1`).toBeGreaterThan(1);
    }
  });

  it("the global token keeps the body ratio (the file's correct example)", () => {
    // 22 / 14 — body line-height over body size, matching DESIGN.md 4.1.
    expect(antdTheme.token.lineHeight).toBeCloseTo(22 / 14, 5);
  });
});
