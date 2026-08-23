/**
 * 三表财务模型工作台状态机（spec: docs/specs/financial_model_workbench_refactor.md）。
 *
 * 深模块：异步生命周期、错误分诊、scope_denied 保因（AGENTS.md 红线）、
 * 假设 JSON 校验、期初闸 payload 组装全部收在这里。页面壳只做
 * 「事件 → reduce → 渲染」，轮询定时器与 fetch 是壳侧副作用，不进纯函数。
 *
 * 与规格的一处偏差：WbState 增加 "cancelled" 相位——用户主动取消是独立
 * 结果，压进 failed 会让 UI 显示「失败」误导排障。
 */
import type { FinModelRunStatus } from "./enums";

export type TieOutRow = {
  check_code: string;
  period: string;
  expected?: number | null;
  actual?: number | null;
  diff?: number | null;
  status: string;
};

export type ModelGap = { kind: string; period?: string; detail: string };

export type ModelRun = {
  id?: string;
  periods: string[];
  tie_out_status: string;
  tie_outs: TieOutRow[];
  gaps?: ModelGap[];
};

export type WbEvent =
  | { t: "select_definition"; id: string }
  | { t: "edit_assumptions"; text: string }
  | { t: "run_requested" }
  | { t: "run_dispatched"; runId: string }
  | { t: "run_sync_done"; run: ModelRun; persisted?: boolean }
  | { t: "run_polled"; status: FinModelRunStatus; run?: ModelRun }
  | { t: "cancel_done" }
  | { t: "error"; kind: "parse" | "scope_denied" | "failed"; message?: string }
  | { t: "reset" };

export type WbState =
  | { phase: "idle"; definitionId: string; assumptionsError?: string }
  | { phase: "dispatching"; definitionId: string }
  | { phase: "polling"; runId: string; status: "queued" | "running"; definitionId: string }
  | { phase: "completed"; definitionId: string; run: ModelRun; persisted?: boolean }
  | { phase: "cancelled"; definitionId: string }
  | { phase: "failed"; definitionId: string; message?: string }
  | { phase: "scope_denied"; definitionId?: string; reason?: string };

export const initialWorkbenchState: Extract<WbState, { phase: "idle" }> = { phase: "idle", definitionId: "" };

/** scope_denied 的判定只看错误契约码，保持原因不被软化为「无数据」。 */
export function isScopeDenied(err: unknown): boolean {
  return typeof err === "object" && err !== null && (err as { code?: string }).code === "scope_denied";
}

/** 假设 JSON 解析：纯函数，reducer 与页面共用同一份判定。 */
export function parseAssumptions(text: string): { ok: true; value: Record<string, unknown> } | { ok: false; error: string } {
  const trimmed = text.trim();
  if (!trimmed) return { ok: true, value: {} };
  try {
    const parsed = JSON.parse(trimmed) as unknown;
    if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
      return { ok: false, error: "not_an_object" };
    }
    return { ok: true, value: parsed as Record<string, unknown> };
  } catch {
    return { ok: false, error: "invalid_json" };
  }
}

/**
 * F3-1：键值表单与「高级：粘贴 JSON」两个入口汇合的唯一接缝。
 *
 * 表单只改已知键的数值；JSON 里前端不认识的键原样保留（它们仍会传给
 * 后端，由引擎决定是否产出 assumption_missing Gap）。value=null 表示删除
 * 该键——「未提供」≠ 0，缺键由引擎显式降级而不是静默补零。
 *
 * 返回新的 JSON 文本，页面仍把它交给 edit_assumptions → parseAssumptions
 * 走同一条解析/校验路径；文本非法时原样返回（表单冻结，不覆盖用户正在
 * 编辑的 JSON）。
 */
export function applyAssumptionFormValues(text: string, changes: Record<string, number | null>): string {
  const parsed = parseAssumptions(text);
  if (!parsed.ok) return text;
  const next: Record<string, unknown> = { ...parsed.value };
  for (const [key, value] of Object.entries(changes)) {
    if (value == null) delete next[key];
    else next[key] = value;
  }
  if (Object.keys(next).length === 0) return "";
  return JSON.stringify(next, null, 2);
}

function withAssumptions(state: WbState, text: string): WbState {
  if (state.phase === "dispatching" || state.phase === "polling") return state;
  if (state.phase === "scope_denied") return state;
  const parsed = parseAssumptions(text);
  // completed/failed/cancelled 上改假设 = 准备下一次运行，回落到 idle 语义
  // 但保留已完成的 run 供查看？——不。结果区属于右栏独立生命周期，这里
  // 只管输入侧；run 展示由页面持有，不进状态机。
  const base: WbState =
    state.phase === "completed"
      ? { phase: "idle", definitionId: state.definitionId }
      : state.phase === "failed" || state.phase === "cancelled"
        ? { phase: "idle", definitionId: state.definitionId }
        : state;
  if (base.phase !== "idle") return base;
  return { ...base, assumptionsError: parsed.ok ? undefined : parsed.error };
}

export function reduceWorkbench(state: WbState, event: WbEvent): WbState {
  switch (event.t) {
    case "reset":
      return initialWorkbenchState;
    case "select_definition": {
      if (state.phase === "dispatching" || state.phase === "polling" || state.phase === "scope_denied") return state;
      if (state.phase === "idle") return { ...state, definitionId: event.id };
      return { phase: "idle", definitionId: event.id };
    }
    case "edit_assumptions":
      return withAssumptions(state, event.text);
    case "run_requested": {
      if (state.phase !== "idle") return state;
      if (!state.definitionId || state.assumptionsError) return state;
      return { phase: "dispatching", definitionId: state.definitionId };
    }
    case "run_dispatched":
      if (state.phase !== "dispatching") return state;
      return { phase: "polling", runId: event.runId, status: "queued", definitionId: state.definitionId };
    case "run_sync_done":
      if (state.phase !== "dispatching") return state;
      return { phase: "completed", definitionId: state.definitionId, run: event.run, persisted: event.persisted };
    case "run_polled": {
      if (state.phase !== "polling") return state;
      if (event.status === "queued" || event.status === "running") {
        return { ...state, status: event.status };
      }
      if (event.status === "completed" && event.run) {
        return { phase: "completed", definitionId: state.definitionId, run: event.run };
      }
      if (event.status === "cancelled") {
        return { phase: "cancelled", definitionId: state.definitionId };
      }
      return { phase: "failed", definitionId: state.definitionId };
    }
    case "cancel_done":
      if (state.phase !== "polling") return state;
      return { phase: "cancelled", definitionId: state.definitionId };
    case "error":
      if (event.kind === "scope_denied") {
        return { phase: "scope_denied", definitionId: state.phase === "scope_denied" ? state.definitionId : "definitionId" in state ? state.definitionId : undefined, reason: event.message };
      }
      if (event.kind === "parse") {
        return state.phase === "idle" ? { ...state, assumptionsError: event.message ?? "invalid_json" } : state;
      }
      if (state.phase === "dispatching" || state.phase === "polling") {
        return { phase: "failed", definitionId: state.definitionId, message: event.message };
      }
      return state;
    default:
      return state;
  }
}

// ─── 期初三道闸：结构化表单 → 后端契约 ──────────────────────────
//
// 交互层目标流程（spec 追加决策 ②）：选期间 → 引擎侧自动取数 →
// 导入侧上传 → 校验；手打合约行降级为高级模式。引擎取数与文件上传
// 的后端端点属 B 阶段（③ 地基层），未上线前页面渲染诚实的不可用态，
// 手动路径是唯一能真正走到校验的入口——不造假按钮。

export type OpeningContractRow = { contract_id: string; lease_liability: string; rou_asset: string };

export type OpeningFormState = {
  legalEntityId: string;
  currency: string;
  policyVersion: string;
  /** 第 1 步·选期间：结构化选择，去重、保序。 */
  periods: string[];
  /** 高级模式：period → {lines, mapping}。每个已选期间都必须有余额，
   *  否则三道闸会对缺数期间「空转通过」——恒真勾稽，payload 组装拒绝。 */
  balancesJson: string;
  leaseRef: OpeningContractRow[];
  engine: OpeningContractRow[];
};

export const emptyOpeningForm: OpeningFormState = {
  legalEntityId: "",
  currency: "",
  policyVersion: "v1",
  periods: [],
  balancesJson: "",
  leaseRef: [],
  engine: [],
};

/** 添加期间：trim、拒绝空串与重复。返回原数组表示没有变化。 */
export function addPeriod(periods: string[], raw: string): string[] {
  const period = raw.trim();
  if (!period || periods.includes(period)) return periods;
  return [...periods, period];
}

export function removePeriod(periods: string[], raw: string): string[] {
  return periods.filter((p) => p !== raw);
}

export type OpeningPayload = {
  balance: { legal_entity_id: string; currency: string; periods: unknown[] };
  lease_ref: { contract_id: string; lease_liability: number; rou_asset: number }[];
  engine: { contract_id: string; lease_liability: number; rou_asset: number }[];
  policy: { version: string };
};

export type RawBalanceEntry = { lines?: Record<string, number>; mapping?: Record<string, string> };

/**
 * balancesJson 解析：{期间: {lines: 数值map, mapping: 字符串map}}。
 * lines/mapping 的值类型在这里锁死——放行字符串数值会让后端
 * ShouldBindJSON 直接 400，前端先诚实报错。
 */
export function parseBalances(text: string): { ok: true; value: Record<string, RawBalanceEntry> } | { ok: false; error: "bad_balances" } {
  const trimmed = text.trim();
  if (!trimmed) return { ok: true, value: {} };
  let parsed: unknown;
  try {
    parsed = JSON.parse(trimmed);
  } catch {
    return { ok: false, error: "bad_balances" };
  }
  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) return { ok: false, error: "bad_balances" };
  const value: Record<string, RawBalanceEntry> = {};
  for (const [period, rawEntry] of Object.entries(parsed as Record<string, unknown>)) {
    if (typeof rawEntry !== "object" || rawEntry === null || Array.isArray(rawEntry)) return { ok: false, error: "bad_balances" };
    const entry = rawEntry as Record<string, unknown>;
    let lines: Record<string, number> | undefined;
    if (entry.lines !== undefined) {
      if (typeof entry.lines !== "object" || entry.lines === null || Array.isArray(entry.lines)) return { ok: false, error: "bad_balances" };
      lines = {};
      for (const [line, v] of Object.entries(entry.lines as Record<string, unknown>)) {
        if (typeof v !== "number" || !Number.isFinite(v)) return { ok: false, error: "bad_balances" };
        lines[line] = v;
      }
    }
    let mapping: Record<string, string> | undefined;
    if (entry.mapping !== undefined) {
      if (typeof entry.mapping !== "object" || entry.mapping === null || Array.isArray(entry.mapping)) return { ok: false, error: "bad_balances" };
      mapping = {};
      for (const [account, v] of Object.entries(entry.mapping as Record<string, unknown>)) {
        if (typeof v !== "string") return { ok: false, error: "bad_balances" };
        mapping[account] = v;
      }
    }
    value[period] = { lines, mapping };
  }
  return { ok: true, value };
}

function coerceRows(rows: OpeningContractRow[]): { ok: true; value: OpeningPayload["lease_ref"] } | { ok: false; error: "row_incomplete" } {
  const value: OpeningPayload["lease_ref"] = [];
  for (const row of rows) {
    const liability = Number(row.lease_liability);
    const rou = Number(row.rou_asset);
    if (!row.contract_id.trim() || !Number.isFinite(liability) || !Number.isFinite(rou)) {
      return { ok: false, error: "row_incomplete" };
    }
    value.push({ contract_id: row.contract_id.trim(), lease_liability: liability, rou_asset: rou });
  }
  return { ok: true, value };
}

export type OpeningPayloadResult =
  | { ok: true; payload: OpeningPayload }
  | {
      ok: false;
      error:
        | "missing_entity"
        | "missing_currency"
        | "no_periods"
        | "bad_balances"
        | "missing_balance_for_period"
        | "row_incomplete";
    };

export function buildOpeningPayload(form: OpeningFormState): OpeningPayloadResult {
  if (!form.legalEntityId.trim()) return { ok: false, error: "missing_entity" };
  if (!form.currency.trim()) return { ok: false, error: "missing_currency" };
  // 空期间集会让三道闸对零条记录「全部通过」——恒真勾稽，直接拒绝。
  // 去重 + 保序：UI 层 addPeriod 已拦重复，这里防御性再兜一次。
  const periods = Array.from(new Set(form.periods.map((p) => p.trim()).filter(Boolean)));
  if (periods.length === 0) return { ok: false, error: "no_periods" };
  const balances = parseBalances(form.balancesJson);
  if (!balances.ok) return balances;
  for (const period of periods) {
    if (!(period in balances.value)) return { ok: false, error: "missing_balance_for_period" };
  }
  const leaseRef = coerceRows(form.leaseRef);
  if (!leaseRef.ok) return leaseRef;
  const engine = coerceRows(form.engine);
  if (!engine.ok) return engine;
  return {
    ok: true,
    payload: {
      balance: {
        legal_entity_id: form.legalEntityId.trim(),
        currency: form.currency.trim().toUpperCase(),
        periods: periods.map((period) => ({
          period,
          lines: balances.value[period].lines ?? {},
          mapping: balances.value[period].mapping ?? {},
        })),
      },
      lease_ref: leaseRef.value,
      engine: engine.value,
      policy: { version: form.policyVersion.trim() || "v1" },
    },
  };
}
