/**
 * applyPlanFill is the consumption seam for the agent-produced budget/plan
 * version prefill (agent-universal-pagefill-v1 P0-B①). The artifact lands on
 * /retail-data-import via ?plan_fill=<artifactId> and this module is the only
 * path that turns it into plan-import form values.
 *
 * Safety invariants:
 *  - target_page must be "retail-data-import" — a cross-page paste is refused;
 *  - envelope values pass shape checks before touching form state (a hostile
 *    artifact cannot set nonsense into the version identity fields);
 *  - the parsed plan lines live in suggestions and are only surfaced as a
 *    reviewable summary — this module never POSTs anything; the import is
 *    always fired by the human from the page with the real file.
 */

export interface PlanFillFormValues {
  name?: string;
  version_type?: string;
  source?: string;
  as_of_period?: string;
  from_period?: string;
  to_period?: string;
  is_official?: boolean;
}

export interface PlanFillSummary {
  valid_rows: number;
  skipped_rows: number;
  skip_reasons?: string[];
  min_period?: string;
  max_period?: string;
  store_count?: number;
  total_revenue?: number;
}

export type PlanFillApplication =
  | { ok: true; formValues: PlanFillFormValues; summary: PlanFillSummary }
  | { ok: false; reason: "mismatch" | "malformed" };

const PERIOD_SHAPE = /^\d{4}-(0[1-9]|1[0-2])$/;
const VERSION_TYPES = new Set(["budget", "forecast", "scenario"]);

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function entryValue(container: Record<string, unknown> | undefined, key: string): unknown {
  const entry = container?.[key];
  return isRecord(entry) ? entry.value : undefined;
}

function entryString(container: Record<string, unknown> | undefined, key: string, pattern?: RegExp): string | undefined {
  const value = entryValue(container, key);
  if (typeof value !== "string" || value === "") return undefined;
  if (pattern && !pattern.test(value)) return undefined;
  return value;
}

export function applyPlanFill(artifactData: unknown): PlanFillApplication {
  if (!isRecord(artifactData) || artifactData.target_page !== "retail-data-import") {
    return { ok: false, reason: "mismatch" };
  }
  const payload = isRecord(artifactData.payload) ? artifactData.payload : {};
  const formValues: PlanFillFormValues = {};
  const name = entryString(payload, "name");
  const versionType = entryString(payload, "version_type");
  const asOf = entryString(payload, "as_of_period", PERIOD_SHAPE);
  const from = entryString(payload, "from_period", PERIOD_SHAPE);
  const to = entryString(payload, "to_period", PERIOD_SHAPE);
  // 版本信封必须成套成立：类型与覆盖期间齐了才算一次可信的预填，
  // 缺任何一角都拒绝——半套信封导入只会造出坏版本。
  if (!versionType || !VERSION_TYPES.has(versionType) || !asOf || !from || !to) {
    return { ok: false, reason: "malformed" };
  }
  formValues.name = name ?? "";
  formValues.version_type = versionType;
  formValues.as_of_period = asOf;
  formValues.from_period = from;
  formValues.to_period = to;
  const source = entryString(payload, "source");
  if (source) formValues.source = source;
  if (entryValue(payload, "is_official") === "true") formValues.is_official = true;

  const suggestions = isRecord(artifactData.suggestions) ? artifactData.suggestions : {};
  const summarySource = entryValue(suggestions, "plan_summary");
  const summary: PlanFillSummary = isRecord(summarySource)
    ? {
        valid_rows: typeof summarySource.valid_rows === "number" ? summarySource.valid_rows : 0,
        skipped_rows: typeof summarySource.skipped_rows === "number" ? summarySource.skipped_rows : 0,
        skip_reasons: Array.isArray(summarySource.skip_reasons) ? summarySource.skip_reasons.map(String) : undefined,
        min_period: typeof summarySource.min_period === "string" ? summarySource.min_period : undefined,
        max_period: typeof summarySource.max_period === "string" ? summarySource.max_period : undefined,
        store_count: typeof summarySource.store_count === "number" ? summarySource.store_count : undefined,
        total_revenue: typeof summarySource.total_revenue === "number" ? summarySource.total_revenue : undefined,
      }
    : { valid_rows: 0, skipped_rows: 0 };

  return { ok: true, formValues, summary };
}
