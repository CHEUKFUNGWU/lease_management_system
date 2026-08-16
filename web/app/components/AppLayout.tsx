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
import ThemeToggle from "./ThemeToggle";
import BrandIcon from "./BrandIcon";

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
    "operating-pulse": t("nav.operating_pulse", language as any),
    "store-360": t("nav.store_360", language as any),
    "scenario-workbench": t("nav.scenario_workbench", language as any),
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
      const groups: any[] = [];
      if (canViewAnalysis) {
        groups.push({
          type: "group",
          key: "analysis",
          label: t("nav.group_analysis", language as any),
          children: [
            item("/operating-pulse", "/operating-pulse", <LineChartOutlined />, t("nav.operating_pulse", language as any)),
            item("/store-360", "/store-360", <LineChartOutlined />, t("nav.store_360", language as any)),
            item("/scenario-workbench", "/scenario-workbench", <LineChartOutlined />, t("nav.scenario_workbench", language as any)),
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
      groups.push({
        type: "group",
        key: "daily-work",
        label: t("nav.group_daily", language as any),
        children: [
          item("/todo", "/todo", <CheckSquareOutlined />, t("nav.todo", language as any)),
          item("/contracts", "/contracts", <FileTextOutlined />, t("nav.contracts", language as any)),
          item("/ai-chat", "/ai-chat", <RobotOutlined />, t("nav.ai_chat", language as any)),
        ],
      });
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
    icon: <SafetyOutlined className="app-menu-admin-icon" />,
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
      { title: <Link href="/"><HomeOutlined className="app-breadcrumb-home-icon" /></Link> },
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
    <Layout className="app-root">
      {/* ── Header ── */}
      <Header className="app-header">
        {/* Left: Logo + Collapse + Breadcrumb */}
        <div className="app-header-left">
          {/* Logo */}
          <Link
            href="/"
            aria-label={t("app.title", language)}
            className="app-logo"
          >
            <BrandIcon size={28} className="app-logo-icon" />
            <span className="app-logo-title">
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
          >
            {collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
          </button>

          {/* Breadcrumb */}
          {pathname !== "/" && (
            <div className="app-breadcrumb-wrap">
              <Breadcrumb
                items={breadcrumbs}
                separator={<span className="app-breadcrumb-separator">/</span>}
                className="app-breadcrumb"
              />
            </div>
          )}
        </div>

        {/* Right: Search + Notifications + Language + User */}
        <div className="app-header-right">
          {/* Global Search */}
          <GlobalSearch />

          {/* Notifications */}
          <NotificationBell />

          {/* DARK-001: theme toggle (follows OS by default, manual choice persists) */}
          <ThemeToggle />

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
                className="app-language-button"
              >
                <GlobalOutlined className="app-language-icon" />
                <span className="app-language-code">
                  {language === "zh-CN" ? "CN" : language === "zh-HK" ? "HK" : "EN"}
                </span>
              </button>
            </Dropdown>
          )}

          {/* Divider */}
          <div className="app-header-divider" />

          {/* User */}
          {user && (
            <Dropdown menu={{ items: userMenuItems }} placement="bottomRight">
              <button
                type="button"
                aria-label={t("user.menu", language)}
                aria-haspopup="menu"
                className="app-user-button"
              >
                <Avatar
                  size={28}
                  icon={<UserOutlined />}
                  className="app-avatar"
                />
                <span className="app-username">
                  {user.username}
                </span>
              </button>
            </Dropdown>
          )}
        </div>
      </Header>

      <Layout className="app-content-shell">
        {/* ── Sidebar ── */}
        {!isMobile && <Sider
          width={240}
          collapsed={collapsed}
          collapsedWidth={64}
          trigger={null}
          className="app-sider"
        >
          <div className="app-sider-inner">
            <Menu
              mode="inline"
              selectedKeys={[activeMenuKey]}
              inlineCollapsed={collapsed}
              className="app-sider-menu"
              items={menuItems}
            />
          </div>

          {/* Sidebar Footer */}
          {!collapsed && (
            <div className="app-sider-footer">
              <div>{t("app.title", language)}</div>
              <div className="app-sider-footer-version">v0.1.0</div>
            </div>
          )}
        </Sider>}

        <Drawer
          title={t("app.title", language)}
          placement="left"
          open={isMobile && mobileNavOpen}
          onClose={() => setMobileNavOpen(false)}
          width={272}
          classNames={{ body: "app-drawer-body", header: "app-drawer-header" }}
        >
          <Menu
            mode="inline"
            selectedKeys={[activeMenuKey]}
            items={menuItems}
            onClick={() => setMobileNavOpen(false)}
            className="app-drawer-menu"
          />
        </Drawer>

        {/* ── Content ── */}
        <Content
          className="app-content"
        >
          <AnimatePresence mode="wait">
            <motion.div
              key={pathname}
              initial={false}
              animate={pageTransition.animate}
              exit={undefined}
              transition={pageTransition.transition as any}
              className="app-content-inner"
            >
              {children}
            </motion.div>
          </AnimatePresence>
        </Content>
      </Layout>
    </Layout>
  );
}
