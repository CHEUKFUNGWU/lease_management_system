"use client";

import { useEffect, useMemo, useState, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Alert, Button, Card, Col, Collapse, DatePicker, Empty, Flex, Input, Radio, Row, Select, Segmented, Space, Spin, Table, Tag, Typography } from "antd";
import { ArrowLeftOutlined, ReloadOutlined } from "@ant-design/icons";
import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip as ChartTooltip, XAxis, YAxis } from "recharts";
import dayjs from "dayjs";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import DataTrustBar, { KPIReadyBadge } from "../components/DataTrustBar";
import RetailAIDrawer from "../components/RetailAIDrawer";
import ProtectedRoute from "../components/ProtectedRoute";
import { StatusTag } from "../components/StatusTag";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t, type Language } from "../lib/i18n";
import { apiErrorMessage, retailAnalyticsApi, type RetailDataClassification, type RetailSimulationDatasetData, type RetailStore360Option, type RetailStoreDiagnosticsResponse, type RetailSummaryMetric } from "../lib/api";
import { changeTone, formatChange, formatKPIValue, kpiLabel, latestAnomalyDate, type PulseMetricCode } from "../operating-pulse/logic";
import { bridgeConservation, bridgeTone, displayMetric, formatBridgeItem, formatPeerBenchmarkStatus, formatTrendTooltip, optionFields, returnPulseQuery, STORE360_AUX_CODES, STORE360_CODES, summaryStatus, trendValue, validWindow, WINDOW_OPTIONS } from "./logic";

const TODAY = dayjs().format("YYYY-MM-DD");

function queryFromURL(searchParams: URLSearchParams) {
  const classification = searchParams.get("data_classification") as RetailDataClassification | null;
  const datasetVersion = searchParams.get("dataset_version") || "";
  const asOf = searchParams.get("as_of") || "";
  const rawWindow = Number(searchParams.get("window_days") || 14);
  return { storeID: searchParams.get("store_id") || "", classification: classification === "production" || classification === "simulated" ? classification : "", datasetVersion, asOf, windowDays: rawWindow, sourceSystem: searchParams.get("source_system") || "", returnQuery: searchParams.get("return_query") || "" };
}

function writeQuery(router: ReturnType<typeof useRouter>, value: { storeID?: string; classification: RetailDataClassification; datasetVersion?: string; asOf: string; windowDays: number; sourceSystem?: string; returnQuery?: string }) {
  const query = new URLSearchParams();
  if (value.storeID) query.set("store_id", value.storeID);
  query.set("data_classification", value.classification);
  if (value.classification === "simulated" && value.datasetVersion) query.set("dataset_version", value.datasetVersion);
  query.set("as_of", value.asOf);
  query.set("window_days", String(validWindow(value.windowDays) ? value.windowDays : 14));
  if (value.sourceSystem) query.set("source_system", value.sourceSystem);
  if (value.returnQuery) query.set("return_query", value.returnQuery);
  router.replace(`/store-360?${query.toString()}`);
}

function Status({ metric, language }: { metric?: RetailStoreDiagnosticsResponse["summary"][string]; language: Language }) {
  const label = summaryStatus(metric, language).label;
  const kind = summaryStatus(metric, language).status === "complete" ? "neutral" : "warning";
  return <StatusTag kind={kind}>{label}</StatusTag>;
}

function MetricCard({ code, metric, currency, notReady, language }: { code: PulseMetricCode; metric?: RetailStoreDiagnosticsResponse["summary"][string]; currency: string; notReady?: boolean; language: Language }) {
  const tone = changeTone(code, metric as RetailSummaryMetric | undefined);
  const reason = metric?.reason || metric?.current.reason || metric?.comparison.reason;
  return <Card size="small" className="store-360-kpi-card" data-testid={`store360-kpi-${code}`}>
    <Flex justify="space-between" align="center"><Typography.Text type="secondary">{kpiLabel(code, language)}</Typography.Text><Flex align="center" gap={4}>{notReady && <KPIReadyBadge />}<Status metric={metric} language={language} /></Flex></Flex>
    <Typography.Title level={3} style={{ margin: "12px 0 4px", fontVariantNumeric: "tabular-nums" }}>{displayMetric(metric, currency, language)}</Typography.Title>
    <Typography.Text className={`pulse-change pulse-change-${tone}`}>{formatChange(metric as RetailSummaryMetric | undefined)} {reason ? `· ${reason}` : ""}</Typography.Text>
    <Typography.Text type="secondary" className="pulse-kpi-comparison">{t("common.contrast", language)} {formatKPIValue(metric?.comparison, currency, language)}</Typography.Text>
  </Card>;
}

function Trend({ response, language }: { response: RetailStoreDiagnosticsResponse; language: Language }) {
  const [code, setCode] = useState<PulseMetricCode>("revenue");
  const data = response.daily_trend.map((row) => ({ date: row.date.slice(5), target: trendValue(row, code), peer: row.peer_median[code] ?? null, gap: row.gap }));
  return <Card title={<Flex justify="space-between" align="center" wrap="wrap" gap={8}><span>{t("store360.trend_title", language)}</span><Segmented size="small" value={code} onChange={(value) => setCode(value as PulseMetricCode)} options={STORE360_CODES.map((item) => ({ label: kpiLabel(item, language), value: item }))} /></Flex>}>
    <div style={{ height: 270 }}>
      {data.length === 0 ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("common.no_trend", language)} /> : <ResponsiveContainer width="100%" height="100%"><LineChart data={data} margin={{ top: 8, right: 12, left: 0, bottom: 4 }}><CartesianGrid strokeDasharray="3 3" stroke="var(--border-subtle)" /><XAxis dataKey="date" tick={{ fontSize: 11 }} /><YAxis tick={{ fontSize: 11 }} /><ChartTooltip formatter={(value, name, item) => formatTrendTooltip(value == null ? null : Number(value), String(name), Boolean(item?.payload?.gap), response.summary[code]?.current.unit || "", response.currency, language)} /><Line type="monotone" dataKey="target" stroke="var(--chart-blue)" strokeWidth={2} dot={false} connectNulls={false} name="target" /><Line type="monotone" dataKey="peer" stroke="var(--chart-purple)" strokeDasharray="5 5" strokeWidth={2} dot={false} connectNulls={false} name="peer" /></LineChart></ResponsiveContainer>}
    </div>
  </Card>;
}

function BridgePanel({ bridges, currency, language }: { bridges: RetailStoreDiagnosticsResponse["bridges"]; currency: string; language: Language }) {
  return <Card title={t("store360.bridge.title", language)}><Space direction="vertical" size={12} style={{ width: "100%" }}>{bridges.map((bridge) => <Card size="small" key={bridge.code} title={<Flex justify="space-between"><span>{bridge.code}</span><StatusTag kind={bridge.status === "complete" ? "neutral" : "warning"}>{bridge.status === "complete" ? t("store360.bridge.complete", language) : t("store360.bridge.unavailable", language)}</StatusTag></Flex>}>
    {bridge.status !== "complete" ? <Typography.Text type="secondary">{bridge.reason || t("store360.bridge.reason_default", language)}</Typography.Text> : <><Flex wrap="wrap" gap={8} style={{ marginBottom: 8 }}><Tag>{t("store360.bridge.contrast", language)} {formatBridgeItem(bridge.comparison, "currency", currency, language)}</Tag><Tag>{t("store360.bridge.current", language)} {formatBridgeItem(bridge.current, "currency", currency, language)}</Tag><Tag>{t("store360.bridge.change", language)} {formatBridgeItem(bridge.total_change, "currency", currency, language)}</Tag><Tag>{t("store360.bridge.residual", language)} {formatBridgeItem(bridge.rounding_residual, "currency", currency, language)}</Tag></Flex><Table size="small" pagination={false} rowKey="code" dataSource={bridge.items} columns={[{ title: t("store360.bridge.item", language), dataIndex: "label" }, { title: t("store360.bridge.contribution", language), render: (_: unknown, row: typeof bridge.items[number]) => <Typography.Text className={`store-360-bridge-${bridgeTone(row.contribution)}`}>{formatBridgeItem(row.contribution, row.unit, currency, language)}</Typography.Text> }]} /></>}
  </Card>)}</Space></Card>;
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
  const [response, setResponse] = useState<RetailStoreDiagnosticsResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [discoveryLoading, setDiscoveryLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [optionsError, setOptionsError] = useState<string | null>(null);
  const [retry, setRetry] = useState(0);
  const [sourceInput, setSourceInput] = useState(query.sourceSystem);

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

  useEffect(() => {
    if (!token || !query.storeID || !query.classification || !query.asOf || !validWindow(query.windowDays) || (query.classification === "simulated" && !query.datasetVersion)) {
      setResponse(null);
      return;
    }
    let active = true;
    setLoading(true);
    setError(null);
    setResponse(null);
    retailAnalyticsApi.storeDiagnostics({ store_id: query.storeID, data_classification: query.classification as RetailDataClassification, dataset_version: query.datasetVersion || undefined, as_of: query.asOf, window_days: query.windowDays, source_system: query.sourceSystem || undefined }, token).then((result) => { if (active) setResponse(result); }).catch((err) => { if (active) setError(apiErrorMessage(err)); }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [token, query.storeID, query.classification, query.datasetVersion, query.asOf, query.windowDays, query.sourceSystem, retry]);

  useEffect(() => {
    setSourceInput(query.sourceSystem);
  }, [query.sourceSystem]);

  useEffect(() => {
    if (query.classification || !latest || discoveryLoading) return;
    writeQuery(router, { classification: "simulated", datasetVersion: latest.dataset_version, asOf: latestAnomalyDate(latest), windowDays: 14, returnQuery: query.returnQuery });
  }, [query.classification, latest, discoveryLoading, router]);

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

  const change = (next: Partial<typeof query>) => writeQuery(router, { classification: (next.classification || query.classification || "simulated") as RetailDataClassification, datasetVersion: next.datasetVersion ?? query.datasetVersion, asOf: next.asOf || query.asOf || TODAY, windowDays: next.windowDays ?? (validWindow(query.windowDays) ? query.windowDays : 14), sourceSystem: next.sourceSystem ?? query.sourceSystem, storeID: next.storeID ?? query.storeID, returnQuery: query.returnQuery });

  return <ProtectedRoute><AppLayout><div className="store-360-page">
    <PageHeader title={t("store360.title", language)} subtitle={t("store360.subtitle", language)} primaryAction={<Button icon={<ReloadOutlined />} loading={loading || discoveryLoading} onClick={() => setRetry((value) => value + 1)}>{t("common.refresh", language)}</Button>} secondaryAction={<Space><Button onClick={() => setAiOpen(true)}>{t("common.ai_analysis", language)}</Button><Button onClick={() => router.push(scenarioURL)}>{t("store360.scenario_analysis", language)}</Button><Button icon={<ArrowLeftOutlined />} onClick={() => router.push(backURL)}>{t("store360.back_pulse", language)}</Button></Space>} />
    <Card size="small" className="store-360-filter-card">
      <Flex gap={12} wrap="wrap" align="center">
        <Radio.Group value={query.classification || "simulated"} onChange={(event) => { const next = event.target.value as RetailDataClassification; if (next === "production") change({ classification: next, datasetVersion: "", asOf: TODAY }); else if (latest) change({ classification: next, datasetVersion: latest.dataset_version, asOf: latestAnomalyDate(latest) }); else change({ classification: next, datasetVersion: "" }); }} optionType="button" buttonStyle="solid" options={[{ label: t("retail.classification.simulated", language), value: "simulated" }, { label: t("retail.classification.production", language), value: "production" }]} />
        <Select showSearch allowClear value={query.storeID || undefined} placeholder={t("store360.select_store", language)} style={{ minWidth: 260 }} loading={discoveryLoading || optionsLoading} notFoundContent={optionsLoading ? t("store360.loading_stores", language) : t("store360.no_selectable_stores", language)} options={options.map((option) => { const item = optionFields(option); return { label: `${item.storeCode} · ${item.storeName}`, value: item.storeID, search: `${item.storeCode} ${item.storeName} ${item.brand} ${item.region}` }; })} optionFilterProp="search" onChange={(value) => change({ storeID: value || "" })} />
        <DatePicker allowClear={false} value={query.asOf ? dayjs(query.asOf) : undefined} onChange={(date) => date && change({ asOf: date.format("YYYY-MM-DD") })} />
        <Segmented value={validWindow(query.windowDays) ? query.windowDays : 14} onChange={(value) => change({ windowDays: Number(value) })} options={WINDOW_OPTIONS.map((item) => ({ label: `${item}${t("common.days_suffix", language)}`, value: item }))} />
        <Input aria-label={t("common.source_system", language)} value={sourceInput} onChange={(event) => setSourceInput(event.target.value)} onPressEnter={() => change({ sourceSystem: sourceInput.trim() })} placeholder={t("common.source_system_optional", language)} style={{ width: 190 }} />
        <Button onClick={() => change({ sourceSystem: sourceInput.trim() })}>{t("store360.apply_source", language)}</Button>
      </Flex>
    </Card>
    {response && <DataTrustBar envelope={response.envelope} basis={response.basis} detailExtra={latestMatches ? <span>generator: {latestMatches.generator_version} · latest anomaly: {latestAnomalyDate(latestMatches)}</span> : undefined} />}
    {noQuery && <Card><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={<Space direction="vertical"><Typography.Text strong>{t("store360.no_dataset_title", language)}</Typography.Text><Typography.Text type="secondary">{t("store360.no_dataset_desc", language)}</Typography.Text><Button onClick={() => router.push("/operating-pulse")}>{t("common.go_pulse", language)}</Button></Space>} /></Card>}
    {query.classification === "simulated" && !query.datasetVersion && !discoveryLoading && <Card><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={<Space direction="vertical"><Typography.Text strong>{t("store360.missing_version_title", language)}</Typography.Text><Typography.Text type="secondary">{t("store360.missing_version_desc", language)}</Typography.Text><Button onClick={() => router.push("/operating-pulse")}>{t("common.go_pulse", language)}</Button></Space>} /></Card>}
    {optionsError && <Alert type="error" showIcon message={t("store360.options_error", language)} description={optionsError} action={<Button size="small" onClick={() => setRetry((value) => value + 1)}>{t("common.retry", language)}</Button>} />}
    {query.classification && !optionsLoading && !optionsError && options.length === 0 && <Alert type="info" showIcon message={t("store360.no_authorized_stores", language)} description={t("store360.no_authorized_desc", language)} />}
    {loading && <Card><Flex justify="center" align="center" style={{ minHeight: 220 }}><Spin tip={t("store360.loading", language)} /></Flex></Card>}
    {error && <Alert type="error" showIcon message={t("store360.unavailable", language)} description={error} action={<Button size="small" onClick={() => setRetry((value) => value + 1)}>{t("common.retry", language)}</Button>} />}
    {!loading && !error && !response && query.storeID && <Card><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("store360.pick_filters", language)} /></Card>}
    {response && !response.decision_ready && response.evidence.observed_store_days === 0 && <Alert style={{ marginBottom: 16 }} type="warning" showIcon message={t("store360.no_facts_title", language)} description={t("store360.no_facts_desc", language)} />}
    {response && <>
      <Card className="store-360-identity-card" title={t("store360.identity", language)}><Flex wrap="wrap" gap={24}><div><Typography.Text type="secondary">{t("store360.field.store", language)}</Typography.Text><Typography.Title level={4} style={{ margin: 0 }}>{response.store.store_code} · {response.store.store_name}</Typography.Title></div><div><Typography.Text type="secondary">{t("store360.field.brand_region", language)}</Typography.Text><div>{response.store.brand || "—"} · {response.store.region || "—"}</div></div><div><Typography.Text type="secondary">{t("store360.field.currency", language)}</Typography.Text><div>{response.currency || "—"} · {response.currency_status}</div></div><div><Typography.Text type="secondary">{t("store360.field.fact_version", language)}</Typography.Text><div>{response.fact_version_min}–{response.fact_version_max}</div></div></Flex></Card>
      <Row gutter={[12, 12]} style={{ marginTop: 16 }}>{STORE360_CODES.map((code) => <Col xs={24} sm={12} lg={8} xl={4} key={code}><MetricCard language={language} code={code} metric={response.summary[code]} currency={response.currency} notReady={!response.decision_ready} /></Col>)}</Row>
      <Row gutter={[16, 16]} style={{ marginTop: 16 }}><Col xs={24} lg={16}><Trend response={response} language={language} /></Col><Col xs={24} lg={8}><Card title={t("store360.aux_metrics", language)}><Space direction="vertical" size={8} style={{ width: "100%" }}>{STORE360_AUX_CODES.map((code) => <Flex key={code} justify="space-between" align="center"><span>{kpiLabel(code, language)}</span><Flex gap={8} align="center"><Status metric={response.summary[code]} language={language} /><Typography.Text>{displayMetric(response.summary[code], response.currency, language)}</Typography.Text></Flex></Flex>)}</Space><Alert type="info" showIcon style={{ marginTop: 16 }} message={t("store360.cash_basis_title", language)} description={t("store360.cash_basis_desc", language)} /></Card></Col></Row>
      <Card title={t("store360.peer_benchmark", language)} style={{ marginTop: 16 }}><Typography.Text type="secondary">{response.peer_definition} · {t("store360.peer_definition", language).replace("{n}", String(response.minimum_peer_count))}</Typography.Text><Table style={{ marginTop: 8 }} size="small" pagination={false} rowKey="code" dataSource={response.peer_benchmark} columns={[{ title: t("store360.col.metric", language), render: (_: unknown, row: RetailStoreDiagnosticsResponse["peer_benchmark"][number]) => kpiLabel(row.code as PulseMetricCode, language) || row.code }, { title: t("store360.col.target", language), render: (_: unknown, row) => formatKPIValue({ value: row.target, unit: row.unit, status: "complete", formula_version: "", required_fields: [], available_fact_count: 0, fact_count: 0 }, response.currency, language) }, { title: t("store360.col.quartiles", language), render: (_: unknown, row) => `${formatKPIValue({ value: row.p25, unit: row.unit, status: "complete", formula_version: "", required_fields: [], available_fact_count: 0, fact_count: 0 }, response.currency, language)} / ${formatKPIValue({ value: row.median, unit: row.unit, status: "complete", formula_version: "", required_fields: [], available_fact_count: 0, fact_count: 0 }, response.currency, language)} / ${formatKPIValue({ value: row.p75, unit: row.unit, status: "complete", formula_version: "", required_fields: [], available_fact_count: 0, fact_count: 0 }, response.currency, language)}` }, { title: t("store360.col.sample_percentile", language), render: (_: unknown, row) => `${row.peer_count} · ${row.percentile == null ? "—" : `${row.percentile.toFixed(1)}%`}` }, { title: t("store360.col.status", language), render: (_: unknown, row) => <Tag>{formatPeerBenchmarkStatus(row.status, row.reason, language)}</Tag> }]} /></Card>
      <div style={{ marginTop: 16 }}><BridgePanel language={language} bridges={response.bridges} currency={response.currency} /></div>
      <Card title={t("store360.observations", language)} style={{ marginTop: 16 }}><Space direction="vertical" style={{ width: "100%" }}>{response.observations.length ? response.observations.map((item) => <Alert key={`${item.code}-${item.reference}`} type={item.status === "complete" ? "info" : "warning"} showIcon message={item.label} description={item.statement} />) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("store360.no_observations", language)} />}</Space></Card>
      <Collapse style={{ marginTop: 16 }} items={[{ key: "evidence", label: t("store360.evidence_title", language), children: <Space direction="vertical"><Typography.Text>{t("common.current", language)} {response.evidence.current.date_from}–{response.evidence.current.date_to} · {t("common.contrast", language)} {response.evidence.comparison.date_from}–{response.evidence.comparison.date_to}</Typography.Text><Typography.Text>{t("store360.evidence.coverage_source", language).replace("{observed}", String(response.evidence.observed_store_days)).replace("{expected}", String(response.evidence.expected_store_days)).replace("{sources}", response.evidence.source_systems.join(", ") || "—").replace("{datasets}", response.evidence.dataset_versions.join(", ") || "—")}</Typography.Text><Typography.Text>required fields: {response.evidence.required_fields.join(", ")}</Typography.Text><Typography.Text>{t("store360.evidence.fact_version", language).replace("{min}", String(response.evidence.fact_version_min)).replace("{max}", String(response.evidence.fact_version_max))} · <a href={response.evidence.kpi_drilldown_url}>{t("common.view_kpi_drilldown", language)}</a></Typography.Text></Space> }]} />
    </>}
    <RetailAIDrawer open={aiOpen} onClose={() => setAiOpen(false)} pageContext={{ page: "store-360", title: t("store360.title", language), filters: { as_of: query.asOf, window_days: String(query.windowDays), classification: query.classification || "", dataset_version: query.datasetVersion || "", source_system: query.sourceSystem || "", store_id: query.storeID || "" } }} />
  </div></AppLayout></ProtectedRoute>;
}

export default function Store360Page() {
  return <Suspense fallback={<div style={{ minHeight: "100vh", display: "grid", placeItems: "center" }}><Spin /></div>}><Store360Inner /></Suspense>;
}
