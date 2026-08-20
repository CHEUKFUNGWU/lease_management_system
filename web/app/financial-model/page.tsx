"use client";

import { useMemo, useState } from "react";
import { Alert, Button, Card, Input, Select, Space, Table, Typography, message } from "antd";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { StateBlock } from "../components/StateBlock";
import { StatusTag } from "../components/StatusTag";
import { apiErrorMessage, financialModelApi, type RetailDataClassification } from "../lib/api";
import { classifyDataState, type DataState } from "../lib/dataState";
import { t } from "../lib/i18n";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { FIN_MODEL_RUN_TIE_OUT_STATUSES, type FinModelRunTieOutStatus } from "./enums";

type TieOutRow = {
  check_code: string; period: string; expected?: number | null;
  actual?: number | null; diff?: number | null; status: string;
};
type ModelRun = {
  periods: string[];
  tie_out_status: FinModelRunTieOutStatus;
  tie_outs: TieOutRow[];
  gaps?: { kind: string; period?: string; detail: string }[];
};

export default function FinancialModelPage() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const [definitionId, setDefinitionId] = useState("");
  const [assumptions, setAssumptions] = useState('{\n  "sssg": 0.02,\n  "gross_margin_rate": 0.4\n}');
  // P2-3：data_classification 不再硬编码 production，UI 可产出 simulated run。
  const [classification, setClassification] = useState<RetailDataClassification>("production");
  const [running, setRunning] = useState(false);
  const [runState, setRunState] = useState<DataState<{ run: ModelRun; persisted?: boolean }> | null>(null);
  const [validationError, setValidationError] = useState<string | null>(null);

  const [openingJSON, setOpeningJSON] = useState("");
  const [openingResult, setOpeningResult] = useState<any>(null);

  // P2-2：幂等键不再用时间戳拼（时间戳不是真幂等，同输入重复点会拿新 key）。
  // 改为当次会话内同一份输入（definitionId/assumptions/classification）复用
  // 同一个 key；输入变化才重新生成。
  const idempotencyKey = useMemo(
    () => (typeof crypto !== "undefined" && "randomUUID" in crypto ? crypto.randomUUID() : `run-${Date.now()}-${Math.random()}`),
    [definitionId, assumptions, classification],
  );

  const runModel = async () => {
    if (!token || !definitionId) return;
    setRunning(true);
    setValidationError(null);
    try {
      let parsed: Record<string, unknown> = {};
      try {
        parsed = JSON.parse(assumptions || "{}");
      } catch {
        setValidationError(t("finmodel.bad_assumptions", language));
        setRunning(false);
        return;
      }
      const body = await financialModelApi.run(
        definitionId,
        {
          definition_id: definitionId,
          assumptions: parsed,
          data_classification: classification,
          versions: { data_version: "working", assumption_version: "v1", model_definition_version: "v1" },
          idempotency_key: idempotencyKey,
        },
        token,
      );
      setRunState({ kind: "ready", data: body as { run: ModelRun; persisted?: boolean } });
      if (body.persisted) message.success(t("finmodel.persisted", language));
    } catch (err: unknown) {
      setRunState(classifyDataState<{ run: ModelRun; persisted?: boolean }>({ error: err, data: null }));
    } finally {
      setRunning(false);
    }
  };

  const run = runState?.kind === "ready" ? (runState.data?.run ?? null) : null;

  const validateOpening = async () => {
    if (!token) return;
    try {
      const parsed = JSON.parse(openingJSON || "{}");
      const body = await financialModelApi.validateOpening(parsed, token);
      setOpeningResult(body);
    } catch (err: any) {
      message.error(apiErrorMessage(err));
    }
  };

  const tieOutColumns = [
    { title: t("finmodel.check", language), dataIndex: "check_code", key: "check_code" },
    { title: t("finmodel.period", language), dataIndex: "period", key: "period" },
    { title: t("finmodel.expected", language), dataIndex: "expected", key: "expected", align: "right" as const,
      render: (v: number | null) => (v == null ? "—" : v.toLocaleString()) },
    { title: t("finmodel.actual", language), dataIndex: "actual", key: "actual", align: "right" as const,
      render: (v: number | null) => (v == null ? "—" : v.toLocaleString()) },
    { title: t("finmodel.diff", language), dataIndex: "diff", key: "diff", align: "right" as const,
      render: (v: number | null) => (v == null ? "—" : v.toLocaleString()) },
    { title: t("finmodel.status", language), dataIndex: "status", key: "status",
      render: (v: string) => <StatusTag kind={v === "passed" ? "success" : v === "failed" ? "error" : "warning"}>{v}</StatusTag> },
  ];

  // P2-3：run 错误态由 StateBlock 呈现（failed / scope_denied / empty），
  // scope_denied 保留原因，不并入「无数据」。
  const runErrorState = runState && runState.kind !== "ready" ? runState : null;
  const runTieOutStatus = run ? (FIN_MODEL_RUN_TIE_OUT_STATUSES as readonly string[]).includes(run.tie_out_status) ? run.tie_out_status : "pending" : null;

  return (
    <ProtectedRoute>
      <AppLayout>
        <PageHeader
          title={t("nav.financial_model", language)}
          meta={t("finmodel.basis_note", language)}
          primaryAction={
            <Button type="primary" loading={running} onClick={runModel} disabled={!definitionId}>
              {t("finmodel.run", language)}
            </Button>
          }
        />
        <Space direction="vertical" size="middle">
          <Card size="small" title={t("finmodel.assumptions", language)}>
            <Space direction="vertical">
              <Input
                placeholder={t("finmodel.definition_id", language)}
                value={definitionId}
                onChange={(event) => setDefinitionId(event.target.value)}
              />
              <Select
                value={classification}
                onChange={(value) => setClassification(value as RetailDataClassification)}
                options={[
                  { value: "production", label: "production" },
                  { value: "simulated", label: "simulated" },
                ]}
              />
              <Input.TextArea
                rows={6}
                value={assumptions}
                onChange={(event) => setAssumptions(event.target.value)}
              />
              <Typography.Text type="secondary">{t("finmodel.assumptions_hint", language)}</Typography.Text>
            </Space>
          </Card>
          {validationError && <Alert type="error" message={validationError} showIcon />}
          {runErrorState && <StateBlock state={runErrorState} language={language} onRetry={runModel} />}
          {run && (
            <>
              <Alert
                type={runTieOutStatus === "passed" ? "success" : runTieOutStatus === "failed" ? "error" : "warning"}
                message={`${t("finmodel.tie_out_status", language)}: ${run.tie_out_status}`}
                showIcon
              />
              {(run.gaps || []).length > 0 && (
                <Alert type="warning" message={(run.gaps || []).map((g) => g.detail).join("；")} showIcon />
              )}
              <Card size="small" title={t("finmodel.tie_outs", language)}>
                <Table size="small" bordered pagination={false} dataSource={run.tie_outs || []} columns={tieOutColumns} rowKey={(row) => `${row.check_code}-${row.period}`} />
              </Card>
            </>
          )}
          <Card size="small" title={t("finmodel.opening_gate", language)}>
            <Space direction="vertical">
              <Input.TextArea rows={8} value={openingJSON} onChange={(event) => setOpeningJSON(event.target.value)} />
              <Button onClick={validateOpening}>{t("finmodel.validate_opening", language)}</Button>
              {openingResult && (
                <Alert
                  type={openingResult.passed ? "success" : "warning"}
                  message={openingResult.passed ? t("finmodel.opening_passed", language) : JSON.stringify(openingResult.failures)}
                  showIcon
                />
              )}
            </Space>
          </Card>
        </Space>
      </AppLayout>
    </ProtectedRoute>
  );
}
