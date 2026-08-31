"use client";

import React, { useState, useEffect, useMemo, useRef } from "react";
import { useRouter } from "next/navigation";
import { Modal, Input, Typography, Tag, Space, Empty } from "antd";
import {
  SearchOutlined,
  EnterOutlined,
  FileTextOutlined,
  LineChartOutlined,
  RobotOutlined,
  CalculatorOutlined,
  SettingOutlined,
  SwapOutlined,
  DollarOutlined,
} from "@ant-design/icons";
import { PALETTE_PAGES } from "../../lib/palette";
import { t } from "../../lib/i18n";
import { useAuth } from "../../context/AuthContext";
import { useLanguage } from "../../context/LanguageContext";

const { Text } = Typography;

export interface CommandItem {
  id: string;
  title: string;
  category: "navigation" | "action" | "entity";
  groupTitle: string;
  icon?: React.ReactNode;
  keywords?: string[];
  onSelect: () => void;
}

export function CommandPalette() {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [selectedIndex, setSelectedIndex] = useState(0);
  const router = useRouter();
  const { user } = useAuth();
  const { language } = useLanguage();
  const inputRef = useRef<any>(null);

  // Global hotkey listener (Cmd+K / Ctrl+K)
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setOpen((prev) => !prev);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  // Reset state on open
  useEffect(() => {
    if (open) {
      setQuery("");
      setSelectedIndex(0);
      setTimeout(() => inputRef.current?.focus?.(), 50);
    }
  }, [open]);

  // Build searchable commands
  const commands = useMemo<CommandItem[]>(() => {
    const list: CommandItem[] = [];

    // 1. Navigation items from registered PALETTE_PAGES
    for (const p of PALETTE_PAGES) {
      if (!p.visible(user)) continue;
      const title = t(p.labelKey, language as any);
      const groupName = t(`search.group_${p.group}`, language as any);

      list.push({
        id: `nav_${p.path}`,
        title,
        category: "navigation",
        groupTitle: groupName || "导航",
        icon: <LineChartOutlined />,
        keywords: [p.path, p.group],
        onSelect: () => {
          setOpen(false);
          router.push(p.path);
        },
      });
    }

    // 2. High-frequency Action Runner shortcuts
    list.push({
      id: "act_new_contract",
      title: "新建合同录入 / 补录",
      category: "action",
      groupTitle: "快捷操作",
      icon: <FileTextOutlined />,
      keywords: ["new", "create", "contract", "新建"],
      onSelect: () => {
        setOpen(false);
        router.push("/contracts/new");
      },
    });

    list.push({
      id: "act_ai_consult",
      title: "呼叫零售经营 AI 诊断",
      category: "action",
      groupTitle: "快捷操作",
      icon: <RobotOutlined />,
      keywords: ["ai", "chat", "assistant", "诊断"],
      onSelect: () => {
        setOpen(false);
        router.push("/ai-chat");
      },
    });

    list.push({
      id: "act_forecast",
      title: "开启租金谈判与现金流测算",
      category: "action",
      groupTitle: "快捷操作",
      icon: <DollarOutlined />,
      keywords: ["cashflow", "forecast", "测算", "谈判"],
      onSelect: () => {
        setOpen(false);
        router.push("/cashflow-forecast");
      },
    });

    return list;
  }, [user, language, router]);

  // Filter commands
  const filteredCommands = useMemo(() => {
    if (!query.trim()) return commands;
    const q = query.trim().toLowerCase();

    return commands.filter((cmd) => {
      const matchTitle = cmd.title.toLowerCase().includes(q);
      const matchGroup = cmd.groupTitle.toLowerCase().includes(q);
      const matchKeyword = cmd.keywords?.some((k) => k.toLowerCase().includes(q));
      return matchTitle || matchGroup || matchKeyword;
    });
  }, [commands, query]);

  // Keyboard navigation within palette
  const handleInputKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setSelectedIndex((prev) => (prev + 1) % (filteredCommands.length || 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setSelectedIndex((prev) => (prev - 1 + (filteredCommands.length || 1)) % (filteredCommands.length || 1));
    } else if (e.key === "Enter") {
      e.preventDefault();
      const current = filteredCommands[selectedIndex];
      if (current) current.onSelect();
    } else if (e.key === "Escape") {
      setOpen(false);
    }
  };

  return (
    <Modal
      open={open}
      onCancel={() => setOpen(false)}
      footer={null}
      closable={false}
      width={560}
      style={{ top: 100 }}
      styles={{
        content: {
          padding: 0,
          borderRadius: 12,
          overflow: "hidden",
          boxShadow: "0 20px 40px rgba(0,0,0,0.15), 0 0 0 1px var(--border-default)",
          background: "var(--bg-surface)",
        },
      }}
    >
      {/* Search Input Bar */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 10,
          padding: "12px 16px",
          borderBottom: "1px solid var(--border-default)",
        }}
      >
        <SearchOutlined style={{ fontSize: 16, color: "var(--fg-muted)" }} />
        <Input
          ref={inputRef}
          value={query}
          onChange={(e) => {
            setQuery(e.target.value);
            setSelectedIndex(0);
          }}
          onKeyDown={handleInputKeyDown}
          placeholder="搜索页面、执行快捷操作 (↑↓ 移动, ↵ 执行)..."
          bordered={false}
          style={{ fontSize: 14, padding: 0 }}
        />
        <Tag style={{ fontSize: 11, color: "var(--fg-muted)", borderRadius: 4 }}>ESC 关闭</Tag>
      </div>

      {/* Results List */}
      <div style={{ maxHeight: 360, overflowY: "auto", padding: "8px 0" }}>
        {filteredCommands.length === 0 ? (
          <div style={{ padding: "30px 0", textAlign: "center" }}>
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="未找到相关指令或页面" />
          </div>
        ) : (
          filteredCommands.map((cmd, idx) => {
            const isSelected = idx === selectedIndex;
            return (
              <div
                key={cmd.id}
                onClick={cmd.onSelect}
                onMouseEnter={() => setSelectedIndex(idx)}
                style={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  padding: "9px 16px",
                  cursor: "pointer",
                  background: isSelected ? "var(--bg-inset)" : "transparent",
                  borderLeft: isSelected ? "3px solid var(--fg-secondary)" : "3px solid transparent",
                  transition: "all 0.1s ease",
                }}
              >
                <Space size={10}>
                  <span style={{ color: isSelected ? "var(--fg-primary)" : "var(--fg-muted)" }}>
                    {cmd.icon}
                  </span>
                  <Text style={{ fontSize: 13, fontWeight: isSelected ? 600 : 400 }}>
                    {cmd.title}
                  </Text>
                </Space>
                <Space size={6}>
                  <Tag
                    bordered={false}
                    style={{
                      fontSize: 10,
                      background: "var(--bg-inset)",
                      color: "var(--fg-secondary)",
                      borderRadius: 4,
                    }}
                  >
                    {cmd.groupTitle}
                  </Tag>
                  {isSelected && <EnterOutlined style={{ fontSize: 11, color: "var(--fg-muted)" }} />}
                </Space>
              </div>
            );
          })
        )}
      </div>

      {/* Footer Info */}
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          padding: "8px 16px",
          background: "var(--bg-inset)",
          borderTop: "1px solid var(--border-subtle)",
          fontSize: 11,
          color: "var(--fg-muted)",
        }}
      >
        <span>
          按 <kbd style={{ padding: "1px 4px", background: "var(--bg-inset)", borderRadius: 3 }}>↑</kbd>{" "}
          <kbd style={{ padding: "1px 4px", background: "var(--bg-inset)", borderRadius: 3 }}>↓</kbd> 切换选项
        </span>
        <span>
          按 <kbd style={{ padding: "1px 4px", background: "var(--bg-inset)", borderRadius: 3 }}>↵ Enter</kbd> 确认执行
        </span>
      </div>
    </Modal>
  );
}
