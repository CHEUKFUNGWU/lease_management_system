"use client";

import { StatusTag } from "../../../components/StatusTag";

import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Col,
  DatePicker,
  Descriptions,
  Divider,
  Form,
  Input,
  InputNumber,
  Modal,
  Row,
  Select,
  Table,
  Tag,
} from "antd";
import type { FormInstance } from "antd";
import dayjs from "dayjs";

import type { Language } from "../../../lib/i18n";
import { t } from "../../../lib/i18n";
import { fmtMoney } from "../../../lib/format";
import { DEFAULT_TAG_SUGGESTIONS } from "../../../lib/tags";
import { eventApi } from "../../../lib/api";
import { useAuth } from "../../../context/AuthContext";
import { buildRevisionParameters } from "./forms";

import type {
  ContractWorkspaceState,
} from "./types";
import type {
  WorkspaceAction,
  WorkspaceCommand,
} from "./workspace";

// The clause kinds the derivation engine understands, named the way a lease
// states them rather than the way the engine spells them.
const CLAUSE_KINDS = [
  { value: "percentage", label: "固定比例调升/调降" },
  { value: "index", label: "指数联动（CPI）" },
  { value: "stepped", label: "阶梯租金" },
  { value: "set_amount", label: "直接指定新租金" },
];

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
    contract,
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
  const { token } = useAuth();

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
  // The amount input sits next to the currency selector, so its prefix follows
  // that selection instead of asserting yuan for every schedule line.
  const scheduleCurrency = Form.useWatch("currency", form);

  // The clause fields shown depend on the kind of clause being recorded: a CPI
  // review and a stepped ladder need different things said about them.
  const clauseKind = Form.useWatch("revision_kind", eventForm);
  const [clauseDraft, setClauseDraft] = useState<any>(null);
  const [clauseDraftLoading, setClauseDraftLoading] = useState(false);
  const [clauseDraftError, setClauseDraftError] = useState<string | null>(null);

  // A draft derived from earlier terms would misdescribe the current ones, so
  // changing the clause type clears it rather than leaving it on screen.
  useEffect(() => {
    setClauseDraft(null);
    setClauseDraftError(null);
  }, [clauseKind]);

  const handlePreviewPayments = async () => {
    if (!token || !contract) return;
    const values = eventForm.getFieldsValue();
    const effectiveDate = values.revision_applies_from || values.effective_date;
    if (!effectiveDate) {
      setClauseDraftError("请先填写生效日期或条款起始日");
      return;
    }
    const clause = buildRevisionParameters(values);
    if (!clause) {
      setClauseDraftError("请先选择条款类型");
      return;
    }

    setClauseDraftLoading(true);
    setClauseDraftError(null);
    try {
      const response = await eventApi.previewPayments(
        contract.id,
        {
          effective_date: effectiveDate.format ? effectiveDate.format("YYYY-MM-DD") : effectiveDate,
          revision_parameters: clause,
        },
        token
      );
      setClauseDraft(response.draft);
    } catch (error: any) {
      // The message comes from the engine and names what is wrong with the
      // clause, which is more use than a generic failure notice.
      setClauseDraftError(error?.message || "推导失败");
      setClauseDraft(null);
    } finally {
      setClauseDraftLoading(false);
    }
  };

  const adjustmentModalData = adjustment?.data as any;
  const adjustmentCurrency = contract?.currency;
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
                  <DatePicker className="sty-70ea3314" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label={t("contract.amount", language)}
                  name="amount"
                  rules={[{ required: true, message: t("contract_detail.validation.amount", language) }]}
                >
                  <InputNumber
                    className="sty-70ea3314"
                    min={0}
                    precision={2}
                    prefix={scheduleCurrency || ""}
                  />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label={t("contract.currency", language)} name="currency">
                  <Input placeholder="CNY" />
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
                  <DatePicker className="sty-70ea3314" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label={t("contract.effective_end_date", language)}
                  name="effective_end_date"
                >
                  <DatePicker className="sty-70ea3314" />
                </Form.Item>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label={t("contract.lease_end_date_label", language)}
                  name="lease_end_date"
                  rules={[{ required: true, message: t("contract_detail.validation.lease_end_date", language) }]}
                >
                  <DatePicker className="sty-70ea3314" />
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
                  <InputNumber className="sty-70ea3314" min={0} step={0.01} placeholder={t("contract_detail.discount_rate_placeholder", language)} />
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
              <DatePicker className="sty-4ffb9b7c" />
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

            {/* The clause replaces editing the payment schedule by hand. Stating
                it here means the event and the revised rent can no longer
                disagree, because one is derived from the other. */}
            <Divider className="sty-b8bb6f7b">
              <span className="sty-70ea3314">调租条款（可选）</span>
            </Divider>

            <Form.Item
              label="条款类型"
              name="revision_kind"
              extra="填写后，系统据此推导修订付款流；留空则沿用手工维护的付款计划。"
            >
              <Select allowClear placeholder="不按条款推导" options={CLAUSE_KINDS} />
            </Form.Item>

            {clauseKind === "set_amount" && (
              <Form.Item label="调整后租金" name="revision_amount" rules={[{ required: true, message: "请填写调整后租金" }]}>
                <InputNumber className="sty-70ea3314" min={0} precision={2} prefix={contract?.currency || ""} />
              </Form.Item>
            )}

            {clauseKind === "percentage" && (
              <Form.Item
                label="涨跌幅（%）"
                name="revision_percentage"
                rules={[{ required: true, message: "请填写涨跌幅" }]}
                extra="正数为上调，负数为下调。例如装修期减租填 -50。"
              >
                <InputNumber className="sty-70ea3314" precision={4} />
              </Form.Item>
            )}

            {clauseKind === "index" && (
              <>
                <Row gutter={16}>
                  <Col span={12}>
                    <Form.Item label="基期指数" name="revision_base_index" rules={[{ required: true, message: "请填写基期指数" }]}>
                      <InputNumber className="sty-70ea3314" min={0} precision={4} />
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item label="现期指数" name="revision_new_index" rules={[{ required: true, message: "请填写现期指数" }]}>
                      <InputNumber className="sty-70ea3314" min={0} precision={4} />
                    </Form.Item>
                  </Col>
                </Row>
                <Row gutter={16}>
                  <Col span={12}>
                    <Form.Item label="封顶涨幅（%）" name="revision_cap" extra="留空表示不封顶">
                      <InputNumber className="sty-70ea3314" precision={4} />
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item label="保底涨幅（%）" name="revision_floor" extra="留空表示无下限">
                      <InputNumber className="sty-7f21e1ba" precision={4} />
                    </Form.Item>
                  </Col>
                </Row>
              </>
            )}

            {clauseKind === "stepped" && (
              <Form.List name="revision_steps">
                {(fields, { add, remove }) => (
                  <div className="sty-5822ad35">
                    {fields.map((field) => (
                      <Row gutter={8} key={field.key} className="sty-70ea3314">
                        <Col span={11}>
                          <Form.Item
                            {...field}
                            key={`${field.key}-date`}
                            name={[field.name, "from_date"]}
                            noStyle
                          >
                            <DatePicker className="sty-70ea3314" placeholder="自该日起" />
                          </Form.Item>
                        </Col>
                        <Col span={10}>
                          <Form.Item
                            {...field}
                            key={`${field.key}-amount`}
                            name={[field.name, "amount"]}
                            noStyle
                          >
                            <InputNumber className="sty-70ea3314" min={0} precision={2} placeholder="租金" />
                          </Form.Item>
                        </Col>
                        <Col span={3}>
                          <Button danger size="small" onClick={() => remove(field.name)} className="sty-70ea3314">
                            删除
                          </Button>
                        </Col>
                      </Row>
                    ))}
                    <Button size="small" onClick={() => add()} className="sty-70ea3314">
                      添加一级阶梯
                    </Button>
                  </div>
                )}
              </Form.List>
            )}

            {clauseKind && (
              <>
                <Row gutter={16}>
                  <Col span={12}>
                    <Form.Item label="条款起始日" name="revision_applies_from" extra="留空则取生效日">
                      <DatePicker className="sty-70ea3314" />
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item label="条款结束日" name="revision_applies_to" extra="留空则至租期结束">
                      <DatePicker className="sty-37c08d18" />
                    </Form.Item>
                  </Col>
                </Row>

                <Button
                  onClick={handlePreviewPayments}
                  loading={clauseDraftLoading}
                  className="sty-37c08d18"
                  block
                >
                  推导修订付款流
                </Button>

                {clauseDraftError && (
                  <Alert type="error" showIcon message={clauseDraftError} className="sty-37c08d18" />
                )}

                {clauseDraft && (
                  <>
                    <Alert
                      type="info"
                      showIcon
                      className="sty-a8f26b1e"
                      message={
                        <span>
                          {clauseDraft.changed_count} 笔付款变动，合计{" "}
                          <strong>{fmtMoney(clauseDraft.delta, contract?.currency)}</strong>
                          {clauseDraft.cap_applied && "（封顶已生效）"}
                          {clauseDraft.floor_applied && "（保底已生效）"}
                        </span>
                      }
                      description={
                        clauseKind === "index" || clauseKind === "percentage"
                          ? `实际采用系数 ${clauseDraft.applied_factor.toFixed(6)}`
                          : undefined
                      }
                    />
                    <Table
                      size="small"
                      rowKey={(row: any) => `${row.date}-${row.type}-${row.original_amount}`}
                      dataSource={clauseDraft.changes.filter((row: any) => row.changed)}
                      pagination={{ pageSize: 6, size: "small" }}
                      columns={[
                        {
                          title: "付款日",
                          dataIndex: "date",
                          render: (value: string) => dayjs(value).format("YYYY-MM-DD"),
                        },
                        {
                          title: "原金额",
                          dataIndex: "original_amount",
                          align: "right" as const,
                          render: (value: number) => fmtMoney(value, contract?.currency),
                        },
                        {
                          title: "新金额",
                          dataIndex: "revised_amount",
                          align: "right" as const,
                          render: (value: number) => fmtMoney(value, contract?.currency),
                        },
                        {
                          title: "差额",
                          dataIndex: "delta",
                          align: "right" as const,
                          render: (value: number) => (
                            <span className="sty-70ea3314">
                              {fmtMoney(value, contract?.currency)}
                            </span>
                          ),
                        },
                      ]}
                    />
                  </>
                )}
              </>
            )}
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
                  <Input placeholder="CNY" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label={t("contract.signing_date", language)} name="signing_date">
                  <DatePicker className="sty-70ea3314" />
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
                  <DatePicker className="sty-70ea3314" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item
                  label={t("contract.lease_start_date", language)}
                  name="lease_start_date"
                  rules={[{ required: true, message: t("contract_detail.validation.lease_start_date", language) }]}
                >
                  <DatePicker className="sty-70ea3314" />
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
                  <DatePicker className="sty-70ea3314" />
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
                  <DatePicker className="sty-70ea3314" />
                </Form.Item>
              </Col>
            </Row>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item label="提前提醒天数" name="reminder_days">
                  <InputNumber min={0} max={365} className="sty-70ea3314" />
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
                  <InputNumber min={1} className="sty-5822ad35" />
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
          <p className="sty-5822ad35">
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
          <p className="sty-a8f26b1e">
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
                <StatusTag kind="processing">{adjustmentModalData.adjustment_type || "-"}</StatusTag>
              </Descriptions.Item>
              <Descriptions.Item label={t("contract.effective_date", language)}>
                {adjustmentModalData.effective_date ? dayjs(adjustmentModalData.effective_date).format("YYYY-MM-DD") : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract.discount_rate", language)}>
                {adjustmentModalData.discount_rate != null ? `${(adjustmentModalData.discount_rate * 100).toFixed(2)}%` : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract_detail.liability_before", language)}>
                {adjustmentModalData.liability_before != null ? fmtMoney(adjustmentModalData.liability_before, adjustmentCurrency) : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract_detail.liability_after", language)}>
                {adjustmentModalData.liability_after != null ? (
                  <span className="sty-a8f26b1e">{fmtMoney(adjustmentModalData.liability_after, adjustmentCurrency)}</span>
                ) : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract_detail.liability_change", language)}>
                {adjustmentModalData.liability_change != null ? (
                  <span className="sty-a8f26b1e">
                    {adjustmentModalData.liability_change >= 0 ? "+" : ""}
                    {fmtMoney(adjustmentModalData.liability_change, adjustmentCurrency)}
                  </span>
                ) : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract_detail.asset_before", language)}>
                {adjustmentModalData.asset_before != null ? fmtMoney(adjustmentModalData.asset_before, adjustmentCurrency) : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract_detail.asset_after", language)}>
                {adjustmentModalData.asset_after != null ? (
                  <span className="sty-a8f26b1e">{fmtMoney(adjustmentModalData.asset_after, adjustmentCurrency)}</span>
                ) : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract_detail.asset_change", language)}>
                {adjustmentModalData.asset_change != null ? (
                  <span style={{ fontWeight: "bold", color: adjustmentModalData.asset_change >= 0 ? "var(--fg-primary)" : "var(--fg-primary)" }}>
                    {adjustmentModalData.asset_change >= 0 ? "+" : ""}
                    {fmtMoney(adjustmentModalData.asset_change, adjustmentCurrency)}
                  </span>
                ) : "-"}
              </Descriptions.Item>
              <Descriptions.Item label={t("contract_detail.pnl_impact", language)}>
                {adjustmentModalData.pnl_impact != null ? (
                  <span style={{ fontWeight: "bold", color: adjustmentModalData.pnl_impact >= 0 ? "var(--fg-primary)" : "var(--fg-primary)" }}>
                    {adjustmentModalData.pnl_impact >= 0 ? "+" : ""}
                    {fmtMoney(adjustmentModalData.pnl_impact, adjustmentCurrency)}
                  </span>
                ) : "-"}
              </Descriptions.Item>
            </Descriptions>
          )}
        </Modal>
    </>
  );
}
