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
  period: string; // e.g. "2024", "2025", "2026" or "2024-Q1"
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
              <stop offset="5%" stopColor="#1F4E9C" stopOpacity={0.4} />
              <stop offset="95%" stopColor="#1F4E9C" stopOpacity={0.05} />
            </linearGradient>
            <linearGradient id="variableRentGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="#216E39" stopOpacity={0.4} />
              <stop offset="95%" stopColor="#216E39" stopOpacity={0.05} />
            </linearGradient>
            <linearGradient id="nonLeaseGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="#8A5300" stopOpacity={0.4} />
              <stop offset="95%" stopColor="#8A5300" stopOpacity={0.05} />
            </linearGradient>
          </defs>
          <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="var(--border-default, #D9D9D9)" opacity={0.6} />
          <XAxis
            dataKey="period"
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
            width={52}
          />
          <Tooltip
            content={({ active, payload, label }) => {
              if (!active || !payload || !payload.length) return null;
              const point = payload[0]?.payload as CashflowPeriodData | undefined;
              if (!point) return null;
              const total = (point.fixedRent || 0) + (point.variableRent || 0) + (point.nonLease || 0);
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
                    期间: {label}
                  </div>
                  <div style={{ color: "#1F4E9C", marginBottom: 2 }}>
                    固定租金: {fmtMoney(point.fixedRent, currency)}
                  </div>
                  <div style={{ color: "#216E39", marginBottom: 2 }}>
                    预计变动租金: {fmtMoney(point.variableRent, currency)}
                  </div>
                  <div style={{ color: "#8A5300", marginBottom: 2 }}>
                    非租赁物业成本: {fmtMoney(point.nonLease, currency)}
                  </div>
                  <div style={{ fontWeight: 600, color: "var(--fg-primary, #000000)", marginTop: 4, borderTop: "1px dashed var(--border-default, #D9D9D9)", paddingTop: 4 }}>
                    期间现金流总计: {fmtMoney(total, currency)}
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
              <span style={{ color: "var(--fg-secondary, #262626)", marginRight: 8 }}>
                {value === "fixedRent"
                  ? "固定租金支出"
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
            stroke="#1F4E9C"
            strokeWidth={1.8}
            fill="url(#fixedRentGrad)"
            name="fixedRent"
            isAnimationActive={false}
          />
          <Area
            type="monotone"
            dataKey="variableRent"
            stackId="cashflow"
            stroke="#216E39"
            strokeWidth={1.8}
            fill="url(#variableRentGrad)"
            name="variableRent"
            isAnimationActive={false}
          />
          <Area
            type="monotone"
            dataKey="nonLease"
            stackId="cashflow"
            stroke="#8A5300"
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
