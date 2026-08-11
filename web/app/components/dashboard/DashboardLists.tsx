"use client";

import type { ReactNode } from "react";
import { motion } from "framer-motion";
import { ArrowRightOutlined, BellOutlined } from "@ant-design/icons";
import { Button, Card, Empty, List, Space, Tag } from "antd";
import dayjs from "dayjs";
import { t, type Language } from "../../lib/i18n";
import type { DashboardRecentContract, DashboardUpcomingDate, DashboardWorkQueue } from "./types";
import { StatusTag, type StatusKind } from "../StatusTag";

const CRITICAL_DATE_KEYS: Record<string, string> = {
  renewal_deadline: "critical_date.renewal_deadline",
  break_notice: "critical_date.break_notice",
  rent_review: "critical_date.rent_review",
  lease_expiry: "critical_date.lease_expiry",
  insurance_renewal: "critical_date.insurance_renewal",
  other: "critical_date.other",
};

interface RecentContractsCardProps {
  contracts: DashboardRecentContract[];
  language: Language;
  getStatusTag: (status: string) => ReactNode;
  onOpenAll: () => void;
  onOpenContract: (contractId: string) => void;
}

export function RecentContractsCard({
  contracts,
  language,
  getStatusTag,
  onOpenAll,
  onOpenContract,
}: RecentContractsCardProps) {
  return (
    <Card
      title={<span style={{ fontSize: 15, fontWeight: 600, letterSpacing: "-0.01em" }}>{t("dashboard.recent_contracts", language)}</span>}
      extra={
        <Button type="link" size="small" onClick={onOpenAll} style={{ fontSize: 13 }}>
          {t("dashboard.view_all", language)} <ArrowRightOutlined />
        </Button>
      }
      styles={{ body: { padding: 0 } }}
      style={{ borderRadius: 10 }}
    >
      {contracts.length === 0 ? (
        <div style={{ padding: 40 }}>
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("dashboard.no_contracts", language)} />
        </div>
      ) : (
        <List
          dataSource={contracts}
          renderItem={(contract, index) => (
            <motion.div
              initial={false}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: index * 0.04, duration: 0.2 }}
            >
              <List.Item
                style={{
                  padding: "14px 24px",
                  borderBottom: "1px solid var(--bg-inset)",
                  cursor: "pointer",
                  transition: "background 0.1s",
                }}
                onMouseEnter={(event) => {
                  event.currentTarget.style.background = "var(--bg-surface)";
                }}
                onMouseLeave={(event) => {
                  event.currentTarget.style.background = "transparent";
                }}
                onClick={() => onOpenContract(contract.id)}
                actions={[
                  getStatusTag(contract.approval_status),
                  <ArrowRightOutlined key="arrow" style={{ color: "var(--fg-muted)", fontSize: 12 }} />,
                ]}
              >
                <List.Item.Meta
                  title={
                    <span style={{ fontWeight: 600, fontSize: 14, color: "var(--fg-primary)" }}>
                      {contract.contract_number || contract.contract_name}
                    </span>
                  }
                  description={
                    <span style={{ color: "var(--fg-muted)", fontSize: 13 }}>
                      {contract.store_name || contract.lessor_name || contract.store_id || contract.legal_entity_id}
                    </span>
                  }
                />
              </List.Item>
            </motion.div>
          )}
        />
      )}
    </Card>
  );
}

export function WorkQueueSummaryCard({ queue, language, onOpen }: { queue: DashboardWorkQueue; language: Language; onOpen: () => void }) {
  const rows = [
    ["todo.contracts_pending_review", queue.contracts_pending_review],
    ["todo.contracts_pending_approval", queue.contracts_pending_approval],
    ["todo.events_pending", queue.events_pending],
    ["todo.entries_pending_approval", queue.entries_pending_approval],
    ["todo.entries_pending_posting", queue.entries_pending_posting],
    ["todo.critical_dates_due", queue.critical_dates_due],
  ] as const;
  return (
    <Card
      title={<span style={{ fontSize: 15, fontWeight: 600 }}>{t("dashboard.work_queue_title", language)}</span>}
      extra={<Button type="link" size="small" onClick={onOpen}>{t("dashboard.open_work_queue", language)} <ArrowRightOutlined /></Button>}
      styles={{ body: { padding: "12px 20px 20px" } }}
      style={{ borderRadius: 10 }}
    >
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))", gap: 8 }}>
        {rows.map(([key, count]) => (
          <div key={key} style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, padding: "10px 12px", background: "var(--bg-surface)", border: "1px solid var(--bg-inset)", borderRadius: 6 }}>
            <span style={{ fontSize: 13, color: "var(--fg-secondary)" }}>{t(key, language)}</span>
            <strong style={{ fontSize: 18, fontVariantNumeric: "tabular-nums" }}>{count}</strong>
          </div>
        ))}
      </div>
    </Card>
  );
}

interface UpcomingDatesCardProps {
  dates: DashboardUpcomingDate[];
  language: Language;
  getDateUrgency: (targetDate: string) => { kind: StatusKind; text: string };
  onOpenContract: (contractId: string) => void;
}

export function UpcomingDatesCard({
  dates,
  language,
  getDateUrgency,
  onOpenContract,
}: UpcomingDatesCardProps) {
  return (
    <Card
      title={
        <Space>
          <BellOutlined />
          <span style={{ fontSize: 15, fontWeight: 600, letterSpacing: "-0.01em" }}>
            {t("dashboard.upcoming_critical_dates", language)}
          </span>
          {dates.length > 0 && <StatusTag kind="processing">{dates.length}</StatusTag>}
        </Space>
      }
      styles={{ body: { padding: 0 } }}
      style={{ borderRadius: 10 }}
    >
      {dates.length === 0 ? (
        <div style={{ padding: 32 }}>
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("dashboard.no_upcoming_dates", language)} />
        </div>
      ) : (
        <List
          dataSource={dates}
          renderItem={(item) => {
            const urgency = getDateUrgency(item.target_date);
            return (
              <List.Item
                style={{ padding: "14px 24px", cursor: "pointer" }}
                onClick={() => onOpenContract(item.contract_id)}
                actions={[
                  <StatusTag key="type">{t(CRITICAL_DATE_KEYS[item.date_type] || "critical_date.other", language)}</StatusTag>,
                  <StatusTag key="urgency" kind={urgency.kind}>{urgency.text}</StatusTag>,
                  <ArrowRightOutlined key="arrow" style={{ color: "var(--fg-muted)", fontSize: 12 }} />,
                ]}
              >
                <List.Item.Meta
                  title={<span style={{ fontWeight: 600 }}>{item.title}</span>}
                  description={
                    <span style={{ color: "var(--fg-muted)" }}>
                      {dayjs(item.target_date).format("YYYY-MM-DD")} · {t("dashboard.reminder_days", language, { days: String(item.reminder_days) })}
                      {item.description ? ` · ${item.description}` : ""}
                    </span>
                  }
                />
              </List.Item>
            );
          }}
        />
      )}
    </Card>
  );
}
