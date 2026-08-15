import { ApiError, apiErrorMessage } from "./api";

/**
 * STATE-001: 数据状态的三分法判定（判定层，纯函数）。
 *
 * 页面不再各自 `if (error) ... else if (!data) ...`；统一问「这一次到底是
 * 空、是需要用户先做某件事、还是真异常」：
 *
 *   - empty     请求成功（或后端明确说没有），事实为空 → 空态 + 说明为什么空
 *   - actionable 用户能自己解决（缺数据、缺配置、选错了分类）→ 空态 + 明确的
 *                下一步动作（带入口）
 *   - failed    真异常，用户无能为力 → 错误态 + 原因 + 重试
 *
 * scope_denied 永不并入三分法（AGENTS.md 红线：权限拒绝必须保持原因）。
 *
 * 判定规则（任务书 §2.3 要求区分「后端说没有」与「请求没打通」）：
 *   - 网络失败 / 5xx / timeout      → failed（请求没打通，重试才有意义）
 *   - 404                          → empty（后端明确说没有；调用方可经
 *                                     actionFor 升级为 actionable）
 *   - 422 data_unavailable         → empty（数据条件不满足；调用方可经
 *                                     actionFor 升级为 actionable，如折现率
 *                                     缺失 → 「去补折现率」）
 *   - 200 且数据为空                → empty
 *   - 其余 4xx（无 actionFor）      → failed
 */

export type DataStateKind = "empty" | "actionable" | "failed" | "scope_denied" | "ready";

export interface DataState<T> {
  kind: DataStateKind;
  data?: T;
  /** empty 时的说明（为什么空）。 */
  reason?: string;
  /** actionable / failed 的呈现文案。 */
  message?: string;
  /** actionable 的下一步动作标签。 */
  actionLabel?: string;
}

export interface ClassifyInput<T> {
  error: unknown;
  data: T | null | undefined;
  /** 自定义「空」判定（默认：null/undefined 或空数组）。 */
  isEmpty?: (data: T) => boolean;
  /** 把某个错误升级为 actionable：返回文案与动作标签，返回 null 则不升级。 */
  actionFor?: (error: unknown) => { message: string; actionLabel: string } | null;
}

export function classifyDataState<T>(input: ClassifyInput<T>): DataState<T> {
  const { error, data, isEmpty, actionFor } = input;
  if (error) {
    if (error instanceof ApiError && error.code === "scope_denied") {
      // 独立第四态：权限拒绝，原因保留，绝不并入 empty / actionable。
      return { kind: "scope_denied", message: apiErrorMessage(error) };
    }
    if (actionFor) {
      const action = actionFor(error);
      if (action) return { kind: "actionable", message: action.message, actionLabel: action.actionLabel };
    }
    if (isRequestFailure(error)) {
      return { kind: "failed", message: apiErrorMessage(error) };
    }
    if (error instanceof ApiError && (error.status === 404 || error.status === 422)) {
      // 后端明确说「没有」/「条件不满足」：这是空（或可行动），不是故障。
      return { kind: "empty", reason: apiErrorMessage(error) };
    }
    return { kind: "failed", message: apiErrorMessage(error) };
  }
  if (data == null || (isEmpty ? isEmpty(data) : Array.isArray(data) && data.length === 0)) {
    return { kind: "empty" };
  }
  return { kind: "ready", data };
}

/** 请求没打通：网络失败、5xx、超时、取消。 */
export function isRequestFailure(error: unknown): boolean {
  if (!(error instanceof ApiError)) return true;
  if (error.code === "network_error" || error.code === "timeout" || error.code === "cancelled") return true;
  return error.status >= 500;
}
