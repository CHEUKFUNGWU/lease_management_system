"use client";

import { useState } from "react";
import {
  Alert,
  Button,
  Card,
  Col,
  DatePicker,
  Form,
  Input,
  InputNumber,
  Row,
  Statistic,
  Table,
  Typography,
  message,
} from "antd";
import { FileSearchOutlined } from "@ant-design/icons";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ReferenceLine,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import dayjs from "dayjs";
import { motion } from "framer-motion";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { dealApi } from "../lib/api";
import { fmtMoney } from "../lib/format";
import { useAuth } from "../context/AuthContext";

const { Title, Text } = Typography;

interface YearlyImpact {
  year: number;
  cash_rent: number;
  straight_line_rent: number;
  interest: number;
  depreciation: number;
  ifrs16_expense: number;
  expense_vs_straight_line: number;
  closing_liability: number;
  closing_rou: number;
}

interface BridgeRow {
  year: number;
  rent_above_ebitda: number;
  ebitda_uplift: number;
  depreciation_below_ebitda: number;
  interest_below_ebit: number;
  net_profit_impact: number;
}

interface ExitPoint {
  year: number;
  remaining_commitment: number;
  liability_released: number;
  rou_written_off: number;
  penalty: number;
  pnl_impact: number;
  total_cash_to_exit: number;
}

interface Briefing {
  name: string;
  currency: string;
  term_months: number;
  balance_sheet: {
    initial_liability: number;
    initial_rou: number;
    undiscounted_commitment: number;
    discounting_effect: number;
  };
  yearly: YearlyImpact[];
  ebitda_bridge: BridgeRow[];
  exit_curve: ExitPoint[];
  front_loaded_years: number;
  headline: string;
}

export default function PreDealPage() {
  const { token } = useAuth();
  const [form] = Form.useForm();
  const [briefing, setBriefing] = useState<Briefing | null>(null);
  const [loading, setLoading] = useState(false);

  const handleBuild = async (values: any) => {
    if (!token) return;
    setLoading(true);
    try {
      setBriefing(
        await dealApi.briefing(
          {
            name: values.name,
            commencement_date: values.commencement_date.format("YYYY-MM-DD"),
            term_months: values.term_months,
            monthly_rent: values.monthly_rent,
            rent_free_months: values.rent_free_months || 0,
            annual_escalation_percent: values.annual_escalation_percent || 0,
            discount_rate: values.discount_rate,
            currency: values.currency || "CNY",
            initial_direct_cost: values.initial_direct_cost || 0,
            early_exit_penalty_months: values.early_exit_penalty_months || 0,
          },
          token
        )
      );
    } catch (error: any) {
      message.error(error?.message || "测算失败");
      setBriefing(null);
    } finally {
      setLoading(false);
    }
  };

  const currency = briefing?.currency;

  return (
    <ProtectedRoute>
      <AppLayout>
        <motion.div initial={{ opacity: 0, y: 4 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.25 }}>
          <div style={{ marginBottom: 24 }}>
            <Title level={2} style={{ marginBottom: 4, letterSpacing: "-0.04em" }}>
              签约前决策
            </Title>
            <Text type="secondary">
              签长期租约前，财务要回答的不是「IFRS 16 怎么记账」，而是「这个决定如何影响未来几年的经营结果」。
            </Text>
          </div>

          <Form
            form={form}
            layout="vertical"
            onFinish={handleBuild}
            initialValues={{
              name: "厂房 5 年租约",
              commencement_date: dayjs().add(1, "month").startOf("month"),
              term_months: 60,
              monthly_rent: 100000,
              discount_rate: 0.05,
              currency: "CNY",
              early_exit_penalty_months: 3,
            }}
          >
            <Card title="条款草案" style={{ borderRadius: 10, marginBottom: 16 }}>
              <Row gutter={16}>
                <Col xs={24} md={6}>
                  <Form.Item label="方案名称" name="name" rules={[{ required: true, message: "请填写名称" }]}>
                    <Input />
                  </Form.Item>
                </Col>
                <Col xs={12} md={4}>
                  <Form.Item label="起租日" name="commencement_date" rules={[{ required: true, message: "请选择起租日" }]}>
                    <DatePicker style={{ width: "100%" }} />
                  </Form.Item>
                </Col>
                <Col xs={12} md={3}>
                  <Form.Item label="租期（月）" name="term_months" rules={[{ required: true, message: "请填写租期" }]}>
                    <InputNumber style={{ width: "100%" }} min={1} />
                  </Form.Item>
                </Col>
                <Col xs={12} md={4}>
                  <Form.Item label="月租金" name="monthly_rent" rules={[{ required: true, message: "请填写月租金" }]}>
                    <InputNumber style={{ width: "100%" }} min={0} precision={2} />
                  </Form.Item>
                </Col>
                <Col xs={12} md={3}>
                  <Form.Item
                    label="折现率"
                    name="discount_rate"
                    rules={[{ required: true, message: "请填写折现率" }]}
                    extra="入表金额取决于它"
                  >
                    <InputNumber style={{ width: "100%" }} min={0.0001} max={1} step={0.005} precision={4} />
                  </Form.Item>
                </Col>
                <Col xs={12} md={4}>
                  <Form.Item label="币种" name="currency">
                    <Input />
                  </Form.Item>
                </Col>
              </Row>
              <Row gutter={16}>
                <Col xs={12} md={4}>
                  <Form.Item label="免租期（月）" name="rent_free_months">
                    <InputNumber style={{ width: "100%" }} min={0} />
                  </Form.Item>
                </Col>
                <Col xs={12} md={4}>
                  <Form.Item label="年递增（%）" name="annual_escalation_percent">
                    <InputNumber style={{ width: "100%" }} precision={2} />
                  </Form.Item>
                </Col>
                <Col xs={12} md={5}>
                  <Form.Item label="初始直接费用" name="initial_direct_cost" extra="计入资产，不计入负债">
                    <InputNumber style={{ width: "100%" }} min={0} precision={2} />
                  </Form.Item>
                </Col>
                <Col xs={12} md={5}>
                  <Form.Item label="提前退出罚金（月租金）" name="early_exit_penalty_months" extra="按退出时点在租的租金计">
                    <InputNumber style={{ width: "100%" }} min={0} precision={1} />
                  </Form.Item>
                </Col>
                <Col xs={24} md={6}>
                  <Form.Item label=" ">
                    <Button type="primary" icon={<FileSearchOutlined />} htmlType="submit" loading={loading} block>
                      生成决策简报
                    </Button>
                  </Form.Item>
                </Col>
              </Row>
            </Card>
          </Form>

          {briefing && (
            <>
              <Alert
                type="info"
                showIcon
                style={{ marginBottom: 16, borderRadius: 10 }}
                message="决策简报"
                description={briefing.headline}
              />

              <Row gutter={[12, 12]} style={{ marginBottom: 16 }}>
                <Col xs={12} md={6}>
                  <Card style={{ borderRadius: 10 }}>
                    <Statistic
                      title="入表负债"
                      value={briefing.balance_sheet.initial_liability}
                      formatter={() => fmtMoney(briefing.balance_sheet.initial_liability, currency)}
                    />
                  </Card>
                </Col>
                <Col xs={12} md={6}>
                  <Card style={{ borderRadius: 10 }}>
                    <Statistic
                      title="使用权资产"
                      value={briefing.balance_sheet.initial_rou}
                      formatter={() => fmtMoney(briefing.balance_sheet.initial_rou, currency)}
                    />
                  </Card>
                </Col>
                <Col xs={12} md={6}>
                  <Card style={{ borderRadius: 10 }}>
                    <Statistic
                      title="全期承诺（未折现）"
                      value={briefing.balance_sheet.undiscounted_commitment}
                      formatter={() => fmtMoney(briefing.balance_sheet.undiscounted_commitment, currency)}
                    />
                  </Card>
                </Col>
                <Col xs={12} md={6}>
                  <Card style={{ borderRadius: 10 }}>
                    <Statistic
                      title="折现影响"
                      value={briefing.balance_sheet.discounting_effect}
                      formatter={() => fmtMoney(briefing.balance_sheet.discounting_effect, currency)}
                    />
                  </Card>
                </Col>
              </Row>

              <Card title="IFRS 16 费用曲线 vs 直线租金" style={{ borderRadius: 10, marginBottom: 16 }}>
                <div style={{ color: "var(--fg-muted)", fontSize: 13, marginBottom: 12 }}>
                  利息在负债最大时最高，因此会计费用前高后低。两条线交叉之前的年份，实际入账费用高于按租金做的预算——
                  本方案前 {briefing.front_loaded_years} 年如此，之后反向，全期相抵。
                </div>
                <div style={{ width: "100%", height: 300 }}>
                  <ResponsiveContainer>
                    <LineChart data={briefing.yearly}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="year" tickFormatter={(value) => `第${value}年`} />
                      <YAxis tickFormatter={(value) => `${Math.round(Number(value) / 10000)}万`} />
                      <Tooltip formatter={(value) => fmtMoney(Number(value), currency)} />
                      <Legend />
                      <Line type="monotone" dataKey="ifrs16_expense" name="IFRS 16 费用" stroke="#CF1322" strokeWidth={2} dot={false} />
                      <Line type="monotone" dataKey="straight_line_rent" name="直线租金" stroke="#1677FF" strokeWidth={2} strokeDasharray="5 5" dot={false} />
                      <Line type="monotone" dataKey="cash_rent" name="现金租金" stroke="#8C8C8C" strokeWidth={1} dot={false} />
                    </LineChart>
                  </ResponsiveContainer>
                </div>
              </Card>

              <Card title="EBITDA 三层影响" style={{ borderRadius: 10, marginBottom: 16 }}>
                <div style={{ color: "var(--fg-muted)", fontSize: 13, marginBottom: 12 }}>
                  租金从 EBITDA 线上移到线下，EBITDA 被动抬升——业务没有任何改善。抬升额等于原本计入经营费用的租金，
                  它去了折旧（EBITDA 与 EBIT 之间）和利息（EBIT 与净利润之间）。
                </div>
                <div style={{ width: "100%", height: 280 }}>
                  <ResponsiveContainer>
                    <BarChart data={briefing.ebitda_bridge}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="year" tickFormatter={(value) => `第${value}年`} />
                      <YAxis tickFormatter={(value) => `${Math.round(Number(value) / 10000)}万`} />
                      <Tooltip formatter={(value) => fmtMoney(Number(value), currency)} />
                      <Legend />
                      <ReferenceLine y={0} stroke="#000" />
                      <Bar dataKey="ebitda_uplift" name="EBITDA 抬升" fill="#389E0D" />
                      <Bar dataKey="depreciation_below_ebitda" name="折旧（线下）" fill="#1677FF" />
                      <Bar dataKey="interest_below_ebit" name="利息（EBIT 之下）" fill="#D48806" />
                      <Bar dataKey="net_profit_impact" name="净利润影响" fill="#CF1322" />
                    </BarChart>
                  </ResponsiveContainer>
                </div>
              </Card>

              <Card title="退出成本曲线" style={{ borderRadius: 10 }}>
                <div style={{ color: "var(--fg-muted)", fontSize: 13, marginBottom: 12 }}>
                  策略变化时才会问、却没人备着答案的问题：第 N 年退出要花多少。剩余租金因退出而免付，故不计入「退出现金支出」；
                  罚金按退出时点在租的租金计算。
                </div>
                <Table
                  dataSource={briefing.exit_curve}
                  rowKey="year"
                  pagination={false}
                  size="small"
                  scroll={{ x: 800 }}
                  columns={[
                    { title: "退出时点", dataIndex: "year", render: (value: number) => `第 ${value} 年末` },
                    {
                      title: "剩余承诺（免付）",
                      dataIndex: "remaining_commitment",
                      align: "right" as const,
                      render: (value: number) => fmtMoney(value, currency),
                    },
                    {
                      title: "解除负债",
                      dataIndex: "liability_released",
                      align: "right" as const,
                      render: (value: number) => fmtMoney(value, currency),
                    },
                    {
                      title: "核销使用权资产",
                      dataIndex: "rou_written_off",
                      align: "right" as const,
                      render: (value: number) => fmtMoney(value, currency),
                    },
                    {
                      title: "罚金",
                      dataIndex: "penalty",
                      align: "right" as const,
                      render: (value: number) => fmtMoney(value, currency),
                    },
                    {
                      title: "损益影响",
                      dataIndex: "pnl_impact",
                      align: "right" as const,
                      render: (value: number) => (
                        <strong style={{ color: value > 0 ? "#CF1322" : "#389E0D" }}>
                          {fmtMoney(value, currency)}
                        </strong>
                      ),
                    },
                    {
                      title: "退出现金支出",
                      dataIndex: "total_cash_to_exit",
                      align: "right" as const,
                      render: (value: number) => fmtMoney(value, currency),
                    },
                  ]}
                />
              </Card>
            </>
          )}
        </motion.div>
      </AppLayout>
    </ProtectedRoute>
  );
}
