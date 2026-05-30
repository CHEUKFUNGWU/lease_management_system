"use client";

import { useState, useCallback } from "react";
import { motion } from "framer-motion";
import {
  Card,
  Button,
  Form,
  Input,
  DatePicker,
  message,
  Table,
  Tag,
  Spin,
  Row,
  Col,
  Tabs,
  Alert,
  Space,
  Modal,
  Popconfirm,
  Descriptions,
  Empty,
  Skeleton,
} from "antd";
import {
  CalculatorOutlined,
  HistoryOutlined,
  LockOutlined,
  UnlockOutlined,
  CheckOutlined,
  SendOutlined,
  RollbackOutlined,
} from "@ant-design/icons";
import dayjs from "dayjs";
import { useRouter } from "next/navigation";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { monthlyClosingApi } from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";

// ─── Page Header ───────────────────────────────────────────────

function PageHeader({ selectedPeriod, entries }: { selectedPeriod: string; entries: any[] }) {
  const { language } = useLanguage();
  const draftCount = entries.filter((e: any) => e.posting_status === "draft").length;
  const approvedCount = entries.filter((e: any) => e.posting_status === "approved").length;
  const postedCount = entries.filter((e: any) => e.posting_status === "posted").length;

  return (
    <div
      style={{
        display: "flex",
        justifyContent: "space-between",
        alignItems: "flex-start",
        marginBottom: 32,
      }}
    >
      <div>
        <h1 style={{ marginBottom: 4, fontSize: 28, letterSpacing: "-0.04em", fontWeight: 700 }}>
          {t("monthly.title", language)}
        </h1>
        <p style={{ color: "var(--fg-muted)", fontSize: 14, margin: 0 }}>
          {t("monthly.subtitle", language)}
        </p>
        {selectedPeriod && (
          <Space size={12} style={{ marginTop: 8 }}>
            <span style={{ fontSize: 13, color: "var(--fg-tertiary)" }}>
              {t("monthly.current_period", language)}：<strong style={{ color: "var(--fg-primary)" }}>{selectedPeriod}</strong>
            </span>
            <span style={{ color: "var(--border-strong)" }}>·</span>
            <span style={{ fontSize: 13, color: "var(--fg-muted)" }}>
              {t("monthly.status_summary", language, {
                draftCount: String(draftCount),
                approvedCount: String(approvedCount),
                postedCount: String(postedCount),
              })}
            </span>
          </Space>
        )}
      </div>
    </div>
  );
}

// ─── Main Component ────────────────────────────────────────────

export default function MonthlyClosingPage() {
  const { language } = useLanguage();
  const { token, user } = useAuth();
  const router = useRouter();
  const [loading, setLoading] = useState(false);
  const [entriesLoading, setEntriesLoading] = useState(false);
  const [batchesLoading, setBatchesLoading] = useState(false);
  const [entriesLoaded, setEntriesLoaded] = useState(false);
  const [batchesLoaded, setBatchesLoaded] = useState(false);
  const [actionLoading, setActionLoading] = useState<Record<string, boolean>>({});
  const [result, setResult] = useState<any>(null);
  const [entries, setEntries] = useState<any[]>([]);
  const [batches, setBatches] = useState<any[]>([]);
  const [activeTab, setActiveTab] = useState("generate");
  const [selectedPeriod, setSelectedPeriod] = useState<string>("");
  const [isLocked, setIsLocked] = useState(false);
  const [lockLoading, setLockLoading] = useState(false);
  const [lockStatusLoading, setLockStatusLoading] = useState(false);
  const [postModalOpen, setPostModalOpen] = useState(false);
  const [postingEntry, setPostingEntry] = useState<any>(null);
  const [erpReference, setErpReference] = useState("");

  const role = user?.role || "";
  const isAdmin = role === "admin";
  const isApprover = role === "approver" || isAdmin;
  const isReviewer = role === "reviewer" || isApprover;
  const canManage = isReviewer;

  const checkLockStatus = useCallback(
    async (period: string) => {
      if (!token || !period) return;
      setLockStatusLoading(true);
      try {
        const data = await monthlyClosingApi.getLockStatus(period, token);
        setIsLocked(data.is_locked);
      } catch {
        setIsLocked(false);
      } finally {
        setLockStatusLoading(false);
      }
    },
    [token]
  );

  const handleGenerate = async (values: any) => {
    if (!token) return;
    const period = values.period?.format("YYYY-MM");
    if (!period) {
      message.error(t("monthly.select_period", language));
      return;
    }
    setLoading(true);
    try {
      const data = await monthlyClosingApi.generate(
        {
          accounting_period: period,
          discount_rate: values.discount_rate || 0.05,
        },
        token
      );
      setResult(data);
      setSelectedPeriod(period);
      message.success(t("monthly.generate_success", language, { count: String(data.processed_contracts) }));
      await checkLockStatus(period);
      loadBatches(period);
      loadEntries(period);
      setActiveTab("entries");
    } catch (error: any) {
      message.error(error.message || t("monthly.generate_failed", language));
    } finally {
      setLoading(false);
    }
  };

  const loadEntries = async (period: string, status?: string) => {
    if (!token) return;
    setEntriesLoading(true);
    try {
      const data = await monthlyClosingApi.getEntries(
        { period, status: status || undefined },
        token
      );
      setEntries(data.data || []);
      setEntriesLoaded(true);
    } catch (error: any) {
      message.error(error.message || t("monthly.load_entries_failed", language));
    } finally {
      setEntriesLoading(false);
    }
  };

  const loadBatches = async (period: string) => {
    if (!token) return;
    setBatchesLoading(true);
    try {
      const data = await monthlyClosingApi.listBatches(period, token);
      setBatches(data.data || []);
      setBatchesLoaded(true);
    } catch (error: any) {
      console.error(error);
    } finally {
      setBatchesLoading(false);
    }
  };

  const refresh = (period?: string) => {
    const p = period || selectedPeriod;
    if (p) {
      loadEntries(p);
      loadBatches(p);
      checkLockStatus(p);
    }
  };

  const handleApproveEntry = async (entryId: string) => {
    if (!token) return;
    setActionLoading((prev) => ({ ...prev, [entryId]: true }));
    try {
      await monthlyClosingApi.approveEntry(entryId, token);
      message.success(t("monthly.approve_success", language));
      refresh();
    } catch (error: any) {
      message.error(error.message || t("monthly.approve_failed", language));
    } finally {
      setActionLoading((prev) => ({ ...prev, [entryId]: false }));
    }
  };

  const handlePostEntry = async () => {
    if (!token || !postingEntry) return;
    setActionLoading((prev) => ({ ...prev, [postingEntry.id]: true }));
    try {
      await monthlyClosingApi.postEntry(postingEntry.id, erpReference, token);
      message.success(t("monthly.post_success", language));
      setPostModalOpen(false);
      setPostingEntry(null);
      setErpReference("");
      refresh();
    } catch (error: any) {
      message.error(error.message || t("monthly.post_failed", language));
    } finally {
      setActionLoading((prev) => ({ ...prev, [postingEntry.id]: false }));
    }
  };

  const handleApproveBatch = async (batchId: string) => {
    if (!token) return;
    setActionLoading((prev) => ({ ...prev, [`batch_${batchId}`]: true }));
    try {
      const data = await monthlyClosingApi.approveBatch(batchId, token);
      message.success(t("monthly.batch_approve_success", language, { count: String(data.approved_count) }));
      refresh();
    } catch (error: any) {
      message.error(error.message || t("monthly.batch_approve_failed", language));
    } finally {
      setActionLoading((prev) => ({ ...prev, [`batch_${batchId}`]: false }));
    }
  };

  const handlePostBatch = async (batchId: string) => {
    if (!token) return;
    setActionLoading((prev) => ({ ...prev, [`postbatch_${batchId}`]: true }));
    try {
      const data = await monthlyClosingApi.postBatch(batchId, token);
      message.success(t("monthly.batch_post_success", language, { count: String(data.posted_count) }));
      refresh();
    } catch (error: any) {
      message.error(error.message || t("monthly.batch_post_failed", language));
    } finally {
      setActionLoading((prev) => ({ ...prev, [`postbatch_${batchId}`]: false }));
    }
  };

  const handleLockPeriod = async () => {
    if (!token || !selectedPeriod) {
      message.error(t("monthly.no_period", language));
      return;
    }
    setLockLoading(true);
    try {
      await monthlyClosingApi.lockPeriod(selectedPeriod, token);
      message.success(t("monthly.lock_success", language, { period: selectedPeriod }));
      await checkLockStatus(selectedPeriod);
    } catch (error: any) {
      message.error(error.message || t("monthly.lock_failed", language));
    } finally {
      setLockLoading(false);
    }
  };

  const handleUnlockPeriod = async () => {
    if (!token || !selectedPeriod) {
      message.error(t("monthly.no_period", language));
      return;
    }
    setLockLoading(true);
    try {
      await monthlyClosingApi.unlockPeriod(selectedPeriod, token);
      message.success(t("monthly.unlock_success", language, { period: selectedPeriod }));
      await checkLockStatus(selectedPeriod);
    } catch (error: any) {
      message.error(error.message || t("monthly.unlock_failed", language));
    } finally {
      setLockLoading(false);
    }
  };

  const openPostModal = (entry: any) => {
    setPostingEntry(entry);
    setErpReference("");
    setPostModalOpen(true);
  };

  // ─── entryColumns ────────────────────────────────────────────

  const entryColumns = [
    { title: t("monthly.col_period", language), dataIndex: "accounting_period", width: 90 },
    {
      title: t("monthly.col_entry_type", language),
      dataIndex: "entry_type",
      width: 100,
      render: (v: string) => {
        const labels: Record<string, string> = {
          interest: t("monthly.entry_interest", language),
          depreciation: t("monthly.entry_depreciation", language),
          payment: t("monthly.entry_payment", language),
        };
        return <Tag color="processing">{labels[v] || v}</Tag>;
      },
    },
    { title: t("monthly.col_debit_account", language), dataIndex: "debit_account", width: 150 },
    { title: t("monthly.col_credit_account", language), dataIndex: "credit_account", width: 150 },
    {
      title: t("monthly.col_amount", language),
      dataIndex: "amount",
      width: 120,
      align: "right" as const,
      render: (v: number) =>
        `¥${v.toLocaleString(undefined, { minimumFractionDigits: 2 })}`,
    },
    { title: t("monthly.col_currency", language), dataIndex: "currency", width: 60 },
    { title: t("monthly.col_description", language), dataIndex: "description", ellipsis: true },
    {
      title: t("monthly.col_status", language),
      dataIndex: "posting_status",
      width: 80,
      render: (v: string) => {
        const statusLabels: Record<string, string> = {
          posted: t("monthly.status_posted", language),
          approved: t("monthly.status_approved", language),
          draft: t("monthly.status_draft", language),
        };
        return (
          <Tag
            color={
              v === "posted" ? "success" : v === "approved" ? "processing" : "default"
            }
          >
            {statusLabels[v] || t("monthly.status_draft", language)}
          </Tag>
        );
      },
    },
    {
      title: t("monthly.col_actions", language),
      key: "actions",
      width: 220,
      render: (_: any, record: any) => {
        const status = record.posting_status;
        const actions: React.ReactNode[] = [];
        if (status === "draft" && isReviewer) {
          actions.push(
            <Popconfirm
              key="approve"
              title={t("monthly.approve_confirm", language)}
              onConfirm={() => handleApproveEntry(record.id)}
            >
              <Button
                type="text"
                size="small"
                icon={<CheckOutlined />}
                loading={actionLoading[record.id]}
              >
                {t("monthly.approve_entry", language)}
              </Button>
            </Popconfirm>
          );
        }
        if (status === "approved" && isApprover) {
          actions.push(
            <Button
              key="post"
              type="text"
              size="small"
              icon={<SendOutlined />}
              loading={actionLoading[record.id]}
              onClick={() => openPostModal(record)}
            >
              {t("monthly.post_entry", language)}
            </Button>
          );
          actions.push(
            <Popconfirm
              key="reject"
              title={t("monthly.reject_confirm", language)}
              onConfirm={() => message.info(t("monthly.reject_coming_soon", language))}
            >
              <Button type="text" size="small" icon={<RollbackOutlined />}>
                {t("monthly.reject_entry", language)}
              </Button>
            </Popconfirm>
          );
        }
        if (status === "posted" && isAdmin) {
          actions.push(
            <Button
              key="reverse"
              type="text"
              size="small"
              icon={<RollbackOutlined />}
              onClick={() => message.info(t("monthly.reverse_confirm", language))}
            >
              {t("monthly.reverse_entry", language)}
            </Button>
          );
        }
        if (actions.length === 0) {
          return <span style={{ color: "var(--fg-muted)" }}>-</span>;
        }
        return <Space size={0}>{actions}</Space>;
      },
    },
  ];

  // ─── batchColumns ────────────────────────────────────────────

  const batchColumns = [
    { title: t("monthly.col_batch_number", language), dataIndex: "batch_number" },
    { title: t("monthly.col_period", language), dataIndex: "accounting_period" },
    {
      title: t("monthly.col_status", language),
      dataIndex: "status",
      render: (v: string) => (
        <Tag color={v === "completed" ? "processing" : "warning"}>{v}</Tag>
      ),
    },
    { title: t("monthly.col_total_contracts", language), dataIndex: "total_contracts" },
    { title: t("monthly.col_processed", language), dataIndex: "processed_contracts" },
    { title: t("monthly.col_failed", language), dataIndex: "failed_contracts" },
    { title: t("monthly.col_total_entries", language), dataIndex: "total_entries" },
    { title: t("monthly.col_posted_entries", language), dataIndex: "posted_entries" },
    {
      title: t("monthly.col_created_at", language),
      dataIndex: "created_at",
      render: (v: string) => dayjs(v).format("YYYY-MM-DD HH:mm"),
    },
    {
      title: t("monthly.col_actions", language),
      key: "actions",
      width: 220,
      render: (_: any, record: any) => {
        if (!canManage)
          return <span style={{ color: "var(--fg-muted)" }}>-</span>;
        return (
          <Space size={0}>
            <Popconfirm
              title={t("monthly.approve_all_confirm", language, { batch: record.batch_number })}
              onConfirm={() => handleApproveBatch(record.id)}
            >
              <Button
                type="text"
                size="small"
                icon={<CheckOutlined />}
                loading={actionLoading[`batch_${record.id}`]}
              >
                {t("monthly.approve_all", language)}
              </Button>
            </Popconfirm>
            <Popconfirm
              title={t("monthly.post_all_confirm", language, { batch: record.batch_number })}
              onConfirm={() => handlePostBatch(record.id)}
            >
              <Button
                type="text"
                size="small"
                icon={<SendOutlined />}
                loading={actionLoading[`postbatch_${record.id}`]}
              >
                {t("monthly.post_all", language)}
              </Button>
            </Popconfirm>
          </Space>
        );
      },
    },
  ];

  // ─── Skeleton Loading for Entries ───────────────────────────

  const EntrySkeleton = () => (
    <div style={{ padding: "8px 0" }}>
      {Array.from({ length: 5 }).map((_, i) => (
        <Skeleton
          key={i}
          active
          paragraph={{ rows: 1, width: ["100%"] }}
          title={false}
          style={{ padding: "8px 16px" }}
        />
      ))}
    </div>
  );

  // ─── Skeleton Loading for Batches ───────────────────────────

  const BatchSkeleton = () => (
    <div style={{ padding: "8px 0" }}>
      {Array.from({ length: 4 }).map((_, i) => (
        <Skeleton
          key={i}
          active
          paragraph={{ rows: 1, width: ["100%"] }}
          title={false}
          style={{ padding: "8px 16px" }}
        />
      ))}
    </div>
  );

  // ─── Tab Items ──────────────────────────────────────────────

  const tabItems = [
    {
      key: "generate",
      label: t("monthly.tab_generate", language),
      children: (
        <Card
          title={
            <span style={{ fontSize: 15, fontWeight: 600, letterSpacing: "-0.01em" }}>
              {t("monthly.generate_closing", language)}
            </span>
          }
        >
          <Form layout="vertical" onFinish={handleGenerate}>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  label={t("monthly.accounting_period", language)}
                  name="period"
                  rules={[{ required: true, message: t("monthly.select_period", language) }]}
                >
                  <DatePicker.MonthPicker
                    style={{ width: "100%" }}
                    placeholder="YYYY-MM"
                  />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label={t("monthly.discount_rate", language)} name="discount_rate" initialValue={0.05}>
                  <Input type="number" step={0.001} />
                </Form.Item>
              </Col>
            </Row>
            <Button
              type="primary"
              icon={<CalculatorOutlined />}
              htmlType="submit"
              loading={loading}
              size="large"
            >
              {t("monthly.generate_btn", language)}
            </Button>
          </Form>

          {result && (
            <Alert
              message={t("monthly.result_title", language)}
              description={
                <Space direction="vertical">
                  <span>
                    {t("monthly.batch_number", language)}:{" "}
                    <strong style={{ color: "var(--fg-primary)" }}>
                      {result.batch_number}
                    </strong>
                  </span>
                  <span>
                    {t("monthly.status", language)}:{" "}
                    <Tag color={result.status === "completed" ? "processing" : "warning"}>
                      {result.status}
                    </Tag>
                  </span>
                  <span>
                    {t("monthly.processed_contracts", language)}: {result.processed_contracts} / {result.total_contracts}
                  </span>
                  <span>{t("monthly.failed_contracts", language)}: {result.failed_contracts}</span>
                  <span>{t("monthly.total_entries", language)}: {result.total_entries} 笔</span>
                </Space>
              }
              type="info"
              showIcon
              style={{ marginTop: 24 }}
            />
          )}
        </Card>
      ),
    },
    {
      key: "entries",
      label: t("monthly.tab_entries", language),
      children: (
        <Card
          title={
            <span style={{ fontSize: 15, fontWeight: 600, letterSpacing: "-0.01em" }}>
              {t("monthly.entries_preview", language)}
            </span>
          }
          extra={
            canManage && entries.length > 0 ? (
              <Space>
                <Button
                  size="small"
                  icon={<CheckOutlined />}
                  onClick={() => {
                    const draftEntries = entries.filter(
                      (e: any) => e.posting_status === "draft"
                    );
                    if (draftEntries.length === 0) {
                      message.info(t("monthly.no_draft_entries", language));
                      return;
                    }
                    Promise.all(
                      draftEntries.map((e: any) =>
                        monthlyClosingApi
                          .approveEntry(e.id, token!)
                          .catch((err) => ({ error: err }))
                      )
                    ).then((results) => {
                      const succeeded = results.filter((r: any) => !r.error).length;
                      message.success(t("monthly.batch_approve_success_msg", language, { count: String(succeeded) }));
                      refresh();
                    });
                  }}
                >
                  {t("monthly.batch_approve", language)}
                </Button>
                <Button
                  size="small"
                  icon={<SendOutlined />}
                  onClick={() => {
                    const approvedEntries = entries.filter(
                      (e: any) => e.posting_status === "approved"
                    );
                    if (approvedEntries.length === 0) {
                      message.info(t("monthly.no_approved_entries", language));
                      return;
                    }
                    Promise.all(
                      approvedEntries.map((e: any) =>
                        monthlyClosingApi
                          .postEntry(e.id, "", token!)
                          .catch((err) => ({ error: err }))
                      )
                    ).then((results) => {
                      const succeeded = results.filter((r: any) => !r.error).length;
                      message.success(t("monthly.batch_post_success_msg", language, { count: String(succeeded) }));
                      refresh();
                    });
                  }}
                >
                  {t("monthly.batch_post", language)}
                </Button>
                <Button size="small" onClick={() => refresh()}>
                  {t("monthly.refresh", language)}
                </Button>
              </Space>
            ) : undefined
          }
        >
          {isLocked && (
            <Alert
              message={t("monthly.locked_warning", language, { period: selectedPeriod })}
              type="warning"
              showIcon
              style={{ marginBottom: 16 }}
            />
          )}
          {entriesLoading && !entriesLoaded ? (
            <EntrySkeleton />
          ) : entries.length === 0 ? (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description={t("monthly.no_entries", language)}
            />
          ) : (
            <Spin spinning={entriesLoading && entriesLoaded}>
              <Table
                columns={entryColumns}
                dataSource={entries}
                rowKey="id"
                pagination={{ pageSize: 20 }}
                size="small"
                scroll={{ x: 1100 }}
              />
            </Spin>
          )}
        </Card>
      ),
    },
    {
      key: "batches",
      label: t("monthly.tab_batches", language),
      children: (
        <Card
          title={
            <span style={{ fontSize: 15, fontWeight: 600, letterSpacing: "-0.01em" }}>
              {t("monthly.batch_history", language)}
            </span>
          }
          extra={
            <Button size="small" onClick={() => refresh()}>
              {t("monthly.refresh", language)}
            </Button>
          }
        >
          {batchesLoading && !batchesLoaded ? (
            <BatchSkeleton />
          ) : batches.length === 0 ? (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description={t("monthly.no_batches", language)}
            />
          ) : (
            <Spin spinning={batchesLoading && batchesLoaded}>
              <Table
                columns={batchColumns}
                dataSource={batches}
                rowKey="id"
                pagination={{ pageSize: 10 }}
                size="small"
                scroll={{ x: 1200 }}
              />
            </Spin>
          )}
        </Card>
      ),
    },
    {
      key: "lock",
      label: t("monthly.tab_lock", language),
      children: (
        <Card
          title={
            <span style={{ fontSize: 15, fontWeight: 600, letterSpacing: "-0.01em" }}>
              {t("monthly.lock_control", language)}
            </span>
          }
        >
          {!selectedPeriod ? (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description={t("monthly.lock_first", language)}
            />
          ) : lockStatusLoading ? (
            <div style={{ padding: "24px 0" }}>
              <Skeleton active paragraph={{ rows: 2 }} />
            </div>
          ) : (
            <>
              {/* Lock Status Indicator */}
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 20,
                  padding: "24px 28px",
                  borderRadius: 10,
                  marginBottom: 24,
                  border: `1px solid ${
                    isLocked ? "var(--border-strong)" : "var(--border-default)"
                  }`,
                  background: isLocked
                    ? "var(--bg-inset)"
                    : "var(--bg-page)",
                }}
              >
                <div
                  style={{
                    width: 48,
                    height: 48,
                    borderRadius: 10,
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    fontSize: 22,
                    background: isLocked
                      ? "var(--fg-primary)"
                      : "var(--bg-inset)",
                    color: isLocked ? "var(--fg-inverse)" : "var(--fg-tertiary)",
                    flexShrink: 0,
                  }}
                >
                  {isLocked ? <LockOutlined /> : <UnlockOutlined />}
                </div>
                <div style={{ flex: 1 }}>
                  <div style={{ fontSize: 15, fontWeight: 600, color: "var(--fg-primary)", marginBottom: 2 }}>
                    {t("monthly.accounting_period_label", language)} {selectedPeriod}
                  </div>
                  <div
                    style={{
                      fontSize: 13,
                      color: "var(--fg-muted)",
                    }}
                  >
                    {isLocked
                      ? t("monthly.lock_desc_locked", language)
                      : t("monthly.lock_desc_unlocked", language)}
                  </div>
                </div>
                <div style={{ flexShrink: 0 }}>
                  {isLocked ? (
                    <Tag color="error" style={{ margin: 0, fontSize: 13 }}>
                      <LockOutlined style={{ marginRight: 4 }} />
                      {t("monthly.locked", language)}
                    </Tag>
                  ) : (
                    <Tag color="success" style={{ margin: 0, fontSize: 13 }}>
                      <UnlockOutlined style={{ marginRight: 4 }} />
                      {t("monthly.unlocked", language)}
                    </Tag>
                  )}
                </div>
              </div>

              {/* Action Buttons */}
              <Space>
                {!isLocked ? (
                  <Button
                    type="primary"
                    icon={<LockOutlined />}
                    loading={lockLoading}
                    disabled={!isApprover}
                    onClick={handleLockPeriod}
                  >
                    {isApprover ? t("monthly.lock_btn", language) : t("monthly.lock_btn_disabled", language)}
                  </Button>
                ) : (
                  <Button
                    icon={<UnlockOutlined />}
                    loading={lockLoading}
                    disabled={!isAdmin}
                    onClick={handleUnlockPeriod}
                  >
                    {isAdmin ? t("monthly.unlock_btn", language) : t("monthly.unlock_btn_disabled", language)}
                  </Button>
                )}
                <Button
                  onClick={() => checkLockStatus(selectedPeriod)}
                  loading={lockStatusLoading}
                >
                  {t("monthly.refresh_status", language)}
                </Button>
              </Space>

              {!isAdmin && isLocked && (
                <Alert
                  message={t("monthly.contact_admin", language)}
                  type="info"
                  showIcon
                  style={{ marginTop: 16 }}
                />
              )}
            </>
          )}
        </Card>
      ),
    },
  ];

  // ─── Render ─────────────────────────────────────────────────

  return (
    <ProtectedRoute>
      <AppLayout>
        <motion.div
          initial={{ opacity: 0, y: 4 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25, ease: "easeOut" }}
        >
          <PageHeader selectedPeriod={selectedPeriod} entries={entries} />

          <Tabs
            activeKey={activeTab}
            onChange={(key) => {
              setActiveTab(key);
              if (key === "entries" && selectedPeriod) {
                loadEntries(selectedPeriod);
                checkLockStatus(selectedPeriod);
              }
              if (key === "batches" && selectedPeriod) {
                loadBatches(selectedPeriod);
              }
              if (key === "lock" && selectedPeriod) {
                checkLockStatus(selectedPeriod);
              }
            }}
            items={tabItems}
          />

          <Modal
            title={t("monthly.posting_confirm", language)}
            open={postModalOpen}
            onOk={handlePostEntry}
            onCancel={() => {
              setPostModalOpen(false);
              setPostingEntry(null);
              setErpReference("");
            }}
            confirmLoading={postingEntry ? actionLoading[postingEntry.id] : false}
            okText={t("monthly.ok", language) + t("monthly.post_entry", language)}
            cancelText={t("monthly.cancel", language)}
          >
            <p style={{ marginBottom: 16, color: "var(--fg-secondary)" }}>
              {t("monthly.posting_confirm_desc", language)}
            </p>
            {postingEntry && (
              <Descriptions bordered size="small" column={1} style={{ marginBottom: 16 }}>
                <Descriptions.Item label={t("monthly.entry_type", language)}>
                  <Tag color="processing">{postingEntry.entry_type}</Tag>
                </Descriptions.Item>
                <Descriptions.Item label={t("monthly.amount", language)}>
                  ¥
                  {postingEntry.amount?.toLocaleString(undefined, {
                    minimumFractionDigits: 2,
                  })}
                </Descriptions.Item>
                <Descriptions.Item label={t("monthly.description", language)}>
                  {postingEntry.description}
                </Descriptions.Item>
              </Descriptions>
            )}
            <Form.Item label={t("monthly.erp_reference", language)}>
              <Input
                placeholder={t("monthly.erp_placeholder", language)}
                value={erpReference}
                onChange={(e) => setErpReference(e.target.value)}
              />
            </Form.Item>
          </Modal>
        </motion.div>
      </AppLayout>
    </ProtectedRoute>
  );
}
