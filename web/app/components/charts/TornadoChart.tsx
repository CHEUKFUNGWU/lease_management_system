"use client";

import React, { useMemo } from "react";
import {
  ResponsiveContainer,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ReferenceLine,
} from "recharts";
import { useLanguage } from "../../context/LanguageContext";
import { t } from "../../lib/i18n";
import { fmtMoney } from "../../lib/format";

export interface TornadoFactor {
  name: string;
  lowValue: number; // Impact under adverse / low shock
  highValue: number; // Impact under favorable / high shock
  lowLabel?: string;
  highLabel?: string;
}

export interface TornadoChartProps {
  factors: TornadoFactor[];
  baseValue?: number;
  currency?: string;
  height?: number;
  emptyText?: string;
}

/**
 * 深度模块：敏感度龙卷风图 (Sensitivity Tornado Chart)
 * - 自动按影响跨度降序排列（最具杠杆的谈判抓手置顶）
 * - 双向水平对比呈现负向冲击与正向收益
 */
export default function TornadoChart({
  factors,
  baseValue = 0,
  currency = "CNY",
  height = 260,
  emptyText,
}: TornadoChartProps) {
  const { language } = useLanguage();
  const resolvedEmptyText = emptyText || t("sensitivity.empty", language);
  const chartData = useMemo(() => {
    const sorted = [...factors].sort((a, b) => {
      const swingA = Math.abs(a.highValue - a.lowValue);
      const swingB = Math.abs(b.highValue - b.lowValue);
      return swingB - swingA;
    });

    return sorted.map((f) => {
      const lowDelta = f.lowValue - baseValue;
      const highDelta = f.highValue - baseValue;
      return {
        name: f.name,
        lowDelta,
        highDelta,
        lowValue: f.lowValue,
        highValue: f.highValue,
        lowLabel: f.lowLabel || t("sensitivity.low_scenario", language),
        highLabel: f.highLabel || t("sensitivity.high_scenario", language),
      };
    });
  }, [factors, baseValue, language]);

  if (!factors || factors.length === 0) {
    return (
      <div className="chart-empty-state" style={{ height }}>
        {resolvedEmptyText}
      </div>
    );
  }

  return (
    <div className="chart-container" style={{ height }}>
      <ResponsiveContainer width="100%" height="100%">
        <BarChart
          layout="vertical"
          data={chartData}
          margin={{ top: 12, right: 24, bottom: 8, left: 24 }}
        >
          <CartesianGrid strokeDasharray="3 3" horizontal={false} stroke="var(--border-default)" opacity={0.6} />
          <XAxis
            type="number"
            tickLine={false}
            stroke="var(--fg-tertiary)"
            fontSize={11}
            tickFormatter={(v) => (Math.abs(Number(v)) >= 1000 ? `${(Number(v) / 1000).toFixed(0)}k` : String(v))}
          />
          <YAxis
            dataKey="name"
            type="category"
            tickLine={false}
            stroke="var(--fg-tertiary)"
            fontSize={12}
            width={110}
          />
          <ReferenceLine x={0} stroke="var(--mono-20)" strokeWidth={1.5} />
          <Tooltip
            content={({ active, payload }) => {
              if (!active || !payload || !payload.length) return null;
              const point = payload[0]?.payload;
              if (!point) return null;
              return (
                <div className="chart-tooltip is-compact">
                  <div className="chart-tooltip-title">{point.name}</div>
                  <div className="chart-tooltip-series is-negative">
                    {point.lowLabel}: {fmtMoney(point.lowValue, currency)} (
                    {point.lowDelta >= 0 ? "+" : ""}
                    {fmtMoney(point.lowDelta, currency)})
                  </div>
                  <div className="chart-tooltip-series is-positive">
                    {point.highLabel}: {fmtMoney(point.highValue, currency)} (
                    {point.highDelta >= 0 ? "+" : ""}
                    {fmtMoney(point.highDelta, currency)})
                  </div>
                </div>
              );
            }}
          />
          <Legend
            verticalAlign="top"
            align="right"
            iconType="circle"
            wrapperStyle={{ fontSize: 11, paddingBottom: 6 }}
            formatter={(value) => (
              <span className="chart-legend-label">
                {value === "lowDelta" ? t("sensitivity.low_impact", language) : t("sensitivity.high_impact", language)}
              </span>
            )}
          />
          <Bar
            dataKey="lowDelta"
            name="lowDelta"
            fill="var(--state-error-text)"
            opacity={0.85}
            radius={[2, 0, 0, 2]}
            isAnimationActive={false}
          />
          <Bar
            dataKey="highDelta"
            name="highDelta"
            fill="var(--state-success-text)"
            opacity={0.85}
            radius={[0, 2, 2, 0]}
            isAnimationActive={false}
          />
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
