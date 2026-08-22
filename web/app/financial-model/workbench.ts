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

export type OpeningContractRow = { contract_id: string; lease_liability: string; rou_asset: string };

export type OpeningFormState = {
  legalEntityId: string;
  currency: string;
  policyVersion: string;
  /** 期间余额走紧凑 JSON（Lines 是自由 map，表格化收益低），带解析校验。 */
  periodsJson: string;
  leaseRef: OpeningContractRow[];
  engine: OpeningContractRow[];
};

export const emptyOpeningForm: OpeningFormState = {
  legalEntityId: "",
  currency: "",
  policyVersion: "v1",
  periodsJson: "",
  leaseRef: [],
  engine: [],
};

export type OpeningPayload = {
  balance: { legal_entity_id: string; currency: string; periods: unknown[] };
  lease_ref: { contract_id: string; lease_liability: number; rou_asset: number }[];
  engine: { contract_id: string; lease_liability: number; rou_asset: number }[];
  policy: { version: string };
};

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

export function buildOpeningPayload(
  form: OpeningFormState,
): { ok: true; payload: OpeningPayload } | { ok: false; error: "missing_entity" | "missing_currency" | "bad_periods" | "row_incomplete" } {
  if (!form.legalEntityId.trim()) return { ok: false, error: "missing_entity" };
  if (!form.currency.trim()) return { ok: false, error: "missing_currency" };
  let periods: unknown[] = [];
  if (form.periodsJson.trim()) {
    try {
      const parsed = JSON.parse(form.periodsJson) as unknown;
      if (!Array.isArray(parsed)) return { ok: false, error: "bad_periods" };
      periods = parsed;
    } catch {
      return { ok: false, error: "bad_periods" };
    }
  }
  const leaseRef = coerceRows(form.leaseRef);
  if (!leaseRef.ok) return leaseRef;
  const engine = coerceRows(form.engine);
  if (!engine.ok) return engine;
  return {
    ok: true,
    payload: {
      balance: { legal_entity_id: form.legalEntityId.trim(), currency: form.currency.trim().toUpperCase(), periods },
      lease_ref: leaseRef.value,
      engine: engine.value,
      policy: { version: form.policyVersion.trim() || "v1" },
    },
  };
}
