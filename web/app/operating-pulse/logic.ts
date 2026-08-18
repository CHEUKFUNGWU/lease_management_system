import { t, type Language } from "../lib/i18n";
import type {
  RetailAttention,
  RetailKPIValue,
  RetailPulsePartition,
  RetailPulseResponse,
  RetailSummaryMetric,
  RetailSimulationDatasetData,
} from "../lib/api";

export const PULSE_KPI_CODES = [
  "revenue",
  "gross_profit",
  "gross_margin_rate",
  "footfall",
  "conversion_rate",
  "store_contribution",
] as const;

export const PULSE_AUXILIARY_CODES = [
  "average_transaction_value",
  "labor_cost_rate",
  "occupancy_cash_cost_rate",
  "store_contribution_margin",
  "sales_per_sqm",
] as const;

export type PulseMetricCode = (typeof PULSE_KPI_CODES)[number] | (typeof PULSE_AUXILIARY_CODES)[number];

const KPI_LABEL_KEYS: Record<PulseMetricCode, string> = {
  revenue: "retail.kpi.revenue",
  gross_profit: "retail.kpi.gross_profit",
  gross_margin_rate: "retail.kpi.gross_margin_rate",
  footfall: "retail.kpi.footfall",
  conversion_rate: "retail.kpi.conversion_rate",
  store_contribution: "retail.kpi.store_contribution",
  average_transaction_value: "retail.kpi.average_transaction_value",
  labor_cost_rate: "retail.kpi.labor_cost_rate",
  occupancy_cash_cost_rate: "retail.kpi.occupancy_cash_cost_rate",
  store_contribution_margin: "retail.kpi.store_contribution_margin",
  sales_per_sqm: "retail.kpi.sales_per_sqm",
};

export function kpiLabel(code: string, language: Language): string {
  const key = KPI_LABEL_KEYS[code as PulseMetricCode];
  return key ? t(key, language) : code;
}

const SIGNAL_LABEL_KEYS: Record<string, string> = {
  revenue_decline: "retail.signal.revenue_decline",
  footfall_decline: "retail.signal.footfall_decline",
  footfall_continuous_decline: "retail.signal.footfall_continuous_decline",
  conversion_drop: "retail.signal.conversion_drop",
  conversion_rate_drop: "retail.signal.conversion_rate_drop",
  average_ticket_drop: "retail.signal.average_ticket_drop",
  gross_margin_compression: "retail.signal.gross_margin_compression",
  labor_cost_rate_spike: "retail.signal.labor_cost_rate_spike",
  labor_cost_spike: "retail.signal.labor_cost_spike",
  occupancy_cost_rate_spike: "retail.signal.occupancy_cost_rate_spike",
  occupancy_cost_burden: "retail.signal.occupancy_cost_burden",
  contribution_turns_negative: "retail.signal.contribution_turns_negative",
};

export const metricUnitLabel = (unit: string, currency: string | undefined, language: Language): string => {
  if (unit === "currency") return currency || t("retail.unit.currency", language);
  if (unit === "currency_per_sqm") return `${currency || t("retail.unit.currency", language)}/㎡`;
  if (unit === "percent") return "%";
  if (unit === "count") return t("retail.unit.count", language);
  return unit;
};

export function formatUnitValue(value: number | null | undefined, unit: string, currency: string | undefined, language: Language): string {
  if (value === null || value === undefined) return "—";
  const precision = unit === "count" ? 0 : 2;
  const formatted = value.toLocaleString("zh-CN", { minimumFractionDigits: precision, maximumFractionDigits: precision });
  if (unit === "percent") return `${formatted}%`;
  if (unit === "percentage_point") return `${formatted}pp`;
  if (unit === "currency" || unit === "currency_per_sqm") return `${formatted} ${metricUnitLabel(unit, currency, language)}`;
  if (unit === "count") return `${formatted} ${metricUnitLabel(unit, currency, language)}`;
  return `${formatted} ${unit}`;
}

export function formatSignalValue(value: number | null | undefined, unit: string, currency: string | undefined, language: Language): string {
  return formatUnitValue(value, unit, currency, language);
}

export function formatKPIValue(value: RetailKPIValue | null | undefined, currency: string | undefined, language: Language): string {
  return value ? formatUnitValue(value.value, value.unit, currency, language) : "—";
}

export function formatChange(metric: RetailSummaryMetric | null | undefined): string {
  if (!metric || metric.change_value === null || metric.change_value === undefined) return "—";
  const suffix = metric.change_type === "percentage_point" ? "pp" : "%";
  const prefix = metric.change_value > 0 ? "+" : "";
  return `${prefix}${metric.change_value.toFixed(2)}${suffix}`;
}

export function changeTone(code: PulseMetricCode, metric: RetailSummaryMetric | null | undefined): "bad" | "good" | "neutral" | "missing" {
  if (!metric || metric.change_value === null || metric.change_value === undefined) return "missing";
  if (metric.change_value === 0) return "neutral";
  const costRate = code === "labor_cost_rate" || code === "occupancy_cash_cost_rate";
  const unfavorable = costRate ? metric.change_value > 0 : metric.change_value < 0;
  return unfavorable ? "bad" : "good";
}

export function translateReason(reason: string | undefined, language: Language): string {
  if (!reason) return "";
  const key = `reason.${reason}`;
  const translated = t(key, language);
  return translated !== key ? translated : reason;
}

export function formatSeverity(severity: string | undefined, language: Language): string {
  if (!severity) return "—";
  const key = `severity.${severity.toLowerCase()}`;
  const translated = t(key, language);
  return translated !== key ? translated : severity;
}

export function formatSourceSystem(source: string | undefined, language: Language): string {
  if (!source) return "—";
  const key = `source.${source.replace(/-/g, "_")}`;
  const translated = t(key, language);
  return translated !== key ? translated : source;
}

export function metricStatusLabel(metric: RetailSummaryMetric | null | undefined, language: Language): { status: "complete" | "partial" | "missing"; label: string; reason?: string } {
  if (!metric) return { status: "missing", label: t("retail.status.missing", language), reason: t("retail.status_reason.unavailable", language) };
  const statuses = [metric.current.status, metric.comparison.status];
  const rawReasons = [metric.current.reason, metric.comparison.reason, metric.reason].filter(Boolean) as string[];
  const reasons = rawReasons.map((r) => translateReason(r, language));
  if (statuses.some((status) => status === "unavailable")) return { status: "missing", label: t("retail.status.missing", language), reason: reasons.join(" / ") || t("retail.status_reason.facts_unavailable", language) };
  if (statuses.some((status) => status === "partial")) return { status: "partial", label: t("retail.status.partial", language), reason: reasons.join(" / ") || t("retail.status_reason.coverage_incomplete", language) };
  return { status: "complete", label: t("retail.status.complete", language), reason: reasons.join(" / ") || undefined };
}

export function signalLabel(code: string, language: Language): string {
  const key = SIGNAL_LABEL_KEYS[code];
  return key ? t(key, language) : code;
}

/** FIX-018: score_contribution was returned by the API and never read. Rolled
 *  up per signal code it answers the one question the attention table cannot:
 *  whether the flagged stores share one cause or each have their own. */
export interface SignalMixRow {
  code: string;
  label: string;
  stores: number;
  weight: number;
}

export function signalMix(attention: RetailAttention[], language: Language): SignalMixRow[] {
  const rows = new Map<string, SignalMixRow>();
  for (const store of attention) {
    for (const signal of store.observed_signals) {
      const row = rows.get(signal.signal_code) ?? { code: signal.signal_code, label: signalLabel(signal.signal_code, language), stores: 0, weight: 0 };
      row.stores += 1;
      row.weight += signal.score_contribution;
      rows.set(signal.signal_code, row);
    }
  }
  return Array.from(rows.values()).sort((a, b) => b.weight - a.weight || b.stores - a.stores);
}

export function latestAnomalyDate(dataset: RetailSimulationDatasetData): string {
  return dataset.anomaly_manifest.reduce((max, anomaly) => anomaly.date_to > max ? anomaly.date_to : max, "") || dataset.date_to;
}

export type ClassificationSwitch = {
  classification: "production" | "simulated";
  datasetVersion?: string;
  asOf: string;
  sourceSystem?: string;
  clearToEmpty: boolean;
};

export function switchClassification(next: "production" | "simulated", latest: RetailSimulationDatasetData | null | undefined, currentAsOf: string, today: string): ClassificationSwitch {
  if (next === "production") return { classification: "production", asOf: today, clearToEmpty: false };
  if (!latest) return { classification: "simulated", asOf: currentAsOf || today, sourceSystem: "retail_simulator", clearToEmpty: true };
  return { classification: "simulated", datasetVersion: latest.dataset_version, asOf: latestAnomalyDate(latest), sourceSystem: "retail_simulator", clearToEmpty: false };
}

export function responsePartitions(response: RetailPulseResponse): RetailPulsePartition[] {
  if (response.partitions && response.partitions.length > 0) return response.partitions;
  return [{
    currency: response.currency || "",
    currency_status: response.currency_status,
    current: response.current,
    comparison: response.comparison,
    current_coverage: response.current_coverage,
    comparison_coverage: response.comparison_coverage,
    decision_ready: response.decision_ready,
    summary: response.summary,
    daily_trend: response.daily_trend || [],
    attention: response.attention || [],
    suppressed_attention: response.suppressed_attention,
    attention_count: response.attention_count,
  }];
}

export function trendValue(row: RetailPulsePartition["daily_trend"][number], code: PulseMetricCode): number | null {
  if (row.gap) return null;
  const value = row.kpis?.[code]?.value;
  return value === undefined ? null : value;
}

export function attentionSignalSummary(attention: RetailAttention, language: Language): string {
  return attention.observed_signals.map((signal) => `${signalLabel(signal.signal_code, language)} ${signal.observed_change === null ? "—" : `${signal.observed_change > 0 ? "+" : ""}${signal.observed_change.toFixed(1)}${signal.unit === "percentage_point" ? "pp" : "%"}`}`).join(" · ");
}
