/**
 * CONTRACT-001: 后端能力清单 ↔ 前端选项清单必须机械一致。
 *
 * 每个「双清单」场景由本文件跨语言断言（前端 vitest 直接读取后端 Go 源码）：
 * 从后端白名单里删掉一个 code，对应的断言必须红。
 */
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";
import { PULSE_KPI_CODES } from "../operating-pulse/logic";
import { STORE360_CODES } from "../store-360/logic";

const repoRoot = path.join(import.meta.dirname, "../../../");
const pulseBackend = readFileSync(path.join(repoRoot, "core-service/internal/services/retailpulse/retail_pulse.go"), "utf8");
const kpiBackend = readFileSync(path.join(repoRoot, "core-service/internal/services/retailkpi/retail_kpi.go"), "utf8");
const plFlowBackend = readFileSync(path.join(repoRoot, "core-service/internal/services/retailstore360/pl_flow.go"), "utf8");
const plFlowPanel = readFileSync(path.join(import.meta.dirname, "../store-360/ProfitFlowPanel.tsx"), "utf8");
const ingestBackend = readFileSync(path.join(repoRoot, "core-service/internal/services/retailingest/retailingest.go"), "utf8");
const ingestPage = readFileSync(path.join(import.meta.dirname, "../retail-data-import/page.tsx"), "utf8");

function quotedTokens(source: string, pattern: RegExp): string[] {
  return Array.from(source.matchAll(pattern), (m) => m[1]);
}

describe("CONTRACT-001 code-list contracts", () => {
  it("经营脉搏趋势选项 ⊆ selectTrendKPIs 白名单", () => {
    const match = /selectTrendKPIs[\s\S]*?\[\]string\{([^}]*)\}/.exec(pulseBackend);
    expect(match, "selectTrendKPIs whitelist found").not.toBeNull();
    const whitelist = quotedTokens(match![1], /"([^"]+)"/g);
    for (const code of PULSE_KPI_CODES) {
      expect(whitelist, `backend whitelist covers ${code}`).toContain(code);
    }
  });

  it("门店 360 卡片选项 ⊆ retailkpi.Definitions 的 code 清单", () => {
    const definitions = quotedTokens(kpiBackend, /\{Code: "([^"]+)"/g);
    expect(definitions.length).toBeGreaterThan(10);
    for (const code of STORE360_CODES) {
      expect(definitions, `retailkpi defines ${code}`).toContain(code);
    }
  });

  it("桑基节点单一来源：前端不持有节点 key 清单", () => {
    // ProfitFlowPanel 只消费 flow.nodes；若前端出现后端节点的字面量 key，
    // 说明有人在前端复制了一份清单——那正是 CONTRACT-001 要拦的形状。
    for (const key of ["\"revenue\"", "\"labor\"", "\"rent\"", "\"non_lease\"", "\"contribution\""]) {
      expect(plFlowPanel, `panel does not hard-code node key ${key}`).not.toContain(key);
    }
    expect(plFlowPanel).toContain("flow.nodes.map");
  });

  it("pl_flow 后端节点 key 自洽（from/to 都指向已定义节点）", () => {
    const nodeKeys = quotedTokens(plFlowBackend, /\{Key: "([^"]+)"/g);
    expect(nodeKeys).toContain("revenue");
    expect(nodeKeys).toContain("contribution");
    const linkRefs = quotedTokens(plFlowBackend, /\{From: "([^"]+)"/g).concat(quotedTokens(plFlowBackend, /To: "([^"]+)"/g));
    for (const ref of linkRefs) {
      expect(nodeKeys, `link ref ${ref} has a node`).toContain(ref);
    }
  });

  it("导入页必填字段清单 = retailingest.RequiredFields（单一来源）", () => {
    // The Go module declares fields as constants; resolve FieldX -> value
    // first, then map the RequiredFields identifiers through them.
    const fieldConstants = new Map<string, string>();
    for (const match of Array.from(ingestBackend.matchAll(/(Field\w+)\s+=\s+"([^"]+)"/g))) {
      fieldConstants.set(match[1], match[2]);
    }
    expect(fieldConstants.size, "retailingest field constants found").toBeGreaterThan(10);
    const backendMatch = /RequiredFields = \[\]string\{([^}]*)\}/.exec(ingestBackend);
    expect(backendMatch, "retailingest RequiredFields found").not.toBeNull();
    const backendRequired = backendMatch![1].split(",").map((token) => fieldConstants.get(token.trim()) ?? "").filter(Boolean).sort();
    expect(backendRequired.length, "RequiredFields resolves to string values").toBe(4);
    const pageMatch = /REQUIRED_FIELDS = \[([^}]*)\]/.exec(ingestPage);
    expect(pageMatch, "import page REQUIRED_FIELDS found").not.toBeNull();
    const pageRequired = quotedTokens(pageMatch![1], /"([^"]+)"/g).sort();
    expect(pageRequired, "page required fields mirror the backend list").toEqual(backendRequired);
    // The full standard-field list comes from the preview response, never a
    // front-end copy — the optional fields must not appear as literals.
    expect(ingestPage).toContain("standard_fields.map");
    for (const optionalField of ["\"gross_profit\"", "\"area_sqm\"", "\"other_controllable_cost\""]) {
      expect(ingestPage, `page must not copy the backend field list (${optionalField})`).not.toContain(optionalField);
    }
  });
});
