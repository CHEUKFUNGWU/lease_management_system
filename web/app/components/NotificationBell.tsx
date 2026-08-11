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
    <div
      style={{
        width: 360,
        background: "var(--fg-inverse)",
        borderRadius: 10,
        border: "1px solid var(--border-default)",
        boxShadow: "0 0 0 1px rgba(0,0,0,0.04), 0 8px 24px rgba(0,0,0,0.1)",
        overflow: "hidden",
      }}
    >
      <div
        style={{
          padding: "12px 16px",
          borderBottom: "1px solid var(--bg-inset)",
          fontSize: 13,
          fontWeight: 600,
          display: "flex",
          alignItems: "center",
          gap: 8,
        }}
      >
        <CalendarOutlined style={{ fontSize: 13 }} />
        {t("notif.title", language)}
        {items.length > 0 && (
          <StatusTag kind={urgentCount > 0 ? "error" : "neutral"} style={{ fontSize: 11, marginLeft: "auto", padding: "1px 7px" }}>
            {items.length}
          </StatusTag>
        )}
      </div>
      <div style={{ maxHeight: 380, overflowY: "auto" }}>
        {items.length === 0 ? (
          <div style={{ padding: "32px 0" }}>
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("notif.empty", language)} />
          </div>
        ) : (
          items.map((item, index) => {
            const info = item.due_date ? urgency(item.due_date, language) : { kind: "processing" as const, text: t("notif.pending", language) };
            return (
              <button
                type="button"
                key={`${item.record_id || item.contract_id || item.title}-${index}`}
                onClick={() => {
                  setOpen(false);
                  router.push(item.contract_id ? `/contracts/${item.contract_id}` : "/todo");
                }}
                style={{
                  width: "100%",
                  textAlign: "left",
                  background: "transparent",
                  border: 0,
                  padding: "10px 16px",
                  borderBottom: "1px solid var(--bg-surface)",
                  cursor: "pointer",
                }}
                onMouseEnter={(e) => (e.currentTarget.style.background = "var(--bg-surface)")}
                onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
              >
                <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                  <span
                    style={{
                      fontSize: 13,
                      fontWeight: 500,
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                      flex: 1,
                    }}
                  >
                    {item.title}
                  </span>
                  <StatusTag kind={info.kind} style={{ fontSize: 11, padding: "1px 7px", marginRight: 0, flexShrink: 0 }}>
                    {info.text}
                  </StatusTag>
                </div>
                <div style={{ fontSize: 12, color: "var(--fg-muted)", marginTop: 2 }}>
                  {item.subtitle || t("notif.work_queue_item", language)}
                  {item.due_date && <span style={{ marginLeft: 8 }}>{dayjs(item.due_date).format("YYYY-MM-DD")}</span>}
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
        style={{
          cursor: "pointer",
          border: 0,
          background: "transparent",
          padding: "8px",
          borderRadius: 8,
          transition: "background 0.15s",
          color: "var(--fg-tertiary)",
          position: "relative",
        }}
        onMouseEnter={(e) => (e.currentTarget.style.background = "var(--bg-inset)")}
        onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
      >
        <Badge count={urgentCount} size="small" offset={[-2, 2]}>
          <BellOutlined style={{ fontSize: 16 }} />
        </Badge>
      </button>
    </Dropdown>
  );
}
