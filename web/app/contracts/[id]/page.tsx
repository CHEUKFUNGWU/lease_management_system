"use client";

import { StatusTag, statusKindFromAntColor } from "../../components/StatusTag";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import {
  Card,
  Descriptions,
  Tag,
  Button,
  Table,
  Form,
  Spin,
  Divider,
  Space,
  Row,
  Col,
  Statistic,
  Tabs,
  Timeline,
  Alert,
  message,
} from "antd";
import {
  ArrowLeftOutlined,
  PlusOutlined,
  CalculatorOutlined,
  FileTextOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  CloseCircleOutlined,
  RobotOutlined,
  EditOutlined,
} from "@ant-design/icons";
import AppLayout from "../../components/AppLayout";
import PageHeader from "../../components/PageHeader";
import ProtectedRoute from "../../components/ProtectedRoute";
import { hasRole, useAuth } from "../../context/AuthContext";
import { useLanguage } from "../../context/LanguageContext";
import { t } from "../../lib/i18n";
import { fmtMoney } from "../../lib/format";
import { RenewalCard } from "./RenewalCard";
import dayjs from "dayjs";
import { motion } from "framer-motion";
import { buildContractEditValues } from "./workspace/forms";
import { ContractWorkspaceDialogs } from "./workspace/ContractWorkspaceDialogs";
import type {
  CriticalDate,
  LeaseObligation,
  MonthlyEntry,
  PaymentSchedule,
} from "./workspace/types";
import { useContractWorkspace } from "./workspace/useContractWorkspace";
import { applyScheduleFill, type ScheduleFillSummary } from "./workspace/scheduleFill";
import { apiRequest } from "../../lib/api";
import { tableScrollX } from "../../lib/tableScroll";

export default function ContractDetailPage() {
  const params = useParams();
  const router = useRouter();
  const { token, user } = useAuth();
  const { language } = useLanguage();
  const contractId = params.id as string;
  const canEdit = hasRole(user, "editor") || hasRole(user, "admin");
  const canReview = hasRole(user, "reviewer") || hasRole(user, "admin");
  const canApprove = hasRole(user, "approver") || hasRole(user, "admin");

  const [form] = Form.useForm();
  const [eventForm] = Form.useForm();
  const [editForm] = Form.useForm();
  const [criticalDateForm] = Form.useForm();
  const [documentForm] = Form.useForm();
  const [obligationForm] = Form.useForm();

  const { state, dispatch, execute } = useContractWorkspace({
    contractId,
    token,
    language,
  });
  const {
    contract,
    schedules,
    calculation: calcResult,
    events,
    criticalDates,
    documents,
    obligations,
    activeTab,
    loading: workspaceLoading,
  } = state;

  // agent-universal-pagefill-v1 P0-B①：Agent 产出的付款计划预填经
  // ?schedule_fill=<artifactId> 深链落到本页。payload 只含人提供的信封字段，
  // first_row 建议进表单前先过 applyScheduleFill 的 target_page/合同校验；
  // 预填永远只是表单默认值，提交（写库）仍由人点「保存」触发。
  const [scheduleFillNotice, setScheduleFillNotice] = useState<ScheduleFillSummary | null>(null);
  useEffect(() => {
    if (!token) return;
    const searchParams = new URLSearchParams(window.location.search);
    const fillId = searchParams.get("schedule_fill");
    if (!fillId) return;
    let active = true;
    (async () => {
      try {
        const body = await apiRequest<{ artifact?: { data?: unknown } }>(
          `/api/v1/ai/chat/artifacts/${encodeURIComponent(fillId)}`,
          { token },
        );
        if (!active) return;
        const result = applyScheduleFill(body?.artifact?.data, contractId);
        if (!result.ok) {
          message.warning(t("contract.schedule_fill_refused", language));
          return;
        }
        form.setFieldsValue({
          due_date: result.formValues.due_date ? dayjs(result.formValues.due_date) : undefined,
          amount: result.formValues.amount,
          currency: result.formValues.currency,
          payment_timing: result.formValues.payment_timing,
          effective_start_date: result.formValues.effective_start_date ? dayjs(result.formValues.effective_start_date) : undefined,
          effective_end_date: result.formValues.effective_end_date ? dayjs(result.formValues.effective_end_date) : undefined,
        });
        setScheduleFillNotice(result.summary);
        dispatch({ type: "dialog.open", dialog: "schedule" });
      } catch {
        if (active) message.warning(t("contract.schedule_fill_refused", language));
      }
    })();
    return () => {
      active = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- 深链只在挂载时消费一次；form/dispatch 随首挂稳定。
  }, [token, contractId]);

  const loading = workspaceLoading.initial;
  const calcLoading = workspaceLoading.calculation;
  const eventActionLoading = workspaceLoading.eventCommand;
  const previewLoading = workspaceLoading.adjustment;
  const actionLoadingAliases: Record<string, string> = {
    "contract.submit": "submit",
    "contract.review.approve": "review_approve",
    "contract.approve": "approve",
  };
  const actionLoading = workspaceLoading.command
    ? actionLoadingAliases[workspaceLoading.command] || workspaceLoading.command
    : null;
  const setActiveTab = (tab: string) => dispatch({ type: "tab.select", tab });

  const handleUpdateCriticalDateStatus = (dateId: string, status: string) =>
    execute({ type: "criticalDate.status", dateId, status });

  const handleUpdateObligationStatus = (obligationId: string, status: string) =>
    execute({ type: "obligation.status", obligationId, status });

  const handleEventSubmitForReview = (eventId: string) =>
    execute({ type: "event.submit", eventId });
  const handleEventReviewApprove = (eventId: string) =>
    execute({ type: "event.review.approve", eventId });
  const handleEventApprove = (eventId: string) =>
    execute({ type: "event.approve", eventId });
  const handleEventRejectOpen = (eventId: string, stage: "review" | "approve") =>
    dispatch({ type: "event.reject.open", eventId, stage });
  const handlePreviewAdjustment = (eventId: string) =>
    execute({ type: "event.adjustment.preview", eventId });
  const handleViewAdjustment = (eventId: string) =>
    execute({ type: "event.adjustment.view", eventId });
  const handleRecalculateEvent = (eventId: string) =>
    execute({ type: "event.recalculate", eventId });

  const handleSubmitForReview = () => execute({ type: "contract.submit" });
  const handleReviewApprove = () => execute({ type: "contract.review.approve" });
  const handleApprove = () => execute({ type: "contract.approve" });
  const handleCalculate = () => execute({ type: "contract.calculate" });

  const handleEditOpen = () => {
    if (!contract) return;
    editForm.setFieldsValue(buildContractEditValues(contract));
    dispatch({ type: "dialog.open", dialog: "contractEdit" });
  };

  const openPaymentScheduleAgent = () => {
    const title = contract
      ? `${contract.contract_number} ${contract.contract_name}`
      : t("contract.tab_payments", language);
    const search = new URLSearchParams({
      page: "contract-detail",
      contract_id: contractId,
      title,
      summary: t("contract_detail.agent_payment_summary", language),
    });
    router.push(`/ai-chat?${search.toString()}`);
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

  const leaseScopeLabels: Record<string, string> = {
    in_scope: "资本化租赁",
    short_term_exempt: "短期豁免",
    low_value_exempt: "低价值豁免",
    not_a_lease: "非租赁",
  };

  const leaseScopeColors: Record<string, string> = {
    in_scope: "blue",
    short_term_exempt: "gold",
    low_value_exempt: "purple",
    not_a_lease: "default",
  };

  // P1-D（UIUX 审查报告 2026-08-21）：原为硬编码中文映射，与录入表单共用
  // contract.asset_* 键组；值是 i18n key，渲染时经 t() 解析。
  const assetTypeLabels: Record<string, string> = {
    real_estate: "contract.asset_real_estate",
    vehicle: "contract.asset_vehicle",
    it_equipment: "contract.asset_it_equipment",
    machinery: "contract.asset_machinery",
    other: "contract.asset_other",
  };

  const criticalDateKeys: Record<string, string> = {
    renewal_deadline: "critical_date.renewal_deadline",
    break_notice: "critical_date.break_notice",
    rent_review: "critical_date.rent_review",
    lease_expiry: "critical_date.lease_expiry",
    insurance_renewal: "critical_date.insurance_renewal",
    other: "critical_date.other",
  };

  const criticalStatusColors: Record<string, string> = {
    open: "processing",
    snoozed: "warning",
    completed: "success",
    cancelled: "default",
  };

  const obligationTypeLabels: Record<string, string> = {
    maintenance: "维修维护",
    cam: "CAM / 管理费",
    insurance: "保险",
    index_adjustment: "指数调整",
    restoration: "复原义务",
    security_deposit: "押金",
    notice: "通知义务",
    other: "其他",
  };

  const responsiblePartyLabels: Record<string, string> = {
    lessee: "承租方",
    lessor: "出租方",
    shared: "双方共同",
    third_party: "第三方",
  };

  const obligationStatusColors: Record<string, string> = {
    active: "processing",
    completed: "success",
    waived: "warning",
    cancelled: "default",
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
      render: (v: number, r: PaymentSchedule) => fmtMoney(v, r.currency || contract?.currency),
    },
    { title: t("contract.currency", language), dataIndex: "currency", width: 80 },
    {
      title: t("contract.payment_timing", language),
      dataIndex: "payment_timing",
      width: 90,
      render: (v: string) =>
        v === "prepaid" ? (
          <StatusTag kind="processing">{t("contract.prepaid", language)}</StatusTag>
        ) : (
          <StatusTag kind="success">{t("contract.postpaid", language)}</StatusTag>
        ),
    },
    { title: t("contract.amount_type", language), dataIndex: "amount_type", width: 120 },
    {
      title: `${t("contract.fixed", language)}/${t("contract.variable", language)}`,
      width: 100,
      render: (_: any, r: PaymentSchedule) => (
        <Space>
          {r.is_fixed && <StatusTag>{t("contract.fixed", language)}</StatusTag>}
          {r.is_variable && <StatusTag kind="warning">{t("contract.variable", language)}</StatusTag>}
        </Space>
      ),
    },
    {
      title: t("contract.lease_component", language),
      width: 90,
      render: (_: any, r: PaymentSchedule) =>
        r.is_lease_component ? <StatusTag kind="success">{t("contract.yes", language)}</StatusTag> : <StatusTag>{t("contract.no", language)}</StatusTag>,
    },
    {
      title: t("contract.include_liability", language),
      width: 90,
      render: (_: any, r: PaymentSchedule) =>
        r.included_in_liability_pv ? (
          <StatusTag kind="success">{t("contract.yes", language)}</StatusTag>
        ) : (
          <StatusTag>{t("contract.no", language)}</StatusTag>
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
      render: (v: number) => fmtMoney(v, contract?.currency),
      align: "right" as const,
    },
    {
      title: t("contract.interest_expense", language),
      dataIndex: "InterestExpense",
      render: (v: number) => fmtMoney(v, contract?.currency),
      align: "right" as const,
    },
    {
      title: t("contract.payment", language),
      dataIndex: "TotalPayments",
      render: (v: number) => fmtMoney(v, contract?.currency),
      align: "right" as const,
    },
    {
      title: t("contract.closing_liability", language),
      dataIndex: "ClosingLiability",
      render: (v: number) => fmtMoney(v, contract?.currency),
      align: "right" as const,
    },
    {
      title: t("contract.opening_rou", language),
      dataIndex: "OpeningROUAsset",
      render: (v: number) => fmtMoney(v, contract?.currency),
      align: "right" as const,
    },
    {
      title: t("contract.depreciation", language),
      dataIndex: "Depreciation",
      render: (v: number) => fmtMoney(v, contract?.currency),
      align: "right" as const,
    },
    {
      title: t("contract.closing_rou", language),
      dataIndex: "ClosingROUAsset",
      render: (v: number) => fmtMoney(v, contract?.currency),
      align: "right" as const,
    },
    {
      title: t("contract.variable_rent", language),
      dataIndex: "VariableRentExpense",
      render: (v: number) => fmtMoney(v, contract?.currency),
      align: "right" as const,
    },
    {
      title: "豁免费用",
      dataIndex: "ExemptLeaseExpense",
      render: (v: number) => fmtMoney(v || 0, contract?.currency),
      align: "right" as const,
    },
    {
      title: t("contract.non_lease_expense", language),
      dataIndex: "NonLeaseExpense",
      render: (v: number) => fmtMoney(v, contract?.currency),
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
        <PageHeader
          title={<>{contract?.contract_name || t("contract.detail_title", language)}{contract?.contract_number && <span className="page-header-count">{contract.contract_number}</span>}</>}
          secondaryAction={
            <Button
              type="text"
              icon={<ArrowLeftOutlined />}
              onClick={() => router.push("/contracts")}
            >
              {t("contract.back_to_list", language)}
            </Button>
          }
        />

        <motion.div
          initial={false}
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
                        <StatusTag kind={statusKindFromAntColor(statusColors[contract.approval_status])}>
                          {statusLabels[contract.approval_status]}
                        </StatusTag>
                        {contract.is_official_version && (
                          <StatusTag kind="processing">{t("contracts.official", language)}</StatusTag>
                        )}
                      </Space>
                    }
                    extra={
                      <Space>
                        {/* Approval workflow buttons */}
                        {contract.approval_status === 'draft' && canEdit && (
                          <Button
                            type="primary"
                            onClick={handleSubmitForReview}
                            loading={actionLoading === 'submit'}
                          >
                            {t("contract.submit_review", language)}
                          </Button>
                        )}

                        {contract.approval_status === 'submitted' && canReview && (
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
                              onClick={() => dispatch({ type: "contract.reject.open", stage: "review" })}
                            >
                              {t("contract.return_editor", language)}
                            </Button>
                          </>
                        )}

                        {(contract.approval_status === 'reviewed' || contract.approval_status === 'pending_approval') && canApprove && (
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
                              onClick={() => dispatch({ type: "contract.reject.open", stage: "approve" })}
                            >
                              {t("contract.reject", language)}
                            </Button>
                          </>
                        )}

                        {contract.approval_status === 'rejected' && canEdit && (
                          <Button
                            type="primary"
                            onClick={handleSubmitForReview}
                            loading={actionLoading === 'submit'}
                          >
                            {t("contract.resubmit", language)}
                          </Button>
                        )}

                        {(contract.approval_status === 'draft' || contract.approval_status === 'rejected') && canEdit && (
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
                      <Descriptions.Item label={t("contract.asset_type", language)}>
                        <StatusTag>{t(assetTypeLabels[contract.asset_type || "real_estate"], language)}</StatusTag>
                      </Descriptions.Item>
                      <Descriptions.Item label={t("contract_detail.area_sqm", language)}>
                        {contract.area_sqm != null ? `${Number(contract.area_sqm).toLocaleString()} ㎡` : "-"}
                      </Descriptions.Item>
                      <Descriptions.Item label={t("contract.discount_rate", language)}>
                        {contract.discount_rate_value != null ? (
                          <StatusTag kind="success">{(contract.discount_rate_value * 100).toFixed(2)}%</StatusTag>
                        ) : contract.discount_rate_missing ? (
                          <StatusTag kind="error">{t("contracts.missing", language)}</StatusTag>
                        ) : (
                          <StatusTag kind="success">
                            {contract.discount_rate_type} / {contract.discount_rate_version}
                          </StatusTag>
                        )}
                      </Descriptions.Item>
                      <Descriptions.Item label="IFRS 16 范围">
                        <StatusTag kind={statusKindFromAntColor(leaseScopeColors[contract.lease_scope || "in_scope"])}>
                          {leaseScopeLabels[contract.lease_scope || "in_scope"]}
                        </StatusTag>
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
                      {contract.exemption_reason && (
                        <Descriptions.Item label="范围依据" span={3}>
                          {contract.exemption_reason}
                        </Descriptions.Item>
                      )}
                    </Descriptions>
                  </Card>
                </Col>

                <Col span={6}>
                  <Card title={t("contract.approval_progress", language)} size="small">
                    <Timeline
                      items={[
                        {
                          dot: <ClockCircleOutlined />,
                          color: contract.created_at ? "var(--fg-primary)" : "var(--border-strong)",
                          children: `${t("contract.created", language)} ${dayjs(contract.created_at).format("YYYY-MM-DD")}`,
                        },
                        {
                          dot: <FileTextOutlined />,
                          color: contract.submitted_at ? "var(--fg-primary)" : "var(--border-strong)",
                          children: contract.submitted_at
                            ? `${t("contract.submitted", language)} ${dayjs(contract.submitted_at).format("YYYY-MM-DD")}`
                            : `${t("contract.pending", language)}${t("contract.submitted", language)}`,
                        },
                        {
                          dot: <CheckCircleOutlined />,
                          color: contract.reviewed_at ? "var(--fg-primary)" : "var(--border-strong)",
                          children: contract.reviewed_at
                            ? `${t("contract.reviewed", language)} ${dayjs(contract.reviewed_at).format("YYYY-MM-DD")}`
                            : `${t("contract.pending", language)}${t("contract.reviewed", language)}`,
                        },
                        {
                          dot: <CheckCircleOutlined />,
                          color: contract.approved_at ? "var(--fg-primary)" : "var(--border-strong)",
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
                className="contract-block-gap"
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
                          <StatusTag kind="processing">{schedules.length} {t("contract_detail.item_unit", language)}</StatusTag>
                        )}
                      </Space>
                    }
                    extra={
                      <Space>
                        <Button
                          icon={<RobotOutlined />}
                          onClick={openPaymentScheduleAgent}
                        >
                          {t("contract.ai_agent_intake", language)}
                        </Button>
                        <Button
                          type="primary"
                          icon={<PlusOutlined />}
                          onClick={() => dispatch({ type: "dialog.open", dialog: "schedule" })}
                        >
                          {t("contract.manual_add", language)}
                        </Button>
                      </Space>
                    }
                  >
                    {scheduleFillNotice && (
                      <Alert
                        className="contract-block-gap"
                        type="warning"
                        showIcon
                        closable
                        onClose={() => setScheduleFillNotice(null)}
                        message={t("contract.schedule_fill_title", language)}
                        description={t("contract.schedule_fill_desc", language, {
                          valid: String(scheduleFillNotice.valid_rows),
                          total: scheduleFillNotice.total_amount != null ? scheduleFillNotice.total_amount.toFixed(2) : "—",
                          range:
                            scheduleFillNotice.min_due_date && scheduleFillNotice.max_due_date
                              ? `${scheduleFillNotice.min_due_date} ~ ${scheduleFillNotice.max_due_date}`
                              : "—",
                        })}
                      />
                    )}
                    <Table
                      columns={scheduleColumns}
                      dataSource={schedules}
                      rowKey="id"
                      pagination={{ pageSize: 12 }}
                      size="small"
                      scroll={tableScrollX((schedules || []).length, 900)}
                      locale={{ emptyText: t("contract.no_schedules", language) }}
                    />
                  </Card>
                    ),
                  },
                  {
                    key: "critical_dates",
                    label: `关键日期 (${criticalDates.length})`,
                    children: (
                      <Card
                        title={
                          <Space>
                            <span>关键日期与提醒</span>
                            {criticalDates.filter((d) => d.status === "open").length > 0 && (
                              <StatusTag kind="processing">{criticalDates.filter((d) => d.status === "open").length} 待处理</StatusTag>
                            )}
                          </Space>
                        }
                        extra={
                          <Button type="primary" icon={<PlusOutlined />} onClick={() => dispatch({ type: "dialog.open", dialog: "criticalDate" })}>
                            新增关键日期
                          </Button>
                        }
                      >
                        <Table
                          columns={[
                            {
                              title: "类型",
                              dataIndex: "date_type",
                              width: 130,
                              render: (v: string) => t(criticalDateKeys[v] || "critical_date.other", language),
                            },
                            { title: "标题", dataIndex: "title" },
                            {
                              title: "目标日期",
                              dataIndex: "target_date",
                              width: 130,
                              render: (v: string) => dayjs(v).format("YYYY-MM-DD"),
                            },
                            { title: "提前提醒", dataIndex: "reminder_days", width: 100, render: (v: number) => `${v} 天` },
                            {
                              title: "状态",
                              dataIndex: "status",
                              width: 100,
                              render: (v: string) => <StatusTag kind={statusKindFromAntColor(criticalStatusColors[v])}>{v}</StatusTag>,
                            },
                            {
                              title: "操作",
                              key: "action",
                              width: 150,
                              render: (_: any, record: CriticalDate) => (
                                <Space>
                                  {record.status !== "completed" && (
                                    <Button size="small" onClick={() => handleUpdateCriticalDateStatus(record.id, "completed")}>
                                      完成
                                    </Button>
                                  )}
                                  {record.status !== "cancelled" && (
                                    <Button size="small" danger onClick={() => handleUpdateCriticalDateStatus(record.id, "cancelled")}>
                                      取消
                                    </Button>
                                  )}
                                </Space>
                              ),
                            },
                          ]}
                          dataSource={criticalDates}
                          rowKey="id"
                          pagination={{ pageSize: 8 }}
                          size="small"
                        />
                        {/* A reminder that a lease expires is only half the job;
                            the decision it triggers belongs next to it. */}
                        <RenewalCard contractId={contractId} />
                      </Card>
                    ),
                  },
                  {
                    key: "documents",
                    label: `文档库 (${documents.length})`,
                    children: (
                      <Card
                        title="集中合同文档库"
                        extra={
                          <Button type="primary" icon={<PlusOutlined />} onClick={() => dispatch({ type: "dialog.open", dialog: "document" })}>
                            新增文档记录
                          </Button>
                        }
                      >
                        <Table
                          columns={[
                            { title: "文件名", dataIndex: "file_name" },
                            { title: "类型", dataIndex: "document_type", width: 130 },
                            { title: "版本", dataIndex: "document_version", width: 100, render: (v: string) => v || "-" },
                            {
                              title: "上传时间",
                              dataIndex: "uploaded_at",
                              width: 160,
                              render: (v: string) => dayjs(v).format("YYYY-MM-DD HH:mm"),
                            },
                            { title: "备注", dataIndex: "notes", render: (v: string) => v || "-" },
                          ]}
                          dataSource={documents}
                          rowKey="id"
                          pagination={{ pageSize: 8 }}
                          size="small"
                        />
                      </Card>
                    ),
                  },
                  {
                    key: "obligations",
                    label: `条款/义务 (${obligations.length})`,
                    children: (
                      <Card
                        title={
                          <Space>
                            <span>运营条款与义务</span>
                            {obligations.filter((item) => item.status === "active").length > 0 && (
                              <StatusTag kind="processing">{obligations.filter((item) => item.status === "active").length} 生效中</StatusTag>
                            )}
                          </Space>
                        }
                        extra={
                          <Button type="primary" icon={<PlusOutlined />} onClick={() => dispatch({ type: "dialog.open", dialog: "obligation" })}>
                            新增条款义务
                          </Button>
                        }
                      >
                        <Table
                          columns={[
                            {
                              title: "类型",
                              dataIndex: "obligation_type",
                              width: 130,
                              render: (v: string) => obligationTypeLabels[v] || v,
                            },
                            { title: "标题", dataIndex: "title", width: 180 },
                            {
                              title: "责任方",
                              dataIndex: "responsible_party",
                              width: 110,
                              render: (v: string) => responsiblePartyLabels[v] || v,
                            },
                            {
                              title: "状态",
                              dataIndex: "status",
                              width: 100,
                              render: (v: string) => <StatusTag kind={statusKindFromAntColor(obligationStatusColors[v])}>{v}</StatusTag>,
                            },
                            { title: "条款摘录", dataIndex: "source_clause", ellipsis: true, render: (v: string) => v || "-" },
                            { title: "页码", dataIndex: "source_page", width: 80, render: (v: number) => v || "-" },
                            {
                              title: "操作",
                              key: "action",
                              width: 210,
                              render: (_: any, record: LeaseObligation) => (
                                <Space>
                                  {record.status !== "completed" && (
                                    <Button size="small" onClick={() => handleUpdateObligationStatus(record.id, "completed")}>
                                      完成
                                    </Button>
                                  )}
                                  {record.status !== "waived" && (
                                    <Button size="small" onClick={() => handleUpdateObligationStatus(record.id, "waived")}>
                                      豁免
                                    </Button>
                                  )}
                                  {record.status !== "cancelled" && (
                                    <Button size="small" danger onClick={() => handleUpdateObligationStatus(record.id, "cancelled")}>
                                      取消
                                    </Button>
                                  )}
                                </Space>
                              ),
                            },
                          ]}
                          dataSource={obligations}
                          rowKey="id"
                          pagination={{ pageSize: 8 }}
                          size="small"
                          scroll={tableScrollX((obligations || []).length, 980)}
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
                            {events.length > 0 && <StatusTag kind="processing">{events.length}</StatusTag>}
                          </Space>
                        }
                        extra={
                          <Button type="primary" icon={<PlusOutlined />} onClick={() => dispatch({ type: "dialog.open", dialog: "event" })}>
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
                                  <StatusTag kind={statusKindFromAntColor(eventStatusColors[v] || "default")}>
                                    {eventStatusLabels[v] || v}
                                  </StatusTag>
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
                                  {event.approval_status === "draft" && isModifiable && canEdit && (
                                    <Button size="small" onClick={() => handleRecalculateEvent(event.id)}>{t("contract.recalculate", language)}</Button>
                                  )}
                                  {event.approval_status === 'draft' && canEdit && (
                                    <Button
                                      size="small"
                                      type="primary"
                                      onClick={() => handleEventSubmitForReview(event.id)}
                                      loading={eventActionLoading === event.id + '_submit'}
                                    >
                                      {t("contract.submit_review", language)}
                                    </Button>
                                  )}
                                  {event.approval_status === 'submitted' && canReview && (
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
                                  {event.approval_status === 'reviewed' && canApprove && (
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
                                  {event.approval_status === 'rejected' && canEdit && (
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
                          scroll={tableScrollX((events || []).length, 1000)}
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
                        <Alert
                          message={`计量路径：${calcResult.measurement_basis === "capitalized" ? "资本化计量" : calcResult.measurement_basis === "straight_line_expense" ? "豁免租赁直线法费用化" : "不进入 IFRS 16 计量"}`}
                          description={`范围判定：${leaseScopeLabels[calcResult.lease_scope || "in_scope"] || calcResult.lease_scope}`}
                          type={calcResult.measurement_basis === "capitalized" ? "info" : "warning"}
                          showIcon
                          className="contract-block-gap-bottom"
                        />
                        <Row gutter={16} className="contract-block-gap-bottom">
                          <Col span={8}>
                            <Card>
                              <Statistic
                                title={t("contract.initial_liability", language)}
                                value={calcResult.initial_liability}
                                precision={2}
                                prefix={contract?.currency || ""}
                              />
                            </Card>
                          </Col>
                          <Col span={8}>
                            <Card>
                              <Statistic
                                title={t("contract.initial_rou", language)}
                                value={calcResult.initial_rou_asset}
                                precision={2}
                                prefix={contract?.currency || ""}
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
                            scroll={tableScrollX((sortedMonthly || []).length, 1000)}
                          />
                        </Card>
                      </>
                    ) : (
                      <Card>
                        <div className="contract-empty-block">
                          <CalculatorOutlined className="contract-empty-icon" />
                          <p className="contract-empty-note">
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

        <ContractWorkspaceDialogs
          language={language}
          state={state}
          dispatch={dispatch}
          execute={execute}
          forms={{
            schedule: form,
            event: eventForm,
            edit: editForm,
            criticalDate: criticalDateForm,
            document: documentForm,
            obligation: obligationForm,
          }}
        />
      </AppLayout>
    </ProtectedRoute>
  );
}
