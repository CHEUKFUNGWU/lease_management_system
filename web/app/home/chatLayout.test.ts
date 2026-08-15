/**
 * HOME-004 §4 版面规矩 L1–L8 的机械契约（源码层）。
 *
 * 这些断言钉住「规矩落在哪个类/哪个值上」——回归立刻红。观感（疏密、
 * 对齐、气质）由用户视觉确认，见交付报告。
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";

const css = readFileSync(path.join(import.meta.dirname, "../globals.css"), "utf8");
const briefColumn = readFileSync(path.join(import.meta.dirname, "BriefColumn.tsx"), "utf8");
const briefBand = readFileSync(path.join(import.meta.dirname, "BriefBand.tsx"), "utf8");
const rightColumn = readFileSync(path.join(import.meta.dirname, "RightColumn.tsx"), "utf8");
const homePage = readFileSync(path.join(import.meta.dirname, "../page.tsx"), "utf8");

function ruleBody(selector: string): string {
  const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&").replace(/\\/g, "\\");
  const match = new RegExp(`${escaped}\\s*\\{([^}]*)\\}`).exec(css);
  return match ? match[1] : "";
}

describe("HOME-004 layout contract (L1–L8)", () => {
  it("L1: conversation body is measure-limited to 68ch", () => {
    expect(ruleBody(".home-chat-messages")).toMatch(/max-width:\s*68ch/);
    expect(ruleBody(".home-chat-starters")).toMatch(/max-width:\s*68ch/);
  });

  it("L2: vertical rhythm uses 4px-base gaps on one flex rhythm", () => {
    expect(ruleBody(".home-chat-column")).toMatch(/gap:\s*16px/);
    expect(ruleBody(".home-chat-body")).toMatch(/gap:\s*12px/);
    expect(ruleBody(".home-chat-messages")).toMatch(/gap:\s*16px/);
    expect(ruleBody(".home-brief-band")).toMatch(/gap:\s*8px/);
  });

  it("L3: hierarchy is carried by size/spacing, weights stay 400/500/600", () => {
    const band = ruleBody(".home-brief-band-title");
    expect(band).toMatch(/font-size:\s*13px/);
    expect(band).toMatch(/font-weight:\s*600/);
    expect(css).not.toMatch(/font-weight:\s*(700|800|900)/);
  });

  it("L4: confidence badge sits on the tool-chip row in both renderers", () => {
    for (const [name, source] of [["band", briefBand], ["chat", briefColumn]] as const) {
      const metaBlock = source.slice(source.indexOf("ai-tool-row"));
      expect(metaBlock, `${name} has a tool row`).toContain("ToolChip");
      expect(metaBlock, `${name} badge on the same row`).toContain("ConfidenceBadge");
    }
  });

  it("L5: the trust bar renders grouped fields, not one text blob", () => {
    const trust = readFileSync(path.join(import.meta.dirname, "../components/DataTrustBar.tsx"), "utf8");
    expect(trust).toContain("data-trust-bar-classification");
    expect(trust).toContain("data-trust-bar-reason");
    expect(trust).toContain("data-trust-bar-detail");
  });

  it("L6: numeric slots are tabular-nums", () => {
    expect(ruleBody(".home-band-kpi-value")).toMatch(/font-variant-numeric:\s*tabular-nums/);
    expect(ruleBody(".home-band-kpi-change")).toMatch(/font-variant-numeric:\s*tabular-nums/);
    expect(ruleBody(".home-band-attention-signals")).toMatch(/font-variant-numeric:\s*tabular-nums/);
    expect(ruleBody(".home-brief-band-count")).toMatch(/font-variant-numeric:\s*tabular-nums/);
  });

  it("L7: empty proposals collapse to one quiet line, not a tall Empty", () => {
    expect(rightColumn).toContain("home-proposals-empty-line");
    // The old tall Empty block (class + art) is gone; only the line remains.
    expect(rightColumn).not.toMatch(/home-proposals-empty(?!-line)/);
    expect(rightColumn).not.toContain("Empty.PRESENTED_IMAGE_SIMPLE");
    expect(ruleBody(".home-proposals-empty-line")).toMatch(/font-size:\s*12px/);
  });

  it("L8: three columns — middle at least 640px at 1440, right stays usable", () => {
    expect(ruleBody(".home-grid")).toMatch(/minmax\(640px,\s*1fr\)/);
    expect(ruleBody(".home-grid")).toMatch(/300px/);
    const narrow = css.slice(css.indexOf("@media (max-width: 1439px)"));
    expect(narrow.slice(0, 200)).toMatch(/minmax\(0,\s*1fr\)\s*300px/);
  });

  it("§3: composer is pinned to the column bottom, starters reuse /ai-chat keys", () => {
    expect(briefColumn).toContain("home-chat-composer");
    expect(briefColumn).toContain("ai.chip_missing_dr");
    expect(briefColumn).toContain("scrollIntoView");
  });

  it("§5: role branching and right-column wiring unchanged", () => {
    expect(homePage).toContain("canViewHomeBrief(user)");
    expect(homePage).toContain("<BriefColumn token={token} language={language} onProposal={handleProposal} />");
    expect(homePage).toContain("<WorkQueueFocus {...rightColumnProps} />");
    expect(homePage).toContain("<RightColumn {...rightColumnProps} />");
  });
});
