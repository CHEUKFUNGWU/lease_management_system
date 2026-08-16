import { t, type Language } from "../lib/i18n";
import type { RetailBridge, RetailKPIValue, RetailStore360Option, RetailStore360SummaryMetric, RetailStoreDiagnosticsResponse, RetailSummaryMetric } from "../lib/api";
import { formatKPIValue, formatSignalValue, kpiLabel, metricStatusLabel, metricUnitLabel } from "../operating-pulse/logic";

export const STORE360_CODES = ["revenue", "gross_profit", "gross_margin_rate", "footfall", "conversion_rate", "store_contribution"] as const;
export const STORE360_AUX_CODES = ["average_transaction_value", "labor_cost_rate", "occupancy_cash_cost_rate", "store_contribution_margin", "sales_per_sqm"] as const;
export const WINDOW_OPTIONS = [7, 14, 28] as const;

export function validWindow(value: number): boolean {
  // M2: custom rolling windows are legal anywhere in 7-28 (server contract);
  // WINDOW_OPTIONS stays the quick-pick list.
  return Number.isInteger(value) && value >= 7 && value <= 28;
}

export function optionFields(option: RetailStore360Option) {
  return {
    storeID: option.store_id,
    storeCode: option.store_code,
    storeName: option.store_name,
    brand: option.brand,
    region: option.region,
  };
}

export function diagnosticQueryKey(params: { storeID: string; classification: string; datasetVersion: string; asOf: string; windowDays: number; sourceSystem: string }): string {
  return [params.storeID, params.classification, params.datasetVersion, params.asOf, params.windowDays, params.sourceSystem].join("|");
}

export function returnPulseQuery(params: URLSearchParams): string {
  const encodedReturn = params.get("return_query");
  if (encodedReturn) {
    return `/operating-pulse?${encodedReturn}`;
  }
  const next = new URLSearchParams();
  ["data_classification", "dataset_version", "as_of", "window_days", "source_system"].forEach((key) => {
    const value = params.get(key);
    if (value) next.set(key, value);
  });
  params.getAll("store_id").filter(Boolean).forEach((id) => next.append("store_id", id));
  return `/operating-pulse${next.toString() ? `?${next.toString()}` : ""}`;
}

export function bridgeTone(value: number | null): "positive" | "negative" | "neutral" {
  if (value == null || Math.abs(value) < 0.005) return "neutral";
  return value > 0 ? "positive" : "negative";
}

export function formatBridgeItem(value: number | null, unit: string, currency: string, language: Language): string {
  return formatSignalValue(value, unit, currency, language);
}

/** FIX-018: the bridge payload is already waterfall-shaped (comparison ->
 *  items -> current, with a rounding residual that must be shown for the bars
 *  to reconcile). This turns it into floating [from, to] ranges so recharts
 *  can draw it directly; an incomplete bridge yields no steps. */
export interface BridgeWaterfallStep {
  name: string;
  range: [number, number];
  contribution: number;
  tone: "positive" | "negative" | "neutral";
}

export function bridgeWaterfall(bridge: RetailBridge, labels: { start: string; end: string; residual: string }): BridgeWaterfallStep[] {
  if (bridge.status !== "complete" || bridge.comparison == null || bridge.current == null) return [];
  let running = bridge.comparison;
  const steps: BridgeWaterfallStep[] = [{ name: labels.start, range: [0, running], contribution: running, tone: "neutral" }];
  const items = bridge.items.map((item) => ({ label: item.label, contribution: item.contribution ?? 0 }));
  if (bridge.rounding_residual) items.push({ label: labels.residual, contribution: bridge.rounding_residual });
  for (const item of items) {
    const next = running + item.contribution;
    steps.push({ name: item.label, range: [Math.min(running, next), Math.max(running, next)], contribution: item.contribution, tone: bridgeTone(item.contribution) });
    running = next;
  }
  steps.push({ name: labels.end, range: [0, bridge.current], contribution: bridge.current, tone: "neutral" });
  return steps;
}

/** FIX-018a: a waterfall on a large base is unreadable against a zero-based
 *  axis — a 500 contribution on a 48,000 opening is one pixel. The domain
 *  brackets the values the steps actually span, padded by a tenth of that
 *  span, so the contributions are the visible part. Returns null when every
 *  step is flat (nothing to bracket) and the caller should let recharts pick. */
export function bridgeWaterfallDomain(steps: BridgeWaterfallStep[]): [number, number] | null {
  if (steps.length === 0) return null;
  const values = steps.flatMap((step) => step.range);
  const low = Math.min(...values.filter((value) => value !== 0));
  const high = Math.max(...values);
  if (!Number.isFinite(low) || high === low) return null;
  const pad = (high - low) / 10;
  // FIX-025: the raw padded bounds made recharts derive ticks off an arbitrary
  // float — the axis read "25,351.727 / 25,217.723 / 24,917.723". Snapping the
  // bounds outward to a round step gives the axis whole numbers to divide.
  // The step comes from the *span*, not from the values: a 1,034-wide bracket
  // sitting at 24,000 snaps to 100s (widening it by ~6%), whereas snapping by
  // the value's own magnitude would round to 1,000s and double the bracket —
  // undoing exactly the zoom FIX-018a added. Outward only: never clip a step.
  const span = (high + pad) - (low - pad);
  return [niceFloor(low - pad, span), niceCeil(high + pad, span)];
}

/** Step size for snapping: the largest power of ten that still divides the
 *  span into at least a few slices, so a 1,000-wide span snaps to 100s and a
 *  10-wide span snaps to 1s. */
function niceStep(span: number): number {
  if (!(span > 0)) return 1;
  return Math.pow(10, Math.floor(Math.log10(span)) - 1);
}

export function niceFloor(value: number, span?: number): number {
  const step = niceStep(span ?? Math.abs(value));
  return Math.floor(value / step) * step;
}

export function niceCeil(value: number, span?: number): number {
  const step = niceStep(span ?? Math.abs(value));
  return Math.ceil(value / step) * step;
}

export function bridgeConservation(bridge: RetailBridge): number | null {
  if (bridge.total_change == null) return null;
  const sum = bridge.items.reduce((total, item) => total + (item.contribution || 0), 0);
  return sum + (bridge.rounding_residual || 0) - bridge.total_change;
}

export function trendValue(row: RetailStoreDiagnosticsResponse["daily_trend"][number], code: string): number | null {
	if (row.gap) return null;
	return row.target_kpis[code]?.value ?? null;
}

export function formatPeerBenchmarkStatus(status: string, reason: string | undefined, language: Language): string {
	const labels: Record<string, string> = {
		complete: "store360.peer_status.complete",
		insufficient_peers: "store360.peer_status.insufficient_peers",
		unavailable: "store360.peer_status.unavailable",
	};
	const key = labels[status];
	const label = key ? t(key, language) : status;
	return reason ? `${label} · ${reason}` : label;
}

export function formatTrendTooltip(value: number | null, series: string, gap: boolean, unit: string, currency: string, language: Language): [string, string] {
	const label = series === "target" ? t("store360.trend.target", language) : t("store360.trend.peer_median", language);
	if (series === "target" && gap) return [t("store360.trend.data_gap", language), label];
	return [formatSignalValue(value, unit, currency, language), label];
}

export function summaryStatus(metric: RetailStore360SummaryMetric | undefined, language: Language): { status: string; label: string; reason?: string } {
  return metricStatusLabel(metric as RetailSummaryMetric | undefined, language);
}

export function displayMetric(metric: RetailStore360SummaryMetric | undefined, currency: string, language: Language): string {
  return metric ? formatKPIValue(metric.current, currency, language) : "—";
}

export function signalUnit(unit: string, currency: string, language: Language): string {
  return metricUnitLabel(unit, currency, language);
}

export { kpiLabel };
