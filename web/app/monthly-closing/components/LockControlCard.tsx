"use client";

import { Alert, Button, Card, Empty, Skeleton, Space, Tag } from "antd";
import { LockOutlined, UnlockOutlined } from "@ant-design/icons";
import { t, type Language } from "../../lib/i18n";

interface LockControlCardProps {
  language: Language;
  selectedPeriod: string;
  isLocked: boolean;
  lockLoading: boolean;
  lockStatusLoading: boolean;
  isApprover: boolean;
  isAdmin: boolean;
  onLock: () => void;
  onUnlock: () => void;
  onRefreshStatus: () => void;
}

export function LockControlCard({
  language,
  selectedPeriod,
  isLocked,
  lockLoading,
  lockStatusLoading,
  isApprover,
  isAdmin,
  onLock,
  onUnlock,
  onRefreshStatus,
}: LockControlCardProps) {
  return (
    <Card
      title={
        <span style={{ fontSize: 15, fontWeight: 600, letterSpacing: "-0.01em" }}>
          {t("monthly.lock_control", language)}
        </span>
      }
    >
      {!selectedPeriod ? (
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={t("monthly.lock_first", language)}
        />
      ) : lockStatusLoading ? (
        <div style={{ padding: "24px 0" }}>
          <Skeleton active paragraph={{ rows: 2 }} />
        </div>
      ) : (
        <>
          <div
            style={{
              display: "flex",
              alignItems: "center",
              gap: 20,
              padding: "24px 28px",
              borderRadius: 10,
              marginBottom: 24,
              border: `1px solid ${
                isLocked ? "var(--border-strong)" : "var(--border-default)"
              }`,
              background: isLocked ? "var(--bg-inset)" : "var(--bg-page)",
            }}
          >
            <div
              style={{
                width: 48,
                height: 48,
                borderRadius: 10,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                fontSize: 22,
                background: isLocked ? "var(--fg-primary)" : "var(--bg-inset)",
                color: isLocked ? "var(--fg-inverse)" : "var(--fg-tertiary)",
                flexShrink: 0,
              }}
            >
              {isLocked ? <LockOutlined /> : <UnlockOutlined />}
            </div>
            <div style={{ flex: 1 }}>
              <div
                style={{
                  fontSize: 15,
                  fontWeight: 600,
                  color: "var(--fg-primary)",
                  marginBottom: 2,
                }}
              >
                {t("monthly.accounting_period_label", language)} {selectedPeriod}
              </div>
              <div
                style={{
                  fontSize: 13,
                  color: "var(--fg-muted)",
                }}
              >
                {isLocked
                  ? t("monthly.lock_desc_locked", language)
                  : t("monthly.lock_desc_unlocked", language)}
              </div>
            </div>
            <div style={{ flexShrink: 0 }}>
              {isLocked ? (
                <Tag color="error" style={{ margin: 0, fontSize: 13 }}>
                  <LockOutlined style={{ marginRight: 4 }} />
                  {t("monthly.locked", language)}
                </Tag>
              ) : (
                <Tag color="success" style={{ margin: 0, fontSize: 13 }}>
                  <UnlockOutlined style={{ marginRight: 4 }} />
                  {t("monthly.unlocked", language)}
                </Tag>
              )}
            </div>
          </div>

          <Space>
            {!isLocked ? (
              <Button
                type="primary"
                icon={<LockOutlined />}
                loading={lockLoading}
                disabled={!isApprover}
                onClick={onLock}
              >
                {isApprover
                  ? t("monthly.lock_btn", language)
                  : t("monthly.lock_btn_disabled", language)}
              </Button>
            ) : (
              <Button
                icon={<UnlockOutlined />}
                loading={lockLoading}
                disabled={!isAdmin}
                onClick={onUnlock}
              >
                {isAdmin
                  ? t("monthly.unlock_btn", language)
                  : t("monthly.unlock_btn_disabled", language)}
              </Button>
            )}
            <Button onClick={onRefreshStatus} loading={lockStatusLoading}>
              {t("monthly.refresh_status", language)}
            </Button>
          </Space>

          {!isAdmin && isLocked && (
            <Alert
              message={t("monthly.contact_admin", language)}
              type="info"
              showIcon
              style={{ marginTop: 16 }}
            />
          )}
        </>
      )}
    </Card>
  );
}
