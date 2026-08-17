/**
 * DARK-003: the contracts that keep dark mode rendering at all.
 *
 * DARK-001 shipped a dark palette whose 23 contrast pairs all measured >= 4.5:1
 * and still produced an unusable screen, because the pairs were computed from
 * token values rather than from what the browser rendered. Three separate
 * mechanisms broke, and each one is pinned here:
 *
 *   1. the theme changed on the client, so antd generated styles under one
 *      cache hash while the mounted elements kept another and every antd
 *      component matched no rule at all;
 *   2. antd paints primary-button text with `colorTextLightSolid`, which
 *      defaults to white — correct against a black primary, invisible against
 *      the dark theme's white one (measured 1.37:1);
 *   3. the login brand slab drew its surface from `--fg-primary` and its text
 *      from `--fg-inverse`, so the dark theme flipped a black slab white and
 *      then wrote near-black text on it (1.14:1).
 */
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { colors, darkColors } from "./tokens";
import { antdTheme, antdDarkTheme } from "./theme";

const appDir = join(import.meta.dirname, "..");
const layout = readFileSync(join(appDir, "layout.tsx"), "utf8");
const provider = readFileSync(join(appDir, "components", "ThemeProvider.tsx"), "utf8");
const css = readFileSync(join(appDir, "globals.css"), "utf8");

describe("DARK-003: the server decides the theme", () => {
  it("the layout reads the cookie and hands the provider an initial theme", () => {
    expect(layout).toContain("cookies()");
    expect(layout).toContain("THEME_COOKIE");
    expect(layout).toMatch(/data-theme=\{theme\}/);
    expect(layout).toContain("initialTheme={theme}");
  });

  it("the cookie name is defined outside the client module", () => {
    // A Server Component importing a value from a "use client" module gets a
    // client reference, not the string: cookies().get() silently missed.
    expect(layout).toMatch(/THEME_COOKIE[^;]*from "\.\/lib\/theme-cookie"/);
  });

  it("the provider holds no theme state — a change is a reload", () => {
    expect(provider).not.toMatch(/useState<AppTheme>/);
    expect(provider).toContain("window.location.reload()");
  });
});

describe("DARK-003: colour scheme and identity surfaces", () => {
  it("declares color-scheme in both themes so UA controls follow", () => {
    expect(css).toMatch(/:root[\s\S]*?color-scheme:\s*light/);
    expect(css).toMatch(/\[data-theme="dark"\][\s\S]*?color-scheme:\s*dark/);
  });

  it("pins the brand slab and its text across themes", () => {
    // An identity surface is not a foreground: it must not invert.
    expect(colors.background.brandSlab).toBe(darkColors.background.brandSlab);
    expect(colors.background.onBrandSlab).toBe(darkColors.background.onBrandSlab);
  });

  it("the login slab consumes the pinned tokens, not the theme-following ones", () => {
    const rule = /\.login-brand \{([^}]*)\}/.exec(css);
    expect(rule, "the login brand rule exists").not.toBeNull();
    expect(rule![1]).toContain("var(--bg-brand-slab)");
    expect(rule![1]).toContain("var(--fg-on-brand-slab)");
    expect(rule![1]).not.toContain("var(--fg-primary)");
    expect(rule![1]).not.toContain("var(--fg-inverse)");
  });
});

describe("DARK-003: text on a solid primary surface", () => {
  it("is the page canvas, so it inverts with the primary fill", () => {
    expect(antdTheme.token.colorTextLightSolid).toBe(colors.background.page);
    expect(antdDarkTheme.token.colorTextLightSolid).toBe(darkColors.background.page);
  });

  it("contrasts with the primary fill in both themes", () => {
    const luminance = (hex: string) => {
      const channels = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255);
      const [r, g, b] = channels.map((c) => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4));
      return 0.2126 * r + 0.7152 * g + 0.0722 * b;
    };
    const ratio = (a: string, b: string) => {
      const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x);
      return (hi + 0.05) / (lo + 0.05);
    };
    expect(ratio(colors.foreground.primary, colors.background.page)).toBeGreaterThanOrEqual(4.5);
    expect(ratio(darkColors.foreground.primary, darkColors.background.page)).toBeGreaterThanOrEqual(4.5);
  });
});
