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
