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
import { parseTagString, normalizeTagValues, DEFAULT_TAG_SUGGESTIONS } from "../../lib/tags";
import dayjs from "dayjs";

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
      message.error(error.message || "加载合同失败");
    } finally {
      setLoading(false);
    }
  };

  const loadSchedules = async () => {
    try {
      const data = await paymentScheduleApi.list(contractId, token!);
      setSchedules(data.data || []);
    } catch (error: any) {
      message.error(error.message || "加载付款计划失败");
    }
  };

  const loadEvents = async () => {
    try {
      const data = await eventApi.list(contractId, token!);
      setEvents(data.data || []);
    } catch (error: any) {
      message.error(error.message || "加载事件失败");
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
      message.success("事件创建成功");
      setEventModalOpen(false);
      eventForm.resetFields();
      loadEvents();
    } catch (error: any) {
      message.error(error.message || "创建事件失败");
    } finally {
      setEventLoading(false);
    }
  };

  const handleEventSubmitForReview = async (eventId: string) => {
    setEventActionLoading(eventId + '_submit');
    try {
      await eventApi.submitForReview(contractId, eventId, token!);
      message.success('事件已提交复核');
      loadEvents();
    } catch (error: any) {
      message.error(error.message || '提交失败');
    } finally {
      setEventActionLoading(null);
    }
  };

  const handleEventReviewApprove = async (eventId: string) => {
    setEventActionLoading(eventId + '_review');
    try {
      await eventApi.review(contractId, eventId, true, '', token!);
      message.success('复核通过');
      loadEvents();
    } catch (error: any) {
      message.error(error.message || '复核失败');
    } finally {
      setEventActionLoading(null);
    }
  };

  const handleEventApprove = async (eventId: string) => {
    setEventActionLoading(eventId + '_approve');
    try {
      await eventApi.approve(contractId, eventId, token!);
      message.success('审批通过');
      loadEvents();
    } catch (error: any) {
      message.error(error.message || '审批失败');
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
      message.warning('请输入原因');
      return;
    }
    if (!eventRejectEventId) return;

    setEventActionLoading(eventRejectEventId + '_reject');
    try {
      if (eventRejectType === 'review') {
        await eventApi.review(contractId, eventRejectEventId, false, eventRejectReason, token!);
        message.success('已退回编辑');
      } else {
        await eventApi.reject(contractId, eventRejectEventId, eventRejectReason, token!);
        message.success('已驳回');
      }
      setEventRejectModalOpen(false);
      setEventRejectReason('');
      setEventRejectEventId(null);
      loadEvents();
    } catch (error: any) {
      message.error(error.message || '操作失败');
    } finally {
      setEventActionLoading(null);
    }
  };

  const handlePreviewAdjustment = async (eventId: string) => {
    setPreviewLoading(true);
    try {
      const data = await eventApi.previewAdjustment(contractId, eventId, token!);
      setAdjustmentModalTitle("事件影响预览");
      setAdjustmentModalData(data);
      setAdjustmentModalOpen(true);
    } catch (error: any) {
      message.error(error.message || "预览失败");
    } finally {
      setPreviewLoading(false);
    }
  };

  const handleViewAdjustment = async (eventId: string) => {
    try {
      const data = await eventApi.getAdjustment(contractId, eventId, token!);
      setAdjustmentModalTitle("事件调整详情");
      setAdjustmentModalData(data);
      setAdjustmentModalOpen(true);
    } catch (error: any) {
      message.error(error.message || "获取调整详情失败");
    }
  };

  const handleRecalculateEvent = async (eventId: string) => {
    try {
      await eventApi.recalculate(contractId, eventId, token!);
      message.success("事件重算完成");
      loadEvents();
    } catch (error: any) {
      message.error(error.message || "重算失败");
    }
  };

  const handleSubmitForReview = async () => {
    setActionLoading('submit');
    try {
      await contractApi.submitForReview(contractId, token!);
      message.success('提交复核成功');
      loadContract();
    } catch (error: any) {
      message.error(error.message || '提交失败');
    } finally {
      setActionLoading(null);
    }
  };

  const handleReviewApprove = async () => {
    setActionLoading('review_approve');
    try {
      await contractApi.review(contractId, true, '', token!);
      message.success('复核通过');
      loadContract();
    } catch (error: any) {
      message.error(error.message || '复核失败');
    } finally {
      setActionLoading(null);
    }
  };

  const handleApprove = async () => {
    setActionLoading('approve');
    try {
      await contractApi.approve(contractId, token!);
      message.success('审批通过');
      loadContract();
    } catch (error: any) {
      message.error(error.message || '审批失败');
    } finally {
      setActionLoading(null);
    }
  };

  const handleRejectSubmit = async () => {
    if (!rejectReason.trim()) {
      message.warning('请输入原因');
      return;
    }

    const loadingKey = rejectModalType === 'review' ? 'review_reject' : 'approve_reject';
    setActionLoading(loadingKey);
    try {
      if (rejectModalType === 'review') {
        await contractApi.review(contractId, false, rejectReason, token!);
        message.success('已退回编辑');
      } else {
        await contractApi.reject(contractId, rejectReason, token!);
        message.success('已驳回');
      }
      setRejectModalOpen(false);
      setRejectReason('');
      loadContract();
    } catch (error: any) {
      message.error(error.message || '操作失败');
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
      message.success("IFRS 16 计算完成");
      setActiveTab("calculation");
    } catch (error: any) {
      message.error(error.message || "计算失败");
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
      message.success("合同更新成功");
      setEditModalOpen(false);
      loadContract();
    } catch (error: any) {
      message.error(error.message || "更新失败");
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
        message.warning("AI 识别结果需要人工复核，请检查低置信度字段");
      } else {
        message.success(`AI 识别完成：${parseRes.schedules?.length || 0} 笔付款计划`);
      }
    } catch (error: any) {
      message.error(error.message || "AI 解析失败");
    } finally {
      setUploadLoading(false);
    }
    return false; // prevent default upload
  };

  const handleImportDrafts = async () => {
    const confirmedDrafts = aiDrafts.filter((d) => d.confirmed && !d.skipped);
    if (confirmedDrafts.length === 0) {
      message.warning("没有已确认的草稿可导入");
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
      message.success(`成功导入 ${successCount} 笔付款计划`);
      setShowDraftPanel(false);
      setAiDrafts([]);
      loadSchedules();
    } catch (error: any) {
      message.error(error.message || "导入失败");
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
    message.success("草稿行已更新");
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
    message.success("已全选确认");
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
      message.success("付款计划创建成功");
      setModalOpen(false);
      form.resetFields();
      loadSchedules();
    } catch (error: any) {
      message.error(error.message || "创建失败");
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
    draft: "草稿",
    submitted: "已提交",
    reviewed: "已复核",
    pending_approval: "待审批",
    approved: "已审批",
    rejected: "已驳回",
  };

  const eventTypeLabels: Record<string, string> = {
    area_adjustment: "面积调整",
    rent_change: "租金变更",
    renewal: "续租",
    early_termination: "提前终止",
    index_update: "指数更新",
    discount_rate_change: "折现率变更",
    impairment: "减值",
  };

  const scheduleColumns = [
    { title: "付款日", dataIndex: "due_date", width: 110 },
    {
      title: "金额",
      dataIndex: "amount",
      width: 120,
      render: (v: number, r: PaymentSchedule) =>
        `¥${v.toLocaleString()}`,
    },
    { title: "币种", dataIndex: "currency", width: 80 },
    {
      title: "时点",
      dataIndex: "payment_timing",
      width: 90,
      render: (v: string) =>
        v === "prepaid" ? (
          <Tag color="blue">先付</Tag>
        ) : (
          <Tag color="green">后付</Tag>
        ),
    },
    { title: "类型", dataIndex: "amount_type", width: 120 },
    {
      title: "固定/变量",
      width: 100,
      render: (_: any, r: PaymentSchedule) => (
        <Space>
          {r.is_fixed && <Tag>固定</Tag>}
          {r.is_variable && <Tag color="orange">变量</Tag>}
        </Space>
      ),
    },
    {
      title: "租赁成分",
      width: 90,
      render: (_: any, r: PaymentSchedule) =>
        r.is_lease_component ? <Tag color="green">是</Tag> : <Tag>否</Tag>,
    },
    {
      title: "计入负债",
      width: 90,
      render: (_: any, r: PaymentSchedule) =>
        r.included_in_liability_pv ? (
          <Tag color="green">是</Tag>
        ) : (
          <Tag>否</Tag>
        ),
    },
  ];

  const calcColumns = [
    {
      title: "期间",
      render: (_: any, r: MonthlyEntry) => `${r.Year}-${String(r.Month).padStart(2, "0")}`,
      width: 90,
    },
    {
      title: "期初负债",
      dataIndex: "OpeningLiability",
      render: (v: number) => `¥${v.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right" as const,
    },
    {
      title: "利息费用",
      dataIndex: "InterestExpense",
      render: (v: number) => `¥${v.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right" as const,
    },
    {
      title: "付款",
      dataIndex: "TotalPayments",
      render: (v: number) => `¥${v.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right" as const,
    },
    {
      title: "期末负债",
      dataIndex: "ClosingLiability",
      render: (v: number) => `¥${v.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right" as const,
    },
    {
      title: "期初ROU",
      dataIndex: "OpeningROUAsset",
      render: (v: number) => `¥${v.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right" as const,
    },
    {
      title: "折旧",
      dataIndex: "Depreciation",
      render: (v: number) => `¥${v.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right" as const,
    },
    {
      title: "期末ROU",
      dataIndex: "ClosingROUAsset",
      render: (v: number) => `¥${v.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right" as const,
    },
    {
      title: "变量租金",
      dataIndex: "VariableRentExpense",
      render: (v: number) => `¥${v.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`,
      align: "right" as const,
    },
    {
      title: "非租赁费用",
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
        <Spin spinning={loading}>
          <div style={{ marginBottom: 16 }}>
            <Button
              icon={<ArrowLeftOutlined />}
              onClick={() => router.push("/contracts")}
            >
              返回列表
            </Button>
          </div>

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
                          <Tag color="blue">正式版</Tag>
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
                            提交复核
                          </Button>
                        )}

                        {user && contract.approval_status === 'submitted' && (user.role === 'reviewer' || user.role === 'admin') && (
                          <>
                            <Button
                              type="primary"
                              style={{ background: '#10B981', borderColor: '#10B981' }}
                              onClick={handleReviewApprove}
                              loading={actionLoading === 'review_approve'}
                            >
                              复核通过
                            </Button>
                            <Button
                              danger
                              onClick={() => { setRejectModalType('review'); setRejectReason(''); setRejectModalOpen(true); }}
                            >
                              退回编辑
                            </Button>
                          </>
                        )}

                        {user && (contract.approval_status === 'reviewed' || contract.approval_status === 'pending_approval') && (user.role === 'approver' || user.role === 'admin') && (
                          <>
                            <Button
                              type="primary"
                              style={{ background: '#10B981', borderColor: '#10B981' }}
                              onClick={handleApprove}
                              loading={actionLoading === 'approve'}
                            >
                              审批通过
                            </Button>
                            <Button
                              danger
                              onClick={() => { setRejectModalType('approve'); setRejectReason(''); setRejectModalOpen(true); }}
                            >
                              驳回
                            </Button>
                          </>
                        )}

                        {user && contract.approval_status === 'rejected' && (user.role === 'editor' || user.role === 'admin') && (
                          <Button
                            type="primary"
                            onClick={handleSubmitForReview}
                            loading={actionLoading === 'submit'}
                          >
                            重新提交
                          </Button>
                        )}

                        {user && (contract.approval_status === 'draft' || contract.approval_status === 'rejected') && (user.role === 'editor' || user.role === 'admin') && (
                          <Button
                            icon={<EditOutlined />}
                            onClick={handleEditOpen}
                          >
                            编辑
                          </Button>
                        )}

                        <Button
                          icon={<CalculatorOutlined />}
                          onClick={handleCalculate}
                          loading={calcLoading}
                          type="primary"
                        >
                          IFRS 16 计算
                        </Button>

                        <Button
                          icon={<RobotOutlined />}
                          onClick={() => router.push(`/ai-chat?page=contract-detail&title=合同详情&contract_id=${encodeURIComponent(contractId)}`)}
                        >
                          AI 分析
                        </Button>
                      </Space>
                    }
                  >
                    <Descriptions column={3} size="small">
                      <Descriptions.Item label="合同编号">
                        {contract.contract_number}
                      </Descriptions.Item>
                      <Descriptions.Item label="币种">
                        {contract.currency}
                      </Descriptions.Item>
                      <Descriptions.Item label="折现率">
                        {contract.discount_rate_value != null ? (
                          <Tag color="success">{(contract.discount_rate_value * 100).toFixed(2)}%</Tag>
                        ) : contract.discount_rate_missing ? (
                          <Tag color="error">缺失</Tag>
                        ) : (
                          <Tag color="success">
                            {contract.discount_rate_type} / {contract.discount_rate_version}
                          </Tag>
                        )}
                      </Descriptions.Item>
                      <Descriptions.Item label="租赁起始日">
                        {dayjs(contract.commencement_date).format("YYYY-MM-DD")}
                      </Descriptions.Item>
                      <Descriptions.Item label="租赁开始日">
                        {dayjs(contract.lease_start_date).format("YYYY-MM-DD")}
                      </Descriptions.Item>
                      <Descriptions.Item label="租赁结束日">
                        {dayjs(contract.lease_end_date).format("YYYY-MM-DD")}
                      </Descriptions.Item>
                    </Descriptions>
                  </Card>
                </Col>

                <Col span={6}>
                  <Card title="审批进度" size="small">
                    <Timeline
                      items={[
                        {
                          dot: <ClockCircleOutlined />,
                          color: contract.created_at ? "green" : "gray",
                          children: `创建 ${dayjs(contract.created_at).format("YYYY-MM-DD")}`,
                        },
                        {
                          dot: <FileTextOutlined />,
                          color: contract.submitted_at ? "green" : "gray",
                          children: contract.submitted_at
                            ? `提交 ${dayjs(contract.submitted_at).format("YYYY-MM-DD")}`
                            : "待提交",
                        },
                        {
                          dot: <CheckCircleOutlined />,
                          color: contract.reviewed_at ? "green" : "gray",
                          children: contract.reviewed_at
                            ? `复核 ${dayjs(contract.reviewed_at).format("YYYY-MM-DD")}`
                            : "待复核",
                        },
                        {
                          dot: <CheckCircleOutlined />,
                          color: contract.approved_at ? "green" : "gray",
                          children: contract.approved_at
                            ? `审批 ${dayjs(contract.approved_at).format("YYYY-MM-DD")}`
                            : "待审批",
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
                    label: "合同信息",
                    children: (
                      <Card>
                        <Descriptions column={2} bordered>
                          <Descriptions.Item label="合同ID">
                            {contract.id}
                          </Descriptions.Item>
                          <Descriptions.Item label="法人主体ID">
                            {contract.legal_entity_id}
                          </Descriptions.Item>
                          <Descriptions.Item label="门店ID">
                            {contract.store_id}
                          </Descriptions.Item>
                          <Descriptions.Item label="出租方ID">
                            {contract.landlord_id}
                          </Descriptions.Item>
                          <Descriptions.Item label="创建时间">
                            {dayjs(contract.created_at).format("YYYY-MM-DD HH:mm")}
                          </Descriptions.Item>
                          {contract.rejected_reason && (
                            <Descriptions.Item label="驳回原因">
                              {contract.rejected_reason}
                            </Descriptions.Item>
                          )}
                        </Descriptions>
                      </Card>
                    ),
                  },
                  {
                    key: "payments",
                    label: `付款计划 (${schedules.length})`,
                    children: (
                  <Card
                    title={
                      <Space>
                        <span>付款计划</span>
                        {schedules.length > 0 && (
                          <Tag color="blue">{schedules.length} 笔</Tag>
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
                            AI 识别租金表
                          </Button>
                        </Upload>
                        <Button
                          type="primary"
                          icon={<PlusOutlined />}
                          onClick={() => setModalOpen(true)}
                        >
                          手动添加
                        </Button>
                      </Space>
                    }
                  >
                    {/* AI Draft Preview */}
                    {showDraftPanel && aiDrafts.length > 0 && (
                      <div style={{ marginBottom: 16, padding: 16, background: "#FAFAFA", border: "1px solid #EAEAEA", borderRadius: 12 }}>
                        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12, flexWrap: "wrap", gap: 8 }}>
                          <Space>
                            <RobotOutlined style={{ color: "#10B981" }} />
                            <span style={{ fontWeight: "bold" }}>AI 识别结果草稿</span>
                            <Tag color="green">
                              {aiDrafts.filter((d) => d.confirmed && !d.skipped).length} 笔已确认 / {aiDrafts.length} 笔总计
                            </Tag>
                          </Space>
                          <Space>
                            <Button size="small" onClick={handleConfirmAll}>
                              全选确认
                            </Button>
                            <Button size="small" onClick={() => { setShowDraftPanel(false); setAiDrafts([]); }}>
                              取消
                            </Button>
                            <Button
                              type="primary"
                              size="small"
                              icon={<ImportOutlined />}
                              onClick={handleImportDrafts}
                              loading={importLoading}
                            >
                              导入已确认行
                            </Button>
                          </Space>
                        </div>

                        {aiWarnings.length > 0 && (
                          <Alert
                            message="AI 警告"
                            description={
                              <ul style={{ margin: 0, paddingLeft: 16 }}>
                                {aiWarnings.slice(0, 5).map((w, i) => (
                                  <li key={i}><WarningOutlined style={{ color: "#F59E0B" }} /> {w}</li>
                                ))}
                                {aiWarnings.length > 5 && <li>... 等 {aiWarnings.length - 5} 条警告</li>}
                              </ul>
                            }
                            type="warning"
                            showIcon
                            style={{ marginBottom: 12 }}
                          />
                        )}

                        <Table
                          columns={[
                            { title: "付款日", dataIndex: "due_date", width: 110 },
                            { title: "金额", dataIndex: "amount", width: 100, render: (v: number) => `¥${v.toLocaleString()}` },
                            { title: "时点", dataIndex: "payment_timing", width: 80, render: (v: string) => v === "prepaid" ? <Tag color="blue">先付</Tag> : <Tag color="green">后付</Tag> },
                            { title: "类型", dataIndex: "amount_type", width: 100 },
                            { title: "固定", dataIndex: "is_fixed", width: 60, render: (v: boolean) => v ? "是" : "否" },
                            { title: "租赁", dataIndex: "is_lease_component", width: 60, render: (v: boolean) => v ? "是" : "否" },
                            {
                              title: "置信度",
                              dataIndex: "confidence",
                              width: 80,
                              render: (v: number) => (
                                <Tag color={v >= 0.9 ? "green" : v >= 0.8 ? "orange" : "red"}>
                                  {(v * 100).toFixed(0)}%
                                </Tag>
                              ),
                            },
                            {
                              title: "操作",
                              key: "action",
                              width: 210,
                              render: (_: any, record: any, index: number) => (
                                <Space size="small">
                                  <Button
                                    size="small"
                                    icon={<EditOutlined />}
                                    onClick={() => handleOpenEditDraft(index)}
                                  >
                                    编辑
                                  </Button>
                                  {!record.skipped && !record.confirmed && (
                                    <>
                                      <Button
                                        size="small"
                                        icon={<CheckOutlined />}
                                        style={{ color: "#10B981", borderColor: "#10B981" }}
                                        onClick={() => handleConfirmDraft(index)}
                                      >
                                        确认
                                      </Button>
                                      <Button
                                        size="small"
                                        icon={<StopOutlined />}
                                        danger
                                        onClick={() => handleSkipDraft(index)}
                                      >
                                        跳过
                                      </Button>
                                    </>
                                  )}
                                  {record.confirmed && (
                                    <Tag color="success" icon={<CheckCircleOutlined />}>已确认</Tag>
                                  )}
                                  {record.skipped && (
                                    <Button
                                      size="small"
                                      icon={<UndoOutlined />}
                                      onClick={() => handleRestoreDraft(index)}
                                    >
                                      恢复
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
                                ? "#FAFAFA"
                                : record.skipped
                                ? "#fafafa"
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
                      locale={{ emptyText: "暂无付款计划" }}
                    />
                  </Card>
                    ),
                  },
                  {
                    key: "events",
                    label: `变更事件 (${events.length})`,
                    children: (
                      <Card
                        title={
                          <Space>
                            <span>变更事件</span>
                            {events.length > 0 && <Tag color="blue">{events.length}</Tag>}
                          </Space>
                        }
                        extra={
                          <Button type="primary" icon={<PlusOutlined />} onClick={() => setEventModalOpen(true)}>
                            登记事件
                          </Button>
                        }
                      >
                        <Table
                          columns={[
                            { title: "事件类型", dataIndex: "event_type", width: 150, render: (v: string) => eventTypeLabels[v] || v },
                            { title: "生效日", dataIndex: "effective_date", width: 110, render: (v: string) => dayjs(v).format("YYYY-MM-DD") },
                            { title: "原值", dataIndex: "original_value", width: 120 },
                            { title: "新值", dataIndex: "new_value", width: 120 },
                            { title: "变更原因", dataIndex: "change_reason", ellipsis: true },
                            {
                              title: "状态",
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
                                  draft: "草稿",
                                  submitted: "已提交",
                                  reviewed: "已复核",
                                  approved: "已审批",
                                  rejected: "已驳回",
                                  returned_to_editor: "已退回",
                                };
                                return (
                                  <Tag color={eventStatusColors[v] || "default"}>
                                    {eventStatusLabels[v] || v}
                                  </Tag>
                                );
                              },
                            },
                            { title: "创建时间", dataIndex: "created_at", width: 150, render: (v: string) => dayjs(v).format("YYYY-MM-DD HH:mm") },
                            {
                              title: "操作",
                              key: "action",
                              width: 280,
                              render: (_: any, event: any) => {
                                const isModifiable = ["area_adjustment", "rent_change", "renewal", "early_termination", "index_update", "discount_rate_change", "impairment"].includes(event.event_type);
                                return (
                                <Space size="small">
                                  {event.approval_status === "approved" && isModifiable && (
                                    <Button size="small" onClick={() => handleViewAdjustment(event.id)}>查看调整</Button>
                                  )}
                                  {(event.approval_status === "submitted" || event.approval_status === "reviewed") && isModifiable && (
                                    <Button size="small" onClick={() => handlePreviewAdjustment(event.id)} loading={previewLoading}>预览影响</Button>
                                  )}
                                  {event.approval_status === "draft" && isModifiable && (user?.role === "editor" || user?.role === "admin") && (
                                    <Button size="small" onClick={() => handleRecalculateEvent(event.id)}>重算</Button>
                                  )}
                                  {user && event.approval_status === 'draft' && (user.role === 'editor' || user.role === 'admin') && (
                                    <Button
                                      size="small"
                                      type="primary"
                                      onClick={() => handleEventSubmitForReview(event.id)}
                                      loading={eventActionLoading === event.id + '_submit'}
                                    >
                                      提交复核
                                    </Button>
                                  )}
                                  {user && event.approval_status === 'submitted' && (user.role === 'reviewer' || user.role === 'admin') && (
                                    <>
                                      <Button
                                        size="small"
                                        type="primary"
                                        style={{ background: '#10B981', borderColor: '#10B981' }}
                                        onClick={() => handleEventReviewApprove(event.id)}
                                        loading={eventActionLoading === event.id + '_review'}
                                      >
                                        复核通过
                                      </Button>
                                      <Button
                                        size="small"
                                        danger
                                        onClick={() => handleEventRejectOpen(event.id, 'review')}
                                      >
                                        退回编辑
                                      </Button>
                                    </>
                                  )}
                                  {user && event.approval_status === 'reviewed' && (user.role === 'approver' || user.role === 'admin') && (
                                    <>
                                      <Button
                                        size="small"
                                        type="primary"
                                        style={{ background: '#10B981', borderColor: '#10B981' }}
                                        onClick={() => handleEventApprove(event.id)}
                                        loading={eventActionLoading === event.id + '_approve'}
                                      >
                                        审批通过
                                      </Button>
                                      <Button
                                        size="small"
                                        danger
                                        onClick={() => handleEventRejectOpen(event.id, 'approve')}
                                      >
                                        驳回
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
                                      重新提交
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
                          locale={{ emptyText: "暂无事件记录" }}
                        />
                      </Card>
                    ),
                  },
                  {
                    key: "calculation",
                    label: "IFRS 16 计算",
                    children: calcResult ? (
                      <>
                        <Row gutter={16} style={{ marginBottom: 16 }}>
                          <Col span={8}>
                            <Card>
                              <Statistic
                                title="初始租赁负债"
                                value={calcResult.initial_liability}
                                precision={2}
                                prefix="¥"
                              />
                            </Card>
                          </Col>
                          <Col span={8}>
                            <Card>
                              <Statistic
                                title="初始使用权资产"
                                value={calcResult.initial_rou_asset}
                                precision={2}
                                prefix="¥"
                              />
                            </Card>
                          </Col>
                          <Col span={8}>
                            <Card>
                              <Statistic
                                title="租赁总天数"
                                value={calcResult.total_days}
                                suffix="天"
                              />
                            </Card>
                          </Col>
                        </Row>
                        <Card title="月度摊销表">
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
                          <CalculatorOutlined style={{ fontSize: 48, color: "#ccc" }} />
                          <p style={{ marginTop: 16, color: "#999" }}>
                            点击上方「IFRS 16 计算」按钮生成摊销表
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

        <Modal
          title="添加付款计划"
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
                  label="付款日"
                  name="due_date"
                  rules={[{ required: true, message: "请选择付款日" }]}
                >
                  <DatePicker style={{ width: "100%" }} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label="金额"
                  name="amount"
                  rules={[{ required: true, message: "请输入金额" }]}
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
                <Form.Item label="币种" name="currency">
                  <Select>
                    <Select.Option value="CNY">人民币 (CNY)</Select.Option>
                    <Select.Option value="USD">美元 (USD)</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label="付款时点"
                  name="payment_timing"
                  rules={[{ required: true }]}
                >
                  <Select>
                    <Select.Option value="prepaid">
                      先付 (Prepaid)
                    </Select.Option>
                    <Select.Option value="postpaid">
                      后付 (Postpaid)
                    </Select.Option>
                  </Select>
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  label="生效开始日"
                  name="effective_start_date"
                >
                  <DatePicker style={{ width: "100%" }} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label="生效结束日"
                  name="effective_end_date"
                >
                  <DatePicker style={{ width: "100%" }} />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label="租期结束日"
                  name="lease_end_date"
                  rules={[{ required: true, message: "请选择租期结束日" }]}
                >
                  <DatePicker style={{ width: "100%" }} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="折现率类型" name="discount_rate_type">
                  <Input placeholder="例如：incremental_borrowing_rate" />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  label="折现率数值 (%)"
                  name="discount_rate_value"
                  help="可直接填写年化折现率，填写 5 会自动按 5% 处理。"
                >
                  <InputNumber style={{ width: "100%" }} min={0} step={0.01} placeholder="例如 5 或 5.25" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="折现率版本" name="discount_rate_version">
                  <Input placeholder="例如：v1.0" />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  label="标签"
                  name="tags"
                  tooltip="用于报表按标签汇总，例如 #华东 #直营 #旗舰店"
                >
                  <Select
                    mode="tags"
                    tokenSeparators={[",", "，", ";", "；", " ", "|"]}
                    placeholder="输入标签后回车，例如 #华东、#直营、#旗舰店"
                    options={DEFAULT_TAG_SUGGESTIONS.map((tag) => ({
                      value: tag,
                      label: tag,
                    }))}
                  />
                </Form.Item>
              </Col>
            </Row>

            <Form.Item label="金额类型" name="amount_type">
              <Input placeholder="例如：fixed_rent, turnover_rent, CAM" />
            </Form.Item>

            <Row gutter={16}>
              <Col span={8}>
                <Form.Item
                  label="固定租金"
                  name="is_fixed"
                  valuePropName="checked"
                >
                  <Select>
                    <Select.Option value={true}>是</Select.Option>
                    <Select.Option value={false}>否</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item
                  label="租赁成分"
                  name="is_lease_component"
                  valuePropName="checked"
                >
                  <Select>
                    <Select.Option value={true}>是</Select.Option>
                    <Select.Option value={false}>否</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item
                  label="计入负债"
                  name="included_in_liability_pv"
                  valuePropName="checked"
                >
                  <Select>
                    <Select.Option value={true}>是</Select.Option>
                    <Select.Option value={false}>否</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
            </Row>
          </Form>
        </Modal>

        <Modal
          title="登记变更事件"
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
              label="事件类型"
              name="event_type"
              rules={[{ required: true, message: "请选择事件类型" }]}
            >
              <Select placeholder="选择事件类型">
                <Select.Option value="area_adjustment">面积调整</Select.Option>
                <Select.Option value="rent_change">租金变更</Select.Option>
                <Select.Option value="renewal">续租</Select.Option>
                <Select.Option value="early_termination">提前终止</Select.Option>
                <Select.Option value="index_update">指数更新</Select.Option>
                <Select.Option value="discount_rate_change">折现率变更</Select.Option>
                <Select.Option value="impairment">减值</Select.Option>
              </Select>
            </Form.Item>

            <Form.Item
              label="生效日期"
              name="effective_date"
              rules={[{ required: true, message: "请选择生效日期" }]}
            >
              <DatePicker style={{ width: "100%" }} />
            </Form.Item>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label="原值" name="original_value">
                  <Input placeholder="变更前的值" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="新值" name="new_value">
                  <Input placeholder="变更后的值" />
                </Form.Item>
              </Col>
            </Row>

            <Form.Item
              label="变更原因"
              name="change_reason"
              rules={[{ required: true, message: "请输入变更原因" }]}
            >
              <Input.TextArea rows={2} placeholder="例如：根据合同补充协议第3条，租金调整为每月12万元" />
            </Form.Item>

            <Form.Item label="判断依据" name="judgment_basis">
              <Input.TextArea rows={2} placeholder="例如：合同补充协议签署日期 2024-06-15" />
            </Form.Item>
          </Form>
        </Modal>

        <Modal
          title="编辑合同"
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
                  label="合同编号"
                  name="contract_number"
                  rules={[{ required: true, message: "请输入合同编号" }]}
                >
                  <Input />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label="合同名称"
                  name="contract_name"
                  rules={[{ required: true, message: "请输入合同名称" }]}
                >
                  <Input />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label="承租方" name="lessee_name">
                  <Input />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="出租方" name="lessor_name">
                  <Input />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label="门店名称" name="store_name">
                  <Input />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="门店地址" name="store_address">
                  <Input />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  label="币种"
                  name="currency"
                  rules={[{ required: true, message: "请选择币种" }]}
                >
                  <Select>
                    <Select.Option value="CNY">人民币 (CNY)</Select.Option>
                    <Select.Option value="USD">美元 (USD)</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="签约日期" name="signing_date">
                  <DatePicker style={{ width: "100%" }} />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  label="租赁起始日"
                  name="commencement_date"
                  rules={[{ required: true, message: "请选择租赁起始日" }]}
                >
                  <DatePicker style={{ width: "100%" }} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label="租赁开始日"
                  name="lease_start_date"
                  rules={[{ required: true, message: "请选择租赁开始日" }]}
                >
                  <DatePicker style={{ width: "100%" }} />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  label="租期结束日"
                  name="lease_end_date"
                  rules={[{ required: true, message: "请选择租期结束日" }]}
                >
                  <DatePicker style={{ width: "100%" }} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="折现率类型" name="discount_rate_type">
                  <Input placeholder="例如：incremental_borrowing_rate" />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label="折现率版本" name="discount_rate_version">
                  <Input placeholder="例如：v1.0" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label="标签"
                  name="tags"
                  tooltip="用于报表按标签汇总，例如 #华东 #直营 #旗舰店"
                >
                  <Select
                    mode="tags"
                    tokenSeparators={[",", "，", ";", "；", " ", "|"]}
                    placeholder="输入标签后回车，例如 #华东、#直营、#旗舰店"
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
          title={rejectModalType === 'review' ? '退回编辑' : '驳回'}
          open={rejectModalOpen}
          onCancel={() => setRejectModalOpen(false)}
          onOk={handleRejectSubmit}
          confirmLoading={actionLoading === (rejectModalType === 'review' ? 'review_reject' : 'approve_reject')}
          okText="确认"
          cancelText="取消"
        >
          <p style={{ marginBottom: 8 }}>
            {rejectModalType === 'review' ? '请输入退回编辑的原因：' : '请输入驳回的原因：'}
          </p>
          <Input.TextArea
            rows={4}
            value={rejectReason}
            onChange={(e) => setRejectReason(e.target.value)}
            placeholder="请输入原因..."
          />
        </Modal>

        <Modal
          title={eventRejectType === 'review' ? '退回编辑' : '驳回'}
          open={eventRejectModalOpen}
          onCancel={() => setEventRejectModalOpen(false)}
          onOk={handleEventRejectSubmit}
          confirmLoading={eventRejectEventId ? eventActionLoading === eventRejectEventId + '_reject' : false}
          okText="确认"
          cancelText="取消"
        >
          <p style={{ marginBottom: 8 }}>
            {eventRejectType === 'review' ? '请输入退回编辑的原因：' : '请输入驳回的原因：'}
          </p>
          <Input.TextArea
            rows={4}
            value={eventRejectReason}
            onChange={(e) => setEventRejectReason(e.target.value)}
            placeholder="请输入原因..."
          />
        </Modal>

        <Modal
          title="编辑草稿行"
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
                  label="付款日"
                  name="due_date"
                  rules={[{ required: true, message: "请选择付款日" }]}
                >
                  <DatePicker style={{ width: "100%" }} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label="金额"
                  name="amount"
                  rules={[{ required: true, message: "请输入金额" }]}
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
              label="付款时点"
              name="payment_timing"
              rules={[{ required: true }]}
            >
              <Select>
                <Select.Option value="prepaid">先付 (Prepaid)</Select.Option>
                <Select.Option value="postpaid">后付 (Postpaid)</Select.Option>
              </Select>
            </Form.Item>

            <Form.Item label="金额类型" name="amount_type">
              <Input placeholder="例如：fixed_rent, turnover_rent, CAM" />
            </Form.Item>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label="固定租金" name="is_fixed">
                  <Select>
                    <Select.Option value={true}>是</Select.Option>
                    <Select.Option value={false}>否</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="租赁成分" name="is_lease_component">
                  <Select>
                    <Select.Option value={true}>是</Select.Option>
                    <Select.Option value={false}>否</Select.Option>
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
            <Button key="close" onClick={() => setAdjustmentModalOpen(false)}>关闭</Button>,
          ]}
          width={700}
        >
          {adjustmentModalData && (
            <Descriptions column={1} bordered size="small">
              <Descriptions.Item label="调整类型">
                <Tag color="blue">{adjustmentModalData.adjustment_type || "-"}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label="生效日期">
                {adjustmentModalData.effective_date ? dayjs(adjustmentModalData.effective_date).format("YYYY-MM-DD") : "-"}
              </Descriptions.Item>
              <Descriptions.Item label="折现率">
                {adjustmentModalData.discount_rate != null ? `${(adjustmentModalData.discount_rate * 100).toFixed(2)}%` : "-"}
              </Descriptions.Item>
              <Descriptions.Item label="调整前租赁负债">
                {adjustmentModalData.liability_before != null ? `¥${adjustmentModalData.liability_before.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}` : "-"}
              </Descriptions.Item>
              <Descriptions.Item label="调整后租赁负债">
                {adjustmentModalData.liability_after != null ? (
                  <span style={{ fontWeight: "bold", color: "#1677ff" }}>¥{adjustmentModalData.liability_after.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}</span>
                ) : "-"}
              </Descriptions.Item>
              <Descriptions.Item label="负债变动额">
                {adjustmentModalData.liability_change != null ? (
                  <span style={{ fontWeight: "bold", color: adjustmentModalData.liability_change >= 0 ? "#cf1322" : "#389e0d" }}>
                    {adjustmentModalData.liability_change >= 0 ? "+" : ""}¥{adjustmentModalData.liability_change.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                  </span>
                ) : "-"}
              </Descriptions.Item>
              <Descriptions.Item label="调整前使用权资产">
                {adjustmentModalData.asset_before != null ? `¥${adjustmentModalData.asset_before.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}` : "-"}
              </Descriptions.Item>
              <Descriptions.Item label="调整后使用权资产">
                {adjustmentModalData.asset_after != null ? (
                  <span style={{ fontWeight: "bold", color: "#1677ff" }}>¥{adjustmentModalData.asset_after.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}</span>
                ) : "-"}
              </Descriptions.Item>
              <Descriptions.Item label="资产变动额">
                {adjustmentModalData.asset_change != null ? (
                  <span style={{ fontWeight: "bold", color: adjustmentModalData.asset_change >= 0 ? "#cf1322" : "#389e0d" }}>
                    {adjustmentModalData.asset_change >= 0 ? "+" : ""}¥{adjustmentModalData.asset_change.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                  </span>
                ) : "-"}
              </Descriptions.Item>
              <Descriptions.Item label="损益影响 (PnL)">
                {adjustmentModalData.pnl_impact != null ? (
                  <span style={{ fontWeight: "bold", color: adjustmentModalData.pnl_impact >= 0 ? "#389e0d" : "#cf1322" }}>
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
