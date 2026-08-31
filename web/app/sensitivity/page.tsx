"use client";

import { StatusTag } from "../components/StatusTag";

import { Suspense, useMemo, useState } from "react";
import { Button, Card, Col, Form, InputNumber, Row, Select, Table, Typography, message } from "antd";
import { CalculatorOutlined } from "@ant-design/icons";
import AppLayout from "../components/AppLayout";
import PageHeader from "../components/PageHeader";
import ProtectedRoute from "../components/ProtectedRoute";
import { contractApi, reportApi } from "../lib/api";
import { useRetailQuery } from "../retail/useRetailQuery";
import TornadoChart from "../components/charts/TornadoChart";
import { fmtMoney } from "../lib/format";
import { useAuth } from "../context/AuthContext";
import { useLanguage } from "../context/LanguageContext";
import { t } from "../lib/i18n";
import { motion } from "framer-motion";
import { useUrlState } from "../hooks/useUrlState";
import { notifyError } from "../lib/notify";

interface ContractOption {
  id: string;
  contract_number: string;
  contract_name: string;
  lease_scope?: string;
}

interface ScenarioRow {
  scenario_name: string;
  discount_rate: number;
  rate_delta: number;
  initial_liability: number;
  initial_rou_asset: number;
  liability_delta: number;
  liability_delta_percent: number;
}

const fmt = (value: number) => value.toLocaleString(undefined, { maximumFractionDigits: 2 });

function SensitivityPage() {
  const { token } = useAuth();
  const { language } = useLanguage();
  const [form] = Form.useForm();
  const [contractParam, setContractParam] = useUrlState("contract_id", "");
  const [baseRateParam, setBaseRateParam] = useUrlState("base_rate", "");
  const [shockParam, setShockParam] = useUrlState("shock", "1");
  const [rows, setRows] = useState<ScenarioRow[]>([]);
  const [meta, setMeta] = useState<any>(null);
  const [loading, setLoading] = useState(false);

  const summary = useMemo(() => {
    if (!rows.length) return { base: null, maxUp: null, maxDown: null };
    const base = rows.find((row) => Math.abs(row.rate_delta) < 0.0000001)?.initial_liability ?? rows[0].initial_liability;
    return rows.reduce(
      (acc, row) => {
        acc.maxUp = Math.max(acc.maxUp, row.liability_delta);
        acc.maxDown = Math.min(acc.maxDown, row.liability_delta);
        return acc;
      },
      { base, maxUp: 0, maxDown: 0 }
    );
  }, [rows]);

  const tornadoFactors = useMemo(() => {
    const shockMagnitudes = Array.from(new Set(rows.map((r) => Math.abs(r.rate_delta)).filter((d) => d > 0.0001)));
    return shockMagnitudes.map((mag) => {
      const upRow = rows.find((r) => Math.abs(r.rate_delta - mag) < 0.0001);
      const downRow = rows.find((r) => Math.abs(r.rate_delta + mag) < 0.0001);
      return {
        name: t("sensitivity.rate_shock", language, { percent: (mag * 100).toFixed(1) }),
        lowValue: downRow?.initial_liability ?? summary.base ?? 0,
        highValue: upRow?.initial_liability ?? summary.base ?? 0,
      };
    });
  }, [rows, summary.base, language]);

  // FETCH-003: the approved-contract dropdown runs through the shared
  // fetch seam (race gate / token injection / error exit).
  const { state: contractsState } = useRetailQuery({
    token,
    params: { status: "approved" as const, sort_by: "created_at" as const, sort_order: "desc" as const },
    paramsKey: "approved-contracts",
    fetcher: (p, t) => contractApi.list<{ data?: ContractOption[] }>(t, p).then((res) => res.data ?? []),
  });
  const contracts: ContractOption[] = contractsState.kind === "ready" ? (contractsState.data ?? []) : [];

  const runAnalysis = async (values: any) => {
    if (!token) return;
    setLoading(true);
    try {
      const shock = values.shock_percent / 100;
      setContractParam(values.contract_id || "");
      setBaseRateParam(values.base_rate_percent == null ? "" : String(values.base_rate_percent));
      setShockParam(values.shock_percent == null ? "1" : String(values.shock_percent));
      const res = await reportApi.sensitivity<{ data?: ScenarioRow[] }>(
        {
          contract_id: values.contract_id,
          base_rate: values.base_rate_percent ? values.base_rate_percent / 100 : undefined,
          shocks: [-shock, -shock / 2, 0, shock / 2, shock].join(","),
        },
        token
      );
      setRows(res.data || []);
      setMeta(res);
      message.success(t("sensitivity.generated", language));
    } catch (error: any) {
      notifyError(error.message || t("sensitivity.failed", language));
    } finally {
      setLoading(false);
    }
  };

  return (
    <ProtectedRoute>
      <AppLayout>
        <motion.div initial={false} animate={{ opacity: 1, y: 0 }} transition={{ duration: 0.2 }}>
          <div className="sensitivity-page-stack">
            <PageHeader title={t("sensitivity.title", language)} />

            <Card>
              <Form
                form={form}
                layout="inline"
                onFinish={runAnalysis}
                initialValues={{
                  contract_id: contractParam || undefined,
                  base_rate_percent: baseRateParam ? Number(baseRateParam) : undefined,
                  shock_percent: Number(shockParam) || 1,
                }}
              >
                <Form.Item name="contract_id" rules={[{ required: true, message: t("sensitivity.contract_required", language) }]}>
                  <Select
                    showSearch
                    className="sensitivity-contract-select"
                    placeholder={t("sensitivity.select_approved", language)}
                    optionFilterProp="label"
                    options={contracts.map((contract) => ({
                      value: contract.id,
                      label: `${contract.contract_number} · ${contract.contract_name}`,
                    }))}
                  />
                </Form.Item>
                <Form.Item name="base_rate_percent" label={t("sensitivity.base_rate", language)}>
                  <InputNumber min={0} max={50} precision={2} addonAfter="%" placeholder={t("sensitivity.rate_placeholder", language)} />
                </Form.Item>
                <Form.Item name="shock_percent" label={t("sensitivity.shock", language)} rules={[{ required: true, message: t("sensitivity.shock_required", language) }]}>
                  <InputNumber min={0.1} max={10} precision={2} addonAfter="%" />
                </Form.Item>
                <Form.Item>
                  <Button type="primary" htmlType="submit" loading={loading} icon={<CalculatorOutlined />}>
                    {t("sensitivity.generate", language)}
                  </Button>
                </Form.Item>
              </Form>
            </Card>

            {meta && (
              <div className="card-context sensitivity-result-meta" role="status">
                <strong>{meta.contract_number}</strong>
                <span>{meta.contract_name}</span>
                <span>{t("sensitivity.base_rate", language)}: {typeof meta.base_rate === "number" ? `${(meta.base_rate * 100).toFixed(2)}%` : "—"}</span>
                <span>{t("sensitivity.scope", language)}: {meta.lease_scope || "—"}</span>
                <span>{t("sensitivity.currency", language)}: {meta.currency || "—"}</span>
              </div>
            )}

            <div className="stripe-metric-grid sensitivity-summary-grid">
              <div className="pulse-kpi-card sensitivity-summary-card">
                <span className="sensitivity-summary-label">{t("sensitivity.base_liability", language)}</span>
                <Typography.Text className="font-tabular sensitivity-summary-value">
                  {fmtMoney(summary.base, meta?.currency)}
                </Typography.Text>
              </div>
              <div className="pulse-kpi-card sensitivity-summary-card">
                <span className="sensitivity-summary-label">{t("sensitivity.max_up", language)}</span>
                <Typography.Text className={`font-tabular sensitivity-summary-value${summary.maxUp != null && summary.maxUp > 0 ? " is-negative" : ""}`}>
                  {fmtMoney(summary.maxUp, meta?.currency)}
                </Typography.Text>
              </div>
              <div className="pulse-kpi-card sensitivity-summary-card">
                <span className="sensitivity-summary-label">{t("sensitivity.max_down", language)}</span>
                <Typography.Text className={`font-tabular sensitivity-summary-value${summary.maxDown != null && summary.maxDown < 0 ? " is-positive" : ""}`}>
                  {fmtMoney(summary.maxDown, meta?.currency)}
                </Typography.Text>
              </div>
            </div>

            {rows.length > 0 && (
              <Card title={t("sensitivity.chart_title", language)}>
                <TornadoChart factors={tornadoFactors} baseValue={summary.base ?? 0} currency={meta?.currency || "CNY"} height={280} />
              </Card>
            )}

            <Card title={t("sensitivity.detail_title", language)}>
              <Table
                loading={loading}
                dataSource={rows}
                rowKey="scenario_name"
                pagination={false}
                size="small"
                columns={[
                  { title: t("sensitivity.scenario", language), dataIndex: "scenario_name", width: 100, render: (v: string) => <StatusTag kind={v === "+0.00%" ? "success" : "processing"}>{v}</StatusTag> },
                  { title: t("sensitivity.rate", language), dataIndex: "discount_rate", width: 100, render: (v: number) => `${(v * 100).toFixed(2)}%` },
                  { title: t("sensitivity.initial_liability", language), dataIndex: "initial_liability", align: "right" as const, render: (v: number) => fmt(v) },
                  { title: t("sensitivity.rou_asset", language), dataIndex: "initial_rou_asset", align: "right" as const, render: (v: number) => fmt(v) },
                  { title: t("sensitivity.liability_delta", language), dataIndex: "liability_delta", align: "right" as const, render: (v: number) => fmt(v) },
                  { title: t("sensitivity.delta_percent", language), dataIndex: "liability_delta_percent", align: "right" as const, render: (v: number) => `${(v * 100).toFixed(2)}%` },
                ]}
              />
            </Card>
          </div>
        </motion.div>
      </AppLayout>
    </ProtectedRoute>
  );
}

export default function SensitivityPageWithUrlState() {
  return (
    <Suspense fallback={<div className="sensitivity-loading" />}>
      <SensitivityPage />
    </Suspense>
  );
}
