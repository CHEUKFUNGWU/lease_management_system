"use client";

import { StatusTag } from "../components/StatusTag";

import { useState } from "react";
import { Alert, Button, Card, Col, Form, InputNumber, Row, Space, Table, Tag, message } from "antd";
import { ExperimentOutlined } from "@ant-design/icons";
import { Bar, BarChart, CartesianGrid, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { reportApi } from "../lib/api";
import { fmtMoney } from "../lib/format";

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
      setResults(response.results || []);
      setWarning(response.currency_warning || null);
    } catch (error: any) {
      message.error(error?.message || "情景测算失败");
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
      title="到期阶梯与组合情景"
      style={{ borderRadius: 10, marginTop: 16 }}
      extra={
        <Button type="primary" icon={<ExperimentOutlined />} loading={loading} onClick={() => form.submit()}>
          测算
        </Button>
      }
    >
      <div style={{ color: "var(--fg-muted)", fontSize: 13, marginBottom: 16 }}>
        按到期日排的租金流水是一张日程表，它假设每份租约都在到期日干净地停掉——而这是没人会去规划的结果。
        下面按比例假设「续掉多少、关掉多少」，并始终与不做假设的基准并排显示。
      </div>

      <Form
        form={form}
        layout="vertical"
        onFinish={run}
      >
        <Row gutter={12}>
          <Col xs={12} md={4}>
            <Form.Item label="测算期（月）" name="horizon_months" rules={[{ required: true, message: "请填写测算期" }]}>
              <InputNumber style={{ width: "100%" }} min={12} max={120} />
            </Form.Item>
          </Col>
          <Col xs={12} md={4}>
            <Form.Item label="续租比例" name="renewal_rate" extra="0–1">
              <InputNumber style={{ width: "100%" }} min={0} max={1} step={0.05} />
            </Form.Item>
          </Col>
          <Col xs={12} md={4}>
            <Form.Item label="续租租期（月）" name="renewal_term_months" rules={[{ required: true, message: "请填写续租租期" }]}>
              <InputNumber style={{ width: "100%" }} min={1} />
            </Form.Item>
          </Col>
          <Col xs={12} md={4}>
            <Form.Item label="续租涨幅（%）" name="renewal_uplift_percent">
              <InputNumber style={{ width: "100%" }} precision={1} />
            </Form.Item>
          </Col>
          <Col xs={12} md={4}>
            <Form.Item label="关店比例" name="closure_rate" extra="与续租比例合计不超过 1">
              <InputNumber style={{ width: "100%" }} min={0} max={1} step={0.05} />
            </Form.Item>
          </Col>
          <Col xs={12} md={4}>
            <Form.Item label="退出成本（月租金）" name="closure_cost_months">
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
            message={`测算期内有 ${results[0].expiring_leases} 份租约到期`}
            description={results[results.length - 1].caveat}
          />

          <Table
            dataSource={results}
            rowKey={(row) => row.scenario.name}
            pagination={false}
            size="small"
            style={{ marginBottom: 16 }}
            scroll={{ x: 700 }}
            columns={[
              {
                title: "情景",
                dataIndex: ["scenario", "name"],
                render: (name: string, row: ScenarioResult) => (
                  <Space>
                    <strong>{name}</strong>
                    {row.renewal_total === 0 && row.closure_total === 0 && <StatusTag>基准</StatusTag>}
                  </Space>
                ),
              },
              {
                title: "已签约承诺",
                dataIndex: "committed_total",
                align: "right" as const,
                render: (value: number) => fmtMoney(value, currency),
              },
              {
                title: "假设续租",
                dataIndex: "renewal_total",
                align: "right" as const,
                render: (value: number) => (value ? fmtMoney(value, currency) : "—"),
              },
              {
                title: "假设关店成本",
                dataIndex: "closure_total",
                align: "right" as const,
                render: (value: number) => (value ? fmtMoney(value, currency) : "—"),
              },
              {
                title: "合计",
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
