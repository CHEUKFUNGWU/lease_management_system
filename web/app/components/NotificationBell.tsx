"use client";

import { useEffect, useState } from "react";
import { Badge, Dropdown, Empty, Tag } from "antd";
import { BellOutlined, CalendarOutlined } from "@ant-design/icons";
import { useRouter } from "next/navigation";
import dayjs from "dayjs";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { leaseAdminApi } from "../lib/api";
import { t } from "../lib/i18n";

interface UpcomingCriticalDate {
  id: string;
  contract_id: string;
  contract_number?: string;
  contract_name?: string;
  date_type: string;
  target_date: string;
  title: string;
}

function urgency(targetDate: string, language: Parameters<typeof t>[1]) {
  const days = dayjs(targetDate).startOf("day").diff(dayjs().startOf("day"), "day");
  if (days < 0) {
    return { color: "error" as const, text: t("notif.overdue", language, { days: String(Math.abs(days)) }) };
  }
  if (days === 0) {
    return { color: "warning" as const, text: t("notif.due_today", language) };
  }
  if (days <= 30) {
    return { color: "warning" as const, text: t("notif.due_in", language, { days: String(days) }) };
  }
  return { color: "processing" as const, text: t("notif.due_in", language, { days: String(days) }) };
}

export default function NotificationBell() {
  const router = useRouter();
  const { token } = useAuth();
  const { language } = useLanguage();
  const [dates, setDates] = useState<UpcomingCriticalDate[]>([]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    const load = async () => {
      try {
        const res = await leaseAdminApi.listUpcomingCriticalDates(token, { days: 90, limit: 10 });
        if (!cancelled) setDates(res.data || []);
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

  const urgentCount = dates.filter(
    (d) => dayjs(d.target_date).startOf("day").diff(dayjs().startOf("day"), "day") <= 30
  ).length;

  const dropdown = (
    <div
      style={{
        width: 360,
        background: "#fff",
        borderRadius: 10,
        border: "1px solid #E5E5E5",
        boxShadow: "0 0 0 1px rgba(0,0,0,0.04), 0 8px 24px rgba(0,0,0,0.1)",
        overflow: "hidden",
      }}
    >
      <div
        style={{
          padding: "12px 16px",
          borderBottom: "1px solid #F0F0F0",
          fontSize: 13,
          fontWeight: 600,
          display: "flex",
          alignItems: "center",
          gap: 8,
        }}
      >
        <CalendarOutlined style={{ fontSize: 13 }} />
        {t("notif.title", language)}
        {urgentCount > 0 && (
          <Tag color="error" style={{ fontSize: 11, marginLeft: "auto", marginRight: 0 }}>
            {urgentCount}
          </Tag>
        )}
      </div>
      <div style={{ maxHeight: 380, overflowY: "auto" }}>
        {dates.length === 0 ? (
          <div style={{ padding: "32px 0" }}>
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("notif.empty", language)} />
          </div>
        ) : (
          dates.map((item) => {
            const info = urgency(item.target_date, language);
            return (
              <div
                key={item.id}
                onClick={() => router.push(`/contracts/${item.contract_id}`)}
                style={{
                  padding: "10px 16px",
                  borderBottom: "1px solid #F7F7F7",
                  cursor: "pointer",
                  transition: "background 0.15s",
                }}
                onMouseEnter={(e) => (e.currentTarget.style.background = "#FAFAFA")}
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
                  <Tag color={info.color} style={{ fontSize: 11, marginRight: 0, flexShrink: 0 }}>
                    {info.text}
                  </Tag>
                </div>
                <div style={{ fontSize: 12, color: "#8C8C8C", marginTop: 2 }}>
                  {[item.contract_number, item.contract_name].filter(Boolean).join(" · ")}
                  <span style={{ marginLeft: 8, color: "#BFBFBF" }}>
                    {dayjs(item.target_date).format("YYYY-MM-DD")}
                  </span>
                </div>
              </div>
            );
          })
        )}
      </div>
    </div>
  );

  return (
    <Dropdown popupRender={() => dropdown} placement="bottomRight" trigger={["click"]}>
      <div
        style={{
          cursor: "pointer",
          padding: "8px",
          borderRadius: 8,
          transition: "background 0.15s",
          color: "#595959",
          position: "relative",
        }}
        onMouseEnter={(e) => (e.currentTarget.style.background = "#F5F5F5")}
        onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
      >
        <Badge count={urgentCount} size="small" offset={[-2, 2]}>
          <BellOutlined style={{ fontSize: 16 }} />
        </Badge>
      </div>
    </Dropdown>
  );
}
