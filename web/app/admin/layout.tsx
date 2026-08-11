"use client";

import { StatusTag } from "../components/StatusTag";

import React from "react";
import { Layout, Menu, Button, Avatar, Dropdown, Tag } from "antd";
import {
  SafetyOutlined,
  UserOutlined,
  TeamOutlined,
  LogoutOutlined,
  SettingOutlined,
} from "@ant-design/icons";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useAuth } from "../context/AuthContext";

const { Header, Sider, Content } = Layout;

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { user, logout } = useAuth();

  const handleLogout = () => {
    logout();
    router.push("/admin/login");
  };

  const menuItems = [
    {
      key: "/admin/users",
      icon: <TeamOutlined />,
      label: <Link href="/admin/users">用户管理</Link>,
    },
  ];

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
          background: "var(--admin-surface)",
          padding: "0 24px",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <SafetyOutlined style={{ fontSize: 24, color: "var(--state-error-text)" }} />
          <span style={{ fontSize: 18, fontWeight: 600, color: "var(--fg-inverse)" }}>
            管理后台
          </span>
          <StatusTag kind="error" style={{ marginLeft: 8 }}>
            ADMIN
          </StatusTag>
        </div>

        {user && (
          <Dropdown menu={{ items: userMenuItems }} placement="bottomRight">
            <div style={{ display: "flex", alignItems: "center", gap: 8, cursor: "pointer", color: "var(--fg-inverse)" }}>
              <Avatar icon={<UserOutlined />} />
              <span>{user.username}</span>
            </div>
          </Dropdown>
        )}
      </Header>

      <Layout>
        <Sider
          width={200}
          style={{
            background: "var(--fg-inverse)",
            borderRight: "1px solid var(--bg-inset)",
          }}
        >
          <Menu
            mode="inline"
            selectedKeys={[pathname]}
            style={{ height: "100%", borderRight: 0 }}
            items={menuItems}
          />
        </Sider>

        <Content
          style={{
            margin: 24,
            padding: 24,
            background: "var(--fg-inverse)",
            borderRadius: 8,
            minHeight: 280,
          }}
        >
          {children}
        </Content>
      </Layout>
    </Layout>
  );
}
