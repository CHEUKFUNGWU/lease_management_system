"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { useRouter } from "next/navigation";
import { Table, Button, Tag, Space, message, Input, Select } from "antd";
import { PlusOutlined, EyeOutlined, SearchOutlined } from "@ant-design/icons";
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

const STATUS_OPTIONS = [
  { value: "", label: "全部状态" },
  { value: "draft", label: "草稿" },
  { value: "submitted", label: "已提交" },
  { value: "reviewed", label: "已复核" },
  { value: "pending_approval", label: "待审批" },
  { value: "approved", label: "已审批" },
  { value: "rejected", label: "已驳回" },
  { value: "returned_to_editor", label: "退回编辑" },
];

const STATUS_LABELS: Record<string, string> = {
  draft: "草稿",
  submitted: "已提交",
  reviewed: "已复核",
  pending_approval: "待审批",
  approved: "已审批",
  rejected: "已驳回",
  returned_to_editor: "退回编辑",
};

const STATUS_COLORS: Record<string, string> = {
  draft: "default",
  submitted: "processing",
  reviewed: "processing",
  pending_approval: "warning",
  approved: "success",
  rejected: "error",
  returned_to_editor: "orange",
};

export default function ContractsPage() {
  const [contracts, setContracts] = useState<Contract[]>([]);
  const [loading, setLoading] = useState(false);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [sortBy, setSortBy] = useState("created_at");
  const [sortOrder, setSortOrder] = useState("desc");
  const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const router = useRouter();
  const { token } = useAuth();

  const loadContracts = useCallback(async (
    searchVal: string,
    statusVal: string,
    sortByVal: string,
    sortOrderVal: string,
  ) => {
    if (!token) return;
    setLoading(true);
    try {
      const data = await contractApi.list(token, {
        search: searchVal || undefined,
        status: statusVal || undefined,
        sort_by: sortByVal || undefined,
        sort_order: sortOrderVal || undefined,
      });
      setContracts(data.data || []);
    } catch (error: any) {
      message.error(error.message || "加载合同失败");
    } finally {
      setLoading(false);
    }
  }, [token]);

  // Initial load + reload when filters change (except debounced search)
  useEffect(() => {
    loadContracts(search, statusFilter, sortBy, sortOrder);
  }, [loadContracts, statusFilter, sortBy, sortOrder]);

  // Debounced search
  const handleSearchChange = (value: string) => {
    setSearch(value);
    if (debounceTimer.current) {
      clearTimeout(debounceTimer.current);
    }
    debounceTimer.current = setTimeout(() => {
      loadContracts(value, statusFilter, sortBy, sortOrder);
    }, 300);
  };

  // Cleanup debounce timer
  useEffect(() => {
    return () => {
      if (debounceTimer.current) {
        clearTimeout(debounceTimer.current);
      }
    };
  }, []);

  const columns = [
    {
      title: "合同编号",
      dataIndex: "contract_number",
      key: "contract_number",
      sorter: true,
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
      sorter: true,
    },
    {
      title: "租期结束日",
      dataIndex: "lease_end_date",
      key: "lease_end_date",
      sorter: true,
    },
    {
      title: "审批状态",
      dataIndex: "approval_status",
      key: "approval_status",
      sorter: true,
      render: (status: string, record: Contract) => {
        return (
          <Space>
            <Tag color={STATUS_COLORS[status] || "default"}>{STATUS_LABELS[status] || status}</Tag>
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
          <Button type="link" icon={<EyeOutlined />} onClick={() => router.push(`/contracts/${record.id}`)}>
            查看
          </Button>
        </Space>
      ),
    },
  ];

  const handleTableChange = (_pagination: any, _filters: any, sorter: any) => {
    if (sorter.field) {
      // Map column key to sort_by param
      setSortBy(sorter.field);
      setSortOrder(sorter.order === "ascend" ? "asc" : "desc");
    }
  };

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

        <div
          style={{
            display: "flex",
            gap: 12,
            marginBottom: 16,
            flexWrap: "wrap",
          }}
        >
          <Input
            placeholder="搜索合同编号、名称、承租方、出租方、门店..."
            prefix={<SearchOutlined />}
            value={search}
            onChange={(e) => handleSearchChange(e.target.value)}
            allowClear
            style={{ width: 360 }}
          />
          <Select
            value={statusFilter}
            onChange={(value) => setStatusFilter(value)}
            options={STATUS_OPTIONS}
            style={{ width: 140 }}
          />
        </div>

        <Table
          columns={columns}
          dataSource={contracts}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 10, showSizeChanger: true, showTotal: (total) => `共 ${total} 条` }}
          onChange={handleTableChange}
        />
      </AppLayout>
    </ProtectedRoute>
  );
}
