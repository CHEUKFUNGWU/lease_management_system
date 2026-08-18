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
  Space,
  Tag,
  Alert,
  Typography,
} from "antd";
import { SearchOutlined, AlertOutlined } from "@ant-design/icons";
import { t, type Language } from "../../lib/i18n";
import { fmtNum } from "../../lib/format";
import { tableScrollX } from "../../lib/tableScroll";
import type {
  CompareParams,
  GrainType,
  VarianceLine,
  WorkbenchCommands,
  WorkbenchSnapshot,
} from "../types";
import { GRAIN_TYPES } from "../types";

const { Paragraph } = Typography;

interface Props {
  snapshot: WorkbenchSnapshot;
  commands: WorkbenchCommands;
  language: Language;
}

export function VersionCompareTab({ snapshot, commands, language }: Props) {
  const [leftId, setLeftId] = useState<string>("");
  const [rightId, setRightId] = useState<string>("");
  const [period, setPeriod] = useState<string>(new Date().toISOString().slice(0, 7));
  const [grain, setGrain] = useState<GrainType>("group");
  const [currency] = useState<string>("CNY");
  const [exchangeRateVersion, setExchangeRateVersion] = useState<string>("");
  const [reportingCurrency, setReportingCurrency] = useState<string>("CNY");

  const handleCompare = async () => {
    if (!leftId || !rightId || !period) return;
    const params: CompareParams = {
      left_id: leftId,
      right_id: rightId,
      period,
      grain,
      currency: currency || undefined,
      exchange_rate_version: exchangeRateVersion || undefined,
      reporting_currency: reportingCurrency || undefined,
    };
    await commands.compareVersions(params);
  };

  const compareData = snapshot.compareResult;
  const varianceLines: VarianceLine[] = (compareData?.result?.variance_lines as VarianceLine[]) || [];

  const leftVersion = snapshot.versions.find((v) => v.id === leftId);
  const rightVersion = snapshot.versions.find((v) => v.id === rightId);

  const columns = [
    {
      title: t("fpna.col_subject_metric", language),
      dataIndex: "line_key",
      key: "line_key",
      render: (key: string) => <strong>{key}</strong>,
    },
    {
      title: `${leftVersion?.name || "Left"} (${compareData?.result?.left_basis || "Left"})`,
      dataIndex: "left_amount",
      key: "left_amount",
      align: "right" as const,
      render: (val: number) => fmtNum(val),
    },
    {
      title: `${rightVersion?.name || "Right"} (${compareData?.result?.right_basis || "Right"})`,
      dataIndex: "right_amount",
      key: "right_amount",
      align: "right" as const,
      render: (val: number) => fmtNum(val),
    },
    {
      title: t("fpna.col_variance_amount", language),
      dataIndex: "variance_amount",
      key: "variance_amount",
      align: "right" as const,
      render: (val: number) => {
        const cls = val > 0 ? "fpna-variance-up" : val < 0 ? "fpna-variance-down" : "fpna-variance-neutral";
        return <span className={cls}>{fmtNum(val)}</span>;
      },
    },
    {
      title: t("fpna.col_variance_pct", language),
      dataIndex: "variance_pct",
      key: "variance_pct",
      align: "right" as const,
      render: (val: number, record: VarianceLine) => {
        const pctStr = (val * 100).toFixed(2) + "%";
        if (record.significant_change) {
          return (
            <Tag color="volcano">
              <strong>{pctStr}</strong>
            </Tag>
          );
        }
        return <span>{pctStr}</span>;
      },
    },
  ];

  return (
    <div className="help-flow-vertical">
      {/* Top Filter Bar */}
      <Card size="small" className="fpna-margin-bottom-16">
        <Row gutter={[16, 16]} align="middle">
          <Col xs={24} sm={12} md={6}>
            <div className="fpna-param-block">
              <label className="fpna-param-label">
                {t("fpna.label_left_version", language)}
              </label>
              <Select
                className="fpna-full-width"
                placeholder={t("fpna.placeholder_pick_left_version", language)}
                value={leftId || undefined}
                onChange={setLeftId}
              >
                {snapshot.versions.map((v) => (
                  <Select.Option key={v.id} value={v.id}>
                    {v.name} ({v.version_type})
                  </Select.Option>
                ))}
              </Select>
            </div>
          </Col>

          <Col xs={24} sm={12} md={6}>
            <div className="fpna-param-block">
              <label className="fpna-param-label">
                {t("fpna.label_right_version", language)}
              </label>
              <Select
                className="fpna-full-width"
                placeholder={t("fpna.placeholder_pick_right_version", language)}
                value={rightId || undefined}
                onChange={setRightId}
              >
                {snapshot.versions.map((v) => (
                  <Select.Option key={v.id} value={v.id}>
                    {v.name} ({v.version_type})
                  </Select.Option>
                ))}
              </Select>
            </div>
          </Col>

          <Col xs={24} sm={8} md={3}>
            <div className="fpna-param-block">
              <label className="fpna-param-label">
                {t("fpna.label_period", language)}
              </label>
              <Input
                value={period}
                onChange={(e) => setPeriod(e.target.value)}
                placeholder="2026-01"
              />
            </div>
          </Col>

          <Col xs={24} sm={8} md={3}>
            <div className="fpna-param-block">
              <label className="fpna-param-label">
                {t("fpna.label_grain", language)}
              </label>
              <Select className="fpna-full-width" value={grain} onChange={setGrain}>
                {GRAIN_TYPES.map((g) => (
                  <Select.Option key={g} value={g}>
                    {g}
                  </Select.Option>
                ))}
              </Select>
            </div>
          </Col>

          <Col xs={24} sm={8} md={3}>
            <div className="fpna-param-block">
              <label className="fpna-param-label">
                {t("fpna.translation.reporting_currency", language)}
              </label>
              <Select
                className="fpna-full-width"
                value={reportingCurrency}
                onChange={setReportingCurrency}
                options={[
                  { label: "CNY", value: "CNY" },
                  { label: "HKD", value: "HKD" },
                  { label: "USD", value: "USD" },
                  { label: "EUR", value: "EUR" },
                ]}
              />
            </div>
          </Col>

          <Col xs={24} sm={8} md={3}>
            <div className="fpna-param-block">
              <label className="fpna-param-label">
                {t("fpna.translation.version_label", language)}
              </label>
              <Input
                value={exchangeRateVersion}
                onChange={(e) => setExchangeRateVersion(e.target.value)}
                placeholder="FY2026-Budget"
              />
            </div>
          </Col>

          <Col xs={24} sm={8} md={3}>
            <div className="fpna-param-block">
              <label className="fpna-param-label">&nbsp;</label>
              <Button
                type="primary"
                icon={<SearchOutlined />}
                onClick={handleCompare}
                loading={snapshot.compareLoading}
                disabled={!leftId || !rightId || !period}
                className="fpna-full-width"
              >
                {t("fpna.btn_compare", language)}
              </Button>
            </div>
          </Col>
        </Row>
      </Card>

      {/* Persistent Currency Translation Banner (PRD F3-c / CodebaseDesign §8) */}
      {compareData?.exchange_rate_version && (
        <Alert
          type="info"
          showIcon
          className="fpna-margin-bottom-16"
          message={`跨币种折算视图已启用 · 汇率版本: ${compareData.exchange_rate_version} · 报告币种: ${compareData.reporting_currency || reportingCurrency}`}
        />
      )}

      {/* Mixed Currency Actionable Guidance */}
      {compareData?.mixed_currency_guidance?.required && (
        <Alert
          type="warning"
          showIcon
          icon={<AlertOutlined />}
          className="fpna-margin-bottom-16"
          message={t("fpna.mixed_currency_title", language)}
          description={
            <div className="fpna-param-block">
              <Paragraph type="secondary" className="fpna-font-12">
                {t("fpna.mixed_currency_desc", language)}
              </Paragraph>
              <Space>
                <Input
                  placeholder={t("fpna.placeholder_fx_version", language)}
                  value={exchangeRateVersion}
                  onChange={(e) => setExchangeRateVersion(e.target.value)}
                />
                <Button type="primary" size="small" onClick={handleCompare} loading={snapshot.compareLoading}>
                  {t("fpna.btn_retry_with_fx", language)}
                </Button>
              </Space>
            </div>
          }
        />
      )}

      {compareData?.error && (
        <Alert
          type="error"
          showIcon
          className="fpna-margin-bottom-16"
          message={t("fpna.compare_failed", language)}
          description={compareData.error}
        />
      )}

      {/* Compare Results Table & Meta */}
      {compareData?.result && (
        <Card
          size="small"
          title={
            <Space>
              <span>{t("fpna.compare_result_title", language)}</span>
              <Tag color="blue">{compareData.result.period}</Tag>
              <Tag color="cyan">Basis: {compareData.basis}</Tag>
              {compareData.exchange_rate_version && (
                <Tag color="purple">FX: {compareData.exchange_rate_version}</Tag>
              )}
              {compareData.coverage && (
                <Tag color="green">
                  Coverage: {typeof compareData.coverage.ratio === "number" ? `${(compareData.coverage.ratio * 100).toFixed(0)}%` : compareData.coverage.status || "Complete"}
                </Tag>
              )}
            </Space>
          }
        >
          <Table
            dataSource={varianceLines}
            columns={columns}
            rowKey="line_key"
            pagination={false}
            scroll={tableScrollX(varianceLines.length, 800)}
          />
        </Card>
      )}
    </div>
  );
}
