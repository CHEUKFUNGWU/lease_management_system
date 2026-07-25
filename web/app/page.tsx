"use client";

import { useEffect, useMemo, useState } from "react";
import { motion } from "framer-motion";
import { Button, Col, Row, Spin, Tag } from "antd";
import {
  BarChartOutlined,
  FileTextOutlined,
  PlusOutlined,
  RobotOutlined,
} from "@ant-design/icons";
import { useRouter } from "next/navigation";
import dayjs from "dayjs";
import AppLayout from "./components/AppLayout";
import ProtectedRoute from "./components/ProtectedRoute";
import { DashboardHeader, MoneyKPICard } from "./components/dashboard/DashboardCards";
import { ContractStatusCard, LiabilityTrendCard } from "./components/dashboard/DashboardCharts";
import {
  QuickActionsCard,
  RecentContractsCard,
  UpcomingDatesCard,
} from "./components/dashboard/DashboardLists";
import type {
  DashboardMoneyKPIs,
  DashboardQuickAction,
  DashboardRecentContract,
  DashboardStats,
  DashboardStatusDatum,
  DashboardUpcomingDate,
  LiabilityTrendPoint,
} from "./components/dashboard/types";
import { staggerContainer } from "./design-system/animations";
import { useAuth } from "./context/AuthContext";
import { useLanguage } from "./context/LanguageContext";
import { contractApi, leaseAdminApi, reportApi } from "./lib/api";
import { t, type Language } from "./lib/i18n";

const APPROVAL_STATUS_COLORS: Record<string, string> = {
  draft: "default",
  submitted: "processing",
  reviewed: "processing",
  pending_review: "processing",
  pending_approval: "warning",
  approved: "success",
  rejected: "error",
  returned_to_editor: "orange",
};

const APPROVAL_STATUS_I18N_KEYS: Record<string, string> = {
  draft: "status.draft",
  submitted: "status.submitted",
  reviewed: "status.reviewed",
  pending_review: "status.submitted",
  pending_approval: "status.pending_approval",
  approved: "status.approved",
  rejected: "status.rejected",
  returned_to_editor: "status.returned_to_editor",
};

function getApprovalStatusColor(status: string): string {
  return APPROVAL_STATUS_COLORS[status] || "default";
}

function getApprovalStatusLabel(status: string, language: Language): string {
  const key = APPROVAL_STATUS_I18N_KEYS[status];
  return key ? t(key, language) : status || t("status.draft", language);
}

function getErrorMessage(error: unknown): string | undefined {
  if (typeof error === "object" && error !== null && "message" in error) {
    const candidate = (error as { message?: unknown }).message;
    return typeof candidate === "string" ? candidate : undefined;
  }
  return undefined;
}

export default function HomePage() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const router = useRouter();
  const [loading, setLoading] = useState(true);
  const [stats, setStats] = useState<DashboardStats>({
    total: 0,
    approved: 0,
    pending: 0,
    draft: 0,
  });
  const [recentContracts, setRecentContracts] = useState<DashboardRecentContract[]>([]);
  const [upcomingDates, setUpcomingDates] = useState<DashboardUpcomingDate[]>([]);
  const [moneyKpis, setMoneyKpis] = useState<DashboardMoneyKPIs>({
    totalLiability: 0,
    totalROU: 0,
    monthExpense: 0,
    next12mCashOut: 0,
  });
  const [trendData, setTrendData] = useState<LiabilityTrendPoint[]>([]);
  const [trendLoading, setTrendLoading] = useState(true);

  const statusData = useMemo<DashboardStatusDatum[]>(() => {
    const items: DashboardStatusDatum[] = [
      { name: t("dashboard.approved", language), value: stats.approved, key: "approved" },
      { name: t("dashboard.pending", language), value: stats.pending, key: "pending" },
      { name: t("dashboard.draft", language), value: stats.draft, key: "draft" },
      {
        name: t("status.rejected", language),
        value: stats.total - stats.approved - stats.pending - stats.draft,
        key: "rejected",
      },
    ];
    return items.filter((item) => item.value > 0);
  }, [language, stats]);

  const quickActions = useMemo<DashboardQuickAction[]>(
    () => [
      {
        icon: <PlusOutlined />,
        label: t("dashboard.add_contract", language),
        description: t("dashboard.add_contract_desc", language),
        onClick: () => router.push("/contracts/new"),
      },
      {
        icon: <RobotOutlined />,
        label: t("dashboard.upload_file", language),
        description: t("dashboard.upload_file_desc", language),
        onClick: () => router.push("/ai-chat"),
      },
      {
        icon: <BarChartOutlined />,
        label: t("dashboard.view_report", language),
        description: t("dashboard.view_report_desc", language),
        onClick: () => router.push("/reports"),
      },
    ],
    [language, router]
  );

  useEffect(() => {
    if (!token) {
      return;
    }

    const loadDashboard = async () => {
      setLoading(true);
      try {
        const contractResponse = await contractApi.list(token);
        const contracts: DashboardRecentContract[] = contractResponse.data || [];
        const total = contractResponse.total || contracts.length;

        const approved = contracts.filter((contract) => contract.approval_status === "approved").length;
        const pending = contracts.filter((contract) =>
          contract.approval_status === "pending_review" ||
          contract.approval_status === "pending_approval" ||
          contract.approval_status === "submitted"
        ).length;
        const draft = contracts.filter(
          (contract) => contract.approval_status === "draft" || !contract.approval_status
        ).length;

        setStats({ total, approved, pending, draft });
        setRecentContracts(contracts.slice(0, 6));

        const reminderResponse = await leaseAdminApi.listUpcomingCriticalDates(token, { days: 90, limit: 8 });
        setUpcomingDates(reminderResponse.data || []);
      } catch (error) {
        console.error("Failed to load dashboard data:", error);
        const message = getErrorMessage(error);
        if (message) {
          console.warn(message);
        }
      } finally {
        setLoading(false);
      }
    };

    const loadFinancials = async () => {
      setTrendLoading(true);
      try {
        const start = dayjs().subtract(6, "month").startOf("month");
        const end = dayjs().add(12, "month").endOf("month");
        const res = await reportApi.amortization(
          {
            mode: "working",
            view: "summary",
            granularity: "month",
            start_date: start.format("YYYY-MM-DD"),
            end_date: end.format("YYYY-MM-DD"),
          },
          token
        );
        const rows: any[] = (res.data || []).slice().sort((a: any, b: any) =>
          String(a.period_key).localeCompare(String(b.period_key))
        );

        setTrendData(
          rows.map((row) => ({
            period: row.period_key,
            liability: row.closing_liability || 0,
            rou: row.closing_rou_asset || 0,
          }))
        );

        const currentKey = dayjs().format("YYYY-MM");
        const pastRows = rows.filter((row) => String(row.period_key) <= currentKey);
        const currentRow = pastRows.length ? pastRows[pastRows.length - 1] : null;
        const currentMonthRow = rows.find((row) => String(row.period_key) === currentKey);

        const next12mEnd = dayjs().add(11, "month").format("YYYY-MM");
        const next12mCashOut = rows
          .filter((row) => String(row.period_key) >= currentKey && String(row.period_key) <= next12mEnd)
          .reduce(
            (sum, row) =>
              sum + (row.payment || 0) + (row.variable_rent_expense || 0) + (row.non_lease_expense || 0),
            0
          );

        setMoneyKpis({
          totalLiability: currentRow?.closing_liability || 0,
          totalROU: currentRow?.closing_rou_asset || 0,
          monthExpense: (currentMonthRow?.interest_expense || 0) + (currentMonthRow?.depreciation || 0),
          next12mCashOut,
        });
      } catch (error) {
        console.error("Failed to load dashboard financials:", error);
      } finally {
        setTrendLoading(false);
      }
    };

    loadDashboard();
    loadFinancials();
  }, [token]);

  const getDateUrgency = (targetDate: string) => {
    const days = dayjs(targetDate).startOf("day").diff(dayjs().startOf("day"), "day");
    if (days < 0) {
      return { color: "error", text: t("dashboard.overdue_days", language, { days: String(Math.abs(days)) }) };
    }
    if (days <= 7) {
      return { color: "warning", text: t("dashboard.within_days", language, { days: String(days) }) };
    }
    return { color: "processing", text: t("dashboard.remaining_days", language, { days: String(days) }) };
  };

  const getStatusTag = (status: string) => (
    <Tag color={getApprovalStatusColor(status)}>{getApprovalStatusLabel(status, language)}</Tag>
  );

  return (
    <ProtectedRoute>
      <AppLayout>
        <DashboardHeader
          title={t("dashboard.title", language)}
          subtitle={t("dashboard.subtitle", language)}
          primaryAction={
            <Button
              icon={<PlusOutlined />}
              onClick={() => router.push("/contracts/new")}
              style={{ borderRadius: 9999, fontWeight: 500 }}
            >
              {t("dashboard.add_contract", language)}
            </Button>
          }
          secondaryAction={
            <Button
              icon={<RobotOutlined />}
              onClick={() => router.push("/ai-chat")}
              style={{ borderRadius: 9999, fontWeight: 500 }}
            >
              {t("dashboard.upload_file", language)}
            </Button>
          }
        />

        <Spin spinning={loading} tip={t("dashboard.loading", language)}>
          <motion.div variants={staggerContainer} initial="initial" animate="animate">
            <Row gutter={[16, 16]}>
              <Col xs={24} sm={12} lg={6}>
                <MoneyKPICard
                  title={t("dashboard.kpi_total_liability", language)}
                  value={moneyKpis.totalLiability}
                  loading={trendLoading}
                />
              </Col>
              <Col xs={24} sm={12} lg={6}>
                <MoneyKPICard
                  title={t("dashboard.kpi_total_rou", language)}
                  value={moneyKpis.totalROU}
                  loading={trendLoading}
                />
              </Col>
              <Col xs={24} sm={12} lg={6}>
                <MoneyKPICard
                  title={t("dashboard.kpi_month_expense", language)}
                  value={moneyKpis.monthExpense}
                  subtitle={t("dashboard.kpi_month_expense_sub", language)}
                  loading={trendLoading}
                />
              </Col>
              <Col xs={24} sm={12} lg={6}>
                <MoneyKPICard
                  title={t("dashboard.kpi_next12m_cashout", language)}
                  value={moneyKpis.next12mCashOut}
                  loading={trendLoading}
                />
              </Col>
            </Row>

            {!loading && (
              <div style={{ marginTop: 8, fontSize: 12, color: "#8C8C8C" }}>
                <FileTextOutlined style={{ marginRight: 6, fontSize: 12 }} />
                {t("dashboard.kpi_contracts_sub", language, {
                  total: String(stats.total),
                  approved: String(stats.approved),
                  pending: String(stats.pending),
                  draft: String(stats.draft),
                })}
              </div>
            )}

            <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
              <Col xs={24} lg={16}>
                <LiabilityTrendCard
                  language={language}
                  data={trendData}
                  loading={trendLoading}
                  onOpenReports={() => router.push("/reports")}
                />
              </Col>
              <Col xs={24} lg={8}>
                <ContractStatusCard statusData={statusData} language={language} />
              </Col>
            </Row>

            <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
              <Col xs={24} lg={16}>
                <RecentContractsCard
                  contracts={recentContracts}
                  language={language}
                  getStatusTag={getStatusTag}
                  onOpenAll={() => router.push("/contracts")}
                  onOpenContract={(contractId) => router.push(`/contracts/${contractId}`)}
                />
              </Col>
              <Col xs={24} lg={8}>
                <QuickActionsCard actions={quickActions} language={language} />
              </Col>
            </Row>

            <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
              <Col span={24}>
                <UpcomingDatesCard
                  dates={upcomingDates}
                  language={language}
                  getDateUrgency={getDateUrgency}
                  onOpenContract={(contractId) => router.push(`/contracts/${contractId}`)}
                />
              </Col>
            </Row>
          </motion.div>
        </Spin>
      </AppLayout>
    </ProtectedRoute>
  );
}
