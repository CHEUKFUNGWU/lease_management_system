"use client";

import { useEffect, useMemo, useRef, useState, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Alert, Button, Card, Col, Collapse, DatePicker, Empty, Flex, Input, InputNumber, Modal, Radio, Row, Select, Space, Spin, Table, Tag, Typography, message } from "antd";
import { ArrowLeftOutlined, PlayCircleOutlined, SaveOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import DataTrustBar, { KPIReadyBadge } from "../components/DataTrustBar";
import RetailAIDrawer from "../components/RetailAIDrawer";
import ProtectedRoute from "../components/ProtectedRoute";
import { apiErrorMessage, retailAnalyticsApi, type RetailDataClassification, type RetailScenarioAssumptions, type RetailScenarioResponse, type RetailStore360Option } from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t, type Language } from "../lib/i18n";
import { latestAnomalyDate } from "../operating-pulse/logic";
import { acceptsEvaluation, actionKey, bridgeConservation, canSaveScenario, defaultAssumptions, evaluationSnapshotKey, formatScenarioValue, responseHorizonLabel, returnScenarioQuery, SCENARIO_CODES, scenarioLabel } from "./logic";
import { StatusTag } from "../components/StatusTag";

const WINDOW_OPTIONS = [7, 14, 28] as const;
const DELTA_FIELDS: Array<{ key: keyof RetailScenarioAssumptions; labelKey: string; unit: "pct" | "pp" }> = [
  { key: "revenue_change_pct", labelKey: "scenario.delta.revenue_change_pct", unit: "pct" },
  { key: "gross_margin_rate_change_pp", labelKey: "scenario.delta.gross_margin_rate_change_pp", unit: "pp" },
  { key: "labor_cost_change_pct", labelKey: "scenario.delta.labor_cost_change_pct", unit: "pct" },
  { key: "fixed_rent_change_pct", labelKey: "scenario.delta.fixed_rent_change_pct", unit: "pct" },
  { key: "variable_rent_rate_change_pp", labelKey: "scenario.delta.variable_rent_rate_change_pp", unit: "pp" },
  { key: "non_lease_cost_change_pct", labelKey: "scenario.delta.non_lease_cost_change_pct", unit: "pct" },
  { key: "other_controllable_cost_change_pct", labelKey: "scenario.delta.other_controllable_cost_change_pct", unit: "pct" },
];

function parseURL(params: { get(name: string): string | null }) {
  const classification = params.get("data_classification");
  const validClassification: RetailDataClassification = classification === "production" ? "production" : "simulated";
  const rawWindow = Number(params.get("window_days") || 14);
  const windowDays = WINDOW_OPTIONS.includes(rawWindow as (typeof WINDOW_OPTIONS)[number]) ? rawWindow as 7 | 14 | 28 : 14;
  const defaults = defaultAssumptions();
  const assumptionKeys = Object.keys(defaults) as Array<keyof RetailScenarioAssumptions>;
  const assumptions = { ...defaults };
  for (const key of assumptionKeys) {
    const raw = params.get(key);
    if (raw !== null && Number.isFinite(Number(raw))) assumptions[key] = Number(raw);
  }
  const rawHorizon = Number(params.get("horizon_months") || 12);
  const horizon = rawHorizon === 3 || rawHorizon === 6 || rawHorizon === 12 ? rawHorizon : 12;
  return {
    storeID: params.get("store_id") || "",
    classification: validClassification,
    datasetVersion: params.get("dataset_version") || "",
    asOf: params.get("as_of") || dayjs().format("YYYY-MM-DD"),
    windowDays,
    sourceSystem: params.get("source_system") || "",
    horizon,
    assumptions,
    returnQuery: params.get("return_query") || "",
  };
}

function MetricTable({ response, selectedKey, notReady, language }: { response: RetailScenarioResponse; selectedKey: string; notReady?: boolean; language: Language }) {
  const selected = response.scenarios.find((item) => item.key === selectedKey);
  if (!selected) return null;
  const rows = SCENARIO_CODES.map((code) => ({ code, baseline: response.baseline.metrics[code], result: selected.metrics[code] }));
  return <Table size="small" pagination={false} rowKey="code" dataSource={rows} columns={[
    { title: t("scenario.col.metric", language), render: (_: unknown, row: typeof rows[number]) => scenarioLabel(row.code, language) || row.code },
    { title: t("scenario.col.baseline", language), render: (_: unknown, row: typeof rows[number]) => formatScenarioValue(row.baseline?.result, row.baseline?.unit || "", response.currency, language) },
    { title: t("scenario.col.plan", language), render: (_: unknown, row: typeof rows[number]) => formatScenarioValue(row.result?.result, row.result?.unit || "", response.currency, language) },
    { title: t("scenario.col.change", language), render: (_: unknown, row: typeof rows[number]) => formatScenarioValue(row.result?.delta, row.result?.unit || "", response.currency, language) },
    { title: t("scenario.col.status", language), render: (_: unknown, row: typeof rows[number]) => <Flex align="center" gap={4}>{notReady && <KPIReadyBadge />}<StatusTag kind={row.result?.status === "complete" ? "success" : "warning"}>{row.result?.status || "unavailable"}{row.result?.reason ? ` · ${row.result.reason}` : ""}</StatusTag></Flex> },
  ]} />;
}

function ScenarioPageInner() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { token } = useAuth();
  const { language } = useLanguage();
  const [aiOpen, setAiOpen] = useState(false);
  const query = useMemo(() => parseURL(searchParams), [searchParams]);
  const [latest, setLatest] = useState<import("../lib/api").RetailSimulationDatasetData | null | undefined>(undefined);
  const [options, setOptions] = useState<RetailStore360Option[]>([]);
  const [loadingOptions, setLoadingOptions] = useState(false);
  const [optionsError, setOptionsError] = useState<string | null>(null);
  const [optionsRetry, setOptionsRetry] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [response, setResponse] = useState<RetailScenarioResponse | null>(null);
  const [responseKey, setResponseKey] = useState<string | null>(null);
  const [assumptions, setAssumptions] = useState<RetailScenarioAssumptions>(() => query.assumptions);
  const [horizon, setHorizon] = useState(query.horizon);
  const [selectedKey, setSelectedKey] = useState("plan");
  const [title, setTitle] = useState("");
  const [plannedAction, setPlannedAction] = useState("");
  const [ownerName, setOwnerName] = useState("");
  const [dueDate, setDueDate] = useState<string | null>(null);
  const [verificationPeriod, setVerificationPeriod] = useState(dayjs().format("YYYY-MM"));
  const [saving, setSaving] = useState(false);
  const [actionResult, setActionResult] = useState<{ data: Record<string, unknown>; idempotentReplay: boolean } | null>(null);
  const requestSequence = useRef(0);

  useEffect(() => {
    if (!token) return;
    retailAnalyticsApi.latestSimulationDataset(token).then((result) => setLatest(result.data)).catch(() => setLatest(null));
  }, [token]);

  useEffect(() => {
    if (!token) return;
    let active = true;
    if (query.classification === "simulated" && !query.datasetVersion) { setOptions([]); setOptionsError(null); return () => { active = false; }; }
    setLoadingOptions(true);
    setOptionsError(null);
    retailAnalyticsApi.storeOptions({ data_classification: query.classification, dataset_version: query.datasetVersion || undefined }, token).then((result) => { if (active) setOptions(result.data); }).catch((err) => { if (active) { setOptions([]); setOptionsError(apiErrorMessage(err)); } }).finally(() => { if (active) setLoadingOptions(false); });
    return () => { active = false; };
  }, [token, query.classification, query.datasetVersion, optionsRetry]);

  const latestMatches = query.classification === "simulated" && latest?.dataset_version === query.datasetVersion ? latest : null;
  const selectedStore = options.find((item) => item.store_id === query.storeID);
  const currentScope = useMemo(() => ({ store_id: query.storeID, data_classification: query.classification, dataset_version: query.datasetVersion || undefined, as_of: query.asOf, window_days: query.windowDays, source_system: query.sourceSystem || undefined }), [query.storeID, query.classification, query.datasetVersion, query.asOf, query.windowDays, query.sourceSystem]);
  const currentEvaluationKey = useMemo(() => evaluationSnapshotKey(currentScope, horizon, assumptions), [currentScope, horizon, assumptions]);
  const freshResponse = response && canSaveScenario(response, selectedKey, responseKey, currentEvaluationKey) ? response : null;
  const setQuery = (next: Partial<typeof query>) => {
    const q = new URLSearchParams();
    const classification = next.classification || query.classification;
    q.set("data_classification", classification);
    const datasetVersion = next.datasetVersion ?? query.datasetVersion;
    if (classification === "simulated" && datasetVersion) q.set("dataset_version", datasetVersion);
    q.set("as_of", next.asOf || query.asOf);
    q.set("window_days", String(next.windowDays || query.windowDays));
    const storeID = next.storeID ?? query.storeID;
    if (storeID) q.set("store_id", storeID);
    const source = next.sourceSystem ?? query.sourceSystem;
    if (source) q.set("source_system", source);
    if (query.returnQuery) q.set("return_query", query.returnQuery);
    router.replace(`/scenario-workbench?${q.toString()}`);
  };

  useEffect(() => {
    if (query.classification !== "simulated" || query.datasetVersion || !latest) return;
    setQuery({ datasetVersion: latest.dataset_version, asOf: latestAnomalyDate(latest) });
    // Latest discovery is read-only; no generation or store selection happens here.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query.classification, query.datasetVersion, latest]);

  useEffect(() => {
    requestSequence.current += 1;
    setLoading(false);
    setError(null);
    setActionResult(null);
  }, [currentEvaluationKey]);

  const evaluate = async () => {
    if (!token || !query.storeID) { setError(t("scenario.err_select_store", language)); return; }
    if (query.classification === "simulated" && !query.datasetVersion) { setError(t("scenario.err_dataset_version", language)); return; }
    const sequence = ++requestSequence.current;
    const requestKey = currentEvaluationKey;
    setLoading(true); setError(null); setResponse(null); setResponseKey(null); setActionResult(null);
    try {
      const result = await retailAnalyticsApi.evaluateStoreScenario({ store_id: query.storeID, data_classification: query.classification, dataset_version: query.datasetVersion || undefined, as_of: query.asOf, window_days: query.windowDays, source_system: query.sourceSystem || undefined }, { horizon_months: horizon, scenarios: [{ key: "baseline", name: "Baseline", assumptions: defaultAssumptions() }, { key: "plan", name: "Plan", assumptions }] }, token);
      if (acceptsEvaluation(requestKey, currentEvaluationKey, sequence, requestSequence.current)) {
        setResponse(result);
        setResponseKey(requestKey);
      }
    } catch (err) {
      if (sequence === requestSequence.current) setError(apiErrorMessage(err));
    } finally {
      if (sequence === requestSequence.current) setLoading(false);
    }
  };

  const saveAction = async () => {
    if (!token || !freshResponse || !title.trim() || !plannedAction.trim()) return;
    const selected = freshResponse.scenarios.find((item) => item.key === selectedKey);
    if (!selected) return;
    const body = { horizon_months: horizon, selected_scenario: { key: selected.key, name: selected.name, assumptions: selected.assumptions }, title: title.trim(), planned_action: plannedAction.trim(), owner_name: ownerName.trim(), due_date: dueDate, verification_period: verificationPeriod };
    const key = actionKey({ evaluation: currentEvaluationKey, body });
    Modal.confirm({ title: t("scenario.confirm_save_title", language), content: t("scenario.confirm_save_content", language), onOk: async () => {
      setSaving(true);
      try { const result = await retailAnalyticsApi.saveStoreScenarioAction(currentScope, body, key, token); setActionResult({ data: result.data, idempotentReplay: result.idempotent_replay }); message.success(result.idempotent_replay ? t("scenario.saved_replay", language) : t("scenario.saved", language)); } catch (err) { message.error(apiErrorMessage(err)); } finally { setSaving(false); }
    } });
  };

  const backURL = query.returnQuery ? `/store-360?${query.returnQuery}` : returnScenarioQuery(searchParams);

  return <ProtectedRoute><AppLayout><div className="scenario-workbench-page">
    <PageHeader title={t("scenario.title", language)} meta={t("scenario.scope_note", language)} primaryAction={<Button type="primary" icon={<PlayCircleOutlined />} loading={loading} onClick={evaluate}>{t("scenario.calculate", language)}</Button>} secondaryAction={<Space><Button onClick={() => setAiOpen(true)}>{t("common.ai_analysis", language)}</Button><Button icon={<ArrowLeftOutlined />} onClick={() => router.push(backURL)}>{t("scenario.back_store360", language)}</Button></Space>} />
    {response ? <DataTrustBar envelope={response.envelope} basis="Scenario" detailExtra={latestMatches ? <span>generator: {latestMatches.generator_version} · latest anomaly: {latestAnomalyDate(latestMatches)}</span> : undefined} /> : <DataTrustBar envelope={{ data_classification: query.classification, source_systems: [], dataset_versions: [], fact_version_min: 0, fact_version_max: 0, current_coverage: {}, decision_ready: true, formula_version: "", pulse_version: "", semantic_version: "", generated_at: "" }} basis="Scenario" />}
    <Card size="small" style={{ marginTop: 16 }}><Flex gap={12} wrap="wrap" align="center"><Radio.Group value={query.classification} optionType="button" buttonStyle="solid" options={[{ label: t("retail.classification.simulated", language), value: "simulated" }, { label: t("retail.classification.production", language), value: "production" }]} onChange={(event) => { const next = event.target.value as RetailDataClassification; if (next === "production") setQuery({ classification: next, datasetVersion: "", storeID: "" }); else setQuery({ classification: next, datasetVersion: latest?.dataset_version || "", asOf: latest ? latestAnomalyDate(latest) : query.asOf, storeID: "" }); }} /><Select showSearch allowClear value={query.storeID || undefined} placeholder={t("store360.select_store", language)} loading={loadingOptions} style={{ minWidth: 260 }} options={options.map((item) => ({ label: `${item.store_code} · ${item.store_name}`, value: item.store_id, search: `${item.store_code} ${item.store_name} ${item.brand} ${item.region}` }))} optionFilterProp="search" onChange={(value) => setQuery({ storeID: value || "" })} /><DatePicker value={dayjs(query.asOf)} onChange={(value) => value && setQuery({ asOf: value.format("YYYY-MM-DD") })} /><Select value={query.windowDays} options={WINDOW_OPTIONS.map((value) => ({ label: `${value}${t("common.days_suffix", language)}`, value }))} onChange={(value) => setQuery({ windowDays: value })} /><Input value={query.sourceSystem} onChange={(event) => setQuery({ sourceSystem: event.target.value })} placeholder={t("common.source_system_optional", language)} style={{ width: 190 }} /></Flex></Card>
    {!query.storeID && <Card style={{ marginTop: 16 }}><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t("scenario.pick_store", language)} /></Card>}
    {query.classification === "simulated" && !query.datasetVersion && <Alert style={{ marginTop: 16 }} type="warning" message={t("scenario.missing_version_title", language)} description={t("scenario.missing_version_desc", language)} action={<Button size="small" onClick={() => router.push("/operating-pulse")}>{t("common.go_pulse", language)}</Button>} />}
    {optionsError && <Alert style={{ marginTop: 16 }} type="error" showIcon message={t("scenario.options_error", language)} description={optionsError} action={<Button size="small" onClick={() => setOptionsRetry((value) => value + 1)}>{t("common.retry", language)}</Button>} />}
    {query.classification && !loadingOptions && !optionsError && options.length === 0 && <Alert style={{ marginTop: 16 }} type="info" showIcon message={t("scenario.no_authorized", language)} description={t("scenario.no_authorized_desc", language)} />}
    {response && !freshResponse && <Alert style={{ marginTop: 16 }} type="warning" showIcon message={t("scenario.stale_title", language)} description={t("scenario.stale_desc", language)} action={<Button size="small" onClick={evaluate}>{t("scenario.recalculate", language)}</Button>} />}
    {selectedStore && <Card title={<Flex justify="space-between"><span>{selectedStore.store_code} · {selectedStore.store_name}</span><Typography.Text type="secondary">{selectedStore.brand} · {selectedStore.region}</Typography.Text></Flex>} style={{ marginTop: 16 }}><Row gutter={[12, 12]}>{DELTA_FIELDS.map((field) => <Col xs={24} sm={12} lg={8} key={field.key}><Typography.Text>{t(field.labelKey, language)}（{field.unit === "pp" ? "pp" : "%"}）</Typography.Text><InputNumber style={{ width: "100%" }} value={assumptions[field.key]} onChange={(value) => setAssumptions((current) => ({ ...current, [field.key]: value ?? 0 }))} min={field.unit === "pp" ? -100 : -100} max={field.unit === "pp" ? 100 : 300} /></Col>)}</Row><Flex gap={12} wrap="wrap" align="center" style={{ marginTop: 16 }}><span>{t("scenario.horizon", language)}</span><Select value={horizon} options={[3, 6, 12].map((value) => ({ value, label: t("scenario.months_option", language).replace("{n}", String(value)) }))} onChange={setHorizon} /><Typography.Text type="secondary">{t("scenario.baseline_note", language)}</Typography.Text></Flex></Card>}
    {error && <Alert style={{ marginTop: 16 }} type="error" showIcon message={t("scenario.unavailable", language)} description={error} />}
    {loading && <Card style={{ marginTop: 16 }}><Flex justify="center" style={{ minHeight: 160 }}><Spin tip={t("scenario.loading", language)} /></Flex></Card>}
    {response && <><Card style={{ marginTop: 16 }} title={<Flex justify="space-between"><span>{t("scenario.baseline_title", language).replace("{name}", response.scenarios.find((item) => item.key === selectedKey)?.name || "Plan")}</span><StatusTag kind={freshResponse ? "success" : "warning"}>{freshResponse ? "review_required · Working" : t("scenario.stale", language)}</StatusTag></Flex>}><MetricTable language={language} response={response} selectedKey={selectedKey} notReady={!response.envelope?.decision_ready} /><Flex gap={8} wrap="wrap" style={{ marginTop: 12 }}><Tag>{t("scenario.monthly_change", language)}：{formatScenarioValue(response.scenarios.find((item) => item.key === selectedKey)?.monthly_contribution_change, "currency", response.currency, language)}</Tag><Tag>{responseHorizonLabel(response, language)}：{formatScenarioValue(response.scenarios.find((item) => item.key === selectedKey)?.horizon_contribution_change, "currency", response.currency, language)}</Tag></Flex></Card><Collapse style={{ marginTop: 16 }} items={[{ key: "evidence", label: t("scenario.evidence_title", language), children: <Space direction="vertical"><Typography.Text>{t("scenario.evidence.facts", language).replace("{from}", response.evidence.current.date_from).replace("{to}", response.evidence.current.date_to).replace("{observed}", String(response.evidence.observed_store_days)).replace("{expected}", String(response.evidence.expected_store_days)).replace("{rate}", response.evidence.coverage_rate == null ? "—" : `${response.evidence.coverage_rate.toFixed(2)}%`)}</Typography.Text><Typography.Text>classification={response.data_classification} · dataset={response.dataset_version || "—"} · source={response.source_system || response.evidence.source_systems.join(", ") || "—"}</Typography.Text><Typography.Text>fact version={response.evidence.fact_version_min}–{response.evidence.fact_version_max} · formula={response.formula_version} · scenario={response.scenario_version}</Typography.Text><Typography.Text>required fields：{response.evidence.required_fields.join(", ")}</Typography.Text><Typography.Text>{t("scenario.evidence.formula", language)}</Typography.Text><a href={response.evidence.kpi_drilldown_url}>{t("scenario.view_drilldown", language)}</a></Space> }]} /><Row gutter={[16, 16]} style={{ marginTop: 16 }}><Col xs={24} lg={14}><Card title={t("scenario.bridge.title", language)}><Table size="small" pagination={false} rowKey="code" dataSource={response.scenarios.find((item) => item.key === selectedKey)?.bridge.items || []} columns={[{ title: t("scenario.bridge.item", language), dataIndex: "label" }, { title: t("scenario.bridge.contribution", language), render: (_: unknown, item: { contribution: number | null; unit: string }) => formatScenarioValue(item.contribution, item.unit, response.currency, language) }]} /><Typography.Text type="secondary">{t("scenario.bridge.total", language)} {formatScenarioValue(response.scenarios.find((item) => item.key === selectedKey)?.bridge.total_change, "currency", response.currency, language)} · {t("scenario.bridge.residual", language)} {formatScenarioValue(response.scenarios.find((item) => item.key === selectedKey)?.bridge.rounding_residual, "currency", response.currency, language)} · {t("scenario.bridge.conservation", language)} {bridgeConservation(response.scenarios.find((item) => item.key === selectedKey)?.bridge || { items: [], total_change: null, rounding_residual: null, status: "unavailable" })}</Typography.Text></Card></Col><Col xs={24} lg={10}><Card title={t("scenario.draft.title", language)}><Space direction="vertical" style={{ width: "100%" }}><Input value={title} onChange={(event) => setTitle(event.target.value)} placeholder={t("scenario.draft.title_placeholder", language)} /><Input.TextArea value={plannedAction} onChange={(event) => setPlannedAction(event.target.value)} placeholder={t("scenario.draft.action_placeholder", language)} rows={3} /><Flex gap={8}><Input value={ownerName} onChange={(event) => setOwnerName(event.target.value)} placeholder={t("scenario.draft.owner_placeholder", language)} /><DatePicker allowClear value={dueDate ? dayjs(dueDate) : null} onChange={(value) => setDueDate(value ? value.format("YYYY-MM-DD") : null)} /></Flex><Input value={verificationPeriod} onChange={(event) => setVerificationPeriod(event.target.value)} placeholder={t("scenario.draft.period_placeholder", language)} /><Button icon={<SaveOutlined />} type="primary" loading={saving} disabled={!canSaveScenario(response, selectedKey, responseKey, currentEvaluationKey) || !title.trim() || !plannedAction.trim()} onClick={saveAction}>{t("scenario.draft.save", language)}</Button>{actionResult && <Alert type="success" message={t("scenario.draft.saved_title", language)} description={<Space direction="vertical"><span>{t("scenario.draft.real_id", language).replace("{id}", String(actionResult.data.id || "—")).replace("{status}", String(actionResult.data.status || "open"))}</span><span>Owner：{String(actionResult.data.owner_name || "—")} · Due：{String(actionResult.data.due_date || "—")}</span><span>{t("scenario.draft.idempotency", language).replace("{value}", actionResult.idempotentReplay ? t("scenario.draft.idempotent_yes", language) : t("scenario.draft.idempotent_no", language))}</span><a href="/performance">{t("scenario.draft.go_workbench", language)}</a></Space>} />}</Space></Card></Col></Row></>}
    <RetailAIDrawer open={aiOpen} onClose={() => setAiOpen(false)} pageContext={{ page: "scenario-workbench", title: t("scenario.title", language), filters: { as_of: query.asOf, window_days: String(query.windowDays), classification: query.classification, dataset_version: query.datasetVersion || "", source_system: query.sourceSystem || "", store_id: query.storeID || "", horizon_months: String(horizon) } }} />
  </div></AppLayout></ProtectedRoute>;
}

export default function ScenarioWorkbenchPage() {
  return <Suspense fallback={<div style={{ minHeight: "100vh", display: "grid", placeItems: "center" }}><Spin /></div>}><ScenarioPageInner /></Suspense>;
}
