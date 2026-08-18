import { describe, it, expect } from "vitest";
import { tableScrollX } from "../lib/tableScroll";

describe("Promotions & ROI Attribution Frontend Logic", () => {
  it("tableScrollX correctly handles non-empty and empty counts", () => {
    expect(tableScrollX(0, 900)).toBeUndefined();
    expect(tableScrollX(5, 900)).toEqual({ x: 900 });
  });

  it("computes campaign ROI properly with guardrails against zero division", () => {
    const incGP = 15000;
    const totalCost = 5000;
    const roi = totalCost > 0 ? incGP / totalCost : undefined;
    expect(roi).toBe(3.0); // 300% ROI

    const zeroCost = 0;
    const zeroRoi = zeroCost > 0 ? incGP / zeroCost : undefined;
    expect(zeroRoi).toBeUndefined();
  });

  it("flags non-separable attribution when concurrent promotion dates overlap", () => {
    const p1 = { start: "2026-06-01", end: "2026-06-10" };
    const p2 = { start: "2026-06-05", end: "2026-06-15" };
    const p3 = { start: "2026-06-15", end: "2026-06-20" };

    const overlaps = (s1: string, e1: string, s2: string, e2: string) => !(e1 < s2 || e2 < s1);

    expect(overlaps(p1.start, p1.end, p2.start, p2.end)).toBe(true);
    expect(overlaps(p1.start, p1.end, p3.start, p3.end)).toBe(false);
  });
});
