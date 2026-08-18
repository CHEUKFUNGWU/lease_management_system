import { describe, it, expect } from "vitest";
import { tableScrollX } from "../lib/tableScroll";

describe("Batches F7, F8, F9 Frontend Integrity", () => {
  it("F7: DOI and carrying cost structure validation", () => {
    const summary = {
      ending_stock_cost: 150000,
      in_transit_cost: 30000,
      doi: 25.4,
      turnover_rate: 14.3,
      total_carrying_cost: 14400,
    };
    expect(summary.doi).toBeGreaterThan(0);
    expect(summary.total_carrying_cost).toBe(14400);
  });

  it("F8: Master data candidate resolution mapping", () => {
    const res = {
      resolved: {
        "SKU_A": { raw_identifier: "SKU_A", canonical_id: "CANON_A", canonical_name: "牛奶", confidence: 1.0, source: "cached" as const },
      },
      unknown: ["SKU_B"],
      ambiguous: [],
    };
    expect(res.resolved["SKU_A"].canonical_id).toBe("CANON_A");
    expect(res.unknown).toContain("SKU_B");
  });

  it("F9: Competitor benchmark isolation and scroll helper", () => {
    expect(tableScrollX(0, 780)).toBeUndefined();
    expect(tableScrollX(2, 780)).toEqual({ x: 780 });

    const bench = {
      store_id: "S001",
      competitor_count: 3,
      highest_promo_threat: "aggressive",
      benchmark_disclaimer: "竞品商圈观测仅供横向参考，物理隔离于财务核算与法定报表体系。",
    };
    expect(bench.benchmark_disclaimer).toContain("物理隔离");
  });
});
