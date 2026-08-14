"use client";

import { useEffect, useMemo, useRef, useState, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Alert, Button, Card, Col, Collapse, DatePicker, Empty, Flex, Input, InputNumber, Modal, Radio, Row, Select, Space, Spin, Table, Tag, Typography, message } from "antd";
import { ArrowLeftOutlined, PlayCircleOutlined, SaveOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { apiErrorMessage, retailAnalyticsApi, type RetailDataClassification, type RetailScenarioAssumptions, type RetailScenarioResponse, type RetailStore360Option } from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { latestAnomalyDate } from "../operating-pulse/logic";
import { acceptsEvaluation, actionKey, bridgeConservation, canSaveScenario, defaultAssumptions, evaluationSnapshotKey, formatScenarioValue, responseHorizonLabel, returnScenarioQuery, SCENARIO_CODES, SCENARIO_LABELS } from "./logic";
import { retailAIHref } from "../lib/retailAI";
import { StatusTag } from "../components/StatusTag";

const WINDOW_OPTIONS = [7, 14, 28] as const;
const DELTA_FIELDS: Array<{ key: keyof RetailScenarioAssumptions; label: string; unit: "pct" | "pp" }> = [
  { key: "revenue_change_pct", label: "销售额变化", unit: "pct" },
  { key: "gross_margin_rate_change_pp", label: "毛利率变化", unit: "pp" },
  { key: "labor_cost_change_pct", label: "人工成本变化", unit: "pct" },
  { key: "fixed_rent_change_pct", label: "固定现金租金变化", unit: "pct" },
  { key: "variable_rent_rate_change_pp", label: "变动租金率变化", unit: "pp" },
  { key: "non_lease_cost_change_pct", label: "非租赁占用成本变化", unit: "pct" },
  { key: "other_controllable_cost_change_pct", label: "其他可控成本变化", unit: "pct" },
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

function MetricTable({ response, selectedKey }: { response: RetailScenarioResponse; selectedKey: string }) {
  const selected = response.scenarios.find((item) => item.key === selectedKey);
  if (!selected) return null;
  const rows = SCENARIO_CODES.map((code) => ({ code, baseline: response.baseline.metrics[code], result: selected.metrics[code] }));
  return <Table size="small" pagination={false} rowKey="code" dataSource={rows} columns={[
    { title: "指标", render: (_: unknown, row: typeof rows[number]) => SCENARIO_LABELS[row.code] || row.code },
    { title: "Baseline", render: (_: unknown, row: typeof rows[number]) => formatScenarioValue(row.baseline?.result, row.baseline?.unit || "", response.currency) },
    { title: "Plan", render: (_: unknown, row: typeof rows[number]) => formatScenarioValue(row.result?.result, row.result?.unit || "", response.currency) },
    { title: "变化", render: (_: unknown, row: typeof rows[number]) => formatScenarioValue(row.result?.delta, row.result?.unit || "", response.currency) },
    { title: "状态", render: (_: unknown, row: typeof rows[number]) => <StatusTag kind={row.result?.status === "complete" ? "success" : "warning"}>{row.result?.status || "unavailable"}{row.result?.reason ? ` · ${row.result.reason}` : ""}</StatusTag> },
  ]} />;
}

function ScenarioPageInner() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { token } = useAuth();
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
    if (!token || !query.storeID) { setError("请先选择授权门店。"); return; }
    if (query.classification === "simulated" && !query.datasetVersion) { setError("模拟数据必须明确 dataset version。"); return; }
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
    Modal.confirm({ title: "保存行动草稿？", content: "服务端会重新读取 Working 事实并只写入一条 open 行动草稿。", onOk: async () => {
      setSaving(true);
      try { const result = await retailAnalyticsApi.saveStoreScenarioAction(currentScope, body, key, token); setActionResult({ data: result.data, idempotentReplay: result.idempotent_replay }); message.success(result.idempotent_replay ? "已安全重放现有草稿" : "行动草稿已保存"); } catch (err) { message.error(apiErrorMessage(err)); } finally { setSaving(false); }
    } });
  };

  const backURL = query.returnQuery ? `/store-360?${query.returnQuery}` : returnScenarioQuery(searchParams);

  return <ProtectedRoute><AppLayout><div className="scenario-workbench-page">
    <PageHeader title="情景工作台" subtitle="门店经营 What-if；服务端基于同一 Working 事实重算 30-day run-rate，不输出最优方案或 IFRS 16 影响。" primaryAction={<Button type="primary" icon={<PlayCircleOutlined />} loading={loading} onClick={evaluate}>计算情景</Button>} secondaryAction={<Space><Button onClick={() => router.push(retailAIHref({ page: "scenario-workbench", title: "情景工作台", asOf: query.asOf, windowDays: query.windowDays, classification: query.classification, datasetVersion: query.datasetVersion || undefined, sourceSystem: query.sourceSystem, storeID: query.storeID || undefined, horizonMonths: horizon, assumptions }))}>交给 AI 分析</Button><Button icon={<ArrowLeftOutlined />} onClick={() => router.push(backURL)}>返回门店 360</Button></Space>} />
    <Alert type={query.classification === "production" ? "info" : "warning"} showIcon message={<Flex wrap="wrap" gap={12}><StatusTag kind={query.classification === "production" ? "processing" : "warning"}>{query.classification === "production" ? "正式数据 · Working" : "模拟数据 · Working"}</StatusTag><span>dataset: {query.classification === "simulated" ? query.datasetVersion || "—" : "—"}</span><span>basis: Scenario</span><span>formula: retail-store-scenario-v1 / retail-kpi-v1</span><span>30-day run-rate</span><span>不进入 Official / IFRS 16</span></Flex>} description={latestMatches ? `generator: ${latestMatches.generator_version} · latest anomaly: ${latestAnomalyDate(latestMatches)}` : query.classification === "production" ? "正式数据不会显示模拟 generator；当前仅读 Working 事实。" : "当前 URL 数据集没有匹配的 latest 元数据，系统不会伪造 generator。"} />
    <Card size="small" style={{ marginTop: 16 }}><Flex gap={12} wrap="wrap" align="center"><Radio.Group value={query.classification} optionType="button" buttonStyle="solid" options={[{ label: "模拟数据", value: "simulated" }, { label: "正式数据", value: "production" }]} onChange={(event) => { const next = event.target.value as RetailDataClassification; if (next === "production") setQuery({ classification: next, datasetVersion: "", storeID: "" }); else setQuery({ classification: next, datasetVersion: latest?.dataset_version || "", asOf: latest ? latestAnomalyDate(latest) : query.asOf, storeID: "" }); }} /><Select showSearch allowClear value={query.storeID || undefined} placeholder="选择授权门店" loading={loadingOptions} style={{ minWidth: 260 }} options={options.map((item) => ({ label: `${item.store_code} · ${item.store_name}`, value: item.store_id, search: `${item.store_code} ${item.store_name} ${item.brand} ${item.region}` }))} optionFilterProp="search" onChange={(value) => setQuery({ storeID: value || "" })} /><DatePicker value={dayjs(query.asOf)} onChange={(value) => value && setQuery({ asOf: value.format("YYYY-MM-DD") })} /><Select value={query.windowDays} options={WINDOW_OPTIONS.map((value) => ({ label: `${value}天`, value }))} onChange={(value) => setQuery({ windowDays: value })} /><Input value={query.sourceSystem} onChange={(event) => setQuery({ sourceSystem: event.target.value })} placeholder="source_system（可选）" style={{ width: 190 }} /></Flex></Card>
    {!query.storeID && <Card style={{ marginTop: 16 }}><Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="请选择授权门店后计算情景；本页不会自动生成模拟数据。" /></Card>}
    {query.classification === "simulated" && !query.datasetVersion && <Alert style={{ marginTop: 16 }} type="warning" message="模拟 dataset version 缺失" description="请从经营脉搏选择固定模拟数据集；页面不会把 production 或旧版本伪装成 latest。" action={<Button size="small" onClick={() => router.push("/operating-pulse")}>前往经营脉搏</Button>} />}
    {optionsError && <Alert style={{ marginTop: 16 }} type="error" showIcon message="授权门店加载失败" description={optionsError} action={<Button size="small" onClick={() => setOptionsRetry((value) => value + 1)}>重试</Button>} />}
    {query.classification && !loadingOptions && !optionsError && options.length === 0 && <Alert style={{ marginTop: 16 }} type="info" showIcon message="当前范围没有授权门店" description="请检查法人、classification、dataset 或数据权限；系统不会自动选择或补造门店。" />}
    {response && !freshResponse && <Alert style={{ marginTop: 16 }} type="warning" showIcon message="结果已过期，不能保存" description="门店、日期窗口、来源、预测期或七项情景假设已变化。请重新计算后再生成行动草稿。" action={<Button size="small" onClick={evaluate}>重新计算</Button>} />}
    {selectedStore && <Card title={<Flex justify="space-between"><span>{selectedStore.store_code} · {selectedStore.store_name}</span><Typography.Text type="secondary">{selectedStore.brand} · {selectedStore.region}</Typography.Text></Flex>} style={{ marginTop: 16 }}><Row gutter={[12, 12]}>{DELTA_FIELDS.map((field) => <Col xs={24} sm={12} lg={8} key={field.key}><Typography.Text>{field.label}（{field.unit === "pp" ? "pp" : "%"}）</Typography.Text><InputNumber style={{ width: "100%" }} value={assumptions[field.key]} onChange={(value) => setAssumptions((current) => ({ ...current, [field.key]: value ?? 0 }))} min={field.unit === "pp" ? -100 : -100} max={field.unit === "pp" ? 100 : 300} /></Col>)}</Row><Flex gap={12} wrap="wrap" align="center" style={{ marginTop: 16 }}><span>预测期</span><Select value={horizon} options={[3, 6, 12].map((value) => ({ value, label: `${value}个月` }))} onChange={setHorizon} /><Typography.Text type="secondary">Baseline 为固定零 delta；Plan 的七类变化会由服务端重新计算。</Typography.Text></Flex></Card>}
    {error && <Alert style={{ marginTop: 16 }} type="error" showIcon message="情景不可用" description={error} />}
    {loading && <Card style={{ marginTop: 16 }}><Flex justify="center" style={{ minHeight: 160 }}><Spin tip="服务端读取事实并计算 30-day run-rate…" /></Flex></Card>}
    {response && <><Card style={{ marginTop: 16 }} title={<Flex justify="space-between"><span>Baseline / {response.scenarios.find((item) => item.key === selectedKey)?.name || "Plan"}</span><StatusTag kind={freshResponse ? "success" : "warning"}>{freshResponse ? "review_required · Working" : "结果已过期"}</StatusTag></Flex>}><MetricTable response={response} selectedKey={selectedKey} /><Flex gap={8} wrap="wrap" style={{ marginTop: 12 }}><Tag>月度贡献变化：{formatScenarioValue(response.scenarios.find((item) => item.key === selectedKey)?.monthly_contribution_change, "currency", response.currency)}</Tag><Tag>{responseHorizonLabel(response)}：{formatScenarioValue(response.scenarios.find((item) => item.key === selectedKey)?.horizon_contribution_change, "currency", response.currency)}</Tag></Flex></Card><Collapse style={{ marginTop: 16 }} items={[{ key: "evidence", label: "证据与公式", children: <Space direction="vertical"><Typography.Text>当前事实：{response.evidence.current.date_from}–{response.evidence.current.date_to} · 覆盖 {response.evidence.observed_store_days}/{response.evidence.expected_store_days}（{response.evidence.coverage_rate == null ? "—" : `${response.evidence.coverage_rate.toFixed(2)}%`}）</Typography.Text><Typography.Text>classification={response.data_classification} · dataset={response.dataset_version || "—"} · source={response.source_system || response.evidence.source_systems.join(", ") || "—"}</Typography.Text><Typography.Text>fact version={response.evidence.fact_version_min}–{response.evidence.fact_version_max} · formula={response.formula_version} · scenario={response.scenario_version}</Typography.Text><Typography.Text>required fields：{response.evidence.required_fields.join(", ")}</Typography.Text><Typography.Text>公式：30-day run-rate = 30 ÷ observed store-days；贡献额 = 毛利额 − 人工 − 经营占用现金成本 − 其他可控成本。经营占用现金成本不等同 IFRS 16 会计费用。</Typography.Text><a href={response.evidence.kpi_drilldown_url}>查看 KPI 事实下钻</a></Space> }]} /><Row gutter={[16, 16]} style={{ marginTop: 16 }}><Col xs={24} lg={14}><Card title="变化贡献桥（守恒，非根因）"><Table size="small" pagination={false} rowKey="code" dataSource={response.scenarios.find((item) => item.key === selectedKey)?.bridge.items || []} columns={[{ title: "变化项", dataIndex: "label" }, { title: "贡献", render: (_: unknown, item: { contribution: number | null; unit: string }) => formatScenarioValue(item.contribution, item.unit, response.currency) }]} /><Typography.Text type="secondary">总变化 {formatScenarioValue(response.scenarios.find((item) => item.key === selectedKey)?.bridge.total_change, "currency", response.currency)} · 残差 {formatScenarioValue(response.scenarios.find((item) => item.key === selectedKey)?.bridge.rounding_residual, "currency", response.currency)} · 守恒误差 {bridgeConservation(response.scenarios.find((item) => item.key === selectedKey)?.bridge || { items: [], total_change: null, rounding_residual: null, status: "unavailable" })}</Typography.Text></Card></Col><Col xs={24} lg={10}><Card title="行动草稿（只写 open）"><Space direction="vertical" style={{ width: "100%" }}><Input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="标题" /><Input.TextArea value={plannedAction} onChange={(event) => setPlannedAction(event.target.value)} placeholder="计划动作（需复核验证）" rows={3} /><Flex gap={8}><Input value={ownerName} onChange={(event) => setOwnerName(event.target.value)} placeholder="Owner（可空）" /><DatePicker allowClear value={dueDate ? dayjs(dueDate) : null} onChange={(value) => setDueDate(value ? value.format("YYYY-MM-DD") : null)} /></Flex><Input value={verificationPeriod} onChange={(event) => setVerificationPeriod(event.target.value)} placeholder="验证期间 YYYY-MM" /><Button icon={<SaveOutlined />} type="primary" loading={saving} disabled={!canSaveScenario(response, selectedKey, responseKey, currentEvaluationKey) || !title.trim() || !plannedAction.trim()} onClick={saveAction}>保存行动草稿</Button>{actionResult && <Alert type="success" message="行动草稿已安全保存/重放" description={<Space direction="vertical"><span>真实 ID：{String(actionResult.data.id || "—")} · status={String(actionResult.data.status || "open")}</span><span>Owner：{String(actionResult.data.owner_name || "—")} · Due：{String(actionResult.data.due_date || "—")}</span><span>Idempotency replay：{actionResult.idempotentReplay ? "是" : "否"}</span><a href="/performance">前往经营工作台查看</a></Space>} />}</Space></Card></Col></Row></>}
  </div></AppLayout></ProtectedRoute>;
}

export default function ScenarioWorkbenchPage() {
  return <Suspense fallback={<div style={{ minHeight: "100vh", display: "grid", placeItems: "center" }}><Spin /></div>}><ScenarioPageInner /></Suspense>;
}
