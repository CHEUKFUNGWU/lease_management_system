"use client";

import { useState } from "react";
import { Alert, Button, Card, Input, Space, Table, Typography, message } from "antd";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { StatusTag } from "../components/StatusTag";
import { apiErrorMessage } from "../lib/api";
import { t } from "../lib/i18n";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";

type TieOutRow = {
  check_code: string; period: string; expected?: number | null;
  actual?: number | null; diff?: number | null; status: string;
};
type ModelRun = {
  periods: string[];
  tie_out_status: string;
  tie_outs: TieOutRow[];
  gaps?: { kind: string; period?: string; detail: string }[];
};

export default function FinancialModelPage() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const [definitionId, setDefinitionId] = useState("");
  const [assumptions, setAssumptions] = useState('{\n  "sssg": 0.02,\n  "gross_margin_rate": 0.4\n}');
  const [running, setRunning] = useState(false);
  const [run, setRun] = useState<ModelRun | null>(null);
  const [error, setError] = useState<string | null>(null);

  const [openingJSON, setOpeningJSON] = useState("");
  const [openingResult, setOpeningResult] = useState<any>(null);

  const runModel = async () => {
    if (!token || !definitionId) return;
    setRunning(true);
    setError(null);
    try {
      let parsed: Record<string, unknown> = {};
      try {
        parsed = JSON.parse(assumptions || "{}");
      } catch {
        setError(t("finmodel.bad_assumptions", language));
        setRunning(false);
        return;
      }
      const response = await fetch(`/api/v1/financial-model/definitions/${encodeURIComponent(definitionId)}/runs`, {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify({
          definition_id: definitionId,
          assumptions: parsed,
          data_classification: "production",
          versions: { data_version: "working", assumption_version: "v1", model_definition_version: "v1" },
          idempotency_key: `run-${Date.now()}`,
        }),
      });
      const body = await response.json();
      if (!response.ok) throw new Error(body?.error || "run failed");
      setRun(body.run as ModelRun);
      if (body.persisted) message.success(t("finmodel.persisted", language));
    } catch (err: any) {
      setError(apiErrorMessage(err));
      setRun(null);
    } finally {
      setRunning(false);
    }
  };

  const validateOpening = async () => {
    if (!token) return;
    try {
      const parsed = JSON.parse(openingJSON || "{}");
      const response = await fetch("/api/v1/financial-model/opening/validate", {
        method: "POST",
        headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}` },
        body: JSON.stringify(parsed),
      });
      const body = await response.json();
      if (!response.ok) throw new Error(body?.error || "validate failed");
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
              <Input.TextArea
                rows={6}
                value={assumptions}
                onChange={(event) => setAssumptions(event.target.value)}
              />
              <Typography.Text type="secondary">{t("finmodel.assumptions_hint", language)}</Typography.Text>
            </Space>
          </Card>
          {error && <Alert type="error" message={error} showIcon />}
          {run && (
            <>
              <Alert
                type={run.tie_out_status === "passed" ? "success" : "warning"}
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
