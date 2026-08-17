import { describe, expect, it } from "vitest";
import { fmtDate, fmtMoney } from "./format";

describe("fmtMoney", () => {
  // The point of the function: an amount measured in one currency must never be
  // shown as another. This is the defect it was written to remove.
  it("shows a foreign amount in its own currency, not in yuan", () => {
    const usd = fmtMoney(10000, "USD");
    expect(usd).not.toContain("¥");
    expect(usd).toContain("10,000.00");
  });

  it("distinguishes currencies that share an amount", () => {
    expect(fmtMoney(10000, "USD")).not.toBe(fmtMoney(10000, "CNY"));
  });

  it("accepts a lowercase or padded code", () => {
    expect(fmtMoney(1, " usd ")).toBe(fmtMoney(1, "USD"));
  });

  // Claiming a currency we were not told is exactly the failure being fixed, so
  // an absent code yields a bare number rather than a default symbol.
  it("omits the currency when none is known", () => {
    expect(fmtMoney(1234.5, undefined)).toBe("1,234.50");
    expect(fmtMoney(1234.5, "")).toBe("1,234.50");
  });

  // Intl separates the code from the number with a non-breaking space, so the
  // assertion is about the code being present, not about which space it uses.
  it("names an unrecognised code rather than dropping it", () => {
    expect(fmtMoney(1000, "XYZ").replace(/\s/g, " ")).toBe("XYZ 1,000.00");
  });

  it("renders a missing amount as a dash", () => {
    expect(fmtMoney(null, "CNY")).toBe("—");
  });

  it("renders negative amounts with accounting parentheses", () => {
    expect(fmtMoney(-1234.5, "USD")).toContain("(US$");
    expect(fmtMoney(-1234.5, "USD")).toContain("1,234.50)");
  });
});

describe("fmtDate", () => {
  it("drops the timestamp the API appends to a calendar date", () => {
    expect(fmtDate("2024-01-01T00:00:00Z")).toBe("2024-01-01");
  });

  it("keeps the calendar day regardless of the viewer's timezone", () => {
    // The trap this guards: `new Date("2024-01-01T00:00:00Z")` rendered in a
    // negative-offset zone yields 2023-12-31 — a lease that commences a day
    // early. Slicing the string cannot drift.
    const original = process.env.TZ;
    process.env.TZ = "America/Los_Angeles";
    expect(fmtDate("2024-01-01T00:00:00Z")).toBe("2024-01-01");
    process.env.TZ = original;
  });

  it("passes through a bare date and dashes an empty one", () => {
    expect(fmtDate("2024-06-30")).toBe("2024-06-30");
    expect(fmtDate(null)).toBe("—");
    expect(fmtDate("")).toBe("—");
  });

  it("returns an unrecognised value unchanged rather than inventing a date", () => {
    expect(fmtDate("not a date")).toBe("not a date");
  });
});
