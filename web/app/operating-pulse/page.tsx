"use client";

import { useCallback, useEffect, useMemo, useRef, useState, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import {
  Alert, Button, Card, Col, Collapse, DatePicker, Empty, Flex, Input, Radio, Row, Select, Segmented, Space, Spin, Table, Tag, Tooltip, Typography, message,
} from "antd";
import { ArrowDownOutlined, ArrowUpOutlined, EyeOutlined, InfoCircleOutlined, ReloadOutlined, WarningOutlined } from "@ant-design/icons";
import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip as ChartTooltip, XAxis, YAxis } from "recharts";
import dayjs from "dayjs";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { StatusTag } from "../components/StatusTag";
import { hasRole, useAuth } from "../context/AuthContext";
import { ApiError, retailAnalyticsApi, type RetailAttention, type RetailCoverage, type RetailDailyTrend, type RetailPulsePartition, type RetailPulseResponse, type RetailSimulationDatasetData, type RetailStoreScope, type RetailSuppressedAttention, type RetailSummaryMetric } from "../lib/api";
import { changeTone, formatChange, formatKPIValue, formatSignalValue, KPI_LABELS, latestAnomalyDate, metricStatusLabel, metricUnitLabel, PULSE_AUXILIARY_CODES, PULSE_KPI_CODES, responsePartitions, signalLabel, switchClassification, trendValue, type PulseMetricCode } from "./logic";
import { createLatestRequestGate } from "./requestGate";
import { retailAIHref } from "../lib/retailAI";

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

function coverageRange(response: RetailPulseResponse): string {
  return `${coverageText(response.current_coverage)} current · ${coverageText(response.comparison_coverage)} comparison`;
}

function errorCopy(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 409) return "来源冲突：请指定唯一 source_system 后重试。";
    if (error.status === 403) return "当前账号没有经营报表读取权限，请联系管理员。";
    if (error.status >= 500) return "经营脉搏服务暂时不可用，请稍后刷新。";
    return error.message;
  }
  return "经营脉搏加载失败，请检查网络后重试。";
}

function effectivePartition(response: RetailPulseResponse, selectedCurrency: string): RetailPulsePartition | null {
  const partitions = responsePartitions(response);
  return partitions.find((item) => item.currency === selectedCurrency) || partitions[0] || null;
}

function KPIValueCard({ code, metric, currency }: { code: PulseMetricCode; metric?: RetailPulseResponse["summary"] extends infer T ? T extends Record<string, infer M> ? M : never : never; currency: string }) {
  if (!metric) return <Card size="small" className="pulse-kpi-card"><Typography.Text type="secondary">{KPI_LABELS[code]}</Typography.Text><div className="pulse-kpi-null">—</div></Card>;
  const tone = changeTone(code, metric);
  const arrow = metric.change_value == null ? undefined : metric.change_value < 0 ? <ArrowDownOutlined /> : metric.change_value > 0 ? <ArrowUpOutlined /> : undefined;
  const status = metricStatusLabel(metric);
  const statusKind = status.label === "完整" ? "neutral" : "warning";
  return <Card size="small" className="pulse-kpi-card" data-testid={`pulse-kpi-${code}`}>
    <Flex justify="space-between" align="start" gap={8}><Typography.Text type="secondary">{KPI_LABELS[code]}</Typography.Text><Tooltip title={status.reason}><StatusTag kind={statusKind}>{status.label}</StatusTag></Tooltip></Flex>
    <Typography.Title level={3} style={{ margin: "12px 0 4px", fontVariantNumeric: "tabular-nums" }}>{formatKPIValue(metric.current, currency)}</Typography.Title>
    <Typography.Text className={`pulse-change pulse-change-${tone}`}>{arrow} {formatChange(metric)} {status.reason ? `· ${status.reason}` : ""}</Typography.Text>
    <Typography.Text type="secondary" className="pulse-kpi-comparison">对比 {formatKPIValue(metric.comparison, currency)}</Typography.Text>
  </Card>;
}

function AuxiliaryMetricRow({ code, metric, currency }: { code: PulseMetricCode; metric?: RetailSummaryMetric; currency?: string }) {
  const status = metricStatusLabel(metric);
  const tone = changeTone(code, metric);
  const statusKind = status.label === "完整" ? "neutral" : "warning";
  return <div className="pulse-aux-row">
    <div><Typography.Text>{KPI_LABELS[code]}</Typography.Text><div className="pulse-aux-status"><StatusTag kind={statusKind}>{status.label}</StatusTag>{status.reason && <Typography.Text type="secondary">{status.reason}</Typography.Text>}</div></div>
    <div className="pulse-aux-values"><strong>{formatKPIValue(metric?.current, currency)}</strong><Typography.Text type="secondary">对比 {formatKPIValue(metric?.comparison, currency)}</Typography.Text></div>
    <Typography.Text className={`pulse-change pulse-change-${tone}`}>{formatChange(metric)}</Typography.Text>
  </div>;
}

function trendUnit(code: PulseMetricCode): string {
  if (code === "revenue" || code === "gross_profit" || code === "store_contribution" || code === "average_transaction_value") return "currency";
  if (code.endsWith("rate") || code === "gross_margin_rate" || code === "conversion_rate" || code === "store_contribution_margin") return "percent";
  if (code === "sales_per_sqm") return "currency_per_sqm";
  return "count";
}

function TrendChart({ trend, code, currency, onMetricChange }: { trend: RetailDailyTrend[]; code: PulseMetricCode; currency: string; onMetricChange: (code: PulseMetricCode) => void }) {
  const chartData = trend.map((row) => ({ date: row.date.slice(5), value: trendValue(row, code), gap: row.gap }));
  return <Card title={<Flex justify="space-between" align="center" wrap="wrap" gap={8}><span>每日趋势</span><Segmented size="small" value={code} onChange={(value) => onMetricChange(value as PulseMetricCode)} options={PULSE_KPI_CODES.map((item) => ({ label: KPI_LABELS[item], value: item }))} /></Flex>}>
    <div style={{ height: 270, width: "100%" }}>
      {trend.length === 0 ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无趋势事实" /> : <ResponsiveContainer width="100%" height="100%"><LineChart data={chartData} margin={{ top: 8, right: 12, left: 0, bottom: 4 }}>
        <CartesianGrid strokeDasharray="3 3" stroke="var(--border-subtle)" />
        <XAxis dataKey="date" tick={{ fontSize: 11 }} />
        <YAxis tick={{ fontSize: 11 }} tickFormatter={(value) => value == null ? "—" : Number(value).toLocaleString()} />
        <ChartTooltip formatter={(value, _name, item) => item?.payload?.gap ? ["数据缺口", "状态"] : [`${value == null ? "—" : Number(value).toLocaleString()} ${metricUnitLabel(trendUnit(code), currency)}`, KPI_LABELS[code]]} />
        <Line type="monotone" dataKey="value" stroke="var(--chart-blue)" strokeWidth={2} dot={false} connectNulls={false} />
      </LineChart></ResponsiveContainer>}
    </div>
  </Card>;
}

function AttentionTable({ attention, onSelect, onStore360 }: { attention: RetailAttention[]; onSelect: (storeID: string) => void; onStore360: (storeID: string) => void }) {
  const columns = [
    { title: "优先", dataIndex: "rank", width: 56, render: (value: number) => <strong>#{value}</strong> },
    { title: "门店", key: "store", render: (_: unknown, row: RetailAttention) => <Space direction="vertical" size={0}><strong>{row.store_code} · {row.store_name}</strong><Typography.Text type="secondary">{row.brand} · {row.region}</Typography.Text></Space> },
    { title: "信号", key: "signals", render: (_: unknown, row: RetailAttention) => <Space wrap size={[4, 4]}>{row.observed_signals.map((signal) => <Tooltip key={signal.signal_code} title={`${signal.signal_code} · threshold ${formatSignalValue(signal.threshold, signal.unit, row.currency)}`}><Tag color={row.severity === "critical" || row.severity === "high" ? "red" : "gold"}>{signalLabel(signal.signal_code)}</Tag></Tooltip>)}</Space> },
    { title: "变化", key: "change", render: (_: unknown, row: RetailAttention) => <Space direction="vertical" size={0}>{row.observed_signals.map((signal) => <Tooltip key={signal.signal_code} title={`当前 ${formatSignalValue(signal.current, signal.unit, row.currency)} · 对比 ${formatSignalValue(signal.comparison, signal.unit, row.currency)} · 阈值 ${formatSignalValue(signal.threshold, signal.unit, row.currency)}`}><Typography.Text className="pulse-change-bad">{signalLabel(signal.signal_code)} {formatSignalValue(signal.observed_change, signal.unit, row.currency)} · 阈值 {formatSignalValue(signal.threshold, signal.unit, row.currency)}</Typography.Text></Tooltip>)}</Space> },
    { title: "评分", key: "score", render: (_: unknown, row: RetailAttention) => <Space direction="vertical" size={0}><StatusTag kind={row.severity === "critical" || row.severity === "high" ? "error" : "warning"}>{row.severity}</StatusTag><span>{row.score.toFixed(2)}</span></Space> },
    { title: "数据来源", key: "source", render: (_: unknown, row: RetailAttention) => <Typography.Text type="secondary">{row.evidence.source_systems.join(", ") || "—"}</Typography.Text> },
    { title: "操作", key: "action", width: 220, render: (_: unknown, row: RetailAttention) => <Space><Button size="small" icon={<EyeOutlined />} onClick={() => onSelect(row.store_id)}>查看门店脉搏</Button><Button size="small" onClick={() => onStore360(row.store_id)}>门店 360</Button></Space> },
  ];
  return attention.length ? <Table rowKey="store_id" size="small" columns={columns} dataSource={attention} pagination={false} scroll={{ x: 980 }} /> : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="当前筛选下未触发固定经营信号" />;
}

function SuppressedPanel({ items }: { items: RetailSuppressedAttention[] }) {
  if (!items.length) return null;
  return <Collapse items={[{ key: "suppressed", label: `数据不足而被抑制的门店（${items.length}）`, children: <Table size="small" rowKey={(row: RetailSuppressedAttention) => `${row.store_id}-${row.reason}`} pagination={false} scroll={{ x: 760 }} dataSource={items} columns={[{ title: "门店", render: (_: unknown, row: RetailSuppressedAttention) => `${row.store_code} · ${row.store_name}` }, { title: "品牌 / 区域", render: (_: unknown, row: RetailSuppressedAttention) => `${row.brand || "—"} · ${row.region || "—"}` }, { title: "原因", render: (_: unknown, row: RetailSuppressedAttention) => <Space wrap>{(row.reasons || [row.reason]).map((reason) => <Tag key={reason}>{reason}</Tag>)}</Space> }, { title: "覆盖", render: (_: unknown, row: RetailSuppressedAttention) => `${coverageText(row.current_coverage)} · ${coverageText(row.comparison_coverage)}` }]} /> }]} />;
}

function OperatingPulseInner() {
  const { token, user } = useAuth();
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
      requestGate.current.commit(id, () => setError(errorCopy(loadError)));
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
    }).catch((loadError) => { latestLoaded.current = false; setError(errorCopy(loadError)); setLoading(false); });
  }, [router, searchParams, token, storeIDs, validWindow, windowDays, latestRetryNonce]);

  const storeKey = storeIDs.join("\x1f");
  const pulseKey = `${classification}|${datasetVersion}|${asOf}|${windowDays}|${sourceSystem}|${storeKey}|${refreshNonce}`;
  useEffect(() => {
    if (!token || (classification !== "production" && classification !== "simulated")) return;
    if (classification === "simulated" && !datasetVersion) { setError("模拟数据缺少 dataset_version，请从最新数据集重新进入。"); setLoading(false); return; }
    if (!validWindow) { setResponse(null); setError("窗口仅支持 7、14 或 28 天，请选择一个有效窗口。"); setLoading(false); return; }
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
      message.success("固定演示数据已生成");
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
    } catch (generationError) { message.error(errorCopy(generationError)); } finally { setGenerating(false); }
  };

  const partition = response ? effectivePartition(response, selectedCurrency) : null;
  const partitions = response ? responsePartitions(response) : [];
  const isEmptyInitial = latest === null && !response && !error;
  const isScoped = storeIDs.length > 0;
  const displaySummary = partition?.summary || response?.summary || {};
  const sourceSystems = response?.source_systems || [];
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

  const kpiCards = PULSE_KPI_CODES.map((code) => <Col xs={24} sm={12} lg={8} xl={4} key={code}><KPIValueCard code={code} metric={displaySummary[code]} currency={partition?.currency || response?.currency || ""} /></Col>);
  const aux = PULSE_AUXILIARY_CODES.map((code) => <AuxiliaryMetricRow key={code} code={code} metric={displaySummary[code]} currency={partition?.currency || response?.currency} />);

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
    <PageHeader title="经营脉搏" subtitle="两分钟完成整体表现、数据可信度与优先门店晨检。" primaryAction={<Button icon={<ReloadOutlined />} onClick={refresh} loading={loading}>刷新</Button>} secondaryAction={<Button onClick={() => router.push(retailAIHref({ page: "operating-pulse", title: "经营脉搏", asOf: asOf || TODAY, windowDays: validWindow ? windowDays : 7, classification: currentClassification || undefined, datasetVersion: currentClassification === "simulated" ? datasetVersion : undefined, sourceSystem, storeIDs }))}>交给 AI 分析</Button>} />
    <Card size="small" className="pulse-filter-card" style={{ marginBottom: 16 }}>
      <Flex gap={12} wrap="wrap" align="center">
        <Radio.Group value={currentClassification} onChange={(event) => onClassificationChange(event.target.value as "production" | "simulated")} optionType="button" buttonStyle="solid" options={[{ label: "模拟数据", value: "simulated" }, { label: "正式数据", value: "production" }]} />
        {currentClassification === "simulated" && anomalies.length > 0 && <Select aria-label="模拟场景" value={currentAnomaly?.id || "all"} style={{ minWidth: 220 }} options={[{ label: "全部固定异常", value: "all" }, ...anomalies.map((item) => ({ label: `${item.store_code} · ${signalLabel(item.type)}`, value: item.id }))]} onChange={(id) => { const selected = anomalies.find((item) => item.id === id); if (selected) updateQuery(router, { classification: "simulated", datasetVersion, asOf: selected.date_to, windowDays: 7, storeIDs, sourceSystem }); }} />}
        <DatePicker aria-label="截至日期" value={asOf ? dayjs(asOf) : undefined} onChange={(date) => date && updateQuery(router, { classification: currentClassification, datasetVersion: currentClassification === "simulated" ? datasetVersion : undefined, asOf: date.format("YYYY-MM-DD"), windowDays: validWindow ? windowDays : 7, storeIDs, sourceSystem })} allowClear={false} />
        <Input aria-label="来源系统" allowClear placeholder="source_system（可选）" value={sourceSystem} onChange={(event) => updateQuery(router, { classification: currentClassification, datasetVersion: currentClassification === "simulated" ? datasetVersion : undefined, asOf: asOf || TODAY, windowDays: validWindow ? windowDays : 7, storeIDs, sourceSystem: event.target.value.trim() })} style={{ width: 180 }} />
        <Segmented aria-label="窗口" value={validWindow ? windowDays : 7} onChange={onWindowChange} options={WINDOW_OPTIONS.map((item) => ({ label: `${item}天`, value: item }))} />
        {isScoped ? <Button onClick={clearStore}>返回全部门店</Button> : <Tag>全部授权门店</Tag>}
      </Flex>
    </Card>
    {isEmptyInitial && <Card><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={<Space direction="vertical"><Typography.Text strong>当前法人还没有模拟数据集</Typography.Text><Typography.Text type="secondary">生成固定 60 店演示数据后，可复演六类经营信号。页面不会自动写入数据。</Typography.Text>{hasRole(user, "admin") ? <Button type="primary" loading={generating} onClick={generate}>生成固定演示数据</Button> : <Typography.Text>请联系当前法人管理员生成演示数据。</Typography.Text>}</Space>} /></Card>}
    {error && <Alert type="error" showIcon message="经营脉搏暂不可用" description={error} action={<Button size="small" onClick={refresh}>重试</Button>} />}
    {loading && !response && !isEmptyInitial && <Card><Flex justify="center" align="center" style={{ minHeight: 220 }}><Spin tip="读取经营脉搏…" /></Flex></Card>}
      {response && partition && <>
      <Alert type={response.decision_ready ? "info" : "warning"} showIcon icon={response.decision_ready ? <InfoCircleOutlined /> : <WarningOutlined />} className="pulse-trust-strip" message={<Flex wrap="wrap" gap={12} align="center"><StatusTag kind={response.data_classification === "simulated" ? "warning" : "processing"}>{response.data_classification === "simulated" ? "模拟数据 · 不进入 Official" : "正式数据 · Working"}</StatusTag><StatusTag kind="neutral">{response.basis}</StatusTag><span>{response.current.date_from} – {response.current.date_to} · 对比 {response.comparison.date_from} – {response.comparison.date_to}</span><span>{coverageRange(response)}</span><span>source: {sourceSystems.join(", ") || "—"}</span><span>formula: {response.formula_version}</span></Flex>} description={<Flex wrap="wrap" gap={12}><span>dataset: {response.dataset_version || "—"}</span><span>generator: {latestMetadata?.generator_version || "—"}</span><span>pulse: {response.pulse_version}</span><span>fact version: {response.fact_version_min}–{response.fact_version_max}</span><span>decision-ready: {response.decision_ready ? "是" : "否 · KPI 仅供查看，不可作为完整判断"}</span>{response.highest_as_of && <span>最高事实截至 {dayjs(response.highest_as_of).format("YYYY-MM-DD HH:mm")}</span>}</Flex>} />
      {noFacts ? <Card style={{ marginTop: 16 }}><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={<Space direction="vertical"><Typography.Text strong>当前正式数据窗口没有事实</Typography.Text><Typography.Text type="secondary">请先导入并完成门店日事实映射，再刷新经营脉搏。系统不会用 0 填补缺失。</Typography.Text></Space>} /></Card> : <>
      {response.multi_currency && <Card size="small" style={{ marginTop: 16 }}><Flex align="center" gap={8}><Typography.Text strong>币种分区</Typography.Text><Segmented value={selectedCurrency} onChange={(value) => setSelectedCurrency(String(value))} options={partitions.map((item) => ({ label: item.currency || "未知币种", value: item.currency }))} /></Flex></Card>}
      <Row gutter={[12, 12]} style={{ marginTop: 16 }}>{kpiCards}</Row>
      <Row gutter={[16, 16]} style={{ marginTop: 16 }}><Col xs={24} lg={16}><TrendChart trend={partition.daily_trend} code={trendMetric} currency={partition.currency || response.currency || ""} onMetricChange={setTrendMetric} /></Col><Col xs={24} lg={8}><Card title="辅助指标"><Space direction="vertical" size={0} style={{ width: "100%" }}>{aux}</Space><Alert type="info" showIcon style={{ marginTop: 16 }} message="经营现金口径" description="经营占用现金成本不等于 IFRS 16 折旧、利息、ROU 或租赁负债变动。" /></Card></Col></Row>
      <Card title={<Flex justify="space-between" align="center"><span>{isScoped ? `门店脉搏 · ${scopedTitle}` : "优先关注门店"}</span><Typography.Text type="secondary">按 API rank 原序 · 不在前端重算评分</Typography.Text></Flex>} style={{ marginTop: 16 }}><AttentionTable attention={partition.attention} onSelect={onStoreSelect} onStore360={(storeID) => { const returnQuery = new URLSearchParams(searchParams.toString()); const params = new URLSearchParams(searchParams.toString()); params.delete("store_id"); params.set("store_id", storeID); params.set("return_query", returnQuery.toString()); router.push(`/store-360?${params.toString()}`); }} /></Card>
      <div style={{ marginTop: 16 }}><SuppressedPanel items={partition.suppressed_attention || []} /></div>
      </>}
    </>}
  </div></AppLayout></ProtectedRoute>;
}

export default function OperatingPulsePage() {
  return <Suspense fallback={<div style={{ minHeight: "100vh", display: "grid", placeItems: "center" }}><Spin /></div>}><OperatingPulseInner /></Suspense>;
}
