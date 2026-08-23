/**
 * 文案层契约测试（spec: docs/specs/financial_model_workbench_refactor.md 追加决策①）。
 *
 * CONTRACT-001 惯例：前端持有的「键清单 / 示例数据」必须与后端源码机械一致，
 * 直接读取 Go 源文件断言，删掉后端一个假设键而不同步前端即红：
 *
 * 1. ASSUMPTION_HINTS 的每个键都能在后端引擎/模板源码里找到字面量 ——
 *    前端不得给用户翻译一个后端不认识的键（那是造假引导）。
 * 2. 示例假设 JSON 的键 ⊆ ASSUMPTION_HINTS —— 「填充示例」填出的每个键
 *    都有中文释义；语义化由释义承担，机器键必须保持引擎可读。
 * 3. 期初示例表单按 opening.go 的标准行清单自平衡（闸①）、mapping 目标
 *    都是标准行、闸③两侧逐合约一致 —— 示例必须是真能过三道闸的形状。
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";
import { ASSUMPTION_HINTS, ASSUMPTION_UNITS, EXAMPLE_ASSUMPTION_VALUES, EXAMPLE_OPENING_FORM } from "./hints";
import { parseAssumptions } from "./workbench";

const repoRoot = path.join(import.meta.dirname, "../../../");
const engineGo = readFileSync(path.join(repoRoot, "core-service/internal/finmodel/engine.go"), "utf8");
const defaultsGo = readFileSync(path.join(repoRoot, "core-service/internal/finmodel/template/defaults.go"), "utf8");
const openingGo = readFileSync(path.join(repoRoot, "core-service/internal/finmodel/opening/opening.go"), "utf8");
const backendSource = `${engineGo}\n${defaultsGo}`;

function quotedTokens(source: string): string[] {
  return Array.from(source.matchAll(/"([^"]+)"/g), (m) => m[1]);
}

/** Go 源码里有注释内不配对引号，全文正则会被吞；带引号子串检查足够。 */
function declaresQuoted(key: string): boolean {
  return backendSource.includes(`"${key}"`);
}

/** 从 opening.go 提取标准行清单：先解析 const 块的标识符→行名，再展开 var 字面量。 */
function lineList(varName: string): string[] {
  const constants = new Map<string, string>();
  for (const match of Array.from(openingGo.matchAll(/(Line\w+)\s*=\s*"([^"]+)"/g))) {
    constants.set(match[1], match[2]);
  }
  expect(constants.size).toBeGreaterThanOrEqual(12);
  const match = new RegExp(`var ${varName} = \\[\\]string\\{([^}]*)\\}`).exec(openingGo);
  expect(match, `opening.go defines ${varName}`).not.toBeNull();
  // var 体里是常量标识符（LineCash, ...），不是字符串字面量
  return match![1].split(",").map((id) => id.trim()).filter(Boolean).map((id) => {
    const resolved = constants.get(id);
    expect(resolved, `${varName} entry ${id} resolves to a standard line`).toBeTruthy();
    return resolved!;
  });
}

describe("ASSUMPTION_HINTS ↔ 后端假设键单一来源", () => {
  it("提示表里的每个键都在后端源码中有字面量", () => {
    for (const key of Object.keys(ASSUMPTION_HINTS)) {
      expect(declaresQuoted(key), `backend declares assumption key "${key}"`).toBe(true);
    }
  });

  it("示例假设的键 ⊆ 提示表（填充示例 = 每个键都有中文释义）", () => {
    for (const key of Object.keys(EXAMPLE_ASSUMPTION_VALUES)) {
      expect(ASSUMPTION_HINTS[key], `hint covers example key "${key}"`).toBeTruthy();
    }
  });

  it("F3-1：单位登记表与提示表键集一致——登记一个键必须同时有释义与单位", () => {
    expect(Object.keys(ASSUMPTION_UNITS).sort()).toEqual(Object.keys(ASSUMPTION_HINTS).sort());
    // 单位取值封闭：percent / days / multiple / amount，无第四种
    const legal = new Set(["percent", "days", "multiple", "amount"]);
    for (const [key, unit] of Object.entries(ASSUMPTION_UNITS)) {
      expect(legal.has(unit), `unit of "${key}" is a registered kind`).toBe(true);
    }
  });
});

describe("期初示例表单 ↔ opening.go 三道闸", () => {
  const parsedBalances = JSON.parse(EXAMPLE_OPENING_FORM.balancesJson) as Record<
    string,
    { lines: Record<string, number>; mapping: Record<string, string> }
  >;

  it("闸① 自平衡：示例余额按后端标准行清单计算 资产=负债+权益（±0.01）", () => {
    const assets = new Set(lineList("assetLines"));
    const liabilities = new Set(lineList("liabilityLines"));
    const equity = new Set(lineList("equityLines"));
    expect(parsedBalances).toEqual(
      expect.objectContaining({ [EXAMPLE_OPENING_FORM.periods[0]]: expect.any(Object) }),
    );
    for (const entry of Object.values(parsedBalances)) {
      let assetSum = 0;
      let liabEquitySum = 0;
      for (const [line, value] of Object.entries(entry.lines)) {
        if (assets.has(line)) assetSum += value;
        else if (liabilities.has(line) || equity.has(line)) liabEquitySum += value;
        else throw new Error(`示例用了非标准行 "${line}"——后端三道闸不认识`);
      }
      expect(Math.abs(assetSum - liabEquitySum)).toBeLessThanOrEqual(0.01);
    }
  });

  it("闸② 归并一致：mapping 目标都是标准行", () => {
    const standard = new Set([
      ...lineList("assetLines"),
      ...lineList("liabilityLines"),
      ...lineList("equityLines"),
    ]);
    for (const entry of Object.values(parsedBalances)) {
      for (const [, line] of Object.entries(entry.mapping)) {
        expect(standard.has(line), `mapping target "${line}" is a standard line`).toBe(true);
      }
    }
  });

  it("闸③ 租赁对照：lease_ref 与 engine 两侧逐合约完全一致且非空", () => {
    expect(EXAMPLE_OPENING_FORM.leaseRef.length).toBeGreaterThan(0);
    expect(EXAMPLE_OPENING_FORM.engine).toEqual(EXAMPLE_OPENING_FORM.leaseRef);
  });
});
