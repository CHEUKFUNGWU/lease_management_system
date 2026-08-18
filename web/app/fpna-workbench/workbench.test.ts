import { describe, expect, it, vi, beforeEach } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";
import { buildVersionTree, isValidStatusTransition, canFreeze, canPromoteToOfficial } from "./logic";
import { performanceApi } from "../lib/api";
import type { FPnAPlanVersion } from "./types";

const repoRoot = path.join(import.meta.dirname, "../../../");
const pageSource = readFileSync(path.join(import.meta.dirname, "page.tsx"), "utf8");
const hookSource = readFileSync(path.join(import.meta.dirname, "useFPnAWorkbench.ts"), "utf8");
const appLayoutSource = readFileSync(path.join(repoRoot, "web/app/components/AppLayout.tsx"), "utf8");
const budgetPanelSource = readFileSync(path.join(repoRoot, "web/app/reports/components/BudgetVariancePanel.tsx"), "utf8");

describe("FP&A Workbench Module (N1 / F0)", () => {
  describe("Static Architectural & Seam Contracts", () => {
    it("AppLayout registers /fpna-workbench in navigation", () => {
      expect(appLayoutSource).toContain('"/fpna-workbench"');
      expect(appLayoutSource).toContain('"fpna-workbench"');
    });

    it("Reports BudgetVariancePanel cross-links to /fpna-workbench", () => {
      expect(budgetPanelSource).toContain('href="/fpna-workbench"');
      expect(budgetPanelSource).toContain('"budget.scope_notice_title"');
    });

    it("FPnA Workbench Page clarifies operating basis vs IFRS 16 lease budget", () => {
      expect(pageSource).toContain('href="/reports?tab=budget"');
      expect(pageSource).toContain('"fpna.operating_scope_badge"');
    });

    it("useFPnAWorkbench exposes exact snapshot and commands shape", () => {
      expect(hookSource).toContain("refreshVersions");
      expect(hookSource).toContain("createVersion");
      expect(hookSource).toContain("freezeVersion");
      expect(hookSource).toContain("compareVersions");
      expect(hookSource).toContain("updateDataQualityStatus");
      expect(hookSource).toContain("refreshDataQuality");
      expect(hookSource).toContain("refreshGovernance");
    });

    it("useFPnAWorkbench intercepts mixed currency errors and provides actionable guidance", () => {
      expect(hookSource).toContain("mixed currencies require exchange_rate_version");
      expect(hookSource).toContain("mixed_currency_guidance");
    });
  });

  describe("Version Lineage & State Machine", () => {
    const makeV = (id: string, name: string, priorId?: string, status: FPnAPlanVersion["status"] = "draft"): FPnAPlanVersion => ({
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
      is_official: status === "official",
      created_at: new Date().toISOString(),
    });

    it("builds multi-level lineage tree from flat version list", () => {
      const v1 = makeV("v1", "Budget v1");
      const v2 = makeV("v2", "Budget v2", "v1");
      const v3 = makeV("v3", "Forecast Q3", "v2");

      const tree = buildVersionTree([v3, v1, v2]);
      expect(tree.length).toBe(1);
      expect(tree[0].version.id).toBe("v1");
      expect(tree[0].children[0].version.id).toBe("v2");
      expect(tree[0].children[0].children[0].version.id).toBe("v3");
    });

    it("enforces immutable official status transition guard", () => {
      expect(canFreeze(makeV("v1", "Draft", undefined, "draft"))).toBe(true);
      expect(canFreeze(makeV("v2", "Official", undefined, "official"))).toBe(false);
      expect(canPromoteToOfficial(makeV("v1", "Draft", undefined, "draft"))).toBe(true);
      expect(canPromoteToOfficial(makeV("v2", "Official", undefined, "official"))).toBe(false);
      expect(isValidStatusTransition("official", "draft")).toBe(false);
    });
  });

  describe("API parameter binding", () => {
    beforeEach(() => {
      vi.restoreAllMocks();
      vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL) => ({
        ok: true,
        status: 200,
        json: async () => ({ url: String(input), data: [] }),
      })));
    });

    it("performanceApi.planVersions supports string or filter object", async () => {
      await performanceApi.planVersions("budget", "tok");
      let url = new URL(String(vi.mocked(fetch).mock.calls[0][0]));
      expect(url.searchParams.get("version_type")).toBe("budget");

      await performanceApi.planVersions({ version_type: "forecast", status: "draft", as_of_period: "2026-02" }, "tok");
      url = new URL(String(vi.mocked(fetch).mock.calls[1][0]));
      expect(url.searchParams.get("version_type")).toBe("forecast");
      expect(url.searchParams.get("status")).toBe("draft");
      expect(url.searchParams.get("as_of_period")).toBe("2026-02");
    });

    it("performanceApi.dataQuality supports severity filter", async () => {
      await performanceApi.dataQuality({ period: "2026-01", status: "open", severity: "high" }, "tok");
      const url = new URL(String(vi.mocked(fetch).mock.calls[0][0]));
      expect(url.searchParams.get("period")).toBe("2026-01");
      expect(url.searchParams.get("status")).toBe("open");
      expect(url.searchParams.get("severity")).toBe("high");
    });
  });
});
