"use client";

import { useEffect, useMemo, useState, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Alert, Button, Card, Col, Collapse, DatePicker, Empty, Flex, Input, InputNumber, Radio, Row, Select, Segmented, Space, Spin, Table, Tag, Typography } from "antd";
import { ArrowLeftOutlined, CheckCircleFilled, ReloadOutlined } from "@ant-design/icons";
import { Bar, BarChart, CartesianGrid, Cell, ResponsiveContainer, Tooltip as ChartTooltip, XAxis, YAxis } from "recharts";
import dayjs from "dayjs";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import DataTrustBar, { KPIReadyBadge } from "../components/DataTrustBar";
import RetailAIDrawer from "../components/RetailAIDrawer";
import ProtectedRoute from "../components/ProtectedRoute";
import { StatusTag } from "../components/StatusTag";
import { SparkleGlyph, SlidersGlyph, DownloadGlyph } from "../components/MonochromeGlyphs";
import ConfidenceBandChart from "../components/charts/ConfidenceBandChart";
import { HelpTrigger } from "../components/HelpDrawer";
import { store360HelpContent } from "../components/help-content";
import { StateBlock } from "../components/StateBlock";
import { BentoGrid, BentoTile } from "../components/bento/BentoGrid";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t, type Language } from "../lib/i18n";
import { apiErrorMessage, retailAnalyticsApi, type RetailDataClassification, type RetailPlFlowResponse, type RetailSimulationDatasetData, type RetailStore360Option, type RetailStoreDiagnosticsResponse, type RetailSummaryMetric } from "../lib/api";
import { ApiError } from "../lib/api";
import { classifyDataState } from "../lib/dataState";
import { useRetailQuery } from "../retail/useRetailQuery";
import { RetailExportMenu } from "../components/RetailExportMenu";
import { retailExportApi } from "../lib/api";
import { diagnosticsRowsFromResponse, envelopeFromDiagnostics } from "../lib/retail-export";
import { PlanComparisonPanel } from "../components/PlanComparisonPanel";
import { changeTone, formatChange, formatKPIValue, kpiLabel, latestAnomalyDate, lifecycleStatusLabel, storeFormatLabel, translateReason, type PulseMetricCode } from "../operating-pulse/logic";
import { bridgeConservation, bridgeTone, bridgeWaterfall, bridgeWaterfallDomain, displayMetric, formatBridgeItem, formatPeerBenchmarkStatus, formatTrendTooltip, optionFields, returnPulseQuery, STORE360_AUX_CODES, STORE360_CODES, summaryStatus, trendValue, validWindow, WINDOW_OPTIONS } from "./logic";
import ProfitFlowPanel from "./ProfitFlowPanel";

const TODAY = dayjs().format("YYYY-MM-DD");

function queryFromURL(searchParams: URLSearchParams) {
  const classification = searchParams.get("data_classification") as RetailDataClassification | null;
  const datasetVersion = searchParams.get("dataset_version") || "";
  const asOf = searchParams.get("as_of") || "";
  const rawWindow = Number(searchParams.get("window_days") || 14);
  return { storeID: searchParams.get("store_id") || "", classification: classification === "production" || classification === "simulated" ? classification : "", datasetVersion, asOf, windowDays: rawWindow, period: searchParams.get("period") || "", sourceSystem: searchParams.get("source_system") || "", returnQuery: searchParams.get("return_query") || "" };
}

function writeQuery(router: ReturnType<typeof useRouter>, value: { storeID?: string; classification: RetailDataClassification; datasetVersion?: string; asOf: string; windowDays: number; period?: string; sourceSystem?: string; returnQuery?: string }) {
  const query = new URLSearchParams();
  if (value.storeID) query.set("store_id", value.storeID);
  query.set("data_classification", value.classification);
  if (value.classification === "simulated" && value.datasetVersion) query.set("dataset_version", value.datasetVersion);
  query.set("as_of", value.asOf);
  if (value.period) query.set("period", value.period);
  else query.set("window_days", String(validWindow(value.windowDays) ? value.windowDays : 14));
  if (value.sourceSystem) query.set("source_system", value.sourceSystem);
  if (value.returnQuery) query.set("return_query", value.returnQuery);
  router.replace(`/store-360?${query.toString()}`);
}

function Status({ metric, language }: { metric?: RetailStoreDiagnosticsResponse["summary"][string]; language: Language }) {
  const summary = summaryStatus(metric, language);
  if (summary.status === "complete") return null;
  return <StatusTag kind="warning">{summary.label}</StatusTag>;
}

function MetricCard({ code, metric, currency, notReady, language }: { code: PulseMetricCode; metric?: RetailStoreDiagnosticsResponse["summary"][string]; currency: string; notReady?: boolean; language: Language }) {
  const tone = changeTone(code, metric as RetailSummaryMetric | undefined);
  const rawReason = metric?.reason || metric?.current.reason || metric?.comparison.reason;
  const reason = translateReason(rawReason, language);
  const display = displayMetric(metric, currency, language);
  return (
    <div className="store-360-kpi-card" data-testid={`store360-kpi-${code}`}>
      <Flex justify="space-between" align="center">
        <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{kpiLabel(code, language)}</span>
        <Flex align="center" gap={4}>
          {notReady && <KPIReadyBadge />}
          <Status metric={metric} language={language} />
        </Flex>
      </Flex>
      <Typography.Title level={3} className="pulse-kpi-value" ellipsis={{ tooltip: display }}>{display}</Typography.Title>
      <div style={{ display: "flex", alignItems: "baseline", gap: 6, overflow: "hidden", whiteSpace: "nowrap" }}>
        <Typography.Text className={`pulse-change pulse-change-${tone}`}>
          {formatChange(metric as RetailSummaryMetric | undefined)} {reason ? `· ${reason}` : ""}
        </Typography.Text>
        <Typography.Text type="secondary" className="pulse-kpi-comparison">
          {t("common.contrast", language)}: {formatKPIValue(metric?.comparison, currency, language)}
        </Typography.Text>
      </div>
    </div>
  );
}

function formatPeerDefinition(def: string, language: Language): string {
  if (def.includes("same brand + region + currency")) {
    return t("store360.peer_def_standard", language);
  }
  return def;
}

function trendUnit(code: PulseMetricCode): string {
  if (code === "revenue" || code === "gross_profit" || code === "store_contribution" || code === "average_transaction_value") return "currency";
  if (code.endsWith("rate") || code === "gross_margin_rate" || code === "conversion_rate" || code === "store_contribution_margin") return "percent";
  if (code === "sales_per_sqm") return "currency_per_sqm";
  return "count";
}

function Trend({ response, language }: { response: RetailStoreDiagnosticsResponse; language: Language }) {
  const [code, setCode] = useState<PulseMetricCode>("revenue");
  const chartData = useMemo(() => {
    return response.daily_trend.map((row) => {
      const targetVal = trendValue(row, code);
      const peerMedian = row.peer_median[code] ?? null;
      const p25 = peerMedian != null ? peerMedian * 0.90 : null;
      const p75 = peerMedian != null ? peerMedian * 1.10 : null;
      return {
        date: row.date.slice(5),
        value: targetVal,
        median: peerMedian,
        p25,
        p75,
      };
    });
  }, [response.daily_trend, code]);

  return (
    <Card
      bordered={false}
      title={
        <Flex justify="space-between" align="center" wrap="wrap" gap={8}>
          <span style={{ fontSize: 13, fontWeight: 600, color: "var(--fg-primary)" }}>{t("store360.trend_title", language)}</span>
          <Segmented
            size="small"
            className="precision-segmented"
            value={code}
            onChange={(value) => setCode(value as PulseMetricCode)}
            options={STORE360_CODES.map((item) => ({ label: kpiLabel(item, language), value: item }))}
          />
        </Flex>
      }
      style={{ height: "100%", background: "transparent" }}
    >
      <ConfidenceBandChart
        data={chartData}
        metricLabel={kpiLabel(code, language)}
        unit={trendUnit(code)}
        currency={response.currency}
        height={270}
      />
    </Card>
  );
}

const BRIDGE_TONE_FILL = {
  positive: "#2D4B46",  // Muted pine green
  negative: "#7F473E",  // Muted terracotta rust
  neutral: "#0F172A",   // Midnight obsidian
} as const;
const PL_FLOW_OPTION = "__pl_flow";

function BridgeWaterfall({ bridges, currency, language, plFlow, plFlowError }: { bridges: RetailStoreDiagnosticsResponse["bridges"]; currency: string; language: Language; plFlow: RetailPlFlowResponse | null; plFlowError: string | null }) {
  const complete = bridges.filter((bridge) => bridge.status === "complete" && bridge.comparison != null && bridge.current != null);
  const [code, setCode] = useState<string>("");
  const showPlFlow = code === PL_FLOW_OPTION;
  const bridge = complete.find((item) => item.code === code) || complete[0];
  const steps = bridge ? bridgeWaterfall(bridge, { start: t("store360.bridge.start", language), end: t("store360.bridge.end", language), residual: t("store360.bridge.residual", language) }) : [];
  const options = [
    ...complete.map((item) => ({ label: kpiLabel(item.code as PulseMetricCode, language) || item.code, value: item.code })),
    { label: t("store360.pl_flow.title", language), value: PL_FLOW_OPTION },
  ];
  return (
    <Card
      bordered={false}
      title={
        <Flex justify="space-between" align="center" wrap="wrap" gap={8}>
          <span style={{ fontSize: 13, fontWeight: 600, color: "var(--fg-primary)" }}>{showPlFlow ? t("store360.pl_flow.title", language) : t("store360.bridge.chart_title", language)}</span>
          {options.length > 1 && (
            <Segmented
              size="small"
              className="precision-segmented"
              value={showPlFlow ? PL_FLOW_OPTION : bridge?.code}
              onChange={(value) => setCode(String(value))}
              options={options}
            />
          )}
        </Flex>
      }
      style={{ height: "100%", background: "transparent" }}
    >
      {showPlFlow ? (
        <ProfitFlowPanel flow={plFlow} error={plFlowError} currency={currency} language={language} />
      ) : (
        <div className="chart-frame" style={{ width: "100%", height: 270 }}>
          {steps.length === 0 ? (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("store360.bridge.no_complete", language)} />
          ) : (
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={steps} margin={{ top: 8, right: 12, left: 0, bottom: 4 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="var(--border-subtle, #D9D9D9)" />
                <XAxis dataKey="name" tick={{ fontSize: 11 }} interval={0} />
                <YAxis tick={{ fontSize: 11 }} domain={bridgeWaterfallDomain(steps) ?? ["auto", "auto"]} allowDataOverflow tickFormatter={(value) => value == null ? "—" : Number(value).toLocaleString()} />
                <ChartTooltip formatter={(_value, _name, item) => [formatBridgeItem(item?.payload?.contribution ?? null, "currency", currency, language), item?.payload?.name || ""]} />
                <Bar dataKey="range" radius={2} maxBarSize={48}>
                  {steps.map((step, index) => <Cell key={`${step.name}-${index}`} fill={BRIDGE_TONE_FILL[step.tone]} />)}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          )}
        </div>
      )}
    </Card>
  );
}

function BridgePanel({ bridges, currency, language }: { bridges: RetailStoreDiagnosticsResponse["bridges"]; currency: string; language: Language }) {
  return (
    <Card title={t("store360.bridge.title", language)}>
      <Space direction="vertical" size={12} className="store360-full-width">
        {bridges.map((bridge) => (
          <Card
            size="small"
            key={bridge.code}
            title={
              <Flex justify="space-between" align="center">
                <span style={{ fontWeight: 600, fontSize: 13, color: "var(--fg-primary)" }}>{bridge.code}</span>
                {bridge.status !== "complete" && (
                  <span style={{ fontSize: 11, color: "var(--state-warning-text)" }}>
                    {t("store360.bridge.unavailable", language)}
                  </span>
                )}
              </Flex>
            }
          >
            {bridge.status !== "complete" ? (
              <Typography.Text type="secondary">{bridge.reason || t("store360.bridge.reason_default", language)}</Typography.Text>
            ) : (
              <>
                <Flex wrap="wrap" gap={16} style={{ marginBottom: 10, fontSize: 12, color: "var(--fg-secondary)" }}>
                  <span>{t("store360.bridge.contrast", language)}: <strong>{formatBridgeItem(bridge.comparison, "currency", currency, language)}</strong></span>
                  <span>{t("store360.bridge.current", language)}: <strong>{formatBridgeItem(bridge.current, "currency", currency, language)}</strong></span>
                  <span>{t("store360.bridge.change", language)}: <strong className={`store-360-bridge-${bridgeTone(bridge.total_change)}`}>{formatBridgeItem(bridge.total_change, "currency", currency, language)}</strong></span>
                  {bridge.rounding_residual != null && (
                    <span style={{ color: "var(--fg-muted)" }}>{t("store360.bridge.residual", language)}: {formatBridgeItem(bridge.rounding_residual, "currency", currency, language)}</span>
                  )}
                </Flex>
                <Table
                  size="small"
                  pagination={false}
                  rowKey="code"
                  dataSource={bridge.items}
                  columns={[
                    { title: t("store360.bridge.item", language), dataIndex: "label" },
                    {
                      title: t("store360.bridge.contribution", language),
                      render: (_: unknown, row: typeof bridge.items[number]) => (
                        <Typography.Text className={`store-360-bridge-${bridgeTone(row.contribution)} font-tabular`}>
                          {formatBridgeItem(row.contribution, row.unit, currency, language)}
                        </Typography.Text>
                      ),
                    },
                  ]}
                />
              </>
            )}
          </Card>
        ))}
      </Space>
    </Card>
  );
}

function Store360Inner() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const [aiOpen, setAiOpen] = useState(false);
  const router = useRouter();
  const searchParams = useSearchParams();
  const query = useMemo(() => queryFromURL(searchParams), [searchParams]);
  const [latest, setLatest] = useState<RetailSimulationDatasetData | null | undefined>(undefined);
  const [options, setOptions] = useState<RetailStore360Option[]>([]);
  const [optionsLoading, setOptionsLoading] = useState(false);
  const [discoveryLoading, setDiscoveryLoading] = useState(true);
  const [optionsError, setOptionsError] = useState<string | null>(null);
  const [retry, setRetry] = useState(0);
  const [sourceInput, setSourceInput] = useState(query.sourceSystem);
  const [windowInput, setWindowInput] = useState(query.windowDays);

  useEffect(() => {
    if (!token) return;
    let active = true;
    setDiscoveryLoading(true);
    retailAnalyticsApi.latestSimulationDataset(token).then((result) => {
      if (active) setLatest(result.data);
    }).catch(() => { if (active) setLatest(null); }).finally(() => { if (active) setDiscoveryLoading(false); });
    return () => { active = false; };
  }, [token, retry]);

  useEffect(() => {
    if (!token || query.classification === "") return;
    if (query.classification === "simulated" && !query.datasetVersion) return;
    let active = true;
    setOptionsLoading(true);
    setOptionsError(null);
    retailAnalyticsApi.storeOptions({ data_classification: query.classification as RetailDataClassification, dataset_version: query.datasetVersion || undefined }, token).then((result) => {
      if (active) setOptions(result.data);
    }).catch((err) => { if (active) setOptionsError(apiErrorMessage(err)); }).finally(() => { if (active) setOptionsLoading(false); });
    return () => { active = false; };
  }, [token, query.classification, query.datasetVersion, retry]);

  const queryReady = Boolean(query.storeID && query.classification && query.asOf && (query.period !== "" || validWindow(query.windowDays)) && (query.classification !== "simulated" || query.datasetVersion));
  const diagParams = queryReady
    ? { store_id: query.storeID, data_classification: query.classification as RetailDataClassification, dataset_version: query.datasetVersion || undefined, as_of: query.asOf, ...(query.period ? { period: query.period } : { window_days: query.windowDays }), source_system: query.sourceSystem || undefined }
    : null;
  const diagKey = [query.storeID, query.classification, query.datasetVersion, query.asOf, query.period, query.windowDays, query.sourceSystem].join("|");
  const { loading, state: diagState, retry: diagRetry } = useRetailQuery({
    token,
    params: diagParams,
    paramsKey: diagKey,
    fetcher: (p, t) => retailAnalyticsApi.storeDiagnostics(p, t),
    actionFor: (e) => e instanceof ApiError && e.status === 404 && query.classification === "production"
      ? { message: t("store360.actionable_production_empty", language), actionLabel: t("store360.actionable_switch_simulated", language) }
      : null,
  });
  const response = diagState.kind === "ready" ? diagState.data ?? null : null;

  const plFlowParams = queryReady
    ? { store_id: query.storeID, data_classification: query.classification as RetailDataClassification, dataset_version: query.datasetVersion || undefined, as_of: query.asOf, ...(query.period ? { period: query.period } : { window_days: query.windowDays }), source_system: query.sourceSystem || undefined }
    : null;
  const { state: plFlowState } = useRetailQuery({
    token,
    params: plFlowParams,
    paramsKey: `pl:${diagKey}`,
    fetcher: (p, t) => retailAnalyticsApi.plFlow(p, t),
  });
  const plFlow = plFlowState.kind === "ready" ? plFlowState.data ?? null : null;
  const plFlowError = plFlowState.kind === "failed" ? plFlowState.message ?? null : null;

  useEffect(() => {
    setSourceInput(query.sourceSystem);
  }, [query.sourceSystem]);

  useEffect(() => {
    setWindowInput(query.windowDays);
  }, [query.windowDays]);

  const applyCustomWindow = () => {
    const next = Math.round(windowInput);
    if (next >= 7 && next <= 28) change({ windowDays: next, period: "" });
  };
  const [monthPicking, setMonthPicking] = useState(false);
  const derivedPeriodMode = query.period === "" ? "rolling" : query.period === "last-month" ? "last-month" : query.period === "this-quarter" ? "this-quarter" : "month";
  const periodMode = monthPicking ? "month" : derivedPeriodMode;
  const onPeriodModeChange = (mode: string) => {
    setMonthPicking(mode === "month");
    if (mode === "month") return;
    change({ period: mode === "rolling" ? "" : mode });
  };
  const onPeriodMonthChange = (date: dayjs.Dayjs | null) => {
    if (!date) return;
    setMonthPicking(false);
    change({ period: date.format("YYYY-MM") });
  };

  useEffect(() => {
    if (query.classification || !latest || discoveryLoading) return;
    writeQuery(router, { classification: "simulated", datasetVersion: latest.dataset_version, asOf: latestAnomalyDate(latest), windowDays: validWindow(query.windowDays) ? query.windowDays : 14, storeID: query.storeID, sourceSystem: query.sourceSystem, returnQuery: query.returnQuery });
  }, [query.classification, query.windowDays, query.storeID, query.sourceSystem, latest, discoveryLoading, router]);

  useEffect(() => {
    if (query.classification !== "simulated" || query.datasetVersion || !latest || discoveryLoading) return;
    writeQuery(router, { classification: "simulated", datasetVersion: latest.dataset_version, asOf: query.asOf || latestAnomalyDate(latest), windowDays: validWindow(query.windowDays) ? query.windowDays : 14, storeID: query.storeID, sourceSystem: query.sourceSystem, returnQuery: query.returnQuery });
  }, [query.classification, query.datasetVersion, query.asOf, query.windowDays, query.storeID, query.sourceSystem, latest, discoveryLoading, router]);

  const selected = options.map(optionFields).find((item) => item.storeID === query.storeID);
  const identity = response?.store || (selected ? { store_id: selected.storeID, store_code: selected.storeCode, store_name: selected.storeName, brand: selected.brand, region: selected.region } : null);
  const latestMatches = query.classification === "simulated" && latest?.dataset_version === query.datasetVersion ? latest : null;
  const backURL = returnPulseQuery(searchParams);
  const scenarioURL = useMemo(() => {
    const next = new URLSearchParams(searchParams.toString());
    next.set("return_query", searchParams.toString());
    return `/scenario-workbench?${next.toString()}`;
  }, [searchParams]);
  const noQuery = !query.classification && !discoveryLoading && latest === null;

  const change = (next: Partial<typeof query>) => writeQuery(router, { classification: (next.classification || query.classification || "simulated") as RetailDataClassification, datasetVersion: next.datasetVersion ?? query.datasetVersion, asOf: next.asOf || query.asOf || TODAY, windowDays: next.windowDays ?? (validWindow(query.windowDays) ? query.windowDays : 14), period: next.period !== undefined ? next.period : query.period, sourceSystem: next.sourceSystem ?? query.sourceSystem, storeID: next.storeID ?? query.storeID, returnQuery: query.returnQuery });

  return (
    <ProtectedRoute>
      <AppLayout>
        <div className="store-360-page">
          <PageHeader
            title={t("store360.title", language)}
            meta={t("store360.scope_note", language)}
            help={<HelpTrigger content={store360HelpContent(language)} language={language} />}
            primaryAction={
              <Button icon={<ReloadOutlined />} loading={loading || discoveryLoading} onClick={() => setRetry((value) => value + 1)}>
                {t("common.refresh", language)}
              </Button>
            }
            secondaryAction={
              <Space>
                <RetailExportMenu
                  kind="store_diagnostics"
                  disabled={!response}
                  envelope={response ? envelopeFromDiagnostics(response) : null}
                  rows={() => (response ? diagnosticsRowsFromResponse(response) : [])}
                  csvDownload={() => retailExportApi.downloadDiagnosticsCSV({
                    store_id: query.storeID,
                    data_classification: query.classification as RetailDataClassification,
                    dataset_version: query.datasetVersion || undefined,
                    as_of: query.asOf,
                    window_days: query.windowDays,
                    source_system: query.sourceSystem || undefined,
                  }, token!)}
                />
                <Button icon={<SparkleGlyph size={13} />} onClick={() => setAiOpen(true)}>
                  {t("common.ai_analysis", language)}
                </Button>
                <Button icon={<SlidersGlyph size={13} />} onClick={() => router.push(scenarioURL)}>
                  {t("store360.scenario_analysis", language)}
                </Button>
                <Button icon={<ArrowLeftOutlined />} onClick={() => router.push(backURL)}>
                  {t("store360.back_pulse", language)}
                </Button>
              </Space>
            }
          />
          <div className="precision-filter-bar store-360-filter-card">
            <Flex gap={12} wrap="wrap" align="center">
              <Segmented
                size="small"
                className="precision-segmented"
                value={query.classification || "simulated"}
                onChange={(val) => {
                  const next = val as RetailDataClassification;
                  if (next === "production") change({ classification: next, datasetVersion: "", asOf: TODAY });
                  else if (latest) change({ classification: next, datasetVersion: latest.dataset_version, asOf: latestAnomalyDate(latest) });
                  else change({ classification: next, datasetVersion: "" });
                }}
                options={[
                  { label: t("retail.classification.simulated", language), value: "simulated" },
                  { label: t("retail.classification.production", language), value: "production" },
                ]}
              />
              <Select
                size="small"
                showSearch
                allowClear
                value={query.storeID || undefined}
                placeholder={t("store360.select_store", language)}
                className="store360-store-select"
                loading={discoveryLoading || optionsLoading}
                notFoundContent={optionsLoading ? t("store360.loading_stores", language) : t("store360.no_selectable_stores", language)}
                options={options.map((option) => {
                  const item = optionFields(option);
                  return {
                    label: `${item.storeCode} · ${item.storeName}`,
                    value: item.storeID,
                    search: `${item.storeCode} ${item.storeName} ${item.brand} ${item.region}`,
                  };
                })}
                optionFilterProp="search"
                onChange={(value) => change({ storeID: value || "" })}
              />
              <DatePicker
                size="small"
                allowClear={false}
                value={query.asOf ? dayjs(query.asOf) : undefined}
                onChange={(date) => date && change({ asOf: date.format("YYYY-MM-DD") })}
              />
              <Select
                size="small"
                aria-label={t("pulse.period_mode", language)}
                value={periodMode}
                className="store360-select-min"
                options={[
                  { label: t("pulse.period_rolling", language), value: "rolling" },
                  { label: t("pulse.period_last_month", language), value: "last-month" },
                  { label: t("pulse.period_this_quarter", language), value: "this-quarter" },
                  { label: t("pulse.period_month", language), value: "month" },
                ]}
                onChange={(value) => onPeriodModeChange(String(value))}
              />
              {periodMode === "month" && (
                <DatePicker
                  size="small"
                  picker="month"
                  aria-label={t("pulse.period_month", language)}
                  value={derivedPeriodMode === "month" && query.period ? dayjs(`${query.period}-01`) : null}
                  onChange={onPeriodMonthChange}
                />
              )}
              {periodMode === "rolling" && (
                <Segmented
                  size="small"
                  className="precision-segmented"
                  value={WINDOW_OPTIONS.includes((validWindow(query.windowDays) ? query.windowDays : 14) as any) ? (validWindow(query.windowDays) ? query.windowDays : 14) : undefined}
                  onChange={(value) => change({ windowDays: Number(value), period: "" })}
                  options={[
                    { label: t("pulse.day_1", language), value: 1 },
                    { label: t("pulse.days_count", language, { count: "7" }), value: 7 },
                    { label: t("pulse.days_count", language, { count: "14" }), value: 14 },
                    { label: t("pulse.days_count", language, { count: "30" }), value: 30 },
                    { label: t("pulse.days_count", language, { count: "90" }), value: 90 },
                  ]}
                />
              )}
              {periodMode === "rolling" && (
                <InputNumber
                  size="small"
                  aria-label={t("pulse.custom_window", language)}
                  min={1}
                  max={365}
                  addonAfter={t("common.days_suffix", language)}
                  value={windowInput}
                  onChange={(value) => setWindowInput(value ?? 14)}
                  onPressEnter={applyCustomWindow}
                  style={{ width: 100 }}
                />
              )}
              {periodMode === "rolling" && windowInput !== query.windowDays && (
                <Button size="small" onClick={applyCustomWindow}>{t("pulse.apply_window", language)}</Button>
              )}
              <Input
                size="small"
                aria-label={t("common.source_system", language)}
                allowClear
                value={sourceInput}
                onChange={(event) => setSourceInput(event.target.value)}
                onPressEnter={() => change({ sourceSystem: sourceInput.trim() })}
                placeholder={t("common.source_system_optional", language)}
                style={{ width: 150 }}
              />
              {sourceInput !== (query.sourceSystem || "") && (
                <Button size="small" onClick={() => change({ sourceSystem: sourceInput.trim() })}>{t("store360.apply_source", language)}</Button>
              )}
            </Flex>
          </div>
          {response && (
            <DataTrustBar
              envelope={response.envelope}
              basis={response.basis}
              detailExtra={latestMatches ? <span>generator: {latestMatches.generator_version} · anomaly: {latestAnomalyDate(latestMatches)}</span> : undefined}
            />
          )}
          {response && response.plan && <PlanComparisonPanel plan={response.plan} currency={response.currency || ""} language={language} />}
          {noQuery && (
            <div className="store360-block-margin">
              <StateBlock
                state={{ kind: "actionable", message: t("store360.no_dataset_title", language), reason: t("store360.no_dataset_desc", language), actionLabel: t("common.go_pulse", language) }}
                language={language}
                onAction={() => router.push("/operating-pulse")}
              />
            </div>
          )}
          {query.classification === "simulated" && !query.datasetVersion && !discoveryLoading && (
            <div className="store360-block-margin">
              <StateBlock
                state={{ kind: "actionable", message: t("store360.missing_version_title", language), reason: t("store360.missing_version_desc", language), actionLabel: t("common.go_pulse", language) }}
                language={language}
                onAction={() => router.push("/operating-pulse")}
              />
            </div>
          )}
          {optionsError && (
            <div className="store360-block-margin">
              <StateBlock state={{ kind: "failed", message: optionsError }} language={language} onRetry={() => setRetry((value) => value + 1)} />
            </div>
          )}
          {query.classification && !optionsLoading && !optionsError && options.length === 0 && (
            <Alert type="info" showIcon message={t("store360.no_authorized_stores", language)} description={t("store360.no_authorized_desc", language)} />
          )}
          {loading && (
            <Card>
              <Flex justify="center" align="center" className="store360-loading-block">
                <Spin tip={t("store360.loading", language)} />
              </Flex>
            </Card>
          )}

          <StateBlock
            state={diagState}
            language={language}
            onAction={() => writeQuery(router, {
              classification: "simulated",
              datasetVersion: latest?.dataset_version || "",
              asOf: latest ? latestAnomalyDate(latest) : query.asOf,
              windowDays: query.windowDays,
              returnQuery: query.returnQuery,
            })}
            onRetry={diagRetry}
          />
          {!loading && diagState.kind === "empty" && query.storeID && (
            <StateBlock state={{ kind: "empty", reason: t("store360.pick_filters", language) }} language={language} />
          )}
          {response && !response.decision_ready && response.evidence.observed_store_days === 0 && (
            <Alert className="store360-block-margin" type="warning" showIcon message={t("store360.no_facts_title", language)} description={t("store360.no_facts_desc", language)} />
          )}

          {response && (
            <>
              {/* 12-Column Bento Grid Layout for Store 360 Diagnostics */}
              <div className="store360-block-gap" style={{ marginTop: 16 }}>
                <BentoGrid columns={12} gap={16}>
                  <BentoTile span={12} rows={1} variant="feature" noPadding>
                    <div style={{ padding: "14px 18px" }}>
                      <Flex justify="space-between" align="center" wrap="wrap" gap={16} style={{ marginBottom: 12 }}>
                        <Space size={8} align="center" wrap>
                          <Typography.Title level={4} className="store360-identity-title" style={{ margin: 0, fontSize: 16, fontWeight: 600, color: "var(--fg-primary)" }}>
                            {response.store.store_code} · {response.store.store_name}
                          </Typography.Title>
                          {response.store.lifecycle_status && (
                            <Tag bordered={false} color={response.store.lifecycle_status === "mature" ? "blue" : response.store.lifecycle_status === "ramp_up" ? "cyan" : "default"} style={{ margin: 0, fontSize: 11 }}>
                              {lifecycleStatusLabel(response.store.lifecycle_status, language)}
                            </Tag>
                          )}
                          {response.store.store_format && (
                            <Tag bordered={false} color="purple" style={{ margin: 0, fontSize: 11 }}>
                              {storeFormatLabel(response.store.store_format, language)}
                            </Tag>
                          )}
                          {(response.store.brand || response.store.region) && (
                            <span style={{ fontSize: 12, color: "var(--fg-secondary)", fontWeight: 500, marginLeft: 4 }}>
                              · {[response.store.brand, response.store.region].filter(Boolean).join(" · ")}
                            </span>
                          )}
                          {response.store.opening_date && (
                            <span style={{ fontSize: 11, color: "var(--fg-muted)", marginLeft: 4 }}>
                              ({t("retail.store.opening_date", language)}: {response.store.opening_date})
                            </span>
                          )}
                        </Space>
                        <Space size={16}>
                          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                            {t("store360.field.currency", language)}: <strong>{response.currency || "—"}</strong> ({response.currency_status})
                          </Typography.Text>
                          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                            {t("store360.field.fact_version", language)}: <strong>v{response.fact_version_min}–v{response.fact_version_max}</strong>
                          </Typography.Text>
                        </Space>
                      </Flex>
                      <div className="stripe-metric-grid" style={{ marginTop: 12 }}>
                        {STORE360_CODES.map((code) => (
                          <MetricCard
                            language={language}
                            key={code}
                            code={code}
                            metric={response.summary[code]}
                            currency={response.currency}
                            notReady={!response.decision_ready}
                          />
                        ))}
                      </div>
                    </div>
                  </BentoTile>

                  {/* 2. Twin Diagnostic Engine: Daily Trend (Left) */}
                  <BentoTile
                    span={6}
                    rows={2}
                    variant="hero"
                    noPadding
                  >
                    <Trend response={response} language={language} />
                  </BentoTile>

                  {/* 3. Twin Diagnostic Engine: Waterfall Attribution & Profit Flow (Right) */}
                  <BentoTile
                    span={6}
                    rows={2}
                    variant="hero"
                    noPadding
                  >
                    <BridgeWaterfall
                      bridges={response.bridges}
                      currency={response.currency}
                      language={language}
                      plFlow={plFlow}
                      plFlowError={plFlowError}
                    />
                  </BentoTile>

                  {/* 4. Auxiliary Metrics & Cost Structure (Full Width Horizontal Strip) */}
                  <BentoTile
                    span={12}
                    rows={1}
                    variant="feature"
                    noPadding
                  >
                    <div style={{ padding: "14px 18px" }}>
                      <Flex justify="space-between" align="center" style={{ marginBottom: 12 }}>
                        <span style={{ fontSize: 13, fontWeight: 600, color: "var(--fg-primary)" }}>
                          {t("store360.aux_metrics", language)}
                        </span>
                        <Typography.Text type="secondary" style={{ fontSize: 11 }}>
                          {t("store360.cash_basis_desc", language)}
                        </Typography.Text>
                      </Flex>
                      <div className="stripe-metric-grid" style={{ gridTemplateColumns: `repeat(${STORE360_AUX_CODES.length}, minmax(0, 1fr))` }}>
                        {STORE360_AUX_CODES.map((code) => (
                          <div key={code} className="store-360-kpi-card" style={{ height: "auto", minHeight: 80, padding: "14px 18px", borderBottom: "none", display: "flex", flexDirection: "column", justifyContent: "center" }}>
                            <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)", marginBottom: 6 }}>{kpiLabel(code, language)}</span>
                            <div style={{ display: "flex", alignItems: "baseline", justifyContent: "space-between", gap: 8 }}>
                              <Typography.Text className="font-tabular" style={{ fontSize: 19, fontWeight: 600, color: "var(--fg-primary)" }}>
                                {displayMetric(response.summary[code], response.currency, language)}
                              </Typography.Text>
                              <Status metric={response.summary[code]} language={language} />
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  </BentoTile>
                </BentoGrid>
              </div>

              <Card title={t("store360.peer_benchmark", language)} className="store360-block-gap">
                <Typography.Text type="secondary" style={{ display: "block", marginBottom: 8, fontSize: 12 }}>
                  {formatPeerDefinition(response.peer_definition, language)} · {t("store360.peer_definition", language).replace("{n}", String(response.minimum_peer_count))}
                </Typography.Text>
                <Table
                  className="store360-peer-table"
                  size="small"
                  pagination={false}
                  rowKey="code"
                  dataSource={response.peer_benchmark}
                  columns={[
                    { title: t("store360.col.metric", language), render: (_: unknown, row: RetailStoreDiagnosticsResponse["peer_benchmark"][number]) => kpiLabel(row.code as PulseMetricCode, language) || row.code },
                    { title: t("store360.col.target", language), render: (_: unknown, row) => formatKPIValue({ value: row.target, unit: row.unit, status: "complete", formula_version: "", required_fields: [], available_fact_count: 0, fact_count: 0 }, response.currency, language) },
                    { title: t("store360.col.quartiles", language), render: (_: unknown, row) => `${formatKPIValue({ value: row.p25, unit: row.unit, status: "complete", formula_version: "", required_fields: [], available_fact_count: 0, fact_count: 0 }, response.currency, language)} / ${formatKPIValue({ value: row.median, unit: row.unit, status: "complete", formula_version: "", required_fields: [], available_fact_count: 0, fact_count: 0 }, response.currency, language)} / ${formatKPIValue({ value: row.p75, unit: row.unit, status: "complete", formula_version: "", required_fields: [], available_fact_count: 0, fact_count: 0 }, response.currency, language)}` },
                    { title: t("store360.col.sample_percentile", language), render: (_: unknown, row) => `${row.peer_count} · ${row.percentile == null ? "—" : `${row.percentile.toFixed(1)}%`}` },
                    {
                      title: t("store360.col.status", language),
                      render: (_: unknown, row) => row.status === "complete" || row.status === "available" ? (
                        <CheckCircleFilled style={{ color: "#166534", fontSize: 13 }} />
                      ) : (
                        <span style={{ fontSize: 11, color: "var(--state-warning-text)" }}>
                          {formatPeerBenchmarkStatus(row.status, row.reason, language)}
                        </span>
                      ),
                    },
                  ]}
                />
              </Card>

              <div className="store360-block-gap">
                <BridgePanel language={language} bridges={response.bridges} currency={response.currency} />
              </div>

              <Card title={t("store360.observations", language)} className="store360-block-gap">
                <Space direction="vertical" className="store360-full-width">
                  {response.observations.length ? (
                    response.observations.map((item) => (
                      <Alert key={`${item.code}-${item.reference}`} type={item.status === "complete" ? "info" : "warning"} showIcon message={item.label} description={item.statement} />
                    ))
                  ) : (
                    <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("store360.no_observations", language)} />
                  )}
                </Space>
              </Card>

              <Collapse
                className="store360-block-gap"
                items={[
                  {
                    key: "evidence",
                    label: t("store360.evidence_title", language),
                    children: (
                      <Space direction="vertical">
                        <Typography.Text>{t("common.current", language)} {response.evidence.current.date_from}–{response.evidence.current.date_to} · {t("common.contrast", language)} {response.evidence.comparison.date_from}–{response.evidence.comparison.date_to}</Typography.Text>
                        <Typography.Text>{t("store360.evidence.coverage_source", language).replace("{observed}", String(response.evidence.observed_store_days)).replace("{expected}", String(response.evidence.expected_store_days)).replace("{sources}", response.evidence.source_systems.join(", ") || "—").replace("{datasets}", response.evidence.dataset_versions.join(", ") || "—")}</Typography.Text>
                        <Typography.Text>{t("store360.evidence.required_fields", language)}: {response.evidence.required_fields.join(", ")}</Typography.Text>
                        <Typography.Text>{t("store360.evidence.fact_version", language).replace("{min}", String(response.evidence.fact_version_min)).replace("{max}", String(response.evidence.fact_version_max))} · <a href={response.evidence.kpi_drilldown_url}>{t("common.view_kpi_drilldown", language)}</a></Typography.Text>
                      </Space>
                    ),
                  },
                ]}
              />
            </>
          )}

          <RetailAIDrawer
            open={aiOpen}
            onClose={() => setAiOpen(false)}
            pageContext={{
              page: "store-360",
              title: t("store360.title", language),
              filters: {
                as_of: query.asOf,
                window_days: String(query.windowDays),
                classification: query.classification || "",
                dataset_version: query.datasetVersion || "",
                source_system: query.sourceSystem || "",
                store_id: query.storeID || "",
              },
            }}
          />
        </div>
      </AppLayout>
    </ProtectedRoute>
  );
}

export default function Store360Page() {
  return (
    <Suspense fallback={<div className="store360-suspense-fallback"><Spin /></div>}>
      <Store360Inner />
    </Suspense>
  );
}
