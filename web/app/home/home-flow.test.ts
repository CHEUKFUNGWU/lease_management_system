/**
 * HOME 批页面接线断言（源码层）。
 *
 * B3: 中间栏按 canViewHomeBrief 分支 —— 有分析权限的渲染简报，
 *     仅会计权限的渲染工作队列，右栏对所有角色一致。
 * P1: BriefColumn 的产出（含 action proposal）流向右栏。
 * P2: 右栏的采纳回调只接页面层处理，页面层才调用既有 action API。
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";

const pageSource = readFileSync(path.join(__dirname, "../page.tsx"), "utf8");

describe("home page wiring (B3)", () => {
  it("branches the middle column on analysis permission", () => {
    expect(pageSource).toContain("canViewHomeBrief(user)");
    expect(pageSource).toContain("<BriefColumn token={token} language={language} onProposal={handleProposal} />");
    expect(pageSource).toContain("<WorkQueueFocus {...rightColumnProps} />");
  });

  it("keeps the right column identical for every role", () => {
    // The right column is rendered once, outside the role branch — the
    // ternary itself only chooses between BriefColumn and WorkQueueFocus.
    const branchStart = pageSource.indexOf("canViewHomeBrief(user)");
    const branchEnd = pageSource.indexOf("WorkQueueFocus", branchStart);
    const branchHead = pageSource.slice(branchStart, branchEnd);
    expect(branchHead).not.toContain("RightColumn");
    expect(branchHead).toContain("<BriefColumn");
    const branchTail = pageSource.slice(branchEnd, branchEnd + 80);
    expect(branchTail).toContain("WorkQueueFocus {...rightColumnProps} />");
  });
});

describe("home page wiring (HOME-003 P1/P2)", () => {
  it("settles brief and follow-up proposals into the right column", () => {
    expect(pageSource).toContain("onProposal={handleProposal}");
    expect(pageSource).toContain("proposals,");
    expect(pageSource).toContain("onAdoptProposal: handleAdoptProposal,");
    expect(pageSource).toContain("onModifyProposal: handleModifyProposal,");
    expect(pageSource).toContain("onRejectProposal: handleRejectProposal,");
  });

  it("deduplicates proposals by their run key", () => {
    expect(pageSource).toContain("current.some((existing) => existing.key === item.key)");
  });
});
