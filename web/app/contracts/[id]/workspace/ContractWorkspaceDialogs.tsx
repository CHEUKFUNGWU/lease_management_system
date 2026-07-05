"use client";

import {
  Button,
  Col,
  DatePicker,
  Descriptions,
  Form,
  Input,
  InputNumber,
  Modal,
  Row,
  Select,
  Tag,
} from "antd";
import type { FormInstance } from "antd";
import dayjs from "dayjs";

import type { Language } from "../../../lib/i18n";
import { t } from "../../../lib/i18n";
import { DEFAULT_TAG_SUGGESTIONS } from "../../../lib/tags";
import type {
  ContractWorkspaceState,
} from "./types";
import type {
  WorkspaceAction,
  WorkspaceCommand,
} from "./workspace";

interface ContractWorkspaceDialogsProps {
  language: Language;
  state: ContractWorkspaceState;
  dispatch: (action: WorkspaceAction) => void;
  execute: (command: WorkspaceCommand) => Promise<boolean>;
  forms: {
    schedule: FormInstance;
    event: FormInstance;
    edit: FormInstance;
    criticalDate: FormInstance;
    document: FormInstance;
    obligation: FormInstance;
  };
}
export function ContractWorkspaceDialogs({
  language,
  state,
  dispatch,
  execute,
  forms,
}: ContractWorkspaceDialogsProps) {
  const {
    dialogs,
    contractRejection,
    eventRejection,
    adjustment,
    loading,
  } = state;
  const {
    schedule: form,
    event: eventForm,
    edit: editForm,
    criticalDate: criticalDateForm,
    document: documentForm,
    obligation: obligationForm,
  } = forms;

  const setDialog = (dialog: keyof typeof dialogs, open: boolean) =>
    dispatch({ type: open ? "dialog.open" : "dialog.close", dialog });
  const setModalOpen = (open: boolean) => setDialog("schedule", open);
  const setEventModalOpen = (open: boolean) => setDialog("event", open);
  const setEditModalOpen = (open: boolean) => setDialog("contractEdit", open);
  const setCriticalDateModalOpen = (open: boolean) => setDialog("criticalDate", open);
  const setDocumentModalOpen = (open: boolean) => setDialog("document", open);
  const setObligationModalOpen = (open: boolean) => setDialog("obligation", open);
  const setRejectModalOpen = (open: boolean) => setDialog("contractReject", open);
  const setEventRejectModalOpen = (open: boolean) => setDialog("eventReject", open);
  const setAdjustmentModalOpen = (open: boolean) => setDialog("adjustment", open);
  const setRejectReason = (reason: string) =>
    dispatch({ type: "contract.reject.reason", reason });
  const setEventRejectReason = (reason: string) =>
    dispatch({ type: "event.reject.reason", reason });

  const handleCreateSchedule = async (values: Record<string, any>) => {
    if (await execute({ type: "schedule.create", values })) form.resetFields();
  };
  const handleCreateEvent = async (values: Record<string, any>) => {
    if (await execute({ type: "event.create", values })) eventForm.resetFields();
  };
  const handleUpdate = (values: Record<string, any>) =>
    execute({ type: "contract.update", values });
  const handleCreateCriticalDate = async (values: Record<string, any>) => {
    if (await execute({ type: "criticalDate.create", values })) criticalDateForm.resetFields();
  };
  const handleCreateDocument = async (values: Record<string, any>) => {
    if (await execute({ type: "document.create", values })) documentForm.resetFields();
  };
  const handleCreateObligation = async (values: Record<string, any>) => {
    if (await execute({ type: "obligation.create", values })) obligationForm.resetFields();
  };
  const handleRejectSubmit = () => execute({ type: "contract.reject" });
  const handleEventRejectSubmit = () => execute({ type: "event.reject" });

  const modalOpen = dialogs.schedule;
  const eventModalOpen = dialogs.event;
  const editModalOpen = dialogs.contractEdit;
  const criticalDateModalOpen = dialogs.criticalDate;
  const documentModalOpen = dialogs.document;
  const obligationModalOpen = dialogs.obligation;
  const rejectModalOpen = dialogs.contractReject;
  const eventRejectModalOpen = dialogs.eventReject;
  const adjustmentModalOpen = dialogs.adjustment;
  const rejectModalType = contractRejection.stage;
  const rejectReason = contractRejection.reason;
  const eventRejectType = eventRejection.stage;
  const eventRejectReason = eventRejection.reason;
  const eventRejectEventId = eventRejection.eventId;
  const adjustmentModalData = adjustment?.data as any;
  const adjustmentModalTitle = adjustment ? t(adjustment.title, language) : "";

  const actionLoadingAliases: Record<string, string> = {
    "contract.review.reject": "review_reject",
    "contract.reject": "approve_reject",
  };
  const actionLoading = loading.command
    ? actionLoadingAliases[loading.command] || loading.command
    : null;
  const eventActionLoading = loading.eventCommand;
  const eventLoading = loading.command === "event.create";
  const editLoading = loading.command === "contract.update";
  const criticalDateLoading = loading.command === "criticalDate.create";
  const documentLoading = loading.command === "document.create";
  const obligationLoading = loading.command === "obligation.create";

  return (
    <>
        <Modal
          title={t("contract.add_payment", language)}
          open={modalOpen}
          onCancel={() => setModalOpen(false)}
          onOk={() => form.submit()}
          width={700}
        >
          <Form
            form={form}
            layout="vertical"
            onFinish={handleCreateSchedule}
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
                    <Select.Option value="prepaid">
                      {t("contract.prepaid_label", language)}
                    </Select.Option>
                    <Select.Option value="postpaid">
                      {t("contract.postpaid_label", language)}
                    </Select.Option>
                  </Select>
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  label={t("contract.effective_start_date", language)}
                  name="effective_start_date"
                >
                  <DatePicker style={{ width: "100%" }} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label={t("contract.effective_end_date", language)}
                  name="effective_end_date"
                >
                  <DatePicker style={{ width: "100%" }} />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label={t("contract.lease_end_date_label", language)}
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
                <Form.Item
                  label={t("contract.discount_rate_value", language)}
                  name="discount_rate_value"
                  help={t("contract.discount_rate_help", language)}
                >
                  <InputNumber style={{ width: "100%" }} min={0} step={0.01} placeholder={t("contract_detail.discount_rate_placeholder", language)} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label={t("contract.discount_rate_version", language)} name="discount_rate_version">
                  <Input placeholder={t("contract.discount_rate_version_placeholder", language)} />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
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

            <Form.Item label={t("contract.amount_type", language)} name="amount_type">
              <Input placeholder={t("contract.amount_type_placeholder", language)} />
            </Form.Item>

            <Row gutter={16}>
              <Col span={8}>
                <Form.Item
                  label={t("contract.is_fixed", language)}
                  name="is_fixed"
                  valuePropName="checked"
                >
                  <Select>
                    <Select.Option value={true}>{t("contract.yes", language)}</Select.Option>
                    <Select.Option value={false}>{t("contract.no", language)}</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item
                  label={t("contract.is_lease_component", language)}
                  name="is_lease_component"
                  valuePropName="checked"
                >
                  <Select>
                    <Select.Option value={true}>{t("contract.yes", language)}</Select.Option>
                    <Select.Option value={false}>{t("contract.no", language)}</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item
                  label={t("contract.included_in_liability_pv", language)}
                  name="included_in_liability_pv"
                  valuePropName="checked"
                >
                  <Select>
                    <Select.Option value={true}>{t("contract.yes", language)}</Select.Option>
                    <Select.Option value={false}>{t("contract.no", language)}</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
            </Row>
          </Form>
        </Modal>

        <Modal
          title={t("contract.register_event", language)}
          open={eventModalOpen}
          onCancel={() => setEventModalOpen(false)}
          onOk={() => eventForm.submit()}
          confirmLoading={eventLoading}
          width={600}
        >
          <Form
            form={eventForm}
            layout="vertical"
            onFinish={handleCreateEvent}
          >
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

        <Modal
          title={t("contract.edit_contract", language)}
          open={editModalOpen}
          onCancel={() => setEditModalOpen(false)}
          onOk={() => editForm.submit()}
          confirmLoading={editLoading}
          width={700}
        >
          <Form
            form={editForm}
            layout="vertical"
            onFinish={handleUpdate}
          >
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

        <Modal
          title="新增关键日期"
          open={criticalDateModalOpen}
          onCancel={() => setCriticalDateModalOpen(false)}
          onOk={() => criticalDateForm.submit()}
          confirmLoading={criticalDateLoading}
          width={600}
        >
          <Form
            form={criticalDateForm}
            layout="vertical"
            onFinish={handleCreateCriticalDate}
            initialValues={{ date_type: "renewal_deadline", reminder_days: 30 }}
          >
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label="类型" name="date_type" rules={[{ required: true, message: "请选择类型" }]}>
                  <Select>
                    <Select.Option value="renewal_deadline">续租截止</Select.Option>
                    <Select.Option value="break_notice">Break 通知</Select.Option>
                    <Select.Option value="rent_review">租金 Review</Select.Option>
                    <Select.Option value="lease_expiry">租约到期</Select.Option>
                    <Select.Option value="insurance_renewal">保险续保</Select.Option>
                    <Select.Option value="other">其他</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="目标日期" name="target_date" rules={[{ required: true, message: "请选择目标日期" }]}>
                  <DatePicker style={{ width: "100%" }} />
                </Form.Item>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label="提前提醒天数" name="reminder_days">
                  <InputNumber min={0} max={365} style={{ width: "100%" }} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="标题" name="title" rules={[{ required: true, message: "请输入标题" }]}>
                  <Input placeholder="例如：续租通知截止日" />
                </Form.Item>
              </Col>
            </Row>
            <Form.Item label="说明" name="description">
              <Input.TextArea rows={3} placeholder="记录条款依据、通知期、责任人或操作建议" />
            </Form.Item>
          </Form>
        </Modal>

        <Modal
          title="新增文档记录"
          open={documentModalOpen}
          onCancel={() => setDocumentModalOpen(false)}
          onOk={() => documentForm.submit()}
          confirmLoading={documentLoading}
          width={600}
        >
          <Form
            form={documentForm}
            layout="vertical"
            onFinish={handleCreateDocument}
            initialValues={{ document_type: "main_contract" }}
          >
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label="文档类型" name="document_type">
                  <Select>
                    <Select.Option value="main_contract">主合同</Select.Option>
                    <Select.Option value="amendment">补充协议</Select.Option>
                    <Select.Option value="side_letter">Side Letter</Select.Option>
                    <Select.Option value="invoice">发票/账单</Select.Option>
                    <Select.Option value="other">其他</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="版本" name="document_version">
                  <Input placeholder="例如：v1 / 2024-签署版" />
                </Form.Item>
              </Col>
            </Row>
            <Form.Item label="文件名" name="file_name" rules={[{ required: true, message: "请输入文件名" }]}>
              <Input placeholder="例如：LEASE-2024-001 主合同.pdf" />
            </Form.Item>
            <Form.Item label="文件类型" name="file_type">
              <Input placeholder="application/pdf" />
            </Form.Item>
            <Form.Item label="备注" name="notes">
              <Input.TextArea rows={3} placeholder="记录文件来源、关键条款页码或归档说明" />
            </Form.Item>
          </Form>
        </Modal>

        <Modal
          title="新增条款义务"
          open={obligationModalOpen}
          onCancel={() => setObligationModalOpen(false)}
          onOk={() => obligationForm.submit()}
          confirmLoading={obligationLoading}
          width={720}
        >
          <Form
            form={obligationForm}
            layout="vertical"
            onFinish={handleCreateObligation}
            initialValues={{ obligation_type: "notice", responsible_party: "lessee" }}
          >
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label="义务类型" name="obligation_type" rules={[{ required: true, message: "请选择类型" }]}>
                  <Select>
                    <Select.Option value="maintenance">维修维护</Select.Option>
                    <Select.Option value="cam">CAM / 管理费</Select.Option>
                    <Select.Option value="insurance">保险</Select.Option>
                    <Select.Option value="index_adjustment">指数调整</Select.Option>
                    <Select.Option value="restoration">复原义务</Select.Option>
                    <Select.Option value="security_deposit">押金</Select.Option>
                    <Select.Option value="notice">通知义务</Select.Option>
                    <Select.Option value="other">其他</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="责任方" name="responsible_party" rules={[{ required: true, message: "请选择责任方" }]}>
                  <Select>
                    <Select.Option value="lessee">承租方</Select.Option>
                    <Select.Option value="lessor">出租方</Select.Option>
                    <Select.Option value="shared">双方共同</Select.Option>
                    <Select.Option value="third_party">第三方</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
            </Row>
            <Form.Item label="标题" name="title" rules={[{ required: true, message: "请输入标题" }]}>
              <Input placeholder="例如：提前 6 个月提交续租通知" />
            </Form.Item>
            <Form.Item label="说明" name="description">
              <Input.TextArea rows={3} placeholder="记录义务内容、触发条件、管理动作或财务影响" />
            </Form.Item>
            <Row gutter={16}>
              <Col span={8}>
                <Form.Item label="来源页码" name="source_page">
                  <InputNumber min={1} style={{ width: "100%" }} />
                </Form.Item>
              </Col>
              <Col span={16}>
                <Form.Item label="原文条款摘录" name="source_clause">
                  <Input placeholder="粘贴合同条款原文，便于审计追溯" />
                </Form.Item>
              </Col>
            </Row>
          </Form>
        </Modal>

        <Modal
          title={rejectModalType === 'review' ? t("contract.review_reject_title", language) : t("contract.approve_reject_title", language)}
          open={rejectModalOpen}
          onCancel={() => setRejectModalOpen(false)}
          onOk={handleRejectSubmit}
          confirmLoading={actionLoading === (rejectModalType === 'review' ? 'review_reject' : 'approve_reject')}
          okText={t("contract.ok", language)}
          cancelText={t("contract.cancel", language)}
        >
          <p style={{ marginBottom: 8 }}>
            {rejectModalType === 'review' ? t("contract.review_reject_reason", language) : t("contract.approve_reject_reason", language)}
          </p>
          <Input.TextArea
            rows={4}
            value={rejectReason}
            onChange={(e) => setRejectReason(e.target.value)}
            placeholder={t("contract.reason_placeholder", language)}
          />
        </Modal>

        <Modal
          title={eventRejectType === 'review' ? t("contract.review_reject_title", language) : t("contract.approve_reject_title", language)}
          open={eventRejectModalOpen}
          onCancel={() => setEventRejectModalOpen(false)}
          onOk={handleEventRejectSubmit}
          confirmLoading={eventRejectEventId ? eventActionLoading === eventRejectEventId + '_reject' : false}
          okText={t("contract.ok", language)}
          cancelText={t("contract.cancel", language)}
        >
          <p style={{ marginBottom: 8 }}>
            {eventRejectType === 'review' ? t("contract.review_reject_reason", language) : t("contract.approve_reject_reason", language)}
          </p>
          <Input.TextArea
            rows={4}
            value={eventRejectReason}
            onChange={(e) => setEventRejectReason(e.target.value)}
            placeholder={t("contract.reason_placeholder", language)}
          />
        </Modal>

        <Modal
          title={adjustmentModalTitle}
          open={adjustmentModalOpen}
          onCancel={() => setAdjustmentModalOpen(false)}
          footer={[
            <Button key="close" onClick={() => setAdjustmentModalOpen(false)}>{t("contract.close", language)}</Button>,
          ]}
          width={700}
        >
          {adjustmentModalData && (
            <Descriptions column={1} bordered size="small">
              <Descriptions.Item label={t("contract_detail.adjustment_type_label", language)}>
                <Tag color="processing">{adjustmentModalData.adjustment_type || "-"}</Tag>
              </Descriptions.Item>
              <Descriptions.Item label={t("contract.effective_date", language)}>
                {adjustmentModalData.effective_date ? dayjs(adjustmentModalData.effective_date).format("YYYY-MM-DD") : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract.discount_rate", language)}>
                {adjustmentModalData.discount_rate != null ? `${(adjustmentModalData.discount_rate * 100).toFixed(2)}%` : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract_detail.liability_before", language)}>
                {adjustmentModalData.liability_before != null ? `¥${adjustmentModalData.liability_before.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}` : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract_detail.liability_after", language)}>
                {adjustmentModalData.liability_after != null ? (
                  <span style={{ fontWeight: "bold", color: "#000" }}>¥{adjustmentModalData.liability_after.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}</span>
                ) : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract_detail.liability_change", language)}>
                {adjustmentModalData.liability_change != null ? (
                  <span style={{ fontWeight: "bold", color: adjustmentModalData.liability_change >= 0 ? "#000" : "#000" }}>
                    {adjustmentModalData.liability_change >= 0 ? "+" : ""}¥{adjustmentModalData.liability_change.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                  </span>
                ) : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract_detail.asset_before", language)}>
                {adjustmentModalData.asset_before != null ? `¥${adjustmentModalData.asset_before.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}` : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract_detail.asset_after", language)}>
                {adjustmentModalData.asset_after != null ? (
                  <span style={{ fontWeight: "bold", color: "#000" }}>¥{adjustmentModalData.asset_after.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}</span>
                ) : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract_detail.asset_change", language)}>
                {adjustmentModalData.asset_change != null ? (
                  <span style={{ fontWeight: "bold", color: adjustmentModalData.asset_change >= 0 ? "#000" : "#000" }}>
                    {adjustmentModalData.asset_change >= 0 ? "+" : ""}¥{adjustmentModalData.asset_change.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                  </span>
                ) : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract_detail.pnl_impact", language)}>
                {adjustmentModalData.pnl_impact != null ? (
                  <span style={{ fontWeight: "bold", color: adjustmentModalData.pnl_impact >= 0 ? "#000" : "#000" }}>
                    {adjustmentModalData.pnl_impact >= 0 ? "+" : ""}¥{adjustmentModalData.pnl_impact.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}
                  </span>
                ) : "-"}
              </Descriptions.Item>
            </Descriptions>
          )}
        </Modal>
    </>
  );
}
