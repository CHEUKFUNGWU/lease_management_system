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
