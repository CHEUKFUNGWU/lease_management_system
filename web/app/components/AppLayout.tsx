"use client";

import React, { useState, useMemo } from "react";
import { Layout, Menu, Avatar, Dropdown, Breadcrumb, Input, Badge } from "antd";
import {
  HomeOutlined,
  FileTextOutlined,
  UploadOutlined,
  RobotOutlined,
  SettingOutlined,
  LogoutOutlined,
  UserOutlined,
  BarChartOutlined,
  LineChartOutlined,
  SafetyOutlined,
  AuditOutlined,
  CalculatorOutlined,
  GlobalOutlined,
  SearchOutlined,
  BellOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
} from "@ant-design/icons";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { motion, AnimatePresence } from "framer-motion";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { pageTransition } from "../design-system/animations";

const { Header, Sider, Content } = Layout;

// ─── Breadcrumb Mapping ────────────────────────────────────────

function getBreadcrumbMap(language: string): Record<string, string> {
  return {
    "": t("nav.home", language as any),
    contracts: t("nav.contracts", language as any),
    upload: t("nav.upload", language as any),
    "ai-chat": t("nav.ai_chat", language as any),
    reports: t("nav.reports", language as any),
    "cashflow-forecast": t("nav.cashflow", language as any),
    "monthly-closing": t("nav.monthly_closing", language as any),
    "audit-logs": t("nav.audit_logs", language as any),
    settings: t("nav.settings", language as any),
    admin: t("nav.admin", language as any),
    users: "用户管理",
    new: "新增",
  };
}

// ─── Menu Items ────────────────────────────────────────────────

function useMenuItems(language: string) {
  return useMemo(
    () => [
      {
        key: "/",
        icon: <HomeOutlined style={{ fontSize: 16 }} />,
        label: <Link href="/">{t("nav.home", language as any)}</Link>,
      },
      {
        key: "/contracts",
        icon: <FileTextOutlined style={{ fontSize: 16 }} />,
        label: <Link href="/contracts">{t("nav.contracts", language as any)}</Link>,
      },
      {
        key: "/upload",
        icon: <UploadOutlined style={{ fontSize: 16 }} />,
        label: <Link href="/upload">{t("nav.upload", language as any)}</Link>,
      },
      {
        key: "/ai-chat",
        icon: <RobotOutlined style={{ fontSize: 16 }} />,
        label: <Link href="/ai-chat">{t("nav.ai_chat", language as any)}</Link>,
      },
      {
        key: "/reports",
        icon: <BarChartOutlined style={{ fontSize: 16 }} />,
        label: <Link href="/reports">{t("nav.reports", language as any)}</Link>,
      },
      {
        key: "/cashflow-forecast",
        icon: <LineChartOutlined style={{ fontSize: 16 }} />,
        label: <Link href="/cashflow-forecast">{t("nav.cashflow", language as any)}</Link>,
      },
      {
        key: "/monthly-closing",
        icon: <CalculatorOutlined style={{ fontSize: 16 }} />,
        label: <Link href="/monthly-closing">{t("nav.monthly_closing", language as any)}</Link>,
      },
      {
        key: "/audit-logs",
        icon: <AuditOutlined style={{ fontSize: 16 }} />,
        label: <Link href="/audit-logs">{t("nav.audit_logs", language as any)}</Link>,
      },
      {
        key: "/settings",
        icon: <SettingOutlined style={{ fontSize: 16 }} />,
        label: <Link href="/settings">{t("nav.settings", language as any)}</Link>,
      },
    ],
    [language]
  );
}

// ─── App Layout ────────────────────────────────────────────────

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { user, logout } = useAuth();
  const { language, setLanguage } = useLanguage();
  const [collapsed, setCollapsed] = useState(false);
  const [searchFocused, setSearchFocused] = useState(false);

  const handleLogout = () => {
    logout();
    router.push("/login");
  };

  const baseMenuItems = useMenuItems(language);
  const adminMenuItem = {
    key: "/admin/users",
    icon: <SafetyOutlined style={{ fontSize: 16 }} />,
    label: <Link href="/admin/users">{t("nav.admin", language)}</Link>,
  };

  const menuItems = user?.role === "admin"
    ? [...baseMenuItems, adminMenuItem]
    : baseMenuItems;

  // Generate breadcrumbs from pathname
  const breadcrumbs = useMemo(() => {
    const breadcrumbMap = getBreadcrumbMap(language);
    const segments = pathname.split("/").filter(Boolean);
    const items: { title: React.ReactNode; href?: string }[] = [
      { title: <Link href="/"><HomeOutlined style={{ fontSize: 14 }} /></Link> },
    ];

    let currentPath = "";
    segments.forEach((segment) => {
      currentPath += `/${segment}`;
      const isId = segment.length > 20 && !breadcrumbMap[segment]; // Likely UUID
      const label = isId ? "详情" : (breadcrumbMap[segment] || segment);
      
      if (isId) {
        items.push({ title: label });
      } else if (segment === "new") {
        items.push({ title: label });
      } else {
        items.push({ title: <Link href={currentPath}>{label}</Link> });
      }
    });

    return items;
  }, [pathname, language]);

  const userMenuItems = [
    {
      key: "profile",
      icon: <UserOutlined />,
      label: t("user.profile", language),
    },
    {
      key: "logout",
      icon: <LogoutOutlined />,
      label: t("user.logout", language),
      danger: true,
      onClick: handleLogout,
    },
  ];

  // Find active menu key (handle nested routes like /contracts/:id)
  const activeMenuKey = useMemo(() => {
    if (pathname === "/") return "/";
    const basePath = "/" + pathname.split("/")[1];
    return basePath;
  }, [pathname]);

  return (
    <Layout style={{ minHeight: "100vh" }}>
      {/* ── Header ── */}
      <Header
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          background: "#fff",
          borderBottom: "1px solid #E5E5E5",
          padding: "0 32px",
          height: 60,
          position: "sticky",
          top: 0,
          zIndex: 200,
          flexShrink: 0,
        }}
      >
        {/* Left: Logo + Collapse + Breadcrumb */}
        <div style={{ display: "flex", alignItems: "center", gap: 16, flex: 1, minWidth: 0 }}>
          {/* Logo */}
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: 10,
              flexShrink: 0,
            }}
          >
            <div
              style={{
                width: 28,
                height: 28,
                borderRadius: 7,
                background: "#000",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
              }}
            >
              <RobotOutlined style={{ fontSize: 15, color: "#fff" }} />
            </div>
            <span
              style={{
                fontSize: 16,
                fontWeight: 700,
                letterSpacing: "-0.5px",
                color: "#000",
                whiteSpace: "nowrap",
              }}
            >
              IFRS 16
            </span>
          </div>

          {/* Collapse Toggle */}
          <div
            onClick={() => setCollapsed(!collapsed)}
            style={{
              cursor: "pointer",
              padding: "6px",
              borderRadius: 6,
              transition: "background 0.15s",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              color: "#595959",
            }}
            onMouseEnter={(e) => (e.currentTarget.style.background = "#F5F5F5")}
            onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
          >
            {collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
          </div>

          {/* Breadcrumb */}
          {pathname !== "/" && (
            <div style={{ marginLeft: 8, minWidth: 0, overflow: "hidden" }}>
              <Breadcrumb
                items={breadcrumbs}
                separator={<span style={{ color: "#D9D9D9" }}>/</span>}
                style={{ fontSize: 13 }}
              />
            </div>
          )}
        </div>

        {/* Right: Search + Notifications + Language + User */}
        <div style={{ display: "flex", alignItems: "center", gap: 8, flexShrink: 0 }}>
          {/* Global Search */}
          <div
            style={{
              position: "relative",
              width: searchFocused ? 280 : 200,
              transition: "width 0.2s cubic-bezier(0.4, 0, 0.2, 1)",
            }}
          >
            <Input
              placeholder="全局搜索..."
              prefix={<SearchOutlined style={{ color: "#8C8C8C", fontSize: 14 }} />}
              onFocus={() => setSearchFocused(true)}
              onBlur={() => setSearchFocused(false)}
              style={{
                borderRadius: 9999,
                background: "#F5F5F5",
                border: "none",
                fontSize: 13,
                height: 34,
                paddingLeft: 14,
              }}
            />
            <kbd
              style={{
                position: "absolute",
                right: 10,
                top: "50%",
                transform: "translateY(-50%)",
                fontSize: 10,
                color: "#BFBFBF",
                background: "#fff",
                border: "1px solid #E5E5E5",
                borderRadius: 4,
                padding: "0 5px",
                lineHeight: "16px",
                fontFamily: "monospace",
                pointerEvents: "none",
              }}
            >
              ⌘K
            </kbd>
          </div>

          {/* Notifications */}
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
            <Badge dot offset={[-2, 2]}>
              <BellOutlined style={{ fontSize: 16 }} />
            </Badge>
          </div>

          {/* Language Switcher */}
          <Dropdown
            menu={{
              items: [
                { key: "zh-CN", label: "简体中文", onClick: () => setLanguage("zh-CN" as any) },
                { key: "zh-HK", label: "繁體中文", onClick: () => setLanguage("zh-HK" as any) },
                { key: "en", label: "English", onClick: () => setLanguage("en" as any) },
              ],
            }}
            placement="bottomRight"
          >
            <div
              style={{
                cursor: "pointer",
                padding: "8px",
                borderRadius: 8,
                transition: "background 0.15s",
                color: "#595959",
                display: "flex",
                alignItems: "center",
                gap: 4,
                fontSize: 13,
                fontWeight: 500,
              }}
              onMouseEnter={(e) => (e.currentTarget.style.background = "#F5F5F5")}
              onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
            >
              <GlobalOutlined style={{ fontSize: 14 }} />
              <span style={{ textTransform: "uppercase", fontSize: 12 }}>
                {language === "zh-CN" ? "CN" : language === "zh-HK" ? "HK" : "EN"}
              </span>
            </div>
          </Dropdown>

          {/* Divider */}
          <div style={{ width: 1, height: 20, background: "#E5E5E5" }} />

          {/* User */}
          {user && (
            <Dropdown menu={{ items: userMenuItems }} placement="bottomRight">
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 10,
                  cursor: "pointer",
                  padding: "6px 12px",
                  borderRadius: 9999,
                  transition: "background 0.15s",
                }}
                onMouseEnter={(e) => (e.currentTarget.style.background = "#F5F5F5")}
                onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
              >
                <Avatar
                  size={28}
                  icon={<UserOutlined />}
                  style={{ background: "#000", fontSize: 12 }}
                />
                <span style={{ fontSize: 13, fontWeight: 500, color: "#262626" }}>
                  {user.username}
                </span>
              </div>
            </Dropdown>
          )}
        </div>
      </Header>

      <Layout style={{ flex: 1, overflow: "hidden" }}>
        {/* ── Sidebar ── */}
        <Sider
          width={240}
          collapsed={collapsed}
          collapsedWidth={64}
          trigger={null}
          style={{
            background: "#fff",
            borderRight: "1px solid #E5E5E5",
            flexShrink: 0,
            overflowY: "auto",
            overflowX: "hidden",
          }}
        >
          <div style={{ padding: "12px 8px" }}>
            <Menu
              mode="inline"
              selectedKeys={[activeMenuKey]}
              inlineCollapsed={collapsed}
              style={{
                borderRight: 0,
                background: "transparent",
              }}
              items={menuItems}
            />
          </div>

          {/* Sidebar Footer */}
          {!collapsed && (
            <div
              style={{
                position: "absolute",
                bottom: 0,
                left: 0,
                right: 0,
                padding: "12px 16px",
                borderTop: "1px solid #F0F0F0",
                fontSize: 11,
                color: "#BFBFBF",
                textAlign: "center",
                lineHeight: 1.5,
              }}
            >
              <div>IFRS 16 租赁管理系统</div>
              <div style={{ marginTop: 2 }}>v0.1.0</div>
            </div>
          )}
        </Sider>

        {/* ── Content ── */}
        <Content
          style={{
            padding: "32px 48px",
            background: "#FFFFFF",
            overflowY: "auto",
            overflowX: "hidden",
            minWidth: 0,
          }}
        >
          <AnimatePresence mode="wait">
            <motion.div
              key={pathname}
              initial={pageTransition.initial}
              animate={pageTransition.animate}
              exit={pageTransition.exit}
              transition={pageTransition.transition as any}
              style={{ maxWidth: 1440, margin: "0 auto" }}
            >
              {children}
            </motion.div>
          </AnimatePresence>
        </Content>
      </Layout>
    </Layout>
  );
}
