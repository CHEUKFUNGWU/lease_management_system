"use client";

import { useState } from "react";
import {
  Alert,
  Button,
  Card,
  Col,
  Form,
  Input,
  InputNumber,
  Row,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from "antd";
import { DeleteOutlined, PlusOutlined, SwapOutlined } from "@ant-design/icons";
import { Line, LineChart, CartesianGrid, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { motion } from "framer-motion";
import AppLayout from "../components/AppLayout";
import ProtectedRoute from "../components/ProtectedRoute";
import { dealApi } from "../lib/api";
import { fmtMoney } from "../lib/format";
import { useAuth } from "../context/AuthContext";

const { Title, Text } = Typography;

interface OfferResult {
  name: string;
  total_rent: number;
  total_cost: number;
  effective_monthly_rent: number;
  effective_rent_per_sqm: number;
  present_value: number;
  first_year_rent: number;
  schedule: { month: number; rent: number; other: number; total: number }[];
}

interface Comparison {
  discount_rate: number;
  currency: string;
  offers: OfferResult[];
  best_by_effective_rent: string;
  best_by_present_value: string;
  measures_disagree: boolean;
  conclusion: string;
}

const LINE_COLORS = ["#1677FF", "#CF1322", "#389E0D", "#D48806", "#722ED1"];

// A comparison starts from the two shapes a negotiation usually offers: an
// incentive package with escalation, and a flat deal.
const INITIAL_OFFERS = [
  {
    name: "方案 A",
    term_months: 60,
    base_monthly_rent: 100000,
    rent_free_months: 6,
    annual_escalation_percent: 5,
    other_monthly_cost: 0,
    upfront_cost: 0,
    landlord_contribution: 0,
    area_sqm: 500,
  },
  {
    name: "方案 B",
    term_months: 60,
    base_monthly_rent: 100000,
    rent_free_months: 0,
    annual_escalation_percent: 0,
    other_monthly_cost: 0,
    upfront_cost: 0,
    landlord_contribution: 0,
    area_sqm: 500,
  },
];

export default function DealComparePage() {
  const { token } = useAuth();
  const [form] = Form.useForm();
  const [result, setResult] = useState<Comparison | null>(null);
  const [loading, setLoading] = useState(false);

  const handleCompare = async (values: any) => {
    if (!token) return;
    setLoading(true);
    try {
      setResult(
        await dealApi.compare(
          {
            discount_rate: values.discount_rate,
            currency: values.currency || "CNY",
            offers: (values.offers || []).map((offer: any) => ({
              ...offer,
              // The form keeps blanks as null; the engine reads them as zero.
              rent_free_months: offer.rent_free_months || 0,
              annual_escalation_percent: offer.annual_escalation_percent || 0,
              other_monthly_cost: offer.other_monthly_cost || 0,
              upfront_cost: offer.upfront_cost || 0,
              landlord_contribution: offer.landlord_contribution || 0,
              area_sqm: offer.area_sqm || 0,
            })),
          },
          token
        )
      );
    } catch (error: any) {
      message.error(error?.message || "比价失败");
      setResult(null);
    } finally {
      setLoading(false);
    }
  };

  const currency = result?.currency;

  // The chart plots cumulative cash so the shape of each deal is visible: a
  // rent-free period is a flat start, an escalation is a steepening curve.
  const chartData = result
    ? Array.from({ length: Math.max(...result.offers.map((o) => o.schedule.length)) }, (_, index) => {
        const point: Record<string, number> = { month: index + 1 };
        result.offers.forEach((offer) => {
          const running = offer.schedule
            .slice(0, index + 1)
            .reduce((sum, row) => sum + row.total, 0);
          point[offer.name] = Math.round(running);
        });
        return point;
      })
    : [];

  return (
    <ProtectedRoute>
      <AppLayout>
        <motion.div initial={{ opacity: 0, y: 4 }} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.25 }}>
          <div style={{ marginBottom: 24 }}>
            <Title level={2} style={{ marginBottom: 4, letterSpacing: "-0.04em" }}>
              条款比价
            </Title>
            <Text type="secondary">
              把免租期、递增、装修补贴这些谈不拢的条款，折算成同一把尺子：直线化有效租金与现金流现值。
            </Text>
          </div>

          <Form
            form={form}
            layout="vertical"
            onFinish={handleCompare}
            initialValues={{ discount_rate: 0.05, currency: "CNY", offers: INITIAL_OFFERS }}
          >
            <Card style={{ borderRadius: 10, marginBottom: 16 }}>
              <Row gutter={16}>
                <Col xs={24} md={8}>
                  <Form.Item
                    label="折现率（年化，小数）"
                    name="discount_rate"
                    rules={[{ required: true, message: "请填写折现率" }]}
                    extra="排序结果取决于它，系统不会替你假设一个"
                  >
                    <InputNumber style={{ width: "100%" }} min={0.0001} max={1} step={0.005} precision={4} />
                  </Form.Item>
                </Col>
                <Col xs={24} md={8}>
                  <Form.Item label="币种" name="currency">
                    <Input placeholder="CNY" />
                  </Form.Item>
                </Col>
              </Row>
            </Card>

            <Form.List name="offers">
              {(fields, { add, remove }) => (
                <>
                  <Row gutter={[16, 16]}>
                    {fields.map((field, index) => (
                      <Col xs={24} lg={12} key={field.key}>
                        <Card
                          style={{ borderRadius: 10 }}
                          title={
                            <Form.Item
                              {...field}
                              key={`${field.key}-name`}
                              name={[field.name, "name"]}
                              noStyle
                              rules={[{ required: true, message: "请填写方案名称" }]}
                            >
                              <Input variant="borderless" style={{ fontWeight: 600, padding: 0 }} />
                            </Form.Item>
                          }
                          extra={
                            fields.length > 2 ? (
                              <Button
                                size="small"
                                danger
                                icon={<DeleteOutlined />}
                                onClick={() => remove(field.name)}
                              />
                            ) : null
                          }
                        >
                          <Row gutter={12}>
                            <Col span={12}>
                              <Form.Item
                                {...field}
                                key={`${field.key}-term`}
                                label="租期（月）"
                                name={[field.name, "term_months"]}
                                rules={[{ required: true, message: "请填写租期" }]}
                              >
                                <InputNumber style={{ width: "100%" }} min={1} />
                              </Form.Item>
                            </Col>
                            <Col span={12}>
                              <Form.Item
                                {...field}
                                key={`${field.key}-rent`}
                                label="月租金"
                                name={[field.name, "base_monthly_rent"]}
                                rules={[{ required: true, message: "请填写月租金" }]}
                              >
                                <InputNumber style={{ width: "100%" }} min={0} precision={2} />
                              </Form.Item>
                            </Col>
                            <Col span={12}>
                              <Form.Item
                                {...field}
                                key={`${field.key}-free`}
                                label="免租期（月）"
                                name={[field.name, "rent_free_months"]}
                              >
                                <InputNumber style={{ width: "100%" }} min={0} />
                              </Form.Item>
                            </Col>
                            <Col span={12}>
                              <Form.Item
                                {...field}
                                key={`${field.key}-esc`}
                                label="年递增（%）"
                                name={[field.name, "annual_escalation_percent"]}
                              >
                                <InputNumber style={{ width: "100%" }} precision={2} />
                              </Form.Item>
                            </Col>
                            <Col span={12}>
                              <Form.Item
                                {...field}
                                key={`${field.key}-other`}
                                label="月度其他成本"
                                name={[field.name, "other_monthly_cost"]}
                                extra="物业费等，不随调租变动"
                              >
                                <InputNumber style={{ width: "100%" }} min={0} precision={2} />
                              </Form.Item>
                            </Col>
                            <Col span={12}>
                              <Form.Item
                                {...field}
                                key={`${field.key}-area`}
                                label="面积（㎡）"
                                name={[field.name, "area_sqm"]}
                                extra="留空则不出每平米单价"
                              >
                                <InputNumber style={{ width: "100%" }} min={0} precision={2} />
                              </Form.Item>
                            </Col>
                            <Col span={12}>
                              <Form.Item
                                {...field}
                                key={`${field.key}-upfront`}
                                label="前期投入"
                                name={[field.name, "upfront_cost"]}
                              >
                                <InputNumber style={{ width: "100%" }} min={0} precision={2} />
                              </Form.Item>
                            </Col>
                            <Col span={12}>
                              <Form.Item
                                {...field}
                                key={`${field.key}-contrib`}
                                label="出租方装修补贴"
                                name={[field.name, "landlord_contribution"]}
                              >
                                <InputNumber style={{ width: "100%" }} min={0} precision={2} />
                              </Form.Item>
                            </Col>
                          </Row>
                        </Card>
                      </Col>
                    ))}
                  </Row>

                  <Space style={{ margin: "16px 0" }}>
                    {fields.length < 5 && (
                      <Button
                        icon={<PlusOutlined />}
                        onClick={() => add({ ...INITIAL_OFFERS[1], name: `方案 ${String.fromCharCode(65 + fields.length)}` })}
                      >
                        添加方案
                      </Button>
                    )}
                    <Button type="primary" icon={<SwapOutlined />} htmlType="submit" loading={loading}>
                      比价
                    </Button>
                  </Space>
                </>
              )}
            </Form.List>
          </Form>

          {result && (
            <>
              <Alert
                type={result.measures_disagree ? "warning" : "success"}
                showIcon
                style={{ marginBottom: 16, borderRadius: 10 }}
                message={result.measures_disagree ? "两个口径结论不一致" : "两个口径结论一致"}
                description={result.conclusion}
              />

              <Card title="条款比价结果" style={{ borderRadius: 10, marginBottom: 16 }}>
                <Table
                  dataSource={result.offers}
                  rowKey="name"
                  pagination={false}
                  size="small"
                  scroll={{ x: 900 }}
                  columns={[
                    {
                      title: "方案",
                      dataIndex: "name",
                      render: (name: string) => (
                        <Space>
                          <strong>{name}</strong>
                          {name === result.best_by_present_value && <Tag color="green">现值最优</Tag>}
                          {name === result.best_by_effective_rent && <Tag color="blue">有效租金最优</Tag>}
                        </Space>
                      ),
                    },
                    {
                      title: "有效租金（月）",
                      dataIndex: "effective_monthly_rent",
                      align: "right" as const,
                      render: (value: number) => fmtMoney(value, currency),
                    },
                    {
                      title: "每平米有效单价",
                      dataIndex: "effective_rent_per_sqm",
                      align: "right" as const,
                      render: (value: number) => (value > 0 ? fmtMoney(value, currency) : "—"),
                    },
                    {
                      title: "首年租金",
                      dataIndex: "first_year_rent",
                      align: "right" as const,
                      render: (value: number) => fmtMoney(value, currency),
                    },
                    {
                      title: "全期租金",
                      dataIndex: "total_rent",
                      align: "right" as const,
                      render: (value: number) => fmtMoney(value, currency),
                    },
                    {
                      title: "全期总成本",
                      dataIndex: "total_cost",
                      align: "right" as const,
                      render: (value: number) => fmtMoney(value, currency),
                    },
                    {
                      title: "现值",
                      dataIndex: "present_value",
                      align: "right" as const,
                      render: (value: number) => <strong>{fmtMoney(value, currency)}</strong>,
                    },
                  ]}
                />
              </Card>

              <Card title="累计现金支出" style={{ borderRadius: 10 }}>
                <div style={{ color: "var(--fg-muted)", fontSize: 13, marginBottom: 12 }}>
                  免租期是一段平线，年递增是一段逐渐变陡的曲线——两条线交叉的位置，就是两个方案成本反超的时点。
                </div>
                <div style={{ width: "100%", height: 320 }}>
                  <ResponsiveContainer>
                    <LineChart data={chartData}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="month" tickFormatter={(value) => `${value}月`} />
                      <YAxis tickFormatter={(value) => `${Math.round(Number(value) / 10000)}万`} />
                      <Tooltip formatter={(value) => fmtMoney(Number(value), currency)} />
                      <Legend />
                      {result.offers.map((offer, index) => (
                        <Line
                          key={offer.name}
                          type="monotone"
                          dataKey={offer.name}
                          stroke={LINE_COLORS[index % LINE_COLORS.length]}
                          dot={false}
                          strokeWidth={2}
                        />
                      ))}
                    </LineChart>
                  </ResponsiveContainer>
                </div>
              </Card>
            </>
          )}
        </motion.div>
      </AppLayout>
    </ProtectedRoute>
  );
}
