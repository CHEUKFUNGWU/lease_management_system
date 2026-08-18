"use client";

import React, { useMemo } from "react";
import {
  ResponsiveContainer,
  ComposedChart,
  Area,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
} from "recharts";
import { fmtMoney, fmtPct } from "../../lib/format";

export interface ConfidenceBandPoint {
  date: string;
  value: number | null;
  p25?: number | null;
  p75?: number | null;
  median?: number | null;
}

export interface ConfidenceBandChartProps {
  data: ConfidenceBandPoint[];
  metricLabel: string;
  unit?: string; // "currency" | "percent" | "count" | string
  height?: number;
  currency?: string;
  emptyText?: string;
}

/**
 * 深度模块：置信安全带趋势图 (Confidence Band Trend Chart)
 * - 封装同群 P25~P75 灰色正常波动区间（安全带）
 * - 呈现基准中位数虚线与本店真实走势渐变主线
 * - 严格保留数据缺口（null 隔离），杜绝虚假连线
 */
export default function ConfidenceBandChart({
  data,
  metricLabel,
  unit = "currency",
  height = 300,
  currency = "CNY",
  emptyText = "暂无走势数据",
}: ConfidenceBandChartProps) {
  const chartData = useMemo(() => {
    return data.map((d) => {
      const hasBand = d.p25 != null && d.p75 != null && d.p75 >= d.p25;
      return {
        ...d,
        bandBase: hasBand ? d.p25 : null,
        bandRange: hasBand ? (d.p75 as number) - (d.p25 as number) : null,
      };
    });
  }, [data]);

  const formatValue = (val: number | null | undefined) => {
    if (val == null) return "—";
    if (unit === "percent") return fmtPct(val);
    if (unit === "currency" || unit === "CNY" || unit === "HKD" || unit === "USD") {
      return fmtMoney(val, currency);
    }
    return `${val.toLocaleString()} ${unit}`;
  };

  if (!data || data.length === 0) {
    return (
      <div
        className="chart-empty-state"
        style={{ height }}
      >
        {emptyText}
      </div>
    );
  }

  return (
    <div style={{ width: "100%", height }}>
      <ResponsiveContainer width="100%" height="100%">
        <ComposedChart
          data={chartData}
          margin={{ top: 12, right: 24, bottom: 8, left: 12 }}
        >
          <defs>
            <linearGradient id="trendLineGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="var(--chart-accent)" stopOpacity={0.25} />
              <stop offset="95%" stopColor="var(--chart-fill)" stopOpacity={0.0} />
            </linearGradient>
          </defs>
          <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="var(--border-subtle)" opacity={0.6} />
          <XAxis
            dataKey="date"
            tickLine={false}
            stroke="var(--fg-tertiary)"
            fontSize={11}
            tickMargin={6}
          />
          <YAxis
            tickLine={false}
            stroke="var(--fg-tertiary)"
            fontSize={11}
            tickFormatter={(v) => (unit === "percent" ? `${(v * 100).toFixed(0)}%` : Number(v) >= 1000 ? `${(Number(v) / 1000).toFixed(0)}k` : String(v))}
            width={48}
          />
          <Tooltip
            content={({ active, payload, label }) => {
              if (!active || !payload || !payload.length) return null;
              const point = payload[0]?.payload as ConfidenceBandPoint | undefined;
              if (!point) return null;
              return (
                <div
                  style={{
                    background: "var(--bg-surface)",
                    boxShadow: "0 4px 16px rgba(0,0,0,0.08)",
                    borderRadius: 8,
                    padding: "10px 14px",
                    fontSize: 12,
                    border: "1px solid var(--border-default)",
                  }}
                >
                  <div style={{ fontWeight: 600, color: "var(--fg-primary)", marginBottom: 4 }}>
                    {label}
                  </div>
                  <div style={{ color: "var(--fg-primary)", fontWeight: 500, marginBottom: 2 }}>
                    {metricLabel}: <strong>{formatValue(point.value)}</strong>
                  </div>
                  {point.median != null && (
                    <div style={{ color: "var(--fg-secondary)", marginBottom: 2 }}>
                      同群中位数: {formatValue(point.median)}
                    </div>
                  )}
                  {point.p25 != null && point.p75 != null && (
                    <div style={{ color: "var(--fg-muted)", fontSize: 11 }}>
                      正常波动带 (P25~P75): {formatValue(point.p25)} ~ {formatValue(point.p75)}
                    </div>
                  )}
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
              <span style={{ color: "var(--fg-secondary)", fontWeight: 500, marginRight: 8 }}>
                {value === "bandRange"
                  ? "同群置信安全带 (P25~P75)"
                  : value === "median"
                  ? "基准中位数"
                  : metricLabel}
              </span>
            )}
          />

          {/* 堆叠置信带：下边界透明，差值渲染清晰高辨识度安全带 */}
          <Area
            type="monotone"
            dataKey="bandBase"
            stackId="band"
            stroke="none"
            fill="transparent"
            isAnimationActive={false}
            legendType="none"
          />
          <Area
            type="monotone"
            dataKey="bandRange"
            stackId="band"
            stroke="none"
            fill="var(--chart-fill)"
            fillOpacity={0.65}
            name="bandRange"
            isAnimationActive={false}
          />

          {/* 基准中位数虚线：清晰高对比度对比基准 */}
          <Line
            type="monotone"
            dataKey="median"
            stroke="var(--chart-secondary)"
            strokeDasharray="4 4"
            strokeWidth={1.75}
            dot={false}
            name="median"
            isAnimationActive={false}
          />

          {/* 本店实际走势：主线 */}
          <Line
            type="monotone"
            dataKey="value"
            stroke="var(--chart-primary)"
            strokeWidth={2.5}
            dot={{ r: 3.5, fill: "var(--chart-primary)", stroke: "var(--bg-surface)", strokeWidth: 1.5 }}
            activeDot={{ r: 5.5, stroke: "var(--bg-surface)", strokeWidth: 2, fill: "var(--chart-primary)" }}
            name="value"
            connectNulls={false}
          />
        </ComposedChart>
      </ResponsiveContainer>
    </div>
  );
}
