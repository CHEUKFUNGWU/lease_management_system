"use client";

import { Button, Empty, Skeleton } from "antd";
import { ArrowRightOutlined } from "@ant-design/icons";
import {
  PieChart, Pie, Cell, ResponsiveContainer, Tooltip,
  LineChart, Line, XAxis, YAxis, CartesianGrid, Legend,
} from "recharts";
import { t, type Language } from "../../lib/i18n";
import { fmtNum } from "../../lib/format";
import { ChartCard } from "./DashboardCards";
import type { DashboardStatusDatum, DashboardTooltipDatum, LiabilityTrendPoint } from "./types";

const PIE_COLORS = ["#000000", "#595959", "#BFBFBF", "#E5E5E5"];

interface TooltipProps {
  active?: boolean;
  payload?: DashboardTooltipDatum[];
}

function StatusPieTooltip({ active, payload, language }: TooltipProps & { language: Language }) {
  if (!active || !payload?.length) {
    return null;
  }

  const data = payload[0];
  return (
    <div
      style={{
        background: "#fff",
        border: "1px solid #E5E5E5",
        borderRadius: 8,
        padding: "8px 12px",
        boxShadow: "0 0 0 1px rgba(0,0,0,0.04), 0 4px 12px rgba(0,0,0,0.06)",
        fontSize: 13,
      }}
    >
      <span style={{ fontWeight: 600 }}>{data.name}</span>
      <span style={{ marginLeft: 8, color: "#595959" }}>
        {data.value} {t("dashboard.copies", language)}
      </span>
    </div>
  );
}

function compactAmount(value: number, language: Language): string {
  return new Intl.NumberFormat(language === "en" ? "en" : "zh-CN", {
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(value);
}

function TrendTooltip({
  active,
  payload,
  label,
}: {
  active?: boolean;
  payload?: { name: string; value?: number; color?: string }[];
  label?: string;
}) {
  if (!active || !payload?.length) {
    return null;
  }
  return (
    <div
      style={{
        background: "#fff",
        border: "1px solid #E5E5E5",
        borderRadius: 8,
        padding: "8px 12px",
        boxShadow: "0 0 0 1px rgba(0,0,0,0.04), 0 4px 12px rgba(0,0,0,0.06)",
        fontSize: 13,
      }}
    >
      <div style={{ fontWeight: 600, marginBottom: 4 }}>{label}</div>
      {payload.map((item) => (
        <div key={item.name} style={{ display: "flex", alignItems: "center", gap: 6 }}>
          <span style={{ width: 8, height: 8, borderRadius: 2, background: item.color }} />
          <span style={{ color: "#595959" }}>{item.name}</span>
          <span style={{ fontWeight: 600, marginLeft: "auto", paddingLeft: 12 }}>{fmtNum(item.value)}</span>
        </div>
      ))}
    </div>
  );
}

export function LiabilityTrendCard({
  language,
  data,
  loading,
  onOpenReports,
}: {
  language: Language;
  data: LiabilityTrendPoint[];
  loading: boolean;
  onOpenReports: () => void;
}) {
  const hasData = data.length > 0;

  return (
    <ChartCard
      title={t("dashboard.liability_trend", language)}
      extra={
        <Button type="link" size="small" style={{ fontSize: 13 }} onClick={onOpenReports}>
          {t("dashboard.view_reports", language)} <ArrowRightOutlined />
        </Button>
      }
    >
      <div style={{ height: 280 }}>
        {loading ? (
          <Skeleton active paragraph={{ rows: 5 }} />
        ) : hasData ? (
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={data} margin={{ top: 8, right: 8, bottom: 0, left: 8 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#F0F0F0" vertical={false} />
              <XAxis
                dataKey="period"
                tick={{ fontSize: 11, fill: "#8C8C8C" }}
                tickLine={false}
                axisLine={{ stroke: "#E5E5E5" }}
                minTickGap={24}
              />
              <YAxis
                tick={{ fontSize: 11, fill: "#8C8C8C" }}
                tickLine={false}
                axisLine={false}
                width={56}
                tickFormatter={(v: number) => compactAmount(v, language)}
              />
              <Tooltip content={<TrendTooltip />} />
              <Legend
                verticalAlign="top"
                height={28}
                iconType="plainline"
                wrapperStyle={{ fontSize: 12 }}
              />
              <Line
                type="monotone"
                dataKey="liability"
                name={t("dashboard.trend_liability", language)}
                stroke="#000000"
                strokeWidth={2}
                dot={false}
                activeDot={{ r: 4 }}
              />
              <Line
                type="monotone"
                dataKey="rou"
                name={t("dashboard.trend_rou", language)}
                stroke="#8C8C8C"
                strokeWidth={2}
                strokeDasharray="5 4"
                dot={false}
                activeDot={{ r: 4 }}
              />
            </LineChart>
          </ResponsiveContainer>
        ) : (
          <div style={{ height: "100%", display: "flex", alignItems: "center", justifyContent: "center" }}>
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("dashboard.no_liability_data", language)} />
          </div>
        )}
      </div>
    </ChartCard>
  );
}

export function ContractStatusCard({
  statusData,
  language,
}: {
  statusData: DashboardStatusDatum[];
  language: Language;
}) {
  const hasStatusData = statusData.length > 0;

  return (
    <ChartCard title={t("dashboard.contract_status", language)}>
      <div style={{ height: 280, display: "flex", alignItems: "center", justifyContent: "center" }}>
        {hasStatusData ? (
          <ResponsiveContainer width="100%" height="100%">
            <PieChart>
              <Pie
                data={statusData}
                cx="50%"
                cy="50%"
                innerRadius={60}
                outerRadius={90}
                paddingAngle={3}
                dataKey="value"
                stroke="none"
              >
                {statusData.map((entry, index) => (
                  <Cell key={entry.key} fill={PIE_COLORS[index % PIE_COLORS.length]} />
                ))}
              </Pie>
              <Tooltip content={<StatusPieTooltip language={language} />} />
            </PieChart>
          </ResponsiveContainer>
        ) : (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("dashboard.no_status_data", language)} />
        )}
      </div>

      {hasStatusData && (
        <div
          style={{
            display: "flex",
            justifyContent: "center",
            gap: 16,
            marginTop: -8,
            flexWrap: "wrap",
          }}
        >
          {statusData.map((item, index) => (
            <div key={item.key} style={{ display: "flex", alignItems: "center", gap: 6 }}>
              <div
                style={{
                  width: 8,
                  height: 8,
                  borderRadius: 2,
                  background: PIE_COLORS[index],
                }}
              />
              <span style={{ fontSize: 12, color: "#595959" }}>
                {item.name} ({item.value})
              </span>
            </div>
          ))}
        </div>
      )}
    </ChartCard>
  );
}
