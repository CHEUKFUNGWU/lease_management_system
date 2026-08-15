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
  // Anchor the selector end so "-chip" never matches "-chips", and allow
  // selector groups ("a,\nb {") by running to the first brace.
  const match = new RegExp(`${escaped}(?![\\w-])[^{}]*\\{([^}]*)\\}`).exec(css);
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
    expect(ruleBody(".home-grid")).toMatch(/330px/);
    const narrow = css.slice(css.indexOf("@media (max-width: 1439px)"));
    expect(narrow.slice(0, 200)).toMatch(/minmax\(0,\s*1fr\)\s*330px/);
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

describe("FIX-005: right column is at least 320px (measured 330px keeps money values single-line)", () => {
  it("FIX-005: right column is at least 320px (measured 330px keeps money values single-line)", () => {
    expect(ruleBody(".home-grid")).toMatch(/330px/);
    expect(ruleBody("@media (max-width: 1439px)") || ruleBody(".home-grid")).toMatch(/330px/);
    // The money KPI value line must never reflow: the flex value row has no
    // wrap permission anywhere in the home right column.
    expect(css).toMatch(/\.home-right-kpis[\s\S]*?flex-direction:\s*column/);
  });
});

describe("FIX-007: the conversation column is the scroll container, composer outside it", () => {
  it("FIX-007: the conversation column is the scroll container, composer outside it", () => {
    const column = ruleBody(".home-chat-column");
    expect(column).toMatch(/height:\s*calc\(100dvh/);
    expect(column).toMatch(/position:\s*sticky/);
    const body = ruleBody(".home-chat-body");
    expect(body).toMatch(/overflow-y:\s*auto/);
    expect(body).toMatch(/flex:\s*1/);
    // composer is a sibling of the scrolling body, not inside it
    const bodyIndex = briefColumn.indexOf('className="home-chat-body"');
    const composerIndex = briefColumn.indexOf('className="home-chat-composer"');
    expect(bodyIndex).toBeGreaterThan(-1);
    expect(composerIndex).toBeGreaterThan(bodyIndex);
    expect(briefColumn.slice(composerIndex)).not.toContain('className="home-chat-body"');
  });
});

describe("FIX-008: chip radius has a unit, user row reverses, starters send directly", () => {
  it("FIX-008: chip radius has a unit, user row reverses, starters send directly", () => {
    expect(ruleBody(".home-chat-starter-chip")).toMatch(/border-radius:\s*(9999px|50px)/);
    expect(ruleBody(".home-msg.is-user")).toMatch(/row-reverse/);
    expect(briefColumn).toMatch(/sendText\(t\(key, language\)\)/);
    expect(briefColumn).not.toMatch(/askStarter/);
  });
});

describe("FIX-009: Spin-wrapped home columns restore their gaps", () => {
  it("FIX-009: Spin-wrapped home columns restore their gaps", () => {
    expect(ruleBody(".home-right-stack .ant-spin-container")).toMatch(/gap:\s*24px/);
    expect(ruleBody(".home-work-focus .ant-spin-container")).toMatch(/gap:\s*16px/);
    // The readiness card joins the AntD card language (border, no ring).
    expect(ruleBody(".home-readiness-card")).toMatch(/border:\s*1px solid var\(--border-default\)/);
    expect(ruleBody(".home-readiness-card")).toMatch(/box-shadow:\s*none/);
  });
});

describe("FIX-013: pending bubble shows the step scaffold, results stagger in", () => {
  it("FIX-013: pending bubble shows the step scaffold, results stagger in", () => {
    const steps = ruleBody(".home-chat-steps");
    expect(steps).toMatch(/flex-direction:\s*column/);
    expect(briefColumn).toContain("home-chat-step is-pending");
    expect(briefColumn).toContain("home-chat-step-mark");
    // The thinking copy is no longer the only pending expression.
    expect(briefColumn).toContain("home.chat_thinking");
    expect(css).toContain("@keyframes home-step-in");
  });
});

describe("FIX-005: right column is at least 320px (measured 330px keeps money values single-line)", () => {
  it("FIX-005: right column is at least 320px (measured 330px keeps money values single-line)", () => {
    expect(ruleBody(".home-grid")).toMatch(/330px/);
    expect(ruleBody("@media (max-width: 1439px)") || ruleBody(".home-grid")).toMatch(/330px/);
    // The money KPI value line must never reflow: the flex value row has no
    // wrap permission anywhere in the home right column.
    expect(css).toMatch(/\.home-right-kpis[\s\S]*?flex-direction:\s*column/);
  });
});

describe("FIX-005: right column is at least 320px (measured 330px keeps money values single-line)", () => {
  it("FIX-005: right column is at least 320px (measured 330px keeps money values single-line)", () => {
    expect(ruleBody(".home-grid")).toMatch(/330px/);
    expect(ruleBody("@media (max-width: 1439px)") || ruleBody(".home-grid")).toMatch(/330px/);
    // The money KPI value line must never reflow: the flex value row has no
    // wrap permission anywhere in the home right column.
    expect(css).toMatch(/\.home-right-kpis[\s\S]*?flex-direction:\s*column/);
  });
});

describe("FIX-007: the conversation column is the scroll container, composer outside it", () => {
  it("FIX-007: the conversation column is the scroll container, composer outside it", () => {
    const column = ruleBody(".home-chat-column");
    expect(column).toMatch(/height:\s*calc\(100dvh/);
    expect(column).toMatch(/position:\s*sticky/);
    const body = ruleBody(".home-chat-body");
    expect(body).toMatch(/overflow-y:\s*auto/);
    expect(body).toMatch(/flex:\s*1/);
    // composer is a sibling of the scrolling body, not inside it
    const bodyIndex = briefColumn.indexOf('className="home-chat-body"');
    const composerIndex = briefColumn.indexOf('className="home-chat-composer"');
    expect(bodyIndex).toBeGreaterThan(-1);
    expect(composerIndex).toBeGreaterThan(bodyIndex);
    expect(briefColumn.slice(composerIndex)).not.toContain('className="home-chat-body"');
  });
});

describe("FIX-008: chip radius has a unit, user row reverses, starters send directly", () => {
  it("FIX-008: chip radius has a unit, user row reverses, starters send directly", () => {
    expect(ruleBody(".home-chat-starter-chip")).toMatch(/border-radius:\s*(9999px|50px)/);
    expect(ruleBody(".home-msg.is-user")).toMatch(/row-reverse/);
    expect(briefColumn).toMatch(/sendText\(t\(key, language\)\)/);
    expect(briefColumn).not.toMatch(/askStarter/);
  });
});

describe("FIX-009: Spin-wrapped home columns restore their gaps", () => {
  it("FIX-009: Spin-wrapped home columns restore their gaps", () => {
    expect(ruleBody(".home-right-stack .ant-spin-container")).toMatch(/gap:\s*24px/);
    expect(ruleBody(".home-work-focus .ant-spin-container")).toMatch(/gap:\s*16px/);
    // The readiness card joins the AntD card language (border, no ring).
    expect(ruleBody(".home-readiness-card")).toMatch(/border:\s*1px solid var\(--border-default\)/);
    expect(ruleBody(".home-readiness-card")).toMatch(/box-shadow:\s*none/);
  });
});

describe("FIX-013: pending bubble shows the step scaffold, results stagger in", () => {
  it("FIX-013: pending bubble shows the step scaffold, results stagger in", () => {
    const steps = ruleBody(".home-chat-steps");
    expect(steps).toMatch(/flex-direction:\s*column/);
    expect(briefColumn).toContain("home-chat-step is-pending");
    expect(briefColumn).toContain("home-chat-step-mark");
    // The thinking copy is no longer the only pending expression.
    expect(briefColumn).toContain("home.chat_thinking");
    expect(css).toContain("@keyframes home-step-in");
  });
});

