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
  emptyText = "暂无现金流预测数据",
}: StackedCashflowChartProps) {
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
        <AreaChart
          data={data}
          margin={{ top: 16, right: 24, bottom: 8, left: 16 }}
        >
          <defs>
            <linearGradient id="fixedRentGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="var(--chart-blue)" stopOpacity={0.4} />
              <stop offset="95%" stopColor="var(--chart-blue)" stopOpacity={0.05} />
            </linearGradient>
            <linearGradient id="variableRentGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="var(--chart-accent)" stopOpacity={0.4} />
              <stop offset="95%" stopColor="var(--chart-accent)" stopOpacity={0.05} />
            </linearGradient>
            <linearGradient id="nonLeaseGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="var(--chart-purple)" stopOpacity={0.4} />
              <stop offset="95%" stopColor="var(--chart-purple)" stopOpacity={0.05} />
            </linearGradient>
          </defs>
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
                    月份: {label}
                  </div>
                  <div style={{ color: "var(--chart-blue)", marginBottom: 2 }}>
                    固定租金: <strong>{fmtMoney(fixed, currency)}</strong>
                  </div>
                  <div style={{ color: "var(--chart-accent)", marginBottom: 2 }}>
                    变动租金: <strong>{fmtMoney(variable, currency)}</strong>
                  </div>
                  <div style={{ color: "var(--chart-purple)", marginBottom: 2 }}>
                    非租及物业费: <strong>{fmtMoney(nonLease, currency)}</strong>
                  </div>
                  <div style={{ fontWeight: 600, color: "var(--fg-primary)", marginTop: 4, borderTop: "1px dashed var(--border-default)", paddingTop: 4 }}>
                    当月总支出: <strong>{fmtMoney(total, currency)}</strong>
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
              <span style={{ color: "var(--fg-secondary)", marginRight: 8 }}>
                {value === "fixedRent"
                  ? "固定租金"
                  : value === "variableRent"
                  ? "预计变动租金"
                  : "非租及物业费"}
              </span>
            )}
          />
          <Area
            type="monotone"
            dataKey="fixedRent"
            stackId="cashflow"
            stroke="var(--chart-blue)"
            strokeWidth={1.8}
            fill="url(#fixedRentGrad)"
            name="fixedRent"
            isAnimationActive={false}
          />
          <Area
            type="monotone"
            dataKey="variableRent"
            stackId="cashflow"
            stroke="var(--chart-accent)"
            strokeWidth={1.8}
            fill="url(#variableRentGrad)"
            name="variableRent"
            isAnimationActive={false}
          />
          <Area
            type="monotone"
            dataKey="nonLease"
            stackId="cashflow"
            stroke="var(--chart-purple)"
            strokeWidth={1.8}
            fill="url(#nonLeaseGrad)"
            name="nonLease"
            isAnimationActive={false}
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
