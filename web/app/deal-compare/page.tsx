"use client";

import { StatusTag } from "../components/StatusTag";

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
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { dealApi } from "../lib/api";
import { fmtMoney } from "../lib/format";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { notifyError } from "../lib/notify";
import { t } from "../lib/i18n";
import { tableScrollX } from "../lib/tableScroll";

const { Text } = Typography;

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

const LINE_COLORS = ["var(--chart-blue)", "var(--state-error-text)", "var(--state-success-text)", "var(--state-warning-text)", "var(--chart-purple)"];

// A comparison starts from the two shapes a negotiation usually offers: an
// incentive package with escalation, and a flat deal.
const INITIAL_OFFERS = [
  {
    name: "A",
    term_months: undefined,
    base_monthly_rent: undefined,
    rent_free_months: undefined,
    annual_escalation_percent: undefined,
    other_monthly_cost: undefined,
    upfront_cost: undefined,
    landlord_contribution: undefined,
    area_sqm: undefined,
  },
  {
    name: "B",
    term_months: undefined,
    base_monthly_rent: undefined,
    rent_free_months: undefined,
    annual_escalation_percent: undefined,
    other_monthly_cost: undefined,
    upfront_cost: undefined,
    landlord_contribution: undefined,
    area_sqm: undefined,
  },
];

export default function DealComparePage() {
  const { token } = useAuth();
  const { language } = useLanguage();
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
            currency: values.currency,
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
      notifyError(error?.message || t("deal_compare.err_failed", language));
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
        <motion.div initial={false} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.25 }}>
          <PageHeader
            title={<>{t("deal_compare.title", language)}<span className="page-header-count">{t("deal_compare.header_count", language, { count: String(chartData.length) })}</span></>}
          />

          <Form
            form={form}
            layout="vertical"
            onFinish={handleCompare}
            initialValues={{ offers: INITIAL_OFFERS }}
          >
            <Card style={{ borderRadius: 10, marginBottom: 16 }}>
              <Row gutter={16}>
                <Col xs={24} md={8}>
                  <Form.Item
                    label={t("deal_compare.label_rate", language)}
                    name="discount_rate"
                    rules={[{ required: true, message: t("deal_compare.err_rate", language) }]}
                    extra={t("deal_compare.hint_rate", language)}
                  >
                    <InputNumber style={{ width: "100%" }} min={0.0001} max={1} step={0.005} precision={4} />
                  </Form.Item>
                </Col>
                <Col xs={24} md={8}>
                  <Form.Item label={t("deal_compare.label_currency", language)} name="currency" rules={[{ required: true, message: t("deal_compare.err_currency", language) }]}>
                    <Input placeholder="ISO 4217" />
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
                              rules={[{ required: true, message: t("deal_compare.err_name", language) }]}
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
                                label={t("deal_compare.label_term", language)}
                                name={[field.name, "term_months"]}
                                rules={[{ required: true, message: t("deal_compare.err_term", language) }]}
                              >
                                <InputNumber style={{ width: "100%" }} min={1} />
                              </Form.Item>
                            </Col>
                            <Col span={12}>
                              <Form.Item
                                {...field}
                                key={`${field.key}-rent`}
                                label={t("deal_compare.label_rent", language)}
                                name={[field.name, "base_monthly_rent"]}
                                rules={[{ required: true, message: t("deal_compare.err_rent", language) }]}
                              >
                                <InputNumber style={{ width: "100%" }} min={0} precision={2} />
                              </Form.Item>
                            </Col>
                            <Col span={12}>
                              <Form.Item
                                {...field}
                                key={`${field.key}-free`}
                                label={t("deal_compare.label_free", language)}
                                name={[field.name, "rent_free_months"]}
                              >
                                <InputNumber style={{ width: "100%" }} min={0} />
                              </Form.Item>
                            </Col>
                            <Col span={12}>
                              <Form.Item
                                {...field}
                                key={`${field.key}-esc`}
                                label={t("deal_compare.label_esc", language)}
                                name={[field.name, "annual_escalation_percent"]}
                              >
                                <InputNumber style={{ width: "100%" }} precision={2} />
                              </Form.Item>
                            </Col>
                            <Col span={12}>
                              <Form.Item
                                {...field}
                                key={`${field.key}-other`}
                                label={t("deal_compare.label_other", language)}
                                name={[field.name, "other_monthly_cost"]}
                                extra={t("deal_compare.hint_other", language)}
                              >
                                <InputNumber style={{ width: "100%" }} min={0} precision={2} />
                              </Form.Item>
                            </Col>
                            <Col span={12}>
                              <Form.Item
                                {...field}
                                key={`${field.key}-area`}
                                label={t("deal_compare.label_area", language)}
                                name={[field.name, "area_sqm"]}
                                extra={t("deal_compare.hint_area", language)}
                              >
                                <InputNumber style={{ width: "100%" }} min={0} precision={2} />
                              </Form.Item>
                            </Col>
                            <Col span={12}>
                              <Form.Item
                                {...field}
                                key={`${field.key}-upfront`}
                                label={t("deal_compare.label_upfront", language)}
                                name={[field.name, "upfront_cost"]}
                              >
                                <InputNumber style={{ width: "100%" }} min={0} precision={2} />
                              </Form.Item>
                            </Col>
                            <Col span={12}>
                              <Form.Item
                                {...field}
                                key={`${field.key}-contrib`}
                                label={t("deal_compare.label_contrib", language)}
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
                        onClick={() => add({ ...INITIAL_OFFERS[1], name: t("deal_compare.plan_name", language, { letter: String.fromCharCode(65 + fields.length) }) })}
                      >
                        {t("deal_compare.add_plan", language)}
                      </Button>
                    )}
                    <Button type="primary" icon={<SwapOutlined />} htmlType="submit" loading={loading}>
                      {t("deal_compare.btn_compare", language)}
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
                message={result.measures_disagree ? t("deal_compare.disagree", language) : t("deal_compare.agree", language)}
                description={result.conclusion}
              />

              <Card title={t("deal_compare.card_result", language)} style={{ borderRadius: 10, marginBottom: 16 }}>
                <Table
                  dataSource={result.offers}
                  rowKey="name"
                  pagination={false}
                  size="small"
                  scroll={tableScrollX((result.offers || []).length, 900)}
                  columns={[
                    {
                      title: t("deal_compare.col_plan", language),
                      dataIndex: "name",
                      render: (name: string) => (
                        <Space>
                          <strong>{/^[A-Z]$/.test(name) ? t("deal_compare.plan_name", language, { letter: name }) : name}</strong>
                          {name === result.best_by_present_value && <StatusTag kind="success">{t("deal_compare.badge_pv", language)}</StatusTag>}
                          {name === result.best_by_effective_rent && <StatusTag kind="processing">{t("deal_compare.badge_rent", language)}</StatusTag>}
                        </Space>
                      ),
                    },
                    {
                      title: t("deal_compare.col_eff_rent", language),
                      dataIndex: "effective_monthly_rent",
                      align: "right" as const,
                      render: (value: number) => fmtMoney(value, currency),
                    },
                    {
                      title: t("deal_compare.col_eff_sqm", language),
                      dataIndex: "effective_rent_per_sqm",
                      align: "right" as const,
                      render: (value: number) => (value > 0 ? fmtMoney(value, currency) : "—"),
                    },
                    {
                      title: t("deal_compare.col_first_year", language),
                      dataIndex: "first_year_rent",
                      align: "right" as const,
                      render: (value: number) => fmtMoney(value, currency),
                    },
                    {
                      title: t("deal_compare.col_total_rent", language),
                      dataIndex: "total_rent",
                      align: "right" as const,
                      render: (value: number) => fmtMoney(value, currency),
                    },
                    {
                      title: t("deal_compare.col_total_cost", language),
                      dataIndex: "total_cost",
                      align: "right" as const,
                      render: (value: number) => fmtMoney(value, currency),
                    },
                    {
                      title: t("deal_compare.col_pv", language),
                      dataIndex: "present_value",
                      align: "right" as const,
                      render: (value: number) => <strong>{fmtMoney(value, currency)}</strong>,
                    },
                  ]}
                />
              </Card>

              <Card title={t("deal_compare.card_cash", language)} style={{ borderRadius: 10 }}>
                <div style={{ color: "var(--fg-muted)", fontSize: 13, marginBottom: 12 }}>
                  {t("deal_compare.cash_note", language)}
                </div>
                <div style={{ width: "100%", height: 320 }}>
                  <ResponsiveContainer>
                    <LineChart data={chartData}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="month" tickFormatter={(value) => t("deal_compare.month_suffix", language, { value: String(value) })} />
                      <YAxis tickFormatter={(value) => t("deal_compare.axis_wan", language, { value: String(Math.round(Number(value) / 10000)) })} />
                      <Tooltip formatter={(value) => fmtMoney(Number(value), currency)} />
                      <Legend />
                      {result.offers.map((offer, index) => (
                        <Line isAnimationActive={false}
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
