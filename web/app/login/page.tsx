"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Form, Input, Button, message } from "antd";
import { LockOutlined, UserOutlined, SafetyOutlined, ArrowRightOutlined } from "@ant-design/icons";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { ApiError, authApi } from "../lib/api";
import { notifyError } from "../lib/notify";
import BrandIcon from "../components/BrandIcon";

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
        roles: data.roles || [data.role],
        legal_entity_id: data.legal_entity_id || undefined,
      }, data.refresh_token);
      message.success(t("login.success", language));
      router.push("/");
    } catch (error: any) {
      // The shared mapper turns any codeless 401 into "session expired", which
      // is wrong here: failing to sign in is not a lapsed session. Until the
      // auth handler carries an error code, the login page owns this case.
      const credentialsRejected = error instanceof ApiError && error.status === 401;
      notifyError(
        credentialsRejected
          ? t("login.invalid_credentials", language)
          : error.message || t("login.failed", language),
      );
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="login-shell">
      <section className="login-brand">
        <div className="login-brand-inner">
          <div className="login-lockup">
            <span className="login-badge" aria-hidden="true">
              <BrandIcon size={26} variant="inverse" />
            </span>
            <span className="login-lockup-name">{t("login.title", language)}</span>
          </div>
          <p className="login-tagline">{t("login.tagline", language)}</p>
          <ul className="login-points">
            <li className="login-point">{t("login.point_pulse", language)}</li>
            <li className="login-point">{t("login.point_store", language)}</li>
            <li className="login-point">{t("login.point_lease", language)}</li>
          </ul>
        </div>
      </section>

      <section className="login-panel">
        <div className="login-panel-inner">
          <div className="login-lockup login-lockup-compact">
            <span className="login-badge" aria-hidden="true">
              <BrandIcon size={26} />
            </span>
            <span className="login-lockup-name">{t("login.title", language)}</span>
          </div>

          <h1 className="login-heading">{t("login.welcome_back", language)}</h1>
          <p className="login-subheading">{t("login.continue_hint", language)}</p>

          <Form
            name="login"
            className="login-form"
            onFinish={handleLogin}
            autoComplete="off"
            layout="vertical"
            requiredMark={false}
          >
            <Form.Item
              name="username"
              rules={[{ required: true, message: t("login.username_required", language) }]}
            >
              <Input
                prefix={<UserOutlined />}
                placeholder={t("login.username", language)}
                size="large"
                autoComplete="username"
              />
            </Form.Item>

            <Form.Item
              name="password"
              rules={[{ required: true, message: t("login.password_required", language) }]}
            >
              <Input.Password
                prefix={<LockOutlined />}
                placeholder={t("login.password", language)}
                size="large"
                autoComplete="current-password"
              />
            </Form.Item>

            <Form.Item className="login-submit-item">
              <Button type="primary" htmlType="submit" loading={loading} size="large" block>
                {t("login.submit", language)}
              </Button>
            </Form.Item>
          </Form>

          <p className="login-note">
            <SafetyOutlined aria-hidden="true" />
            {t("login.no_register", language)}
          </p>
        </div>
      </section>
    </div>
  );
}
