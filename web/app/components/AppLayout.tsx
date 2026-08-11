"use client";

import React, { useState, useMemo, useEffect } from "react";
import { Layout, Menu, Avatar, Dropdown, Breadcrumb, Drawer } from "antd";
import {
  HomeOutlined,
  FileTextOutlined,
  RobotOutlined,
  SettingOutlined,
  LogoutOutlined,
  UserOutlined,
  BarChartOutlined,
  LineChartOutlined,
  SafetyOutlined,
  AuditOutlined,
  CalculatorOutlined,
  CheckSquareOutlined,
  GlobalOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  DollarOutlined,
  PieChartOutlined,
  SwapOutlined,
  FileSearchOutlined,
  DashboardOutlined,
} from "@ant-design/icons";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { motion, AnimatePresence } from "framer-motion";
import { hasRole, useAuth } from "../context/AuthContext";
import { LANGUAGE_LABELS, SUPPORTED_LANGUAGES, useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { pageTransition } from "../design-system/animations";
import GlobalSearch from "./GlobalSearch";
import NotificationBell from "./NotificationBell";

const { Header, Sider, Content } = Layout;

// ─── Breadcrumb Mapping ────────────────────────────────────────

function getBreadcrumbMap(language: string): Record<string, string> {
  return {
    "": t("nav.home", language as any),
    contracts: t("nav.contracts", language as any),
    todo: t("nav.todo", language as any),
    "ai-chat": t("nav.ai_chat", language as any),
    reports: t("nav.reports", language as any),
    performance: t("nav.performance", language as any),
    portfolio: t("nav.portfolio", language as any),
    sensitivity: t("nav.sensitivity", language as any),
    "deal-compare": t("nav.deal_compare", language as any),
    "pre-deal": t("nav.pre_deal", language as any),
    standards: t("nav.standards", language as any),
    "cashflow-forecast": t("nav.cashflow", language as any),
    "monthly-closing": t("nav.monthly_closing", language as any),
    roi: t("nav.roi", language as any),
    "audit-logs": t("nav.audit_logs", language as any),
    "agent-metrics": t("nav.agent_metrics", language as any),
    settings: t("nav.settings", language as any),
    admin: t("nav.admin", language as any),
    users: t("nav.users", language as any),
    new: t("nav.new", language as any),
  };
}

// ─── Menu Items ────────────────────────────────────────────────

function useMenuItems(language: string, user: ReturnType<typeof useAuth>["user"]) {
  const isAdmin = hasRole(user, "admin");
  const isAuditor = hasRole(user, "auditor");
  const isReadonly = hasRole(user, "readonly");
  const canViewAccounting = isAdmin || isAuditor || hasRole(user, "editor") || hasRole(user, "reviewer") || hasRole(user, "approver");
  // Admins need the complete decision surface. Keep role cropping for other
  // users, but never hide existing product pages from the system owner.
  const canViewAnalysis = isAdmin || isReadonly || isAuditor;
  const canViewSystem = isAdmin || isAuditor;
  const item = (key: string, href: string, icon: React.ReactNode, label: string) => ({
    key,
    icon,
    label: <Link href={href}>{label}</Link>,
  });
  return useMemo(
    () => {
      const groups: any[] = [
        {
          type: "group",
          key: "daily-work",
          label: t("nav.group_daily", language as any),
          children: [
            item("/todo", "/todo", <CheckSquareOutlined />, t("nav.todo", language as any)),
            item("/contracts", "/contracts", <FileTextOutlined />, t("nav.contracts", language as any)),
            item("/ai-chat", "/ai-chat", <RobotOutlined />, t("nav.ai_chat", language as any)),
          ],
        },
      ];
      if (canViewAnalysis) {
        groups.push({
          type: "group",
          key: "analysis",
          label: t("nav.group_analysis", language as any),
          children: [
            item("/performance", "/performance", <DashboardOutlined />, t("nav.performance", language as any)),
            item("/portfolio", "/portfolio", <PieChartOutlined />, t("nav.portfolio", language as any)),
            item("/pre-deal", "/pre-deal", <FileSearchOutlined />, t("nav.pre_deal", language as any)),
            item("/deal-compare", "/deal-compare", <SwapOutlined />, t("nav.deal_compare", language as any)),
            item("/sensitivity", "/sensitivity", <LineChartOutlined />, t("nav.sensitivity", language as any)),
            item("/cashflow-forecast", "/cashflow-forecast", <DollarOutlined />, t("nav.cashflow", language as any)),
            item("/roi", "/roi", <CalculatorOutlined />, t("nav.roi", language as any)),
          ],
        });
      }
      if (canViewAccounting) {
        groups.push({
          type: "group",
          key: "accounting",
          label: t("nav.group_accounting", language as any),
          children: [
            item("/reports", "/reports", <BarChartOutlined />, t("nav.reports", language as any)),
            item("/monthly-closing", "/monthly-closing", <CalculatorOutlined />, t("nav.monthly_closing", language as any)),
            item("/standards", "/standards", <SafetyOutlined />, t("nav.standards", language as any)),
            item("/audit-logs", "/audit-logs", <AuditOutlined />, t("nav.audit_logs", language as any)),
          ],
        });
      }
      if (canViewSystem) {
        groups.push({
          type: "group",
          key: "system",
          label: t("nav.group_system", language as any),
          children: [
            ...(isAuditor || isAdmin ? [item("/agent-metrics", "/agent-metrics", <BarChartOutlined />, t("nav.agent_metrics", language as any))] : []),
            ...(isAdmin ? [item("/settings", "/settings", <SettingOutlined />, t("nav.settings", language as any))] : []),
          ],
        });
      }
      return groups.filter((group) => group.children.length > 0);
    },
    [canViewAccounting, canViewAnalysis, canViewSystem, isAdmin, isAuditor, language]
  );
}

// ─── App Layout ────────────────────────────────────────────────

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { user, logout } = useAuth();
  const { language, setLanguage } = useLanguage();
  const [collapsed, setCollapsed] = useState(false);
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const [isMobile, setIsMobile] = useState(false);

  useEffect(() => {
    const syncViewport = () => {
      const mobile = window.innerWidth < 768;
      setIsMobile(mobile);
      setCollapsed(mobile || window.innerWidth < 1024);
      if (!mobile) setMobileNavOpen(false);
    };
    syncViewport();
    window.addEventListener("resize", syncViewport);
    return () => window.removeEventListener("resize", syncViewport);
  }, []);

  const handleLogout = () => {
    logout();
    router.push("/login");
  };

  const baseMenuItems = useMenuItems(language, user);
  const adminMenuItem = {
    key: "/admin/users",
    icon: <SafetyOutlined style={{ fontSize: 16 }} />,
    label: <Link href="/admin/users">{t("nav.admin", language)}</Link>,
  };

  const menuItems = hasRole(user, "admin")
    ? baseMenuItems.map((group: any) => group.key === "system"
      ? { ...group, children: [...group.children, adminMenuItem] }
      : group)
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
          background: "var(--fg-inverse)",
          borderBottom: "1px solid var(--border-default)",
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
          <Link
            href="/"
            aria-label={t("app.title", language)}
            className="app-logo"
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
                background: "var(--fg-primary)",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
              }}
            >
              <span aria-hidden="true" style={{ fontSize: 11, fontWeight: 800, color: "var(--fg-inverse)", letterSpacing: "-0.5px" }}>L16</span>
            </div>
            <span
              style={{
                fontSize: 16,
                fontWeight: 700,
                letterSpacing: "-0.5px",
                color: "var(--fg-primary)",
                whiteSpace: "nowrap",
              }}
            >
              {t("app.title", language)}
            </span>
          </Link>

          {/* Collapse Toggle */}
          <button
            type="button"
            aria-label={collapsed ? t("nav.expand", language) : t("nav.collapse", language)}
            aria-expanded={!collapsed}
            onClick={() => (isMobile ? setMobileNavOpen(true) : setCollapsed(!collapsed))}
            className="layout-icon-button"
            style={{
              padding: "6px",
              borderRadius: 6,
              transition: "background 0.15s",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              color: "var(--fg-tertiary)",
            }}
            onMouseEnter={(e) => (e.currentTarget.style.background = "var(--bg-inset)")}
            onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
          >
            {collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
          </button>

          {/* Breadcrumb */}
          {pathname !== "/" && (
            <div style={{ marginLeft: 8, minWidth: 0, overflow: "hidden" }}>
              <Breadcrumb
                items={breadcrumbs}
                separator={<span style={{ color: "var(--border-strong)" }}>/</span>}
                style={{ fontSize: 13 }}
              />
            </div>
          )}
        </div>

        {/* Right: Search + Notifications + Language + User */}
        <div style={{ display: "flex", alignItems: "center", gap: 8, flexShrink: 0 }}>
          {/* Global Search */}
          <GlobalSearch />

          {/* Notifications */}
          <NotificationBell />

          {/* Language switcher — shown only once more than one language is offered.
              See SUPPORTED_LANGUAGES for why it is currently a single language. */}
          {SUPPORTED_LANGUAGES.length > 1 && (
            <Dropdown
              menu={{
                items: SUPPORTED_LANGUAGES.map((code) => ({
                  key: code,
                  label: LANGUAGE_LABELS[code],
                  onClick: () => setLanguage(code),
                })),
              }}
              placement="bottomRight"
            >
              <button
                type="button"
                aria-label={t("nav.language", language)}
                aria-haspopup="menu"
                style={{
                  cursor: "pointer",
                  padding: "8px",
                  borderRadius: 8,
                  transition: "background 0.15s",
                  color: "var(--fg-tertiary)",
                  display: "flex",
                  alignItems: "center",
                  gap: 4,
                  fontSize: 13,
                  fontWeight: 500,
                }}
                onMouseEnter={(e) => (e.currentTarget.style.background = "var(--bg-inset)")}
                onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
              >
                <GlobalOutlined style={{ fontSize: 14 }} />
                <span style={{ textTransform: "uppercase", fontSize: 12 }}>
                  {language === "zh-CN" ? "CN" : language === "zh-HK" ? "HK" : "EN"}
                </span>
              </button>
            </Dropdown>
          )}

          {/* Divider */}
          <div style={{ width: 1, height: 20, background: "var(--border-default)" }} />

          {/* User */}
          {user && (
            <Dropdown menu={{ items: userMenuItems }} placement="bottomRight">
              <button
                type="button"
                aria-label={t("user.menu", language)}
                aria-haspopup="menu"
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 10,
                  cursor: "pointer",
                  padding: "6px 12px",
                  borderRadius: 9999,
                  transition: "background 0.15s",
                }}
                onMouseEnter={(e) => (e.currentTarget.style.background = "var(--bg-inset)")}
                onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
              >
                <Avatar
                  size={28}
                  icon={<UserOutlined />}
                  style={{ background: "var(--fg-primary)", fontSize: 12 }}
                />
                <span style={{ fontSize: 13, fontWeight: 500, color: "var(--fg-secondary)" }}>
                  {user.username}
                </span>
              </button>
            </Dropdown>
          )}
        </div>
      </Header>

      <Layout style={{ flex: 1, overflow: "hidden" }}>
        {/* ── Sidebar ── */}
        {!isMobile && <Sider
          width={240}
          collapsed={collapsed}
          collapsedWidth={64}
          trigger={null}
          style={{
            background: "var(--fg-inverse)",
            borderRight: "1px solid var(--border-default)",
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
                borderTop: "1px solid var(--bg-inset)",
                fontSize: 11,
                color: "var(--fg-muted)",
                textAlign: "center",
                lineHeight: 1.5,
              }}
            >
              <div>{t("app.title", language)}</div>
              <div style={{ marginTop: 2 }}>v0.1.0</div>
            </div>
          )}
        </Sider>}

        <Drawer
          title={t("app.title", language)}
          placement="left"
          open={isMobile && mobileNavOpen}
          onClose={() => setMobileNavOpen(false)}
          width={272}
          styles={{ body: { padding: "8px 0" }, header: { padding: "16px 20px" } }}
        >
          <Menu
            mode="inline"
            selectedKeys={[activeMenuKey]}
            items={menuItems}
            onClick={() => setMobileNavOpen(false)}
            style={{ borderRight: 0 }}
          />
        </Drawer>

        {/* ── Content ── */}
        <Content
          style={{
            padding: "32px 48px",
            background: "var(--fg-inverse)",
            overflowY: "auto",
            overflowX: "hidden",
            minWidth: 0,
          }}
        >
          <AnimatePresence mode="wait">
            <motion.div
              key={pathname}
              initial={false}
              animate={pageTransition.animate}
              exit={undefined}
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
