"use client";

import { useState, useEffect, useMemo } from "react";
import { motion } from "framer-motion";
import {
  Card,
  Row,
  Col,
  Statistic,
  Spin,
  List,
  Tag,
  Button,
  Empty,
  Skeleton,
} from "antd";
import {
  FileTextOutlined,
  CheckCircleOutlined,
  ClockCircleOutlined,
  RobotOutlined,
  ArrowRightOutlined,
  PlusOutlined,
  UploadOutlined,
  BarChartOutlined,
} from "@ant-design/icons";
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
} from "recharts";
import AppLayout from "./components/AppLayout";
import ProtectedRoute from "./components/ProtectedRoute";
import { useAuth } from "./context/AuthContext";
import { useLanguage } from "./context/LanguageContext";
import { t } from "./lib/i18n";
import { contractApi } from "./lib/api";
import { useRouter } from "next/navigation";
import { staggerContainer, staggerItem } from "./design-system/animations";

// ─── Mock Chart Data (will be replaced with real API data) ─────

const PIE_COLORS = ["#000000", "#595959", "#BFBFBF", "#E5E5E5"];

// ─── Components ────────────────────────────────────────────────

function KPICard({
  title,
  value,
  prefix,
  loading,
  delay,
}: {
  title: string;
  value: number;
  prefix: React.ReactNode;
  loading: boolean;
  delay: number;
}) {
  return (
    <motion.div variants={staggerItem}>
      <Card
        bodyStyle={{ padding: "20px 24px" }}
        style={{ borderRadius: 10, height: "100%" }}
      >
        {loading ? (
          <Skeleton active paragraph={false} title={{ width: "60%" }} />
        ) : (
          <Statistic
            title={
              <span
                style={{
                  fontSize: 12,
                  fontWeight: 500,
                  color: "#8C8C8C",
                  textTransform: "uppercase",
                  letterSpacing: "0.02em",
                }}
              >
                {title}
              </span>
            }
            value={value}
            prefix={
              <span style={{ marginRight: 8, display: "inline-flex" }}>
                {prefix}
              </span>
            }
            valueStyle={{
              fontSize: 28,
              fontWeight: 700,
              letterSpacing: "-0.03em",
              color: "#000",
            }}
          />
        )}
      </Card>
    </motion.div>
  );
}

function ChartCard({ title, children, extra }: { title: string; children: React.ReactNode; extra?: React.ReactNode }) {
  return (
    <Card
      title={
        <span style={{ fontSize: 15, fontWeight: 600, letterSpacing: "-0.01em" }}>
          {title}
        </span>
      }
      extra={extra}
      bodyStyle={{ padding: "20px 24px 24px" }}
      style={{ borderRadius: 10, height: "100%" }}
    >
      {children}
    </Card>
  );
}

// ─── Custom Tooltip for Recharts ───────────────────────────────

function CustomTooltip({ active, payload, label }: any) {
  if (!active || !payload?.length) return null;
  return (
    <div
      style={{
        background: "#fff",
        border: "1px solid #E5E5E5",
        borderRadius: 8,
        padding: "10px 14px",
        boxShadow: "0 0 0 1px rgba(0,0,0,0.04), 0 4px 12px rgba(0,0,0,0.06)",
        fontSize: 13,
      }}
    >
      <div style={{ fontWeight: 600, marginBottom: 4, color: "#000" }}>
        {label}
      </div>
      {payload.map((entry: any, idx: number) => (
        <div key={idx} style={{ color: "#595959" }}>
          {entry.name}: {" "}
          <span style={{ fontWeight: 600, color: "#000" }}>
            ¥{entry.value?.toLocaleString()}
          </span>
        </div>
      ))}
    </div>
  );
}

// ─── Main Page ─────────────────────────────────────────────────

export default function HomePage() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const router = useRouter();
  const [loading, setLoading] = useState(true);
  const [stats, setStats] = useState({
    total: 0,
    approved: 0,
    pending: 0,
    draft: 0,
  });
  const [recentContracts, setRecentContracts] = useState<any[]>([]);

  // ─── Translated Chart Data ─────────────────────────────────️

  const liabilityData = useMemo(
    () => [
      { month: `1${t("dashboard.month_short", language)}`, value: 3255676 },
      { month: `2${t("dashboard.month_short", language)}`, value: 3210395 },
      { month: `3${t("dashboard.month_short", language)}`, value: 3164892 },
      { month: `4${t("dashboard.month_short", language)}`, value: 3119165 },
      { month: `5${t("dashboard.month_short", language)}`, value: 3073213 },
      { month: `6${t("dashboard.month_short", language)}`, value: 3027033 },
      { month: `7${t("dashboard.month_short", language)}`, value: 2980624 },
      { month: `8${t("dashboard.month_short", language)}`, value: 2933984 },
      { month: `9${t("dashboard.month_short", language)}`, value: 2887112 },
      { month: `10${t("dashboard.month_short", language)}`, value: 2840005 },
      { month: `11${t("dashboard.month_short", language)}`, value: 2792662 },
      { month: `12${t("dashboard.month_short", language)}`, value: 2745080 },
    ],
    [language],
  );

  const statusData = useMemo(
    () => [
      { name: t("dashboard.approved", language), value: 12, key: "approved" },
      { name: t("dashboard.pending", language), value: 5, key: "pending" },
      { name: t("dashboard.draft", language), value: 3, key: "draft" },
      { name: t("status.rejected", language), value: 1, key: "rejected" },
    ],
    [language],
  );

  // ─── Pie Tooltip defined inside component for language access

  function PieTooltip({ active, payload }: any) {
    if (!active || !payload?.length) return null;
    const data = payload[0];
    return (
      <div
        style={{
          background: "#fff",
          border: "1px solid #E5E5E5",
          borderRadius: 8,
          padding: "8px 12px",
          boxShadow: "0 0 0 1px rgba(0,0,0,0.04), 0 4px 12px rgba(0,0,0,0.06)",
          fontSize: 13,
        }}
      >
        <span style={{ fontWeight: 600 }}>{data.name}</span>
        <span style={{ marginLeft: 8, color: "#595959" }}>
          {data.value} {t("dashboard.copies", language)}
        </span>
      </div>
    );
  }

  useEffect(() => {
    if (!token) return;
    loadDashboard();
  }, [token]);

  const loadDashboard = async () => {
    try {
      const data = await contractApi.list(token!);
      const contracts = data.data || data || [];
      const total = data.total || contracts.length;

      const approved = contracts.filter(
        (c: any) => c.approval_status === "approved"
      ).length;
      const pending = contracts.filter(
        (c: any) =>
          c.approval_status === "pending_review" ||
          c.approval_status === "pending_approval" ||
          c.approval_status === "submitted"
      ).length;
      const draft = contracts.filter(
        (c: any) => c.approval_status === "draft" || !c.approval_status
      ).length;

      setStats({ total, approved, pending, draft });
      setRecentContracts(contracts.slice(0, 6));
    } catch (error) {
      console.error("Failed to load dashboard data:", error);
    } finally {
      setLoading(false);
    }
  };

  const getStatusTag = (status: string) => {
    const statusMap: Record<string, { color: string; text: string }> = {
      draft: { color: "default", text: t("status.draft", language) },
      submitted: { color: "processing", text: t("status.submitted", language) },
      pending_review: { color: "processing", text: t("status.submitted", language) },
      pending_approval: { color: "warning", text: t("status.pending_approval", language) },
      approved: { color: "success", text: t("status.approved", language) },
      rejected: { color: "error", text: t("status.rejected", language) },
    };
    const s = statusMap[status] || { color: "default", text: status || t("status.draft", language) };
    return <Tag color={s.color}>{s.text}</Tag>;
  };

  return (
    <ProtectedRoute>
      <AppLayout>
        {/* Page Header */}
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "flex-start",
            marginBottom: 32,
          }}
        >
          <div>
            <h1 style={{ marginBottom: 4, fontSize: 28, letterSpacing: "-0.04em" }}>
              {t("dashboard.title", language)}
            </h1>
            <p style={{ color: "#8C8C8C", fontSize: 14, margin: 0 }}>
              {t("dashboard.subtitle", language)}
            </p>
          </div>
          <div style={{ display: "flex", gap: 8 }}>
            <Button
              icon={<PlusOutlined />}
              onClick={() => router.push("/contracts/new")}
              style={{ borderRadius: 9999, fontWeight: 500 }}
            >
              {t("dashboard.add_contract", language)}
            </Button>
            <Button
              icon={<UploadOutlined />}
              onClick={() => router.push("/upload")}
              style={{ borderRadius: 9999, fontWeight: 500 }}
            >
              {t("dashboard.upload_file", language)}
            </Button>
          </div>
        </div>

        <Spin spinning={loading} tip={t("dashboard.loading", language)}>
          <motion.div
            variants={staggerContainer}
            initial="initial"
            animate="animate"
          >
            {/* KPI Cards */}
            <Row gutter={[16, 16]}>
              <Col xs={24} sm={12} lg={6}>
                <KPICard
                  title={t("dashboard.total_contracts", language)}
                  value={stats.total}
                  prefix={<FileTextOutlined style={{ color: "#000", fontSize: 18 }} />}
                  loading={loading}
                  delay={0}
                />
              </Col>
              <Col xs={24} sm={12} lg={6}>
                <KPICard
                  title={t("dashboard.approved", language)}
                  value={stats.approved}
                  prefix={<CheckCircleOutlined style={{ color: "#000", fontSize: 18 }} />}
                  loading={loading}
                  delay={0.05}
                />
              </Col>
              <Col xs={24} sm={12} lg={6}>
                <KPICard
                  title={t("dashboard.pending", language)}
                  value={stats.pending}
                  prefix={<ClockCircleOutlined style={{ color: "#595959", fontSize: 18 }} />}
                  loading={loading}
                  delay={0.1}
                />
              </Col>
              <Col xs={24} sm={12} lg={6}>
                <KPICard
                  title={t("dashboard.draft", language)}
                  value={stats.draft}
                  prefix={<RobotOutlined style={{ color: "#8C8C8C", fontSize: 18 }} />}
                  loading={loading}
                  delay={0.15}
                />
              </Col>
            </Row>

            {/* Charts Row */}
            <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
              <Col xs={24} lg={16}>
                <ChartCard
                  title={t("dashboard.liability_trend", language)}
                  extra={
                    <Button type="link" size="small" style={{ fontSize: 13 }}>
                      {t("dashboard.view_reports", language)} <ArrowRightOutlined />
                    </Button>
                  }
                >
                  <div style={{ height: 280 }}>
                    <ResponsiveContainer width="100%" height="100%">
                      <AreaChart data={liabilityData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
                        <defs>
                          <linearGradient id="liabilityGradient" x1="0" y1="0" x2="0" y2="1">
                            <stop offset="5%" stopColor="#000" stopOpacity={0.08} />
                            <stop offset="95%" stopColor="#000" stopOpacity={0} />
                          </linearGradient>
                        </defs>
                        <CartesianGrid strokeDasharray="3 3" stroke="#F0F0F0" vertical={false} />
                        <XAxis
                          dataKey="month"
                          axisLine={false}
                          tickLine={false}
                          tick={{ fontSize: 12, fill: "#8C8C8C" }}
                          dy={10}
                        />
                        <YAxis
                          axisLine={false}
                          tickLine={false}
                          tick={{ fontSize: 12, fill: "#8C8C8C" }}
                          tickFormatter={(v) => `¥${(v / 10000).toFixed(0)}${t("dashboard.ten_thousand", language)}`}
                          dx={-10}
                        />
                        <Tooltip content={<CustomTooltip />} />
                        <Area
                          type="monotone"
                          dataKey="value"
                          name={t("dashboard.lease_liability", language)}
                          stroke="#000"
                          strokeWidth={2}
                          fill="url(#liabilityGradient)"
                          dot={{ r: 3, fill: "#000", strokeWidth: 0 }}
                          activeDot={{ r: 5, fill: "#000", strokeWidth: 2, stroke: "#fff" }}
                        />
                      </AreaChart>
                    </ResponsiveContainer>
                  </div>
                </ChartCard>
              </Col>

              <Col xs={24} lg={8}>
                <ChartCard title={t("dashboard.contract_status", language)}>
                  <div style={{ height: 280, display: "flex", alignItems: "center", justifyContent: "center" }}>
                    <ResponsiveContainer width="100%" height="100%">
                      <PieChart>
                        <Pie
                          data={statusData}
                          cx="50%"
                          cy="50%"
                          innerRadius={60}
                          outerRadius={90}
                          paddingAngle={3}
                          dataKey="value"
                          stroke="none"
                        >
                          {statusData.map((entry, index) => (
                            <Cell key={`cell-${index}`} fill={PIE_COLORS[index % PIE_COLORS.length]} />
                          ))}
                        </Pie>
                        <Tooltip content={<PieTooltip />} />
                      </PieChart>
                    </ResponsiveContainer>
                  </div>
                  {/* Legend */}
                  <div
                    style={{
                      display: "flex",
                      justifyContent: "center",
                      gap: 16,
                      marginTop: -8,
                      flexWrap: "wrap",
                    }}
                  >
                    {statusData.map((item, idx) => (
                      <div key={item.key} style={{ display: "flex", alignItems: "center", gap: 6 }}>
                        <div
                          style={{
                            width: 8,
                            height: 8,
                            borderRadius: 2,
                            background: PIE_COLORS[idx],
                          }}
                        />
                        <span style={{ fontSize: 12, color: "#595959" }}>
                          {item.name} ({item.value})
                        </span>
                      </div>
                    ))}
                  </div>
                </ChartCard>
              </Col>
            </Row>

            {/* Recent Contracts + Quick Actions */}
            <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
              <Col xs={24} lg={16}>
                <Card
                  title={
                    <span style={{ fontSize: 15, fontWeight: 600, letterSpacing: "-0.01em" }}>
                      {t("dashboard.recent_contracts", language)}
                    </span>
                  }
                  extra={
                    <Button
                      type="link"
                      size="small"
                      onClick={() => router.push("/contracts")}
                      style={{ fontSize: 13 }}
                    >
                      {t("dashboard.view_all", language)} <ArrowRightOutlined />
                    </Button>
                  }
                  bodyStyle={{ padding: 0 }}
                  style={{ borderRadius: 10 }}
                >
                  {recentContracts.length === 0 ? (
                    <div style={{ padding: 40 }}>
                      <Empty
                        image={Empty.PRESENTED_IMAGE_SIMPLE}
                        description={t("dashboard.no_contracts", language)}
                      />
                    </div>
                  ) : (
                    <List
                      dataSource={recentContracts}
                      renderItem={(contract: any, index: number) => (
                        <motion.div
                          initial={{ opacity: 0, y: 4 }}
                          animate={{ opacity: 1, y: 0 }}
                          transition={{ delay: index * 0.04, duration: 0.2 }}
                        >
                          <List.Item
                            style={{
                              padding: "14px 24px",
                              borderBottom: "1px solid #F0F0F0",
                              cursor: "pointer",
                              transition: "background 0.1s",
                            }}
                            onMouseEnter={(e) =>
                              (e.currentTarget.style.background = "#FAFAFA")
                            }
                            onMouseLeave={(e) =>
                              (e.currentTarget.style.background = "transparent")
                            }
                            onClick={() => router.push(`/contracts/${contract.id}`)}
                            actions={[
                              getStatusTag(contract.approval_status),
                              <ArrowRightOutlined
                                key="arrow"
                                style={{ color: "#BFBFBF", fontSize: 12 }}
                              />,
                            ]}
                          >
                            <List.Item.Meta
                              title={
                                <span
                                  style={{
                                    fontWeight: 600,
                                    fontSize: 14,
                                    color: "#000",
                                  }}
                                >
                                  {contract.contract_number || contract.name}
                                </span>
                              }
                              description={
                                <span style={{ color: "#8C8C8C", fontSize: 13 }}>
                                  {contract.landlord || contract.store_name}
                                </span>
                              }
                            />
                          </List.Item>
                        </motion.div>
                      )}
                    />
                  )}
                </Card>
              </Col>

              <Col xs={24} lg={8}>
                <Card
                  title={
                    <span style={{ fontSize: 15, fontWeight: 600, letterSpacing: "-0.01em" }}>
                      {t("dashboard.quick_actions", language)}
                    </span>
                  }
                  bodyStyle={{ padding: "12px 24px 24px" }}
                  style={{ borderRadius: 10, height: "100%" }}
                >
                  <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                    {[
                      {
                        icon: <PlusOutlined />,
                        label: t("dashboard.add_contract", language),
                        desc: t("dashboard.add_contract_desc", language),
                        action: () => router.push("/contracts/new"),
                      },
                      {
                        icon: <UploadOutlined />,
                        label: t("dashboard.upload_file", language),
                        desc: t("dashboard.upload_file_desc", language),
                        action: () => router.push("/upload"),
                      },
                      {
                        icon: <BarChartOutlined />,
                        label: t("dashboard.view_reports", language),
                        desc: t("dashboard.view_report_desc", language),
                        action: () => router.push("/reports"),
                      },
                      {
                        icon: <RobotOutlined />,
                        label: t("dashboard.ai_assistant", language),
                        desc: t("dashboard.ai_assistant_desc", language),
                        action: () => router.push("/ai-chat"),
                      },
                    ].map((item, idx) => (
                      <motion.div
                        key={item.label}
                        initial={{ opacity: 0, x: -4 }}
                        animate={{ opacity: 1, x: 0 }}
                        transition={{ delay: 0.2 + idx * 0.05 }}
                        onClick={item.action}
                        style={{
                          display: "flex",
                          alignItems: "center",
                          gap: 14,
                          padding: "12px 14px",
                          borderRadius: 8,
                          cursor: "pointer",
                          transition: "background 0.1s",
                          border: "1px solid transparent",
                        }}
                        onMouseEnter={(e) => {
                          e.currentTarget.style.background = "#FAFAFA";
                          e.currentTarget.style.borderColor = "#E5E5E5";
                        }}
                        onMouseLeave={(e) => {
                          e.currentTarget.style.background = "transparent";
                          e.currentTarget.style.borderColor = "transparent";
                        }}
                      >
                        <div
                          style={{
                            width: 36,
                            height: 36,
                            borderRadius: 8,
                            background: "#F0F0F0",
                            display: "flex",
                            alignItems: "center",
                            justifyContent: "center",
                            fontSize: 16,
                            color: "#000",
                            flexShrink: 0,
                          }}
                        >
                          {item.icon}
                        </div>
                        <div style={{ minWidth: 0 }}>
                          <div
                            style={{
                              fontWeight: 600,
                              fontSize: 14,
                              color: "#000",
                              marginBottom: 2,
                            }}
                          >
                            {item.label}
                          </div>
                          <div style={{ fontSize: 12, color: "#8C8C8C" }}>
                            {item.desc}
                          </div>
                        </div>
                        <ArrowRightOutlined
                          style={{
                            marginLeft: "auto",
                            color: "#BFBFBF",
                            fontSize: 12,
                            flexShrink: 0,
                          }}
                        />
                      </motion.div>
                    ))}
                  </div>
                </Card>
              </Col>
            </Row>
          </motion.div>
        </Spin>
      </AppLayout>
    </ProtectedRoute>
  );
}
