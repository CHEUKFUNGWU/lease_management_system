"use client";

import { useState, useCallback, useEffect } from "react";
import {
  Card,
  Table,
  Select,
  Input,
  DatePicker,
  Button,
  Tag,
  message,
  Row,
  Col,
  Modal,
} from "antd";
import { SearchOutlined, ReloadOutlined, ExpandAltOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { auditApi } from "../lib/api";
import { useAuth } from "../context/AuthContext";

const { RangePicker } = DatePicker;

const TABLE_NAME_OPTIONS = [
  { value: "", label: "全部表" },
  { value: "lease_contracts", label: "合同" },
  { value: "lease_events", label: "事件" },
  { value: "lease_payment_schedules", label: "付款计划" },
  { value: "monthly_closing_batches", label: "月结批次" },
  { value: "journal_entries", label: "会计分录" },
  { value: "period_locks", label: "期间锁账" },
];

const ACTION_OPTIONS = [
  { value: "", label: "全部操作" },
  { value: "create", label: "创建" },
  { value: "update", label: "更新" },
  { value: "delete", label: "删除" },
  { value: "submit", label: "提交" },
  { value: "review_approved", label: "复核通过" },
  { value: "review_returned", label: "复核退回" },
  { value: "approve", label: "审批通过" },
  { value: "reject", label: "驳回" },
  { value: "generate", label: "生成" },
  { value: "post", label: "过账" },
  { value: "lock", label: "锁账" },
  { value: "unlock", label: "解锁" },
  { value: "recalculate", label: "重算" },
];

const TABLE_NAME_LABELS: Record<string, string> = {
  lease_contracts: "合同",
  lease_events: "事件",
  lease_payment_schedules: "付款计划",
  monthly_closing_batches: "月结批次",
  journal_entries: "会计分录",
  period_locks: "期间锁账",
};

const ACTION_COLORS: Record<string, string> = {
  create: "green",
  update: "blue",
  delete: "red",
  submit: "purple",
  review_approved: "cyan",
  review_returned: "orange",
  approve: "success",
  reject: "error",
  generate: "geekblue",
  post: "lime",
  lock: "volcano",
  unlock: "gold",
  recalculate: "magenta",
};

const ACTION_LABELS: Record<string, string> = {
  create: "创建",
  update: "更新",
  delete: "删除",
  submit: "提交",
  review_approved: "复核通过",
  review_returned: "复核退回",
  approve: "审批通过",
  reject: "驳回",
  generate: "生成",
  post: "过账",
  lock: "锁账",
  unlock: "解锁",
  recalculate: "重算",
};

export default function AuditLogsPage() {
  const { token } = useAuth();
  const [loading, setLoading] = useState(false);
  const [logs, setLogs] = useState<any[]>([]);
  const [total, setTotal] = useState(0);
  const [pagination, setPagination] = useState({ current: 1, pageSize: 50 });

  // Filters
  const [tableName, setTableName] = useState("");
  const [action, setAction] = useState("");
  const [recordId, setRecordId] = useState("");
  const [dateRange, setDateRange] = useState<[dayjs.Dayjs | null, dayjs.Dayjs | null] | null>(null);

  const fetchLogs = useCallback(async (page = 1, pageSize = 50) => {
    if (!token) return;
    setLoading(true);
    try {
      const params: any = {
        limit: pageSize,
        offset: (page - 1) * pageSize,
      };
      if (tableName) params.table_name = tableName;
      if (action) params.action = action;
      if (recordId) params.record_id = recordId;
      if (dateRange && dateRange[0]) params.start_date = dateRange[0].format("YYYY-MM-DD");
      if (dateRange && dateRange[1]) params.end_date = dateRange[1].format("YYYY-MM-DD");

      const data = await auditApi.list(params, token);
      setLogs(data.data || []);
      setTotal(data.total || 0);
    } catch (err: any) {
      message.error("查询审计日志失败: " + (err.message || "未知错误"));
    } finally {
      setLoading(false);
    }
  }, [token, tableName, action, recordId, dateRange]);

  useEffect(() => {
    fetchLogs(1, pagination.pageSize);
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const handleSearch = () => {
    fetchLogs(1, pagination.pageSize);
    setPagination({ ...pagination, current: 1 });
  };

  const handleReset = () => {
    setTableName("");
    setAction("");
    setRecordId("");
    setDateRange(null);
  };

  const handleTableChange = (pag: any) => {
    setPagination(pag);
    fetchLogs(pag.current, pag.pageSize);
  };

  const formatJsonContent = (jsonStr: string | null | undefined) => {
    if (!jsonStr || jsonStr === "null" || jsonStr === "{}") return "无";
    try {
      const parsed = JSON.parse(jsonStr);
      return JSON.stringify(parsed, null, 2);
    } catch {
      return jsonStr;
    }
  };

  const columns = [
    {
      title: "时间",
      dataIndex: "changed_at",
      key: "changed_at",
      width: 170,
      render: (val: string) => dayjs(val).format("YYYY-MM-DD HH:mm:ss"),
    },
    {
      title: "操作人",
      dataIndex: "changed_by_name",
      key: "changed_by_name",
      width: 120,
      render: (val: string, record: any) => val || (record.changed_by ? record.changed_by.substring(0, 8) : "-"),
    },
    {
      title: "操作类型",
      dataIndex: "action",
      key: "action",
      width: 110,
      render: (val: string) => (
        <Tag color={ACTION_COLORS[val] || "default"}>
          {ACTION_LABELS[val] || val}
        </Tag>
      ),
    },
    {
      title: "表名",
      dataIndex: "table_name",
      key: "table_name",
      width: 110,
      render: (val: string) => TABLE_NAME_LABELS[val] || val,
    },
    {
      title: "记录ID",
      dataIndex: "record_id",
      key: "record_id",
      width: 130,
      render: (val: string) => (
        <span style={{ fontFamily: "monospace", fontSize: 12 }}>
          {val ? val.substring(0, 8) + "..." : "-"}
        </span>
      ),
    },
    {
      title: "旧值",
      dataIndex: "old_values",
      key: "old_values",
      width: 60,
      render: (val: string) => {
        const text = formatJsonContent(val);
        if (text === "无") return <span style={{ color: "#999" }}>无</span>;
        return (
          <Button
            type="link"
            size="small"
            icon={<ExpandAltOutlined />}
            onClick={() => {
              Modal.info({
                title: "变更前数据",
                width: 600,
                content: <pre style={{ maxHeight: 400, overflow: "auto", fontSize: 12 }}>{text}</pre>,
              });
            }}
          >
            查看
          </Button>
        );
      },
    },
    {
      title: "新值",
      dataIndex: "new_values",
      key: "new_values",
      width: 60,
      render: (val: string) => {
        const text = formatJsonContent(val);
        if (text === "无") return <span style={{ color: "#999" }}>无</span>;
        return (
          <Button
            type="link"
            size="small"
            icon={<ExpandAltOutlined />}
            onClick={() => {
              Modal.info({
                title: "变更后数据",
                width: 600,
                content: <pre style={{ maxHeight: 400, overflow: "auto", fontSize: 12 }}>{text}</pre>,
              });
            }}
          >
            查看
          </Button>
        );
      },
    },
  ];

  return (
    <ProtectedRoute>
      <AppLayout>
        <div style={{ padding: 0 }}>
          <h2 style={{ marginBottom: 16 }}>审计日志</h2>

          {/* Filters */}
          <Card size="small" style={{ marginBottom: 16 }}>
            <Row gutter={[12, 12]}>
              <Col xs={24} sm={6}>
                <span style={{ fontSize: 13, marginBottom: 4, display: "block" }}>表名</span>
                <Select
                  value={tableName}
                  onChange={setTableName}
                  options={TABLE_NAME_OPTIONS}
                  style={{ width: "100%" }}
                  allowClear
                />
              </Col>
              <Col xs={24} sm={6}>
                <span style={{ fontSize: 13, marginBottom: 4, display: "block" }}>操作类型</span>
                <Select
                  value={action}
                  onChange={setAction}
                  options={ACTION_OPTIONS}
                  style={{ width: "100%" }}
                  allowClear
                />
              </Col>
              <Col xs={24} sm={6}>
                <span style={{ fontSize: 13, marginBottom: 4, display: "block" }}>记录ID</span>
                <Input
                  value={recordId}
                  onChange={(e) => setRecordId(e.target.value)}
                  placeholder="UUID 搜索"
                  allowClear
                />
              </Col>
              <Col xs={24} sm={6}>
                <span style={{ fontSize: 13, marginBottom: 4, display: "block" }}>时间范围</span>
                <RangePicker
                  value={dateRange as any}
                  onChange={(dates) => setDateRange(dates as any)}
                  style={{ width: "100%" }}
                />
              </Col>
            </Row>
            <div style={{ marginTop: 12, display: "flex", gap: 8 }}>
              <Button type="primary" icon={<SearchOutlined />} onClick={handleSearch}>
                查询
              </Button>
              <Button icon={<ReloadOutlined />} onClick={handleReset}>
                重置
              </Button>
            </div>
          </Card>

          {/* Table */}
          <Card>
            <Table
              dataSource={logs}
              columns={columns}
              rowKey="id"
              loading={loading}
              pagination={{
                current: pagination.current,
                pageSize: pagination.pageSize,
                total: total,
                showSizeChanger: true,
                showTotal: (t) => `共 ${t} 条记录`,
                pageSizeOptions: ["20", "50", "100"],
              }}
              onChange={handleTableChange}
              scroll={{ x: 900 }}
              size="small"
            />
          </Card>
        </div>
      </AppLayout>
    </ProtectedRoute>
  );
}
