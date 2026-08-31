"use client";

import type { ReactNode } from "react";
import { Card, Skeleton, Statistic } from "antd";
import { motion } from "framer-motion";
import { staggerItem } from "../../design-system/animations";
import { fmtMoney } from "../../lib/format";
import type { MoneySlice } from "./types";
import PageHeader from "../PageHeader";

interface KPICardProps {
  title: string;
  value: number;
  prefix: ReactNode;
  loading: boolean;
}

export function KPICard({ title, value, prefix, loading }: KPICardProps) {
  return (
    <motion.div variants={staggerItem}>
      <Card className="dashboard-kpi-card">
        {loading ? (
          <Skeleton active paragraph={false} title={{ width: "60%" }} />
        ) : (
          <Statistic
            title={
              <span className="dashboard-kpi-label">
                {title}
              </span>
            }
            value={value}
            prefix={<span className="dashboard-kpi-prefix">{prefix}</span>}
            className="dashboard-kpi-stat"
          />
        )}
      </Card>
    </motion.div>
  );
}

interface MoneyKPICardProps {
  title: string;
  value: MoneySlice[];
  subtitle?: string;
  loading: boolean;
}

export function MoneyKPICard({ title, value, subtitle, loading }: MoneyKPICardProps) {
  return (
    <motion.div variants={staggerItem}>
      <Card className="dashboard-money-card">
        {loading ? (
          <Skeleton active paragraph={false} title={{ width: "60%" }} />
        ) : (
          <>
            <div className="dashboard-money-label">
              {title}
            </div>
            <div className="dashboard-money-stack">
              {value.length === 0 ? (
                <div className="dashboard-money-empty">—</div>
              ) : value.map((slice) => (
                <div key={slice.currency} title={fmtMoney(slice.value, slice.currency)} className="money-kpi-value-line">
                  <span className="dashboard-money-value">
                    {slice.value === 0 ? "0.00" : compactMoney(slice.value, slice.currency)}
                  </span>
                  <span className="dashboard-money-currency">{slice.currency || "—"}</span>
                </div>
              ))}
            </div>
            {subtitle && (
              <div className="dashboard-money-subtitle">{subtitle}</div>
            )}
          </>
        )}
      </Card>
    </motion.div>
  );
}

function compactMoney(value: number, currency: string): string {
  const locale = currency === "CNY" || currency === "HKD" ? "zh-CN" : "en-US";
  const formatted = new Intl.NumberFormat(locale, { notation: "compact", maximumFractionDigits: 2 }).format(Math.abs(value));
  return value < 0 ? `(${formatted})` : formatted;
}

interface ChartCardProps {
  title: string;
  children: ReactNode;
  extra?: ReactNode;
}

export function ChartCard({ title, children, extra }: ChartCardProps) {
  return (
    <Card
      title={<span className="dashboard-chart-title">{title}</span>}
      extra={extra}
      className="dashboard-chart-card"
    >
      {children}
    </Card>
  );
}

interface DashboardHeaderProps {
  title: string;
  subtitle: string;
  // Both actions are optional: a page may legitimately offer one or none.
  // PageHeader already renders nothing for an absent action.
  primaryAction?: ReactNode;
  secondaryAction?: ReactNode;
}

export function DashboardHeader({
  title,
  subtitle,
  primaryAction,
  secondaryAction,
}: DashboardHeaderProps) {
  return <PageHeader title={title} primaryAction={primaryAction} secondaryAction={secondaryAction} />;
}
