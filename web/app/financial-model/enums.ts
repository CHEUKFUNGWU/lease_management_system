/**
 * DB-enum registrations for the financial-model surface (CONTRACT-001).
 *
 * Each const mirrors a CHECK constraint in db/init/01_init.sql; the
 * code-lists contract test asserts exact equality against those constraints so
 * a backend enum addition fails the front-end test until registered here.
 */
export type FinModelRunStatus = "queued" | "running" | "completed" | "failed" | "cancelled";
export type FinModelRunTieOutStatus = "pending" | "passed" | "failed" | "degraded";
export type FinSavedViewKind = "store_pnl" | "financial_model" | "group_view";

export const FIN_MODEL_RUN_STATUSES: readonly FinModelRunStatus[] = [
  "queued",
  "running",
  "completed",
  "failed",
  "cancelled",
] as const;

export const FIN_MODEL_RUN_TIE_OUT_STATUSES: readonly FinModelRunTieOutStatus[] = [
  "pending",
  "passed",
  "failed",
  "degraded",
] as const;

export const FIN_SAVED_VIEW_KINDS: readonly FinSavedViewKind[] = [
  "store_pnl",
  "financial_model",
  "group_view",
] as const;

// ─── F0-2：机器枚举 → 用户文案 ──────────────────────────────
//
// 枚举值是接口契约，不是界面文案（§0.2 纪律）。映射表的键用联合类型
// 锁死——Record<联合类型, …> 少一个值 TypeScript 就编不过，不许退化为
// Record<string, string>。

/** 运行状态 → i18n 键。键集 = fin_model_runs.status CHECK 约束。 */
export const RUN_STATUS_LABEL: Record<FinModelRunStatus, string> = {
  queued: "finmodel.run_status.queued",
  running: "finmodel.run_status.running",
  completed: "finmodel.run_status.completed",
  failed: "finmodel.run_status.failed",
  cancelled: "finmodel.run_status.cancelled",
};

/** 勾稽总状态 → i18n 键。键集 = fin_model_runs.tie_out_status CHECK 约束。 */
export const TIE_OUT_LABEL: Record<FinModelRunTieOutStatus, string> = {
  pending: "finmodel.tie_out.pending",
  passed: "finmodel.tie_out.passed",
  failed: "finmodel.tie_out.failed",
  degraded: "finmodel.tie_out.degraded",
};

/** 导出折叠粒度 → i18n 键。这是 API 参数不是 DB 枚举；取值集合以后端导出 handler 为准。 */
export type FinModelPeriodGrain = "month" | "quarter" | "year";
export const FIN_MODEL_PERIOD_GRAINS: readonly FinModelPeriodGrain[] = ["month", "quarter", "year"];
export const PERIOD_GRAIN_LABEL: Record<FinModelPeriodGrain, string> = {
  month: "finmodel.grain.month",
  quarter: "finmodel.grain.quarter",
  year: "finmodel.grain.year",
};

/**
 * 缺口类型 → i18n 键。后端 DataGap.Kind 是开放字符串（engine.go 逐点 append），
 * 没有封闭联合可锁——按 assumptionHint 的既有模式：已知键给中文，未知键由
 * 调用方原样显示并标注「未识别」。每个已知键都必须能在后端源码里找到字面量，
 * 由 gap-kinds.test.ts 跨语言断言（CONTRACT-001 惯例）。
 */
export const GAP_KIND_LABEL: Record<string, string> = {
  opening_missing: "finmodel.gap.opening_missing",
  fact_port_unavailable: "finmodel.gap.fact_port_unavailable",
  fact_coverage: "finmodel.gap.fact_coverage",
  fact_missing: "finmodel.gap.fact_missing",
  lease_port_unavailable: "finmodel.gap.lease_port_unavailable",
  schedule_port_unavailable: "finmodel.gap.schedule_port_unavailable",
  assumption_port_unavailable: "finmodel.gap.assumption_port_unavailable",
  assumption_missing: "finmodel.gap.assumption_missing",
  unregistered_source: "finmodel.gap.unregistered_source",
  forecast_driver_missing: "finmodel.gap.forecast_driver_missing",
  revenue_driver_note: "finmodel.gap.revenue_driver_note",
  interest_presentation_missing: "finmodel.gap.interest_presentation_missing",
  subtotal_partial: "finmodel.gap.subtotal_partial",
};
