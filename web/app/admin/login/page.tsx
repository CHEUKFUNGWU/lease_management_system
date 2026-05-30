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
import { useLanguage } from "../../context/LanguageContext";
import { authApi } from "../../lib/api";
import { t } from "../../lib/i18n";

const { Title, Text } = Typography;

export default function AdminLoginPage() {
  const [loading, setLoading] = useState(false);
  const router = useRouter();
  const { login } = useAuth();
  const { language } = useLanguage();

  const handleLogin = async (values: any) => {
    setLoading(true);
    try {
      const data = await authApi.login(values.username, values.password);
      if (data.role !== "admin") {
        message.error(t("admin_login.not_admin", language));
        setLoading(false);
        return;
      }
      login(data.token, {
        id: data.user_id || "",
        username: data.username,
        role: data.role,
        legal_entity_id: data.legal_entity_id || undefined,
      });
      message.success(t("admin_login.success", language));
      router.push("/admin/users");
    } catch (error: any) {
      message.error(error.message || t("admin_login.failed", language));
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
                {t("admin_login.title", language)}
              </Title>
              <Text type="secondary">
                {t("admin_login.subtitle", language)}
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
              rules={[{ required: true, message: t("admin_login.username_required", language) }]}
            >
              <Input
                prefix={<UserOutlined />}
                placeholder={t("admin_login.username_placeholder", language)}
                size="large"
              />
            </Form.Item>

            <Form.Item
              name="password"
              rules={[{ required: true, message: t("admin_login.password_required", language) }]}
            >
              <Input.Password
                prefix={<LockOutlined />}
                placeholder={t("admin_login.password_placeholder", language)}
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
                {t("admin_login.login_button", language)}
              </Button>
            </Form.Item>
          </Form>

          <div style={{ textAlign: "center", marginTop: 16 }}>
            <Text type="secondary" style={{ fontSize: 12 }}>
              <a href="/login">{t("admin_login.back_to_user", language)}</a>
            </Text>
          </div>
        </Card>
      </Col>
    </Row>
  );
}
