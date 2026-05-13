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
import { useRouter } from "next/navigation";
import { adminApi, legalEntityApi } from "../../lib/api";

const { Title } = Typography;

interface User {
  id: string;
  username: string;
  email: string;
  role: string;
  legal_entity_id?: string;
  is_active: boolean;
  created_at: string;
}

export default function AdminUsersPage() {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [form] = Form.useForm();
  const [legalEntities, setLegalEntities] = useState<any[]>([]);
  const { user, token } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!user || user.role !== "admin") {
      message.error("需要管理员权限");
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
      message.error(error.message || "获取用户列表失败");
    } finally {
      setLoading(false);
    }
  };

  const fetchLegalEntities = async () => {
    try {
      const data = await legalEntityApi.list();
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
          role: values.role,
          legal_entity_id: values.legal_entity_id || undefined,
        },
        token
      );
      message.success("用户创建成功");
      setModalVisible(false);
      form.resetFields();
      fetchUsers();
    } catch (error: any) {
      message.error(error.message || "创建用户失败");
    }
  };

  const roleColorMap: Record<string, string> = {
    admin: "red",
    reviewer: "blue",
    approver: "green",
    user: "default",
  };

  const roleLabelMap: Record<string, string> = {
    admin: "管理员",
    reviewer: "复核员",
    approver: "审批员",
    user: "普通用户",
  };

  const columns = [
    {
      title: "用户名",
      dataIndex: "username",
      key: "username",
    },
    {
      title: "邮箱",
      dataIndex: "email",
      key: "email",
    },
    {
      title: "角色",
      dataIndex: "role",
      key: "role",
      render: (role: string) => (
        <Tag color={roleColorMap[role] || "default"}>
          {roleLabelMap[role] || role}
        </Tag>
      ),
    },
    {
      title: "法人主体",
      dataIndex: "legal_entity_id",
      key: "legal_entity_id",
      render: (id: string) => {
        const entity = legalEntities.find((e) => e.id === id);
        return entity ? `${entity.code} - ${entity.name}` : id || "-";
      },
    },
    {
      title: "状态",
      dataIndex: "is_active",
      key: "is_active",
      render: (active: boolean) => (
        <Tag color={active ? "success" : "default"}>
          {active ? "活跃" : "禁用"}
        </Tag>
      ),
    },
    {
      title: "创建时间",
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
          <UserOutlined /> 用户管理
        </Title>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => setModalVisible(true)}
        >
          新建用户
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
        title="新建用户"
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
            label="用户名"
            rules={[
              { required: true, message: "请输入用户名" },
              { min: 3, message: "至少3个字符" },
            ]}
          >
            <Input placeholder="用户名" />
          </Form.Item>

          <Form.Item
            name="email"
            label="邮箱"
            rules={[
              { required: true, message: "请输入邮箱" },
              { type: "email", message: "请输入有效邮箱" },
            ]}
          >
            <Input placeholder="邮箱" />
          </Form.Item>

          <Form.Item
            name="password"
            label="密码"
            rules={[
              { required: true, message: "请输入密码" },
              { min: 6, message: "至少6个字符" },
            ]}
          >
            <Input.Password placeholder="密码" />
          </Form.Item>

          <Form.Item
            name="role"
            label="角色"
            initialValue="user"
            rules={[{ required: true, message: "请选择角色" }]}
          >
            <Select placeholder="选择角色">
              <Select.Option value="user">普通用户</Select.Option>
              <Select.Option value="reviewer">复核员</Select.Option>
              <Select.Option value="approver">审批员</Select.Option>
              <Select.Option value="admin">管理员</Select.Option>
            </Select>
          </Form.Item>

          <Form.Item
            name="legal_entity_id"
            label="所属法人"
            rules={[{ required: true, message: "请选择所属法人" }]}
          >
            <Select
              placeholder="选择所属法人"
              options={legalEntities.map((e) => ({
                value: e.id,
                label: `${e.code} - ${e.name}`,
              }))}
            />
          </Form.Item>

          <Form.Item>
            <Space style={{ width: "100%", justifyContent: "flex-end" }}>
              <Button onClick={() => setModalVisible(false)}>取消</Button>
              <Button type="primary" htmlType="submit">
                创建
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
