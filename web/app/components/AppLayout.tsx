"use client";

import React from "react";
import { Layout, Menu, Button, Avatar, Dropdown, FloatButton, Select } from "antd";
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
} from "@ant-design/icons";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";

const { Header, Sider, Content } = Layout;

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { user, logout } = useAuth();
  const { language, setLanguage } = useLanguage();

  const handleLogout = () => {
    logout();
    router.push("/login");
  };

  const buildAIChatUrl = () => {
    const params = new URLSearchParams();

    if (pathname === "/reports") {
      params.set("page", "reports");
      params.set("title", "报表查询");
    } else if (pathname === "/monthly-closing") {
      params.set("page", "monthly-closing");
      params.set("title", "结账中心");
    } else if (pathname.startsWith("/contracts/") && pathname !== "/contracts/new") {
      const segments = pathname.split("/").filter(Boolean);
      const contractId = segments[1];
      if (contractId) {
        params.set("page", "contract-detail");
        params.set("title", "合同详情");
        params.set("contract_id", contractId);
      }
    } else if (pathname === "/settings") {
      params.set("page", "settings");
      params.set("title", "设置 / 标签总管");
    } else if (pathname === "/cashflow-forecast") {
      params.set("page", "cashflow-forecast");
      params.set("title", "现金流预测");
    }

    const query = params.toString();
    return query ? `/ai-chat?${query}` : "/ai-chat";
  };

  const baseMenuItems = [
    {
      key: "/",
      icon: <HomeOutlined />,
      label: <Link href="/">{t("nav.home", language)}</Link>,
    },
    {
      key: "/contracts",
      icon: <FileTextOutlined />,
      label: <Link href="/contracts">{t("nav.contracts", language)}</Link>,
    },
    {
      key: "/upload",
      icon: <UploadOutlined />,
      label: <Link href="/upload">{t("nav.upload", language)}</Link>,
    },
    {
      key: "/ai-chat",
      icon: <RobotOutlined />,
      label: <Link href="/ai-chat">{t("nav.ai_chat", language)}</Link>,
    },
    {
      key: "/reports",
      icon: <BarChartOutlined />,
      label: <Link href="/reports">{t("nav.reports", language)}</Link>,
    },
    {
      key: "/cashflow-forecast",
      icon: <LineChartOutlined />,
      label: <Link href="/cashflow-forecast">{t("nav.cashflow", language)}</Link>,
    },
    {
      key: "/monthly-closing",
      icon: <CalculatorOutlined />,
      label: <Link href="/monthly-closing">{t("nav.monthly_closing", language)}</Link>,
    },
    {
      key: "/audit-logs",
      icon: <AuditOutlined />,
      label: <Link href="/audit-logs">{t("nav.audit_logs", language)}</Link>,
    },
    {
      key: "/settings",
      icon: <SettingOutlined />,
      label: <Link href="/settings">{t("nav.settings", language)}</Link>,
    },
  ];

  const adminMenuItem = {
    key: "/admin/users",
    icon: <SafetyOutlined />,
    label: <Link href="/admin/users">{t("nav.admin", language)}</Link>,
  };

  const menuItems = user?.role === "admin"
    ? [...baseMenuItems, adminMenuItem]
    : baseMenuItems;

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

  return (
    <Layout style={{ minHeight: "100vh" }}>
      <Header
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          background: "#fff",
          borderBottom: "1px solid #EAEAEA",
          padding: "0 32px",
          height: 64,
          position: "sticky",
          top: 0,
          zIndex: 100,
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <div
            style={{
              width: 32,
              height: 32,
              borderRadius: 8,
              background: "#000",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            <RobotOutlined style={{ fontSize: 18, color: "#fff" }} />
          </div>
          <span
            style={{
              fontSize: 18,
              fontWeight: 700,
              letterSpacing: "-0.5px",
              color: "#000",
            }}
          >
            IFRS 16
          </span>
          <span
            style={{
              fontSize: 12,
              color: "#999",
              fontWeight: 400,
              marginLeft: 4,
            }}
          >
            {t("app.title", language)}
          </span>
        </div>

        <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
          <Select
            value={language}
            onChange={(value) => setLanguage(value as any)}
            style={{ width: 120 }}
            variant="borderless"
            options={[
              { value: "zh-CN", label: t("lang.zh_CN", language) },
              { value: "zh-TW", label: t("lang.zh_TW", language) },
              { value: "en", label: t("lang.en", language) },
            ]}
            suffixIcon={<GlobalOutlined />}
          />

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
                onMouseEnter={(e) =>
                  (e.currentTarget.style.background = "#F5F5F5")
                }
                onMouseLeave={(e) =>
                  (e.currentTarget.style.background = "transparent")
                }
              >
                <Avatar
                  size={32}
                  icon={<UserOutlined />}
                  style={{ background: "#000" }}
                />
                <span style={{ fontSize: 14, fontWeight: 500 }}>
                  {user.username}
                </span>
              </div>
            </Dropdown>
          )}
        </div>
      </Header>

      <Layout>
        <Sider
          width={220}
          style={{
            background: "#fff",
            borderRight: "1px solid #EAEAEA",
            padding: "12px 8px",
            position: "sticky",
            top: 64,
            height: "calc(100vh - 64px)",
            overflowY: "auto",
          }}
        >
          <Menu
            mode="inline"
            selectedKeys={[pathname]}
            style={{
              height: "100%",
              borderRight: 0,
              background: "transparent",
            }}
            items={menuItems}
          />
        </Sider>

        <Content
          style={{
            padding: "32px 48px",
            background: "#FFFFFF",
            minHeight: "calc(100vh - 64px)",
            maxWidth: 1440,
            width: "100%",
          }}
        >
          {children}
        </Content>
      </Layout>

      <FloatButton
        icon={<RobotOutlined />}
        tooltip={<span>{t("nav.ai_chat", language)}</span>}
        onClick={() => router.push(buildAIChatUrl())}
        style={{
          right: 28,
          bottom: 28,
        }}
        type="primary"
      />
    </Layout>
  );
}
