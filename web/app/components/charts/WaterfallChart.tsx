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
  emptyText = "暂无瀑布图数据",
}: WaterfallChartProps) {
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
        {emptyText}
      </div>
    );
  }

  return (
    <div style={{ width: "100%", height }}>
      <ResponsiveContainer width="100%" height="100%">
        <BarChart
          data={chartData}
          margin={{ top: 16, right: 16, bottom: 8, left: 16 }}
        >
          <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="var(--border-default, #D9D9D9)" opacity={0.6} />
          <XAxis
            dataKey="label"
            tickLine={false}
            stroke="var(--fg-tertiary, #595959)"
            fontSize={11}
            tickMargin={6}
          />
          <YAxis
            tickLine={false}
            stroke="var(--fg-tertiary, #595959)"
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
                <div
                  style={{
                    background: "var(--bg-surface, #FFFFFF)",
                    boxShadow: "var(--shadow-dropdown, 0 4px 12px rgba(0,0,0,0.08))",
                    borderRadius: 6,
                    padding: "8px 12px",
                    fontSize: 12,
                    border: "1px solid var(--border-default, #D9D9D9)",
                  }}
                >
                  <div style={{ fontWeight: 600, color: "var(--fg-primary, #000000)", marginBottom: 4 }}>
                    {point.label}
                  </div>
                  <div style={{ color: point.displayValue >= 0 ? "var(--fg-success, #216E39)" : "var(--fg-error, #A8071A)" }}>
                    {point.isTotal ? "结算总额" : "变动金额"}: <strong>{fmtMoney(point.displayValue, currency)}</strong>
                  </div>
                  <div style={{ color: "var(--fg-tertiary, #595959)", marginTop: 2 }}>
                    累计净额: {fmtMoney(point.runningTotal, currency)}
                  </div>
                </div>
              );
            }}
          />

          {/* 隐形占位底座 */}
          <Bar dataKey="base" stackId="waterfall" fill="transparent" isAnimationActive={false} />

          {/* 真实瀑布阶梯柱 */}
          <Bar dataKey="value" stackId="waterfall" isAnimationActive={false} radius={[2, 2, 0, 0]}>
            {chartData.map((entry, idx) => {
              let fill = "var(--chart-blue, #1F4E9C)";
              if (entry.isTotal) {
                fill = "var(--mono-20, #262626)";
              } else if (entry.displayValue < 0) {
                fill = "var(--fg-error, #A8071A)";
              } else {
                fill = "var(--fg-success, #216E39)";
              }
              return <Cell key={`cell-${idx}`} fill={fill} />;
            })}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
