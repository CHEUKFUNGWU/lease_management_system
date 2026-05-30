"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { motion } from "framer-motion";
import {
  Form,
  Input,
  Button,
  Card,
  Typography,
  message,
  Row,
  Col,
  Divider,
} from "antd";
import { RobotOutlined, LockOutlined, UserOutlined, SafetyOutlined } from "@ant-design/icons";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { authApi } from "../lib/api";

const { Title, Text } = Typography;

export default function LoginPage() {
  const [loading, setLoading] = useState(false);
  const router = useRouter();
  const { login } = useAuth();
  const { language } = useLanguage();

  const handleLogin = async (values: any) => {
    setLoading(true);
    try {
      const data = await authApi.login(values.username, values.password);
      login(data.token, {
        id: data.user_id || "",
        username: data.username,
        role: data.role,
        legal_entity_id: data.legal_entity_id || undefined,
      });
      message.success(t("login.success", language));
      router.push("/");
    } catch (error: any) {
      message.error(error.message || t("login.failed", language));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Row
      justify="center"
      align="middle"
      style={{
        minHeight: "100vh",
        background: "#F7F7F7",
        position: "relative",
      }}
    >
      {/* Subtle background pattern */}
      <div
        style={{
          position: "absolute",
          inset: 0,
          backgroundImage: `radial-gradient(circle at 1px 1px, #E5E5E5 1px, transparent 0)`,
          backgroundSize: "32px 32px",
          opacity: 0.5,
        }}
      />

      <Col xs={24} sm={16} md={12} lg={8} xl={6} style={{ position: "relative", zIndex: 1 }}>
        <motion.div
          initial={{ opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.35, ease: [0.4, 0, 0.2, 1] }}
        >
          <Card
            style={{
              borderRadius: 16,
              boxShadow: "0 0 0 1px rgba(0, 0, 0, 0.04), 0 8px 24px rgba(0, 0, 0, 0.08)",
              border: "none",
            }}
            bodyStyle={{ padding: "40px 36px" }}
          >
            <div style={{ textAlign: "center", marginBottom: 32 }}>
              <motion.div
                initial={{ scale: 0.9, opacity: 0 }}
                animate={{ scale: 1, opacity: 1 }}
                transition={{ delay: 0.1, duration: 0.3 }}
                style={{
                  width: 56,
                  height: 56,
                  borderRadius: 14,
                  background: "#000",
                  display: "inline-flex",
                  alignItems: "center",
                  justifyContent: "center",
                  marginBottom: 20,
                }}
              >
                <RobotOutlined style={{ fontSize: 28, color: "#fff" }} />
              </motion.div>

              <Title
                level={3}
                style={{
                  margin: "0 0 8px",
                  fontSize: 22,
                  fontWeight: 700,
                  letterSpacing: "-0.03em",
                  color: "#000",
                }}
              >
                {t("login.title", language)}
              </Title>
            </div>

            <Form
              name="login"
              onFinish={handleLogin}
              autoComplete="off"
              layout="vertical"
            >
              <Form.Item
                name="username"
                rules={[{ required: true, message: t("login.username_required", language) }]}
                style={{ marginBottom: 20 }}
              >
                <Input
                  prefix={
                    <UserOutlined style={{ color: "#BFBFBF", fontSize: 16 }} />
                  }
                  placeholder={t("login.username", language)}
                  size="large"
                  style={{
                    borderRadius: 10,
                    height: 44,
                    fontSize: 14,
                    paddingLeft: 12,
                  }}
                />
              </Form.Item>

              <Form.Item
                name="password"
                rules={[{ required: true, message: t("login.password_required", language) }]}
                style={{ marginBottom: 24 }}
              >
                <Input.Password
                  prefix={
                    <LockOutlined style={{ color: "#BFBFBF", fontSize: 16 }} />
                  }
                  placeholder={t("login.password", language)}
                  size="large"
                  style={{
                    borderRadius: 10,
                    height: 44,
                    fontSize: 14,
                    paddingLeft: 12,
                  }}
                />
              </Form.Item>

              <Form.Item style={{ marginBottom: 0 }}>
                <Button
                  type="primary"
                  htmlType="submit"
                  loading={loading}
                  size="large"
                  block
                  style={{
                    height: 44,
                    borderRadius: 9999,
                    fontSize: 15,
                    fontWeight: 600,
                    boxShadow: "none",
                  }}
                >
                  {t("login.submit", language)}
                </Button>
              </Form.Item>
            </Form>

            <Divider style={{ margin: "24px 0 16px", borderColor: "#F0F0F0" }} />

            <div style={{ textAlign: "center" }}>
              <SafetyOutlined style={{ fontSize: 12, color: "#BFBFBF", marginRight: 6 }} />
              <Text
                style={{
                  fontSize: 12,
                  color: "#BFBFBF",
                  lineHeight: 1.5,
                }}
              >
                {t("login.no_register", language)}
              </Text>
            </div>
          </Card>
        </motion.div>
      </Col>
    </Row>
  );
}
