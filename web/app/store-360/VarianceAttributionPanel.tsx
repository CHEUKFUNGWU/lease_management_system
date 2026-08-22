"use client";

// R2-3：利润差异归因面板（RH5）。数据全部来自
// GET /api/v1/retail/store-variance-attribution——前端零计算，瀑布图只是
// 把后端给的因子贡献画出来。
//
// 三条死线的落点：
//   顺序回显 → order note 常驻（不随数据状态消失），文案即替代顺序本身；
//   残差不摊 → 残差行单独渲染，residual_material 时追加警示；
//   缺一不可用 → unavailable 时 StateBlock 列出缺失字段，不出半张图。

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

  // 瀑布图数据：基期柱 → 各因子浮动柱（累计）→ 当期柱。
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
    <Card size="small" title={<span>{t("store360.attribution.title", language)}</span>}
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
      {/* Order note is persistent: numbers change with order, so it always ships with the chart */}
      <div className="variance-order-note">{t("store360.attribution.order", language)}</div>

      {loading ? (
        <div className="variance-chart-frame variance-loading" />
      ) : !result || result.status !== "complete" ? (
        <>
          <Alert type="warning" showIcon message={t("store360.attribution.unavailable", language, { fields: (result?.missing_facts ?? []).join(", ") || "—" })} />
        </>
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
    </Card>
  );
}
