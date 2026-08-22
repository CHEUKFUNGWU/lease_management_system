"use client";

import { StatusTag } from "../components/StatusTag";

import React, { useEffect, useMemo, useState } from "react";
import { Alert, Button, Card, Col, Empty, Input, InputNumber, Row, Space, Statistic, Table, Tabs, Tag, Tooltip, Typography, Upload, message } from "antd";
import { ReloadOutlined, CheckCircleOutlined, DownloadOutlined, InfoCircleOutlined, UploadOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { HelpTrigger } from "../components/HelpDrawer";
import { performanceHelpContent } from "../components/help-content";
import { SparkleGlyph, DownloadGlyph, UploadGlyph } from "../components/MonochromeGlyphs";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { apiErrorMessage, operatingFactsApi, performanceApi } from "../lib/api";
import { useRetailQuery } from "../retail/useRetailQuery";
import { notifyError } from "../lib/notify";
import { tableScrollX } from "../lib/tableScroll";
import ScopeNote from "../lib/ScopeNote";
import PeerBenchmarkBlock, { type PeerBenchmarkItem } from "./PeerBenchmarkBlock";
import { ActionCategoryText, ActionStatusTag, SeverityTag } from "./ActionCells";

type Overview = { period: string; store_fact_count: number; store_fact_ready_count: number; store_fact_missing_count: number; store_fact_unmapped_count: number; store_fact_unreconciled_count: number; equipment_fact_count: number; equipment_fact_unreconciled_count: number; open_action_count: number; open_action_impact: number; latest_store_as_of?: string; latest_equipment_as_of?: string };
type FourWall = { store_id: string; store_code: string; store_name: string; brand: string; region: string; currency: string; revenue: number; gross_profit?: number; four_wall_ebitda?: number; rent_to_sales?: number; occupancy_cost_ratio?: number; sales_per_sqm?: number; break_even_sales?: number; data_ready: boolean; data_gaps?: string[]; reconciliation_status: string };
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
  // P5-2: the monthly importer's controlled-template UI — the backend path
  // (row-level isolation, batches, idempotency) has always existed; this
  // wires the page to it instead of leaving it backend-only.
  const [importSource, setImportSource] = useState("erp");
  const [importing, setImporting] = useState(false);
  const handleMonthlyImport = async (file: File) => {
    if (!token) return;
    setImporting(true);
    try {
      const source = importSource.trim() || undefined;
      const result = file.name.toLowerCase().endsWith(".xlsx")
        ? await operatingFactsApi.importStoresXLSX(file, token, source)
        : await operatingFactsApi.importStoresCSV(file, token, source);
      const payload = result as { saved_count?: number; failed_count?: number; idempotent_replay?: boolean };
      const text = t("perf.import_done", language)
        .replace("{saved}", String(payload.saved_count ?? 0))
        .replace("{failed}", String(payload.failed_count ?? 0));
      message.success(payload.idempotent_replay ? `${text}（${t("perf.import_replay", language)}）` : text);
      applyPeriod();
    } catch (err) {
      message.error(apiErrorMessage(err));
    } finally {
      setImporting(false);
    }
  };
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
  const peerBenchmarks: PeerBenchmarkItem[] = cockpitState.kind === "ready" ? (cockpitState.data?.equipment ?? []) : [];
  const actions: Action[] = cockpitState.kind === "ready" ? (cockpitState.data?.actions ?? []) : [];
  const governanceCounts = [
    { label: t("perf.gov.missing_fields", language), value: overview?.store_fact_missing_count ?? 0 },
    { label: t("perf.gov.unmapped", language), value: overview?.store_fact_unmapped_count ?? 0 },
    { label: t("perf.gov.unreconciled", language), value: overview?.store_fact_unreconciled_count ?? 0 },
    { label: t("perf.gov.equipment_unreconciled", language), value: overview?.equipment_fact_unreconciled_count ?? 0 },
  ];
  useEffect(() => {
    if (cockpitState.kind === "failed") notifyError(cockpitState.message || t("performance.load_failed", language));
  }, [cockpitState, language]);

  const acknowledge = async (action: Action) => {
    if (!token) return;
    try { await performanceApi.updateAction(action.id, { status: "acknowledged" }, token); message.success(t("perf.action_acknowledged", language)); retry(); }
    catch (error: any) { notifyError(error?.message || t("perf.action_update_failed", language)); }
  };

  const acknowledgeSelected = async () => {
    if (!token || selectedActionIds.length === 0) return;
    try {
      await performanceApi.bulkUpdateActions({ ids: selectedActionIds, status: "acknowledged" }, token);
      message.success(t("perf.actions_acknowledged", language).replace("{count}", String(selectedActionIds.length)));
      setSelectedActionIds([]);
      retry();
    } catch (error: any) { notifyError(error?.message || t("perf.bulk_update_failed", language)); }
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
    } catch (error: any) { notifyError(error?.message || t("perf.export_failed", language)); }
  };

  const simulateStore = async () => {
    if (!token) return;
    try {
      const response = await performanceApi.storeScenario([
        { name: "续约谈判", decision: "renew", currency: "CNY", horizon_months: 36, discount_rate: scenarioInput.discount, monthly_sales: scenarioInput.sales, gross_margin_pct: scenarioInput.margin, monthly_labor: scenarioInput.sales * 0.1, monthly_other_cost: scenarioInput.sales * 0.05, monthly_rent: scenarioInput.rent, variable_rent_pct: 2 },
        { name: "提前解约", decision: "close", currency: "CNY", horizon_months: 36, discount_rate: scenarioInput.discount, monthly_sales: scenarioInput.sales, gross_margin_pct: scenarioInput.margin, monthly_labor: scenarioInput.sales * 0.1, monthly_other_cost: scenarioInput.sales * 0.05, monthly_rent: scenarioInput.rent, exit_cost: scenarioInput.rent * 3 },
      ], token);
      setScenarioResult(response.data || []);
    } catch (error: any) { notifyError(error?.message || t("perf.scenario_failed", language)); }
  };

  const storeColumns = useMemo(() => [
    { title: t("perf.col.store", language), key: "store", render: (_: unknown, row: FourWall) => <Space direction="vertical" size={0}><strong>{row.store_code} {row.store_name}</strong><Typography.Text type="secondary">{row.brand} · {row.region}</Typography.Text></Space> },
    { title: t("perf.col.revenue", language), dataIndex: "revenue", render: (value: number, row: FourWall) => money(value, row.currency) },
    { title: t("perf.col.four_wall_ebitda", language), dataIndex: "four_wall_ebitda", render: (value: number, row: FourWall) => money(value, row.currency) },
    { title: t("perf.col.rent_to_sales", language), dataIndex: "rent_to_sales", render: (value: number) => pct(value) },
    { title: t("perf.col.sales_per_sqm", language), dataIndex: "sales_per_sqm", render: (value: number, row: FourWall) => value == null ? "—" : `${value.toLocaleString()} ${row.currency}/㎡` },
    { title: t("perf.col.data_status", language), key: "status", render: (_: unknown, row: FourWall) => row.data_ready ? <StatusTag kind="success">{t("perf.status.decision_ready", language)}</StatusTag> : <StatusTag kind="warning">{t("perf.status.gap", language)}：{(row.data_gaps || []).join(", ") || row.reconciliation_status}</StatusTag> },
  ], [language]);

  // R0-1：设备事实块整体下沉到 PeerBenchmarkBlock，三分支（空 / 口径不可用 / 表格）
  // 由 resolveBasis 判定。页面不再持有 peerColumns。

  const actionColumns = useMemo(() => [
    { title: t("perf.col.action", language), key: "title", render: (_: unknown, row: Action) => <Space direction="vertical" size={0}><strong>{row.title}</strong><Typography.Text type="secondary"><ActionCategoryText value={row.category} language={language} /></Typography.Text></Space> },
    { title: t("perf.col.impact", language), render: (_: unknown, row: Action) => money(row.impact_amount, row.currency) },
    { title: t("perf.col.severity", language), dataIndex: "severity", render: (value: string) => <SeverityTag value={value} language={language} /> },
    { title: t("perf.col.status", language), dataIndex: "status", render: (value: string) => <ActionStatusTag value={value} language={language} /> },
    { title: t("perf.col.owner_due", language), render: (_: unknown, row: Action) => <Space direction="vertical" size={0}><span>{row.owner_name || t("perf.unassigned", language)}</span><Typography.Text type="secondary">{row.due_date || t("perf.no_date", language)}</Typography.Text></Space> },
    { title: t("perf.col.operation", language), key: "action", render: (_: unknown, row: Action) => row.status === "open" ? <Button size="small" icon={<CheckCircleOutlined />} onClick={() => acknowledge(row)}>{t("perf.acknowledge", language)}</Button> : null },
  // eslint-disable-next-line react-hooks/exhaustive-deps -- P2-C gate close-out: legacy dep semantics kept as-is; loaders are rebuilt every render so adding them would loop refetches. useCallback refactor tracked separately; do not add new exemptions.
  ], [token, language]);

  return (
    <ProtectedRoute>
      <AppLayout>
        <div>
          <PageHeader
            title={t("perf.title", language)}
            meta={t("perf.meta", language).replace("{period}", period).replace("{stamp}", dayjs().format("YYYY-MM-DD HH:mm"))}
            help={<HelpTrigger content={performanceHelpContent(language)} language={language} />}
          />
          {/* R0-3：页面定位说明——与 /operating-pulse 的分工与数据源差异 */}
          <ScopeNote noteKey="perf.scope_note" className="perf-scope-note" language={language} />
          <div className="precision-filter-bar" style={{ marginBottom: 16 }}>
            <Space wrap size={12} align="center">
              <div className="precision-filter-group">
                <span className="precision-filter-label">{t("perf.analysis_period", language)}:</span>
                <Input
                  value={periodDraft}
                  size="small"
                  onChange={event => setPeriodDraft(event.target.value)}
                  onPressEnter={applyPeriod}
                  status={periodDraftValid ? undefined : "error"}
                  style={{ width: 110 }}
                  placeholder="YYYY-MM"
                />
              </div>
              <Button size="small" icon={<ReloadOutlined />} onClick={applyPeriod} disabled={!periodDraftValid} loading={loading}>
                {t("common.refresh", language)}
              </Button>
              <Button
                size="small"
                icon={<SparkleGlyph size={13} />}
                onClick={() => window.location.href = `/ai-chat?message=${encodeURIComponent(t("perf.ai_prompt", language).replace("{period}", period))}`}
              >
                {t("perf.ask_ai", language)}
              </Button>
              <div className="precision-filter-group">
                <Input
                  size="small"
                  aria-label={t("perf.import_source", language)}
                  value={importSource}
                  onChange={(event) => setImportSource(event.target.value)}
                  placeholder={t("perf.import_source", language)}
                  className="perf-import-source"
                  style={{ width: 130 }}
                />
              </div>
              <Upload
                accept=".csv,.xlsx"
                maxCount={1}
                showUploadList={false}
                beforeUpload={(file) => { void handleMonthlyImport(file); return false; }}
              >
                <Button size="small" icon={<UploadOutlined />} loading={importing}>
                  {t("perf.import_monthly", language)}
                </Button>
              </Upload>
            </Space>
          </div>
          {overview && (
            <div className="stripe-metric-grid" style={{ gridTemplateColumns: "repeat(4, minmax(0, 1fr))", marginBottom: 16 }}>
              <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 90, padding: "16px 20px" }}>
                <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("perf.kpi.store_facts", language)}</span>
                <div style={{ margin: "8px 0 0", display: "flex", alignItems: "baseline", gap: 6 }}>
                  <Typography.Text className="font-tabular" style={{ fontSize: 22, fontWeight: 600, color: "var(--fg-primary)" }}>
                    {overview.store_fact_count}
                  </Typography.Text>
                  <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                    / {overview.store_fact_ready_count} {t("perf.kpi.reconciled_suffix", language)}
                  </Typography.Text>
                </div>
              </div>
              <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 90, padding: "16px 20px" }}>
                <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("perf.kpi.equipment_facts", language)}</span>
                <div style={{ margin: "8px 0 0" }}>
                  <Typography.Text className="font-tabular" style={{ fontSize: 22, fontWeight: 600, color: "var(--fg-primary)" }}>
                    {overview.equipment_fact_count}
                  </Typography.Text>
                </div>
              </div>
              <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 90, padding: "16px 20px" }}>
                <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("perf.kpi.open_actions", language)}</span>
                <div style={{ margin: "8px 0 0" }}>
                  <Typography.Text className="font-tabular" style={{ fontSize: 22, fontWeight: 600, color: "var(--fg-primary)" }}>
                    {overview.open_action_count}
                  </Typography.Text>
                </div>
              </div>
              <div className="pulse-kpi-card" style={{ height: "auto", minHeight: 90, padding: "16px 20px" }}>
                <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{t("perf.kpi.open_impact", language)}</span>
                <div style={{ margin: "8px 0 0" }}>
                  <Typography.Text className="font-tabular" style={{ fontSize: 22, fontWeight: 600, color: "var(--fg-primary)" }}>
                    {overview.open_action_impact.toFixed(2)}
                  </Typography.Text>
                </div>
              </div>
            </div>
          )}

          {governanceCounts.some((item) => item.value > 0) ? (
            <Alert
              type="warning"
              showIcon
              className="perf-governance-alert"
              style={{ marginBottom: 16 }}
              message={
                <Space wrap>
                  {governanceCounts.filter((item) => item.value > 0).map((item) => (
                    <span key={item.label}>{item.label} {item.value}</span>
                  ))}
                  <Tooltip title={t("perf.gov.note", language)}>
                    <InfoCircleOutlined className="perf-governance-hint" />
                  </Tooltip>
                </Space>
              }
            />
          ) : null}

          <Card>
            <Tabs items={[
              {
                key: "actions",
                label: `${t("perf.tab.actions", language)} (${actions.length})`,
                children: actions.length ? (
                  <Space direction="vertical" style={{ width: "100%" }}>
                    <Space>
                      <Button size="small" icon={<CheckCircleOutlined />} disabled={!selectedActionIds.length} onClick={acknowledgeSelected}>
                        {t("perf.bulk_acknowledge", language)}
                      </Button>
                      <Button size="small" icon={<DownloadOutlined />} onClick={exportActions}>
                        {t("perf.export_working_csv", language)}
                      </Button>
                    </Space>
                    <Table
                      rowKey="id"
                      size="small"
                      rowSelection={{ selectedRowKeys: selectedActionIds, onChange: keys => setSelectedActionIds(keys as string[]) }}
                      columns={actionColumns}
                      dataSource={actions}
                      pagination={{ pageSize: 8 }}
                      scroll={tableScrollX(actions.length, 900)}
                    />
                  </Space>
                ) : (
                  <Empty description={t("perf.empty.actions", language)} />
                ),
              },
              {
                key: "stores",
                label: `${t("perf.tab.stores", language)} (${stores.length})`,
                children: stores.length ? (
                  <Table
                    rowKey="store_id"
                    size="small"
                    columns={storeColumns}
                    dataSource={stores}
                    pagination={{ pageSize: 8 }}
                    scroll={tableScrollX(stores.length, 900)}
                  />
                ) : (
                  <Empty description={t("perf.empty.stores", language)} />
                ),
              },
              {
                key: "equipment",
                label: `${t("perf.tab.equipment", language)} (${peerBenchmarks.length})`,
                children: <PeerBenchmarkBlock items={peerBenchmarks} language={language} />,
              },
              {
                key: "scenario",
                label: t("perf.tab.scenario", language),
                children: (
                  <Space direction="vertical" style={{ width: "100%" }} size={16}>
                    <Alert type="warning" showIcon message={t("perf.scenario.draft_title", language)} description={t("perf.scenario.draft_note", language)} />
                    <Space wrap>
                      <span>{t("perf.scenario.monthly_sales", language)}</span>
                      <InputNumber min={0} value={scenarioInput.sales} onChange={value => setScenarioInput(current => ({ ...current, sales: Number(value || 0) }))} />
                      <span>{t("perf.scenario.monthly_rent", language)}</span>
                      <InputNumber min={0} value={scenarioInput.rent} onChange={value => setScenarioInput(current => ({ ...current, rent: Number(value || 0) }))} />
                      <span>{t("perf.scenario.margin", language)}</span>
                      <InputNumber min={0} max={100} value={scenarioInput.margin} onChange={value => setScenarioInput(current => ({ ...current, margin: Number(value || 0) }))} />
                      <span>{t("perf.scenario.discount", language)}</span>
                      <InputNumber min={0.0001} max={1} step={0.01} value={scenarioInput.discount} onChange={value => setScenarioInput(current => ({ ...current, discount: Number(value || 0) }))} />
                      <Button type="primary" onClick={simulateStore}>{t("perf.scenario.run", language)}</Button>
                    </Space>
                    {scenarioResult ? (
                      <Table
                        rowKey="name"
                        size="small"
                        dataSource={scenarioResult}
                        pagination={false}
                        columns={[
                          { title: t("perf.scenario.col_name", language), dataIndex: "name" },
                          { title: "NPV 净现值", dataIndex: "npv", render: (value: number) => money(value, "CNY") },
                          { title: t("perf.scenario.col_payback", language), dataIndex: "payback_months", render: (value: number) => value == null ? "—" : value.toFixed(1) },
                          { title: t("perf.scenario.col_breakeven", language), dataIndex: "break_even_monthly_rent", render: (value: number) => money(value, "CNY") },
                          { title: t("perf.scenario.col_negotiation", language), render: (_: unknown, row: any) => `${money(row.target_negotiation_low, "CNY")} – ${money(row.target_negotiation_high, "CNY")}` },
                        ]}
                      />
                    ) : (
                      <Empty description={t("perf.scenario.empty", language)} />
                    )}
                  </Space>
                ),
              },
            ]} />
          </Card>
        </div>
      </AppLayout>
    </ProtectedRoute>
  );
}
