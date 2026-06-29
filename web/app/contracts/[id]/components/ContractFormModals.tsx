"use client";

import { Col, DatePicker, Form, Input, InputNumber, Modal, Row, Select } from "antd";
import type { FormInstance } from "antd";
import { DEFAULT_TAG_SUGGESTIONS } from "../../../lib/tags";
import { t, type Language } from "../../../lib/i18n";
import type {
  ContractUpdateFormValues,
  CreateEventFormValues,
  ScheduleFormValues,
} from "./types";

export function CreateScheduleModal({
  open,
  form,
  language,
  onCancel,
  onSubmit,
}: {
  open: boolean;
  form: FormInstance<ScheduleFormValues>;
  language: Language;
  onCancel: () => void;
  onSubmit: (values: ScheduleFormValues) => void;
}) {
  return (
    <Modal
      title={t("contract.add_payment", language)}
      open={open}
      onCancel={onCancel}
      onOk={() => form.submit()}
      width={700}
    >
      <Form
        form={form}
        layout="vertical"
        onFinish={onSubmit}
        initialValues={{
          currency: "CNY",
          payment_timing: "postpaid",
          amount_type: "fixed_rent",
          is_fixed: true,
          is_lease_component: true,
          included_in_liability_pv: true,
        }}
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

        <Row gutter={16}>
          <Col span={12}>
            <Form.Item label={t("contract.currency", language)} name="currency">
              <Select>
                <Select.Option value="CNY">{t("contract.currency_cny", language)}</Select.Option>
                <Select.Option value="USD">{t("contract.currency_usd", language)}</Select.Option>
              </Select>
            </Form.Item>
          </Col>
          <Col span={12}>
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
          </Col>
        </Row>

        <Row gutter={16}>
          <Col span={12}>
            <Form.Item label={t("contract.effective_start_date", language)} name="effective_start_date">
              <DatePicker style={{ width: "100%" }} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item label={t("contract.effective_end_date", language)} name="effective_end_date">
              <DatePicker style={{ width: "100%" }} />
            </Form.Item>
          </Col>
        </Row>

        <Row gutter={16}>
          <Col span={12}>
            <Form.Item label={t("contract.coverage_start_date", language)} name="coverage_start_date">
              <DatePicker style={{ width: "100%" }} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item label={t("contract.coverage_end_date", language)} name="coverage_end_date">
              <DatePicker style={{ width: "100%" }} />
            </Form.Item>
          </Col>
        </Row>

        <Form.Item label={t("contract.amount_type", language)} name="amount_type">
          <Input placeholder={t("contract.amount_type_placeholder", language)} />
        </Form.Item>

        <Row gutter={16}>
          <Col span={8}>
            <Form.Item label={t("contract.is_fixed", language)} name="is_fixed">
              <Select>
                <Select.Option value={true}>{t("contract.yes", language)}</Select.Option>
                <Select.Option value={false}>{t("contract.no", language)}</Select.Option>
              </Select>
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item label={t("contract.is_lease_component", language)} name="is_lease_component">
              <Select>
                <Select.Option value={true}>{t("contract.yes", language)}</Select.Option>
                <Select.Option value={false}>{t("contract.no", language)}</Select.Option>
              </Select>
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item label={t("contract.included_in_liability_pv", language)} name="included_in_liability_pv">
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

export function CreateEventModal({
  open,
  loading,
  form,
  language,
  onCancel,
  onSubmit,
}: {
  open: boolean;
  loading: boolean;
  form: FormInstance<CreateEventFormValues>;
  language: Language;
  onCancel: () => void;
  onSubmit: (values: CreateEventFormValues) => void;
}) {
  return (
    <Modal
      title={t("contract.register_event", language)}
      open={open}
      onCancel={onCancel}
      onOk={() => form.submit()}
      confirmLoading={loading}
      width={600}
    >
      <Form form={form} layout="vertical" onFinish={onSubmit}>
        <Form.Item
          label={t("contract.tab_events", language)}
          name="event_type"
          rules={[{ required: true, message: t("contract_detail.validation.event_type", language) }]}
        >
          <Select placeholder={t("contract.select_event_type", language)}>
            <Select.Option value="area_adjustment">{t("contract.event_type.area_adjustment", language)}</Select.Option>
            <Select.Option value="rent_change">{t("contract.event_type.rent_change", language)}</Select.Option>
            <Select.Option value="renewal">{t("contract.event_type.renewal", language)}</Select.Option>
            <Select.Option value="early_termination">{t("contract.event_type.early_termination", language)}</Select.Option>
            <Select.Option value="index_update">{t("contract.event_type.index_update", language)}</Select.Option>
            <Select.Option value="discount_rate_change">{t("contract.event_type.discount_rate_change", language)}</Select.Option>
            <Select.Option value="impairment">{t("contract.event_type.impairment", language)}</Select.Option>
          </Select>
        </Form.Item>

        <Form.Item
          label={t("contract.effective_date", language)}
          name="effective_date"
          rules={[{ required: true, message: t("contract_detail.validation.effective_date", language) }]}
        >
          <DatePicker style={{ width: "100%" }} />
        </Form.Item>

        <Row gutter={16}>
          <Col span={12}>
            <Form.Item label={t("contract.original_value", language)} name="original_value">
              <Input placeholder={t("contract_detail.original_value_placeholder", language)} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item label={t("contract.new_value", language)} name="new_value">
              <Input placeholder={t("contract_detail.new_value_placeholder", language)} />
            </Form.Item>
          </Col>
        </Row>

        <Form.Item
          label={t("contract.change_reason", language)}
          name="change_reason"
          rules={[{ required: true, message: t("contract_detail.validation.change_reason", language) }]}
        >
          <Input.TextArea rows={2} placeholder={t("contract.change_reason_placeholder", language)} />
        </Form.Item>

        <Form.Item label={t("contract.judgment_basis", language)} name="judgment_basis">
          <Input.TextArea rows={2} placeholder={t("contract.judgment_basis_placeholder", language)} />
        </Form.Item>
      </Form>
    </Modal>
  );
}

export function EditContractModal({
  open,
  loading,
  form,
  language,
  onCancel,
  onSubmit,
}: {
  open: boolean;
  loading: boolean;
  form: FormInstance<ContractUpdateFormValues>;
  language: Language;
  onCancel: () => void;
  onSubmit: (values: ContractUpdateFormValues) => void;
}) {
  return (
    <Modal
      title={t("contract.edit_contract", language)}
      open={open}
      onCancel={onCancel}
      onOk={() => form.submit()}
      confirmLoading={loading}
      width={700}
    >
      <Form form={form} layout="vertical" onFinish={onSubmit}>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item
              label={t("contract.contract_number", language)}
              name="contract_number"
              rules={[{ required: true, message: t("contract_detail.validation.contract_number", language) }]}
            >
              <Input />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              label={t("contracts.col_name", language)}
              name="contract_name"
              rules={[{ required: true, message: t("contract_detail.validation.contract_name", language) }]}
            >
              <Input />
            </Form.Item>
          </Col>
        </Row>

        <Row gutter={16}>
          <Col span={12}>
            <Form.Item label={t("contract.lessee", language)} name="lessee_name">
              <Input />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item label={t("contract.lessor", language)} name="lessor_name">
              <Input />
            </Form.Item>
          </Col>
        </Row>

        <Row gutter={16}>
          <Col span={12}>
            <Form.Item label={t("contract.store_name", language)} name="store_name">
              <Input />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item label={t("contract.store_address", language)} name="store_address">
              <Input />
            </Form.Item>
          </Col>
        </Row>

        <Row gutter={16}>
          <Col span={12}>
            <Form.Item
              label={t("contract.currency", language)}
              name="currency"
              rules={[{ required: true, message: t("contract_detail.validation.currency", language) }]}
            >
              <Select>
                <Select.Option value="CNY">{t("contract.currency_cny", language)}</Select.Option>
                <Select.Option value="USD">{t("contract.currency_usd", language)}</Select.Option>
              </Select>
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item label={t("contract.signing_date", language)} name="signing_date">
              <DatePicker style={{ width: "100%" }} />
            </Form.Item>
          </Col>
        </Row>

        <Form.Item label="资产类型" name="asset_type">
          <Select>
            <Select.Option value="real_estate">不动产</Select.Option>
            <Select.Option value="vehicle">车辆</Select.Option>
            <Select.Option value="it_equipment">IT 设备</Select.Option>
            <Select.Option value="machinery">机器设备</Select.Option>
            <Select.Option value="other">其他</Select.Option>
          </Select>
        </Form.Item>

        <Row gutter={16}>
          <Col span={12}>
            <Form.Item
              label={t("contract.commencement_date", language)}
              name="commencement_date"
              rules={[{ required: true, message: t("contract_detail.validation.commencement_date", language) }]}
            >
              <DatePicker style={{ width: "100%" }} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item
              label={t("contract.lease_start_date", language)}
              name="lease_start_date"
              rules={[{ required: true, message: t("contract_detail.validation.lease_start_date", language) }]}
            >
              <DatePicker style={{ width: "100%" }} />
            </Form.Item>
          </Col>
        </Row>

        <Row gutter={16}>
          <Col span={12}>
            <Form.Item
              label={t("contract.lease_end_date_label", language)}
              name="lease_end_date"
              rules={[{ required: true, message: t("contract_detail.validation.lease_end_date", language) }]}
            >
              <DatePicker style={{ width: "100%" }} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item label={t("contract.discount_rate_type", language)} name="discount_rate_type">
              <Input placeholder={t("contract.discount_rate_type_placeholder", language)} />
            </Form.Item>
          </Col>
        </Row>

        <Row gutter={16}>
          <Col span={12}>
            <Form.Item label={t("contract.discount_rate_version", language)} name="discount_rate_version">
              <Input placeholder={t("contract.discount_rate_version_placeholder", language)} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item label="IFRS 16 范围" name="lease_scope" rules={[{ required: true, message: "请选择 IFRS 16 范围" }]}>
              <Select>
                <Select.Option value="in_scope">资本化租赁</Select.Option>
                <Select.Option value="short_term_exempt">短期租赁豁免</Select.Option>
                <Select.Option value="low_value_exempt">低价值资产豁免</Select.Option>
                <Select.Option value="not_a_lease">非租赁合同</Select.Option>
              </Select>
            </Form.Item>
          </Col>
        </Row>

        <Form.Item label="豁免/排除依据" name="exemption_reason">
          <Input.TextArea rows={2} placeholder="例如：租期 10 个月且无续租意图；未识别特定资产；低价值 IT 设备" />
        </Form.Item>

        <Row gutter={16}>
          <Col span={24}>
            <Form.Item
              label={t("contract.tags", language)}
              name="tags"
              tooltip={t("contract.tags_tooltip", language)}
            >
              <Select
                mode="tags"
                tokenSeparators={[",", "，", ";", "；", " ", "|"]}
                placeholder={t("contract.tags_placeholder", language)}
                options={DEFAULT_TAG_SUGGESTIONS.map((tag) => ({
                  value: tag,
                  label: tag,
                }))}
              />
            </Form.Item>
          </Col>
        </Row>
      </Form>
    </Modal>
  );
}
