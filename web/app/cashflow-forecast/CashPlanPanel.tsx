"use client";

import { useEffect, useState } from "react";
import {
  Card,
  Table,
  Typography,
  Space,
  Select,
  Segmented,
  Button,
  Spin,
  Alert,
} from "antd";
import { SyncOutlined } from "@ant-design/icons";
import { StatusTag } from "../components/StatusTag";
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
        <span className={`font-tabular cash-plan-table-value ${v >= 0 ? "is-positive" : "is-negative"}`}>
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
        <span className="font-tabular cash-plan-table-value is-offset">
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
        <span className="font-tabular cash-plan-table-value is-negative">
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
        <span className={`font-tabular cash-plan-table-value ${v > 0 ? "is-warning" : "is-muted"}`}>
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
        <strong className={`font-tabular cash-plan-table-value ${v >= 0 ? "is-primary" : "is-negative"}`}>
          {fmtMoney(v, activePartition?.currency || "CNY")}
        </strong>
      ),
    },
  ];

  return (
    <div className="cash-plan-panel">
      {/* Precision Filter Bar */}
      <div className="precision-filter-bar cash-plan-filter-bar">
        <Space wrap size={16} align="center">
          <Space size={8} align="center">
            <Text strong className="cash-plan-filter-label">{t("cashflow.plan.period_range", language)}:</Text>
            <Select
              value={fromPeriod}
              onChange={setFromPeriod}
              className="cash-plan-period-select"
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
              className="cash-plan-period-select"
              options={[
                { label: "2026-03", value: "2026-03" },
                { label: "2026-06", value: "2026-06" },
                { label: "2026-09", value: "2026-09" },
                { label: "2026-12", value: "2026-12" },
              ]}
            />
          </Space>

          <Space size={8} align="center">
            <Text strong className="cash-plan-filter-label">{t("cashflow.plan.data_classification", language)}:</Text>
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
              <Text strong className="cash-plan-filter-label">{t("cashflow.plan.currency_partition", language)}:</Text>
              <Select
                value={selectedCurrency}
                onChange={setSelectedCurrency}
                className="cash-plan-currency-select"
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

      {error && <Alert type="error" message={error} showIcon closable className="cash-plan-error" />}

      {loading && !planData ? (
        <div className="cash-plan-loading">
          <Spin size="large" />
        </div>
      ) : activePartition ? (
        <>
          {/* Top KPI Metric Grid */}
          <div className="stripe-metric-grid cash-plan-kpi-grid">
            <div className="pulse-kpi-card cash-plan-kpi-card">
              <span className="cash-plan-kpi-label">{t("cashflow.plan.operating_cash", language)}</span>
              <div className="cash-plan-kpi-value-wrap">
                <Typography.Text className={`font-tabular cash-plan-kpi-value ${activePartition.total_operating_cash >= 0 ? "is-positive" : "is-negative"}`}>
                  {fmtMoney(activePartition.total_operating_cash, activePartition.currency)}
                </Typography.Text>
              </div>
              <Text type="secondary" className="cash-plan-kpi-subtext">{t("cashflow.plan.sub_operating", language)}</Text>
            </div>

            <div className="pulse-kpi-card cash-plan-kpi-card">
              <span className="cash-plan-kpi-label">{t("cashflow.plan.rent_offset", language)}</span>
              <div className="cash-plan-kpi-value-wrap">
                <Typography.Text className="font-tabular cash-plan-kpi-value is-primary">
                  +{fmtMoney(activePartition.total_rent_offset, activePartition.currency)}
                </Typography.Text>
              </div>
              <Text type="secondary" className="cash-plan-kpi-subtext">{t("cashflow.plan.sub_offset", language)}</Text>
            </div>

            <div className="pulse-kpi-card cash-plan-kpi-card">
              <span className="cash-plan-kpi-label">{t("cashflow.plan.lease_outflow", language)}</span>
              <div className="cash-plan-kpi-value-wrap">
                <Typography.Text className="font-tabular cash-plan-kpi-value is-negative">
                  -{fmtMoney(activePartition.total_lease_outflow, activePartition.currency)}
                </Typography.Text>
              </div>
              <Text type="secondary" className="cash-plan-kpi-subtext">{t("cashflow.plan.sub_lease", language)}</Text>
            </div>

            <div className="pulse-kpi-card cash-plan-kpi-card">
              <span className="cash-plan-kpi-label">{t("cashflow.plan.capex_outflow", language)}</span>
              <div className="cash-plan-kpi-value-wrap">
                <Typography.Text className="font-tabular cash-plan-kpi-value is-warning">
                  -{fmtMoney(activePartition.total_capex_outflow, activePartition.currency)}
                </Typography.Text>
              </div>
              <Text type="secondary" className="cash-plan-kpi-subtext">{t("cashflow.plan.sub_capex", language)}</Text>
            </div>

            <div className="pulse-kpi-card cash-plan-kpi-card">
              <span className="cash-plan-kpi-label">{t("cashflow.plan.net_cash", language)}</span>
              <div className="cash-plan-kpi-value-wrap">
                <Typography.Text className={`font-tabular cash-plan-kpi-value ${activePartition.total_net_cash_plan >= 0 ? "is-primary" : "is-negative"}`}>
                  {fmtMoney(activePartition.total_net_cash_plan, activePartition.currency)}
                </Typography.Text>
              </div>
              <Text type="secondary" className="cash-plan-kpi-subtext">{t("cashflow.plan.sub_net", language)}</Text>
            </div>
          </div>

          {/* Conservation Bridge Card */}
          <Card
            size="small"
            className="cash-plan-bridge-card"
            title={
              <Space>
                <span>{t("cashflow.plan.bridge_title", language)}</span>
                {activePartition.bridge.is_conserved ? (
                  <StatusTag kind="success">{t("cashflow.plan.conserved", language)}</StatusTag>
                ) : (
                  <StatusTag kind="error">
                    {t("cashflow.plan.unconserved", language)} ({t("cashflow.plan.residual", language)}: {activePartition.bridge.rounding_residual})
                  </StatusTag>
                )}
                {activePartition.weakest_coverage_ratio != null && activePartition.weakest_coverage_ratio < 100 && (
                  <StatusTag kind="warning">
                    {t("cashflow.plan.coverage", language)}: {activePartition.weakest_coverage_ratio.toFixed(0)}%
                  </StatusTag>
                )}
              </Space>
            }
          >
            <div
              className="cash-plan-bridge-grid"
              style={{ gridTemplateColumns: `repeat(${activePartition.bridge.steps.length + 1}, minmax(0, 1fr))` }}
            >
              {activePartition.bridge.steps.map((step, idx) => (
                <div key={idx} className={`cash-plan-bridge-step ${step.is_deduction ? "is-deduction" : "is-addition"}`}>
                  <div className="cash-plan-bridge-label">{step.label}</div>
                  <div className="font-tabular cash-plan-bridge-value">
                    {step.is_deduction ? "-" : "+"}{fmtMoney(step.amount, activePartition.currency)}
                  </div>
                </div>
              ))}
              <div className="cash-plan-bridge-total">
                <div className="cash-plan-bridge-label">{t("cashflow.plan.net_cash", language)}</div>
                <div className="font-tabular cash-plan-bridge-total-value">
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

