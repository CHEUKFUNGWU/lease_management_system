"use client";

import { useCallback, useEffect, useMemo, useRef, useState, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import {
  Alert, Button, Card, Col, Collapse, DatePicker, Empty, Flex, Input, Radio, Row, Select, Segmented, Space, Spin, Table, Tag, Tooltip, Typography, message,
} from "antd";
import { ArrowDownOutlined, ArrowUpOutlined, EyeOutlined, ReloadOutlined } from "@ant-design/icons";
import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip as ChartTooltip, XAxis, YAxis } from "recharts";
import dayjs from "dayjs";
import AppLayout from "../components/AppLayout";
import { SeverityDot, toSeverity } from "../components/SeverityDot";
import PageHeader from "../components/PageHeader";
import DataTrustBar, { KPIReadyBadge } from "../components/DataTrustBar";
import RetailAIDrawer from "../components/RetailAIDrawer";
import ProtectedRoute from "../components/ProtectedRoute";
import { StatusTag } from "../components/StatusTag";
import { hasRole, useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t, type Language } from "../lib/i18n";
import { apiErrorMessage, retailAnalyticsApi, type RetailAttention, type RetailCoverage, type RetailDailyTrend, type RetailPulsePartition, type RetailPulseResponse, type RetailSimulationDatasetData, type RetailStoreScope, type RetailSuppressedAttention, type RetailSummaryMetric } from "../lib/api";
import { changeTone, formatChange, formatKPIValue, formatSignalValue, kpiLabel, latestAnomalyDate, metricStatusLabel, metricUnitLabel, PULSE_AUXILIARY_CODES, PULSE_KPI_CODES, responsePartitions, signalLabel, switchClassification, trendValue, type PulseMetricCode } from "./logic";
import { createLatestRequestGate } from "./requestGate";

const WINDOW_OPTIONS = [7, 14, 28] as const;
const TODAY = dayjs().format("YYYY-MM-DD");

function updateQuery(router: ReturnType<typeof useRouter>, params: { classification: "production" | "simulated"; datasetVersion?: string; asOf: string; windowDays: number; storeIDs: string[]; sourceSystem?: string }) {
  const query = new URLSearchParams();
  query.set("data_classification", params.classification);
  if (params.classification === "simulated" && params.datasetVersion) query.set("dataset_version", params.datasetVersion);
  query.set("as_of", params.asOf);
  query.set("window_days", String(params.windowDays));
  if (params.sourceSystem) query.set("source_system", params.sourceSystem);
  params.storeIDs.forEach((storeID) => query.append("store_id", storeID));
  router.replace(`/operating-pulse?${query.toString()}`);
}

function coverageText(coverage: RetailCoverage): string {
  const rate = coverage.coverage_rate == null ? "—" : `${coverage.coverage_rate.toFixed(1)}%`;
  return `${coverage.observed_store_days}/${coverage.expected_store_days} store-days · ${rate}`;
}

function effectivePartition(response: RetailPulseResponse, selectedCurrency: string): RetailPulsePartition | null {
  const partitions = responsePartitions(response);
  return partitions.find((item) => item.currency === selectedCurrency) || partitions[0] || null;
}

function KPIValueCard({ code, metric, currency, notReady, language }: { code: PulseMetricCode; metric?: RetailPulseResponse["summary"] extends infer T ? T extends Record<string, infer M> ? M : never : never; currency: string; notReady?: boolean; language: Language }) {
  if (!metric) return <Card size="small" className="pulse-kpi-card"><Typography.Text type="secondary">{kpiLabel(code, language)}</Typography.Text><div className="pulse-kpi-null">—</div></Card>;
  const tone = changeTone(code, metric);
  const arrow = metric.change_value == null ? undefined : metric.change_value < 0 ? <ArrowDownOutlined /> : metric.change_value > 0 ? <ArrowUpOutlined /> : undefined;
  const status = metricStatusLabel(metric, language);
  const statusKind = status.status === "complete" ? "neutral" : "warning";
  // FIX-003: the value line truncates with a tooltip instead of wrapping,
  // so a long figure can never stretch the fixed-height card.
  const display = formatKPIValue(metric.current, currency, language);
  return <Card size="small" className="pulse-kpi-card" data-testid={`pulse-kpi-${code}`}>
    <Flex justify="space-between" align="start" gap={8}><Typography.Text type="secondary">{kpiLabel(code, language)}</Typography.Text><Flex align="center" gap={4}>{notReady && <KPIReadyBadge />}<Tooltip title={status.reason}><StatusTag kind={statusKind}>{status.label}</StatusTag></Tooltip></Flex></Flex>
    <Typography.Title level={3} className="pulse-kpi-value" ellipsis={{ tooltip: display }}>{display}</Typography.Title>
    <Typography.Text className={`pulse-change pulse-change-${tone}`}>{arrow} {formatChange(metric)} {status.reason ? `· ${status.reason}` : ""}</Typography.Text>
    <Typography.Text type="secondary" className="pulse-kpi-comparison">{t("common.contrast", language)} {formatKPIValue(metric.comparison, currency, language)}</Typography.Text>
  </Card>;
}

function AuxiliaryMetricRow({ code, metric, currency, language }: { code: PulseMetricCode; metric?: RetailSummaryMetric; currency?: string; language: Language }) {
  const status = metricStatusLabel(metric, language);
  const tone = changeTone(code, metric);
  const statusKind = status.status === "complete" ? "neutral" : "warning";
  return <div className="pulse-aux-row">
    <div><Typography.Text>{kpiLabel(code, language)}</Typography.Text><div className="pulse-aux-status"><StatusTag kind={statusKind}>{status.label}</StatusTag>{status.reason && <Typography.Text type="secondary">{status.reason}</Typography.Text>}</div></div>
    <div className="pulse-aux-values"><strong>{formatKPIValue(metric?.current, currency, language)}</strong><Typography.Text type="secondary">{t("common.contrast", language)} {formatKPIValue(metric?.comparison, currency, language)}</Typography.Text></div>
    <Typography.Text className={`pulse-change pulse-change-${tone}`}>{formatChange(metric)}</Typography.Text>
  </div>;
}

function trendUnit(code: PulseMetricCode): string {
  if (code === "revenue" || code === "gross_profit" || code === "store_contribution" || code === "average_transaction_value") return "currency";
  if (code.endsWith("rate") || code === "gross_margin_rate" || code === "conversion_rate" || code === "store_contribution_margin") return "percent";
  if (code === "sales_per_sqm") return "currency_per_sqm";
  return "count";
}

function TrendChart({ trend, code, currency, onMetricChange, language }: { trend: RetailDailyTrend[]; code: PulseMetricCode; currency: string; onMetricChange: (code: PulseMetricCode) => void; language: Language }) {
  const chartData = trend.map((row) => ({ date: row.date.slice(5), value: trendValue(row, code), gap: row.gap }));
  return <Card title={<Flex justify="space-between" align="center" wrap="wrap" gap={8}><span>{t("pulse.trend_title", language)}</span><Segmented size="small" value={code} onChange={(value) => onMetricChange(value as PulseMetricCode)} options={PULSE_KPI_CODES.map((item) => ({ label: kpiLabel(item, language), value: item }))} /></Flex>}>
    <div style={{ height: 270, width: "100%" }}>
      {trend.length === 0 ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("common.no_trend", language)} /> : <ResponsiveContainer width="100%" height="100%"><LineChart data={chartData} margin={{ top: 8, right: 12, left: 0, bottom: 4 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--border-subtle)" />
        <XAxis dataKey="date" tick={{ fontSize: 11 }} />
        <YAxis tick={{ fontSize: 11 }} tickFormatter={(value) => value == null ? "—" : Number(value).toLocaleString()} />
        <ChartTooltip formatter={(value, _name, item) => item?.payload?.gap ? [t("store360.trend.data_gap", language), t("pulse.col.status", language)] : [`${value == null ? "—" : Number(value).toLocaleString()} ${metricUnitLabel(trendUnit(code), currency, language)}`, kpiLabel(code, language)]} />
        <Line type="monotone" dataKey="value" stroke="var(--chart-blue)" strokeWidth={2} dot={false} connectNulls={false} />
      </LineChart></ResponsiveContainer>}
    </div>
  </Card>;
}

function AttentionTable({ attention, onSelect, onStore360, language }: { attention: RetailAttention[]; onSelect: (storeID: string) => void; onStore360: (storeID: string) => void; language: Language }) {
  const columns = [
    { title: t("pulse.col.priority", language), dataIndex: "rank", width: 56, render: (value: number) => <strong>#{value}</strong> },
    { title: t("pulse.col.store", language), key: "store", render: (_: unknown, row: RetailAttention) => <Space direction="vertical" size={0}><strong>{row.store_code} · {row.store_name}</strong><Typography.Text type="secondary">{row.brand} · {row.region}</Typography.Text></Space> },
    { title: t("pulse.col.signal", language), key: "signals", render: (_: unknown, row: RetailAttention) => <Space wrap size={[4, 4]}>{row.observed_signals.map((signal) => <Tooltip key={signal.signal_code} title={`${signal.signal_code} · ${t("common.threshold", language)} ${formatSignalValue(signal.threshold, signal.unit, row.currency, language)}`}><span className="severity-label"><SeverityDot severity={toSeverity(row.severity)} />{signalLabel(signal.signal_code, language)}</span></Tooltip>)}</Space> },
    { title: t("pulse.col.change", language), key: "change", render: (_: unknown, row: RetailAttention) => <Space direction="vertical" size={0}>{row.observed_signals.map((signal) => <Tooltip key={signal.signal_code} title={`${t("common.current", language)} ${formatSignalValue(signal.current, signal.unit, row.currency, language)} · ${t("common.contrast", language)} ${formatSignalValue(signal.comparison, signal.unit, row.currency, language)} · ${t("common.threshold", language)} ${formatSignalValue(signal.threshold, signal.unit, row.currency, language)}`}><Typography.Text className="pulse-change-bad">{signalLabel(signal.signal_code, language)} {formatSignalValue(signal.observed_change, signal.unit, row.currency, language)} · {t("common.threshold", language)} {formatSignalValue(signal.threshold, signal.unit, row.currency, language)}</Typography.Text></Tooltip>)}</Space> },
    { title: t("pulse.col.score", language), key: "score", render: (_: unknown, row: RetailAttention) => <Space direction="vertical" size={0}><StatusTag kind={row.severity === "critical" || row.severity === "high" ? "error" : "warning"}>{row.severity}</StatusTag><span>{row.score.toFixed(2)}</span></Space> },
    { title: t("pulse.col.source", language), key: "source", render: (_: unknown, row: RetailAttention) => <Typography.Text type="secondary">{row.evidence.source_systems.join(", ") || "—"}</Typography.Text> },
    { title: t("pulse.col.action", language), key: "action", width: 220, render: (_: unknown, row: RetailAttention) => <Space><Button size="small" icon={<EyeOutlined />} onClick={() => onSelect(row.store_id)}>{t("pulse.view_store_pulse", language)}</Button><Button size="small" onClick={() => onStore360(row.store_id)}>{t("common.store360", language)}</Button></Space> },
  ];
  return attention.length ? <Table rowKey="store_id" size="small" columns={columns} dataSource={attention} pagination={false} scroll={{ x: 980 }} /> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("pulse.no_signals", language)} />;
}

function SuppressedPanel({ items, language }: { items: RetailSuppressedAttention[]; language: Language }) {
  if (!items.length) return null;
  return <Collapse items={[{ key: "suppressed", label: `${t("pulse.suppressed_title", language)}（${items.length}）`, children: <Table size="small" rowKey={(row: RetailSuppressedAttention) => `${row.store_id}-${row.reason}`} pagination={false} scroll={{ x: 760 }} dataSource={items} columns={[{ title: t("pulse.col.store", language), render: (_: unknown, row: RetailSuppressedAttention) => `${row.store_code} · ${row.store_name}` }, { title: t("pulse.col.brand_region", language), render: (_: unknown, row: RetailSuppressedAttention) => `${row.brand || "—"} · ${row.region || "—"}` }, { title: t("pulse.col.reason", language), render: (_: unknown, row: RetailSuppressedAttention) => <Space wrap>{(row.reasons || [row.reason]).map((reason) => <Tag key={reason}>{reason}</Tag>)}</Space> }, { title: t("pulse.col.coverage", language), render: (_: unknown, row: RetailSuppressedAttention) => `${coverageText(row.current_coverage)} · ${coverageText(row.comparison_coverage)}` }]} /> }]} />;
}

function OperatingPulseInner() {
  const { token, user } = useAuth();
  const { language } = useLanguage();
  const [aiOpen, setAiOpen] = useState(false);
  const router = useRouter();
  const searchParams = useSearchParams();
  const [latest, setLatest] = useState<RetailSimulationDatasetData | null | undefined>(undefined);
  const [response, setResponse] = useState<RetailPulseResponse | null>(null);
  const [selectedCurrency, setSelectedCurrency] = useState("");
  const [trendMetric, setTrendMetric] = useState<PulseMetricCode>("revenue");
  const [loading, setLoading] = useState(true);
  const [generating, setGenerating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const requestGate = useRef(createLatestRequestGate());
  const latestLoaded = useRef(false);
  const [refreshNonce, setRefreshNonce] = useState(0);
  const [latestRetryNonce, setLatestRetryNonce] = useState(0);

  const classification = (searchParams.get("data_classification") || "") as "production" | "simulated" | "";
  const datasetVersion = searchParams.get("dataset_version") || "";
  const asOf = searchParams.get("as_of") || "";
  const windowDays = Number(searchParams.get("window_days") || 7);
  const storeIDs = searchParams.getAll("store_id");
  const sourceSystem = searchParams.get("source_system") || "";
  const validWindow = WINDOW_OPTIONS.includes(windowDays as (typeof WINDOW_OPTIONS)[number]);
  const currentClassification = classification === "production" || classification === "simulated" ? classification : "simulated";

  const loadPulse = useCallback(async (params: { classification: "production" | "simulated"; datasetVersion?: string; asOf: string; windowDays: 7 | 14 | 28; storeIDs: string[]; sourceSystem?: string }) => {
    if (!token) return;
    const id = requestGate.current.begin();
    setLoading(true);
    setError(null);
    try {
      const result = await retailAnalyticsApi.operatingPulse({ data_classification: params.classification, dataset_version: params.datasetVersion, as_of: params.asOf, window_days: params.windowDays, store_ids: params.storeIDs, source_system: params.sourceSystem }, token);
      requestGate.current.commit(id, () => {
        setResponse(result);
        const partitions = responsePartitions(result);
        setSelectedCurrency((current) => current && partitions.some((item) => item.currency === current) ? current : (partitions[0]?.currency || ""));
      });
    } catch (loadError) {
      requestGate.current.commit(id, () => setError(apiErrorMessage(loadError)));
    } finally {
      requestGate.current.commit(id, () => setLoading(false));
    }
  }, [token]);

  useEffect(() => {
    if (!token || latestLoaded.current) return;
    latestLoaded.current = true;
    retailAnalyticsApi.latestSimulationDataset(token).then((result) => {
      setLatest(result.data);
      // Only an entirely parameterless entry gets automatic simulated
      // discovery. Explicit production/simulated URLs remain the source of
      // truth while the discovery request populates the scenario selector.
      if (!searchParams.toString() && result.data) {
        updateQuery(router, { classification: "simulated", datasetVersion: result.data.dataset_version, asOf: latestAnomalyDate(result.data), windowDays: 7, storeIDs: [], sourceSystem: "retail_simulator" });
      } else if (result.data && searchParams.get("data_classification") === "simulated" && !searchParams.get("dataset_version")) {
        // Direct simulated URLs still use latest discovery to hydrate the
        // scenario selector and complete the canonical query before fetching.
        updateQuery(router, { classification: "simulated", datasetVersion: result.data.dataset_version, asOf: searchParams.get("as_of") || latestAnomalyDate(result.data), windowDays: validWindow ? windowDays : 7, storeIDs, sourceSystem: sourceSystem || "retail_simulator" });
      } else if (!result.data && !searchParams.toString()) {
        setLoading(false);
      }
    }).catch((loadError) => { latestLoaded.current = false; setError(apiErrorMessage(loadError)); setLoading(false); });
  }, [router, searchParams, token, storeIDs, validWindow, windowDays, latestRetryNonce]);

  const storeKey = storeIDs.join("\x1f");
  const pulseKey = `${classification}|${datasetVersion}|${asOf}|${windowDays}|${sourceSystem}|${storeKey}|${refreshNonce}`;
  useEffect(() => {
    if (!token || (classification !== "production" && classification !== "simulated")) return;
    if (classification === "simulated" && !datasetVersion) { setError(t("pulse.err_missing_dataset_version", language)); setLoading(false); return; }
    if (!validWindow) { setResponse(null); setError(t("pulse.err_invalid_window", language)); setLoading(false); return; }
    if (!asOf) {
      if (classification === "production") updateQuery(router, { classification: "production", asOf: TODAY, windowDays, storeIDs, sourceSystem });
      return;
    }
    setResponse(null);
    setSelectedCurrency("");
    loadPulse({ classification, datasetVersion: datasetVersion || undefined, asOf, windowDays: windowDays as 7 | 14 | 28, storeIDs, sourceSystem });
    // pulseKey intentionally serializes all URL query state, including
    // repeated store_id values, while loadPulse remains token-stable.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pulseKey, token, validWindow, loadPulse, router]);

  const generate = async () => {
    if (!token) return;
    setGenerating(true);
    try {
      const generated = await retailAnalyticsApi.generateDefaultSimulation(token);
      message.success(t("pulse.demo_generated", language));
      const generatedDataset: RetailSimulationDatasetData = {
        id: generated.dataset_id,
        dataset_version: generated.dataset_version,
        generator_version: generated.generator_version,
        seed: generated.seed,
        date_from: generated.date_from,
        date_to: generated.date_to,
        store_count: generated.store_count,
        fact_count: generated.fact_count,
        status: "completed",
        anomaly_manifest: generated.anomaly_manifest,
        completed_at: generated.source.created_at,
        created_at: generated.source.created_at,
      };
      setLatest(generatedDataset);
      updateQuery(router, { classification: "simulated", datasetVersion: generated.dataset_version, asOf: latestAnomalyDate(generatedDataset), windowDays: 7, storeIDs: [], sourceSystem: "retail_simulator" });
    } catch (generationError) { message.error(apiErrorMessage(generationError)); } finally { setGenerating(false); }
  };

  const partition = response ? effectivePartition(response, selectedCurrency) : null;
  const partitions = response ? responsePartitions(response) : [];
  const isEmptyInitial = latest === null && !response && !error;
  const isScoped = storeIDs.length > 0;
  const displaySummary = partition?.summary || response?.summary || {};
  const latestMetadata = latest && currentClassification === "simulated" && latest.dataset_version === datasetVersion ? latest : undefined;
  const anomalies = latestMetadata?.anomaly_manifest || [];
  const currentAnomaly = anomalies.find((item) => item.date_to === asOf);
  const scopedStore: RetailStoreScope | RetailAttention | RetailSuppressedAttention | undefined = isScoped
    ? response?.requested_stores?.find((store) => store.store_id === storeIDs[0]) || partition?.attention.find((store) => store.store_id === storeIDs[0]) || partition?.suppressed_attention?.find((store) => store.store_id === storeIDs[0])
    : undefined;
  const scopedTitle = scopedStore ? `${scopedStore.store_code} · ${scopedStore.store_name} · ${scopedStore.brand || "—"} · ${scopedStore.region || "—"}` : storeIDs.join(", ");
  const noFacts = Boolean(response && partition && !response.decision_ready && partition.current_coverage.observed_store_days === 0 && partition.comparison_coverage.observed_store_days === 0);

  const onWindowChange = (value: number) => {
    if (!asOf || !WINDOW_OPTIONS.includes(value as (typeof WINDOW_OPTIONS)[number])) return;
    updateQuery(router, { classification: currentClassification, datasetVersion: currentClassification === "simulated" ? datasetVersion : undefined, asOf: asOf || TODAY, windowDays: value, storeIDs, sourceSystem });
  };
  const onStoreSelect = (storeID: string) => updateQuery(router, { classification: currentClassification, datasetVersion: currentClassification === "simulated" ? datasetVersion : undefined, asOf: asOf || TODAY, windowDays: validWindow ? windowDays : 7, storeIDs: [storeID], sourceSystem });
  const clearStore = () => updateQuery(router, { classification: currentClassification, datasetVersion: currentClassification === "simulated" ? datasetVersion : undefined, asOf: asOf || TODAY, windowDays: validWindow ? windowDays : 7, storeIDs: [], sourceSystem });
  const refresh = () => { latestLoaded.current = false; setLatestRetryNonce((value) => value + 1); setRefreshNonce((value) => value + 1); };

  const kpiCards = PULSE_KPI_CODES.map((code) => <Col xs={24} sm={12} lg={8} xl={4} key={code}><KPIValueCard code={code} metric={displaySummary[code]} currency={partition?.currency || response?.currency || ""} notReady={!response?.decision_ready} language={language} /></Col>);
  const aux = PULSE_AUXILIARY_CODES.map((code) => <AuxiliaryMetricRow key={code} code={code} metric={displaySummary[code]} language={language} currency={partition?.currency || response?.currency} />);

  const onClassificationChange = (next: "production" | "simulated") => {
    const switchResult = switchClassification(next, latest, asOf, TODAY);
    if (switchResult.clearToEmpty) {
      setResponse(null);
      setError(null);
      setLoading(false);
      router.replace("/operating-pulse");
      return;
    }
    updateQuery(router, { classification: switchResult.classification, datasetVersion: switchResult.datasetVersion, asOf: switchResult.asOf, windowDays: 7, storeIDs: [], sourceSystem: switchResult.sourceSystem });
  };

  return <ProtectedRoute><AppLayout><div className="operating-pulse-page">
    <PageHeader title={t("pulse.title", language)} subtitle={t("pulse.subtitle", language)} primaryAction={<Button icon={<ReloadOutlined />} onClick={refresh} loading={loading}>{t("common.refresh", language)}</Button>} secondaryAction={<Button onClick={() => setAiOpen(true)}>{t("common.ai_analysis", language)}</Button>} />
    <Card size="small" className="pulse-filter-card" style={{ marginBottom: 16 }}>
      <Flex gap={12} wrap="wrap" align="center">
        <Radio.Group value={currentClassification} onChange={(event) => onClassificationChange(event.target.value as "production" | "simulated")} optionType="button" buttonStyle="solid" options={[{ label: t("retail.classification.simulated", language), value: "simulated" }, { label: t("retail.classification.production", language), value: "production" }]} />
        {currentClassification === "simulated" && anomalies.length > 0 && <Select aria-label={t("pulse.anomaly_select", language)} value={currentAnomaly?.id || "all"} style={{ minWidth: 220 }} options={[{ label: t("pulse.all_anomalies", language), value: "all" }, ...anomalies.map((item) => ({ label: `${item.store_code} · ${signalLabel(item.type, language)}`, value: item.id }))]} onChange={(id) => { const selected = anomalies.find((item) => item.id === id); if (selected) updateQuery(router, { classification: "simulated", datasetVersion, asOf: selected.date_to, windowDays: 7, storeIDs, sourceSystem }); }} />}
        <DatePicker aria-label={t("pulse.as_of", language)} value={asOf ? dayjs(asOf) : undefined} onChange={(date) => date && updateQuery(router, { classification: currentClassification, datasetVersion: currentClassification === "simulated" ? datasetVersion : undefined, asOf: date.format("YYYY-MM-DD"), windowDays: validWindow ? windowDays : 7, storeIDs, sourceSystem })} allowClear={false} />
        <Input aria-label={t("common.source_system", language)} allowClear placeholder={t("common.source_system_optional", language)} value={sourceSystem} onChange={(event) => updateQuery(router, { classification: currentClassification, datasetVersion: currentClassification === "simulated" ? datasetVersion : undefined, asOf: asOf || TODAY, windowDays: validWindow ? windowDays : 7, storeIDs, sourceSystem: event.target.value.trim() })} style={{ width: 180 }} />
        <Segmented aria-label={t("pulse.window", language)} value={validWindow ? windowDays : 7} onChange={onWindowChange} options={WINDOW_OPTIONS.map((item) => ({ label: `${item}${t("common.days_suffix", language)}`, value: item }))} />
        {isScoped ? <Button onClick={clearStore}>{t("pulse.back_all_stores", language)}</Button> : <Tag>{t("pulse.all_authorized_stores", language)}</Tag>}
      </Flex>
    </Card>
    {isEmptyInitial && <Card><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={<Space direction="vertical"><Typography.Text strong>{t("pulse.no_dataset_title", language)}</Typography.Text><Typography.Text type="secondary">{t("pulse.no_dataset_desc", language)}</Typography.Text>{hasRole(user, "admin") ? <Button type="primary" loading={generating} onClick={generate}>{t("pulse.generate_demo", language)}</Button> : <Typography.Text>{t("pulse.contact_admin", language)}</Typography.Text>}</Space>} /></Card>}
    {error && <Alert type="error" showIcon message={t("pulse.unavailable_title", language)} description={error} action={<Button size="small" onClick={refresh}>{t("common.retry", language)}</Button>} />}
    {loading && !response && !isEmptyInitial && <Card><Flex justify="center" align="center" style={{ minHeight: 220 }}><Spin tip={t("pulse.loading", language)} /></Flex></Card>}
      {response && partition && <>
      <DataTrustBar envelope={response.envelope} basis={response.basis} detailExtra={<span>generator: {latestMetadata?.generator_version || "—"}</span>} />
      {noFacts ? <Card style={{ marginTop: 16 }}><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={<Space direction="vertical"><Typography.Text strong>{t("pulse.no_facts_title", language)}</Typography.Text><Typography.Text type="secondary">{t("pulse.no_facts_desc", language)}</Typography.Text></Space>} /></Card> : <>
      {response.multi_currency && <Card size="small" style={{ marginTop: 16 }}><Flex align="center" gap={8}><Typography.Text strong>{t("pulse.currency_partition", language)}</Typography.Text><Segmented value={selectedCurrency} onChange={(value) => setSelectedCurrency(String(value))} options={partitions.map((item) => ({ label: item.currency || t("pulse.unknown_currency", language), value: item.currency }))} /></Flex></Card>}
      <Row gutter={[12, 12]} style={{ marginTop: 16 }}>{kpiCards}</Row>
      <Row gutter={[16, 16]} style={{ marginTop: 16 }}><Col xs={24} lg={16}><TrendChart language={language} trend={partition.daily_trend} code={trendMetric} currency={partition.currency || response.currency || ""} onMetricChange={setTrendMetric} /></Col><Col xs={24} lg={8}><Card title={t("pulse.aux_metrics", language)}><Space direction="vertical" size={0} style={{ width: "100%" }}>{aux}</Space><Alert type="info" showIcon style={{ marginTop: 16 }} message={t("pulse.cash_basis_title", language)} description={t("pulse.cash_basis_desc", language)} /></Card></Col></Row>
      <Card title={<Flex justify="space-between" align="center"><span>{isScoped ? `${t("pulse.store_pulse_title", language)} · ${scopedTitle}` : t("pulse.priority_stores", language)}</span><Typography.Text type="secondary">{t("pulse.api_order", language)}</Typography.Text></Flex>} style={{ marginTop: 16 }}><AttentionTable language={language} attention={partition.attention} onSelect={onStoreSelect} onStore360={(storeID) => { const returnQuery = new URLSearchParams(searchParams.toString()); const params = new URLSearchParams(searchParams.toString()); params.delete("store_id"); params.set("store_id", storeID); params.set("return_query", returnQuery.toString()); router.push(`/store-360?${params.toString()}`); }} /></Card>
      <div style={{ marginTop: 16 }}><SuppressedPanel language={language} items={partition.suppressed_attention || []} /></div>
      </>}
    </>}
    <RetailAIDrawer open={aiOpen} onClose={() => setAiOpen(false)} pageContext={{ page: "operating-pulse", title: t("pulse.title", language), filters: { as_of: asOf || TODAY, window_days: String(validWindow ? windowDays : 7), classification: currentClassification || "", dataset_version: currentClassification === "simulated" ? datasetVersion : "", source_system: sourceSystem || "", store_ids: storeIDs.join(",") } }} />
  </div></AppLayout></ProtectedRoute>;
}

export default function OperatingPulsePage() {
  return <Suspense fallback={<div style={{ minHeight: "100vh", display: "grid", placeItems: "center" }}><Spin /></div>}><OperatingPulseInner /></Suspense>;
}
