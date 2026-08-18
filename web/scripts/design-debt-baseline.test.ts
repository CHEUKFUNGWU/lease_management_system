/**
 * design-debt-baseline.json 生效测试（T6b，DESIGN.md §14/§15）。
 *
 * 基线文件的职责：DESIGN.md §14 登记的存量违规按「文件 × 规则」记录
 * 允许数量。守卫对超出部分失败——把基线挪走、只记总量、或把数字写大，
 * 都会让新增违规重新溜过去。本测试锁三件事：
 *
 * 1. 基线文件格式合法（有日期、按文件按规则记数、数字非负整数）。
 * 2. 基线恰好覆盖守卫当前扫描出的全部违规：扫描结果超出基线（本测试
 *    用 applyBaseline 复算）必须为零条——基线被删、被改小、或新代码
 *    加了违规而没清债，本测试就红。基线写大了不会被本测试抓，
 *    但 §14 的记账表格与 CI 输出会暴露它。
 * 3. 超额检测本身：同组违规数超过 allowance 时，超出的条数进入 excess，
 *    吸收的条数计入 allowed。
 */
import { describe, expect, it } from "vitest";
import { readFileSync, existsSync } from "node:fs";
import path from "node:path";
import { applyBaseline, collectViolations, loadBaseline } from "./enforce-design.mjs";

const baselinePath = path.join(process.cwd(), "scripts/design-debt-baseline.json");

function fakeViolation(file: string, rule: string, n: number) {
  const list: { file: string; line: number; rule: string; message: string }[] = [];
  for (let i = 1; i <= n; i += 1) {
    list.push({ file, line: i, rule, message: `${file}:${i}: fake` });
  }
  return list;
}

describe("design-debt-baseline.json（T6b 存量债务显式记账）", () => {
  it("基线文件存在、带日期、按文件按规则记录非负整数数量", () => {
    expect(existsSync(baselinePath)).toBe(true);
    const baseline = JSON.parse(readFileSync(baselinePath, "utf8"));
    expect(typeof baseline.as_of).toBe("string");
    expect(baseline.as_of).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(baseline.files && typeof baseline.files === "object").toBe(true);
    for (const [file, rules] of Object.entries(baseline.files)) {
      expect(file.startsWith("web/")).toBe(true);
      expect(rules && typeof rules === "object").toBe(true);
      for (const [rule, count] of Object.entries(rules as Record<string, unknown>)) {
        expect(rule).toMatch(/^(13-[1-9]|go-timestamp)$/);
        expect(Number.isInteger(count)).toBe(true);
        expect(count as number).toBeGreaterThanOrEqual(0);
      }
    }
  });

  it("守卫扫描出的违规不超出基线——基线被删或债务上涨即红", () => {
    const violations = collectViolations();
    const baseline = loadBaseline();
    const { excess, allowed } = applyBaseline(violations, baseline);
    expect(excess).toEqual([]);
    expect(allowed).toBeGreaterThan(0); // 基线必须真的吸收了存量，而非空转
  });

  it("同组违规数超过 allowance 时，超出部分进 excess", () => {
    const baseline = { as_of: "2026-08-19", files: { "web/app/x.tsx": { "13-2": 3 } } };
    const r = applyBaseline(fakeViolation("web/app/x.tsx", "13-2", 5), baseline);
    expect(r.allowed).toBe(3);
    expect(r.excess).toHaveLength(2);
    expect(r.summary).toEqual({ "web/app/x.tsx 13-2": { count: 5, allowance: 3 } });
  });

  it("无基线条目的文件/规则零容忍：任何一条都进 excess", () => {
    const baseline = { as_of: "2026-08-19", files: { "web/app/x.tsx": { "13-2": 3 } } };
    const r = applyBaseline(fakeViolation("web/app/y.tsx", "13-4", 1), baseline);
    expect(r.excess).toHaveLength(1);
    expect(r.allowed).toBe(0);
  });

  it("违规数低于 allowance 时放行，不产生 excess", () => {
    const baseline = { as_of: "2026-08-19", files: { "web/app/x.tsx": { "13-7": 10 } } };
    const r = applyBaseline(fakeViolation("web/app/x.tsx", "13-7", 4), baseline);
    expect(r.excess).toEqual([]);
    expect(r.allowed).toBe(4);
  });

  it("基线文件缺失时零容忍（等价于全量收紧，而非静默放行）", () => {
    const r = applyBaseline(fakeViolation("web/app/x.tsx", "13-2", 1), { as_of: "missing", files: {} });
    expect(r.excess).toHaveLength(1);
  });
});
