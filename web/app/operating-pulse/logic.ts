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

export const KPI_LABELS: Record<PulseMetricCode, string> = {
  revenue: "销售额",
  gross_profit: "毛利额",
  gross_margin_rate: "毛利率",
  footfall: "客流",
  conversion_rate: "转化率",
  store_contribution: "门店贡献",
  average_transaction_value: "客单价",
  labor_cost_rate: "人工成本率",
  occupancy_cash_cost_rate: "经营占用成本率",
  store_contribution_margin: "门店贡献率",
  sales_per_sqm: "期间坪效",
};

export const SIGNAL_LABELS: Record<string, string> = {
  revenue_decline: "销售额下降",
  footfall_decline: "客流下降",
  footfall_continuous_decline: "连续客流下降",
  conversion_drop: "转化率下降",
  conversion_rate_drop: "转化率下降",
  average_ticket_drop: "客单价下降",
  gross_margin_compression: "毛利率收窄",
  labor_cost_rate_spike: "人工成本率上升",
  labor_cost_spike: "人工成本率上升",
  occupancy_cost_rate_spike: "经营占用成本率上升",
  occupancy_cost_burden: "经营占用成本率上升",
  contribution_turns_negative: "门店贡献转负",
};

export const metricUnitLabel = (unit: string, currency?: string): string => {
  if (unit === "currency") return currency || "金额";
  if (unit === "currency_per_sqm") return `${currency || "金额"}/㎡`;
  if (unit === "percent") return "%";
  if (unit === "count") return "笔/人次";
  return unit;
};

export function formatUnitValue(value: number | null | undefined, unit: string, currency?: string): string {
  if (value === null || value === undefined) return "—";
  const precision = unit === "count" ? 0 : 2;
  const formatted = value.toLocaleString("zh-CN", { minimumFractionDigits: precision, maximumFractionDigits: precision });
  if (unit === "percent") return `${formatted}%`;
  if (unit === "percentage_point") return `${formatted}pp`;
  if (unit === "currency" || unit === "currency_per_sqm") return `${formatted} ${metricUnitLabel(unit, currency)}`;
  if (unit === "count") return `${formatted} ${metricUnitLabel(unit, currency)}`;
  return `${formatted} ${unit}`;
}

export function formatSignalValue(value: number | null | undefined, unit: string, currency?: string): string {
  return formatUnitValue(value, unit, currency);
}

export function formatKPIValue(value: RetailKPIValue | null | undefined, currency?: string): string {
  return value ? formatUnitValue(value.value, value.unit, currency) : "—";
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

export function metricStatusLabel(metric: RetailSummaryMetric | null | undefined): { label: "完整" | "部分" | "缺失"; reason?: string } {
  if (!metric) return { label: "缺失", reason: "指标不可用" };
  const statuses = [metric.current.status, metric.comparison.status];
  const reasons = [metric.current.reason, metric.comparison.reason, metric.reason].filter(Boolean);
  if (statuses.some((status) => status === "unavailable")) return { label: "缺失", reason: reasons.join(" / ") || "所需事实不可用" };
  if (statuses.some((status) => status === "partial")) return { label: "部分", reason: reasons.join(" / ") || "覆盖不足或字段不完整" };
  return { label: "完整", reason: reasons.join(" / ") || undefined };
}

export function signalLabel(code: string): string {
  return SIGNAL_LABELS[code] || code;
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

export function attentionSignalSummary(attention: RetailAttention): string {
  return attention.observed_signals.map((signal) => `${signalLabel(signal.signal_code)} ${signal.observed_change === null ? "—" : `${signal.observed_change > 0 ? "+" : ""}${signal.observed_change.toFixed(1)}${signal.unit === "percentage_point" ? "pp" : "%"}`}`).join(" · ");
}
