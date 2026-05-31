"use client";

import { useEffect, useMemo, useState } from "react";
import { Alert, Button, Card, Col, Form, InputNumber, Row, Select, Space, Statistic, Table, Tag, message } from "antd";
import { CalculatorOutlined } from "@ant-design/icons";
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { contractApi, reportApi } from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { motion } from "framer-motion";

interface ContractOption {
  id: string;
  contract_number: string;
  contract_name: string;
  lease_scope?: string;
}

interface ScenarioRow {
  scenario_name: string;
  discount_rate: number;
  rate_delta: number;
  initial_liability: number;
  initial_rou_asset: number;
  liability_delta: number;
  liability_delta_percent: number;
}

const fmt = (value: number) => value.toLocaleString(undefined, { maximumFractionDigits: 2 });

export default function SensitivityPage() {
  const { token } = useAuth();
  const [form] = Form.useForm();
  const [contracts, setContracts] = useState<ContractOption[]>([]);
  const [rows, setRows] = useState<ScenarioRow[]>([]);
  const [meta, setMeta] = useState<any>(null);
  const [loading, setLoading] = useState(false);

  const summary = useMemo(() => {
    if (!rows.length) return { base: 0, maxUp: 0, maxDown: 0 };
    const base = rows.find((row) => Math.abs(row.rate_delta) < 0.0000001)?.initial_liability || rows[0].initial_liability;
    return rows.reduce(
      (acc, row) => {
        acc.maxUp = Math.max(acc.maxUp, row.liability_delta);
        acc.maxDown = Math.min(acc.maxDown, row.liability_delta);
        return acc;
      },
      { base, maxUp: 0, maxDown: 0 }
    );
  }, [rows]);

  useEffect(() => {
    if (!token) return;
    contractApi
      .list(token, { status: "approved", sort_by: "created_at", sort_order: "desc" })
      .then((res) => setContracts(res.data || []))
      .catch((error: any) => message.error(error.message || "合同列表加载失败"));
  }, [token]);

  const runAnalysis = async (values: any) => {
    if (!token) return;
    setLoading(true);
    try {
      const shock = (values.shock_percent || 1) / 100;
      const res = await reportApi.sensitivity(
        {
          contract_id: values.contract_id,
          base_rate: values.base_rate_percent ? values.base_rate_percent / 100 : undefined,
          shocks: [-shock, -shock / 2, 0, shock / 2, shock].join(","),
        },
        token
      );
      setRows(res.data || []);
      setMeta(res);
      message.success("敏感性分析已生成");
    } catch (error: any) {
      message.error(error.message || "敏感性分析失败");
    } finally {
      setLoading(false);
    }
  };

  return (
    <ProtectedRoute>
      <AppLayout>
        <motion.div initial={{ opacity: 0, y: 12 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.2 }}>
          <Space direction="vertical" size={16} style={{ width: "100%" }}>
            <div>
              <h1 style={{ margin: 0, fontSize: 24 }}>敏感性分析</h1>
              <div style={{ color: "#6B7280", marginTop: 6 }}>
                复用 IFRS 16 计量引擎，量化折现率假设变动对初始租赁负债和 ROU 资产的影响。
              </div>
            </div>

            <Card>
              <Form
                form={form}
                layout="inline"
                onFinish={runAnalysis}
                initialValues={{ shock_percent: 1 }}
              >
                <Form.Item name="contract_id" rules={[{ required: true, message: "请选择合同" }]} style={{ minWidth: 320 }}>
                  <Select
                    showSearch
                    placeholder="选择已审批合同"
                    optionFilterProp="label"
                    options={contracts.map((contract) => ({
                      value: contract.id,
                      label: `${contract.contract_number} - ${contract.contract_name}`,
                    }))}
                  />
                </Form.Item>
                <Form.Item name="base_rate_percent" label="基准折现率">
                  <InputNumber min={0} max={50} precision={2} addonAfter="%" placeholder="默认合同/系统利率" />
                </Form.Item>
                <Form.Item name="shock_percent" label="冲击幅度">
                  <InputNumber min={0.1} max={10} precision={2} addonAfter="%" />
                </Form.Item>
                <Form.Item>
                  <Button type="primary" htmlType="submit" loading={loading} icon={<CalculatorOutlined />}>
                    生成分析
                  </Button>
                </Form.Item>
              </Form>
            </Card>

            {meta && (
              <Alert
                type="info"
                showIcon
                message={`${meta.contract_number} / ${meta.contract_name}`}
                description={`基准折现率 ${(meta.base_rate * 100).toFixed(2)}%，范围判定 ${meta.lease_scope}，币种 ${meta.currency}`}
              />
            )}

            <Row gutter={16}>
              <Col xs={24} md={8}>
                <Card>
                  <Statistic title="基准初始负债" value={summary.base} precision={2} prefix="¥" />
                </Card>
              </Col>
              <Col xs={24} md={8}>
                <Card>
                  <Statistic title="最大上行影响" value={summary.maxUp} precision={2} prefix="¥" valueStyle={{ color: summary.maxUp > 0 ? "#CF1322" : undefined }} />
                </Card>
              </Col>
              <Col xs={24} md={8}>
                <Card>
                  <Statistic title="最大下行影响" value={summary.maxDown} precision={2} prefix="¥" valueStyle={{ color: summary.maxDown < 0 ? "#3F8600" : undefined }} />
                </Card>
              </Col>
            </Row>

            {rows.length > 0 && (
              <Card title="负债影响图">
                <div style={{ width: "100%", height: 280 }}>
                  <ResponsiveContainer>
                    <BarChart data={rows}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="scenario_name" />
                      <YAxis tickFormatter={(value) => `${Math.round(Number(value) / 1000)}k`} />
                      <Tooltip formatter={(value) => `¥${fmt(Number(value || 0))}`} />
                      <Bar dataKey="liability_delta" fill="#1677FF" name="负债变动" />
                    </BarChart>
                  </ResponsiveContainer>
                </div>
              </Card>
            )}

            <Card title="场景明细">
              <Table
                loading={loading}
                dataSource={rows}
                rowKey="scenario_name"
                pagination={false}
                size="small"
                columns={[
                  { title: "场景", dataIndex: "scenario_name", width: 100, render: (v: string) => <Tag color={v === "+0.00%" ? "success" : "processing"}>{v}</Tag> },
                  { title: "折现率", dataIndex: "discount_rate", width: 100, render: (v: number) => `${(v * 100).toFixed(2)}%` },
                  { title: "初始负债", dataIndex: "initial_liability", align: "right" as const, render: (v: number) => fmt(v) },
                  { title: "ROU 资产", dataIndex: "initial_rou_asset", align: "right" as const, render: (v: number) => fmt(v) },
                  { title: "负债变动", dataIndex: "liability_delta", align: "right" as const, render: (v: number) => fmt(v) },
                  { title: "变动比例", dataIndex: "liability_delta_percent", align: "right" as const, render: (v: number) => `${(v * 100).toFixed(2)}%` },
                ]}
              />
            </Card>
          </Space>
        </motion.div>
      </AppLayout>
    </ProtectedRoute>
  );
}
