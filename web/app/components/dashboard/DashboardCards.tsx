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
      <Card styles={{ body: { padding: "20px 24px" } }} style={{ borderRadius: 10, height: "100%" }}>
        {loading ? (
          <Skeleton active paragraph={false} title={{ width: "60%" }} />
        ) : (
          <Statistic
            title={
              <span
                style={{
                  fontSize: 12,
                  fontWeight: 500,
                  color: "var(--fg-muted)",
                  textTransform: "uppercase",
                  letterSpacing: "0.02em",
                }}
              >
                {title}
              </span>
            }
            value={value}
            prefix={<span style={{ marginRight: 8, display: "inline-flex" }}>{prefix}</span>}
            valueStyle={{
              fontSize: 28,
              fontWeight: 600,
              letterSpacing: "-0.03em",
              color: "var(--fg-primary)",
            }}
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
      <Card styles={{ body: { padding: "20px 24px" } }} style={{ borderRadius: 10, height: "100%" }}>
        {loading ? (
          <Skeleton active paragraph={false} title={{ width: "60%" }} />
        ) : (
          <>
            <div
              style={{
                fontSize: 12,
                fontWeight: 500,
                color: "var(--fg-muted)",
                textTransform: "uppercase",
                letterSpacing: "0.02em",
                marginBottom: 4,
              }}
            >
              {title}
            </div>
            <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
              {value.length === 0 ? (
                <div style={{ fontSize: 26, fontWeight: 600, color: "var(--fg-primary)" }}>—</div>
              ) : value.map((slice) => (
                <div key={slice.currency} title={fmtMoney(slice.value, slice.currency)} style={{ display: "flex", alignItems: "baseline", gap: 8, fontVariantNumeric: "tabular-nums" }}>
                  <span style={{ fontSize: 26, fontWeight: 600, letterSpacing: "-0.03em", color: "var(--fg-primary)" }}>
                    {slice.value === 0 ? "0.00" : compactMoney(slice.value, slice.currency)}
                  </span>
                  <span style={{ fontSize: 12, color: "var(--fg-tertiary)", fontWeight: 600 }}>{slice.currency || "—"}</span>
                </div>
              ))}
            </div>
            {subtitle && (
              <div style={{ fontSize: 12, color: "var(--fg-muted)", marginTop: 2 }}>{subtitle}</div>
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
      title={<span style={{ fontSize: 15, fontWeight: 600, letterSpacing: "-0.01em" }}>{title}</span>}
      extra={extra}
      styles={{ body: { padding: "20px 24px 24px" } }}
      style={{ borderRadius: 10, height: "100%" }}
    >
      {children}
    </Card>
  );
}

interface DashboardHeaderProps {
  title: string;
  subtitle: string;
  primaryAction: ReactNode;
  secondaryAction: ReactNode;
}

export function DashboardHeader({
  title,
  subtitle,
  primaryAction,
  secondaryAction,
}: DashboardHeaderProps) {
  return <PageHeader title={title} subtitle={subtitle} primaryAction={primaryAction} secondaryAction={secondaryAction} />;
}
