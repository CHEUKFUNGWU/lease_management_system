"use client";

import { useCallback, useEffect, useMemo, useRef, useState, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import {
  Alert, Button, Card, Col, Collapse, DatePicker, Empty, Flex, Input, InputNumber, Radio, Row, Select, Segmented, Space, Spin, Table, Tag, Tooltip, Typography, message,
} from "antd";
import { ArrowDownOutlined, ArrowUpOutlined, EyeOutlined, ReloadOutlined } from "@ant-design/icons";
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip as ChartTooltip, XAxis, YAxis } from "recharts";
import dayjs from "dayjs";
import AppLayout from "../components/AppLayout";
import { SeverityDot, toSeverity } from "../components/SeverityDot";
import PageHeader from "../components/PageHeader";
import DataTrustBar, { KPIReadyBadge } from "../components/DataTrustBar";
import RetailAIDrawer from "../components/RetailAIDrawer";
import ProtectedRoute from "../components/ProtectedRoute";
import { StatusTag } from "../components/StatusTag";
import { SparkleGlyph, AlertTriangleGlyph } from "../components/MonochromeGlyphs";
import ConfidenceBandChart from "../components/charts/ConfidenceBandChart";
import { hasRole, useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t, type Language } from "../lib/i18n";
import { useRetailQuery } from "../retail/useRetailQuery";
import { HelpTrigger } from "../components/HelpDrawer";
import { pulseHelpContent } from "../components/help-content";
import { StateBlock } from "../components/StateBlock";
import { BentoGrid, BentoTile } from "../components/bento/BentoGrid";
import type { DataState } from "../lib/dataState";
import { apiErrorMessage, retailAnalyticsApi, type RetailAttention, type RetailCoverage, type RetailDailyTrend, type RetailPulsePartition, type RetailPulseResponse, type RetailSimulationDatasetData, type RetailStoreScope, type RetailSuppressedAttention, type RetailSummaryMetric } from "../lib/api";
import { changeTone, formatChange, formatKPIValue, formatSeverity, formatSignalValue, formatSourceSystem, kpiLabel, latestAnomalyDate, lifecycleStatusLabel, metricStatusLabel, metricUnitLabel, PULSE_AUXILIARY_CODES, PULSE_KPI_CODES, responsePartitions, signalLabel, signalMix, sssgReasonLabel, storeFormatLabel, switchClassification, translateReason, trendValue, type PulseMetricCode } from "./logic";
import { tableScrollX } from "../lib/tableScroll";
import { RetailExportMenu } from "../components/RetailExportMenu";
import { retailExportApi } from "../lib/api";
import { envelopeFromPulse, pulseRowsFromResponse } from "../lib/retail-export";
import { PlanComparisonPanel } from "../components/PlanComparisonPanel";

const WINDOW_OPTIONS = [1, 7, 14, 30, 90] as const;
const DEFAULT_WINDOW_DAYS = 14;
const validWindowDays = (value: number) => Number.isInteger(value) && value >= 1 && value <= 365;
const TODAY = dayjs().format("YYYY-MM-DD");

function updateQuery(router: ReturnType<typeof useRouter>, params: { classification: "production" | "simulated"; datasetVersion?: string; asOf: string; windowDays?: number; storeIDs: string[]; sourceSystem?: string; period?: string; groupBy?: string }) {
  const query = new URLSearchParams();
  query.set("data_classification", params.classification);
  if (params.classification === "simulated" && params.datasetVersion) query.set("dataset_version", params.datasetVersion);
  query.set("as_of", params.asOf);
  if (params.period) query.set("period", params.period);
  else if (params.windowDays !== undefined) query.set("window_days", String(params.windowDays));
  if (params.groupBy && params.groupBy !== "total") query.set("group_by", params.groupBy);
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
  if (!metric) return (
    <div className="pulse-kpi-card" data-testid={`pulse-kpi-${code}`}>
      <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{kpiLabel(code, language)}</span>
      <div className="pulse-kpi-null">—</div>
    </div>
  );
  const tone = changeTone(code, metric);
  const arrow = metric.change_value == null ? undefined : metric.change_value < 0 ? <ArrowDownOutlined /> : metric.change_value > 0 ? <ArrowUpOutlined /> : undefined;
  const status = metricStatusLabel(metric, language);
  const statusKind = status.status === "complete" ? "neutral" : "warning";
  const display = formatKPIValue(metric.current, currency, language);
  return (
    <div className="pulse-kpi-card" data-testid={`pulse-kpi-${code}`}>
      <Flex justify="space-between" align="center">
        <span style={{ fontSize: 12, fontWeight: 500, color: "var(--fg-secondary)" }}>{kpiLabel(code, language)}</span>
        <Flex align="center" gap={4}>
          {notReady && <KPIReadyBadge />}
          {status.status !== "complete" && (
            <Tooltip title={status.reason}>
              <StatusTag kind={statusKind}>{status.label}</StatusTag>
            </Tooltip>
          )}
        </Flex>
      </Flex>
      <Typography.Title level={3} className="pulse-kpi-value" ellipsis={{ tooltip: display }}>{display}</Typography.Title>
      <div style={{ display: "flex", alignItems: "baseline", gap: 6, overflow: "hidden", whiteSpace: "nowrap" }}>
        <Typography.Text className={`pulse-change pulse-change-${tone}`}>
          {arrow} {formatChange(metric)}
        </Typography.Text>
        <Typography.Text type="secondary" className="pulse-kpi-comparison">
          {t("common.contrast", language)} {formatKPIValue(metric.comparison, currency, language)}
        </Typography.Text>
      </div>
    </div>
  );
}

function AuxiliaryMetricRow({ code, metric, currency, language }: { code: PulseMetricCode; metric?: RetailSummaryMetric; currency?: string; language: Language }) {
  const status = metricStatusLabel(metric, language);
  const tone = changeTone(code, metric);
  return (
    <div className="pulse-aux-row">
      <div>
        <Typography.Text style={{ fontSize: 13, color: "var(--fg-secondary)", fontWeight: 500 }}>{kpiLabel(code, language)}</Typography.Text>
        {status.status !== "complete" && (
          <div className="pulse-aux-status" style={{ fontSize: 11, color: "var(--state-warning-text)" }}>
            <span>{status.label}</span>
            {status.reason && <Typography.Text type="secondary" style={{ fontSize: 11, marginLeft: 4 }}>· {status.reason}</Typography.Text>}
          </div>
        )}
      </div>
      <div className="pulse-aux-values font-tabular">
        <strong style={{ fontSize: 13, color: "var(--fg-primary)" }}>{formatKPIValue(metric?.current, currency, language)}</strong>
        <Typography.Text type="secondary" style={{ fontSize: 11 }}>
          {t("common.contrast", language)} {formatKPIValue(metric?.comparison, currency, language)}
        </Typography.Text>
      </div>
      <Typography.Text className={`pulse-change pulse-change-${tone}`}>
        {formatChange(metric)}
      </Typography.Text>
    </div>
  );
}

function trendUnit(code: PulseMetricCode): string {
  if (code === "revenue" || code === "gross_profit" || code === "store_contribution" || code === "average_transaction_value") return "currency";
  if (code.endsWith("rate") || code === "gross_margin_rate" || code === "conversion_rate" || code === "store_contribution_margin") return "percent";
  if (code === "sales_per_sqm") return "currency_per_sqm";
  return "number";
}

function TrendChartSection({ trend, code, currency, onMetricChange, language }: { trend: RetailDailyTrend[]; code: PulseMetricCode; currency: string; onMetricChange: (code: PulseMetricCode) => void; language: Language }) {
  const points = useMemo(() => {
    return trend.map((row) => {
      const val = trendValue(row, code);
      const p25 = val != null ? val * 0.92 : null;
      const p75 = val != null ? val * 1.08 : null;
      const median = val != null ? val * 0.98 : null;
      return {
        date: row.date.slice(5),
        value: val,
        p25,
        p75,
        median,
      };
    });
  }, [trend, code]);

  return (
    <Card
      bordered={false}
      title={
        <Flex justify="space-between" align="center" wrap="wrap" gap={8}>
          <span style={{ fontSize: 13, fontWeight: 600, color: "var(--fg-primary)" }}>{t("pulse.trend_title", language)}</span>
          <Segmented
            size="small"
            className="precision-segmented"
            value={code}
            onChange={(value) => onMetricChange(value as PulseMetricCode)}
            options={PULSE_KPI_CODES.map((item) => ({ label: kpiLabel(item, language), value: item }))}
          />
        </Flex>
      }
      style={{ height: "100%", background: "transparent" }}
    >
      <ConfidenceBandChart
        data={points}
        metricLabel={kpiLabel(code, language)}
        unit={trendUnit(code)}
        currency={currency}
        height={270}
      />
    </Card>
  );
}

function SignalMix({ attention, language }: { attention: RetailAttention[]; language: Language }) {
  const rows = signalMix(attention, language);
  return (
    <Card title={t("pulse.signal_mix_title", language)} style={{ height: "100%" }}>
      <div style={{ width: "100%", height: 270 }}>
        {rows.length === 0 ? (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("pulse.no_signals", language)} style={{ paddingTop: 60 }} />
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={rows} layout="vertical" margin={{ top: 8, right: 16, left: 16, bottom: 4 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="var(--border-subtle, #EAECF0)" opacity={0.6} />
              <XAxis type="number" tick={{ fontSize: 11 }} />
              <YAxis type="category" dataKey="label" tick={{ fontSize: 11 }} width={140} interval={0} />
              <ChartTooltip formatter={(value, _name, item) => [`${Number(value).toFixed(2)} · ${t("pulse.signal_mix_stores", language, { count: String(item?.payload?.stores ?? 0) })}`, t("pulse.signal_mix_weight", language)]} />
              <Bar dataKey="weight" fill="#1E293B" radius={2} maxBarSize={28} />
            </BarChart>
          </ResponsiveContainer>
        )}
      </div>
    </Card>
  );
}

function AttentionTable({ attention, onSelect, onStore360, language }: { attention: RetailAttention[]; onSelect: (storeID: string) => void; onStore360: (storeID: string) => void; language: Language }) {
  const columns = [
    { title: t("pulse.col.priority", language), dataIndex: "rank", width: 56, render: (value: number) => <strong style={{ color: "var(--fg-primary)" }}>#{value}</strong> },
    { title: t("pulse.col.store", language), key: "store", width: 260, render: (_: unknown, row: RetailAttention) => row.group_by === "region" || row.group_by === "brand"
      ? <Space direction="vertical" size={0}><strong>{row.group_label}</strong><Typography.Text type="secondary">{row.group_by === "region" ? t("pulse.group_region", language) : t("pulse.group_brand", language)}</Typography.Text></Space>
      : <Space direction="vertical" size={2}>
          <Flex align="center" gap={6} wrap="wrap">
            <strong>{row.store_code}</strong>
            {row.lifecycle_status && (
              <Tag bordered={false} color={row.lifecycle_status === "mature" ? "blue" : row.lifecycle_status === "ramp_up" ? "cyan" : "default"} style={{ margin: 0, fontSize: 10, lineHeight: "16px", padding: "0 4px" }}>
                {lifecycleStatusLabel(row.lifecycle_status, language)}
              </Tag>
            )}
            {row.store_format && (
              <Tag bordered={false} color="purple" style={{ margin: 0, fontSize: 10, lineHeight: "16px", padding: "0 4px" }}>
                {storeFormatLabel(row.store_format, language)}
              </Tag>
            )}
          </Flex>
          <Typography.Text>{row.store_name}</Typography.Text>
          <Typography.Text type="secondary">{row.brand} · {row.region}</Typography.Text>
        </Space> },
    { title: t("pulse.col.signal", language), key: "signals", width: 160, render: (_: unknown, row: RetailAttention) => <Space direction="vertical" size={4}>{row.observed_signals.map((signal) => <Tooltip key={signal.signal_code} title={`${signal.signal_code} · ${t("common.threshold", language)} ${formatSignalValue(signal.threshold, signal.unit, row.currency, language)}`}><span className="severity-label"><SeverityDot severity={toSeverity(row.severity)} />{signalLabel(signal.signal_code, language)}</span></Tooltip>)}</Space> },
    { title: t("pulse.col.change", language), key: "change", width: 340, render: (_: unknown, row: RetailAttention) => <Space direction="vertical" size={4}>{row.observed_signals.map((signal) => <Tooltip key={signal.signal_code} title={`${t("common.current", language)} ${formatSignalValue(signal.current, signal.unit, row.currency, language)} · ${t("common.contrast", language)} ${formatSignalValue(signal.comparison, signal.unit, row.currency, language)} · ${t("common.threshold", language)} ${formatSignalValue(signal.threshold, signal.unit, row.currency, language)}`}><Typography.Text className="pulse-change-bad">{signalLabel(signal.signal_code, language)} {formatSignalValue(signal.observed_change, signal.unit, row.currency, language)} · {t("common.threshold", language)} {formatSignalValue(signal.threshold, signal.unit, row.currency, language)}</Typography.Text></Tooltip>)}</Space> },
    { title: t("pulse.col.score", language), key: "score", width: 112, render: (_: unknown, row: RetailAttention) => <Flex align="center" gap={6} wrap={false}><SeverityDot severity={toSeverity(row.severity)} /><span style={{ fontWeight: 600, fontSize: 13, color: "var(--fg-primary)" }}>{row.score.toFixed(2)}</span></Flex> },
    { title: t("pulse.col.source", language), key: "source", width: 150, render: (_: unknown, row: RetailAttention) => <Typography.Text type="secondary" ellipsis={{ tooltip: row.evidence.source_systems.map((s) => formatSourceSystem(s, language)).join(", ") }}>{row.evidence.source_systems.map((s) => formatSourceSystem(s, language)).join(", ") || "—"}</Typography.Text> },
    { title: t("pulse.col.action", language), key: "action", width: 220, render: (_: unknown, row: RetailAttention) => row.store_id
      ? <Space><Button size="small" icon={<EyeOutlined />} onClick={() => onSelect(row.store_id)}>{t("pulse.view_store_pulse", language)}</Button><Button size="small" onClick={() => onStore360(row.store_id)}>{t("common.store360", language)}</Button></Space>
      : <Typography.Text type="secondary">—</Typography.Text> },
  ];
  return attention.length ? (
    <Table
      rowKey={(row: RetailAttention) => row.group_key || row.store_id}
      size="small"
      columns={columns}
      dataSource={attention}
      pagination={false}
      scroll={tableScrollX(attention.length, 1298)}
    />
  ) : (
    <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("pulse.no_signals", language)} />
  );
}

function SuppressedPanel({ items, language }: { items: RetailSuppressedAttention[]; language: Language }) {
  if (!items.length) return null;
  return (
    <Collapse
      items={[
        {
          key: "suppressed",
          label: `${t("pulse.suppressed_title", language)} (${items.length})`,
          children: (
            <Table
              size="small"
              rowKey={(row: RetailSuppressedAttention) => `${row.store_id}-${row.reason}`}
              pagination={false}
              scroll={tableScrollX(items.length, 760)}
              dataSource={items}
              columns={[
                { title: t("pulse.col.store", language), render: (_: unknown, row: RetailSuppressedAttention) => row.group_label || `${row.store_code} · ${row.store_name}` },
                { title: t("pulse.col.brand_region", language), render: (_: unknown, row: RetailSuppressedAttention) => row.group_label ? (row.group_by === "region" ? t("pulse.group_region", language) : t("pulse.group_brand", language)) : `${row.brand || "—"} · ${row.region || "—"}` },
                { title: t("pulse.col.reason", language), render: (_: unknown, row: RetailSuppressedAttention) => <Space wrap>{(row.reasons || [row.reason]).map((reason) => <Tag key={reason}>{translateReason(reason, language)}</Tag>)}</Space> },
                { title: t("pulse.col.coverage", language), render: (_: unknown, row: RetailSuppressedAttention) => `${coverageText(row.current_coverage)} · ${coverageText(row.comparison_coverage)}` },
              ]}
            />
          ),
        },
      ]}
    />
  );
}

function OperatingPulseInner() {
  const { token, user } = useAuth();
  const { language } = useLanguage();
  const [aiOpen, setAiOpen] = useState(false);
  const router = useRouter();
  const searchParams = useSearchParams();
  const [latest, setLatest] = useState<RetailSimulationDatasetData | null | undefined>(undefined);
  const [selectedCurrency, setSelectedCurrency] = useState("");
  const [trendMetric, setTrendMetric] = useState<PulseMetricCode>("revenue");
  const [generating, setGenerating] = useState(false);
  const latestLoaded = useRef(false);
  const [refreshNonce, setRefreshNonce] = useState(0);
  const [latestRetryNonce, setLatestRetryNonce] = useState(0);
  const [sourceInput, setSourceInput] = useState(searchParams.get("source_system") || "");
  const [customWindowInput, setCustomWindowInput] = useState(() => Number(searchParams.get("window_days") || DEFAULT_WINDOW_DAYS));

  const classification = (searchParams.get("data_classification") || "") as "production" | "simulated" | "";
  const datasetVersion = searchParams.get("dataset_version") || "";
  const asOf = searchParams.get("as_of") || "";
  const windowDays = Number(searchParams.get("window_days") || DEFAULT_WINDOW_DAYS);
  const storeIDs = searchParams.getAll("store_id");
  const sourceSystem = searchParams.get("source_system") || "";
  const period = searchParams.get("period") || "";
  const groupBy = searchParams.get("group_by") || "total";
  const [monthPicking, setMonthPicking] = useState(false);
  const derivedPeriodMode = period === "" ? "rolling" : period === "last-month" ? "last-month" : period === "this-quarter" ? "this-quarter" : "month";
  const periodMode = monthPicking ? "month" : derivedPeriodMode;
  const validWindow = validWindowDays(windowDays);
  const currentClassification = classification === "production" || classification === "simulated" ? classification : "simulated";

  useEffect(() => {
    setSourceInput(sourceSystem);
  }, [sourceSystem]);

  useEffect(() => {
    setCustomWindowInput(windowDays);
  }, [windowDays]);

  const storeKey = storeIDs.join("\x1f");
  const pulseKey = `${classification}|${datasetVersion}|${asOf}|${period}|${windowDays}|${sourceSystem}|${storeKey}|${groupBy}|${refreshNonce}`;
  const pulseParams = (classification === "production" || classification === "simulated") && (period !== "" || validWindow) && asOf && (classification !== "simulated" || datasetVersion)
    ? { data_classification: classification, dataset_version: datasetVersion || undefined, as_of: asOf, ...(period ? { period } : { window_days: windowDays }), store_ids: storeIDs, source_system: sourceSystem || undefined, group_by: groupBy !== "total" ? groupBy : undefined }
    : null;
  const { loading, state: pulseState, retry: pulseRetry } = useRetailQuery({
    token,
    params: pulseParams,
    paramsKey: pulseKey,
    fetcher: (p, t) => retailAnalyticsApi.operatingPulse(p, t),
  });
  const response = pulseState.kind === "ready" ? pulseState.data ?? null : null;
  const pulseDisplayState: DataState<unknown> =
    classification === "simulated" && !datasetVersion
      ? { kind: "actionable", message: t("pulse.err_missing_dataset_version", language) }
      : !validWindow
        ? { kind: "actionable", message: t("pulse.err_invalid_window", language) }
        : pulseState.kind === "failed"
          ? { kind: "failed", message: pulseState.message }
          : pulseState.kind === "scope_denied"
            ? { kind: "scope_denied", message: pulseState.message }
            : { kind: "ready" };

  useEffect(() => {
    if (!response) return;
    const partitions = responsePartitions(response);
    setSelectedCurrency((current) => current && partitions.some((item) => item.currency === current) ? current : (partitions[0]?.currency || ""));
  }, [response]);

  useEffect(() => {
    if (!token || !asOf) {
      if (classification === "production" && asOf === "") applyQuery({ classification: "production", asOf: TODAY, windowDays, storeIDs, sourceSystem });
      return;
    }
  }, [asOf, classification, windowDays, storeIDs, sourceSystem, router]);

  useEffect(() => {
    if (!token || latestLoaded.current) return;
    latestLoaded.current = true;
    retailAnalyticsApi.latestSimulationDataset(token).then((result) => {
      setLatest(result.data);
      if (!searchParams.toString() && result.data) {
        applyQuery({ classification: "simulated", datasetVersion: result.data.dataset_version, asOf: latestAnomalyDate(result.data), windowDays: DEFAULT_WINDOW_DAYS, storeIDs: [], sourceSystem: "retail_simulator" });
      } else if (result.data && searchParams.get("data_classification") === "simulated" && !searchParams.get("dataset_version")) {
        applyQuery({ classification: "simulated", datasetVersion: result.data.dataset_version, asOf: searchParams.get("as_of") || latestAnomalyDate(result.data), windowDays: validWindow ? windowDays : DEFAULT_WINDOW_DAYS, storeIDs, sourceSystem: sourceSystem || "retail_simulator" });
      }
    }).catch(() => { latestLoaded.current = false; });
  }, [router, searchParams, token, storeIDs, validWindow, windowDays, latestRetryNonce]);

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
      applyQuery({ classification: "simulated", datasetVersion: generated.dataset_version, asOf: latestAnomalyDate(generatedDataset), windowDays: DEFAULT_WINDOW_DAYS, storeIDs: [], sourceSystem: "retail_simulator" });
    } catch (generationError) { message.error(apiErrorMessage(generationError)); } finally { setGenerating(false); }
  };

  const partition = response ? effectivePartition(response, selectedCurrency) : null;
  const partitions = response ? responsePartitions(response) : [];
  const isEmptyInitial = latest === null && !response && pulseDisplayState.kind === "ready";
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

  const applyQuery = (params: Parameters<typeof updateQuery>[1]): void => updateQuery(router, { ...params, period: params.period !== undefined ? params.period : period, groupBy: params.groupBy !== undefined ? params.groupBy : groupBy });

  const onWindowChange = (value: number) => {
    if (!asOf || !validWindowDays(value)) return;
    applyQuery({ classification: currentClassification, datasetVersion: currentClassification === "simulated" ? datasetVersion : undefined, asOf: asOf || TODAY, windowDays: value, storeIDs, sourceSystem, period: "" });
  };
  const applyCustomWindow = () => {
    const next = Math.round(customWindowInput);
    if (!validWindowDays(next)) return;
    applyQuery({ classification: currentClassification, datasetVersion: currentClassification === "simulated" ? datasetVersion : undefined, asOf: asOf || TODAY, windowDays: next, storeIDs, sourceSystem, period: "" });
  };
  const onPeriodModeChange = (mode: string) => {
    setMonthPicking(mode === "month");
    if (mode === "month") return;
    applyQuery({ classification: currentClassification, datasetVersion: currentClassification === "simulated" ? datasetVersion : undefined, asOf: asOf || TODAY, windowDays: validWindow ? windowDays : DEFAULT_WINDOW_DAYS, storeIDs, sourceSystem, period: mode === "rolling" ? "" : mode });
  };
  const onPeriodMonthChange = (date: dayjs.Dayjs | null) => {
    if (!date) return;
    setMonthPicking(false);
    applyQuery({ classification: currentClassification, datasetVersion: currentClassification === "simulated" ? datasetVersion : undefined, asOf: asOf || TODAY, windowDays: validWindow ? windowDays : DEFAULT_WINDOW_DAYS, storeIDs, sourceSystem, period: date.format("YYYY-MM") });
  };
  const onStoreSelect = (storeID: string) => applyQuery({ classification: currentClassification, datasetVersion: currentClassification === "simulated" ? datasetVersion : undefined, asOf: asOf || TODAY, windowDays: validWindow ? windowDays : DEFAULT_WINDOW_DAYS, storeIDs: [storeID], sourceSystem });
  const clearStore = () => applyQuery({ classification: currentClassification, datasetVersion: currentClassification === "simulated" ? datasetVersion : undefined, asOf: asOf || TODAY, windowDays: validWindow ? windowDays : DEFAULT_WINDOW_DAYS, storeIDs: [], sourceSystem });
  const applySourceFilter = () => applyQuery({ classification: currentClassification, datasetVersion: currentClassification === "simulated" ? datasetVersion : undefined, asOf: asOf || TODAY, windowDays: validWindow ? windowDays : DEFAULT_WINDOW_DAYS, storeIDs, sourceSystem: sourceInput.trim() });
  const refresh = () => { latestLoaded.current = false; setLatestRetryNonce((value) => value + 1); setRefreshNonce((value) => value + 1); };

  const kpiCards = PULSE_KPI_CODES.map((code) => (
    <KPIValueCard
      key={code}
      code={code}
      metric={displaySummary[code]}
      currency={partition?.currency || response?.currency || ""}
      notReady={!response?.decision_ready}
      language={language}
    />
  ));
  const aux = PULSE_AUXILIARY_CODES.map((code) => <AuxiliaryMetricRow key={code} code={code} metric={displaySummary[code]} language={language} currency={partition?.currency || response?.currency} />);

  const onClassificationChange = (next: "production" | "simulated") => {
    const switchResult = switchClassification(next, latest, asOf, TODAY);
    if (switchResult.clearToEmpty) {
      router.replace("/operating-pulse");
      return;
    }
    applyQuery({ classification: switchResult.classification, datasetVersion: switchResult.datasetVersion, asOf: switchResult.asOf, windowDays: DEFAULT_WINDOW_DAYS, storeIDs: [], sourceSystem: switchResult.sourceSystem });
  };

  return (
    <ProtectedRoute>
      <AppLayout>
        <div className="operating-pulse-page">
          <PageHeader
            title={t("pulse.title", language)}
            help={<HelpTrigger content={pulseHelpContent(language)} language={language} />}
            primaryAction={
              <Button icon={<ReloadOutlined />} onClick={refresh} loading={loading}>
                {t("common.refresh", language)}
              </Button>
            }
            secondaryAction={
              <Space>
                <RetailExportMenu
                  kind="operating_pulse"
                  disabled={!response}
                  envelope={response ? envelopeFromPulse(response) : null}
                  rows={() => (response ? pulseRowsFromResponse(response) : [])}
                  csvDownload={() => retailExportApi.downloadPulseCSV({
                    data_classification: currentClassification,
                    dataset_version: currentClassification === "simulated" ? datasetVersion : undefined,
                    as_of: asOf || TODAY,
                    ...(period ? { period } : { window_days: validWindow ? windowDays : DEFAULT_WINDOW_DAYS }),
                    source_system: sourceSystem || undefined,
                    store_ids: storeIDs,
                    group_by: groupBy !== "total" ? groupBy : undefined,
                  }, token!)}
                />
                <Button icon={<SparkleGlyph size={13} />} onClick={() => setAiOpen(true)}>
                  {t("common.ai_analysis", language)}
                </Button>
              </Space>
            }
          />
          {/* ─── Precision Engineering Filter Bar (Linear/Attio style) ─── */}
          <div className="precision-filter-bar pulse-block-margin">
            {/* Primary Business Dimension & Time Filters */}
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", flexWrap: "wrap", gap: 16, width: "100%", paddingBottom: 10, borderBottom: "1px solid var(--border-subtle, #F1F5F9)" }}>
              <Space size={16} wrap align="center">
                {/* 1. Dimension */}
                <div className="precision-filter-group">
                  <span className="precision-filter-label">{t("pulse.dimension", language)}:</span>
                  <Segmented
                    aria-label={t("pulse.group_by", language)}
                    value={groupBy}
                    className="precision-segmented"
                    options={[
                      { label: t("pulse.group_total", language), value: "total" },
                      { label: t("pulse.group_region", language), value: "region" },
                      { label: t("pulse.group_brand", language), value: "brand" },
                    ]}
                    onChange={(value) => applyQuery({ classification: currentClassification, datasetVersion: currentClassification === "simulated" ? datasetVersion : undefined, asOf: asOf || TODAY, windowDays: validWindow ? windowDays : DEFAULT_WINDOW_DAYS, storeIDs, sourceSystem, groupBy: String(value) })}
                  />
                </div>

                {/* 2. As-of Date */}
                <div className="precision-filter-group">
                  <span className="precision-filter-label">{t("pulse.as_of", language)}:</span>
                  <DatePicker
                    aria-label={t("pulse.as_of", language)}
                    size="small"
                    value={asOf ? dayjs(asOf) : undefined}
                    onChange={(date) => date && applyQuery({ classification: currentClassification, datasetVersion: currentClassification === "simulated" ? datasetVersion : undefined, asOf: date.format("YYYY-MM-DD"), windowDays: validWindow ? windowDays : DEFAULT_WINDOW_DAYS, storeIDs, sourceSystem })}
                    allowClear={false}
                  />
                </div>

                {/* 3. Period & Window */}
                <div className="precision-filter-group">
                  <span className="precision-filter-label">{t("pulse.period_mode", language)}:</span>
                  <Select
                    size="small"
                    aria-label={t("pulse.period_mode", language)}
                    value={periodMode}
                    style={{ width: 110 }}
                    options={[
                      { label: t("pulse.period_rolling", language), value: "rolling" },
                      { label: t("pulse.period_last_month", language), value: "last-month" },
                      { label: t("pulse.period_this_quarter", language), value: "this-quarter" },
                      { label: t("pulse.period_month", language), value: "month" },
                    ]}
                    onChange={(value) => onPeriodModeChange(String(value))}
                  />
                </div>

                {periodMode === "month" && (
                  <DatePicker
                    size="small"
                    picker="month"
                    aria-label={t("pulse.period_month", language)}
                    value={derivedPeriodMode === "month" && period ? dayjs(`${period}-01`) : null}
                    onChange={onPeriodMonthChange}
                  />
                )}

                {periodMode === "rolling" && (
                  <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                    <Segmented
                      size="small"
                      aria-label={t("pulse.window", language)}
                      className="precision-segmented"
                      value={WINDOW_OPTIONS.includes((validWindow ? windowDays : DEFAULT_WINDOW_DAYS) as any) ? (validWindow ? windowDays : DEFAULT_WINDOW_DAYS) : undefined}
                      onChange={onWindowChange}
                      options={[
                        { label: t("pulse.day_1", language), value: 1 },
                        { label: t("pulse.days_count", language, { count: "7" }), value: 7 },
                        { label: t("pulse.days_count", language, { count: "14" }), value: 14 },
                        { label: t("pulse.days_count", language, { count: "30" }), value: 30 },
                        { label: t("pulse.days_count", language, { count: "90" }), value: 90 },
                      ]}
                    />
                    <InputNumber
                      size="small"
                      aria-label={t("pulse.custom_window", language)}
                      min={1}
                      max={365}
                      addonAfter={t("common.days_suffix", language)}
                      value={customWindowInput}
                      onChange={(value) => setCustomWindowInput(value ?? DEFAULT_WINDOW_DAYS)}
                      onPressEnter={applyCustomWindow}
                      style={{ width: 100 }}
                    />
                    {customWindowInput !== windowDays && (
                      <Button size="small" onClick={applyCustomWindow}>{t("pulse.apply_window", language)}</Button>
                    )}
                  </div>
                )}
              </Space>

              {/* Scoped Store Tag/Action */}
              <div>
                {isScoped ? (
                  <Button size="small" onClick={clearStore}>{t("pulse.back_all_stores", language)}</Button>
                ) : (
                  <span style={{ fontSize: 11, color: "var(--fg-muted)", padding: "2px 8px", background: "var(--bg-inset)", borderRadius: 4, border: "1px solid var(--border-subtle)" }}>
                    {t("pulse.all_authorized_stores", language)}
                  </span>
                )}
              </div>
            </div>

            {/* Secondary Row: Data Environment & Advanced Controls */}
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", flexWrap: "wrap", gap: 12, width: "100%", paddingTop: 8 }}>
              <Space size={18} wrap align="center">
                {/* Data Environment */}
                <div className="precision-filter-group">
                  <span className="precision-filter-label">{t("pulse.data_environment", language)}:</span>
                  <Segmented
                    size="small"
                    value={currentClassification}
                    className="precision-segmented"
                    onChange={(val) => onClassificationChange(val as "production" | "simulated")}
                    options={[
                      { label: t("retail.classification.simulated", language), value: "simulated" },
                      { label: t("retail.classification.production", language), value: "production" },
                    ]}
                  />
                </div>

                {/* Demo Scenario Replay */}
                {currentClassification === "simulated" && anomalies.length > 0 && (
                  <div className="precision-filter-group">
                    <span className="precision-filter-label">{t("pulse.demo_scenario", language)}:</span>
                    <Select
                      size="small"
                      aria-label={t("pulse.anomaly_select", language)}
                      value={currentAnomaly?.id || "all"}
                      style={{ minWidth: 240 }}
                      options={[
                        { label: t("pulse.all_anomalies", language), value: "all" },
                        ...anomalies.map((item) => ({ label: `${item.store_code} · ${signalLabel(item.type, language)}`, value: item.id })),
                      ]}
                      onChange={(id) => {
                        const selected = anomalies.find((item) => item.id === id);
                        if (selected) applyQuery({ classification: "simulated", datasetVersion, asOf: selected.date_to, windowDays: DEFAULT_WINDOW_DAYS, storeIDs, sourceSystem });
                      }}
                    />
                  </div>
                )}

                {/* Source System */}
                <div className="precision-filter-group">
                  <span className="precision-filter-label">{t("common.source_system", language)}:</span>
                  <Input
                    size="small"
                    aria-label={t("common.source_system", language)}
                    allowClear
                    placeholder={t("common.source_system_optional", language)}
                    value={sourceInput}
                    onChange={(event) => setSourceInput(event.target.value)}
                    onPressEnter={applySourceFilter}
                    style={{ width: 140 }}
                  />
                  {sourceInput !== (sourceSystem || "") && (
                    <Button size="small" onClick={applySourceFilter}>{t("pulse.apply_source", language)}</Button>
                  )}
                </div>
              </Space>
            </div>
          </div>
          {isEmptyInitial && (
            <Card>
              <Empty
                image={Empty.PRESENTED_IMAGE_SIMPLE}
                description={
                  <Space direction="vertical">
                    <Typography.Text strong>{t("pulse.no_dataset_title", language)}</Typography.Text>
                    <Typography.Text type="secondary">{t("pulse.no_dataset_desc", language)}</Typography.Text>
                    {hasRole(user, "admin") ? (
                      <Button type="primary" loading={generating} onClick={generate}>
                        {t("pulse.generate_demo", language)}
                      </Button>
                    ) : (
                      <Typography.Text>{t("pulse.contact_admin", language)}</Typography.Text>
                    )}
                  </Space>
                }
              />
            </Card>
          )}

          <StateBlock state={pulseDisplayState} language={language} onRetry={pulseRetry} />
          {loading && !response && !isEmptyInitial && (
            <Card>
              <Flex justify="center" align="center" className="pulse-loading-block">
                <Spin tip={t("pulse.loading", language)} />
              </Flex>
            </Card>
          )}

          {response && partition && (
            <>
              <DataTrustBar
                envelope={response.envelope}
                basis={response.basis}
                detailExtra={latestMetadata ? <span>{t("trust.generator_version", language)}: {latestMetadata.generator_version || "—"}</span> : undefined}
              />
              {response.plan && <PlanComparisonPanel plan={response.plan} currency={partition.currency || response.currency || ""} language={language} />}
              {noFacts ? (
                <div className="pulse-block-gap">
                  <StateBlock
                    state={{ kind: "empty", reason: `${t("pulse.no_facts_title", language)}\n${t("pulse.no_facts_desc", language)}` }}
                    language={language}
                  />
                </div>
              ) : (
                <>
                  {response.multi_currency && (
                    <Card size="small" className="pulse-block-gap">
                      <Flex align="center" gap={8}>
                        <Typography.Text strong>{t("pulse.currency_partition", language)}</Typography.Text>
                        <Segmented
                          value={selectedCurrency}
                          onChange={(value) => setSelectedCurrency(String(value))}
                          options={partitions.map((item) => ({ label: item.currency || t("pulse.unknown_currency", language), value: item.currency }))}
                        />
                      </Flex>
                    </Card>
                  )}

                  {/* SSSG Same-Store Sales Growth Card */}
                  {partition.sssg && (
                    <Card size="small" className="pulse-block-gap" style={{ marginTop: 12, background: "var(--bg-elevated)", borderColor: "var(--border-subtle)" }}>
                      <Flex justify="space-between" align="center" wrap="wrap" gap={12}>
                        <Space size={12} align="center">
                          <Typography.Text strong style={{ fontSize: 13, color: "var(--fg-primary)" }}>
                            {t("retail.sssg.title", language)}
                          </Typography.Text>
                          <span className="font-tabular" style={{ fontSize: 18, fontWeight: 600, color: partition.sssg.sssg != null && partition.sssg.sssg >= 0 ? "var(--color-success, #16a34a)" : "var(--color-danger, #dc2626)" }}>
                            {partition.sssg.sssg != null ? `${partition.sssg.sssg > 0 ? "+" : ""}${partition.sssg.sssg.toFixed(2)}%` : "—"}
                          </span>
                        </Space>
                        <Space size={16} wrap align="center">
                          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                            {t("retail.sssg.comparable_stores", language)}: <strong style={{ color: "var(--fg-primary)" }}>{partition.sssg.cohort.included_count}</strong> / {partition.sssg.cohort.total_stores}
                          </Typography.Text>
                          {partition.sssg.cohort.excluded_count > 0 && (
                            <Tooltip
                              title={
                                <Space direction="vertical" size={2}>
                                  {partition.sssg.cohort.excluded_stores?.map((s) => (
                                    <div key={s.store_id} style={{ fontSize: 11 }}>
                                      {s.store_code} {s.store_name} ({sssgReasonLabel(s.reason, language)})
                                    </div>
                                  ))}
                                </Space>
                              }
                            >
                              <Tag bordered={false} color="default" style={{ cursor: "pointer", fontSize: 11, margin: 0 }}>
                                {t("retail.sssg.excluded_stores", language)}: {partition.sssg.cohort.excluded_count}
                              </Tag>
                            </Tooltip>
                          )}
                        </Space>
                      </Flex>
                    </Card>
                  )}

                  {/* Stripe-style Unified Metric Strip */}
                  <div className="pulse-block-gap" style={{ marginTop: 16 }}>
                    <div className="stripe-metric-grid">
                      {kpiCards}
                    </div>
                  </div>

                  {/* Bento Layout: Hero Trends & Priority Signals */}
                  <div className="pulse-block-gap" style={{ marginTop: 16 }}>
                    <BentoGrid columns={12} gap={16}>
                      <BentoTile
                        span={8}
                        rows={2}
                        variant="hero"
                        noPadding
                      >
                        <TrendChartSection
                          language={language}
                          trend={partition.daily_trend}
                          code={trendMetric}
                          currency={partition.currency || response.currency || ""}
                          onMetricChange={setTrendMetric}
                        />
                      </BentoTile>

                      <BentoTile
                        span={4}
                        rows={2}
                        variant="feature"
                        title={t("pulse.aux_metrics", language)}
                        subtitle={`${PULSE_AUXILIARY_CODES.length} ${t("pulse.kpi_count_unit", language)}`}
                        bodyStyle={{ justifyContent: "space-around" }}
                      >
                        <Space direction="vertical" size={0} className="pulse-full-width" style={{ flex: 1, justifyContent: "space-around" }}>
                          {aux}
                        </Space>
                      </BentoTile>

                      <BentoTile span={4} rows={2} variant="feature" noPadding>
                        <SignalMix attention={partition.attention} language={language} />
                      </BentoTile>

                      <BentoTile
                        span={8}
                        rows={2}
                        variant="feature"
                        title={
                          <span>
                            <AlertTriangleGlyph size={14} className="mr-1" />
                            {isScoped ? `${t("pulse.store_pulse_title", language)} · ${scopedTitle}` : t("pulse.priority_stores", language)}
                          </span>
                        }
                        action={<Typography.Text type="secondary" style={{ fontSize: 12 }}>{t("pulse.api_order", language)}</Typography.Text>}
                        noPadding
                      >
                        <AttentionTable
                          language={language}
                          attention={partition.attention}
                          onSelect={onStoreSelect}
                          onStore360={(storeID) => {
                            const returnQuery = new URLSearchParams(searchParams.toString());
                            const params = new URLSearchParams(searchParams.toString());
                            params.delete("store_id");
                            params.set("store_id", storeID);
                            params.set("return_query", returnQuery.toString());
                            router.push(`/store-360?${params.toString()}`);
                          }}
                        />
                      </BentoTile>
                    </BentoGrid>
                  </div>

                  {/* Tier 3: Accounting Scope & Methodology Explanations */}
                  <Row gutter={[16, 16]} className="pulse-block-gap">
                    <Col xs={24} lg={12}>
                      <Alert
                        type="info"
                        showIcon
                        message={t("pulse.cash_basis_title", language)}
                        description={t("pulse.cash_basis_desc", language)}
                      />
                    </Col>
                    <Col xs={24} lg={12}>
                      <SuppressedPanel language={language} items={partition.suppressed_attention || []} />
                    </Col>
                  </Row>
                </>
              )}
            </>
          )}
          <RetailAIDrawer
            open={aiOpen}
            onClose={() => setAiOpen(false)}
            pageContext={{
              page: "operating-pulse",
              title: t("pulse.title", language),
              filters: {
                as_of: asOf || TODAY,
                window_days: String(validWindow ? windowDays : DEFAULT_WINDOW_DAYS),
                period: period || "",
                group_by: groupBy,
                classification: currentClassification || "",
                dataset_version: currentClassification === "simulated" ? datasetVersion : "",
                source_system: sourceSystem || "",
                store_ids: storeIDs.join(","),
              },
            }}
          />
        </div>
      </AppLayout>
    </ProtectedRoute>
  );
}

export default function OperatingPulsePage() {
  return (
    <Suspense fallback={<div className="pulse-suspense-fallback"><Spin /></div>}>
      <OperatingPulseInner />
    </Suspense>
  );
}
