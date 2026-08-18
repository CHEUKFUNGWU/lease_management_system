"use client";

import { useEffect, useState } from "react";
import {
  Card,
  Row,
  Col,
  Statistic,
  Table,
  Typography,
  Tag,
  Space,
  Select,
  Button,
  Spin,
  Alert,
} from "antd";
import {
  DollarOutlined,
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

const { Text, Title } = Typography;

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
  }, [token, fromPeriod, toPeriod, dataClassification, datasetVersion]);

  const activePartition: CashPlanPartition | undefined =
    planData?.partitions?.find((p) => p.currency === selectedCurrency) ||
    planData?.partitions?.[0];

  const monthlyColumns = [
    {
      title: "月份 (Period)",
      dataIndex: "period",
      key: "period",
      width: 100,
      fixed: "left" as const,
    },
    {
      title: "营业收入",
      dataIndex: "revenue",
      key: "revenue",
      align: "right" as const,
      render: (v: number) => fmtMoney(v, activePartition?.currency || "CNY"),
    },
    {
      title: t("cashflow.plan.operating_cash", language),
      dataIndex: "operating_cash",
      key: "operating_cash",
      align: "right" as const,
      render: (v: number) => (
        <span style={{ color: v >= 0 ? "var(--color-success, #52c41a)" : "var(--color-error, #ff4d4f)", fontWeight: 500 }}>
          {fmtMoney(v, activePartition?.currency || "CNY")}
        </span>
      ),
    },
    {
      title: t("cashflow.plan.rent_offset", language),
      dataIndex: "rent_offset",
      key: "rent_offset",
      align: "right" as const,
      render: (v: number) => (
        <span style={{ color: "var(--fg-secondary)" }}>
          +{fmtMoney(v, activePartition?.currency || "CNY")}
        </span>
      ),
    },
    {
      title: t("cashflow.plan.lease_outflow", language),
      dataIndex: "lease_outflow",
      key: "lease_outflow",
      align: "right" as const,
      render: (v: number) => (
        <span style={{ color: "var(--color-error, #ff4d4f)" }}>
          -{fmtMoney(v, activePartition?.currency || "CNY")}
        </span>
      ),
    },
    {
      title: t("cashflow.plan.capex_outflow", language),
      dataIndex: "capex_outflow",
      key: "capex_outflow",
      align: "right" as const,
      render: (v: number) => (
        <span style={{ color: v > 0 ? "var(--color-warning, #faad14)" : "var(--fg-muted)" }}>
          -{fmtMoney(v, activePartition?.currency || "CNY")}
        </span>
      ),
    },
    {
      title: t("cashflow.plan.net_cash", language),
      dataIndex: "net_cash_plan",
      key: "net_cash_plan",
      align: "right" as const,
      render: (v: number) => (
        <strong style={{ color: v >= 0 ? "var(--color-primary, #1890ff)" : "var(--color-error, #ff4d4f)" }}>
          {fmtMoney(v, activePartition?.currency || "CNY")}
        </strong>
      ),
    },
  ];

  return (
    <Space direction="vertical" size={16} style={{ width: "100%" }}>
      {/* Controls Bar */}
      <Card size="small">
        <Row gutter={[16, 16]} align="middle" justify="space-between">
          <Col xs={24} md={18}>
            <Space wrap size={12}>
              <Text strong>期间范围:</Text>
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
              <Text>至</Text>
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

              <Text strong style={{ marginLeft: 8 }}>数据分类:</Text>
              <Select
                value={dataClassification}
                onChange={setDataClassification}
                style={{ width: 120 }}
                options={[
                  { label: "正式生产", value: "production" },
                  { label: "模拟测算", value: "simulated" },
                ]}
              />

              {planData && planData.partitions.length > 1 && (
                <>
                  <Text strong style={{ marginLeft: 8 }}>币种分区:</Text>
                  <Select
                    value={selectedCurrency}
                    onChange={setSelectedCurrency}
                    style={{ width: 90 }}
                    options={planData.partitions.map((p) => ({
                      label: p.currency,
                      value: p.currency,
                    }))}
                  />
                </>
              )}
            </Space>
          </Col>
          <Col xs={24} md={6} style={{ textAlign: "right" }}>
            <Button icon={<SyncOutlined />} onClick={loadPlan} loading={loading}>
              重新合成
            </Button>
          </Col>
        </Row>
      </Card>

      {error && <Alert type="error" message={error} showIcon closable />}

      {loading && !planData ? (
        <div style={{ textAlign: "center", padding: "40px 0" }}>
          <Spin size="large" />
        </div>
      ) : activePartition ? (
        <>
          {/* Top KPI Cards */}
          <Row gutter={[16, 16]}>
            <Col xs={12} sm={8} md={4}>
              <Card size="small">
                <Statistic
                  title={t("cashflow.plan.operating_cash", language)}
                  value={activePartition.total_operating_cash}
                  formatter={(v) => fmtMoney(Number(v), activePartition.currency)}
                  valueStyle={{ fontSize: 18, color: activePartition.total_operating_cash >= 0 ? "#52c41a" : "#ff4d4f" }}
                />
              </Card>
            </Col>
            <Col xs={12} sm={8} md={5}>
              <Card size="small">
                <Statistic
                  title={t("cashflow.plan.rent_offset", language)}
                  value={activePartition.total_rent_offset}
                  formatter={(v) => `+${fmtMoney(Number(v), activePartition.currency)}`}
                  valueStyle={{ fontSize: 18, color: "var(--fg-secondary)" }}
                />
              </Card>
            </Col>
            <Col xs={12} sm={8} md={5}>
              <Card size="small">
                <Statistic
                  title={t("cashflow.plan.lease_outflow", language)}
                  value={activePartition.total_lease_outflow}
                  formatter={(v) => `-${fmtMoney(Number(v), activePartition.currency)}`}
                  valueStyle={{ fontSize: 18, color: "#ff4d4f" }}
                />
              </Card>
            </Col>
            <Col xs={12} sm={8} md={5}>
              <Card size="small">
                <Statistic
                  title={t("cashflow.plan.capex_outflow", language)}
                  value={activePartition.total_capex_outflow}
                  formatter={(v) => `-${fmtMoney(Number(v), activePartition.currency)}`}
                  valueStyle={{ fontSize: 18, color: "#faad14" }}
                />
              </Card>
            </Col>
            <Col xs={12} sm={8} md={5}>
              <Card size="small" style={{ borderLeft: "3px solid #1890ff" }}>
                <Statistic
                  title={t("cashflow.plan.net_cash", language)}
                  value={activePartition.total_net_cash_plan}
                  formatter={(v) => fmtMoney(Number(v), activePartition.currency)}
                  valueStyle={{ fontSize: 20, fontWeight: 600, color: activePartition.total_net_cash_plan >= 0 ? "#1890ff" : "#ff4d4f" }}
                />
              </Card>
            </Col>
          </Row>

          {/* Conservation Bridge Card */}
          <Card
            size="small"
            title={
              <Space>
                <span>{t("cashflow.plan.bridge_title", language)}</span>
                {activePartition.bridge.is_conserved ? (
                  <Tag color="success" icon={<CheckCircleOutlined />}>
                    {t("cashflow.plan.conserved", language)}
                  </Tag>
                ) : (
                  <Tag color="error" icon={<WarningOutlined />}>
                    {t("cashflow.plan.unconserved", language)} (残差: {activePartition.bridge.rounding_residual})
                  </Tag>
                )}
                {activePartition.weakest_coverage_ratio != null && activePartition.weakest_coverage_ratio < 100 && (
                  <Tag color="warning">
                    覆盖度: {activePartition.weakest_coverage_ratio.toFixed(0)}%
                  </Tag>
                )}
              </Space>
            }
          >
            <Row gutter={[16, 16]}>
              {activePartition.bridge.steps.map((step, idx) => (
                <Col key={idx} xs={12} sm={6} md={3}>
                  <Card size="small" style={{ background: "var(--bg-elevated, #fafafa)", textAlign: "center" }}>
                    <div style={{ fontSize: 11, color: "var(--fg-secondary)", marginBottom: 4 }}>
                      {step.label}
                    </div>
                    <div style={{ fontSize: 14, fontWeight: 600, color: step.is_deduction ? "#ff4d4f" : "#52c41a" }}>
                      {step.is_deduction ? "-" : "+"}{fmtMoney(step.amount, activePartition.currency)}
                    </div>
                  </Card>
                </Col>
              ))}
              <Col xs={12} sm={6} md={4}>
                <Card size="small" style={{ background: "rgba(24, 144, 255, 0.08)", borderColor: "#1890ff", textAlign: "center" }}>
                  <div style={{ fontSize: 11, color: "var(--fg-secondary)", marginBottom: 4 }}>
                    {t("cashflow.plan.net_cash", language)}
                  </div>
                  <div style={{ fontSize: 15, fontWeight: 700, color: "#1890ff" }}>
                    {fmtMoney(activePartition.total_net_cash_plan, activePartition.currency)}
                  </div>
                </Card>
              </Col>
            </Row>
          </Card>

          {/* Monthly Breakdown Table */}
          <Card size="small" title="月度资金计划明细">
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
    </Space>
  );
}
