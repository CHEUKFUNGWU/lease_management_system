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
      message.warning("Please provide a name for this forecast version");
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
        message.success(t("fpna.forecast_created_success", language) || "Forecast version committed successfully!");
        setCurrentStep(2);
      }
    } catch (err: unknown) {
      message.error(String(err));
    }
  };

  const handleFetchTrend = async () => {
    if (!trendForecastId || !trendActualId) {
      message.warning("Please select both forecast and actual versions");
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
          {row.replaced ? t("fpna.source_actual_replaced", language) || "Actual (Replaced)" : t("fpna.source_forecast_retained", language) || "Forecast (Retained)"}
        </Tag>
      ),
    },
    {
      title: t("fpna.col_record_count", language) || "Record Count",
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
      title: t("fpna.col_forecast_amount", language) || "Forecast",
      dataIndex: "forecast",
      key: "forecast",
      render: (v: number) => v.toLocaleString(undefined, { minimumFractionDigits: 2 }),
    },
    {
      title: t("fpna.col_actual_amount", language) || "Actual",
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
      title: t("fpna.col_accuracy", language) || "Accuracy",
      dataIndex: "accuracy",
      key: "accuracy",
      render: (acc: number | undefined) => (acc !== undefined ? `${acc.toFixed(1)}%` : "-"),
    },
    {
      title: t("fpna.col_bias", language) || "Bias (Actual - Forecast)",
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
        message={t("fpna.rolling_forecast_notice_title", language) || "滚动预测编制（Forecast Composition）"}
        description={
          t("fpna.rolling_forecast_notice_desc", language) ||
          "系统严格遵守单 Draft 预测不变量与混合替换纪律：截止期（Cutoff）前的期间自动由已关账的实际数替换，未来期间保留基准预测。编制完成后将生成带完整血缘关系的独立 Forecast 版本。"
        }
      />

      {/* Composition Wizard Card */}
      <Card
        title={
          <Space>
            <ThunderboltOutlined className="fpna-tree-icon" />
            <span>{t("fpna.wizard_title", language) || "滚动预测编制向导"}</span>
          </Space>
        }
        className="fpna-margin-bottom-16"
      >
        <Steps
          current={currentStep}
          items={[
            { title: t("fpna.step_config", language) || "选择基准与实际" },
            { title: t("fpna.step_preview", language) || "混合差异预览" },
            { title: t("fpna.step_complete", language) || "固化为新版本" },
          ]}
          className="fpna-margin-bottom-16"
        />

        {currentStep === 0 && (
          <Row gutter={[16, 16]}>
            <Col xs={24} md={8}>
              <Text strong>{t("fpna.label_baseline_version", language) || "基准预测/预算版本"} *</Text>
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
              <Text strong>{t("fpna.label_actual_version", language) || "实际发生数据版本"} *</Text>
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
              <Text strong>{t("fpna.label_actual_cutoff", language) || "实际数据截止期间 (Cutoff)"} *</Text>
              <Input
                placeholder="YYYY-MM (例: 2026-03)"
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
                {t("fpna.btn_preview_blend", language) || "生成混合预测预览 →"}
              </Button>
            </Col>
          </Row>
        )}

        {currentStep === 1 && snapshot.proposedForecast && (
          <div>
            <Row gutter={[16, 16]} className="fpna-margin-bottom-16">
              <Col xs={12} md={6}>
                <Statistic title="From Period" value={snapshot.proposedForecast.from_period} />
              </Col>
              <Col xs={12} md={6}>
                <Statistic title="To Period" value={snapshot.proposedForecast.to_period} />
              </Col>
              <Col xs={12} md={6}>
                <Statistic title="Actual Cutoff" value={snapshot.proposedForecast.actual_cutoff_period} />
              </Col>
              <Col xs={12} md={6}>
                <Statistic title="Total Plan Lines" value={snapshot.proposedForecast.lines.length} />
              </Col>
            </Row>

            <Title level={5}>{t("fpna.title_period_blend_breakdown", language) || "期间替换明细"}</Title>
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
            <Title level={5}>{t("fpna.title_commit_details", language) || "版本固化参数"}</Title>
            <Row gutter={[16, 16]} className="fpna-margin-bottom-16">
              <Col xs={24} md={12}>
                <Text strong>{t("fpna.col_version_name", language)} *</Text>
                <Input
                  value={versionName}
                  onChange={(e) => setVersionName(e.target.value)}
                  placeholder="例: FC-2026-Q1"
                />
              </Col>
              <Col xs={24} md={6}>
                <Text strong>{t("fpna.col_scenario_type", language)}</Text>
                <Select
                  className="fpna-width-full"
                  value={scenarioType}
                  onChange={setScenarioType}
                  options={[
                    { value: "baseline", label: "Baseline" },
                    { value: "upside", label: "Upside" },
                    { value: "downside", label: "Downside" },
                  ]}
                />
              </Col>
              <Col xs={24} md={6}>
                <Text strong>Assumption Version</Text>
                <Input
                  value={assumptionVersion}
                  onChange={(e) => setAssumptionVersion(e.target.value)}
                  placeholder="例: macro-growth-v1"
                />
              </Col>
            </Row>

            <Space>
              <Button onClick={() => setCurrentStep(0)}>
                ← {t("fpna.btn_back_to_config", language) || "返回调整"}
              </Button>
              <Button
                type="primary"
                icon={<SaveOutlined />}
                loading={snapshot.forecastLoading}
                onClick={handleCommit}
              >
                {t("fpna.btn_commit_forecast", language) || "确认保存为正式预测版本"}
              </Button>
            </Space>
          </div>
        )}

        {currentStep === 2 && (
          <div className="fpna-text-center">
            <CheckCircleOutlined className="fpna-success-icon" />
            <Title level={4}>{t("fpna.forecast_commit_success_title", language) || "滚动预测版本已成功创建！"}</Title>
            <Paragraph type="secondary">
              {t("fpna.forecast_commit_success_desc", language) ||
                "新版本已归档并接入版本谱系树（Lineage Tree）。您可在「版本管理」中查看、审核或冻结。"}
            </Paragraph>
            <Button type="primary" onClick={() => setCurrentStep(0)}>
              {t("fpna.btn_create_another", language) || "继续编制下一个预测"}
            </Button>
          </div>
        )}
      </Card>

      {/* Accuracy & Bias Analysis Card */}
      <Card
        title={
          <Space>
            <LineChartOutlined className="fpna-tree-icon" />
            <span>{t("fpna.title_accuracy_trend", language) || "预测准确度与系统性偏差复盘 (Bias Trend)"}</span>
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
            <Text strong>{t("fpna.label_baseline_version", language) || "待复盘预测版本"}</Text>
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
            <Text strong>{t("fpna.label_actual_version", language) || "比对实际版本"}</Text>
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
              {t("fpna.btn_analyze_accuracy", language) || "分析准确度"}
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
                message={t("fpna.alert_systemic_bias_title", language) || "检测到显著系统性偏差 (Systemic Bias Detected)"}
                description={
                  <span>
                    {snapshot.accuracyTrend.systemic_direction === "overestimation"
                      ? "连续 3 期以上实际值低于预测值（系统性高估/Overestimation），建议在下一轮滚动预测中审慎调低基线增长假设。"
                      : "连续 3 期以上实际值高于预测值（系统性低估/Underestimation），业务动能可能强于原定假设。"}
                    （连续同向期数: {snapshot.accuracyTrend.consecutive_bias_count} 期，累计偏差:{" "}
                    {snapshot.accuracyTrend.total_bias.toLocaleString()}）
                  </span>
                }
              />
            )}

            <Row gutter={[16, 16]} className="fpna-margin-bottom-16">
              <Col xs={12} md={6}>
                <Statistic
                  title={t("fpna.stat_overall_mape", language) || "平均绝对百分比误差 (MAPE)"}
                  value={snapshot.accuracyTrend.overall_mean_abs_pct !== undefined ? `${snapshot.accuracyTrend.overall_mean_abs_pct}%` : "-"}
                />
              </Col>
              <Col xs={12} md={6}>
                <Statistic
                  title={t("fpna.stat_total_bias", language) || "累计净偏差 (Total Bias)"}
                  value={snapshot.accuracyTrend.total_bias}
                  precision={2}
                />
              </Col>
              <Col xs={12} md={6}>
                <Statistic
                  title={t("fpna.stat_consecutive_streak", language) || "最长连续同向偏差期数"}
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
