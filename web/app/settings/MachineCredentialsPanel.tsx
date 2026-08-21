"use client";

import { useEffect, useState } from "react";
import {
  Card,
  Table,
  Button,
  Tag,
  Space,
  Modal,
  Form,
  Input,
  InputNumber,
  Select,
  Typography,
  Alert,
  message,
  Popconfirm,
} from "antd";
import {
  KeyOutlined,
  PlusOutlined,
  CopyOutlined,
  CheckCircleOutlined,
  StopOutlined,
  CodeOutlined,
} from "@ant-design/icons";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { machineCredentialApi, type MachineCredential } from "../lib/api";
import { tableScrollX } from "../lib/tableScroll";

const { Text, Paragraph } = Typography;

export function MachineCredentialsPanel() {
  const { language } = useLanguage();
  const { token } = useAuth();

  const [loading, setLoading] = useState(false);
  const [credentials, setCredentials] = useState<MachineCredential[]>([]);
  const [modalOpen, setModalOpen] = useState(false);
  const [newlyIssued, setNewlyIssued] = useState<{ client_id: string; client_secret: string } | null>(null);

  const [form] = Form.useForm();

  const loadCredentials = async () => {
    if (!token) return;
    setLoading(true);
    try {
      const res = await machineCredentialApi.list(token);
      setCredentials(res.credentials || []);
    } catch (err: any) {
      // quiet on non-admin
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadCredentials();
  // eslint-disable-next-line react-hooks/exhaustive-deps -- P2-C gate close-out: legacy dep semantics kept as-is; loaders are rebuilt every render so adding them would loop refetches. useCallback refactor tracked separately; do not add new exemptions.
  }, [token]);

  const handleIssue = async (values: any) => {
    if (!token) return;
    try {
      const res = await machineCredentialApi.issue(
        {
          name: values.name,
          scopes: values.scopes || ["operating_facts:write"],
          expires_in_days: values.expires_in_days || 365,
        },
        token
      );
      message.success(t("machine_cred.issue_success", language));
      setNewlyIssued({
        client_id: res.client_id,
        client_secret: (res as any).client_secret || "",
      });
      setModalOpen(false);
      form.resetFields();
      loadCredentials();
    } catch (err: any) {
      message.error(err?.message || t("machine_cred.issue_failed", language));
    }
  };

  const handleRevoke = async (clientID: string) => {
    if (!token) return;
    try {
      await machineCredentialApi.revoke(clientID, token);
      message.success(t("machine_cred.revoke_success", language));
      loadCredentials();
    } catch (err: any) {
      message.error(err?.message || t("machine_cred.revoke_failed", language));
    }
  };

  const copyText = (text: string) => {
    if (navigator.clipboard) {
      navigator.clipboard.writeText(text);
      message.success(t("machine_cred.copied", language));
    }
  };

  const columns = [
    {
      title: t("machine_cred.col_name", language),
      dataIndex: "name",
      key: "name",
      width: 160,
      render: (name: string) => <Text strong>{name}</Text>,
    },
    {
      title: t("machine_cred.col_client_id", language),
      dataIndex: "client_id",
      key: "client_id",
      width: 220,
      render: (id: string) => <Text code>{id}</Text>,
    },
    {
      title: t("machine_cred.col_scopes", language),
      dataIndex: "scopes",
      key: "scopes",
      width: 200,
      render: (scopes: string[]) => (
        <Space wrap size={[4, 4]}>
          {(scopes || []).map((s) => (
            <Tag color="blue" key={s}>
              {s}
            </Tag>
          ))}
        </Space>
      ),
    },
    {
      title: t("machine_cred.col_status", language),
      key: "status",
      width: 100,
      render: (_: any, r: MachineCredential) =>
        r.revoked_at ? (
          <Tag color="error" icon={<StopOutlined />}>
            {t("machine_cred.status_revoked", language)}
          </Tag>
        ) : (
          <Tag color="success" icon={<CheckCircleOutlined />}>
            {t("machine_cred.status_active", language)}
          </Tag>
        ),
    },
    {
      title: t("machine_cred.col_last_used", language),
      dataIndex: "last_used_at",
      key: "last_used_at",
      width: 160,
      render: (used?: string) => (used ? used.replace("T", " ").substring(0, 19) : t("machine_cred.never_used", language)),
    },
    {
      title: t("machine_cred.col_actions", language),
      key: "actions",
      width: 100,
      render: (_: any, r: MachineCredential) =>
        !r.revoked_at && (
          <Popconfirm
            title={t("machine_cred.revoke_confirm_title", language)}
            description={t("machine_cred.revoke_confirm_desc", language)}
            onConfirm={() => handleRevoke(r.client_id)}
          >
            <Button size="small" danger type="link">
              {t("machine_cred.revoke_btn", language)}
            </Button>
          </Popconfirm>
        ),
    },
  ];

  return (
    <Card
      title={
        <Space>
          <KeyOutlined />
          <span>{t("machine_cred.title", language)}</span>
        </Space>
      }
      extra={
        <Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>
          {t("machine_cred.issue_btn", language)}
        </Button>
      }
      style={{ marginBottom: 16 }}
    >
      <Space direction="vertical" size={16} style={{ width: "100%" }}>
        {newlyIssued && (
          <Alert
            type="warning"
            showIcon
            closable
            onClose={() => setNewlyIssued(null)}
            message={t("machine_cred.secret_alert_msg", language)}
            description={
              <Space direction="vertical" style={{ width: "100%", marginTop: 8 }}>
                <div>
                  <Text strong>Client ID: </Text>
                  <Text code>{newlyIssued.client_id}</Text>
                  <Button
                    size="small"
                    type="link"
                    icon={<CopyOutlined />}
                    onClick={() => copyText(newlyIssued.client_id)}
                  />
                </div>
                <div>
                  <Text strong>Client Secret: </Text>
                  <Text code copyable>{newlyIssued.client_secret}</Text>
                </div>
              </Space>
            }
          />
        )}

        <Table
          dataSource={credentials}
          columns={columns}
          rowKey="client_id"
          loading={loading}
          pagination={false}
          size="small"
          scroll={tableScrollX(credentials.length, 840)}
        />

        {/* Push API curl example */}
        <Card size="small" title={<Space><CodeOutlined /><span>{t("machine_cred.curl_example_title", language)}</span></Space>}>
          <Paragraph style={{ fontSize: 12, marginBottom: 8 }}>
            {t("machine_cred.curl_example_desc", language)}
          </Paragraph>
          <pre style={{ background: "var(--bg-subtle, #f5f5f5)", padding: 12, borderRadius: 4, fontSize: 11, overflowX: "auto" }}>
{`curl -X POST https://<domain>/api/v1/retail/push/facts \\
  -H "X-Client-ID: <your_client_id>" \\
  -H "X-Client-Secret: <your_client_secret>" \\
  -H "Content-Type: application/json" \\
  -d '{
    "source_system": "pos_stream_daily",
    "facts": [
      {
        "store": "S001",
        "business_date": "2026-06-01",
        "currency": "CNY",
        "revenue": 18500.00,
        "gross_profit": 7400.00,
        "transactions": 450
      }
    ]
  }'`}
          </pre>
        </Card>
      </Space>

      <Modal
        title={t("machine_cred.modal_title", language)}
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => form.submit()}
      >
        <Form form={form} layout="vertical" onFinish={handleIssue}>
          <Form.Item
            name="name"
            label={t("machine_cred.label_name", language)}
            rules={[{ required: true, message: t("machine_cred.rule_name_required", language) }]}
          >
            <Input placeholder={t("machine_cred.placeholder_name", language)} />
          </Form.Item>
          <Form.Item
            name="scopes"
            label={t("machine_cred.label_scopes", language)}
            initialValue={["operating_facts:write"]}
          >
            <Select mode="multiple">
              <Select.Option value="operating_facts:write">{t("machine_cred.scope_facts_write", language)}</Select.Option>
              <Select.Option value="store:read">{t("machine_cred.scope_store_read", language)}</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item
            name="expires_in_days"
            label={t("machine_cred.label_expires", language)}
            initialValue={365}
          >
            <InputNumber style={{ width: "100%" }} min={1} />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}
