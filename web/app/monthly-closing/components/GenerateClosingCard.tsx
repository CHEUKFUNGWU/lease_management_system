"use client";

import { Alert, Button, Card, Col, DatePicker, Form, Input, Row, Space, Tag } from "antd";
import { CalculatorOutlined } from "@ant-design/icons";
import { t, type Language } from "../../lib/i18n";

interface GenerateClosingCardProps {
  language: Language;
  loading: boolean;
  result: any;
  onGenerate: (values: any) => void;
}

export function GenerateClosingCard({
  language,
  loading,
  result,
  onGenerate,
}: GenerateClosingCardProps) {
  return (
    <Card
      title={
        <span style={{ fontSize: 15, fontWeight: 600, letterSpacing: "-0.01em" }}>
          {t("monthly.generate_closing", language)}
        </span>
      }
    >
      <Form layout="vertical" onFinish={onGenerate}>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item
              label={t("monthly.accounting_period", language)}
              name="period"
              rules={[{ required: true, message: t("monthly.select_period", language) }]}
            >
              <DatePicker.MonthPicker
                style={{ width: "100%" }}
                placeholder="YYYY-MM"
              />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              label={t("monthly.discount_rate", language)}
              name="discount_rate"
              initialValue={0.05}
            >
              <Input type="number" step={0.001} />
            </Form.Item>
          </Col>
        </Row>
        <Button
          type="primary"
          icon={<CalculatorOutlined />}
          htmlType="submit"
          loading={loading}
          size="large"
        >
          {t("monthly.generate_btn", language)}
        </Button>
      </Form>

      {result && (
        <Alert
          message={t("monthly.result_title", language)}
          description={
            <Space direction="vertical">
              <span>
                {t("monthly.batch_number", language)}:{" "}
                <strong style={{ color: "var(--fg-primary)" }}>
                  {result.batch_number}
                </strong>
              </span>
              <span>
                {t("monthly.status", language)}:{" "}
                <Tag color={result.status === "completed" ? "processing" : "warning"}>
                  {result.status}
                </Tag>
              </span>
              <span>
                {t("monthly.processed_contracts", language)}: {result.processed_contracts} / {result.total_contracts}
              </span>
              <span>{t("monthly.failed_contracts", language)}: {result.failed_contracts}</span>
              <span>{t("monthly.total_entries", language)}: {result.total_entries} 笔</span>
            </Space>
          }
          type="info"
          showIcon
          style={{ marginTop: 24 }}
        />
      )}
    </Card>
  );
}
