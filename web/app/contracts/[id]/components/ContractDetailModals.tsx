"use client";

import dayjs from "dayjs";
import { Button, Col, DatePicker, Descriptions, Form, Input, InputNumber, Modal, Row, Select, Tag } from "antd";
import type { FormInstance } from "antd";
import { t, type Language } from "../../../lib/i18n";
import type { EditDraftFormValues } from "./types";

export interface AdjustmentPreviewData {
  adjustment_type?: string;
  effective_date?: string;
  discount_rate?: number;
  liability_before?: number;
  liability_after?: number;
  liability_change?: number;
  asset_before?: number;
  asset_after?: number;
  asset_change?: number;
  pnl_impact?: number;
}

function formatCurrency(value?: number) {
  if (value == null) return "-";
  return `¥${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

function formatDelta(value?: number) {
  if (value == null) return "-";
  return `${value >= 0 ? "+" : ""}¥${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

export function RejectionReasonModal({
  title,
  open,
  value,
  loading,
  language,
  prompt,
  onCancel,
  onChange,
  onSubmit,
}: {
  title: string;
  open: boolean;
  value: string;
  loading: boolean;
  language: Language;
  prompt: string;
  onCancel: () => void;
  onChange: (value: string) => void;
  onSubmit: () => void;
}) {
  return (
    <Modal
      title={title}
      open={open}
      onCancel={onCancel}
      onOk={onSubmit}
      confirmLoading={loading}
      okText={t("contract.ok", language)}
      cancelText={t("contract.cancel", language)}
    >
      <p style={{ marginBottom: 8 }}>{prompt}</p>
      <Input.TextArea
        rows={4}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={t("contract.reason_placeholder", language)}
      />
    </Modal>
  );
}

export function EditDraftModal({
  open,
  form,
  language,
  onCancel,
  onSubmit,
}: {
  open: boolean;
  form: FormInstance<EditDraftFormValues>;
  language: Language;
  onCancel: () => void;
  onSubmit: (values: EditDraftFormValues) => void;
}) {
  return (
    <Modal
      title={t("contract.edit_draft", language)}
      open={open}
      onCancel={onCancel}
      onOk={() => form.submit()}
      width={500}
    >
      <Form
        form={form}
        layout="vertical"
        onFinish={onSubmit}
      >
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item
              label={t("contract.payment_date", language)}
              name="due_date"
              rules={[{ required: true, message: t("contract_detail.validation.payment_date", language) }]}
            >
              <DatePicker style={{ width: "100%" }} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              label={t("contract.amount", language)}
              name="amount"
              rules={[{ required: true, message: t("contract_detail.validation.amount", language) }]}
            >
              <InputNumber
                style={{ width: "100%" }}
                min={0}
                precision={2}
                prefix="¥"
              />
            </Form.Item>
          </Col>
        </Row>

        <Form.Item
          label={t("contract.payment_timing_label", language)}
          name="payment_timing"
          rules={[{ required: true }]}
        >
          <Select>
            <Select.Option value="prepaid">{t("contract.prepaid_label", language)}</Select.Option>
            <Select.Option value="postpaid">{t("contract.postpaid_label", language)}</Select.Option>
          </Select>
        </Form.Item>

        <Form.Item label={t("contract.amount_type", language)} name="amount_type">
          <Input placeholder={t("contract.amount_type_placeholder", language)} />
        </Form.Item>

        <Row gutter={16}>
          <Col span={12}>
            <Form.Item label={t("contract.is_fixed", language)} name="is_fixed">
              <Select>
                <Select.Option value={true}>{t("contract.yes", language)}</Select.Option>
                <Select.Option value={false}>{t("contract.no", language)}</Select.Option>
              </Select>
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item label={t("contract.is_lease_component", language)} name="is_lease_component">
              <Select>
                <Select.Option value={true}>{t("contract.yes", language)}</Select.Option>
                <Select.Option value={false}>{t("contract.no", language)}</Select.Option>
              </Select>
            </Form.Item>
          </Col>
        </Row>
      </Form>
    </Modal>
  );
}

export function AdjustmentPreviewModal({
  title,
  open,
  data,
  language,
  onClose,
}: {
  title: string;
  open: boolean;
  data: AdjustmentPreviewData | null;
  language: Language;
  onClose: () => void;
}) {
  return (
    <Modal
      title={title}
      open={open}
      onCancel={onClose}
      footer={[
        <Button key="close" onClick={onClose}>{t("contract.close", language)}</Button>,
      ]}
      width={700}
    >
      {data && (
        <Descriptions column={1} bordered size="small">
          <Descriptions.Item label={t("contract_detail.adjustment_type_label", language)}>
            <Tag color="processing">{data.adjustment_type || "-"}</Tag>
          </Descriptions.Item>
          <Descriptions.Item label={t("contract.effective_date", language)}>
            {data.effective_date ? dayjs(data.effective_date).format("YYYY-MM-DD") : "-"}
          </Descriptions.Item>
          <Descriptions.Item label={t("contract.discount_rate", language)}>
            {data.discount_rate != null ? `${(data.discount_rate * 100).toFixed(2)}%` : "-"}
          </Descriptions.Item>
          <Descriptions.Item label={t("contract_detail.liability_before", language)}>
            {formatCurrency(data.liability_before)}
          </Descriptions.Item>
          <Descriptions.Item label={t("contract_detail.liability_after", language)}>
            {data.liability_after != null ? <span style={{ fontWeight: "bold", color: "#000" }}>{formatCurrency(data.liability_after)}</span> : "-"}
          </Descriptions.Item>
          <Descriptions.Item label={t("contract_detail.liability_change", language)}>
            {data.liability_change != null ? <span style={{ fontWeight: "bold", color: "#000" }}>{formatDelta(data.liability_change)}</span> : "-"}
          </Descriptions.Item>
          <Descriptions.Item label={t("contract_detail.asset_before", language)}>
            {formatCurrency(data.asset_before)}
          </Descriptions.Item>
          <Descriptions.Item label={t("contract_detail.asset_after", language)}>
            {data.asset_after != null ? <span style={{ fontWeight: "bold", color: "#000" }}>{formatCurrency(data.asset_after)}</span> : "-"}
          </Descriptions.Item>
          <Descriptions.Item label={t("contract_detail.asset_change", language)}>
            {data.asset_change != null ? <span style={{ fontWeight: "bold", color: "#000" }}>{formatDelta(data.asset_change)}</span> : "-"}
          </Descriptions.Item>
          <Descriptions.Item label={t("contract_detail.pnl_impact", language)}>
            {data.pnl_impact != null ? <span style={{ fontWeight: "bold", color: "#000" }}>{formatDelta(data.pnl_impact)}</span> : "-"}
          </Descriptions.Item>
        </Descriptions>
      )}
    </Modal>
  );
}
