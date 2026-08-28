"use client";

import { StatusTag } from "../components/StatusTag";

import { useState } from "react";
import { Alert, Button, Card, Col, Form, InputNumber, Row, Space, Table, Tag } from "antd";
import { ExperimentOutlined } from "@ant-design/icons";
import { Bar, BarChart, CartesianGrid, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { reportApi } from "../lib/api";
import { fmtMoney } from "../lib/format";
import { notifyError } from "../lib/notify";
import { tableScrollX } from "../lib/tableScroll";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";

interface Ladder {
  labels: string[];
  values: number[];
  total: number;
}

interface ScenarioResult {
  scenario: {
    name: string;
    renewal_rate: number;
    renewal_term_months: number;
    renewal_uplift_percent: number;
    closure_rate: number;
    closure_cost_months: number;
  };
  currency: string;
  ladder: Ladder;
  committed_total: number;
  renewal_total: number;
  closure_total: number;
  total: number;
  expiring_leases: number;
  caveat: string;
}

export function ScenarioPanel({ token }: { token: string | null }) {
  const { language } = useLanguage();
  const [form] = Form.useForm();
  const [results, setResults] = useState<ScenarioResult[]>([]);
  const [warning, setWarning] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const run = async (values: any) => {
    if (!token) return;
    setLoading(true);
    try {
      const response = await reportApi.cashflowScenario(
        {
          horizon_months: values.horizon_months,
          // The do-nothing case is always run alongside the plan: a scenario
          // means nothing without the baseline it moved away from.
          scenarios: [
            { name: "基准（不做假设）" },
            {
              name: `${Math.round((values.renewal_rate || 0) * 100)}% 续租、${Math.round(
                (values.closure_rate || 0) * 100
              )}% 关店`,
              renewal_rate: values.renewal_rate || 0,
              renewal_term_months: values.renewal_term_months,
              renewal_uplift_percent: values.renewal_uplift_percent,
              closure_rate: values.closure_rate || 0,
              closure_cost_months: values.closure_cost_months || 0,
            },
          ],
        },
        token
      );
      setResults(((response as { results?: ScenarioResult[] }).results) || []);
      setWarning((response as { currency_warning?: string | null }).currency_warning || null);
    } catch (error: any) {
      notifyError(error?.message || "情景测算失败");
      setResults([]);
    } finally {
      setLoading(false);
    }
  };

  // When the portfolio spans currencies the totals are a sum in none of them,
  // so they are shown as bare numbers. Stamping the first contract's currency
  // on them would be a claim about money that is not true.
  const currency = warning ? null : results[0]?.currency;
  const chartData = results.length
    ? results[0].ladder.labels.map((label, index) => {
        const point: Record<string, string | number> = { band: label };
        results.forEach((result) => {
          point[result.scenario.name] = result.ladder.values[index];
        });
        return point;
      })
    : [];

  return (
    <Card
      title={t("cashflow.scenario.title", language)}
      style={{ borderRadius: 10, marginTop: 16 }}
      extra={
        <Button type="primary" icon={<ExperimentOutlined />} loading={loading} onClick={() => form.submit()}>
          {t("cashflow.scenario.btn_run", language)}
        </Button>
      }
    >
      <div style={{ color: "var(--fg-muted)", fontSize: 13, marginBottom: 16 }}>
        {t("cashflow.scenario.intro", language)}
      </div>

      <Form
        form={form}
        layout="vertical"
        onFinish={run}
      >
        <Row gutter={12}>
          <Col xs={12} md={4}>
            <Form.Item label={t("cashflow.scenario.horizon_months", language)} name="horizon_months" rules={[{ required: true, message: t("cashflow.scenario.req_horizon", language) }]}>
              <InputNumber style={{ width: "100%" }} min={12} max={120} />
            </Form.Item>
          </Col>
          <Col xs={12} md={4}>
            <Form.Item label={t("cashflow.scenario.renewal_rate", language)} name="renewal_rate" extra="0–1">
              <InputNumber style={{ width: "100%" }} min={0} max={1} step={0.05} />
            </Form.Item>
          </Col>
          <Col xs={12} md={4}>
            <Form.Item label={t("cashflow.scenario.renewal_term_months", language)} name="renewal_term_months" rules={[{ required: true, message: t("cashflow.scenario.req_term", language) }]}>
              <InputNumber style={{ width: "100%" }} min={1} />
            </Form.Item>
          </Col>
          <Col xs={12} md={4}>
            <Form.Item label={t("cashflow.scenario.renewal_uplift", language)} name="renewal_uplift_percent">
              <InputNumber style={{ width: "100%" }} precision={1} />
            </Form.Item>
          </Col>
          <Col xs={12} md={4}>
            <Form.Item label={t("cashflow.scenario.closure_rate", language)} name="closure_rate" extra={t("cashflow.scenario.rate_helper", language)}>
              <InputNumber style={{ width: "100%" }} min={0} max={1} step={0.05} />
            </Form.Item>
          </Col>
          <Col xs={12} md={4}>
            <Form.Item label={t("cashflow.scenario.closure_cost", language)} name="closure_cost_months">
              <InputNumber style={{ width: "100%" }} min={0} precision={1} />
            </Form.Item>
          </Col>
        </Row>
      </Form>

      {warning && <Alert type="warning" showIcon message={warning} style={{ marginBottom: 16 }} />}

      {results.length > 0 && (
        <>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message={t("cashflow.scenario.expiring_msg", language).replace("{count}", String(results[0].expiring_leases))}
            description={results[results.length - 1].caveat}
          />

          <Table
            dataSource={results}
            rowKey={(row) => row.scenario.name}
            pagination={false}
            size="small"
            style={{ marginBottom: 16 }}
            scroll={tableScrollX((results || []).length, 700)}
            columns={[
              {
                title: t("common.scenario", language),
                dataIndex: ["scenario", "name"],
                render: (name: string, row: ScenarioResult) => (
                  <Space>
                    <strong>{name}</strong>
                    {row.renewal_total === 0 && row.closure_total === 0 && <StatusTag>{t("scenario.baseline", language) || "基准"}</StatusTag>}
                  </Space>
                ),
              },
              {
                title: t("cashflow.scenario.committed_total", language),
                dataIndex: "committed_total",
                align: "right" as const,
                render: (value: number) => fmtMoney(value, currency),
              },
              {
                title: t("cashflow.scenario.renewal_total", language),
                dataIndex: "renewal_total",
                align: "right" as const,
                render: (value: number) => (value ? fmtMoney(value, currency) : "—"),
              },
              {
                title: t("cashflow.scenario.closure_total", language),
                dataIndex: "closure_total",
                align: "right" as const,
                render: (value: number) => (value ? fmtMoney(value, currency) : "—"),
              },
              {
                title: t("common.total", language),
                dataIndex: "total",
                align: "right" as const,
                render: (value: number) => <strong>{fmtMoney(value, currency)}</strong>,
              },
            ]}
          />

          <div style={{ width: "100%", height: 300 }}>
            <ResponsiveContainer>
              <BarChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="band" />
                <YAxis tickFormatter={(value) => `${Math.round(Number(value) / 10000)}万`} />
                <Tooltip formatter={(value) => fmtMoney(Number(value), currency)} />
                <Legend />
                {results.map((result, index) => (
                  <Bar isAnimationActive={false}
                    key={result.scenario.name}
                    dataKey={result.scenario.name}
                    fill={index === 0 ? "var(--fg-muted)" : "var(--chart-blue)"}
                  />
                ))}
              </BarChart>
            </ResponsiveContainer>
          </div>
        </>
      )}
    </Card>
  );
}
