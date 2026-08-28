/**
 * applyScheduleFill is the consumption seam for the agent-produced payment
 * schedule prefill (agent-universal-pagefill-v1 P0-B①). The artifact lands on
 * the contract workspace via ?schedule_fill=<artifactId> and this module is
 * the only path that turns it into form values.
 *
 * Safety invariants:
 *  - target_page must be "contract-workspace" — a cross-page paste is refused;
 *  - payload.contract_id must equal the open contract — a cross-contract
 *    paste is refused;
 *  - machine-extracted rows only ever live in suggestions; what reaches the
 *    form is a visible prefill the human confirms by submitting — this module
 *    never POSTs anything.
 */

export interface ScheduleFillRow {
  due_date: string;
  amount: number;
  period_start?: string;
  period_end?: string;
  payment_timing?: string;
}

export interface ScheduleFillSummary {
  valid_rows: number;
  skipped_rows: number;
  skip_reasons?: string[];
  min_due_date?: string;
  max_due_date?: string;
  total_amount?: number;
  sample?: ScheduleFillRow[];
}

/** Raw (dayjs-free) form values; the page converts dates before setFieldsValue. */
export interface ScheduleFillFormValues {
  due_date?: string;
  amount?: number;
  currency?: string;
  payment_timing?: string;
  effective_start_date?: string;
  effective_end_date?: string;
}

export type ScheduleFillApplication =
  | { ok: true; formValues: ScheduleFillFormValues; summary: ScheduleFillSummary }
  | { ok: false; reason: "mismatch" | "contract" | "malformed" };

const WORKSPACE_TARGET_PAGE = "contract-workspace";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/** Fill values travel as {value, provenance} wrappers on the wire. */
function entryValue(container: Record<string, unknown> | undefined, key: string): unknown {
  const entry = container?.[key];
  return isRecord(entry) ? entry.value : undefined;
}

function readRow(value: unknown): ScheduleFillRow | null {
  if (!isRecord(value)) return null;
  if (typeof value.due_date !== "string" || value.due_date === "") return null;
  if (typeof value.amount !== "number" || !Number.isFinite(value.amount) || value.amount <= 0) return null;
  const row: ScheduleFillRow = { due_date: value.due_date, amount: value.amount };
  for (const key of ["period_start", "period_end", "payment_timing"] as const) {
    if (typeof value[key] === "string" && (value[key] as string)) row[key] = value[key] as string;
  }
  return row;
}

export function applyScheduleFill(artifactData: unknown, contractId: string): ScheduleFillApplication {
  if (!isRecord(artifactData) || artifactData.target_page !== WORKSPACE_TARGET_PAGE) {
    return { ok: false, reason: "mismatch" };
  }
  const payload = isRecord(artifactData.payload) ? artifactData.payload : {};
  const payloadContractId = entryValue(payload, "contract_id");
  if (typeof payloadContractId !== "string" || payloadContractId !== contractId) {
    return { ok: false, reason: "contract" };
  }
  const suggestions = isRecord(artifactData.suggestions) ? artifactData.suggestions : {};
  const firstRow = readRow(entryValue(suggestions, "first_row"));
  if (!firstRow) {
    return { ok: false, reason: "malformed" };
  }
  const summarySource = entryValue(suggestions, "schedule_summary");
  const summary: ScheduleFillSummary = isRecord(summarySource)
    ? {
        valid_rows: typeof summarySource.valid_rows === "number" ? summarySource.valid_rows : 1,
        skipped_rows: typeof summarySource.skipped_rows === "number" ? summarySource.skipped_rows : 0,
        skip_reasons: Array.isArray(summarySource.skip_reasons) ? summarySource.skip_reasons.map(String) : undefined,
        min_due_date: typeof summarySource.min_due_date === "string" ? summarySource.min_due_date : undefined,
        max_due_date: typeof summarySource.max_due_date === "string" ? summarySource.max_due_date : undefined,
        total_amount: typeof summarySource.total_amount === "number" ? summarySource.total_amount : undefined,
        sample: Array.isArray(summarySource.sample) ? summarySource.sample.map(readRow).filter((r): r is ScheduleFillRow => r !== null) : undefined,
      }
    : { valid_rows: 1, skipped_rows: 0 };

  const formValues: ScheduleFillFormValues = {
    due_date: firstRow.due_date,
    amount: firstRow.amount,
  };
  const currency = entryValue(payload, "currency");
  if (typeof currency === "string" && currency) formValues.currency = currency;
  const envelopeTiming = entryValue(payload, "payment_timing");
  if (typeof envelopeTiming === "string" && envelopeTiming) formValues.payment_timing = envelopeTiming;
  if (firstRow.payment_timing) formValues.payment_timing = firstRow.payment_timing;
  if (firstRow.period_start) formValues.effective_start_date = firstRow.period_start;
  if (firstRow.period_end) formValues.effective_end_date = firstRow.period_end;

  return { ok: true, formValues, summary };
}
