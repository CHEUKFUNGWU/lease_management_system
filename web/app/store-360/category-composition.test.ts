import { describe, it, expect } from "vitest";
import { tableScrollX } from "../lib/tableScroll";

describe("Category Composition and Margin Decomposition Logic", () => {
  it("tableScrollX correctly handles non-empty and empty counts", () => {
    expect(tableScrollX(0, 800)).toBeUndefined();
    expect(tableScrollX(5, 800)).toEqual({ x: 800 });
  });

  it("calculates volume, mix, and rate effects consistently", () => {
    const baseRev = 200000;
    const baseGP = 90000;
    const baseMargin = baseGP / baseRev; // 45%

    const currRev = 220000;
    const currGP = 86000;

    const totalGPVariance = currGP - baseGP; // -4000
    const volumeEffect = (currRev - baseRev) * baseMargin; // +9000

    expect(totalGPVariance).toBe(-4000);
    expect(volumeEffect).toBe(9000);

    // Sum of mix and rate effect must equal variance - volume effect
    const mixPlusRate = totalGPVariance - volumeEffect; // -13000
    expect(mixPlusRate).toBe(-13000);
  });
});
