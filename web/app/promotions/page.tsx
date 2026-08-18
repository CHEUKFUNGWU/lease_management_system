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
      message.success("促销活动创建成功");
      setCreateModalOpen(false);
      form.resetFields();
      loadPromotions();
    } catch (err: any) {
      message.error(err?.message || "创建失败");
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
      message.success("活动费用记录成功");
      setCostModalOpen(false);
      costForm.resetFields();
      loadPromoDetails(selectedPromo);
    } catch (err: any) {
      message.error(err?.message || "添加费用失败");
    }
  };

  const handleCreateActionItem = () => {
    if (!selectedPromo || !roiResult) return;
    const summary = `[活动复盘行动项] ${selectedPromo.name} (ROI: ${roiResult.roi != null ? (roiResult.roi * 100).toFixed(1) + "%" : "N/A"})\n` +
      `活动编码: ${selectedPromo.promo_code}\n` +
      `增量毛利: ${fmtMoney(roiResult.incremental_gross_profit, roiResult.currency)}\n` +
      `实际总成本: ${fmtMoney(roiResult.total_cost, roiResult.currency)}\n` +
      `归因结论: ${roiResult.is_separable ? "归因独立。" : "存在重叠活动，请结合商圈与门店排期复核。"}`;
    
    if (navigator.clipboard) {
      navigator.clipboard.writeText(summary);
      message.success("已复制活动复盘结论，可直接粘贴生成经营行动项！");
    } else {
      message.info("活动复盘结论已就绪");
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
          draft: { color: "default", label: "草稿" },
          approved: { color: "processing", label: "已审批" },
          completed: { color: "success", label: "已结案" },
          cancelled: { color: "error", label: "已作废" },
        };
        const conf = map[st] || { color: "default", label: st };
        return <Tag color={conf.color}>{conf.label}</Tag>;
      },
    },
    {
      title: "操作",
      key: "actions",
      width: 120,
      render: (_: any, r: Promotion) => (
        <Button size="small" type="link" onClick={() => loadPromoDetails(r)}>
          复盘详情
        </Button>
      ),
    },
  ];

  return (
    <ProtectedRoute>
      <AppLayout>
        <PageHeader
          title={t("promotion.title", language)}
          meta="事前活动预算审查 · 事后增量毛利与 ROI 归因 · 行动项闭环"
          primaryAction={
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateModalOpen(true)}>
              {t("promotion.create", language)}
            </Button>
          }
          secondaryAction={
            <Button icon={<ReloadOutlined />} onClick={loadPromotions}>
              刷新
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
              <Select
                value={statusFilter}
                onChange={setStatusFilter}
                style={{ width: 130 }}
                placeholder="全部状态"
              >
                <Option value="">全部状态</Option>
                <Option value="draft">草稿</Option>
                <Option value="approved">已审批</Option>
                <Option value="completed">已结案</Option>
                <Option value="cancelled">已作废</Option>
              </Select>
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
            title={selectedPromo ? `活动复盘: ${selectedPromo.name} (${selectedPromo.promo_code})` : "活动详情"}
            width={720}
            open={drawerOpen}
            onClose={() => setDrawerOpen(false)}
            extra={
              <Space>
                <Button onClick={() => setCostModalOpen(true)}>
                  添加费用发生额
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
                          description="活动期内无并发重叠活动，基线差值分析具备良好独立性。"
                        />
                      )}

                      {/* Top Metric Cards */}
                      <Row gutter={[16, 16]}>
                        <Col span={12}>
                          <Card size="small">
                            <Statistic
                              title={t("promotion.roi", language)}
                              value={roiResult.roi != null ? (roiResult.roi * 100).toFixed(1) : "—"}
                              suffix={roiResult.roi != null ? "%" : ""}
                              valueStyle={{
                                fontSize: 24,
                                color: (roiResult.roi ?? 0) >= 1.0 ? "#52c41a" : "#faad14",
                              }}
                            />
                            <Text type="secondary" style={{ fontSize: 11 }}>
                              增量毛利 / 实际总成本
                            </Text>
                          </Card>
                        </Col>
                        <Col span={12}>
                          <Card size="small">
                            <Statistic
                              title={t("promotion.field_actual_cost", language)}
                              value={roiResult.total_cost}
                              formatter={(v) => fmtMoney(Number(v), roiResult.currency)}
                              valueStyle={{ fontSize: 20 }}
                            />
                            <Text type="secondary" style={{ fontSize: 11 }}>
                              预算: {fmtMoney(roiResult.budget_amount, roiResult.currency)}
                            </Text>
                          </Card>
                        </Col>
                        <Col span={12}>
                          <Card size="small">
                            <Statistic
                              title={t("promotion.inc_revenue", language)}
                              value={roiResult.incremental_revenue}
                              formatter={(v) => `${Number(v) >= 0 ? "+" : ""}${fmtMoney(Number(v), roiResult.currency)}`}
                              valueStyle={{ fontSize: 18, color: roiResult.incremental_revenue >= 0 ? "#52c41a" : "#ff4d4f" }}
                            />
                            <Text type="secondary" style={{ fontSize: 11 }}>
                              活动期 {roiResult.event_days} 天总实际: {fmtMoney(roiResult.actual_revenue, roiResult.currency)}
                            </Text>
                          </Card>
                        </Col>
                        <Col span={12}>
                          <Card size="small">
                            <Statistic
                              title={t("promotion.inc_gross_profit", language)}
                              value={roiResult.incremental_gross_profit}
                              formatter={(v) => `${Number(v) >= 0 ? "+" : ""}${fmtMoney(Number(v), roiResult.currency)}`}
                              valueStyle={{ fontSize: 18, color: roiResult.incremental_gross_profit >= 0 ? "#52c41a" : "#ff4d4f" }}
                            />
                            <Text type="secondary" style={{ fontSize: 11 }}>
                              基线毛利: {fmtMoney(roiResult.baseline_gross_profit, roiResult.currency)}
                            </Text>
                          </Card>
                        </Col>
                      </Row>

                      {/* Cost breakdown */}
                      <Card size="small" title="费用发生明细 (Cost Breakdown)">
                        {costs.length > 0 ? (
                          <Table
                            dataSource={costs}
                            rowKey="id"
                            size="small"
                            pagination={false}
                            columns={[
                              { title: "类别", dataIndex: "cost_category", key: "cost_category" },
                              { title: "期间", dataIndex: "period", key: "period" },
                              {
                                title: "金额",
                                dataIndex: "amount",
                                key: "amount",
                                align: "right" as const,
                                render: (v: number) => fmtMoney(v, selectedPromo.currency),
                              },
                              { title: "备注", dataIndex: "notes", key: "notes" },
                            ]}
                          />
                        ) : (
                          <div style={{ textAlign: "center", padding: "16px 0", color: "var(--fg-muted)" }}>
                            暂无实际费用录入，请点击上方「添加费用发生额」录入补贴/物料/推广等成本
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
                        <Text type="secondary">活动名称: </Text>
                        <Text strong>{selectedPromo.name}</Text>
                      </div>
                      <div>
                        <Text type="secondary">活动编码: </Text>
                        <Text code>{selectedPromo.promo_code}</Text>
                      </div>
                      <div>
                        <Text type="secondary">活动类型: </Text>
                        <Tag color="blue">{selectedPromo.promo_type}</Tag>
                      </div>
                      <div>
                        <Text type="secondary">活动时间: </Text>
                        <Text>{selectedPromo.start_date} ~ {selectedPromo.end_date}</Text>
                      </div>
                      <div>
                        <Text type="secondary">适用范围: </Text>
                        <Text>{selectedPromo.target_scope}</Text>
                      </div>
                      <div>
                        <Text type="secondary">预算上限: </Text>
                        <Text strong style={{ fontSize: 16, color: "var(--color-primary, #1890ff)" }}>
                          {fmtMoney(selectedPromo.budget_amount, selectedPromo.currency)}
                        </Text>
                      </div>
                      <div>
                        <Text type="secondary">负责人: </Text>
                        <Text>{selectedPromo.owner || "未指定"}</Text>
                      </div>
                      <div>
                        <Text type="secondary">活动方案说明: </Text>
                        <Paragraph style={{ marginTop: 4 }}>
                          {selectedPromo.description || "无详细方案描述"}
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
                rules={[{ required: true, message: "请输入活动编码" }]}
              >
                <Input placeholder="例: PROMO_2026_MEMBER_06" />
              </Form.Item>
              <Form.Item
                name="name"
                label={t("promotion.field_name", language)}
                rules={[{ required: true, message: "请输入活动名称" }]}
              >
                <Input placeholder="例: 6月夏日狂欢会员日" />
              </Form.Item>
              <Row gutter={16}>
                <Col span={12}>
                  <Form.Item
                    name="promo_type"
                    label={t("promotion.field_type", language)}
                    rules={[{ required: true, message: "请选择类型" }]}
                    initialValue="discount"
                  >
                    <Select>
                      <Option value="discount">商品直折 (Discount)</Option>
                      <Option value="coupon">满减满折 (Coupon)</Option>
                      <Option value="gift">买赠/随单礼 (Gift)</Option>
                      <Option value="member_day">会员日专享 (Member Day)</Option>
                      <Option value="other">其他营销 (Other)</Option>
                    </Select>
                  </Form.Item>
                </Col>
                <Col span={12}>
                  <Form.Item
                    name="budget_amount"
                    label={t("promotion.field_budget", language)}
                    rules={[{ required: true, message: "请输入预算金额" }]}
                    initialValue={10000}
                  >
                    <InputNumber style={{ width: "100%" }} min={0} />
                  </Form.Item>
                </Col>
              </Row>
              <Form.Item
                name="dates"
                label={t("promotion.field_period", language)}
                rules={[{ required: true, message: "请选择起止日期" }]}
              >
                <RangePicker style={{ width: "100%" }} />
              </Form.Item>
              <Form.Item name="description" label="活动说明与测算假设">
                <Input.TextArea rows={3} placeholder="输入活动目的、折扣深度及预期拉动指标..." />
              </Form.Item>
            </Form>
          </Modal>

          {/* Add Cost Modal */}
          <Modal
            title="录入实际活动费用"
            open={costModalOpen}
            onCancel={() => setCostModalOpen(false)}
            onOk={() => costForm.submit()}
          >
            <Form form={costForm} layout="vertical" onFinish={handleAddCost}>
              <Form.Item
                name="cost_category"
                label="费用类别"
                rules={[{ required: true, message: "请选择费用类别" }]}
                initialValue="subsidy"
              >
                <Select>
                  <Option value="subsidy">商品毛利让利补贴 (Subsidy)</Option>
                  <Option value="materials">物料制作与布置 (Materials)</Option>
                  <Option value="labor">活动专岗与兼职人力 (Labor)</Option>
                  <Option value="marketing">商圈与线上推广费 (Marketing)</Option>
                  <Option value="other">其他杂费 (Other)</Option>
                </Select>
              </Form.Item>
              <Form.Item
                name="amount"
                label="费用金额"
                rules={[{ required: true, message: "请输入金额" }]}
              >
                <InputNumber style={{ width: "100%" }} min={0} />
              </Form.Item>
              <Form.Item name="notes" label="费用说明/凭据号">
                <Input placeholder="输入报销单号或供应商合同..." />
              </Form.Item>
            </Form>
          </Modal>
        </div>
      </AppLayout>
    </ProtectedRoute>
  );
}
