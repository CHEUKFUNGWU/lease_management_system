"use client";

import { Space, Tag } from "antd";
import type { TableProps } from "antd";
import { t, type Language } from "../../../lib/i18n";
import type { MonthlyEntry, PaymentSchedule } from "../../../lib/types/contracts";

export function buildScheduleColumns(
  language: Language
): TableProps<PaymentSchedule>["columns"] {
  return [
    { title: t("contract.payment_date", language), dataIndex: "due_date", width: 110 },
    {
      title: t("contract.amount", language),
      dataIndex: "amount",
      width: 120,
      render: (value: number) => `¥${value.toLocaleString()}`,
    },
    { title: t("contract.currency", language), dataIndex: "currency", width: 80 },
    {
      title: t("contract.payment_timing", language),
      dataIndex: "payment_timing",
      width: 90,
      render: (value: string) =>
        value === "prepaid" ? (
          <Tag color="processing">{t("contract.prepaid", language)}</Tag>
        ) : (
          <Tag color="success">{t("contract.postpaid", language)}</Tag>
        ),
    },
    { title: t("contract.amount_type", language), dataIndex: "amount_type", width: 120 },
    {
      title: `${t("contract.fixed", language)}/${t("contract.variable", language)}`,
      width: 100,
      render: (_: unknown, record: PaymentSchedule) => (
        <Space>
          {record.is_fixed && <Tag>{t("contract.fixed", language)}</Tag>}
          {record.is_variable && <Tag color="warning">{t("contract.variable", language)}</Tag>}
        </Space>
      ),
    },
    {
      title: t("contract.lease_component", language),
      width: 90,
      render: (_: unknown, record: PaymentSchedule) =>
        record.is_lease_component ? (
          <Tag color="success">{t("contract.yes", language)}</Tag>
        ) : (
          <Tag>{t("contract.no", language)}</Tag>
        ),
    },
    {
      title: t("contract.include_liability", language),
      width: 90,
      render: (_: unknown, record: PaymentSchedule) =>
        record.included_in_liability_pv ? (
          <Tag color="success">{t("contract.yes", language)}</Tag>
        ) : (
          <Tag>{t("contract.no", language)}</Tag>
        ),
    },
  ];
}

export function buildCalculationColumns(
  language: Language
): TableProps<MonthlyEntry>["columns"] {
  return [
    {
      title: t("contract.period", language),
      render: (_: unknown, record: MonthlyEntry) =>
        `${record.Year}-${String(record.Month).padStart(2, "0")}`,
      width: 90,
    },
    {
      title: t("contract.opening_liability", language),
      dataIndex: "OpeningLiability",
      render: (value: number) =>
        `¥${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right",
    },
    {
      title: t("contract.interest_expense", language),
      dataIndex: "InterestExpense",
      render: (value: number) =>
        `¥${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right",
    },
    {
      title: t("contract.payment", language),
      dataIndex: "TotalPayments",
      render: (value: number) =>
        `¥${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right",
    },
    {
      title: t("contract.closing_liability", language),
      dataIndex: "ClosingLiability",
      render: (value: number) =>
        `¥${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right",
    },
    {
      title: t("contract.opening_rou", language),
      dataIndex: "OpeningROUAsset",
      render: (value: number) =>
        `¥${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right",
    },
    {
      title: t("contract.depreciation", language),
      dataIndex: "Depreciation",
      render: (value: number) =>
        `¥${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right",
    },
    {
      title: t("contract.closing_rou", language),
      dataIndex: "ClosingROUAsset",
      render: (value: number) =>
        `¥${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right",
    },
    {
      title: t("contract.variable_rent", language),
      dataIndex: "VariableRentExpense",
      render: (value: number) =>
        `¥${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right",
    },
    {
      title: "豁免费用",
      dataIndex: "ExemptLeaseExpense",
      render: (value: number) =>
        `¥${(value || 0).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right",
    },
    {
      title: t("contract.non_lease_expense", language),
      dataIndex: "NonLeaseExpense",
      render: (value: number) =>
        `¥${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right",
    },
  ];
}
