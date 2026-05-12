"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Table, Button, Tag, Space, message } from "antd";
import { PlusOutlined, EyeOutlined } from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { contractApi } from "../lib/api";
import { useAuth } from "../context/AuthContext";

interface Contract {
  id: string;
  contract_number: string;
  contract_name: string;
  legal_entity_id: string;
  store_id: string;
  landlord_id: string;
  currency: string;
  commencement_date: string;
  lease_start_date: string;
  lease_end_date: string;
  status: string;
  approval_status: string;
  is_official_version: boolean;
  discount_rate_missing: boolean;
  created_at: string;
}

export default function ContractsPage() {
  const [contracts, setContracts] = useState<Contract[]>([]);
  const [loading, setLoading] = useState(false);
  const router = useRouter();
  const { token } = useAuth();

  useEffect(() => {
    loadContracts();
  }, [token]);

  const loadContracts = async () => {
    if (!token) return;
    setLoading(true);
    try {
      const data = await contractApi.list(token);
      setContracts(data.data || []);
    } catch (error: any) {
      message.error(error.message || "加载合同失败");
    } finally {
      setLoading(false);
    }
  };

  const columns = [
    {
      title: "合同编号",
      dataIndex: "contract_number",
      key: "contract_number",
    },
    {
      title: "合同名称",
      dataIndex: "contract_name",
      key: "contract_name",
    },
    {
      title: "币种",
      dataIndex: "currency",
      key: "currency",
    },
    {
      title: "租赁起始日",
      dataIndex: "commencement_date",
      key: "commencement_date",
    },
    {
      title: "租期结束日",
      dataIndex: "lease_end_date",
      key: "lease_end_date",
    },
    {
      title: "审批状态",
      dataIndex: "approval_status",
      key: "approval_status",
      render: (status: string, record: Contract) => {
        const colors: Record<string, string> = {
          draft: "default",
          submitted: "processing",
          reviewed: "processing",
          pending_approval: "warning",
          approved: "success",
          rejected: "error",
          returned_to_editor: "orange",
        };
        const labels: Record<string, string> = {
          draft: "草稿",
          submitted: "已提交",
          reviewed: "已复核",
          pending_approval: "待审批",
          approved: "已审批",
          rejected: "已驳回",
          returned_to_editor: "退回编辑",
        };
        return (
          <Space>
            <Tag color={colors[status] || "default"}>{labels[status] || status}</Tag>
            {record.is_official_version && <Tag color="blue">正式版</Tag>}
            {!record.is_official_version && status !== "draft" && <Tag>工作版</Tag>}
          </Space>
        );
      },
    },
    {
      title: "折现率",
      key: "discount_rate",
      render: (_: any, record: Contract) => (
        <span>
          {record.discount_rate_missing ? (
            <Tag color="error">缺失</Tag>
          ) : (
            <Tag color="success">已设置</Tag>
          )}
        </span>
      ),
    },
    {
      title: "操作",
      key: "action",
      render: (_: any, record: Contract) => (
        <Space size="small">
          <Button type="link" icon={<EyeOutlined />}>
            查看
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <ProtectedRoute>
      <AppLayout>
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            marginBottom: 16,
          }}
        >
          <h1>合同台账</h1>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => router.push("/contracts/new")}>
            新增合同
          </Button>
        </div>

        <Table
          columns={columns}
          dataSource={contracts}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 10 }}
        />
      </AppLayout>
    </ProtectedRoute>
  );
}
