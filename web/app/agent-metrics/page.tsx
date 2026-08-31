"use client";

import { StatusTag, statusKindFromAntColor } from "../components/StatusTag";

import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Empty,
  Select,
  Space,
  Spin,
  Table,
  Typography,
} from "antd";
import {
  ReloadOutlined,
} from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { StateBlock } from "../components/StateBlock";
import { hasRole, useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { agentUsageApi } from "../lib/api";
import { t } from "../lib/i18n";
import { tableScrollX } from "../lib/tableScroll";

const { Text } = Typography;

type RangeKey = "24h" | "7d" | "31d";

interface UsageRollup {
  provider: string;
  model: string;
  pricing_version: string;
  cost_status: string;
  planner_calls: number;
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
  cost_micros: number;
}

interface UsageSummary {
  from: string;
  to: string;
  planner_calls: number;
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
  cost_micros: number;
  cost_accounting_available: boolean;
  unavailable_usage_count: number;
  rollups: UsageRollup[];
}

function formatNumber(value: number | undefined) {
  return value == null ? "—" : value.toLocaleString("zh-CN");
}

function formatCost(micros: number | undefined, available: boolean) {
  if (!available) return "—";
  return `USD ${(micros || 0) / 1_000_000}`;
}

function rangeDuration(range: RangeKey) {
  if (range === "31d") return 31 * 24 * 60 * 60 * 1000;
  if (range === "7d") return 7 * 24 * 60 * 60 * 1000;
  return 24 * 60 * 60 * 1000;
}

export default function AgentMetricsPage() {
  const { token, user } = useAuth();
  const { language } = useLanguage();
  const [range, setRange] = useState<RangeKey>("24h");
  const [summary, setSummary] = useState<UsageSummary | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadSummary = async (selectedRange: RangeKey = range) => {
    if (!token) return;
    setLoading(true);
    setError(null);
    const to = new Date();
    const from = new Date(to.getTime() - rangeDuration(selectedRange));
    try {
      const response = await agentUsageApi.summary(token, {
        from: from.toISOString(),
        to: to.toISOString(),
      });
      setSummary(response as UsageSummary);
    } catch (requestError: any) {
      const errorMessage = requestError?.message || t("agent_metrics.load_failed", language);
      setError(errorMessage);
      setSummary(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadSummary("24h");
    // The initial load is intentionally independent of the selected range.
    // Changing the range is an explicit user action below.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token]);

  const canView = hasRole(user, "admin") || hasRole(user, "auditor");
  const columns = [
    { title: t("agent_metrics.provider", language), dataIndex: "provider", key: "provider" },
    { title: t("agent_metrics.model", language), dataIndex: "model", key: "model" },
    { title: t("agent_metrics.pricing_version", language), dataIndex: "pricing_version", key: "pricing_version" },
    {
      title: t("agent_metrics.calls", language),
      dataIndex: "planner_calls",
      key: "planner_calls",
      render: (value: number) => formatNumber(value),
    },
    {
      title: t("agent_metrics.tokens", language),
      dataIndex: "total_tokens",
      key: "total_tokens",
      render: (value: number) => formatNumber(value),
    },
    {
      title: t("agent_metrics.cost_status", language),
      dataIndex: "cost_status",
      key: "cost_status",
      render: (value: string) => (
        <StatusTag kind={statusKindFromAntColor(value === "measured" || value === "calculated" ? "green" : "orange")}>
          {value === "measured" ? t("agent_metrics.status_measured", language) : value === "calculated" ? t("agent_metrics.status_calculated", language) : t("agent_metrics.status_unavailable", language)}
        </StatusTag>
      ),
    },
    {
      title: t("agent_metrics.cost", language),
      dataIndex: "cost_micros",
      key: "cost_micros",
      render: (value: number, item: UsageRollup) => formatCost(value, item.cost_status === "measured" || item.cost_status === "calculated"),
    },
  ];

  return (
    <ProtectedRoute>
      <AppLayout>
        <PageHeader
          title={t("agent_metrics.title", language)}

          primaryAction={
            <Space wrap>
              <Select<RangeKey>
                value={range}
                onChange={(value) => {
                  setRange(value);
                  void loadSummary(value);
                }}
                options={[
                  { value: "24h", label: t("agent_metrics.range_24h", language) },
                  { value: "7d", label: t("agent_metrics.range_7d", language) },
                  { value: "31d", label: t("agent_metrics.range_31d", language) },
                ]}
                className="agent-metrics-range-select"
              />
              <Button icon={<ReloadOutlined />} onClick={() => void loadSummary()} loading={loading}>
                {t("agent_metrics.refresh", language)}
              </Button>
            </Space>
          }
        />

        {!canView && (
          <Alert
            type="warning"
            showIcon
            message={t("agent_metrics.permission_required", language)}
            className="agent-metrics-permission-alert"
          />
        )}
        {error && <StateBlock state={{ kind: "failed", message: error }} language={language} />}

        {loading && !summary ? (
          <Card><Spin /></Card>
        ) : (
          <>
            <div className="stripe-metric-grid agent-metrics-summary-grid">
              <div className="pulse-kpi-card agent-metrics-summary-card">
                <span className="agent-metrics-summary-label">{t("agent_metrics.calls", language)}</span>
                <Typography.Text className="font-tabular agent-metrics-summary-value">
                  {formatNumber(summary?.planner_calls)}
                </Typography.Text>
              </div>
              <div className="pulse-kpi-card agent-metrics-summary-card">
                <span className="agent-metrics-summary-label">{t("agent_metrics.input_tokens", language)}</span>
                <Typography.Text className="font-tabular agent-metrics-summary-value">
                  {formatNumber(summary?.input_tokens)}
                </Typography.Text>
              </div>
              <div className="pulse-kpi-card agent-metrics-summary-card">
                <span className="agent-metrics-summary-label">{t("agent_metrics.output_tokens", language)}</span>
                <Typography.Text className="font-tabular agent-metrics-summary-value">
                  {formatNumber(summary?.output_tokens)}
                </Typography.Text>
              </div>
              <div className="pulse-kpi-card agent-metrics-summary-card">
                <span className="agent-metrics-summary-label">{t("agent_metrics.cost", language)}</span>
                <Typography.Text className="font-tabular agent-metrics-summary-value">
                  {formatCost(summary?.cost_micros, summary?.cost_accounting_available === true)}
                </Typography.Text>
              </div>
            </div>

            {summary && !summary.cost_accounting_available && summary.planner_calls > 0 && (
              <Alert
                type="info"
                showIcon
                message={t("agent_metrics.cost_unavailable", language)}
                description={t("agent_metrics.cost_unavailable_desc", language)}
                className="agent-metrics-cost-alert"
              />
            )}

            <Card title={t("agent_metrics.breakdown", language)} className="agent-metrics-breakdown-card">
              {summary?.rollups?.length ? (
                <Table<UsageRollup>
                  rowKey={(item) => `${item.provider}:${item.model}:${item.pricing_version}:${item.cost_status}`}
                  columns={columns}
                  dataSource={summary.rollups}
                  pagination={false}
                  size="small"
                  scroll={tableScrollX((summary.rollups || []).length, 900)}
                />
              ) : (
                <Empty description={t("agent_metrics.empty", language)} />
              )}
            </Card>

            <Text type="secondary" className="agent-metrics-audit-note">
              {t("agent_metrics.audit_note", language)}
            </Text>
          </>
        )}
      </AppLayout>
    </ProtectedRoute>
  );
}
