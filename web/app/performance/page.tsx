"use client";

import { StatusTag, statusKindFromAntColor } from "../components/StatusTag";

import { useEffect, useMemo, useState } from "react";
import { Alert, Button, Card, Col, Empty, Input, InputNumber, Row, Space, Statistic, Table, Tabs, Tag, Typography, message } from "antd";
import { ReloadOutlined, RobotOutlined, CheckCircleOutlined, DownloadOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { HelpTrigger } from "../components/HelpDrawer";
import { performanceHelpContent } from "../components/help-content";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { performanceApi } from "../lib/api";
import { useRetailQuery } from "../retail/useRetailQuery";
import { notifyError } from "../lib/notify";

type Overview = { period: string; store_fact_count: number; store_fact_ready_count: number; store_fact_missing_count: number; store_fact_unmapped_count: number; store_fact_unreconciled_count: number; equipment_fact_count: number; equipment_fact_unreconciled_count: number; open_action_count: number; open_action_impact: number; latest_store_as_of?: string; latest_equipment_as_of?: string };
type FourWall = { store_id: string; store_code: string; store_name: string; brand: string; region: string; currency: string; revenue: number; gross_profit?: number; four_wall_ebitda?: number; rent_to_sales?: number; occupancy_cost_ratio?: number; sales_per_sqm?: number; break_even_sales?: number; data_ready: boolean; data_gaps?: string[]; reconciliation_status: string };
type EquipmentItem = { fact: { equipment_id: string; equipment_code: string; equipment_name: string; plant_code: string; production_line_code: string; currency: string; period: string; oee_pct?: number; utilization_pct?: number; actual_cost?: number; standard_cost?: number; reconciliation_status: string }; bridge?: { variance: number; residual: number; ties_out: boolean }; missing?: string[] };
type Action = { id: string; category: string; severity: string; status: string; title: string; description: string; impact_amount?: number; currency?: string; owner_name?: string; due_date?: string; expected_benefit?: number; verification_status: string; human_root_cause?: string; planned_action?: string; source_table: string; source_record_id: string };

const money = (value?: number, currency?: string) => value == null ? "—" : `${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })}${currency ? ` ${currency}` : ""}`;
const pct = (value?: number) => value == null ? "—" : `${value.toFixed(2)}%`;

export default function PerformancePage() {
  const { token } = useAuth();
  const { language } = useLanguage();
  // FIX-026: `period` is the *applied* period — the only thing the loader
  // depends on. `periodDraft` is what the text field holds while it is being
  // edited. They used to be one state in the effect's dependency list, so
  // editing "2026-07" fired a request at every keystroke and the half-typed
  // "2026-0" came back rejected as「请求未成功」. Nothing loads until the draft
  // is a well-formed YYYY-MM and the user applies it.
  const [period, setPeriod] = useState(dayjs().format("YYYY-MM"));
  const [periodDraft, setPeriodDraft] = useState(period);
  const periodDraftValid = /^\d{4}-(0[1-9]|1[0-2])$/.test(periodDraft.trim());
  const applyPeriod = () => {
    const next = periodDraft.trim();
    if (!periodDraftValid) return;
    if (next === period) retry();
    else setPeriod(next);
  };
  const [selectedActionIds, setSelectedActionIds] = useState<string[]>([]);
  const [scenarioInput, setScenarioInput] = useState({ sales: 100000, rent: 12000, margin: 40, discount: 0.12 });
  const [scenarioResult, setScenarioResult] = useState<any[] | null>(null);

  // FETCH-003: the cockpit loads four views for the applied period through
  // the shared fetch seam — the period drives the params, the seam owns
  // loading, the race gate and the error exit.
  const { loading, state: cockpitState, retry } = useRetailQuery({
    token,
    params: { period },
    paramsKey: `cockpit-${period}`,
    fetcher: async (p, t) => {
      const [overviewResult, storeResult, equipmentResult, actionResult] = await Promise.all([
        performanceApi.overview(p.period, t),
        performanceApi.storePerformance(p.period, t),
        performanceApi.equipmentPerformance(p.period, t),
        performanceApi.actions({ period: p.period }, t),
      ]);
      return {
        overview: overviewResult,
        stores: storeResult.data || [],
        equipment: equipmentResult.data || [],
        actions: actionResult.data || [],
      };
    },
  });
  const overview = cockpitState.kind === "ready" ? (cockpitState.data?.overview ?? null) : null;
  const stores: FourWall[] = cockpitState.kind === "ready" ? (cockpitState.data?.stores ?? []) : [];
  const equipment: EquipmentItem[] = cockpitState.kind === "ready" ? (cockpitState.data?.equipment ?? []) : [];
  const actions: Action[] = cockpitState.kind === "ready" ? (cockpitState.data?.actions ?? []) : [];
  useEffect(() => {
    if (cockpitState.kind === "failed") notifyError(cockpitState.message || t("performance.load_failed", language));
  }, [cockpitState, language]);

  const acknowledge = async (action: Action) => {
    if (!token) return;
    try { await performanceApi.updateAction(action.id, { status: "acknowledged" }, token); message.success("已确认行动"); retry(); }
    catch (error: any) { notifyError(error?.message || "行动更新失败"); }
  };

  const acknowledgeSelected = async () => {
    if (!token || selectedActionIds.length === 0) return;
    try {
      await performanceApi.bulkUpdateActions({ ids: selectedActionIds, status: "acknowledged" }, token);
      message.success(`已确认 ${selectedActionIds.length} 项行动`);
      setSelectedActionIds([]);
      retry();
    } catch (error: any) { notifyError(error?.message || "批量更新失败"); }
  };

  const exportActions = async () => {
    if (!token) return;
    try {
      const blob = await performanceApi.exportActions({ period }, token);
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = `fpna-actions-${period}.csv`;
      link.click();
      URL.revokeObjectURL(url);
    } catch (error: any) { notifyError(error?.message || "导出失败"); }
  };

  const simulateStore = async () => {
    if (!token) return;
    try {
      const response = await performanceApi.storeScenario([
        { name: "renew", decision: "renew", currency: "CNY", horizon_months: 36, discount_rate: scenarioInput.discount, monthly_sales: scenarioInput.sales, gross_margin_pct: scenarioInput.margin, monthly_labor: scenarioInput.sales * 0.1, monthly_other_cost: scenarioInput.sales * 0.05, monthly_rent: scenarioInput.rent, variable_rent_pct: 2 },
        { name: "close", decision: "close", currency: "CNY", horizon_months: 36, discount_rate: scenarioInput.discount, monthly_sales: scenarioInput.sales, gross_margin_pct: scenarioInput.margin, monthly_labor: scenarioInput.sales * 0.1, monthly_other_cost: scenarioInput.sales * 0.05, monthly_rent: scenarioInput.rent, exit_cost: scenarioInput.rent * 3 },
      ], token);
      setScenarioResult(response.data || []);
    } catch (error: any) { notifyError(error?.message || "情景测算失败"); }
  };

  const storeColumns = useMemo(() => [
    { title: "门店", key: "store", render: (_: unknown, row: FourWall) => <Space direction="vertical" size={0}><strong>{row.store_code} {row.store_name}</strong><Typography.Text type="secondary">{row.brand} · {row.region}</Typography.Text></Space> },
    { title: "营收", dataIndex: "revenue", render: (value: number, row: FourWall) => money(value, row.currency) },
    { title: "四墙 EBITDA", dataIndex: "four_wall_ebitda", render: (value: number, row: FourWall) => money(value, row.currency) },
    { title: "租售比", dataIndex: "rent_to_sales", render: (value: number) => pct(value) },
    { title: "坪效", dataIndex: "sales_per_sqm", render: (value: number, row: FourWall) => value == null ? "—" : `${value.toLocaleString()} ${row.currency}/㎡` },
    { title: "数据状态", key: "status", render: (_: unknown, row: FourWall) => row.data_ready ? <StatusTag kind="success">可用于决策</StatusTag> : <StatusTag kind="warning">缺口：{(row.data_gaps || []).join(", ") || row.reconciliation_status}</StatusTag> },
  ], []);

  const equipmentColumns = useMemo(() => [
    { title: "设备", key: "equipment", render: (_: unknown, row: EquipmentItem) => <Space direction="vertical" size={0}><strong>{row.fact.equipment_code} {row.fact.equipment_name}</strong><Typography.Text type="secondary">{row.fact.plant_code} · {row.fact.production_line_code}</Typography.Text></Space> },
    { title: "OEE", render: (_: unknown, row: EquipmentItem) => pct(row.fact.oee_pct) },
    { title: "利用率", render: (_: unknown, row: EquipmentItem) => pct(row.fact.utilization_pct) },
    { title: "成本差异", render: (_: unknown, row: EquipmentItem) => row.bridge ? money(row.bridge.variance, row.fact.currency) : "—" },
    { title: "残差", render: (_: unknown, row: EquipmentItem) => row.bridge ? money(row.bridge.residual, row.fact.currency) : "—" },
    { title: "数据状态", render: (_: unknown, row: EquipmentItem) => row.bridge?.ties_out ? <StatusTag kind="success">桥接平衡</StatusTag> : <StatusTag kind="warning">证据不足</StatusTag> },
  ], []);

  const actionColumns = useMemo(() => [
    { title: "异常 / 行动", key: "title", render: (_: unknown, row: Action) => <Space direction="vertical" size={0}><strong>{row.title}</strong><Typography.Text type="secondary">{row.category} · {row.source_table}:{row.source_record_id}</Typography.Text></Space> },
    { title: "影响", render: (_: unknown, row: Action) => money(row.impact_amount, row.currency) },
    { title: "优先级", dataIndex: "severity", render: (value: string) => <StatusTag kind={statusKindFromAntColor(value === "critical" || value === "high" ? "error" : "warning")}>{value}</StatusTag> },
    { title: "状态", dataIndex: "status", render: (value: string) => <StatusTag>{value}</StatusTag> },
    { title: "负责人 / 到期", render: (_: unknown, row: Action) => <Space direction="vertical" size={0}><span>{row.owner_name || "未分配"}</span><Typography.Text type="secondary">{row.due_date || "无日期"}</Typography.Text></Space> },
    { title: "操作", key: "action", render: (_: unknown, row: Action) => row.status === "open" ? <Button size="small" icon={<CheckCircleOutlined />} onClick={() => acknowledge(row)}>确认</Button> : null },
  ], [token]);

  return <ProtectedRoute><AppLayout><div>
    <PageHeader
      title="经营驾驶舱"
      meta={`${period} · Working 经营事实 · 数据截至 ${dayjs().format("YYYY-MM-DD HH:mm")} · 不替代 Official 关账。`}
      help={<HelpTrigger content={performanceHelpContent(language)} language={language} />}
    />
    <Card size="small" style={{ marginBottom: 16 }}><Space wrap><span>分析期间</span><Input value={periodDraft} onChange={event => setPeriodDraft(event.target.value)} onPressEnter={applyPeriod} status={periodDraftValid ? undefined : "error"} style={{ width: 120 }} placeholder="YYYY-MM" /><Button icon={<ReloadOutlined />} onClick={applyPeriod} disabled={!periodDraftValid} loading={loading}>刷新</Button><Button icon={<RobotOutlined />} onClick={() => window.location.href = `/ai-chat?message=${encodeURIComponent(`请生成 ${period} 的经营日报，并列出最重要的偏差和行动`)}`}>让 AI 解释</Button></Space></Card>
    {overview && <Row gutter={[12, 12]} style={{ marginBottom: 16 }}>
      <Col xs={24} sm={12} lg={6}><Card><Statistic title="门店事实" value={overview.store_fact_count} suffix={<Typography.Text type="secondary">/ {overview.store_fact_ready_count} 已对账</Typography.Text>} /></Card></Col>
      <Col xs={24} sm={12} lg={6}><Card><Statistic title="设备事实" value={overview.equipment_fact_count} /></Card></Col>
      <Col xs={24} sm={12} lg={6}><Card><Statistic title="待处理行动" value={overview.open_action_count} /></Card></Col>
      <Col xs={24} sm={12} lg={6}><Card><Statistic title="未兑现影响" value={overview.open_action_impact} precision={2} /></Card></Col>
    </Row>}
    <Alert type="info" showIcon style={{ marginBottom: 16 }} message="数据治理边界" description={<Space wrap><span>缺失字段 {overview?.store_fact_missing_count ?? 0}</span><span>未映射 {overview?.store_fact_unmapped_count ?? 0}</span><span>未对账 {overview?.store_fact_unreconciled_count ?? 0}</span><span>设备未对账 {overview?.equipment_fact_unreconciled_count ?? 0}</span><span>缺失、未映射、未对账与零值会分别展示；AI 只能引用这些系统事实并生成建议，不能自动确认解释、修改 Forecast 或创建正式租赁事件。</span></Space>} />
    <Card><Tabs items={[
      { key: "actions", label: `行动中心 (${actions.length})`, children: actions.length ? <Space direction="vertical" style={{ width: "100%" }}><Space><Button size="small" icon={<CheckCircleOutlined />} disabled={!selectedActionIds.length} onClick={acknowledgeSelected}>批量确认</Button><Button size="small" icon={<DownloadOutlined />} onClick={exportActions}>导出 Working CSV</Button></Space><Table rowKey="id" size="small" rowSelection={{ selectedRowKeys: selectedActionIds, onChange: keys => setSelectedActionIds(keys as string[]) }} columns={actionColumns} dataSource={actions} pagination={{ pageSize: 8 }} /></Space> : <Empty description="当前期间没有待处理行动" /> },
      { key: "stores", label: `零售四墙 (${stores.length})`, children: stores.length ? <Table rowKey="store_id" size="small" columns={storeColumns} dataSource={stores} pagination={{ pageSize: 8 }} scroll={{ x: 900 }} /> : <Empty description="暂无门店经营事实；请先导入受控事实批次" /> },
      { key: "equipment", label: `制造设备 (${equipment.length})`, children: equipment.length ? <Table rowKey={(row: EquipmentItem) => row.fact.equipment_id} size="small" columns={equipmentColumns} dataSource={equipment} pagination={{ pageSize: 8 }} scroll={{ x: 900 }} /> : <Empty description="暂无设备经营事实；请先导入受控事实批次" /> },
      { key: "scenario", label: "门店方案模拟", children: <Space direction="vertical" style={{ width: "100%" }} size={16}>
        <Alert type="warning" showIcon message="Scenario 草稿" description="下面的假设只进入确定性模拟，不会覆盖 Budget/Forecast、创建正式合同或触发会计重算；正式决策仍需人工确认。" />
        <Space wrap><span>月营收</span><InputNumber min={0} value={scenarioInput.sales} onChange={value => setScenarioInput(current => ({ ...current, sales: Number(value || 0) }))} /><span>月租金</span><InputNumber min={0} value={scenarioInput.rent} onChange={value => setScenarioInput(current => ({ ...current, rent: Number(value || 0) }))} /><span>毛利率 %</span><InputNumber min={0} max={100} value={scenarioInput.margin} onChange={value => setScenarioInput(current => ({ ...current, margin: Number(value || 0) }))} /><span>折现率</span><InputNumber min={0.0001} max={1} step={0.01} value={scenarioInput.discount} onChange={value => setScenarioInput(current => ({ ...current, discount: Number(value || 0) }))} /><Button type="primary" onClick={simulateStore}>运行确定性模拟</Button></Space>
        {scenarioResult ? <Table rowKey="name" size="small" dataSource={scenarioResult} pagination={false} columns={[{ title: "方案", dataIndex: "name" }, { title: "NPV", dataIndex: "npv", render: (value: number) => money(value, "CNY") }, { title: "回收期（月）", dataIndex: "payback_months", render: (value: number) => value == null ? "—" : value.toFixed(1) }, { title: "盈亏平衡月租", dataIndex: "break_even_monthly_rent", render: (value: number) => money(value, "CNY") }, { title: "议价范围", render: (_: unknown, row: any) => `${money(row.target_negotiation_low, "CNY")} – ${money(row.target_negotiation_high, "CNY")}` }]} /> : <Empty description="输入关键假设后运行模拟" />}
      </Space> },
    ]} /></Card>
  </div></AppLayout></ProtectedRoute>;
}
