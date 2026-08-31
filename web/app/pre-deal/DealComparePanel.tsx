"use client";

// Pre-signing offer comparison (the "last mile" wiring from the capability
// audit §A/§E-2): POST /deals/compare reduces 2-5 sets of terms to the same
// comparable numbers. Offers are hypothetical terms, not contracts — this
// panel reads and writes no ledger. Effective rent (accounting view) and
// present value (cash view) may pick different winners; the server-side
// conclusion says so, and we render it as-is instead of picking a side.

import { useState } from "react";
import { Alert, Button, Card, Col, Form, Input, InputNumber, Row, Table, Typography } from "antd";
import { DeleteOutlined, PlusOutlined, SwapOutlined } from "@ant-design/icons";
import { dealApi } from "../lib/api";
import { fmtMoney } from "../lib/format";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { notifyError } from "../lib/notify";
import { t } from "../lib/i18n";
import { tableScrollX } from "../lib/tableScroll";

interface OfferResult {
  name: string;
  total_rent: number;
  total_cost: number;
  effective_monthly_rent: number;
  effective_rent_per_sqm: number;
  present_value: number;
  first_year_rent: number;
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

const COMPARE_OFFER_DEFAULTS = {
  term_months: 60,
  base_monthly_rent: undefined as number | undefined,
  rent_free_months: 0,
  annual_escalation_percent: 0,
  other_monthly_cost: 0,
  upfront_cost: 0,
  landlord_contribution: 0,
  area_sqm: 0,
};

export function DealComparePanel() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [comparison, setComparison] = useState<Comparison | null>(null);
  const offers = Form.useWatch("offers", form) as Array<Record<string, unknown>> | undefined;

  const handleCompare = async (values: any) => {
    if (!token) return;
    if (values.discount_rate == null) {
      notifyError(t("pre_deal.err_rate", language));
      return;
    }
    setLoading(true);
    try {
      setComparison(
        await dealApi.compare<Comparison>(
          {
            discount_rate: values.discount_rate,
            currency: values.currency || undefined,
            offers: (values.offers || []).map((offer: Record<string, any>) => ({
              name: offer.name,
              term_months: offer.term_months,
              base_monthly_rent: offer.base_monthly_rent,
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
      notifyError(error?.message || t("pre_deal.compare.failed", language));
      setComparison(null);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card title={t("pre_deal.compare.title", language)} className="dc-panel-card">
      <Typography.Text type="secondary" className="dc-panel-desc">
        {t("pre_deal.compare.desc", language)}
      </Typography.Text>
      <Form
        form={form}
        layout="vertical"
        initialValues={{ discount_rate: undefined, offers: [{ ...COMPARE_OFFER_DEFAULTS, name: "" }, { ...COMPARE_OFFER_DEFAULTS, name: "" }] }}
        onFinish={handleCompare}
      >
        <Row gutter={16}>
          <Col xs={24} md={6}>
            <Form.Item
              label={t("pre_deal.label_rate", language)}
              name="discount_rate"
              rules={[{ required: true, message: t("pre_deal.err_rate", language) }]}
              extra={t("pre_deal.rate_note", language)}
            >
              <InputNumber className="dc-input-full" min={0.0001} max={1} step={0.005} precision={4} />
            </Form.Item>
          </Col>
          <Col xs={12} md={4}>
            <Form.Item label={`${t("pre_deal.label_currency", language)}（${t("common.optional", language)}）`} name="currency">
              <Input maxLength={3} />
            </Form.Item>
          </Col>
        </Row>

        <Form.List name="offers">
          {(fields, { add, remove }) => (
            <>
              {fields.map((field) => (
                <div key={field.key} className="dc-offer-box">
                  <Row gutter={12}>
                    <Col xs={12} md={5}>
                      <Form.Item label={t("pre_deal.compare.offer_name", language)} name={[field.name, "name"]} rules={[{ required: true, message: t("pre_deal.err_name", language) }]}>
                        <Input />
                      </Form.Item>
                    </Col>
                    <Col xs={6} md={3}>
                      <Form.Item label={t("pre_deal.label_term", language)} name={[field.name, "term_months"]} rules={[{ required: true, message: t("pre_deal.err_term", language) }]}>
                        <InputNumber className="dc-input-full" min={1} />
                      </Form.Item>
                    </Col>
                    <Col xs={6} md={4}>
                      <Form.Item label={t("pre_deal.label_rent", language)} name={[field.name, "base_monthly_rent"]} rules={[{ required: true, message: t("pre_deal.err_rent", language) }]}>
                        <InputNumber className="dc-input-full" min={0} precision={2} />
                      </Form.Item>
                    </Col>
                    <Col xs={6} md={3}>
                      <Form.Item label={t("pre_deal.label_free", language)} name={[field.name, "rent_free_months"]}>
                        <InputNumber className="dc-input-full" min={0} />
                      </Form.Item>
                    </Col>
                    <Col xs={6} md={4}>
                      <Form.Item label={t("pre_deal.label_escalation", language)} name={[field.name, "annual_escalation_percent"]}>
                        <InputNumber className="dc-input-full" precision={2} />
                      </Form.Item>
                    </Col>
                    <Col xs={6} md={3}>
                      <Form.Item label={t("pre_deal.compare.other_monthly_cost", language)} name={[field.name, "other_monthly_cost"]}>
                        <InputNumber className="dc-input-full" min={0} precision={2} />
                      </Form.Item>
                    </Col>
                    <Col xs={6} md={2}>
                      <Form.Item label=" ">
                        <Button
                          className="dc-input-full"
                          icon={<DeleteOutlined />}
                          disabled={fields.length <= 2}
                          onClick={() => remove(field.name)}
                        />
                      </Form.Item>
                    </Col>
                  </Row>
                  <Row gutter={12}>
                    <Col xs={6} md={5}>
                      <Form.Item label={t("pre_deal.label_direct_cost", language)} name={[field.name, "upfront_cost"]}>
                        <InputNumber className="dc-input-full" min={0} precision={2} />
                      </Form.Item>
                    </Col>
                    <Col xs={6} md={5}>
                      <Form.Item label={t("pre_deal.compare.landlord_contribution", language)} name={[field.name, "landlord_contribution"]}>
                        <InputNumber className="dc-input-full" min={0} precision={2} />
                      </Form.Item>
                    </Col>
                    <Col xs={6} md={5}>
                      <Form.Item label={t("pre_deal.compare.area_sqm", language)} name={[field.name, "area_sqm"]}>
                        <InputNumber className="dc-input-full" min={0} precision={1} />
                      </Form.Item>
                    </Col>
                  </Row>
                </div>
              ))}
              <Form.Item>
                <Button
                  icon={<PlusOutlined />}
                  disabled={(offers?.length ?? 0) >= 5}
                  onClick={() => add({ ...COMPARE_OFFER_DEFAULTS, name: "" })}
                >
                  {t("pre_deal.compare.add_offer", language)}
                </Button>
              </Form.Item>
            </>
          )}
        </Form.List>

        <Button type="primary" icon={<SwapOutlined />} htmlType="submit" loading={loading}>
          {t("pre_deal.compare.run", language)}
        </Button>
      </Form>

      {comparison && (
        <>
          <Alert
            type={comparison.measures_disagree ? "warning" : "success"}
            showIcon
            className="dc-conclusion-alert"
            message={
              comparison.measures_disagree
                ? t("pre_deal.compare.disagree", language)
                : `${t("pre_deal.compare.conclusion_title", language)} · ${comparison.best_by_present_value}`
            }
            description={comparison.conclusion}
          />
          <Table
            className="dc-result-table"
            dataSource={comparison.offers}
            rowKey="name"
            pagination={false}
            size="small"
            scroll={tableScrollX(comparison.offers.length, 960)}
            columns={[
              {
                title: t("pre_deal.compare.offer_name", language),
                dataIndex: "name",
                render: (value: string, record: OfferResult) => (
                  <strong className={record.name === comparison.best_by_present_value ? "dc-best-name" : undefined}>{value}</strong>
                ),
              },
              { title: t("pre_deal.compare.col_total_rent", language), dataIndex: "total_rent", align: "right" as const, render: (v: number) => fmtMoney(v, comparison.currency) },
              { title: t("pre_deal.compare.col_total_cost", language), dataIndex: "total_cost", align: "right" as const, render: (v: number) => fmtMoney(v, comparison.currency) },
              { title: t("pre_deal.compare.col_effective", language), dataIndex: "effective_monthly_rent", align: "right" as const, render: (v: number) => fmtMoney(v, comparison.currency) },
              {
                title: t("pre_deal.compare.col_per_sqm", language),
                dataIndex: "effective_rent_per_sqm",
                align: "right" as const,
                render: (v: number) => (v > 0 ? fmtMoney(v, comparison.currency) : "—"),
              },
              { title: t("pre_deal.compare.col_pv", language), dataIndex: "present_value", align: "right" as const, render: (v: number) => fmtMoney(v, comparison.currency) },
              { title: t("pre_deal.compare.col_first_year", language), dataIndex: "first_year_rent", align: "right" as const, render: (v: number) => fmtMoney(v, comparison.currency) },
            ]}
          />
          <Typography.Text type="secondary" className="dc-result-note">
            {t("pre_deal.compare.note", language, { rate: (comparison.discount_rate * 100).toFixed(2) })}
          </Typography.Text>
        </>
      )}
    </Card>
  );
}
