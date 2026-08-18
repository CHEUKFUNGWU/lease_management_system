import { describe, expect, it } from "vitest";
import { buildVersionTree, isValidStatusTransition, canFreeze, canPromoteToOfficial } from "./logic";
import type { FPnAPlanVersion } from "./types";

const makeVersion = (id: string, name: string, priorId?: string, status: FPnAPlanVersion["status"] = "draft", isOfficial = false): FPnAPlanVersion => ({
  id,
  name,
  version_type: "budget",
  scenario_type: "baseline",
  source: "manual",
  coverage_scope: {},
  currency: "CNY",
  as_of_period: "2026-01",
  from_period: "2026-01",
  to_period: "2026-12",
  prior_version_id: priorId,
  status,
  is_official: isOfficial,
  created_at: new Date(Date.now() + parseInt(id, 10) * 1000).toISOString(),
});

describe("FP&A Workbench Logic", () => {
  describe("buildVersionTree", () => {
    it("returns empty array for empty input", () => {
      expect(buildVersionTree([])).toEqual([]);
    });

    it("assembles linear lineage into parent-child hierarchy", () => {
      const v1 = makeVersion("1", "Budget v1");
      const v2 = makeVersion("2", "Budget v2", "1");
      const v3 = makeVersion("3", "Forecast Q3", "2");

      const tree = buildVersionTree([v3, v1, v2]);
      expect(tree.length).toBe(1);
      expect(tree[0].version.id).toBe("1");
      expect(tree[0].level).toBe(0);
      expect(tree[0].children.length).toBe(1);
      expect(tree[0].children[0].version.id).toBe("2");
      expect(tree[0].children[0].level).toBe(1);
      expect(tree[0].children[0].children.length).toBe(1);
      expect(tree[0].children[0].children[0].version.id).toBe("3");
      expect(tree[0].children[0].children[0].level).toBe(2);
    });

    it("handles multiple branches and roots", () => {
      const root1 = makeVersion("10", "2025 Budget");
      const root2 = makeVersion("20", "2026 Budget");
      const child2A = makeVersion("21", "2026 Budget v2", "20");
      const child2B = makeVersion("22", "2026 Forecast Q1", "20");

      const tree = buildVersionTree([root1, root2, child2A, child2B]);
      expect(tree.length).toBe(2);
      const root2Node = tree.find((t) => t.version.id === "20");
      expect(root2Node).toBeDefined();
      expect(root2Node?.children.length).toBe(2);
    });

    it("handles circular prior_version_id references without infinite loops", () => {
      const v1 = makeVersion("1", "V1", "2");
      const v2 = makeVersion("2", "V2", "1");

      const tree = buildVersionTree([v1, v2]);
      expect(tree.length).toBeGreaterThan(0);
    });
  });

  describe("isValidStatusTransition", () => {
    it("allows legal forward transitions", () => {
      expect(isValidStatusTransition("draft", "review")).toBe(true);
      expect(isValidStatusTransition("draft", "approved")).toBe(true);
      expect(isValidStatusTransition("draft", "official")).toBe(true);
      expect(isValidStatusTransition("review", "approved")).toBe(true);
      expect(isValidStatusTransition("approved", "official")).toBe(true);
      expect(isValidStatusTransition("official", "retired")).toBe(true);
    });

    it("disallows illegal backward transitions from official or retired", () => {
      expect(isValidStatusTransition("official", "draft")).toBe(false);
      expect(isValidStatusTransition("official", "review")).toBe(false);
      expect(isValidStatusTransition("official", "approved")).toBe(false);
      expect(isValidStatusTransition("retired", "draft")).toBe(false);
      expect(isValidStatusTransition("retired", "approved")).toBe(false);
    });
  });

  describe("action eligibility guards", () => {
    it("canFreeze returns true only for draft and review", () => {
      expect(canFreeze(makeVersion("1", "V1", undefined, "draft"))).toBe(true);
      expect(canFreeze(makeVersion("2", "V2", undefined, "review"))).toBe(true);
      expect(canFreeze(makeVersion("3", "V3", undefined, "approved"))).toBe(false);
      expect(canFreeze(makeVersion("4", "V4", undefined, "official"))).toBe(false);
    });

    it("canPromoteToOfficial returns false for already official or retired versions", () => {
      expect(canPromoteToOfficial(makeVersion("1", "V1", undefined, "approved", false))).toBe(true);
      expect(canPromoteToOfficial(makeVersion("2", "V2", undefined, "official", true))).toBe(false);
      expect(canPromoteToOfficial(makeVersion("3", "V3", undefined, "retired", false))).toBe(false);
    });
  });
});
