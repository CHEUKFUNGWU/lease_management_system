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
  Cell,
} from "recharts";
import { useLanguage } from "../../context/LanguageContext";
import { t } from "../../lib/i18n";
import { fmtMoney } from "../../lib/format";

export interface WaterfallItem {
  label: string;
  value: number;
  isTotal?: boolean;
}

export interface WaterfallChartProps {
  items: WaterfallItem[];
  currency?: string;
  height?: number;
  emptyText?: string;
}

/**
 * 深度模块：损益逐级扣减瀑布图 (P&L Waterfall Chart)
 * - 自动计算浮动柱体起止底座与累计贡献
 * - 区分初始基准、增量贡献、扣减项与最终结余
 * - 严格遵循 Keynote 级单色/语义色规范
 */
export default function WaterfallChart({
  items,
  currency = "CNY",
  height = 280,
  emptyText,
}: WaterfallChartProps) {
  const { language } = useLanguage();
  const resolvedEmptyText = emptyText || t("chart.waterfall.empty", language);
  const chartData = useMemo(() => {
    let runningTotal = 0;
    return items.map((item, index) => {
      const isFirst = index === 0;
      const isTotal = item.isTotal || index === items.length - 1;
      const val = item.value;

      if (isFirst || isTotal) {
        runningTotal = isFirst ? val : runningTotal;
        return {
          label: item.label,
          base: 0,
          value: val,
          displayValue: val,
          isTotal: true,
          runningTotal: val,
        };
      }

      const prevTotal = runningTotal;
      runningTotal += val;
      const base = val < 0 ? runningTotal : prevTotal;
      const barHeight = Math.abs(val);

      return {
        label: item.label,
        base,
        value: barHeight,
        displayValue: val,
        isTotal: false,
        runningTotal,
      };
    });
  }, [items]);

  if (!items || items.length === 0) {
    return (
      <div
        className="chart-empty-state"
        style={{ height }}
      >
        {resolvedEmptyText}
      </div>
    );
  }

  return (
    <div className="chart-container" style={{ height }}>
      <ResponsiveContainer width="100%" height="100%">
        <BarChart
          data={chartData}
          margin={{ top: 16, right: 16, bottom: 8, left: 16 }}
          barCategoryGap="18%"
        >
          <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="var(--border-subtle)" opacity={0.6} />
          <XAxis
            dataKey="label"
            tickLine={false}
            stroke="var(--fg-tertiary)"
            fontSize={11}
            tickMargin={6}
          />
          <YAxis
            tickLine={false}
            stroke="var(--fg-tertiary)"
            fontSize={11}
            tickFormatter={(v) => (Math.abs(Number(v)) >= 1000 ? `${(Number(v) / 1000).toFixed(0)}k` : String(v))}
            width={48}
          />
          <Tooltip
            content={({ active, payload }) => {
              if (!active || !payload || !payload.length) return null;
              const point = payload[0]?.payload;
              if (!point) return null;
              return (
                <div className="chart-tooltip is-compact">
                  <div className="chart-tooltip-title">{point.label}</div>
                  <div className={`chart-tooltip-series ${point.displayValue >= 0 ? "is-positive" : "is-negative"}`}>
                    {point.isTotal ? t("chart.waterfall.total", language) : t("chart.waterfall.change", language)}: <strong>{fmtMoney(point.displayValue, currency)}</strong>
                  </div>
                  <div className="chart-tooltip-secondary">
                    {t("chart.waterfall.running_total", language)}: {fmtMoney(point.runningTotal, currency)}
                  </div>
                </div>
              );
            }}
          />

          {/* 隐形占位底座 */}
          <Bar dataKey="base" stackId="waterfall" fill="transparent" isAnimationActive={false} maxBarSize={36} />

          {/* 真实瀑布阶梯柱 */}
          <Bar dataKey="value" stackId="waterfall" isAnimationActive={false} radius={[3, 3, 0, 0]} maxBarSize={36}>
            {chartData.map((entry, idx) => {
              let fill = "var(--chart-primary)";
              if (entry.isTotal) {
                fill = "var(--chart-primary)";
              } else if (entry.displayValue < 0) {
                fill = "var(--chart-negative)";
              } else {
                fill = "var(--chart-accent)";
              }
              return <Cell key={`cell-${idx}`} fill={fill} />;
            })}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
