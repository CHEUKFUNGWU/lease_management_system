"use client";

import React, { useState } from "react";
import {
  Card,
  Row,
  Col,
  Select,
  Input,
  Button,
  Table,
  Tag,
  Alert,
  Space,
  Typography,
  Steps,
  Divider,
  Statistic,
  message,
} from "antd";
import {
  ThunderboltOutlined,
  CheckCircleOutlined,
  WarningOutlined,
  SaveOutlined,
  EyeOutlined,
  LineChartOutlined,
  DownloadOutlined,
} from "@ant-design/icons";
import { t, type Language } from "../../lib/i18n";
import { tableScrollX } from "../../lib/tableScroll";
import type {
  AccuracyTrendPoint,
  HybridForecastInput,
  PeriodBlendSummary,
  ScenarioType,
  WorkbenchCommands,
  WorkbenchSnapshot,
} from "../types";

const { Text, Title, Paragraph } = Typography;

interface Props {
  snapshot: WorkbenchSnapshot;
  commands: WorkbenchCommands;
  language: Language;
}

export function RollingForecastTab({ snapshot, commands, language }: Props) {
  const [currentStep, setCurrentStep] = useState(0);

  // Wizard parameters
  const [baselineId, setBaselineId] = useState<string>("");
  const [actualId, setActualId] = useState<string>("");
  const [cutoffPeriod, setCutoffPeriod] = useState<string>("");

  // Commit parameters
  const [versionName, setVersionName] = useState<string>("");
  const [scenarioType, setScenarioType] = useState<ScenarioType>("baseline");
  const [assumptionVersion, setAssumptionVersion] = useState<string>("");
  const [fxVersion, setFxVersion] = useState<string>("");
  const [metricVersion, setMetricVersion] = useState<string>("retail-kpi-v1");

  // Accuracy trend selection
  const [trendForecastId, setTrendForecastId] = useState<string>("");
  const [trendActualId, setTrendActualId] = useState<string>("");

  const handlePreview = async () => {
    if (!baselineId || !actualId || !cutoffPeriod) {
      message.warning(t("fpna.err_missing_required", language) || "Please fill all required fields");
      return;
    }
    if (!/^\d{4}-\d{2}$/.test(cutoffPeriod)) {
      message.error(t("fpna.err_period_format", language));
      return;
    }

    try {
      const input: HybridForecastInput = {
        forecast_id: baselineId,
        actual_id: actualId,
        actual_cutoff_period: cutoffPeriod,
        scenario_type: scenarioType,
        assumption_version: assumptionVersion,
        exchange_rate_version: fxVersion,
        metric_definition_version: metricVersion,
      };
      const res = await commands.previewHybridForecast(input);
      if (res) {
        setVersionName(`FC-${cutoffPeriod}-${new Date().toISOString().slice(0, 10)}`);
        setCurrentStep(1);
      }
    } catch (err: unknown) {
      message.error(String(err));
    }
  };

  const handleCommit = async () => {
    if (!versionName) {
      message.warning(t("fpna.msg_name_required", language));
      return;
    }

    try {
      const input: HybridForecastInput = {
        forecast_id: baselineId,
        actual_id: actualId,
        actual_cutoff_period: cutoffPeriod,
        name: versionName,
        scenario_type: scenarioType,
        assumption_version: assumptionVersion,
        exchange_rate_version: fxVersion,
        metric_definition_version: metricVersion,
        persist: true,
      };
      const created = await commands.commitHybridForecast(input);
      if (created) {
        message.success(t("fpna.forecast_created_success", language));
        setCurrentStep(2);
      }
    } catch (err: unknown) {
      message.error(String(err));
    }
  };

  const handleFetchTrend = async () => {
    if (!trendForecastId || !trendActualId) {
      message.warning(t("fpna.msg_select_both_versions", language));
      return;
    }
    await commands.fetchAccuracyTrend(trendForecastId, trendActualId);
  };

  const blendColumns = [
    {
      title: t("fpna.col_period", language),
      dataIndex: "period",
      key: "period",
      render: (p: string) => <strong>{p}</strong>,
    },
    {
      title: t("fpna.col_source_type", language),
      dataIndex: "source_type",
      key: "source_type",
      render: (st: string, row: PeriodBlendSummary) => (
        <Tag color={row.replaced ? "blue" : "purple"}>
          {row.replaced ? t("fpna.source_actual_replaced", language) : t("fpna.source_forecast_retained", language)}
        </Tag>
      ),
    },
    {
      title: t("fpna.col_record_count", language),
      dataIndex: "record_count",
      key: "record_count",
    },
  ];

  const trendColumns = [
    {
      title: t("fpna.col_period", language),
      dataIndex: "period",
      key: "period",
      render: (p: string) => <strong>{p}</strong>,
    },
    {
      title: t("fpna.col_forecast_amount", language),
      dataIndex: "forecast",
      key: "forecast",
      render: (v: number) => v.toLocaleString(undefined, { minimumFractionDigits: 2 }),
    },
    {
      title: t("fpna.col_actual_amount", language),
      dataIndex: "actual",
      key: "actual",
      render: (v: number) => v.toLocaleString(undefined, { minimumFractionDigits: 2 }),
    },
    {
      title: t("fpna.col_variance", language),
      dataIndex: "variance",
      key: "variance",
      render: (v: number) => (
        <span className={v > 0 ? "fpna-variance-up" : v < 0 ? "fpna-variance-down" : "fpna-variance-neutral"}>
          {v > 0 ? `+${v.toLocaleString(undefined, { minimumFractionDigits: 2 })}` : v.toLocaleString(undefined, { minimumFractionDigits: 2 })}
        </span>
      ),
    },
    {
      title: t("fpna.col_accuracy", language),
      dataIndex: "accuracy",
      key: "accuracy",
      render: (acc: number | undefined) => (acc !== undefined ? `${acc.toFixed(1)}%` : "-"),
    },
    {
      title: t("fpna.col_bias", language),
      dataIndex: "bias",
      key: "bias",
      render: (b: number) => (
        <Tag color={b > 0 ? "green" : b < 0 ? "volcano" : "default"}>
          {b > 0 ? `+${b.toFixed(2)}` : b.toFixed(2)}
        </Tag>
      ),
    },
  ];

  const exportTrendCSV = () => {
    if (!snapshot.accuracyTrend?.points?.length) return;
    const headers = ["Period", "Forecast", "Actual", "Variance", "Accuracy", "Bias"];
    const rows = snapshot.accuracyTrend.points.map((p) => [
      p.period,
      p.forecast,
      p.actual,
      p.variance,
      p.accuracy !== undefined ? `${p.accuracy}%` : "",
      p.bias,
    ]);
    const csvContent = "\uFEFF" + [headers.join(","), ...rows.map((r) => r.join(","))].join("\n");
    const blob = new Blob([csvContent], { type: "text/csv;charset=utf-8;" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.setAttribute("href", url);
    link.setAttribute("download", `forecast_accuracy_trend_${new Date().toISOString().slice(0, 10)}.csv`);
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  };

  return (
    <div className="fpna-tab-content">
      {/* Overview Alert */}
      <Alert
        type="info"
        showIcon
        icon={<ThunderboltOutlined />}
        className="fpna-margin-bottom-16"
        message={t("fpna.rolling_forecast_notice_title", language)}
        description={t("fpna.rolling_forecast_notice_desc", language)}
      />

      {/* Composition Wizard Card */}
      <Card
        title={
          <Space>
            <ThunderboltOutlined className="fpna-tree-icon" />
            <span>{t("fpna.wizard_title", language)}</span>
          </Space>
        }
        className="fpna-margin-bottom-16"
      >
        <Steps
          current={currentStep}
          items={[
            { title: t("fpna.step_config", language) },
            { title: t("fpna.step_preview", language) },
            { title: t("fpna.step_complete", language) },
          ]}
          className="fpna-margin-bottom-16"
        />

        {currentStep === 0 && (
          <Row gutter={[16, 16]}>
            <Col xs={24} md={8}>
              <Text strong>{t("fpna.label_baseline_version", language)} *</Text>
              <Select
                className="fpna-width-full"
                placeholder={t("fpna.placeholder_pick_left_version", language)}
                value={baselineId || undefined}
                onChange={setBaselineId}
                options={snapshot.versions.map((v) => ({
                  value: v.id,
                  label: `${v.name} (${v.version_type.toUpperCase()}${v.is_official ? " / Official" : ""})`,
                }))}
              />
            </Col>
            <Col xs={24} md={8}>
              <Text strong>{t("fpna.label_actual_version", language)} *</Text>
              <Select
                className="fpna-width-full"
                placeholder={t("fpna.placeholder_pick_right_version", language)}
                value={actualId || undefined}
                onChange={setActualId}
                options={snapshot.versions.map((v) => ({
                  value: v.id,
                  label: `${v.name} (${v.version_type.toUpperCase()})`,
                }))}
              />
            </Col>
            <Col xs={24} md={8}>
              <Text strong>{t("fpna.label_actual_cutoff", language)} *</Text>
              <Input
                placeholder={t("fpna.placeholder_cutoff", language)}
                value={cutoffPeriod}
                onChange={(e) => setCutoffPeriod(e.target.value.trim())}
              />
            </Col>

            <Col xs={24}>
              <Divider />
              <Button
                type="primary"
                icon={<EyeOutlined />}
                loading={snapshot.forecastLoading}
                onClick={handlePreview}
              >
                {t("fpna.btn_preview_blend", language)}
              </Button>
            </Col>
          </Row>
        )}

        {currentStep === 1 && snapshot.proposedForecast && (
          <div>
            <div className="stripe-metric-grid fpna-margin-bottom-16" style={{ gridTemplateColumns: "repeat(4, minmax(0, 1fr))" }}>
              <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 80, padding: "14px 18px" }}>
                <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("fpna.card_from_period", language)}</span>
                <div style={{ margin: "6px 0 0" }}>
                  <Typography.Text className="font-tabular" style={{ fontSize: 20, fontWeight: 600, color: "var(--fg-primary)" }}>
                    {snapshot.proposedForecast.from_period}
                  </Typography.Text>
                </div>
              </div>
              <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 80, padding: "14px 18px" }}>
                <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("fpna.card_to_period", language)}</span>
                <div style={{ margin: "6px 0 0" }}>
                  <Typography.Text className="font-tabular" style={{ fontSize: 20, fontWeight: 600, color: "var(--fg-primary)" }}>
                    {snapshot.proposedForecast.to_period}
                  </Typography.Text>
                </div>
              </div>
              <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 80, padding: "14px 18px" }}>
                <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("fpna.card_actual_cutoff", language)}</span>
                <div style={{ margin: "6px 0 0" }}>
                  <Typography.Text className="font-tabular" style={{ fontSize: 20, fontWeight: 600, color: "var(--fg-primary)" }}>
                    {snapshot.proposedForecast.actual_cutoff_period}
                  </Typography.Text>
                </div>
              </div>
              <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 80, padding: "14px 18px" }}>
                <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("fpna.card_total_plan_lines", language)}</span>
                <div style={{ margin: "6px 0 0" }}>
                  <Typography.Text className="font-tabular" style={{ fontSize: 20, fontWeight: 600, color: "var(--fg-primary)" }}>
                    {snapshot.proposedForecast.lines.length}
                  </Typography.Text>
                </div>
              </div>
            </div>

            <Title level={5}>{t("fpna.title_period_blend_breakdown", language)}</Title>
            <Table
              dataSource={snapshot.proposedForecast.period_blends}
              columns={blendColumns}
              rowKey="period"
              pagination={false}
              size="small"
              className="fpna-margin-bottom-16"
              scroll={tableScrollX(snapshot.proposedForecast.period_blends.length, 600)}
            />

            <Divider />
            <Title level={5}>{t("fpna.title_commit_details", language)}</Title>
            <Row gutter={[16, 16]} className="fpna-margin-bottom-16">
              <Col xs={24} md={12}>
                <Text strong>{t("fpna.col_version_name", language)} *</Text>
                <Input
                  value={versionName}
                  onChange={(e) => setVersionName(e.target.value)}
                  placeholder={t("fpna.placeholder_version_name", language)}
                />
              </Col>
              <Col xs={24} md={6}>
                <Text strong>{t("fpna.col_scenario_type", language)}</Text>
                <Select
                  className="fpna-width-full"
                  value={scenarioType}
                  onChange={setScenarioType}
                  options={[
                    { value: "baseline", label: t("fpna.scenario_baseline", language) },
                    { value: "upside", label: t("fpna.scenario_upside", language) },
                    { value: "downside", label: t("fpna.scenario_downside", language) },
                  ]}
                />
              </Col>
              <Col xs={24} md={6}>
                <Text strong>{t("fpna.form_assumption_version", language)}</Text>
                <Input
                  value={assumptionVersion}
                  onChange={(e) => setAssumptionVersion(e.target.value)}
                  placeholder={t("fpna.placeholder_assumption_version", language)}
                />
              </Col>
            </Row>

            <Space>
              <Button onClick={() => setCurrentStep(0)}>
                ← {t("fpna.btn_back_to_config", language)}
              </Button>
              <Button
                type="primary"
                icon={<SaveOutlined />}
                loading={snapshot.forecastLoading}
                onClick={handleCommit}
              >
                {t("fpna.btn_commit_forecast", language)}
              </Button>
            </Space>
          </div>
        )}

        {currentStep === 2 && (
          <div className="fpna-text-center">
            <CheckCircleOutlined className="fpna-success-icon" />
            <Title level={4}>{t("fpna.forecast_commit_success_title", language)}</Title>
            <Paragraph type="secondary">
              {t("fpna.forecast_commit_success_desc", language)}
            </Paragraph>
            <Button type="primary" onClick={() => setCurrentStep(0)}>
              {t("fpna.btn_create_another", language)}
            </Button>
          </div>
        )}
      </Card>

      {/* Accuracy & Bias Analysis Card */}
      <Card
        title={
          <Space>
            <LineChartOutlined className="fpna-tree-icon" />
            <span>{t("fpna.title_accuracy_trend", language)}</span>
          </Space>
        }
        extra={
          snapshot.accuracyTrend?.points?.length ? (
            <Button icon={<DownloadOutlined />} onClick={exportTrendCSV}>
              {t("common.export", language)} CSV
            </Button>
          ) : null
        }
      >
        <Row gutter={[16, 16]} className="fpna-margin-bottom-16">
          <Col xs={24} md={10}>
            <Text strong>{t("fpna.label_forecast_to_review", language)}</Text>
            <Select
              className="fpna-width-full"
              placeholder={t("fpna.placeholder_pick_left_version", language)}
              value={trendForecastId || undefined}
              onChange={setTrendForecastId}
              options={snapshot.versions.map((v) => ({
                value: v.id,
                label: `${v.name} (${v.version_type.toUpperCase()})`,
              }))}
            />
          </Col>
          <Col xs={24} md={10}>
            <Text strong>{t("fpna.label_actual_to_compare", language)}</Text>
            <Select
              className="fpna-width-full"
              placeholder={t("fpna.placeholder_pick_right_version", language)}
              value={trendActualId || undefined}
              onChange={setTrendActualId}
              options={snapshot.versions.map((v) => ({
                value: v.id,
                label: `${v.name} (${v.version_type.toUpperCase()})`,
              }))}
            />
          </Col>
          <Col xs={24} md={4}>
            <Button
              type="primary"
              className="fpna-trend-btn"
              loading={snapshot.accuracyLoading}
              onClick={handleFetchTrend}
            >
              {t("fpna.btn_analyze_accuracy", language)}
            </Button>
          </Col>
        </Row>

        {snapshot.accuracyTrend && (
          <div>
            {/* Systemic Bias Warning Alert */}
            {snapshot.accuracyTrend.has_systemic_bias && (
              <Alert
                type="warning"
                showIcon
                icon={<WarningOutlined />}
                className="fpna-margin-bottom-16"
                message={t("fpna.alert_systemic_bias_title", language)}
                description={
                  <span>
                    {snapshot.accuracyTrend.systemic_direction === "overestimation"
                      ? t("fpna.bias_overestimation", language)
                      : t("fpna.bias_underestimation", language)}
                    {t("fpna.bias_streak_count", language, { count: String(snapshot.accuracyTrend.consecutive_bias_count), total: snapshot.accuracyTrend.total_bias.toLocaleString() })}
                  </span>
                }
              />
            )}

            <Row gutter={[16, 16]} className="fpna-margin-bottom-16">
              <Col xs={12} md={6}>
                <Statistic
                  title={t("fpna.stat_overall_mape", language)}
                  value={snapshot.accuracyTrend.overall_mean_abs_pct !== undefined ? `${snapshot.accuracyTrend.overall_mean_abs_pct}%` : "-"}
                />
              </Col>
              <Col xs={12} md={6}>
                <Statistic
                  title={t("fpna.stat_total_bias", language)}
                  value={snapshot.accuracyTrend.total_bias}
                  precision={2}
                />
              </Col>
              <Col xs={12} md={6}>
                <Statistic
                  title={t("fpna.stat_consecutive_streak", language)}
                  value={snapshot.accuracyTrend.consecutive_bias_count}
                />
              </Col>
            </Row>

            <Table
              dataSource={snapshot.accuracyTrend.points}
              columns={trendColumns}
              rowKey="period"
              pagination={false}
              size="small"
              scroll={tableScrollX(snapshot.accuracyTrend.points.length, 600)}
            />
          </div>
        )}
      </Card>
    </div>
  );
}
