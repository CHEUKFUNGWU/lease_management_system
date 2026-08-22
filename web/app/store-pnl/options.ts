/**
 * CONTRACT-001 single source for the store P&L comparison columns.
 *
 * The page renders one comparison column (Actual is the primary). The
 * selectable comparison refs mirror core-service/internal/storepnl's
 * ColumnRef whitelist minus "actual" (the primary column); code-lists-contract
 * asserts each value stays inside that whitelist.
 */
export type StorePnlSecondaryColumn = "budget" | "forecast" | "prior_year";

export const STORE_PNL_SECONDARY_COLUMNS: readonly StorePnlSecondaryColumn[] = [
  "budget",
  "forecast",
  "prior_year",
] as const;

/**
 * F0-3（任务指令：财务视角的 UI/UX 与术语整改）：同业对标状态 → 界面文案。
 * 取值全集在 core-service/internal/storepnl/project.go 的 PeerStatus 注释：
 * complete | insufficient_peers | mixed_currency | unavailable。
 * 枚举值是接口契约，不许直接渲染进中文利润表的「同业对标」列；
 * 键集由 peer-status.test.ts 跨语言断言锁定。
 */
import { t, type Language } from "../lib/i18n";

export type StorePnlPeerStatus = "complete" | "insufficient_peers" | "mixed_currency" | "unavailable";

export const STORE_PNL_PEER_STATUSES: readonly StorePnlPeerStatus[] = [
  "complete",
  "insufficient_peers",
  "mixed_currency",
  "unavailable",
] as const;

export const PEER_STATUS_LABEL: Record<StorePnlPeerStatus, string> = {
  complete: "storepnl.peer_status.complete",
  insufficient_peers: "storepnl.peer_status.insufficient_peers",
  mixed_currency: "storepnl.peer_status.mixed_currency",
  unavailable: "storepnl.peer_status.unavailable",
};

/** 已知状态给中文；未知状态原样透传并标注未识别（后端新增值时不静默）。 */
export function peerStatusLabel(status: string, language: Language): string {
  const key = (PEER_STATUS_LABEL as Record<string, string>)[status];
  if (!key) return `${status} · ${t("storepnl.peer_status_unknown", language)}`;
  return t(key, language);
}
