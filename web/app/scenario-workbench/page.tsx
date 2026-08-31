"use client";

import { useEffect, useMemo, useRef, useState, Suspense } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Alert, Button, Card, Checkbox, Col, Collapse, DatePicker, Empty, Flex, Input, InputNumber, Modal, Radio, Result, Row, Segmented, Select, Slider, Space, Spin, Table, Typography, message } from "antd";
import { ArrowLeftOutlined, CheckCircleFilled, ControlOutlined, PlayCircleOutlined, SaveOutlined, ThunderboltOutlined } from "@ant-design/icons";
import dayjs from "dayjs";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import { HelpTrigger } from "../components/HelpDrawer";
import { scenarioHelpContent } from "../components/help-content";
import { StateBlock } from "../components/StateBlock";
import { useRetailQuery } from "../retail/useRetailQuery";
import DataTrustBar, { KPIReadyBadge } from "../components/DataTrustBar";
import RetailAIDrawer from "../components/RetailAIDrawer";
import ProtectedRoute from "../components/ProtectedRoute";
import { SparkleGlyph, SlidersGlyph } from "../components/MonochromeGlyphs";
import WaterfallChart from "../components/charts/WaterfallChart";
import { apiErrorMessage, performanceApi, retailAnalyticsApi, type RetailDataClassification, type RetailScenarioAssumptions, type RetailScenarioResponse, type RetailStore360Option } from "../lib/api";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t, type Language } from "../lib/i18n";
import { latestAnomalyDate } from "../operating-pulse/logic";
import { RetailExportMenu } from "../components/RetailExportMenu";
import { envelopeFromScenario, scenarioRowsFromResponse } from "../lib/retail-export";
import { acceptsEvaluation, actionKey, bridgeConservation, canSaveScenario, defaultAssumptions, evaluationSnapshotKey, formatScenarioValue, responseHorizonLabel, returnScenarioQuery, SCENARIO_CODES, scenarioLabel } from "./logic";
import { StatusTag } from "../components/StatusTag";

const TODAY = dayjs().format("YYYY-MM-DD");
const WINDOW_OPTIONS = [7, 14, 28] as const;
const DELTA_FIELDS: Array<{ key: keyof RetailScenarioAssumptions; labelKey: string; unit: "pct" | "pp"; min: number; max: number; step: number }> = [
  { key: "fixed_rent_change_pct", labelKey: "scenario.delta.fixed_rent_change_pct", unit: "pct", min: -50, max: 50, step: 1 },
  { key: "variable_rent_rate_change_pp", labelKey: "scenario.delta.variable_rent_rate_change_pp", unit: "pp", min: -10, max: 10, step: 0.5 },
  { key: "revenue_change_pct", labelKey: "scenario.delta.revenue_change_pct", unit: "pct", min: -50, max: 50, step: 1 },
  { key: "gross_margin_rate_change_pp", labelKey: "scenario.delta.gross_margin_rate_change_pp", unit: "pp", min: -20, max: 20, step: 0.5 },
  { key: "labor_cost_change_pct", labelKey: "scenario.delta.labor_cost_change_pct", unit: "pct", min: -50, max: 50, step: 1 },
  { key: "non_lease_cost_change_pct", labelKey: "scenario.delta.non_lease_cost_change_pct", unit: "pct", min: -50, max: 50, step: 1 },
  { key: "other_controllable_cost_change_pct", labelKey: "scenario.delta.other_controllable_cost_change_pct", unit: "pct", min: -50, max: 50, step: 1 },
];

function parseURL(params: { get(name: string): string | null }) {
  const classification = params.get("data_classification");
  const validClassification: RetailDataClassification = classification === "production" ? "production" : "simulated";
  const rawWindow = Number(params.get("window_days") || 14);
  const windowDays = Number.isInteger(rawWindow) && (rawWindow === 7 || rawWindow === 14 || rawWindow === 28) ? rawWindow : 14;
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
  return (
    <Table
      size="small"
      pagination={false}
      rowKey="code"
      dataSource={rows}
      columns={[
        { title: t("scenario.col.metric", language), render: (_: unknown, row: typeof rows[number]) => scenarioLabel(row.code, language) || row.code },
        { title: t("scenario.col.baseline", language), render: (_: unknown, row: typeof rows[number]) => <span className="font-tabular">{formatScenarioValue(row.baseline?.result, row.baseline?.unit || "", response.currency, language)}</span> },
        { title: t("scenario.col.plan", language), render: (_: unknown, row: typeof rows[number]) => <span className="font-tabular">{formatScenarioValue(row.result?.result, row.result?.unit || "", response.currency, language)}</span> },
        {
          title: t("scenario.col.change", language),
          render: (_: unknown, row: typeof rows[number]) => {
            const delta = row.result?.delta;
            const isPos = delta != null && delta > 0;
            const isNeg = delta != null && delta < 0;
            const colorClass = isPos ? "color-pos" : isNeg ? "color-neg" : "";
            return (
              <span className={`font-tabular ${colorClass}`} style={{ color: isPos ? "var(--state-success-text)" : isNeg ? "var(--state-error-text)" : undefined }}>
                {formatScenarioValue(delta, row.result?.unit || "", response.currency, language)}
              </span>
            );
          },
        },
        {
          title: t("scenario.col.status", language),
          render: (_: unknown, row: typeof rows[number]) => (
            <Flex align="center" gap={4}>
              {notReady && <KPIReadyBadge />}
              {row.result?.status === "complete" ? (
                <CheckCircleFilled style={{ color: "#166534", fontSize: 13 }} />
              ) : (
                <span style={{ fontSize: 11, color: "var(--state-warning-text)" }}>
                  {row.result?.status || "unavailable"}{row.result?.reason ? ` · ${row.result.reason}` : ""}
                </span>
              )}
            </Flex>
          ),
        },
      ]}
    />
  );
}

function ScenarioPageInner() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { token } = useAuth();
  const { language } = useLanguage();
  const [aiOpen, setAiOpen] = useState(false);
  const query = useMemo(() => parseURL(searchParams), [searchParams]);
  const [windowInput, setWindowInput] = useState(query.windowDays);
  const [latest, setLatest] = useState<import("../lib/api").RetailSimulationDatasetData | null | undefined>(undefined);
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

  const optionParams = query.classification !== "simulated" || query.datasetVersion
    ? { data_classification: query.classification, dataset_version: query.datasetVersion || undefined }
    : null;
  const { loading: optionsLoading, state: optionsState, retry: optionsRetry } = useRetailQuery({
    token,
    params: optionParams,
    paramsKey: `options:${query.classification}|${query.datasetVersion}`,
    fetcher: (p, t) => retailAnalyticsApi.storeOptions(p, t).then((res) => res.data),
  });
  const options: RetailStore360Option[] = optionsState.kind === "ready" ? optionsState.data ?? [] : [];

  useEffect(() => {
    if (!query.storeID && options.length > 0) {
      setQuery({ storeID: options[0].store_id });
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps -- P2-C gate close-out: legacy dep semantics kept as-is; loaders are rebuilt every render so adding them would loop refetches. useCallback refactor tracked separately; do not add new exemptions.
  }, [query.storeID, options]);

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
    const assumptionsToWrite = next.assumptions ?? assumptions;
    for (const [key, value] of Object.entries(assumptionsToWrite)) q.set(key, String(value));
    q.set("horizon_months", String(next.horizon ?? horizon));
    if (query.returnQuery) q.set("return_query", query.returnQuery);
    router.replace(`/scenario-workbench?${q.toString()}`);
  };

  useEffect(() => {
    setWindowInput(query.windowDays);
  }, [query.windowDays]);

  const applyCustomWindow = () => {
    const next = Math.round(windowInput);
    if (next >= 7 && next <= 28) setQuery({ windowDays: next });
  };

  useEffect(() => {
    if (query.classification !== "simulated" || query.datasetVersion || !latest) return;
    setQuery({ datasetVersion: latest.dataset_version, asOf: latestAnomalyDate(latest) });
  // eslint-disable-next-line react-hooks/exhaustive-deps -- P2-C gate close-out: legacy dep semantics kept as-is; loaders are rebuilt every render so adding them would loop refetches. useCallback refactor tracked separately; do not add new exemptions.
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

  useEffect(() => {
    if (token && query.storeID && (!query.classification || query.classification !== "simulated" || query.datasetVersion)) {
      void evaluate();
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps -- P2-C gate close-out: legacy dep semantics kept as-is; loaders are rebuilt every render so adding them would loop refetches. useCallback refactor tracked separately; do not add new exemptions.
  }, [query.storeID, query.classification, query.datasetVersion, query.asOf, query.windowDays, horizon]);

  const applyPreset = (preset: { fixedRent?: number; variableRent?: number; revenue?: number }) => {
    const next = {
      ...assumptions,
      ...(preset.fixedRent !== undefined ? { fixed_rent_change_pct: preset.fixedRent } : {}),
      ...(preset.variableRent !== undefined ? { variable_rent_rate_change_pp: preset.variableRent } : {}),
      ...(preset.revenue !== undefined ? { revenue_change_pct: preset.revenue } : {}),
    };
    setAssumptions(next);
    setQuery({ assumptions: next });
  };

  const saveAction = async () => {
    if (!token || !freshResponse || selectedKey === "baseline" || !title.trim() || !plannedAction.trim()) return;
    const selected = freshResponse.scenarios.find((item) => item.key === selectedKey);
    if (!selected) return;
    const body = { horizon_months: horizon, selected_scenario: { key: selected.key, name: selected.name, assumptions: selected.assumptions }, title: title.trim(), planned_action: plannedAction.trim(), owner_name: ownerName.trim(), due_date: dueDate, verification_period: verificationPeriod };
    const key = actionKey({ evaluation: currentEvaluationKey, body });
    Modal.confirm({
      title: t("scenario.confirm_save_title", language),
      content: t("scenario.confirm_save_content", language),
      onOk: async () => {
        setSaving(true);
        try {
          const result = await retailAnalyticsApi.saveStoreScenarioAction(currentScope, body, key, token);
          setActionResult({ data: result.data, idempotentReplay: result.idempotent_replay });
          message.success(result.idempotent_replay ? t("scenario.saved_replay", language) : t("scenario.saved", language));
        } catch (err) {
          message.error(apiErrorMessage(err));
        } finally {
          setSaving(false);
        }
      },
    });
  };

  // Scenario decision -> lease event draft (audit §E-2 last-mile wiring; same
  // break-point family as feedback §7.2 P0-2 "scenario output cannot land in
  // actions"). This is the only bridge from a commercial decision into lease
  // accounting: it writes the draft layer only (formal_event_created stays
  // false); approval and recalculation remain separate workflow steps. The
  // backend requires approved=true — the human confirms it via an explicit
  // checkbox, the UI never approves on their behalf.
  const [eventDraftOpen, setEventDraftOpen] = useState(false);
  const [eventDraftContract, setEventDraftContract] = useState("");
  const [eventDraftType, setEventDraftType] = useState("lease_modification");
  const [eventDraftDate, setEventDraftDate] = useState<string | null>(null);
  const [eventDraftReason, setEventDraftReason] = useState("");
  const [eventDraftDecision, setEventDraftDecision] = useState("");
  const [eventDraftOriginal, setEventDraftOriginal] = useState("");
  const [eventDraftNew, setEventDraftNew] = useState("");
  const [eventDraftApproved, setEventDraftApproved] = useState(false);
  const [eventDraftSubmitting, setEventDraftSubmitting] = useState(false);
  const [eventDraftResult, setEventDraftResult] = useState<{ status: string; formalCreated: boolean } | null>(null);
  const eventDraftIdempotencyKey = useRef<string>("");

  const openEventDraftModal = () => {
    if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
      eventDraftIdempotencyKey.current = crypto.randomUUID();
    } else {
      eventDraftIdempotencyKey.current = `scenario-event-${Date.now()}-${Math.random().toString(16).slice(2)}`;
    }
    setEventDraftDecision(title.trim());
    setEventDraftResult(null);
    setEventDraftOpen(true);
  };

  const evidenceRef = () => ({
    page: "scenario-workbench",
    store_id: query.storeID,
    data_classification: query.classification,
    dataset_version: query.datasetVersion || undefined,
    as_of: query.asOf,
    window_days: query.windowDays,
    horizon_months: horizon,
    scenario_key: selectedKey,
    assumptions,
    evaluation_snapshot: currentEvaluationKey,
    ...(actionResult?.data?.id ? { saved_action_id: actionResult.data.id } : {}),
  });

  const submitEventDraft = async () => {
    if (!token || !freshResponse) return;
    if (!eventDraftContract.trim()) { message.warning(t("scenario.event.err_contract", language)); return; }
    if (!eventDraftDate) { message.warning(t("scenario.event.err_date", language)); return; }
    if (!eventDraftReason.trim()) { message.warning(t("scenario.event.err_reason", language)); return; }
    if (!eventDraftApproved) { message.warning(t("scenario.event.err_approved", language)); return; }
    setEventDraftSubmitting(true);
    try {
      const result = (await performanceApi.storeDecisionEventDraft(
        {
          contract_id: eventDraftContract.trim(),
          approved: eventDraftApproved,
          event_type: eventDraftType,
          effective_date: eventDraftDate,
          change_reason: eventDraftReason.trim(),
          decision: eventDraftDecision.trim() || t("scenario.plan_title", language),
          scenario_name: freshResponse.scenarios.find((item) => item.key === selectedKey)?.name || selectedKey,
          original_value: eventDraftOriginal.trim() || undefined,
          new_value: eventDraftNew.trim() || undefined,
          evidence_ref: evidenceRef(),
        },
        token,
        eventDraftIdempotencyKey.current,
      )) as { data?: { status?: string }; formal_event_created?: boolean };
      setEventDraftResult({ status: String(result.data?.status || "pending"), formalCreated: Boolean(result.formal_event_created) });
      message.success(t("scenario.event.success", language));
    } catch (err) {
      message.error(apiErrorMessage(err));
    } finally {
      setEventDraftSubmitting(false);
    }
  };

  const backURL = query.returnQuery ? `/store-360?${query.returnQuery}` : returnScenarioQuery(searchParams);
  const waterfallItems = useMemo(() => {
    if (!response) return [];
    const selected = response.scenarios.find((item) => item.key === selectedKey);
    if (!selected || !selected.bridge || !selected.bridge.items) return [];
    const baseVal = response.baseline?.metrics?.store_contribution?.result ?? 0;
    const allItems = selected.bridge.items.map((item) => ({
      label: item.label,
      value: item.contribution ?? 0,
    }));
    const activeItems = allItems.filter((item) => Math.abs(item.value) >= 0.01);
    const items = activeItems.length > 0 ? activeItems : allItems;
    const planVal = selected.metrics?.store_contribution?.result ?? (baseVal + (selected.monthly_contribution_change || 0));
    return [
      { label: `${t("scenario.option_baseline", language)} (${t("retail.kpi.store_contribution", language)})`, value: baseVal, isTotal: true },
      ...items,
      { label: `${t("scenario.plan_title", language)} (${t("retail.kpi.store_contribution", language)})`, value: planVal, isTotal: true },
    ];
  }, [response, selectedKey, language]);

  return (
    <ProtectedRoute>
      <AppLayout>
        <div className="scenario-workbench-page">
          <PageHeader
            title={t("scenario.title", language)}
            meta={t("scenario.scope_note", language)}
            help={<HelpTrigger content={scenarioHelpContent(language)} language={language} />}
            primaryAction={
              <Button type="primary" icon={<PlayCircleOutlined />} loading={loading} onClick={evaluate}>
                {t("scenario.calculate", language)}
              </Button>
            }
            secondaryAction={
              <Space>
                <RetailExportMenu
                  kind="scenario"
                  disabled={!response}
                  envelope={response ? envelopeFromScenario(response) : null}
                  rows={() => (response ? scenarioRowsFromResponse(response, selectedKey) : [])}
                />
                <Button icon={<SparkleGlyph size={13} />} onClick={() => setAiOpen(true)}>
                  {t("common.ai_analysis", language)}
                </Button>
                <Button icon={<ArrowLeftOutlined />} onClick={() => router.push(backURL)}>
                  {t("scenario.back_store360", language)}
                </Button>
              </Space>
            }
          />
          {response && (
            <DataTrustBar
              envelope={response.envelope}
              basis="Scenario"
              detailExtra={latestMatches ? <span>generator: {latestMatches.generator_version} · anomaly: {latestAnomalyDate(latestMatches)}</span> : undefined}
            />
          )}

          <Row gutter={[16, 16]} style={{ marginTop: 12 }}>
            <Col xs={24} lg={10}>
              <Space direction="vertical" style={{ width: "100%" }} size={12}>
                <Card size="small" title={<Space><ControlOutlined /><span>{t("scenario.controls", language)}</span></Space>}>
                  <Space direction="vertical" style={{ width: "100%" }} size={10}>
                    <Segmented
                      className="precision-segmented"
                      value={query.classification}
                      onChange={(val) => {
                        const next = val as RetailDataClassification;
                        if (next === "production") setQuery({ classification: next, datasetVersion: "", asOf: TODAY });
                        else if (latest) setQuery({ classification: next, datasetVersion: latest.dataset_version, asOf: latestAnomalyDate(latest) });
                        else setQuery({ classification: next, datasetVersion: "" });
                      }}
                      options={[
                        { label: t("retail.classification.simulated", language), value: "simulated" },
                        { label: t("retail.classification.production", language), value: "production" },
                      ]}
                    />
                    <Select
                      showSearch
                      allowClear
                      value={query.storeID || undefined}
                      placeholder={t("store360.select_store", language)}
                      loading={optionsLoading}
                      style={{ width: "100%" }}
                      options={options.map((item) => ({
                        label: `${item.store_code} · ${item.store_name} (${item.brand || "—"})`,
                        value: item.store_id,
                        search: `${item.store_code} ${item.store_name} ${item.brand} ${item.region}`,
                      }))}
                      optionFilterProp="search"
                      onChange={(value) => setQuery({ storeID: value || "" })}
                    />
                    <Flex gap={8}>
                      <DatePicker
                        style={{ width: "50%" }}
                        value={dayjs(query.asOf)}
                        onChange={(value) => value && setQuery({ asOf: value.format("YYYY-MM-DD") })}
                      />
                      <Select
                        style={{ width: "50%" }}
                        value={WINDOW_OPTIONS.includes(query.windowDays as (typeof WINDOW_OPTIONS)[number]) ? query.windowDays : undefined}
                        placeholder={t("pulse.custom_window", language)}
                        options={WINDOW_OPTIONS.map((val) => ({
                          label: t(`period.day_${val}`, language),
                          value: val,
                        }))}
                        onChange={(value) => setQuery({ windowDays: value })}
                      />
                    </Flex>
                  </Space>
                </Card>

                <Card size="small" title={<Space><ThunderboltOutlined /><span>{t("scenario.preset_title", language)}</span></Space>}>
                  <Flex wrap="wrap" gap={8}>
                    <Button size="small" onClick={() => applyPreset({ fixedRent: -5 })}>{t("scenario.preset_rent_5", language)}</Button>
                    <Button size="small" onClick={() => applyPreset({ fixedRent: -10 })}>{t("scenario.preset_rent_10", language)}</Button>
                    <Button size="small" onClick={() => applyPreset({ fixedRent: -15 })}>{t("scenario.preset_rent_15", language)}</Button>
                    <Button size="small" onClick={() => applyPreset({ fixedRent: -20, variableRent: 2 })}>{t("scenario.preset_rent_20_var_2", language)}</Button>
                    <Button size="small" onClick={() => applyPreset({ revenue: 5 })}>{t("scenario.preset_sales_5", language)}</Button>
                    <Button size="small" onClick={() => applyPreset({ fixedRent: 0, variableRent: 0, revenue: 0 })}>{t("scenario.preset_reset", language)}</Button>
                  </Flex>
                </Card>

                <Card
                  size="small"
                  title={
                    <Flex justify="space-between" align="center">
                      <span>{t("scenario.assumptions", language)}</span>
                      <Select
                        size="small"
                        value={horizon}
                        options={[3, 6, 12].map((value) => ({ value, label: t("scenario.months_option", language).replace("{n}", String(value)) }))}
                        onChange={(value) => { setHorizon(value); setQuery({ horizon: value }); }}
                      />
                    </Flex>
                  }
                >
                  <Space direction="vertical" style={{ width: "100%" }} size={12}>
                    {DELTA_FIELDS.map((field) => (
                      <div key={field.key} style={{ padding: "4px 0", borderBottom: "1px dashed var(--border-subtle)" }}>
                        <Flex justify="space-between" align="center">
                          <Typography.Text style={{ fontSize: 13, fontWeight: 500 }}>
                            {t(field.labelKey, language)}
                          </Typography.Text>
                          <Space size={4}>
                            <InputNumber
                              size="small"
                              style={{ width: 80 }}
                              value={assumptions[field.key]}
                              min={field.min}
                              max={field.max}
                              step={field.step}
                              onChange={(value) => {
                                const next = { ...assumptions, [field.key]: value ?? 0 };
                                setAssumptions(next);
                                setQuery({ assumptions: next });
                              }}
                            />
                            <Typography.Text type="secondary" style={{ fontSize: 11 }}>
                              {field.unit === "pp" ? "pp" : "%"}
                            </Typography.Text>
                          </Space>
                        </Flex>
                        <Slider
                          min={field.min}
                          max={field.max}
                          step={field.step}
                          value={assumptions[field.key]}
                          onChange={(value) => {
                            const next = { ...assumptions, [field.key]: value };
                            setAssumptions(next);
                            setQuery({ assumptions: next });
                          }}
                        />
                      </div>
                    ))}
                  </Space>
                </Card>
              </Space>
            </Col>

            <Col xs={24} lg={14}>
              <Space direction="vertical" style={{ width: "100%" }} size={12}>
                {error && (
                  <Card
                    style={{
                      border: "1px solid var(--border-default)",
                      background: "var(--bg-elevated)",
                      borderRadius: 12,
                    }}
                  >
                    <Result
                      status="warning"
                      title={query.classification === "production" ? t("scenario.no_production_facts", language) : t("scenario.unavailable", language)}
                      subTitle={
                        query.classification === "production"
                          ? t("scenario.no_production_facts_desc", language)
                          : error
                      }
                      extra={
                        query.classification === "production" ? (
                          <Button
                            type="primary"
                            onClick={() => {
                              if (latest) {
                                setQuery({
                                  classification: "simulated",
                                  datasetVersion: latest.dataset_version,
                                  asOf: latestAnomalyDate(latest),
                                });
                              } else {
                                setQuery({ classification: "simulated" });
                              }
                            }}
                            style={{
                              background: "var(--fg-primary)",
                              borderColor: "var(--fg-primary)",
                              borderRadius: 6,
                            }}
                          >
                            {t("scenario.switch_to_simulated", language)}
                          </Button>
                        ) : undefined
                      }
                    />
                  </Card>
                )}
                {loading && (
                  <Card>
                    <Flex justify="center" align="center" style={{ minHeight: 200 }}>
                      <Spin tip={t("scenario.loading", language)} />
                    </Flex>
                  </Card>
                )}

                {response && (
                  <>
                    <Row gutter={[12, 12]}>
                      <Col xs={24} sm={12}>
                        <Card size="small">
                          <Typography.Text type="secondary" style={{ fontSize: 12 }}>{t("scenario.table.monthly_diff", language)}</Typography.Text>
                          <Typography.Title level={3} className="font-tabular" style={{ margin: "4px 0 0", color: (response.scenarios.find((s) => s.key === selectedKey)?.monthly_contribution_change || 0) >= 0 ? "var(--state-success-text)" : "var(--state-error-text)" }}>
                            {formatScenarioValue(response.scenarios.find((s) => s.key === selectedKey)?.monthly_contribution_change, "currency", response.currency, language)}
                          </Typography.Title>
                        </Card>
                      </Col>
                      <Col xs={24} sm={12}>
                        <Card size="small">
                          <Typography.Text type="secondary" style={{ fontSize: 12 }}>{responseHorizonLabel(response, language)} {t("scenario.table.horizon_diff", language)}</Typography.Text>
                          <Typography.Title level={3} className="font-tabular" style={{ margin: "4px 0 0", color: (response.scenarios.find((s) => s.key === selectedKey)?.horizon_contribution_change || 0) >= 0 ? "var(--state-success-text)" : "var(--state-error-text)" }}>
                            {formatScenarioValue(response.scenarios.find((s) => s.key === selectedKey)?.horizon_contribution_change, "currency", response.currency, language)}
                          </Typography.Title>
                        </Card>
                      </Col>
                    </Row>

                    <Card size="small" title={t("scenario.bridge_title", language)}>
                      <WaterfallChart
                        items={waterfallItems}
                        currency={response.currency}
                        height={240}
                      />
                    </Card>

                    <Card
                      size="small"
                      title={
                        <Flex justify="space-between" align="center">
                          <span>{t("scenario.title", language)}</span>
                          {response.scenarios.length > 1 ? (
                            <Radio.Group
                              size="small"
                              optionType="button"
                              buttonStyle="solid"
                              value={selectedKey}
                              onChange={(event) => setSelectedKey(event.target.value)}
                              options={response.scenarios.map((item) => ({ label: item.name || item.key, value: item.key }))}
                            />
                          ) : (
                            <StatusTag kind="processing">
                              {response.scenarios[0]?.name === "Plan" ? t("scenario.plan_title", language) : (response.scenarios[0]?.name || t("scenario.plan_title", language))}
                            </StatusTag>
                          )}
                        </Flex>
                      }
                    >
                      <MetricTable response={response} selectedKey={selectedKey} notReady={!response.envelope?.decision_ready} language={language} />
                    </Card>

                    <Card size="small" title={t("scenario.draft.title", language)}>
                      <Space direction="vertical" style={{ width: "100%" }} size={10}>
                        <Input
                          value={title}
                          onChange={(event) => setTitle(event.target.value)}
                          placeholder={t("scenario.draft.title_placeholder", language)}
                        />
                        <Input.TextArea
                          value={plannedAction}
                          onChange={(event) => setPlannedAction(event.target.value)}
                          placeholder={t("scenario.draft.action_placeholder", language)}
                          rows={2}
                        />
                        <Flex gap={8}>
                          <Input
                            value={ownerName}
                            onChange={(event) => setOwnerName(event.target.value)}
                            placeholder={t("scenario.draft.owner_placeholder", language)}
                          />
                          <DatePicker
                            allowClear
                            value={dueDate ? dayjs(dueDate) : null}
                            onChange={(value) => setDueDate(value ? value.format("YYYY-MM-DD") : null)}
                          />
                        </Flex>
                        <Flex justify="space-between" align="center">
                          <Input
                            style={{ width: 180 }}
                            value={verificationPeriod}
                            onChange={(event) => setVerificationPeriod(event.target.value)}
                            placeholder={t("scenario.draft.period_placeholder", language)}
                          />
                          <Space>
                            <Button
                              icon={<SaveOutlined />}
                              type="primary"
                              loading={saving}
                              disabled={selectedKey === "baseline" || !canSaveScenario(response, selectedKey, responseKey, currentEvaluationKey) || !title.trim() || !plannedAction.trim()}
                              onClick={saveAction}
                            >
                              {t("scenario.draft.save", language)}
                            </Button>
                            <Button
                              disabled={selectedKey === "baseline" || !canSaveScenario(response, selectedKey, responseKey, currentEvaluationKey)}
                              onClick={openEventDraftModal}
                            >
                              {t("scenario.event.open", language)}
                            </Button>
                          </Space>
                        </Flex>
                        {actionResult && (
                          <Alert
                            type="success"
                            showIcon
                            message={t("scenario.draft.saved_title", language)}
                            description={
                              <Space direction="vertical" size={2}>
                                <span>{t("scenario.draft.real_id", language).replace("{id}", String(actionResult.data.id || "—")).replace("{status}", String(actionResult.data.status || "open"))}</span>
                                <span>{String(actionResult.data.owner_name || "—")} · {String(actionResult.data.due_date || "—")}</span>
                                <a href="/performance">{t("scenario.draft.go_workbench", language)}</a>
                              </Space>
                            }
                          />
                        )}
                        {eventDraftResult && (
                          <Alert
                            type="warning"
                            showIcon
                            message={t("scenario.event.success_title", language)}
                            description={
                              <Space direction="vertical" size={2}>
                                <span>{t("scenario.event.success_status", language, { status: eventDraftResult.status })}</span>
                                <span>{eventDraftResult.formalCreated ? t("scenario.event.formal_created", language) : t("scenario.event.formal_not_created", language)}</span>
                              </Space>
                            }
                          />
                        )}
                      </Space>
                    </Card>
                  </>
                )}
              </Space>
            </Col>
          </Row>

          <Modal
            title={t("scenario.event.modal_title", language)}
            open={eventDraftOpen}
            onCancel={() => setEventDraftOpen(false)}
            footer={
              <Space>
                <Button onClick={() => setEventDraftOpen(false)}>{t("scenario.event.cancel", language)}</Button>
                <Button type="primary" loading={eventDraftSubmitting} onClick={submitEventDraft}>
                  {t("scenario.event.submit", language)}
                </Button>
              </Space>
            }
            width={560}
          >
            <Space direction="vertical" size={10} className="scenario-event-fields">
              <Typography.Text type="secondary" className="scenario-event-hint">
                {t("scenario.event.modal_desc", language)}
              </Typography.Text>
              <Input
                value={eventDraftContract}
                onChange={(event) => setEventDraftContract(event.target.value)}
                placeholder={t("scenario.event.contract_placeholder", language)}
              />
              <Flex gap={8}>
                <Select
                  className="scenario-event-half"
                  value={eventDraftType}
                  onChange={setEventDraftType}
                  options={[
                    { value: "lease_modification", label: t("scenario.event.type_lease_modification", language) },
                    { value: "reassessment", label: t("scenario.event.type_reassessment", language) },
                    { value: "fixed_rent_change", label: t("scenario.event.type_fixed_rent_change", language) },
                    { value: "early_termination", label: t("scenario.event.type_early_termination", language) },
                  ]}
                />
                <DatePicker
                  className="scenario-event-half"
                  value={eventDraftDate ? dayjs(eventDraftDate) : null}
                  onChange={(value) => setEventDraftDate(value ? value.format("YYYY-MM-DD") : null)}
                  placeholder={t("scenario.event.effective_date", language)}
                />
              </Flex>
              <Input
                value={eventDraftDecision}
                onChange={(event) => setEventDraftDecision(event.target.value)}
                placeholder={t("scenario.event.decision", language)}
              />
              <Input.TextArea
                value={eventDraftReason}
                onChange={(event) => setEventDraftReason(event.target.value)}
                placeholder={t("scenario.event.change_reason", language)}
                rows={2}
              />
              <Flex gap={8}>
                <Input
                  value={eventDraftOriginal}
                  onChange={(event) => setEventDraftOriginal(event.target.value)}
                  placeholder={t("scenario.event.original_value", language)}
                />
                <Input
                  value={eventDraftNew}
                  onChange={(event) => setEventDraftNew(event.target.value)}
                  placeholder={t("scenario.event.new_value", language)}
                />
              </Flex>
              <Checkbox checked={eventDraftApproved} onChange={(event) => setEventDraftApproved(event.target.checked)}>
                {t("scenario.event.approved_label", language)}
              </Checkbox>
              <Typography.Text type="secondary" className="scenario-event-approved-hint">
                {t("scenario.event.approved_hint", language)}
              </Typography.Text>
            </Space>
          </Modal>

          <RetailAIDrawer
            open={aiOpen}
            onClose={() => setAiOpen(false)}
            pageContext={{
              page: "scenario-workbench",
              title: t("scenario.title", language),
              filters: {
                as_of: query.asOf,
                window_days: String(query.windowDays),
                classification: query.classification,
                dataset_version: query.datasetVersion || "",
                source_system: query.sourceSystem || "",
                store_id: query.storeID || "",
                horizon_months: String(horizon),
              },
            }}
          />
        </div>
      </AppLayout>
    </ProtectedRoute>
  );
}

export default function ScenarioWorkbenchPage() {
  return (
    <Suspense fallback={<div className="scenario-suspense-fallback"><Spin /></div>}>
      <ScenarioPageInner />
    </Suspense>
  );
}
