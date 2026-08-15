import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { tableScrollX } from "./tableScroll";

describe("tableScrollX (FIX-004)", () => {
  it("keeps the fixed scroll width when rows exist", () => {
    expect(tableScrollX(3, 900)).toEqual({ x: 900 });
    expect(tableScrollX(1, 1120)).toEqual({ x: 1120 });
  });

  it("drops the scroll area entirely for empty tables", () => {
    expect(tableScrollX(0, 900)).toBeUndefined();
    expect(tableScrollX(0, 1120)).toBeUndefined();
  });
});

describe("portfolio scroll contract (FIX-004)", () => {
  const page = readFileSync(join(import.meta.dirname, "..", "portfolio", "page.tsx"), "utf8");

  it("gates both portfolio tables on their row counts", () => {
    expect(page).toContain("tableScrollX(unitPriceRows.length, 900)");
    expect(page).toContain("tableScrollX(rows.length, 1120)");
  });

  it("leaves no unconditional fixed-width scroll behind in portfolio", () => {
    expect(page).not.toMatch(/scroll=\{\{\s*x:/);
  });
});

/**
 * FIX-033: FIX-004 built the helper but wired only portfolio, so every other
 * table kept its phantom scrollbar — the payment-schedule tab (0 rows) is
 * where it surfaced again. This walks the whole app so a new table cannot
 * quietly reintroduce it.
 *
 * A literal `scroll={{ x: N }}` is allowed only when the table sits behind a
 * row-count guard on the same line (`rows.length ? <Table …>`), the other
 * legitimate way of never rendering the empty case.
 */
describe("no phantom scrollbars anywhere (FIX-033)", () => {
  const appDir = join(import.meta.dirname, "..");

  function walk(dir: string): string[] {
    return readdirSync(dir).flatMap((entry) => {
      const full = join(dir, entry);
      if (entry === "node_modules" || entry === ".next") return [];
      if (statSync(full).isDirectory()) return walk(full);
      return entry.endsWith(".tsx") ? [full] : [];
    });
  }

  it("every fixed-width table either uses tableScrollX or is row-count gated", () => {
    const offenders: string[] = [];
    for (const file of walk(appDir)) {
      readFileSync(file, "utf8").split("\n").forEach((line, index) => {
        if (!/scroll=\{\{\s*x:\s*\d+/.test(line)) return;
        if (/\.length\s*\?/.test(line)) return;
        offenders.push(`${file.slice(appDir.length + 1)}:${index + 1}`);
      });
    }
    expect(offenders, `these tables render a scroll area when empty:\n${offenders.join("\n")}`).toEqual([]);
  });
});
