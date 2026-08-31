"use client";

import { StatusTag, statusKindFromAntColor } from "../components/StatusTag";

import { useState, useEffect } from "react";
import {
  Card,
  Table,
  Select,
  Input,
  DatePicker,
  Button,
  message,
  Row,
  Col,
  Modal,
} from "antd";
import { SearchOutlined, ReloadOutlined, ExpandAltOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { auditApi } from "../lib/api";
import { useRetailQuery } from "../retail/useRetailQuery";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { notifyError } from "../lib/notify";
import { tableScrollX } from "../lib/tableScroll";

const { RangePicker } = DatePicker;

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

export default function AuditLogsPage() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const [pagination, setPagination] = useState({ current: 1, pageSize: 50 });

  // Filters
  const [tableName, setTableName] = useState("");
  const [action, setAction] = useState("");
  const [recordId, setRecordId] = useState("");
  const [runId, setRunId] = useState("");
  const [toolName, setToolName] = useState("");
  const [dateRange, setDateRange] = useState<[dayjs.Dayjs | null, dayjs.Dayjs | null] | null>(null);

  // i18n option arrays
  const TABLE_NAME_OPTIONS = [
    { value: "", label: t("audit.table_all", language) },
    { value: "lease_contracts", label: t("audit.table_contract", language) },
    { value: "lease_events", label: t("audit.table_event", language) },
    { value: "lease_payment_schedules", label: t("audit.table_schedule", language) },
    { value: "monthly_closing_batches", label: t("audit.table_batch", language) },
    { value: "journal_entries", label: t("audit.table_entry", language) },
    { value: "period_locks", label: t("audit.table_lock", language) },
  ];

  const ACTION_OPTIONS = [
    { value: "", label: t("audit.action_all", language) },
    { value: "create", label: t("audit.action_create", language) },
    { value: "update", label: t("audit.action_update", language) },
    { value: "delete", label: t("audit.action_delete", language) },
    { value: "submit", label: t("audit.action_submit", language) },
    { value: "review_approved", label: t("audit.action_review_pass", language) },
    { value: "review_returned", label: t("audit.action_review_reject", language) },
    { value: "approve", label: t("audit.action_approve", language) },
    { value: "reject", label: t("audit.action_reject", language) },
    { value: "generate", label: t("audit.action_generate", language) },
    { value: "post", label: t("audit.action_post", language) },
    { value: "lock", label: t("audit.action_lock", language) },
    { value: "unlock", label: t("audit.action_unlock", language) },
    { value: "recalculate", label: t("audit.action_recalculate", language) },
  ];

  const TABLE_NAME_LABELS: Record<string, string> = {
    lease_contracts: t("audit.table_contract", language),
    lease_events: t("audit.table_event", language),
    lease_payment_schedules: t("audit.table_schedule", language),
    monthly_closing_batches: t("audit.table_batch", language),
    journal_entries: t("audit.table_entry", language),
    period_locks: t("audit.table_lock", language),
  };

  const ACTION_LABELS: Record<string, string> = {
    create: t("audit.action_create", language),
    update: t("audit.action_update", language),
    delete: t("audit.action_delete", language),
    submit: t("audit.action_submit", language),
    review_approved: t("audit.action_review_pass", language),
    review_returned: t("audit.action_review_reject", language),
    approve: t("audit.action_approve", language),
    reject: t("audit.action_reject", language),
    generate: t("audit.action_generate", language),
    post: t("audit.action_post", language),
    lock: t("audit.action_lock", language),
    unlock: t("audit.action_unlock", language),
    recalculate: t("audit.action_recalculate", language),
  };

  // FETCH-003: the audit log is a filter + pagination query — it runs
  // through the shared fetch seam (race gate / token injection / STATE-001
  // exit) instead of a hand-rolled effect. Search/reset change the filter
  // state, which changes paramsKey and refetches.
  const listParams = {
    tableName,
    action,
    recordId,
    runId,
    toolName,
    startDate: dateRange?.[0] ? dateRange[0].format("YYYY-MM-DD") : "",
    endDate: dateRange?.[1] ? dateRange[1].format("YYYY-MM-DD") : "",
    page: pagination.current,
    pageSize: pagination.pageSize,
  };
  const listParamsKey = JSON.stringify(listParams);
  const { loading, state: logsState, retry } = useRetailQuery({
    token,
    params: listParams,
    paramsKey: listParamsKey,
    fetcher: (p, t) => {
      const params: any = { limit: p.pageSize, offset: (p.page - 1) * p.pageSize };
      if (p.tableName) params.table_name = p.tableName;
      if (p.action) params.action = p.action;
      if (p.recordId) params.record_id = p.recordId;
      if (p.runId) params.run_id = p.runId;
      if (p.toolName) params.tool_name = p.toolName;
      if (p.startDate) params.start_date = p.startDate;
      if (p.endDate) params.end_date = p.endDate;
      return auditApi.list<{ data?: unknown[]; total?: number }>(params, t);
    },
  });
  const logs = logsState.kind === "ready" ? logsState.data?.data ?? [] : [];
  const total = logsState.kind === "ready" ? logsState.data?.total ?? 0 : 0;
  useEffect(() => {
    if (logsState.kind === "failed") notifyError(t("audit.query_failed", language) + ": " + (logsState.message || ""));
  }, [logsState, language]);

  const handleSearch = () => {
    setPagination({ ...pagination, current: 1 });
  };

  const handleReset = () => {
    setTableName("");
    setAction("");
    setRecordId("");
    setRunId("");
    setToolName("");
    setDateRange(null);
  };

  const handleTableChange = (pag: any) => {
    // FETCH-003: changing pagination updates the params, the seam refetches.
    setPagination(pag);
  };

  const formatJsonContent = (jsonStr: string | null | undefined) => {
    if (!jsonStr || jsonStr === "null" || jsonStr === "{}") return t("audit.none", language);
    try {
      const parsed = JSON.parse(jsonStr);
      return JSON.stringify(parsed, null, 2);
    } catch {
      return jsonStr;
    }
  };

  const columns = [
    {
      title: t("audit.col_time", language),
      dataIndex: "changed_at",
      key: "changed_at",
      width: 170,
      render: (val: string) => dayjs(val).format("YYYY-MM-DD HH:mm:ss"),
    },
    {
      title: t("audit.col_operator", language),
      dataIndex: "changed_by_name",
      key: "changed_by_name",
      width: 120,
      render: (val: string, record: any) => val || (record.changed_by ? record.changed_by.substring(0, 8) : "—"),
    },
    {
      title: t("audit.col_action", language),
      dataIndex: "action",
      key: "action",
      width: 110,
      render: (val: string) => (
        <StatusTag kind={statusKindFromAntColor(ACTION_COLORS[val] || "default")}>
          {ACTION_LABELS[val] || val}
        </StatusTag>
      ),
    },
    {
      title: t("audit.col_table", language),
      dataIndex: "table_name",
      key: "table_name",
      width: 110,
      render: (val: string) => TABLE_NAME_LABELS[val] || val,
    },
    {
      title: t("audit.col_record_id", language),
      dataIndex: "record_id",
      key: "record_id",
      width: 130,
      render: (val: string) => (
        <span className="audit-record-id">
          {val ? val.substring(0, 8) + "..." : "—"}
        </span>
      ),
    },
    {
      title: t("audit.col_old_value", language),
      dataIndex: "old_values",
      key: "old_values",
      width: 60,
      render: (val: string) => {
        if (!val || val === "null" || val === "{}") return <span className="audit-empty-value">{t("audit.none", language)}</span>;
        const text = formatJsonContent(val);
        return (
          <Button
            type="link"
            size="small"
            icon={<ExpandAltOutlined />}
            onClick={() => {
              Modal.info({
                title: t("audit.view_old", language),
                width: 600,
                content: <pre className="audit-json-preview">{text}</pre>,
              });
            }}
          >
            {t("audit.view", language)}
          </Button>
        );
      },
    },
    {
      title: t("audit.col_new_value", language),
      dataIndex: "new_values",
      key: "new_values",
      width: 60,
      render: (val: string) => {
        if (!val || val === "null" || val === "{}") return <span className="audit-empty-value">{t("audit.none", language)}</span>;
        const text = formatJsonContent(val);
        return (
          <Button
            type="link"
            size="small"
            icon={<ExpandAltOutlined />}
            onClick={() => {
              Modal.info({
                title: t("audit.view_new", language),
                width: 600,
                content: <pre className="audit-json-preview">{text}</pre>,
              });
            }}
          >
            {t("audit.view", language)}
          </Button>
        );
      },
    },
  ];

  return (
    <ProtectedRoute>
      <AppLayout>
        <div className="audit-page">
          <PageHeader title={t("audit.title", language)} />

          {/* Filters */}
          <Card size="small" className="audit-filter-card">
            <Row gutter={[12, 12]}>
              <Col xs={24} sm={6}>
                <span className="audit-filter-label">{t("audit.filter_table", language)}</span>
                <Select
                  size="small"
                  value={tableName}
                  onChange={setTableName}
                  options={TABLE_NAME_OPTIONS}
                  className="audit-filter-control"
                  allowClear
                />
              </Col>
              <Col xs={24} sm={6}>
                <span className="audit-filter-label">{t("audit.filter_action", language)}</span>
                <Select
                  size="small"
                  value={action}
                  onChange={setAction}
                  options={ACTION_OPTIONS}
                  className="audit-filter-control"
                  allowClear
                />
              </Col>
              <Col xs={24} sm={6}>
                <span className="audit-filter-label">{t("audit.filter_record_id", language)}</span>
                <Input
                  size="small"
                  value={recordId}
                  onChange={(e) => setRecordId(e.target.value)}
                  placeholder={t("audit.filter_record_placeholder", language)}
                  allowClear
                />
              </Col>
              <Col xs={24} sm={6}>
                <span className="audit-filter-label">{t("audit.filter_run_id", language)}</span>
                <Input
                  size="small"
                  value={runId}
                  onChange={(e) => setRunId(e.target.value)}
                  placeholder={t("audit.filter_run_placeholder", language)}
                  allowClear
                />
              </Col>
              <Col xs={24} sm={6}>
                <span className="audit-filter-label">{t("audit.filter_tool_name", language)}</span>
                <Input
                  size="small"
                  value={toolName}
                  onChange={(e) => setToolName(e.target.value)}
                  placeholder={t("audit.filter_tool_placeholder", language)}
                  allowClear
                />
              </Col>
              <Col xs={24} sm={6}>
                <span className="audit-filter-label">{t("audit.filter_time_range", language)}</span>
                <RangePicker
                  size="small"
                  value={dateRange as any}
                  onChange={(dates) => setDateRange(dates as any)}
                  className="audit-filter-control"
                />
              </Col>
            </Row>
            <div className="audit-filter-actions">
              <Button size="small" type="primary" icon={<SearchOutlined />} onClick={handleSearch}>
                {t("audit.query", language)}
              </Button>
              <Button size="small" icon={<ReloadOutlined />} onClick={handleReset}>
                {t("audit.reset", language)}
              </Button>
            </div>
          </Card>

          {/* Table */}
          <Card title={t("audit.entries", language)}>
            <div className="card-context audit-result-count">{t("audit.subtitle", language, { total: String(total) })}</div>
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
                showTotal: (paginationTotal) => t("audit.total_records", language, { total: String(paginationTotal) }),
                pageSizeOptions: ["20", "50", "100"],
              }}
              onChange={handleTableChange}
              scroll={tableScrollX((logs || []).length, 900)}
              size="small"
            />
          </Card>
        </div>
      </AppLayout>
    </ProtectedRoute>
  );
}
