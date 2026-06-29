"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import {
  Tag,
  Button,
  Form,
  message,
  Spin,
  Tabs,
} from "antd";
import { ArrowLeftOutlined } from "@ant-design/icons";
import AppLayout from "../../components/AppLayout";
import ProtectedRoute from "../../components/ProtectedRoute";
import { CalculationPanel } from "./components/CalculationPanel";
import { ContractActions } from "./components/ContractActions";
import {
  AdjustmentPreviewData,
  AdjustmentPreviewModal,
  EditDraftModal,
  RejectionReasonModal,
} from "./components/ContractDetailModals";
import {
  CreateEventModal,
  CreateScheduleModal,
  EditContractModal,
} from "./components/ContractFormModals";
import { ContractInfoTab, ContractOverviewPanels } from "./components/ContractOverviewPanels";
import { CriticalDateModal } from "./components/CriticalDateModal";
import { DocumentModal } from "./components/DocumentModal";
import { EventsPanel } from "./components/EventsPanel";
import { PaymentSchedulesPanel } from "./components/PaymentSchedulesPanel";
import { contractApi, paymentScheduleApi, eventApi, leaseAdminApi } from "../../lib/api";
import { ObligationModal } from "./components/ObligationModal";
import {
  getApprovalStatusColor,
  getApprovalStatusLabel,
} from "../../lib/constants/contracts";
import { buildLeaseAdminTabItems } from "./components/lease-admin-tab-items";
import { useAuth } from "../../context/AuthContext";
import { useLanguage } from "../../context/LanguageContext";
import { t } from "../../lib/i18n";
import { parseTagString, normalizeTagValues, DEFAULT_TAG_SUGGESTIONS } from "../../lib/tags";
import type {
  CalculationResult,
  ContractEvent,
  ContractDetail,
  CriticalDate,
  LeaseDocument,
  LeaseObligation,
  PaymentSchedule,
  PaymentScheduleDraftItem,
  UpdateContractRequest,
} from "../../lib/types/contracts";
import dayjs from "dayjs";
import { motion } from "framer-motion";
import {
  CRITICAL_STATUS_COLORS,
  getEventTypeLabels,
  OBLIGATION_STATUS_COLORS,
  OBLIGATION_TYPE_LABELS,
  RESPONSIBLE_PARTY_LABELS,
} from "./components/constants";
import { buildCalculationColumns, buildScheduleColumns } from "./components/tableColumns";
import type {
  ContractUpdateFormValues,
  CreateEventFormValues,
  CriticalDateFormValues,
  DocumentFormValues,
  EditDraftFormValues,
  ObligationFormValues,
  ScheduleFormValues,
} from "./components/types";

function getErrorMessage(error: unknown): string | undefined {
  if (typeof error === "object" && error !== null && "message" in error) {
    const candidate = (error as { message?: unknown }).message;
    return typeof candidate === "string" ? candidate : undefined;
  }
  return undefined;
}

export default function ContractDetailPage() {
  const params = useParams();
  const router = useRouter();
  const { token, user } = useAuth();
  const { language } = useLanguage();
  const contractId = params.id as string;

  const [contract, setContract] = useState<ContractDetail | null>(null);
  const [schedules, setSchedules] = useState<PaymentSchedule[]>([]);
  const [calcResult, setCalcResult] = useState<CalculationResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [calcLoading, setCalcLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [form] = Form.useForm();
  const [activeTab, setActiveTab] = useState("info");
  const [aiDrafts, setAiDrafts] = useState<PaymentScheduleDraftItem[]>([]);
  const [aiWarnings, setAiWarnings] = useState<string[]>([]);
  const [showDraftPanel, setShowDraftPanel] = useState(false);
  const [importLoading, setImportLoading] = useState(false);
  const [events, setEvents] = useState<ContractEvent[]>([]);
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
  const [adjustmentModalData, setAdjustmentModalData] = useState<AdjustmentPreviewData | null>(null);
  const [adjustmentModalTitle, setAdjustmentModalTitle] = useState("");
  const [previewLoading, setPreviewLoading] = useState(false);
  const [criticalDates, setCriticalDates] = useState<CriticalDate[]>([]);
  const [documents, setDocuments] = useState<LeaseDocument[]>([]);
  const [obligations, setObligations] = useState<LeaseObligation[]>([]);
  const [criticalDateModalOpen, setCriticalDateModalOpen] = useState(false);
  const [criticalDateForm] = Form.useForm();
  const [criticalDateLoading, setCriticalDateLoading] = useState(false);
  const [documentModalOpen, setDocumentModalOpen] = useState(false);
  const [documentForm] = Form.useForm();
  const [documentLoading, setDocumentLoading] = useState(false);
  const [obligationModalOpen, setObligationModalOpen] = useState(false);
  const [obligationForm] = Form.useForm();
  const [obligationLoading, setObligationLoading] = useState(false);

  useEffect(() => {
    if (token && contractId) {
      loadContract();
      loadSchedules();
      loadEvents();
      loadCriticalDates();
      loadDocuments();
      loadObligations();
    }
  }, [token, contractId]);

  const loadContract = async () => {
    setLoading(true);
    try {
      const data = await contractApi.get(contractId, token!);
      setContract(data);
    } catch (error) {
      message.error(getErrorMessage(error) || t("contract_detail.load_contract_failed", language));
    } finally {
      setLoading(false);
    }
  };

  const loadSchedules = async () => {
    try {
      const data = await paymentScheduleApi.list(contractId, token!);
      setSchedules(data.data || []);
    } catch (error) {
      message.error(getErrorMessage(error) || t("contract_detail.load_schedules_failed", language));
    }
  };

  const loadEvents = async () => {
    try {
      const data = await eventApi.list(contractId, token!);
      setEvents(data.data || []);
    } catch (error) {
      message.error(getErrorMessage(error) || t("contract_detail.load_events_failed", language));
    }
  };

  const loadCriticalDates = async () => {
    try {
      const data = await leaseAdminApi.listCriticalDates(contractId, token!);
      setCriticalDates(data.data || []);
    } catch (error) {
      message.error(getErrorMessage(error) || "关键日期加载失败");
    }
  };

  const loadDocuments = async () => {
    try {
      const data = await leaseAdminApi.listDocuments(contractId, token!);
      setDocuments(data.data || []);
    } catch (error) {
      message.error(getErrorMessage(error) || "文档列表加载失败");
    }
  };

  const loadObligations = async () => {
    try {
      const data = await leaseAdminApi.listObligations(contractId, token!);
      setObligations(data.data || []);
    } catch (error) {
      message.error(getErrorMessage(error) || "条款义务加载失败");
    }
  };

  const handleCreateCriticalDate = async (values: CriticalDateFormValues) => {
    setCriticalDateLoading(true);
    try {
      const targetDate = values.target_date?.format("YYYY-MM-DD");
      if (!targetDate) {
        message.warning(t("contract_detail.validation.effective_date", language));
        return;
      }
      await leaseAdminApi.createCriticalDate(contractId, {
        date_type: values.date_type,
        target_date: targetDate,
        reminder_days: values.reminder_days || 30,
        title: values.title,
        description: values.description,
        source: "manual",
      }, token!);
      message.success("关键日期已创建");
      setCriticalDateModalOpen(false);
      criticalDateForm.resetFields();
      loadCriticalDates();
    } catch (error) {
      message.error(getErrorMessage(error) || "关键日期创建失败");
    } finally {
      setCriticalDateLoading(false);
    }
  };

  const handleUpdateCriticalDateStatus = async (dateId: string, status: string) => {
    try {
      await leaseAdminApi.updateCriticalDateStatus(contractId, dateId, status, token!);
      message.success("关键日期状态已更新");
      loadCriticalDates();
    } catch (error) {
      message.error(getErrorMessage(error) || "状态更新失败");
    }
  };

  const handleCreateDocument = async (values: DocumentFormValues) => {
    setDocumentLoading(true);
    try {
      await leaseAdminApi.createDocument(contractId, {
        document_type: values.document_type,
        file_name: values.file_name,
        file_type: values.file_type || undefined,
        document_version: values.document_version || undefined,
        notes: values.notes || undefined,
      }, token!);
      message.success("文档记录已创建");
      setDocumentModalOpen(false);
      documentForm.resetFields();
      loadDocuments();
    } catch (error) {
      message.error(getErrorMessage(error) || "文档记录创建失败");
    } finally {
      setDocumentLoading(false);
    }
  };

  const handleCreateObligation = async (values: ObligationFormValues) => {
    setObligationLoading(true);
    try {
      await leaseAdminApi.createObligation(contractId, {
        obligation_type: values.obligation_type,
        responsible_party: values.responsible_party,
        title: values.title,
        description: values.description || undefined,
        source_clause: values.source_clause || undefined,
        source_page: values.source_page ?? undefined,
      }, token!);
      message.success("条款义务已创建");
      setObligationModalOpen(false);
      obligationForm.resetFields();
      loadObligations();
    } catch (error) {
      message.error(getErrorMessage(error) || "条款义务创建失败");
    } finally {
      setObligationLoading(false);
    }
  };

  const handleUpdateObligationStatus = async (obligationId: string, status: string) => {
    try {
      await leaseAdminApi.updateObligationStatus(contractId, obligationId, status, token!);
      message.success("条款义务状态已更新");
      loadObligations();
    } catch (error) {
      message.error(getErrorMessage(error) || "状态更新失败");
    }
  };

  const handleCreateEvent = async (values: CreateEventFormValues) => {
    setEventLoading(true);
    try {
      const effectiveDate = values.effective_date?.format("YYYY-MM-DD");
      if (!effectiveDate) {
        message.warning(t("contract_detail.validation.effective_date", language));
        return;
      }
      await eventApi.create(contractId, {
        contract_id: contractId,
        event_type: values.event_type,
        effective_date: effectiveDate,
        original_value: values.original_value,
        new_value: values.new_value,
        change_reason: values.change_reason,
        judgment_basis: values.judgment_basis,
      }, token!);
      message.success(t("contract_detail.event_created", language));
      setEventModalOpen(false);
      eventForm.resetFields();
      loadEvents();
    } catch (error) {
      message.error(getErrorMessage(error) || t("contract_detail.create_event_failed", language));
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
    } catch (error) {
      message.error(getErrorMessage(error) || t("contract_detail.submit_failed", language));
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
    } catch (error) {
      message.error(getErrorMessage(error) || t("contract_detail.review_failed", language));
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
    } catch (error) {
      message.error(getErrorMessage(error) || t("contract_detail.approve_failed", language));
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
    } catch (error) {
      message.error(getErrorMessage(error) || t("contract_detail.operation_failed", language));
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
    } catch (error) {
      message.error(getErrorMessage(error) || t("contract_detail.preview_failed", language));
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
    } catch (error) {
      message.error(getErrorMessage(error) || t("contract_detail.get_adjustment_failed", language));
    }
  };

  const handleRecalculateEvent = async (eventId: string) => {
    try {
      await eventApi.recalculate(contractId, eventId, token!);
      message.success(t("contract_detail.event_recalculated", language));
      loadEvents();
    } catch (error) {
      message.error(getErrorMessage(error) || t("contract_detail.recalculate_failed", language));
    }
  };

  const handleSubmitForReview = async () => {
    setActionLoading('submit');
    try {
      await contractApi.submitForReview(contractId, token!);
      message.success(t("contract_detail.submit_review_success", language));
      loadContract();
    } catch (error) {
      message.error(getErrorMessage(error) || t("contract_detail.submit_failed", language));
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
    } catch (error) {
      message.error(getErrorMessage(error) || t("contract_detail.review_failed", language));
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
    } catch (error) {
      message.error(getErrorMessage(error) || t("contract_detail.approve_failed", language));
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
    } catch (error) {
      message.error(getErrorMessage(error) || t("contract_detail.operation_failed", language));
    } finally {
      setActionLoading(null);
    }
  };

  const handleCalculate = async () => {
    setCalcLoading(true);
    try {
      const discountRate = contract?.discount_rate_value;
      const data = await contractApi.calculate(
        contractId,
        discountRate,
        token!
      );
      setCalcResult(data);
      message.success(t("contract_detail.ifrs16_calculated", language));
      setActiveTab("calculation");
    } catch (error) {
      message.error(getErrorMessage(error) || t("contract_detail.calculate_failed", language));
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
      asset_type: contract.asset_type || "real_estate",
      discount_rate_type: contract.discount_rate_type || "",
      discount_rate_version: contract.discount_rate_version || "",
      discount_rate_value: contract.discount_rate_value ?? null,
      lease_scope: contract.lease_scope || "in_scope",
      exemption_reason: contract.exemption_reason || "",
      tags: parseTagString(contract.tags || ""),
    });
    setEditModalOpen(true);
  };

  const handleUpdate = async (values: ContractUpdateFormValues) => {
    if (!contract) {
      return;
    }
    setEditLoading(true);
    try {
      const payload: UpdateContractRequest = {
        contract_number: values.contract_number,
        contract_name: values.contract_name,
        lessee_name: values.lessee_name,
        lessor_name: values.lessor_name,
        store_name: values.store_name,
        store_address: values.store_address,
        currency: values.currency,
        asset_type: values.asset_type || contract.asset_type || "real_estate",
        commencement_date: values.commencement_date.format("YYYY-MM-DD"),
        lease_start_date: values.lease_start_date.format("YYYY-MM-DD"),
        lease_end_date: values.lease_end_date.format("YYYY-MM-DD"),
        discount_rate_type: values.discount_rate_type || null,
        discount_rate_version: values.discount_rate_version || null,
        discount_rate_value: values.discount_rate_value ?? null,
        lease_scope: values.lease_scope || contract.lease_scope || "in_scope",
        exemption_reason: values.exemption_reason || null,
        scope_source: "manual",
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
    } catch (error) {
      message.error(getErrorMessage(error) || t("contract_detail.update_failed", language));
    } finally {
      setEditLoading(false);
    }
  };

  const openPaymentScheduleAgent = () => {
    const title = contract
      ? `${contract.contract_number} ${contract.contract_name}`
      : t("contract.tab_payments", language);
    const params = new URLSearchParams({
      page: "contract-detail",
      contract_id: contractId,
      title,
      summary: t("contract_detail.agent_payment_summary", language),
    });
    router.push(`/ai-chat?${params.toString()}`);
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
        const scheduleDate = draft.due_date || draft.period_start || draft.period_end;
        if (!scheduleDate) {
          continue;
        }
        await paymentScheduleApi.create(contractId, {
          contract_id: contractId,
          effective_start_date: draft.period_start || scheduleDate,
          effective_end_date: draft.period_end || scheduleDate,
          coverage_start_date: draft.period_start || scheduleDate,
          coverage_end_date: draft.period_end || scheduleDate,
          due_date: scheduleDate,
          payment_timing: draft.payment_timing || "postpaid",
          amount: draft.amount,
          currency: draft.currency || "CNY",
          amount_type: draft.amount_type || "fixed_rent",
          is_fixed: draft.is_fixed ?? true,
          is_lease_component: draft.is_lease_component ?? true,
          included_in_liability_pv: draft.is_lease_component ?? true,
        }, token!);
        successCount++;
      }
      message.success(t("contract_detail.import_success", language, { count: String(successCount) }));
      setShowDraftPanel(false);
      setAiDrafts([]);
      loadSchedules();
    } catch (error) {
      message.error(getErrorMessage(error) || t("contract_detail.import_failed", language));
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

  const handleSaveEditDraft = (values: EditDraftFormValues) => {
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

  const handleCreateSchedule = async (values: ScheduleFormValues) => {
    try {
      const effectiveStartDate = values.effective_start_date?.format("YYYY-MM-DD");
      const effectiveEndDate = values.effective_end_date?.format("YYYY-MM-DD");
      const coverageStartDate = values.coverage_start_date?.format("YYYY-MM-DD");
      const coverageEndDate = values.coverage_end_date?.format("YYYY-MM-DD");
      const dueDate = values.due_date?.format("YYYY-MM-DD");
      if (!effectiveStartDate || !effectiveEndDate || !coverageStartDate || !coverageEndDate || !dueDate) {
        message.warning(t("contract_detail.validation.payment_date", language));
        return;
      }
      const payload = {
        contract_id: contractId,
        effective_start_date: effectiveStartDate,
        effective_end_date: effectiveEndDate,
        coverage_start_date: coverageStartDate,
        coverage_end_date: coverageEndDate,
        due_date: dueDate,
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
    } catch (error) {
      message.error(getErrorMessage(error) || t("contract_detail.create_schedule_failed", language));
    }
  };

  const eventTypeLabels = getEventTypeLabels(language);

  const leaseAdminTabItems = buildLeaseAdminTabItems({
    criticalDates,
    documents,
    obligations,
    criticalStatusColors: CRITICAL_STATUS_COLORS,
    obligationTypeLabels: OBLIGATION_TYPE_LABELS,
    responsiblePartyLabels: RESPONSIBLE_PARTY_LABELS,
    obligationStatusColors: OBLIGATION_STATUS_COLORS,
    onOpenCriticalDateModal: () => setCriticalDateModalOpen(true),
    onOpenDocumentModal: () => setDocumentModalOpen(true),
    onOpenObligationModal: () => setObligationModalOpen(true),
    onUpdateCriticalDateStatus: handleUpdateCriticalDateStatus,
    onUpdateObligationStatus: handleUpdateObligationStatus,
  });

  const scheduleColumns = buildScheduleColumns(language);
  const calcColumns = buildCalculationColumns(language);

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
              <Tag color={getApprovalStatusColor(contract.approval_status)}>
                {getApprovalStatusLabel(contract.approval_status, language)}
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
              <ContractOverviewPanels
                contract={contract}
                language={language}
                actions={
                  <ContractActions
                    approvalStatus={contract.approval_status}
                    currentUserRole={user?.role}
                    currentUserRoles={user?.roles}
                    actionLoading={actionLoading}
                    calcLoading={calcLoading}
                    language={language}
                    onSubmitForReview={handleSubmitForReview}
                    onOpenReviewReject={() => {
                      setRejectModalType("review");
                      setRejectReason("");
                      setRejectModalOpen(true);
                    }}
                    onReviewApprove={handleReviewApprove}
                    onOpenApproveReject={() => {
                      setRejectModalType("approve");
                      setRejectReason("");
                      setRejectModalOpen(true);
                    }}
                    onApprove={handleApprove}
                    onEdit={handleEditOpen}
                    onCalculate={handleCalculate}
                  />
                }
              />

              <Tabs
                activeKey={activeTab}
                onChange={setActiveTab}
                style={{ marginTop: 16 }}
                items={[
                  {
                    key: "info",
                    label: t("contract.tab_info", language),
                    children: <ContractInfoTab contract={contract} language={language} />,
                  },
                  {
                    key: "payments",
                    label: `${t("contract.tab_payments", language)} (${schedules.length})`,
                    children: (
                      <PaymentSchedulesPanel
                        schedules={schedules}
                        scheduleColumns={scheduleColumns}
                        aiDrafts={aiDrafts}
                        aiWarnings={aiWarnings}
                        showDraftPanel={showDraftPanel}
                        importLoading={importLoading}
                        language={language}
                        onOpenAgent={openPaymentScheduleAgent}
                        onOpenManualCreate={() => setModalOpen(true)}
                        onConfirmAllDrafts={handleConfirmAll}
                        onDismissDraftPanel={() => {
                          setShowDraftPanel(false);
                          setAiDrafts([]);
                        }}
                        onImportDrafts={handleImportDrafts}
                        onEditDraft={handleOpenEditDraft}
                        onConfirmDraft={handleConfirmDraft}
                        onSkipDraft={handleSkipDraft}
                        onRestoreDraft={handleRestoreDraft}
                      />
                    ),
                  },
                  ...(leaseAdminTabItems || []),
                  {
                    key: "events",
                    label: `${t("contract.tab_events", language)} (${events.length})`,
                    children: (
                      <EventsPanel
                        events={events}
                        eventTypeLabels={eventTypeLabels}
                        language={language}
                        previewLoading={previewLoading}
                        eventActionLoading={eventActionLoading}
                        currentUserRole={user?.role}
                        currentUserRoles={user?.roles}
                        onOpenCreate={() => setEventModalOpen(true)}
                        onViewAdjustment={handleViewAdjustment}
                        onPreviewAdjustment={handlePreviewAdjustment}
                        onRecalculateEvent={handleRecalculateEvent}
                        onSubmitForReview={handleEventSubmitForReview}
                        onReviewApprove={handleEventReviewApprove}
                        onApprove={handleEventApprove}
                        onRejectOpen={handleEventRejectOpen}
                      />
                    ),
                  },
                  {
                    key: "calculation",
                    label: t("contract.tab_calculation", language),
                    children: (
                      <CalculationPanel
                        calcResult={calcResult}
                        calcColumns={calcColumns}
                        sortedMonthly={sortedMonthly}
                        language={language}
                      />
                    ),
                  },
                ]}
              />
            </>
          )}
        </Spin>
        </motion.div>

        <CreateScheduleModal
          open={modalOpen}
          form={form}
          language={language}
          onCancel={() => setModalOpen(false)}
          onSubmit={handleCreateSchedule}
        />

        <CreateEventModal
          open={eventModalOpen}
          loading={eventLoading}
          form={eventForm}
          language={language}
          onCancel={() => setEventModalOpen(false)}
          onSubmit={handleCreateEvent}
        />

        <EditContractModal
          open={editModalOpen}
          loading={editLoading}
          form={editForm}
          language={language}
          onCancel={() => setEditModalOpen(false)}
          onSubmit={handleUpdate}
        />

        <CriticalDateModal
          open={criticalDateModalOpen}
          loading={criticalDateLoading}
          form={criticalDateForm}
          onCancel={() => setCriticalDateModalOpen(false)}
          onSubmit={handleCreateCriticalDate}
        />

        <DocumentModal
          open={documentModalOpen}
          loading={documentLoading}
          form={documentForm}
          onCancel={() => setDocumentModalOpen(false)}
          onSubmit={handleCreateDocument}
        />

        <ObligationModal
          open={obligationModalOpen}
          loading={obligationLoading}
          form={obligationForm}
          onCancel={() => setObligationModalOpen(false)}
          onSubmit={handleCreateObligation}
        />

        <RejectionReasonModal
          title={rejectModalType === 'review' ? t("contract.review_reject_title", language) : t("contract.approve_reject_title", language)}
          open={rejectModalOpen}
          value={rejectReason}
          loading={actionLoading === (rejectModalType === 'review' ? 'review_reject' : 'approve_reject')}
          language={language}
          prompt={rejectModalType === 'review' ? t("contract.review_reject_reason", language) : t("contract.approve_reject_reason", language)}
          onCancel={() => setRejectModalOpen(false)}
          onChange={setRejectReason}
          onSubmit={handleRejectSubmit}
        />

        <RejectionReasonModal
          title={eventRejectType === 'review' ? t("contract.review_reject_title", language) : t("contract.approve_reject_title", language)}
          open={eventRejectModalOpen}
          value={eventRejectReason}
          loading={Boolean(eventRejectEventId && eventActionLoading === eventRejectEventId + '_reject')}
          language={language}
          prompt={eventRejectType === 'review' ? t("contract.review_reject_reason", language) : t("contract.approve_reject_reason", language)}
          onCancel={() => setEventRejectModalOpen(false)}
          onChange={setEventRejectReason}
          onSubmit={handleEventRejectSubmit}
        />

        <EditDraftModal
          open={editDraftModalOpen}
          form={editDraftForm}
          language={language}
          onCancel={() => setEditDraftModalOpen(false)}
          onSubmit={handleSaveEditDraft}
        />

        <AdjustmentPreviewModal
          title={adjustmentModalTitle}
          open={adjustmentModalOpen}
          data={adjustmentModalData}
          language={language}
          onClose={() => setAdjustmentModalOpen(false)}
        />
      </AppLayout>
    </ProtectedRoute>
  );
}
