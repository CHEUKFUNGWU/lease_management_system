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
      message.success("机器凭据颁发成功！");
      setNewlyIssued({
        client_id: res.client_id,
        client_secret: (res as any).client_secret || "",
      });
      setModalOpen(false);
      form.resetFields();
      loadCredentials();
    } catch (err: any) {
      message.error(err?.message || "颁发失败");
    }
  };

  const handleRevoke = async (clientID: string) => {
    if (!token) return;
    try {
      await machineCredentialApi.revoke(clientID, token);
      message.success("凭据已成功吊销");
      loadCredentials();
    } catch (err: any) {
      message.error(err?.message || "吊销失败");
    }
  };

  const copyText = (text: string) => {
    if (navigator.clipboard) {
      navigator.clipboard.writeText(text);
      message.success("已复制到剪贴板");
    }
  };

  const columns = [
    {
      title: "凭据名称",
      dataIndex: "name",
      key: "name",
      width: 160,
      render: (name: string) => <Text strong>{name}</Text>,
    },
    {
      title: "Client ID",
      dataIndex: "client_id",
      key: "client_id",
      width: 220,
      render: (id: string) => <Text code>{id}</Text>,
    },
    {
      title: "权限 Scope",
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
      title: "状态",
      key: "status",
      width: 100,
      render: (_: any, r: MachineCredential) =>
        r.revoked_at ? (
          <Tag color="error" icon={<StopOutlined />}>
            已吊销
          </Tag>
        ) : (
          <Tag color="success" icon={<CheckCircleOutlined />}>
            有效
          </Tag>
        ),
    },
    {
      title: "最近调用",
      dataIndex: "last_used_at",
      key: "last_used_at",
      width: 160,
      render: (t?: string) => (t ? t.replace("T", " ").substring(0, 19) : "从未调用"),
    },
    {
      title: "操作",
      key: "actions",
      width: 100,
      render: (_: any, r: MachineCredential) =>
        !r.revoked_at && (
          <Popconfirm
            title="确定吊销该机器凭据？"
            description="吊销后使用该凭据的 POS/外部系统推送将立即被拒绝。"
            onConfirm={() => handleRevoke(r.client_id)}
          >
            <Button size="small" danger type="link">
              吊销
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
          <span>机器凭据与外部数据接入 (Machine API & Feeds)</span>
        </Space>
      }
      extra={
        <Button type="primary" size="small" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>
          颁发机器凭据
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
            message="请立即保存 Client Secret（密钥仅展示一次，关闭后不可再次查看）"
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
        <Card size="small" title={<Space><CodeOutlined /><span>POS / ERP 外部推送接口调用示例</span></Space>}>
          <Paragraph style={{ fontSize: 12, marginBottom: 8 }}>
            外部系统可通过标准 HTTP POST 调用推送 store-day 事实数据（支持 <code>Idempotency-Key</code> 幂等重放）：
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
        title="颁发机器凭据 (Client Credentials)"
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => form.submit()}
      >
        <Form form={form} layout="vertical" onFinish={handleIssue}>
          <Form.Item
            name="name"
            label="凭据用途名称"
            rules={[{ required: true, message: "请输入凭据名称" }]}
          >
            <Input placeholder="例: 上海区 POS 每日自动同步" />
          </Form.Item>
          <Form.Item
            name="scopes"
            label="授权范围 (Scopes)"
            initialValue={["operating_facts:write"]}
          >
            <Select mode="multiple">
              <Select.Option value="operating_facts:write">经营事实写入 (operating_facts:write)</Select.Option>
              <Select.Option value="store:read">门店主数据只读 (store:read)</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item
            name="expires_in_days"
            label="有效期 (天)"
            initialValue={365}
          >
            <InputNumber style={{ width: "100%" }} min={1} />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}
