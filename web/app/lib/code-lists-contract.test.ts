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
import { STORE_PNL_SECONDARY_COLUMNS } from "../store-pnl/options";
import {
  FIN_MODEL_RUN_STATUSES,
  FIN_MODEL_RUN_TIE_OUT_STATUSES,
  FIN_SAVED_VIEW_KINDS,
} from "../financial-model/enums";
import {
  VERSION_TYPES,
  SCENARIO_TYPES,
  PLAN_STATUSES,
  DQ_CATEGORIES,
  DQ_STATUSES,
} from "../fpna-workbench/types";
import { DRAFT_REVIEW_STATUSES, DRAFT_DATA_CLASSIFICATIONS } from "../contracts/drafts/enums";
import {
  SETTLEMENT_CATEGORIES,
  SETTLEMENT_RUN_STATUSES,
  ECOM_AD_BASIS,
} from "./api";

const repoRoot = path.join(import.meta.dirname, "../../../");
const pulseBackend = readFileSync(path.join(repoRoot, "core-service/internal/services/retailpulse/retail_pulse.go"), "utf8");
const kpiBackend = readFileSync(path.join(repoRoot, "core-service/internal/services/retailkpi/retail_kpi.go"), "utf8");
const plFlowBackend = readFileSync(path.join(repoRoot, "core-service/internal/services/retailstore360/pl_flow.go"), "utf8");
const plFlowPanel = readFileSync(path.join(import.meta.dirname, "../store-360/ProfitFlowPanel.tsx"), "utf8");
const ingestBackend = readFileSync(path.join(repoRoot, "core-service/internal/services/retailingest/retailingest.go"), "utf8");
const ingestPage = readFileSync(path.join(import.meta.dirname, "../retail-data-import/page.tsx"), "utf8");
const storepnlBackend = readFileSync(path.join(repoRoot, "core-service/internal/storepnl/project.go"), "utf8");
const sqlInit = readFileSync(path.join(repoRoot, "db/init/01_init.sql"), "utf8");
const settlementMatchBackend = readFileSync(path.join(repoRoot, "core-service/internal/services/settlement/match.go"), "utf8");
const draftReviewBackend = readFileSync(path.join(repoRoot, "core-service/internal/services/draftreview/service.go"), "utf8");

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

  it("store-pnl 对比列选项 ⊆ storepnl.ColumnRef 白名单", () => {
    const columnRefs = quotedTokens(storepnlBackend, /ColumnRef = "([^"]+)"/g);
    expect(columnRefs, "storepnl ColumnRef whitelist found").toContain("actual");
    for (const column of STORE_PNL_SECONDARY_COLUMNS) {
      expect(columnRefs, `backend ColumnRef covers ${column}`).toContain(column);
    }
  });

  it("fin_model_runs 状态/勾稽与 fin_saved_views.kind = DB CHECK 约束（单一来源）", () => {
    const statusMatch = /fin_model_runs[\s\S]*?CHECK\s*\(\s*status\s+IN\s*\(([^)]+)\)\s*\)/.exec(sqlInit);
    expect(statusMatch, "fin_model_runs status CHECK constraint found").not.toBeNull();
    const dbStatuses = quotedTokens(statusMatch![1], /'([^']+)'/g).sort();
    expect([...FIN_MODEL_RUN_STATUSES].sort()).toEqual(dbStatuses);

    const tieOutMatch = /fin_model_runs[\s\S]*?CHECK\s*\(\s*tie_out_status\s+IN\s*\(([^)]+)\)\s*\)/.exec(sqlInit);
    expect(tieOutMatch, "fin_model_runs tie_out_status CHECK constraint found").not.toBeNull();
    const dbTieOuts = quotedTokens(tieOutMatch![1], /'([^']+)'/g).sort();
    expect([...FIN_MODEL_RUN_TIE_OUT_STATUSES].sort()).toEqual(dbTieOuts);

    const dcMatch = /fin_model_runs[\s\S]*?CHECK\s*\(\s*data_classification\s+IN\s*\(([^)]+)\)\s*\)/.exec(sqlInit);
    expect(dcMatch, "fin_model_runs data_classification CHECK constraint found").not.toBeNull();
    const dbClassifications = quotedTokens(dcMatch![1], /'([^']+)'/g).sort();
    // 前端 run 分类下拉只有 production/simulated 两档，后端多一个 mixed —— 断言子集
    for (const value of ["production", "simulated"]) {
      expect(dbClassifications, `data_classification covers ${value}`).toContain(value);
    }

    const kindMatch = /fin_saved_views[\s\S]*?CHECK\s*\(\s*kind\s+IN\s*\(([^)]+)\)\s*\)/.exec(sqlInit);
    expect(kindMatch, "fin_saved_views kind CHECK constraint found").not.toBeNull();
    const dbKinds = quotedTokens(kindMatch![1], /'([^']+)'/g).sort();
    expect([...FIN_SAVED_VIEW_KINDS].sort()).toEqual(dbKinds);
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

  it("FP&A 计划版本与情景枚举 = DB CHECK 约束（单一来源）", () => {
    const versionMatch = /fpna_plan_versions[\s\S]*?CHECK\s*\(\s*version_type\s+IN\s*\(([^)]+)\)\s*\)/.exec(sqlInit);
    expect(versionMatch, "version_type CHECK constraint found").not.toBeNull();
    const dbVersions = quotedTokens(versionMatch![1], /'([^']+)'/g).sort();
    expect([...VERSION_TYPES].sort()).toEqual(dbVersions);

    const scenarioMatch = /fpna_plan_versions[\s\S]*?CHECK\s*\(\s*scenario_type\s+IN\s*\(([^)]+)\)\s*\)/.exec(sqlInit);
    expect(scenarioMatch, "scenario_type CHECK constraint found").not.toBeNull();
    const dbScenarios = quotedTokens(scenarioMatch![1], /'([^']+)'/g).sort();
    expect([...SCENARIO_TYPES].sort()).toEqual(dbScenarios);

    const statusMatch = /fpna_plan_versions[\s\S]*?CHECK\s*\(\s*status\s+IN\s*\(([^)]+)\)\s*\)/.exec(sqlInit);
    expect(statusMatch, "fpna_plan_versions status CHECK constraint found").not.toBeNull();
    const dbStatuses = quotedTokens(statusMatch![1], /'([^']+)'/g).sort();
    expect([...PLAN_STATUSES].sort()).toEqual(dbStatuses);
  });

  it("FP&A 数据质量类别与状态枚举 = DB CHECK 约束（单一来源）", () => {
    const categoryMatch = /fpna_data_quality_items[\s\S]*?CHECK\s*\(\s*category\s+IN\s*\(([^)]+)\)\s*\)/.exec(sqlInit);
    expect(categoryMatch, "category CHECK constraint found").not.toBeNull();
    const dbCategories = quotedTokens(categoryMatch![1], /'([^']+)'/g).sort();
    expect([...DQ_CATEGORIES].sort()).toEqual(dbCategories);

    const dqStatusMatch = /fpna_data_quality_items[\s\S]*?CHECK\s*\(\s*status\s+IN\s*\(([^)]+)\)\s*\)/.exec(sqlInit);
    expect(dqStatusMatch, "fpna_data_quality_items status CHECK constraint found").not.toBeNull();
    const dbDqStatuses = quotedTokens(dqStatusMatch![1], /'([^']+)'/g).sort();
    expect([...DQ_STATUSES].sort()).toEqual(dbDqStatuses);
  });

  it("草稿复核：数据分类 = ai_contract_drafts 分类 CHECK 约束（双向相等）", () => {
    const match = /ai_contract_drafts_classification_check[\s\S]*?CHECK\s*\([^)]*IN\s*\(([^)]+)\)\s*\)/.exec(sqlInit);
    expect(match, "ai_contract_drafts_classification_check found").not.toBeNull();
    const dbClassifications = quotedTokens(match![1], /'([^']+)'/g).sort();
    expect([...DRAFT_DATA_CLASSIFICATIONS].sort()).toEqual(dbClassifications);
  });

  it("草稿复核状态 ⊆ 后端 draftreview 服务与库默认值（pending 来自 DB default）", () => {
    // ai_contract_drafts.status 无 CHECK 约束；单一来源拆两半：
    //   pending   ← db/init 的 DEFAULT 'pending'
    //   其余      ← draftreview 服务里的状态字面量
    // 后端删掉某个状态的流转，对应断言即红。
    for (const status of DRAFT_REVIEW_STATUSES) {
      if (status === "pending") {
        expect(
          /ai_contract_drafts \([\s\S]*?status VARCHAR\(50\) NOT NULL DEFAULT 'pending'/.test(sqlInit),
          "01_init.sql pins ai_contract_drafts.status DEFAULT 'pending'",
        ).toBe(true);
      } else {
        expect(draftReviewBackend, `draftreview service references status '${status}'`).toContain(`"${status}"`);
      }
    }
  });

  it("对账差异六类枚举 = 后端 settlement.Category 六值封闭枚举（R-E4-1）", () => {
    const categories = quotedTokens(settlementMatchBackend, /Category(?:Fee|FX|Chargeback|InTransit|Adjustment|Reserve) +Category = "([^"]+)"/g);
    expect(categories).toHaveLength(6);
    expect([...SETTLEMENT_CATEGORIES].sort()).toEqual([...categories].sort());
  });

  it("对账 run 状态五值 = settlement_runs.status CHECK 约束（单一来源）", () => {
    const statusMatch = /settlement_runs[\s\S]*?CHECK\s*\(\s*status\s+IN\s*\(([^)]+)\)\s*\)/.exec(sqlInit);
    expect(statusMatch, "settlement_runs status CHECK constraint found").not.toBeNull();
    const dbStatuses = quotedTokens(statusMatch![1], /'([^']+)'/g).sort();
    expect([...SETTLEMENT_RUN_STATUSES].sort()).toEqual(dbStatuses);
  });

  it("广告口径两值 = campaign_day_facts.basis CHECK 约束（R-T3 无第三种取值）", () => {
    const basisMatch = /campaign_day_facts[\s\S]*?CHECK\s*\(\s*basis\s+IN\s*\(([^)]+)\)\s*\)/.exec(sqlInit);
    expect(basisMatch, "campaign_day_facts basis CHECK constraint found").not.toBeNull();
    const dbBasis = quotedTokens(basisMatch![1], /'([^']+)'/g).sort();
    expect([...ECOM_AD_BASIS].sort()).toEqual(dbBasis);
  });
});