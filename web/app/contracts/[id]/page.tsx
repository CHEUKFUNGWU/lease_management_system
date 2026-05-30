"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import {
  Card,
  Descriptions,
  Tag,
  Button,
  Table,
  Modal,
  Form,
  Input,
  InputNumber,
  DatePicker,
  Select,
  message,
  Spin,
  Divider,
  Space,
  Row,
  Col,
  Statistic,
  Tabs,
  Timeline,
  Upload,
  Alert,
} from "antd";
import {
  ArrowLeftOutlined,
  PlusOutlined,
  CalculatorOutlined,
  FileTextOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  CloseCircleOutlined,
  UploadOutlined,
  RobotOutlined,
  WarningOutlined,
  EditOutlined,
  ImportOutlined,
  CheckOutlined,
  StopOutlined,
  UndoOutlined,
} from "@ant-design/icons";
import AppLayout from "../../components/AppLayout";
import ProtectedRoute from "../../components/ProtectedRoute";
import { contractApi, paymentScheduleApi, eventApi, aiApi } from "../../lib/api";
import { useAuth } from "../../context/AuthContext";
import { useLanguage } from "../../context/LanguageContext";
import { t } from "../../lib/i18n";
import { parseTagString, normalizeTagValues, DEFAULT_TAG_SUGGESTIONS } from "../../lib/tags";
import dayjs from "dayjs";
import { motion } from "framer-motion";

interface ContractDetail {
  id: string;
  contract_number: string;
  contract_name: string;
  legal_entity_id: string;
  store_id: string;
  landlord_id: string;
  lessee_name: string;
  lessor_name: string;
  store_name: string;
  store_address: string;
  tags: string;
  currency: string;
  signing_date?: string;
  commencement_date: string;
  lease_start_date: string;
  lease_end_date: string;
  discount_rate_type?: string;
  discount_rate_version?: string;
  discount_rate_value?: number;
  discount_rate_missing: boolean;
  status: string;
  approval_status: string;
  is_official_version: boolean;
  reviewed_by?: string;
  reviewed_at?: string;
  approved_by?: string;
  approved_at?: string;
  submitted_at?: string;
  rejected_reason?: string;
  created_at: string;
}

interface PaymentSchedule {
  id: string;
  due_date: string;
  amount: number;
  currency: string;
  payment_timing: string;
  amount_type: string;
  is_fixed: boolean;
  is_variable: boolean;
  is_lease_component: boolean;
  included_in_liability_pv: boolean;
  approval_status: string;
}

interface CalcResult {
  initial_liability: number;
  initial_rou_asset: number;
  total_days: number;
  monthly_summary: MonthlyEntry[];
}

interface MonthlyEntry {
  Year: number;
  Month: number;
  OpeningLiability: number;
  InterestExpense: number;
  TotalPayments: number;
  ClosingLiability: number;
  OpeningROUAsset: number;
  Depreciation: number;
  ClosingROUAsset: number;
  VariableRentExpense: number;
  NonLeaseExpense: number;
}

export default function ContractDetailPage() {
  const params = useParams();
  const router = useRouter();
  const { token, user } = useAuth();
  const { language } = useLanguage();
  const contractId = params.id as string;

  const [contract, setContract] = useState<ContractDetail | null>(null);
  const [schedules, setSchedules] = useState<PaymentSchedule[]>([]);
  const [calcResult, setCalcResult] = useState<CalcResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [calcLoading, setCalcLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm();
  const [activeTab, setActiveTab] = useState("info");
  const [uploadLoading, setUploadLoading] = useState(false);
  const [aiDrafts, setAiDrafts] = useState<any[]>([]);
  const [aiWarnings, setAiWarnings] = useState<string[]>([]);
  const [showDraftPanel, setShowDraftPanel] = useState(false);
  const [importLoading, setImportLoading] = useState(false);
  const [events, setEvents] = useState<any[]>([]);
  const [eventModalOpen, setEventModalOpen] = useState(false);
  const [eventForm] = Form.useForm();
  const [eventLoading, setEventLoading] = useState(false);
  const [rejectModalOpen, setRejectModalOpen] = useState(false);
  const [rejectModalType, setRejectModalType] = useState<'review' | 'approve'>('review');
  const [rejectReason, setRejectReason] = useState('');
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [editModalOpen, setEditModalOpen] = useState(false);
  const [editForm] = Form.useForm();
  const [editLoading, setEditLoading] = useState(false);
  const [eventRejectModalOpen, setEventRejectModalOpen] = useState(false);
  const [eventRejectReason, setEventRejectReason] = useState('');
  const [eventRejectEventId, setEventRejectEventId] = useState<string | null>(null);
  const [eventRejectType, setEventRejectType] = useState<'review' | 'approve'>('review');
  const [eventActionLoading, setEventActionLoading] = useState<string | null>(null);
  const [editDraftModalOpen, setEditDraftModalOpen] = useState(false);
  const [editDraftIndex, setEditDraftIndex] = useState<number>(-1);
  const [editDraftForm] = Form.useForm();
  const [adjustmentModalOpen, setAdjustmentModalOpen] = useState(false);
  const [adjustmentModalData, setAdjustmentModalData] = useState<any>(null);
  const [adjustmentModalTitle, setAdjustmentModalTitle] = useState("");
  const [previewLoading, setPreviewLoading] = useState(false);

  useEffect(() => {
    if (token && contractId) {
      loadContract();
      loadSchedules();
      loadEvents();
    }
  }, [token, contractId]);

  const loadContract = async () => {
    setLoading(true);
    try {
      const data = await contractApi.get(contractId, token!);
      setContract(data);
    } catch (error: any) {
      message.error(error.message || t("contract_detail.load_contract_failed", language));
    } finally {
      setLoading(false);
    }
  };

  const loadSchedules = async () => {
    try {
      const data = await paymentScheduleApi.list(contractId, token!);
      setSchedules(data.data || []);
    } catch (error: any) {
      message.error(error.message || t("contract_detail.load_schedules_failed", language));
    }
  };

  const loadEvents = async () => {
    try {
      const data = await eventApi.list(contractId, token!);
      setEvents(data.data || []);
    } catch (error: any) {
      message.error(error.message || t("contract_detail.load_events_failed", language));
    }
  };

  const handleCreateEvent = async (values: any) => {
    setEventLoading(true);
    try {
      await eventApi.create(contractId, {
        contract_id: contractId,
        event_type: values.event_type,
        effective_date: values.effective_date?.format("YYYY-MM-DD"),
        original_value: values.original_value,
        new_value: values.new_value,
        change_reason: values.change_reason,
        judgment_basis: values.judgment_basis,
      }, token!);
      message.success(t("contract_detail.event_created", language));
      setEventModalOpen(false);
      eventForm.resetFields();
      loadEvents();
    } catch (error: any) {
      message.error(error.message || t("contract_detail.create_event_failed", language));
    } finally {
      setEventLoading(false);
    }
  };

  const handleEventSubmitForReview = async (eventId: string) => {
    setEventActionLoading(eventId + '_submit');
    try {
      await eventApi.submitForReview(contractId, eventId, token!);
      message.success(t("contract_detail.event_submitted", language));
      loadEvents();
    } catch (error: any) {
      message.error(error.message || t("contract_detail.submit_failed", language));
    } finally {
      setEventActionLoading(null);
    }
  };

  const handleEventReviewApprove = async (eventId: string) => {
    setEventActionLoading(eventId + '_review');
    try {
      await eventApi.review(contractId, eventId, true, '', token!);
      message.success(t("contract_detail.review_passed", language));
      loadEvents();
    } catch (error: any) {
      message.error(error.message || t("contract_detail.review_failed", language));
    } finally {
      setEventActionLoading(null);
    }
  };

  const handleEventApprove = async (eventId: string) => {
    setEventActionLoading(eventId + '_approve');
    try {
      await eventApi.approve(contractId, eventId, token!);
      message.success(t("contract_detail.approval_passed", language));
      loadEvents();
    } catch (error: any) {
      message.error(error.message || t("contract_detail.approve_failed", language));
    } finally {
      setEventActionLoading(null);
    }
  };

  const handleEventRejectOpen = (eventId: string, type: 'review' | 'approve') => {
    setEventRejectEventId(eventId);
    setEventRejectType(type);
    setEventRejectReason('');
    setEventRejectModalOpen(true);
  };

  const handleEventRejectSubmit = async () => {
    if (!eventRejectReason.trim()) {
      message.warning(t("contract_detail.please_enter_reason", language));
      return;
    }
    if (!eventRejectEventId) return;

    setEventActionLoading(eventRejectEventId + '_reject');
    try {
      if (eventRejectType === 'review') {
        await eventApi.review(contractId, eventRejectEventId, false, eventRejectReason, token!);
        message.success(t("contract_detail.returned_to_editor", language));
      } else {
        await eventApi.reject(contractId, eventRejectEventId, eventRejectReason, token!);
        message.success(t("contract_detail.rejected", language));
      }
      setEventRejectModalOpen(false);
      setEventRejectReason('');
      setEventRejectEventId(null);
      loadEvents();
    } catch (error: any) {
      message.error(error.message || t("contract_detail.operation_failed", language));
    } finally {
      setEventActionLoading(null);
    }
  };

  const handlePreviewAdjustment = async (eventId: string) => {
    setPreviewLoading(true);
    try {
      const data = await eventApi.previewAdjustment(contractId, eventId, token!);
      setAdjustmentModalTitle(t("contract_detail.adjustment_event_impact_preview", language));
      setAdjustmentModalData(data);
      setAdjustmentModalOpen(true);
    } catch (error: any) {
      message.error(error.message || t("contract_detail.preview_failed", language));
    } finally {
      setPreviewLoading(false);
    }
  };

  const handleViewAdjustment = async (eventId: string) => {
    try {
      const data = await eventApi.getAdjustment(contractId, eventId, token!);
      setAdjustmentModalTitle(t("contract_detail.adjustment_event_detail", language));
      setAdjustmentModalData(data);
      setAdjustmentModalOpen(true);
    } catch (error: any) {
      message.error(error.message || t("contract_detail.get_adjustment_failed", language));
    }
  };

  const handleRecalculateEvent = async (eventId: string) => {
    try {
      await eventApi.recalculate(contractId, eventId, token!);
      message.success(t("contract_detail.event_recalculated", language));
      loadEvents();
    } catch (error: any) {
      message.error(error.message || t("contract_detail.recalculate_failed", language));
    }
  };

  const handleSubmitForReview = async () => {
    setActionLoading('submit');
    try {
      await contractApi.submitForReview(contractId, token!);
      message.success(t("contract_detail.submit_review_success", language));
      loadContract();
    } catch (error: any) {
      message.error(error.message || t("contract_detail.submit_failed", language));
    } finally {
      setActionLoading(null);
    }
  };

  const handleReviewApprove = async () => {
    setActionLoading('review_approve');
    try {
      await contractApi.review(contractId, true, '', token!);
      message.success(t("contract_detail.review_passed", language));
      loadContract();
    } catch (error: any) {
      message.error(error.message || t("contract_detail.review_failed", language));
    } finally {
      setActionLoading(null);
    }
  };

  const handleApprove = async () => {
    setActionLoading('approve');
    try {
      await contractApi.approve(contractId, token!);
      message.success(t("contract_detail.approval_passed", language));
      loadContract();
    } catch (error: any) {
      message.error(error.message || t("contract_detail.approve_failed", language));
    } finally {
      setActionLoading(null);
    }
  };

  const handleRejectSubmit = async () => {
    if (!rejectReason.trim()) {
      message.warning(t("contract_detail.please_enter_reason", language));
      return;
    }

    const loadingKey = rejectModalType === 'review' ? 'review_reject' : 'approve_reject';
    setActionLoading(loadingKey);
    try {
      if (rejectModalType === 'review') {
        await contractApi.review(contractId, false, rejectReason, token!);
        message.success(t("contract_detail.returned_to_editor", language));
      } else {
        await contractApi.reject(contractId, rejectReason, token!);
        message.success(t("contract_detail.rejected", language));
      }
      setRejectModalOpen(false);
      setRejectReason('');
      loadContract();
    } catch (error: any) {
      message.error(error.message || t("contract_detail.operation_failed", language));
    } finally {
      setActionLoading(null);
    }
  };

  const handleCalculate = async () => {
    setCalcLoading(true);
    try {
      const discountRate = contract?.discount_rate_value ?? 0.05;
      const data = await contractApi.calculate(
        contractId,
        discountRate,
        token!
      );
      setCalcResult(data);
      message.success(t("contract_detail.ifrs16_calculated", language));
      setActiveTab("calculation");
    } catch (error: any) {
      message.error(error.message || t("contract_detail.calculate_failed", language));
    } finally {
      setCalcLoading(false);
    }
  };

  const handleEditOpen = () => {
    if (!contract) return;
    editForm.setFieldsValue({
      contract_number: contract.contract_number,
      contract_name: contract.contract_name,
      lessee_name: contract.lessee_name,
      lessor_name: contract.lessor_name,
      store_name: contract.store_name,
      store_address: contract.store_address,
      currency: contract.currency,
      signing_date: contract.signing_date ? dayjs(contract.signing_date) : null,
      commencement_date: dayjs(contract.commencement_date),
      lease_start_date: dayjs(contract.lease_start_date),
      lease_end_date: dayjs(contract.lease_end_date),
      discount_rate_type: contract.discount_rate_type || "",
      discount_rate_version: contract.discount_rate_version || "",
      discount_rate_value: contract.discount_rate_value ?? null,
      tags: parseTagString(contract.tags || ""),
    });
    setEditModalOpen(true);
  };

  const handleUpdate = async (values: any) => {
    setEditLoading(true);
    try {
      const payload: Record<string, any> = {
        contract_number: values.contract_number,
        contract_name: values.contract_name,
        lessee_name: values.lessee_name,
        lessor_name: values.lessor_name,
        store_name: values.store_name,
        store_address: values.store_address,
        currency: values.currency,
        commencement_date: values.commencement_date.format("YYYY-MM-DD"),
        lease_start_date: values.lease_start_date.format("YYYY-MM-DD"),
        lease_end_date: values.lease_end_date.format("YYYY-MM-DD"),
        discount_rate_type: values.discount_rate_type || null,
        discount_rate_version: values.discount_rate_version || null,
        discount_rate_value: values.discount_rate_value ?? null,
        tags: normalizeTagValues(values.tags),
      };
      if (values.signing_date) {
        payload.signing_date = values.signing_date.format("YYYY-MM-DD");
      } else {
        payload.signing_date = null;
      }
      await contractApi.update(contractId, payload, token!);
      message.success(t("contract_detail.contract_updated", language));
      setEditModalOpen(false);
      loadContract();
    } catch (error: any) {
      message.error(error.message || t("contract_detail.update_failed", language));
    } finally {
      setEditLoading(false);
    }
  };

  const handleAiUpload = async (file: File) => {
    setUploadLoading(true);
    try {
      const formData = new FormData();
      formData.append("file", file);
      formData.append("task_type", "payment_schedule");

      const uploadRes = await aiApi.upload(formData);

      const parseRes = await aiApi.parsePaymentSchedule({
        file_id: uploadRes.file_id,
        object_name: uploadRes.object_name,
        content_type: uploadRes.content_type,
      });

      setAiDrafts(parseRes.schedules || []);
      setAiWarnings(parseRes.warnings || []);
      setShowDraftPanel(true);

      if (parseRes.requires_human_confirmation) {
        message.warning(t("contract_detail.ai_parse_warning", language));
      } else {
        message.success(t("contract_detail.ai_parse_success", language, { count: String(parseRes.schedules?.length || 0) }));
      }
    } catch (error: any) {
      message.error(error.message || t("contract_detail.ai_parse_failed", language));
    } finally {
      setUploadLoading(false);
    }
    return false; // prevent default upload
  };

  const handleImportDrafts = async () => {
    const confirmedDrafts = aiDrafts.filter((d) => d.confirmed && !d.skipped);
    if (confirmedDrafts.length === 0) {
      message.warning(t("contract_detail.no_confirmed_drafts", language));
      return;
    }
    setImportLoading(true);
    try {
      let successCount = 0;
      for (const draft of confirmedDrafts) {
        await paymentScheduleApi.create(contractId, {
          contract_id: contractId,
          effective_start_date: draft.period_start || draft.due_date,
          effective_end_date: draft.period_end || draft.due_date,
          coverage_start_date: draft.period_start || draft.due_date,
          coverage_end_date: draft.period_end || draft.due_date,
          due_date: draft.due_date,
          payment_timing: draft.payment_timing,
          amount: draft.amount,
          currency: draft.currency || "CNY",
          amount_type: draft.amount_type || "fixed_rent",
          is_fixed: draft.is_fixed,
          is_lease_component: draft.is_lease_component,
          included_in_liability_pv: draft.is_lease_component,
        }, token!);
        successCount++;
      }
      message.success(t("contract_detail.import_success", language, { count: String(successCount) }));
      setShowDraftPanel(false);
      setAiDrafts([]);
      loadSchedules();
    } catch (error: any) {
      message.error(error.message || t("contract_detail.import_failed", language));
    } finally {
      setImportLoading(false);
    }
  };

  const handleOpenEditDraft = (index: number) => {
    const draft = aiDrafts[index];
    editDraftForm.setFieldsValue({
      due_date: draft.due_date ? dayjs(draft.due_date) : null,
      amount: draft.amount,
      payment_timing: draft.payment_timing || "postpaid",
      amount_type: draft.amount_type || "",
      is_fixed: draft.is_fixed ?? true,
      is_lease_component: draft.is_lease_component ?? true,
    });
    setEditDraftIndex(index);
    setEditDraftModalOpen(true);
  };

  const handleSaveEditDraft = (values: any) => {
    const updated = [...aiDrafts];
    updated[editDraftIndex] = {
      ...updated[editDraftIndex],
      due_date: values.due_date?.format("YYYY-MM-DD"),
      amount: values.amount,
      payment_timing: values.payment_timing,
      amount_type: values.amount_type,
      is_fixed: values.is_fixed,
      is_lease_component: values.is_lease_component,
    };
    setAiDrafts(updated);
    setEditDraftModalOpen(false);
    message.success(t("contract_detail.draft_updated", language));
  };

  const handleConfirmDraft = (index: number) => {
    const updated = [...aiDrafts];
    updated[index] = { ...updated[index], confirmed: true };
    setAiDrafts(updated);
  };

  const handleSkipDraft = (index: number) => {
    const updated = [...aiDrafts];
    updated[index] = { ...updated[index], skipped: true, confirmed: false };
    setAiDrafts(updated);
  };

  const handleRestoreDraft = (index: number) => {
    const updated = [...aiDrafts];
    updated[index] = { ...updated[index], skipped: false };
    setAiDrafts(updated);
  };

  const handleConfirmAll = () => {
    const updated = aiDrafts.map((d) =>
      d.skipped ? d : { ...d, confirmed: true }
    );
    setAiDrafts(updated);
    message.success(t("contract_detail.confirmed_all", language));
  };

  const handleCreateSchedule = async (values: any) => {
    try {
      const payload = {
        contract_id: contractId,
        effective_start_date: values.effective_start_date?.format("YYYY-MM-DD"),
        effective_end_date: values.effective_end_date?.format("YYYY-MM-DD"),
        coverage_start_date: values.coverage_start_date?.format("YYYY-MM-DD"),
        coverage_end_date: values.coverage_end_date?.format("YYYY-MM-DD"),
        due_date: values.due_date?.format("YYYY-MM-DD"),
        payment_timing: values.payment_timing,
        amount: values.amount,
        currency: values.currency || "CNY",
        amount_type: values.amount_type,
        is_fixed: values.is_fixed ?? true,
        is_lease_component: values.is_lease_component ?? true,
        included_in_liability_pv: values.included_in_liability_pv ?? true,
      };
      await paymentScheduleApi.create(contractId, payload, token!);
      message.success(t("contract_detail.schedule_created", language));
      setModalOpen(false);
      form.resetFields();
      loadSchedules();
    } catch (error: any) {
      message.error(error.message || t("contract_detail.create_schedule_failed", language));
    }
  };

  const statusColors: Record<string, string> = {
    draft: "default",
    submitted: "processing",
    reviewed: "warning",
    pending_approval: "orange",
    approved: "success",
    rejected: "error",
  };

  const statusLabels: Record<string, string> = {
    draft: t("status.draft", language),
    submitted: t("status.submitted", language),
    reviewed: t("status.reviewed", language),
    pending_approval: t("status.pending_approval", language),
    approved: t("status.approved", language),
    rejected: t("status.rejected", language),
  };

  const eventTypeLabels: Record<string, string> = {
    area_adjustment: t("contract.event_type.area_adjustment", language),
    rent_change: t("contract.event_type.rent_change", language),
    renewal: t("contract.event_type.renewal", language),
    early_termination: t("contract.event_type.early_termination", language),
    index_update: t("contract.event_type.index_update", language),
    discount_rate_change: t("contract.event_type.discount_rate_change", language),
    impairment: t("contract.event_type.impairment", language),
  };

  const scheduleColumns = [
    { title: t("contract.payment_date", language), dataIndex: "due_date", width: 110 },
    {
      title: t("contract.amount", language),
      dataIndex: "amount",
      width: 120,
      render: (v: number, r: PaymentSchedule) =>
        `¥${v.toLocaleString()}`,
    },
    { title: t("contract.currency", language), dataIndex: "currency", width: 80 },
    {
      title: t("contract.payment_timing", language),
      dataIndex: "payment_timing",
      width: 90,
      render: (v: string) =>
        v === "prepaid" ? (
          <Tag color="processing">{t("contract.prepaid", language)}</Tag>
        ) : (
          <Tag color="success">{t("contract.postpaid", language)}</Tag>
        ),
    },
    { title: t("contract.amount_type", language), dataIndex: "amount_type", width: 120 },
    {
      title: `${t("contract.fixed", language)}/${t("contract.variable", language)}`,
      width: 100,
      render: (_: any, r: PaymentSchedule) => (
        <Space>
          {r.is_fixed && <Tag>{t("contract.fixed", language)}</Tag>}
          {r.is_variable && <Tag color="warning">{t("contract.variable", language)}</Tag>}
        </Space>
      ),
    },
    {
      title: t("contract.lease_component", language),
      width: 90,
      render: (_: any, r: PaymentSchedule) =>
        r.is_lease_component ? <Tag color="success">{t("contract.yes", language)}</Tag> : <Tag>{t("contract.no", language)}</Tag>,
    },
    {
      title: t("contract.include_liability", language),
      width: 90,
      render: (_: any, r: PaymentSchedule) =>
        r.included_in_liability_pv ? (
          <Tag color="success">{t("contract.yes", language)}</Tag>
        ) : (
          <Tag>{t("contract.no", language)}</Tag>
        ),
    },
  ];

  const calcColumns = [
    {
      title: t("contract.period", language),
      render: (_: any, r: MonthlyEntry) => `${r.Year}-${String(r.Month).padStart(2, "0")}`,
      width: 90,
    },
    {
      title: t("contract.opening_liability", language),
      dataIndex: "OpeningLiability",
      render: (v: number) => `¥${v.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right" as const,
    },
    {
      title: t("contract.interest_expense", language),
      dataIndex: "InterestExpense",
      render: (v: number) => `¥${v.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right" as const,
    },
    {
      title: t("contract.payment", language),
      dataIndex: "TotalPayments",
      render: (v: number) => `¥${v.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right" as const,
    },
    {
      title: t("contract.closing_liability", language),
      dataIndex: "ClosingLiability",
      render: (v: number) => `¥${v.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right" as const,
    },
    {
      title: t("contract.opening_rou", language),
      dataIndex: "OpeningROUAsset",
      render: (v: number) => `¥${v.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right" as const,
    },
    {
      title: t("contract.depreciation", language),
      dataIndex: "Depreciation",
      render: (v: number) => `¥${v.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right" as const,
    },
    {
      title: t("contract.closing_rou", language),
      dataIndex: "ClosingROUAsset",
      render: (v: number) => `¥${v.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right" as const,
    },
    {
      title: t("contract.variable_rent", language),
      dataIndex: "VariableRentExpense",
      render: (v: number) => `¥${v.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right" as const,
    },
    {
      title: t("contract.non_lease_expense", language),
      dataIndex: "NonLeaseExpense",
      render: (v: number) => `¥${v.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right" as const,
    },
  ];

  const sortedMonthly = calcResult
    ? [...calcResult.monthly_summary].sort(
        (a, b) => a.Year * 100 + a.Month - (b.Year * 100 + b.Month)
      )
    : [];

  return (
    <ProtectedRoute>
      <AppLayout>
        {/* Page Header */}
        <div style={{ marginBottom: 24 }}>
          <Button
            type="text"
            icon={<ArrowLeftOutlined />}
            onClick={() => router.push("/contracts")}
            style={{ marginBottom: 8, paddingLeft: 0 }}
          >
            {t("contract.back_to_list", language)}
          </Button>
          <h1 style={{ fontSize: 28, fontWeight: 700, letterSpacing: '-0.04em', margin: 0, lineHeight: 1.3 }}>
            {contract?.contract_name || t("contract.detail_title", language)}
          </h1>
          {contract && (
            <div style={{ fontSize: 14, color: '#8C8C8C', marginTop: 4 }}>
              {contract.contract_number} ·{" "}
              <Tag color={statusColors[contract.approval_status]}>
                {statusLabels[contract.approval_status]}
              </Tag>
            </div>
          )}
        </div>

        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.4, ease: "easeOut" }}
        >
          <Spin spinning={loading}>
            {contract && (
            <>
              <Row gutter={[16, 16]}>
                <Col span={18}>
                  <Card
                    title={
                      <Space>
                        <span>{contract.contract_name}</span>
                        <Tag color={statusColors[contract.approval_status]}>
                          {statusLabels[contract.approval_status]}
                        </Tag>
                        {contract.is_official_version && (
                          <Tag color="processing">{t("contracts.official", language)}</Tag>
                        )}
                      </Space>
                    }
                    extra={
                      <Space>
                        {/* Approval workflow buttons */}
                        {user && contract.approval_status === 'draft' && (user.role === 'editor' || user.role === 'admin') && (
                          <Button
                            type="primary"
                            onClick={handleSubmitForReview}
                            loading={actionLoading === 'submit'}
                          >
                            {t("contract.submit_review", language)}
                          </Button>
                        )}

                        {user && contract.approval_status === 'submitted' && (user.role === 'reviewer' || user.role === 'admin') && (
                          <>
                            <Button
                              type="primary"
                              onClick={handleReviewApprove}
                              loading={actionLoading === 'review_approve'}
                            >
                              {t("contract.review_pass", language)}
                            </Button>
                            <Button
                              danger
                              onClick={() => { setRejectModalType('review'); setRejectReason(''); setRejectModalOpen(true); }}
                            >
                              {t("contract.return_editor", language)}
                            </Button>
                          </>
                        )}

                        {user && (contract.approval_status === 'reviewed' || contract.approval_status === 'pending_approval') && (user.role === 'approver' || user.role === 'admin') && (
                          <>
                            <Button
                              type="primary"
                              onClick={handleApprove}
                              loading={actionLoading === 'approve'}
                            >
                              {t("contract.approve", language)}
                            </Button>
                            <Button
                              danger
                              onClick={() => { setRejectModalType('approve'); setRejectReason(''); setRejectModalOpen(true); }}
                            >
                              {t("contract.reject", language)}
                            </Button>
                          </>
                        )}

                        {user && contract.approval_status === 'rejected' && (user.role === 'editor' || user.role === 'admin') && (
                          <Button
                            type="primary"
                            onClick={handleSubmitForReview}
                            loading={actionLoading === 'submit'}
                          >
                            {t("contract.resubmit", language)}
                          </Button>
                        )}

                        {user && (contract.approval_status === 'draft' || contract.approval_status === 'rejected') && (user.role === 'editor' || user.role === 'admin') && (
                          <Button
                            icon={<EditOutlined />}
                            onClick={handleEditOpen}
                          >
                            {t("contract.edit", language)}
                          </Button>
                        )}

                        <Button
                          icon={<CalculatorOutlined />}
                          onClick={handleCalculate}
                          loading={calcLoading}
                          type="primary"
                        >
                          {t("contract.calculate", language)}
                        </Button>

                      </Space>
                    }
                  >
                    <Descriptions column={3} size="small">
                      <Descriptions.Item label={t("contract.contract_number", language)}>
                        {contract.contract_number}
                      </Descriptions.Item>
                      <Descriptions.Item label={t("contract.currency", language)}>
                        {contract.currency}
                      </Descriptions.Item>
                      <Descriptions.Item label={t("contract.discount_rate", language)}>
                        {contract.discount_rate_value != null ? (
                          <Tag color="success">{(contract.discount_rate_value * 100).toFixed(2)}%</Tag>
                        ) : contract.discount_rate_missing ? (
                          <Tag color="error">{t("contracts.missing", language)}</Tag>
                        ) : (
                          <Tag color="success">
                            {contract.discount_rate_type} / {contract.discount_rate_version}
                          </Tag>
                        )}
                      </Descriptions.Item>
                      <Descriptions.Item label={t("contract.commencement_date", language)}>
                        {dayjs(contract.commencement_date).format("YYYY-MM-DD")}
                      </Descriptions.Item>
                      <Descriptions.Item label={t("contract.lease_start_date", language)}>
                        {dayjs(contract.lease_start_date).format("YYYY-MM-DD")}
                      </Descriptions.Item>
                      <Descriptions.Item label={t("contract.lease_end_date", language)}>
                        {dayjs(contract.lease_end_date).format("YYYY-MM-DD")}
                      </Descriptions.Item>
                    </Descriptions>
                  </Card>
                </Col>

                <Col span={6}>
                  <Card title={t("contract.approval_progress", language)} size="small">
                    <Timeline
                      items={[
                        {
                          dot: <ClockCircleOutlined />,
                          color: contract.created_at ? "#000000" : "#D9D9D9",
                          children: `${t("contract.created", language)} ${dayjs(contract.created_at).format("YYYY-MM-DD")}`,
                        },
                        {
                          dot: <FileTextOutlined />,
                          color: contract.submitted_at ? "#000000" : "#D9D9D9",
                          children: contract.submitted_at
                            ? `${t("contract.submitted", language)} ${dayjs(contract.submitted_at).format("YYYY-MM-DD")}`
                            : `${t("contract.pending", language)}${t("contract.submitted", language)}`,
                        },
                        {
                          dot: <CheckCircleOutlined />,
                          color: contract.reviewed_at ? "#000000" : "#D9D9D9",
                          children: contract.reviewed_at
                            ? `${t("contract.reviewed", language)} ${dayjs(contract.reviewed_at).format("YYYY-MM-DD")}`
                            : `${t("contract.pending", language)}${t("contract.reviewed", language)}`,
                        },
                        {
                          dot: <CheckCircleOutlined />,
                          color: contract.approved_at ? "#000000" : "#D9D9D9",
                          children: contract.approved_at
                            ? `${t("contract.approved", language)} ${dayjs(contract.approved_at).format("YYYY-MM-DD")}`
                            : `${t("contract.pending", language)}${t("contract.approved", language)}`,
                        },
                      ]}
                    />
                  </Card>
                </Col>
              </Row>

              <Tabs
                activeKey={activeTab}
                onChange={setActiveTab}
                style={{ marginTop: 16 }}
                items={[
                  {
                    key: "info",
                    label: t("contract.tab_info", language),
                    children: (
                      <Card>
                        <Descriptions column={2} bordered>
                          <Descriptions.Item label={t("contract.contract_id", language)}>
                            {contract.id}
                          </Descriptions.Item>
                          <Descriptions.Item label={t("contract.legal_entity_id", language)}>
                            {contract.legal_entity_id}
                          </Descriptions.Item>
                          <Descriptions.Item label={t("contract.store_id", language)}>
                            {contract.store_id}
                          </Descriptions.Item>
                          <Descriptions.Item label={t("contract.landlord_id", language)}>
                            {contract.landlord_id}
                          </Descriptions.Item>
                          <Descriptions.Item label={t("contract.created_at", language)}>
                            {dayjs(contract.created_at).format("YYYY-MM-DD HH:mm")}
                          </Descriptions.Item>
                          {contract.rejected_reason && (
                            <Descriptions.Item label={t("contract.rejected_reason", language)}>
                              {contract.rejected_reason}
                            </Descriptions.Item>
                          )}
                        </Descriptions>
                      </Card>
                    ),
                  },
                  {
                    key: "payments",
                    label: `${t("contract.tab_payments", language)} (${schedules.length})`,
                    children: (
                  <Card
                    title={
                      <Space>
                        <span>{t("contract.tab_payments", language)}</span>
                        {schedules.length > 0 && (
                          <Tag color="processing">{schedules.length} {t("contract_detail.item_unit", language)}</Tag>
                        )}
                      </Space>
                    }
                    extra={
                      <Space>
                        <Upload
                          accept=".pdf,.xlsx,.xls"
                          showUploadList={false}
                          beforeUpload={handleAiUpload}
                          disabled={uploadLoading}
                        >
                          <Button
                            icon={<RobotOutlined />}
                            loading={uploadLoading}
                          >
                            {t("contract.ai_parse", language)}
                          </Button>
                        </Upload>
                        <Button
                          type="primary"
                          icon={<PlusOutlined />}
                          onClick={() => setModalOpen(true)}
                        >
                          {t("contract.manual_add", language)}
                        </Button>
                      </Space>
                    }
                  >
                    {/* AI Draft Preview */}
                    {showDraftPanel && aiDrafts.length > 0 && (
                      <div style={{ marginBottom: 16, padding: 16, background: "#F7F7F7", border: "1px solid #EAEAEA", borderRadius: 12 }}>
                        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12, flexWrap: "wrap", gap: 8 }}>
                          <Space>
                            <RobotOutlined style={{ color: "#000" }} />
                            <span style={{ fontWeight: "bold" }}>{t("contract.ai_draft_title", language)}</span>
                            <Tag color="success">
                              {t("contract_detail.ai_draft_count", language, { confirmed: String(aiDrafts.filter((d) => d.confirmed && !d.skipped).length), total: String(aiDrafts.length) })}
                            </Tag>
                          </Space>
                          <Space>
                            <Button size="small" onClick={handleConfirmAll}>
                              {t("contract.confirm_all", language)}
                            </Button>
                            <Button size="small" onClick={() => { setShowDraftPanel(false); setAiDrafts([]); }}>
                              {t("contract.cancel", language)}
                            </Button>
                            <Button
                              type="primary"
                              size="small"
                              icon={<ImportOutlined />}
                              onClick={handleImportDrafts}
                              loading={importLoading}
                            >
                              {t("contract.import_confirmed", language)}
                            </Button>
                          </Space>
                        </div>

                        {aiWarnings.length > 0 && (
                          <Alert
                            message={t("contract.ai_warning", language)}
                            description={
                              <ul style={{ margin: 0, paddingLeft: 16 }}>
                                {aiWarnings.slice(0, 5).map((w, i) => (
                                  <li key={i}><WarningOutlined style={{ color: "#8C8C8C" }} /> {w}</li>
                                ))}
                                {aiWarnings.length > 5 && <li>{t("contract_detail.ai_warning_more", language, { count: String(aiWarnings.length - 5) })}</li>}
                              </ul>
                            }
                            type="warning"
                            showIcon
                            style={{ marginBottom: 12 }}
                          />
                        )}

                        <Table
                          columns={[
                            { title: t("contract.payment_date", language), dataIndex: "due_date", width: 110 },
                            { title: t("contract.amount", language), dataIndex: "amount", width: 100, render: (v: number) => `¥${v.toLocaleString()}` },
                            { title: t("contract.payment_timing", language), dataIndex: "payment_timing", width: 80, render: (v: string) => v === "prepaid" ? <Tag color="processing">{t("contract.prepaid", language)}</Tag> : <Tag color="success">{t("contract.postpaid", language)}</Tag> },
                            { title: t("contract.amount_type", language), dataIndex: "amount_type", width: 100 },
                            { title: t("contract.fixed", language), dataIndex: "is_fixed", width: 60, render: (v: boolean) => v ? t("contract.yes", language) : t("contract.no", language) },
                            { title: t("contract.lease_component", language), dataIndex: "is_lease_component", width: 60, render: (v: boolean) => v ? t("contract.yes", language) : t("contract.no", language) },
                            {
                              title: t("contract.confidence", language),
                              dataIndex: "confidence",
                              width: 80,
                              render: (v: number) => (
                                <Tag color={v >= 0.9 ? "success" : v >= 0.8 ? "warning" : "error"}>
                                  {(v * 100).toFixed(0)}%
                                </Tag>
                              ),
                            },
                            {
                              title: t("contract.action", language),
                              key: "action",
                              width: 210,
                              render: (_: any, record: any, index: number) => (
                                <Space size="small">
                                  <Button
                                    size="small"
                                    icon={<EditOutlined />}
                                    onClick={() => handleOpenEditDraft(index)}
                                  >
                                    {t("contract.edit", language)}
                                  </Button>
                                  {!record.skipped && !record.confirmed && (
                                    <>
                                      <Button
                                        size="small"
                                        icon={<CheckOutlined />}
                                        style={{ color: "#000", borderColor: "#000" }}
                                        onClick={() => handleConfirmDraft(index)}
                                      >
                                        {t("contract.ok", language)}
                                      </Button>
                                      <Button
                                        size="small"
                                        icon={<StopOutlined />}
                                        danger
                                        onClick={() => handleSkipDraft(index)}
                                      >
                                        {t("contract.skip", language)}
                                      </Button>
                                    </>
                                  )}
                                  {record.confirmed && (
                                    <Tag color="success" icon={<CheckCircleOutlined />}>{t("contract.confirmed", language)}</Tag>
                                  )}
                                  {record.skipped && (
                                    <Button
                                      size="small"
                                      icon={<UndoOutlined />}
                                      onClick={() => handleRestoreDraft(index)}
                                    >
                                      {t("contract.restore", language)}
                                    </Button>
                                  )}
                                </Space>
                              ),
                            },
                          ]}
                          dataSource={aiDrafts}
                          rowKey={(r, i) => `draft-${i}`}
                          pagination={false}
                          size="small"
                          scroll={{ x: 850 }}
                          onRow={(record: any) => ({
                            style: {
                              backgroundColor: record.confirmed
                                ? "#F7F7F7"
                                : record.skipped
                                ? "#F7F7F7"
                                : undefined,
                              textDecoration: record.skipped ? "line-through" : undefined,
                              opacity: record.skipped ? 0.6 : undefined,
                            },
                          })}
                        />
                      </div>
                    )}

                    <Table
                      columns={scheduleColumns}
                      dataSource={schedules}
                      rowKey="id"
                      pagination={{ pageSize: 12 }}
                      size="small"
                      scroll={{ x: 900 }}
                      locale={{ emptyText: t("contract.no_schedules", language) }}
                    />
                  </Card>
                    ),
                  },
                  {
                    key: "events",
                    label: `${t("contract.tab_events", language)} (${events.length})`,
                    children: (
                      <Card
                        title={
                          <Space>
                            <span>{t("contract.tab_events", language)}</span>
                            {events.length > 0 && <Tag color="processing">{events.length}</Tag>}
                          </Space>
                        }
                        extra={
                          <Button type="primary" icon={<PlusOutlined />} onClick={() => setEventModalOpen(true)}>
                            {t("contract.register_event", language)}
                          </Button>
                        }
                      >
                        <Table
                          columns={[
                            { title: t("contract.tab_events", language), dataIndex: "event_type", width: 150, render: (v: string) => eventTypeLabels[v] || v },
                            { title: t("contract.effective_date", language), dataIndex: "effective_date", width: 110, render: (v: string) => dayjs(v).format("YYYY-MM-DD") },
                            { title: t("contract.original_value", language), dataIndex: "original_value", width: 120 },
                            { title: t("contract.new_value", language), dataIndex: "new_value", width: 120 },
                            { title: t("contract.change_reason", language), dataIndex: "change_reason", ellipsis: true },
                            {
                              title: t("contracts.col_status", language),
                              dataIndex: "approval_status",
                              width: 100,
                              render: (v: string) => {
                                const eventStatusColors: Record<string, string> = {
                                  draft: "default",
                                  submitted: "processing",
                                  reviewed: "warning",
                                  approved: "success",
                                  rejected: "error",
                                  returned_to_editor: "orange",
                                };
                                const eventStatusLabels: Record<string, string> = {
                                  draft: t("status.draft", language),
                                  submitted: t("status.submitted", language),
                                  reviewed: t("status.reviewed", language),
                                  approved: t("status.approved", language),
                                  rejected: t("status.rejected", language),
                                  returned_to_editor: t("status.returned_to_editor", language),
                                };
                                return (
                                  <Tag color={eventStatusColors[v] || "default"}>
                                    {eventStatusLabels[v] || v}
                                  </Tag>
                                );
                              },
                            },
                            { title: t("contract.created_at", language), dataIndex: "created_at", width: 150, render: (v: string) => dayjs(v).format("YYYY-MM-DD HH:mm") },
                            {
                              title: t("contract.action", language),
                              key: "action",
                              width: 280,
                              render: (_: any, event: any) => {
                                const isModifiable = ["area_adjustment", "rent_change", "renewal", "early_termination", "index_update", "discount_rate_change", "impairment"].includes(event.event_type);
                                return (
                                <Space size="small">
                                  {event.approval_status === "approved" && isModifiable && (
                                    <Button size="small" onClick={() => handleViewAdjustment(event.id)}>{t("contract.view_adjustment", language)}</Button>
                                  )}
                                  {(event.approval_status === "submitted" || event.approval_status === "reviewed") && isModifiable && (
                                    <Button size="small" onClick={() => handlePreviewAdjustment(event.id)} loading={previewLoading}>{t("contract.preview_impact", language)}</Button>
                                  )}
                                  {event.approval_status === "draft" && isModifiable && (user?.role === "editor" || user?.role === "admin") && (
                                    <Button size="small" onClick={() => handleRecalculateEvent(event.id)}>{t("contract.recalculate", language)}</Button>
                                  )}
                                  {user && event.approval_status === 'draft' && (user.role === 'editor' || user.role === 'admin') && (
                                    <Button
                                      size="small"
                                      type="primary"
                                      onClick={() => handleEventSubmitForReview(event.id)}
                                      loading={eventActionLoading === event.id + '_submit'}
                                    >
                                      {t("contract.submit_review", language)}
                                    </Button>
                                  )}
                                  {user && event.approval_status === 'submitted' && (user.role === 'reviewer' || user.role === 'admin') && (
                                    <>
                                      <Button
                                        size="small"
                                        type="primary"
                                        onClick={() => handleEventReviewApprove(event.id)}
                                        loading={eventActionLoading === event.id + '_review'}
                                      >
                                        {t("contract.review_pass", language)}
                                      </Button>
                                      <Button
                                        size="small"
                                        danger
                                        onClick={() => handleEventRejectOpen(event.id, 'review')}
                                      >
                                        {t("contract.return_editor", language)}
                                      </Button>
                                    </>
                                  )}
                                  {user && event.approval_status === 'reviewed' && (user.role === 'approver' || user.role === 'admin') && (
                                    <>
                                      <Button
                                        size="small"
                                        type="primary"
                                        onClick={() => handleEventApprove(event.id)}
                                        loading={eventActionLoading === event.id + '_approve'}
                                      >
                                        {t("contract.approve", language)}
                                      </Button>
                                      <Button
                                        size="small"
                                        danger
                                        onClick={() => handleEventRejectOpen(event.id, 'approve')}
                                      >
                                        {t("contract.reject", language)}
                                      </Button>
                                    </>
                                  )}
                                  {user && event.approval_status === 'rejected' && (user.role === 'editor' || user.role === 'admin') && (
                                    <Button
                                      size="small"
                                      type="primary"
                                      onClick={() => handleEventSubmitForReview(event.id)}
                                      loading={eventActionLoading === event.id + '_submit'}
                                    >
                                      {t("contract.resubmit", language)}
                                    </Button>
                                  )}
                                </Space>
                                );
                              },
                            },
                          ]}
                          dataSource={events}
                          rowKey="id"
                          pagination={{ pageSize: 10 }}
                          size="small"
                          scroll={{ x: 1000 }}
                          locale={{ emptyText: t("contract.no_events", language) }}
                        />
                      </Card>
                    ),
                  },
                  {
                    key: "calculation",
                    label: t("contract.tab_calculation", language),
                    children: calcResult ? (
                      <>
                        <Row gutter={16} style={{ marginBottom: 16 }}>
                          <Col span={8}>
                            <Card>
                              <Statistic
                                title={t("contract.initial_liability", language)}
                                value={calcResult.initial_liability}
                                precision={2}
                                prefix="¥"
                              />
                            </Card>
                          </Col>
                          <Col span={8}>
                            <Card>
                              <Statistic
                                title={t("contract.initial_rou", language)}
                                value={calcResult.initial_rou_asset}
                                precision={2}
                                prefix="¥"
                              />
                            </Card>
                          </Col>
                          <Col span={8}>
                            <Card>
                              <Statistic
                                title={t("contract.total_days", language)}
                                value={calcResult.total_days}
                                suffix={t("contract_detail.days_unit", language)}
                              />
                            </Card>
                          </Col>
                        </Row>
                        <Card title={t("contract.monthly_amortization", language)}>
                          <Table
                            columns={calcColumns}
                            dataSource={sortedMonthly}
                            rowKey={(r: MonthlyEntry) => `${r.Year}-${r.Month}`}
                            pagination={{ pageSize: 12 }}
                            size="small"
                            scroll={{ x: 1000 }}
                          />
                        </Card>
                      </>
                    ) : (
                      <Card>
                        <div style={{ textAlign: "center", padding: 40 }}>
                          <CalculatorOutlined style={{ fontSize: 48, color: "#BFBFBF" }} />
                          <p style={{ marginTop: 16, color: "#8C8C8C" }}>
                            {t("contract.click_calculate", language)}
                          </p>
                        </div>
                      </Card>
                    ),
                  },
                ]}
              />
            </>
          )}
        </Spin>
        </motion.div>

        <Modal
          title={t("contract.add_payment", language)}
          open={modalOpen}
          onCancel={() => setModalOpen(false)}
          onOk={() => form.submit()}
          width={700}
        >
          <Form
            form={form}
            layout="vertical"
            onFinish={handleCreateSchedule}
            initialValues={{
              currency: "CNY",
              payment_timing: "postpaid",
              amount_type: "fixed_rent",
              is_fixed: true,
              is_lease_component: true,
              included_in_liability_pv: true,
            }}
          >
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  label={t("contract.payment_date", language)}
                  name="due_date"
                  rules={[{ required: true, message: t("contract_detail.validation.payment_date", language) }]}
                >
                  <DatePicker style={{ width: "100%" }} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label={t("contract.amount", language)}
                  name="amount"
                  rules={[{ required: true, message: t("contract_detail.validation.amount", language) }]}
                >
                  <InputNumber
                    style={{ width: "100%" }}
                    min={0}
                    precision={2}
                    prefix="¥"
                  />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label={t("contract.currency", language)} name="currency">
                  <Select>
                    <Select.Option value="CNY">{t("contract.currency_cny", language)}</Select.Option>
                    <Select.Option value="USD">{t("contract.currency_usd", language)}</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label={t("contract.payment_timing_label", language)}
                  name="payment_timing"
                  rules={[{ required: true }]}
                >
                  <Select>
                    <Select.Option value="prepaid">
                      {t("contract.prepaid_label", language)}
                    </Select.Option>
                    <Select.Option value="postpaid">
                      {t("contract.postpaid_label", language)}
                    </Select.Option>
                  </Select>
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  label={t("contract.effective_start_date", language)}
                  name="effective_start_date"
                >
                  <DatePicker style={{ width: "100%" }} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label={t("contract.effective_end_date", language)}
                  name="effective_end_date"
                >
                  <DatePicker style={{ width: "100%" }} />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label={t("contract.lease_end_date_label", language)}
                  name="lease_end_date"
                  rules={[{ required: true, message: t("contract_detail.validation.lease_end_date", language) }]}
                >
                  <DatePicker style={{ width: "100%" }} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label={t("contract.discount_rate_type", language)} name="discount_rate_type">
                  <Input placeholder={t("contract.discount_rate_type_placeholder", language)} />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  label={t("contract.discount_rate_value", language)}
                  name="discount_rate_value"
                  help={t("contract.discount_rate_help", language)}
                >
                  <InputNumber style={{ width: "100%" }} min={0} step={0.01} placeholder={t("contract_detail.discount_rate_placeholder", language)} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label={t("contract.discount_rate_version", language)} name="discount_rate_version">
                  <Input placeholder={t("contract.discount_rate_version_placeholder", language)} />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  label={t("contract.tags", language)}
                  name="tags"
                  tooltip={t("contract.tags_tooltip", language)}
                >
                  <Select
                    mode="tags"
                    tokenSeparators={[",", "，", ";", "；", " ", "|"]}
                    placeholder={t("contract.tags_placeholder", language)}
                    options={DEFAULT_TAG_SUGGESTIONS.map((tag) => ({
                      value: tag,
                      label: tag,
                    }))}
                  />
                </Form.Item>
              </Col>
            </Row>

            <Form.Item label={t("contract.amount_type", language)} name="amount_type">
              <Input placeholder={t("contract.amount_type_placeholder", language)} />
            </Form.Item>

            <Row gutter={16}>
              <Col span={8}>
                <Form.Item
                  label={t("contract.is_fixed", language)}
                  name="is_fixed"
                  valuePropName="checked"
                >
                  <Select>
                    <Select.Option value={true}>{t("contract.yes", language)}</Select.Option>
                    <Select.Option value={false}>{t("contract.no", language)}</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item
                  label={t("contract.is_lease_component", language)}
                  name="is_lease_component"
                  valuePropName="checked"
                >
                  <Select>
                    <Select.Option value={true}>{t("contract.yes", language)}</Select.Option>
                    <Select.Option value={false}>{t("contract.no", language)}</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item
                  label={t("contract.included_in_liability_pv", language)}
                  name="included_in_liability_pv"
                  valuePropName="checked"
                >
                  <Select>
                    <Select.Option value={true}>{t("contract.yes", language)}</Select.Option>
                    <Select.Option value={false}>{t("contract.no", language)}</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
            </Row>
          </Form>
        </Modal>

        <Modal
          title={t("contract.register_event", language)}
          open={eventModalOpen}
          onCancel={() => setEventModalOpen(false)}
          onOk={() => eventForm.submit()}
          confirmLoading={eventLoading}
          width={600}
        >
          <Form
            form={eventForm}
            layout="vertical"
            onFinish={handleCreateEvent}
          >
            <Form.Item
              label={t("contract.tab_events", language)}
              name="event_type"
              rules={[{ required: true, message: t("contract_detail.validation.event_type", language) }]}
            >
              <Select placeholder={t("contract.select_event_type", language)}>
                <Select.Option value="area_adjustment">{t("contract.event_type.area_adjustment", language)}</Select.Option>
                <Select.Option value="rent_change">{t("contract.event_type.rent_change", language)}</Select.Option>
                <Select.Option value="renewal">{t("contract.event_type.renewal", language)}</Select.Option>
                <Select.Option value="early_termination">{t("contract.event_type.early_termination", language)}</Select.Option>
                <Select.Option value="index_update">{t("contract.event_type.index_update", language)}</Select.Option>
                <Select.Option value="discount_rate_change">{t("contract.event_type.discount_rate_change", language)}</Select.Option>
                <Select.Option value="impairment">{t("contract.event_type.impairment", language)}</Select.Option>
              </Select>
            </Form.Item>

            <Form.Item
              label={t("contract.effective_date", language)}
              name="effective_date"
              rules={[{ required: true, message: t("contract_detail.validation.effective_date", language) }]}
            >
              <DatePicker style={{ width: "100%" }} />
            </Form.Item>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label={t("contract.original_value", language)} name="original_value">
                  <Input placeholder={t("contract_detail.original_value_placeholder", language)} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label={t("contract.new_value", language)} name="new_value">
                  <Input placeholder={t("contract_detail.new_value_placeholder", language)} />
                </Form.Item>
              </Col>
            </Row>

            <Form.Item
              label={t("contract.change_reason", language)}
              name="change_reason"
              rules={[{ required: true, message: t("contract_detail.validation.change_reason", language) }]}
            >
              <Input.TextArea rows={2} placeholder={t("contract.change_reason_placeholder", language)} />
            </Form.Item>

            <Form.Item label={t("contract.judgment_basis", language)} name="judgment_basis">
              <Input.TextArea rows={2} placeholder={t("contract.judgment_basis_placeholder", language)} />
            </Form.Item>
          </Form>
        </Modal>

        <Modal
          title={t("contract.edit_contract", language)}
          open={editModalOpen}
          onCancel={() => setEditModalOpen(false)}
          onOk={() => editForm.submit()}
          confirmLoading={editLoading}
          width={700}
        >
          <Form
            form={editForm}
            layout="vertical"
            onFinish={handleUpdate}
          >
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  label={t("contract.contract_number", language)}
                  name="contract_number"
                  rules={[{ required: true, message: t("contract_detail.validation.contract_number", language) }]}
                >
                  <Input />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label={t("contracts.col_name", language)}
                  name="contract_name"
                  rules={[{ required: true, message: t("contract_detail.validation.contract_name", language) }]}
                >
                  <Input />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label={t("contract.lessee", language)} name="lessee_name">
                  <Input />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label={t("contract.lessor", language)} name="lessor_name">
                  <Input />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label={t("contract.store_name", language)} name="store_name">
                  <Input />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label={t("contract.store_address", language)} name="store_address">
                  <Input />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  label={t("contract.currency", language)}
                  name="currency"
                  rules={[{ required: true, message: t("contract_detail.validation.currency", language) }]}
                >
                  <Select>
                    <Select.Option value="CNY">{t("contract.currency_cny", language)}</Select.Option>
                    <Select.Option value="USD">{t("contract.currency_usd", language)}</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label={t("contract.signing_date", language)} name="signing_date">
                  <DatePicker style={{ width: "100%" }} />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  label={t("contract.commencement_date", language)}
                  name="commencement_date"
                  rules={[{ required: true, message: t("contract_detail.validation.commencement_date", language) }]}
                >
                  <DatePicker style={{ width: "100%" }} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label={t("contract.lease_start_date", language)}
                  name="lease_start_date"
                  rules={[{ required: true, message: t("contract_detail.validation.lease_start_date", language) }]}
                >
                  <DatePicker style={{ width: "100%" }} />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  label={t("contract.lease_end_date_label", language)}
                  name="lease_end_date"
                  rules={[{ required: true, message: t("contract_detail.validation.lease_end_date", language) }]}
                >
                  <DatePicker style={{ width: "100%" }} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label={t("contract.discount_rate_type", language)} name="discount_rate_type">
                  <Input placeholder={t("contract.discount_rate_type_placeholder", language)} />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label={t("contract.discount_rate_version", language)} name="discount_rate_version">
                  <Input placeholder={t("contract.discount_rate_version_placeholder", language)} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label={t("contract.tags", language)}
                  name="tags"
                  tooltip={t("contract.tags_tooltip", language)}
                >
                  <Select
                    mode="tags"
                    tokenSeparators={[",", "，", ";", "；", " ", "|"]}
                    placeholder={t("contract.tags_placeholder", language)}
                    options={DEFAULT_TAG_SUGGESTIONS.map((tag) => ({
                      value: tag,
                      label: tag,
                    }))}
                  />
                </Form.Item>
              </Col>
            </Row>
          </Form>
        </Modal>

        <Modal
          title={rejectModalType === 'review' ? t("contract.review_reject_title", language) : t("contract.approve_reject_title", language)}
          open={rejectModalOpen}
          onCancel={() => setRejectModalOpen(false)}
          onOk={handleRejectSubmit}
          confirmLoading={actionLoading === (rejectModalType === 'review' ? 'review_reject' : 'approve_reject')}
          okText={t("contract.ok", language)}
          cancelText={t("contract.cancel", language)}
        >
          <p style={{ marginBottom: 8 }}>
            {rejectModalType === 'review' ? t("contract.review_reject_reason", language) : t("contract.approve_reject_reason", language)}
          </p>
          <Input.TextArea
            rows={4}
            value={rejectReason}
            onChange={(e) => setRejectReason(e.target.value)}
            placeholder={t("contract.reason_placeholder", language)}
          />
        </Modal>

        <Modal
          title={eventRejectType === 'review' ? t("contract.review_reject_title", language) : t("contract.approve_reject_title", language)}
          open={eventRejectModalOpen}
          onCancel={() => setEventRejectModalOpen(false)}
          onOk={handleEventRejectSubmit}
          confirmLoading={eventRejectEventId ? eventActionLoading === eventRejectEventId + '_reject' : false}
          okText={t("contract.ok", language)}
          cancelText={t("contract.cancel", language)}
        >
          <p style={{ marginBottom: 8 }}>
            {eventRejectType === 'review' ? t("contract.review_reject_reason", language) : t("contract.approve_reject_reason", language)}
          </p>
          <Input.TextArea
            rows={4}
            value={eventRejectReason}
            onChange={(e) => setEventRejectReason(e.target.value)}
            placeholder={t("contract.reason_placeholder", language)}
          />
        </Modal>

        <Modal
          title={t("contract.edit_draft", language)}
          open={editDraftModalOpen}
          onCancel={() => setEditDraftModalOpen(false)}
          onOk={() => editDraftForm.submit()}
          width={500}
        >
          <Form
            form={editDraftForm}
            layout="vertical"
            onFinish={handleSaveEditDraft}
          >
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  label={t("contract.payment_date", language)}
                  name="due_date"
                  rules={[{ required: true, message: t("contract_detail.validation.payment_date", language) }]}
                >
                  <DatePicker style={{ width: "100%" }} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label={t("contract.amount", language)}
                  name="amount"
                  rules={[{ required: true, message: t("contract_detail.validation.amount", language) }]}
                >
                  <InputNumber
                    style={{ width: "100%" }}
                    min={0}
                    precision={2}
                    prefix="¥"
                  />
                </Form.Item>
              </Col>
            </Row>

            <Form.Item
              label={t("contract.payment_timing_label", language)}
              name="payment_timing"
              rules={[{ required: true }]}
            >
              <Select>
                <Select.Option value="prepaid">{t("contract.prepaid_label", language)}</Select.Option>
                <Select.Option value="postpaid">{t("contract.postpaid_label", language)}</Select.Option>
              </Select>
            </Form.Item>

            <Form.Item label={t("contract.amount_type", language)} name="amount_type">
              <Input placeholder={t("contract.amount_type_placeholder", language)} />
            </Form.Item>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label={t("contract.is_fixed", language)} name="is_fixed">
                  <Select>
                    <Select.Option value={true}>{t("contract.yes", language)}</Select.Option>
                    <Select.Option value={false}>{t("contract.no", language)}</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label={t("contract.is_lease_component", language)} name="is_lease_component">
                  <Select>
                    <Select.Option value={true}>{t("contract.yes", language)}</Select.Option>
                    <Select.Option value={false}>{t("contract.no", language)}</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
            </Row>
          </Form>
        </Modal>

        <Modal
          title={adjustmentModalTitle}
          open={adjustmentModalOpen}
          onCancel={() => setAdjustmentModalOpen(false)}
          footer={[
            <Button key="close" onClick={() => setAdjustmentModalOpen(false)}>{t("contract.close", language)}</Button>,
          ]}
          width={700}
        >
          {adjustmentModalData && (
            <Descriptions column={1} bordered size="small">
              <Descriptions.Item label={t("contract_detail.adjustment_type_label", language)}>
                <Tag color="processing">{adjustmentModalData.adjustment_type || "-"}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label={t("contract.effective_date", language)}>
                {adjustmentModalData.effective_date ? dayjs(adjustmentModalData.effective_date).format("YYYY-MM-DD") : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract.discount_rate", language)}>
                {adjustmentModalData.discount_rate != null ? `${(adjustmentModalData.discount_rate * 100).toFixed(2)}%` : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract_detail.liability_before", language)}>
                {adjustmentModalData.liability_before != null ? `¥${adjustmentModalData.liability_before.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}` : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract_detail.liability_after", language)}>
                {adjustmentModalData.liability_after != null ? (
                  <span style={{ fontWeight: "bold", color: "#000" }}>¥{adjustmentModalData.liability_after.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}</span>
                ) : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract_detail.liability_change", language)}>
                {adjustmentModalData.liability_change != null ? (
                  <span style={{ fontWeight: "bold", color: adjustmentModalData.liability_change >= 0 ? "#000" : "#000" }}>
                    {adjustmentModalData.liability_change >= 0 ? "+" : ""}¥{adjustmentModalData.liability_change.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                  </span>
                ) : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract_detail.asset_before", language)}>
                {adjustmentModalData.asset_before != null ? `¥${adjustmentModalData.asset_before.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}` : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract_detail.asset_after", language)}>
                {adjustmentModalData.asset_after != null ? (
                  <span style={{ fontWeight: "bold", color: "#000" }}>¥{adjustmentModalData.asset_after.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}</span>
                ) : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract_detail.asset_change", language)}>
                {adjustmentModalData.asset_change != null ? (
                  <span style={{ fontWeight: "bold", color: adjustmentModalData.asset_change >= 0 ? "#000" : "#000" }}>
                    {adjustmentModalData.asset_change >= 0 ? "+" : ""}¥{adjustmentModalData.asset_change.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                  </span>
                ) : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract_detail.pnl_impact", language)}>
                {adjustmentModalData.pnl_impact != null ? (
                  <span style={{ fontWeight: "bold", color: adjustmentModalData.pnl_impact >= 0 ? "#000" : "#000" }}>
                    {adjustmentModalData.pnl_impact >= 0 ? "+" : ""}¥{adjustmentModalData.pnl_impact.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                  </span>
                ) : "-"}
              </Descriptions.Item>
            </Descriptions>
          )}
        </Modal>
      </AppLayout>
    </ProtectedRoute>
  );
}
