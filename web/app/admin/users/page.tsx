"use client";

import { StatusTag, statusKindFromAntColor } from "../../components/StatusTag";
import PageHeader from "../../components/PageHeader";

import { useEffect, useState } from "react";
import {
  Card,
  Table,
  Button,
  Modal,
  Form,
  Input,
  Select,
  message,
  Tag,
  Space,
} from "antd";
import {
  PlusOutlined,
  UserOutlined,
} from "@ant-design/icons";
import { hasRole, useAuth } from "../../context/AuthContext";
import { useLanguage } from "../../context/LanguageContext";
import { useRouter } from "next/navigation";
import { adminApi, legalEntityApi } from "../../lib/api";
import { useRetailQuery } from "../../retail/useRetailQuery";
import { t } from "../../lib/i18n";
import { notifyError } from "../../lib/notify";

interface User {
  id: string;
  username: string;
  email: string;
  role: string;
  roles?: string[];
  legal_entity_id?: string;
  is_active: boolean;
  created_at: string;
}

export default function AdminUsersPage() {
  const [modalVisible, setModalVisible] = useState(false);
  const [form] = Form.useForm();
  const [legalEntities, setLegalEntities] = useState<any[]>([]);
  const { user, token, isLoading } = useAuth();
  const { language } = useLanguage();
  const router = useRouter();

  useEffect(() => {
    if (isLoading) return;
    if (!hasRole(user, "admin")) {
      notifyError(t("admin_users.need_admin", language));
      router.push("/login");
      return;
    }
    fetchLegalEntities();
  // eslint-disable-next-line react-hooks/exhaustive-deps -- P2-C gate close-out: legacy dep semantics kept as-is; loaders are rebuilt every render so adding them would loop refetches. useCallback refactor tracked separately; do not add new exemptions.
  }, [isLoading, user, token]);

  // FETCH-003: the user list runs through the shared fetch seam.
  const usersQuery = useRetailQuery({
    token,
    params: { all: true as const },
    paramsKey: "admin-users",
    fetcher: (p, t) => adminApi.listUsers(t).then((res) => res.data ?? []),
  });
  const users: User[] = usersQuery.state.kind === "ready" ? (usersQuery.state.data ?? []) : [];
  const loading = usersQuery.loading;
  useEffect(() => {
    if (usersQuery.state.kind === "failed") notifyError(usersQuery.state.message || t("admin_users.load_failed", language));
  }, [usersQuery.state, language]);

  const fetchLegalEntities = async () => {
    if (!token) return;
    try {
      const data = await legalEntityApi.list(token);
      setLegalEntities(data.legal_entities || []);
    } catch {
      setLegalEntities([]);
    }
  };

  const handleCreateUser = async (values: any) => {
    if (!token) return;
    try {
      await adminApi.createUser(
        {
          username: values.username,
          email: values.email,
          password: values.password,
          roles: values.roles,
          legal_entity_id: values.legal_entity_id || undefined,
        },
        token
      );
      message.success(t("admin_users.create_success", language));
      setModalVisible(false);
      form.resetFields();
      usersQuery.retry();
    } catch (error: any) {
      notifyError(error.message || t("admin_users.create_failed", language));
    }
  };

  const roleColorMap: Record<string, string> = {
    admin: "red",
    editor: "gold",
    reviewer: "blue",
    approver: "green",
    auditor: "cyan",
    readonly: "default",
    user: "default",
  };

  const roleLabelMap: Record<string, string> = {
    admin: t("admin_users.role_admin", language),
    editor: "Finance Editor",
    reviewer: t("admin_users.role_reviewer", language),
    approver: t("admin_users.role_approver", language),
    auditor: "Auditor Readonly",
    readonly: "Business Readonly",
    user: t("admin_users.role_user", language),
  };

  const columns = [
    {
      title: t("admin_users.col_username", language),
      dataIndex: "username",
      key: "username",
    },
    {
      title: t("admin_users.col_email", language),
      dataIndex: "email",
      key: "email",
    },
    {
      title: t("admin_users.col_role", language),
      dataIndex: "role",
      key: "role",
      render: (role: string, record: User) => (
        <Space size={[4, 4]} wrap>
          {(record.roles?.length ? record.roles : [role]).map((assignedRole) => (
            <StatusTag key={assignedRole} kind={statusKindFromAntColor(roleColorMap[assignedRole] || "default")}>
              {roleLabelMap[assignedRole] || assignedRole}
            </StatusTag>
          ))}
        </Space>
      ),
    },
    {
      title: t("admin_users.col_legal_entity", language),
      dataIndex: "legal_entity_id",
      key: "legal_entity_id",
      render: (id: string) => {
        const entity = legalEntities.find((e) => e.id === id);
        return entity ? `${entity.code} - ${entity.name}` : id || "-";
      },
    },
    {
      title: t("admin_users.col_status", language),
      dataIndex: "is_active",
      key: "is_active",
      render: (active: boolean) => (
        <StatusTag kind={statusKindFromAntColor(active ? "success" : "default")}>
          {active ? t("admin_users.status_active", language) : t("admin_users.status_disabled", language)}
        </StatusTag>
      ),
    },
    {
      title: t("admin_users.col_created_at", language),
      dataIndex: "created_at",
      key: "created_at",
      render: (date: string) => new Date(date).toLocaleString("zh-CN"),
    },
  ];

  return (
    <div>
      <PageHeader
        title={<><UserOutlined /> {t("admin_users.title", language)}<span className="page-header-count">{t("admin_users.subtitle", language, { count: String(users.length) })}</span></>}
        primaryAction={
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setModalVisible(true)}
          >
            {t("admin_users.new_user", language)}
          </Button>
        }
      />

      <Card>
        <Table
          dataSource={users}
          columns={columns}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 10 }}
        />
      </Card>

      <Modal
        title={t("admin_users.modal_title", language)}
        open={modalVisible}
        onCancel={() => {
          setModalVisible(false);
          form.resetFields();
        }}
        footer={null}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleCreateUser}
          autoComplete="off"
        >
          <Form.Item
            name="username"
            label={t("admin_users.label_username", language)}
            rules={[
              { required: true, message: t("admin_users.username_placeholder", language) },
              { min: 3, message: t("admin_users.username_min", language) },
            ]}
          >
            <Input placeholder={t("admin_users.username_placeholder", language)} />
          </Form.Item>

          <Form.Item
            name="email"
            label={t("admin_users.label_email", language)}
            rules={[
              { required: true, message: t("admin_users.email_placeholder", language) },
              { type: "email", message: t("admin_users.email_invalid", language) },
            ]}
          >
            <Input placeholder={t("admin_users.email_placeholder", language)} />
          </Form.Item>

          <Form.Item
            name="password"
            label={t("admin_users.label_password", language)}
            rules={[
              { required: true, message: t("admin_users.password_placeholder", language) },
              { min: 6, message: t("admin_users.password_min", language) },
            ]}
          >
            <Input.Password placeholder={t("admin_users.password_placeholder", language)} />
          </Form.Item>

          <Form.Item
            name="roles"
            label={t("admin_users.label_role", language)}
            initialValue={["readonly"]}
            rules={[{ required: true, message: t("admin_users.role_placeholder", language) }]}
          >
            <Select mode="multiple" placeholder={t("admin_users.role_placeholder", language)}>
              <Select.Option value="editor">Finance Editor</Select.Option>
              <Select.Option value="reviewer">{t("admin_users.role_reviewer", language)}</Select.Option>
              <Select.Option value="approver">{t("admin_users.role_approver", language)}</Select.Option>
              <Select.Option value="auditor">Auditor Readonly</Select.Option>
              <Select.Option value="readonly">Business Readonly</Select.Option>
              <Select.Option value="admin">{t("admin_users.role_admin", language)}</Select.Option>
            </Select>
          </Form.Item>

          <Form.Item
            name="legal_entity_id"
            label={t("admin_users.label_legal_entity", language)}
            rules={[{ required: true, message: t("admin_users.legal_entity_placeholder", language) }]}
          >
            <Select
              placeholder={t("admin_users.legal_entity_placeholder", language)}
              options={legalEntities.map((e) => ({
                value: e.id,
                label: `${e.code} - ${e.name}`,
              }))}
            />
          </Form.Item>

          <Form.Item>
            <Space style={{ width: "100%", justifyContent: "flex-end" }}>
              <Button onClick={() => setModalVisible(false)}>{t("admin_users.cancel", language)}</Button>
              <Button type="primary" htmlType="submit">
                {t("admin_users.create", language)}
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
