/**
 * DB-enum registrations for the /store-360 surface (R0-2).
 *
 * currency_status 的取值集合来自 retailstore360.singleCurrency()
 * （store360.go:435-452）：known | conflict | unknown，Go 侧封闭。
 * 键用联合类型锁死；运行时若收到表外值（后端扩枚举前端未跟上），
 * 原样显示并标注「未识别」，不静默、不猜中文。
 */
import { t } from "../lib/i18n";

export type CurrencyStatus = "known" | "conflict" | "unknown";

export const CURRENCY_STATUSES: readonly CurrencyStatus[] = ["known", "conflict", "unknown"] as const;

export const CURRENCY_STATUS_LABEL: Record<CurrencyStatus, string> = {
  known: "store360.currency_status.known",
  conflict: "store360.currency_status.conflict",
  unknown: "store360.currency_status.unknown",
};

/** 页面消费入口：已知值给中文，表外值原样加未识别标注。 */
export function currencyStatusLabel(status: string, language: "zh-CN" | "zh-HK" | "en"): string {
  return status in CURRENCY_STATUS_LABEL
    ? t(CURRENCY_STATUS_LABEL[status as CurrencyStatus], language)
    : `${status} · ${t("store360.currency_status.unrecognized", language)}`;
}
