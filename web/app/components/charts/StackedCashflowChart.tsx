"use client";

import React from "react";
import {
  ResponsiveContainer,
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
} from "recharts";
import { useLanguage } from "../../context/LanguageContext";
import { t } from "../../lib/i18n";
import { fmtMoney } from "../../lib/format";

export interface CashflowPeriodData {
  month: string; // e.g. "2024", "2025", "2026" or "2024-Q1"
  fixedRent: number;
  variableRent: number;
  nonLease: number;
  total?: number;
}

export interface StackedCashflowChartProps {
  data: CashflowPeriodData[];
  currency?: string;
  height?: number;
  emptyText?: string;
}

/**
 * 深度模块：到期现金流堆叠面积图 (Stacked Cashflow Forecast Chart)
 * - 按合同到期年份/期间堆叠呈现固定租金、预估变动租金与非租成本
 * - 清晰展示未来各期间支付义务与占用成本结构
 */
export default function StackedCashflowChart({
  data,
  currency = "CNY",
  height = 300,
  emptyText,
}: StackedCashflowChartProps) {
  const { language } = useLanguage();
  const resolvedEmptyText = emptyText || t("cashflow.empty_title", language);

  if (!data || data.length === 0) {
    return <div className="chart-empty-state" style={{ height }}>{resolvedEmptyText}</div>;
  }

  return (
    <div className="chart-container" style={{ height }}>
      <ResponsiveContainer width="100%" height="100%">
        <AreaChart
          data={data}
          margin={{ top: 16, right: 24, bottom: 8, left: 16 }}
        >
          <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="var(--border-subtle)" opacity={0.6} />
          <XAxis
            dataKey="month"
            tickLine={false}
            stroke="var(--fg-tertiary)"
            fontSize={11}
            tickMargin={4}
          />
          <YAxis
            tickLine={false}
            stroke="var(--fg-tertiary)"
            fontSize={11}
            tickFormatter={(v) => (Math.abs(Number(v)) >= 1000 ? `${(Number(v) / 1000).toFixed(0)}k` : String(v))}
            width={52}
          />
          <Tooltip
            content={({ active, payload, label }) => {
              if (!active || !payload || !payload.length) return null;
              const fixed = Number(payload.find((p) => p.dataKey === "fixedRent")?.value || 0);
              const variable = Number(payload.find((p) => p.dataKey === "variableRent")?.value || 0);
              const nonLease = Number(payload.find((p) => p.dataKey === "nonLease")?.value || 0);
              const total = fixed + variable + nonLease;

              return (
                <div className="chart-tooltip is-compact">
                  <div className="chart-tooltip-title">{t("cashflow.col_period", language)}: {label}</div>
                  <div className="chart-tooltip-series is-blue">
                    {t("cashflow.stat_fixed_rent", language)}: <strong>{fmtMoney(fixed, currency)}</strong>
                  </div>
                  <div className="chart-tooltip-series is-accent">
                    {t("cashflow.stat_variable_rent", language)}: <strong>{fmtMoney(variable, currency)}</strong>
                  </div>
                  <div className="chart-tooltip-series is-purple">
                    {t("cashflow.stat_non_lease", language)}: <strong>{fmtMoney(nonLease, currency)}</strong>
                  </div>
                  <div className="chart-tooltip-total">
                    {t("cashflow.stat_total_outflow", language)}: <strong>{fmtMoney(total, currency)}</strong>
                  </div>
                </div>
              );
            }}
          />
          <Legend
            verticalAlign="top"
            align="right"
            iconType="circle"
            wrapperStyle={{ fontSize: 11, paddingBottom: 8 }}
            formatter={(value) => (
              <span className="chart-legend-label">
                {value === "fixedRent"
                  ? t("cashflow.stat_fixed_rent", language)
                  : value === "variableRent"
                  ? t("cashflow.stat_variable_rent", language)
                  : t("cashflow.stat_non_lease", language)}
              </span>
            )}
          />
          <Area
            type="monotone"
            dataKey="fixedRent"
            stackId="cashflow"
            stroke="var(--chart-blue)"
            strokeWidth={1.8}
            fill="var(--chart-blue)"
            fillOpacity={0.22}
            name="fixedRent"
            isAnimationActive={false}
          />
          <Area
            type="monotone"
            dataKey="variableRent"
            stackId="cashflow"
            stroke="var(--chart-accent)"
            strokeWidth={1.8}
            fill="var(--chart-accent)"
            fillOpacity={0.22}
            name="variableRent"
            isAnimationActive={false}
          />
          <Area
            type="monotone"
            dataKey="nonLease"
            stackId="cashflow"
            stroke="var(--chart-purple)"
            strokeWidth={1.8}
            fill="var(--chart-purple)"
            fillOpacity={0.22}
            name="nonLease"
            isAnimationActive={false}
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
