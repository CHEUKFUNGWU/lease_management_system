"use client";

import type { ReactNode } from "react";
import { Card, Skeleton, Statistic } from "antd";
import { motion } from "framer-motion";
import { staggerItem } from "../../design-system/animations";

interface KPICardProps {
  title: string;
  value: number;
  prefix: ReactNode;
  loading: boolean;
}

export function KPICard({ title, value, prefix, loading }: KPICardProps) {
  return (
    <motion.div variants={staggerItem}>
      <Card bodyStyle={{ padding: "20px 24px" }} style={{ borderRadius: 10, height: "100%" }}>
        {loading ? (
          <Skeleton active paragraph={false} title={{ width: "60%" }} />
        ) : (
          <Statistic
            title={
              <span
                style={{
                  fontSize: 12,
                  fontWeight: 500,
                  color: "#8C8C8C",
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
              fontWeight: 700,
              letterSpacing: "-0.03em",
              color: "#000",
            }}
          />
        )}
      </Card>
    </motion.div>
  );
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
      bodyStyle={{ padding: "20px 24px 24px" }}
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
  return (
    <div
      style={{
        display: "flex",
        justifyContent: "space-between",
        alignItems: "flex-start",
        marginBottom: 32,
      }}
    >
      <div>
        <h1 style={{ marginBottom: 4, fontSize: 28, letterSpacing: "-0.04em" }}>{title}</h1>
        <p style={{ color: "#8C8C8C", fontSize: 14, margin: 0 }}>{subtitle}</p>
      </div>
      <div style={{ display: "flex", gap: 8 }}>
        {primaryAction}
        {secondaryAction}
      </div>
    </div>
  );
}
