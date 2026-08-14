"use client";

import { useEffect, useMemo, useState, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Alert, Button, Card, Col, Collapse, DatePicker, Empty, Flex, Input, Radio, Row, Select, Segmented, Space, Spin, Table, Tag, Typography } from "antd";
import { ArrowLeftOutlined, InfoCircleOutlined, ReloadOutlined, WarningOutlined } from "@ant-design/icons";
import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip as ChartTooltip, XAxis, YAxis } from "recharts";
import dayjs from "dayjs";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { StatusTag } from "../components/StatusTag";
import { useAuth } from "../context/AuthContext";
import { apiErrorMessage, retailAnalyticsApi, type RetailDataClassification, type RetailSimulationDatasetData, type RetailStore360Option, type RetailStoreDiagnosticsResponse, type RetailSummaryMetric } from "../lib/api";
import { changeTone, formatChange, formatKPIValue, KPI_LABELS, latestAnomalyDate, type PulseMetricCode } from "../operating-pulse/logic";
import { bridgeConservation, bridgeTone, displayMetric, formatBridgeItem, formatPeerBenchmarkStatus, formatTrendTooltip, optionFields, returnPulseQuery, STORE360_AUX_CODES, STORE360_CODES, trendValue, validWindow, WINDOW_OPTIONS } from "./logic";
import { retailAIHref } from "../lib/retailAI";

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

function Status({ metric }: { metric?: RetailStoreDiagnosticsResponse["summary"][string] }) {
  const current = metric?.current.status;
  const comparison = metric?.comparison.status;
  const kind = current === "complete" && comparison === "complete" ? "neutral" : "warning";
  const label = current === "unavailable" || comparison === "unavailable" ? "缺失" : current === "partial" || comparison === "partial" ? "部分" : "完整";
  return <StatusTag kind={kind}>{label}</StatusTag>;
}

function MetricCard({ code, metric, currency }: { code: PulseMetricCode; metric?: RetailStoreDiagnosticsResponse["summary"][string]; currency: string }) {
  const tone = changeTone(code, metric as RetailSummaryMetric | undefined);
  const reason = metric?.reason || metric?.current.reason || metric?.comparison.reason;
  return <Card size="small" className="store-360-kpi-card" data-testid={`store360-kpi-${code}`}>
    <Flex justify="space-between" align="center"><Typography.Text type="secondary">{KPI_LABELS[code]}</Typography.Text><Status metric={metric} /></Flex>
    <Typography.Title level={3} style={{ margin: "12px 0 4px", fontVariantNumeric: "tabular-nums" }}>{displayMetric(metric, currency)}</Typography.Title>
    <Typography.Text className={`pulse-change pulse-change-${tone}`}>{formatChange(metric as RetailSummaryMetric | undefined)} {reason ? `· ${reason}` : ""}</Typography.Text>
    <Typography.Text type="secondary" className="pulse-kpi-comparison">对比 {formatKPIValue(metric?.comparison, currency)}</Typography.Text>
  </Card>;
}

function Trend({ response }: { response: RetailStoreDiagnosticsResponse }) {
  const [code, setCode] = useState<PulseMetricCode>("revenue");
  const data = response.daily_trend.map((row) => ({ date: row.date.slice(5), target: trendValue(row, code), peer: row.peer_median[code] ?? null, gap: row.gap }));
  return <Card title={<Flex justify="space-between" align="center" wrap="wrap" gap={8}><span>每日趋势（目标门店 / 同群中位数）</span><Segmented size="small" value={code} onChange={(value) => setCode(value as PulseMetricCode)} options={STORE360_CODES.map((item) => ({ label: KPI_LABELS[item], value: item }))} /></Flex>}>
    <div style={{ height: 270 }}>
      {data.length === 0 ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无趋势事实" /> : <ResponsiveContainer width="100%" height="100%"><LineChart data={data} margin={{ top: 8, right: 12, left: 0, bottom: 4 }}><CartesianGrid strokeDasharray="3 3" stroke="var(--border-subtle)" /><XAxis dataKey="date" tick={{ fontSize: 11 }} /><YAxis tick={{ fontSize: 11 }} /><ChartTooltip formatter={(value, name, item) => formatTrendTooltip(value == null ? null : Number(value), String(name), Boolean(item?.payload?.gap), response.summary[code]?.current.unit || "", response.currency)} /><Line type="monotone" dataKey="target" stroke="var(--chart-blue)" strokeWidth={2} dot={false} connectNulls={false} name="target" /><Line type="monotone" dataKey="peer" stroke="var(--chart-purple)" strokeDasharray="5 5" strokeWidth={2} dot={false} connectNulls={false} name="peer" /></LineChart></ResponsiveContainer>}
    </div>
  </Card>;
}

function BridgePanel({ bridges, currency }: { bridges: RetailStoreDiagnosticsResponse["bridges"]; currency: string }) {
  return <Card title="变化贡献桥（仅观察信号）"><Space direction="vertical" size={12} style={{ width: "100%" }}>{bridges.map((bridge) => <Card size="small" key={bridge.code} title={<Flex justify="space-between"><span>{bridge.code}</span><StatusTag kind={bridge.status === "complete" ? "neutral" : "warning"}>{bridge.status === "complete" ? "可用" : "不可用"}</StatusTag></Flex>}>
    {bridge.status !== "complete" ? <Typography.Text type="secondary">{bridge.reason || "所需字段不可用，未补零。"}</Typography.Text> : <><Flex wrap="wrap" gap={8} style={{ marginBottom: 8 }}><Tag>对比 {formatBridgeItem(bridge.comparison, "currency", currency)}</Tag><Tag>当前 {formatBridgeItem(bridge.current, "currency", currency)}</Tag><Tag>变化 {formatBridgeItem(bridge.total_change, "currency", currency)}</Tag><Tag>守恒残差 {formatBridgeItem(bridge.rounding_residual, "currency", currency)}</Tag></Flex><Table size="small" pagination={false} rowKey="code" dataSource={bridge.items} columns={[{ title: "变化项", dataIndex: "label" }, { title: "贡献", render: (_: unknown, row: typeof bridge.items[number]) => <Typography.Text className={`store-360-bridge-${bridgeTone(row.contribution)}`}>{formatBridgeItem(row.contribution, row.unit, currency)}</Typography.Text> }]} /></>}
  </Card>)}</Space></Card>;
}

function Store360Inner() {
  const { token } = useAuth();
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
    <PageHeader title="门店 360" subtitle="围绕单店的事实、同群对比与变化贡献；仅供 Working 经营分析，不作解释性判断。" primaryAction={<Button icon={<ReloadOutlined />} loading={loading || discoveryLoading} onClick={() => setRetry((value) => value + 1)}>刷新</Button>} secondaryAction={<Space><Button onClick={() => router.push(retailAIHref({ page: "store-360", title: "门店 360", asOf: query.asOf, windowDays: query.windowDays, classification: query.classification as "production" | "simulated", datasetVersion: query.datasetVersion || undefined, sourceSystem: query.sourceSystem, storeID: query.storeID || undefined }))}>交给 AI 分析</Button><Button onClick={() => router.push(scenarioURL)}>情景分析</Button><Button icon={<ArrowLeftOutlined />} onClick={() => router.push(backURL)}>返回经营脉搏</Button></Space>} />
    <Card size="small" className="store-360-filter-card">
      <Flex gap={12} wrap="wrap" align="center">
        <Radio.Group value={query.classification || "simulated"} onChange={(event) => { const next = event.target.value as RetailDataClassification; if (next === "production") change({ classification: next, datasetVersion: "", asOf: TODAY }); else if (latest) change({ classification: next, datasetVersion: latest.dataset_version, asOf: latestAnomalyDate(latest) }); else change({ classification: next, datasetVersion: "" }); }} optionType="button" buttonStyle="solid" options={[{ label: "模拟数据", value: "simulated" }, { label: "正式数据", value: "production" }]} />
        <Select showSearch allowClear value={query.storeID || undefined} placeholder="选择授权门店" style={{ minWidth: 260 }} loading={discoveryLoading || optionsLoading} notFoundContent={optionsLoading ? "加载门店…" : "当前范围没有可选门店"} options={options.map((option) => { const item = optionFields(option); return { label: `${item.storeCode} · ${item.storeName}`, value: item.storeID, search: `${item.storeCode} ${item.storeName} ${item.brand} ${item.region}` }; })} optionFilterProp="search" onChange={(value) => change({ storeID: value || "" })} />
        <DatePicker allowClear={false} value={query.asOf ? dayjs(query.asOf) : undefined} onChange={(date) => date && change({ asOf: date.format("YYYY-MM-DD") })} />
        <Segmented value={validWindow(query.windowDays) ? query.windowDays : 14} onChange={(value) => change({ windowDays: Number(value) })} options={WINDOW_OPTIONS.map((item) => ({ label: `${item}天`, value: item }))} />
        <Input aria-label="来源系统" value={sourceInput} onChange={(event) => setSourceInput(event.target.value)} onPressEnter={() => change({ sourceSystem: sourceInput.trim() })} placeholder="source_system（可选）" style={{ width: 190 }} />
        <Button onClick={() => change({ sourceSystem: sourceInput.trim() })}>应用来源</Button>
      </Flex>
    </Card>
    <Alert className="store-360-trust-strip" type={query.classification === "production" ? "info" : "warning"} showIcon icon={query.classification === "production" ? <InfoCircleOutlined /> : <WarningOutlined />} message={<Flex wrap="wrap" gap={12}><StatusTag kind={query.classification === "production" ? "processing" : "warning"}>{query.classification === "production" ? "正式数据 · Working" : "模拟数据 · 不进入 Official"}</StatusTag><span>dataset: {query.classification === "simulated" ? query.datasetVersion || "—" : "—"}</span><span>formula: retail-kpi-v1</span><span>诊断: retail-store-diagnostics-v1</span>{response && <span>decision-ready: {response.decision_ready ? "是" : "否"} · coverage {response.target_coverage.observed_store_days}/{response.target_coverage.expected_store_days}</span>}<span>经营占用现金成本 ≠ IFRS 16 会计费用</span></Flex>} description={latestMatches ? `generator: ${latestMatches.generator_version} · latest anomaly: ${latestAnomalyDate(latestMatches)}` : query.classification === "production" ? "正式数据不会显示模拟 generator；当前仅读取 Working 事实。" : "当前 URL 数据集没有可用的 latest 元数据。"} />
    {noQuery && <Card><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={<Space direction="vertical"><Typography.Text strong>还没有可用的模拟数据集</Typography.Text><Typography.Text type="secondary">请先在经营脉搏由管理员按固定流程生成演示数据，之后从门店关注行进入门店 360。</Typography.Text><Button onClick={() => router.push("/operating-pulse")}>前往经营脉搏</Button></Space>} /></Card>}
    {query.classification === "simulated" && !query.datasetVersion && !discoveryLoading && <Card><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={<Space direction="vertical"><Typography.Text strong>模拟数据集版本缺失</Typography.Text><Typography.Text type="secondary">请从经营脉搏选择一个可用数据集后再进入门店 360；本页不会自动生成或补写数据。</Typography.Text><Button onClick={() => router.push("/operating-pulse")}>前往经营脉搏</Button></Space>} /></Card>}
    {optionsError && <Alert type="error" showIcon message="门店列表加载失败" description={optionsError} action={<Button size="small" onClick={() => setRetry((value) => value + 1)}>重试</Button>} />}
    {query.classification && !optionsLoading && !optionsError && options.length === 0 && <Alert type="info" showIcon message="当前范围没有授权门店" description="请检查法人、region/brand/store 数据权限或选择其他 classification/dataset；系统不会自动选择或补造门店。" />}
    {loading && <Card><Flex justify="center" align="center" style={{ minHeight: 220 }}><Spin tip="读取门店诊断…" /></Flex></Card>}
    {error && <Alert type="error" showIcon message="门店诊断暂不可用" description={error} action={<Button size="small" onClick={() => setRetry((value) => value + 1)}>重试</Button>} />}
    {!loading && !error && !response && query.storeID && <Card><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="请选择完整筛选条件后读取门店事实。" /></Card>}
    {response && !response.decision_ready && response.evidence.observed_store_days === 0 && <Alert style={{ marginBottom: 16 }} type="warning" showIcon message="当前窗口没有门店事实" description="请先导入并完成该门店的经营日事实，或选择包含有效事实的数据集；系统不会用 0 填补缺失。" />}
    {response && <>
      <Card className="store-360-identity-card" title="门店身份"><Flex wrap="wrap" gap={24}><div><Typography.Text type="secondary">门店</Typography.Text><Typography.Title level={4} style={{ margin: 0 }}>{response.store.store_code} · {response.store.store_name}</Typography.Title></div><div><Typography.Text type="secondary">品牌 / 区域</Typography.Text><div>{response.store.brand || "—"} · {response.store.region || "—"}</div></div><div><Typography.Text type="secondary">币种</Typography.Text><div>{response.currency || "—"} · {response.currency_status}</div></div><div><Typography.Text type="secondary">事实版本</Typography.Text><div>{response.fact_version_min}–{response.fact_version_max}</div></div></Flex></Card>
      <Row gutter={[12, 12]} style={{ marginTop: 16 }}>{STORE360_CODES.map((code) => <Col xs={24} sm={12} lg={8} xl={4} key={code}><MetricCard code={code} metric={response.summary[code]} currency={response.currency} /></Col>)}</Row>
      <Row gutter={[16, 16]} style={{ marginTop: 16 }}><Col xs={24} lg={16}><Trend response={response} /></Col><Col xs={24} lg={8}><Card title="辅助指标"><Space direction="vertical" size={8} style={{ width: "100%" }}>{STORE360_AUX_CODES.map((code) => <Flex key={code} justify="space-between" align="center"><span>{KPI_LABELS[code]}</span><Flex gap={8} align="center"><Status metric={response.summary[code]} /><Typography.Text>{displayMetric(response.summary[code], response.currency)}</Typography.Text></Flex></Flex>)}</Space><Alert type="info" showIcon style={{ marginTop: 16 }} message="经营口径" description="经营占用现金成本仅用于经营分析，未混入 IFRS 16 计量或 Official 过账链路。" /></Card></Col></Row>
      <Card title="同群基准" style={{ marginTop: 16 }}><Typography.Text type="secondary">{response.peer_definition} · 最少 {response.minimum_peer_count} 家同群门店</Typography.Text><Table style={{ marginTop: 8 }} size="small" pagination={false} rowKey="code" dataSource={response.peer_benchmark} columns={[{ title: "指标", render: (_: unknown, row: RetailStoreDiagnosticsResponse["peer_benchmark"][number]) => KPI_LABELS[row.code as PulseMetricCode] || row.code }, { title: "目标", render: (_: unknown, row) => formatKPIValue({ value: row.target, unit: row.unit, status: "complete", formula_version: "", required_fields: [], available_fact_count: 0, fact_count: 0 }, response.currency) }, { title: "P25 / 中位 / P75", render: (_: unknown, row) => `${formatKPIValue({ value: row.p25, unit: row.unit, status: "complete", formula_version: "", required_fields: [], available_fact_count: 0, fact_count: 0 }, response.currency)} / ${formatKPIValue({ value: row.median, unit: row.unit, status: "complete", formula_version: "", required_fields: [], available_fact_count: 0, fact_count: 0 }, response.currency)} / ${formatKPIValue({ value: row.p75, unit: row.unit, status: "complete", formula_version: "", required_fields: [], available_fact_count: 0, fact_count: 0 }, response.currency)}` }, { title: "样本 / 百分位", render: (_: unknown, row) => `${row.peer_count} · ${row.percentile == null ? "—" : `${row.percentile.toFixed(1)}%`}` }, { title: "状态", render: (_: unknown, row) => <Tag>{formatPeerBenchmarkStatus(row.status, row.reason)}</Tag> }]} /></Card>
      <div style={{ marginTop: 16 }}><BridgePanel bridges={response.bridges} currency={response.currency} /></div>
      <Card title="观察信号" style={{ marginTop: 16 }}><Space direction="vertical" style={{ width: "100%" }}>{response.observations.length ? response.observations.map((item) => <Alert key={`${item.code}-${item.reference}`} type={item.status === "complete" ? "info" : "warning"} showIcon message={item.label} description={item.statement} />) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前期间没有可用观察信号" />}</Space></Card>
      <Collapse style={{ marginTop: 16 }} items={[{ key: "evidence", label: "证据与可追溯性", children: <Space direction="vertical"><Typography.Text>当前 {response.evidence.current.date_from}–{response.evidence.current.date_to} · 对比 {response.evidence.comparison.date_from}–{response.evidence.comparison.date_to}</Typography.Text><Typography.Text>覆盖 {response.evidence.observed_store_days}/{response.evidence.expected_store_days} store-days · 来源 {response.evidence.source_systems.join(", ") || "—"} · dataset {response.evidence.dataset_versions.join(", ") || "—"}</Typography.Text><Typography.Text>required fields: {response.evidence.required_fields.join(", ")}</Typography.Text><Typography.Text>fact version {response.evidence.fact_version_min}–{response.evidence.fact_version_max} · <a href={response.evidence.kpi_drilldown_url}>查看 KPI 下钻</a></Typography.Text></Space> }]} />
    </>}
  </div></AppLayout></ProtectedRoute>;
}

export default function Store360Page() {
  return <Suspense fallback={<div style={{ minHeight: "100vh", display: "grid", placeItems: "center" }}><Spin /></div>}><Store360Inner /></Suspense>;
}
