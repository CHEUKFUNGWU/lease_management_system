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
  Tabs,
  Row,
  Col,
  DatePicker,
  Select,
  InputNumber,
} from "antd";
import { RobotOutlined, LockOutlined, UserOutlined, MailOutlined } from "@ant-design/icons";
import { useAuth } from "../context/AuthContext";
import { authApi } from "../lib/api";

const { Title } = Typography;

export default function LoginPage() {
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState("login");
  const router = useRouter();
  const { login } = useAuth();

  const handleLogin = async (values: any) => {
    setLoading(true);
    try {
      const data = await authApi.login(values.username, values.password);
      login(data.token, {
        id: data.user_id || "",
        username: data.username,
        role: data.role,
      });
      message.success("登录成功！");
      router.push("/");
    } catch (error: any) {
      message.error(error.message || "登录失败");
    } finally {
      setLoading(false);
    }
  };

  const handleRegister = async (values: any) => {
    setLoading(true);
    try {
      const data = await authApi.register(
        values.username,
        values.email,
        values.password,
        values.role || "user"
      );
      login(data.token, {
        id: data.user_id || "",
        username: data.username,
        role: data.role,
      });
      message.success("注册成功！");
      router.push("/");
    } catch (error: any) {
      message.error(error.message || "注册失败");
    } finally {
      setLoading(false);
    }
  };

  const loginForm = (
    <Form
      name="login"
      onFinish={handleLogin}
      autoComplete="off"
      layout="vertical"
    >
      <Form.Item
        name="username"
        rules={[{ required: true, message: "请输入用户名" }]}
      >
        <Input
          prefix={<UserOutlined />}
          placeholder="用户名"
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
        >
          登录
        </Button>
      </Form.Item>
    </Form>
  );

  const registerForm = (
    <Form
      name="register"
      onFinish={handleRegister}
      autoComplete="off"
      layout="vertical"
    >
      <Form.Item
        name="username"
        rules={[
          { required: true, message: "请输入用户名" },
          { min: 3, message: "用户名至少3个字符" },
        ]}
      >
        <Input
          prefix={<UserOutlined />}
          placeholder="用户名"
          size="large"
        />
      </Form.Item>

      <Form.Item
        name="email"
        rules={[
          { required: true, message: "请输入邮箱" },
          { type: "email", message: "请输入有效的邮箱" },
        ]}
      >
        <Input
          prefix={<MailOutlined />}
          placeholder="邮箱"
          size="large"
        />
      </Form.Item>

      <Form.Item
        name="password"
        rules={[
          { required: true, message: "请输入密码" },
          { min: 6, message: "密码至少6个字符" },
        ]}
      >
        <Input.Password
          prefix={<LockOutlined />}
          placeholder="密码"
          size="large"
        />
      </Form.Item>

      <Form.Item
        name="role"
        initialValue="user"
      >
        <Select placeholder="选择角色" size="large">
          <Select.Option value="user">普通用户</Select.Option>
          <Select.Option value="reviewer">复核员</Select.Option>
          <Select.Option value="approver">审批员</Select.Option>
          <Select.Option value="admin">管理员</Select.Option>
        </Select>
      </Form.Item>

      <Form.Item>
        <Button
          type="primary"
          htmlType="submit"
          loading={loading}
          size="large"
          block
        >
          注册
        </Button>
      </Form.Item>
    </Form>
  );

  return (
    <Row
      justify="center"
      align="middle"
      style={{ minHeight: "100vh", background: "#f0f2f5" }}
    >
      <Col xs={24} sm={16} md={12} lg={8} xl={6}>
        <Card>
          <div style={{ textAlign: "center", marginBottom: 24 }}>
            <RobotOutlined style={{ fontSize: 48, color: "#1890ff" }} />
            <Title level={3} style={{ marginTop: 16 }}>
              IFRS 16 租赁管理系统
            </Title>
          </div>

          <Tabs
            activeKey={activeTab}
            onChange={setActiveTab}
            centered
            items={[
              { key: "login", label: "登录", children: loginForm },
              { key: "register", label: "注册", children: registerForm },
            ]}
          />
        </Card>
      </Col>
    </Row>
  );
}
