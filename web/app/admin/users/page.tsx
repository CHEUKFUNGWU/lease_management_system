"use client";

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
  Typography,
} from "antd";
import {
  PlusOutlined,
  UserOutlined,
} from "@ant-design/icons";
import { useAuth } from "../../context/AuthContext";
import { useLanguage } from "../../context/LanguageContext";
import { useRouter } from "next/navigation";
import { adminApi, legalEntityApi } from "../../lib/api";
import { ROLE_COLOR_MAP, ROLE_I18N_KEYS } from "../../lib/constants/roles";
import { t } from "../../lib/i18n";
import {
  type AdminUser,
  ASSIGNABLE_USER_ROLES,
  type CreateUserRequest,
} from "../../lib/types/auth";
import type { LegalEntityOption } from "../../lib/types/master-data";

const { Title } = Typography;

export default function AdminUsersPage() {
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [form] = Form.useForm();
  const [legalEntities, setLegalEntities] = useState<LegalEntityOption[]>([]);
  const { user, token } = useAuth();
  const { language } = useLanguage();
  const router = useRouter();

  useEffect(() => {
    if (!user || !(user.roles?.includes("admin") || user.role === "admin")) {
      message.error(t("admin_users.need_admin", language));
      router.push("/login");
      return;
    }
    fetchUsers();
    fetchLegalEntities();
  }, [user, token]);

  const fetchUsers = async () => {
    if (!token) return;
    setLoading(true);
    try {
      const data = await adminApi.listUsers(token);
      setUsers(data.data || []);
    } catch (error: any) {
      message.error(error.message || t("admin_users.load_failed", language));
    } finally {
      setLoading(false);
    }
  };

  const fetchLegalEntities = async () => {
    if (!token) return;
    try {
      const data = await legalEntityApi.list(token);
      setLegalEntities(data.legal_entities || []);
    } catch {
      setLegalEntities([]);
    }
  };

  const handleCreateUser = async (values: CreateUserRequest) => {
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
      fetchUsers();
    } catch (error: any) {
      message.error(error.message || t("admin_users.create_failed", language));
    }
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
      render: (role: AdminUser["role"], record: AdminUser) => (
        <Space size={[4, 4]} wrap>
          {(record.roles?.length ? record.roles : [role]).map((assignedRole) => (
            <Tag key={assignedRole} color={ROLE_COLOR_MAP[assignedRole] || "default"}>
              {t(ROLE_I18N_KEYS[assignedRole] || "admin_users.role_user", language)}
            </Tag>
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
        <Tag color={active ? "success" : "default"}>
          {active ? t("admin_users.status_active", language) : t("admin_users.status_disabled", language)}
        </Tag>
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
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          marginBottom: 24,
        }}
      >
        <Title level={3} style={{ margin: 0 }}>
          <UserOutlined /> {t("admin_users.title", language)}
        </Title>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => setModalVisible(true)}
        >
          {t("admin_users.new_user", language)}
        </Button>
      </div>

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
              {ASSIGNABLE_USER_ROLES.map((role) => (
                <Select.Option key={role} value={role}>
                  {t(ROLE_I18N_KEYS[role], language)}
                </Select.Option>
              ))}
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
