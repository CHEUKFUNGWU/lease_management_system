"use client";

import { Alert, Card, Flex, Spin, Table, Tooltip, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import {
  storePnlApi,
  type RetailDataClassification,
  type RetailPlanComparison,
  type StorePnlAggregateResult,
  type StorePnlProjection,
  type StorePnlRowValue,
} from "../lib/api";
import { t, type Language } from "../lib/i18n";
import { fmtMoney } from "../lib/format";
import { tableScrollX } from "../lib/tableScroll";
import { useRetailQuery } from "../retail/useRetailQuery";
import { StateBlock } from "./StateBlock";
import { StatusTag } from "./StatusTag";

const DISPLAY_ROWS = new Set(["revenue", "gross_profit", "labor_cost", "non_lease_cost", "other_controllable", "other_controllable_cost", "fixed_rent", "variable_rent", "store_contribution"]);

type ComparisonRow = {
  key: string;
  group?: string;
  currency?: string;
  label: string;
  actual: number | null;
  budget: number | null;
  variance: number | null;
  variancePct: number | null;
  reason?: string;
};

type ProjectionResult =
  | { kind: "store"; pnl: StorePnlProjection }
  | { kind: "aggregate"; aggregate: StorePnlAggregateResult };

export interface PlanComparisonPanelProps {
  plan: RetailPlanComparison;
  currency?: string;
  language: Language;
  token: string | null;
  dataClassification: RetailDataClassification;
  datasetVersion?: string;
  asOf: string;
  storeId?: string;
  groupBy?: string;
}

export function PlanComparisonUnavailable({ period, currency, language, dataClassification, reason, actual }: {
  period?: string;
  currency?: string;
  language: Language;
  dataClassification: RetailDataClassification;
  reason: string;
  actual?: Record<string, number | null | undefined>;
}) {
  const rows: ComparisonRow[] = ["revenue", "gross_profit", "store_contribution"].map((key) => ({
    key,
    label: t(`retail.kpi.${key}`, language),
    actual: actual?.[key] ?? null,
    budget: null,
    variance: null,
    variancePct: null,
    reason,
  }));
  const columns: ColumnsType<ComparisonRow> = [
    { title: t("plan.metric", language), dataIndex: "label", key: "label" },
    { title: t("plan.actual", language), dataIndex: "actual", key: "actual", align: "right", render: (value: number | null) => value == null ? <Tooltip title={reason}>—</Tooltip> : money(value, currency || "") },
    { title: t("plan.budget", language), dataIndex: "budget", key: "budget", align: "right", render: () => <Tooltip title={reason}>—</Tooltip> },
    { title: t("plan.variance_amount", language), dataIndex: "variance", key: "variance", align: "right", render: () => <Tooltip title={reason}>—</Tooltip> },
    { title: t("plan.variance_rate", language), dataIndex: "variancePct", key: "variancePct", align: "right", render: () => <Tooltip title={reason}>—</Tooltip> },
  ];
  return (
    <Card size="small" className="plan-comparison-card plan-block-gap" title={
      <Flex justify="space-between" align="center" wrap="wrap" gap={8}>
        <span>{t("plan.title", language)}</span>
        <Flex gap={6} wrap="wrap" align="center">
          <StatusTag kind={dataClassification === "simulated" ? "warning" : "neutral"}>{t("plan.actual_classification", language)}: {dataClassification}</StatusTag>
          <StatusTag kind="neutral">{t("plan.budget_classification", language)}: —</StatusTag>
          <StatusTag kind="neutral">{period || "—"}</StatusTag>
          <StatusTag kind="neutral">{currency || "—"}</StatusTag>
        </Flex>
      </Flex>
    }>
      <Flex gap={8} wrap="wrap" align="center" className="plan-comparison-meta">
        <Typography.Text>{t("plan.budget_version_missing", language)}</Typography.Text>
      </Flex>
      <Alert type="warning" showIcon message={t("plan.not_available", language)} description={reason} className="plan-block-gap-sm" />
      <Table rowKey="key" size="small" pagination={false} columns={columns} dataSource={rows} scroll={tableScrollX(rows.length, 820)} />
    </Card>
  );
}

function money(value: number | null, currency: string): string {
  return fmtMoney(value, currency);
}

function reasonFor(row: StorePnlRowValue, fallback?: string): string | undefined {
  if (row.other == null) return fallback || "budget_missing";
  if (row.pct == null && row.variance != null) return "budget_zero_or_rate_unavailable";
  return fallback;
}

function projectionRows(pnl: StorePnlProjection, fallback?: string): ComparisonRow[] {
  return (pnl.operating?.rows || [])
    .filter((row) => DISPLAY_ROWS.has(row.key))
    .map((row) => ({
      key: row.key,
      currency: pnl.currency,
      label: row.label,
      actual: row.actual ?? null,
      budget: row.other ?? null,
      variance: row.variance ?? null,
      variancePct: row.pct == null ? null : row.pct * 100,
      reason: reasonFor(row, fallback || pnl.gaps?.join("; ")),
    }));
}

function aggregateRows(result: StorePnlAggregateResult): ComparisonRow[] {
  return result.groups.flatMap((group) => group.partitions.flatMap((partition) =>
    (partition.operating?.rows || [])
      .filter((row) => DISPLAY_ROWS.has(row.key))
      .map((row) => ({
        key: `${group.key}-${partition.currency}-${row.key}`,
        group: group.key || "—",
        currency: partition.currency,
        label: row.label,
        actual: row.actual ?? null,
        budget: row.other ?? null,
        variance: row.variance ?? null,
        variancePct: row.pct == null ? null : row.pct * 100,
        reason: reasonFor(row, partition.gaps?.join("; ")),
      })),
  ));
}

function summaryRows(plan: RetailPlanComparison, language: Language): ComparisonRow[] {
  return plan.variances
    .filter((variance) => DISPLAY_ROWS.has(variance.kpi))
    .map((variance) => ({
      key: variance.kpi,
      currency: plan.currency,
      label: t(`retail.kpi.${variance.kpi}`, language),
      actual: variance.actual ?? null,
      budget: variance.plan ?? null,
      variance: variance.variance ?? null,
      variancePct: variance.variance_pct ?? null,
      reason: variance.downgrade_reason || plan.downgrade_reason,
    }));
}

function daysInMonth(period: string): number {
  const [year, month] = period.split("-").map(Number);
  if (!Number.isInteger(year) || !Number.isInteger(month) || month < 1 || month > 12) return 1;
  return new Date(Date.UTC(year, month, 0)).getUTCDate();
}

export function PlanComparisonPanel(props: PlanComparisonPanelProps) {
  const { plan, currency, language, token, dataClassification, datasetVersion, asOf, storeId, groupBy } = props;
  const mode = groupBy === "region" || groupBy === "brand" ? "aggregate" : storeId ? "store" : "summary";
  // Pulse/360 can be on a rolling or month-to-date window while plan lines
  // are monthly. Do not widen the actual query to the whole month (which can
  // include future days relative to as_of); keep the honest downgraded view
  // with actuals and dashes until the user selects a comparable period.
  const periodMismatch = plan.downgrade_reason?.includes("actual_day_coverage_insufficient") ?? false;
  const params = mode === "summary" || !plan.plan_version_id || periodMismatch ? null : {
    store_id: storeId || "",
    group_by: groupBy,
    as_of: asOf,
    window_days: daysInMonth(plan.period),
    period: plan.period,
    basis: "operating",
    primary: "actual",
    secondary: "budget",
    plan_version_id: plan.plan_version_id,
    data_classification: dataClassification,
    dataset_version: datasetVersion,
  };
  const queryKey = `${mode}|${storeId || ""}|${groupBy || ""}|${plan.plan_version_id || ""}|${plan.period}|${asOf}|${dataClassification}|${datasetVersion || ""}`;
  const { loading, state, retry } = useRetailQuery<ProjectionResult, NonNullable<typeof params>>({
    token,
    params,
    paramsKey: queryKey,
    fetcher: async (query, authToken) => {
      if (mode === "aggregate") {
        const response = await storePnlApi.getAggregate({ ...query, group_by: query.group_by as "region" | "brand" }, authToken);
        return { kind: "aggregate", aggregate: response.aggregate };
      }
      const response = await storePnlApi.getPnl(query, authToken);
      return { kind: "store", pnl: response.pnl };
    },
  });
  const result = state.kind === "ready" ? state.data : undefined;
  const rows = periodMismatch
    ? summaryRows(plan, language).map((row) => ({ ...row, budget: null, variance: null, variancePct: null, reason: plan.downgrade_reason }))
    : result?.kind === "store"
    ? projectionRows(result.pnl, plan.downgrade_reason)
    : result?.kind === "aggregate"
      ? aggregateRows(result.aggregate)
      : mode === "summary" ? summaryRows(plan, language) : [];
  const mixedCurrency = !periodMismatch && result?.kind === "aggregate" && result.aggregate.groups.some((group) => group.mixed_currency);
  const showGroup = !periodMismatch && result?.kind === "aggregate";
  const budgetClassification = plan.plan_data_classification || "—";
  const columns: ColumnsType<ComparisonRow> = [
    ...(showGroup ? [{ title: groupBy === "brand" ? t("plan.brand", language) : t("plan.region", language), dataIndex: "group", key: "group" }] : []),
    { title: t("plan.metric", language), dataIndex: "label", key: "label" },
    { title: t("plan.actual", language), dataIndex: "actual", key: "actual", align: "right", render: (value: number | null, row) => money(value, row.currency || currency || "") },
    { title: t("plan.budget", language), dataIndex: "budget", key: "budget", align: "right", render: (value: number | null, row) => value == null ? <Tooltip title={row.reason || t("plan.missing_budget_reason", language)}>—</Tooltip> : money(value, row.currency || currency || "") },
    { title: t("plan.variance_amount", language), dataIndex: "variance", key: "variance", align: "right", render: (value: number | null, row) => value == null ? <Tooltip title={row.reason || t("plan.unavailable_reason", language)}>—</Tooltip> : money(value, row.currency || currency || "") },
    { title: t("plan.variance_rate", language), dataIndex: "variancePct", key: "variancePct", align: "right", render: (value: number | null, row) => value == null ? <Tooltip title={row.reason || t("plan.unavailable_reason", language)}>—</Tooltip> : `${value.toFixed(2)}%` },
  ];

  return (
    <Card
      size="small"
      className="plan-comparison-card plan-block-gap"
      title={
        <Flex justify="space-between" align="center" wrap="wrap" gap={8}>
          <span>{t("plan.title", language)}</span>
          <Flex gap={6} wrap="wrap" align="center">
            <StatusTag kind={dataClassification === "simulated" ? "warning" : "neutral"}>
              {t("plan.actual_classification", language)}: {dataClassification}
            </StatusTag>
            <StatusTag kind={budgetClassification === "simulated" ? "warning" : "neutral"}>
              {t("plan.budget_classification", language)}: {budgetClassification}
            </StatusTag>
            <StatusTag kind="neutral">{plan.period}</StatusTag>
            <StatusTag kind="neutral">{currency || plan.currency || "—"}</StatusTag>
          </Flex>
        </Flex>
      }
    >
      <Flex gap={8} wrap="wrap" align="center" className="plan-comparison-meta">
        <Typography.Text>{plan.plan_version_name || plan.plan_version_id || "—"}</Typography.Text>
        <Typography.Text type="secondary">{plan.plan_version_type || "budget"} · {plan.plan_source || "—"}</Typography.Text>
        {plan.plan_is_official && <StatusTag kind="processing">{t("plan_import.official", language)}</StatusTag>}
      </Flex>
      {!plan.decision_ready && plan.downgrade_reason && (
        <Alert className="plan-block-gap-sm" type="warning" showIcon message={t("plan.not_ready", language)} description={plan.downgrade_reason} />
      )}
      {mixedCurrency && (
        <Alert className="plan-block-gap-sm" type="warning" showIcon message={t("plan.mixed_currency_title", language)} description={`${t("plan.cross_currency_total", language)}: — · ${t("plan.mixed_currency_reason", language)}`} />
      )}
      {loading ? <Flex justify="center" className="plan-comparison-loading"><Spin /></Flex> : null}
      {mode !== "summary" && !periodMismatch && !loading ? <StateBlock state={state} language={language} onRetry={retry} /> : null}
      {!loading ? (
        <Table rowKey="key" size="small" pagination={false} columns={columns} dataSource={rows} scroll={tableScrollX(rows.length, showGroup ? 960 : 820)} locale={{ emptyText: t("plan.no_comparable_rows", language) }} />
      ) : null}
    </Card>
  );
}

export const planComparisonTestables = { projectionRows, aggregateRows, summaryRows };
