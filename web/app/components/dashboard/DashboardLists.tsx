"use client";

import type { ReactNode } from "react";
import { motion } from "framer-motion";
import { ArrowRightOutlined, BellOutlined } from "@ant-design/icons";
import { Button, Card, Empty, List, Space, Tag } from "antd";
import dayjs from "dayjs";
import { t, type Language } from "../../lib/i18n";
import type { DashboardQuickAction, DashboardRecentContract, DashboardUpcomingDate } from "./types";

const CRITICAL_DATE_LABELS: Record<string, string> = {
  renewal_deadline: "续租截止",
  break_notice: "Break 通知",
  rent_review: "租金 Review",
  lease_expiry: "租约到期",
  insurance_renewal: "保险续保",
  other: "其他",
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
      bodyStyle={{ padding: 0 }}
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
              initial={{ opacity: 0, y: 4 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ delay: index * 0.04, duration: 0.2 }}
            >
              <List.Item
                style={{
                  padding: "14px 24px",
                  borderBottom: "1px solid #F0F0F0",
                  cursor: "pointer",
                  transition: "background 0.1s",
                }}
                onMouseEnter={(event) => {
                  event.currentTarget.style.background = "#FAFAFA";
                }}
                onMouseLeave={(event) => {
                  event.currentTarget.style.background = "transparent";
                }}
                onClick={() => onOpenContract(contract.id)}
                actions={[
                  getStatusTag(contract.approval_status),
                  <ArrowRightOutlined key="arrow" style={{ color: "#BFBFBF", fontSize: 12 }} />,
                ]}
              >
                <List.Item.Meta
                  title={
                    <span style={{ fontWeight: 600, fontSize: 14, color: "#000" }}>
                      {contract.contract_number || contract.contract_name}
                    </span>
                  }
                  description={
                    <span style={{ color: "#8C8C8C", fontSize: 13 }}>
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

interface QuickActionsCardProps {
  actions: DashboardQuickAction[];
  language: Language;
}

export function QuickActionsCard({ actions, language }: QuickActionsCardProps) {
  return (
    <Card
      title={<span style={{ fontSize: 15, fontWeight: 600, letterSpacing: "-0.01em" }}>{t("dashboard.quick_actions", language)}</span>}
      bodyStyle={{ padding: "12px 24px 24px" }}
      style={{ borderRadius: 10, height: "100%" }}
    >
      <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
        {actions.map((item, index) => (
          <motion.div
            key={item.label}
            initial={{ opacity: 0, x: -4 }}
            animate={{ opacity: 1, x: 0 }}
            transition={{ delay: 0.2 + index * 0.05 }}
            onClick={item.onClick}
            style={{
              display: "flex",
              alignItems: "center",
              gap: 14,
              padding: "12px 14px",
              borderRadius: 8,
              cursor: "pointer",
              transition: "background 0.1s",
              border: "1px solid transparent",
            }}
            onMouseEnter={(event) => {
              event.currentTarget.style.background = "#FAFAFA";
              event.currentTarget.style.borderColor = "#E5E5E5";
            }}
            onMouseLeave={(event) => {
              event.currentTarget.style.background = "transparent";
              event.currentTarget.style.borderColor = "transparent";
            }}
          >
            <div
              style={{
                width: 36,
                height: 36,
                borderRadius: 8,
                background: "#F0F0F0",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                fontSize: 16,
                color: "#000",
                flexShrink: 0,
              }}
            >
              {item.icon}
            </div>
            <div style={{ minWidth: 0 }}>
              <div style={{ fontWeight: 600, fontSize: 14, color: "#000", marginBottom: 2 }}>{item.label}</div>
              <div style={{ fontSize: 12, color: "#8C8C8C" }}>{item.description}</div>
            </div>
            <ArrowRightOutlined style={{ marginLeft: "auto", color: "#BFBFBF", fontSize: 12, flexShrink: 0 }} />
          </motion.div>
        ))}
      </div>
    </Card>
  );
}

interface UpcomingDatesCardProps {
  dates: DashboardUpcomingDate[];
  language: Language;
  getDateUrgency: (targetDate: string) => { color: string; text: string };
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
          {dates.length > 0 && <Tag color="processing">{dates.length}</Tag>}
        </Space>
      }
      bodyStyle={{ padding: 0 }}
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
                  <Tag key="type">{CRITICAL_DATE_LABELS[item.date_type] || item.date_type}</Tag>,
                  <Tag key="urgency" color={urgency.color}>{urgency.text}</Tag>,
                  <ArrowRightOutlined key="arrow" style={{ color: "#BFBFBF", fontSize: 12 }} />,
                ]}
              >
                <List.Item.Meta
                  title={<span style={{ fontWeight: 600 }}>{item.title}</span>}
                  description={
                    <span style={{ color: "#8C8C8C" }}>
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
