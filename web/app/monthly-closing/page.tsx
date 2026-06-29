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
  DownloadOutlined,
  ImportOutlined,
} from "@ant-design/icons";
import dayjs from "dayjs";
import { useRouter } from "next/navigation";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { BatchHistoryCard } from "./components/BatchHistoryCard";
import { EntriesPreviewCard } from "./components/EntriesPreviewCard";
import { GenerateClosingCard } from "./components/GenerateClosingCard";
import { LockControlCard } from "./components/LockControlCard";
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
  const [writebackModalOpen, setWritebackModalOpen] = useState(false);
  const [writebackText, setWritebackText] = useState("");
  const [writebackLoading, setWritebackLoading] = useState(false);

  const role = user?.role || "";
  const hasRole = (candidate: string) => user?.roles?.includes(candidate as any) || role === candidate;
  const isAdmin = hasRole("admin");
  const isApprover = hasRole("approver") || isAdmin;
  const isReviewer = hasRole("reviewer") || isApprover;
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

  const handleExportEntries = async () => {
    if (!token || !selectedPeriod) {
      message.warning("请先选择或生成月结期间");
      return;
    }
    setActionLoading((prev) => ({ ...prev, export_entries: true }));
    try {
      const blob = await monthlyClosingApi.exportEntries(
        { period: selectedPeriod, status: "approved", template: "kingdee" },
        token
      );
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `Lease_GL_${selectedPeriod}_kingdee.csv`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
      message.success("ERP 分录文件已导出");
    } catch (error: any) {
      message.error(error.message || "ERP 分录导出失败");
    } finally {
      setActionLoading((prev) => ({ ...prev, export_entries: false }));
    }
  };

  const handleApplyWriteback = async () => {
    if (!token) return;
    const items = writebackText
      .split(/\r?\n/)
      .map((line) => line.trim())
      .filter(Boolean)
      .map((line) => {
        const [entry_id, erp_reference, voucher_number] = line.split(",").map((part) => part.trim());
        return { entry_id, erp_reference, voucher_number };
      })
      .filter((item) => item.entry_id && item.entry_id !== "entry_id");

    if (items.length === 0) {
      message.warning("请粘贴至少一行回写数据");
      return;
    }

    setWritebackLoading(true);
    try {
      const res = await monthlyClosingApi.applyERPWriteback(items, token);
      message.success(`已回写 ${res.applied_count || 0} 条，失败 ${res.failed_count || 0} 条`);
      setWritebackModalOpen(false);
      setWritebackText("");
      refresh();
    } catch (error: any) {
      message.error(error.message || "凭证回写失败");
    } finally {
      setWritebackLoading(false);
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
    { title: "ERP 引用", dataIndex: "erp_reference", width: 130, render: (v: string) => v || "-" },
    { title: "凭证号", dataIndex: "voucher_number", width: 120, render: (v: string) => v || "-" },
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
        <GenerateClosingCard
          language={language}
          loading={loading}
          result={result}
          onGenerate={handleGenerate}
        />
      ),
    },
    {
      key: "entries",
      label: t("monthly.tab_entries", language),
      children: (
        <EntriesPreviewCard
          language={language}
          canManage={canManage}
          entries={entries}
          entriesLoading={entriesLoading}
          entriesLoaded={entriesLoaded}
          isLocked={isLocked}
          selectedPeriod={selectedPeriod}
          actionLoading={actionLoading}
          entryColumns={entryColumns}
          onExportEntries={handleExportEntries}
          onOpenWritebackModal={() => setWritebackModalOpen(true)}
          onBatchApprove={() => {
            const draftEntries = entries.filter((entry: any) => entry.posting_status === "draft");
            if (draftEntries.length === 0) {
              message.info(t("monthly.no_draft_entries", language));
              return;
            }
            Promise.all(
              draftEntries.map((entry: any) =>
                monthlyClosingApi.approveEntry(entry.id, token!).catch((error) => ({ error }))
              )
            ).then((results) => {
              const succeeded = results.filter((result: any) => !result.error).length;
              message.success(
                t("monthly.batch_approve_success_msg", language, { count: String(succeeded) })
              );
              refresh();
            });
          }}
          onBatchPost={() => {
            const approvedEntries = entries.filter((entry: any) => entry.posting_status === "approved");
            if (approvedEntries.length === 0) {
              message.info(t("monthly.no_approved_entries", language));
              return;
            }
            Promise.all(
              approvedEntries.map((entry: any) =>
                monthlyClosingApi.postEntry(entry.id, "", token!).catch((error) => ({ error }))
              )
            ).then((results) => {
              const succeeded = results.filter((result: any) => !result.error).length;
              message.success(
                t("monthly.batch_post_success_msg", language, { count: String(succeeded) })
              );
              refresh();
            });
          }}
          onRefresh={() => refresh()}
          entrySkeleton={<EntrySkeleton />}
        />
      ),
    },
    {
      key: "batches",
      label: t("monthly.tab_batches", language),
      children: (
        <BatchHistoryCard
          language={language}
          batchesLoading={batchesLoading}
          batchesLoaded={batchesLoaded}
          batches={batches}
          batchColumns={batchColumns}
          batchSkeleton={<BatchSkeleton />}
          onRefresh={() => refresh()}
        />
      ),
    },
    {
      key: "lock",
      label: t("monthly.tab_lock", language),
      children: (
        <LockControlCard
          language={language}
          selectedPeriod={selectedPeriod}
          isLocked={isLocked}
          lockLoading={lockLoading}
          lockStatusLoading={lockStatusLoading}
          isApprover={isApprover}
          isAdmin={isAdmin}
          onLock={handleLockPeriod}
          onUnlock={handleUnlockPeriod}
          onRefreshStatus={() => checkLockStatus(selectedPeriod)}
        />
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
            title="ERP 凭证回写"
            open={writebackModalOpen}
            onOk={handleApplyWriteback}
            onCancel={() => setWritebackModalOpen(false)}
            confirmLoading={writebackLoading}
            okText="回写并标记已过账"
            cancelText={t("monthly.cancel", language)}
            width={720}
          >
            <Alert
              type="info"
              showIcon
              message="粘贴 ERP 返回结果，每行格式：entry_id,erp_reference,voucher_number"
              description="导出的 CSV 第一列就是 entry_id。回写成功后，系统会记录 ERP 引用与凭证号，并将对应已审批分录标记为已过账。"
              style={{ marginBottom: 16 }}
            />
            <Input.TextArea
              rows={8}
              value={writebackText}
              onChange={(e) => setWritebackText(e.target.value)}
              placeholder={"entry_id,erp_reference,voucher_number\n550e8400-e29b-41d4-a716-446655440000,KD-2024-001,记-0001"}
            />
          </Modal>

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
