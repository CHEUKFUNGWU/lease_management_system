"use client";

// R0-1：/performance 的设备事实块。三种互斥分支，判定走 resolveBasis 接缝
// （web/app/lib/displayBasis.ts，模块设计 RH2 / D-R11）：
//
//   1. items 为空        → perf.empty.equipment（本期没有导入设备事实 → 去导入）
//   2. 有行但口径不可用   → perf.peer.unavailable_title/_body（这个源不是门店
//      口径，用户在这里做不了任何事；门店口径的同群对标在「门店 360」）
//   3. 口径可用          → 表格
//
// 分支 3 今天对这份 equipment 数据源走不到，但必须留着：删掉它 resolveBasis
// 的 usable:true 半边就没有任何行为差异，守卫的哨兵值断言也无从验起。
//
// 本组件不渲染任何「同群平均坪效达成率」列、不含「核心商圈」「标杆同群」
// 兜底——那两处兜底曾把空工厂代码伪装成商圈词（R0-1 修的正是这个）。

import React from "react";
import { Empty, Space, Table, Typography } from "antd";
import { StatusTag, statusKindFromAntColor } from "../components/StatusTag";
import { t, type Language } from "../lib/i18n";
import { tableScrollX } from "../lib/tableScroll";
import { resolveBasis } from "../lib/displayBasis";

export type PeerBenchmarkItem = {
  fact: {
    equipment_id: string;
    equipment_code: string;
    equipment_name: string;
    plant_code: string;
    production_line_code: string;
    currency: string;
    period: string;
    oee_pct?: number;
    utilization_pct?: number;
    actual_cost?: number;
    standard_cost?: number;
    reconciliation_status: string;
  };
  bridge?: { variance: number; residual: number; ties_out: boolean };
  missing?: string[];
};

/** 数据源事实口径：performanceApi.equipmentPerformance 只含设备事实。 */
const SOURCE_BASIS = "equipment" as const;
/** 页面展示语境：/performance 是零售门店经营驾驶舱。 */
const DISPLAY_CONTEXT = "retail_store" as const;

const money = (value?: number, currency?: string) =>
  value == null ? "—" : `${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}${currency ? ` ${currency}` : ""}`;
const pct = (value?: number) => (value == null ? "—" : `${value.toFixed(2)}%`);

export default function PeerBenchmarkBlock({ items, language }: { items: PeerBenchmarkItem[]; language: Language }) {
  const basis = resolveBasis("oee_pct", SOURCE_BASIS, DISPLAY_CONTEXT);

  if (items.length === 0) {
    return <Empty description={t("perf.empty.equipment", language)} />;
  }

  if (!basis.usable) {
    return (
      <Empty
        description={
          <Space direction="vertical" size={4}>
            <Typography.Text strong>{t("perf.peer.unavailable_title", language)}</Typography.Text>
            <Typography.Text type="secondary">{t("perf.peer.unavailable_body", language)}</Typography.Text>
          </Space>
        }
      />
    );
  }

  const columns = [
    { title: t("perf.col.equipment", language), key: "equipment", render: (_: unknown, row: PeerBenchmarkItem) => <Space direction="vertical" size={0}><strong>{row.fact.equipment_code || row.fact.equipment_name}</strong><Typography.Text type="secondary">{row.fact.plant_code || "—"} · {row.fact.production_line_code || "—"}</Typography.Text></Space> },
    { title: t("perf.col.utilization", language), key: "utilization", render: (_: unknown, row: PeerBenchmarkItem) => pct(row.fact.utilization_pct) },
    { title: t("perf.col.cost_variance", language), key: "variance", render: (_: unknown, row: PeerBenchmarkItem) => row.bridge ? money(row.bridge.variance, row.fact.currency) : "—" },
    { title: t("perf.col.residual", language), key: "residual", render: (_: unknown, row: PeerBenchmarkItem) => row.bridge ? money(row.bridge.residual, row.fact.currency) : "—" },
    { title: t("perf.col.data_status", language), key: "status", render: (_: unknown, row: PeerBenchmarkItem) => row.bridge?.ties_out ? <StatusTag kind="success">{t("perf.status.bridge_balanced", language)}</StatusTag> : <StatusTag kind={statusKindFromAntColor("warning")}>{t("perf.status.insufficient_evidence", language)}</StatusTag> },
  ];

  return (
    <Table
      rowKey={(row: PeerBenchmarkItem) => row.fact.equipment_id}
      size="small"
      columns={columns}
      dataSource={items}
      pagination={{ pageSize: 8 }}
      scroll={tableScrollX(items.length, 900)}
    />
  );
}
