"use client";

import React from "react";
import { Layout, Menu, Button, Avatar, Dropdown } from "antd";
import {
  HomeOutlined,
  FileTextOutlined,
  UploadOutlined,
  RobotOutlined,
  SettingOutlined,
  LogoutOutlined,
  UserOutlined,
  BarChartOutlined,
  SafetyOutlined,
  AuditOutlined,
  CalculatorOutlined,
} from "@ant-design/icons";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useAuth } from "../context/AuthContext";

const { Header, Sider, Content } = Layout;

export default function AppLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { user, logout } = useAuth();

  const handleLogout = () => {
    logout();
    router.push("/login");
  };

  const baseMenuItems = [
    {
      key: "/",
      icon: <HomeOutlined />,
      label: <Link href="/">首页</Link>,
    },
    {
      key: "/contracts",
      icon: <FileTextOutlined />,
      label: <Link href="/contracts">合同台账</Link>,
    },
    {
      key: "/upload",
      icon: <UploadOutlined />,
      label: <Link href="/upload">文件上传</Link>,
    },
    {
      key: "/ai-chat",
      icon: <RobotOutlined />,
      label: <Link href="/ai-chat">AI 助手</Link>,
    },
    {
      key: "/reports",
      icon: <BarChartOutlined />,
      label: <Link href="/reports">报表查询</Link>,
    },
    {
      key: "/monthly-closing",
      icon: <CalculatorOutlined />,
      label: <Link href="/monthly-closing">月结跑批</Link>,
    },
    {
      key: "/audit-logs",
      icon: <AuditOutlined />,
      label: <Link href="/audit-logs">审计日志</Link>,
    },
    {
      key: "/settings",
      icon: <SettingOutlined />,
      label: <Link href="/settings">设置</Link>,
    },
  ];

  const adminMenuItem = {
    key: "/admin/users",
    icon: <SafetyOutlined />,
    label: <Link href="/admin/users">管理后台</Link>,
  };

  const menuItems = user?.role === "admin"
    ? [...baseMenuItems, adminMenuItem]
    : baseMenuItems;

  const userMenuItems = [
    {
      key: "profile",
      icon: <UserOutlined />,
      label: "个人资料",
    },
    {
      key: "logout",
      icon: <LogoutOutlined />,
      label: "退出登录",
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
            租赁管理系统
          </span>
        </div>

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
    </Layout>
  );
}
