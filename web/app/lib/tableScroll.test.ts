import { readFileSync } from "node:fs";
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
