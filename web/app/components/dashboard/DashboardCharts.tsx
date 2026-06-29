"use client";

import { Button, Empty } from "antd";
import { ArrowRightOutlined } from "@ant-design/icons";
import { PieChart, Pie, Cell, ResponsiveContainer, Tooltip } from "recharts";
import { t, type Language } from "../../lib/i18n";
import { ChartCard } from "./DashboardCards";
import type { DashboardStatusDatum, DashboardTooltipDatum } from "./types";

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

export function LiabilityTrendCard({
  language,
  onOpenReports,
}: {
  language: Language;
  onOpenReports: () => void;
}) {
  return (
    <ChartCard
      title={t("dashboard.liability_trend", language)}
      extra={
        <Button type="link" size="small" style={{ fontSize: 13 }} onClick={onOpenReports}>
          {t("dashboard.view_reports", language)} <ArrowRightOutlined />
        </Button>
      }
    >
      <div style={{ height: 280, display: "flex", alignItems: "center", justifyContent: "center" }}>
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("dashboard.no_liability_data", language)} />
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
