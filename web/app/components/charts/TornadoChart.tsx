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
  emptyText = "暂无敏感度数据",
}: TornadoChartProps) {
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
        lowLabel: f.lowLabel || "悲观/下调",
        highLabel: f.highLabel || "乐观/上调",
      };
    });
  }, [factors, baseValue]);

  if (!factors || factors.length === 0) {
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
                <div
                  style={{
                    background: "var(--bg-surface)",
                    boxShadow: "var(--shadow-dropdown, 0 4px 12px rgba(0,0,0,0.08))",
                    borderRadius: 6,
                    padding: "8px 12px",
                    fontSize: 12,
                    border: "1px solid var(--border-default)",
                  }}
                >
                  <div style={{ fontWeight: 600, color: "var(--fg-primary)", marginBottom: 4 }}>
                    {point.name}
                  </div>
                  <div style={{ color: "var(--fg-error, #A8071A)", marginBottom: 2 }}>
                    {point.lowLabel}: {fmtMoney(point.lowValue, currency)} (
                    {point.lowDelta >= 0 ? "+" : ""}
                    {fmtMoney(point.lowDelta, currency)})
                  </div>
                  <div style={{ color: "var(--fg-success, #216E39)" }}>
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
              <span style={{ color: "var(--fg-secondary)", marginRight: 8 }}>
                {value === "lowDelta" ? "悲观/下调冲击" : "乐观/上调收益"}
              </span>
            )}
          />
          <Bar
            dataKey="lowDelta"
            name="lowDelta"
            fill="var(--fg-error, #A8071A)"
            opacity={0.85}
            radius={[2, 0, 0, 2]}
            isAnimationActive={false}
          />
          <Bar
            dataKey="highDelta"
            name="highDelta"
            fill="var(--fg-success, #216E39)"
            opacity={0.85}
            radius={[0, 2, 2, 0]}
            isAnimationActive={false}
          />
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
