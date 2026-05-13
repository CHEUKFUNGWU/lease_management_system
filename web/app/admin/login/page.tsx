"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import {
  Form,
  Input,
  Button,
  Card,
  Typography,
  message,
  Row,
  Col,
} from "antd";
import { SafetyOutlined, LockOutlined, UserOutlined } from "@ant-design/icons";
import { useAuth } from "../../context/AuthContext";
import { authApi } from "../../lib/api";

const { Title, Text } = Typography;

export default function AdminLoginPage() {
  const [loading, setLoading] = useState(false);
  const router = useRouter();
  const { login } = useAuth();

  const handleLogin = async (values: any) => {
    setLoading(true);
    try {
      const data = await authApi.login(values.username, values.password);
      if (data.role !== "admin") {
        message.error("非管理员账号，请使用普通登录入口");
        setLoading(false);
        return;
      }
      login(data.token, {
        id: data.user_id || "",
        username: data.username,
        role: data.role,
        legal_entity_id: data.legal_entity_id || undefined,
      });
      message.success("管理员登录成功！");
      router.push("/admin/users");
    } catch (error: any) {
      message.error(error.message || "登录失败");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Row
      justify="center"
      align="middle"
      style={{ minHeight: "100vh", background: "#001529" }}
    >
      <Col xs={24} sm={16} md={12} lg={8} xl={6}>
        <Card>
          <div style={{ textAlign: "center", marginBottom: 24 }}>
            <SafetyOutlined style={{ fontSize: 48, color: "#cf1322" }} />
            <Title level={3} style={{ marginTop: 16 }}>
              管理后台登录
            </Title>
            <Text type="secondary">
              IFRS 16 租赁管理系统 — 管理员通道
            </Text>
          </div>

          <Form
            name="admin-login"
            onFinish={handleLogin}
            autoComplete="off"
            layout="vertical"
          >
            <Form.Item
              name="username"
              rules={[{ required: true, message: "请输入管理员用户名" }]}
            >
              <Input
                prefix={<UserOutlined />}
                placeholder="管理员用户名"
                size="large"
              />
            </Form.Item>

            <Form.Item
              name="password"
              rules={[{ required: true, message: "请输入密码" }]}
            >
              <Input.Password
                prefix={<LockOutlined />}
                placeholder="密码"
                size="large"
              />
            </Form.Item>

            <Form.Item>
              <Button
                type="primary"
                htmlType="submit"
                loading={loading}
                size="large"
                block
                danger
              >
                管理员登录
              </Button>
            </Form.Item>
          </Form>

          <div style={{ textAlign: "center", marginTop: 16 }}>
            <Text type="secondary" style={{ fontSize: 12 }}>
              <a href="/login">返回普通用户登录</a>
            </Text>
          </div>
        </Card>
      </Col>
    </Row>
  );
}
