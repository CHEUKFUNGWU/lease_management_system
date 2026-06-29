"use client";

import { useEffect, useMemo, useState } from "react";
import { motion } from "framer-motion";
import { Button, Col, Row, Spin, Tag } from "antd";
import {
  BarChartOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  FileTextOutlined,
  PlusOutlined,
  RobotOutlined,
} from "@ant-design/icons";
import { useRouter } from "next/navigation";
import dayjs from "dayjs";
import AppLayout from "./components/AppLayout";
import ProtectedRoute from "./components/ProtectedRoute";
import { DashboardHeader, KPICard } from "./components/dashboard/DashboardCards";
import { ContractStatusCard, LiabilityTrendCard } from "./components/dashboard/DashboardCharts";
import {
  QuickActionsCard,
  RecentContractsCard,
  UpcomingDatesCard,
} from "./components/dashboard/DashboardLists";
import type {
  DashboardQuickAction,
  DashboardRecentContract,
  DashboardStats,
  DashboardStatusDatum,
  DashboardUpcomingDate,
} from "./components/dashboard/types";
import { staggerContainer } from "./design-system/animations";
import { useAuth } from "./context/AuthContext";
import { useLanguage } from "./context/LanguageContext";
import { contractApi, leaseAdminApi } from "./lib/api";
import { getApprovalStatusColor, getApprovalStatusLabel } from "./lib/constants/contracts";
import { t } from "./lib/i18n";

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
        const contracts = contractResponse.data || [];
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

    loadDashboard();
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
                <KPICard
                  title={t("dashboard.total_contracts", language)}
                  value={stats.total}
                  prefix={<FileTextOutlined style={{ color: "#000", fontSize: 18 }} />}
                  loading={loading}
                />
              </Col>
              <Col xs={24} sm={12} lg={6}>
                <KPICard
                  title={t("dashboard.approved", language)}
                  value={stats.approved}
                  prefix={<CheckCircleOutlined style={{ color: "#000", fontSize: 18 }} />}
                  loading={loading}
                />
              </Col>
              <Col xs={24} sm={12} lg={6}>
                <KPICard
                  title={t("dashboard.pending", language)}
                  value={stats.pending}
                  prefix={<ClockCircleOutlined style={{ color: "#595959", fontSize: 18 }} />}
                  loading={loading}
                />
              </Col>
              <Col xs={24} sm={12} lg={6}>
                <KPICard
                  title={t("dashboard.draft", language)}
                  value={stats.draft}
                  prefix={<RobotOutlined style={{ color: "#8C8C8C", fontSize: 18 }} />}
                  loading={loading}
                />
              </Col>
            </Row>

            <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
              <Col xs={24} lg={16}>
                <LiabilityTrendCard language={language} onOpenReports={() => router.push("/reports")} />
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
