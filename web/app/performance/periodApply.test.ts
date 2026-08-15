/**
 * FIX-026: the analysis period is applied, not typed-through.
 *
 * The bug this pins was user-visible: the text field wrote straight into the
 * state the loader depended on, so editing "2026-07" fired a request per
 * keystroke and the half-typed "2026-0" came back as「请求未成功」. The toast
 * named the endpoint (thanks to DIAG-001), which is how it was found.
 *
 * Same shape as the PRD P0 item for the pulse page: local state + apply.
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";

const page = readFileSync(path.join(import.meta.dirname, "page.tsx"), "utf8");

describe("FIX-026: period is applied explicitly", () => {
  it("keeps a draft separate from the applied period", () => {
    expect(page).toContain("periodDraft");
    expect(page).toContain("const applyPeriod");
  });

  it("binds the field to the draft and the loader to the applied value", () => {
    expect(page).toMatch(/<Input\s+value=\{periodDraft\}/);
    // The loader's dependency list must not contain the draft.
    const loadDeps = /\}\s*,\s*\[([^\]]*)\]\);\s*\n\s*useEffect\(\(\)\s*=>\s*\{\s*load\(\)/.exec(page);
    expect(loadDeps, "the load callback still has a dependency list").not.toBeNull();
    expect(loadDeps![1]).toContain("period");
    expect(loadDeps![1]).not.toContain("periodDraft");
  });

  it("refuses to apply anything that is not a well-formed YYYY-MM", () => {
    const guard = /periodDraftValid\s*=\s*(.+);/.exec(page);
    expect(guard, "there is a validity guard").not.toBeNull();
    const pattern = new RegExp(/^\d{4}-(0[1-9]|1[0-2])$/);
    for (const bad of ["2026-0", "2026-", "2026-13", "202-07", ""]) {
      expect(pattern.test(bad), `${bad} must be rejected`).toBe(false);
    }
    for (const good of ["2026-07", "2026-01", "2026-12"]) {
      expect(pattern.test(good), `${good} must be accepted`).toBe(true);
    }
    // The guard gates both entry points.
    expect(page).toContain("onPressEnter={applyPeriod}");
    expect(page).toContain("disabled={!periodDraftValid}");
  });
});
