"use client";

import { useEffect, useState, useCallback, useRef } from "react";
import { useRouter } from "next/navigation";
import { motion } from "framer-motion";
import {
  Table,
  Button,
  Tag,
  Space,
  message,
  Input,
  Select,
  Card,
  Empty,
  Skeleton,
  Badge,
} from "antd";
import {
  PlusOutlined,
  EyeOutlined,
  SearchOutlined,
  FilterOutlined,
  SortAscendingOutlined,
  SortDescendingOutlined,
  ArrowRightOutlined,
} from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { contractApi } from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { staggerContainer, staggerItem } from "../design-system/animations";

interface Contract {
  id: string;
  contract_number: string;
  contract_name: string;
  legal_entity_id: string;
  store_id: string;
  landlord_id: string;
  currency: string;
  asset_type: string;
  commencement_date: string;
  lease_start_date: string;
  lease_end_date: string;
  status: string;
  approval_status: string;
  is_official_version: boolean;
  discount_rate_missing: boolean;
  lease_scope: string;
  created_at: string;
}

const STATUS_COLORS: Record<string, string> = {
  draft: "default",
  submitted: "processing",
  reviewed: "processing",
  pending_approval: "warning",
  approved: "success",
  rejected: "error",
  returned_to_editor: "orange",
};

const LEASE_SCOPE_LABELS: Record<string, string> = {
  in_scope: "资本化",
  short_term_exempt: "短期豁免",
  low_value_exempt: "低价值豁免",
  not_a_lease: "非租赁",
};

const LEASE_SCOPE_COLORS: Record<string, string> = {
  in_scope: "blue",
  short_term_exempt: "gold",
  low_value_exempt: "purple",
  not_a_lease: "default",
};

const ASSET_TYPE_LABELS: Record<string, string> = {
  real_estate: "不动产",
  vehicle: "车辆",
  it_equipment: "IT 设备",
  machinery: "机器设备",
  other: "其他",
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
  const { language } = useLanguage();

  const STATUS_LABELS: Record<string, string> = {
    draft: t("status.draft", language),
    submitted: t("status.submitted", language),
    reviewed: t("status.reviewed", language),
    pending_approval: t("status.pending_approval", language),
    approved: t("status.approved", language),
    rejected: t("status.rejected", language),
    returned_to_editor: t("status.returned_to_editor", language),
  };

  const STATUS_OPTIONS = [
    { value: "", label: t("contracts.all_status", language) },
    { value: "draft", label: t("status.draft", language) },
    { value: "submitted", label: t("status.submitted", language) },
    { value: "reviewed", label: t("status.reviewed", language) },
    { value: "pending_approval", label: t("status.pending_approval", language) },
    { value: "approved", label: t("status.approved", language) },
    { value: "rejected", label: t("status.rejected", language) },
    { value: "returned_to_editor", label: t("status.returned_to_editor", language) },
  ];

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
      message.error(error.message || t("contracts.load_failed", language));
    } finally {
      setLoading(false);
    }
  }, [token, language]);

  useEffect(() => {
    loadContracts(search, statusFilter, sortBy, sortOrder);
  }, [loadContracts, statusFilter, sortBy, sortOrder]);

  const handleSearchChange = (value: string) => {
    setSearch(value);
    if (debounceTimer.current) {
      clearTimeout(debounceTimer.current);
    }
    debounceTimer.current = setTimeout(() => {
      loadContracts(value, statusFilter, sortBy, sortOrder);
    }, 300);
  };

  useEffect(() => {
    return () => {
      if (debounceTimer.current) {
        clearTimeout(debounceTimer.current);
      }
    };
  }, []);

  const columns = [
    {
      title: t("contracts.col_number", language),
      dataIndex: "contract_number",
      key: "contract_number",
      sorter: true,
      width: 160,
      render: (text: string) => (
        <span style={{ fontFamily: "monospace", fontWeight: 500, fontSize: 13 }}>
          {text}
        </span>
      ),
    },
    {
      title: t("contracts.col_name", language),
      dataIndex: "contract_name",
      key: "contract_name",
      ellipsis: true,
      render: (text: string) => (
        <span style={{ fontWeight: 500, color: "#000" }}>{text}</span>
      ),
    },
    {
      title: t("contracts.col_currency", language),
      dataIndex: "currency",
      key: "currency",
      width: 80,
      render: (text: string) => (
        <span style={{ fontWeight: 500, fontSize: 13, color: "#595959" }}>
          {text}
        </span>
      ),
    },
    {
      title: t("contracts.col_start_date", language),
      dataIndex: "commencement_date",
      key: "commencement_date",
      sorter: true,
      width: 130,
      render: (text: string) => (
        <span style={{ fontSize: 13, color: "#595959", fontFamily: "monospace" }}>
          {text}
        </span>
      ),
    },
    {
      title: t("contracts.col_end_date", language),
      dataIndex: "lease_end_date",
      key: "lease_end_date",
      sorter: true,
      width: 130,
      render: (text: string) => (
        <span style={{ fontSize: 13, color: "#595959", fontFamily: "monospace" }}>
          {text}
        </span>
      ),
    },
    {
      title: t("contracts.col_status", language),
      dataIndex: "approval_status",
      key: "approval_status",
      sorter: true,
      width: 160,
      render: (status: string, record: Contract) => {
        return (
          <Space size={4}>
            <Tag
              color={STATUS_COLORS[status] || "default"}
              style={{ fontWeight: 500, margin: 0 }}
            >
              {STATUS_LABELS[status] || status}
            </Tag>
            {record.is_official_version && (
              <Badge
                count={t("contracts.official", language)}
                style={{
                  background: "#000",
                  color: "#fff",
                  fontSize: 10,
                  fontWeight: 600,
                  padding: "0 6px",
                  height: 18,
                  lineHeight: "18px",
                  borderRadius: 4,
                }}
              />
            )}
            {!record.is_official_version && status !== "draft" && (
              <span
                style={{
                  fontSize: 11,
                  color: "#BFBFBF",
                  fontWeight: 500,
                }}
              >
                {t("contracts.working", language)}
              </span>
            )}
          </Space>
        );
      },
    },

    {
      title: "范围",
      key: "lease_scope",
      width: 110,
      render: (_: any, record: Contract) => (
        <Tag color={LEASE_SCOPE_COLORS[record.lease_scope || "in_scope"]} style={{ margin: 0 }}>
          {LEASE_SCOPE_LABELS[record.lease_scope || "in_scope"]}
        </Tag>
      ),
    },
    {
      title: "资产",
      key: "asset_type",
      width: 100,
      render: (_: any, record: Contract) => ASSET_TYPE_LABELS[record.asset_type || "real_estate"],
    },
    {
      title: "",
      key: "action",
      width: 80,
      align: "right" as const,
      render: (_: any, record: Contract) => (
        <Button
          type="text"
          size="small"
          icon={<ArrowRightOutlined style={{ fontSize: 12 }} />}
          onClick={() => router.push(`/contracts/${record.id}`)}
          style={{
            color: "#BFBFBF",
            borderRadius: 6,
            width: 28,
            height: 28,
            padding: 0,
            display: "inline-flex",
            alignItems: "center",
            justifyContent: "center",
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.color = "#000";
            e.currentTarget.style.background = "#F5F5F5";
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.color = "#BFBFBF";
            e.currentTarget.style.background = "transparent";
          }}
        />
      ),
    },
  ];

  const handleTableChange = (_pagination: any, _filters: any, sorter: any) => {
    if (sorter.field) {
      setSortBy(sorter.field);
      setSortOrder(sorter.order === "ascend" ? "asc" : "desc");
    }
  };

  return (
    <ProtectedRoute>
      <AppLayout>
        {/* Page Header */}
        <motion.div
          initial={{ opacity: 0, y: 4 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25 }}
        >
          <div
            style={{
              display: "flex",
              justifyContent: "space-between",
              alignItems: "flex-start",
              marginBottom: 32,
            }}
          >
            <div>
              <h1 style={{ marginBottom: 4, fontSize: 28, letterSpacing: "-0.04em" }}>
                {t("contracts.title", language)}
              </h1>
              <p style={{ color: "#8C8C8C", fontSize: 14, margin: 0 }}>
                {t("contracts.subtitle", language, { count: String(contracts.length) })}
              </p>
            </div>
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => router.push("/contracts/new")}
              style={{ borderRadius: 9999, fontWeight: 500 }}
            >
              {t("contracts.add_contract", language)}
            </Button>
          </div>
        </motion.div>

        {/* Filter Bar */}
        <motion.div
          initial={{ opacity: 0, y: 4 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25, delay: 0.05 }}
        >
          <Card
            bodyStyle={{ padding: "16px 20px" }}
            style={{ borderRadius: 10, marginBottom: 16 }}
          >
            <div
              style={{
                display: "flex",
                gap: 12,
                alignItems: "center",
                flexWrap: "wrap",
              }}
            >
              <div style={{ position: "relative", flex: 1, minWidth: 280, maxWidth: 400 }}>
                <SearchOutlined
                  style={{
                    position: "absolute",
                    left: 12,
                    top: "50%",
                    transform: "translateY(-50%)",
                    color: "#8C8C8C",
                    fontSize: 14,
                    zIndex: 1,
                  }}
                />
                <Input
                  placeholder={t("contracts.search_placeholder", language)}
                  value={search}
                  onChange={(e) => handleSearchChange(e.target.value)}
                  allowClear
                  style={{
                    paddingLeft: 36,
                    borderRadius: 9999,
                    height: 36,
                    fontSize: 13,
                  }}
                />
              </div>

              <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                <FilterOutlined style={{ color: "#8C8C8C", fontSize: 14 }} />
                <Select
                  value={statusFilter}
                  onChange={(value) => setStatusFilter(value)}
                  options={STATUS_OPTIONS}
                  style={{ width: 140 }}
                  size="middle"
                  placeholder={t("contracts.filter_status", language)}
                />
              </div>

              <div style={{ display: "flex", alignItems: "center", gap: 6, marginLeft: "auto" }}>
                <Button
                  type={sortOrder === "desc" ? "primary" : "default"}
                  size="small"
                  icon={<SortDescendingOutlined />}
                  onClick={() => setSortOrder("desc")}
                  style={{ borderRadius: 6 }}
                />
                <Button
                  type={sortOrder === "asc" ? "primary" : "default"}
                  size="small"
                  icon={<SortAscendingOutlined />}
                  onClick={() => setSortOrder("asc")}
                  style={{ borderRadius: 6 }}
                />
              </div>
            </div>
          </Card>
        </motion.div>

        {/* Table */}
        <motion.div
          initial={{ opacity: 0, y: 4 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25, delay: 0.1 }}
        >
          <Card
            bodyStyle={{ padding: 0 }}
            style={{ borderRadius: 10, overflow: "hidden" }}
          >
            <Table
              columns={columns}
              dataSource={contracts}
              rowKey="id"
              loading={{
                spinning: loading,
                indicator: <Skeleton active paragraph={{ rows: 5 }} />,
              }}
              pagination={{
                pageSize: 10,
                showSizeChanger: true,
                showTotal: (total) => {
                  const text = t("contracts.total_items", language, { total: "__TOTAL__" });
                  const [before, after] = text.split("__TOTAL__");
                  return (
                    <span style={{ fontSize: 13, color: "#8C8C8C" }}>
                      {before}
                      <strong style={{ color: "#000" }}>{total}</strong>
                      {after}
                    </span>
                  );
                },
              }}
              onChange={handleTableChange}
              locale={{
                emptyText: (
                  <Empty
                    image={Empty.PRESENTED_IMAGE_SIMPLE}
                    description={
                      <span style={{ color: "#8C8C8C" }}>
                        {search || statusFilter
                          ? t("contracts.no_search_results", language)
                          : t("contracts.no_data", language)}
                      </span>
                    }
                  />
                ),
              }}
              rowClassName={() => "contract-row"}
            />
          </Card>
        </motion.div>
      </AppLayout>
    </ProtectedRoute>
  );
}
