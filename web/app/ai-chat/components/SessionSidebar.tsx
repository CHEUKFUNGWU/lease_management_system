"use client";

import { motion } from "framer-motion";
import { Button, Dropdown, Empty, Tooltip } from "antd";
import {
  ClockCircleOutlined,
  DeleteOutlined,
  MessageOutlined,
  MoreOutlined,
  PlusOutlined,
} from "@ant-design/icons";
import { t, type Language } from "../../lib/i18n";
import type { ChatSession } from "../../lib/types/ai-chat";

interface SessionSidebarProps {
  sessions: ChatSession[];
  activeSessionId: string | null;
  language: Language;
  onSelect: (id: string) => void;
  onNew: () => void;
  onDelete: (id: string) => void;
}

function formatTime(timestamp: number) {
  const date = new Date(timestamp);
  const now = new Date();
  const diff = now.getTime() - date.getTime();

  if (diff < 60 * 1000) return "Just now";
  if (diff < 60 * 60 * 1000) return `${Math.floor(diff / (60 * 1000))}m ago`;
  if (diff < 24 * 60 * 60 * 1000) return `${Math.floor(diff / (60 * 60 * 1000))}h ago`;
  return date.toLocaleDateString();
}

export function SessionSidebar({
  sessions,
  activeSessionId,
  language,
  onSelect,
  onNew,
  onDelete,
}: SessionSidebarProps) {
  return (
    <div
      style={{
        width: 260,
        borderRight: "1px solid #E5E5E5",
        background: "#FAFAFA",
        display: "flex",
        flexDirection: "column",
        height: "100%",
      }}
    >
      <div
        style={{
          padding: "16px 12px",
          borderBottom: "1px solid #E5E5E5",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
        }}
      >
        <span style={{ fontSize: 14, fontWeight: 600, color: "#000" }}>
          {t("nav.ai_chat", language)}
        </span>
        <Tooltip title={t("ai.new_session_btn", language)}>
          <Button type="text" icon={<PlusOutlined />} onClick={onNew} style={{ color: "#000" }} />
        </Tooltip>
      </div>

      <div style={{ flex: 1, overflowY: "auto", padding: "8px" }}>
        {sessions.length === 0 ? (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={t("ai.no_sessions", language)}
            style={{ marginTop: 40 }}
          />
        ) : (
          <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            {sessions.map((session) => (
              <motion.div key={session.id} whileHover={{ scale: 1.01 }} whileTap={{ scale: 0.99 }}>
                <div
                  onClick={() => onSelect(session.id)}
                  style={{
                    padding: "10px 12px",
                    borderRadius: 8,
                    cursor: "pointer",
                    background: activeSessionId === session.id ? "#000" : "transparent",
                    color: activeSessionId === session.id ? "#fff" : "#262626",
                    transition: "all 0.15s",
                    display: "flex",
                    alignItems: "center",
                    gap: 10,
                    position: "relative",
                  }}
                  onMouseEnter={(e) => {
                    if (activeSessionId !== session.id) {
                      e.currentTarget.style.background = "#F0F0F0";
                    }
                  }}
                  onMouseLeave={(e) => {
                    if (activeSessionId !== session.id) {
                      e.currentTarget.style.background = "transparent";
                    }
                  }}
                >
                  <MessageOutlined
                    style={{
                      fontSize: 14,
                      flexShrink: 0,
                      color: activeSessionId === session.id ? "#fff" : "#8C8C8C",
                    }}
                  />
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div
                      style={{
                        fontSize: 13,
                        fontWeight: 500,
                        whiteSpace: "nowrap",
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                        lineHeight: 1.4,
                      }}
                    >
                      {session.title}
                    </div>
                    <div
                      style={{
                        fontSize: 11,
                        color:
                          activeSessionId === session.id ? "rgba(255,255,255,0.6)" : "#8C8C8C",
                        marginTop: 2,
                        display: "flex",
                        alignItems: "center",
                        gap: 4,
                      }}
                    >
                      <ClockCircleOutlined style={{ fontSize: 10 }} />
                      {formatTime(session.updatedAt)}
                    </div>
                  </div>

                  <Dropdown
                    menu={{
                      items: [
                        {
                          key: "delete",
                          label: t("ai.delete_session", language),
                          icon: <DeleteOutlined />,
                          danger: true,
                          onClick: (e) => {
                            e.domEvent.stopPropagation();
                            onDelete(session.id);
                          },
                        },
                      ],
                    }}
                    trigger={["click"]}
                    placement="bottomRight"
                  >
                    <Button
                      type="text"
                      icon={<MoreOutlined />}
                      onClick={(e) => e.stopPropagation()}
                      style={{
                        opacity: 0,
                        transition: "opacity 0.15s",
                        color: activeSessionId === session.id ? "#fff" : "#8C8C8C",
                        padding: 0,
                        width: 24,
                        height: 24,
                      }}
                      className="session-more-btn"
                    />
                  </Dropdown>
                </div>
              </motion.div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
