"use client";

import { useEffect, useState } from "react";
import {
  Card,
  Table,
  Button,
  Space,
  Modal,
  Form,
  Input,
  InputNumber,
  Select,
  Segmented,
  DatePicker,
  Row,
  Col,
  Statistic,
  Drawer,
  Alert,
  Tabs,
  Typography,
  Divider,
  message,
  Spin,
} from "antd";
import { StatusTag } from "../components/StatusTag";
import {
  PlusOutlined,
  DollarOutlined,
  LineChartOutlined,
  WarningOutlined,
  CheckCircleOutlined,
  AuditOutlined,
  ReloadOutlined,
  SendOutlined,
} from "@ant-design/icons";
import dayjs from "dayjs";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { useLanguage } from "../context/LanguageContext";
import { useAuth } from "../context/AuthContext";
import { t } from "../lib/i18n";
import { fmtMoney, fmtPct } from "../lib/format";
import { tableScrollX } from "../lib/tableScroll";
import {
  promotionApi,
  type Promotion,
  type PromotionCost,
  type PromotionROIResult,
  type PromotionBreakevenResult,
} from "../lib/api";
import { BreakevenPanel, buildBreakevenRequest } from "./BreakevenPanel";
import { PromotionROIReportPanel } from "./PromotionROIReportPanel";

const { Text, Title, Paragraph } = Typography;
const { Option } = Select;
const { RangePicker } = DatePicker;

const PROMOTION_TYPE_KEYS: Record<string, string> = {
  discount: "promotion.type_discount",
  coupon: "promotion.type_coupon",
  gift: "promotion.type_gift",
  member_day: "promotion.type_member_day",
  other: "promotion.type_other",
};
const PROMOTION_COST_KEYS: Record<string, string> = {
  subsidy: "promotion.cost_type_subsidy",
  materials: "promotion.cost_type_materials",
  labor: "promotion.cost_type_labor",
  marketing: "promotion.cost_type_marketing",
  other: "promotion.cost_type_other",
};

function promotionTypeLabel(type: string, language: Parameters<typeof t>[1]): string {
  return PROMOTION_TYPE_KEYS[type] ? t(PROMOTION_TYPE_KEYS[type], language) : t("promotion.type_unknown", language);
}

function promotionCostLabel(type: string, language: Parameters<typeof t>[1]): string {
  return PROMOTION_COST_KEYS[type] ? t(PROMOTION_COST_KEYS[type], language) : t("promotion.cost_type_unknown", language);
}

function promotionScopeLabel(scope: string | undefined, language: Parameters<typeof t>[1]): string {
  return scope === "all" ? t("promotion.scope_all", language) : scope ? t("promotion.scope_unknown", language) : "—";
}

export default function PromotionsPage() {
  const { language } = useLanguage();
  const { token, user } = useAuth();

  const [loading, setLoading] = useState(false);
  const [promotions, setPromotions] = useState<Promotion[]>([]);
  const [statusFilter, setStatusFilter] = useState<string>("");
  const [selectedPromo, setSelectedPromo] = useState<Promotion | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [createModalOpen, setCreateModalOpen] = useState(false);
  const [costModalOpen, setCostModalOpen] = useState(false);

  // ROI evaluation state
  const [roiLoading, setRoiLoading] = useState(false);
  const [roiResult, setRoiResult] = useState<PromotionROIResult | null>(null);
  const [costs, setCosts] = useState<PromotionCost[]>([]);

  // R2-1：投前保本测算状态（promo_id 模式，基线自动取活动前事实）
  const [breakevenLoading, setBreakevenLoading] = useState(false);
  const [breakevenRate, setBreakevenRate] = useState<number | null>(null);
  const [breakevenCost, setBreakevenCost] = useState<number | null>(null);
  const [breakevenResult, setBreakevenResult] = useState<PromotionBreakevenResult | null>(null);

  const runBreakeven = async () => {
    if (!token || !selectedPromo) return;
      const req = buildBreakevenRequest(selectedPromo.id, breakevenRate, breakevenCost);
      if (!req.valid) {
        message.warning(t("promotion.breakeven.missing_input", language));
        return;
      }
      setBreakevenLoading(true);
      try {
        const res = await promotionApi.evaluateBreakeven(
          { promo_id: req.promo_id, promo_margin_rate: req.promo_margin_rate, fixed_marketing_cost: req.fixed_marketing_cost },
          token
        );
      setBreakevenResult(res);
    } catch (err: unknown) {
      message.error((err as Error)?.message || t("promotion.breakeven.failed", language));
    } finally {
      setBreakevenLoading(false);
    }
  };

  const [form] = Form.useForm();
  const [costForm] = Form.useForm();

  const loadPromotions = async () => {
    if (!token) return;
    setLoading(true);
    try {
      const res = await promotionApi.list(token, statusFilter || undefined);
      setPromotions(res.promotions || []);
    } catch (err: any) {
      message.error(err?.message || t("promotion.msg_load_failed", language));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadPromotions();
  // eslint-disable-next-line react-hooks/exhaustive-deps -- P2-C gate close-out: legacy dep semantics kept as-is; loaders are rebuilt every render so adding them would loop refetches. useCallback refactor tracked separately; do not add new exemptions.
  }, [token, statusFilter]);

  const loadPromoDetails = async (promo: Promotion) => {
    if (!token) return;
    setSelectedPromo(promo);
    setDrawerOpen(true);
    setRoiLoading(true);
    try {
      const [roiData, costData] = await Promise.all([
        promotionApi.evaluateROI(promo.id, token),
        promotionApi.listCosts(promo.id, token),
      ]);
      setRoiResult(roiData);
      setCosts(costData.costs || []);
    } catch (err: any) {
      message.error(err?.message || t("promotion.msg_roi_failed", language));
    } finally {
      setRoiLoading(false);
    }
  };

  const handleCreatePromo = async (values: any) => {
    if (!token) return;
    try {
      const [startDate, endDate] = values.dates;
      await promotionApi.create(
        {
          promo_code: values.promo_code,
          name: values.name,
          promo_type: values.promo_type,
          start_date: startDate.format("YYYY-MM-DD"),
          end_date: endDate.format("YYYY-MM-DD"),
          target_scope: values.target_scope || "all",
          budget_amount: values.budget_amount || 0,
          currency: values.currency || "CNY",
          owner: values.owner || user?.username,
          description: values.description,
          approval_status: "draft",
        },
        token
      );
      message.success(t("promotion.msg_create_success", language));
      setCreateModalOpen(false);
      form.resetFields();
      loadPromotions();
    } catch (err: any) {
      message.error(err?.message || t("promotion.msg_create_failed", language));
    }
  };

  const handleAddCost = async (values: any) => {
    if (!token || !selectedPromo) return;
    try {
      await promotionApi.addCost(
        selectedPromo.id,
        {
          cost_category: values.cost_category,
          amount: values.amount,
          currency: selectedPromo.currency || "CNY",
          period: values.period || selectedPromo.start_date.substring(0, 7),
          notes: values.notes,
        },
        token
      );
      message.success(t("promotion.msg_cost_success", language));
      setCostModalOpen(false);
      costForm.resetFields();
      loadPromoDetails(selectedPromo);
    } catch (err: any) {
      message.error(err?.message || t("promotion.msg_cost_failed", language));
    }
  };

  const handleCreateActionItem = () => {
    if (!selectedPromo || !roiResult) return;
    const summary = `${t("promotion.action_item_prefix", language)} ${selectedPromo.name} (ROI: ${roiResult.roi != null ? (roiResult.roi * 100).toFixed(1) + "%" : "—"})\n` +
      `${t("promotion.code_label", language)}: ${selectedPromo.promo_code}\n` +
      `${t("promotion.inc_gross_profit_label", language)}: ${fmtMoney(roiResult.incremental_gross_profit, roiResult.currency)}\n` +
      `${t("promotion.total_cost_label", language)}: ${fmtMoney(roiResult.total_cost, roiResult.currency)}\n` +
      `${t("promotion.attribution_conclusion", language)}: ${roiResult.is_separable ? t("promotion.attribution_separable", language) : t("promotion.attribution_overlap", language)}`;
    
    if (navigator.clipboard) {
      navigator.clipboard.writeText(summary);
      message.success(t("promotion.msg_copied_action", language));
    } else {
      message.info(t("promotion.msg_action_ready", language));
    }
  };

  const columns = [
    {
      title: t("promotion.field_code", language),
      dataIndex: "promo_code",
      key: "promo_code",
      width: 140,
      render: (code: string) => <Text strong>{code}</Text>,
    },
    {
      title: t("promotion.field_name", language),
      dataIndex: "name",
      key: "name",
      width: 180,
    },
    {
      title: t("promotion.field_type", language),
      dataIndex: "promo_type",
      key: "promo_type",
      width: 110,
      render: (type: string) => {
        const kinds: Record<string, "processing" | "neutral"> = {
          discount: "processing",
          coupon: "neutral",
          gift: "neutral",
          member_day: "neutral",
        };
        return <StatusTag kind={kinds[type] || "neutral"}>{promotionTypeLabel(type, language)}</StatusTag>;
      },
    },
    {
      title: t("promotion.field_period", language),
      key: "period",
      width: 190,
      render: (_: any, r: Promotion) => `${r.start_date} ~ ${r.end_date}`,
    },
    {
      title: t("promotion.field_budget", language),
      dataIndex: "budget_amount",
      key: "budget_amount",
      align: "right" as const,
      width: 130,
      render: (v: number, r: Promotion) => fmtMoney(v, r.currency),
    },
    {
      title: t("promotion.field_status", language),
      dataIndex: "approval_status",
      key: "approval_status",
      width: 110,
      render: (st: string) => {
        const map: Record<string, { kind: "neutral" | "processing" | "success" | "error"; label: string }> = {
          draft: { kind: "neutral", label: t("promotion.status_draft", language) },
          approved: { kind: "processing", label: t("promotion.status_approved", language) },
          completed: { kind: "success", label: t("promotion.status_completed", language) },
          cancelled: { kind: "error", label: t("promotion.status_cancelled", language) },
        };
        const conf = map[st] || { kind: "neutral" as const, label: t("promotion.status_unknown", language) };
        return <StatusTag kind={conf.kind}>{conf.label}</StatusTag>;
      },
    },
    {
      title: t("promotion.col_actions", language),
      key: "actions",
      width: 120,
      render: (_: any, r: Promotion) => (
        <Button size="small" type="link" onClick={() => loadPromoDetails(r)}>
          {t("promotion.btn_review_details", language)}
        </Button>
      ),
    },
  ];

  return (
    <ProtectedRoute>
      <AppLayout>
        <PageHeader
          title={t("promotion.title", language)}
          primaryAction={
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateModalOpen(true)}>
              {t("promotion.create", language)}
            </Button>
          }
          secondaryAction={
            <Button icon={<ReloadOutlined />} onClick={loadPromotions}>
              {t("promotion.btn_refresh", language)}
            </Button>
          }
        />

        <div className="promotions-content">
          <Card
            className="promotions-list-card"
            title={
              <Space>
                <AuditOutlined />
                <span>{t("promotion.tab_list", language)}</span>
              </Space>
            }
            extra={
              <Segmented
                className="precision-segmented"
                value={statusFilter}
                onChange={(val) => setStatusFilter(String(val))}
                options={[
                  { label: t("promotion.all_status", language), value: "" },
                  { label: t("promotion.status_draft", language), value: "draft" },
                  { label: t("promotion.status_approved", language), value: "approved" },
                  { label: t("promotion.status_completed", language), value: "completed" },
                  { label: t("promotion.status_cancelled", language), value: "cancelled" },
                ]}
              />
            }
          >
            <Table
              dataSource={promotions}
              columns={columns}
              rowKey="id"
              loading={loading}
              scroll={tableScrollX(promotions.length, 900)}
              pagination={{ pageSize: 10 }}
            />
          </Card>

          {/* 报告版 ROI 假设测算（审计 §E-2 接线）：全局假设输入，不挂具体活动 */}
          <PromotionROIReportPanel />

          {/* Promotion Detail & ROI Drawer */}
          <Drawer
            title={selectedPromo ? `${t("promotion.drawer_title", language)}: ${selectedPromo.name} (${selectedPromo.promo_code})` : t("promotion.drawer_default_title", language)}
            width={720}
            open={drawerOpen}
            onClose={() => setDrawerOpen(false)}
            extra={
              <Space>
                <Button onClick={() => setCostModalOpen(true)}>
                  {t("promotion.btn_add_cost", language)}
                </Button>
                <Button type="primary" icon={<SendOutlined />} onClick={handleCreateActionItem}>
                  {t("promotion.create_action", language)}
                </Button>
              </Space>
            }
          >
            {selectedPromo && (
              <Tabs defaultActiveKey="roi">
                <Tabs.TabPane tab={t("promotion.tab_roi_attribution", language)} key="roi">
                  {roiLoading ? (
                    <div className="promotion-roi-loading">
                      <Spin size="large" />
                    </div>
                  ) : roiResult ? (
                    <Space direction="vertical" size={16} className="promotion-roi-stack">
                      {/* Overlap Non-separable Alert */}
                      {!roiResult.is_separable ? (
                        <Alert
                          type="warning"
                          showIcon
                          icon={<WarningOutlined />}
                          message={t("promotion.status_non_separable", language)}
                          description={
                            <div>
                              {roiResult.overlap_warnings.map((w, idx) => (
                                <div key={idx}>• {w}</div>
                              ))}
                            </div>
                          }
                        />
                      ) : (
                        <Alert
                          type="success"
                          showIcon
                          icon={<CheckCircleOutlined />}
                          message={t("promotion.status_separable", language)}
                          description={t("promotion.status_separable_desc", language)}
                        />
                      )}

                      {/* Top Metric Grid */}
                      <div className="stripe-metric-grid promotion-roi-grid">
                        <div className="pulse-kpi-card promotion-roi-card">
                          <span className="promotion-metric-label">{t("promotion.roi", language)}</span>
                          <Typography.Text className={`font-tabular promotion-roi-value ${roiResult.roi != null && roiResult.roi >= 1 ? "is-positive" : "is-warning"}`}>
                            {roiResult.roi != null ? `${(roiResult.roi * 100).toFixed(1)}%` : "—"}
                          </Typography.Text>
                          <Text type="secondary" className="promotion-metric-note">
                            {t("promotion.metric_roi_formula", language)}
                          </Text>
                        </div>

                        <div className="pulse-kpi-card promotion-roi-card">
                          <span className="promotion-metric-label">{t("promotion.field_actual_cost", language)}</span>
                          <Typography.Text className="font-tabular promotion-money-value">
                            {fmtMoney(roiResult.total_cost, roiResult.currency)}
                          </Typography.Text>
                          <Text type="secondary" className="promotion-metric-note">
                            {t("promotion.metric_budget_label", language, { amount: fmtMoney(roiResult.budget_amount, roiResult.currency) })}
                          </Text>
                        </div>

                        <div className="pulse-kpi-card promotion-roi-card">
                          <span className="promotion-metric-label">{t("promotion.inc_revenue", language)}</span>
                          <Typography.Text className={`font-tabular promotion-money-value ${roiResult.incremental_revenue >= 0 ? "is-positive" : "is-negative"}`}>
                            {`${Number(roiResult.incremental_revenue) >= 0 ? "+" : ""}${fmtMoney(Number(roiResult.incremental_revenue), roiResult.currency)}`}
                          </Typography.Text>
                          <Text type="secondary" className="promotion-metric-note">
                            {t("promotion.metric_actual_days_label", language, { days: String(roiResult.event_days), amount: fmtMoney(roiResult.actual_revenue, roiResult.currency) })}
                          </Text>
                        </div>

                        <div className="pulse-kpi-card promotion-roi-card">
                          <span className="promotion-metric-label">{t("promotion.inc_gross_profit", language)}</span>
                          <Typography.Text className={`font-tabular promotion-money-value ${roiResult.incremental_gross_profit >= 0 ? "is-positive" : "is-negative"}`}>
                            {`${Number(roiResult.incremental_gross_profit) >= 0 ? "+" : ""}${fmtMoney(Number(roiResult.incremental_gross_profit), roiResult.currency)}`}
                          </Typography.Text>
                          <Text type="secondary" className="promotion-metric-note">
                            {t("promotion.metric_baseline_gp_label", language, { amount: fmtMoney(roiResult.baseline_gross_profit, roiResult.currency) })}
                          </Text>
                        </div>
                      </div>

                      {/* Cost breakdown */}
                      <Card size="small" title={t("promotion.cost_breakdown_title", language)}>
                        {costs.length > 0 ? (
                          <Table
                            dataSource={costs}
                            rowKey="id"
                            size="small"
                            pagination={false}
                            columns={[
                              { title: t("promotion.col_cost_category", language), dataIndex: "cost_category", key: "cost_category", render: (value: string) => promotionCostLabel(value, language) },
                              { title: t("promotion.col_cost_period", language), dataIndex: "period", key: "period" },
                              {
                                title: t("promotion.col_cost_amount", language),
                                dataIndex: "amount",
                                key: "amount",
                                align: "right" as const,
                                render: (v: number) => fmtMoney(v, selectedPromo.currency),
                              },
                              { title: t("promotion.col_cost_notes", language), dataIndex: "notes", key: "notes" },
                            ]}
                          />
                        ) : (
                          <div className="promotion-cost-empty">
                            {t("promotion.cost_empty", language)}
                          </div>
                        )}
                      </Card>

                      {/* Disclaimers & Integrity */}
                      <Card size="small" className="promotion-disclaimer-card">
                        <Text strong className="promotion-disclaimer-title">
                          {t("promotion.disclaimer", language)}:
                        </Text>
                        <ul className="promotion-disclaimer-list">
                          {roiResult.disclaimers.map((d, i) => (
                            <li key={i}>{d}</li>
                          ))}
                        </ul>
                      </Card>
                    </Space>
                  ) : null}
                </Tabs.TabPane>

                {/* R2-1：投前保本——与投后复盘并列，不替换；渲染下沉到 BreakevenPanel 以便守卫直接打 */}
                <Tabs.TabPane tab={t("promotion.breakeven.tab", language)} key="breakeven">
                  <BreakevenPanel
                    loading={breakevenLoading}
                    rate={breakevenRate}
                    cost={breakevenCost}
                    onRateChange={setBreakevenRate}
                    onCostChange={setBreakevenCost}
                    onRun={runBreakeven}
                    result={breakevenResult}
                  />
                </Tabs.TabPane>
                <Tabs.TabPane tab={t("promotion.tab_budget_review", language)} key="budget">
                  <Card size="small" bordered={false}>
                    <Space direction="vertical" size={12} className="promotion-budget-stack">
                      <div>
                        <Text type="secondary">{t("promotion.info_name", language)}: </Text>
                        <Text strong>{selectedPromo.name}</Text>
                      </div>
                      <div>
                        <Text type="secondary">{t("promotion.info_code", language)}: </Text>
                        <Text code>{selectedPromo.promo_code}</Text>
                      </div>
                      <div>
                        <Text type="secondary">{t("promotion.info_type", language)}: </Text>
                        <StatusTag kind="processing">{promotionTypeLabel(selectedPromo.promo_type, language)}</StatusTag>
                      </div>
                      <div>
                        <Text type="secondary">{t("promotion.info_period", language)}: </Text>
                        <Text>{selectedPromo.start_date} ~ {selectedPromo.end_date}</Text>
                      </div>
                      <div>
                        <Text type="secondary">{t("promotion.info_scope", language)}: </Text>
                        <Text>{promotionScopeLabel(selectedPromo.target_scope, language)}</Text>
                      </div>
                      <div>
                        <Text type="secondary">{t("promotion.info_budget", language)}: </Text>
                        <Text strong className="promotion-budget-value">
                          {fmtMoney(selectedPromo.budget_amount, selectedPromo.currency)}
                        </Text>
                      </div>
                      <div>
                        <Text type="secondary">{t("promotion.info_owner", language)}: </Text>
                        <Text>{selectedPromo.owner || t("promotion.info_owner_unassigned", language)}</Text>
                      </div>
                      <div>
                        <Text type="secondary">{t("promotion.info_desc", language)}: </Text>
                        <Paragraph className="promotion-description">
                          {selectedPromo.description || t("promotion.info_no_desc", language)}
                        </Paragraph>
                      </div>
                    </Space>
                  </Card>
                </Tabs.TabPane>
              </Tabs>
            )}
          </Drawer>

          {/* Create Promotion Modal */}
          <Modal
            title={t("promotion.create", language)}
            open={createModalOpen}
            onCancel={() => setCreateModalOpen(false)}
            onOk={() => form.submit()}
          >
            <Form form={form} layout="vertical" onFinish={handleCreatePromo}>
              <Form.Item
                name="promo_code"
                label={t("promotion.field_code", language)}
                rules={[{ required: true, message: t("promotion.rule_code_required", language) }]}
              >
                <Input placeholder={t("promotion.placeholder_code", language)} />
              </Form.Item>
              <Form.Item
                name="name"
                label={t("promotion.field_name", language)}
                rules={[{ required: true, message: t("promotion.rule_name_required", language) }]}
              >
                <Input placeholder={t("promotion.placeholder_name", language)} />
              </Form.Item>
              <Row gutter={16}>
                <Col span={12}>
                  <Form.Item
                    name="promo_type"
                    label={t("promotion.field_type", language)}
                    rules={[{ required: true, message: t("promotion.rule_type_required", language) }]}
                    initialValue="discount"
                  >
                    <Select>
                      <Option value="discount">{t("promotion.type_discount", language)}</Option>
                      <Option value="coupon">{t("promotion.type_coupon", language)}</Option>
                      <Option value="gift">{t("promotion.type_gift", language)}</Option>
                      <Option value="member_day">{t("promotion.type_member_day", language)}</Option>
                      <Option value="other">{t("promotion.type_other", language)}</Option>
                    </Select>
                  </Form.Item>
                </Col>
                <Col span={12}>
                  <Form.Item
                    name="budget_amount"
                    label={t("promotion.field_budget", language)}
                    rules={[{ required: true, message: t("promotion.rule_budget_required", language) }]}
                    initialValue={10000}
                  >
                    <InputNumber className="promotion-full-width" min={0} />
                  </Form.Item>
                </Col>
              </Row>
              <Form.Item
                name="dates"
                label={t("promotion.field_period", language)}
                rules={[{ required: true, message: t("promotion.rule_dates_required", language) }]}
              >
                <RangePicker className="promotion-full-width" />
              </Form.Item>
              <Form.Item name="description" label={t("promotion.label_desc_assumptions", language)}>
                <Input.TextArea rows={3} placeholder={t("promotion.placeholder_desc", language)} />
              </Form.Item>
            </Form>
          </Modal>

          {/* Add Cost Modal */}
          <Modal
            title={t("promotion.modal_add_cost_title", language)}
            open={costModalOpen}
            onCancel={() => setCostModalOpen(false)}
            onOk={() => costForm.submit()}
          >
            <Form form={costForm} layout="vertical" onFinish={handleAddCost}>
              <Form.Item
                name="cost_category"
                label={t("promotion.col_cost_category", language)}
                rules={[{ required: true, message: t("promotion.rule_cost_category_required", language) }]}
                initialValue="subsidy"
              >
                <Select>
                  <Option value="subsidy">{t("promotion.cost_type_subsidy", language)}</Option>
                  <Option value="materials">{t("promotion.cost_type_materials", language)}</Option>
                  <Option value="labor">{t("promotion.cost_type_labor", language)}</Option>
                  <Option value="marketing">{t("promotion.cost_type_marketing", language)}</Option>
                  <Option value="other">{t("promotion.cost_type_other", language)}</Option>
                </Select>
              </Form.Item>
              <Form.Item
                name="amount"
                label={t("promotion.col_cost_amount", language)}
                rules={[{ required: true, message: t("promotion.rule_cost_amount_required", language) }]}
              >
                <InputNumber className="promotion-full-width" min={0} />
              </Form.Item>
              <Form.Item name="notes" label={t("promotion.label_cost_notes", language)}>
                <Input placeholder={t("promotion.placeholder_cost_notes", language)} />
              </Form.Item>
            </Form>
          </Modal>
        </div>
      </AppLayout>
    </ProtectedRoute>
  );
}
