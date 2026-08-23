"use client";

import { useEffect, useState } from "react";
import {
  Card,
  Table,
  Typography,
  Tag,
  Space,
  Select,
  Segmented,
  Button,
  Spin,
  Alert,
} from "antd";
import {
  CheckCircleOutlined,
  WarningOutlined,
  SyncOutlined,
} from "@ant-design/icons";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { fmtMoney } from "../lib/format";
import { tableScrollX } from "../lib/tableScroll";
import { cashPlanApi, type CashPlanPartition, type CashPlanResponse } from "../lib/api";
import dayjs from "dayjs";

const { Text } = Typography;

export function CashPlanPanel({
  token,
}: {
  token: string;
}) {
  const { language } = useLanguage();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fromPeriod, setFromPeriod] = useState(dayjs().format("YYYY-01"));
  const [toPeriod, setToPeriod] = useState(dayjs().format("YYYY-12"));
  const [dataClassification, setDataClassification] = useState("production");
  const [datasetVersion, setDatasetVersion] = useState("");
  const [planData, setPlanData] = useState<CashPlanResponse | null>(null);
  const [selectedCurrency, setSelectedCurrency] = useState<string>("CNY");

  const loadPlan = async () => {
    if (!token) return;
    setLoading(true);
    setError(null);
    try {
      const res = await cashPlanApi.compose(
        {
          from_period: fromPeriod,
          to_period: toPeriod,
          data_classification: dataClassification,
          dataset_version: datasetVersion || undefined,
        },
        token
      );
      setPlanData(res);
      if (res.partitions && res.partitions.length > 0) {
        setSelectedCurrency(res.partitions[0].currency);
      }
    } catch (err: any) {
      setError(err?.message || "Failed to load cash plan");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadPlan();
  // eslint-disable-next-line react-hooks/exhaustive-deps -- P2-C gate close-out: legacy dep semantics kept as-is; loaders are rebuilt every render so adding them would loop refetches. useCallback refactor tracked separately; do not add new exemptions.
  }, [token, fromPeriod, toPeriod, dataClassification, datasetVersion]);

  const activePartition: CashPlanPartition | undefined =
    planData?.partitions?.find((p) => p.currency === selectedCurrency) ||
    planData?.partitions?.[0];

  const monthlyColumns = [
    {
      title: t("cashflow.col_period", language),
      dataIndex: "period",
      key: "period",
      width: 110,
      fixed: "left" as const,
      render: (p: string) => <Text strong>{p}</Text>,
    },
    {
      title: t("cashflow.stat_store_revenue", language),
      dataIndex: "revenue",
      key: "revenue",
      width: 140,
      align: "right" as const,
      render: (v: number) => <span className="font-tabular">{fmtMoney(v, activePartition?.currency || "CNY")}</span>,
    },
    {
      title: t("cashflow.plan.operating_cash", language),
      dataIndex: "operating_cash",
      key: "operating_cash",
      width: 160,
      align: "right" as const,
      render: (v: number) => (
        <span
          className="font-tabular"
          style={{
            color: v >= 0 ? "var(--state-success-text, #216E39)" : "var(--state-error-text, #C93B2B)",
            fontWeight: 500,
          }}
        >
          {fmtMoney(v, activePartition?.currency || "CNY")}
        </span>
      ),
    },
    {
      title: t("cashflow.plan.rent_offset", language),
      dataIndex: "rent_offset",
      key: "rent_offset",
      width: 150,
      align: "right" as const,
      render: (v: number) => (
        <span className="font-tabular" style={{ color: "var(--fg-secondary)" }}>
          +{fmtMoney(v, activePartition?.currency || "CNY")}
        </span>
      ),
    },
    {
      title: t("cashflow.plan.lease_outflow", language),
      dataIndex: "lease_outflow",
      key: "lease_outflow",
      width: 150,
      align: "right" as const,
      render: (v: number) => (
        <span className="font-tabular" style={{ color: "var(--chart-negative, #DC2626)" }}>
          -{fmtMoney(v, activePartition?.currency || "CNY")}
        </span>
      ),
    },
    {
      title: t("cashflow.plan.capex_outflow", language),
      dataIndex: "capex_outflow",
      key: "capex_outflow",
      width: 150,
      align: "right" as const,
      render: (v: number) => (
        <span className="font-tabular" style={{ color: v > 0 ? "var(--state-warning-text, #9A6700)" : "var(--fg-muted)" }}>
          -{fmtMoney(v, activePartition?.currency || "CNY")}
        </span>
      ),
    },
    {
      title: t("cashflow.plan.net_cash", language),
      dataIndex: "net_cash_plan",
      key: "net_cash_plan",
      width: 160,
      align: "right" as const,
      render: (v: number) => (
        <strong
          className="font-tabular"
          style={{
            color: v >= 0 ? "var(--fg-primary)" : "var(--state-error-text, #C93B2B)",
          }}
        >
          {fmtMoney(v, activePartition?.currency || "CNY")}
        </strong>
      ),
    },
  ];

  return (
    <div style={{ width: "100%" }}>
      {/* Precision Filter Bar */}
      <div
        className="precision-filter-bar"
        style={{
          display: "flex",
          flexWrap: "wrap",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 12,
          padding: "12px 16px",
          background: "var(--bg-surface)",
          border: "1px solid var(--border-default)",
          borderRadius: 8,
          marginBottom: 16,
        }}
      >
        <Space wrap size={16} align="center">
          <Space size={8} align="center">
            <Text strong style={{ fontSize: 13, color: "var(--fg-secondary)" }}>{t("cashflow.plan.period_range", language)}:</Text>
            <Select
              value={fromPeriod}
              onChange={setFromPeriod}
              style={{ width: 110 }}
              options={[
                { label: "2026-01", value: "2026-01" },
                { label: "2026-04", value: "2026-04" },
                { label: "2026-07", value: "2026-07" },
                { label: "2026-10", value: "2026-10" },
              ]}
            />
            <Text type="secondary">{t("cashflow.plan.to", language)}</Text>
            <Select
              value={toPeriod}
              onChange={setToPeriod}
              style={{ width: 110 }}
              options={[
                { label: "2026-03", value: "2026-03" },
                { label: "2026-06", value: "2026-06" },
                { label: "2026-09", value: "2026-09" },
                { label: "2026-12", value: "2026-12" },
              ]}
            />
          </Space>

          <Space size={8} align="center">
            <Text strong style={{ fontSize: 13, color: "var(--fg-secondary)" }}>{t("cashflow.plan.data_classification", language)}:</Text>
            <Segmented
              className="precision-segmented"
              value={dataClassification}
              onChange={(v) => setDataClassification(String(v))}
              options={[
                { label: t("cashflow.plan.prod", language), value: "production" },
                { label: t("cashflow.plan.sim", language), value: "simulated" },
              ]}
            />
          </Space>

          {planData && planData.partitions.length > 1 && (
            <Space size={8} align="center">
              <Text strong style={{ fontSize: 13, color: "var(--fg-secondary)" }}>{t("cashflow.plan.currency_partition", language)}:</Text>
              <Select
                value={selectedCurrency}
                onChange={setSelectedCurrency}
                style={{ width: 90 }}
                options={planData.partitions.map((p) => ({
                  label: p.currency,
                  value: p.currency,
                }))}
              />
            </Space>
          )}
        </Space>

        <Button icon={<SyncOutlined />} onClick={loadPlan} loading={loading}>
          {t("cashflow.plan.recompose", language)}
        </Button>
      </div>

      {error && <Alert type="error" message={error} showIcon closable style={{ marginBottom: 16 }} />}

      {loading && !planData ? (
        <div style={{ textAlign: "center", padding: "40px 0" }}>
          <Spin size="large" />
        </div>
      ) : activePartition ? (
        <>
          {/* Top KPI Metric Grid */}
          <div className="stripe-metric-grid" style={{ gridTemplateColumns: "repeat(5, minmax(0, 1fr))", marginBottom: 16 }}>
            <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 84, padding: "14px 16px" }}>
              <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>
                {t("cashflow.plan.operating_cash", language)}
              </span>
              <div style={{ margin: "6px 0 2px" }}>
                <Typography.Text
                  className="font-tabular"
                  style={{
                    fontSize: 20,
                    fontWeight: 600,
                    color: activePartition.total_operating_cash >= 0 ? "var(--state-success-text, #216E39)" : "var(--state-error-text, #C93B2B)",
                  }}
                >
                  {fmtMoney(activePartition.total_operating_cash, activePartition.currency)}
                </Typography.Text>
              </div>
              <Text type="secondary" style={{ fontSize: 11 }}>
                {t("cashflow.plan.sub_operating", language)}
              </Text>
            </div>

            <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 84, padding: "14px 16px" }}>
              <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>
                {t("cashflow.plan.rent_offset", language)}
              </span>
              <div style={{ margin: "6px 0 2px" }}>
                <Typography.Text className="font-tabular" style={{ fontSize: 20, fontWeight: 600, color: "var(--fg-primary)" }}>
                  +{fmtMoney(activePartition.total_rent_offset, activePartition.currency)}
                </Typography.Text>
              </div>
              <Text type="secondary" style={{ fontSize: 11 }}>
                {t("cashflow.plan.sub_offset", language)}
              </Text>
            </div>

            <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 84, padding: "14px 16px" }}>
              <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>
                {t("cashflow.plan.lease_outflow", language)}
              </span>
              <div style={{ margin: "6px 0 2px" }}>
                <Typography.Text className="font-tabular" style={{ fontSize: 20, fontWeight: 600, color: "var(--chart-negative, #DC2626)" }}>
                  -{fmtMoney(activePartition.total_lease_outflow, activePartition.currency)}
                </Typography.Text>
              </div>
              <Text type="secondary" style={{ fontSize: 11 }}>
                {t("cashflow.plan.sub_lease", language)}
              </Text>
            </div>

            <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 84, padding: "14px 16px" }}>
              <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>
                {t("cashflow.plan.capex_outflow", language)}
              </span>
              <div style={{ margin: "6px 0 2px" }}>
                <Typography.Text className="font-tabular" style={{ fontSize: 20, fontWeight: 600, color: "var(--state-warning-text, #9A6700)" }}>
                  -{fmtMoney(activePartition.total_capex_outflow, activePartition.currency)}
                </Typography.Text>
              </div>
              <Text type="secondary" style={{ fontSize: 11 }}>
                {t("cashflow.plan.sub_capex", language)}
              </Text>
            </div>

            <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 84, padding: "14px 16px" }}>
              <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>
                {t("cashflow.plan.net_cash", language)}
              </span>
              <div style={{ margin: "6px 0 2px" }}>
                <Typography.Text
                  className="font-tabular"
                  style={{
                    fontSize: 20,
                    fontWeight: 600,
                    color: activePartition.total_net_cash_plan >= 0 ? "var(--fg-primary)" : "var(--state-error-text, #C93B2B)",
                  }}
                >
                  {fmtMoney(activePartition.total_net_cash_plan, activePartition.currency)}
                </Typography.Text>
              </div>
              <Text type="secondary" style={{ fontSize: 11 }}>
                {t("cashflow.plan.sub_net", language)}
              </Text>
            </div>
          </div>

          {/* Conservation Bridge Card */}
          <Card
            size="small"
            style={{ marginBottom: 16 }}
            title={
              <Space>
                <span>{t("cashflow.plan.bridge_title", language)}</span>
                {activePartition.bridge.is_conserved ? (
                  <Tag color="success" icon={<CheckCircleOutlined />}>
                    {t("cashflow.plan.conserved", language)}
                  </Tag>
                ) : (
                  <Tag color="error" icon={<WarningOutlined />}>
                    {t("cashflow.plan.unconserved", language)} ({t("cashflow.plan.residual", language)}: {activePartition.bridge.rounding_residual})
                  </Tag>
                )}
                {activePartition.weakest_coverage_ratio != null && activePartition.weakest_coverage_ratio < 100 && (
                  <Tag color="warning">
                    {t("cashflow.plan.coverage", language)}: {activePartition.weakest_coverage_ratio.toFixed(0)}%
                  </Tag>
                )}
              </Space>
            }
          >
            <div style={{ display: "grid", gridTemplateColumns: `repeat(${activePartition.bridge.steps.length + 1}, minmax(0, 1fr))`, gap: 8 }}>
              {activePartition.bridge.steps.map((step, idx) => (
                <div
                  key={idx}
                  style={{
                    padding: "10px 12px",
                    background: "var(--bg-subtle, #F8FAFC)",
                    border: "1px solid var(--border-default)",
                    borderRadius: 6,
                    textAlign: "center",
                  }}
                >
                  <div style={{ fontSize: 11, color: "var(--fg-secondary)", marginBottom: 4 }}>
                    {step.label}
                  </div>
                  <div
                    className="font-tabular"
                    style={{
                      fontSize: 14,
                      fontWeight: 600,
                      color: step.is_deduction ? "var(--state-error-text, #C93B2B)" : "var(--state-success-text, #216E39)",
                    }}
                  >
                    {step.is_deduction ? "-" : "+"}{fmtMoney(step.amount, activePartition.currency)}
                  </div>
                </div>
              ))}
              <div
                style={{
                  padding: "10px 12px",
                  background: "var(--bg-surface)",
                  border: "1px solid var(--border-strong, #CBD5E1)",
                  borderRadius: 6,
                  textAlign: "center",
                }}
              >
                <div style={{ fontSize: 11, fontWeight: 500, color: "var(--fg-secondary)", marginBottom: 4 }}>
                  {t("cashflow.plan.net_cash", language)}
                </div>
                <div className="font-tabular" style={{ fontSize: 15, fontWeight: 600, color: "var(--fg-primary)" }}>
                  {fmtMoney(activePartition.total_net_cash_plan, activePartition.currency)}
                </div>
              </div>
            </div>
          </Card>

          {/* Monthly Breakdown Table */}
          <Card size="small" title={t("cashflow.plan.table_title", language)}>
            <Table
              dataSource={activePartition.monthly}
              columns={monthlyColumns}
              rowKey="period"
              pagination={false}
              size="small"
              scroll={tableScrollX(activePartition.monthly.length, 800)}
            />
          </Card>
        </>
      ) : null}
    </div>
  );
}

