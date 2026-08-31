"use client";

import { useEffect, useState } from "react";
import { Badge, Dropdown, Empty } from "antd";
import { BellOutlined, CalendarOutlined } from "@ant-design/icons";
import { useRouter } from "next/navigation";
import dayjs from "dayjs";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { workQueueApi } from "../lib/api";
import { t } from "../lib/i18n";
import { StatusTag } from "./StatusTag";

interface WorkQueueItem {
  kind: string;
  contract_id?: string;
  record_id?: string;
  title: string;
  subtitle?: string;
  due_date?: string;
}

function urgency(targetDate: string, language: Parameters<typeof t>[1]) {
  const days = dayjs(targetDate).startOf("day").diff(dayjs().startOf("day"), "day");
  if (days < 0) {
    return { kind: "error" as const, text: t("notif.overdue", language, { days: String(Math.abs(days)) }) };
  }
  if (days === 0) {
    return { kind: "warning" as const, text: t("notif.due_today", language) };
  }
  if (days <= 30) {
    return { kind: "warning" as const, text: t("notif.due_in", language, { days: String(days) }) };
  }
  return { kind: "processing" as const, text: t("notif.due_in", language, { days: String(days) }) };
}

export default function NotificationBell() {
  const router = useRouter();
  const { token } = useAuth();
  const { language } = useLanguage();
  const [items, setItems] = useState<WorkQueueItem[]>([]);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    const load = async () => {
      try {
        const res = await workQueueApi.get(token, 90);
        const queue = res || {};
        const all = Object.entries(queue)
          .filter(([key, value]) => key !== "total" && Array.isArray(value))
          .flatMap(([, value]) => value as WorkQueueItem[])
          .filter((item) => item.due_date || item.kind === "contract_pending_review" || item.kind === "entry_pending_approval")
          .slice(0, 12);
        if (!cancelled) setItems(all);
      } catch (error) {
        console.error("Failed to load critical date notifications:", error);
      }
    };
    load();
    const timer = setInterval(load, 5 * 60 * 1000);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [token]);

  const urgentCount = items.filter(
    (item) => item.due_date && dayjs(item.due_date).startOf("day").diff(dayjs().startOf("day"), "day") <= 30
  ).length;

  const dropdown = (
    <div className="notification-dropdown">
      <div className="notification-dropdown-header">
        <CalendarOutlined className="notification-calendar-icon" />
        {t("notif.title", language)}
        {items.length > 0 && (
          <StatusTag kind={urgentCount > 0 ? "error" : "neutral"} className="notification-count">
            {items.length}
          </StatusTag>
        )}
      </div>
      <div className="notification-dropdown-list">
        {items.length === 0 ? (
          <div className="notification-empty">
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("notif.empty", language)} />
          </div>
        ) : (
          items.map((item, index) => {
            const info = item.due_date ? urgency(item.due_date, language) : { kind: "processing" as const, text: t("notif.pending", language) };
            return (
              <button
                type="button"
                key={`${item.record_id || item.contract_id || item.title}-${index}`}
                className="notification-item"
                onClick={() => {
                  setOpen(false);
                  router.push(item.contract_id ? `/contracts/${item.contract_id}` : "/todo");
                }}
              >
                <div className="notification-item-head">
                  <span className="notification-item-title">
                    {item.title}
                  </span>
                  <StatusTag kind={info.kind} className="notification-item-urgency">
                    {info.text}
                  </StatusTag>
                </div>
                <div className="notification-item-subtitle">
                  {item.subtitle || t("notif.work_queue_item", language)}
                  {item.due_date && <span className="notification-item-date">{dayjs(item.due_date).format("YYYY-MM-DD")}</span>}
                </div>
              </button>
            );
          })
        )}
        <button type="button" className="notification-view-all" onClick={() => { setOpen(false); router.push("/todo"); }}>
          {t("notif.view_all", language)}
        </button>
      </div>
    </div>
  );

  return (
    <Dropdown open={open} onOpenChange={setOpen} popupRender={() => dropdown} placement="bottomRight" trigger={["click"]}>
      <button
        type="button"
        aria-label={t("notif.title", language)}
        aria-expanded={open}
        className="notification-bell-trigger"
      >
        <Badge count={urgentCount} size="small" offset={[-2, 2]}>
          <BellOutlined className="notification-bell-icon" />
        </Badge>
      </button>
    </Dropdown>
  );
}
