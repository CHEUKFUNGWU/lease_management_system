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

  const menuItems = [
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
      key: "/settings",
      icon: <SettingOutlined />,
      label: <Link href="/settings">设置</Link>,
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
          background: "#fff",
          borderBottom: "1px solid #f0f0f0",
          padding: "0 24px",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
          <RobotOutlined style={{ fontSize: 24, color: "#1890ff" }} />
          <span style={{ fontSize: 18, fontWeight: 600 }}>
            IFRS 16 租赁管理系统
          </span>
        </div>

        {user && (
          <Dropdown menu={{ items: userMenuItems }} placement="bottomRight">
            <div style={{ display: "flex", alignItems: "center", gap: 8, cursor: "pointer" }}>
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
            background: "#fff",
            borderRight: "1px solid #f0f0f0",
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
            background: "#fff",
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
