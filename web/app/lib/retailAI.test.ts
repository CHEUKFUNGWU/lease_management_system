import { describe, expect, it } from "vitest";
import { retailAIHref, safeInternalAIURL } from "./retailAI";

describe("retail AI context links", () => {
  it("round-trips scoped simulated context and all seven assumptions", () => {
    const href = retailAIHref({
      page: "scenario-workbench", title: "情景工作台", asOf: "2026-06-14", windowDays: 14,
      classification: "simulated", datasetVersion: "planA-v1", sourceSystem: "retail_simulator",
      storeID: "11111111-1111-4111-8111-111111111111", horizonMonths: 6,
      assumptions: {
        revenue_change_pct: -5, gross_margin_rate_change_pp: 1, labor_cost_change_pct: -10,
        fixed_rent_change_pct: 0, variable_rent_rate_change_pp: 0, non_lease_cost_change_pct: -2,
        other_controllable_cost_change_pct: 0,
      },
    });
    const query = new URLSearchParams(href.split("?")[1]);
    expect(query.get("page")).toBe("scenario-workbench");
    expect(query.get("classification")).toBe("simulated");
    expect(query.get("dataset_version")).toBe("planA-v1");
    expect(query.get("store_id")).toContain("11111111");
    expect(query.get("labor_cost_change_pct")).toBe("-10");
  });

  it("keeps repeated store ids addressable without accepting tenant fields", () => {
    const href = retailAIHref({ page: "operating-pulse", title: "经营脉搏", storeIDs: ["s2", "s1"] });
    const query = new URLSearchParams(href.split("?")[1]);
    expect(query.getAll("store_ids")).toEqual(["s2", "s1"]);
    expect(href).not.toContain("legal_entity");
    expect(href).not.toContain("role");
  });

  it("only makes same-site evidence paths clickable", () => {
    expect(safeInternalAIURL("/store-360?store_id=1")).toBe("/store-360?store_id=1");
    expect(safeInternalAIURL("https://example.com")).toBeUndefined();
    expect(safeInternalAIURL("//example.com")).toBeUndefined();
    expect(safeInternalAIURL("javascript:alert(1)")).toBeUndefined();
    expect(safeInternalAIURL("/javascript:alert(1)")).toBeUndefined();
    expect(safeInternalAIURL("/evil-internal-route")).toBeUndefined();
    expect(safeInternalAIURL("/api/v1/retail/kpis/store-days?store_id=1")).toContain("/api/v1/retail/kpis/store-days");
  });
});
