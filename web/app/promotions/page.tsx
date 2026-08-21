"use client";

import { useEffect, useState, useMemo } from "react";
import {
  Card,
  Table,
  Button,
  Tag,
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
} from "../lib/api";

const { Text, Title, Paragraph } = Typography;
const { Option } = Select;
const { RangePicker } = DatePicker;

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

  const [form] = Form.useForm();
  const [costForm] = Form.useForm();

  const loadPromotions = async () => {
    if (!token) return;
    setLoading(true);
    try {
      const res = await promotionApi.list(token, statusFilter || undefined);
      setPromotions(res.promotions || []);
    } catch (err: any) {
      message.error(err?.message || "Failed to load promotions");
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
      message.error(err?.message || "Failed to evaluate promotion ROI");
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
    const summary = `${t("promotion.action_item_prefix", language)} ${selectedPromo.name} (ROI: ${roiResult.roi != null ? (roiResult.roi * 100).toFixed(1) + "%" : "N/A"})\n` +
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
        const colors: Record<string, string> = {
          discount: "blue",
          coupon: "cyan",
          gift: "purple",
          member_day: "magenta",
        };
        return <Tag color={colors[type] || "default"}>{type}</Tag>;
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
        const map: Record<string, { color: string; label: string }> = {
          draft: { color: "default", label: t("promotion.status_draft", language) },
          approved: { color: "processing", label: t("promotion.status_approved", language) },
          completed: { color: "success", label: t("promotion.status_completed", language) },
          cancelled: { color: "error", label: t("promotion.status_cancelled", language) },
        };
        const conf = map[st] || { color: "default", label: st };
        return <Tag color={conf.color}>{conf.label}</Tag>;
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
          meta={t("promotion.page_meta", language)}
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

        <div style={{ padding: 24 }}>
          <Card
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
                    <div style={{ textAlign: "center", padding: "40px 0" }}>
                      <Spin size="large" />
                    </div>
                  ) : roiResult ? (
                    <Space direction="vertical" size={16} style={{ width: "100%" }}>
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
                      <div className="stripe-metric-grid" style={{ gridTemplateColumns: "repeat(2, minmax(0, 1fr))" }}>
                        <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 90, padding: "14px 18px" }}>
                          <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("promotion.roi", language)}</span>
                          <div style={{ margin: "6px 0 2px" }}>
                            <Typography.Text
                              className="font-tabular"
                              style={{
                                fontSize: 22,
                                fontWeight: 600,
                                color: (roiResult.roi ?? 0) >= 1.0 ? "var(--state-success-text, #216E39)" : "var(--state-warning-text, #9A6700)",
                              }}
                            >
                              {roiResult.roi != null ? `${(roiResult.roi * 100).toFixed(1)}%` : "—"}
                            </Typography.Text>
                          </div>
                          <Text type="secondary" style={{ fontSize: 11 }}>
                            {t("promotion.metric_roi_formula", language)}
                          </Text>
                        </div>

                        <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 90, padding: "14px 18px" }}>
                          <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("promotion.field_actual_cost", language)}</span>
                          <div style={{ margin: "6px 0 2px" }}>
                            <Typography.Text className="font-tabular" style={{ fontSize: 20, fontWeight: 600, color: "var(--fg-primary)" }}>
                              {fmtMoney(roiResult.total_cost, roiResult.currency)}
                            </Typography.Text>
                          </div>
                          <Text type="secondary" style={{ fontSize: 11 }}>
                            {t("promotion.metric_budget_label", language, { amount: fmtMoney(roiResult.budget_amount, roiResult.currency) })}
                          </Text>
                        </div>

                        <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 90, padding: "14px 18px" }}>
                          <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("promotion.inc_revenue", language)}</span>
                          <div style={{ margin: "6px 0 2px" }}>
                            <Typography.Text
                              className="font-tabular"
                              style={{
                                fontSize: 20,
                                fontWeight: 600,
                                color: roiResult.incremental_revenue >= 0 ? "var(--state-success-text, #216E39)" : "var(--state-error-text, #C93B2B)",
                              }}
                            >
                              {`${Number(roiResult.incremental_revenue) >= 0 ? "+" : ""}${fmtMoney(Number(roiResult.incremental_revenue), roiResult.currency)}`}
                            </Typography.Text>
                          </div>
                          <Text type="secondary" style={{ fontSize: 11 }}>
                            {t("promotion.metric_actual_days_label", language, { days: String(roiResult.event_days), amount: fmtMoney(roiResult.actual_revenue, roiResult.currency) })}
                          </Text>
                        </div>

                        <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 90, padding: "14px 18px" }}>
                          <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("promotion.inc_gross_profit", language)}</span>
                          <div style={{ margin: "6px 0 2px" }}>
                            <Typography.Text
                              className="font-tabular"
                              style={{
                                fontSize: 20,
                                fontWeight: 600,
                                color: roiResult.incremental_gross_profit >= 0 ? "var(--state-success-text, #216E39)" : "var(--state-error-text, #C93B2B)",
                              }}
                            >
                              {`${Number(roiResult.incremental_gross_profit) >= 0 ? "+" : ""}${fmtMoney(Number(roiResult.incremental_gross_profit), roiResult.currency)}`}
                            </Typography.Text>
                          </div>
                          <Text type="secondary" style={{ fontSize: 11 }}>
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
                              { title: t("promotion.col_cost_category", language), dataIndex: "cost_category", key: "cost_category" },
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
                          <div style={{ textAlign: "center", padding: "16px 0", color: "var(--fg-muted)" }}>
                            {t("promotion.cost_empty", language)}
                          </div>
                        )}
                      </Card>

                      {/* Disclaimers & Integrity */}
                      <Card size="small" style={{ background: "var(--bg-subtle, #fafafa)" }}>
                        <Text strong style={{ fontSize: 12 }}>
                          {t("promotion.disclaimer", language)}:
                        </Text>
                        <ul style={{ margin: "4px 0 0 16px", padding: 0, fontSize: 11, color: "var(--fg-muted)" }}>
                          {roiResult.disclaimers.map((d, i) => (
                            <li key={i}>{d}</li>
                          ))}
                        </ul>
                      </Card>
                    </Space>
                  ) : null}
                </Tabs.TabPane>

                <Tabs.TabPane tab={t("promotion.tab_budget_review", language)} key="budget">
                  <Card size="small" bordered={false}>
                    <Space direction="vertical" size={12} style={{ width: "100%" }}>
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
                        <Tag color="blue">{selectedPromo.promo_type}</Tag>
                      </div>
                      <div>
                        <Text type="secondary">{t("promotion.info_period", language)}: </Text>
                        <Text>{selectedPromo.start_date} ~ {selectedPromo.end_date}</Text>
                      </div>
                      <div>
                        <Text type="secondary">{t("promotion.info_scope", language)}: </Text>
                        <Text>{selectedPromo.target_scope}</Text>
                      </div>
                      <div>
                        <Text type="secondary">{t("promotion.info_budget", language)}: </Text>
                        <Text strong style={{ fontSize: 16, color: "var(--color-primary, #1890ff)" }}>
                          {fmtMoney(selectedPromo.budget_amount, selectedPromo.currency)}
                        </Text>
                      </div>
                      <div>
                        <Text type="secondary">{t("promotion.info_owner", language)}: </Text>
                        <Text>{selectedPromo.owner || t("promotion.info_owner_unassigned", language)}</Text>
                      </div>
                      <div>
                        <Text type="secondary">{t("promotion.info_desc", language)}: </Text>
                        <Paragraph style={{ marginTop: 4 }}>
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
                <Input placeholder="例: PROMO_2026_MEMBER_06" />
              </Form.Item>
              <Form.Item
                name="name"
                label={t("promotion.field_name", language)}
                rules={[{ required: true, message: t("promotion.rule_name_required", language) }]}
              >
                <Input placeholder="例: 6月夏日狂欢会员日" />
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
                    <InputNumber style={{ width: "100%" }} min={0} />
                  </Form.Item>
                </Col>
              </Row>
              <Form.Item
                name="dates"
                label={t("promotion.field_period", language)}
                rules={[{ required: true, message: t("promotion.rule_dates_required", language) }]}
              >
                <RangePicker style={{ width: "100%" }} />
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
                <InputNumber style={{ width: "100%" }} min={0} />
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
