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

const DEFAULT_DISCOUNT_RATE = 0.0485;

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
  const { language } = useLanguage();
  const [form] = Form.useForm();
  const [briefing, setBriefing] = useState<Briefing | null>(null);
  const [loading, setLoading] = useState(false);
  const selectedDiscountRate = Form.useWatch("discount_rate", form);
  const discountRateOverridden =
    selectedDiscountRate != null && Math.abs(Number(selectedDiscountRate) - DEFAULT_DISCOUNT_RATE) > 0.000001;

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
            currency: values.currency,
            initial_direct_cost: values.initial_direct_cost || 0,
            early_exit_penalty_months: values.early_exit_penalty_months || 0,
          },
          token
        )
      );
    } catch (error: any) {
      notifyError(error?.message || t("pre_deal.err_failed", language));
      setBriefing(null);
    } finally {
      setLoading(false);
    }
  };

  const currency = briefing?.currency;

  return (
    <ProtectedRoute>
      <AppLayout>
        <motion.div initial={false} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.25 }}>
          <PageHeader
            title={<>{t("pre_deal.title", language)}<span className="page-header-count">{t("pre_deal.header_count", language, { currency: currency || "—", count: String(briefing?.yearly?.length ?? 0) })}</span></>}
          />

          <Form
            form={form}
            layout="vertical"
            initialValues={{ discount_rate: DEFAULT_DISCOUNT_RATE }}
            onFinish={handleBuild}
          >
            <Card title={t("pre_deal.card_terms", language)} style={{ borderRadius: 10, marginBottom: 16 }}>
              <Row gutter={16}>
                <Col xs={24} md={6}>
                  <Form.Item label={t("pre_deal.label_name", language)} name="name" rules={[{ required: true, message: t("pre_deal.err_name", language) }]}>
                    <Input />
                  </Form.Item>
                </Col>
                <Col xs={12} md={4}>
                  <Form.Item label={t("pre_deal.label_start", language)} name="commencement_date" rules={[{ required: true, message: t("pre_deal.err_start", language) }]}>
                    <DatePicker style={{ width: "100%" }} />
                  </Form.Item>
                </Col>
                <Col xs={12} md={3}>
                  <Form.Item label={t("pre_deal.label_term", language)} name="term_months" rules={[{ required: true, message: t("pre_deal.err_term", language) }]}>
                    <InputNumber style={{ width: "100%" }} min={1} />
                  </Form.Item>
                </Col>
                <Col xs={12} md={4}>
                  <Form.Item label={t("pre_deal.label_rent", language)} name="monthly_rent" rules={[{ required: true, message: t("pre_deal.err_rent", language) }]}>
                    <InputNumber style={{ width: "100%" }} min={0} precision={2} />
                  </Form.Item>
                </Col>
                <Col xs={12} md={3}>
                  <Form.Item
                    label={t("pre_deal.label_rate", language)}
                    name="discount_rate"
                    rules={[{ required: true, message: t("pre_deal.err_rate", language) }]}
                    extra={
                      <span>
                        {t("pre_deal.rate_source", language)} · {discountRateOverridden ? t("pre_deal.rate_overridden", language) : t("pre_deal.rate_default", language)} · {t("pre_deal.rate_note", language)}
                      </span>
                    }
                  >
                    <InputNumber style={{ width: "100%" }} min={0.0001} max={1} step={0.005} precision={4} />
                  </Form.Item>
                </Col>
                <Col xs={12} md={4}>
                  <Form.Item label={t("pre_deal.label_currency", language)} name="currency" rules={[{ required: true, message: t("pre_deal.err_currency", language) }]}>
                    <Input />
                  </Form.Item>
                </Col>
              </Row>
              <Row gutter={16}>
                <Col xs={12} md={4}>
                  <Form.Item label={t("pre_deal.label_free", language)} name="rent_free_months">
                    <InputNumber style={{ width: "100%" }} min={0} />
                  </Form.Item>
                </Col>
                <Col xs={12} md={4}>
                  <Form.Item label={t("pre_deal.label_escalation", language)} name="annual_escalation_percent">
                    <InputNumber style={{ width: "100%" }} precision={2} />
                  </Form.Item>
                </Col>
                <Col xs={12} md={5}>
                  <Form.Item label={t("pre_deal.label_direct_cost", language)} name="initial_direct_cost" extra={t("pre_deal.hint_direct_cost", language)}>
                    <InputNumber style={{ width: "100%" }} min={0} precision={2} />
                  </Form.Item>
                </Col>
                <Col xs={12} md={5}>
                  <Form.Item label={t("pre_deal.label_exit_penalty", language)} name="early_exit_penalty_months" extra={t("pre_deal.hint_exit_penalty", language)}>
                    <InputNumber style={{ width: "100%" }} min={0} precision={1} />
                  </Form.Item>
                </Col>
                <Col xs={24} md={6}>
                  <Form.Item label=" ">
                    <Button type="primary" icon={<FileSearchOutlined />} htmlType="submit" loading={loading} block>
                      {t("pre_deal.btn_brief", language)}
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
                message={t("pre_deal.alert_title", language)}
                description={briefing.headline}
              />

              <div className="stripe-metric-grid" style={{ gridTemplateColumns: "repeat(4, minmax(0, 1fr))", marginBottom: 16 }}>
                <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 90, padding: "16px 20px" }}>
                  <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("pre_deal.stat_liability", language)}</span>
                  <div style={{ margin: "8px 0 0" }}>
                    <Typography.Text className="font-tabular" style={{ fontSize: 22, fontWeight: 600, color: "var(--fg-primary)" }}>
                      {fmtMoney(briefing.balance_sheet.initial_liability, currency)}
                    </Typography.Text>
                  </div>
                </div>
                <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 90, padding: "16px 20px" }}>
                  <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("pre_deal.stat_rou", language)}</span>
                  <div style={{ margin: "8px 0 0" }}>
                    <Typography.Text className="font-tabular" style={{ fontSize: 22, fontWeight: 600, color: "var(--fg-primary)" }}>
                      {fmtMoney(briefing.balance_sheet.initial_rou, currency)}
                    </Typography.Text>
                  </div>
                </div>
                <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 90, padding: "16px 20px" }}>
                  <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("pre_deal.stat_commitment", language)}</span>
                  <div style={{ margin: "8px 0 0" }}>
                    <Typography.Text className="font-tabular" style={{ fontSize: 22, fontWeight: 600, color: "var(--fg-primary)" }}>
                      {fmtMoney(briefing.balance_sheet.undiscounted_commitment, currency)}
                    </Typography.Text>
                  </div>
                </div>
                <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 90, padding: "16px 20px" }}>
                  <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("pre_deal.stat_discount_effect", language)}</span>
                  <div style={{ margin: "8px 0 0" }}>
                    <Typography.Text className="font-tabular" style={{ fontSize: 22, fontWeight: 600, color: "var(--fg-primary)" }}>
                      {fmtMoney(briefing.balance_sheet.discounting_effect, currency)}
                    </Typography.Text>
                  </div>
                </div>
              </div>

              <Card title={t("pre_deal.card_expense_curve", language)} style={{ borderRadius: 10, marginBottom: 16 }}>
                <div style={{ color: "var(--fg-muted)", fontSize: 13, marginBottom: 12 }}>
                  {t("pre_deal.expense_curve_note", language, { years: String(briefing.front_loaded_years) })}
                </div>
                <div style={{ width: "100%", height: 300 }}>
                  <ResponsiveContainer>
                    <LineChart data={briefing.yearly}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="year" tickFormatter={(value) => t("pre_deal.year_suffix", language, { value: String(value) })} />
                      <YAxis tickFormatter={(value) => t("pre_deal.axis_wan", language, { value: String(Math.round(Number(value) / 10000)) })} />
                      <Tooltip formatter={(value) => fmtMoney(Number(value), currency)} />
                      <Legend />
                      <Line isAnimationActive={false} type="monotone" dataKey="ifrs16_expense" name={t("pre_deal.series_ifrs16", language)} stroke="var(--state-error-text)" strokeWidth={2} dot={false} />
                      <Line isAnimationActive={false} type="monotone" dataKey="straight_line_rent" name={t("pre_deal.series_straight", language)} stroke="var(--chart-blue)" strokeWidth={2} strokeDasharray="5 5" dot={false} />
                      <Line isAnimationActive={false} type="monotone" dataKey="cash_rent" name={t("pre_deal.series_cash", language)} stroke="var(--fg-muted)" strokeWidth={1} dot={false} />
                    </LineChart>
                  </ResponsiveContainer>
                </div>
              </Card>

              <Card title={t("pre_deal.card_ebitda", language)} style={{ borderRadius: 10, marginBottom: 16 }}>
                <div style={{ color: "var(--fg-muted)", fontSize: 13, marginBottom: 12 }}>
                  {t("pre_deal.ebitda_note", language)}
                </div>
                <div style={{ width: "100%", height: 280 }}>
                  <ResponsiveContainer>
                    <BarChart data={briefing.ebitda_bridge}>
                      <CartesianGrid strokeDasharray="3 3" />
                      <XAxis dataKey="year" tickFormatter={(value) => t("pre_deal.year_suffix", language, { value: String(value) })} />
                      <YAxis tickFormatter={(value) => t("pre_deal.axis_wan", language, { value: String(Math.round(Number(value) / 10000)) })} />
                      <Tooltip formatter={(value) => fmtMoney(Number(value), currency)} />
                      <Legend />
                      <ReferenceLine y={0} stroke="var(--fg-primary)" />
                      <Bar isAnimationActive={false} dataKey="ebitda_uplift" name={t("pre_deal.series_ebitda_uplift", language)} fill="var(--state-success-text)" />
                      <Bar isAnimationActive={false} dataKey="depreciation_below_ebitda" name={t("pre_deal.series_depreciation", language)} fill="var(--chart-blue)" />
                      <Bar isAnimationActive={false} dataKey="interest_below_ebit" name={t("pre_deal.series_interest", language)} fill="var(--state-warning-text)" />
                      <Bar isAnimationActive={false} dataKey="net_profit_impact" name={t("pre_deal.series_net_profit", language)} fill="var(--state-error-text)" />
                    </BarChart>
                  </ResponsiveContainer>
                </div>
              </Card>

              <Card title={t("pre_deal.card_exit", language)} style={{ borderRadius: 10 }}>
                <div style={{ color: "var(--fg-muted)", fontSize: 13, marginBottom: 12 }}>
                  {t("pre_deal.exit_note", language)}
                </div>
                <Table
                  dataSource={briefing.exit_curve}
                  rowKey="year"
                  pagination={false}
                  size="small"
                  scroll={tableScrollX((briefing.exit_curve || []).length, 800)}
                  columns={[
                    { title: t("pre_deal.col_exit_point", language), dataIndex: "year", render: (value: number) => t("pre_deal.exit_year", language, { value: String(value) }) },
                    {
                      title: t("pre_deal.col_remaining", language),
                      dataIndex: "remaining_commitment",
                      align: "right" as const,
                      render: (value: number) => fmtMoney(value, currency),
                    },
                    {
                      title: t("pre_deal.col_released", language),
                      dataIndex: "liability_released",
                      align: "right" as const,
                      render: (value: number) => fmtMoney(value, currency),
                    },
                    {
                      title: t("pre_deal.col_rou", language),
                      dataIndex: "rou_written_off",
                      align: "right" as const,
                      render: (value: number) => fmtMoney(value, currency),
                    },
                    {
                      title: t("pre_deal.col_penalty", language),
                      dataIndex: "penalty",
                      align: "right" as const,
                      render: (value: number) => fmtMoney(value, currency),
                    },
                    {
                      title: t("pre_deal.col_pnl", language),
                      dataIndex: "pnl_impact",
                      align: "right" as const,
                      render: (value: number) => (
                        <strong style={{ color: value > 0 ? "var(--state-error-text)" : "var(--state-success-text)" }}>
                          {fmtMoney(value, currency)}
                        </strong>
                      ),
                    },
                    {
                      title: t("pre_deal.col_cash_out", language),
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
