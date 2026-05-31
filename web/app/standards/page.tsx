"use client";

import { useEffect, useMemo, useState } from "react";
import { Alert, Button, Card, Col, Form, InputNumber, Row, Select, Space, Statistic, Table, Tag, Typography, message } from "antd";
import { SafetyOutlined } from "@ant-design/icons";
import { Bar, BarChart, CartesianGrid, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { contractApi, reportApi } from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { motion } from "framer-motion";

const { Text } = Typography;

interface ContractOption {
  id: string;
  contract_number: string;
  contract_name: string;
}

interface StandardRow {
  standard: string;
  standard_name: string;
  classification: string;
  measurement_basis: string;
  initial_liability: number;
  initial_rou_asset: number;
  first_period_expense: number;
  total_recognized_cost: number;
  balance_sheet_treatment: string;
  pnl_pattern: string;
  key_differences: string[];
}

const fmt = (value: number) => value.toLocaleString(undefined, { maximumFractionDigits: 2 });

export default function StandardsPage() {
  const { token } = useAuth();
  const [form] = Form.useForm();
  const [contracts, setContracts] = useState<ContractOption[]>([]);
  const [rows, setRows] = useState<StandardRow[]>([]);
  const [meta, setMeta] = useState<any>(null);
  const [loading, setLoading] = useState(false);

  const delta = useMemo(() => {
    const ifrs = rows.find((row) => row.standard === "ifrs16");
    const operating = rows.find((row) => row.standard === "asc842_operating");
    if (!ifrs || !operating) return 0;
    return operating.first_period_expense - ifrs.first_period_expense;
  }, [rows]);

  useEffect(() => {
    if (!token) return;
    contractApi
      .list(token, { status: "approved", sort_by: "created_at", sort_order: "desc" })
      .then((res) => setContracts(res.data || []))
      .catch((error: any) => message.error(error.message || "合同列表加载失败"));
  }, [token]);

  const runComparison = async (values: any) => {
    if (!token) return;
    setLoading(true);
    try {
      const res = await reportApi.standardComparison(
        {
          contract_id: values.contract_id,
          discount_rate: values.discount_rate_percent ? values.discount_rate_percent / 100 : undefined,
        },
        token
      );
      setRows(res.data || []);
      setMeta(res);
      message.success("多准则对比已生成");
    } catch (error: any) {
      message.error(error.message || "多准则对比失败");
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
              <h1 style={{ margin: 0, fontSize: 24 }}>多准则对比</h1>
              <div style={{ color: "#6B7280", marginTop: 6 }}>
                对同一合同展示 IFRS 16、ASC 842 与中国租赁准则下的资产负债表和损益表差异。
              </div>
            </div>

            <Card>
              <Form form={form} layout="inline" onFinish={runComparison}>
                <Form.Item name="contract_id" rules={[{ required: true, message: "请选择合同" }]} style={{ minWidth: 360 }}>
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
                <Form.Item name="discount_rate_percent" label="折现率">
                  <InputNumber min={0} max={50} precision={2} addonAfter="%" placeholder="默认合同/系统利率" />
                </Form.Item>
                <Form.Item>
                  <Button type="primary" htmlType="submit" loading={loading} icon={<SafetyOutlined />}>
                    生成对比
                  </Button>
                </Form.Item>
              </Form>
            </Card>

            <Alert
              type="warning"
              showIcon
              message="这是管理对比视图，不替代完整法定准则子账"
              description="IFRS 16 仍是受控计量基线。ASC 842 operating / finance 与本地准则行用于展示政策差异、售前沟通和管理层影响分析。"
            />

            {meta && (
              <Alert
                type="info"
                showIcon
                message={`${meta.contract_number} / ${meta.contract_name}`}
                description={`范围判定 ${meta.lease_scope}，折现率 ${(meta.discount_rate * 100).toFixed(2)}%，币种 ${meta.currency}`}
              />
            )}

            <Row gutter={16}>
              <Col xs={24} md={8}>
                <Card>
                  <Statistic title="准则视图数" value={rows.length} />
                </Card>
              </Col>
              <Col xs={24} md={8}>
                <Card>
                  <Statistic title="IFRS 初始负债" value={rows.find((row) => row.standard === "ifrs16")?.initial_liability || 0} precision={2} prefix="¥" />
                </Card>
              </Col>
              <Col xs={24} md={8}>
                <Card>
                  <Statistic title="ASC Operating 首期损益差异" value={delta} precision={2} prefix="¥" />
                </Card>
              </Col>
            </Row>

            {rows.length > 0 && (
              <Card title="初始确认与首期损益">
                <div style={{ width: "100%", height: 300 }}>
                  <ResponsiveContainer>
                    <BarChart data={rows}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="standard_name" interval={0} height={70} tick={{ fontSize: 11 }} />
                      <YAxis tickFormatter={(value) => `${Math.round(Number(value) / 1000)}k`} />
                      <Tooltip formatter={(value) => `¥${fmt(Number(value || 0))}`} />
                      <Legend />
                      <Bar dataKey="initial_liability" fill="#1677FF" name="初始负债" />
                      <Bar dataKey="first_period_expense" fill="#52C41A" name="首期费用" />
                    </BarChart>
                  </ResponsiveContainer>
                </div>
              </Card>
            )}

            <Card title="准则差异明细">
              <Table
                loading={loading}
                dataSource={rows}
                rowKey="standard"
                size="small"
                pagination={false}
                scroll={{ x: 1320 }}
                columns={[
                  { title: "准则", dataIndex: "standard_name", width: 210, fixed: "left" },
                  { title: "分类", dataIndex: "classification", width: 160, render: (v: string) => <Tag color="processing">{v}</Tag> },
                  { title: "计量路径", dataIndex: "measurement_basis", width: 150 },
                  { title: "初始负债", dataIndex: "initial_liability", width: 120, align: "right" as const, render: (v: number) => fmt(v) },
                  { title: "ROU 资产", dataIndex: "initial_rou_asset", width: 120, align: "right" as const, render: (v: number) => fmt(v) },
                  { title: "首期费用", dataIndex: "first_period_expense", width: 120, align: "right" as const, render: (v: number) => fmt(v) },
                  { title: "总确认成本", dataIndex: "total_recognized_cost", width: 130, align: "right" as const, render: (v: number) => fmt(v) },
                  { title: "资产负债表", dataIndex: "balance_sheet_treatment", width: 280 },
                  { title: "损益模式", dataIndex: "pnl_pattern", width: 280 },
                ]}
                expandable={{
                  expandedRowRender: (record) => (
                    <Space direction="vertical" size={4}>
                      {record.key_differences.map((item) => (
                        <Text key={item}>- {item}</Text>
                      ))}
                    </Space>
                  ),
                }}
              />
            </Card>
          </Space>
        </motion.div>
      </AppLayout>
    </ProtectedRoute>
  );
}
