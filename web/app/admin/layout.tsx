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
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";

const { Header, Sider, Content } = Layout;

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { user, logout } = useAuth();
  const { language } = useLanguage();

  const handleLogout = () => {
    logout();
    router.push("/admin/login");
  };

  const menuItems = [
    {
      key: "/admin/users",
      icon: <TeamOutlined />,
      label: <Link href="/admin/users">{t("admin.users", language)}</Link>,
    },
  ];

  const userMenuItems = [
    {
      key: "profile",
      icon: <UserOutlined />,
      label: t("admin.profile", language),
    },
    {
      key: "logout",
      icon: <LogoutOutlined />,
      label: t("admin.logout", language),
      danger: true,
      onClick: handleLogout,
    },
  ];

  return (
    <Layout className="admin-layout">
      <Header className="admin-header">
        <div className="admin-brand">
          <SafetyOutlined className="admin-brand-icon" />
          <span className="admin-brand-title">
            {t("admin.title", language)}
          </span>
          <StatusTag kind="error" className="admin-brand-status">
            {t("admin.badge", language)}
          </StatusTag>
        </div>

        {user && (
          <Dropdown menu={{ items: userMenuItems }} placement="bottomRight">
            <div className="admin-user-trigger">
              <Avatar icon={<UserOutlined />} />
              <span>{user.username}</span>
            </div>
          </Dropdown>
        )}
      </Header>

      <Layout>
        <Sider width={200} className="admin-sider">
          <Menu
            mode="inline"
            selectedKeys={[pathname]}
            className="admin-menu"
            items={menuItems}
          />
        </Sider>

        <Content className="admin-content">
          {children}
        </Content>
      </Layout>
    </Layout>
  );
}
