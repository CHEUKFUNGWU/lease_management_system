"use client";

import type { ReactNode } from "react";
import { motion } from "framer-motion";
import { ArrowRightOutlined, BellOutlined } from "@ant-design/icons";
import { Button, Card, Empty, List, Space } from "antd";
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
      title={<span className="dashboard-list-card-title">{t("dashboard.recent_contracts", language)}</span>}
      extra={
        <Button type="link" size="small" onClick={onOpenAll} className="dashboard-list-view-all">
          {t("dashboard.view_all", language)} <ArrowRightOutlined />
        </Button>
      }
      className="dashboard-list-card dashboard-recent-contracts-card"
    >
      {contracts.length === 0 ? (
        <div className="dashboard-list-empty dashboard-list-empty-contracts">
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
                className="dashboard-list-item"
                role="button"
                tabIndex={0}
                onClick={() => onOpenContract(contract.id)}
                onKeyDown={(event) => {
                  if (event.key === "Enter" || event.key === " ") {
                    event.preventDefault();
                    onOpenContract(contract.id);
                  }
                }}
                actions={[
                  getStatusTag(contract.approval_status),
                  <ArrowRightOutlined key="arrow" className="dashboard-list-arrow" />,
                ]}
              >
                <List.Item.Meta
                  title={
                    <span className="dashboard-list-item-title">
                      {contract.contract_number || contract.contract_name}
                    </span>
                  }
                  description={
                    <span className="dashboard-list-item-description">
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
      title={<span className="dashboard-list-card-title">{t("dashboard.work_queue_title", language)}</span>}
      extra={<Button type="link" size="small" onClick={onOpen}>{t("dashboard.open_work_queue", language)} <ArrowRightOutlined /></Button>}
      className="dashboard-list-card dashboard-work-queue-card"
    >
      <div className="dashboard-work-queue-grid">
        {rows.map(([key, count]) => (
          <div key={key} className="dashboard-work-queue-item">
            <span className="dashboard-work-queue-label">{t(key, language)}</span>
            <strong className="dashboard-work-queue-count">{count}</strong>
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
          <span className="dashboard-list-card-title">
            {t("dashboard.upcoming_critical_dates", language)}
          </span>
          {dates.length > 0 && <StatusTag kind="processing">{dates.length}</StatusTag>}
        </Space>
      }
      className="dashboard-list-card dashboard-upcoming-dates-card"
    >
      {dates.length === 0 ? (
        <div className="dashboard-list-empty dashboard-list-empty-dates">
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("dashboard.no_upcoming_dates", language)} />
        </div>
      ) : (
        <List
          dataSource={dates}
          renderItem={(item) => {
            const urgency = getDateUrgency(item.target_date);
            return (
              <List.Item
                className="dashboard-list-item dashboard-upcoming-date-item"
                role="button"
                tabIndex={0}
                onClick={() => onOpenContract(item.contract_id)}
                onKeyDown={(event) => {
                  if (event.key === "Enter" || event.key === " ") {
                    event.preventDefault();
                    onOpenContract(item.contract_id);
                  }
                }}
                actions={[
                  <StatusTag key="type">{t(CRITICAL_DATE_KEYS[item.date_type] || "critical_date.other", language)}</StatusTag>,
                  <StatusTag key="urgency" kind={urgency.kind}>{urgency.text}</StatusTag>,
                  <ArrowRightOutlined key="arrow" className="dashboard-list-arrow" />,
                ]}
              >
                <List.Item.Meta
                  title={<span className="dashboard-upcoming-date-title">{item.title}</span>}
                  description={
                    <span className="dashboard-upcoming-date-description">
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
