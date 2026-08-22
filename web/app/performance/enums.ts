/**
 * DB-enum registrations for the /performance surface (R0-2).
 *
 * 纪律同 web/app/financial-model/enums.ts：枚举值是接口契约不是界面文案。
 * 映射表的键用联合类型锁死——Record<联合类型, …> 少一个值 TypeScript 就编
 * 不过，不许退化为 Record<string, string>。键集与后端的一致性由
 * enums-guard.test.ts 跨语言断言（读 db/init 的 CHECK 约束与 Go 源码）。
 */

// fpna_action_items.severity 的 CHECK 约束（db/init/01_init.sql）
export type ActionSeverity = "critical" | "high" | "medium" | "low" | "informational";
// fpna_action_items.status 的 CHECK 约束
export type ActionStatus = "open" | "acknowledged" | "in_progress" | "completed" | "verified" | "accepted" | "dismissed";

export const ACTION_SEVERITIES: readonly ActionSeverity[] = [
  "critical",
  "high",
  "medium",
  "low",
  "informational",
] as const;

export const ACTION_STATUSES: readonly ActionStatus[] = [
  "open",
  "acknowledged",
  "in_progress",
  "completed",
  "verified",
  "accepted",
  "dismissed",
] as const;

/** 严重度 → i18n 键。键集 = fpna_action_items.severity CHECK。 */
export const SEVERITY_LABEL: Record<ActionSeverity, string> = {
  critical: "perf.sev.critical",
  high: "perf.sev.high",
  medium: "perf.sev.medium",
  low: "perf.sev.low",

  informational: "perf.sev.informational",
};

/** 行动状态 → i18n 键。键集 = fpna_action_items.status CHECK。 */
export const ACTION_STATUS_LABEL: Record<ActionStatus, string> = {
  open: "perf.action_status.open",
  acknowledged: "perf.action_status.acknowledged",
  in_progress: "perf.action_status.in_progress",
  completed: "perf.action_status.completed",
  verified: "perf.action_status.verified",
  accepted: "perf.action_status.accepted",
  dismissed: "perf.action_status.dismissed",
};

// category 没有封闭约束（VARCHAR(50)，用户可经 POST /performance/actions 自由填写），
// 不是封闭联合——按 assumptionHint 既有模式：已知键给中文，未知键原样显示并标注未识别。
// 已知键的全集来自后端源码里实际写入的 Category 字面量（enums-guard.test.ts 锁定）。
export const ACTION_CATEGORIES = ["variance_explanation", "retail_store_scenario"] as const;
export type KnownActionCategory = (typeof ACTION_CATEGORIES)[number];

export const CATEGORY_LABEL: Record<KnownActionCategory, string> = {
  variance_explanation: "perf.category.variance_explanation",
  retail_store_scenario: "perf.category.retail_store_scenario",
};
