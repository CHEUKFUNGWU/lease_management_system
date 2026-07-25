"use client";

import { useCallback, useEffect, useState } from "react";
import { Badge, Button, Card, Empty, List, Space, Spin, Tag, Typography, message } from "antd";
import { ReloadOutlined, RightOutlined } from "@ant-design/icons";
import { useRouter } from "next/navigation";
import { motion } from "framer-motion";
import dayjs from "dayjs";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { workQueueApi } from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";

interface WorkQueueItem {
  kind: string;
  stage: string;
  record_id: string;
  contract_id: string;
  title: string;
  subtitle: string;
  due_date?: string;
  amount?: number;
  currency?: string;
  submitted_at?: string;
}

interface WorkQueue {
  contracts_pending_review: WorkQueueItem[];
  contracts_pending_approval: WorkQueueItem[];
  events_pending: WorkQueueItem[];
  entries_pending_approval: WorkQueueItem[];
  entries_pending_posting: WorkQueueItem[];
  critical_dates_due: WorkQueueItem[];
  total: number;
}

const emptyQueue: WorkQueue = {
  contracts_pending_review: [],
  contracts_pending_approval: [],
  events_pending: [],
  entries_pending_approval: [],
  entries_pending_posting: [],
  critical_dates_due: [],
  total: 0,
};

export default function TodoPage() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const router = useRouter();
  const [queue, setQueue] = useState<WorkQueue>(emptyQueue);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    if (!token) return;
    setLoading(true);
    try {
      const res = await workQueueApi.get(token);
      setQueue({ ...emptyQueue, ...res });
    } catch (error: any) {
      message.error(error?.message || t("todo.load_failed", language));
    } finally {
      setLoading(false);
    }
  }, [token, language]);

  useEffect(() => {
    load();
  }, [load]);

  // A due date is what makes an item urgent, so overdue items are called out
  // rather than just sorted first.
  const dueTag = (item: WorkQueueItem) => {
    if (!item.due_date) return null;
    const days = dayjs(item.due_date).startOf("day").diff(dayjs().startOf("day"), "day");
    if (days < 0) {
      return <Tag color="error">{t("todo.overdue", language, { days: String(Math.abs(days)) })}</Tag>;
    }
    if (days === 0) return <Tag color="warning">{t("todo.due_today", language)}</Tag>;
    return <Tag color={days <= 7 ? "warning" : "default"}>{t("todo.due_in", language, { days: String(days) })}</Tag>;
  };

  const section = (
    titleKey: string,
    items: WorkQueueItem[],
    onOpen: (item: WorkQueueItem) => void,
    hint?: string
  ) => (
    <Card
      key={titleKey}
      title={
        <Space>
          <span style={{ fontSize: 15, fontWeight: 600 }}>{t(titleKey, language)}</span>
          <Badge count={items.length} showZero style={{ backgroundColor: items.length ? "#000" : "#D9D9D9" }} />
        </Space>
      }
      style={{ borderRadius: 10, marginBottom: 16 }}
      bodyStyle={{ padding: items.length ? "0 8px" : 24 }}
    >
      {items.length === 0 ? (
        <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("todo.section_clear", language)} />
      ) : (
        <List
          dataSource={items}
          renderItem={(item) => (
            <List.Item
              style={{ cursor: "pointer" }}
              onClick={() => onOpen(item)}
              actions={[<RightOutlined key="open" style={{ color: "#BFBFBF" }} />]}
            >
              <List.Item.Meta
                title={
                  <Space size={8} wrap>
                    <span style={{ fontWeight: 500 }}>{item.title}</span>
                    {dueTag(item)}
                    {item.amount != null && (
                      <span style={{ color: "#595959", fontSize: 13 }}>
                        {item.amount.toLocaleString(undefined, { minimumFractionDigits: 2 })} {item.currency}
                      </span>
                    )}
                  </Space>
                }
                description={<span style={{ fontSize: 12, color: "#8C8C8C" }}>{item.subtitle || hint}</span>}
              />
            </List.Item>
          )}
        />
      )}
    </Card>
  );

  const openContract = (item: WorkQueueItem) => router.push(`/contracts/${item.contract_id}`);
  const openClosing = () => router.push("/monthly-closing");

  return (
    <ProtectedRoute>
      <AppLayout>
        <motion.div initial={{ opacity: 0, y: 8 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.2 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: 24 }}>
            <div>
              <h1 style={{ marginBottom: 4, fontSize: 28, fontWeight: 700, letterSpacing: "-0.04em" }}>
                {t("todo.title", language)}
              </h1>
              <p style={{ color: "#8C8C8C", fontSize: 14, margin: 0 }}>
                {t("todo.subtitle", language, { count: String(queue.total) })}
              </p>
            </div>
            <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>
              {t("todo.refresh", language)}
            </Button>
          </div>

          <Spin spinning={loading}>
            {section("todo.contracts_pending_review", queue.contracts_pending_review, openContract)}
            {section("todo.contracts_pending_approval", queue.contracts_pending_approval, openContract)}
            {section("todo.events_pending", queue.events_pending, openContract)}
            {section("todo.entries_pending_approval", queue.entries_pending_approval, openClosing)}
            {section("todo.entries_pending_posting", queue.entries_pending_posting, openClosing)}
            {section("todo.critical_dates_due", queue.critical_dates_due, openContract)}
          </Spin>

          <Typography.Paragraph type="secondary" style={{ fontSize: 12, marginTop: 8 }}>
            {t("todo.scope_note", language)}
          </Typography.Paragraph>
        </motion.div>
      </AppLayout>
    </ProtectedRoute>
  );
}
