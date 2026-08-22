"use client";

import { useEffect, useMemo, useReducer, useState, type ReactNode } from "react";
import { Alert, Button, Card, Col, Collapse, Input, Row, Select, Space, Spin, Table, Tag, Tooltip, Typography, message } from "antd";
import { PlusOutlined } from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { StateBlock } from "../components/StateBlock";
import { StatusTag } from "../components/StatusTag";
import { apiErrorMessage, financialModelApi, type RetailDataClassification } from "../lib/api";
import { classifyDataState, type DataState } from "../lib/dataState";
import { fmtNum } from "../lib/format";
import { t } from "../lib/i18n";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import {
  FIN_MODEL_PERIOD_GRAINS,
  FIN_MODEL_RUN_STATUSES,
  FIN_MODEL_RUN_TIE_OUT_STATUSES,
  GAP_KIND_LABEL,
  PERIOD_GRAIN_LABEL,
  RUN_STATUS_LABEL,
  TIE_OUT_LABEL,
  type FinModelPeriodGrain,
  type FinModelRunStatus,
  type FinModelRunTieOutStatus,
} from "./enums";
import { ContractRowInputs } from "./contract-rows";
import { AssumptionForm, AssumptionUnknownKeys } from "./assumption-form";
import { financialModelHelpContent } from "../components/help-content";
import { HelpTrigger } from "../components/HelpDrawer";
import { EXAMPLE_ASSUMPTIONS, EXAMPLE_OPENING_FORM, assumptionHint } from "./hints";
import { FormulaEditor } from "./FormulaEditor";
import {
  addPeriod,
  applyAssumptionFormValues,
  buildOpeningPayload,
  emptyOpeningForm,
  initialWorkbenchState,
  isScopeDenied,
  parseAssumptions,
  reduceWorkbench,
  removePeriod,
  type ModelRun,
} from "./workbench";

type RunResultState = DataState<{ run: ModelRun; persisted?: boolean }>;

const RUN_STATUS_TAG: Record<FinModelRunStatus, "processing" | "success" | "error" | "neutral"> = {
  queued: "processing",
  running: "processing",
  completed: "success",
  failed: "error",
  cancelled: "neutral",
};

const TIE_OUT_TAG: Record<string, "success" | "error" | "warning" | "neutral"> = {
  passed: "success",
  failed: "error",
  degraded: "warning",
  pending: "neutral",
};

/** buildOpeningPayload 的错误码 → 人话文案。键集由 workbench 的错误联合锁定。 */
const OPENING_ERROR_KEY: Record<string, string> = {
  missing_entity: "finmodel.opening_invalid",
  missing_currency: "finmodel.opening_invalid",
  row_incomplete: "finmodel.opening_invalid",
  no_periods: "finmodel.opening_no_periods",
  bad_balances: "finmodel.opening_bad_balances",
  missing_balance_for_period: "finmodel.opening_missing_balance",
};

// 引擎侧自动取数与导入侧上传的后端端点属 B 阶段（spec ③ 地基层）：
// 页面渲染诚实的不可用态，不造假按钮；手动路径是唯一能真正走到校验的入口。

export default function FinancialModelPage() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const [wb, dispatch] = useReducer(reduceWorkbench, initialWorkbenchState);
  // 示例即初始值：打开页面就看见「合法输入长什么样」（spec 追加决策①）。
  const [assumptionsText, setAssumptionsText] = useState(EXAMPLE_ASSUMPTIONS);
  const [classification, setClassification] = useState<RetailDataClassification>("production");
  const [definitionOptions, setDefinitionOptions] = useState<{ id: string; title?: string }[]>([]);
  const [published, setPublished] = useState(false);
  const [fold, setFold] = useState<FinModelPeriodGrain>("quarter");
  const [openingForm, setOpeningForm] = useState(emptyOpeningForm);
  const [periodInput, setPeriodInput] = useState("");
  const [openingResult, setOpeningResult] = useState<{ passed: boolean; failures?: unknown } | null>(null);
  const [openingBusy, setOpeningBusy] = useState(false);

  // P2-2：会话内同输入复用同一幂等键，输入变化才换。
  const idempotencyKey = useMemo(
    () => (typeof crypto !== "undefined" && "randomUUID" in crypto ? crypto.randomUUID() : `run-${Date.now()}-${Math.random()}`),
    // eslint-disable-next-line react-hooks/exhaustive-deps -- P2-C gate close-out: legacy dep semantics kept as-is; loaders are rebuilt every render so adding them would loop refetches. useCallback refactor tracked separately; do not add new exemptions.
    [wb.definitionId, assumptionsText, classification],
  );

  // 定义列表：后端当前是桩（返回空数组），空态必须诚实呈现而不是藏起输入。
  useEffect(() => {
    if (!token) return;
    financialModelApi
      .listDefinitions(token)
      .then((body) => {
        const defs = (body.definitions || []) as { id?: string; title?: string }[];
        setDefinitionOptions(defs.filter((d) => typeof d.id === "string") as { id: string; title?: string }[]);
      })
      .catch(() => setDefinitionOptions([]));
  }, [token]);

  // 异步 run 轮询：phase 离开 polling 时清理定时器；瞬态查询失败继续轮询。
  const polling = wb.phase === "polling";
  const pollingRunId = polling ? wb.runId : null;
  useEffect(() => {
    if (!polling || !pollingRunId || !token) return;
    let active = true;
    const timer = setInterval(async () => {
      try {
        const body = await financialModelApi.getRun(pollingRunId, token);
        if (!active) return;
        const raw = body.run as (ModelRun & { status?: string }) | null;
        const status = (FIN_MODEL_RUN_STATUSES as readonly string[]).includes(raw?.status || "")
          ? (raw!.status as FinModelRunStatus)
          : "running";
        dispatch({ t: "run_polled", status, run: status === "completed" && raw ? raw : undefined });
      } catch {
        /* 瞬态错误：保持轮询 */
      }
    }, 2000);
    return () => {
      active = false;
      clearInterval(timer);
    };
  }, [polling, pollingRunId, token]);

  const assumptionsParse = parseAssumptions(assumptionsText);
  const canRun = wb.phase === "idle" && !!wb.definitionId && assumptionsParse.ok;

  const startRun = async () => {
    if (wb.phase !== "idle" || !wb.definitionId || !assumptionsParse.ok || !token) return;
    dispatch({ t: "run_requested" });
    setPublished(false);
    try {
      const body = await financialModelApi.run(
        wb.definitionId,
        {
          definition_id: wb.definitionId,
          assumptions: assumptionsParse.value,
          data_classification: classification,
          versions: { data_version: "working", assumption_version: "v1", model_definition_version: "v1" },
          idempotency_key: idempotencyKey,
          async: true,
        },
        token,
      );
      if ("run_id" in body && (body as { run_id?: string }).run_id) {
        dispatch({ t: "run_dispatched", runId: (body as { run_id: string }).run_id });
      } else {
        const syncBody = body as { run: ModelRun; persisted?: boolean };
        dispatch({ t: "run_sync_done", run: syncBody.run, persisted: syncBody.persisted });
        if (syncBody.persisted) message.success(t("finmodel.persisted", language));
      }
    } catch (err) {
      dispatch({
        t: "error",
        kind: isScopeDenied(err) ? "scope_denied" : "failed",
        message: apiErrorMessage(err),
      });
    }
  };

  const cancelRun = async () => {
    if (wb.phase !== "polling" || !token) return;
    try {
      await financialModelApi.cancelRun(wb.runId, token);
    } catch {
      /* 取消请求失败不阻塞：轮询仍会收敛到终态 */
    }
    dispatch({ t: "cancel_done" });
  };

  const doPublish = async (runId: string) => {
    if (!token) return;
    try {
      await financialModelApi.publishRun(runId, token);
      setPublished(true);
      message.success(t("finmodel.published", language));
    } catch (err) {
      message.error(apiErrorMessage(err));
    }
  };

  const doExport = async (runId: string) => {
    if (!token || !runId) return;
    try {
      const blob = await financialModelApi.exportRun(runId, fold, token);
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = `financial-model-run-${runId}-${fold}.xlsx`;
      anchor.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      message.error(apiErrorMessage(err));
    }
  };

  const fillAssumptionExample = () => {
    setAssumptionsText(EXAMPLE_ASSUMPTIONS);
    dispatch({ t: "edit_assumptions", text: EXAMPLE_ASSUMPTIONS });
    message.success(t("finmodel.example_filled", language));
  };

  const fillOpeningExample = () => {
    setOpeningForm(JSON.parse(JSON.stringify(EXAMPLE_OPENING_FORM)) as typeof emptyOpeningForm);
    setOpeningResult(null);
    message.success(t("finmodel.example_filled", language));
  };

  const submitPeriod = () => {
    setOpeningForm((f) => ({ ...f, periods: addPeriod(f.periods, periodInput) }));
    setPeriodInput("");
  };

  const validateOpening = async () => {
    if (!token) return;
    const payload = buildOpeningPayload(openingForm);
    if (!payload.ok) {
      message.error(t(OPENING_ERROR_KEY[payload.error] ?? "finmodel.opening_invalid", language));
      return;
    }
    setOpeningBusy(true);
    try {
      const body = await financialModelApi.validateOpening(payload.payload, token);
      setOpeningResult(body as { passed: boolean; failures?: unknown });
    } catch (err) {
      message.error(apiErrorMessage(err));
    } finally {
      setOpeningBusy(false);
    }
  };

  const updateRows = (key: "leaseRef" | "engine", index: number, field: keyof (typeof emptyOpeningForm)["leaseRef"][number], value: string) =>
    setOpeningForm((f) => ({ ...f, [key]: f[key].map((row, i) => (i === index ? { ...row, [field]: value } : row)) }));
  const addRow = (key: "leaseRef" | "engine") =>
    setOpeningForm((f) => ({ ...f, [key]: [...f[key], { contract_id: "", lease_liability: "", rou_asset: "" }] }));
  const removeRow = (key: "leaseRef" | "engine", index: number) =>
    setOpeningForm((f) => ({ ...f, [key]: f[key].filter((_, i) => i !== index) }));

  const tieOutColumns = [
    { title: t("finmodel.check", language), dataIndex: "check_code", key: "check_code" },
    { title: t("finmodel.period", language), dataIndex: "period", key: "period" },
    { title: t("finmodel.expected", language), dataIndex: "expected", key: "expected", align: "right" as const, render: (v: number | null) => fmtNum(v) },
    { title: t("finmodel.actual", language), dataIndex: "actual", key: "actual", align: "right" as const, render: (v: number | null) => fmtNum(v) },
    { title: t("finmodel.diff", language), dataIndex: "diff", key: "diff", align: "right" as const, render: (v: number | null) => fmtNum(v) },
    {
      title: t("finmodel.status", language),
      dataIndex: "status",
      key: "status",
      render: (v: string) => {
        const known = (FIN_MODEL_RUN_TIE_OUT_STATUSES as readonly string[]).includes(v);
        return (
          <StatusTag kind={TIE_OUT_TAG[v] ?? "neutral"}>
            {known ? t(TIE_OUT_LABEL[v as FinModelRunTieOutStatus], language) : v}
          </StatusTag>
        );
      },
    },
  ];

  const contractTable = (key: "leaseRef" | "engine", title: ReactNode) => (
    <Card
      size="small"
      type="inner"
      title={title}
      extra={
        <Button size="small" icon={<PlusOutlined />} onClick={() => addRow(key)}>
          {t("finmodel.add_row", language)}
        </Button>
      }
    >
      {openingForm[key].length === 0 ? (
        <Typography.Text type="secondary">{t("finmodel.add_row_hint", language)}</Typography.Text>
      ) : (
        <Space direction="vertical" size="small">
          {openingForm[key].map((row, index) => (
            <ContractRowInputs
              key={index}
              row={row}
              index={index}
              idPrefix={`fm-opening-${key}`}
              language={language}
              onChange={(field, value) => updateRows(key, index, field, value)}
              onRemove={() => removeRow(key, index)}
            />
          ))}
        </Space>
      )}
    </Card>
  );

  /** 三道闸图例 chip：hover 出人话解释（spec 追加决策①）。 */
  const gateChip = (labelKey: string, tooltipKey: string) => (
    <Tooltip title={t(tooltipKey, language)}>
      <span className="fm-gate-chip">{t(labelKey, language)}</span>
    </Tooltip>
  );

  const run = wb.phase === "completed" ? wb.run : null;
  const tieOutStatus: FinModelRunTieOutStatus =
    run && (FIN_MODEL_RUN_TIE_OUT_STATUSES as readonly string[]).includes(run.tie_out_status)
      ? (run.tie_out_status as FinModelRunTieOutStatus)
      : "pending";

  const rightPanel = (() => {
    if (wb.phase === "polling") {
      return (
        <Card size="small" title={t("finmodel.step_run", language)}>
          <Space direction="vertical">
            <Space>
              <Spin size="small" />
              <StatusTag kind={RUN_STATUS_TAG[wb.status]}>{t(RUN_STATUS_LABEL[wb.status], language)}</StatusTag>
              <Typography.Text type="secondary">{wb.runId}</Typography.Text>
            </Space>
            <Button onClick={cancelRun}>{t("finmodel.cancel_run", language)}</Button>
          </Space>
        </Card>
      );
    }
    if (wb.phase === "dispatching") {
      return (
        <Card size="small" title={t("finmodel.step_run", language)}>
          <Spin tip={t("finmodel.run_async_hint", language)}>
            <div className="fm-dispatch-placeholder" />
          </Spin>
        </Card>
      );
    }
    if (wb.phase === "cancelled") {
      return (
        <Card size="small" title={t("finmodel.step_run", language)}>
          <Alert type="info" showIcon message={t("finmodel.cancelled", language)} />
        </Card>
      );
    }
    if (wb.phase === "failed" || wb.phase === "scope_denied") {
      const state =
        wb.phase === "scope_denied"
          ? ({ kind: "scope_denied", reason: wb.reason } as RunResultState)
          : ({ kind: "failed", message: wb.message } as RunResultState);
      return (
        <Card size="small" title={t("finmodel.step_run", language)}>
          <StateBlock state={state} language={language} onRetry={startRun} />
        </Card>
      );
    }
    if (!run) {
      return (
        <Card size="small" title={t("finmodel.step_run", language)}>
          <StateBlock state={classifyDataState({ error: null, data: null })} language={language} />
        </Card>
      );
    }
    return (
      <Space direction="vertical" size="middle" className="fm-result-stack">
        <Card size="small" title={`${t("finmodel.step_run", language)} · ${t("finmodel.result_title", language)}`}>
          <Space direction="vertical" size="middle" className="fm-full">
            <Alert
              type={tieOutStatus === "passed" ? "success" : tieOutStatus === "failed" ? "error" : "warning"}
              showIcon
              message={`${t("finmodel.tie_out_status", language)}：${t(TIE_OUT_LABEL[tieOutStatus], language)}`}
              description={
                tieOutStatus === "passed"
                  ? undefined
                  : t("finmodel.tie_out_gate_note", language)
              }
            />
            {(run.gaps || []).length > 0 && (
              <Card size="small" type="inner" title={t("finmodel.gaps", language)}>
                <Space direction="vertical" size="small">
                  {(run.gaps || []).map((gap, i) => {
                    const gapLabelKey = GAP_KIND_LABEL[gap.kind];
                    return (
                      <Space key={i} wrap>
                        <StatusTag kind="neutral">
                          {gapLabelKey
                            ? t(gapLabelKey, language)
                            : `${gap.kind} · ${t("finmodel.gap_kind_unknown", language)}`}
                        </StatusTag>
                        {gap.period && <Typography.Text type="secondary">{gap.period}</Typography.Text>}
                        <span>{gap.detail}</span>
                      </Space>
                    );
                  })}
                </Space>
              </Card>
            )}
            <Card size="small" type="inner" title={t("finmodel.tie_outs", language)}>
              <Table size="small" bordered pagination={false} dataSource={run.tie_outs || []} columns={tieOutColumns} rowKey={(r) => `${r.check_code}-${r.period}`} />
            </Card>
          </Space>
        </Card>
        <Card size="small" title={t("finmodel.step_publish_export", language)}>
          <Space wrap>
            <Button type="primary" disabled={tieOutStatus !== "passed" || published || !run.id} onClick={() => doPublish(run.id || "")}>
              {published ? t("finmodel.published", language) : t("finmodel.publish", language)}
            </Button>
            <Select value={fold} onChange={(v: FinModelPeriodGrain) => setFold(v)} options={FIN_MODEL_PERIOD_GRAINS.map((grain) => ({ value: grain, label: t(PERIOD_GRAIN_LABEL[grain], language) }))} />
            <Button disabled={!run.id} onClick={() => doExport(run.id || "")}>{t("finmodel.export", language)}</Button>
          </Space>
        </Card>
      </Space>
    );
  })();

  return (
    <ProtectedRoute>
      <AppLayout>
        <PageHeader
          title={t("nav.financial_model", language)}
          help={<HelpTrigger content={financialModelHelpContent(language)} language={language} />}
          meta={
            <>
              {t("finmodel.page_intro", language)}
              {" · "}
              {t("finmodel.basis_note", language)}
            </>
          }
          primaryAction={
            <Button type="primary" loading={wb.phase === "dispatching" || wb.phase === "polling"} disabled={!canRun} onClick={startRun}>
              {t("finmodel.run", language)}
            </Button>
          }
        />
        <Row gutter={[24, 24]}>
          <Col xs={24} lg={10}>
            <Space direction="vertical" size="middle" className="fm-input-stack">
              <Card size="small" title={t("finmodel.step_select_def", language)}>
                <Space direction="vertical" size="small" className="fm-full">
                  <Typography.Text type="secondary">{t("finmodel.card_hint_select_def", language)}</Typography.Text>
                  {definitionOptions.length > 0 ? (
                    <Select
                      className="fm-full"
                      placeholder={t("finmodel.definition_select", language)}
                      value={wb.definitionId || undefined}
                      onChange={(id: string) => dispatch({ t: "select_definition", id })}
                      options={definitionOptions.map((d) => ({ value: d.id, label: d.title ? `${d.title} (${d.id})` : d.id }))}
                    />
                  ) : (
                    <Alert type="info" showIcon message={t("finmodel.definition_empty", language)} />
                  )}
                  <Input
                    placeholder={t("finmodel.definition_manual", language)}
                    value={wb.definitionId}
                    onChange={(e) => dispatch({ t: "select_definition", id: e.target.value })}
                  />
                </Space>
              </Card>

              <Card
                size="small"
                title={t("finmodel.step_assumptions", language)}
                extra={
                  <Button size="small" onClick={fillAssumptionExample}>
                    {t("finmodel.fill_example", language)}
                  </Button>
                }
              >
                <Space direction="vertical" size="small" className="fm-full">
                  <Typography.Text type="secondary">{t("finmodel.classification_label", language)}</Typography.Text>
                  <Space wrap size={8} className="fm-full">
                    <Select
                      value={classification}
                      onChange={(v: RetailDataClassification) => setClassification(v)}
                      options={[
                        { value: "production", label: t("trust.classification_production", language) },
                        { value: "simulated", label: t("trust.classification_simulated", language) },
                      ]}
                    />
                    <Tooltip title={t("finmodel.version_state_tooltip", language)}>
                      <StatusTag kind="neutral">{t("trust.basis_working", language)}</StatusTag>
                    </Tooltip>
                  </Space>
                  {/* F3-1: key-value form is the primary entry; JSON demoted
                      to the advanced collapse. Frozen while JSON invalid. */}
                  {!assumptionsParse.ok && (
                    <Alert type="error" showIcon message={`${t("finmodel.bad_assumptions", language)} (${assumptionsParse.error})`} />
                  )}
                  <AssumptionForm
                    values={assumptionsParse.ok ? assumptionsParse.value : {}}
                    disabled={!assumptionsParse.ok}
                    language={language}
                    onChange={(changes) =>
                      dispatch({ t: "edit_assumptions", text: applyAssumptionFormValues(assumptionsText, changes) })
                    }
                  />
                  {assumptionsParse.ok && <AssumptionUnknownKeys values={assumptionsParse.value} language={language} />}
                  <Collapse
                    items={[
                      {
                        key: "advanced",
                        label: t("finmodel.assumptions_advanced", language),
                        children: (
                          <Space direction="vertical" size="small" className="fm-full">
                            <Typography.Text type="secondary">{t("finmodel.assumptions_hint", language)}</Typography.Text>
                            <Input.TextArea rows={6} value={assumptionsText} onChange={(e) => dispatch({ t: "edit_assumptions", text: e.target.value })} />
                            {assumptionsParse.ok
                              ? Object.keys(assumptionsParse.value).length > 0 && (
                                  <div className="fm-hint-list">
                                    {Object.keys(assumptionsParse.value).map((key) => {
                                      const hint = assumptionHint(key, language);
                                      return (
                                        <div key={key} className="fm-hint-row">
                                          <Typography.Text code className="fm-hint-key">{key}</Typography.Text>
                                          {hint.known ? (
                                            <Typography.Text type="secondary">{hint.label}</Typography.Text>
                                          ) : (
                                            <StatusTag kind="neutral">{t("finmodel.hint_unknown", language)}</StatusTag>
                                          )}
                                        </div>
                                      );
                                    })}
                                  </div>
                                )
                              : (
                                  <Alert type="error" showIcon message={`${t("finmodel.bad_assumptions", language)} (${assumptionsParse.error})`} />
                                )}
                          </Space>
                        ),
                      },
                    ]}
                  />
                  <Typography.Text type="secondary">{t("finmodel.run_async_hint", language)}</Typography.Text>
                </Space>
              </Card>

              {/* R2-4: formula editor (RH6). Validation is backend-only by contract (D-R16). */}
              <Card size="small" title={t("finmodel.formula.editor_title", language)}>
                <FormulaEditor language={language} />
              </Card>

              <Card
                size="small"
                title={t("finmodel.step_opening", language)}
                extra={
                  <Button size="small" onClick={fillOpeningExample}>
                    {t("finmodel.fill_example", language)}
                  </Button>
                }
              >
                <Space direction="vertical" size="small" className="fm-full">
                  <Typography.Text type="secondary">{t("finmodel.card_hint_opening", language)}</Typography.Text>
                  <Space wrap size={8} className="fm-gate-legend">
                    {gateChip("finmodel.gate1_label", "finmodel.gate1_tooltip")}
                    {gateChip("finmodel.gate2_label", "finmodel.gate2_tooltip")}
                    {gateChip("finmodel.gate3_label", "finmodel.gate3_tooltip")}
                  </Space>
                  <div className="fm-full">
                    <Typography.Text type="secondary" className="fm-step-label">{t("finmodel.opening_step1", language)}</Typography.Text>
                    <Space wrap>
                      <Input
                        className="fm-opening-period"
                        placeholder={t("finmodel.opening_period_placeholder", language)}
                        value={periodInput}
                        onChange={(e) => setPeriodInput(e.target.value)}
                        onPressEnter={submitPeriod}
                      />
                      <Button onClick={submitPeriod}>{t("finmodel.opening_add_period", language)}</Button>
                    </Space>
                    {openingForm.periods.length > 0 && (
                      <div className="fm-period-tags">
                        {openingForm.periods.map((p) => (
                          <Tag
                            key={p}
                            closable
                            onClose={() => setOpeningForm((f) => ({ ...f, periods: removePeriod(f.periods, p) }))}
                          >
                            {p}
                          </Tag>
                        ))}
                      </div>
                    )}
                  </div>
                  <div className="fm-full">
                    <Typography.Text type="secondary" className="fm-step-label">{t("finmodel.opening_step2", language)}</Typography.Text>
                    <Alert type="info" showIcon message={t("finmodel.opening_auto_unavailable", language)} />
                  </div>
                  <div className="fm-full">
                    <Typography.Text type="secondary" className="fm-step-label">{t("finmodel.opening_step3", language)}</Typography.Text>
                    <Alert type="info" showIcon message={t("finmodel.opening_import_unavailable", language)} />
                  </div>
                  <div className="fm-full">
                    <Typography.Text type="secondary" className="fm-step-label">{t("finmodel.opening_step4", language)}</Typography.Text>
                    <Space wrap>
                      <Button loading={openingBusy} onClick={validateOpening}>
                        {t("finmodel.validate_opening", language)}
                      </Button>
                      {openingResult &&
                        (openingResult.passed ? (
                          <Alert type="success" showIcon message={t("finmodel.opening_passed", language)} />
                        ) : (
                          <Alert type="warning" showIcon message={JSON.stringify(openingResult.failures ?? {})} />
                        ))}
                    </Space>
                  </div>
                  <Collapse
                    items={[
                      {
                        key: "advanced",
                        label: t("finmodel.opening_advanced", language),
                        children: (
                          <Space direction="vertical" size="small" className="fm-full">
                            <Space wrap>
                              <Input
                                className="fm-opening-entity"
                                placeholder={t("finmodel.opening_entity", language)}
                                value={openingForm.legalEntityId}
                                onChange={(e) => setOpeningForm((f) => ({ ...f, legalEntityId: e.target.value }))}
                              />
                              <Input
                                className="fm-opening-currency"
                                placeholder={t("finmodel.opening_currency", language)}
                                value={openingForm.currency}
                                onChange={(e) => setOpeningForm((f) => ({ ...f, currency: e.target.value }))}
                              />
                              <Input
                                className="fm-opening-policy"
                                placeholder={t("finmodel.opening_policy", language)}
                                value={openingForm.policyVersion}
                                onChange={(e) => setOpeningForm((f) => ({ ...f, policyVersion: e.target.value }))}
                              />
                            </Space>
                            <Space wrap size={8}>
                              {gateChip("finmodel.gate1_label", "finmodel.gate1_tooltip")}
                              {gateChip("finmodel.gate2_label", "finmodel.gate2_tooltip")}
                            </Space>
                            <Input.TextArea
                              rows={6}
                              placeholder={t("finmodel.balances_json_placeholder", language)}
                              value={openingForm.balancesJson}
                              onChange={(e) => setOpeningForm((f) => ({ ...f, balancesJson: e.target.value }))}
                            />
                            {contractTable(
                              "leaseRef",
                              <Tooltip title={t("finmodel.gate3_tooltip", language)}>
                                <span>{t("finmodel.opening_lease_ref", language)} · {t("finmodel.gate3_label", language)}</span>
                              </Tooltip>,
                            )}
                            {contractTable(
                              "engine",
                              <Tooltip title={t("finmodel.gate3_tooltip", language)}>
                                <span>{t("finmodel.opening_engine", language)} · {t("finmodel.gate3_label", language)}</span>
                              </Tooltip>,
                            )}
                          </Space>
                        ),
                      },
                    ]}
                  />
                </Space>
              </Card>
            </Space>
          </Col>
          <Col xs={24} lg={14}>{rightPanel}</Col>
        </Row>
      </AppLayout>
    </ProtectedRoute>
  );
}
