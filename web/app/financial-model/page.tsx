"use client";

import { useEffect, useMemo, useReducer, useState } from "react";
import { Alert, Button, Card, Col, Input, Row, Select, Space, Spin, Table, Typography, message } from "antd";
import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
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
import { FIN_MODEL_RUN_STATUSES, FIN_MODEL_RUN_TIE_OUT_STATUSES, type FinModelRunStatus, type FinModelRunTieOutStatus } from "./enums";
import {
  buildOpeningPayload,
  emptyOpeningForm,
  initialWorkbenchState,
  isScopeDenied,
  parseAssumptions,
  reduceWorkbench,
  type ModelRun,
  type OpeningContractRow,
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

export default function FinancialModelPage() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const [wb, dispatch] = useReducer(reduceWorkbench, initialWorkbenchState);
  const [assumptionsText, setAssumptionsText] = useState('{\n  "sssg": 0.02,\n  "gross_margin_rate": 0.4\n}');
  const [classification, setClassification] = useState<RetailDataClassification>("production");
  const [definitionOptions, setDefinitionOptions] = useState<{ id: string; title?: string }[]>([]);
  const [published, setPublished] = useState(false);
  const [fold, setFold] = useState<"month" | "quarter" | "year">("quarter");
  const [openingForm, setOpeningForm] = useState(emptyOpeningForm);
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

  const validateOpening = async () => {
    if (!token) return;
    const payload = buildOpeningPayload(openingForm);
    if (!payload.ok) {
      message.error(t(payload.error === "bad_periods" ? "finmodel.opening_bad_periods" : "finmodel.opening_invalid", language));
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

  const updateRows = (key: "leaseRef" | "engine", index: number, field: keyof OpeningContractRow, value: string) =>
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
      render: (v: string) => <StatusTag kind={TIE_OUT_TAG[v] ?? "neutral"}>{v}</StatusTag>,
    },
  ];

  const contractTable = (key: "leaseRef" | "engine", labelKey: string) => (
    <Card
      size="small"
      type="inner"
      title={t(labelKey, language)}
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
            <Space key={index} wrap>
              <Input
                className="fm-input-contract"
                placeholder={t("finmodel.opening_contract", language)}
                value={row.contract_id}
                onChange={(e) => updateRows(key, index, "contract_id", e.target.value)}
              />
              <Input
                className="fm-input-amount"
                placeholder={t("finmodel.opening_liability", language)}
                value={row.lease_liability}
                onChange={(e) => updateRows(key, index, "lease_liability", e.target.value)}
              />
              <Input
                className="fm-input-amount"
                placeholder={t("finmodel.opening_rou", language)}
                value={row.rou_asset}
                onChange={(e) => updateRows(key, index, "rou_asset", e.target.value)}
              />
              <Button size="small" type="text" icon={<DeleteOutlined />} aria-label={t("finmodel.remove", language)} onClick={() => removeRow(key, index)} />
            </Space>
          ))}
        </Space>
      )}
    </Card>
  );

  const run = wb.phase === "completed" ? wb.run : null;
  const tieOutStatus: FinModelRunTieOutStatus =
    run && (FIN_MODEL_RUN_TIE_OUT_STATUSES as readonly string[]).includes(run.tie_out_status)
      ? (run.tie_out_status as FinModelRunTieOutStatus)
      : "pending";

  const rightPanel = (() => {
    if (wb.phase === "polling") {
      return (
        <Card size="small" title={t("finmodel.result_title", language)}>
          <Space direction="vertical">
            <Space>
              <Spin size="small" />
              <StatusTag kind={RUN_STATUS_TAG[wb.status]}>{wb.status}</StatusTag>
              <Typography.Text type="secondary">{wb.runId}</Typography.Text>
            </Space>
            <Button onClick={cancelRun}>{t("finmodel.cancel_run", language)}</Button>
          </Space>
        </Card>
      );
    }
    if (wb.phase === "dispatching") {
      return (
        <Card size="small" title={t("finmodel.result_title", language)}>
          <Spin tip={t("finmodel.run_async_hint", language)}>
            <div className="fm-dispatch-placeholder" />
          </Spin>
        </Card>
      );
    }
    if (wb.phase === "cancelled") {
      return (
        <Card size="small" title={t("finmodel.result_title", language)}>
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
        <Card size="small" title={t("finmodel.result_title", language)}>
          <StateBlock state={state} language={language} onRetry={startRun} />
        </Card>
      );
    }
    if (!run) {
      return (
        <Card size="small" title={t("finmodel.result_title", language)}>
          <StateBlock state={classifyDataState({ error: null, data: null })} language={language} />
        </Card>
      );
    }
    return (
      <Space direction="vertical" size="middle" className="fm-result-stack">
        <Alert
          type={tieOutStatus === "passed" ? "success" : tieOutStatus === "failed" ? "error" : "warning"}
          showIcon
          message={`${t("finmodel.tie_out_status", language)}: ${run.tie_out_status}`}
          description={
            tieOutStatus === "passed"
              ? undefined
              : t("finmodel.tie_out_gate_note", language)
          }
        />
        {(run.gaps || []).length > 0 && (
          <Card size="small" title={t("finmodel.gaps", language)}>
            <Space direction="vertical" size="small">
              {(run.gaps || []).map((gap, i) => (
                <Space key={i} wrap>
                  <StatusTag kind="neutral">{gap.kind}</StatusTag>
                  {gap.period && <Typography.Text type="secondary">{gap.period}</Typography.Text>}
                  <span>{gap.detail}</span>
                </Space>
              ))}
            </Space>
          </Card>
        )}
        <Card size="small" title={t("finmodel.tie_outs", language)}>
          <Table size="small" bordered pagination={false} dataSource={run.tie_outs || []} columns={tieOutColumns} rowKey={(r) => `${r.check_code}-${r.period}`} />
        </Card>
        <Space wrap>
          <Button type="primary" disabled={tieOutStatus !== "passed" || published || !run.id} onClick={() => doPublish(run.id || "")}>
            {published ? t("finmodel.published", language) : t("finmodel.publish", language)}
          </Button>
          <Select value={fold} onChange={setFold} options={[
            { value: "month", label: "month" },
            { value: "quarter", label: "quarter" },
            { value: "year", label: "year" },
          ]} />
          <Button disabled={!run.id} onClick={() => doExport(run.id || "")}>{t("finmodel.export", language)}</Button>
        </Space>
      </Space>
    );
  })();

  return (
    <ProtectedRoute>
      <AppLayout>
        <PageHeader
          title={t("nav.financial_model", language)}
          meta={t("finmodel.basis_note", language)}
          primaryAction={
            <Button type="primary" loading={wb.phase === "dispatching" || wb.phase === "polling"} disabled={!canRun} onClick={startRun}>
              {t("finmodel.run", language)}
            </Button>
          }
        />
        <Row gutter={[24, 24]}>
          <Col xs={24} lg={10}>
            <Space direction="vertical" size="middle" className="fm-input-stack">
              <Card size="small" title={t("finmodel.assumptions_title", language)}>
                <Space direction="vertical" className="fm-full">
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
                  <Select
                    value={classification}
                    onChange={(v: RetailDataClassification) => setClassification(v)}
                    options={[
                      { value: "production", label: "production · Working" },
                      { value: "simulated", label: "simulated · Working" },
                    ]}
                  />
                  <Input.TextArea rows={6} value={assumptionsText} onChange={(e) => dispatch({ t: "edit_assumptions", text: e.target.value })} />
                  {!assumptionsParse.ok && (
                    <Alert type="error" showIcon message={`${t("finmodel.bad_assumptions", language)} (${assumptionsParse.error})`} />
                  )}
                  <Typography.Text type="secondary">{t("finmodel.assumptions_hint", language)}</Typography.Text>
                  <Typography.Text type="secondary">{t("finmodel.run_async_hint", language)}</Typography.Text>
                </Space>
              </Card>

              <Card size="small" title={t("finmodel.opening_gate", language)}>
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
                  <Input.TextArea
                    rows={3}
                    placeholder={t("finmodel.opening_periods", language)}
                    value={openingForm.periodsJson}
                    onChange={(e) => setOpeningForm((f) => ({ ...f, periodsJson: e.target.value }))}
                  />
                  {contractTable("leaseRef", "finmodel.opening_lease_ref")}
                  {contractTable("engine", "finmodel.opening_engine")}
                  <Space>
                    <Button loading={openingBusy} onClick={validateOpening}>
                      {t("finmodel.validate_opening", language)}
                    </Button>
                  </Space>
                  {openingResult &&
                    (openingResult.passed ? (
                      <Alert type="success" showIcon message={t("finmodel.opening_passed", language)} />
                    ) : (
                      <Alert type="warning" showIcon message={JSON.stringify(openingResult.failures ?? {})} />
                    ))}
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
