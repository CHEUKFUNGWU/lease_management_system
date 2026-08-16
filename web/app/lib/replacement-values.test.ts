import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

// STY-006: STY-005 replaced inline styles with classes; FIX-034 backfilled
// the rules. This pins the three replacement points whose rule values did
// NOT match the inline values they replaced (peer table 8px, scenario
// loading 160px, scenario small gap 12px) — each was grouped into a shared
// rule with a different value. Guard-rule shape follows kpi-card-height:
// exact rule body, no cross-rule regex.
const root = join(import.meta.dirname, "..");
const css = readFileSync(join(root, "globals.css"), "utf8");

function rule(selector: string): string | null {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = new RegExp(`${escaped}\\s*\\{([^}]*)\\}`).exec(css);
  return match ? match[1] : null;
}

describe("STY-006 replacement values match the inline styles they replaced", () => {
  it("keeps .store360-peer-table at its original 8px top gap", () => {
    const body = rule(".store360-peer-table");
    expect(body, "rule exists").not.toBeNull();
    expect(body).toMatch(/margin-top:\s*8px/);
    expect(body).not.toMatch(/margin-top:\s*16px/);
  });

  it("keeps .scenario-loading-block at its original 160px height", () => {
    const body = rule(".scenario-loading-block");
    expect(body, "rule exists").not.toBeNull();
    expect(body).toMatch(/min-height:\s*160px/);
    expect(body).not.toMatch(/min-height:\s*220px/);
  });

  it("keeps .scenario-block-gap-sm at its original 12px top gap", () => {
    const body = rule(".scenario-block-gap-sm");
    expect(body, "rule exists").not.toBeNull();
    expect(body).toMatch(/margin-top:\s*12px/);
    expect(body).not.toMatch(/margin-top:\s*8px/);
  });

  it("does not re-merge the three classes into the shared 16px group", () => {
    const group = rule(
      ".pulse-block-gap,\n.store360-block-gap,\n.scenario-block-gap,\n.contract-block-gap",
    );
    expect(group).not.toBeNull();
    expect(group).toMatch(/margin-top:\s*16px/);
    expect(group).not.toContain("store360-peer-table");
    expect(group).not.toContain("scenario-loading-block");
    expect(group).not.toContain("scenario-block-gap-sm");
  });
});
