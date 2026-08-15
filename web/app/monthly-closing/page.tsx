"use client";

import { StatusTag, statusKindFromAntColor } from "../components/StatusTag";

import { Suspense, useState, useCallback } from "react";
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
  Select,
  Steps,
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
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { monthlyClosingApi } from "../lib/api";
import { hasRole, useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { fmtMoney } from "../lib/format";
import { useUrlState } from "../hooks/useUrlState";
import { notifyError } from "../lib/notify";

// ─── Types ─────────────────────────────────────────────────────

interface EntrySummary {
  total: number;
  draft_count: number;
  approved_count: number;
  posted_count: number;
  reversed_count: number;
  total_amount: number;
  contract_count: number;
}

interface EntryPeriod {
  accounting_period: string;
  entry_count: number;
  contract_count: number;
  draft_count: number;
  posted_count: number;
  total_amount: number;
  is_locked: boolean;
}

const ENTRY_STATUS_OPTIONS = [
  { value: "draft", label: "草稿" },
  { value: "approved", label: "已审批" },
  { value: "posted", label: "已过账" },
  { value: "reversed", label: "已红冲" },
];

// The values are the entry types the close and the event engine actually
// produce; anything else would filter to an empty page that looks like data.
const ENTRY_TYPE_OPTIONS = [
  { value: "interest", label: "利息费用" },
  { value: "depreciation", label: "折旧" },
  { value: "payment", label: "租金支付" },
  { value: "fx_remeasurement", label: "外币重估" },
  { value: "remeasurement", label: "重新计量" },
  { value: "modification", label: "合同变更" },
  { value: "reassessment", label: "重新评估" },
  { value: "impairment", label: "减值" },
];

function CloseProcessRail({
  language,
  period,
  summary,
  result,
  isLocked,
  onNext,
}: {
  language: Parameters<typeof t>[1];
  period: string;
  summary: EntrySummary | null;
  result: any;
  isLocked: boolean;
  onNext: () => void;
}) {
  const current = isLocked ? 5 : summary?.posted_count ? 4 : summary?.approved_count ? 3 : summary?.total ? 2 : result ? 1 : 0;
  const nextLabel = current === 0 ? t("monthly.process_generate", language) : current === 1 ? t("monthly.process_review", language) : current === 2 ? t("monthly.process_approve", language) : current === 3 ? t("monthly.process_post", language) : current === 4 ? t("monthly.process_lock", language) : t("monthly.process_complete", language);
  return (
    <Card styles={{ body: { padding: "16px 20px" } }} style={{ marginBottom: 16, borderRadius: 10 }}>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 16, marginBottom: 12 }}>
        <div>
          <div style={{ fontSize: 12, color: "var(--fg-tertiary)", marginBottom: 4 }}>{period || t("monthly.process_select_period", language)}</div>
          <div style={{ fontSize: 15, fontWeight: 600 }}>{t("monthly.process_title", language)}</div>
        </div>
        <Button type="primary" onClick={onNext} disabled={!period && current > 0}>{nextLabel}</Button>
      </div>
      <Steps
        size="small"
        current={current}
        responsive
        items={[
          { title: t("monthly.process_readiness", language) },
          { title: t("monthly.process_generate", language) },
          { title: t("monthly.process_review", language) },
          { title: t("monthly.process_approve", language) },
          { title: t("monthly.process_post", language) },
          { title: t("monthly.process_lock", language) },
        ]}
      />
    </Card>
  );
}

// ─── Main Component ────────────────────────────────────────────

function MonthlyClosingPage() {
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
  const [entrySummary, setEntrySummary] = useState<EntrySummary | null>(null);
  const [entryPeriods, setEntryPeriods] = useState<EntryPeriod[]>([]);
  const [entryStatus, setEntryStatus] = useState<string | undefined>();
  const [entryType, setEntryType] = useState<string | undefined>();
  const [entryPage, setEntryPage] = useState(1);
  const [entryPageSize, setEntryPageSize] = useState(20);
  const [batches, setBatches] = useState<any[]>([]);
  const [activeTab, setActiveTab] = useUrlState("tab", "generate");
  const [selectedPeriod, setSelectedPeriod] = useUrlState("period", "");
  const [isLocked, setIsLocked] = useState(false);
  const [lockLoading, setLockLoading] = useState(false);
  const [lockStatusLoading, setLockStatusLoading] = useState(false);
  const [postModalOpen, setPostModalOpen] = useState(false);
  const [postingEntry, setPostingEntry] = useState<any>(null);
  const [reverseModalOpen, setReverseModalOpen] = useState(false);
  const [reversingEntry, setReversingEntry] = useState<any>(null);
  const [reverseReason, setReverseReason] = useState("");
  const [reversePeriod, setReversePeriod] = useState("");
  const [rejectModalOpen, setRejectModalOpen] = useState(false);
  const [rejectingEntry, setRejectingEntry] = useState<any>(null);
  const [rejectReason, setRejectReason] = useState("");
  const [erpReference, setErpReference] = useState("");
  const [erpTemplate, setErpTemplate] = useState("");
  const [writebackModalOpen, setWritebackModalOpen] = useState(false);
  const [writebackText, setWritebackText] = useState("");
  const [writebackLoading, setWritebackLoading] = useState(false);

  const isAdmin = hasRole(user, "admin");
  const isApprover = hasRole(user, "approver") || isAdmin;
  const isReviewer = hasRole(user, "reviewer") || isApprover;
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
      notifyError(t("monthly.select_period", language));
      return;
    }
    setLoading(true);
    try {
      const data = await monthlyClosingApi.generate(
        {
          accounting_period: period,
          discount_rate: values.discount_rate,
        },
        token
      );
      setResult(data);
      setSelectedPeriod(period);
      message.success(t("monthly.generate_success", language, { count: String(data.processed_contracts) }));
      await checkLockStatus(period);
      loadBatches(period);
      loadEntryPeriods();
      loadEntries(period);
      setActiveTab("entries");
    } catch (error: any) {
      notifyError(error.message || t("monthly.generate_failed", language));
    } finally {
      setLoading(false);
    }
  };

  // Entries are read straight from the ledger for the chosen period, so a
  // closed period can be reviewed without regenerating anything. Paging and
  // filtering happen on the server; the page never holds a whole period.
  const loadEntries = useCallback(
    async (
      period: string,
      overrides?: { status?: string; entryType?: string; page?: number; pageSize?: number }
    ) => {
      if (!token) return;
      setEntriesLoading(true);
      try {
        const data = await monthlyClosingApi.getEntries(
          {
            period,
            status: overrides?.status ?? entryStatus,
            entry_type: overrides?.entryType ?? entryType,
            page: overrides?.page ?? entryPage,
            page_size: overrides?.pageSize ?? entryPageSize,
          },
          token
        );
        setEntries(data.data || []);
        setEntrySummary(data.summary || null);
        setEntriesLoaded(true);
      } catch (error: any) {
        notifyError(error.message || t("monthly.load_entries_failed", language));
      } finally {
        setEntriesLoading(false);
      }
    },
    [token, language, entryStatus, entryType, entryPage, entryPageSize]
  );

  // The period list is what makes the query independent: it comes from the
  // entries that exist, not from a date the user has to remember.
  const loadEntryPeriods = useCallback(async () => {
    if (!token) return [] as EntryPeriod[];
    try {
      const data = await monthlyClosingApi.listPeriods(token);
      const list: EntryPeriod[] = data.data || [];
      setEntryPeriods(list);
      return list;
    } catch {
      return [] as EntryPeriod[];
    }
  }, [token]);

  // Opening the entries tab with nothing selected falls back to the most recent
  // period that has entries, which is almost always the one being worked on.
  const openEntriesTab = useCallback(async () => {
    const list = await loadEntryPeriods();
    const period = selectedPeriod || list[0]?.accounting_period;
    if (!period) {
      setEntriesLoaded(true);
      return;
    }
    if (period !== selectedPeriod) setSelectedPeriod(period);
    loadEntries(period);
    checkLockStatus(period);
  }, [loadEntryPeriods, selectedPeriod, loadEntries, checkLockStatus]);

  const changeEntryPeriod = (period: string) => {
    setSelectedPeriod(period);
    setEntryPage(1);
    loadEntries(period, { page: 1 });
    checkLockStatus(period);
    loadBatches(period);
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
      notifyError(error.message || t("monthly.approve_failed", language));
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
      notifyError(error.message || t("monthly.post_failed", language));
    } finally {
      setActionLoading((prev) => ({ ...prev, [postingEntry.id]: false }));
    }
  };

  const handleExportEntries = async () => {
    if (!token || !selectedPeriod) {
      message.warning("请先选择或生成月结期间");
      return;
    }
    if (!erpTemplate.trim()) {
      message.warning(t("monthly.export_template_required", language));
      return;
    }
    setActionLoading((prev) => ({ ...prev, export_entries: true }));
    try {
      const blob = await monthlyClosingApi.exportEntries(
        { period: selectedPeriod, status: "approved", template: erpTemplate.trim() },
        token
      );
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `Lease_GL_${selectedPeriod}_${erpTemplate.trim()}.csv`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
      message.success("ERP 分录文件已导出");
    } catch (error: any) {
      notifyError(error.message || "ERP 分录导出失败");
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
      notifyError(error.message || "凭证回写失败");
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
      notifyError(error.message || t("monthly.batch_approve_failed", language));
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
      notifyError(error.message || t("monthly.batch_post_failed", language));
    } finally {
      setActionLoading((prev) => ({ ...prev, [`postbatch_${batchId}`]: false }));
    }
  };

  const handleLockPeriod = async () => {
    if (!token || !selectedPeriod) {
      notifyError(t("monthly.no_period", language));
      return;
    }
    setLockLoading(true);
    try {
      await monthlyClosingApi.lockPeriod(selectedPeriod, token);
      message.success(t("monthly.lock_success", language, { period: selectedPeriod }));
      await checkLockStatus(selectedPeriod);
    } catch (error: any) {
      notifyError(error.message || t("monthly.lock_failed", language));
    } finally {
      setLockLoading(false);
    }
  };

  const handleUnlockPeriod = async () => {
    if (!token || !selectedPeriod) {
      notifyError(t("monthly.no_period", language));
      return;
    }
    setLockLoading(true);
    try {
      await monthlyClosingApi.unlockPeriod(selectedPeriod, token);
      message.success(t("monthly.unlock_success", language, { period: selectedPeriod }));
      await checkLockStatus(selectedPeriod);
    } catch (error: any) {
      notifyError(error.message || t("monthly.unlock_failed", language));
    } finally {
      setLockLoading(false);
    }
  };

  const openPostModal = (entry: any) => {
    setPostingEntry(entry);
    setErpReference("");
    setPostModalOpen(true);
  };

  const openRejectModal = (entry: any) => {
    setRejectingEntry(entry);
    setRejectReason("");
    setRejectModalOpen(true);
  };

  const closeRejectModal = () => {
    setRejectModalOpen(false);
    setRejectingEntry(null);
    setRejectReason("");
  };

  const handleRejectEntry = async () => {
    if (!token || !rejectingEntry) return;
    if (!rejectReason.trim()) {
      message.warning(t("monthly.reject_reason_required", language));
      return;
    }
    setActionLoading((prev) => ({ ...prev, [rejectingEntry.id]: true }));
    try {
      await monthlyClosingApi.rejectEntry(rejectingEntry.id, rejectReason.trim(), token);
      message.success(t("monthly.reject_success", language));
      closeRejectModal();
      refresh();
    } catch (error: any) {
      notifyError(error?.message || t("monthly.reject_failed", language));
    } finally {
      setActionLoading((prev) => ({ ...prev, [rejectingEntry.id]: false }));
    }
  };

  const openReverseModal = (entry: any) => {
    setReversingEntry(entry);
    setReverseReason("");
    // Blank means "the original entry's own period", which is what the backend
    // defaults to. It only needs overriding once that period has been locked.
    setReversePeriod("");
    setReverseModalOpen(true);
  };

  const closeReverseModal = () => {
    setReverseModalOpen(false);
    setReversingEntry(null);
    setReverseReason("");
    setReversePeriod("");
  };

  const handleReverseEntry = async () => {
    if (!token || !reversingEntry) return;
    if (!reverseReason.trim()) {
      message.warning(t("monthly.reverse_reason_required", language));
      return;
    }
    setActionLoading((prev) => ({ ...prev, [reversingEntry.id]: true }));
    try {
      await monthlyClosingApi.reverseEntry(
        reversingEntry.id,
        { reason: reverseReason.trim(), accounting_period: reversePeriod.trim() },
        token
      );
      message.success(t("monthly.reverse_success", language));
      closeReverseModal();
      refresh();
    } catch (error: any) {
      notifyError(error?.message || t("monthly.reverse_failed", language));
    } finally {
      setActionLoading((prev) => ({ ...prev, [reversingEntry.id]: false }));
    }
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
          // The remaining types are named once, in the filter, so the column and
          // the filter can never disagree about what a type is called.
          ...Object.fromEntries(ENTRY_TYPE_OPTIONS.map((o) => [o.value, o.label])),
        };
        return <StatusTag kind="processing">{labels[v] || v}</StatusTag>;
      },
    },
    { title: t("monthly.col_debit_account", language), dataIndex: "debit_account", width: 150 },
    { title: t("monthly.col_credit_account", language), dataIndex: "credit_account", width: 150 },
    {
      title: t("monthly.col_amount", language),
      dataIndex: "amount",
      width: 120,
      align: "right" as const,
      render: (v: number, record: any) => fmtMoney(v, record.currency),
    },
    { title: t("monthly.col_currency", language), dataIndex: "currency", width: 60 },
    { title: t("monthly.col_description", language), dataIndex: "description", ellipsis: true },
    { title: "ERP 引用", dataIndex: "erp_reference", width: 130, render: (v: string) => v || "-" },
    { title: "凭证号", dataIndex: "voucher_number", width: 120, render: (v: string) => v || "-" },
    {
      title: t("monthly.col_status", language),
      dataIndex: "posting_status",
      width: 110,
      render: (v: string, record: any) => {
        const statusLabels: Record<string, string> = {
          posted: t("monthly.status_posted", language),
          approved: t("monthly.status_approved", language),
          draft: t("monthly.status_draft", language),
          reversed: t("monthly.status_reversed", language),
        };
        const colors: Record<string, string> = {
          posted: "success",
          approved: "processing",
          reversed: "error",
        };
        return (
          <Space direction="vertical" size={2}>
            <StatusTag kind={statusKindFromAntColor(colors[v] || "default")}>
              {statusLabels[v] || t("monthly.status_draft", language)}
            </StatusTag>
            {/* A reversing entry is only readable next to what it cancels. */}
            {record.reversal_of_entry_id && (
              <StatusTag kind="warning" style={{ fontSize: 11 }}>
                {t("monthly.is_reversal_entry", language)}
              </StatusTag>
            )}
          </Space>
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
            <Button
              key="reject"
              type="text"
              size="small"
              icon={<RollbackOutlined />}
              loading={actionLoading[record.id]}
              onClick={() => openRejectModal(record)}
            >
              {t("monthly.reject_entry", language)}
            </Button>
          );
        }
        if (status === "posted" && isApprover) {
          actions.push(
            <Button
              key="reverse"
              type="text"
              size="small"
              icon={<RollbackOutlined />}
              loading={actionLoading[record.id]}
              onClick={() => openReverseModal(record)}
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
        <StatusTag kind={statusKindFromAntColor(v === "completed" ? "processing" : "warning")}>{v}</StatusTag>
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
                <Form.Item label={t("monthly.discount_rate", language)} name="discount_rate">
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
                    <StatusTag kind={statusKindFromAntColor(result.status === "completed" ? "processing" : "warning")}>
                      {result.status}
                    </StatusTag>
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
                <Input
                  size="small"
                  value={erpTemplate}
                  onChange={(event) => setErpTemplate(event.target.value)}
                  placeholder={t("monthly.export_template_placeholder", language)}
                  style={{ width: 150 }}
                />
                <Button
                  size="small"
                  icon={<DownloadOutlined />}
                  loading={actionLoading.export_entries}
                  onClick={handleExportEntries}
                >
                  {t("monthly.export_erp_csv", language)}
                </Button>
                <Button
                  size="small"
                  icon={<ImportOutlined />}
                  onClick={() => setWritebackModalOpen(true)}
                >
                  凭证回写
                </Button>
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
                  {t("monthly.batch_approve", language)}（本页）
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
                  {t("monthly.batch_post", language)}（本页）
                </Button>
                <Button size="small" onClick={() => refresh()}>
                  {t("monthly.refresh", language)}
                </Button>
              </Space>
            ) : undefined
          }
        >
          <Space wrap size={12} style={{ marginBottom: 16 }}>
            <span style={{ fontSize: 13, color: "var(--fg-tertiary)" }}>会计期间</span>
            <Select
              style={{ width: 260 }}
              value={selectedPeriod || undefined}
              onChange={changeEntryPeriod}
              placeholder="选择一个有分录的期间"
              notFoundContent="暂无已生成分录的期间"
              options={entryPeriods.map((p) => ({
                value: p.accounting_period,
                label: `${p.accounting_period}（${p.entry_count} 笔 / ${p.contract_count} 份合同${
                  p.is_locked ? "，已锁账" : ""
                }）`,
              }))}
            />
            <Select
              style={{ width: 140 }}
              allowClear
              value={entryStatus}
              onChange={(value) => {
                setEntryStatus(value);
                setEntryPage(1);
                if (selectedPeriod) loadEntries(selectedPeriod, { status: value, page: 1 });
              }}
              placeholder="全部状态"
              options={ENTRY_STATUS_OPTIONS}
            />
            <Select
              style={{ width: 160 }}
              allowClear
              value={entryType}
              onChange={(value) => {
                setEntryType(value);
                setEntryPage(1);
                if (selectedPeriod) loadEntries(selectedPeriod, { entryType: value, page: 1 });
              }}
              placeholder="全部分录类型"
              options={ENTRY_TYPE_OPTIONS}
            />
            {entrySummary && (
              <span style={{ fontSize: 13, color: "var(--fg-muted)" }}>
                共 {entrySummary.total} 笔 · {entrySummary.contract_count} 份合同 · 合计{" "}
                {entrySummary.total_amount.toLocaleString(undefined, {
                  minimumFractionDigits: 2,
                  maximumFractionDigits: 2,
                })}
              </span>
            )}
          </Space>

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
              description={
                selectedPeriod
                  ? t("monthly.no_entries", language)
                  : "尚无任何已生成分录的期间，请先在「生成月结」中运行一次结账"
              }
            />
          ) : (
            <Spin spinning={entriesLoading && entriesLoaded}>
              <Table
                columns={entryColumns}
                dataSource={entries}
                rowKey="id"
                // The server holds the period; the table shows one page of it,
                // so the count comes from the summary rather than the rows.
                pagination={{
                  current: entryPage,
                  pageSize: entryPageSize,
                  total: entrySummary?.total ?? entries.length,
                  showSizeChanger: true,
                  pageSizeOptions: ["20", "50", "100", "200"],
                  showTotal: (total) => `共 ${total} 笔`,
                  onChange: (page, size) => {
                    setEntryPage(page);
                    setEntryPageSize(size);
                    if (selectedPeriod) loadEntries(selectedPeriod, { page, pageSize: size });
                  },
                }}
                size="small"
                scroll={{ x: 1360 }}
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
                    <StatusTag kind="error" style={{ margin: 0, fontSize: 13 }}>
                      <LockOutlined style={{ marginRight: 4 }} />
                      {t("monthly.locked", language)}
                    </StatusTag>
                  ) : (
                    <StatusTag kind="success" style={{ margin: 0, fontSize: 13 }}>
                      <UnlockOutlined style={{ marginRight: 4 }} />
                      {t("monthly.unlocked", language)}
                    </StatusTag>
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
          initial={false}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.25, ease: "easeOut" }}
        >
          <PageHeader
            title={t("monthly.title", language)}

            meta={selectedPeriod ? (
              <Space size={12} style={{ marginTop: 8 }}>
                <span style={{ fontSize: 13, color: "var(--fg-tertiary)" }}>
                  {t("monthly.current_period", language)}：<strong style={{ color: "var(--fg-primary)" }}>{selectedPeriod}</strong>
                </span>
                <span style={{ color: "var(--border-strong)" }}>·</span>
                <span style={{ fontSize: 13, color: "var(--fg-muted)" }}>
                  {t("monthly.status_summary", language, {
                    draftCount: String(entrySummary?.draft_count ?? 0),
                    approvedCount: String(entrySummary?.approved_count ?? 0),
                    postedCount: String(entrySummary?.posted_count ?? 0),
                  })}
                </span>
              </Space>
            ) : undefined}
          />

          <CloseProcessRail
            language={language}
            period={selectedPeriod}
            summary={entrySummary}
            result={result}
            isLocked={isLocked}
            onNext={() => {
              if (!selectedPeriod) {
                setActiveTab("generate");
                return;
              }
              const nextTab = isLocked ? "lock" : entrySummary?.posted_count ? "lock" : entrySummary?.approved_count ? "entries" : entrySummary?.total ? "entries" : "generate";
              setActiveTab(nextTab);
              if (nextTab === "entries") openEntriesTab();
              if (nextTab === "lock") checkLockStatus(selectedPeriod);
            }}
          />

          <Tabs
            activeKey={activeTab}
            onChange={(key) => {
              setActiveTab(key);
              if (key === "entries") {
                openEntriesTab();
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
                  <StatusTag kind="processing">{postingEntry.entry_type}</StatusTag>
                </Descriptions.Item>
                <Descriptions.Item label={t("monthly.amount", language)}>
                  {fmtMoney(postingEntry.amount, postingEntry.currency)}
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

          <Modal
            title={t("monthly.reject_title", language)}
            open={rejectModalOpen}
            onOk={handleRejectEntry}
            onCancel={closeRejectModal}
            confirmLoading={rejectingEntry ? actionLoading[rejectingEntry.id] : false}
            okText={t("monthly.reject_entry", language)}
            okButtonProps={{ danger: true }}
            cancelText={t("monthly.cancel", language)}
          >
            <p style={{ marginBottom: 16, color: "var(--fg-secondary)" }}>
              {t("monthly.reject_desc", language)}
            </p>
            {rejectingEntry && (
              <Descriptions bordered size="small" column={1} style={{ marginBottom: 16 }}>
                <Descriptions.Item label={t("monthly.entry_type", language)}>
                  <StatusTag kind="processing">{rejectingEntry.entry_type}</StatusTag>
                </Descriptions.Item>
                <Descriptions.Item label={t("monthly.amount", language)}>
                  {rejectingEntry.amount?.toLocaleString(undefined, {
                    minimumFractionDigits: 2,
                  })}{" "}
                  {rejectingEntry.currency}
                </Descriptions.Item>
              </Descriptions>
            )}
            <Form.Item label={t("monthly.reject_reason", language)} required>
              <Input.TextArea
                rows={2}
                placeholder={t("monthly.reject_reason_placeholder", language)}
                value={rejectReason}
                onChange={(e) => setRejectReason(e.target.value)}
              />
            </Form.Item>
          </Modal>

          <Modal
            title={t("monthly.reverse_title", language)}
            open={reverseModalOpen}
            onOk={handleReverseEntry}
            onCancel={closeReverseModal}
            confirmLoading={reversingEntry ? actionLoading[reversingEntry.id] : false}
            okText={t("monthly.reverse_entry", language)}
            okButtonProps={{ danger: true }}
            cancelText={t("monthly.cancel", language)}
          >
            <p style={{ marginBottom: 16, color: "var(--fg-secondary)" }}>
              {t("monthly.reverse_desc", language)}
            </p>
            {reversingEntry && (
              <Descriptions bordered size="small" column={1} style={{ marginBottom: 16 }}>
                <Descriptions.Item label={t("monthly.entry_type", language)}>
                  <StatusTag kind="processing">{reversingEntry.entry_type}</StatusTag>
                </Descriptions.Item>
                <Descriptions.Item label={t("monthly.amount", language)}>
                  {reversingEntry.amount?.toLocaleString(undefined, {
                    minimumFractionDigits: 2,
                  })}{" "}
                  {reversingEntry.currency}
                </Descriptions.Item>
                <Descriptions.Item label={t("monthly.reverse_direction", language)}>
                  {t("monthly.reverse_direction_value", language, {
                    debit: reversingEntry.credit_account,
                    credit: reversingEntry.debit_account,
                  })}
                </Descriptions.Item>
              </Descriptions>
            )}
            <Form.Item label={t("monthly.reverse_reason", language)} required>
              <Input.TextArea
                rows={2}
                placeholder={t("monthly.reverse_reason_placeholder", language)}
                value={reverseReason}
                onChange={(e) => setReverseReason(e.target.value)}
              />
            </Form.Item>
            <Form.Item
              label={t("monthly.reverse_period", language)}
              extra={t("monthly.reverse_period_hint", language)}
            >
              <Input
                placeholder={reversingEntry?.accounting_period || "YYYY-MM"}
                value={reversePeriod}
                onChange={(e) => setReversePeriod(e.target.value)}
              />
            </Form.Item>
          </Modal>
        </motion.div>
      </AppLayout>
    </ProtectedRoute>
  );
}

export default function MonthlyClosingPageWithUrlState() {
  return (
    <Suspense fallback={<div style={{ minHeight: "100vh", background: "var(--bg-page)" }} />}>
      <MonthlyClosingPage />
    </Suspense>
  );
}
