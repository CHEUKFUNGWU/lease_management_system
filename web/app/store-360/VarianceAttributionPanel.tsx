"use client";

// R2-3: profit variance attribution panel (RH5). All numbers come from
// GET /api/v1/retail/store-variance-attribution - zero client-side math;
// the waterfall just draws the factor contributions the backend produced.
//
// Three hard rules land here:
//   order echo        -> the order note is persistent (outside data branches);
//   residual unspread -> residual renders as its own line, material adds a warning;
//   all-or-nothing    -> unavailable lists the missing fields, no half chart.

import React, { useEffect } from "react";
import { Alert, Card, Space, Typography } from "antd";
import { BarChart, Bar, XAxis, YAxis, Tooltip as ChartTooltip, ResponsiveContainer, Cell } from "recharts";
import { useLanguage } from "../context/LanguageContext";
import { useAuth } from "../context/AuthContext";
import { t } from "../lib/i18n";
import { fmtMoney } from "../lib/format";
import { retailVarianceApi, type VarianceAttributionResult } from "../lib/api";
import { useRetailQuery } from "../retail/useRetailQuery";
import { notifyError } from "../lib/notify";

const { Text } = Typography;

export interface VarianceAttributionPanelProps {
  storeId: string;
  asOf: string;
  windowDays: number;
  classification: string;
  datasetVersion?: string;
  sourceSystem?: string;
  currency?: string;
}

const FACTOR_LABEL_KEYS: Record<string, string> = {
  footfall: "store360.attribution.f.footfall",
  conversion_rate: "store360.attribution.f.conversion",
  average_transaction_value: "store360.attribution.f.ticket",
  gross_margin_rate: "store360.attribution.f.margin",
  labor_cost: "store360.attribution.f.labor",
  occupancy_cost: "store360.attribution.f.occupancy",
  other_controllable_cost: "store360.attribution.f.other",
};

export function factorLabel(factor: string, language: Parameters<typeof t>[1]): string {
  const key = FACTOR_LABEL_KEYS[factor];
  return key ? t(key, language) : factor;
}

export default function VarianceAttributionPanel({ storeId, asOf, windowDays, classification, datasetVersion, sourceSystem, currency }: VarianceAttributionPanelProps) {
  const { language } = useLanguage();
  const { token } = useAuth();

  const ready = Boolean(token && storeId && asOf && windowDays);
  const params = ready
    ? {
        store_id: storeId,
        data_classification: classification,
        dataset_version: datasetVersion,
        as_of: asOf,
        window_days: windowDays,
        source_system: sourceSystem,
      }
    : null;
  const paramsKey = [storeId, classification, datasetVersion, asOf, windowDays, sourceSystem].join("|");
  const { loading, state, retry } = useRetailQuery({
    token,
    params,
    paramsKey,
    fetcher: (p, tk) => retailVarianceApi.attribution(p, tk),
  });

  useEffect(() => {
    if (state.kind === "failed") notifyError(`${state.message ?? "request failed"}`);
  }, [state, language]);

  const result = state.kind === "ready" ? (state.data ?? null) : null;

  return (
    <Card
      size="small"
      title={<span>{t("store360.attribution.title", language)}</span>}
      extra={
        <Space size={8}>
          {result?.status === "complete" && (
            <Text type="secondary" className="variance-total-text">
              {fmtMoney(result.total_variance, currency)}
            </Text>
          )}
          <a onClick={() => retry()} role="button">{t("common.refresh", language)}</a>
        </Space>
      }
    >
      <AttributionView result={result} loading={loading} currency={currency} />
    </Card>
  );
}

export interface AttributionViewProps {
  result: VarianceAttributionResult | null;
  loading?: boolean;
  currency?: string;
}

/** Pure presentation: three branches (complete waterfall / material warning / unavailable list). */
export function AttributionView({ result, loading = false, currency }: AttributionViewProps) {
  const { language } = useLanguage();

  // Waterfall rows: base column -> floating factor columns (cumulative) -> current column.
  const rows: { name: string; base: number; delta: number; isEndpoint: boolean; negative: boolean }[] = [];
  if (result && result.status === "complete") {
    let running = result.base_profit;
    rows.push({ name: t("store360.attribution.base", language), base: 0, delta: result.base_profit, isEndpoint: true, negative: false });
    for (const f of result.factors) {
      const from = running;
      running += f.effect;
      rows.push({
        name: factorLabel(f.factor, language),
        base: Math.min(from, running),
        delta: Math.abs(f.effect),
        isEndpoint: false,
        negative: f.effect < 0,
      });
    }
    rows.push({ name: t("store360.attribution.current", language), base: 0, delta: result.current_profit, isEndpoint: true, negative: false });
  }

  return (
    <>
      {/* Order note is persistent: rendered outside every data branch */}
      <div className="variance-order-note">{t("store360.attribution.order", language)}</div>

      {loading ? (
        <div className="variance-chart-frame variance-loading" />
      ) : !result || result.status !== "complete" ? (
        <Alert type="warning" showIcon message={t("store360.attribution.unavailable", language, { fields: (result?.missing_facts ?? []).join(", ") || "—" })} />
      ) : (
        <>
          {result.residual_material && (
            <Alert type="warning" showIcon className="variance-material-warning" message={t("store360.attribution.material_warning", language)} />
          )}
          <div className="variance-chart-frame">
            <ResponsiveContainer width="100%" height={260}>
              <BarChart data={rows} margin={{ top: 8, right: 8, bottom: 0, left: 8 }}>
                <XAxis dataKey="name" tick={{ fontSize: 10 }} interval={0} />
                <YAxis hide domain={["dataMin", "dataMax"]} />
                <ChartTooltip
                  formatter={((value: unknown, name: unknown) => [
                    typeof value === "number" ? fmtMoney(value, currency) : String(value ?? ""),
                    String(name ?? ""),
                  ]) as never}
                />
                <Bar dataKey="base" stackId="wf" fill="transparent" isAnimationActive={false} />
                <Bar dataKey="delta" stackId="wf" isAnimationActive={false} radius={[2, 2, 0, 0]}>
                  {rows.map((row, i) => (
                    <Cell key={i} fill={row.isEndpoint ? "var(--chart-primary)" : row.negative ? "var(--chart-negative)" : "var(--chart-accent)"} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </div>
          <Space direction="vertical" size={4} className="variance-residual-block">
            <div>{t("store360.attribution.residual", language, {
              amount: fmtMoney(result.residual, currency),
              threshold: "5%",
            })}</div>
          </Space>
        </>
      )}
    </>
  );
}
